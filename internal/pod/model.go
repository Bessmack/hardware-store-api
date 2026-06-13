package pod

import "time"

// ── Core models ───────────────────────────────────────────────────────────────

// ProofOfDelivery is one row from the proof_of_delivery table.
// Created when an order is dispatched; completed when the delivery person
// submits the OTP, GPS, and photo at the delivery address.
type ProofOfDelivery struct {
	ID          string     `db:"id"`
	OrderID     string     `db:"order_id"`

	// OTP layer — customer receives this via WhatsApp when order is dispatched
	OTP         string     `db:"otp"`          // plaintext in DB (short-lived, not a password)
	OTPVerified bool       `db:"otp_verified"`

	// GPS layer — delivery person's coordinates at the moment of submission
	DeliveryLat float64    `db:"delivery_lat"`
	DeliveryLng float64    `db:"delivery_lng"`
	DistanceM   float64    `db:"distance_m"`   // computed at submission time

	// Photo layer — uploaded to Cloudinary, URL stored here
	PhotoURL    string     `db:"photo_url"`
	PhotoPublicID string   `db:"photo_public_id"` // Cloudinary public_id for deletion

	DeliveredAt *time.Time `db:"delivered_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

// Dispute is raised by a customer after delivery within the dispute window.
type Dispute struct {
	ID          string     `db:"id"`
	OrderID     string     `db:"order_id"`
	CustomerID  string     `db:"customer_id"`
	Description string     `db:"description"`
	EvidenceURL string     `db:"evidence_url"`    // optional Cloudinary URL
	EvidencePublicID string `db:"evidence_public_id"`
	Status      string     `db:"status"`          // "open" | "resolved" | "rejected"
	Resolution  string     `db:"resolution"`      // staff notes on outcome
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// ── Request types ─────────────────────────────────────────────────────────────

// SubmitPODRequest is sent by the delivery person when they arrive.
// All three layers must pass for the delivery to be confirmed.
type SubmitPODRequest struct {
	OrderID string  `json:"order_id" validate:"required"`
	OTP     string  `json:"otp"      validate:"required"`
	Lat     float64 `json:"lat"      validate:"required"`
	Lng     float64 `json:"lng"      validate:"required"`
	// Photo is uploaded as multipart/form-data — not in this struct.
	// The handler extracts it from r.FormFile("photo").
}

// RaiseDisputeRequest is sent by a customer after delivery.
type RaiseDisputeRequest struct {
	Description string `json:"description" validate:"required,min=10"`
	// Evidence photo is uploaded as multipart/form-data — optional.
}

// ResolveDisputeRequest is used by staff to close a dispute.
type ResolveDisputeRequest struct {
	Status     string `json:"status"     validate:"required,oneof=resolved rejected"`
	Resolution string `json:"resolution" validate:"required"`
}

// ── Response types ────────────────────────────────────────────────────────────

// PODResponse is what staff see for a proof of delivery record.
type PODResponse struct {
	OrderID     string     `json:"order_id"`
	OTPVerified bool       `json:"otp_verified"`
	DeliveryLat float64    `json:"delivery_lat,omitempty"`
	DeliveryLng float64    `json:"delivery_lng,omitempty"`
	DistanceM   float64    `json:"distance_m,omitempty"`
	PhotoURL    string     `json:"photo_url,omitempty"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

// DisputeResponse is the customer-facing view of their dispute.
type DisputeResponse struct {
	ID          string    `json:"id"`
	OrderID     string    `json:"order_id"`
	Description string    `json:"description"`
	EvidenceURL string    `json:"evidence_url,omitempty"`
	Status      string    `json:"status"`
	Resolution  string    `json:"resolution,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// SubmitPODResponse is returned to the delivery person after a successful submission.
type SubmitPODResponse struct {
	Message     string  `json:"message"`
	DistanceM   float64 `json:"distance_m"`
	OrderRef    string  `json:"order_ref"`
}