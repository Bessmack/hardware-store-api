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
			(name, description, category, subcategory_id, weight_kg, length_cm, width_cm, height_cm,
			 constraint_type, min_vehicle_type, images, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, name, COALESCE(description,''), COALESCE(category,''),
		          COALESCE(subcategory_id::text,''),
		          COALESCE(weight_kg,0), COALESCE(length_cm,0), COALESCE(width_cm,0), COALESCE(height_cm,0),
		          constraint_type, COALESCE(min_vehicle_type,''),
		          images, is_active, created_at, updated_at, COALESCE(updated_by::text,'')
	`,
		req.Name, nullIfEmpty(req.Description), nullIfEmpty(req.Name),
		nullIfEmpty(req.SubcategoryID),
		nullIfZero(req.WeightKg), nullIfZero(req.LengthCm), nullIfZero(req.WidthCm), nullIfZero(req.HeightCm),
		req.ConstraintType, nullIfEmpty(string(req.MinVehicleType)),
		req.Images, updatedBy,
	)
	return scanProduct(row)
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Product, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(description,''), COALESCE(category,''),
		       COALESCE(subcategory_id::text,''),
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
			name             = COALESCE(NULLIF($1,''),  name),
			description      = COALESCE(NULLIF($2,''),  description),
			subcategory_id   = COALESCE(NULLIF($3,'')::uuid, subcategory_id),
			weight_kg        = CASE WHEN $4 > 0 THEN $4 ELSE weight_kg END,
			length_cm        = CASE WHEN $5 > 0 THEN $5 ELSE length_cm END,
			width_cm         = CASE WHEN $6 > 0 THEN $6 ELSE width_cm END,
			height_cm        = CASE WHEN $7 > 0 THEN $7 ELSE height_cm END,
			constraint_type  = COALESCE(NULLIF($8,''),  constraint_type::text)::constraint_type_enum,
			min_vehicle_type = COALESCE(NULLIF($9,''),  min_vehicle_type::text)::vehicle_type_enum,
			updated_by       = $10
		WHERE id = $11
		RETURNING id, name, COALESCE(description,''), COALESCE(category,''),
		          COALESCE(subcategory_id::text,''),
		          COALESCE(weight_kg,0), COALESCE(length_cm,0), COALESCE(width_cm,0), COALESCE(height_cm,0),
		          constraint_type, COALESCE(min_vehicle_type,''),
		          images, is_active, created_at, updated_at, COALESCE(updated_by::text,'')
	`,
		req.Name, req.Description, req.SubcategoryID,
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

// ── Per-store joined queries ───────────────────────────────────────────────────
// Used by staff and customer endpoints scoped to a specific store.

func (r *Repository) ListWithInventory(ctx context.Context, storeID string) ([]ProductWithInventory, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			p.id, p.name, COALESCE(p.description,''), COALESCE(p.category,''),
			COALESCE(p.subcategory_id::text,''),
			COALESCE(p.weight_kg,0), COALESCE(p.length_cm,0),
			COALESCE(p.width_cm,0), COALESCE(p.height_cm,0),
			p.constraint_type, COALESCE(p.min_vehicle_type,''),
			p.images, p.is_active, p.created_at, p.updated_at, COALESCE(p.updated_by::text,''),
			COALESCE(si.price, 0),
			COALESCE(s.currency, 'KES'),
			COALESCE(si.stock_quantity, 0),
			COALESCE(si.low_stock_alert, 10),
			COALESCE(si.is_available, false)
		FROM products p
		LEFT JOIN store_inventory si ON si.product_id = p.id AND si.store_id = $1
		LEFT JOIN stores s ON s.id = $1
		WHERE p.is_active = TRUE
		ORDER BY p.name
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectWithInventory(rows)
}

func (r *Repository) GetWithInventory(ctx context.Context, productID, storeID string) (*ProductWithInventory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			p.id, p.name, COALESCE(p.description,''), COALESCE(p.category,''),
			COALESCE(p.subcategory_id::text,''),
			COALESCE(p.weight_kg,0), COALESCE(p.length_cm,0),
			COALESCE(p.width_cm,0), COALESCE(p.height_cm,0),
			p.constraint_type, COALESCE(p.min_vehicle_type,''),
			p.images, p.is_active, p.created_at, p.updated_at, COALESCE(p.updated_by::text,''),
			COALESCE(si.price, 0),
			COALESCE(s.currency, 'KES'),
			COALESCE(si.stock_quantity, 0),
			COALESCE(si.low_stock_alert, 10),
			COALESCE(si.is_available, false)
		FROM products p
		LEFT JOIN store_inventory si ON si.product_id = p.id AND si.store_id = $2
		LEFT JOIN stores s ON s.id = $2
		WHERE p.id = $1
	`, productID, storeID)

	var p ProductWithInventory
	if err := scanProductWithInventory(row, &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// ── Global public queries ─────────────────────────────────────────────────────
// Used by the storefront product catalogue — no store required.
// Price shown is the minimum across all active stores.
// Full-text search via PostgreSQL tsvector; exact-match ILIKE as fallback.

// ListAll returns the global product catalogue with optional filtering and
// full-text search. Results are relevance-ranked when q is non-empty.
func (r *Repository) ListAll(ctx context.Context, f ListFilter) ([]ProductWithInventory, error) {
	offset := (f.Page - 1) * f.Limit
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			p.id, p.name, COALESCE(p.description,''), COALESCE(p.category,''),
			COALESCE(p.subcategory_id::text,''),
			COALESCE(p.weight_kg,0), COALESCE(p.length_cm,0),
			COALESCE(p.width_cm,0), COALESCE(p.height_cm,0),
			p.constraint_type, COALESCE(p.min_vehicle_type,''),
			p.images, p.is_active, p.created_at, p.updated_at, COALESCE(p.updated_by::text,''),
			COALESCE(MIN(si.price), 0)                                         AS price,
			COALESCE(MAX(s.currency), 'KES')                                   AS currency,
			COALESCE(MAX(si.stock_quantity), 0)::int                           AS stock_quantity,
			COALESCE(MAX(si.low_stock_alert), 10)::int                         AS low_stock_alert,
			BOOL_OR(COALESCE(si.is_available, false)
			        AND COALESCE(si.stock_quantity, 0) > 0)                    AS is_available,
			COALESCE(MAX(sc.name), '')                                         AS subcategory_name,
			COALESCE(MAX(c.name),  '')                                         AS category_name,
			COALESCE(MAX(c.slug),  '')                                         AS category_slug
		FROM products p
		LEFT JOIN subcategories sc ON sc.id = p.subcategory_id
		LEFT JOIN categories    c  ON c.id  = sc.category_id
		LEFT JOIN store_inventory si ON si.product_id = p.id
		LEFT JOIN stores s ON s.id = si.store_id AND s.is_active = true
		WHERE p.is_active = true
		  AND ($1::text = '' OR c.slug              = $1)
		  AND ($2::text = '' OR p.subcategory_id::text = $2)
		  AND ($3::text = '' OR (
		        to_tsvector('english',
		            p.name || ' ' ||
		            COALESCE(p.description,'') || ' ' ||
		            COALESCE(sc.name,'') || ' ' ||
		            COALESCE(c.name,'')
		        ) @@ plainto_tsquery('english', $3)
		        OR p.name ILIKE '%' || $3 || '%'
		  ))
		GROUP BY p.id
		ORDER BY
		  CASE WHEN $3 <> '' THEN
		    ts_rank(
		      to_tsvector('english', p.name || ' ' || COALESCE(p.description,'')),
		      plainto_tsquery('english', $3)
		    )
		  ELSE 0 END DESC,
		  p.name ASC
		LIMIT $4 OFFSET $5
	`, f.CategorySlug, f.SubcategoryID, f.Query, f.Limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGlobal(rows)
}

// CountAll returns the total number of products matching the same filter as
// ListAll — used to build pagination metadata.
func (r *Repository) CountAll(ctx context.Context, f ListFilter) (int, error) {
	var total int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT p.id)
		FROM products p
		LEFT JOIN subcategories sc ON sc.id = p.subcategory_id
		LEFT JOIN categories    c  ON c.id  = sc.category_id
		WHERE p.is_active = true
		  AND ($1::text = '' OR c.slug                 = $1)
		  AND ($2::text = '' OR p.subcategory_id::text = $2)
		  AND ($3::text = '' OR (
		        to_tsvector('english',
		            p.name || ' ' || COALESCE(p.description,'') || ' ' ||
		            COALESCE(sc.name,'') || ' ' || COALESCE(c.name,'')
		        ) @@ plainto_tsquery('english', $3)
		        OR p.name ILIKE '%' || $3 || '%'
		  ))
	`, f.CategorySlug, f.SubcategoryID, f.Query).Scan(&total)
	return total, err
}

// GetByIDGlobal returns a single product with category denorm data.
// Prices across all stores are fetched separately via GetAllStorePrices.
func (r *Repository) GetByIDGlobal(ctx context.Context, id string) (*ProductWithInventory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			p.id, p.name, COALESCE(p.description,''), COALESCE(p.category,''),
			COALESCE(p.subcategory_id::text,''),
			COALESCE(p.weight_kg,0), COALESCE(p.length_cm,0),
			COALESCE(p.width_cm,0), COALESCE(p.height_cm,0),
			p.constraint_type, COALESCE(p.min_vehicle_type,''),
			p.images, p.is_active, p.created_at, p.updated_at, COALESCE(p.updated_by::text,''),
			0::numeric, 'KES', 0::int, 10::int, false,
			COALESCE(sc.name,''), COALESCE(c.name,''), COALESCE(c.slug,'')
		FROM products p
		LEFT JOIN subcategories sc ON sc.id = p.subcategory_id
		LEFT JOIN categories    c  ON c.id  = sc.category_id
		WHERE p.id = $1 AND p.is_active = true
	`, id)

	var p ProductWithInventory
	if err := scanProductGlobal(row, &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// GetAllStorePrices returns the price and stock for a product across every
// active store — used on the product detail page for cross-store comparison.
func (r *Repository) GetAllStorePrices(ctx context.Context, productID string) ([]StorePriceEntry, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			s.id,
			s.name,
			COALESCE(s.county,''),
			COALESCE(s.currency, 'KES'),
			COALESCE(si.price, 0),
			COALESCE(si.is_available AND si.stock_quantity > 0, false)
		FROM stores s
		LEFT JOIN store_inventory si
			ON si.store_id = s.id AND si.product_id = $1
		WHERE s.is_active = TRUE
		ORDER BY si.price ASC NULLS LAST, s.name ASC
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []StorePriceEntry
	for rows.Next() {
		var e StorePriceEntry
		if err := rows.Scan(&e.StoreID, &e.StoreName, &e.County, &e.Currency, &e.Price, &e.InStock); err != nil {
			return nil, fmt.Errorf("products: scan store price: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// ── Scan helpers ──────────────────────────────────────────────────────────────

// scanProduct scans 16 columns — base product fields including subcategory_id.
func scanProduct(row pgx.Row) (*Product, error) {
	var p Product
	var images pgtype.Array[string]
	if err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Category,
		&p.SubcategoryID,
		&p.WeightKg, &p.LengthCm, &p.WidthCm, &p.HeightCm,
		&p.ConstraintType, &p.MinVehicleType,
		&images, &p.IsActive, &p.CreatedAt, &p.UpdatedAt, &p.UpdatedBy,
	); err != nil {
		return nil, err
	}
	p.Images = images.Elements
	return &p, nil
}

// scanProductWithInventory scans 21 columns — base product (16) + store inventory (5).
// Used by per-store queries where category denorm is not needed.
func scanProductWithInventory(row pgx.Row, p *ProductWithInventory) error {
	var images pgtype.Array[string]
	if err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Category,
		&p.SubcategoryID,
		&p.WeightKg, &p.LengthCm, &p.WidthCm, &p.HeightCm,
		&p.ConstraintType, &p.MinVehicleType,
		&images, &p.IsActive, &p.CreatedAt, &p.UpdatedAt, &p.UpdatedBy,
		&p.Price, &p.Currency, &p.StockQuantity, &p.LowStockAlert, &p.IsAvailable,
	); err != nil {
		return err
	}
	p.Images = images.Elements
	return nil
}

// scanProductGlobal scans 24 columns — base product (16) + inventory (5) + category denorm (3).
// Used by global listing and global detail queries.
func scanProductGlobal(row pgx.Row, p *ProductWithInventory) error {
	var images pgtype.Array[string]
	if err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Category,
		&p.SubcategoryID,
		&p.WeightKg, &p.LengthCm, &p.WidthCm, &p.HeightCm,
		&p.ConstraintType, &p.MinVehicleType,
		&images, &p.IsActive, &p.CreatedAt, &p.UpdatedAt, &p.UpdatedBy,
		&p.Price, &p.Currency, &p.StockQuantity, &p.LowStockAlert, &p.IsAvailable,
		&p.SubcategoryName, &p.CategoryName, &p.CategorySlug,
	); err != nil {
		return err
	}
	p.Images = images.Elements
	return nil
}

func collectWithInventory(rows pgx.Rows) ([]ProductWithInventory, error) {
	var result []ProductWithInventory
	for rows.Next() {
		var p ProductWithInventory
		if err := scanProductWithInventory(rows, &p); err != nil {
			return nil, fmt.Errorf("products: scan: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func collectGlobal(rows pgx.Rows) ([]ProductWithInventory, error) {
	var result []ProductWithInventory
	for rows.Next() {
		var p ProductWithInventory
		if err := scanProductGlobal(rows, &p); err != nil {
			return nil, fmt.Errorf("products: global scan: %w", err)
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