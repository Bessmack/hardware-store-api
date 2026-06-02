package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Bessmack/hardware-store-api/internal/middleware"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/Bessmack/hardware-store-api/pkg/validator"
	"github.com/go-chi/chi/v5"
)

// ── Service ───────────────────────────────────────────────────────────────────

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Upsert creates or replaces an inventory entry. Admin and superadmin only.
// Used when adding a product to a store for the first time.
func (s *Service) Upsert(ctx context.Context, storeID string, req UpsertRequest, by *users.User) (*StoreInventory, error) {
	if !by.Role.CanManageStore() {
		return nil, errors.New("only admins and superadmin can manage inventory")
	}
	return s.repo.Upsert(ctx, storeID, req, by.ID)
}

// UpdatePrice changes the price of a product at the given store.
// The DB trigger automatically records the change in inventory_price_history.
func (s *Service) UpdatePrice(ctx context.Context, storeID, productID string, req UpdatePriceRequest, by *users.User) (*StoreInventory, error) {
	if !by.Role.CanManageStore() {
		return nil, errors.New("only admins and superadmin can update prices")
	}
	return s.repo.UpdatePrice(ctx, storeID, productID, req, by.ID)
}

// UpdateStock adjusts stock quantity and availability. Admin and superadmin only.
func (s *Service) UpdateStock(ctx context.Context, storeID, productID string, req UpdateStockRequest, by *users.User) (*StoreInventory, error) {
	if !by.Role.CanManageStore() {
		return nil, errors.New("only admins and superadmin can update stock")
	}
	return s.repo.UpdateStock(ctx, storeID, productID, req, by.ID)
}

// ListByStore returns all inventory entries for the given store.
func (s *Service) ListByStore(ctx context.Context, storeID string) ([]InventoryResponse, error) {
	return s.repo.ListByStore(ctx, storeID)
}

// GetPriceHistory returns the price audit trail for a product at a store.
func (s *Service) GetPriceHistory(ctx context.Context, storeID, productID string) ([]PriceHistoryResponse, error) {
	return s.repo.GetPriceHistory(ctx, storeID, productID)
}

// ── Handler ───────────────────────────────────────────────────────────────────

// Handler is in the same file as Service since inventory has no public routes —
// all endpoints are store-scoped admin/superadmin only.

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Routes (registered in server/routes.go) ───────────────────────────────────
//
// All routes require: RequireAuth + RequireRole(admin, superadmin) + StoreScope
//
//   GET  /api/v1/store/inventory                        List all inventory for store
//   POST /api/v1/store/inventory                        Add product to store (upsert)
//   PUT  /api/v1/store/inventory/:productID/price       Update price
//   PUT  /api/v1/store/inventory/:productID/stock       Update stock + availability
//   GET  /api/v1/store/inventory/:productID/history     Price change audit trail

// List returns all inventory entries for the store.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}

	items, err := h.service.ListByStore(r.Context(), storeID)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, items)
}

// Upsert adds a product to a store's inventory or resets an existing entry.
func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}

	var req UpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	inv, err := h.service.Upsert(r.Context(), storeID, req, by)
	if err != nil {
		response.Forbidden(w, err.Error())
		return
	}
	response.Created(w, inv)
}

// UpdatePrice changes the price of a product at the scoped store.
func (h *Handler) UpdatePrice(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	productID := chi.URLParam(r, "productID")

	var req UpdatePriceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	inv, err := h.service.UpdatePrice(r.Context(), storeID, productID, req, by)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "product not found in this store's inventory")
			return
		}
		response.Forbidden(w, err.Error())
		return
	}
	response.Success(w, inv)
}

// UpdateStock adjusts stock quantity and availability.
func (h *Handler) UpdateStock(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	productID := chi.URLParam(r, "productID")

	var req UpdateStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	inv, err := h.service.UpdateStock(r.Context(), storeID, productID, req, by)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "product not found in this store's inventory")
			return
		}
		response.Forbidden(w, err.Error())
		return
	}
	response.Success(w, inv)
}

// GetPriceHistory returns the price change audit trail for a product.
func (h *Handler) GetPriceHistory(w http.ResponseWriter, r *http.Request) {
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	productID := chi.URLParam(r, "productID")

	history, err := h.service.GetPriceHistory(r.Context(), storeID, productID)
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, history)
}