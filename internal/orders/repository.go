package orders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound       = errors.New("order not found")
	ErrForbidden      = errors.New("you do not have permission to access this order")
	ErrCannotTransition = errors.New("this status change is not allowed")
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// ── Order creation ────────────────────────────────────────────────────────────

// Create inserts the order, its items, and the initial status history entry
// in a single transaction. Stock is NOT reduced here — that is the service's
// responsibility before calling Create.
func (r *Repository) Create(ctx context.Context, order *Order, items []OrderItem) (*Order, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("orders: failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert order
	row := tx.QueryRow(ctx, `
		INSERT INTO orders (
			reference, customer_id, fulfilling_store_id,
			delivery_type, delivery_address_text, delivery_lat, delivery_lng,
			vehicle_type, vehicle_reason,
			items_total, delivery_fee, grand_total,
			payment_provider, payment_status, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, reference, customer_id, fulfilling_store_id,
		          delivery_type,
		          COALESCE(delivery_address_text,''), COALESCE(delivery_lat,0), COALESCE(delivery_lng,0),
		          COALESCE(vehicle_type,''), COALESCE(vehicle_reason,''),
		          items_total, delivery_fee, grand_total,
		          COALESCE(payment_provider,''), COALESCE(payment_provider_ref,''),
		          payment_status, paid_at,
		          status, created_at, updated_at
	`,
		order.Reference, order.CustomerID, order.FulfillingStoreID,
		order.DeliveryType,
		nullIfEmpty(order.DeliveryAddressText),
		nullIfZero(order.DeliveryLat), nullIfZero(order.DeliveryLng),
		nullIfEmpty(order.VehicleType), nullIfEmpty(order.VehicleReason),
		order.ItemsTotal, order.DeliveryFee, order.GrandTotal,
		order.PaymentProvider, "pending", StatusPlaced,
	)

	created, err := scanOrder(row)
	if err != nil {
		return nil, fmt.Errorf("orders: failed to insert order: %w", err)
	}

	// Insert order items (snapshot of product at order time)
	for _, item := range items {
		_, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, product_id, product_name, quantity, unit_price, subtotal)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, created.ID, item.ProductID, item.ProductName, item.Quantity, item.UnitPrice, item.Subtotal)
		if err != nil {
			return nil, fmt.Errorf("orders: failed to insert order item: %w", err)
		}
	}

	// Initial status history entry
	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, status, changed_by)
		VALUES ($1, $2, $3)
	`, created.ID, StatusPlaced, order.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("orders: failed to insert status history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("orders: transaction commit failed: %w", err)
	}

	return created, nil
}

// ── Lookups ───────────────────────────────────────────────────────────────────

// GetByID fetches an order by UUID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Order, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, reference, customer_id, fulfilling_store_id,
		       delivery_type,
		       COALESCE(delivery_address_text,''), COALESCE(delivery_lat,0), COALESCE(delivery_lng,0),
		       COALESCE(vehicle_type,''), COALESCE(vehicle_reason,''),
		       items_total, delivery_fee, grand_total,
		       COALESCE(payment_provider,''), COALESCE(payment_provider_ref,''),
		       payment_status, paid_at,
		       status, created_at, updated_at
		FROM orders WHERE id = $1
	`, id)
	o, err := scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

// GetByReference fetches an order by its human-readable reference (e.g. KMB-00001).
func (r *Repository) GetByReference(ctx context.Context, ref string) (*Order, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, reference, customer_id, fulfilling_store_id,
		       delivery_type,
		       COALESCE(delivery_address_text,''), COALESCE(delivery_lat,0), COALESCE(delivery_lng,0),
		       COALESCE(vehicle_type,''), COALESCE(vehicle_reason,''),
		       items_total, delivery_fee, grand_total,
		       COALESCE(payment_provider,''), COALESCE(payment_provider_ref,''),
		       payment_status, paid_at,
		       status, created_at, updated_at
		FROM orders WHERE reference = $1
	`, ref)
	o, err := scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

// GetByPaymentProviderRef fetches an order by the payment provider's reference.
// Used by payment callback handlers to locate the order being paid.
func (r *Repository) GetByPaymentProviderRef(ctx context.Context, providerRef string) (*Order, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, reference, customer_id, fulfilling_store_id,
		       delivery_type,
		       COALESCE(delivery_address_text,''), COALESCE(delivery_lat,0), COALESCE(delivery_lng,0),
		       COALESCE(vehicle_type,''), COALESCE(vehicle_reason,''),
		       items_total, delivery_fee, grand_total,
		       COALESCE(payment_provider,''), COALESCE(payment_provider_ref,''),
		       payment_status, paid_at,
		       status, created_at, updated_at
		FROM orders WHERE payment_provider_ref = $1
	`, providerRef)
	o, err := scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

