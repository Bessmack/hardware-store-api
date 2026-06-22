-- ── Stores ────────────────────────────────────────────────────────────────────
-- Each store has its own M-Pesa shortcode and passkey so payments land
-- directly in that store's M-Pesa account — not a central account.
-- The global Daraja consumer key/secret (for generating access tokens)
-- lives in the .env file, not here.

CREATE TABLE IF NOT EXISTS stores (
    id                  UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                VARCHAR(100) NOT NULL,
    address             TEXT,
    county              VARCHAR(100),
    latitude            DECIMAL(9,6) NOT NULL,
    longitude           DECIMAL(9,6) NOT NULL,
    phone               VARCHAR(20),
    email               VARCHAR(255),

    -- ── M-Pesa credentials (per store) ───────────────────────────────────────
    mpesa_paybill       VARCHAR(20),
    mpesa_account_ref   VARCHAR(50),
    mpesa_shortcode     VARCHAR(20),
    mpesa_passkey       TEXT,

    -- ── Airtel Money credentials (per store) ─────────────────────────────────
    airtel_merchant_id  VARCHAR(50),

    is_active           BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Only add constraint if it doesn't already exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'unique_mpesa_paybill' 
        AND conrelid = 'stores'::regclass
    ) THEN
        ALTER TABLE stores ADD CONSTRAINT unique_mpesa_paybill UNIQUE (mpesa_paybill);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_stores_county  ON stores (county);
CREATE INDEX IF NOT EXISTS idx_stores_active  ON stores (is_active);

-- Drop trigger if it exists (idempotent)
DROP TRIGGER IF EXISTS set_updated_at ON stores;
CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON stores
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();