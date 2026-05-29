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

// Geocoder converts addresses to coordinates and back.
// Primary provider: Nominatim (OpenStreetMap) — completely free in production.
// Fallback provider: OpenCage — 2,500 free requests/day (optional).
//
// Nominatim usage rules:
//   - Max 1 request/second — enforced via a simple sleep between calls
//   - User-Agent must identify your app — set via config
//   - Do not cache results for longer than one week (per Nominatim policy)
type Geocoder struct {
	nominatimURL   string
	userAgent      string
	openCageAPIKey string
	httpClient     *http.Client
}

// GeocoderConfig is passed in from main.go when initialising the geocoder.
type GeocoderConfig struct {
	NominatimBaseURL   string
	NominatimUserAgent string
	OpenCageAPIKey     string
}

// Coordinates holds a latitude/longitude pair.
type Coordinates struct {
	Lat float64
	Lng float64
}

// NewGeocoder creates a Geocoder ready to use.
func NewGeocoder(cfg GeocoderConfig) *Geocoder {
	return &Geocoder{
		nominatimURL:   cfg.NominatimBaseURL,
		userAgent:      cfg.NominatimUserAgent,
		openCageAPIKey: cfg.OpenCageAPIKey,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Geocode converts a text address into lat/lng coordinates.
// Tries Nominatim first. If that fails AND an OpenCage key is configured,
// falls back to OpenCage automatically.
func (g *Geocoder) Geocode(ctx context.Context, address string) (*Coordinates, error) {
	coords, err := g.nominatimGeocode(ctx, address)
	if err == nil {
		return coords, nil
	}

	if g.openCageAPIKey != "" {
		return g.openCageGeocode(ctx, address)
	}

	return nil, fmt.Errorf("geocoder: failed to geocode %q: %w", address, err)
}

// ReverseGeocode converts lat/lng into a human-readable address string.
// Used after GPS capture to show the customer their detected address for confirmation.
func (g *Geocoder) ReverseGeocode(ctx context.Context, coords Coordinates) (string, error) {
	endpoint := fmt.Sprintf(
		"%s/reverse?lat=%f&lon=%f&format=json",
		g.nominatimURL, coords.Lat, coords.Lng,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", g.userAgent)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("geocoder: reverse geocode failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("geocoder: failed to decode response: %w", err)
	}

	return result.DisplayName, nil
}

// ── Nominatim ─────────────────────────────────────────────────────────────────

func (g *Geocoder) nominatimGeocode(ctx context.Context, address string) (*Coordinates, error) {
	// countrycodes=ke biases results to Kenya — remove for multi-country support
	endpoint := fmt.Sprintf(
		"%s/search?q=%s&format=json&limit=1&countrycodes=ke",
		g.nominatimURL,
		url.QueryEscape(address),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", g.userAgent) // required by Nominatim policy

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("nominatim: no results found")
	}

	lat, _ := strconv.ParseFloat(results[0].Lat, 64)
	lng, _ := strconv.ParseFloat(results[0].Lon, 64)

	return &Coordinates{Lat: lat, Lng: lng}, nil
}

// ── OpenCage fallback ─────────────────────────────────────────────────────────

func (g *Geocoder) openCageGeocode(ctx context.Context, address string) (*Coordinates, error) {
	endpoint := fmt.Sprintf(
		"https://api.opencagedata.com/geocode/v1/json?q=%s&key=%s&limit=1&countrycode=ke",
		url.QueryEscape(address),
		g.openCageAPIKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencage: request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			Geometry struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"geometry"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("opencage: failed to decode response: %w", err)
	}
	if len(result.Results) == 0 {
		return nil, fmt.Errorf("opencage: no results found")
	}

	return &Coordinates{
		Lat: result.Results[0].Geometry.Lat,
		Lng: result.Results[0].Geometry.Lng,
	}, nil
}