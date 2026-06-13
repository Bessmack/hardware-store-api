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
