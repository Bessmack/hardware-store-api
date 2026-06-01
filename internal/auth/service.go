package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/middleware"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/cache"
)

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	// ErrInvalidCredentials is returned for wrong email or wrong password.
	// Never reveal which one failed — that leaks which emails are registered.
	ErrInvalidCredentials = errors.New("invalid email or password")

	// ErrAccountDeactivated is returned when a valid but inactive user tries to log in.
	ErrAccountDeactivated = errors.New("your account has been deactivated — contact support")

	// ErrInvalidRefreshToken is returned when the refresh token is missing,
	// expired, or has already been used (rotation means one-time use).
	ErrInvalidRefreshToken = errors.New("refresh token is invalid or has expired — please log in again")
)

// refreshTokenPrefix is the Redis key prefix for all refresh tokens.
// Full key format: rt:{random_hex_64}  →  {user_id}
const refreshTokenPrefix = "rt:"

// ── Service ───────────────────────────────────────────────────────────────────

// ServiceConfig groups the JWT and refresh token settings passed to NewService.
type ServiceConfig struct {
	JWTSecret           string
	AccessExpiryMinutes int
	RefreshExpiryDays   int
}

type Service struct {
	userService       *users.Service
	cache             *cache.Cache
	jwtSecret         string
	accessExpiryMins  int
	refreshExpiryDays int
}

func NewService(userService *users.Service, c *cache.Cache, cfg ServiceConfig) *Service {
	return &Service{
		userService:       userService,
		cache:             c,
		jwtSecret:         cfg.JWTSecret,
		accessExpiryMins:  cfg.AccessExpiryMinutes,
		refreshExpiryDays: cfg.RefreshExpiryDays,
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

// Login verifies credentials and returns a fresh access + refresh token pair.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.userService.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth: login lookup failed: %w", err)
	}

	if !s.userService.VerifyPassword(user.PasswordHash, req.Password) {
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, ErrAccountDeactivated
	}

	return s.issueTokenPair(ctx, user)
}

// ── Register ──────────────────────────────────────────────────────────────────

// Register creates a new customer account and auto-logs them in.
func (s *Service) Register(ctx context.Context, req users.RegisterRequest) (*LoginResponse, error) {
	if _, err := s.userService.CreateCustomer(ctx, req); err != nil {
		return nil, err // ErrEmailTaken / ErrPhoneTaken bubble up unchanged
	}

	// Load the full user record — CreateCustomer returns UserResponse which
	// does not include the fields GenerateToken needs (password hash aside,
	// we need the raw *User to pass to middleware.GenerateToken).
	user, err := s.userService.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("auth: registered but could not load user: %w", err)
	}

	return s.issueTokenPair(ctx, user)
}

// ── Refresh ───────────────────────────────────────────────────────────────────

// Refresh validates a refresh token, issues a new access token, and rotates
// the refresh token (old one is deleted, new one is created).
//
// Rotation means each refresh token is single-use. If an attacker steals a
// refresh token and uses it, the legitimate user's next refresh will fail
// (their copy of the token is now gone), alerting them to re-authenticate.
func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error) {
	redisKey := refreshTokenPrefix + req.RefreshToken

	// Look up the refresh token in Redis
	userID, err := s.cache.Get(ctx, redisKey)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}

	// Load the user to get the latest role and active status
	// (role or active state may have changed since the token was issued)
	user, err := s.userService.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}
	if !user.IsActive {
		_ = s.cache.Delete(ctx, redisKey) // clean up on deactivated account
		return nil, ErrAccountDeactivated
	}

	// Delete the old refresh token before issuing new ones (rotation)
	if err := s.cache.Delete(ctx, redisKey); err != nil {
		return nil, fmt.Errorf("auth: failed to rotate refresh token: %w", err)
	}

	// Issue new access token
	accessToken, err := middleware.GenerateToken(user, s.jwtSecret, s.accessExpiryMins)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to generate access token: %w", err)
	}

	// Issue new refresh token
	newRefreshToken, err := s.storeRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to issue new refresh token: %w", err)
	}

	return &RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    s.accessExpiryMins * 60,
	}, nil
}

// ── Logout ────────────────────────────────────────────────────────────────────

// RevokeRefreshToken deletes a refresh token from Redis, immediately
// preventing it from being used to obtain new access tokens.
// Called by the logout handler.
func (s *Service) RevokeRefreshToken(ctx context.Context, refreshToken string) {
	if refreshToken == "" {
		return
	}
	_ = s.cache.Delete(ctx, refreshTokenPrefix+refreshToken)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// issueTokenPair generates an access token + refresh token for the given user
// and returns a LoginResponse ready to send to the client.
func (s *Service) issueTokenPair(ctx context.Context, user *users.User) (*LoginResponse, error) {
	accessToken, err := middleware.GenerateToken(user, s.jwtSecret, s.accessExpiryMins)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to generate access token: %w", err)
	}

	refreshToken, err := s.storeRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to store refresh token: %w", err)
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.accessExpiryMins * 60, // seconds — frontend uses for proactive refresh
		User:         users.ToResponse(user),
		RedirectTo:   dashboardForRole(user.Role),
	}, nil
}

// storeRefreshToken generates a cryptographically random refresh token,
// stores it in Redis with the user ID as the value, and returns the token.
func (s *Service) storeRefreshToken(ctx context.Context, userID string) (string, error) {
	token, err := generateSecureToken()
	if err != nil {
		return "", err
	}

	ttl := time.Duration(s.refreshExpiryDays) * 24 * time.Hour
	if err := s.cache.Set(ctx, refreshTokenPrefix+token, userID, ttl); err != nil {
		return "", fmt.Errorf("auth: failed to save refresh token to redis: %w", err)
	}

	return token, nil
}

// generateSecureToken returns a 64-character cryptographically random hex string.
// This is the format used for refresh tokens.
func generateSecureToken() (string, error) {
	b := make([]byte, 32) // 32 bytes = 64 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: failed to generate secure token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// dashboardForRole returns the frontend path for the user to land on after login.
func dashboardForRole(role users.Role) string {
	switch role {
	case users.RoleCustomer:
		return "/account"
	case users.RoleCashier:
		return "/staff/orders"
	case users.RoleAdmin:
		return "/staff/dashboard"
	case users.RoleSuperAdmin:
		return "/admin/dashboard"
	default:
		return "/"
	}
}