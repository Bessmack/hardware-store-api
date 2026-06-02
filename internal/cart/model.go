package cart

import (
	"fmt"
	"time"
)

// ── Vehicle determination ─────────────────────────────────────────────────────

// vehicleHierarchy ranks vehicle types for comparison.
var vehicleHierarchy = map[string]int{"bike": 1, "van": 2, "truck": 3}

// VehicleResult holds the required vehicle and the human-readable reason.
type VehicleResult struct {
	VehicleType string // "bike" | "van" | "truck"
	Reason      string // shown to customer if they cannot choose a smaller option
}

// CartItemForVehicle holds the product data needed for vehicle determination.
// Populated by joining cart_items with products in the repository.
type CartItemForVehicle struct {
	ProductName    string
	ConstraintType string  // "weight" | "dimension" | "hazardous"
	MinVehicleType string  // only set for dimension/hazardous items
	WeightKg       float64
	Quantity       int
}

// DetermineVehicle analyses a list of cart items and returns the minimum
// vehicle type required to fulfil the delivery.
//
// Rules:
//   - Dimension/hazardous items hard-set the vehicle regardless of quantity
//   - Weight-based items are governed by total order weight
//   - The highest requirement across all items wins
func DetermineVehicle(items []CartItemForVehicle, weightThresholds WeightThresholds) VehicleResult {
	required := "bike"
	reason := ""

	// Step 1 — check hard physical/hazardous constraints first
	for _, item := range items {
		if item.ConstraintType == "dimension" || item.ConstraintType == "hazardous" {
			minV := item.MinVehicleType
			if vehicleHierarchy[minV] > vehicleHierarchy[required] {
				required = minV
				reason = fmt.Sprintf(
					"Your order contains %s, which requires a %s due to its size",
					item.ProductName, minV,
				)
			}
		}
	}

	// Step 2 — total weight across all weight-constrained items
	totalWeight := 0.0
	for _, item := range items {
		if item.ConstraintType == "weight" {
			totalWeight += item.WeightKg * float64(item.Quantity)
		}
	}

	weightVehicle := vehicleForWeight(totalWeight, weightThresholds)
	if vehicleHierarchy[weightVehicle] > vehicleHierarchy[required] {
		required = weightVehicle
		reason = fmt.Sprintf(
			"Your total order weight is %.1fkg, which requires a %s",
			totalWeight, required,
		)
	}

	if reason == "" {
		reason = fmt.Sprintf("Standard %s delivery", required)
	}

	return VehicleResult{VehicleType: required, Reason: reason}
}

// WeightThresholds holds the max weight per vehicle, loaded from delivery_rates.
type WeightThresholds struct {
	BikeMaxKg float64 // default 30
	VanMaxKg  float64 // default 500
	// Anything above VanMaxKg requires truck
}

func vehicleForWeight(kg float64, t WeightThresholds) string {
	switch {
	case kg <= t.BikeMaxKg:
		return "bike"
	case kg <= t.VanMaxKg:
		return "van"
	default:
		return "truck"
	}
}

// ── Core models ───────────────────────────────────────────────────────────────

type Cart struct {
	ID             string    `db:"id"`
	CustomerID     string    `db:"customer_id"`      // empty for guests
	GuestSessionID string    `db:"guest_session_id"` // empty for registered users
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type CartItem struct {
	ID        string    `db:"id"`
	CartID    string    `db:"cart_id"`
	ProductID string    `db:"product_id"`
	StoreID   string    `db:"store_id"`
	Quantity  int       `db:"quantity"`
	UnitPrice float64   `db:"unit_price"` // locked at add-to-cart time
	Currency  string    `db:"currency"`   // locked at add-to-cart time
	AddedAt   time.Time `db:"added_at"`
}

// ── Request types ─────────────────────────────────────────────────────────────

// AddItemRequest adds a product to the cart.
// StoreID is required — the cart must know which store's price to lock.
// If empty, the caller should resolve it from the customer's cached location.
type AddItemRequest struct {
	ProductID string `json:"product_id" validate:"required"`
	StoreID   string `json:"store_id"   validate:"required"`
	Quantity  int    `json:"quantity"   validate:"required,gt=0"`
}

// UpdateQuantityRequest changes the quantity of an existing cart item.
type UpdateQuantityRequest struct {
	Quantity int `json:"quantity" validate:"required,gt=0"`
}

// ── Response types ────────────────────────────────────────────────────────────

// CartItemResponse is what the customer sees for each item.
// StockQuantity is intentionally absent — customers only see InStock.
type CartItemResponse struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Image       string  `json:"image,omitempty"` // first image only
	StoreID     string  `json:"store_id"`
	StoreName   string  `json:"store_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Subtotal    float64 `json:"subtotal"`
	Currency    string  `json:"currency"`
	InStock     bool    `json:"in_stock"`          // current stock check
	PriceChanged bool   `json:"price_changed"`     // true if price changed since add
	CurrentPrice float64 `json:"current_price"`   // actual current price
}

// CartResponse is the full cart with summary and vehicle determination.
type CartResponse struct {
	ID              string             `json:"id"`
	Items           []CartItemResponse `json:"items"`
	ItemCount       int                `json:"item_count"`
	Subtotal        float64            `json:"subtotal"`
	Currency        string             `json:"currency,omitempty"` // set if all items share one currency
	Warnings        []string           `json:"warnings,omitempty"` // price changes, stock alerts
	RequiredVehicle string             `json:"required_vehicle"`   // "bike" | "van" | "truck"
	VehicleReason   string             `json:"vehicle_reason"`
}

// ValidationResult is returned by ValidateCart before checkout.
type ValidationResult struct {
	IsValid         bool               `json:"is_valid"`
	Errors          []string           `json:"errors,omitempty"`   // blocks checkout
	Warnings        []string           `json:"warnings,omitempty"` // informational
	UpdatedItems    []CartItemResponse `json:"updated_items,omitempty"`
	RequiredVehicle string             `json:"required_vehicle"`
	VehicleReason   string             `json:"vehicle_reason"`
}