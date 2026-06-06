package delivery

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/internal/cart"
	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
)

var ErrRateNotFound = errors.New("delivery rate not found")

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// ── Rate lookup ───────────────────────────────────────────────────────────────

// GetRateForStore returns the delivery rate for a vehicle type at a store.
// Resolution order:
//  1. Store-specific rate (store_id = storeID)
//  2. Global default (store_id IS NULL)
//
// This means a store only needs to configure rates it wants to override;
// everything else falls through to the global defaults.
func (r *Repository) GetRateForStore(ctx context.Context, storeID, vehicleType string) (*DeliveryRate, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			COALESCE(store_id::text, ''), vehicle_type,
			base_fee, per_km,
			COALESCE(max_weight_kg, 0), COALESCE(max_radius_km, 0),
			updated_at, COALESCE(updated_by::text, '')
		FROM delivery_rates
		WHERE vehicle_type = $1
		  AND (store_id = $2 OR store_id IS NULL)
		ORDER BY store_id NULLS LAST
		LIMIT 1
	`, vehicleType, storeID)

	rate, err := scanRate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRateNotFound
	}
	return rate, err
}

// ListRatesForStore returns all three vehicle rates for a store, showing
// whether each is a store-specific override or the global default.
// Used in the admin dashboard to manage delivery pricing.
func (r *Repository) ListRatesForStore(ctx context.Context, storeID string) ([]RateResponse, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT DISTINCT ON (vehicle_type)
			COALESCE(store_id::text, ''), vehicle_type,
			base_fee, per_km,
			COALESCE(max_weight_kg, 0), COALESCE(max_radius_km, 0),
			updated_at
		FROM delivery_rates
		WHERE store_id = $1 OR store_id IS NULL
		ORDER BY vehicle_type, store_id NULLS LAST
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RateResponse
	for rows.Next() {
		var storeIDVal string
		var r RateResponse
		if err := rows.Scan(
			&storeIDVal, &r.VehicleType,
			&r.BaseFee, &r.PerKm,
			&r.MaxWeightKg, &r.MaxRadiusKm,
			&r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("delivery: rate scan error: %w", err)
		}
		r.IsStoreRate = storeIDVal != ""
		result = append(result, r)
	}
	return result, rows.Err()
}

// ListGlobalRates returns all global default rates. SuperAdmin only.
func (r *Repository) ListGlobalRates(ctx context.Context) ([]RateResponse, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			COALESCE(store_id::text, ''), vehicle_type,
			base_fee, per_km,
			COALESCE(max_weight_kg, 0), COALESCE(max_radius_km, 0),
			updated_at
		FROM delivery_rates
		WHERE store_id IS NULL
		ORDER BY vehicle_type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RateResponse
	for rows.Next() {
		var storeIDVal string
		var rr RateResponse
		if err := rows.Scan(
			&storeIDVal, &rr.VehicleType,
			&rr.BaseFee, &rr.PerKm,
			&rr.MaxWeightKg, &rr.MaxRadiusKm,
			&rr.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("delivery: global rate scan error: %w", err)
		}
		rr.IsStoreRate = false
		result = append(result, rr)
	}
	return result, rows.Err()
}

// ── Rate management ───────────────────────────────────────────────────────────

// UpsertStoreRate creates or replaces a store-specific delivery rate.
// Used by admins to override the global default for their store.
func (r *Repository) UpsertStoreRate(ctx context.Context, storeID, vehicleType string, req UpdateRateRequest, updatedBy string) (*DeliveryRate, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO delivery_rates
			(store_id, vehicle_type, base_fee, per_km, max_weight_kg, max_radius_km, updated_by)
		VALUES ($1, $2, $3, $4, NULLIF($5, 0), NULLIF($6, 0), $7)
		ON CONFLICT (store_id, vehicle_type)
		DO UPDATE SET
			base_fee      = EXCLUDED.base_fee,
			per_km        = EXCLUDED.per_km,
			max_weight_kg = EXCLUDED.max_weight_kg,
			max_radius_km = EXCLUDED.max_radius_km,
			updated_by    = EXCLUDED.updated_by
		RETURNING
			COALESCE(store_id::text, ''), vehicle_type,
			base_fee, per_km,
			COALESCE(max_weight_kg, 0), COALESCE(max_radius_km, 0),
			updated_at, COALESCE(updated_by::text, '')
	`, storeID, vehicleType, req.BaseFee, req.PerKm, req.MaxWeightKg, req.MaxRadiusKm, updatedBy)

	return scanRate(row)
}

// UpdateGlobalRate updates the global default rate for a vehicle type.
// SuperAdmin only — affects all stores that have not set their own rate.
func (r *Repository) UpdateGlobalRate(ctx context.Context, vehicleType string, req UpdateRateRequest, updatedBy string) (*DeliveryRate, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE delivery_rates
		SET
			base_fee      = $1,
			per_km        = $2,
			max_weight_kg = NULLIF($3, 0),
			max_radius_km = NULLIF($4, 0),
			updated_by    = $5
		WHERE vehicle_type = $6 AND store_id IS NULL
		RETURNING
			COALESCE(store_id::text, ''), vehicle_type,
			base_fee, per_km,
			COALESCE(max_weight_kg, 0), COALESCE(max_radius_km, 0),
			updated_at, COALESCE(updated_by::text, '')
	`, req.BaseFee, req.PerKm, req.MaxWeightKg, req.MaxRadiusKm, updatedBy, vehicleType)

	rate, err := scanRate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRateNotFound
	}
	return rate, err
}

// DeleteStoreRate removes a store's rate override, reverting it to the global default.
func (r *Repository) DeleteStoreRate(ctx context.Context, storeID, vehicleType string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM delivery_rates WHERE store_id = $1 AND vehicle_type = $2`,
		storeID, vehicleType,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRateNotFound
	}
	return nil
}

// ── cart.WeightThresholdReader implementation ─────────────────────────────────

// GetWeightThresholds satisfies the cart.WeightThresholdReader interface.
// Returns the max weight the bike and van can carry for the given store,
// using store-specific rates if they exist, falling back to global defaults.
func (r *Repository) GetWeightThresholds(ctx context.Context, storeID string) (cart.WeightThresholds, error) {
	defaults := cart.WeightThresholds{BikeMaxKg: 30, VanMaxKg: 500}

	bikeRate, err := r.GetRateForStore(ctx, storeID, "bike")
	if err == nil && bikeRate.MaxWeightKg > 0 {
		defaults.BikeMaxKg = bikeRate.MaxWeightKg
	}

	vanRate, err := r.GetRateForStore(ctx, storeID, "van")
	if err == nil && vanRate.MaxWeightKg > 0 {
		defaults.VanMaxKg = vanRate.MaxWeightKg
	}

	return defaults, nil
}

// ── Helper ────────────────────────────────────────────────────────────────────

func scanRate(row pgx.Row) (*DeliveryRate, error) {
	var d DeliveryRate
	if err := row.Scan(
		&d.StoreID, &d.VehicleType,
		&d.BaseFee, &d.PerKm,
		&d.MaxWeightKg, &d.MaxRadiusKm,
		&d.UpdatedAt, &d.UpdatedBy,
	); err != nil {
		return nil, err
	}
	return &d, nil
}