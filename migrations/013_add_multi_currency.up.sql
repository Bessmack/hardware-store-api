-- ── Migration 013: Multi-currency support ────────────────────────────────────
--
-- Three changes:
--   1. Add currency (ISO 4217) to the stores table
--   2. Rename all *_kes price columns to generic names
--   3. Make delivery_rates per-store with global defaults as fallback
--
-- Run this BEFORE running the application for the first time.
-- If you already ran earlier migrations, apply this in order.

-- ── 1. Currency on stores ─────────────────────────────────────────────────────
-- ISO 4217 three-letter currency code (KES, USD, TZS, UGX, EUR, etc.)
-- Default is KES (Kenyan Shilling) since that is the primary market.
-- Validated by a CHECK constraint — only uppercase 3-letter codes accepted.

DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='stores' AND column_name='currency') THEN
        ALTER TABLE stores ADD COLUMN currency VARCHAR(3) NOT NULL DEFAULT 'KES'
            CHECK (currency ~ '^[A-Z]{3}$');
    END IF;
END $$;

COMMENT ON COLUMN stores.currency IS
    'ISO 4217 currency code. All prices at this store are in this currency.';

-- ── 2. Rename _kes price columns ──────────────────────────────────────────────
-- The _kes suffix will be misleading for overseas stores.
-- Rename now while the schema is still fresh.

DO $$ 
BEGIN
    -- store_inventory
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='store_inventory' AND column_name='price_kes') THEN
        ALTER TABLE store_inventory RENAME COLUMN price_kes TO price;
    END IF;
    
    -- inventory_price_history
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='inventory_price_history' AND column_name='old_price_kes') THEN
        ALTER TABLE inventory_price_history RENAME COLUMN old_price_kes TO old_price;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='inventory_price_history' AND column_name='new_price_kes') THEN
        ALTER TABLE inventory_price_history RENAME COLUMN new_price_kes TO new_price;
    END IF;
    
    -- cart_items
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='cart_items' AND column_name='unit_price_kes') THEN
        ALTER TABLE cart_items RENAME COLUMN unit_price_kes TO unit_price;
    END IF;
    
    -- orders
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='orders' AND column_name='items_total_kes') THEN
        ALTER TABLE orders RENAME COLUMN items_total_kes TO items_total;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='orders' AND column_name='delivery_fee_kes') THEN
        ALTER TABLE orders RENAME COLUMN delivery_fee_kes TO delivery_fee;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='orders' AND column_name='grand_total_kes') THEN
        ALTER TABLE orders RENAME COLUMN grand_total_kes TO grand_total;
    END IF;
END $$;

-- ── 3. Per-store delivery rates ───────────────────────────────────────────────
-- Vehicle types (from migration 011): bike | pickup | mini-truck | truck | prime-mover
-- The CHECK constraint on vehicle_type is inherited from migration 011 and unchanged here.
-- Add store_id (nullable) to delivery_rates.
-- NULL store_id = global default, used when a store has no specific rate.
-- A store can override any vehicle type by inserting a row with its store_id.
--
-- Lookup logic in the delivery service:
--   SELECT * FROM delivery_rates
--   WHERE vehicle_type = $type
--     AND (store_id = $storeID OR store_id IS NULL)
--   ORDER BY store_id NULLS LAST   -- store-specific beats global
--   LIMIT 1

DO $$ 
BEGIN
    -- Check if store_id column exists
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='delivery_rates' AND column_name='store_id') THEN
        ALTER TABLE delivery_rates ADD COLUMN store_id UUID REFERENCES stores(id) ON DELETE CASCADE;
    END IF;
    
    -- Check if base_fee column exists (rename from base_fee_kes)
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='delivery_rates' AND column_name='base_fee_kes') THEN
        ALTER TABLE delivery_rates RENAME COLUMN base_fee_kes TO base_fee;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='delivery_rates' AND column_name='per_km_kes') THEN
        ALTER TABLE delivery_rates RENAME COLUMN per_km_kes TO per_km;
    END IF;
END $$;

-- Drop the old primary key if it exists
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'delivery_rates_pkey') THEN
        ALTER TABLE delivery_rates DROP CONSTRAINT delivery_rates_pkey;
    END IF;
END $$;

-- Add new composite unique constraint if it doesn't exist
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'delivery_rates_store_vehicle_key') THEN
        ALTER TABLE delivery_rates
            ADD CONSTRAINT delivery_rates_store_vehicle_key
            UNIQUE (store_id, vehicle_type);
    END IF;
END $$;

-- Ensure only one global default per vehicle type (store_id IS NULL rows)
CREATE UNIQUE INDEX IF NOT EXISTS delivery_rates_global_default
    ON delivery_rates (vehicle_type)
    WHERE store_id IS NULL;

COMMENT ON COLUMN delivery_rates.store_id IS
    'NULL = global default. Set to a store ID to override rates for that store only.';

COMMENT ON COLUMN delivery_rates.base_fee IS
    'Fixed base delivery fee in the store''s own currency.';

COMMENT ON COLUMN delivery_rates.per_km IS
    'Per-kilometre rate in the store''s own currency.';