-- ── Proof of Delivery ─────────────────────────────────────────────────────────
-- Three-layer verification before an order can be marked as delivered:
--   1. OTP    — customer provides a code sent to them via WhatsApp
--   2. Photo  — delivery person photographs the goods at the door
--   3. GPS    — device coordinates must be within 200m of the delivery address
--
-- photo_public_id stores the Cloudinary public ID (not the full URL).
-- Full URL is constructed at serve time: storage.URL(photo_public_id).
-- Photos live in delivery-photos/ (auto-deleted after 30 days) until a
-- dispute is raised, at which point they are moved to dispute-evidence/.

CREATE TABLE proof_of_delivery (
    id                       UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- One POD record per order
    order_id                 UUID         NOT NULL REFERENCES orders(id) UNIQUE,
    delivery_person_id       UUID         NOT NULL REFERENCES users(id),

    -- ── Layer 1: OTP ─────────────────────────────────────────────────────────
    otp_code                 VARCHAR(10)  NOT NULL,
    otp_sent_at              TIMESTAMPTZ  NOT NULL,
    otp_verified_at          TIMESTAMPTZ,           -- NULL until customer provides code

    -- ── Layer 2: Photo ───────────────────────────────────────────────────────
    -- Cloudinary public ID — e.g. "delivery-photos/order-abc123"
    photo_public_id          TEXT,
    photo_taken_at           TIMESTAMPTZ,

    -- ── Layer 3: GPS ─────────────────────────────────────────────────────────
    submitted_lat            DECIMAL(9,6),
    submitted_lng            DECIMAL(9,6),
    distance_from_address_m  DECIMAL(8,2),          -- calculated at submission time

    -- ── Result ───────────────────────────────────────────────────────────────
    status                   VARCHAR(20)  NOT NULL DEFAULT 'pending'
                                          CHECK (status IN ('pending', 'verified', 'failed')),
    failure_reason           TEXT,                  -- which layer failed and why
    verified_at              TIMESTAMPTZ,

    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Disputes ──────────────────────────────────────────────────────────────────
-- Customers have DISPUTE_WINDOW_HOURS (default 24h) after delivery to raise
-- a dispute. When raised, the POD photo is automatically moved to
-- dispute-evidence/ in Cloudinary before the 30-day auto-delete removes it.
--
-- Support staff review the dispute alongside the full POD record
-- (OTP timestamp, photo, GPS coordinates) in one place.

CREATE TABLE disputes (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id    UUID        NOT NULL REFERENCES orders(id),
    customer_id UUID        NOT NULL REFERENCES users(id),
    reason      TEXT        NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'open'
                            CHECK (status IN ('open', 'resolved', 'rejected')),
    resolved_by UUID        REFERENCES users(id),    -- staff member who closed it
    resolved_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Only one active dispute per order
    CONSTRAINT unique_order_dispute UNIQUE (order_id)
);

CREATE INDEX idx_pod_order          ON proof_of_delivery (order_id);
CREATE INDEX idx_pod_status         ON proof_of_delivery (status);
CREATE INDEX idx_disputes_order     ON disputes (order_id);
CREATE INDEX idx_disputes_customer  ON disputes (customer_id);
CREATE INDEX idx_disputes_status    ON disputes (status);