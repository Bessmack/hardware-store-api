package categories

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/Bessmack/hardware-store-api/pkg/validator"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Public routes ─────────────────────────────────────────────────────────────
// GET  /categories
// GET  /categories/{slug}
// GET  /categories/{slug}/subcategories

// List returns all categories with their subcategories embedded.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	cats, err := h.service.ListAll(r.Context())
	if err != nil {
		response.InternalServerError(w)
		return
	}
	response.Success(w, cats)
}

// Get returns a single category with its subcategories.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cat, err := h.service.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "category not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, cat)
}

// ListSubcategories returns subcategories for a given category slug.
func (h *Handler) ListSubcategories(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	subs, err := h.service.ListSubcategories(r.Context(), slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "category not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, subs)
}

// ── SuperAdmin routes ─────────────────────────────────────────────────────────
// POST   /admin/categories
// PUT    /admin/categories/{id}
// DELETE /admin/categories/{id}
// POST   /admin/categories/{id}/subcategories
// PUT    /admin/subcategories/{id}
// DELETE /admin/subcategories/{id}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	cat, err := h.service.CreateCategory(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrSlugTaken) {
			response.UnprocessableEntity(w, err.Error())
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Created(w, cat)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	cat, err := h.service.UpdateCategory(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "category not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, cat)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteCategory(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "category not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.NoContent(w)
}

func (h *Handler) CreateSubcategory(w http.ResponseWriter, r *http.Request) {
	categoryID := chi.URLParam(r, "id")

	var req CreateSubcategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	sub, err := h.service.CreateSubcategory(r.Context(), categoryID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(w, "category not found")
		case errors.Is(err, ErrSlugTaken):
			response.UnprocessableEntity(w, "slug is already used in this category")
		default:
			response.InternalServerError(w)
		}
		return
	}
	response.Created(w, sub)
}

func (h *Handler) UpdateSubcategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req UpdateSubcategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	sub, err := h.service.UpdateSubcategory(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrSubNotFound) {
			response.NotFound(w, "subcategory not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, sub)
}

func (h *Handler) DeleteSubcategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteSubcategory(r.Context(), id); err != nil {
		if errors.Is(err, ErrSubNotFound) {
			response.NotFound(w, "subcategory not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.NoContent(w)
}