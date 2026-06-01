package auth

import "github.com/Bessmack/hardware-store-api/internal/users"

// ── Login / Register ──────────────────────────────────────────────────────────

// LoginRequest is the body for POST /api/v1/auth/login.
// GuestSessionID is optional — when provided, the guest location cache is
// migrated to the user account so pricing stays consistent after login.
type LoginRequest struct {
	Email          string `json:"email"            validate:"required,email"`
	Password       string `json:"password"         validate:"required"`
	GuestSessionID string `json:"guest_session_id"`
}

// LoginResponse is returned on successful login and registration.
//
//   AccessToken  short-lived JWT (30 min); send in Authorization: Bearer header
//   RefreshToken long-lived opaque token (7 days); send only to POST /auth/refresh
//   ExpiresIn    access token lifetime in seconds — frontend uses this to know
//                when to call /auth/refresh before the next request fails
//   RedirectTo   role-based dashboard path the frontend should navigate to
type LoginResponse struct {
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	ExpiresIn    int                `json:"expires_in"`
	User         users.UserResponse `json:"user"`
	RedirectTo   string             `json:"redirect_to"`
}

// ── Token refresh ─────────────────────────────────────────────────────────────

// RefreshRequest is the body for POST /api/v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// RefreshResponse is returned on a successful token refresh.
// Both tokens are rotated — store the new refresh token and discard the old one.
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// ── Logout ────────────────────────────────────────────────────────────────────

// LogoutRequest is the body for POST /api/v1/auth/logout.
// RefreshToken should be provided so it can be invalidated in Redis immediately.
// If omitted, the location cache is still cleared and the logout still succeeds —
// the refresh token will simply expire naturally after JWT_REFRESH_EXPIRY_DAYS.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}