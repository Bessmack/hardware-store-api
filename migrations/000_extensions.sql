-- ── Extensions ────────────────────────────────────────────────────────────────
-- Run this file first. Everything else depends on uuid-ossp.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ── Shared updated_at trigger ─────────────────────────────────────────────────
-- Attach this trigger to any table that has an updated_at column.
-- It automatically sets updated_at = NOW() on every UPDATE — no app code needed.
--
-- Usage (add after each table creation):
--   CREATE TRIGGER set_updated_at
--       BEFORE UPDATE ON <table_name>
--       FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;