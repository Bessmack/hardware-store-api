package orders

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/logger"
)

// ── Interfaces (implemented by other domains) ─────────────────────────────────
// Defined here so orders stays decoupled from concrete packages.

// CartReader loads a customer's cart items for order placement.
// Implemented by cart.Repository.
type CartReader interface {
	GetItemsForOrder(ctx context.Context, customerID, sessionID string) ([]CartItemForOrder, error)
	ClearCart(ctx context.Context, customerID, sessionID string) error
}

// CartItemForOrder is the minimal cart item data needed to build an order.
type CartItemForOrder struct {
	ProductID   string
	ProductName string
	StoreID     string
	Quantity    int
	UnitPrice   float64 // already validated and current from cart validation
	Currency    string
	InStock     bool
}

// StockManager reduces and restores inventory when orders are placed or cancelled.
// Implemented by inventory.Repository.
type StockManager interface {
	ReduceStock(ctx context.Context, storeID, productID string, qty int) error
	RestoreStock(ctx context.Context, storeID, productID string, qty int) error
}

// DeliveryFeeCalculator computes the delivery fee server-side.
// Client-supplied fees are never trusted — always recalculated here.
// Implemented by delivery.Service.
type DeliveryFeeCalculator interface {
	CalculateFee(ctx context.Context, storeID string, lat, lng float64, vehicleType string) (fee float64, currency string, err error)
}

// PaymentInitiator triggers a payment request with the chosen provider.
// Implemented by payments.Service (built in the payments domain).
type PaymentInitiator interface {
	Initiate(ctx context.Context, req PaymentInitRequest) (*PaymentInitResult, error) // optional: returns instructions for mobile money payments, or redirect URL for card payments
}


type PaymentInitResult struct {
	ProviderRef     string // M-Pesa CheckoutRequestID etc.
	Instructions    string // "Check your phone for an M-Pesa prompt"
	AwaitingPayment bool   // true for mobile money (async), false for card (sync)
	RedirectURL     string // for card payments — frontend redirects customer here to complete payment on hosted checkout page
}

// StoreInfoReader fetches store metadata for reference generation and responses.
// Implemented by stores.Repository.
// Currency is the store's trading currency (e.g. "KES", "UGX") — all financial figures on an order use this currency, not the cart item's currency.
type StoreInfoReader interface {
	GetStoreInfo(ctx context.Context, storeID string) (name, county, currency string, err error)
}

// CustomerInfoReader fetches customer contact details for notifications.
// Implemented by users.Repository.
type CustomerInfoReader interface {
	GetCustomerInfo(ctx context.Context, customerID string) (name, phone, email string, err error)
}

// PODDispatcher is called when an order is dispatched for delivery.
// Implemented by pod.Service — defined here to break the circular import
// (pod imports orders for OrderReader; orders imports pod would be circular).
type PODDispatcher interface {
	Dispatch(ctx context.Context, orderID, customerID, customerPhone, customerName, orderRef string) error
}

// OrderNotifier sends order-related notifications via all channels.
// Implemented by notifications.Service.
type OrderNotifier interface {
	OrderConfirmed(phone, email, name, orderRef, storeName string, total float64)
	OrderStatusChanged(phone, email, name, orderRef, statusLabel, description string)
	OutForDelivery(phone, name, orderRef, otp string)
	OrderDelivered(phone, email, name, orderRef string)
}

// ── Service ───────────────────────────────────────────────────────────────────

type Service struct {
	repo      *Repository
	cart      CartReader
	stock     StockManager
	delivery  DeliveryFeeCalculator
	payment   PaymentInitiator
	pod       PODDispatcher // optional — set via SetPODDispatcher after NewService
	stores    StoreInfoReader
	customers CustomerInfoReader
	notifier  OrderNotifier
}

func NewService(
	repo *Repository,
	cart CartReader,
	stock StockManager,
	delivery DeliveryFeeCalculator,
	payment PaymentInitiator,
	pod PODDispatcher,
	stores StoreInfoReader,
	customers CustomerInfoReader,
	notifier OrderNotifier,
) *Service {
	return &Service{
		repo:      repo,
		cart:      cart,
		stock:     stock,
		delivery:  delivery,
		payment:   payment,
		pod:       pod,
		stores:    stores,
		customers: customers,
		notifier:  notifier,
	}
}

func (s *Service) SetPaymentInitiator(initiator PaymentInitiator) {
	s.payment = initiator
}

// SetPODDispatcher injects the POD dispatcher after construction.
// Called in main.go after both orderService and podService are created.
func (s *Service) SetPODDispatcher(d PODDispatcher) {
	s.pod = d
}

