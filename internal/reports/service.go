package reports

import (
	"context"
	"fmt"
	"time"
)

// Service provides report generation for store admins and superadmins.
// It is intentionally thin — all heavy lifting is SQL in the repository.
// The service's job is to validate the filter and enforce business rules (e.g. max period length, minimum date).
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ── Store report ──────────────────────────────────────────────────────────────

// GetStoreReport returns a full report for a single store.
// Available to store admin and superadmin.
func (s *Service) GetStoreReport(ctx context.Context, storeID string, f ReportFilter) (*StoreReport, error) {
	f = normalise(f)
	if err := validateFilter(f); err != nil {
		return nil, err
	}
	return s.repo.GetStoreReport(ctx, storeID, f)
}

// ── Global report ─────────────────────────────────────────────────────────────

// GetGlobalReport returns a platform-wide report across all stores.
// Superadmin only — enforced in the handler via RequireRole.
func (s *Service) GetGlobalReport(ctx context.Context, f ReportFilter) (*GlobalReport, error) {
	f = normalise(f)
	if err := validateFilter(f); err != nil {
		return nil, err
	}
	return s.repo.GetGlobalReport(ctx, f)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// normalise fills in zero-value From/To with sensible defaults:
//   - From  → first day of the current calendar month at midnight
//   - To    → right now
func normalise(f ReportFilter) ReportFilter {
	now := time.Now()
	if f.From.IsZero() {
		f.From = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
	if f.To.IsZero() {
		f.To = now
	}
	return f
}

// validateFilter rejects filters that would produce unreasonably large queries.
// Maximum period: 12 months.
func validateFilter(f ReportFilter) error {
	if f.To.Before(f.From) {
		return fmt.Errorf("reports: 'to' date must be after 'from' date")
	}
	if f.From.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return fmt.Errorf("reports: 'from' date cannot be before 2020-01-01")
	}
	if f.To.Sub(f.From) > 366*24*time.Hour {
		return fmt.Errorf("reports: report period cannot exceed 12 months")
	}
	return nil
}