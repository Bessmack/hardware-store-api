-- ── Inventory Price History ───────────────────────────────────────────────────
-- Every price change on store_inventory is automatically recorded here
-- via a PostgreSQL trigger — no application code needed to log changes.
--
-- Use cases:
--   - Customer disputes a price → check what price was active at order time
--   - Audit trail for admin price changes
--   - Reporting on price trends per store

CREATE TABLE IF NOT EXISTS inventory_price_history (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id      UUID         NOT NULL REFERENCES stores(id),
    product_id    UUID         NOT NULL REFERENCES products(id),
    old_price     DECIMAL(10,2),             -- NULL on very first price set
    new_price     DECIMAL(10,2) NOT NULL,
    changed_by    UUID         NOT NULL REFERENCES users(id),
    changed_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    reason        TEXT                        -- optional: "supplier increase", "promotion"
);

CREATE INDEX IF NOT EXISTS idx_price_history_store_product
    ON inventory_price_history (store_id, product_id, changed_at DESC);

-- ── Trigger ───────────────────────────────────────────────────────────────────
-- Fires automatically on every UPDATE to store_inventory.price.
-- Uses IS DISTINCT FROM instead of != to handle NULL comparisons safely.

CREATE OR REPLACE FUNCTION log_price_change()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.price IS DISTINCT FROM NEW.price THEN
        INSERT INTO inventory_price_history (
            store_id,
            product_id,
            old_price,
            new_price,
            changed_by,
            changed_at
        ) VALUES (
            NEW.store_id,
            NEW.product_id,
            OLD.price,
            NEW.price,
            NEW.updated_by,
            NOW()
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop the price_change_audit trigger if it exists, then create it
DROP TRIGGER IF EXISTS price_change_audit ON store_inventory;
CREATE TRIGGER price_change_audit
    AFTER UPDATE ON store_inventory
    FOR EACH ROW EXECUTE FUNCTION log_price_change();

-- Note: The set_updated_at trigger is already created in migration 005
-- No need to recreate it here!