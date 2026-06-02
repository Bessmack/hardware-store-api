package cart

import (
	"context"
	"errors"
	"fmt"
)

// InventoryReader is the subset of the products repository the cart service needs.
// Using an interface keeps cart decoupled from the products package.
type InventoryReader interface {
	GetCurrentPrice(ctx context.Context, productID, storeID string) (price float64, currency string, inStock bool, err error)
}

// WeightThresholdReader provides vehicle weight thresholds from delivery_rates.
type WeightThresholdReader interface {
	GetWeightThresholds(ctx context.Context, storeID string) (WeightThresholds, error)
}

type Service struct {
	repo       *Repository
	inventory  InventoryReader
	thresholds WeightThresholdReader
}

func NewService(repo *Repository, inventory InventoryReader, thresholds WeightThresholdReader) *Service {
	return &Service{
		repo:       repo,
		inventory:  inventory,
		thresholds: thresholds,
	}
}

// ── Cart retrieval ────────────────────────────────────────────────────────────

// GetCart returns the full cart for a customer or guest.
// Creates the cart if it does not yet exist.
func (s *Service) GetCart(ctx context.Context, customerID, sessionID string) (*CartResponse, error) {
	cart, err := s.resolveCart(ctx, customerID, sessionID)
	if err != nil {
		return nil, err
	}
	return s.buildResponse(ctx, cart)
}

// ── Item management ───────────────────────────────────────────────────────────

// AddItem adds a product to the cart with the current price locked in.
// If the product is already in the cart the quantity is incremented.
func (s *Service) AddItem(ctx context.Context, customerID, sessionID string, req AddItemRequest) (*CartResponse, error) {
	// Fetch the current price from inventory — this gets locked into the cart item
	price, currency, inStock, err := s.inventory.GetCurrentPrice(ctx, req.ProductID, req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("cart: could not fetch product price: %w", err)
	}
	if !inStock {
		return nil, errors.New("this product is currently out of stock at the selected store")
	}

	cart, err := s.resolveCart(ctx, customerID, sessionID)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.AddItem(ctx, cart.ID, req, price, currency); err != nil {
		return nil, fmt.Errorf("cart: failed to add item: %w", err)
	}

	return s.buildResponse(ctx, cart)
}

// UpdateQuantity changes the quantity of an existing cart item.
func (s *Service) UpdateQuantity(ctx context.Context, customerID, sessionID, itemID string, qty int) (*CartResponse, error) {
	cart, err := s.resolveCart(ctx, customerID, sessionID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateQuantity(ctx, cart.ID, itemID, qty); err != nil {
		if errors.Is(err, ErrItemNotFound) {
			return nil, ErrItemNotFound
		}
		return nil, fmt.Errorf("cart: failed to update quantity: %w", err)
	}

	return s.buildResponse(ctx, cart)
}

// RemoveItem removes a single item from the cart.
func (s *Service) RemoveItem(ctx context.Context, customerID, sessionID, itemID string) (*CartResponse, error) {
	cart, err := s.resolveCart(ctx, customerID, sessionID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.RemoveItem(ctx, cart.ID, itemID); err != nil {
		return nil, err
	}

	return s.buildResponse(ctx, cart)
}

// ClearCart removes all items but keeps the cart record.
func (s *Service) ClearCart(ctx context.Context, customerID, sessionID string) error {
	cart, err := s.resolveCart(ctx, customerID, sessionID)
	if err != nil {
		return err
	}
	return s.repo.ClearItems(ctx, cart.ID)
}

// ── Validation ────────────────────────────────────────────────────────────────

// ValidateCart runs pre-checkout checks and returns a full validation result.
//
// Checks performed:
//  1. Each item is still available (blocks checkout if not)
//  2. Prices have not changed since the item was added (updates locked price if so)
//  3. Required vehicle type for the cart contents
func (s *Service) ValidateCart(ctx context.Context, customerID, sessionID string) (*ValidationResult, error) {
	cart, err := s.resolveCart(ctx, customerID, sessionID)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.GetItems(ctx, cart.ID)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return &ValidationResult{
			IsValid: false,
			Errors:  []string{"your cart is empty"},
		}, nil
	}

	result := &ValidationResult{IsValid: true}

	for i := range items {
		item := &items[i]

		if !item.InStock {
			result.IsValid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s is no longer available at the selected store", item.ProductName),
			)
			continue
		}

		if item.PriceChanged {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Price of %s changed from %s %.0f to %s %.0f",
				item.ProductName,
				item.Currency, item.UnitPrice,
				item.Currency, item.CurrentPrice,
			))
			// Update the locked price in the DB so payment uses current price
			_ = s.repo.UpdateItemPrice(ctx, item.ID, item.CurrentPrice)
			item.UnitPrice = item.CurrentPrice
			item.Subtotal = item.UnitPrice * float64(item.Quantity)
			result.UpdatedItems = append(result.UpdatedItems, *item)
		}
	}

	// Determine required vehicle
	vehicleItems, err := s.repo.GetItemsForVehicle(ctx, cart.ID)
	if err != nil {
		return nil, err
	}

	// Use a default store to look up thresholds — take from first item
	storeID := ""
	if len(items) > 0 {
		storeID = items[0].StoreID
	}

	thresholds, err := s.thresholds.GetWeightThresholds(ctx, storeID)
	if err != nil {
		// Fall back to sensible defaults if DB lookup fails
		thresholds = WeightThresholds{BikeMaxKg: 30, VanMaxKg: 500}
	}

	vehicle := DetermineVehicle(vehicleItems, thresholds)
	result.RequiredVehicle = vehicle.VehicleType
	result.VehicleReason = vehicle.Reason

	return result, nil
}

