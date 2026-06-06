package delivery

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/users"
)

// StoreCoordinateReader fetches the coordinates of a store for distance calculation.
// Implemented by the stores repository — defined as an interface here to
// keep delivery decoupled from the stores package.
type StoreCoordinateReader interface {
	GetStoreCoordinates(ctx context.Context, storeID string) (name string, lat, lng float64, currency string, err error)
}

type Service struct {
	repo   *Repository
	stores StoreCoordinateReader
}

func NewService(repo *Repository, stores StoreCoordinateReader) *Service {
	return &Service{repo: repo, stores: stores}
}

// ── Quote generation ──────────────────────────────────────────────────────────

// Quote calculates delivery options for a store to a delivery address.
//
// For each vehicle type (bike, van, truck):
//   - Looks up the rate (store-specific first, global fallback)
//   - Calculates fee: ≤1km → base_fee; >1km → distance × per_km
//   - Estimates delivery time with +59 min buffer for traffic
//   - Marks unavailable if distance exceeds the vehicle's max_radius_km
//
// If req.VehicleType is set, only that vehicle is calculated.
// Otherwise all three are returned so the customer can compare.
func (s *Service) Quote(ctx context.Context, req QuoteRequest) (*QuoteResponse, error) {
	// Get store name and coordinates for distance calculation
	storeName, storeLat, storeLng, currency, err := s.stores.GetStoreCoordinates(ctx, req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("delivery: could not load store: %w", err)
	}

	distanceKm := geo.HaversineDistance(storeLat, storeLng, req.DeliveryLat, req.DeliveryLng)

	vehicles := []string{"bike", "van", "truck"}
	if req.VehicleType != "" {
		vehicles = []string{req.VehicleType}
	}

	var options []VehicleOption
	for _, v := range vehicles {
		option := s.buildOption(ctx, req.StoreID, v, distanceKm, currency, req.RequiredVehicle)
		options = append(options, option)
	}

	recommended := req.RequiredVehicle
	if recommended == "" {
		recommended = "bike" // lowest cost by default if not specified
	}

	return &QuoteResponse{
		StoreID:            req.StoreID,
		StoreName:          storeName,
		DistanceKm:         roundKm(distanceKm),
		Options:            options,
		RecommendedVehicle: recommended,
	}, nil
}

func (s *Service) buildOption(ctx context.Context, storeID, vehicleType string, distanceKm float64, currency, requiredVehicle string) VehicleOption {
	rate, err := s.repo.GetRateForStore(ctx, storeID, vehicleType)
	if err != nil {
		return VehicleOption{
			VehicleType:       vehicleType,
			IsAvailable:       false,
			UnavailableReason: "delivery rate not configured for this vehicle",
			Currency:          currency,
		}
	}

	// Check radius limit
	if rate.MaxRadiusKm > 0 && distanceKm > rate.MaxRadiusKm {
		return VehicleOption{
			VehicleType: vehicleType,
			IsAvailable: false,
			UnavailableReason: fmt.Sprintf(
				"%s delivery is only available within %.0f km — your location is %.1f km away",
				vehicleType, rate.MaxRadiusKm, distanceKm,
			),
			Currency: currency,
		}
	}

	fee := CalculateFee(distanceKm, rate.BaseFee, rate.PerKm)
	estimatedMins := EstimateDeliveryMins(vehicleType, distanceKm)

	// Determine if this vehicle is required (cart has items that mandate it)
	isRequired := vehicleType == requiredVehicle

	return VehicleOption{
		VehicleType:    vehicleType,
		Fee:            roundFee(fee),
		Currency:       currency,
		EstimatedMins:  estimatedMins,
		EstimatedLabel: FormatEstimate(estimatedMins),
		IsAvailable:    true,
		IsRequired:     isRequired,
	}
}

// ── Rate management ───────────────────────────────────────────────────────────

// ListRates returns delivery rates for a store (admin) or all global rates (superadmin).
func (s *Service) ListRates(ctx context.Context, storeID string, requestedBy *users.User) ([]RateResponse, error) {
	if requestedBy.Role == users.RoleSuperAdmin && storeID == "" {
		return s.repo.ListGlobalRates(ctx)
	}
	return s.repo.ListRatesForStore(ctx, storeID)
}

// UpsertStoreRate sets a store-specific delivery rate, overriding the global default.
// Admin can update their own store's rates. SuperAdmin can update any store.
func (s *Service) UpsertStoreRate(ctx context.Context, storeID, vehicleType string, req UpdateRateRequest, requestedBy *users.User) (*DeliveryRate, error) {
	if !requestedBy.Role.CanManageStore() {
		return nil, errors.New("only admins and superadmin can manage delivery rates")
	}
	return s.repo.UpsertStoreRate(ctx, storeID, vehicleType, req, requestedBy.ID)
}

// UpdateGlobalRate updates the global default rate. SuperAdmin only.
func (s *Service) UpdateGlobalRate(ctx context.Context, vehicleType string, req UpdateRateRequest, requestedBy *users.User) (*DeliveryRate, error) {
	if requestedBy.Role != users.RoleSuperAdmin {
		return nil, errors.New("only superadmin can update global delivery rates")
	}
	return s.repo.UpdateGlobalRate(ctx, vehicleType, req, requestedBy.ID)
}

// DeleteStoreRate removes a store's rate override, reverting to the global default.
func (s *Service) DeleteStoreRate(ctx context.Context, storeID, vehicleType string, requestedBy *users.User) error {
	if !requestedBy.Role.CanManageStore() {
		return errors.New("only admins and superadmin can manage delivery rates")
	}
	return s.repo.DeleteStoreRate(ctx, storeID, vehicleType)
}

// ── cart.WeightThresholdReader proxy ─────────────────────────────────────────

// GetWeightThresholds satisfies the cart.WeightThresholdReader interface,
// delegating to the repository.
func (s *Service) GetWeightThresholds(ctx context.Context, storeID string) (bikeMax, vanMax float64, err error) {
	thresholds, err := s.repo.GetWeightThresholds(ctx, storeID)
	if err != nil {
		return 30, 500, nil // safe defaults if lookup fails
	}
	return thresholds.BikeMaxKg, thresholds.VanMaxKg, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func roundKm(km float64) float64 {
	return float64(int(km*100)) / 100 // 2 decimal places
}

func roundFee(fee float64) float64 {
	return float64(int(fee*100)) / 100
}