// ── PlaceOrder ────────────────────────────────────────────────────────────────

// PlaceOrder is the main entry point for checkout.
//
// Flow:
//  1. Load and validate cart items
//  2. Recalculate delivery fee server-side (never trust the client)
//  3. Generate order reference using store county prefix
//  4. Persist order + items in a single transaction
//  5. Reduce stock for each item
//  6. Initiate payment with the chosen provider
//  7. Clear the cart
//  8. Send confirmation notification
func (s *Service) PlaceOrder(ctx context.Context, customerID, sessionID string, req PlaceOrderRequest) (*PlaceOrderResponse, error) {
	// 1. Load cart items
	cartItems, err := s.cart.GetItemsForOrder(ctx, customerID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("orders: failed to load cart: %w", err)
	}
	if len(cartItems) == 0 {
		return nil, errors.New("your cart is empty")
	}

	// 2. Validate all items are from the requested store and in stock
	for _, item := range cartItems {
		if item.StoreID != req.StoreID {
			return nil, fmt.Errorf("orders: item %q belongs to a different store — please review your cart", item.ProductName)
		}
		if !item.InStock {
			return nil, fmt.Errorf("orders: %q is no longer in stock — please remove it from your cart", item.ProductName)
		}
	}

	// 3. Compute totals
	var itemsTotal float64
	for _, item := range cartItems {
		itemsTotal += item.UnitPrice * float64(item.Quantity)
	}

	// 4. Calculate delivery fee server-side
	deliveryFee := 0.0
	vehicleReason := ""
	if req.DeliveryType == "delivery" {
		if req.DeliveryLat == 0 || req.DeliveryLng == 0 {
			return nil, errors.New("delivery coordinates are required for home delivery")
		}
		if req.VehicleType == "" {
			return nil, errors.New("vehicle type is required for home delivery — run cart validation first")
		}
		fee, _, err := s.delivery.CalculateFee(ctx, req.StoreID, req.DeliveryLat, req.DeliveryLng, req.VehicleType)
		if err != nil {
			return nil, fmt.Errorf("orders: could not calculate delivery fee: %w", err)
		}
		deliveryFee = fee
		vehicleReason = fmt.Sprintf("Selected vehicle: %s", req.VehicleType)
	}

	grandTotal := itemsTotal + deliveryFee

	// 5. Get store info for reference generation (also obtain store currency)
	storeName, county, currency, err := s.stores.GetStoreInfo(ctx, req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("orders: could not load store info: %w", err)
	}

	existingCount, err := s.repo.CountByStore(ctx, req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("orders: could not generate reference: %w", err)
	}
	reference := GenerateReference(county, existingCount)

	// 6. Build order items from cart
	orderItems := make([]OrderItem, len(cartItems))
	for i, item := range cartItems {
		orderItems[i] = OrderItem{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Subtotal:    item.UnitPrice * float64(item.Quantity),
		}
	}

	// 7. Persist order
	order := &Order{
		Reference:           reference,
		CustomerID:          customerID,
		FulfillingStoreID:   req.StoreID,
		DeliveryType:        req.DeliveryType,
		DeliveryAddressText: req.DeliveryAddress,
		DeliveryLat:         req.DeliveryLat,
		DeliveryLng:         req.DeliveryLng,
		VehicleType:         req.VehicleType,
		VehicleReason:       vehicleReason,
		ItemsTotal:          itemsTotal,
		DeliveryFee:         deliveryFee,
		GrandTotal:          grandTotal,
		PaymentProvider:     req.PaymentProvider,
	}

	created, err := s.repo.Create(ctx, order, orderItems)
	if err != nil {
		return nil, fmt.Errorf("orders: failed to create order: %w", err)
	}

	// 8. Reduce stock for each item
	for _, item := range cartItems {
		if err := s.stock.ReduceStock(ctx, req.StoreID, item.ProductID, item.Quantity); err != nil {
			// Log but do not fail the order — staff can reconcile manually
			logger.Get().Error().
				Err(err).
				Str("order", created.ID).
				Str("product", item.ProductID).
				Msg("orders: failed to reduce stock — manual reconciliation needed")
		}
	}

	// 9. Initiate payment
	paymentResult := &PaymentInitResult{
		Instructions:    "Order placed. Awaiting payment.",
		AwaitingPayment: false,
	}
	if s.payment != nil {
		result, err := s.payment.Initiate(ctx, PaymentInitRequest{
			OrderID:        created.ID,
			StoreID:        req.StoreID,
			Amount:         grandTotal,
			Currency:       currency,
			Phone:          req.Phone,
			Provider:       req.PaymentProvider,
			Description:    fmt.Sprintf("Payment for order %s", reference),
			PaymentChannel: req.PaymentChannel,
		})
		if err != nil {
			logger.Get().Error().Err(err).Str("order", created.ID).Msg("orders: payment initiation failed")
		} else {
			paymentResult = result
			// Save the provider reference so callbacks can find this order
			if result.ProviderRef != "" {
				_ = s.repo.UpdatePaymentStatus(ctx, created.ID, "pending", result.ProviderRef)
			}
		}
	}

	// 10. Clear cart (non-fatal)
	if err := s.cart.ClearCart(ctx, customerID, sessionID); err != nil {
		logger.Get().Warn().Err(err).Str("order", created.ID).Msg("orders: cart clear failed")
	}

	// 11. Notify customer (non-fatal)
	if s.notifier != nil {
		name, phone, email, _ := s.customers.GetCustomerInfo(ctx, customerID)
		go s.notifier.OrderConfirmed(phone, email, name, reference, storeName, grandTotal)
	}

	resp := s.buildOrderResponse(created, storeName, currency)
	return &PlaceOrderResponse{
		Order:               resp,
		PaymentInstructions: paymentResult.Instructions,
		AwaitingPayment:     paymentResult.AwaitingPayment,
		RedirectURL:         paymentResult.RedirectURL,
	}, nil
}

