package stores

import "time"

// ── Core model ────────────────────────────────────────────────────────────────

// Store mirrors the stores table exactly, including sensitive payment credentials.
// Never serialise this struct directly — use ToPublicResponse or ToStaffResponse.
type Store struct {
	ID               string    `db:"id"`
	Name             string    `db:"name"`
	Address          string    `db:"address"`
	County           string    `db:"county"`
	Latitude         float64   `db:"latitude"`
	Longitude        float64   `db:"longitude"`
	Phone            string    `db:"phone"`
	Email            string    `db:"email"`
	Currency         string    `db:"currency"`          // ISO 4217 e.g. "KES", "USD", "TZS"
	MpesaPaybill     string    `db:"mpesa_paybill"`
	MpesaAccountRef  string    `db:"mpesa_account_ref"`
	MpesaShortcode   string    `db:"mpesa_shortcode"`
	MpesaPasskey     string    `db:"mpesa_passkey"` // NEVER included in any response
	MpesaConsumerKey    string    `db:"mpesa_consumer_key"`
	MpesaConsumerSecret string    `db:"mpesa_consumer_secret"` // NEVER returned in any API response
	AirtelMerchantID string    `db:"airtel_merchant_id"`
	IsActive         bool      `db:"is_active"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

// ── Request types ─────────────────────────────────────────────────────────────

// CreateStoreRequest is used by superadmin to register a new branch.
type CreateStoreRequest struct {
	Name      string  `json:"name"      validate:"required"`
	Address   string  `json:"address"   validate:"required"`
	County    string  `json:"county"    validate:"required"`
	Latitude  float64 `json:"latitude"  validate:"required"`
	Longitude float64 `json:"longitude" validate:"required"`
	Phone     string  `json:"phone"`
	Email     string  `json:"email"     validate:"omitempty,email"`
	// Currency is the ISO 4217 code for all prices at this store. Defaults to KES.
	Currency string `json:"currency" validate:"omitempty,len=3"`
}

// UpdateStoreRequest is used by superadmin to update basic store information.
// Payment credentials are updated separately via UpdateCredentialsRequest.
type UpdateStoreRequest struct {
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	County    string  `json:"county"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Phone     string  `json:"phone"`
	Email     string  `json:"email"     validate:"omitempty,email"`
	Currency  string  `json:"currency"  validate:"omitempty,len=3"`
}

// UpdateCredentialsRequest is used by superadmin to configure a store's
// M-Pesa and Airtel payment credentials. Handled separately from store info
// because credentials are sensitive and require extra care.
type UpdateCredentialsRequest struct {
	MpesaPaybill     string `json:"mpesa_paybill"`
	MpesaAccountRef  string `json:"mpesa_account_ref"`
	MpesaShortcode   string `json:"mpesa_shortcode"`
	MpesaPasskey     string `json:"mpesa_passkey"`
	MpesaConsumerKey    string `json:"mpesa_consumer_key"`
	MpesaConsumerSecret string `json:"mpesa_consumer_secret"`
	AirtelMerchantID string `json:"airtel_merchant_id"`
}

// ── Response types ────────────────────────────────────────────────────────────

// StorePublicResponse is what customers and guests see.
// Only operational info — zero payment credentials exposed.
type StorePublicResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Address   string  `json:"address,omitempty"`
	County    string  `json:"county,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Phone     string  `json:"phone,omitempty"`
	Currency  string  `json:"currency"`
}

// StoreStaffResponse is what admins and superadmin see.
// Includes operational payment info (paybill, shortcode) but
// NEVER the passkey — that never leaves the server.
type StoreStaffResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Address          string    `json:"address,omitempty"`
	County           string    `json:"county,omitempty"`
	Latitude         float64   `json:"latitude"`
	Longitude        float64   `json:"longitude"`
	Phone            string    `json:"phone,omitempty"`
	Email            string    `json:"email,omitempty"`
	MpesaPaybill     string    `json:"mpesa_paybill,omitempty"`
	MpesaAccountRef  string    `json:"mpesa_account_ref,omitempty"`
	MpesaShortcode   string    `json:"mpesa_shortcode,omitempty"`
	// MpesaPasskey intentionally absent — never serialized
	AirtelMerchantID string    `json:"airtel_merchant_id,omitempty"`
	Currency         string    `json:"currency"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ── Mappers ───────────────────────────────────────────────────────────────────

// ToPublicResponse converts a Store to the safe customer-facing view.
func ToPublicResponse(s *Store) StorePublicResponse {
	return StorePublicResponse{
		ID:        s.ID,
		Name:      s.Name,
		Address:   s.Address,
		County:    s.County,
		Latitude:  s.Latitude,
		Longitude: s.Longitude,
		Phone:     s.Phone,
		Currency:  s.Currency,
	}
}

// ToStaffResponse converts a Store to the staff/admin view.
// Passkey is never included regardless of who calls this.
func ToStaffResponse(s *Store) StoreStaffResponse {
	return StoreStaffResponse{
		ID:               s.ID,
		Name:             s.Name,
		Address:          s.Address,
		County:           s.County,
		Latitude:         s.Latitude,
		Longitude:        s.Longitude,
		Phone:            s.Phone,
		Email:            s.Email,
		MpesaPaybill:     s.MpesaPaybill,
		MpesaAccountRef:  s.MpesaAccountRef,
		MpesaShortcode:   s.MpesaShortcode,
		AirtelMerchantID: s.AirtelMerchantID,
		Currency:         s.Currency,
		IsActive:         s.IsActive,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}