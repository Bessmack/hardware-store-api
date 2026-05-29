package users

import "time"

// ── Roles ─────────────────────────────────────────────────────────────────────

type Role string

const (
	RoleCustomer   Role = "customer"
	RoleCashier    Role = "cashier"
	RoleAdmin      Role = "admin"
	RoleSuperAdmin Role = "superadmin"
)

// IsStaff returns true for cashier and admin — roles scoped to a specific store.
func (r Role) IsStaff() bool {
	return r == RoleCashier || r == RoleAdmin
}

// CanManageStore returns true for roles allowed to manage store operations.
func (r Role) CanManageStore() bool {
	return r == RoleAdmin || r == RoleSuperAdmin
}

// IsAtLeast returns true if this role is equal to or above the given role
// in the hierarchy: customer < cashier < admin < superadmin.
func (r Role) IsAtLeast(minimum Role) bool {
	order := map[Role]int{
		RoleCustomer:   1,
		RoleCashier:    2,
		RoleAdmin:      3,
		RoleSuperAdmin: 4,
	}
	return order[r] >= order[minimum]
}

// ── Core model ────────────────────────────────────────────────────────────────

type User struct {
	ID           string    `db:"id"`
	Email        string    `db:"email"`
	Phone        string    `db:"phone"`
	PasswordHash string    `db:"password_hash"` // never serialized to JSON
	FirstName    string    `db:"first_name"`
	LastName     string    `db:"last_name"`
	Role         Role      `db:"role"`
	IsActive     bool      `db:"is_active"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// FullName returns first and last name combined.
func (u *User) FullName() string {
	if u.LastName == "" {
		return u.FirstName
	}
	return u.FirstName + " " + u.LastName
}

type StoreAssignment struct {
	UserID     string    `db:"user_id"`
	StoreID    string    `db:"store_id"`
	AssignedAt time.Time `db:"assigned_at"`
	AssignedBy string    `db:"assigned_by"`
}

// ── Request types ─────────────────────────────────────────────────────────────

// RegisterRequest is used for customer self-registration (public route).
type RegisterRequest struct {
	Email     string `json:"email"      validate:"required,email"`
	Phone     string `json:"phone"      validate:"required"`
	Password  string `json:"password"   validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name"`
}

// CreateStaffRequest is used by admins (cashier) and superadmin (admin).
type CreateStaffRequest struct {
	Email     string `json:"email"      validate:"required,email"`
	Phone     string `json:"phone"      validate:"required"`
	Password  string `json:"password"   validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name"`
	// Role must be cashier (created by admin/superadmin) or admin (superadmin only)
	Role    Role   `json:"role"     validate:"required,oneof=cashier admin"`
	StoreID string `json:"store_id" validate:"required"`
}

// UpdateProfileRequest is used by customers updating their own profile.
type UpdateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

// AssignToStoreRequest is used by superadmin to assign/reassign a staff member.
type AssignToStoreRequest struct {
	StoreID string `json:"store_id" validate:"required"`
}

// ── Response types ────────────────────────────────────────────────────────────

// UserResponse is the safe public representation — PasswordHash is absent.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name,omitempty"`
	FullName  string    `json:"full_name"`
	Role      Role      `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// StaffResponse extends UserResponse with the store the staff member is assigned to.
type StaffResponse struct {
	UserResponse
	StoreID    string    `json:"store_id,omitempty"`
	AssignedAt time.Time `json:"assigned_at,omitempty"`
}

// ToResponse safely converts an internal User into a UserResponse.
// This is the only function that should be used to serialize a user outward.
func ToResponse(u *User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Phone:     u.Phone,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		FullName:  u.FullName(),
		Role:      u.Role,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
	}
}