// ── Customer-facing reads ─────────────────────────────────────────────────────

// GetOwnOrder returns one order, verifying it belongs to the customer.
func (s *Service) GetOwnOrder(ctx context.Context, customerID, orderID string) (*OrderResponse, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.CustomerID != customerID {
		return nil, ErrForbidden
	}
	storeName, _, _, _ := s.stores.GetStoreInfo(ctx, order.FulfillingStoreID)
	resp := s.buildOrderResponse(order, storeName, order.Currency)
	return &resp, nil
}

// ListOwnOrders returns a paginated list of the customer's orders.
func (s *Service) ListOwnOrders(ctx context.Context, customerID string, page, perPage int) ([]OrderResponse, error) {
	orders, err := s.repo.ListByCustomer(ctx, customerID, page, perPage)
	if err != nil {
		return nil, err
	}
	result := make([]OrderResponse, len(orders))
	for i, o := range orders {
		storeName, _, _, _ := s.stores.GetStoreInfo(ctx, o.FulfillingStoreID)
		result[i] = s.buildOrderResponse(&o, storeName, o.Currency)
	}
	return result, nil
}

// TrackOrder returns the status timeline for a customer's order.
func (s *Service) TrackOrder(ctx context.Context, customerID, orderID string) (*TrackingResponse, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.CustomerID != customerID {
		return nil, ErrForbidden
	}

	history, err := s.repo.GetStatusHistory(ctx, orderID)
	if err != nil {
		return nil, err
	}

	var timeline []TrackingEvent
	for _, h := range history {
		details := StatusDetails[h.Status]
		timeline = append(timeline, TrackingEvent{
			Status:      h.Status,
			StatusLabel: details.Label,
			Description: details.Description,
			OccurredAt:  h.CreatedAt,
			IsCurrent:   h.Status == order.Status,
		})
	}

	details := StatusDetails[order.Status]
	return &TrackingResponse{
		Reference:   order.Reference,
		Status:      order.Status,
		StatusLabel: details.Label,
		Timeline:    timeline,
	}, nil
}

// CancelOrder cancels an order if it is in a cancellable state.

// Customer cancellation rules:
//   - "placed"    → allowed (payment not yet confirmed)
//   - "confirmed" → allowed (payment confirmed but store not yet packing)
//   - "preparing" → NOT allowed (store has started packing — contact store)
//   - beyond      → NOT allowed
//
// Staff can cancel at any stage up to dispatch via UpdateStatus.
func (s *Service) CancelOrder(ctx context.Context, customerID, orderID string, req CancelOrderRequest) error {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.CustomerID != customerID {
		return ErrForbidden
	}

	// Customers may only cancel before the store starts packing.
	// Once status reaches "preparing", physical work has begun and the order cannot be self-cancelled — the customer must contact the store.
	customerCancellable := map[OrderStatus]bool{
		StatusPlaced:    true,
		StatusConfirmed: true,
	}
	if !customerCancellable[order.Status] {
		return fmt.Errorf(
			"orders: cancellation is no longer available — your order is already being prepared. " +
				"Please contact the store directly for assistance",
		)
	}

	// If payment was already confirmed, flag for manual refund processing.
	// Refund automation is not implemented — staff must action this manually.
	note := req.Reason
	if order.Status == StatusConfirmed {
		note = "[REFUND REQUIRED] " + req.Reason
		logger.Get().Warn().
			Str("order", orderID).
			Str("payment_provider", order.PaymentProvider).
			Str("grand_total", fmt.Sprintf("%.2f %s", order.GrandTotal, order.Currency)).
			Msg("orders: customer cancelled a paid order — manual refund required")
	}

	if err := s.repo.UpdateStatus(ctx, orderID, StatusCancelled, note, customerID); err != nil {
		return err
	}

	// Restore stock (non-fatal)
	s.restoreStockForOrder(ctx, orderID, order.FulfillingStoreID)

	return nil
}

