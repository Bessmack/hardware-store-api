package stores

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
// Public:
//   GET  /api/v1/stores                    List active stores (customers)
//   GET  /api/v1/stores/:id                Get single store (customers)
//
// SuperAdmin only:
//   GET  /api/v1/stores/all                List all stores including inactive
//   POST /api/v1/stores                    Create a new store
//   PUT  /api/v1/stores/:id                Update store info
//   PUT  /api/v1/stores/:id/credentials    Update M-Pesa/Airtel credentials
//   PUT  /api/v1/stores/:id/deactivate     Deactivate a store
//   PUT  /api/v1/stores/:id/reactivate     Reactivate a store

// ── Public handlers ───────────────────────────────────────────────────────────

// ListActive returns all active stores in the public view.
// Customers use this to see all branches on a map or a list.
func (h *Handler) ListActive(w http.ResponseWriter, r *http.Request) {
	stores, err := h.service.ListActive(r.Context())
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, stores)
}

// GetPublic returns a single store in the public view.
func (h *Handler) GetPublic(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	store, err := h.service.GetPublic(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "store not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, store)
}

// ── SuperAdmin handlers ───────────────────────────────────────────────────────

// ListAll returns all stores including inactive. SuperAdmin only.
func (h *Handler) ListAll(w http.ResponseWriter, r *http.Request) {
	stores, err := h.service.ListAll(r.Context())
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, stores)
}

// Create registers a new store branch. SuperAdmin only.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	requestedBy := users.UserFromContext(r.Context())

	var req CreateStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	store, err := h.service.Create(r.Context(), req, requestedBy)
	if err != nil {
		response.Forbidden(w, err.Error())
		return
	}
	response.Created(w, store)
}

// Update updates a store's basic information. SuperAdmin only.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	requestedBy := users.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req UpdateStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	store, err := h.service.Update(r.Context(), id, req, requestedBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "store not found")
		default:
			response.Forbidden(w, err.Error())
		}
		return
	}
	response.Success(w, store)
}

// UpdateCredentials sets a store's M-Pesa and Airtel payment credentials.
// SuperAdmin only. The passkey is stored server-side and never returned.
func (h *Handler) UpdateCredentials(w http.ResponseWriter, r *http.Request) {
	requestedBy := users.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req UpdateCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	store, err := h.service.UpdateCredentials(r.Context(), id, req, requestedBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "store not found")
		default:
			response.Forbidden(w, err.Error())
		}
		return
	}
	response.Success(w, store)
}

// Deactivate marks a store as inactive. SuperAdmin only.
func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	requestedBy := users.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.service.Deactivate(r.Context(), id, requestedBy); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "store not found")
		default:
			response.Forbidden(w, err.Error())
		}
		return
	}
	response.NoContent(w)
}

// Reactivate re-enables a deactivated store. SuperAdmin only.
func (h *Handler) Reactivate(w http.ResponseWriter, r *http.Request) {
	requestedBy := users.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.service.Reactivate(r.Context(), id, requestedBy); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "store not found")
		default:
			response.Forbidden(w, err.Error())
		}
		return
	}
	response.NoContent(w)
}