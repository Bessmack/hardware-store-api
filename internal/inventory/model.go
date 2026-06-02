package inventory

import "time"

// ── Core models ───────────────────────────────────────────────────────────────

// StoreInventory is a single row from the store_inventory table.
type StoreInventory struct {
	ID             string    `db:"id"`
	StoreID        string    `db:"store_id"`
	ProductID      string    `db:"product_id"`
	Price          float64   `db:"price"`
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
	OldPrice     float64   `db:"old_price"`
	NewPrice     float64   `db:"new_price"`
	ChangedBy    string    `db:"changed_by"`
	ChangedAt    time.Time `db:"changed_at"`
	Reason       string    `db:"reason"`
}

// ── Request types ─────────────────────────────────────────────────────────────

// UpsertRequest creates or replaces a store's inventory entry for a product.
// Used when adding a product to a store for the first time or doing a full reset.
type UpsertRequest struct {
	ProductID     string  `json:"product_id"     validate:"required"`
	Price         float64 `json:"price"      validate:"required,gt=0"`
	StockQuantity int     `json:"stock_quantity" validate:"gte=0"`
	LowStockAlert int     `json:"low_stock_alert"`
	IsAvailable   bool    `json:"is_available"`
}

// UpdatePriceRequest changes a product's price at the store.
// A reason is required for the audit trail.
type UpdatePriceRequest struct {
	Price    float64 `json:"price" validate:"required,gt=0"`
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
	Price         float64   `json:"price"`
	Currency      string    `json:"currency"`
	StockQuantity int       `json:"stock_quantity"`
	LowStockAlert int       `json:"low_stock_alert"`
	IsAvailable   bool      `json:"is_available"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PriceHistoryResponse is returned when viewing audit trail for a product's price.
type PriceHistoryResponse struct {
	OldPrice    float64   `json:"old_price"`
	NewPrice    float64   `json:"new_price"`
	ChangedBy   string    `json:"changed_by"`
	ChangedAt   time.Time `json:"changed_at"`
	Reason      string    `json:"reason,omitempty"`
}