// ── Staff-facing operations ───────────────────────────────────────────────────

// GetForStore returns full order detail for staff, including customer info.
func (s *Service) GetForStore(ctx context.Context, storeID, orderID string) (*StaffOrderResponse, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.FulfillingStoreID != storeID {
		return nil, ErrForbidden
	}

	storeName, _, _, _ := s.stores.GetStoreInfo(ctx, storeID)
	name, phone, email, _ := s.customers.GetCustomerInfo(ctx, order.CustomerID)
	history, _ := s.repo.GetStatusHistory(ctx, orderID)

	base := s.buildOrderResponse(order, storeName, order.Currency)

	var historyEntries []StatusHistoryEntry
	for _, h := range history {
		historyEntries = append(historyEntries, StatusHistoryEntry{
			Status:    h.Status,
			Label:     StatusDetails[h.Status].Label,
			Note:      h.Note,
			ChangedAt: h.CreatedAt,
		})
	}

	return &StaffOrderResponse{
		OrderResponse: base,
		CustomerName:  name,
		CustomerPhone: phone,
		CustomerEmail: email,
		VehicleReason: order.VehicleReason,
		StatusHistory: historyEntries,
	}, nil
}

// ListForStore returns orders for the scoped store, with optional status filter.
func (s *Service) ListForStore(ctx context.Context, storeID string, filters OrderFilters) ([]OrderResponse, error) {
	orders, err := s.repo.ListByStore(ctx, storeID, filters)
	if err != nil {
		return nil, err
	}
	storeName, _, _, _ := s.stores.GetStoreInfo(ctx, storeID)

	result := make([]OrderResponse, len(orders))
	for i, o := range orders {
		result[i] = s.buildOrderResponse(&o, storeName, o.Currency)
	}
	return result, nil
}

// UpdateStatus moves an order to the next status.
// Validates the transition is permitted before applying it.
// When dispatching (→ out_for_delivery), returns the OTP so the handler
// can pass it to the POD domain for storage and notification.
func (s *Service) UpdateStatus(ctx context.Context, orderID string, req UpdateStatusRequest, changedBy *users.User) (otp string, err error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return "", err
	}

	if !order.Status.CanTransitionTo(req.Status) {
		return "", fmt.Errorf("%w: cannot move from %q to %q", ErrCannotTransition, order.Status, req.Status)
	}

	if err := s.repo.UpdateStatus(ctx, orderID, req.Status, req.Note, changedBy.ID); err != nil {
		return "", err
	}

	// When dispatching, trigger POD: generate OTP, create record, notify customer
	if req.Status == StatusOutForDelivery && s.pod != nil {
		name, phone, _, _ := s.customers.GetCustomerInfo(ctx, order.CustomerID)
		go func() {
			if err := s.pod.Dispatch(ctx, orderID, order.CustomerID, phone, name, order.Reference); err != nil {
				logger.Get().Error().Err(err).Str("order", orderID).Msg("orders: POD dispatch failed")
			}
		}()
	}

	// Notify customer of status change (non-fatal, run in background)
	if s.notifier != nil {
		name, phone, email, _ := s.customers.GetCustomerInfo(ctx, order.CustomerID)
		storeName, _, _, _ := s.stores.GetStoreInfo(ctx, order.FulfillingStoreID)
		details := StatusDetails[req.Status]
		go func() {
			switch req.Status {
			case StatusOutForDelivery:
				s.notifier.OrderStatusChanged(phone, email, name, order.Reference,
					details.Label, fmt.Sprintf("Your order is on its way from %s.", storeName))
			case StatusDelivered:
				s.notifier.OrderDelivered(phone, email, name, order.Reference)
			default:
				s.notifier.OrderStatusChanged(phone, email, name, order.Reference,
					details.Label, details.Description)
			}
		}()
	}

	// When cancelling via staff, restore stock
	if req.Status == StatusCancelled {
		s.restoreStockForOrder(ctx, orderID, order.FulfillingStoreID)
	}

	return "", nil
}

