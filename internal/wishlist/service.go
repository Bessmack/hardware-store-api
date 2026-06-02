package wishlist

import (
	"context"
	"errors"
	"fmt"
)

// LivePriceFetcher provides current pricing for wishlist display.
// Implemented by the inventory/products layer; defined as an interface here
// to keep wishlist decoupled from those packages.
type LivePriceFetcher interface {
	// GetLivePrice returns current price, currency, in-stock flag, and
	// limited-availability flag for a product at a given store.
	// limited is true when stock is low (below low_stock_alert).
	GetLivePrice(ctx context.Context, productID, storeID string) (price float64, currency string, inStock bool, limited bool, err error)
}

type Service struct {
	repo         *Repository
	priceFetcher LivePriceFetcher
}

func NewService(repo *Repository, priceFetcher LivePriceFetcher) *Service {
	return &Service{repo: repo, priceFetcher: priceFetcher}
}

// ── Wishlist management ───────────────────────────────────────────────────────

// Create makes a new named wishlist for the customer.
func (s *Service) Create(ctx context.Context, customerID string, req CreateWishlistRequest) (*WishlistSummary, error) {
	w, err := s.repo.Create(ctx, customerID, req.Name)
	if err != nil {
		return nil, fmt.Errorf("wishlist: failed to create: %w", err)
	}
	return &WishlistSummary{
		ID:        w.ID,
		Name:      w.Name,
		ItemCount: 0,
		CreatedAt: w.CreatedAt,
	}, nil
}

// ListAll returns all wishlists for the customer with item counts.
func (s *Service) ListAll(ctx context.Context, customerID string) ([]WishlistSummary, error) {
	return s.repo.ListByCustomer(ctx, customerID)
}

// Delete removes a wishlist. Ownership is verified inside the repository.
func (s *Service) Delete(ctx context.Context, wishlistID, customerID string) error {
	return s.repo.Delete(ctx, wishlistID, customerID)
}

// ── Item management ───────────────────────────────────────────────────────────

// AddItem adds a product to a customer's wishlist.
// If wishlistID is empty, the item is added to the default "My Wishlist"
// (which is created automatically if it does not yet exist).
func (s *Service) AddItem(ctx context.Context, customerID, wishlistID string, req AddItemRequest) error {
	// Resolve wishlist
	if wishlistID == "" {
		w, err := s.repo.EnsureDefault(ctx, customerID)
		if err != nil {
			return fmt.Errorf("wishlist: could not resolve default wishlist: %w", err)
		}
		wishlistID = w.ID
	} else {
		// Verify ownership
		if _, err := s.repo.GetByID(ctx, wishlistID, customerID); err != nil {
			return err
		}
	}

	_, err := s.repo.AddItem(ctx, wishlistID, req.ProductID, req.Note)
	if errors.Is(err, ErrItemAlreadyAdded) {
		return err // surface this cleanly — not a system error
	}
	return err
}

// RemoveItem removes a product from a wishlist. Ownership verified in repo.
func (s *Service) RemoveItem(ctx context.Context, customerID, wishlistID, itemID string) error {
	return s.repo.RemoveItem(ctx, wishlistID, itemID, customerID)
}

// ── Get wishlist with live prices ─────────────────────────────────────────────

// GetWithPrices returns a full wishlist with live pricing from the customer's
// nearest store. Items unavailable at the nearest store are still shown but
// with InStock = false and a hint about the nearest available store.
//
// nearestStoreID comes from the customer's cached location (Redis).
// nearestStoreName is included for display.
func (s *Service) GetWithPrices(
	ctx context.Context,
	customerID, wishlistID string,
	nearestStoreID, nearestStoreName string,
) (*WishlistResponse, error) {
	// Verify ownership and get wishlist metadata
	w, err := s.repo.GetByID(ctx, wishlistID, customerID)
	if err != nil {
		return nil, err
	}

	// Get raw items (no pricing yet)
	items, err := s.repo.GetRawItems(ctx, wishlistID)
	if err != nil {
		return nil, err
	}

	// Inject live pricing for each item
	for i := range items {
		if nearestStoreID == "" {
			// No location — show items without pricing
			continue
		}

		price, currency, inStock, limited, err := s.priceFetcher.GetLivePrice(
			ctx, items[i].ProductID, nearestStoreID,
		)
		if err != nil {
			// Product may not be stocked at this store — show as unavailable
			items[i].InStock = false
			continue
		}

		items[i].Price = price
		items[i].Currency = currency
		items[i].InStock = inStock
		items[i].LimitedAvailability = limited
		items[i].NearestStoreID = nearestStoreID
		items[i].NearestStoreName = nearestStoreName
	}

	return &WishlistResponse{
		ID:        w.ID,
		Name:      w.Name,
		CreatedAt: w.CreatedAt,
		Items:     items,
	}, nil
}