// GetItems fetches all items for an order.
func (r *Repository) GetItems(ctx context.Context, orderID string) ([]OrderItem, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, order_id, product_id, product_name, quantity, unit_price, subtotal
		FROM order_items WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OrderItem
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(
			&item.ID, &item.OrderID, &item.ProductID,
			&item.ProductName, &item.Quantity,
			&item.UnitPrice, &item.Subtotal,
		); err != nil {
			return nil, fmt.Errorf("orders: item scan error: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetStatusHistory returns the full status timeline for an order, oldest first.
func (r *Repository) GetStatusHistory(ctx context.Context, orderID string) ([]OrderStatusHistory, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, order_id, status, COALESCE(note,''), COALESCE(changed_by::text,''), created_at
		FROM order_status_history
		WHERE order_id = $1
		ORDER BY created_at ASC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []OrderStatusHistory
	for rows.Next() {
		var h OrderStatusHistory
		if err := rows.Scan(&h.ID, &h.OrderID, &h.Status, &h.Note, &h.ChangedBy, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("orders: history scan error: %w", err)
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

// ListByCustomer returns a customer's own orders, newest first.
func (r *Repository) ListByCustomer(ctx context.Context, customerID string, page, perPage int) ([]Order, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, reference, customer_id, fulfilling_store_id,
		       delivery_type,
		       COALESCE(delivery_address_text,''), COALESCE(delivery_lat,0), COALESCE(delivery_lng,0),
		       COALESCE(vehicle_type,''), COALESCE(vehicle_reason,''),
		       items_total, delivery_fee, grand_total,
		       COALESCE(payment_provider,''), COALESCE(payment_provider_ref,''),
		       payment_status, paid_at,
		       status, created_at, updated_at
		FROM orders
		WHERE customer_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, customerID, perPage, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectOrders(rows)
}

// ListByStore returns orders for a store, with optional status filter.
func (r *Repository) ListByStore(ctx context.Context, storeID string, f OrderFilters) ([]Order, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 || f.PerPage > 50 {
		f.PerPage = 20
	}
	offset := (f.Page - 1) * f.PerPage

	query := `
		SELECT id, reference, customer_id, fulfilling_store_id,
		       delivery_type,
		       COALESCE(delivery_address_text,''), COALESCE(delivery_lat,0), COALESCE(delivery_lng,0),
		       COALESCE(vehicle_type,''), COALESCE(vehicle_reason,''),
		       items_total, delivery_fee, grand_total,
		       COALESCE(payment_provider,''), COALESCE(payment_provider_ref,''),
		       payment_status, paid_at,
		       status, created_at, updated_at
		FROM orders
		WHERE fulfilling_store_id = $1
	`
	args := []interface{}{storeID}

	if f.Status != "" {
		args = append(args, f.Status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}

	args = append(args, f.PerPage, offset)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectOrders(rows)
}

// ── Status management ─────────────────────────────────────────────────────────

// UpdateStatus transitions an order to a new status and records it in history.
// The note is internal — never shown to customers.
func (r *Repository) UpdateStatus(ctx context.Context, orderID string, status OrderStatus, note, changedBy string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE orders SET status = $1 WHERE id = $2`,
		status, orderID,
	)
	if err != nil {
		return fmt.Errorf("orders: status update failed: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, status, note, changed_by)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,'')::uuid)
	`, orderID, status, note, changedBy)
	if err != nil {
		return fmt.Errorf("orders: status history insert failed: %w", err)
	}

	return tx.Commit(ctx)
}

// UpdatePaymentStatus records a payment result. Called by payment callbacks.
func (r *Repository) UpdatePaymentStatus(ctx context.Context, orderID, paymentStatus, providerRef string) error {
	var paidAt interface{}
	if paymentStatus == "paid" {
		t := time.Now()
		paidAt = t
	}

	_, err := r.db.Pool.Exec(ctx, `
		UPDATE orders
		SET payment_status       = $1,
		    payment_provider_ref = COALESCE(NULLIF($2,''), payment_provider_ref),
		    paid_at              = $3
		WHERE id = $4
	`, paymentStatus, providerRef, paidAt, orderID)
	return err
}

// CountByStore returns the number of orders for a store.
// Used to generate sequential order references.
func (r *Repository) CountByStore(ctx context.Context, storeID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE fulfilling_store_id = $1`, storeID,
	).Scan(&count)
	return count, err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func scanOrder(row pgx.Row) (*Order, error) {
	var o Order
	if err := row.Scan(
		&o.ID, &o.Reference, &o.CustomerID, &o.FulfillingStoreID,
		&o.DeliveryType,
		&o.DeliveryAddressText, &o.DeliveryLat, &o.DeliveryLng,
		&o.VehicleType, &o.VehicleReason,
		&o.ItemsTotal, &o.DeliveryFee, &o.GrandTotal,
		&o.PaymentProvider, &o.PaymentProviderRef,
		&o.PaymentStatus, &o.PaidAt,
		&o.Status, &o.CreatedAt, &o.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &o, nil
}

func collectOrders(rows pgx.Rows) ([]Order, error) {
	var result []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(
			&o.ID, &o.Reference, &o.CustomerID, &o.FulfillingStoreID,
			&o.DeliveryType,
			&o.DeliveryAddressText, &o.DeliveryLat, &o.DeliveryLng,
			&o.VehicleType, &o.VehicleReason,
			&o.ItemsTotal, &o.DeliveryFee, &o.GrandTotal,
			&o.PaymentProvider, &o.PaymentProviderRef,
			&o.PaymentStatus, &o.PaidAt,
			&o.Status, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("orders: scan error: %w", err)
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(f float64) interface{} {
	if f == 0 {
		return nil
	}
	return f
}

// GenerateReference builds a human-readable order reference.
// Format: {COUNTY_PREFIX}-{5-DIGIT-PADDED-COUNT}
// e.g.  KMB-00042, MSA-00001
// The county prefix is the first 3 letters of the store's county, uppercase.
func GenerateReference(county string, existingCount int) string {
	county = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(county), " ", ""))
	prefix := county
	if len(prefix) > 3 {
		prefix = prefix[:3]
	}
	if prefix == "" {
		prefix = "ORD"
	}
	return fmt.Sprintf("%s-%05d", prefix, existingCount+1)
}