package wishlist

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound         = errors.New("wishlist not found")
	ErrItemNotFound     = errors.New("wishlist item not found")
	ErrItemAlreadyAdded = errors.New("product is already in this wishlist")
	ErrNotOwner         = errors.New("this wishlist does not belong to you")
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// ── Wishlists ─────────────────────────────────────────────────────────────────

// Create makes a new wishlist for the customer.
func (r *Repository) Create(ctx context.Context, customerID, name string) (*Wishlist, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO wishlists (customer_id, name) VALUES ($1, $2)
		RETURNING id, customer_id, name, created_at
	`, customerID, name)

	var w Wishlist
	if err := row.Scan(&w.ID, &w.CustomerID, &w.Name, &w.CreatedAt); err != nil {
		return nil, err
	}
	return &w, nil
}

// GetByID returns a wishlist, verifying it belongs to the customer.
func (r *Repository) GetByID(ctx context.Context, id, customerID string) (*Wishlist, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, customer_id, name, created_at
		FROM wishlists WHERE id = $1
	`, id)

	var w Wishlist
	if err := row.Scan(&w.ID, &w.CustomerID, &w.Name, &w.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if w.CustomerID != customerID {
		return nil, ErrNotOwner
	}
	return &w, nil
}

// ListByCustomer returns all wishlists for a customer with item counts.
func (r *Repository) ListByCustomer(ctx context.Context, customerID string) ([]WishlistSummary, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT w.id, w.name, w.created_at, COUNT(wi.id) AS item_count
		FROM wishlists w
		LEFT JOIN wishlist_items wi ON wi.wishlist_id = w.id
		WHERE w.customer_id = $1
		GROUP BY w.id, w.name, w.created_at
		ORDER BY w.created_at
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []WishlistSummary
	for rows.Next() {
		var ws WishlistSummary
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.CreatedAt, &ws.ItemCount); err != nil {
			return nil, fmt.Errorf("wishlist: summary scan error: %w", err)
		}
		result = append(result, ws)
	}
	return result, rows.Err()
}

// EnsureDefault returns the customer's default wishlist, creating it if needed.
// Called automatically when a customer adds their first item.
func (r *Repository) EnsureDefault(ctx context.Context, customerID string) (*Wishlist, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, customer_id, name, created_at
		FROM wishlists WHERE customer_id = $1 AND name = 'My Wishlist'
		LIMIT 1
	`, customerID)

	var w Wishlist
	if err := row.Scan(&w.ID, &w.CustomerID, &w.Name, &w.CreatedAt); err == nil {
		return &w, nil
	}

	return r.Create(ctx, customerID, "My Wishlist")
}

// Delete removes a wishlist and all its items.
func (r *Repository) Delete(ctx context.Context, id, customerID string) error {
	if _, err := r.GetByID(ctx, id, customerID); err != nil {
		return err
	}
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM wishlists WHERE id = $1`, id)
	return err
}

// ── Items ─────────────────────────────────────────────────────────────────────

// AddItem adds a product to the wishlist.
func (r *Repository) AddItem(ctx context.Context, wishlistID, productID, note string) (*WishlistItem, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO wishlist_items (wishlist_id, product_id, note)
		VALUES ($1, $2, NULLIF($3, ''))
		RETURNING id, wishlist_id, product_id, COALESCE(note, ''), added_at
	`, wishlistID, productID, note)

	var item WishlistItem
	if err := row.Scan(&item.ID, &item.WishlistID, &item.ProductID, &item.Note, &item.AddedAt); err != nil {
		// Unique constraint violation means item already added
		if err.Error() == `ERROR: duplicate key value violates unique constraint "unique_wishlist_product" (SQLSTATE 23505)` {
			return nil, ErrItemAlreadyAdded
		}
		return nil, err
	}
	return &item, nil
}

// RemoveItem deletes an item from a wishlist, verifying ownership.
func (r *Repository) RemoveItem(ctx context.Context, wishlistID, itemID, customerID string) error {
	// Verify the wishlist belongs to this customer first
	if _, err := r.GetByID(ctx, wishlistID, customerID); err != nil {
		return err
	}

	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM wishlist_items WHERE id = $1 AND wishlist_id = $2`,
		itemID, wishlistID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrItemNotFound
	}
	return nil
}

// GetRawItems returns raw wishlist items with basic product info.
// Pricing is NOT fetched here — it is added in the service layer using the
// nearest store for each customer.
func (r *Repository) GetRawItems(ctx context.Context, wishlistID string) ([]WishlistItemResponse, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			wi.id, wi.product_id,
			p.name, COALESCE(p.category, ''),
			p.images,
			COALESCE(wi.note, ''),
			wi.added_at
		FROM wishlist_items wi
		JOIN products p ON p.id = wi.product_id
		WHERE wi.wishlist_id = $1
		ORDER BY wi.added_at DESC
	`, wishlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []WishlistItemResponse
	for rows.Next() {
		var item WishlistItemResponse
		var images pgtype.Array[string]
		if err := rows.Scan(
			&item.ID, &item.ProductID,
			&item.ProductName, &item.Category,
			&images,
			&item.Note, &item.AddedAt,
		); err != nil {
			return nil, fmt.Errorf("wishlist: item scan error: %w", err)
		}
		item.Images = images.Elements
		items = append(items, item)
	}
	return items, rows.Err()
}