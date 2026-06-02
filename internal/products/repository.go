package products

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrNotFound = errors.New("product not found")

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// ── Product CRUD ──────────────────────────────────────────────────────────────

func (r *Repository) Create(ctx context.Context, req CreateProductRequest, updatedBy string) (*Product, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO products
			(name, description, category, weight_kg, length_cm, width_cm, height_cm,
			 constraint_type, min_vehicle_type, images, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, name, COALESCE(description,''), COALESCE(category,''),
		          COALESCE(weight_kg,0), COALESCE(length_cm,0), COALESCE(width_cm,0), COALESCE(height_cm,0),
		          constraint_type, COALESCE(min_vehicle_type,''),
		          images, is_active, created_at, updated_at, COALESCE(updated_by::text,'')
	`,
		req.Name, nullIfEmpty(req.Description), nullIfEmpty(req.Category),
		nullIfZero(req.WeightKg), nullIfZero(req.LengthCm), nullIfZero(req.WidthCm), nullIfZero(req.HeightCm),
		req.ConstraintType, nullIfEmpty(string(req.MinVehicleType)),
		req.Images, updatedBy,
	)
	return scanProduct(row)
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Product, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(description,''), COALESCE(category,''),
		       COALESCE(weight_kg,0), COALESCE(length_cm,0), COALESCE(width_cm,0), COALESCE(height_cm,0),
		       constraint_type, COALESCE(min_vehicle_type,''),
		       images, is_active, created_at, updated_at, COALESCE(updated_by::text,'')
		FROM products WHERE id = $1
	`, id)
	p, err := scanProduct(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *Repository) Update(ctx context.Context, id string, req UpdateProductRequest, updatedBy string) (*Product, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE products SET
			name            = COALESCE(NULLIF($1,''),  name),
			description     = COALESCE(NULLIF($2,''),  description),
			category        = COALESCE(NULLIF($3,''),  category),
			weight_kg       = CASE WHEN $4 > 0 THEN $4 ELSE weight_kg END,
			length_cm       = CASE WHEN $5 > 0 THEN $5 ELSE length_cm END,
			width_cm        = CASE WHEN $6 > 0 THEN $6 ELSE width_cm END,
			height_cm       = CASE WHEN $7 > 0 THEN $7 ELSE height_cm END,
			constraint_type = COALESCE(NULLIF($8,''),  constraint_type::text)::constraint_type_enum,
			min_vehicle_type= COALESCE(NULLIF($9,''),  min_vehicle_type::text)::vehicle_type_enum,
			updated_by      = $10
		WHERE id = $11
		RETURNING id, name, COALESCE(description,''), COALESCE(category,''),
		          COALESCE(weight_kg,0), COALESCE(length_cm,0), COALESCE(width_cm,0), COALESCE(height_cm,0),
		          constraint_type, COALESCE(min_vehicle_type,''),
		          images, is_active, created_at, updated_at, COALESCE(updated_by::text,'')
	`,
		req.Name, req.Description, req.Category,
		req.WeightKg, req.LengthCm, req.WidthCm, req.HeightCm,
		string(req.ConstraintType), string(req.MinVehicleType),
		updatedBy, id,
	)
	p, err := scanProduct(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *Repository) SetActive(ctx context.Context, id string, active bool) error {
	result, err := r.db.Pool.Exec(ctx,
		`UPDATE products SET is_active = $1 WHERE id = $2`, active, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Joined product + inventory queries ───────────────────────────────────────
// These are the main read paths — they join products with store_inventory
// so callers get price and stock in a single query.

// ListWithInventory returns all active products with their store-specific
// price and stock for the given store.
func (r *Repository) ListWithInventory(ctx context.Context, storeID string) ([]ProductWithInventory, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			p.id, p.name, COALESCE(p.description,''), COALESCE(p.category,''),
			COALESCE(p.weight_kg,0), COALESCE(p.length_cm,0),
			COALESCE(p.width_cm,0), COALESCE(p.height_cm,0),
			p.constraint_type, COALESCE(p.min_vehicle_type,''),
			p.images, p.is_active, p.created_at, p.updated_at, COALESCE(p.updated_by::text,''),
			COALESCE(si.price_kes, 0),
			COALESCE(si.stock_quantity, 0),
			COALESCE(si.low_stock_alert, 10),
			COALESCE(si.is_available, false)
		FROM products p
		LEFT JOIN store_inventory si
			ON si.product_id = p.id AND si.store_id = $1
		WHERE p.is_active = TRUE
		ORDER BY p.category, p.name
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectWithInventory(rows)
}

// GetWithInventory returns a single product with its store-specific price and stock.
func (r *Repository) GetWithInventory(ctx context.Context, productID, storeID string) (*ProductWithInventory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			p.id, p.name, COALESCE(p.description,''), COALESCE(p.category,''),
			COALESCE(p.weight_kg,0), COALESCE(p.length_cm,0),
			COALESCE(p.width_cm,0), COALESCE(p.height_cm,0),
			p.constraint_type, COALESCE(p.min_vehicle_type,''),
			p.images, p.is_active, p.created_at, p.updated_at, COALESCE(p.updated_by::text,''),
			COALESCE(si.price_kes, 0),
			COALESCE(si.stock_quantity, 0),
			COALESCE(si.low_stock_alert, 10),
			COALESCE(si.is_available, false)
		FROM products p
		LEFT JOIN store_inventory si
			ON si.product_id = p.id AND si.store_id = $2
		WHERE p.id = $1
	`, productID, storeID)

	var p ProductWithInventory
	err := scanProductWithInventory(row, &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

// GetAllStorePrices returns the price and stock for a product across every
// active store — used on the product detail page to show price comparisons.
func (r *Repository) GetAllStorePrices(ctx context.Context, productID string) ([]StorePriceEntry, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			s.id, s.name, COALESCE(s.county,''),
			COALESCE(si.price_kes, 0),
			COALESCE(si.stock_quantity > 0 AND si.is_available, false)
		FROM stores s
		LEFT JOIN store_inventory si
			ON si.store_id = s.id AND si.product_id = $1
		WHERE s.is_active = TRUE
		ORDER BY si.price_kes ASC NULLS LAST
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []StorePriceEntry
	for rows.Next() {
		var e StorePriceEntry
		if err := rows.Scan(&e.StoreID, &e.StoreName, &e.County, &e.Price, &e.InStock); err != nil {
			return nil, fmt.Errorf("products: scan store price error: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func scanProduct(row pgx.Row) (*Product, error) {
	var p Product
	var images pgtype.Array[string]
	if err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Category,
		&p.WeightKg, &p.LengthCm, &p.WidthCm, &p.HeightCm,
		&p.ConstraintType, &p.MinVehicleType,
		&images, &p.IsActive, &p.CreatedAt, &p.UpdatedAt, &p.UpdatedBy,
	); err != nil {
		return nil, err
	}
	p.Images = images.Elements
	return &p, nil
}

func scanProductWithInventory(row pgx.Row, p *ProductWithInventory) error {
	var images pgtype.Array[string]
	return row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Category,
		&p.WeightKg, &p.LengthCm, &p.WidthCm, &p.HeightCm,
		&p.ConstraintType, &p.MinVehicleType,
		&images, &p.IsActive, &p.CreatedAt, &p.UpdatedAt, &p.UpdatedBy,
		&p.Price, &p.StockQuantity, &p.LowStockAlert, &p.IsAvailable,
	)
}

func collectWithInventory(rows pgx.Rows) ([]ProductWithInventory, error) {
	var result []ProductWithInventory
	for rows.Next() {
		var p ProductWithInventory
		if err := scanProductWithInventory(rows, &p); err != nil {
			return nil, fmt.Errorf("products: scan error: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(f float64) interface{} {
	if f == 0 {
		return nil
	}
	return f
}