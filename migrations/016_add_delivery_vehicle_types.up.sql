-- ── Migration 016: Delivery vehicle types management ──────────────────────
-- Superadmin can manage vehicle types; stores use them for delivery options.

-- 1. Create vehicle types table (superadmin managed)
CREATE TABLE IF NOT EXISTS delivery_vehicle_types (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(50)  NOT NULL UNIQUE,  -- 'bike', 'pickup', 'mini-truck'
    label           VARCHAR(100) NOT NULL,          -- 'Motorcycle', 'Pickup Truck'
    icon            VARCHAR(50)  NOT NULL DEFAULT 'Truck', -- Lucide icon name
    max_weight_kg   DECIMAL(8,2),                    -- NULL = no limit
    default_base_fee DECIMAL(10,2) NOT NULL DEFAULT 0,
    default_per_km  DECIMAL(10,2) NOT NULL DEFAULT 0,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order      INT          NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 2. Add vehicle_type_id to delivery_rates instead of string
ALTER TABLE delivery_rates 
    ADD COLUMN vehicle_type_id UUID REFERENCES delivery_vehicle_types(id);

-- 3. Migrate existing data (if any)
-- Copy vehicle_type string values to the new table
INSERT INTO delivery_vehicle_types (name, label, max_weight_kg, default_base_fee, default_per_km, sort_order)
VALUES 
    ('bike', 'Motorcycle', 130.00, 100.00, 60.00, 1),
    ('pickup', 'Pickup Truck', 2000.00, 600.00, 380.00, 2),
    ('mini-truck', 'Mini Truck', 5000.00, 1000.00, 700.00, 3),
    ('truck', 'Truck', 11000.00, 1700.00, 1100.00, 4),
    ('prime-mover', 'Prime Mover', 26000.00, 8500.00, 4500.00, 5)
ON CONFLICT (name) DO NOTHING;

-- 4. Update delivery_rates to use UUID
UPDATE delivery_rates dr 
SET vehicle_type_id = (
    SELECT id FROM delivery_vehicle_types dvt WHERE dvt.name = dr.vehicle_type
);

-- 5. Make vehicle_type_id NOT NULL after migration
ALTER TABLE delivery_rates ALTER COLUMN vehicle_type_id SET NOT NULL;

-- 6. Drop the old vehicle_type column
ALTER TABLE delivery_rates DROP COLUMN vehicle_type;

-- 7. Update the unique constraint
ALTER TABLE delivery_rates DROP CONSTRAINT IF EXISTS delivery_rates_store_vehicle_key;
ALTER TABLE delivery_rates ADD CONSTRAINT delivery_rates_store_vehicle_key 
    UNIQUE (store_id, vehicle_type_id);

-- 8. Update the global default index
DROP INDEX IF EXISTS delivery_rates_global_default;
CREATE UNIQUE INDEX delivery_rates_global_default 
    ON delivery_rates (vehicle_type_id) WHERE store_id IS NULL;

-- 9. Auto-update trigger for vehicle types
DROP TRIGGER IF EXISTS set_updated_at ON delivery_vehicle_types;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON delivery_vehicle_types
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();