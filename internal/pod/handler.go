package pod

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/Bessmack/hardware-store-api/internal/middleware"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/Bessmack/hardware-store-api/pkg/validator"
	"github.com/go-chi/chi/v5"
)

const maxPhotoSize = 5 << 20 // 5 MB

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ── Routes (registered in server/routes.go) ───────────────────────────────────
//
// Staff (cashier+, StoreScope):
//   GET  /api/v1/store/orders/:orderID/pod         View POD record
//   GET  /api/v1/store/orders/:orderID/dispute     View dispute
//   PUT  /api/v1/store/disputes/:disputeID/resolve Resolve a dispute
//
// Delivery person (cashier+, StoreScope):
//   POST /api/v1/pod/submit                        Submit OTP + GPS + photo
//                                                  (multipart/form-data)
//
// Customer (RequireRole customer):
//   POST /api/v1/orders/:orderID/dispute           Raise a dispute
//                                                  (multipart/form-data, optional photo)

// ── Staff handlers ────────────────────────────────────────────────────────────

// GetPOD returns the proof of delivery record for a store's order.
func (h *Handler) GetPOD(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	orderID := chi.URLParam(r, "orderID")

	pod, err := h.service.GetPOD(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "no proof of delivery found for this order")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, pod)
}

// GetDispute returns the dispute raised against a store's order.
func (h *Handler) GetDispute(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	orderID := chi.URLParam(r, "orderID")

	dispute, err := h.service.GetDispute(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, ErrDisputeNotFound) {
			response.NotFound(w, "no dispute found for this order")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.Success(w, dispute)
}

// ResolveDispute closes a dispute with a resolution note. Admin+ only.
//
// Body: { status: "resolved"|"rejected", resolution: "..." }
func (h *Handler) ResolveDispute(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.AssertStoreScoped(w, r)
	if !ok {
		return
	}
	disputeID := chi.URLParam(r, "disputeID")

	var req ResolveDisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	if err := h.service.ResolveDispute(r.Context(), disputeID, req); err != nil {
		if errors.Is(err, ErrDisputeNotFound) {
			response.NotFound(w, "dispute not found")
			return
		}
		response.InternalServerError(w)
		return
	}
	response.NoContent(w)
}

// ── Delivery person handler ───────────────────────────────────────────────────

// SubmitPOD is called by the delivery person at the customer's location.
// Accepts multipart/form-data with:
//   - order_id (text)
//   - otp      (text)
//   - lat      (text, parsed as float)
//   - lng      (text, parsed as float)
//   - photo    (file, JPEG/PNG, max 5MB)
//
// All three layers (OTP + GPS + photo) must pass for delivery to be confirmed.
func (h *Handler) SubmitPOD(w http.ResponseWriter, r *http.Request) {
	by := users.UserFromContext(r.Context())

	if err := r.ParseMultipartForm(maxPhotoSize); err != nil {
		response.BadRequest(w, "request too large or not multipart/form-data")
		return
	}

	var req SubmitPODRequest
	req.OrderID = r.FormValue("order_id")
	req.OTP = r.FormValue("otp")

	_, err := parseFloat(r.FormValue("lat"), &req.Lat)
	if err != nil || req.Lat == 0 {
		response.UnprocessableEntity(w, "lat is required and must be a valid decimal number")
		return
	}
	_, err = parseFloat(r.FormValue("lng"), &req.Lng)
	if err != nil || req.Lng == 0 {
		response.UnprocessableEntity(w, "lng is required and must be a valid decimal number")
		return
	}

	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	photo, photoHeader, err := r.FormFile("photo")
	if err != nil {
		response.UnprocessableEntity(w, "a delivery photo is required")
		return
	}
	defer photo.Close()

	result, err := h.service.Submit(r.Context(), by.ID, req, photo, photoHeader)
	if err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}
	response.Success(w, result)
}

// ── Customer handler ──────────────────────────────────────────────────────────

// RaiseDispute lets a customer contest a delivery within 24 hours.
// Accepts multipart/form-data with:
//   - description (text, min 10 chars)
//   - evidence    (file, optional — JPEG/PNG photo of the issue)
func (h *Handler) RaiseDispute(w http.ResponseWriter, r *http.Request) {
	customer := users.UserFromContext(r.Context())
	orderID := chi.URLParam(r, "orderID")

	if err := r.ParseMultipartForm(maxPhotoSize); err != nil {
		response.BadRequest(w, "request too large or not multipart/form-data")
		return
	}

	req := RaiseDisputeRequest{
		Description: r.FormValue("description"),
	}
	if err := validator.Validate(req); err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}

	// Evidence photo is optional
	var evidence multipart.File
	var evidenceHeader *multipart.FileHeader
	evidence, evidenceHeader, _ = r.FormFile("evidence") // error ignored — optional
	if evidence != nil {
		defer evidence.Close()
	}

	dispute, err := h.service.RaiseDispute(r.Context(), customer.ID, orderID, req, evidence, evidenceHeader)
	if err != nil {
		response.UnprocessableEntity(w, err.Error())
		return
	}
	response.Created(w, dispute)
}

// ── Helper ────────────────────────────────────────────────────────────────────

func parseFloat(s string, dst *float64) (float64, error) {
	if s == "" {
		return 0, errors.New("empty string")
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	*dst = f
	return f, err
}