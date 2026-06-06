package delivery

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Bessmack/hardware-store-api/internal/middleware"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/Bessmack/hardware-store-api/pkg/validator"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Routes (registered in server/routes.go) ───────────────────────────────────
//
// Public:
//   POST /api/v1/delivery/quote                    Calculate delivery options
//
// Admin + SuperAdmin (behind StoreScope):
//   GET  /api/v1/store/delivery-rates              List rates for scoped store
//   PUT  /api/v1/store/delivery-rates/:vehicle     Set store-specific rate
//   DELETE /api/v1/store/delivery-rates/:vehicle   Remove store override (revert to global)
//
// SuperAdmin only:
//   GET  /api/v1/delivery-rates                    List global default rates
//   PUT  /api/v1/delivery-rates/:vehicle           Update a global default rate

// Quote calculates delivery options and fees for a store → delivery address.
//
// Called at checkout after the customer confirms their delivery address.
// The frontend passes required_vehicle from the cart validation result so the
// correct option is pre-selected in the UI.
//
// Body: { store_id, lat, lng, vehicle_type?, required_vehicle? }
func (h *Handler) Quote(w http.ResponseWriter, r *http.Request) {
	var req QuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	quote, err := h.service.Quote(r.Context(), req)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, quote)
}

// ── Admin rate management ─────────────────────────────────────────────────────

// ListRates returns delivery rates for the scoped store.
// Shows which rates are store-specific vs using the global default.
func (h *Handler) ListRates(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}

	rates, err := h.service.ListRates(r.Context(), storeID, by)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, rates)
}

// UpsertStoreRate creates or replaces the delivery rate for one vehicle type
// at the scoped store, overriding the global default.
//
// Body: { base_fee, per_km, max_weight_kg?, max_radius_km? }
func (h *Handler) UpsertStoreRate(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	vehicleType := chi.URLParam(r, "vehicle")

	var req UpdateRateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	rate, err := h.service.UpsertStoreRate(r.Context(), storeID, vehicleType, req, by)
	if err != nil {
		response.Forbidden(w, err.Error())
		return
	}
	response.Success(w, rate)
}

// DeleteStoreRate removes a store-specific rate override.
// After deletion the store reverts to the global default for that vehicle.
func (h *Handler) DeleteStoreRate(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	vehicleType := chi.URLParam(r, "vehicle")

	if err := h.service.DeleteStoreRate(r.Context(), storeID, vehicleType, by); err != nil {
		if errors.Is(err, ErrRateNotFound) {
			response.NotFound(w, "no store-specific rate found for this vehicle")
			return
		}
		response.Forbidden(w, err.Error())
		return
	}
	response.NoContent(w)
}

// ── SuperAdmin rate management ────────────────────────────────────────────────

// ListGlobalRates returns all global default delivery rates. SuperAdmin only.
func (h *Handler) ListGlobalRates(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())

	rates, err := h.service.ListRates(r.Context(), "", by)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, rates)
}

// UpdateGlobalRate updates a global default delivery rate. SuperAdmin only.
// Affects all stores that have not overridden this vehicle type.
//
// Body: { base_fee, per_km, max_weight_kg?, max_radius_km? }
func (h *Handler) UpdateGlobalRate(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())
	vehicleType := chi.URLParam(r, "vehicle")

	var req UpdateRateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	rate, err := h.service.UpdateGlobalRate(r.Context(), vehicleType, req, by)
	if err != nil {
		if errors.Is(err, ErrRateNotFound) {
			response.NotFound(w, "global rate not found for this vehicle type")
			return
		}
		response.Forbidden(w, err.Error())
		return
	}
	response.Success(w, rate)
}