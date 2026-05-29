package users

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ── Customer ──────────────────────────────────────────────────────────────────

// CreateCustomer registers a new customer account.
// Called from the auth handler on the public POST /auth/register route.
func (s *Service) CreateCustomer(ctx context.Context, req RegisterRequest) (*UserResponse, error) {
	hash, err := hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("users: failed to hash password: %w", err)
	}

	u, err := s.repo.Create(ctx, &User{
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: hash,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         RoleCustomer,
	})
	if err != nil {
		return nil, err // ErrEmailTaken / ErrPhoneTaken already mapped in repository
	}

	resp := ToResponse(u)
	return &resp, nil
}

// ── Staff ─────────────────────────────────────────────────────────────────────

// CreateStaff creates a cashier or admin account and assigns them to a store.
//
// Permission rules:
//   - Admin can create cashiers for their own store only
//   - SuperAdmin can create cashiers or admins for any store
func (s *Service) CreateStaff(ctx context.Context, req CreateStaffRequest, createdBy *User) (*StaffResponse, error) {
	switch req.Role {
	case RoleCashier:
		if !createdBy.Role.IsAtLeast(RoleAdmin) {
			return nil, errors.New("only admins and superadmin can create cashier accounts")
		}
	case RoleAdmin:
		if createdBy.Role != RoleSuperAdmin {
			return nil, errors.New("only superadmin can create admin accounts")
		}
	default:
		return nil, errors.New("role must be cashier or admin")
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("users: failed to hash password: %w", err)
	}

	u, err := s.repo.Create(ctx, &User{
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: hash,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         req.Role,
	})
	if err != nil {
		return nil, err
	}

	// Assign to store immediately on creation
	if err := s.repo.AssignToStore(ctx, u.ID, req.StoreID, createdBy.ID); err != nil {
		return nil, fmt.Errorf("users: created staff but failed to assign to store: %w", err)
	}

	return &StaffResponse{
		UserResponse: ToResponse(u),
		StoreID:      req.StoreID,
	}, nil
}

// DeactivateUser deactivates an account, preventing login.
//
// Permission rules:
//   - SuperAdmin can deactivate any account except their own
//   - Admin can deactivate cashiers in their store only
//   - The superadmin account can never be deactivated
func (s *Service) DeactivateUser(ctx context.Context, targetID string, requestedBy *User) error {
	if targetID == requestedBy.ID {
		return errors.New("you cannot deactivate your own account")
	}

	target, err := s.repo.GetByID(ctx, targetID)
	if err != nil {
		return err
	}

	if target.Role == RoleSuperAdmin {
		return errors.New("the superadmin account cannot be deactivated")
	}

	// Admins can only deactivate cashiers, not other admins
	if requestedBy.Role == RoleAdmin && target.Role != RoleCashier {
		return errors.New("admins can only deactivate cashier accounts")
	}

	return s.repo.SetActive(ctx, targetID, false)
}

// ReactivateUser re-enables a deactivated account. SuperAdmin only.
func (s *Service) ReactivateUser(ctx context.Context, targetID string) error {
	if _, err := s.repo.GetByID(ctx, targetID); err != nil {
		return err
	}
	return s.repo.SetActive(ctx, targetID, true)
}

// AssignToStore assigns or reassigns a staff member to a store. SuperAdmin only.
func (s *Service) AssignToStore(ctx context.Context, userID, storeID string, assignedBy *User) error {
	if assignedBy.Role != RoleSuperAdmin {
		return errors.New("only superadmin can assign staff to stores")
	}

	target, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !target.Role.IsStaff() {
		return errors.New("only cashiers and admins can be assigned to stores")
	}

	return s.repo.AssignToStore(ctx, userID, storeID, assignedBy.ID)
}

// ── Profile ───────────────────────────────────────────────────────────────────

// GetProfile returns the profile for the given user ID.
func (s *Service) GetProfile(ctx context.Context, userID string) (*UserResponse, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := ToResponse(u)
	return &resp, nil
}

// UpdateProfile updates editable fields for the given user.
func (s *Service) UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*UserResponse, error) {
	u, err := s.repo.UpdateProfile(ctx, userID, req)
	if err != nil {
		return nil, err
	}
	resp := ToResponse(u)
	return &resp, nil
}

// ── Staff lists ───────────────────────────────────────────────────────────────

// ListStoreStaff returns all staff assigned to a store, with their assignment info.
func (s *Service) ListStoreStaff(ctx context.Context, storeID string) ([]StaffResponse, error) {
	users, err := s.repo.ListByStore(ctx, storeID)
	if err != nil {
		return nil, err
	}

	result := make([]StaffResponse, 0, len(users))
	for _, u := range users {
		sr := StaffResponse{UserResponse: ToResponse(u)}
		if assignment, err := s.repo.GetStoreAssignment(ctx, u.ID); err == nil {
			sr.StoreID = assignment.StoreID
			sr.AssignedAt = assignment.AssignedAt
		}
		result = append(result, sr)
	}
	return result, nil
}

// ListAdmins returns all admin accounts with their store assignments. SuperAdmin only.
func (s *Service) ListAdmins(ctx context.Context) ([]StaffResponse, error) {
	admins, err := s.repo.ListByRole(ctx, RoleAdmin)
	if err != nil {
		return nil, err
	}

	result := make([]StaffResponse, 0, len(admins))
	for _, u := range admins {
		sr := StaffResponse{UserResponse: ToResponse(u)}
		if assignment, err := s.repo.GetStoreAssignment(ctx, u.ID); err == nil {
			sr.StoreID = assignment.StoreID
			sr.AssignedAt = assignment.AssignedAt
		}
		result = append(result, sr)
	}
	return result, nil
}

// ── Methods used by other packages ───────────────────────────────────────────
// These are intentionally kept minimal — only expose what other packages need.

// GetByEmail fetches a user by email. Used by the auth service during login.
func (s *Service) GetByEmail(ctx context.Context, email string) (*User, error) {
	return s.repo.GetByEmail(ctx, email)
}

// GetByID fetches a user by ID. Used by middleware to hydrate JWT claims.
func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

// GetStoreAssignment returns the store a staff member is assigned to.
// Used by StoreScope middleware to determine which store a staff user can access.
func (s *Service) GetStoreAssignment(ctx context.Context, userID string) (*StoreAssignment, error) {
	return s.repo.GetStoreAssignment(ctx, userID)
}

// VerifyPassword checks a plaintext password against a bcrypt hash.
// Used by the auth service during login.
func (s *Service) VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}