-- ── Customer Addresses ────────────────────────────────────────────────────────
-- Customers can save multiple delivery addresses (Home, Office, Site etc.)
-- and reuse them at checkout without re-entering or re-geocoding.
--
-- lat/lng are captured once (via GPS, Maps autocomplete, or manual geocoding)
-- and stored permanently so delivery fee calculation never needs to re-geocode.

CREATE TABLE customer_addresses (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id  UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Human-friendly label the customer assigns: "Home", "Office", "Nairobi Site"
    label        VARCHAR(50),

    address_text TEXT         NOT NULL,       -- full formatted address string
    latitude     DECIMAL(9,6) NOT NULL,
    longitude    DECIMAL(9,6) NOT NULL,

    is_default   BOOLEAN      NOT NULL DEFAULT FALSE,

    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- A customer can only have one default address at a time
CREATE UNIQUE INDEX one_default_address
    ON customer_addresses (customer_id)
    WHERE is_default = TRUE;

CREATE INDEX idx_addresses_customer ON customer_addresses (customer_id);