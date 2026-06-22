-- ── Wishlists ─────────────────────────────────────────────────────────────────
-- Customers (not staff) can maintain named wishlists for future purchases.
-- Hardware customers often run multiple projects simultaneously
-- (e.g. "Kitchen Renovation", "Perimeter Wall", "Office Fit-Out").
--
-- Price is NOT stored here. When the customer views their wishlist,
-- current prices are fetched live from their nearest store's inventory.

CREATE TABLE IF NOT EXISTS wishlists (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL DEFAULT 'My Wishlist',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- A customer cannot have two wishlists with the same name
    CONSTRAINT unique_customer_wishlist UNIQUE (customer_id, name)
);

-- ── Wishlist Items ────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS wishlist_items (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    wishlist_id UUID        NOT NULL REFERENCES wishlists(id)  ON DELETE CASCADE,
    product_id  UUID        NOT NULL REFERENCES products(id)   ON DELETE CASCADE,

    -- Optional personal note from the customer
    -- e.g. "Need 20 bags for the perimeter wall", "Check price before buying"
    note        TEXT,

    added_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A product can only appear once per wishlist
    CONSTRAINT unique_wishlist_product UNIQUE (wishlist_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_wishlists_customer       ON wishlists (customer_id);
CREATE INDEX IF NOT EXISTS idx_wishlist_items_wishlist  ON wishlist_items (wishlist_id);
CREATE INDEX IF NOT EXISTS idx_wishlist_items_product   ON wishlist_items (product_id);