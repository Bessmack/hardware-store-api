package geo

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/response"
)

// Handler exposes geo functionality over HTTP.
type Handler struct {
	locationService *LocationService
	autocompleter   *Autocompleter
	geocoder        *Geocoder
}

func NewHandler(ls *LocationService, ac *Autocompleter, gc *Geocoder) *Handler {
	return &Handler{
		locationService: ls,
		autocompleter:   ac,
		geocoder:        gc,
	}
}

// ── Routes (registered in server/routes.go) ───────────────────────────────────
//
// POST /api/v1/location               OptionalAuth  — save GPS or manual location
// GET  /api/v1/location               OptionalAuth  — get current cached location
// GET  /api/v1/geo/autocomplete       Public        — Photon address suggestions
// GET  /api/v1/geo/geocode            Public        — Nominatim text → coords

// SaveLocation saves the customer's location to the 4-hour Redis cache.
//
// Called by the frontend in two situations:
//  1. On app open — Browser Geolocation API fires, frontend sends GPS coords automatically
//  2. When customer manually selects an address — sends the Photon result's coords
//
// Body:
//
//	{ "lat": -1.2921, "lng": 36.8219, "source": "gps" | "manual" }
func (h *Handler) SaveLocation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lat    float64        `json:"lat"`
		Lng    float64        `json:"lng"`
		Source LocationSource `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Lat == 0 && req.Lng == 0 {
		response.BadRequest(w, "lat and lng are required")
		return
	}
	if req.Source == "" {
		req.Source = SourceGPS
	}

	key := resolveLocationKey(r)

	loc, err := h.locationService.Save(r.Context(), key, req.Lat, req.Lng, req.Source)
	if err != nil {
		response.InternalServerError(w)
		return
	}

	response.Success(w, loc)
}

// GetLocation returns the currently cached location for this user/session.
// The frontend uses this to show "Prices at: Kiambu Branch" in the header.
func (h *Handler) GetLocation(w http.ResponseWriter, r *http.Request) {
	key := resolveLocationKey(r)

	loc, err := h.locationService.Get(r.Context(), key)
	if err != nil {
		// Not an error — just means no location has been saved yet
		response.Success(w, nil)
		return
	}

	response.Success(w, loc)
}

// Autocomplete returns address suggestions for a search query.
// Powers the address input dropdown during checkout and manual location selection.
//
// Query params:
//
//	q        (required) — search term, e.g. "Westlands"
//	limit    (optional) — number of results, default 5
//	bias_lat (optional) — lat to bias results toward customer's current area
//	bias_lng (optional) — lng to bias results toward customer's current area
//
// Example: GET /api/v1/geo/autocomplete?q=Westlands&limit=5&bias_lat=-1.28&bias_lng=36.82
func (h *Handler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		response.BadRequest(w, "q (search query) is required")
		return
	}
	if len(q) < 2 {
		// Avoid hitting Photon with single-character queries
		response.Success(w, []AutocompleteResult{})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	biasLat, _ := strconv.ParseFloat(r.URL.Query().Get("bias_lat"), 64)
	biasLng, _ := strconv.ParseFloat(r.URL.Query().Get("bias_lng"), 64)

	results, err := h.autocompleter.Search(r.Context(), q, biasLat, biasLng, limit)
	if err != nil {
		response.InternalServerError(w)
		return
	}

	response.Success(w, results)
}

// Geocode converts a plain text address to coordinates using Nominatim.
// Used as a fallback when the customer enters an address without using autocomplete.
//
// Query params:
//
//	address (required) — full address string
//
// Example: GET /api/v1/geo/geocode?address=Tom+Mboya+Street+Nairobi
func (h *Handler) Geocode(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if address == "" {
		response.BadRequest(w, "address is required")
		return
	}

	coords, err := h.geocoder.Geocode(r.Context(), address)
	if err != nil {
		response.NotFound(w, "could not find coordinates for this address")
		return
	}

	response.Success(w, coords)
}

// ── Helper ────────────────────────────────────────────────────────────────────

// resolveLocationKey returns the correct Redis key for the current request.
// Registered users: keyed by user ID.
// Guests:           keyed by X-Session-ID header (generated and persisted by the frontend).
func resolveLocationKey(r *http.Request) string {
	user := users.UserFromContext(r.Context())
	if user != nil {
		return LocationKey(user.ID, "")
	}
	sessionID := r.Header.Get("X-Session-ID")
	return LocationKey("", sessionID)
}