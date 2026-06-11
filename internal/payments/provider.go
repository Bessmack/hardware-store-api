package payments

import "context"

// ── Provider interface ────────────────────────────────────────────────────────

// Provider is the contract every payment channel must satisfy.
// Adding a new provider (T-Kash, card gateway, etc.) means creating a new
// file that implements this interface and registering it — nothing else changes.
type Provider interface {
	// Name returns the unique identifier for this provider.
	// Must match the payment_provider value stored on the orders table.
	Name() string

	// Initiate triggers a payment request and returns a provider reference
	// (e.g. M-Pesa CheckoutRequestID) used to match the incoming callback.
	Initiate(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)

	// HandleCallback processes an incoming webhook from the payment provider
	// and returns a normalised response. rawPayload is the raw request body.
	HandleCallback(ctx context.Context, storeID string, rawPayload []byte) (*PaymentResponse, error)
}

// ── Shared types ──────────────────────────────────────────────────────────────

// PaymentRequest is the normalised input passed to every provider.
type PaymentRequest struct {
	OrderID     string
	StoreID     string
	Amount      float64
	Currency    string
	Phone       string // required for mobile money providers
	Description string
	CallbackURL string // resolved per-provider in service.go
}

// PaymentResponse is the normalised output from every provider.
type PaymentResponse struct {
	ProviderRef     string // provider's transaction/checkout reference
	Status          string // "pending" | "success" | "failed"
	Instructions    string // shown to the customer (e.g. "Check your phone")
	AwaitingPayment bool   // true = async (mobile money); false = sync (card)
	FailureReason   string // populated when Status == "failed"
	// RedirectURL is populated by card providers — the frontend redirects the customer to this URL to complete payment on the hosted checkout page.
	RedirectURL string
}

// StoreCredentials holds a store's payment credentials.
// Read from the stores table — each store has its own paybill.
type StoreCredentials struct {
	// M-Pesa
	MpesaShortcode   string
	MpesaPasskey     string
	MpesaAccountRef  string
	MpesaConsumerKey    string
	MpesaConsumerSecret   string

	// Airtel Money
	AirtelMerchantID string

	// Store currency — carried here so providers know which currency to record
	Currency string
}

// StoreCredentialsReader fetches a store's payment credentials.
// Implemented by stores.Repository — defined as an interface here so the
// payments package stays decoupled from the stores package.
type StoreCredentialsReader interface {
	GetPaymentCredentials(ctx context.Context, storeID string) (*StoreCredentials, error)
}

// ── PaymentConfirmer ──────────────────────────────────────────────────────────

// PaymentConfirmer is called by callback handlers when a payment result arrives.
// Implemented by orders.Service — defined here to break the circular import:
//
//	orders → payments (PaymentInitiator, defined in orders package)
//	payments → orders would be circular — so we use this interface instead
//
// main.go passes orderService (which satisfies this interface) to NewHandler.
type PaymentConfirmer interface {
	// ConfirmPayment advances the order to "confirmed" and notifies the customer.
	// providerRef is used to locate the order (M-Pesa CheckoutRequestID, etc.).
	ConfirmPayment(ctx context.Context, providerRef string) error

	// FailPayment marks the order payment as failed.
	// The order stays in "placed" state so the customer can retry.
	FailPayment(ctx context.Context, providerRef string) error
}