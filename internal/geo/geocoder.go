package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Geocoder converts a text address into lat/lng coordinates.
// Provider: Nominatim (OpenStreetMap) — completely free, no API key.
//
// Nominatim rules (must follow in production):
//   - Max 1 request/second
//   - User-Agent header must identify your application
//   - Results biased to Kenya via countrycodes=ke
//   - Docs: https://nominatim.org/release-docs/latest/api/Search/
type Geocoder struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
}

// Coordinates holds a lat/lng pair returned by any geocoding operation.
type Coordinates struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func NewGeocoder(baseURL, userAgent string) *Geocoder {
	return &Geocoder{
		baseURL:   baseURL,
		userAgent: userAgent,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Geocode converts a text address into coordinates.
// Used when a customer submits a full address string and we need its lat/lng.
// For live search-as-you-type, use the Autocompleter (Photon) instead.
func (g *Geocoder) Geocode(ctx context.Context, address string) (*Coordinates, error) {
	endpoint := fmt.Sprintf(
		"%s/search?q=%s&format=json&limit=1&countrycodes=ke",
		g.baseURL,
		url.QueryEscape(address),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("geocoder: failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", g.userAgent) // required by Nominatim

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocoder: request failed: %w", err)
	}
	defer resp.Body.Close()

	var results []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("geocoder: failed to decode response: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("geocoder: no results for address %q", address)
	}

	lat, _ := strconv.ParseFloat(results[0].Lat, 64)
	lng, _ := strconv.ParseFloat(results[0].Lon, 64)

	return &Coordinates{Lat: lat, Lng: lng}, nil
}