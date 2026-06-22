package products

import "time"

// ── Constraint types ──────────────────────────────────────────────────────────

// ConstraintType determines how the delivery vehicle is chosen for an item.
type ConstraintType string

const (
	// ConstraintWeight — vehicle is chosen by total order weight.
	// Most products fall in this category (cement bags, paint, tiles).
	ConstraintWeight ConstraintType = "weight"

	// ConstraintDimension — vehicle is hard-set regardless of quantity.
	// Use for items that physically cannot fit on a smaller vehicle
	// (e.g. iron sheets, long steel pipes, door frames).
	ConstraintDimension ConstraintType = "dimension"

	// ConstraintHazardous — always requires van minimum regardless of weight.
	// Use for gas cylinders, solvents, and other hazardous materials.
	ConstraintHazardous ConstraintType = "hazardous"
)

// VehicleType mirrors the delivery_rates table primary key.
type VehicleType string

const (
	VehicleBike       VehicleType = "bike"
	VehiclePickup     VehicleType = "pickup"
	VehicleMiniTruck  VehicleType = "mini-truck"
	VehicleTruck      VehicleType = "truck"
	VehiclePrimeMover VehicleType = "prime-mover"
)

// ── Core model ────────────────────────────────────────────────────────────────

// Product holds universal product information shared across all stores.
// Price and stock are NOT stored here — they live in store_inventory.
type Product struct {
	ID             string         `db:"id"`
	Name           string         `db:"name"`
	Description    string         `db:"description"`
	Category       string         `db:"category"`        // legacy free-text; prefer SubcategoryID
	SubcategoryID  string         `db:"subcategory_id"`  // FK → subcategories.id
	WeightKg       float64        `db:"weight_kg"`
	LengthCm       float64        `db:"length_cm"`
	WidthCm        float64        `db:"width_cm"`
	HeightCm       float64        `db:"height_cm"`
	ConstraintType ConstraintType `db:"constraint_type"`
	MinVehicleType VehicleType    `db:"min_vehicle_type"` // only set for dimension/hazardous
	Images         []string       `db:"images"`
	IsActive       bool           `db:"is_active"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
	UpdatedBy      string         `db:"updated_by"`
}

// ProductWithInventory joins a product with its store-specific price and stock.
// This is the internal model — never serialized directly.
type ProductWithInventory struct {
	Product
	Price          float64 `db:"price"`
	Currency       string  `db:"currency"`
	StockQuantity  int     `db:"stock_quantity"`
	LowStockAlert  int     `db:"low_stock_alert"`
	IsAvailable    bool    `db:"is_available"`
	// Denormalised from JOIN — populated by ListAll and detail queries.
	SubcategoryName string `db:"subcategory_name"`
	CategoryName    string `db:"category_name"`
	CategorySlug    string `db:"category_slug"`
}

// ── Request types ─────────────────────────────────────────────────────────────

type CreateProductRequest struct {
	Name           string         `json:"name"            validate:"required"`
	Description    string         `json:"description"`
	SubcategoryID  string         `json:"subcategory_id"`
	WeightKg       float64        `json:"weight_kg"`
	LengthCm       float64        `json:"length_cm"`
	WidthCm        float64        `json:"width_cm"`
	HeightCm       float64        `json:"height_cm"`
	ConstraintType ConstraintType `json:"constraint_type" validate:"required,oneof=weight dimension hazardous"`
	MinVehicleType VehicleType    `json:"min_vehicle_type" validate:"omitempty,oneof=bike pickup mini-truck truck prime-mover"`
	Images         []string       `json:"images"`
}

type UpdateProductRequest struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	SubcategoryID  string         `json:"subcategory_id"`
	WeightKg       float64        `json:"weight_kg"`
	LengthCm       float64        `json:"length_cm"`
	WidthCm        float64        `json:"width_cm"`
	HeightCm       float64        `json:"height_cm"`
	ConstraintType ConstraintType `json:"constraint_type" validate:"omitempty,oneof=weight dimension hazardous"`
	MinVehicleType VehicleType    `json:"min_vehicle_type" validate:"omitempty,oneof=bike pickup mini-truck truck prime-mover"`
	Images         []string       `json:"images"`
}

// ── Response types ────────────────────────────────────────────────────────────

// ProductCustomerResponse is what guests and customers see.
// StockQuantity is deliberately absent — customers only see InStock and
// LimitedAvailability to prevent theft-inducing stock visibility.
type ProductCustomerResponse struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description,omitempty"`
	SubcategoryID       string   `json:"subcategory_id,omitempty"`
	SubcategoryName     string   `json:"subcategory_name,omitempty"`
	CategoryName        string   `json:"category_name,omitempty"`
	CategorySlug        string   `json:"category_slug,omitempty"`
	Images              []string `json:"images"`
	Price               float64  `json:"price"`
	Currency            string   `json:"currency"`
	InStock             bool     `json:"in_stock"`
	LimitedAvailability bool     `json:"limited_availability"`
}

