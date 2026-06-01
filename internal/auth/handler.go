package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/Bessmack/hardware-store-api/pkg/validator"
)

type Handler struct {
	service         *Service
	locationService *geo.LocationService
}

func NewHandler(service *Service, locationService *geo.LocationService) *Handler {
	return &Handler{
		service:         service,
		locationService: locationService,
	}
}

// ── Routes (registered in server/routes.go) ───────────────────────────────────
//
// POST /api/v1/auth/register   Public
// POST /api/v1/auth/login      Public
// POST /api/v1/auth/refresh    Public  (refresh token in body, no Bearer header needed)
// POST /api/v1/auth/logout     RequireAuth

// Register creates a new customer account and returns a token pair immediately.
//
// Body: { email, phone, password, first_name, last_name? }
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req users.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	resp, err := h.service.Register(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, users.ErrEmailTaken):
			response.UnprocessableEntity(w, err.Error())
		case errors.Is(err, users.ErrPhoneTaken):
			response.UnprocessableEntity(w, err.Error())
		default:
			response.InternalServerError(w)
		}
		return
	}

	response.Created(w, resp)
}

// Login verifies credentials and returns an access token + refresh token.
//
// Body: { email, password, guest_session_id? }
//
// The frontend should:
//  1. Store access_token in memory (NOT localStorage — XSS risk)
//  2. Store refresh_token in an httpOnly cookie or secure storage
//  3. Use expires_in to schedule a proactive token refresh before expiry
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	resp, err := h.service.Login(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			response.Unauthorized(w, err.Error())
		case errors.Is(err, ErrAccountDeactivated):
			response.Forbidden(w, err.Error())
		default:
			response.InternalServerError(w)
		}
		return
	}

	// Migrate guest location cache -> user location cache (non-fatal)
	if req.GuestSessionID != "" && resp.User.Role == users.RoleCustomer {
		guestKey := geo.LocationKey("", req.GuestSessionID)
		userKey := geo.LocationKey(resp.User.ID, "")
		h.locationService.MigrateGuestLocation(r.Context(), guestKey, userKey)
	}

	response.Success(w, resp)
}

// Refresh exchanges a valid refresh token for a new access token + new refresh token.
// Both tokens are rotated — the old refresh token is invalidated immediately.
//
// Body: { refresh_token: "..." }
//
// Call this endpoint when:
//  - The access token has expired (frontend receives 401)
//  - Proactively, shortly before expires_in seconds elapses
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	resp, err := h.service.Refresh(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidRefreshToken):
			response.Unauthorized(w, err.Error())
		case errors.Is(err, ErrAccountDeactivated):
			response.Forbidden(w, err.Error())
		default:
			response.InternalServerError(w)
		}
		return
	}

	response.Success(w, resp)
}

// Logout revokes the refresh token and clears the location cache.
//
// Body: { refresh_token?: "..." }
//
// The access token expires naturally after JWT_ACCESS_EXPIRY_MINUTES.
// The frontend must delete both tokens from its storage on receiving 204.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	// Body is optional — decode but do not fail if missing or malformed
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Revoke refresh token in Redis (immediate invalidation)
	h.service.RevokeRefreshToken(r.Context(), req.RefreshToken)

	// Clear location cache so a fresh GPS capture happens on next login
	user := users.UserFromContext(r.Context())
	if user != nil {
		key := geo.LocationKey(user.ID, "")
		_ = h.locationService.Clear(r.Context(), key)
	}

	response.NoContent(w)
}