package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/golang-jwt/jwt/v5"
)

// Claims is the payload embedded in every JWT issued by the auth service.
type Claims struct {
	UserID string     `json:"user_id"`
	Email  string     `json:"email"`
	Role   users.Role `json:"role"`
	jwt.RegisteredClaims
}

// AuthMiddleware holds dependencies needed to verify tokens and load users.
type AuthMiddleware struct {
	jwtSecret   string
	userService UserServiceInterface
}

// UserServiceInterface is the subset of users.Service needed by middleware.
// Using an interface keeps the middleware package decoupled from the full service.
type UserServiceInterface interface {
	GetByID(ctx context.Context, id string) (*users.User, error)
}

func NewAuthMiddleware(jwtSecret string, userService UserServiceInterface) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret:   jwtSecret,
		userService: userService,
	}
}

// RequireAuth rejects unauthenticated requests with 401.
// Use on routes that require a logged-in user of any role.
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := m.extractUser(r)
		if err != nil || user == nil {
			response.Unauthorized(w, "authentication required")
			return
		}
		if !user.IsActive {
			response.Unauthorized(w, "your account has been deactivated")
			return
		}

		ctx := users.SetUserContext(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth extracts the user if a valid token is present, but does not
// reject the request if there is none. Used on cart/browse routes accessible
// by both guests and logged-in customers.
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := m.extractUser(r) // ignore error — user may be nil (guest)
		if user != nil && user.IsActive {
			ctx := users.SetUserContext(r.Context(), user)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole rejects requests where the authenticated user's role is not
// in the allowed list. Must be chained after RequireAuth.
//
// Example:
//
//	r.Use(mw.RequireAuth)
//	r.Use(mw.RequireRole(users.RoleAdmin, users.RoleSuperAdmin))
func (m *AuthMiddleware) RequireRole(allowed ...users.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := users.UserFromContext(r.Context())
			if user == nil {
				response.Unauthorized(w, "authentication required")
				return
			}

			for _, role := range allowed {
				if user.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			response.Forbidden(w, "you do not have permission to access this resource")
		})
	}
}

// GenerateToken creates a signed JWT for the given user.
// Called by the auth service after successful login.
func GenerateToken(u *users.User, secret string, expiryHours int) (string, error) {
	claims := Claims{
		UserID: u.ID,
		Email:  u.Email,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// extractUser parses the Bearer token from the Authorization header,
// verifies it, and loads the full user record from the database.
func (m *AuthMiddleware) extractUser(r *http.Request) (*users.User, error) {
	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		return nil, errors.New("no token")
	}

	claims, err := parseToken(tokenStr, m.jwtSecret)
	if err != nil {
		return nil, err
	}

	// Load full user from DB — ensures we get the latest role and is_active state
	return m.userService.GetByID(r.Context(), claims.UserID)
}

func extractBearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

func parseToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}