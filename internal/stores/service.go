package stores

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

// ── Public ────────────────────────────────────────────────────────────────────

// ListActive returns all active stores in the public view (no credentials).
// Used by: customers browsing, geo routing, delivery fee calculation.
func (s *Service) ListActive(ctx context.Context) ([]StorePublicResponse, error) {
	stores, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]StorePublicResponse, len(stores))
	for i, store := range stores {
		result[i] = ToPublicResponse(store)
	}
	return result, nil
}

// GetPublic returns a single store in the public view.
// Used on the store detail page visible to customers.
func (s *Service) GetPublic(ctx context.Context, id string) (*StorePublicResponse, error) {
	store, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := ToPublicResponse(store)
	return &resp, nil
}

// ── Staff / Admin ─────────────────────────────────────────────────────────────

// Get returns a store in the staff view (includes payment info, excludes passkey).
// Admin can only get their own store; superadmin can get any.
func (s *Service) Get(ctx context.Context, id string, requestedBy *users.User) (*StoreStaffResponse, error) {
	store, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := ToStaffResponse(store)
	return &resp, nil
}

// ListAll returns all stores (including inactive) in staff view. SuperAdmin only.
func (s *Service) ListAll(ctx context.Context) ([]StoreStaffResponse, error) {
	stores, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]StoreStaffResponse, len(stores))
	for i, store := range stores {
		result[i] = ToStaffResponse(store)
	}
	return result, nil
}

// ── SuperAdmin ────────────────────────────────────────────────────────────────

// Create registers a new store branch. SuperAdmin only.
func (s *Service) Create(ctx context.Context, req CreateStoreRequest, createdBy *users.User) (*StoreStaffResponse, error) {
	if createdBy.Role != users.RoleSuperAdmin {
		return nil, errors.New("only superadmin can create stores")
	}

	store, err := s.repo.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	resp := ToStaffResponse(store)
	return &resp, nil
}

// Update updates a store's basic information. SuperAdmin only.
func (s *Service) Update(ctx context.Context, id string, req UpdateStoreRequest, requestedBy *users.User) (*StoreStaffResponse, error) {
	if requestedBy.Role != users.RoleSuperAdmin {
		return nil, errors.New("only superadmin can update store information")
	}

	store, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}

	resp := ToStaffResponse(store)
	return &resp, nil
}

// UpdateCredentials sets or replaces a store's M-Pesa / Airtel credentials.
// SuperAdmin only. The passkey is stored but never returned in any response.
func (s *Service) UpdateCredentials(ctx context.Context, id string, req UpdateCredentialsRequest, requestedBy *users.User) (*StoreStaffResponse, error) {
	if requestedBy.Role != users.RoleSuperAdmin {
		return nil, errors.New("only superadmin can manage payment credentials")
	}

	store, err := s.repo.UpdateCredentials(ctx, id, req)
	if err != nil {
		return nil, err
	}

	resp := ToStaffResponse(store)
	return &resp, nil
}

// Deactivate makes a store inactive. SuperAdmin only.
// Active orders at the store are not affected — they continue to completion.
func (s *Service) Deactivate(ctx context.Context, id string, requestedBy *users.User) error {
	if requestedBy.Role != users.RoleSuperAdmin {
		return errors.New("only superadmin can deactivate stores")
	}
	return s.repo.SetActive(ctx, id, false)
}

// Reactivate re-enables a deactivated store. SuperAdmin only.
func (s *Service) Reactivate(ctx context.Context, id string, requestedBy *users.User) error {
	if requestedBy.Role != users.RoleSuperAdmin {
		return errors.New("only superadmin can reactivate stores")
	}
	return s.repo.SetActive(ctx, id, true)
}

// ── geo.StoreLister implementation ────────────────────────────────────────────
// The geo package's LocationService calls this to find the nearest store
// whenever a customer's location is saved.

// ListActiveStores satisfies the geo.StoreLister interface.
func (s *Service) ListActiveStores(ctx context.Context) ([]geo.StoreInfo, error) {
	return s.repo.ListActiveStores(ctx)
}

// ── Internal use (payments package) ──────────────────────────────────────────

// GetCredentials returns the full store including the M-Pesa passkey.
// Only called server-side by the payments package to initiate STK push.
// Never called from a handler — no HTTP response ever contains passkey.
func (s *Service) GetCredentials(ctx context.Context, storeID string) (*Store, error) {
	return s.repo.GetByID(ctx, storeID)
}