// Package card implements card (and mobile money) payments via Pesapal v3.
//
// Pesapal is widely used across Kenya, Uganda, Tanzania, and Rwanda.
// It accepts Visa, Mastercard, M-Pesa, Airtel Money, and bank transfers
// through a single hosted checkout page.
//
// Flow:
//  1. Initiate()        — registers IPN URL (once, cached), submits order,
//                         returns a redirect URL to Pesapal's hosted page
//  2. Customer pays     — on Pesapal's secure hosted checkout page
//  3. Pesapal IPN       — POSTs notification to /api/v1/payments/card/callback
//  4. HandleCallback()  — calls GetTransactionStatus to verify, returns result
//  5. Pesapal redirect  — sends customer to PESAPAL_REDIRECT_URL on frontend
//
// Docs: https://developer.pesapal.com/how-to-integrate/e-commerce/api-30/api-reference
package card

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/payments"
	"github.com/Bessmack/hardware-store-api/pkg/cache"
	"github.com/Bessmack/hardware-store-api/pkg/logger"
)

const (
	// tokenCacheKey holds the short-lived Pesapal OAuth token.
	// Pesapal tokens expire in ~5 minutes; we cache for 4 to be safe.
	tokenCacheKey = "pesapal:access_token"
	tokenTTL      = 4 * time.Minute

	// ipnIDCacheKey holds the registered IPN ID.
	// This only changes if you change the IPN URL — cache for 30 days.
	ipnIDCacheKey = "pesapal:ipn_id"
	ipnIDTTL      = 30 * 24 * time.Hour
)

// Provider implements payments.Provider for Pesapal v3.
type Provider struct {
	consumerKey    string
	consumerSecret string
	baseURL        string // sandbox or production
	callbackURL    string // IPN URL (our API endpoint)
	redirectURL    string // frontend URL after payment
	httpClient     *http.Client
	cache          *cache.Cache
}

// Config is populated from config.CardConfig in main.go.
type Config struct {
	ConsumerKey    string
	ConsumerSecret string
	BaseURL        string // https://cybqa.pesapal.com/pesapalv3 (sandbox) https://pay.pesapal.com/v3 (production)
	CallbackURL    string // https://yourapi.com/api/v1/payments/card/callback
	RedirectURL    string // https://yourstore.co.ke/payment/complete
}

func New(cfg Config, c *cache.Cache) *Provider {
	return &Provider{
		consumerKey:    cfg.ConsumerKey,
		consumerSecret: cfg.ConsumerSecret,
		baseURL:        cfg.BaseURL,
		callbackURL:    cfg.CallbackURL,
		redirectURL:    cfg.RedirectURL,
		httpClient:     &http.Client{Timeout: 15 * time.Second},
		cache:          c,
	}
}

func (p *Provider) Name() string { return "card" }

// ── Initiate — create hosted payment link ─────────────────────────────────────

