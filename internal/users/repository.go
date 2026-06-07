package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrNotFound  = errors.New("user not found")
	ErrEmailTaken = errors.New("email address is already in use")
	ErrPhoneTaken = errors.New("phone number is already in use")
)

// ── Repository ────────────────────────────────────────────────────────────────

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new user and returns the created record.
func (r *Repository) Create(ctx context.Context, u *User) (*User, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO users (email, phone, password_hash, first_name, last_name, role)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, COALESCE(phone, ''), password_hash,
		          COALESCE(first_name, ''), COALESCE(last_name, ''),
		          role, is_active, created_at, updated_at
	`, nullIfEmpty(u.Email), nullIfEmpty(u.Phone), u.PasswordHash,
		nullIfEmpty(u.FirstName), nullIfEmpty(u.LastName), u.Role)

	created, err := scanUser(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return created, nil
}

// GetByID fetches a user by their UUID.
func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, email, COALESCE(phone, ''), password_hash,
		       COALESCE(first_name, ''), COALESCE(last_name, ''),
		       role, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`, id)

	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetByEmail fetches a user by email — used during login.
func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, email, COALESCE(phone, ''), password_hash,
		       COALESCE(first_name, ''), COALESCE(last_name, ''),
		       role, is_active, created_at, updated_at
		FROM users WHERE email = $1
	`, email)

	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// UpdateProfile updates editable profile fields.
// Uses COALESCE so empty strings leave existing values untouched.
func (r *Repository) UpdateProfile(ctx context.Context, id string, req UpdateProfileRequest) (*User, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE users
		SET first_name = COALESCE(NULLIF($1, ''), first_name),
		    last_name  = COALESCE(NULLIF($2, ''), last_name),
		    phone      = COALESCE(NULLIF($3, ''), phone)
		WHERE id = $4
		RETURNING id, email, COALESCE(phone, ''), password_hash,
		          COALESCE(first_name, ''), COALESCE(last_name, ''),
		          role, is_active, created_at, updated_at
	`, req.FirstName, req.LastName, req.Phone, id)

	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// SetActive activates or deactivates a user account.
func (r *Repository) SetActive(ctx context.Context, id string, active bool) error {
	result, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET is_active = $1 WHERE id = $2`,
		active, id,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByRole returns all users with the given role, newest first.
func (r *Repository) ListByRole(ctx context.Context, role Role) ([]*User, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, email, COALESCE(phone, ''), password_hash,
		       COALESCE(first_name, ''), COALESCE(last_name, ''),
		       role, is_active, created_at, updated_at
		FROM users
		WHERE role = $1
		ORDER BY created_at DESC
	`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectUsers(rows)
}

// ListByStore returns all staff assigned to the given store.
func (r *Repository) ListByStore(ctx context.Context, storeID string) ([]*User, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.email, COALESCE(u.phone, ''), u.password_hash,
		       COALESCE(u.first_name, ''), COALESCE(u.last_name, ''),
		       u.role, u.is_active, u.created_at, u.updated_at
		FROM users u
		JOIN staff_store_assignments sa ON sa.user_id = u.id
		WHERE sa.store_id = $1
		ORDER BY u.role, u.first_name
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectUsers(rows)
}

// ── Store assignments ─────────────────────────────────────────────────────────

// GetStoreAssignment returns the store assignment for a staff member.
// Returns ErrNotFound if the user is not assigned to any store.
func (r *Repository) GetStoreAssignment(ctx context.Context, userID string) (*StoreAssignment, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT user_id, store_id, assigned_at, COALESCE(assigned_by::text, '')
		FROM staff_store_assignments
		WHERE user_id = $1
	`, userID)

	var sa StoreAssignment
	if err := row.Scan(&sa.UserID, &sa.StoreID, &sa.AssignedAt, &sa.AssignedBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &sa, nil
}

// AssignToStore creates or updates a store assignment for a staff member.
// Uses ON CONFLICT ... DO UPDATE so reassignment is a single upsert.
func (r *Repository) AssignToStore(ctx context.Context, userID, storeID, assignedBy string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO staff_store_assignments (user_id, store_id, assigned_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		    SET store_id    = EXCLUDED.store_id,
		        assigned_by = EXCLUDED.assigned_by,
		        assigned_at = NOW()
	`, userID, storeID, assignedBy)
	return err
}

// UnassignFromStore removes a staff member's store assignment.
func (r *Repository) UnassignFromStore(ctx context.Context, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM staff_store_assignments WHERE user_id = $1`,
		userID,
	)
	return err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func scanUser(row pgx.Row) (*User, error) {
	var u User
	if err := row.Scan(
		&u.ID, &u.Email, &u.Phone, &u.PasswordHash,
		&u.FirstName, &u.LastName, &u.Role,
		&u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &u, nil
}

func collectUsers(rows pgx.Rows) ([]*User, error) {
	var result []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Phone, &u.PasswordHash,
			&u.FirstName, &u.LastName, &u.Role,
			&u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("users: scan error: %w", err)
		}
		result = append(result, &u)
	}
	return result, rows.Err()
}

// mapDBError converts PostgreSQL constraint errors into readable sentinel errors.
func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_email_key":
			return ErrEmailTaken
		case "users_phone_key":
			return ErrPhoneTaken
		}
	}
	return err
}

// nullIfEmpty returns nil for empty strings so PostgreSQL stores NULL instead of "".
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ── orders.CustomerInfoReader implementation ─────────────────────────────────

// GetCustomerInfo returns a customer's display name, phone, and email.
// Used by the orders service to send notifications after order events.
func (r *Repository) GetCustomerInfo(ctx context.Context, customerID string) (name, phone, email string, err error) {
	var firstName, lastName string
	row := r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(first_name,''), COALESCE(last_name,''),
		        COALESCE(phone,''), email
		 FROM users WHERE id = $1`,
		customerID,
	)
	if scanErr := row.Scan(&firstName, &lastName, &phone, &email); scanErr != nil {
		return "", "", "", scanErr
	}
	name = firstName
	if lastName != "" {
		name = firstName + " " + lastName
	}
	return name, phone, email, nil
}