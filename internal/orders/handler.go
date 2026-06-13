package orders

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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
// Customer (RequireRole customer, superadmin):
//   POST /api/v1/orders                    Place an order
//   GET  /api/v1/orders                    List own orders
//   GET  /api/v1/orders/:id                Get own order
//   GET  /api/v1/orders/:id/track          Order tracking timeline
//   POST /api/v1/orders/:id/cancel         Cancel a placed order
//
// Staff (RequireRole cashier+, StoreScope):
//   GET  /api/v1/store/orders              List store orders (with filters)
//   GET  /api/v1/store/orders/:id          Get full order detail
//   PUT  /api/v1/store/orders/:id/status   Update order status

// ── Customer handlers ─────────────────────────────────────────────────────────

// PlaceOrder handles checkout. The delivery fee is always recalculated
// server-side — any fee the client sends is ignored.
//
// Body: { store_id, delivery_type, delivery_lat?, delivery_lng?,
//         delivery_address?, vehicle_type?, payment_provider, phone? }
func (h *Handler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	sessionID := r.Header.Get("X-Session-ID") // guest cart fallback

	var req PlaceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	// Extra validation: delivery fields are required when delivery_type = "delivery"
	if req.DeliveryType == "delivery" {
		if req.DeliveryLat == 0 || req.DeliveryLng == 0 {
			response.UnprocessableEntity(w, "delivery_lat and delivery_lng are required for home delivery")
			return
		}
		if req.VehicleType == "" {
			response.UnprocessableEntity(w, "vehicle_type is required for home delivery — validate your cart first")
			return
		}
		if req.PaymentProvider == "mpesa" || req.PaymentProvider == "airtel" {
			if req.Phone == "" {
				response.UnprocessableEntity(w, "phone is required for M-Pesa and Airtel Money payments")
				return
			}
		}
	}

	result, err := h.service.PlaceOrder(r.Context(), customer.ID, sessionID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, err.Error())
		default:
			response.UnprocessableEntity(w, err.Error())
		}
		return
	}

	response.Created(w, result)
}

// ListOwnOrders returns the authenticated customer's order history.
//
// Query params:
//   page     — page number (default 1)
//   per_page — items per page (default 20, max 50)
func (h *Handler) ListOwnOrders(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	orders, err := h.service.ListOwnOrders(r.Context(), customer.ID, page, perPage)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, orders)
}

// GetOwnOrder returns one of the customer's orders by ID.
func (h *Handler) GetOwnOrder(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	orderID := chi.URLParam(r, "id")

	order, err := h.service.GetOwnOrder(r.Context(), customer.ID, orderID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "order not found")
		case errors.Is(err, ErrForbidden):
			response.Forbidden(w, "this order does not belong to you")
		default:
			response.InternalServerError(w)
		}
		return
	}
	response.Success(w, order)
}

// TrackOrder returns the full status timeline for an order.
// This is the endpoint shown on the "Track my order" page.
func (h *Handler) TrackOrder(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	orderID := chi.URLParam(r, "id")

	tracking, err := h.service.TrackOrder(r.Context(), customer.ID, orderID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "order not found")
		case errors.Is(err, ErrForbidden):
			response.Forbidden(w, "this order does not belong to you")
		default:
			response.InternalServerError(w)
		}
		return
	}
	response.Success(w, tracking)
}

// CancelOrder lets a customer cancel their own order.

// Allowed when status is "placed" or "confirmed" — i.e. before the store starts packing. 
// Once the order reaches "preparing" or beyond, the customer must contact the store directly. Only staff can cancel at that point.
//
// Body: { reason? }
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	orderID := chi.URLParam(r, "id")

	var req CancelOrderRequest
	// Body is optional — decode but don't fail if missing
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.service.CancelOrder(r.Context(), customer.ID, orderID, req); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "order not found")
		case errors.Is(err, ErrForbidden):
			response.Forbidden(w, "this order does not belong to you")
		default:
			response.UnprocessableEntity(w, err.Error())
		}
		return
	}
	response.NoContent(w)
}

// ── Staff handlers ────────────────────────────────────────────────────────────

// ListForStore returns orders for the scoped store with optional filters.
//
// Query params:
//   status   — filter by order status (placed|confirmed|preparing|out_for_delivery|delivered|cancelled)
//   page     — page number (default 1)
//   per_page — items per page (default 20, max 50)
func (h *Handler) ListForStore(w http.ResponseWriter, r *http.Request) {
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	filters := OrderFilters{
		Status:  r.URL.Query().Get("status"),
		Page:    page,
		PerPage: perPage,
	}

	orders, err := h.service.ListForStore(r.Context(), storeID, filters)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, orders)
}

// GetForStore returns full order detail for staff, including customer contact
// info and internal status history notes.
func (h *Handler) GetForStore(w http.ResponseWriter, r *http.Request) {
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	orderID := chi.URLParam(r, "id")

	order, err := h.service.GetForStore(r.Context(), storeID, orderID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "order not found")
		case errors.Is(err, ErrForbidden):
			response.Forbidden(w, "this order belongs to a different store")
		default:
			response.InternalServerError(w)
		}
		return
	}
	response.Success(w, order)
}

// UpdateStatus advances an order through its lifecycle.
// Validates the transition is permitted before applying it.
//
// Allowed transitions:
//   placed         → confirmed | cancelled
//   confirmed      → preparing | cancelled
//   preparing      → out_for_delivery | cancelled
//   out_for_delivery → delivered   (normally triggered by POD, not directly)
//
// Body: { status, note? }
//
// Note: when status = "out_for_delivery", the POD domain must also be called
// to generate the delivery OTP and create the proof_of_delivery record.
// This coordination is done in server/routes.go by chaining both calls.
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	orderID := chi.URLParam(r, "id")

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	// Verify the order belongs to the scoped store before allowing the update
	order, err := h.service.GetForStore(r.Context(), storeID, orderID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "order not found")
		case errors.Is(err, ErrForbidden):
			response.Forbidden(w, "this order belongs to a different store")
		default:
			response.InternalServerError(w)
		}
		return
	}
	_ = order // used above for store ownership check

	_, err = h.service.UpdateStatus(r.Context(), orderID, req, by)
	if err != nil {
		switch {
		case errors.Is(err, ErrCannotTransition):
			response.UnprocessableEntity(w, err.Error())
		default:
			response.InternalServerError(w)
		}
		return
	}

	response.NoContent(w)
}