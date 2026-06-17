package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/Bessmack/hardware-store-api/pkg/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// ── Store report ──────────────────────────────────────────────────────────────

// GetStoreReport assembles a full StoreReport for one store across the given period.
// Each metric is a separate focused query — easier to read, easier to extend.
func (r *Repository) GetStoreReport(ctx context.Context, storeID string, f ReportFilter) (*StoreReport, error) {
	report := &StoreReport{
		StoreID:            storeID,
		Period:             Period{From: f.From, To: f.To},
		OrdersByStatus:     make(map[string]int),
		PaymentsByProvider: make(map[string]int),
		RevenueByProvider:  make(map[string]float64),
		GeneratedAt:        time.Now(),
	}

	// ── Store name + currency ─────────────────────────────────────────────────
	if err := r.db.Pool.QueryRow(ctx,
		`SELECT name, COALESCE(currency, 'KES') FROM stores WHERE id = $1`,
		storeID,
	).Scan(&report.StoreName, &report.Currency); err != nil {
		return nil, fmt.Errorf("reports: store not found: %w", err)
	}

	// ── Revenue + order totals ────────────────────────────────────────────────
	// Only count orders with a "successful" payment — excludes placed/cancelled.
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*)                          AS total_orders,
			COALESCE(SUM(grand_total), 0)     AS total_revenue,
			COALESCE(AVG(grand_total), 0)     AS avg_order_value,
			COALESCE(AVG(delivery_fee), 0)    AS avg_delivery_fee,
			COUNT(*) FILTER (WHERE delivery_type = 'delivery') AS delivery_orders,
			COUNT(*) FILTER (WHERE delivery_type = 'pickup')   AS pickup_orders
		FROM orders
		WHERE fulfilling_store_id = $1
		  AND payment_status = 'paid'
		  AND created_at BETWEEN $2 AND $3
	`, storeID, f.From, f.To).Scan(
		&report.TotalOrders,
		&report.TotalRevenue,
		&report.AverageOrderValue,
		&report.AverageDeliveryFee,
		&report.DeliveryOrders,
		&report.PickupOrders,
	); err != nil {
		return nil, fmt.Errorf("reports: revenue query failed: %w", err)
	}

	// ── Orders by status ──────────────────────────────────────────────────────
	rows, err := r.db.Pool.Query(ctx, `
		SELECT status::text, COUNT(*) AS cnt
		FROM orders
		WHERE fulfilling_store_id = $1
		  AND created_at BETWEEN $2 AND $3
		GROUP BY status
	`, storeID, f.From, f.To)
	if err != nil {
		return nil, fmt.Errorf("reports: orders by status query failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var cnt int
		if err := rows.Scan(&status, &cnt); err != nil {
			return nil, err
		}
		report.OrdersByStatus[status] = cnt
	}
	rows.Close()

	// ── Payments by provider ──────────────────────────────────────────────────
	rows, err = r.db.Pool.Query(ctx, `
		SELECT
			COALESCE(payment_provider, 'unknown') AS provider,
			COUNT(*)                               AS cnt,
			COALESCE(SUM(grand_total), 0)          AS revenue
		FROM orders
		WHERE fulfilling_store_id = $1
		  AND payment_status = 'paid'
		  AND created_at BETWEEN $2 AND $3
		GROUP BY payment_provider
	`, storeID, f.From, f.To)
	if err != nil {
		return nil, fmt.Errorf("reports: payments by provider query failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var provider string
		var cnt int
		var revenue float64
		if err := rows.Scan(&provider, &cnt, &revenue); err != nil {
			return nil, err
		}
		report.PaymentsByProvider[provider] = cnt
		report.RevenueByProvider[provider] = revenue
	}
	rows.Close()

	// ── Top 10 products by revenue ────────────────────────────────────────────
	rows, err = r.db.Pool.Query(ctx, `
		SELECT
			oi.product_id,
			oi.product_name,
			SUM(oi.quantity)                         AS units_sold,
			SUM(oi.subtotal)                         AS revenue,
			COALESCE(o.currency, $4)                 AS currency
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.fulfilling_store_id = $1
		  AND o.payment_status = 'paid'
		  AND o.created_at BETWEEN $2 AND $3
		GROUP BY oi.product_id, oi.product_name, o.currency
		ORDER BY revenue DESC
		LIMIT 10
	`, storeID, f.From, f.To, report.Currency)
	if err != nil {
		return nil, fmt.Errorf("reports: top products query failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p ProductSalesSummary
		if err := rows.Scan(&p.ProductID, &p.ProductName, &p.UnitsSold, &p.Revenue, &p.Currency); err != nil {
			return nil, err
		}
		report.TopProducts = append(report.TopProducts, p)
	}
	rows.Close()

	// ── Inventory: low stock + out of stock ───────────────────────────────────
	rows, err = r.db.Pool.Query(ctx, `
		SELECT
			si.product_id,
			p.name,
			si.quantity,
			COALESCE(si.low_stock_alert, 0)
		FROM store_inventory si
		JOIN products p ON p.id = si.product_id
		WHERE si.store_id = $1
		  AND si.quantity <= COALESCE(si.low_stock_alert, 0)
		ORDER BY si.quantity ASC
	`, storeID)
	if err != nil {
		return nil, fmt.Errorf("reports: low stock query failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item LowStockItem
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.StockQuantity, &item.LowStockAlert); err != nil {
			return nil, err
		}
		if item.StockQuantity == 0 {
			report.OutOfStockItems = append(report.OutOfStockItems, item)
		} else {
			report.LowStockItems = append(report.LowStockItems, item)
		}
	}
	rows.Close()

	// ── POD + disputes ────────────────────────────────────────────────────────
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*)                                           AS total_deliveries,
			COUNT(d.id)                                        AS total_disputes,
			COUNT(d.id) FILTER (WHERE d.status = 'open')      AS open_disputes
		FROM proof_of_delivery pod
		JOIN orders o ON o.id = pod.order_id
		LEFT JOIN disputes d ON d.order_id = o.id
		WHERE o.fulfilling_store_id = $1
		  AND pod.delivered_at BETWEEN $2 AND $3
	`, storeID, f.From, f.To).Scan(
		&report.TotalDeliveries,
		&report.TotalDisputes,
		&report.OpenDisputes,
	); err != nil {
		return nil, fmt.Errorf("reports: POD/dispute query failed: %w", err)
	}

	return report, nil
}

