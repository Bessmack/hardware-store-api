package categories

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ── Public ────────────────────────────────────────────────────────────────────

// ListAll returns every category with its subcategories embedded.
// The frontend calls this once on page load and caches the result — no
// separate subcategory requests needed.
func (s *Service) ListAll(ctx context.Context) ([]CategoryResponse, error) {
	cats, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]CategoryResponse, 0, len(cats))
	for _, c := range cats {
		subs, err := s.repo.ListSubcategories(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, toCategoryResponse(c, subs))
	}
	return result, nil
}

// GetBySlug returns a single category with its subcategories.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*CategoryResponse, error) {
	c, err := s.repo.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	subs, err := s.repo.ListSubcategories(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	resp := toCategoryResponse(*c, subs)
	return &resp, nil
}

// ListSubcategories returns subcategories for a given category slug.
func (s *Service) ListSubcategories(ctx context.Context, categorySlug string) ([]SubcategoryResponse, error) {
	c, err := s.repo.GetCategoryBySlug(ctx, categorySlug)
	if err != nil {
		return nil, err
	}
	subs, err := s.repo.ListSubcategories(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	result := make([]SubcategoryResponse, 0, len(subs))
	for _, sub := range subs {
		result = append(result, toSubcategoryResponse(sub))
	}
	return result, nil
}

// ── Admin mutations ───────────────────────────────────────────────────────────

func (s *Service) CreateCategory(ctx context.Context, req CreateCategoryRequest) (*CategoryResponse, error) {
	c, err := s.repo.CreateCategory(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := toCategoryResponse(*c, nil)
	return &resp, nil
}

func (s *Service) UpdateCategory(ctx context.Context, id string, req UpdateCategoryRequest) (*CategoryResponse, error) {
	c, err := s.repo.UpdateCategory(ctx, id, req)
	if err != nil {
		return nil, err
	}
	subs, err := s.repo.ListSubcategories(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	resp := toCategoryResponse(*c, subs)
	return &resp, nil
}

func (s *Service) DeleteCategory(ctx context.Context, id string) error {
	return s.repo.DeleteCategory(ctx, id)
}

func (s *Service) CreateSubcategory(ctx context.Context, categoryID string, req CreateSubcategoryRequest) (*SubcategoryResponse, error) {
	// Verify the category exists before inserting.
	if _, err := s.repo.GetCategoryByID(ctx, categoryID); err != nil {
		return nil, err
	}
	sub, err := s.repo.CreateSubcategory(ctx, categoryID, req)
	if err != nil {
		return nil, err
	}
	resp := toSubcategoryResponse(*sub)
	return &resp, nil
}

func (s *Service) UpdateSubcategory(ctx context.Context, id string, req UpdateSubcategoryRequest) (*SubcategoryResponse, error) {
	sub, err := s.repo.UpdateSubcategory(ctx, id, req)
	if err != nil {
		return nil, err
	}
	resp := toSubcategoryResponse(*sub)
	return &resp, nil
}

func (s *Service) DeleteSubcategory(ctx context.Context, id string) error {
	return s.repo.DeleteSubcategory(ctx, id)
}