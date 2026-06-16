package reports

import "time"

// ── Store report ──────────────────────────────────────────────────────────────

// StoreReport is the full dashboard report for a single store.
// Available to admin and superadmin of that store.
type StoreReport struct {
	StoreID   string    `json:"store_id"`
	StoreName string    `json:"store_name"`
	Currency  string    `json:"currency"`
	Period    Period    `json:"period"`

	// Revenue
	TotalRevenue    float64 `json:"total_revenue"`
	AverageOrderValue float64 `json:"average_order_value"`

	// Orders
	TotalOrders     int `json:"total_orders"`
	OrdersByStatus  map[string]int `json:"orders_by_status"`

	// Delivery
	DeliveryOrders  int     `json:"delivery_orders"`
	PickupOrders    int     `json:"pickup_orders"`
	AverageDeliveryFee float64 `json:"average_delivery_fee"`

	// Payments
	PaymentsByProvider map[string]int     `json:"payments_by_provider"`
	RevenueByProvider  map[string]float64 `json:"revenue_by_provider"`

	// Inventory
	LowStockItems   []LowStockItem `json:"low_stock_items"`
	OutOfStockItems []LowStockItem `json:"out_of_stock_items"`

	// Top products
	TopProducts []ProductSalesSummary `json:"top_products"`

	// POD & disputes
	TotalDeliveries  int `json:"total_deliveries"`
	TotalDisputes    int `json:"total_disputes"`
	OpenDisputes     int `json:"open_disputes"`

	GeneratedAt time.Time `json:"generated_at"`
}

// ── Global report (superadmin only) ──────────────────────────────────────────

// GlobalReport aggregates key metrics across all stores.
// Superadmin only.
type GlobalReport struct {
	Period  Period `json:"period"`

	// Platform-wide totals
	TotalStores    int     `json:"total_stores"`
	ActiveStores   int     `json:"active_stores"`
	TotalOrders    int     `json:"total_orders"`
	TotalRevenue   float64 `json:"total_revenue"` // note: mixed currencies, use with caution
	TotalCustomers int     `json:"total_customers"`
	NewCustomers   int     `json:"new_customers"` // registered within the period

	// Per-store breakdown
	StoreBreakdowns []StoreBreakdown `json:"store_breakdown"`

	// Platform-wide payment split
	PaymentsByProvider map[string]int `json:"payments_by_provider"`

	// Disputes
	TotalDisputes int `json:"total_disputes"`
	OpenDisputes  int `json:"open_disputes"`

	GeneratedAt time.Time `json:"generated_at"`
}

// ── Supporting types ──────────────────────────────────────────────────────────

// Period is the date range for a report.
type Period struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// LowStockItem is a product that is below its low_stock_alert threshold.
type LowStockItem struct {
	ProductID     string `json:"product_id"`
	ProductName   string `json:"product_name"`
	StockQuantity int    `json:"stock_quantity"`
	LowStockAlert int    `json:"low_stock_alert"`
}

// ProductSalesSummary shows how a product performed in the period.
type ProductSalesSummary struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	UnitsSold   int     `json:"units_sold"`
	Revenue     float64 `json:"revenue"`
	Currency    string  `json:"currency"`
}

// StoreBreakdown is one store's contribution to the global report.
type StoreBreakdown struct {
	StoreID     string  `json:"store_id"`
	StoreName   string  `json:"store_name"`
	County      string  `json:"county"`
	Currency    string  `json:"currency"`
	TotalOrders int     `json:"total_orders"`
	Revenue     float64 `json:"revenue"`
}

// ── Query filters ─────────────────────────────────────────────────────────────

// ReportFilter controls the period for any report.
// Both fields default to the current calendar month if not provided.
type ReportFilter struct {
	From time.Time
	To   time.Time
}

// DefaultFilter returns a filter for the current calendar month.
func DefaultFilter() ReportFilter {
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	to := now
	return ReportFilter{From: from, To: to}
}