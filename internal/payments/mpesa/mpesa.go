package mpesa

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/payments"
	"github.com/Bessmack/hardware-store-api/pkg/cache"
	"github.com/Bessmack/hardware-store-api/pkg/logger"
)

// tokenCacheKey is the Redis key used to cache the Daraja OAuth token.
const tokenCacheKey = "mpesa:access_token"

// tokenTTL is slightly less than Safaricom's 1-hour token lifetime.
// Caching avoids a round-trip to Safaricom on every payment initiation.
const tokenTTL = 50 * time.Minute

// Provider implements payments.Provider for Safaricom M-Pesa (Daraja API).
type Provider struct {
	consumerKey      string
	consumerSecret   string
	baseURL          string
	defaultShortcode string // global fallback if store has no own shortcode
	defaultPasskey   string // global fallback if store has no own passkey
	httpClient       *http.Client
	cache            *cache.Cache
	storeCreds       payments.StoreCredentialsReader
}

// Config is populated from config.MpesaConfig in main.go.
type Config struct {
	ConsumerKey      string
	ConsumerSecret   string
	BaseURL          string
	DefaultShortcode string
	DefaultPasskey   string
}

func New(cfg Config, c *cache.Cache, storeCreds payments.StoreCredentialsReader) *Provider {
	return &Provider{
		consumerKey:      cfg.ConsumerKey,
		consumerSecret:   cfg.ConsumerSecret,
		baseURL:          cfg.BaseURL,
		defaultShortcode: cfg.DefaultShortcode,
		defaultPasskey:   cfg.DefaultPasskey,
		httpClient:       &http.Client{Timeout: 15 * time.Second},
		cache:            c,
		storeCreds:       storeCreds,
	}
}

func (p *Provider) Name() string { return "mpesa" }

// ── Initiate — STK Push ───────────────────────────────────────────────────────

// Initiate sends an STK push to the customer's phone.
// The customer sees a payment prompt and enters their M-Pesa PIN to confirm.
// Safaricom then POSTs the result to the store-specific callback URL.
func (p *Provider) Initiate(ctx context.Context, req payments.PaymentRequest) (*payments.PaymentResponse, error) {
	// Resolve store-specific credentials, fall back to global defaults
	shortcode, passkey, accountRef, err := p.resolveCredentials(ctx, req.StoreID)
	if err != nil {
		return nil, err
	}

	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("mpesa: failed to get access token: %w", err)
	}

	timestamp := time.Now().Format("20060102150405")
	password := base64.StdEncoding.EncodeToString(
		[]byte(shortcode + passkey + timestamp),
	)

	if accountRef == "" {
		accountRef = req.OrderID[:8] // fallback to first 8 chars of order ID
	}

	body := map[string]interface{}{
		"BusinessShortCode": shortcode,
		"Password":          password,
		"Timestamp":         timestamp,
		"TransactionType":   "CustomerPayBillOnline",
		"Amount":            int(req.Amount), // M-Pesa requires integer amounts
		"PartyA":            req.Phone,
		"PartyB":            shortcode,
		"PhoneNumber":       req.Phone,
		"CallBackURL":       req.CallbackURL,
		"AccountReference":  accountRef,
		"TransactionDesc":   req.Description,
	}

	respBody, err := p.post(ctx, "/mpesa/stkpush/v1/processrequest", token, body)
	if err != nil {
		return nil, fmt.Errorf("mpesa: STK push request failed: %w", err)
	}

	var stkResp struct {
		CheckoutRequestID   string `json:"CheckoutRequestID"`
		ResponseCode        string `json:"ResponseCode"`
		ResponseDescription string `json:"ResponseDescription"`
		CustomerMessage     string `json:"CustomerMessage"`
	}
	if err := json.Unmarshal(respBody, &stkResp); err != nil {
		return nil, fmt.Errorf("mpesa: failed to decode STK response: %w", err)
	}

	if stkResp.ResponseCode != "0" {
		return nil, fmt.Errorf("mpesa: STK push rejected — %s", stkResp.ResponseDescription)
	}

	return &payments.PaymentResponse{
		ProviderRef:     stkResp.CheckoutRequestID,
		Status:          "pending",
		Instructions:    "Please check your phone for an M-Pesa payment prompt and enter your PIN to complete the payment.",
		AwaitingPayment: true,
	}, nil
}

