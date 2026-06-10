package payments

import (
	"context"
	"fmt"

	"github.com/Bessmack/hardware-store-api/internal/config"
)

// Service implements orders.PaymentInitiator.
// It resolves the correct provider from the registry and delegates to it.
type Service struct {
	registry *Registry
	stores   StoreCredentialsReader
	cfg      config.MpesaConfig // used to build per-provider callback URLs
	appURL   string             // base URL for callback construction
}

func NewService(registry *Registry, stores StoreCredentialsReader, cfg config.MpesaConfig, appURL string) *Service {
	return &Service{
		registry: registry,
		stores:   stores,
		cfg:      cfg,
		appURL:   appURL,
	}
}

// Initiate satisfies the orders.PaymentInitiator interface.
// Called by orders.Service.PlaceOrder after the order record is persisted.
func (s *Service) Initiate(ctx context.Context, req InitiateRequest) (*InitiateResult, error) {
	provider, err := s.registry.Get(req.Provider)
	if err != nil {
		return nil, err
	}

	// Build the store-specific callback URL so Safaricom/Airtel know
	// which store's payment they are confirming.
	callbackURL := s.buildCallbackURL(req.Provider, req.StoreID)

	payReq := PaymentRequest{
		OrderID:     req.OrderID,
		StoreID:     req.StoreID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Phone:       req.Phone,
		Description: req.Description,
		CallbackURL: callbackURL,
	}

	resp, err := provider.Initiate(ctx, payReq)
	if err != nil {
		return nil, fmt.Errorf("payments: %s initiation failed: %w", req.Provider, err)
	}

	return &InitiateResult{
		ProviderRef:     resp.ProviderRef,
		Instructions:    resp.Instructions,
		AwaitingPayment: resp.AwaitingPayment,
	}, nil
}

// AvailableProviders returns the list of registered payment method names.
// Shown to the customer at checkout so they can pick a method.
func (s *Service) AvailableProviders() []string {
	return s.registry.Names()
}

// buildCallbackURL constructs the HTTPS endpoint for a specific provider + store.
// Safaricom/Airtel will POST payment results to this URL.
// Format: {MPESA_CALLBACK_URL}/{storeID}
// e.g. https://ngrok-id.ngrok-free.app/api/v1/payments/mpesa/callback/store-uuid
func (s *Service) buildCallbackURL(provider, storeID string) string {
	return fmt.Sprintf("%s/%s - %s", s.cfg.CallbackURL, storeID, provider)
}

// ── Request/response types used by orders.PaymentInitiator ───────────────────
// Defined here (not in provider.go) because they are specific to the service
// layer, not to individual providers.

// InitiateRequest mirrors orders.PaymentInitRequest — redefined here to keep
// the payments package self-contained and avoid importing orders.
type InitiateRequest struct {
	OrderID     string
	StoreID     string
	Amount      float64
	Currency    string
	Phone       string
	Provider    string // "mpesa" | "airtel" | "card"
	Description string
}

// InitiateResult mirrors orders.PaymentInitResult.
type InitiateResult struct {
	ProviderRef     string
	Instructions    string
	AwaitingPayment bool
}