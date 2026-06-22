-- ── Migration 014: Per-store M-Pesa consumer credentials ──────────────────
--   This keeps the migration history readable and rollback-safe.
--
-- WHAT this adds:
--   Two optional columns on the stores table for stores that have their own
--   Safaricom Daraja developer account (Scenario B).
--
-- HOW the M-Pesa provider uses these:
--   - If a store has mpesa_consumer_key + mpesa_consumer_secret → use them
--     to get an OAuth token specific to that store's Daraja app.
--   - If NULL → fall back to the global MPESA_CONSUMER_KEY / MPESA_CONSUMER_SECRET
--     from .env (the platform's shared Daraja app).
--
--
-- OAUTH TOKEN CACHING:
--   The Redis cache key is per-consumer-key so tokens from different Daraja
--   apps never overwrite each other:
--     mpesa:token:global          → shared platform token
--     mpesa:token:{storeID}       → store-specific token

-- Add columns if they don't exist
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='stores' AND column_name='mpesa_consumer_key') THEN
        ALTER TABLE stores ADD COLUMN mpesa_consumer_key TEXT;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='stores' AND column_name='mpesa_consumer_secret') THEN
        ALTER TABLE stores ADD COLUMN mpesa_consumer_secret TEXT;
    END IF;
END $$;

-- Note: The unique_mpesa_paybill constraint was already created in migration 002
-- No need to create it again here!

COMMENT ON COLUMN stores.mpesa_consumer_key IS
    'Optional. Set only when this store has its own Safaricom Daraja developer account. '
    'NULL means the platform-wide consumer key from .env is used.';

COMMENT ON COLUMN stores.mpesa_consumer_secret IS
    'Optional. Paired with mpesa_consumer_key. Never returned in any API response.';