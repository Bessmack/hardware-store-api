-- ── Orders ────────────────────────────────────────────────────────────────────
-- All monetary values are locked at placement time — they never change
-- after the order is created, regardless of later price or fee adjustments.

CREATE TABLE orders (
    id                   UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Human-readable reference shown to customers and staff (e.g. KMB-00482)
    -- Generated in the application layer using the store's county code prefix.
    reference            VARCHAR(20)   UNIQUE NOT NULL,

    customer_id          UUID          NOT NULL REFERENCES users(id),
    fulfilling_store_id  UUID          NOT NULL REFERENCES stores(id),

    -- ── Delivery ─────────────────────────────────────────────────────────────
    delivery_type        VARCHAR(20)   NOT NULL CHECK (delivery_type IN ('delivery', 'pickup')),
    delivery_address_text TEXT,                      -- NULL for pickup orders
    delivery_lat         DECIMAL(9,6),               -- NULL for pickup orders
    delivery_lng         DECIMAL(9,6),               -- NULL for pickup orders
    vehicle_type         VARCHAR(20)   CHECK (vehicle_type IN ('bike', 'pickup', 'mini-truck','truck', 'prime-mover')), -- NULL until order is confirmed and vehicle is assigned
    vehicle_reason       TEXT,                       -- shown to customer at checkout

    -- ── Pricing (locked at placement) ────────────────────────────────────────
    items_total_kes      DECIMAL(10,2) NOT NULL,
    delivery_fee_kes     DECIMAL(10,2) NOT NULL DEFAULT 0,
    grand_total_kes      DECIMAL(10,2) NOT NULL,     -- items_total + delivery_fee

    -- ── Payment ──────────────────────────────────────────────────────────────
    payment_provider     VARCHAR(20)   CHECK (payment_provider IN ('mpesa', 'airtel', 'card')),
    payment_provider_ref VARCHAR(100),               -- M-Pesa CheckoutRequestID etc.
    payment_status       VARCHAR(20)   NOT NULL DEFAULT 'pending'
                                       CHECK (payment_status IN ('pending', 'paid', 'failed')),
    paid_at              TIMESTAMPTZ,

    -- ── Status ───────────────────────────────────────────────────────────────
    -- Full history is in order_status_history; this column is the current state.
    status               VARCHAR(30)   NOT NULL DEFAULT 'placed'
                                       CHECK (status IN (
                                           'placed',
                                           'confirmed',
                                           'preparing',
                                           'out_for_delivery',
                                           'delivered',
                                           'cancelled'
                                       )),

    created_at           TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- ── Order Items ───────────────────────────────────────────────────────────────
-- Snapshot of each product at the time of ordering.
-- product_name is stored here because the product name could change later —
-- the order record must reflect exactly what was purchased.

CREATE TABLE order_items (
    id              UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID          NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id      UUID          NOT NULL REFERENCES products(id),

    -- Snapshot values — locked forever
    product_name    VARCHAR(255)  NOT NULL,
    quantity        INT           NOT NULL CHECK (quantity > 0),
    unit_price_kes  DECIMAL(10,2) NOT NULL,
    subtotal_kes    DECIMAL(10,2) NOT NULL    -- unit_price * quantity
);

-- ── Order Status History ──────────────────────────────────────────────────────
-- Every status transition is inserted as a new row — orders are never just updated.
-- This gives customers a visible timeline and gives staff a full audit trail.
-- The 'note' column is internal only — never shown to the customer.

CREATE TABLE order_status_history (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id   UUID        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status     VARCHAR(30) NOT NULL,
    note       TEXT,                       -- internal note (e.g. "Customer called to confirm")
    changed_by UUID        REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_customer         ON orders (customer_id);
CREATE INDEX idx_orders_store            ON orders (fulfilling_store_id);
CREATE INDEX idx_orders_status           ON orders (status);
CREATE INDEX idx_orders_payment_status   ON orders (payment_status);
CREATE INDEX idx_orders_reference        ON orders (reference);
CREATE INDEX idx_order_items_order       ON order_items (order_id);
CREATE INDEX idx_order_status_history    ON order_status_history (order_id, created_at DESC);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();