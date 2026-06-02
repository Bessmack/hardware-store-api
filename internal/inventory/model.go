package inventory

import "time"

// ── Core models ───────────────────────────────────────────────────────────────

// StoreInventory is a single row from the store_inventory table.
type StoreInventory struct {
	ID             string    `db:"id"`
	StoreID        string    `db:"store_id"`
	ProductID      string    `db:"product_id"`
	PriceKES       float64   `db:"price_kes"`
	StockQuantity  int       `db:"stock_quantity"`
	LowStockAlert  int       `db:"low_stock_alert"`
	IsAvailable    bool      `db:"is_available"`
	UpdatedAt      time.Time `db:"updated_at"`
	UpdatedBy      string    `db:"updated_by"`
}

// PriceHistory is one row from inventory_price_history.
type PriceHistory struct {
	ID           string    `db:"id"`
	StoreID      string    `db:"store_id"`
	ProductID    string    `db:"product_id"`
	OldPriceKES  float64   `db:"old_price_kes"`
	NewPriceKES  float64   `db:"new_price_kes"`
	ChangedBy    string    `db:"changed_by"`
	ChangedAt    time.Time `db:"changed_at"`
	Reason       string    `db:"reason"`
}

// ── Request types ─────────────────────────────────────────────────────────────

// UpsertRequest creates or replaces a store's inventory entry for a product.
// Used when adding a product to a store for the first time or doing a full reset.
type UpsertRequest struct {
	ProductID     string  `json:"product_id"     validate:"required"`
	PriceKES      float64 `json:"price_kes"      validate:"required,gt=0"`
	StockQuantity int     `json:"stock_quantity" validate:"gte=0"`
	LowStockAlert int     `json:"low_stock_alert"`
	IsAvailable   bool    `json:"is_available"`
}

// UpdatePriceRequest changes a product's price at the store.
// A reason is required for the audit trail.
type UpdatePriceRequest struct {
	PriceKES float64 `json:"price_kes" validate:"required,gt=0"`
	Reason   string  `json:"reason"`
}

// UpdateStockRequest adjusts stock quantity and availability.
type UpdateStockRequest struct {
	StockQuantity int  `json:"stock_quantity" validate:"gte=0"`
	IsAvailable   bool `json:"is_available"`
	LowStockAlert int  `json:"low_stock_alert"`
}

// ── Response types ────────────────────────────────────────────────────────────

// InventoryResponse is what staff see for a single inventory entry.
type InventoryResponse struct {
	ProductID     string    `json:"product_id"`
	ProductName   string    `json:"product_name"`
	PriceKES      float64   `json:"price_kes"`
	StockQuantity int       `json:"stock_quantity"`
	LowStockAlert int       `json:"low_stock_alert"`
	IsAvailable   bool      `json:"is_available"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PriceHistoryResponse is returned when viewing audit trail for a product's price.
type PriceHistoryResponse struct {
	OldPriceKES float64   `json:"old_price_kes"`
	NewPriceKES float64   `json:"new_price_kes"`
	ChangedBy   string    `json:"changed_by"`
	ChangedAt   time.Time `json:"changed_at"`
	Reason      string    `json:"reason,omitempty"`
}