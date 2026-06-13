package payments

import (
	"context"
	"fmt"
)

// Service implements orders.PaymentInitiator.
// It resolves the correct provider from the registry and delegates to it.
type Service struct {
	registry        *Registry
	stores          StoreCredentialsReader
	mpesaCallbackURL string // POST /api/v1/payments/mpesa/callback/:storeID
	airtelCallbackURL string // POST /api/v1/payments/airtel/callback/:storeID
	cardCallbackURL  string // POST /api/v1/payments/card/callback (Pesapal IPN)
}

// ServiceConfig groups all callback URLs needed by the payment service.
type ServiceConfig struct {
	MpesaCallbackURL  string // from cfg.Mpesa.CallbackURL
	AirtelCallbackURL string // from cfg.Airtel.CallbackURL (if separate) or derived
	CardCallbackURL   string // from cfg.Card.CallbackURL
}

func NewService(registry *Registry, stores StoreCredentialsReader, cfg ServiceConfig) *Service {
	return &Service{
		registry:         registry,
		stores:           stores,
		mpesaCallbackURL:  cfg.MpesaCallbackURL,
		airtelCallbackURL: cfg.AirtelCallbackURL,
		cardCallbackURL:   cfg.CardCallbackURL,
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
		PaymentChannel: req.PaymentChannel,
	}

	resp, err := provider.Initiate(ctx, payReq)
	if err != nil {
		return nil, fmt.Errorf("payments: %s initiation failed: %w", req.Provider, err)
	}

	return &InitiateResult{
		ProviderRef:     resp.ProviderRef,
		Instructions:    resp.Instructions,
		AwaitingPayment: resp.AwaitingPayment,
		RedirectURL:     resp.RedirectURL,
	}, nil
}

// AvailableProviders returns the list of registered payment method names.
// Shown to the customer at checkout so they can pick a method.
func (s *Service) AvailableProviders() []string {
	return s.registry.Names()
}

// buildCallbackURL constructs the callback endpoint for a specific provider.
//
// For M-Pesa and Airtel, the storeID is appended so the callback handler knows
// which store's credentials to use when confirming the payment.
//
// For Pesapal (card), no storeID is appended — Pesapal sends the order reference
// in the payload body, which is how the order is looked up.
func (s *Service) buildCallbackURL(provider, storeID string) string {
	switch provider {
	case "mpesa":
		return fmt.Sprintf("%s/%s", s.mpesaCallbackURL, storeID)
	case "airtel":
		return fmt.Sprintf("%s/%s", s.airtelCallbackURL, storeID)
	case "card":
		return s.cardCallbackURL // Pesapal does not need storeID in the URL
	default:
		return fmt.Sprintf("%s/%s", s.mpesaCallbackURL, storeID)
	}
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
	PaymentChannel PaymentChannel
}

// InitiateResult mirrors orders.PaymentInitResult.
type InitiateResult struct {
	ProviderRef     string
	Instructions    string
	AwaitingPayment bool
	RedirectURL     string
}