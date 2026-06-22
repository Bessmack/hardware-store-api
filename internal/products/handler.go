package products

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/middleware"
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
// Public (guests + customers):
//   GET /api/v1/products                    List products for nearest/selected store
//   GET /api/v1/products/:id                Get single product with all store prices
//
// Staff (cashier, admin, superadmin) — behind StoreScope middleware:
//   GET /api/v1/store/products              List all products with stock for staff
//   GET /api/v1/store/products/:id          Get single product with full inventory
//
// Admin + SuperAdmin:
//   POST   /api/v1/store/products           Create product
//   PUT    /api/v1/store/products/:id       Update product
//   PUT    /api/v1/store/products/:id/deactivate
//   PUT    /api/v1/store/products/:id/reactivate

// ── Public handlers ───────────────────────────────────────────────────────────

// ListAll handles GET /api/v1/products
// Query params:
//
//	q             full-text search term
//	category      category slug (e.g. "plumbing")
//	subcategory   subcategory UUID
//	page          page number (default 1)
//	limit         results per page (default 24, max 100)
func (h *Handler) ListAll(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	f := ListFilter{
		Query:         q.Get("q"),
		CategorySlug:  q.Get("category"),
		SubcategoryID: q.Get("subcategory"),
		Page:          page,
		Limit:         limit,
	}

	products, total, err := h.service.ListAll(r.Context(), f)
	if err != nil {
		response.InternalServerError(w)
		return
	}

	// Normalise after service sets defaults
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 24
	}

	response.Paginated(w, products, response.Meta{
		Page:       f.Page,
		PerPage:    f.Limit,
		TotalItems: total,
		TotalPages: (total + f.Limit - 1) / f.Limit,
	})
}

// GetByID handles GET /api/v1/products/{productID}
// Returns the product detail with prices across all active stores.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "productID")

	detail, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "product not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, detail)
}

// List handles GET /api/v1/stores/{storeID}/products
// Returns products for a specific store in the customer-safe view.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	storeID := r.URL.Query().Get("store_id")

	// Try to enrich from cached location if no explicit store was requested
	if storeID == "" {
		loc := h.resolveLocation(r)
		storeID = ResolveStoreID("", loc)
	}

	if storeID == "" {
		response.BadRequest(w, "could not determine your nearest store — Allow location access or pass store_id")
		return
	}

	products, err := h.service.ListForCustomer(r.Context(), storeID)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, products)
}

// Get returns a single product with all store prices for comparison.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	storeID := r.URL.Query().Get("store_id")

	if storeID == "" {
		loc := h.resolveLocation(r)
		storeID = ResolveStoreID("", loc)
	}

		// Product detail with nearest store price
	var customerView *ProductCustomerResponse
	if storeID != "" {
		p, err := h.service.GetForCustomer(r.Context(), id, storeID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				response.NotFound(w, "product not found")
				return
			}
			response.InternalServerError(w)
			return
		}
		customerView = p
	}

	// All store prices for the comparison section
	allPrices, err := h.service.GetAllStorePrices(r.Context(), id)
	if err != nil {
		response.InternalServerError(w)
		return
	}

	response.Success(w, map[string]interface{}{
		"product":    customerView,
		"all_prices": allPrices,
	})
	}

// ── Staff handlers ────────────────────────────────────────────────────────────

// ListForStaff returns all products with full inventory detail for the scoped store.
func (h *Handler) ListForStaff(w http.ResponseWriter, r *http.Request) {
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}

	products, err := h.service.ListForStaff(r.Context(), storeID)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, products)
}

// GetForStaff returns a single product with full inventory detail.
func (h *Handler) GetForStaff(w http.ResponseWriter, r *http.Request) {
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}

	p, err := h.service.GetForStaff(r.Context(), chi.URLParam(r, "id"), storeID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "product not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, p)
}

// ── Admin handlers ────────────────────────────────────────────────────────────

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	requestedBy := users.UserFromContext(r.Context())

	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	product, err := h.service.Create(r.Context(), req, requestedBy)
	if err != nil {
		response.Forbidden(w, err.Error())
		return
	}
	response.Created(w, product)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	requestedBy := users.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	product, err := h.service.Update(r.Context(), id, req, requestedBy)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "product not found")
			return
		}
		response.Forbidden(w, err.Error())
		return
	}
	response.Success(w, product)
}

func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	requestedBy := users.UserFromContext(r.Context())
	if err := h.service.Deactivate(r.Context(), chi.URLParam(r, "id"), requestedBy); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "product not found")
			return
		}
		response.Forbidden(w, err.Error())
		return
	}
	response.NoContent(w)
}

func (h *Handler) Reactivate(w http.ResponseWriter, r *http.Request) {
	requestedBy := users.UserFromContext(r.Context())
	if err := h.service.Reactivate(r.Context(), chi.URLParam(r, "id"), requestedBy); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "product not found")
			return
		}
		response.Forbidden(w, err.Error())
		return
	}
	response.NoContent(w)
}

// ── Helper ────────────────────────────────────────────────────────────────────

func (h *Handler) resolveLocation(r *http.Request) *geo.CachedLocation {
	user := users.UserFromContext(r.Context())
	var key string
	if user != nil {
		key = geo.LocationKey(user.ID, "")
	} else {
		key = geo.LocationKey("", r.Header.Get("X-Session-ID"))
	}
	loc, _ := h.locationService.Get(r.Context(), key)
	return loc
}