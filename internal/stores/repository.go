package stores

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/payments"
	"github.com/Bessmack/hardware-store-api/pkg/crypto"
	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("store not found")

type Repository struct {
	db     *database.DB
	cipher *crypto.Cipher
}

func NewRepository(db *database.DB, c *crypto.Cipher) *Repository {
	return &Repository{db: db, cipher: c}
}

// Create inserts a new store and returns the created record.
func (r *Repository) Create(ctx context.Context, req CreateStoreRequest) (*Store, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO stores (name, address, county, latitude, longitude, phone, email, currency)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING
			id, name,
			COALESCE(address, ''), COALESCE(county, ''),
			latitude, longitude,
			COALESCE(phone, ''), COALESCE(email, ''),
			COALESCE(mpesa_paybill, ''), COALESCE(mpesa_account_ref, ''),
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(mpesa_consumer_key, ''), COALESCE(mpesa_consumer_secret, ''),
			COALESCE(airtel_merchant_id, ''),
			COALESCE(currency, 'KES'),
			is_active, created_at, updated_at
	`,
		req.Name, nullIfEmpty(req.Address), nullIfEmpty(req.County),
		req.Latitude, req.Longitude,
		nullIfEmpty(req.Phone), nullIfEmpty(req.Email),
		nullIfEmpty(req.Currency),
	)

	s, err := scanStore(row)
	if err != nil {
		return nil, err
	}
	if err := r.decryptStore(s); err != nil {
		return nil, err
	}
	return s, nil
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
			COALESCE(mpesa_consumer_key, ''), COALESCE(mpesa_consumer_secret, ''),
			COALESCE(airtel_merchant_id, ''),
			COALESCE(currency, 'KES'),
			is_active, created_at, updated_at
		FROM stores WHERE id = $1
	`, id)

	s, err := scanStore(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := r.decryptStore(s); err != nil {
		return nil, err
	}
	return s, nil
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
			email     = COALESCE(NULLIF($7, ''), email),
			currency  = COALESCE(NULLIF($8, ''), currency)
		WHERE id = $9
		RETURNING
			id, name,
			COALESCE(address, ''), COALESCE(county, ''),
			latitude, longitude,
			COALESCE(phone, ''), COALESCE(email, ''),
			COALESCE(mpesa_paybill, ''), COALESCE(mpesa_account_ref, ''),
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(mpesa_consumer_key, ''), COALESCE(mpesa_consumer_secret, ''),
			COALESCE(airtel_merchant_id, ''),
			COALESCE(currency, 'KES'),
			is_active, created_at, updated_at
	`,
		req.Name, req.Address, req.County,
		req.Latitude, req.Longitude,
		req.Phone, req.Email,
		req.Currency, id,
	)

	s, err := scanStore(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := r.decryptStore(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) UpdateCredentials(ctx context.Context, id string, req UpdateCredentialsRequest) (*Store, error) {
	// Encrypt sensitive fields before writing them to the database
	var (
		encPasskey        = req.MpesaPasskey
		encConsumerKey    = req.MpesaConsumerKey
		encConsumerSecret = req.MpesaConsumerSecret
		encAirtelID       = req.AirtelMerchantID
		err               error
	)
	if r.cipher != nil {
		if req.MpesaPasskey != "" {
			encPasskey, err = r.cipher.Encrypt(req.MpesaPasskey)
			if err != nil {
				return nil, fmt.Errorf("stores: encrypt passkey: %w", err)
			}
		}
		if req.MpesaConsumerKey != "" {
			encConsumerKey, err = r.cipher.Encrypt(req.MpesaConsumerKey)
			if err != nil {
				return nil, fmt.Errorf("stores: encrypt consumer key: %w", err)
			}
		}
		if req.MpesaConsumerSecret != "" {
			encConsumerSecret, err = r.cipher.Encrypt(req.MpesaConsumerSecret)
			if err != nil {
				return nil, fmt.Errorf("stores: encrypt consumer secret: %w", err)
			}
		}
		if req.AirtelMerchantID != "" {
			encAirtelID, err = r.cipher.Encrypt(req.AirtelMerchantID)
			if err != nil {
				return nil, fmt.Errorf("stores: encrypt airtel id: %w", err)
			}
		}
	}

	row := r.db.Pool.QueryRow(ctx, `
		UPDATE stores SET
			mpesa_paybill       = COALESCE(NULLIF($1, ''), mpesa_paybill),
			mpesa_account_ref   = COALESCE(NULLIF($2, ''), mpesa_account_ref),
			mpesa_shortcode     = COALESCE(NULLIF($3, ''), mpesa_shortcode),
			mpesa_passkey       = COALESCE(NULLIF($4, ''), mpesa_passkey),
			mpesa_consumer_key      = COALESCE(NULLIF($5, ''), mpesa_consumer_key),
			mpesa_consumer_secret   = COALESCE(NULLIF($6, ''), mpesa_consumer_secret),
			airtel_merchant_id  = COALESCE(NULLIF($7, ''), airtel_merchant_id)
		WHERE id = $8
		RETURNING
			id, name,
			COALESCE(address, ''), COALESCE(county, ''),
			latitude, longitude,
			COALESCE(phone, ''), COALESCE(email, ''),
			COALESCE(mpesa_paybill, ''), COALESCE(mpesa_account_ref, ''),
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(mpesa_consumer_key, ''), COALESCE(mpesa_consumer_secret, ''),
			COALESCE(airtel_merchant_id, ''),
			COALESCE(currency, 'KES'),
			is_active, created_at, updated_at
	`,
		req.MpesaPaybill, req.MpesaAccountRef, req.MpesaShortcode,
		encPasskey, encConsumerKey, encConsumerSecret,
		encAirtelID, id,
	)

	s, err := scanStore(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := r.decryptStore(s); err != nil {
		return nil, err
	}
	return s, nil
}

// SetActive activates or deactivates a store.
func (r *Repository) SetActive(ctx context.Context, id string, active bool) error {
	result, err := r.db.Pool.Exec(ctx,
		`UPDATE stores SET is_active = $1 WHERE id = $2`, active, id)
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
	return r.list(ctx, `WHERE is_active = TRUE`)
}

func (r *Repository) ListInactive(ctx context.Context) ([]*Store, error) {
	return r.list(ctx, `WHERE is_active = FALSE`)
}

// ListAll returns every store including inactive — superadmin only.
func (r *Repository) ListAll(ctx context.Context) ([]*Store, error) {
	return r.list(ctx, ``)
}

// list is the shared query for all list variants.
// The filter string is appended as a WHERE clause (or empty for all stores).
func (r *Repository) list(ctx context.Context, filter string) ([]*Store, error) {
	query := `
		SELECT
			id, name,
			COALESCE(address, ''), COALESCE(county, ''),
			latitude, longitude,
			COALESCE(phone, ''), COALESCE(email, ''),
			COALESCE(mpesa_paybill, ''), COALESCE(mpesa_account_ref, ''),
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(mpesa_consumer_key, ''), COALESCE(mpesa_consumer_secret, ''),
			COALESCE(airtel_merchant_id, ''),
			COALESCE(currency, 'KES'),
			is_active, created_at, updated_at
		FROM stores
	` + filter + ` ORDER BY county, name`

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
		if err := r.decryptStore(s); err != nil {
			return nil, err
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
		&s.MpesaConsumerKey, &s.MpesaConsumerSecret,
		&s.AirtelMerchantID,
		&s.Currency,
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
		&s.MpesaConsumerKey, &s.MpesaConsumerSecret,
		&s.AirtelMerchantID,
		&s.Currency,
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

// decryptStore decrypts sensitive fields on a Store after reading from DB.
func (r *Repository) decryptStore(s *Store) error {
	if r.cipher == nil || s == nil {
		return nil
	}
	var err error
	if s.MpesaPasskey != "" {
		s.MpesaPasskey, err = r.cipher.Decrypt(s.MpesaPasskey)
		if err != nil {
			return fmt.Errorf("stores: decrypt mpesa_passkey: %w", err)
		}
	}
	if s.MpesaConsumerKey != "" {
		s.MpesaConsumerKey, err = r.cipher.Decrypt(s.MpesaConsumerKey)
		if err != nil {
			return fmt.Errorf("stores: decrypt mpesa_consumer_key: %w", err)
		}
	}
	if s.MpesaConsumerSecret != "" {
		s.MpesaConsumerSecret, err = r.cipher.Decrypt(s.MpesaConsumerSecret)
		if err != nil {
			return fmt.Errorf("stores: decrypt mpesa_consumer_secret: %w", err)
		}
	}
	if s.AirtelMerchantID != "" {
		s.AirtelMerchantID, err = r.cipher.Decrypt(s.AirtelMerchantID)
		if err != nil {
			return fmt.Errorf("stores: decrypt airtel_merchant_id: %w", err)
		}
	}
	return nil
}

// ── delivery.StoreCoordinateReader implementation ────────────────────────────

// GetStoreCoordinates satisfies the delivery.StoreCoordinateReader interface.
// Returns the store's name, lat/lng, and currency for delivery quote calculation.
func (r *Repository) GetStoreCoordinates(ctx context.Context, storeID string) (name string, lat, lng float64, currency string, err error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT name, latitude, longitude, COALESCE(currency, 'KES')
		 FROM stores WHERE id = $1 AND is_active = TRUE`, storeID)
	if scanErr := row.Scan(&name, &lat, &lng, &currency); scanErr != nil {
		return "", 0, 0, "", fmt.Errorf("stores: store not found: %w", scanErr)
	}
	return name, lat, lng, currency, nil
}