// ── Global report ─────────────────────────────────────────────────────────────

// GetGlobalReport assembles a platform-wide report across all stores.
func (r *Repository) GetGlobalReport(ctx context.Context, f ReportFilter) (*GlobalReport, error) {
	report := &GlobalReport{
		Period:             Period{From: f.From, To: f.To},
		PaymentsByProvider: make(map[string]int),
		GeneratedAt:        time.Now(),
	}

	// ── Platform totals ───────────────────────────────────────────────────────
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM stores)                         AS total_stores,
			(SELECT COUNT(*) FROM stores WHERE is_active = TRUE)  AS active_stores,
			(SELECT COUNT(*) FROM users WHERE role = 'customer')  AS total_customers,
			(SELECT COUNT(*) FROM users
			  WHERE role = 'customer' AND created_at BETWEEN $1 AND $2) AS new_customers
	`, f.From, f.To).Scan(
		&report.TotalStores,
		&report.ActiveStores,
		&report.TotalCustomers,
		&report.NewCustomers,
	); err != nil {
		return nil, fmt.Errorf("reports: platform totals query failed: %w", err)
	}

	// ── Order totals across all stores ───────────────────────────────────────
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM orders
		WHERE payment_status = 'paid'
		  AND created_at BETWEEN $1 AND $2
	`, f.From, f.To).Scan(&report.TotalOrders); err != nil {
		return nil, fmt.Errorf("reports: global order totals failed: %w", err)
	}

	// ── Revenue by currency to avoid mixing different currencies ─────────────
	report.RevenueByCurrency = make(map[string]float64)
	currencyRows, err := r.db.Pool.Query(ctx, `
		SELECT COALESCE(currency, 'KES') AS currency, COALESCE(SUM(grand_total), 0) AS revenue
		FROM orders
		WHERE payment_status = 'paid'
		  AND created_at BETWEEN $1 AND $2
		GROUP BY COALESCE(currency, 'KES')
	`, f.From, f.To)
	if err != nil {
		return nil, fmt.Errorf("reports: revenue by currency query failed: %w", err)
	}
	defer currencyRows.Close()
	for currencyRows.Next() {
		var cur string
		var rev float64
		if err := currencyRows.Scan(&cur, &rev); err != nil {
			return nil, err
		}
		report.RevenueByCurrency[cur] = rev
	}
	currencyRows.Close()

	// ── Payment method split ──────────────────────────────────────────────────
	rows, err := r.db.Pool.Query(ctx, `
		SELECT COALESCE(payment_provider, 'unknown'), COUNT(*)
		FROM orders
		WHERE payment_status = 'paid'
		  AND created_at BETWEEN $1 AND $2
		GROUP BY payment_provider
	`, f.From, f.To)
	if err != nil {
		return nil, fmt.Errorf("reports: global payment split failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var provider string
		var cnt int
		if err := rows.Scan(&provider, &cnt); err != nil {
			return nil, err
		}
		report.PaymentsByProvider[provider] = cnt
	}
	rows.Close()

	// ── Per-store breakdown ───────────────────────────────────────────────────
	rows, err = r.db.Pool.Query(ctx, `
		SELECT
			s.id,
			s.name,
			COALESCE(s.county, ''),
			COALESCE(s.currency, 'KES'),
			COUNT(o.id)                   AS total_orders,
			COALESCE(SUM(o.grand_total), 0) AS revenue
		FROM stores s
		LEFT JOIN orders o
			ON  o.fulfilling_store_id = s.id
			AND o.payment_status = 'paid'
			AND o.created_at BETWEEN $1 AND $2
		GROUP BY s.id, s.name, s.county, s.currency
		ORDER BY revenue DESC
	`, f.From, f.To)
	if err != nil {
		return nil, fmt.Errorf("reports: store breakdown query failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var b StoreBreakdown
		if err := rows.Scan(&b.StoreID, &b.StoreName, &b.County, &b.Currency, &b.TotalOrders, &b.Revenue); err != nil {
			return nil, err
		}
		report.StoreBreakdowns = append(report.StoreBreakdowns, b)
	}
	rows.Close()

	// ── Disputes ──────────────────────────────────────────────────────────────
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'open')
		FROM disputes
		WHERE created_at BETWEEN $1 AND $2
	`, f.From, f.To).Scan(&report.TotalDisputes, &report.OpenDisputes); err != nil {
		return nil, fmt.Errorf("reports: global disputes query failed: %w", err)
	}

	return report, nil
}