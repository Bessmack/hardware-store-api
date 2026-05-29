package users

import "context"

// contextKey is an unexported type for context keys in this package.
// Using a custom type prevents collisions with keys from other packages.
type contextKey string

const userContextKey contextKey = "authenticated_user"

// SetUserContext returns a new context with the authenticated user embedded.
// Called by middleware after JWT token verification succeeds.
func SetUserContext(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// UserFromContext extracts the authenticated user from the request context.
// Returns nil for unauthenticated requests (guests).
// Handlers on protected routes can safely assume this is non-nil because
// the auth middleware would have rejected the request otherwise.
func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}