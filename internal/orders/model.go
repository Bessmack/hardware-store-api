package orders

import "time"

// ── Order statuses ────────────────────────────────────────────────────────────

type OrderStatus string

const (
	StatusPlaced          OrderStatus = "placed"           // received, payment pending
	StatusConfirmed       OrderStatus = "confirmed"         // payment received
	StatusPreparing       OrderStatus = "preparing"         // staff packing items
	StatusOutForDelivery  OrderStatus = "out_for_delivery"  // rider dispatched
	StatusDelivered       OrderStatus = "delivered"         // POD complete
	StatusCancelled       OrderStatus = "cancelled"
)

// StatusDetails maps each status to the label and description shown to customers.
// The description is intentionally store-agnostic — the store name is injected
// at serve time where needed (e.g. "Being prepared at Kiambu Branch").
var StatusDetails = map[OrderStatus]struct {
	Label       string
	Description string
}{
	StatusPlaced:         {"Order Placed", "We have received your order. Awaiting payment confirmation."},
	StatusConfirmed:      {"Payment Confirmed", "Payment received. Your order is being prepared."},
	StatusPreparing:      {"Being Prepared", "Your items are being packed and made ready."},
	StatusOutForDelivery: {"Out for Delivery", "Your order is on its way to you."},
	StatusDelivered:      {"Delivered", "Your order has been delivered. Thank you for shopping with us!"},
	StatusCancelled:      {"Cancelled", "This order has been cancelled."},
}

// ValidTransitions defines which status changes are permitted and who can make them.
// Keyed by current status; values are the statuses that can follow.
var ValidTransitions = map[OrderStatus][]OrderStatus{
	StatusPlaced:         {StatusConfirmed, StatusCancelled},
	StatusConfirmed:      {StatusPreparing, StatusCancelled},
	StatusPreparing:      {StatusOutForDelivery, StatusCancelled},
	StatusOutForDelivery: {StatusDelivered}, // cannot cancel once rider is dispatched
	StatusDelivered:      {},                // terminal state
	StatusCancelled:      {},                // terminal state
}

// CanTransitionTo reports whether moving from current to next is a valid transition.
func (current OrderStatus) CanTransitionTo(next OrderStatus) bool {
	for _, allowed := range ValidTransitions[current] {
		if allowed == next {
			return true
		}
	}
	return false
}

type PaymentChannel string

// ── Core models ───────────────────────────────────────────────────────────────

type Order struct {
	ID                 string      `db:"id"`
	Reference          string      `db:"reference"`
	CustomerID         string      `db:"customer_id"`
	FulfillingStoreID  string      `db:"fulfilling_store_id"`

	DeliveryType       string      `db:"delivery_type"`      // "delivery" | "pickup"
	DeliveryAddressText string     `db:"delivery_address_text"`
	DeliveryLat        float64     `db:"delivery_lat"`
	DeliveryLng        float64     `db:"delivery_lng"`
	VehicleType        string      `db:"vehicle_type"`
	VehicleReason      string      `db:"vehicle_reason"`

	ItemsTotal         float64     `db:"items_total"`
	DeliveryFee        float64     `db:"delivery_fee"`
	GrandTotal         float64     `db:"grand_total"`
	Currency           string      `db:"currency"`

	PaymentProvider    string      `db:"payment_provider"`
	PaymentProviderRef string      `db:"payment_provider_ref"`
	PaymentStatus      string      `db:"payment_status"`
	PaidAt             *time.Time  `db:"paid_at"`

	Status             OrderStatus `db:"status"`
	CreatedAt          time.Time   `db:"created_at"`
	UpdatedAt          time.Time   `db:"updated_at"`
}

type OrderItem struct {
	ID          string  `db:"id"`
	OrderID     string  `db:"order_id"`
	ProductID   string  `db:"product_id"`
	ProductName string  `db:"product_name"` // snapshot — never changes
	Quantity    int     `db:"quantity"`
	UnitPrice   float64 `db:"unit_price"`   // locked at order time
	Subtotal    float64 `db:"subtotal"`
}

type OrderStatusHistory struct {
	ID        string      `db:"id"`
	OrderID   string      `db:"order_id"`
	Status    OrderStatus `db:"status"`
	Note      string      `db:"note"`       // internal only, never shown to customer
	ChangedBy string      `db:"changed_by"`
	CreatedAt time.Time   `db:"created_at"`
}

// ── Request types ─────────────────────────────────────────────────────────────