// ── Merge ─────────────────────────────────────────────────────────────────────

// MergeGuestCart merges a guest session's cart into the customer's cart.
// Called by the auth handler after successful login.
func (s *Service) MergeGuestCart(ctx context.Context, guestSessionID, customerID string) error {
	return s.repo.MergeGuestCart(ctx, guestSessionID, customerID)
}

// ── Internals ─────────────────────────────────────────────────────────────────

func (s *Service) resolveCart(ctx context.Context, customerID, sessionID string) (*Cart, error) {
	if customerID != "" {
		return s.repo.GetOrCreateByCustomerID(ctx, customerID)
	}
	if sessionID != "" {
		return s.repo.GetOrCreateBySessionID(ctx, sessionID)
	}
	return nil, errors.New("cart: neither customer ID nor session ID provided")
}

func (s *Service) buildResponse(ctx context.Context, cart *Cart) (*CartResponse, error) {
	items, err := s.repo.GetItems(ctx, cart.ID)
	if err != nil {
		return nil, err
	}

	resp := &CartResponse{
		ID:    cart.ID,
		Items: items,
	}

	// Compute summary values
	var subtotal float64
	var firstCurrency string
	mixedCurrencies := false

	for _, item := range items {
		subtotal += item.Subtotal
		resp.ItemCount += item.Quantity

		if firstCurrency == "" {
			firstCurrency = item.Currency
		} else if item.Currency != firstCurrency {
			mixedCurrencies = true
		}

		if item.PriceChanged {
			resp.Warnings = append(resp.Warnings, fmt.Sprintf(
				"Price of %s has changed — review before checkout", item.ProductName,
			))
		}
	}

	resp.Subtotal = subtotal
	if !mixedCurrencies {
		resp.Currency = firstCurrency
	}

	// Vehicle determination
	if len(items) > 0 {
		vehicleItems, err := s.repo.GetItemsForVehicle(ctx, cart.ID)
		if err == nil && len(vehicleItems) > 0 {
			thresholds, err := s.thresholds.GetWeightThresholds(ctx, items[0].StoreID)
			if err != nil {
				thresholds = WeightThresholds{BikeMaxKg: 30, VanMaxKg: 500}
			}
			v := DetermineVehicle(vehicleItems, thresholds)
			resp.RequiredVehicle = v.VehicleType
			resp.VehicleReason = v.Reason
		}
	}

	return resp, nil
}