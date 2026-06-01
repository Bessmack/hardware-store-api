package middleware

import (
	"context"
	"net/http"

	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/go-chi/chi/v5"
)

// ── Context key ───────────────────────────────────────────────────────────────

type storeScopeKey string

const scopedStoreIDKey storeScopeKey = "scoped_store_id"

// ScopedStoreID extracts the resolved store ID from the request context.
// Call this inside any handler that sits behind the StoreScope middleware.
//
// Example:
//
//	storeID := middleware.ScopedStoreID(r.Context())
func ScopedStoreID(ctx context.Context) string {
	id, _ := ctx.Value(scopedStoreIDKey).(string)
	return id
}

// ── Store assignment interface ────────────────────────────────────────────────

// StoreAssignmentReader is the subset of users.Service needed by StoreScope.
// Defined as an interface to keep middleware decoupled from the full service.
type StoreAssignmentReader interface {
	GetStoreAssignment(ctx context.Context, userID string) (*users.StoreAssignment, error)
}

// ── Middleware ────────────────────────────────────────────────────────────────

// StoreScopeMiddleware holds the dependencies needed to resolve store scope.
type StoreScopeMiddleware struct {
	assignments StoreAssignmentReader
}

func NewStoreScopeMiddleware(assignments StoreAssignmentReader) *StoreScopeMiddleware {
	return &StoreScopeMiddleware{assignments: assignments}
}

// StoreScope resolves which store the current user is allowed to access and
// injects it into the request context via ScopedStoreID().
//
// Resolution rules:
//
//	Cashier / Admin   → always their assigned store from staff_store_assignments.
//	                    Any store_id they pass in the URL is ignored and overridden.
//	SuperAdmin        → uses the :storeID URL parameter (chi) if present,
//	                    otherwise falls back to the store_id query param.
//	                    If neither is present, store_id is left empty (global access).
//
// Apply this middleware to every route group that is store-scoped:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(mw.RequireAuth)
//	    r.Use(mw.RequireRole(users.RoleCashier, users.RoleAdmin, users.RoleSuperAdmin))
//	    r.Use(storeScopeMw.StoreScope)
//	    r.Get("/store/orders", orderHandler.ListForStore)
//	})
func (m *StoreScopeMiddleware) StoreScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := users.UserFromContext(r.Context())
		if user == nil {
			// Should never happen — StoreScope must be chained after RequireAuth
			response.Unauthorized(w, "authentication required")
			return
		}

		var storeID string

		switch user.Role {

		case users.RoleCashier, users.RoleAdmin:
			// Staff are always locked to their assigned store.
			// We load the assignment from the DB — the user cannot override this
			// by passing a different store_id in the URL or query string.
			assignment, err := m.assignments.GetStoreAssignment(r.Context(), user.ID)
			if err != nil {
				response.Forbidden(w, "your account is not assigned to any store — contact superadmin")
				return
			}
			storeID = assignment.StoreID

		case users.RoleSuperAdmin:
			// SuperAdmin can target any store.
			// Check chi URL param first (e.g. /stores/:storeID/orders),
			// then fall back to query param (e.g. /store/orders?store_id=...).
			// If neither is provided, storeID stays empty → global access.
			storeID = chi.URLParam(r, "storeID")
			if storeID == "" {
				storeID = r.URL.Query().Get("store_id")
			}

		default:
			response.Forbidden(w, "you do not have permission to access store data")
			return
		}

		ctx := context.WithValue(r.Context(), scopedStoreIDKey, storeID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── Guard helper ──────────────────────────────────────────────────────────────

// AssertStoreScoped checks that the handler received a non-empty store ID
// from the StoreScope middleware. Call at the top of any store-scoped handler
// where a store_id is mandatory (e.g. listing orders, updating status).
//
// Returns false and writes a 400 if the store ID is missing, so the handler
// can return immediately:
//
//	storeID, ok := middleware.AssertStoreScoped(w, r)
//	if !ok { return }
func AssertStoreScoped(w http.ResponseWriter, r *http.Request) (string, bool) {
	storeID := ScopedStoreID(r.Context())
	if storeID == "" {
		response.BadRequest(w, "store_id is required — pass it as a URL parameter or query string")
		return "", false
	}
	return storeID, true
}