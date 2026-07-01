package server

import (
	"net/http"

	"github.com/Bessmack/hardware-store-api/internal/auth"
	"github.com/Bessmack/hardware-store-api/internal/cart"
	"github.com/Bessmack/hardware-store-api/internal/categories"
	"github.com/Bessmack/hardware-store-api/internal/config"
	"github.com/Bessmack/hardware-store-api/internal/delivery"
	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/inventory"
	"github.com/Bessmack/hardware-store-api/internal/middleware"
	"github.com/Bessmack/hardware-store-api/internal/orders"
	"github.com/Bessmack/hardware-store-api/internal/payments"
	"github.com/Bessmack/hardware-store-api/internal/pod"
	"github.com/Bessmack/hardware-store-api/internal/products"
	"github.com/Bessmack/hardware-store-api/internal/reports"
	"github.com/Bessmack/hardware-store-api/internal/stores"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/internal/wishlist"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// NewRouter assembles and returns the fully configured HTTP router.
// All handlers, middleware, and route groups are defined here — nothing is registered anywhere else in the codebase.
func NewRouter(
	cfg *config.Config,
	// Middleware
	authMw *middleware.AuthMiddleware,
	storeScopeMw *middleware.StoreScopeMiddleware,
	rateLimiter *middleware.RateLimiter,
	corsMw func(http.Handler) http.Handler,
	// Handlers
	authHandler *auth.Handler,
	userHandler *users.Handler,
	storeHandler *stores.Handler,
	geoHandler *geo.Handler,
	productHandler *products.Handler,
	inventoryHandler *inventory.Handler,
	cartHandler *cart.Handler,
	wishlistHandler *wishlist.Handler,
	deliveryHandler *delivery.Handler,
	orderHandler *orders.Handler,
	podHandler *pod.Handler,
	paymentHandler *payments.Handler,
	reportsHandler *reports.Handler,
	categoryHandler *categories.Handler,
) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware ─────────────────────────────────────────────────────
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(corsMw)
	// Default rate limit applied to every route.
	// Specific routes override this with a tighter preset (login, register, geo).
	r.Use(rateLimiter.ForAPI())

	r.Route("/api/v1", func(r chi.Router) {

		// ── Health check (no auth, no rate limit) ─────────────────────────────
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})

		// ── Auth ──────────────────────────────────────────────────────────────
		r.Route("/auth", func(r chi.Router) {
			r.With(rateLimiter.ForRegister()).
				Post("/register", authHandler.Register)
			r.With(rateLimiter.ForLogin()).
				Post("/login", authHandler.Login)
			r.With(rateLimiter.ForRefresh()).
				Post("/refresh", authHandler.Refresh)
			r.With(authMw.RequireAuth).
				Post("/logout", authHandler.Logout)
		})

		// ── Geo (location, autocomplete, geocode) ─────────────────────────────
		// Tighter rate limit — geocoding providers have usage policies.
		r.Route("/geo", func(r chi.Router) {
			r.Use(rateLimiter.ForGeo())
			r.With(authMw.OptionalAuth).Post("/location", geoHandler.SaveLocation)
			r.Get("/autocomplete", geoHandler.Autocomplete)
			r.Get("/geocode", geoHandler.Geocode)
		})

		// ── Stores (public) ───────────────────────────────────────────────────
		r.Route("/stores", func(r chi.Router) {
			r.Get("/", storeHandler.ListActive)
			r.Get("/{storeID}", storeHandler.GetPublic)
			r.Get("/{storeID}/products", productHandler.List)
		})

		// ── Categories (public) ───────────────────────────────────────────────
		// Returns all categories with subcategories embedded. Cached aggressively
		// on the frontend — categories change rarely.
		r.Route("/categories", func(r chi.Router) {
			r.Get("/", categoryHandler.List)
			r.Get("/{slug}", categoryHandler.Get)
			r.Get("/{slug}/subcategories", categoryHandler.ListSubcategories)
		})

		// ── Products (public global listing) ─────────────────────────────────
		// Supports ?category={slug} and ?subcategory_id={id} query params.
		// Per-store listings are at /stores/{storeID}/products above.
		r.Route("/products", func(r chi.Router) {
			r.Get("/", productHandler.ListAll)
			r.Get("/{productID}", productHandler.GetByID)
		})

		// ── Payment methods (public) ──────────────────────────────────────────
		r.Get("/payments/methods", paymentHandler.AvailableMethods)

		// ── Payment callbacks (no auth — providers call these directly) ───────
		// M-Pesa and Airtel are protected by IP allowlist middleware.
		// Pesapal (card) uses IPN verification inside HandleCallback.
		r.Route("/payments", func(r chi.Router) {
			r.With(middleware.AllowSafaricom).
				Post("/mpesa/callback/{storeID}", paymentHandler.MpesaCallback)
			r.With(middleware.AllowAirtel).
				Post("/airtel/callback/{storeID}", paymentHandler.AirtelCallback)
			// Pesapal IPN — no IP allowlist; security is via GetTransactionStatus
			r.Post("/card/callback", paymentHandler.CardCallback)
		})

		// ── Authenticated customer routes ─────────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireAuth)
			r.Use(authMw.RequireRole("customer", "cashier", "admin", "superadmin"))

			// Profile
			r.Get("/users/me", userHandler.GetProfile)
			r.Put("/users/me", userHandler.UpdateProfile)

			// Cart
			r.Route("/cart", func(r chi.Router) {
				r.Get("/", cartHandler.GetCart)
				r.Post("/items", cartHandler.AddItem)
				r.Put("/items/{itemID}", cartHandler.UpdateQuantity)
				r.Delete("/items/{itemID}", cartHandler.RemoveItem)
				r.Get("/validate", cartHandler.Validate)
			})

			// Wishlist
			r.Route("/wishlist", func(r chi.Router) {
				r.Get("/", wishlistHandler.Get)
				r.Post("/items", wishlistHandler.AddItem)
				r.Delete("/items/{productID}", wishlistHandler.RemoveItem)
			})

			// Delivery quote (public-ish — available to any logged-in user)
			r.Get("/delivery/quote", deliveryHandler.Quote)

			// Orders (customer's own)
			r.Route("/orders", func(r chi.Router) {
				r.Post("/", orderHandler.PlaceOrder)
				r.Get("/", orderHandler.ListOwnOrders)
				r.Get("/{orderID}", orderHandler.GetOwnOrder)
				r.Get("/{orderID}/track", orderHandler.TrackOrder)
				r.Delete("/{orderID}", orderHandler.CancelOrder)
				// Dispute raised by the customer after delivery
				r.Post("/{orderID}/dispute", podHandler.RaiseDispute)
			})

			// Products (browsing — available to all authenticated users)
			r.Get("/products/{productID}", productHandler.Get)
		})

		// ── Staff routes (cashier and above, store-scoped) ────────────────────
		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireAuth)
			r.Use(authMw.RequireRole("cashier", "admin", "superadmin"))
			r.Use(storeScopeMw.StoreScope)

			r.Route("/store", func(r chi.Router) {

				// Orders
				r.Get("/orders", orderHandler.ListForStore)
				r.Get("/orders/{orderID}", orderHandler.GetForStore)
				r.Put("/orders/{orderID}/status", orderHandler.UpdateStatus)

				// POD (delivery person submits at door)
				r.Post("/pod/submit", podHandler.SubmitPOD)

				// POD review (staff view)
				r.Get("/orders/{orderID}/pod", podHandler.GetPOD)
				r.Get("/orders/{orderID}/dispute", podHandler.GetDispute)

				// Inventory
				r.Get("/inventory", inventoryHandler.List)
				r.Put("/inventory/{productID}", inventoryHandler.Upsert)

				// Products (staff manage)
				r.Post("/products", productHandler.Create)
				r.Put("/products/{productID}", productHandler.Update)
				r.Delete("/products/{productID}", productHandler.Deactivate)

				// Delivery rates
				r.Get("/delivery/rates", deliveryHandler.ListRates)
				r.Put("/delivery/rates", deliveryHandler.UpsertStoreRate)
				r.Delete("/delivery/rates/{vehicleType}", deliveryHandler.DeleteStoreRate)

				// Store report
				r.Get("/report", reportsHandler.StoreReport)

				// Disputes resolved by admin
				r.Put("/disputes/{disputeID}/resolve", podHandler.ResolveDispute)
			})
		})

		// ── Superadmin routes ─────────────────────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireAuth)
			r.Use(authMw.RequireRole("superadmin"))

			// Store management
			r.Post("/stores", storeHandler.Create)
			r.Put("/stores/{storeID}", storeHandler.Update)
			r.Put("/stores/{storeID}/credentials", storeHandler.UpdateCredentials)
			r.Put("/stores/{storeID}/deactivate", storeHandler.Deactivate)
			r.Put("/stores/{storeID}/reactivate", storeHandler.Reactivate)
			r.Get("/stores/all", storeHandler.ListAll)

			// Delivery: update global default rates
			r.Put("/delivery/rates/global", deliveryHandler.UpdateGlobalRate)

			// Global report
			r.Get("/reports/global", reportsHandler.GlobalReport)

			// User management
			r.Post("/admins", userHandler.CreateAdmin)
			r.Get("/admins", userHandler.ListAdmins)
			r.Put("/admins/{id}/store", userHandler.AssignAdminToStore)
			r.Get("/users/{id}", userHandler.GetByID)
			r.Put("/users/{id}/deactivate", userHandler.DeactivateUser)
			r.Put("/users/{id}/reactivate", userHandler.ReactivateUser)

			// Staff management — also in superadmin block so superadmin
			// can create/list/deactivate cashiers without a store assignment.
			// The store-scoped group below handles the same for admins.
			r.Post("/store/staff", userHandler.CreateStaff)
			r.Get("/store/staff", userHandler.ListStoreStaff)
			r.Put("/store/staff/{id}/deactivate", userHandler.DeactivateStaff)
			r.Put("/store/staff/{id}/reactivate", userHandler.ReactivateStaff)

			// Category management
			r.Post("/categories", categoryHandler.Create)
			r.Put("/categories/{id}", categoryHandler.Update)
			r.Delete("/categories/{id}", categoryHandler.Delete)
			r.Post("/categories/{id}/subcategories", categoryHandler.CreateSubcategory)
			r.Put("/subcategories/{id}", categoryHandler.UpdateSubcategory)
			r.Delete("/subcategories/{id}", categoryHandler.DeleteSubcategory)
		})
	})

	return r
}