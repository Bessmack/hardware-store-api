package categories

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// ── Categories ────────────────────────────────────────────────────────────────

func (r *Repository) CreateCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO categories (name, slug, icon, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, slug, COALESCE(icon,''), sort_order, created_at
	`, req.Name, req.Slug, req.Icon, req.SortOrder)

	c, err := scanCategory(row)
	if err != nil {
		return nil, mapErr(err)
	}
	return c, nil
}

func (r *Repository) GetCategoryBySlug(ctx context.Context, slug string) (*Category, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, slug, COALESCE(icon,''), sort_order, created_at
		FROM categories WHERE slug = $1
	`, slug)
	c, err := scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (r *Repository) GetCategoryByID(ctx context.Context, id string) (*Category, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, slug, COALESCE(icon,''), sort_order, created_at
		FROM categories WHERE id = $1
	`, id)
	c, err := scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// ListCategories returns all categories ordered by sort_order then name.
// Subcategories are NOT included — call ListSubcategories separately per
// category, or use ListCategoriesWithSubs for the all-in-one response.
func (r *Repository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, name, slug, COALESCE(icon,''), sort_order, created_at
		FROM categories
		ORDER BY sort_order ASC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Icon, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("categories: scan: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *Repository) UpdateCategory(ctx context.Context, id string, req UpdateCategoryRequest) (*Category, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE categories SET
			name       = COALESCE(NULLIF($1,''), name),
			icon       = COALESCE(NULLIF($2,''), icon),
			sort_order = COALESCE($3, sort_order)
		WHERE id = $4
		RETURNING id, name, slug, COALESCE(icon,''), sort_order, created_at
	`, req.Name, req.Icon, req.SortOrder, id)
	c, err := scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, mapErr(err)
}

func (r *Repository) DeleteCategory(ctx context.Context, id string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Subcategories ─────────────────────────────────────────────────────────────

func (r *Repository) CreateSubcategory(ctx context.Context, categoryID string, req CreateSubcategoryRequest) (*Subcategory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO subcategories (category_id, name, slug, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING id, category_id, name, slug, sort_order, created_at
	`, categoryID, req.Name, req.Slug, req.SortOrder)

	s, err := scanSubcategory(row)
	if err != nil {
		return nil, mapErr(err)
	}
	return s, nil
}

func (r *Repository) GetSubcategoryByID(ctx context.Context, id string) (*Subcategory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, category_id, name, slug, sort_order, created_at
		FROM subcategories WHERE id = $1
	`, id)
	s, err := scanSubcategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSubNotFound
	}
	return s, err
}

func (r *Repository) ListSubcategories(ctx context.Context, categoryID string) ([]Subcategory, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, category_id, name, slug, sort_order, created_at
		FROM subcategories
		WHERE category_id = $1
		ORDER BY sort_order ASC, name ASC
	`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Subcategory
	for rows.Next() {
		var s Subcategory
		if err := rows.Scan(&s.ID, &s.CategoryID, &s.Name, &s.Slug, &s.SortOrder, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("categories: scan subcategory: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *Repository) UpdateSubcategory(ctx context.Context, id string, req UpdateSubcategoryRequest) (*Subcategory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		UPDATE subcategories SET
			name       = COALESCE(NULLIF($1,''), name),
			sort_order = COALESCE($2, sort_order)
		WHERE id = $3
		RETURNING id, category_id, name, slug, sort_order, created_at
	`, req.Name, req.SortOrder, id)
	s, err := scanSubcategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSubNotFound
	}
	return s, mapErr(err)
}

func (r *Repository) DeleteSubcategory(ctx context.Context, id string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM subcategories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrSubNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func scanCategory(row pgx.Row) (*Category, error) {
	var c Category
	if err := row.Scan(&c.ID, &c.Name, &c.Slug, &c.Icon, &c.SortOrder, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func scanSubcategory(row pgx.Row) (*Subcategory, error) {
	var s Subcategory
	if err := row.Scan(&s.ID, &s.CategoryID, &s.Name, &s.Slug, &s.SortOrder, &s.CreatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrSlugTaken
	}
	return err
}