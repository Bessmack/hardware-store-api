-- ── Carts ─────────────────────────────────────────────────────────────────────
-- Supports both guest carts (session-based) and registered user carts.
-- When a guest logs in, their guest cart is merged into their account cart
-- and the guest cart is deleted.
--
-- A cart belongs to EITHER a customer OR a guest session — never both.
-- The CHECK constraint below enforces this at the database level.

CREATE TABLE carts (
    id               UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id      UUID         REFERENCES users(id) ON DELETE CASCADE,
    guest_session_id VARCHAR(100) UNIQUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT cart_must_have_owner CHECK (
        (customer_id IS NOT NULL AND guest_session_id IS NULL) OR
        (customer_id IS NULL     AND guest_session_id IS NOT NULL)
    )
);

-- ── Cart Items ────────────────────────────────────────────────────────────────
-- unit_price_kes is locked at the moment the item is added.
-- If the store changes the price later, the cart shows a "price changed" warning
-- and updates the locked price — but only when the customer views the cart again.
-- The payment is always initiated for the validated price, never a stale one.

CREATE TABLE cart_items (
    id             UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    cart_id        UUID          NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    product_id     UUID          NOT NULL REFERENCES products(id),
    store_id       UUID          NOT NULL REFERENCES stores(id),
    quantity       INT           NOT NULL DEFAULT 1 CHECK (quantity > 0),

    -- Price locked at time of add-to-cart — not recalculated until cart validation
    unit_price_kes DECIMAL(10,2) NOT NULL,

    added_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    -- A product can only appear once per cart — quantity is updated, not duplicated
    CONSTRAINT unique_cart_product UNIQUE (cart_id, product_id)
);

CREATE INDEX idx_carts_customer   ON carts (customer_id);
CREATE INDEX idx_carts_session    ON carts (guest_session_id);
CREATE INDEX idx_cart_items_cart  ON cart_items (cart_id);
CREATE INDEX idx_cart_items_store ON cart_items (store_id);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON carts
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();