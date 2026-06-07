package cart

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/internal/orders"
	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound      = errors.New("cart not found")
	ErrItemNotFound  = errors.New("cart item not found")
	ErrCartMismatch  = errors.New("item does not belong to this cart")
)

type Repository struct {
	db *database.DB
}

// ClearCart implements [orders.CartReader].
func (r *Repository) ClearCart(ctx context.Context, customerID string, sessionID string) error {
	panic("unimplemented")
}


// ── orders.CartReader implementation ─────────────────────────────────────────

// GetItemsForOrder returns cart items in the format the orders service needs.
// Resolves the cart from customerID (logged in) or sessionID (guest).
func (r *Repository) GetItemsForOrder(ctx context.Context, customerID, sessionID string) ([]orders.CartItemForOrder, error) {
	var cartID string

	if customerID != "" {
		err := r.db.Pool.QueryRow(ctx,
			`SELECT id FROM carts WHERE customer_id = $1`, customerID,
		).Scan(&cartID)
		if err != nil {
			return nil, nil // no cart = empty
		}
	} else if sessionID != "" {
		err := r.db.Pool.QueryRow(ctx,
			`SELECT id FROM carts WHERE guest_session_id = $1`, sessionID,
		).Scan(&cartID)
		if err != nil {
			return nil, nil
		}
	}
	if cartID == "" {
		return nil, nil
	}

	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			ci.product_id, p.name, ci.store_id,
			ci.quantity, ci.unit_price, ci.currency,
			COALESCE(si.stock_quantity > 0 AND si.is_available, false)
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		LEFT JOIN store_inventory si
			ON si.product_id = ci.product_id AND si.store_id = ci.store_id
		WHERE ci.cart_id = $1
	`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []orders.CartItemForOrder
	for rows.Next() {
		var item orders.CartItemForOrder
		if err := rows.Scan(
			&item.ProductID, &item.ProductName, &item.StoreID,
			&item.Quantity, &item.UnitPrice, &item.Currency,
			&item.InStock,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// ── Cart lifecycle ────────────────────────────────────────────────────────────

// GetOrCreateByCustomerID returns the customer's existing cart or creates a new one.
func (r *Repository) GetOrCreateByCustomerID(ctx context.Context, customerID string) (*Cart, error) {
	// Try existing
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, COALESCE(customer_id::text,''), COALESCE(guest_session_id,''), created_at, updated_at
		 FROM carts WHERE customer_id = $1`, customerID)

	c, err := scanCart(row)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Create new
	row = r.db.Pool.QueryRow(ctx,
		`INSERT INTO carts (customer_id) VALUES ($1)
		 RETURNING id, COALESCE(customer_id::text,''), COALESCE(guest_session_id,''), created_at, updated_at`,
		customerID)
	return scanCart(row)
}

// GetOrCreateBySessionID returns a guest cart or creates one.
func (r *Repository) GetOrCreateBySessionID(ctx context.Context, sessionID string) (*Cart, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, COALESCE(customer_id::text,''), COALESCE(guest_session_id,''), created_at, updated_at
		 FROM carts WHERE guest_session_id = $1`, sessionID)

	c, err := scanCart(row)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	row = r.db.Pool.QueryRow(ctx,
		`INSERT INTO carts (guest_session_id) VALUES ($1)
		 RETURNING id, COALESCE(customer_id::text,''), COALESCE(guest_session_id,''), created_at, updated_at`,
		sessionID)
	return scanCart(row)
}

// GetByID fetches a cart by UUID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Cart, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, COALESCE(customer_id::text,''), COALESCE(guest_session_id,''), created_at, updated_at
		 FROM carts WHERE id = $1`, id)
	c, err := scanCart(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// Delete removes a cart and all its items (cascade).
func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM carts WHERE id = $1`, id)
	return err
}

// ── Items ─────────────────────────────────────────────────────────────────────

// GetItems returns all items in a cart with product and store data joined.
func (r *Repository) GetItems(ctx context.Context, cartID string) ([]CartItemResponse, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			ci.id, ci.product_id,
			p.name, COALESCE(p.images[1], ''),
			ci.store_id, s.name,
			ci.quantity,
			ci.unit_price, ci.unit_price * ci.quantity,
			ci.currency,
			COALESCE(si.stock_quantity > 0 AND si.is_available, false),
			COALESCE(si.price, 0)
		FROM cart_items ci
		JOIN products p  ON p.id  = ci.product_id
		JOIN stores   s  ON s.id  = ci.store_id
		LEFT JOIN store_inventory si
			ON si.product_id = ci.product_id AND si.store_id = ci.store_id
		WHERE ci.cart_id = $1
		ORDER BY ci.added_at
	`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CartItemResponse
	for rows.Next() {
		var item CartItemResponse
		var currentPrice float64
		if err := rows.Scan(
			&item.ID, &item.ProductID,
			&item.ProductName, &item.Image,
			&item.StoreID, &item.StoreName,
			&item.Quantity,
			&item.UnitPrice, &item.Subtotal,
			&item.Currency,
			&item.InStock,
			&currentPrice,
		); err != nil {
			return nil, fmt.Errorf("cart: item scan error: %w", err)
		}

		// Flag price changes since the item was added
		item.CurrentPrice = currentPrice
		item.PriceChanged = currentPrice != item.UnitPrice

		items = append(items, item)
	}
	return items, rows.Err()
}