// Initiate registers the IPN URL with Pesapal (once, cached), submits the order, and returns the redirect URL for the customer to complete payment.
func (p *Provider) Initiate(ctx context.Context, req payments.PaymentRequest) (*payments.PaymentResponse, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("pesapal: failed to get token: %w", err)
	}

	// Ensure the IPN URL is registered with Pesapal
	ipnID, err := p.ensureIPNRegistered(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("pesapal: failed to register IPN URL: %w", err)
	}

	body := map[string]interface{}{
		"id":              req.OrderID,
		"currency":        req.Currency,
		"amount":          req.Amount,
		"description":     req.Description,
		"callback_url":    p.redirectURL, // frontend redirect after payment
		"redirect_mode":   "",
		"notification_id": ipnID,
		"billing_address": map[string]string{
			"phone_number": req.Phone,
			"country_code": "KE",
		},
	}

	respBody, err := p.post(ctx, "/api/Transactions/SubmitOrderRequest", token, body)
	if err != nil {
		return nil, fmt.Errorf("pesapal: submit order failed: %w", err)
	}

	var resp struct {
		OrderTrackingID    string `json:"order_tracking_id"`
		MerchantReference  string `json:"merchant_reference"`
		RedirectURL        string `json:"redirect_url"`
		Error              *struct {
			Message string `json:"message"`
		} `json:"error"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("pesapal: failed to decode submit response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("pesapal: order rejected — %s", resp.Error.Message)
	}
	if resp.RedirectURL == "" {
		return nil, fmt.Errorf("pesapal: no redirect URL returned")
	}

	return &payments.PaymentResponse{
		ProviderRef:     resp.OrderTrackingID,
		Status:          "pending",
		Instructions:    "You will be redirected to a secure Pesapal page to complete your payment.",
		AwaitingPayment: false, // redirect-based, not push-based
		RedirectURL:     resp.RedirectURL,
	}, nil
}

// ── HandleCallback — verify via GetTransactionStatus ─────────────────────────

// HandleCallback receives Pesapal's IPN notification and verifies the result by calling GetTransactionStatus. Pesapal does not sign its IPN payloads;
// verification is done by pulling the definitive status from their API.
//
// IPN payload from Pesapal:
//
//	{
//	  "OrderTrackingId":         "b945e4af-...",
//	  "OrderMerchantReference":  "order-id",
//	  "OrderNotificationType":   "PAYMENT"
//	}
func (p *Provider) HandleCallback(ctx context.Context, _ string, rawPayload []byte) (*payments.PaymentResponse, error) {
	var ipn struct {
		OrderTrackingID       string `json:"OrderTrackingId"`
		OrderMerchantRef      string `json:"OrderMerchantReference"`
		OrderNotificationType string `json:"OrderNotificationType"`
	}
	if err := json.Unmarshal(rawPayload, &ipn); err != nil {
		return nil, fmt.Errorf("pesapal: failed to parse IPN: %w", err)
	}

	logger.Get().Info().
		Str("tracking_id", ipn.OrderTrackingID).
		Str("merchant_ref", ipn.OrderMerchantRef).
		Str("notification_type", ipn.OrderNotificationType).
		Msg("pesapal: IPN received")

	// Only process PAYMENT notifications
	if ipn.OrderNotificationType != "PAYMENT" {
		return &payments.PaymentResponse{
			ProviderRef: ipn.OrderTrackingID,
			Status:      "pending",
		}, nil
	}

	// Verify by calling GetTransactionStatus
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("pesapal: failed to get token for verification: %w", err)
	}

	status, err := p.getTransactionStatus(ctx, token, ipn.OrderTrackingID)
	if err != nil {
		return nil, fmt.Errorf("pesapal: failed to verify transaction: %w", err)
	}

	// Pesapal status codes: 1=COMPLETED, 2=FAILED, 3=REVERSED, 0=INVALID
	if status.StatusCode != 1 {
		return &payments.PaymentResponse{
			ProviderRef:   ipn.OrderTrackingID,
			Status:        "failed",
			FailureReason: status.PaymentStatusDescription,
		}, nil
	}

	return &payments.PaymentResponse{
		ProviderRef:     ipn.OrderTrackingID,
		Status:          "success",
		AwaitingPayment: false,
	}, nil
}

// ── Transaction status ────────────────────────────────────────────────────────

type transactionStatus struct {
	PaymentMethod              string  `json:"payment_method"`
	Amount                     float64 `json:"amount"`
	ConfirmationCode           string  `json:"confirmation_code"`
	MerchantReference          string  `json:"merchant_reference"`
	PaymentStatusDescription   string  `json:"payment_status_description"`
	StatusCode                 int     `json:"status_code"`
	Status                     string  `json:"status"`
}

func (p *Provider) getTransactionStatus(ctx context.Context, token, orderTrackingID string) (*transactionStatus, error) {
	url := fmt.Sprintf("%s/api/Transactions/GetTransactionStatus?orderTrackingId=%s",
		p.baseURL, orderTrackingID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pesapal: status check request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result transactionStatus
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("pesapal: failed to decode status response: %w", err)
	}
	return &result, nil
}

// ── IPN registration ──────────────────────────────────────────────────────────

// ensureIPNRegistered returns a cached IPN ID or registers the IPN URL and
// caches the result. The IPN ID is required on every SubmitOrderRequest.
func (p *Provider) ensureIPNRegistered(ctx context.Context, token string) (string, error) {
	// Return cached IPN ID if available
	if id, err := p.cache.Get(ctx, ipnIDCacheKey); err == nil && id != "" {
		return id, nil
	}

	body := map[string]string{
		"url":                    p.callbackURL,
		"ipn_notification_type":  "POST",
	}

	respBody, err := p.post(ctx, "/api/URLSetup/RegisterIPN", token, body)
	if err != nil {
		return "", fmt.Errorf("pesapal: IPN registration request failed: %w", err)
	}

	var resp struct {
		IPNID   string `json:"ipn_id"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("pesapal: failed to decode IPN response: %w", err)
	}
	if resp.IPNID == "" {
		return "", fmt.Errorf("pesapal: IPN registration failed — %s", resp.Message)
	}

	// Cache the IPN ID — it does not change unless the callback URL changes
	_ = p.cache.Set(ctx, ipnIDCacheKey, resp.IPNID, ipnIDTTL)

	logger.Get().Info().Str("ipn_id", resp.IPNID).Msg("pesapal: IPN URL registered")
	return resp.IPNID, nil
}

// ── OAuth token ───────────────────────────────────────────────────────────────

func (p *Provider) getToken(ctx context.Context) (string, error) {
	if token, err := p.cache.Get(ctx, tokenCacheKey); err == nil && token != "" {
		return token, nil
	}

	body := map[string]string{
		"consumer_key":    p.consumerKey,
		"consumer_secret": p.consumerSecret,
	}

	respBody, err := p.post(ctx, "/api/Auth/RequestToken", "", body)
	if err != nil {
		return "", fmt.Errorf("pesapal: token request failed: %w", err)
	}

	var resp struct {
		Token   string `json:"token"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("pesapal: failed to decode token response: %w", err)
	}
	if resp.Token == "" {
		return "", fmt.Errorf("pesapal: received empty token — %s", resp.Message)
	}

	_ = p.cache.Set(ctx, tokenCacheKey, resp.Token, tokenTTL)
	return resp.Token, nil
}

// ── HTTP helper ───────────────────────────────────────────────────────────────

func (p *Provider) post(ctx context.Context, path, token string, body interface{}) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}