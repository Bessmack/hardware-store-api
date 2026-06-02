package wishlist

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/Bessmack/hardware-store-api/pkg/validator"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service         *Service
	locationService *geo.LocationService
}

func NewHandler(service *Service, locationService *geo.LocationService) *Handler {
	return &Handler{service: service, locationService: locationService}
}

// ── Routes (registered in server/routes.go) ───────────────────────────────────
//
// All routes require: RequireAuth + RequireRole(customer)
// Staff cannot wishlist — they have a separate work interface.
//
//   GET    /api/v1/wishlist                         List all wishlists
//   POST   /api/v1/wishlist                         Create a named wishlist
//   DELETE /api/v1/wishlist/:id                     Delete a wishlist
//   GET    /api/v1/wishlist/:id                     Get wishlist with live prices
//   POST   /api/v1/wishlist/:id/items               Add item
//   POST   /api/v1/wishlist/items                   Add to default wishlist (shortcut)
//   DELETE /api/v1/wishlist/:id/items/:itemID       Remove item

// ListAll returns all wishlists for the authenticated customer with item counts.
func (h *Handler) ListAll(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())

	summaries, err := h.service.ListAll(r.Context(), customer.ID)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, summaries)
}

// Create makes a new named wishlist.
//
// Body: { name }
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())

	var req CreateWishlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	wishlist, err := h.service.Create(r.Context(), customer.ID, req)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Created(w, wishlist)
}

// Delete removes a wishlist and all its items.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.service.Delete(r.Context(), id, customer.ID); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "wishlist not found")
		case errors.Is(err, ErrNotOwner):
			response.Forbidden(w, "this wishlist does not belong to you")
		default:
			response.InternalServerError(w)
		}
		return
	}
	response.NoContent(w)
}

// Get returns a wishlist with live prices from the customer's nearest store.
// The nearest store is resolved from the 4-hour Redis location cache.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	// Resolve nearest store from cached location
	nearestStoreID, nearestStoreName := h.resolveNearestStore(r)

	result, err := h.service.GetWithPrices(r.Context(), customer.ID, id, nearestStoreID, nearestStoreName)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "wishlist not found")
		case errors.Is(err, ErrNotOwner):
			response.Forbidden(w, "this wishlist does not belong to you")
		default:
			response.InternalServerError(w)
		}
		return
	}
	response.Success(w, result)
}

// AddItem adds a product to a specific wishlist.
//
// Body: { product_id, note? }
func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	wishlistID := chi.URLParam(r, "id")

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	if err := h.service.AddItem(r.Context(), customer.ID, wishlistID, req); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "wishlist not found")
		case errors.Is(err, ErrNotOwner):
			response.Forbidden(w, "this wishlist does not belong to you")
		case errors.Is(err, ErrItemAlreadyAdded):
			response.UnprocessableEntity(w, err.Error())
		default:
			response.InternalServerError(w)
		}
		return
	}
	response.NoContent(w)
}

// AddItemToDefault adds a product to the default "My Wishlist",
// creating it automatically if it does not exist yet.
// This is a convenience shortcut so the frontend does not need to
// know the wishlist ID for the common "save for later" action.
//
// Body: { product_id, note? }
func (h *Handler) AddItemToDefault(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	// Pass empty wishlistID — service resolves to default
	if err := h.service.AddItem(r.Context(), customer.ID, "", req); err != nil {
		if errors.Is(err, ErrItemAlreadyAdded) {
			response.UnprocessableEntity(w, err.Error())
			return
		}
		response.InternalServerError(w)
		return
	}
	response.NoContent(w)
}

// RemoveItem removes a product from a wishlist.
func (h *Handler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	wishlistID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemID")

	if err := h.service.RemoveItem(r.Context(), customer.ID, wishlistID, itemID); err != nil {
		switch {
		case errors.Is(err, ErrNotFound), errors.Is(err, ErrItemNotFound):
			response.NotFound(w, "item not found")
		case errors.Is(err, ErrNotOwner):
			response.Forbidden(w, "this wishlist does not belong to you")
		default:
			response.InternalServerError(w)
		}
		return
	}
	response.NoContent(w)
}

// ── Helper ────────────────────────────────────────────────────────────────────

func (h *Handler) resolveNearestStore(r *http.Request) (storeID, storeName string) {
	user := users.UserFromContext(r.Context())
	if user == nil {
		return "", ""
	}
	key := geo.LocationKey(user.ID, "")
	loc, err := h.locationService.Get(r.Context(), key)
	if err != nil || loc == nil {
		return "", ""
	}
	return loc.NearestStoreID, loc.NearestStoreName
}