// GetItemsForVehicle returns lightweight product data needed for vehicle determination.
func (r *Repository) GetItemsForVehicle(ctx context.Context, cartID string) ([]CartItemForVehicle, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			p.name,
			p.constraint_type,
			COALESCE(p.min_vehicle_type, ''),
			COALESCE(p.weight_kg, 0),
			ci.quantity
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.cart_id = $1
	`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CartItemForVehicle
	for rows.Next() {
		var i CartItemForVehicle
		if err := rows.Scan(&i.ProductName, &i.ConstraintType, &i.MinVehicleType, &i.WeightKg, &i.Quantity); err != nil {
			return nil, fmt.Errorf("cart: vehicle item scan error: %w", err)
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// AddItem inserts an item with a locked price, or increments quantity if already present.
func (r *Repository) AddItem(ctx context.Context, cartID string, req AddItemRequest, unitPrice float64, currency string) (*CartItem, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO cart_items (cart_id, product_id, store_id, quantity, unit_price, currency)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (cart_id, product_id)
		DO UPDATE SET
			quantity   = cart_items.quantity + EXCLUDED.quantity,
			unit_price = EXCLUDED.unit_price,
			currency   = EXCLUDED.currency
		RETURNING id, cart_id, product_id, store_id, quantity, unit_price, currency, added_at
	`, cartID, req.ProductID, req.StoreID, req.Quantity, unitPrice, currency)

	return scanCartItem(row)
}

// UpdateQuantity sets the quantity of an existing cart item.
func (r *Repository) UpdateQuantity(ctx context.Context, cartID, itemID string, qty int) error {
	result, err := r.db.Pool.Exec(ctx,
		`UPDATE cart_items SET quantity = $1 WHERE id = $2 AND cart_id = $3`,
		qty, itemID, cartID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrItemNotFound
	}
	return nil
}

// UpdateItemPrice updates a locked price — called when validation detects a change.
func (r *Repository) UpdateItemPrice(ctx context.Context, itemID string, newPrice float64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE cart_items SET unit_price = $1 WHERE id = $2`,
		newPrice, itemID,
	)
	return err
}

// RemoveItem deletes one item from the cart.
func (r *Repository) RemoveItem(ctx context.Context, cartID, itemID string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM cart_items WHERE id = $1 AND cart_id = $2`,
		itemID, cartID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrItemNotFound
	}
	return nil
}

// ClearItems deletes all items from a cart without deleting the cart itself.
func (r *Repository) ClearItems(ctx context.Context, cartID string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, cartID)
	return err
}

// ── Cart merge (guest → user on login) ───────────────────────────────────────

// MergeGuestCart moves all items from the guest cart into the customer cart.
// If an item already exists in the customer cart, the higher quantity is kept.
// The guest cart is deleted after a successful merge.
func (r *Repository) MergeGuestCart(ctx context.Context, guestSessionID, customerID string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Find the guest cart
	var guestCartID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM carts WHERE guest_session_id = $1`, guestSessionID,
	).Scan(&guestCartID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // nothing to merge
	}
	if err != nil {
		return err
	}

	// Get or create the customer cart
	var customerCartID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM carts WHERE customer_id = $1`, customerID,
	).Scan(&customerCartID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx,
			`INSERT INTO carts (customer_id) VALUES ($1) RETURNING id`, customerID,
		).Scan(&customerCartID)
	}
	if err != nil {
		return err
	}

	// Merge items — upsert with highest quantity winning
	_, err = tx.Exec(ctx, `
		INSERT INTO cart_items (cart_id, product_id, store_id, quantity, unit_price, currency)
		SELECT $1, product_id, store_id, quantity, unit_price, currency
		FROM cart_items WHERE cart_id = $2
		ON CONFLICT (cart_id, product_id) DO UPDATE
			SET quantity = GREATEST(cart_items.quantity, EXCLUDED.quantity)
	`, customerCartID, guestCartID)
	if err != nil {
		return fmt.Errorf("cart: merge failed: %w", err)
	}

	// Delete guest cart (cascades to items)
	if _, err := tx.Exec(ctx, `DELETE FROM carts WHERE id = $1`, guestCartID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func scanCart(row pgx.Row) (*Cart, error) {
	var c Cart
	if err := row.Scan(&c.ID, &c.CustomerID, &c.GuestSessionID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func scanCartItem(row pgx.Row) (*CartItem, error) {
	var i CartItem
	if err := row.Scan(&i.ID, &i.CartID, &i.ProductID, &i.StoreID, &i.Quantity, &i.UnitPrice, &i.Currency, &i.AddedAt); err != nil {
		return nil, err
	}
	return &i, nil
}