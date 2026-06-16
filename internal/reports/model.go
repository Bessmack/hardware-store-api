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