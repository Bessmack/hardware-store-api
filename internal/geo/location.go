package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Bessmack/hardware-store-api/pkg/cache"
)

// locationTTL is how long a customer's detected location stays cached.
// After this, the next app load re-captures GPS automatically.
const locationTTL = 4 * time.Hour

// LocationSource indicates how the location was captured.
type LocationSource string

const (
	// SourceGPS means the location came from the browser Geolocation API.
	SourceGPS LocationSource = "gps"
	// SourceManual means the customer explicitly selected a different location
	// (e.g. via the address autocomplete). This overrides GPS.
	SourceManual LocationSource = "manual"
)

// CachedLocation is what gets stored in Redis for each customer/guest session.
// The nearest_store_id is pre-computed so product listing never needs to
// re-run the Haversine calculation — it just reads this value.
type CachedLocation struct {
	Lat              float64        `json:"lat"`
	Lng              float64        `json:"lng"`
	Address          string         `json:"address"`           // human-readable, shown to customer
	Source           LocationSource `json:"source"`
	NearestStoreID   string         `json:"nearest_store_id"`
	NearestStoreName string         `json:"nearest_store_name"`
	CachedAt         time.Time      `json:"cached_at"`
	ExpiresAt        time.Time      `json:"expires_at"`
}

// StoreLister is the interface the location service needs from the stores package.
// Defined here as an interface to avoid a circular import between geo ↔ stores.
type StoreLister interface {
	ListActiveStores(ctx context.Context) ([]StoreInfo, error)
}

// LocationService manages reading and writing customer location data to Redis.
type LocationService struct {
	cache           *cache.Cache
	reverseGeocoder *ReverseGeocoder
	storeLister     StoreLister
}

func NewLocationService(c *cache.Cache, rg *ReverseGeocoder, sl StoreLister) *LocationService {
	return &LocationService{
		cache:           c,
		reverseGeocoder: rg,
		storeLister:     sl,
	}
}

// Save captures a location, reverse-geocodes it to a readable address,
// finds the nearest store, and writes everything to Redis with a 4-hour TTL.
//
// Called when:
//   - The app opens and Browser Geolocation API returns GPS coords (source: gps)
//   - The customer manually selects a different location (source: manual)
//
// The key is built by LocationKey() — different for registered vs guest users.
func (s *LocationService) Save(ctx context.Context, key string, lat, lng float64, source LocationSource) (*CachedLocation, error) {
	// Reverse geocode for human-readable address shown to customer
	address, err := s.reverseGeocoder.ReverseGeocode(ctx, lat, lng)
	if err != nil {
		// Non-fatal — fall back to coordinate string if geocoding fails
		address = fmt.Sprintf("%.4f°, %.4f°", lat, lng)
	}

	// Find nearest store (pure Haversine math — no API call)
	stores, err := s.storeLister.ListActiveStores(ctx)
	if err != nil {
		return nil, fmt.Errorf("location: failed to load stores: %w", err)
	}

	now := time.Now()
	loc := &CachedLocation{
		Lat:       lat,
		Lng:       lng,
		Address:   address,
		Source:    source,
		CachedAt:  now,
		ExpiresAt: now.Add(locationTTL),
	}

	if nearest := FindNearestStore(stores, lat, lng); nearest != nil {
		loc.NearestStoreID = nearest.ID
		loc.NearestStoreName = nearest.Name
	}

	data, err := json.Marshal(loc)
	if err != nil {
		return nil, fmt.Errorf("location: failed to marshal: %w", err)
	}

	if err := s.cache.Set(ctx, key, string(data), locationTTL); err != nil {
		return nil, fmt.Errorf("location: failed to write to cache: %w", err)
	}

	return loc, nil
}

// Get reads the cached location for the given key.
// Returns an error if the key is missing (expired or never set).
func (s *LocationService) Get(ctx context.Context, key string) (*CachedLocation, error) {
	data, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("location: not found or expired")
	}

	var loc CachedLocation
	if err := json.Unmarshal([]byte(data), &loc); err != nil {
		return nil, fmt.Errorf("location: failed to decode cached value: %w", err)
	}

	return &loc, nil
}

// Clear removes the cached location for the given key.
// Called when a customer logs out, so a fresh GPS capture happens on next login.
func (s *LocationService) Clear(ctx context.Context, key string) error {
	return s.cache.Delete(ctx, key)
}

// LocationKey builds the Redis key for a given user or guest session.
//
// Registered user:  loc:user:{userID}
// Guest session:    loc:guest:{sessionID}
//
// sessionID comes from the X-Session-ID header sent by the frontend for guests.
// The frontend generates and persists this ID in localStorage.
func LocationKey(userID, sessionID string) string {
	if userID != "" {
		return fmt.Sprintf("loc:user:%s", userID)
	}
	return fmt.Sprintf("loc:guest:%s", sessionID)
}