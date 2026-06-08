package payments

import (
	"io"
	"net/http"

	"github.com/Bessmack/hardware-store-api/pkg/logger"
	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/go-chi/chi/v5"
)

// Handler processes incoming payment callbacks from Safaricom and Airtel.
// All callback routes are protected by the IP allowlist middleware in
// internal/middleware/ip_allowlist.go — requests from unknown IPs are
// rejected before they reach any handler logic.
type Handler struct {
	registry  *Registry
	confirmer PaymentConfirmer // orders.Service satisfies this interface
}

func NewHandler(registry *Registry, confirmer PaymentConfirmer) *Handler {
	return &Handler{registry: registry, confirmer: confirmer}
}

// ── Routes (registered in server/routes.go) ───────────────────────────────────
//
// These routes sit behind AllowSafaricom / AllowAirtel IP allowlist middleware.
// They must also NOT be rate-limited — Safaricom controls the call frequency.
//
//   POST /api/v1/payments/mpesa/callback/:storeID
//   POST /api/v1/payments/airtel/callback/:storeID
//
// GET  /api/v1/payments/methods   — public, lists available payment methods

// MpesaCallback receives the STK push result from Safaricom's servers.
//
// Safaricom sends a JSON body to this endpoint after the customer
// approves or rejects the payment prompt on their phone.
// ResultCode 0 = success; anything else = failure or cancellation.
//
// This endpoint must always respond 200 OK — Safaricom retries if it
// does not receive a success response.
func (h *Handler) MpesaCallback(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "storeID")
	h.handleCallback(w, r, "mpesa", storeID)
}

// AirtelCallback receives the payment result from Airtel Money's servers.
func (h *Handler) AirtelCallback(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "storeID")
	h.handleCallback(w, r, "airtel", storeID)
}

// AvailableMethods returns the list of payment providers registered at startup.
// The checkout page calls this to render the payment method selector.
func (h *Handler) AvailableMethods(w http.ResponseWriter, r *http.Request) {
	response.Success(w, h.registry.Names())
}

// ── Shared callback logic ─────────────────────────────────────────────────────

func (h *Handler) handleCallback(w http.ResponseWriter, r *http.Request, providerName, storeID string) {
	l := logger.Get()

	// Read raw body — passed to the provider's HandleCallback unchanged
	body, err := io.ReadAll(r.Body)
	if err != nil {
		l.Error().Err(err).Str("provider", providerName).Msg("payments: failed to read callback body")
		// Always respond 200 to prevent retries for malformed requests
		w.WriteHeader(http.StatusOK)
		return
	}

	// Locate the correct provider
	provider, err := h.registry.Get(providerName)
	if err != nil {
		l.Error().Err(err).Str("provider", providerName).Msg("payments: unknown provider in callback")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Delegate payload parsing to the provider
	result, err := provider.HandleCallback(r.Context(), storeID, body)
	if err != nil {
		l.Error().Err(err).
			Str("provider", providerName).
			Str("store", storeID).
			Msg("payments: callback processing error")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Act on the result
	switch result.Status {
	case "success":
		if err := h.confirmer.ConfirmPayment(r.Context(), result.ProviderRef); err != nil {
			l.Error().Err(err).
				Str("provider_ref", result.ProviderRef).
				Msg("payments: failed to confirm order after successful payment")
		}

	case "failed":
		l.Warn().
			Str("provider", providerName).
			Str("provider_ref", result.ProviderRef).
			Str("reason", result.FailureReason).
			Msg("payments: payment failed or was cancelled by customer")

		if err := h.confirmer.FailPayment(r.Context(), result.ProviderRef); err != nil {
			l.Error().Err(err).
				Str("provider_ref", result.ProviderRef).
				Msg("payments: failed to mark order as payment-failed")
		}
	}

	// Always respond 200 — payment providers retry on non-200 responses
	// which would create duplicate confirmations.
	w.WriteHeader(http.StatusOK)
}