// ProductStaffResponse is what cashiers, admins, and superadmin see.
// Includes full inventory detail — stock quantity, low stock threshold, etc.
type ProductStaffResponse struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	SubcategoryID   string         `json:"subcategory_id,omitempty"`
	SubcategoryName string         `json:"subcategory_name,omitempty"`
	CategoryName    string         `json:"category_name,omitempty"`
	WeightKg       float64        `json:"weight_kg"`
	LengthCm       float64        `json:"length_cm,omitempty"`
	WidthCm        float64        `json:"width_cm,omitempty"`
	HeightCm       float64        `json:"height_cm,omitempty"`
	ConstraintType ConstraintType `json:"constraint_type"`
	MinVehicleType VehicleType    `json:"min_vehicle_type,omitempty"`
	Images         []string       `json:"images"`
	Price          float64        `json:"price"`
	Currency       string         `json:"currency"`
	StockQuantity  int            `json:"stock_quantity"`
	LowStockAlert  int            `json:"low_stock_alert"`
	IsAvailable    bool           `json:"is_available"`
	IsActive       bool           `json:"is_active"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// AllPricesResponse is used on a product detail page to show pricing across
// all stores — helps customers find the cheapest branch.
type StorePriceEntry struct {
	StoreID    string  `json:"store_id"`
	StoreName  string  `json:"store_name"`
	County     string  `json:"county"`
	Price      float64 `json:"price"`
	Currency   string  `json:"currency"`
	InStock    bool    `json:"in_stock"`
}

// ProductDetailResponse is returned by GET /api/v1/products/{id}.
// Includes physical specs and prices across every active store so the
// customer can compare branches in a single request.
type ProductDetailResponse struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	SubcategoryID   string            `json:"subcategory_id,omitempty"`
	SubcategoryName string            `json:"subcategory_name,omitempty"`
	CategoryName    string            `json:"category_name,omitempty"`
	CategorySlug    string            `json:"category_slug,omitempty"`
	Images          []string          `json:"images"`
	WeightKg        float64           `json:"weight_kg,omitempty"`
	LengthCm        float64           `json:"length_cm,omitempty"`
	WidthCm         float64           `json:"width_cm,omitempty"`
	HeightCm        float64           `json:"height_cm,omitempty"`
	ConstraintType  ConstraintType    `json:"constraint_type"`
	MinVehicleType  VehicleType       `json:"min_vehicle_type,omitempty"`
	StorePrices     []StorePriceEntry `json:"store_prices"`
}

// ── Mappers ───────────────────────────────────────────────────────────────────

// ToCustomerResponse converts an internal ProductWithInventory to the safe
// customer-facing view. Stock quantity is never included.
func ToCustomerResponse(p ProductWithInventory) ProductCustomerResponse {
	return ProductCustomerResponse{
		ID:                  p.ID,
		Name:                p.Name,
		Description:         p.Description,
		SubcategoryID:       p.SubcategoryID,
		SubcategoryName:     p.SubcategoryName,
		CategoryName:        p.CategoryName,
		CategorySlug:        p.CategorySlug,
		Images:              p.Images,
		Price:               p.Price,
		Currency:            p.Currency,
		InStock:             p.StockQuantity > 0 && p.IsAvailable,
		LimitedAvailability: p.StockQuantity > 0 && p.StockQuantity <= p.LowStockAlert,
	}
}

// ToStaffResponse converts to the full staff view including stock numbers.
func ToStaffResponse(p ProductWithInventory) ProductStaffResponse {
	return ProductStaffResponse{
		ID:             p.ID,
		Name:           p.Name,
		Description:    p.Description,
		SubcategoryID:   p.SubcategoryID,
		SubcategoryName: p.SubcategoryName,
		CategoryName:    p.CategoryName,
		WeightKg:       p.WeightKg,
		LengthCm:       p.LengthCm,
		WidthCm:        p.WidthCm,
		HeightCm:       p.HeightCm,
		ConstraintType: p.ConstraintType,
		MinVehicleType: p.MinVehicleType,
		Images:         p.Images,
		Price:          p.Price,
		Currency:       p.Currency,
		StockQuantity:  p.StockQuantity,
		LowStockAlert:  p.LowStockAlert,
		IsAvailable:    p.IsAvailable,
		IsActive:       p.IsActive,
		UpdatedAt:      p.UpdatedAt,
	}
}

func ToDetailResponse(p ProductWithInventory, prices []StorePriceEntry) ProductDetailResponse {
	return ProductDetailResponse{
		ID:              p.ID,
		Name:            p.Name,
		Description:     p.Description,
		SubcategoryID:   p.SubcategoryID,
		SubcategoryName: p.SubcategoryName,
		CategoryName:    p.CategoryName,
		CategorySlug:    p.CategorySlug,
		Images:          p.Images,
		WeightKg:        p.WeightKg,
		LengthCm:        p.LengthCm,
		WidthCm:         p.WidthCm,
		HeightCm:        p.HeightCm,
		ConstraintType:  p.ConstraintType,
		MinVehicleType:  p.MinVehicleType,
		StorePrices:     prices,
	}
}