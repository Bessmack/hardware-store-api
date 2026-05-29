package users

import (
	"encoding/json"
	"errors"
	"net/http"

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

// ── Customer routes ───────────────────────────────────────────────────────────
// GET  /profile
// PUT  /profile

// GetProfile returns the authenticated customer's own profile.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	profile, err := h.service.GetProfile(r.Context(), user.ID)
	if err != nil {
		response.InternalServerError(w)
		return
	}

	response.Success(w, profile)
}

// UpdateProfile updates the authenticated customer's own profile.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	updated, err := h.service.UpdateProfile(r.Context(), user.ID, req)
	if err != nil {
		response.InternalServerError(w)
		return
	}

	response.Success(w, updated)
}

// ── Admin routes ──────────────────────────────────────────────────────────────
// POST /store/staff
// GET  /store/staff
// PUT  /store/staff/:id/deactivate
// PUT  /store/staff/:id/reactivate  (superadmin only — enforced in routes.go)

// CreateStaff creates a cashier account and assigns them to the requesting
// admin's store. SuperAdmin can create admins and specify any store.
func (h *Handler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	requestedBy := UserFromContext(r.Context())

	var req CreateStaffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	staff, err := h.service.CreateStaff(r.Context(), req, requestedBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			response.UnprocessableEntity(w, err.Error())
		case errors.Is(err, ErrPhoneTaken):
			response.UnprocessableEntity(w, err.Error())
		default:
			response.Forbidden(w, err.Error())
		}
		return
	}

	response.Created(w, staff)
}

// ListStoreStaff returns all staff assigned to the requesting admin's store.
func (h *Handler) ListStoreStaff(w http.ResponseWriter, r *http.Request) {
	// storeID is injected by StoreScope middleware into the query param
	storeID := r.URL.Query().Get("store_id")
	if storeID == "" {
		response.BadRequest(w, "store_id is required")
		return
	}

	staff, err := h.service.ListStoreStaff(r.Context(), storeID)
	if err != nil {
		response.InternalServerError(w)
		return
	}

	response.Success(w, staff)
}

// DeactivateStaff deactivates a staff account.
// Admin can deactivate cashiers in their own store.
// SuperAdmin can deactivate any non-superadmin account.
func (h *Handler) DeactivateStaff(w http.ResponseWriter, r *http.Request) {
	requestedBy := UserFromContext(r.Context())
	targetID := chi.URLParam(r, "id")

	if err := h.service.DeactivateUser(r.Context(), targetID, requestedBy); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "user not found")
		default:
			response.Forbidden(w, err.Error())
		}
		return
	}

	response.NoContent(w)
}

// ReactivateStaff re-enables a deactivated staff account. SuperAdmin only.
func (h *Handler) ReactivateStaff(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "id")

	if err := h.service.ReactivateUser(r.Context(), targetID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "user not found")
			return
		}
		response.InternalServerError(w)
		return
	}

	response.NoContent(w)
}

// ── SuperAdmin routes ─────────────────────────────────────────────────────────
// POST /admins
// GET  /admins
// PUT  /admins/:id/store
// PUT  /users/:id/deactivate
// PUT  /users/:id/reactivate

// CreateAdmin creates a new admin account. SuperAdmin only.
func (h *Handler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	requestedBy := UserFromContext(r.Context())

	var req CreateStaffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	// Force the role to admin regardless of what was sent in the body
	req.Role = RoleAdmin

	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	admin, err := h.service.CreateStaff(r.Context(), req, requestedBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			response.UnprocessableEntity(w, err.Error())
		case errors.Is(err, ErrPhoneTaken):
			response.UnprocessableEntity(w, err.Error())
		default:
			response.Forbidden(w, err.Error())
		}
		return
	}

	response.Created(w, admin)
}

// ListAdmins returns all admin accounts with their store assignments. SuperAdmin only.
func (h *Handler) ListAdmins(w http.ResponseWriter, r *http.Request) {
	admins, err := h.service.ListAdmins(r.Context())
	if err != nil {
		response.InternalServerError(w)
		return
	}

	response.Success(w, admins)
}

// AssignAdminToStore assigns or reassigns an admin to a store. SuperAdmin only.
func (h *Handler) AssignAdminToStore(w http.ResponseWriter, r *http.Request) {
	requestedBy := UserFromContext(r.Context())
	adminID := chi.URLParam(r, "id")

	var req AssignToStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	if err := h.service.AssignToStore(r.Context(), adminID, req.StoreID, requestedBy); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "user not found")
		default:
			response.Forbidden(w, err.Error())
		}
		return
	}

	response.NoContent(w)
}

// DeactivateUser deactivates any user by ID. SuperAdmin only.
func (h *Handler) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	requestedBy := UserFromContext(r.Context())
	targetID := chi.URLParam(r, "id")

	if err := h.service.DeactivateUser(r.Context(), targetID, requestedBy); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "user not found")
		default:
			response.Forbidden(w, err.Error())
		}
		return
	}

	response.NoContent(w)
}

// ReactivateUser reactivates any user by ID. SuperAdmin only.
func (h *Handler) ReactivateUser(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "id")

	if err := h.service.ReactivateUser(r.Context(), targetID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "user not found")
			return
		}
		response.InternalServerError(w)
		return
	}

	response.NoContent(w)
}