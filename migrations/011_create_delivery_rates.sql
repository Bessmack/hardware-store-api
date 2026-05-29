-- ── Delivery Rates ────────────────────────────────────────────────────────────
-- Rates are stored in the database so the superadmin or admin can update them
-- via the dashboard without redeploying the application.
--
-- max_weight_kg: maximum cargo weight for this vehicle type
--   NULL on truck = no weight limit
-- max_radius_km: maximum delivery distance from the store
--   NULL on truck = no distance limit

CREATE TABLE delivery_rates (
    vehicle_type   VARCHAR(20)   PRIMARY KEY
                                 CHECK (vehicle_type IN ('bike', 'pickup', 'mini-truck','truck', 'prime-mover')),
    base_fee_kes   DECIMAL(10,2) NOT NULL CHECK (base_fee_kes >= 0),
    per_km_kes     DECIMAL(10,2) NOT NULL CHECK (per_km_kes >= 0),
    max_weight_kg  DECIMAL(8,2),
    max_radius_km  DECIMAL(6,2), -- NULL = no limit (truck)
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_by     UUID          REFERENCES users(id)
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON delivery_rates
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- ── Initial rates ─────────────────────────────────────────────────────────────
-- Adjust these to match your actual operational costs before going live.
-- Superadmin can update them from the dashboard at any time.

INSERT INTO delivery_rates (vehicle_type, base_fee_kes, per_km_kes, max_weight_kg, max_radius_km)
VALUES
    ('bike',  150.00,   30.00,   130.00,  15.00),
    ('pickup', 600.00, 120.00,  300.00,  80.00),
    ('mini-truck', 1000.00, 190.00,   5000.00, NULL),
    ('truck', 2000.00, 250.00, 1100.00, NULL),
    ('prime-mover', 2500.00, 300.00,   26000.00, NULL);