// GetPaymentCredentials satisfies the payments.StoreCredentialsReader interface.
// Returns the store's payment credentials for processing transactions.
func (r *Repository) GetPaymentCredentials(ctx context.Context, storeID string) (*payments.StoreCredentials, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			COALESCE(mpesa_shortcode, ''), COALESCE(mpesa_passkey, ''),
			COALESCE(mpesa_account_ref, ''), COALESCE(airtel_merchant_id, ''),
			COALESCE(currency, 'KES'),
			COALESCE(mpesa_consumer_key, ''), COALESCE(mpesa_consumer_secret, '')
		FROM stores WHERE id = $1
	`, storeID)

	var creds payments.StoreCredentials
	if err := row.Scan(
		&creds.MpesaShortcode, &creds.MpesaPasskey,
		&creds.MpesaAccountRef, &creds.AirtelMerchantID,
		&creds.Currency,
		&creds.MpesaConsumerKey, &creds.MpesaConsumerSecret,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Decrypt sensitive fields before returning
	if r.cipher != nil {
		var dErr error
		if creds.MpesaPasskey != "" {
			creds.MpesaPasskey, dErr = r.cipher.Decrypt(creds.MpesaPasskey)
			if dErr != nil {
				return nil, fmt.Errorf("stores: decrypt mpesa_passkey: %w", dErr)
			}
		}
		if creds.MpesaConsumerKey != "" {
			creds.MpesaConsumerKey, dErr = r.cipher.Decrypt(creds.MpesaConsumerKey)
			if dErr != nil {
				return nil, fmt.Errorf("stores: decrypt mpesa_consumer_key: %w", dErr)
			}
		}
		if creds.MpesaConsumerSecret != "" {
			creds.MpesaConsumerSecret, dErr = r.cipher.Decrypt(creds.MpesaConsumerSecret)
			if dErr != nil {
				return nil, fmt.Errorf("stores: decrypt mpesa_consumer_secret: %w", dErr)
			}
		}
		if creds.AirtelMerchantID != "" {
			creds.AirtelMerchantID, dErr = r.cipher.Decrypt(creds.AirtelMerchantID)
			if dErr != nil {
				return nil, fmt.Errorf("stores: decrypt airtel_merchant_id: %w", dErr)
			}
		}
	}

	return &creds, nil
}

// GetStoreInfo satisfies orders.StoreInfoReader.
func (r *Repository) GetStoreInfo(ctx context.Context, storeID string) (name, county, currency string, err error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT name, COALESCE(county, ''), COALESCE(currency, 'KES') FROM stores WHERE id = $1`,
		storeID,
	)
	if scanErr := row.Scan(&name, &county, &currency); scanErr != nil {
		return "", "", "", fmt.Errorf("stores: not found: %w", scanErr)
	}
	return name, county, currency, nil
}