-- ── Products ──────────────────────────────────────────────────────────────────
-- Products hold universal information only — name, description, dimensions.
-- Price is NOT stored here. Each store sets its own price in store_inventory.
--
-- constraint_type controls how the vehicle type for delivery is determined:
--   'weight'    → vehicle decided by total order weight (most products)
--   'dimension' → vehicle hard-set regardless of quantity (e.g. iron sheets, long pipes)
--   'hazardous' → always van minimum regardless of weight (e.g. gas cylinders, chemicals)
--
-- min_vehicle_type is only set for 'dimension' and 'hazardous' items.
-- For 'weight' items it is NULL — the vehicle is calculated from the total cart weight.

CREATE TABLE products (
    id               UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name             VARCHAR(255) NOT NULL,
    description      TEXT,
    category         VARCHAR(100),

    -- Physical properties — used for vehicle type determination
    weight_kg        DECIMAL(8,2),
    length_cm        DECIMAL(8,2),
    width_cm         DECIMAL(8,2),
    height_cm        DECIMAL(8,2),

    -- Delivery constraint
    constraint_type  VARCHAR(20)  NOT NULL DEFAULT 'weight'
                                  CHECK (constraint_type IN ('weight', 'dimension', 'hazardous')),

    -- Only populated for dimension/hazardous items
    min_vehicle_type VARCHAR(20)  CHECK (min_vehicle_type IN ('bike', 'pickup', 'mini-truck', 'truck', 'prime-mover')),

    -- Array of Cloudinary public IDs (not full URLs — URLs are constructed at serve time)
    images           TEXT[]       NOT NULL DEFAULT '{}',

    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by       UUID         REFERENCES users(id)
);

CREATE INDEX idx_products_category ON products (category);
CREATE INDEX idx_products_active   ON products (is_active);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();