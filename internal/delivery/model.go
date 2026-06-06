package delivery

import (
	"fmt"
	"time"
)

// ── Core model ────────────────────────────────────────────────────────────────

// DeliveryRate is one row from the delivery_rates table.
// store_id = "" means this is a global default (applies to all stores that
// have not set their own rate for this vehicle type).
type DeliveryRate struct {
	StoreID     string    `db:"store_id"`
	VehicleType string    `db:"vehicle_type"` // "bike" | "van" | "truck"
	BaseFee     float64   `db:"base_fee"`
	PerKm       float64   `db:"per_km"`
	MaxWeightKg float64   `db:"max_weight_kg"` // 0 = no limit (truck)
	MaxRadiusKm float64   `db:"max_radius_km"` // 0 = no limit (truck)
	UpdatedAt   time.Time `db:"updated_at"`
	UpdatedBy   string    `db:"updated_by"`
}

// ── Fee calculation ───────────────────────────────────────────────────────────

// CalculateFee computes the delivery fee for a given distance.
//
//	distance ≤ 1 km → flat base_fee   (covers the cost of dispatching at all)
//	distance  > 1 km → distance × per_km  (pure per-km, no base added)
func CalculateFee(distanceKm, baseFee, perKm float64) float64 {
	if distanceKm <= 1.0 {
		return baseFee
	}
	return distanceKm * perKm
}

// ── Estimated delivery time ───────────────────────────────────────────────────

// EstimateDeliveryMins returns a conservative delivery time in minutes.
//
// Formula: base_minutes_for_vehicle + (distance_km × mins_per_km) + 59
//
// The +59 minute buffer accounts for real-world delays:
// traffic, loading time, finding the exact address, etc.
// Better to underpromise and overdeliver than the reverse.
func EstimateDeliveryMins(vehicleType string, distanceKm float64) int {
	type profile struct {
		baseMins   int
		minsPerKm  float64
	}

	profiles := map[string]profile{
		"bike":  {baseMins: 20, minsPerKm: 4},
		"van":   {baseMins: 30, minsPerKm: 5},
		"truck": {baseMins: 45, minsPerKm: 7},
	}

	p, ok := profiles[vehicleType]
	if !ok {
		p = profiles["van"] // safe fallback
	}

	raw := p.baseMins + int(distanceKm*p.minsPerKm)
	return raw + 59 // traffic + loading buffer
}

// FormatEstimate converts minutes into a human-readable label.
//
//	79  mins → "About 1 hour 19 minutes"
//	60  mins → "About 1 hour"
//	45  mins → "About 45 minutes"
func FormatEstimate(mins int) string {
	if mins < 60 {
		return fmt.Sprintf("About %d minutes", mins)
	}
	hours := mins / 60
	remaining := mins % 60
	if remaining == 0 {
		if hours == 1 {
			return "About 1 hour"
		}
		return fmt.Sprintf("About %d hours", hours)
	}
	if hours == 1 {
		return fmt.Sprintf("About 1 hour %d minutes", remaining)
	}
	return fmt.Sprintf("About %d hours %d minutes", hours, remaining)
}

// ── Request types ─────────────────────────────────────────────────────────────

// QuoteRequest asks for a delivery quote for a given store and delivery location.
// VehicleType is optional — if omitted, quotes for all vehicle types are returned.
// The frontend uses the required_vehicle from cart validation to pre-select the
// appropriate option.
type QuoteRequest struct {
	StoreID         string  `json:"store_id"      validate:"required"`
	DeliveryLat     float64 `json:"lat"           validate:"required"`
	DeliveryLng     float64 `json:"lng"           validate:"required"`
	VehicleType     string  `json:"vehicle_type"` // optional
	RequiredVehicle string  `json:"required_vehicle"` // from cart validation
}

// UpdateRateRequest is used by admins (store-specific) and superadmin (global).
type UpdateRateRequest struct {
	BaseFee     float64 `json:"base_fee"      validate:"required,gte=0"`
	PerKm       float64 `json:"per_km"        validate:"required,gte=0"`
	MaxWeightKg float64 `json:"max_weight_kg"` // 0 = no limit
	MaxRadiusKm float64 `json:"max_radius_km"` // 0 = no limit
}

// ── Response types ────────────────────────────────────────────────────────────

// VehicleOption is one delivery option shown to the customer at checkout.
type VehicleOption struct {
	VehicleType       string  `json:"vehicle_type"`
	Fee               float64 `json:"fee"`
	Currency          string  `json:"currency"`
	EstimatedMins     int     `json:"estimated_mins"`
	EstimatedLabel    string  `json:"estimated_label"`
	IsAvailable       bool    `json:"is_available"`
	UnavailableReason string  `json:"unavailable_reason,omitempty"`
	IsRequired        bool    `json:"is_required"` // true = cart requires at least this vehicle
}

// QuoteResponse is returned to the customer before checkout.
// All vehicle options are included; unavailable ones are marked with a reason.
type QuoteResponse struct {
	StoreID            string          `json:"store_id"`
	StoreName          string          `json:"store_name"`
	DistanceKm         float64         `json:"distance_km"`
	Options            []VehicleOption `json:"options"`
	RecommendedVehicle string          `json:"recommended_vehicle"` // from cart analysis
}

// RateResponse is what admins see when managing delivery rates.
type RateResponse struct {
	VehicleType string    `json:"vehicle_type"`
	BaseFee     float64   `json:"base_fee"`
	PerKm       float64   `json:"per_km"`
	MaxWeightKg float64   `json:"max_weight_kg"`
	MaxRadiusKm float64   `json:"max_radius_km"`
	IsStoreRate bool      `json:"is_store_rate"` // true = store override, false = global default
	UpdatedAt   time.Time `json:"updated_at"`
}