package products

import (
	"context"
	"errors"

	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/users"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ── Public / customer-facing ──────────────────────────────────────────────────

// ListForCustomer returns all available products for a store with the
// customer-safe view (no stock quantities).
// The storeID is resolved from the customer's cached location before this is called.
func (s *Service) ListForCustomer(ctx context.Context, storeID string) ([]ProductCustomerResponse, error) {
	products, err := s.repo.ListWithInventory(ctx, storeID)
	if err != nil {
		return nil, err
	}

	result := make([]ProductCustomerResponse, 0, len(products))
	for _, p := range products {
		if p.IsAvailable {
			result = append(result, ToCustomerResponse(p))
		}
	}
	return result, nil
}

// GetForCustomer returns a single product in the customer-safe view.
func (s *Service) GetForCustomer(ctx context.Context, productID, storeID string) (*ProductCustomerResponse, error) {
	p, err := s.repo.GetWithInventory(ctx, productID, storeID)
	if err != nil {
		return nil, err
	}
	resp := ToCustomerResponse(*p)
	return &resp, nil
}

// GetAllStorePrices returns the price of a product across every active store.
// Shown on the product detail page so customers can compare branches.
func (s *Service) GetAllStorePrices(ctx context.Context, productID string) ([]StorePriceEntry, error) {
	return s.repo.GetAllStorePrices(ctx, productID)
}

// ── Staff-facing ──────────────────────────────────────────────────────────────

// ListForStaff returns all products (including inactive) with full inventory
// detail for the given store.
func (s *Service) ListForStaff(ctx context.Context, storeID string) ([]ProductStaffResponse, error) {
	products, err := s.repo.ListWithInventory(ctx, storeID)
	if err != nil {
		return nil, err
	}

	result := make([]ProductStaffResponse, 0, len(products))
	for _, p := range products {
		result = append(result, ToStaffResponse(p))
	}
	return result, nil
}

// GetForStaff returns a single product with full inventory detail.
func (s *Service) GetForStaff(ctx context.Context, productID, storeID string) (*ProductStaffResponse, error) {
	p, err := s.repo.GetWithInventory(ctx, productID, storeID)
	if err != nil {
		return nil, err
	}
	resp := ToStaffResponse(*p)
	return &resp, nil
}

// ── Admin / SuperAdmin ────────────────────────────────────────────────────────

// Create adds a new product to the catalogue. Admin and superadmin only.
func (s *Service) Create(ctx context.Context, req CreateProductRequest, requestedBy *users.User) (*Product, error) {
	if !requestedBy.Role.CanManageStore() {
		return nil, errors.New("only admins and superadmin can create products")
	}
	return s.repo.Create(ctx, req, requestedBy.ID)
}

// Update modifies a product's universal fields. Admin and superadmin only.
func (s *Service) Update(ctx context.Context, id string, req UpdateProductRequest, requestedBy *users.User) (*Product, error) {
	if !requestedBy.Role.CanManageStore() {
		return nil, errors.New("only admins and superadmin can update products")
	}
	return s.repo.Update(ctx, id, req, requestedBy.ID)
}

// Deactivate hides a product from all stores. Admin and superadmin only.
func (s *Service) Deactivate(ctx context.Context, id string, requestedBy *users.User) error {
	if !requestedBy.Role.CanManageStore() {
		return errors.New("only admins and superadmin can deactivate products")
	}
	return s.repo.SetActive(ctx, id, false)
}

// Reactivate re-enables a product. Admin and superadmin only.
func (s *Service) Reactivate(ctx context.Context, id string, requestedBy *users.User) error {
	if !requestedBy.Role.CanManageStore() {
		return errors.New("only admins and superadmin can reactivate products")
	}
	return s.repo.SetActive(ctx, id, true)
}

// ── Nearest store resolution helper ──────────────────────────────────────────

// ResolveStoreID determines which store's prices to use for a customer request.
// Priority:
//  1. Explicit storeID from query param (customer browsing a specific branch)
//  2. NearestStoreID from the customer's cached Redis location
//  3. Empty string — caller should handle gracefully (show all stores / prompt)
func ResolveStoreID(explicit string, location *geo.CachedLocation) string {
	if explicit != "" {
		return explicit
	}
	if location != nil && location.NearestStoreID != "" {
		return location.NearestStoreID
	}
	return ""
}