package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Autocompleter provides live address suggestions as the customer types.
// Provider: Photon (https://photon.komoot.io) — free, OpenStreetMap-based.
// No API key required. Results can be biased toward a lat/lng to stay relevant.
//
// Photon docs: https://photon.komoot.io
// Self-hostable if you need higher rate limits in the future.
type Autocompleter struct {
	baseURL    string
	httpClient *http.Client
}

// AutocompleteResult is a single address suggestion returned to the frontend.
type AutocompleteResult struct {
	Name        string  `json:"name"`
	Street      string  `json:"street,omitempty"`
	City        string  `json:"city,omitempty"`
	County      string  `json:"county,omitempty"`
	Country     string  `json:"country,omitempty"`
	DisplayName string  `json:"display_name"` // full formatted label shown in dropdown
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

func NewAutocompleter(baseURL string) *Autocompleter {
	return &Autocompleter{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

// Search returns address suggestions for the given query string.
//
// biaLat / biasLng: optional coordinates to bias results toward the customer's
// current area. Pass 0, 0 to skip the bias.
//
// limit: number of suggestions to return (3–5 is usually enough for a dropdown).
func (a *Autocompleter) Search(ctx context.Context, query string, biasLat, biasLng float64, limit int) ([]AutocompleteResult, error) {
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	endpoint := fmt.Sprintf(
		"%s/api/?q=%s&limit=%d&lang=en",
		a.baseURL,
		url.QueryEscape(query),
		limit,
	)

	// Bias results toward customer's current location so "Westlands" returns
	// Nairobi results, not Westlands in another country.
	if biasLat != 0 && biasLng != 0 {
		endpoint += fmt.Sprintf("&lat=%f&lon=%f", biasLat, biasLng)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("autocomplete: failed to build request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autocomplete: request failed: %w", err)
	}
	defer resp.Body.Close()

	// Photon returns GeoJSON FeatureCollection
	var featureCollection struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"` // [lng, lat]
			} `json:"geometry"`
			Properties struct {
				Name    string `json:"name"`
				Street  string `json:"street"`
				City    string `json:"city"`
				County  string `json:"county"`
				Country string `json:"country"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&featureCollection); err != nil {
		return nil, fmt.Errorf("autocomplete: failed to decode response: %w", err)
	}

	results := make([]AutocompleteResult, 0, len(featureCollection.Features))

	for _, f := range featureCollection.Features {
		if len(f.Geometry.Coordinates) < 2 {
			continue
		}

		p := f.Properties
		result := AutocompleteResult{
			Name:      p.Name,
			Street:    p.Street,
			City:      p.City,
			County:    p.County,
			Country:   p.Country,
			Latitude:  f.Geometry.Coordinates[1], // GeoJSON is [lng, lat]
			Longitude: f.Geometry.Coordinates[0],
		}
		result.DisplayName = buildDisplayName(p.Name, p.Street, p.City, p.County)
		results = append(results, result)
	}

	return results, nil
}

// buildDisplayName produces a clean label like "Westlands, Nairobi, Kenya"
// from the individual Photon property fields.
func buildDisplayName(parts ...string) string {
	result := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if result == "" {
			result = p
		} else {
			result += ", " + p
		}
	}
	return result
}