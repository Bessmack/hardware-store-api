package inventory

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("inventory entry not found")

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// Upsert creates or fully replaces a store's inventory entry for a product.
func (r *Repository) Upsert(ctx context.Context, storeID string, req UpsertRequest, updatedBy string) (*StoreInventory, error) {
	lowStockAlert := req.LowStockAlert
	if lowStockAlert == 0 {
		lowStockAlert = 10 // sensible default
	}

	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO store_inventory
			(store_id, product_id, price, stock_quantity, low_stock_alert, is_available, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (store_id, product_id) DO UPDATE SET
			price      = EXCLUDED.price,
			stock_quantity = EXCLUDED.stock_quantity,
			low_stock_alert= EXCLUDED.low_stock_alert,
			is_available   = EXCLUDED.is_available,
			updated_by     = EXCLUDED.updated_by
		RETURNING id, store_id, product_id, price, stock_quantity,
		          low_stock_alert, is_available, updated_at, COALESCE(updated_by::text,'')
	`, storeID, req.ProductID, req.Price, req.StockQuantity,
		lowStockAlert, req.IsAvailable, updatedBy)

	return scanInventory(row)
}

// GetByStoreAndProduct returns the inventory entry for a specific product at a store.
func (r *Repository) GetByStoreAndProduct(ctx context.Context, storeID, productID string) (*StoreInventory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, store_id, product_id, price, stock_quantity,
		       low_stock_alert, is_available, updated_at, COALESCE(updated_by::text,'')
		FROM store_inventory
		WHERE store_id = $1 AND product_id = $2
	`, storeID, productID)

	inv, err := scanInventory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return inv, err
}

// UpdatePrice changes the price for a product at a store.
// The price change trigger in the DB logs this to inventory_price_history automatically.
func (r *Repository) UpdatePrice(ctx context.Context, storeID, productID string, req UpdatePriceRequest, updatedBy string) (*StoreInventory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE store_inventory
		SET price  = $1,
		    updated_by = $2
		WHERE store_id = $3 AND product_id = $4
		RETURNING id, store_id, product_id, price, stock_quantity,
		          low_stock_alert, is_available, updated_at, COALESCE(updated_by::text,'')
	`, req.Price, updatedBy, storeID, productID)

	inv, err := scanInventory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return inv, err
}

// UpdateStock adjusts stock quantity and availability for a product at a store.
func (r *Repository) UpdateStock(ctx context.Context, storeID, productID string, req UpdateStockRequest, updatedBy string) (*StoreInventory, error) {
	lowStockAlert := req.LowStockAlert
	if lowStockAlert == 0 {
		lowStockAlert = 10
	}

	row := r.db.Pool.QueryRow(ctx, `
		UPDATE store_inventory
		SET stock_quantity  = $1,
		    is_available    = $2,
		    low_stock_alert = $3,
		    updated_by      = $4
		WHERE store_id = $5 AND product_id = $6
		RETURNING id, store_id, product_id, price, stock_quantity,
		          low_stock_alert, is_available, updated_at, COALESCE(updated_by::text,'')
	`, req.StockQuantity, req.IsAvailable, lowStockAlert, updatedBy, storeID, productID)

	inv, err := scanInventory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return inv, err
}

// ListByStore returns all inventory entries for a store with product names.
func (r *Repository) ListByStore(ctx context.Context, storeID string) ([]InventoryResponse, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT si.product_id, p.name,
		       si.price, si.stock_quantity, si.low_stock_alert,
		       si.is_available, si.updated_at
		FROM store_inventory si
		JOIN products p ON p.id = si.product_id
		WHERE si.store_id = $1
		ORDER BY p.category, p.name
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []InventoryResponse
	for rows.Next() {
		var inv InventoryResponse
		if err := rows.Scan(
			&inv.ProductID, &inv.ProductName,
			&inv.Price, &inv.StockQuantity, &inv.LowStockAlert,
			&inv.IsAvailable, &inv.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("inventory: scan error: %w", err)
		}
		result = append(result, inv)
	}
	return result, rows.Err()
}

// GetPriceHistory returns the price change audit trail for a product at a store.
func (r *Repository) GetPriceHistory(ctx context.Context, storeID, productID string) ([]PriceHistoryResponse, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			COALESCE(old_price, 0), new_price,
			COALESCE(changed_by::text,''), changed_at,
			COALESCE(reason,'')
		FROM inventory_price_history
		WHERE store_id = $1 AND product_id = $2
		ORDER BY changed_at DESC
		LIMIT 50
	`, storeID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PriceHistoryResponse
	for rows.Next() {
		var h PriceHistoryResponse
		if err := rows.Scan(
			&h.OldPrice, &h.NewPrice,
			&h.ChangedBy, &h.ChangedAt, &h.Reason,
		); err != nil {
			return nil, fmt.Errorf("inventory: price history scan error: %w", err)
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

// ── Helper ────────────────────────────────────────────────────────────────────

func scanInventory(row pgx.Row) (*StoreInventory, error) {
	var inv StoreInventory
	if err := row.Scan(
		&inv.ID, &inv.StoreID, &inv.ProductID,
		&inv.Price, &inv.StockQuantity, &inv.LowStockAlert,
		&inv.IsAvailable, &inv.UpdatedAt, &inv.UpdatedBy,
	); err != nil {
		return nil, err
	}
	return &inv, nil
}

// ── cart.InventoryReader implementation ──────────────────────────────────────

// GetCurrentPrice satisfies the cart.InventoryReader interface.
// Returns the current price, currency, and stock availability for a product
// at a specific store. Called when adding an item to the cart to lock the price.
func (r *Repository) GetCurrentPrice(ctx context.Context, productID, storeID string) (price float64, currency string, inStock bool, err error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT si.price, COALESCE(s.currency, 'KES'),
		       si.stock_quantity > 0 AND si.is_available
		FROM store_inventory si
		JOIN stores s ON s.id = si.store_id
		WHERE si.product_id = $1 AND si.store_id = $2
	`, productID, storeID)

	if err := row.Scan(&price, &currency, &inStock); err != nil {
		return 0, "", false, fmt.Errorf("inventory: product not found at this store: %w", err)
	}
	return price, currency, inStock, nil
}

// ── wishlist.LivePriceFetcher implementation ──────────────────────────────────

// GetLivePrice satisfies the wishlist.LivePriceFetcher interface.
// Returns current price, currency, in-stock flag, and limited-availability flag.
// limited is true when stock_quantity > 0 but <= low_stock_alert (low stock nudge).
// Stock quantity itself is never returned — customers only see the boolean flags.
func (r *Repository) GetLivePrice(ctx context.Context, productID, storeID string) (price float64, currency string, inStock bool, limited bool, err error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			si.price,
			COALESCE(s.currency, 'KES'),
			si.stock_quantity > 0 AND si.is_available,
			si.stock_quantity > 0 AND si.stock_quantity <= si.low_stock_alert AND si.is_available
		FROM store_inventory si
		JOIN stores s ON s.id = si.store_id
		WHERE si.product_id = $1 AND si.store_id = $2
	`, productID, storeID)

	if scanErr := row.Scan(&price, &currency, &inStock, &limited); scanErr != nil {
		return 0, "", false, false, fmt.Errorf("inventory: product not stocked at this store: %w", scanErr)
	}
	return price, currency, inStock, limited, nil
}