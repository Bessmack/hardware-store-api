-- ── Staff Store Assignments ───────────────────────────────────────────────────
-- Links cashiers and admins to exactly one store each.
-- Customers and superadmin have NO row here — they are not store-scoped.
--
-- Rules enforced here:
--   - A staff member can only be assigned to one store at a time.
--     (The PRIMARY KEY on user_id alone enforces this — not a composite key.)
--   - Deleting a user or store cascades to remove the assignment automatically.

CREATE TABLE staff_store_assignments (
    user_id     UUID        NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    store_id    UUID        NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by UUID        REFERENCES users(id),          -- superadmin who made the assignment

    -- user_id is the sole primary key — one staff member, one store, always.
    PRIMARY KEY (user_id)
);

-- Fast lookup of all staff at a given store
CREATE INDEX idx_staff_assignments_store ON staff_store_assignments (store_id);