// ── HandleCallback — process Safaricom's async result ─────────────────────────

// HandleCallback parses the STK push result posted by Safaricom.
// ResultCode 0 = payment approved. Anything else = cancelled or failed.
func (p *Provider) HandleCallback(ctx context.Context, storeID string, rawPayload []byte) (*payments.PaymentResponse, error) {
	var callback struct {
		Body struct {
			StkCallback struct {
				MerchantRequestID string `json:"MerchantRequestID"`
				CheckoutRequestID string `json:"CheckoutRequestID"`
				ResultCode        int    `json:"ResultCode"`
				ResultDesc        string `json:"ResultDesc"`
				CallbackMetadata  struct {
					Item []struct {
						Name  string      `json:"Name"`
						Value interface{} `json:"Value"`
					} `json:"Item"`
				} `json:"CallbackMetadata"`
			} `json:"stkCallback"`
		} `json:"Body"`
	}

	if err := json.Unmarshal(rawPayload, &callback); err != nil {
		return nil, fmt.Errorf("mpesa: failed to parse callback payload: %w", err)
	}

	stk := callback.Body.StkCallback

	logger.Get().Info().
		Str("checkout_request_id", stk.CheckoutRequestID).
		Int("result_code", stk.ResultCode).
		Str("result_desc", stk.ResultDesc).
		Msg("mpesa: callback received")

	if stk.ResultCode != 0 {
		return &payments.PaymentResponse{
			ProviderRef:   stk.CheckoutRequestID,
			Status:        "failed",
			FailureReason: stk.ResultDesc,
		}, nil
	}

	return &payments.PaymentResponse{
		ProviderRef:     stk.CheckoutRequestID,
		Status:          "success",
		AwaitingPayment: false,
	}, nil
}

// ── OAuth token management ─────────────────────────────────────────────────────

// getAccessToken returns a cached token or fetches a fresh one from Safaricom.
func (p *Provider) getAccessToken(ctx context.Context) (string, error) {
	// Try cache first
	if token, err := p.cache.Get(ctx, tokenCacheKey); err == nil && token != "" {
		return token, nil
	}

	// Request new token
	url := p.baseURL + "/oauth/v1/generate?grant_type=client_credentials"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	creds := base64.StdEncoding.EncodeToString(
		[]byte(p.consumerKey + ":" + p.consumerSecret),
	)
	req.Header.Set("Authorization", "Basic "+creds)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("mpesa: token request failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("mpesa: failed to decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("mpesa: received empty access token")
	}

	// Cache for 50 minutes (token valid for 60)
	_ = p.cache.Set(ctx, tokenCacheKey, tokenResp.AccessToken, tokenTTL)

	return tokenResp.AccessToken, nil
}

// ── Credential resolution ─────────────────────────────────────────────────────

// resolveCredentials returns the shortcode, passkey, and account reference
// for the given store. Falls back to global defaults if the store has not
// configured its own credentials.
func (p *Provider) resolveCredentials(ctx context.Context, storeID string) (shortcode, passkey, accountRef string, err error) {
	creds, err := p.storeCreds.GetPaymentCredentials(ctx, storeID)
	if err == nil && creds.MpesaShortcode != "" && creds.MpesaPasskey != "" {
		return creds.MpesaShortcode, creds.MpesaPasskey, creds.MpesaAccountRef, nil
	}

	// Fall back to global defaults from .env
	if p.defaultShortcode == "" || p.defaultPasskey == "" {
		return "", "", "", fmt.Errorf("mpesa: no credentials configured for store %q and no global defaults set", storeID)
	}
	return p.defaultShortcode, p.defaultPasskey, "", nil
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
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}