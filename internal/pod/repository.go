package pod

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound        = errors.New("proof of delivery record not found")
	ErrDisputeNotFound = errors.New("dispute not found")
	ErrAlreadyExists   = errors.New("a proof of delivery record already exists for this order")
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// ── Proof of delivery ─────────────────────────────────────────────────────────

// Create inserts a new POD record when an order is dispatched.
// Called by the service when the order status transitions to out_for_delivery.
func (r *Repository) Create(ctx context.Context, orderID, otp string) (*ProofOfDelivery, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO proof_of_delivery (order_id, otp)
		VALUES ($1, $2)
		RETURNING id, order_id, otp, otp_verified,
		          COALESCE(delivery_lat, 0), COALESCE(delivery_lng, 0),
		          COALESCE(distance_m, 0),
		          COALESCE(photo_url, ''), COALESCE(photo_public_id, ''),
		          delivered_at, created_at
	`, orderID, otp)

	return scanPOD(row)
}

// GetByOrderID fetches the POD record for a given order.
func (r *Repository) GetByOrderID(ctx context.Context, orderID string) (*ProofOfDelivery, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, order_id, otp, otp_verified,
		       COALESCE(delivery_lat, 0), COALESCE(delivery_lng, 0),
		       COALESCE(distance_m, 0),
		       COALESCE(photo_url, ''), COALESCE(photo_public_id, ''),
		       delivered_at, created_at
		FROM proof_of_delivery WHERE order_id = $1
	`, orderID)

	pod, err := scanPOD(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return pod, err
}

// Submit records the delivery person's OTP verification, GPS, and photo.
// Called after all three layers have been validated in the service.
func (r *Repository) Submit(ctx context.Context, orderID, photoURL, photoPublicID string, lat, lng, distanceM float64) error {
	now := time.Now()
	result, err := r.db.Pool.Exec(ctx, `
		UPDATE proof_of_delivery SET
			otp_verified    = TRUE,
			delivery_lat    = $1,
			delivery_lng    = $2,
			distance_m      = $3,
			photo_url       = $4,
			photo_public_id = $5,
			delivered_at    = $6
		WHERE order_id = $7
	`, lat, lng, distanceM, photoURL, photoPublicID, now, orderID)
	if err != nil {
		return fmt.Errorf("pod: failed to submit POD: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Disputes ──────────────────────────────────────────────────────────────────

// CreateDispute inserts a new dispute record.
func (r *Repository) CreateDispute(ctx context.Context, orderID, customerID, description, evidenceURL, evidencePublicID string) (*Dispute, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO disputes
			(order_id, customer_id, description, evidence_url, evidence_public_id, status)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), 'open')
		RETURNING id, order_id, customer_id, description,
		          COALESCE(evidence_url, ''), COALESCE(evidence_public_id, ''),
		          status, COALESCE(resolution, ''),
		          created_at, updated_at
	`, orderID, customerID, description, evidenceURL, evidencePublicID)

	return scanDispute(row)
}

// GetDisputeByOrderID returns the open dispute for an order, if any.
func (r *Repository) GetDisputeByOrderID(ctx context.Context, orderID string) (*Dispute, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, order_id, customer_id, description,
		       COALESCE(evidence_url, ''), COALESCE(evidence_public_id, ''),
		       status, COALESCE(resolution, ''),
		       created_at, updated_at
		FROM disputes WHERE order_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, orderID)

	d, err := scanDispute(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDisputeNotFound
	}
	return d, err
}

// ResolveDispute updates a dispute's status and adds a resolution note.
func (r *Repository) ResolveDispute(ctx context.Context, disputeID, status, resolution string) error {
	result, err := r.db.Pool.Exec(ctx, `
		UPDATE disputes
		SET status = $1, resolution = $2
		WHERE id = $3
	`, status, resolution, disputeID)
	if err != nil {
		return fmt.Errorf("pod: failed to resolve dispute: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrDisputeNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func scanPOD(row pgx.Row) (*ProofOfDelivery, error) {
	var p ProofOfDelivery
	if err := row.Scan(
		&p.ID, &p.OrderID, &p.OTP, &p.OTPVerified,
		&p.DeliveryLat, &p.DeliveryLng, &p.DistanceM,
		&p.PhotoURL, &p.PhotoPublicID,
		&p.DeliveredAt, &p.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func scanDispute(row pgx.Row) (*Dispute, error) {
	var d Dispute
	if err := row.Scan(
		&d.ID, &d.OrderID, &d.CustomerID, &d.Description,
		&d.EvidenceURL, &d.EvidencePublicID,
		&d.Status, &d.Resolution,
		&d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &d, nil
}