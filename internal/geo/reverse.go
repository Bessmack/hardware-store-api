package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ReverseGeocoder converts lat/lng coordinates into a human-readable address.
//
// Primary:  OpenCage — 2,500 free requests/day, no billing required.
//           Used after GPS capture to show the customer their detected address.
// Fallback: Nominatim — used when no OpenCage key is configured.
//
// OpenCage docs: https://opencagedata.com/api
type ReverseGeocoder struct {
	openCageAPIKey string
	nominatimURL   string
	nominatimAgent string
	httpClient     *http.Client
}

func NewReverseGeocoder(openCageAPIKey, nominatimURL, nominatimAgent string) *ReverseGeocoder {
	return &ReverseGeocoder{
		openCageAPIKey: openCageAPIKey,
		nominatimURL:   nominatimURL,
		nominatimAgent: nominatimAgent,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

// ReverseGeocode returns a human-readable address for the given coordinates.
// Tries OpenCage first; falls back to Nominatim if no OpenCage key is set.
func (r *ReverseGeocoder) ReverseGeocode(ctx context.Context, lat, lng float64) (string, error) {
	if r.openCageAPIKey != "" {
		address, err := r.openCageReverse(ctx, lat, lng)
		if err == nil {
			return address, nil
		}
		// Fall through to Nominatim on OpenCage failure
	}
	return r.nominatimReverse(ctx, lat, lng)
}

// ── OpenCage ──────────────────────────────────────────────────────────────────

func (r *ReverseGeocoder) openCageReverse(ctx context.Context, lat, lng float64) (string, error) {
	endpoint := fmt.Sprintf(
		"https://api.opencagedata.com/geocode/v1/json?q=%f+%f&key=%s&limit=1&no_annotations=1",
		lat, lng, r.openCageAPIKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("reverse: opencage request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			Formatted string `json:"formatted"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("reverse: failed to decode opencage response: %w", err)
	}
	if len(result.Results) == 0 {
		return "", fmt.Errorf("reverse: opencage returned no results")
	}

	return result.Results[0].Formatted, nil
}

// ── Nominatim fallback ────────────────────────────────────────────────────────

func (r *ReverseGeocoder) nominatimReverse(ctx context.Context, lat, lng float64) (string, error) {
	endpoint := fmt.Sprintf(
		"%s/reverse?lat=%f&lon=%f&format=json",
		r.nominatimURL, lat, lng,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", r.nominatimAgent)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("reverse: nominatim request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("reverse: failed to decode nominatim response: %w", err)
	}
	if result.DisplayName == "" {
		return "", fmt.Errorf("reverse: nominatim returned empty address")
	}

	return result.DisplayName, nil
}