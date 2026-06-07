-- ── Stores ────────────────────────────────────────────────────────────────────
-- Each store has its own M-Pesa shortcode and passkey so payments land
-- directly in that store's M-Pesa account — not a central account.
-- The global Daraja consumer key/secret (for generating access tokens)
-- lives in the .env file, not here.

CREATE TABLE stores (
    id                  UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                VARCHAR(100) NOT NULL,
    address             TEXT,
    county              VARCHAR(100),
    latitude            DECIMAL(9,6) NOT NULL,
    longitude           DECIMAL(9,6) NOT NULL,
    phone               VARCHAR(20),
    email               VARCHAR(255),

    -- ── M-Pesa credentials (per store) ───────────────────────────────────────
    -- mpesa_paybill:    the business number customers pay to (e.g. 522522)
    -- mpesa_account_ref: prefix for the account reference shown on STK push
    -- mpesa_shortcode:  Lipa Na M-Pesa shortcode for STK push
    -- mpesa_passkey:    Daraja passkey — treat like a password, keep secret
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

-- Two stores cannot share the same paybill number
ALTER TABLE stores
    ADD CONSTRAINT unique_mpesa_paybill UNIQUE (mpesa_paybill);

CREATE INDEX idx_stores_county  ON stores (county);
CREATE INDEX idx_stores_active  ON stores (is_active);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON stores
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();