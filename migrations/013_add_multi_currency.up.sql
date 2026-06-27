-- ── Migration 013: Multi-currency support ────────────────────────────────────
-- Adds currency to stores and renames price columns

-- 1. Currency on stores
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

-- 2. Rename _kes price columns (idempotent)
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='store_inventory' AND column_name='price_kes') THEN
        ALTER TABLE store_inventory RENAME COLUMN price_kes TO price;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='inventory_price_history' AND column_name='old_price_kes') THEN
        ALTER TABLE inventory_price_history RENAME COLUMN old_price_kes TO old_price;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='inventory_price_history' AND column_name='new_price_kes') THEN
        ALTER TABLE inventory_price_history RENAME COLUMN new_price_kes TO new_price;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='cart_items' AND column_name='unit_price_kes') THEN
        ALTER TABLE cart_items RENAME COLUMN unit_price_kes TO unit_price;
    END IF;
    
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

-- 3. Add store_id to delivery_rates
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='delivery_rates' AND column_name='store_id') THEN
        ALTER TABLE delivery_rates ADD COLUMN store_id UUID REFERENCES stores(id) ON DELETE CASCADE;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='delivery_rates' AND column_name='base_fee_kes') THEN
        ALTER TABLE delivery_rates RENAME COLUMN base_fee_kes TO base_fee;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='delivery_rates' AND column_name='per_km_kes') THEN
        ALTER TABLE delivery_rates RENAME COLUMN per_km_kes TO per_km;
    END IF;
END $$;