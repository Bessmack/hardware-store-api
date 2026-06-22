-- ── Store Inventory ───────────────────────────────────────────────────────────
-- Each store sets its own price and tracks its own stock per product.
-- The same product can have different prices at different stores
-- (e.g. cement KES 1,000 in Nairobi, KES 830 in Mombasa).
--
-- Customers NEVER see stock_quantity — only is_available (boolean).
-- Staff (cashiers, admins, superadmin) see stock_quantity and low_stock_alert.
-- This is enforced in the application response layer, not here.

CREATE TABLE IF NOT EXISTS store_inventory (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id        UUID         NOT NULL REFERENCES stores(id)   ON DELETE CASCADE,
    product_id      UUID         NOT NULL REFERENCES products(id) ON DELETE CASCADE,

    price_kes       DECIMAL(10,2) NOT NULL CHECK (price_kes >= 0),
    stock_quantity  INT           NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),

    -- When stock_quantity falls at or below this number, staff see a low-stock warning.
    -- Customers never see this value — they only see limited_availability: true/false.
    low_stock_alert INT           NOT NULL DEFAULT 10,

    -- Set to FALSE to hide a product from this store without deleting it
    is_available    BOOLEAN       NOT NULL DEFAULT TRUE,

    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_by      UUID          REFERENCES users(id),

    -- One price/stock entry per product per store
    CONSTRAINT unique_store_product UNIQUE (store_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_inventory_store     ON store_inventory (store_id);
CREATE INDEX IF NOT EXISTS idx_inventory_product   ON store_inventory (product_id);
CREATE INDEX IF NOT EXISTS idx_inventory_available ON store_inventory (store_id, is_available);

-- Drop trigger if it exists (idempotent)
DROP TRIGGER IF EXISTS set_updated_at ON store_inventory;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON store_inventory
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();