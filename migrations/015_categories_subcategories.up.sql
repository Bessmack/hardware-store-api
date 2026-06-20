-- ── Categories ────────────────────────────────────────────────────────────────
-- Top-level product groupings shown on the storefront (Electrical, Plumbing…).
-- icon stores the Lucide icon component name so the frontend can render it
-- without a separate icon lookup.

CREATE TABLE IF NOT EXISTS categories (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL UNIQUE,
    icon       TEXT        NOT NULL DEFAULT '',
    sort_order INT         NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Subcategories ─────────────────────────────────────────────────────────────
-- Second-level classification within a category (Electrical → Sockets).
-- slug is unique per category, not globally.

CREATE TABLE IF NOT EXISTS subcategories (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID        NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL,
    sort_order  INT         NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(category_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_subcategories_category_id ON subcategories(category_id);

-- ── Products — add subcategory FK ─────────────────────────────────────────────
-- The old category TEXT column is kept for backward compatibility; new products
-- should always set subcategory_id. Both can coexist during migration.

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS subcategory_id UUID REFERENCES subcategories(id);

CREATE INDEX IF NOT EXISTS idx_products_subcategory_id ON products(subcategory_id);

-- ── Seed data ─────────────────────────────────────────────────────────────────

INSERT INTO categories (name, slug, icon, sort_order) VALUES
    ('Electrical',          'electrical', 'Zap',         1),
    ('Plumbing',            'plumbing',   'Droplets',    2),
    ('Tools',               'tools',      'Wrench',      3),
    ('Paint & Finishes',    'paint',      'Paintbrush',  4),
    ('Roofing',             'roofing',    'Layers',      5),
    ('Safety Gear',         'safety',     'ShieldCheck', 6),
    ('Building Materials',  'building',   'Building2',   7),
    ('Tiles & Flooring',    'tiles',      'LayoutGrid',  8)
ON CONFLICT (slug) DO NOTHING;

-- Electrical
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Showers & Water Heaters','showers',1           FROM categories WHERE slug='electrical'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Sockets & Switches','sockets',2                FROM categories WHERE slug='electrical'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Circuit Breakers & MCBs','circuit-breakers',3  FROM categories WHERE slug='electrical'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Cables & Wiring','cables',4                    FROM categories WHERE slug='electrical'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Lighting','lighting',5                         FROM categories WHERE slug='electrical'
ON CONFLICT (category_id,slug) DO NOTHING;

-- Plumbing
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Pipes & Fittings','pipes',1      FROM categories WHERE slug='plumbing'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Tanks & Cisterns','tanks',2      FROM categories WHERE slug='plumbing'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Taps & Valves','taps',3          FROM categories WHERE slug='plumbing'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Drainage','drainage',4           FROM categories WHERE slug='plumbing'
ON CONFLICT (category_id,slug) DO NOTHING;

-- Tools
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Hand Tools','hand-tools',1           FROM categories WHERE slug='tools'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Power Tools','power-tools',2         FROM categories WHERE slug='tools'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Measuring & Levelling','measuring',3 FROM categories WHERE slug='tools'
ON CONFLICT (category_id,slug) DO NOTHING;

-- Paint
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Interior Paint','interior',1         FROM categories WHERE slug='paint'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Exterior Paint','exterior',2         FROM categories WHERE slug='paint'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Varnishes & Stains','varnish',3      FROM categories WHERE slug='paint'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Primers & Undercoats','primers',4    FROM categories WHERE slug='paint'
ON CONFLICT (category_id,slug) DO NOTHING;

-- Roofing
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Mabati Sheets','sheets',1                FROM categories WHERE slug='roofing'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Ridge Caps','ridge-caps',2               FROM categories WHERE slug='roofing'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Roofing Nails & Screws','fixings',3      FROM categories WHERE slug='roofing'
ON CONFLICT (category_id,slug) DO NOTHING;

-- Safety
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Helmets & Hard Hats','helmets',1  FROM categories WHERE slug='safety'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Gloves','gloves',2                FROM categories WHERE slug='safety'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Safety Boots','boots',3           FROM categories WHERE slug='safety'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Eye Protection','goggles',4       FROM categories WHERE slug='safety'
ON CONFLICT (category_id,slug) DO NOTHING;

-- Building Materials
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Cement & Concrete','cement',1    FROM categories WHERE slug='building'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Sand & Aggregates','aggregates',2 FROM categories WHERE slug='building'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Steel & Iron','steel',3          FROM categories WHERE slug='building'
ON CONFLICT (category_id,slug) DO NOTHING;

-- Tiles & Flooring
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Ceramic Tiles','ceramic',1           FROM categories WHERE slug='tiles'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Vinyl Flooring','vinyl',2            FROM categories WHERE slug='tiles'
ON CONFLICT (category_id,slug) DO NOTHING;
INSERT INTO subcategories (category_id, name, slug, sort_order)
SELECT id,'Tile Adhesive & Grout','adhesive',3  FROM categories WHERE slug='tiles'
ON CONFLICT (category_id,slug) DO NOTHING;