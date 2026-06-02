package wishlist

import "time"

// ── Core models ───────────────────────────────────────────────────────────────

// Wishlist is a named collection of products a customer saves for later.
// Customers can maintain multiple named wishlists for different projects
// (e.g. "Kitchen Renovation", "Perimeter Wall", "Office Fit-Out").
type Wishlist struct {
	ID         string    `db:"id"`
	CustomerID string    `db:"customer_id"`
	Name       string    `db:"name"`
	CreatedAt  time.Time `db:"created_at"`
}

type WishlistItem struct {
	ID         string    `db:"id"`
	WishlistID string    `db:"wishlist_id"`
	ProductID  string    `db:"product_id"`
	Note       string    `db:"note"`
	AddedAt    time.Time `db:"added_at"`
}

// ── Request types ─────────────────────────────────────────────────────────────

type CreateWishlistRequest struct {
	Name string `json:"name" validate:"required,max=100"`
}

type AddItemRequest struct {
	ProductID string `json:"product_id" validate:"required"`
	Note      string `json:"note"` // optional personal note
}

// ── Response types ────────────────────────────────────────────────────────────

// WishlistSummary is used when listing all of a customer's wishlists.
type WishlistSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ItemCount int       `json:"item_count"`
	CreatedAt time.Time `json:"created_at"`
}

// WishlistItemResponse is what the customer sees for each wishlisted product.
//
// Key design decisions:
//   - Price and currency are fetched live from the nearest store — not stored
//   - InStock is a bool only — never reveal stock quantity
//   - LimitedAvailability is true when stock is low — no threshold number shown
//   - NearestStoreID/Name tells the customer which store the price is from
type WishlistItemResponse struct {
	ID          string    `json:"id"`           // wishlist item ID
	ProductID   string    `json:"product_id"`
	ProductName string    `json:"product_name"`
	Category    string    `json:"category"`
	Images      []string  `json:"images"`
	Note        string    `json:"note,omitempty"`
	AddedAt     time.Time `json:"added_at"`

	// Live pricing from nearest store — fetched at serve time, not stored
	Price               float64 `json:"price"`
	Currency            string  `json:"currency"`
	InStock             bool    `json:"in_stock"`
	LimitedAvailability bool    `json:"limited_availability"` // low stock flag, no count
	NearestStoreID      string  `json:"nearest_store_id"`
	NearestStoreName    string  `json:"nearest_store_name"`
}

// WishlistResponse is the full wishlist with all items.
type WishlistResponse struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	CreatedAt time.Time              `json:"created_at"`
	Items     []WishlistItemResponse `json:"items"`
}