// PlaceOrderRequest is the body for POST /api/v1/orders.
// Delivery fee is NOT accepted from the client — it is recalculated
// server-side to prevent tampering.
type PlaceOrderRequest struct {
	// Which store will fulfil this order — resolved from cart items or geo routing
	StoreID         string `json:"store_id"         validate:"required"`

	DeliveryType    string `json:"delivery_type"    validate:"required,oneof=delivery pickup"`

	// Required when delivery_type = "delivery"
	DeliveryLat     float64 `json:"delivery_lat"`
	DeliveryLng     float64 `json:"delivery_lng"`
	DeliveryAddress string  `json:"delivery_address"`
	VehicleType     string  `json:"vehicle_type"`

	PaymentProvider string `json:"payment_provider" validate:"required,oneof=mpesa airtel card"`
	PaymentChannel PaymentChannel `json:"payment_channel"` // optional hint for hosted checkout pages
	Phone           string `json:"phone"` // required for mpesa / airtel
}

// UpdateStatusRequest is used by staff to move an order through its lifecycle.
type UpdateStatusRequest struct {
	Status OrderStatus `json:"status" validate:"required"`
	Note   string      `json:"note"`  // internal note, never shown to customer
}

// CancelOrderRequest is used by customers to cancel a placed order.
type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

// OrderFilters is used when listing orders for a store.
type OrderFilters struct {
	Status  string
	Page    int
	PerPage int
}

// ── Response types ────────────────────────────────────────────────────────────

// OrderItemResponse is one line in an order.
type OrderItemResponse struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Subtotal    float64 `json:"subtotal"`
	Currency    string  `json:"currency"`
}

// OrderResponse is the customer-facing view of an order.
type OrderResponse struct {
	ID                  string              `json:"id"`
	Reference           string              `json:"reference"`
	Status              OrderStatus         `json:"status"`
	StatusLabel         string              `json:"status_label"`
	StatusDescription   string              `json:"status_description"`
	DeliveryType        string              `json:"delivery_type"`
	DeliveryAddress     string              `json:"delivery_address,omitempty"`
	VehicleType         string              `json:"vehicle_type,omitempty"`
	Items               []OrderItemResponse `json:"items"`
	ItemsTotal          float64             `json:"items_total"`
	DeliveryFee         float64             `json:"delivery_fee"`
	GrandTotal          float64             `json:"grand_total"`
	Currency            string              `json:"currency"`
	PaymentProvider     string              `json:"payment_provider"`
	PaymentStatus       string              `json:"payment_status"`
	FulfillingStoreName string              `json:"fulfilling_store_name"`
	CreatedAt           time.Time           `json:"created_at"`
}

// TrackingEvent is one step in the order timeline shown to the customer.
type TrackingEvent struct {
	Status      OrderStatus `json:"status"`
	StatusLabel string      `json:"status_label"`
	Description string      `json:"description"`
	OccurredAt  time.Time   `json:"occurred_at"`
	IsCurrent   bool        `json:"is_current"`
}

// TrackingResponse is returned by GET /orders/:id/track.
type TrackingResponse struct {
	Reference   string          `json:"reference"`
	Status      OrderStatus     `json:"status"`
	StatusLabel string          `json:"status_label"`
	Timeline    []TrackingEvent `json:"timeline"`
}

// StaffOrderResponse extends OrderResponse with internal details for staff.
type StaffOrderResponse struct {
	OrderResponse
	CustomerPhone string               `json:"customer_phone"`
	CustomerEmail string               `json:"customer_email"`
	CustomerName  string               `json:"customer_name"`
	VehicleReason string               `json:"vehicle_reason,omitempty"`
	StatusHistory []StatusHistoryEntry `json:"status_history"`
}

// StatusHistoryEntry is one row of the status history shown to staff.
type StatusHistoryEntry struct {
	Status    OrderStatus `json:"status"`
	Label     string      `json:"label"`
	Note      string      `json:"note,omitempty"` // internal note
	ChangedAt time.Time   `json:"changed_at"`
}

// PaymentInitResponse is returned to the customer after PlaceOrder.
// For M-Pesa/Airtel: includes instructions to check their phone.
// For card: includes redirect URL.
type PlaceOrderResponse struct {
	Order               OrderResponse `json:"order"`
	PaymentInstructions string        `json:"payment_instructions"`
	// M-Pesa specific — tells the frontend the STK push is pending
	AwaitingPayment     bool          `json:"awaiting_payment"`
}