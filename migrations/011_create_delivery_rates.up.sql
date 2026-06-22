-- ── Delivery Rates ────────────────────────────────────────────────────────────
-- Rates are stored in the database so the superadmin or admin can update them
-- via the dashboard without redeploying the application.
--
-- max_weight_kg: maximum cargo weight for this vehicle type
--   NULL on truck = no weight limit
--   NULL on prime-mover = no weight limit
-- max_radius_km: maximum delivery distance from the store
--   NULL on truck = no distance limit
--   NULL on prime-mover = no distance limit

CREATE TABLE IF NOT EXISTS delivery_rates (
    vehicle_type   VARCHAR(20)   PRIMARY KEY
                                 CHECK (vehicle_type IN ('bike', 'pickup', 'mini-truck','truck', 'prime-mover')),
    base_fee       DECIMAL(10,2) NOT NULL CHECK (base_fee >= 0),
    per_km         DECIMAL(10,2) NOT NULL CHECK (per_km >= 0),
    max_weight_kg  DECIMAL(8,2),               -- NULL = no limit
    max_radius_km  DECIMAL(6,2),               -- NULL = no limit
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_by     UUID          REFERENCES users(id)
);

-- Drop trigger if it exists (idempotent)
DROP TRIGGER IF EXISTS set_updated_at ON delivery_rates;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON delivery_rates
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- ── Initial rates ─────────────────────────────────────────────────────────────
-- Rates reflect realistic Kenyan market costs.
-- Superadmin can adjust any of these via the dashboard without redeploying.
-- Prime-mover has no max_radius_km (long-haul) but has a 5km MINIMUM — see delivery service.
-- Using DO block to check existence before inserting (idempotent)

DO $$
BEGIN
    -- Only insert if the table is empty
    IF NOT EXISTS (SELECT 1 FROM delivery_rates LIMIT 1) THEN
        INSERT INTO delivery_rates (vehicle_type, base_fee, per_km, max_weight_kg, max_radius_km)
        VALUES
            ('bike', 60.00,  60.00,   130.00, 20.00),
            ('pickup', 500.00, 380.00,  2000.00, 120.00),
            ('mini-truck', 1000.00, 700.00,  5000.00, 690.00),
            ('truck', 1500.00, 1000.00, 11000.00, 690.00),
            ('prime-mover', 4500.00, 2500.00, 26000.00, NULL);
    END IF;
END $$;