package categories

import (
	"errors"
	"time"
)

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrNotFound    = errors.New("category not found")
	ErrSubNotFound = errors.New("subcategory not found")
	ErrSlugTaken   = errors.New("slug is already in use")
)

// ── Core models ───────────────────────────────────────────────────────────────

type Category struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Slug      string    `db:"slug"`
	Icon      string    `db:"icon"`  // Lucide component name, e.g. "Zap"
	SortOrder int       `db:"sort_order"`
	CreatedAt time.Time `db:"created_at"`
}

type Subcategory struct {
	ID         string    `db:"id"`
	CategoryID string    `db:"category_id"`
	Name       string    `db:"name"`
	Slug       string    `db:"slug"`
	SortOrder  int       `db:"sort_order"`
	CreatedAt  time.Time `db:"created_at"`
}

// ── Request types ─────────────────────────────────────────────────────────────

type CreateCategoryRequest struct {
	Name      string `json:"name"       validate:"required"`
	Slug      string `json:"slug"       validate:"required"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
}

type UpdateCategoryRequest struct {
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder *int   `json:"sort_order"`
}

type CreateSubcategoryRequest struct {
	Name      string `json:"name" validate:"required"`
	Slug      string `json:"slug" validate:"required"`
	SortOrder int    `json:"sort_order"`
}

type UpdateSubcategoryRequest struct {
	Name      string `json:"name"`
	SortOrder *int   `json:"sort_order"`
}

// ── Response types ────────────────────────────────────────────────────────────

type SubcategoryResponse struct {
	ID         string    `json:"id"`
	CategoryID string    `json:"category_id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	SortOrder  int       `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
}

// CategoryResponse always embeds its subcategories — the frontend never
// fetches categories and subcategories in separate round trips.
type CategoryResponse struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Slug          string                `json:"slug"`
	Icon          string                `json:"icon"`
	SortOrder     int                   `json:"sort_order"`
	Subcategories []SubcategoryResponse `json:"subcategories"`
	CreatedAt     time.Time             `json:"created_at"`
}

// ── Mappers ───────────────────────────────────────────────────────────────────

func toSubcategoryResponse(s Subcategory) SubcategoryResponse {
	return SubcategoryResponse{
		ID:         s.ID,
		CategoryID: s.CategoryID,
		Name:       s.Name,
		Slug:       s.Slug,
		SortOrder:  s.SortOrder,
		CreatedAt:  s.CreatedAt,
	}
}

func toCategoryResponse(c Category, subs []Subcategory) CategoryResponse {
	subResponses := make([]SubcategoryResponse, 0, len(subs))
	for _, s := range subs {
		subResponses = append(subResponses, toSubcategoryResponse(s))
	}
	return CategoryResponse{
		ID:            c.ID,
		Name:          c.Name,
		Slug:          c.Slug,
		Icon:          c.Icon,
		SortOrder:     c.SortOrder,
		Subcategories: subResponses,
		CreatedAt:     c.CreatedAt,
	}
}