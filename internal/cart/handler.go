package cart

import (
	"encoding/json"
	"errors"
	"net/http"

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
// All routes use OptionalAuth — guests and logged-in customers both have carts.
// The frontend sends X-Session-ID for guests; registered users are identified by JWT.
//
//   GET    /api/v1/cart                   Get cart
//   POST   /api/v1/cart/items             Add item
//   PUT    /api/v1/cart/items/:itemID     Update quantity
//   DELETE /api/v1/cart/items/:itemID     Remove item
//   DELETE /api/v1/cart                   Clear cart
//   GET    /api/v1/cart/validate          Pre-checkout validation

// GetCart returns the customer's or guest's current cart.
func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	customerID, sessionID := resolveIdentity(r)

	cart, err := h.service.GetCart(r.Context(), customerID, sessionID)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, cart)
}

// AddItem adds a product to the cart with the current price locked in.
//
// Body: { product_id, store_id, quantity }
func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	customerID, sessionID := resolveIdentity(r)

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	cart, err := h.service.AddItem(r.Context(), customerID, sessionID, req)
	if err != nil {
		if err.Error() == "this product is currently out of stock at the selected store" {
			response.UnprocessableEntity(w, err.Error())
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Created(w, cart)
}

// UpdateQuantity changes the quantity of a cart item.
//
// Body: { quantity }
func (h *Handler) UpdateQuantity(w http.ResponseWriter, r *http.Request) {
	customerID, sessionID := resolveIdentity(r)
	itemID := chi.URLParam(r, "itemID")

	var req UpdateQuantityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	cart, err := h.service.UpdateQuantity(r.Context(), customerID, sessionID, itemID, req.Quantity)
	if err != nil {
		if errors.Is(err, ErrItemNotFound) {
			response.NotFound(w, "cart item not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, cart)
}

// RemoveItem removes a single item from the cart.
func (h *Handler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	customerID, sessionID := resolveIdentity(r)
	itemID := chi.URLParam(r, "itemID")

	cart, err := h.service.RemoveItem(r.Context(), customerID, sessionID, itemID)
	if err != nil {
		if errors.Is(err, ErrItemNotFound) {
			response.NotFound(w, "cart item not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, cart)
}

// ClearCart removes all items from the cart.
func (h *Handler) ClearCart(w http.ResponseWriter, r *http.Request) {
	customerID, sessionID := resolveIdentity(r)

	if err := h.service.ClearCart(r.Context(), customerID, sessionID); err != nil {
		response.InternalServerError(w)
		return
	}
	response.NoContent(w)
}

// Validate runs pre-checkout validation and returns vehicle requirements.
// Call this before presenting the checkout summary to the customer.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	customerID, sessionID := resolveIdentity(r)

	result, err := h.service.ValidateCart(r.Context(), customerID, sessionID)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, result)
}

// ── Helper ────────────────────────────────────────────────────────────────────

// resolveIdentity extracts the customer ID (logged in) or session ID (guest)
// from the request. Both cannot be present simultaneously.
func resolveIdentity(r *http.Request) (customerID, sessionID string) {
	if user := users.UserFromContext(r.Context()); user != nil {
		return user.ID, ""
	}
	return "", r.Header.Get("X-Session-ID")
}