// ConfirmPayment is called by the payment callback handler when payment succeeds.
// It marks the order as paid and advances the status to "confirmed".
func (s *Service) ConfirmPayment(ctx context.Context, providerRef string) error {
	order, err := s.repo.GetByPaymentProviderRef(ctx, providerRef)
	if err != nil {
		return fmt.Errorf("orders: could not find order by provider ref: %w", err)
	}

	if err := s.repo.UpdatePaymentStatus(ctx, order.ID, "paid", providerRef); err != nil {
		return fmt.Errorf("orders: failed to update payment status: %w", err)
	}

	// reload to get latest fields
	order, err = s.repo.GetByID(ctx, order.ID)
	if err != nil {
		return err
	}

	if order.Status != StatusPlaced {
		logger.Get().Warn().
			Str("order", order.ID).
			Str("current_status", string(order.Status)).
			Msg("orders: payment confirmed for order not in 'placed' status — no status change applied")
		return nil
	}

	// Check for idempotency: if the order is already marked as paid, do not attempt to confirm again
	if order.PaymentStatus == "paid" {
		logger.Get().Info().
			Str("order", order.ID).
			Msg("orders: payment already marked as paid — skipping status update")
		return nil
	}

	if err := s.repo.UpdateStatus(ctx, order.ID, StatusConfirmed, "Payment received", ""); err != nil {
		return fmt.Errorf("orders: failed to confirm order status: %w", err)
	}

	// Notify customer
	if s.notifier != nil {
		name, phone, email, _ := s.customers.GetCustomerInfo(ctx, order.CustomerID)
		storeName, _, _, _ := s.stores.GetStoreInfo(ctx, order.FulfillingStoreID)
		go s.notifier.OrderConfirmed(phone, email, name, order.Reference, storeName, order.GrandTotal)
	}

	return nil
}

func (s *Service) FailPayment(ctx context.Context, providerRef string) error {
	order, err := s.repo.GetByPaymentProviderRef(ctx, providerRef)
	if err != nil {
		return fmt.Errorf("orders: could not find order by provider ref: %w", err)
	}
	if err := s.repo.UpdatePaymentStatus(ctx, order.ID, "failed", providerRef); err != nil {
		return fmt.Errorf("orders: failed to update payment status: %w", err)
	}
	return nil
}

// MarkDelivered advances an order to "delivered" status.
// Called by pod.Service after all three POD layers pass (OTP + GPS + photo).
// Satisfies the pod.OrderStatusUpdater interface.
func (s *Service) MarkDelivered(ctx context.Context, orderID, changedBy string) error {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if !order.Status.CanTransitionTo(StatusDelivered) {
		return fmt.Errorf("orders: cannot mark order as delivered from status %q", order.Status)
	}
	return s.repo.UpdateStatus(ctx, orderID, StatusDelivered, "Delivery confirmed via POD", changedBy)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (s *Service) buildOrderResponse(order *Order, storeName, currency string) OrderResponse {
	details := StatusDetails[order.Status]
	return OrderResponse{
		ID:                  order.ID,
		Reference:           order.Reference,
		Status:              order.Status,
		StatusLabel:         details.Label,
		StatusDescription:   details.Description,
		DeliveryType:        order.DeliveryType,
		DeliveryAddress:     order.DeliveryAddressText,
		VehicleType:         order.VehicleType,
		ItemsTotal:          order.ItemsTotal,
		DeliveryFee:         order.DeliveryFee,
		GrandTotal:          order.GrandTotal,
		Currency:            currency,
		PaymentProvider:     order.PaymentProvider,
		PaymentStatus:       order.PaymentStatus,
		FulfillingStoreName: storeName,
		CreatedAt:           order.CreatedAt,
	}
}

func (s *Service) restoreStockForOrder(ctx context.Context, orderID, storeID string) {
	items, err := s.repo.GetItems(ctx, orderID)
	if err != nil {
		logger.Get().Error().Err(err).Str("order", orderID).Msg("orders: could not load items for stock restore")
		return
	}
	for _, item := range items {
		if err := s.stock.RestoreStock(ctx, storeID, item.ProductID, item.Quantity); err != nil {
			logger.Get().Error().Err(err).
				Str("order", orderID).
				Str("product", item.ProductID).
				Msg("orders: failed to restore stock after cancellation")
		}
	}
}

// unused import guard
var _ = time.Now
