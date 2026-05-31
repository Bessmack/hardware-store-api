package stores

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("store not found")

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new store and returns the created record.
func (r *Repository) Create(ctx context.Context, req CreateStoreRequest) (*Store, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO stores (name, address, county, latitude, longitude, phone, email)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id, name,
			COALESCE(address, ''), COALESCE(county, ''),
			latitude, longitude,
			COALESCE(phone, ''), COALESCE(email, ''),
			COALESCE(mpesa_paybill, ''), COALESCE(mpesa_account_ref, ''),
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(airtel_merchant_id, ''),
			is_active, created_at, updated_at
	`,
		req.Name, nullIfEmpty(req.Address), nullIfEmpty(req.County),
		req.Latitude, req.Longitude,
		nullIfEmpty(req.Phone), nullIfEmpty(req.Email),
	)

	return scanStore(row)
}

// GetByID fetches a store by UUID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Store, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			id, name,
			COALESCE(address, ''), COALESCE(county, ''),
			latitude, longitude,
			COALESCE(phone, ''), COALESCE(email, ''),
			COALESCE(mpesa_paybill, ''), COALESCE(mpesa_account_ref, ''),
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(airtel_merchant_id, ''),
			is_active, created_at, updated_at
		FROM stores WHERE id = $1
	`, id)

	s, err := scanStore(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// Update updates editable store fields. Uses COALESCE so empty strings leave
// existing values untouched — partial updates are safe.
func (r *Repository) Update(ctx context.Context, id string, req UpdateStoreRequest) (*Store, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE stores SET
			name      = COALESCE(NULLIF($1, ''), name),
			address   = COALESCE(NULLIF($2, ''), address),
			county    = COALESCE(NULLIF($3, ''), county),
			latitude  = CASE WHEN $4 != 0 THEN $4 ELSE latitude END,
			longitude = CASE WHEN $5 != 0 THEN $5 ELSE longitude END,
			phone     = COALESCE(NULLIF($6, ''), phone),
			email     = COALESCE(NULLIF($7, ''), email)
		WHERE id = $8
		RETURNING
			id, name,
			COALESCE(address, ''), COALESCE(county, ''),
			latitude, longitude,
			COALESCE(phone, ''), COALESCE(email, ''),
			COALESCE(mpesa_paybill, ''), COALESCE(mpesa_account_ref, ''),
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(airtel_merchant_id, ''),
			is_active, created_at, updated_at
	`,
		req.Name, req.Address, req.County,
		req.Latitude, req.Longitude,
		req.Phone, req.Email,
		id,
	)

	s, err := scanStore(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// UpdateCredentials sets or replaces a store's payment credentials.
// Only fields with non-empty values are updated — send empty string to leave unchanged.
func (r *Repository) UpdateCredentials(ctx context.Context, id string, req UpdateCredentialsRequest) (*Store, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE stores SET
			mpesa_paybill       = COALESCE(NULLIF($1, ''), mpesa_paybill),
			mpesa_account_ref   = COALESCE(NULLIF($2, ''), mpesa_account_ref),
			mpesa_shortcode     = COALESCE(NULLIF($3, ''), mpesa_shortcode),
			mpesa_passkey       = COALESCE(NULLIF($4, ''), mpesa_passkey),
			airtel_merchant_id  = COALESCE(NULLIF($5, ''), airtel_merchant_id)
		WHERE id = $6
		RETURNING
			id, name,
			COALESCE(address, ''), COALESCE(county, ''),
			latitude, longitude,
			COALESCE(phone, ''), COALESCE(email, ''),
			COALESCE(mpesa_paybill, ''), COALESCE(mpesa_account_ref, ''),
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(airtel_merchant_id, ''),
			is_active, created_at, updated_at
	`,
		req.MpesaPaybill, req.MpesaAccountRef,
		req.MpesaShortcode, req.MpesaPasskey,
		req.AirtelMerchantID,
		id,
	)

	s, err := scanStore(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// SetActive activates or deactivates a store.
func (r *Repository) SetActive(ctx context.Context, id string, active bool) error {
	result, err := r.db.Pool.Exec(ctx,
		`UPDATE stores SET is_active = $1 WHERE id = $2`,
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

// ListActive returns all active stores — used by customers and geo routing.
func (r *Repository) ListActive(ctx context.Context) ([]*Store, error) {
	return r.list(ctx, true, false)
}

// ListAll returns every store including inactive — superadmin only.
func (r *Repository) ListAll(ctx context.Context) ([]*Store, error) {
	return r.list(ctx, false, true)
}

func (r *Repository) list(ctx context.Context, activeOnly, includeInactive bool) ([]*Store, error) {
	query := `
		SELECT
			id, name,
			COALESCE(address, ''), COALESCE(county, ''),
			latitude, longitude,
			COALESCE(phone, ''), COALESCE(email, ''),
			COALESCE(mpesa_paybill, ''), COALESCE(mpesa_account_ref, ''),
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(airtel_merchant_id, ''),
			is_active, created_at, updated_at
		FROM stores
	`
	if activeOnly {
		query += " WHERE is_active = TRUE"
	}
	query += " ORDER BY county, name"

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Store
	for rows.Next() {
		s, err := scanStoreFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("stores: scan error: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ── geo.StoreLister implementation ────────────────────────────────────────────
// The geo package uses this to find the nearest store to a customer's location.
// Defined on Repository directly so geo.LocationService can use it without
// needing the full stores.Service.

// ListActiveStores satisfies the geo.StoreLister interface.
// Returns only the fields the geo package needs for distance calculations.
func (r *Repository) ListActiveStores(ctx context.Context) ([]geo.StoreInfo, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, name, COALESCE(county, ''), latitude, longitude
		FROM stores
		WHERE is_active = TRUE
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []geo.StoreInfo
	for rows.Next() {
		var s geo.StoreInfo
		if err := rows.Scan(&s.ID, &s.Name, &s.County, &s.Latitude, &s.Longitude); err != nil {
			return nil, fmt.Errorf("stores: geo scan error: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func scanStore(row pgx.Row) (*Store, error) {
	var s Store
	if err := row.Scan(
		&s.ID, &s.Name, &s.Address, &s.County,
		&s.Latitude, &s.Longitude,
		&s.Phone, &s.Email,
		&s.MpesaPaybill, &s.MpesaAccountRef,
		&s.MpesaShortcode, &s.MpesaPasskey,
		&s.AirtelMerchantID,
		&s.IsActive, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

func scanStoreFromRows(rows pgx.Rows) (*Store, error) {
	var s Store
	if err := rows.Scan(
		&s.ID, &s.Name, &s.Address, &s.County,
		&s.Latitude, &s.Longitude,
		&s.Phone, &s.Email,
		&s.MpesaPaybill, &s.MpesaAccountRef,
		&s.MpesaShortcode, &s.MpesaPasskey,
		&s.AirtelMerchantID,
		&s.IsActive, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}