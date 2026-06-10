package airtel

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

// Cache keys and TTLs for Airtel's OAuth token. The token is valid for 60 minutes, but we refresh it every 50 minutes to avoid edge cases where a token expires during a transaction.
const (
	tokenCacheKey = "airtel:access_token"
	tokenTTL      = 50 * time.Minute
)

// Provider implements payments.Provider for Airtel Money.
// Docs: https://developers.airtel.africa/documentation
type Provider struct {
	clientID     string
	clientSecret string
	baseURL      string
	httpClient   *http.Client
	cache        *cache.Cache
	storeCreds   payments.StoreCredentialsReader
}

type Config struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
}

func New(cfg Config, c *cache.Cache, storeCreds payments.StoreCredentialsReader) *Provider {
	return &Provider{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		baseURL:      cfg.BaseURL,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		cache:        c,
		storeCreds:   storeCreds,
	}
}

func (p *Provider) Name() string { return "airtel" }

// ── Initiate — USSD push ──────────────────────────────────────────────────────

// Initiate sends an Airtel Money payment prompt to the customer's phone.
// The customer receives a USSD push notification and confirms with their PIN.
func (p *Provider) Initiate(ctx context.Context, req payments.PaymentRequest) (*payments.PaymentResponse, error) {
	creds, err := p.storeCreds.GetPaymentCredentials(ctx, req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("airtel: failed to load store credentials: %w", err)
	}

	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("airtel: failed to get access token: %w", err)
	}

	// Airtel expects the currency from the store's configuration
	currency := creds.Currency
	if currency == "" {
		currency = "KES"
	}

	// Derive country code from currency (KES→KE, UGX→UG, TZS→TZ)
	country := currencyToCountry(currency)

	body := map[string]interface{}{
		"reference": req.OrderID,
		"subscriber": map[string]interface{}{
			"country":  country,
			"currency": currency,
			"msisdn":   sanitizePhone(req.Phone),
		},
		"transaction": map[string]interface{}{
			"amount":   req.Amount,
			"country":  country,
			"currency": currency,
			"id":       req.OrderID, // unique transaction identifier
		},
	}

	respBody, err := p.post(ctx, "/merchant/v2/payments/", token, country, currency, body)
	if err != nil {
		return nil, fmt.Errorf("airtel: payment request failed: %w", err)
	}

	var resp struct {
		Data struct {
			Transaction struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"transaction"`
		} `json:"data"`
		Status struct {
			Code        string `json:"code"`
			Message     string `json:"message"`
			ResultCode  string `json:"result_code"`
		} `json:"status"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("airtel: failed to decode response: %w", err)
	}

	if resp.Status.Code != "200" {
		return nil, fmt.Errorf("airtel: payment rejected — %s (%s)",
			resp.Status.Message, resp.Status.ResultCode)
	}

	return &payments.PaymentResponse{
		ProviderRef:     resp.Data.Transaction.ID,
		Status:          "pending",
		Instructions:    "Please check your phone for an Airtel Money payment request and enter your PIN to confirm.",
		AwaitingPayment: true,
	}, nil
}