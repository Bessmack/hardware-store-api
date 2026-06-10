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

// ── HandleCallback — process Airtel's async result ────────────────────────────

// HandleCallback parses the USSD push result posted by Airtel's servers.
// Status code "TS" = Transaction Successful; anything else = failed.
func (p *Provider) HandleCallback(_ context.Context, storeID string, rawPayload []byte) (*payments.PaymentResponse, error) {
	var callback struct {
		Transaction struct {
			ID            string `json:"id"`
			Message       string `json:"message"`
			StatusCode    string `json:"status_code"`    // "TS" = success
			AirtelMoneyID string `json:"airtel_money_id"` // Airtel's internal reference
		} `json:"transaction"`
	}

	if err := json.Unmarshal(rawPayload, &callback); err != nil {
		return nil, fmt.Errorf("airtel: failed to parse callback: %w", err)
	}

	t := callback.Transaction
	logger.Get().Info().
		Str("transaction_id", t.ID).
		Str("status_code", t.StatusCode).
		Str("airtel_money_id", t.AirtelMoneyID).
		Str("store_id", storeID).
		Msg("airtel: callback received")

	if t.StatusCode != "TS" {
		return &payments.PaymentResponse{
			ProviderRef:   t.ID,
			Status:        "failed",
			FailureReason: t.Message,
		}, nil
	}

	return &payments.PaymentResponse{
		ProviderRef:     t.ID,
		Status:          "success",
		AwaitingPayment: false,
	}, nil
}

// ── OAuth token management ────────────────────────────────────────────────────

func (p *Provider) getAccessToken(ctx context.Context) (string, error) {
	if token, err := p.cache.Get(ctx, tokenCacheKey); err == nil && token != "" {
		return token, nil
	}

	body := map[string]string{
		"client_id":     p.clientID,
		"client_secret": p.clientSecret,
		"grant_type":    "client_credentials",
	}

	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/auth/oauth2/token", bytes.NewBuffer(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("airtel: token request failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("airtel: failed to decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("airtel: received empty access token")
	}

	_ = p.cache.Set(ctx, tokenCacheKey, tokenResp.AccessToken, tokenTTL)
	return tokenResp.AccessToken, nil
}

// ── HTTP helper ───────────────────────────────────────────────────────────────

func (p *Provider) post(ctx context.Context, path, token, country, currency string, body interface{}) ([]byte, error) {
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
	req.Header.Set("X-Country", country)
	req.Header.Set("X-Currency", currency)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// sanitizePhone ensures the phone number is in the format Airtel expects.
// Strips leading + or 0 and ensures the international prefix is present.
func sanitizePhone(phone string) string {
	if len(phone) == 0 {
		return phone
	}
	if phone[0] == '+' {
		return phone[1:]
	}
	if phone[0] == '0' {
		return "254" + phone[1:] // Kenya default — extend for other countries
	}
	return phone
}

// currencyToCountry maps ISO 4217 currency codes to ISO 3166-1 alpha-2 country codes.
// Airtel requires both the country and currency headers on every request.
func currencyToCountry(currency string) string {
	m := map[string]string{
		"KES": "KE", // Kenya
		"UGX": "UG", // Uganda
		"TZS": "TZ", // Tanzania
		"RWF": "RW", // Rwanda
		"MWK": "MW", // Malawi
		"ZMW": "ZM", // Zambia
		"XAF": "CG", // Congo
		"MDG": "MG", // Madagascar
	}
	if country, ok := m[currency]; ok {
		return country
	}
	return "KE" // default
}