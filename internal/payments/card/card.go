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