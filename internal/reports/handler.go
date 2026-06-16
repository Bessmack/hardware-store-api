package reports

import (
	"net/http"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/middleware"
	"github.com/Bessmack/hardware-store-api/pkg/response"
)

// Handler exposes two read-only reporting endpoints.
// No write operations — reports are always derived from existing data.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Routes (registered in server/routes.go) ───────────────────────────────────
//
// Store admin:
//   GET /api/v1/store/report
//       ?from=2024-01-01   (optional, defaults to first day of current month)
//       ?to=2024-01-31     (optional, defaults to now)
//
// Superadmin:
//   GET /api/v1/reports/global
//       ?from=2024-01-01
//       ?to=2024-01-31

// StoreReport returns a full report for the scoped store.
// Accessible to admin and superadmin of that store.
func (h *Handler) StoreReport(w http.ResponseWriter, r *http.Request) {
	storeID, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}

	f := parseFilter(r)

	report, err := h.service.GetStoreReport(r.Context(), storeID, f)
	if err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}
	response.Success(w, report)
}

// GlobalReport returns a platform-wide report across all stores.
// Superadmin only — route is protected by RequireRole("superadmin") in routes.go.
func (h *Handler) GlobalReport(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)

	report, err := h.service.GetGlobalReport(r.Context(), f)
	if err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}
	response.Success(w, report)
}

// ── Helper ────────────────────────────────────────────────────────────────────

// parseFilter reads ?from and ?to query parameters as YYYY-MM-DD dates.
// Zero values are left unset — the service fills in sensible defaults.
func parseFilter(r *http.Request) ReportFilter {
	var f ReportFilter
	const layout = "2006-01-02"

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(layout, v); err == nil {
			f.From = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(layout, v); err == nil {
			// Set to end of day so the entire "to" day is included
			f.To = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
		}
	}
	return f
}