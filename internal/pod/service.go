package pod

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/storage"
	"github.com/Bessmack/hardware-store-api/pkg/logger"
	"github.com/Bessmack/hardware-store-api/pkg/otp"
)

// ── Interfaces ────────────────────────────────────────────────────────────────

// OrderReader fetches the order data the POD service needs.
// Implemented by orders.Repository — defined as an interface to avoid
// circular imports (orders imports pod for the dispatcher, pod imports orders).
type OrderReader interface {
	GetDeliveryInfo(ctx context.Context, orderID string) (*DeliveryInfo, error)
}

// DeliveryInfo is the minimal order data needed by the POD service.
type DeliveryInfo struct {
	OrderID     string
	OrderRef    string
	CustomerID  string
	StoreID     string
	DeliveryLat float64
	DeliveryLng float64
	Status      string
	DeliveredAt *time.Time
	Currency    string
}

// OrderStatusUpdater advances the order to "delivered" after successful POD.
// Implemented by orders.Service.
type OrderStatusUpdater interface {
	MarkDelivered(ctx context.Context, orderID, changedBy string) error
}

// CustomerInfoReader fetches the customer's phone and name for the OTP notification.
// Implemented by users.Repository.
type CustomerInfoReader interface {
	GetCustomerInfo(ctx context.Context, customerID string) (name, phone, email string, err error)
}

// PODNotifier sends delivery-specific notifications.
// Implemented by notifications.Service.
type PODNotifier interface {
	OutForDelivery(phone, name, orderRef, otp string)
	OrderDelivered(phone, email, name, orderRef string)
	DisputeRaised(phone, email, name, orderRef string)
}

// ── Service ───────────────────────────────────────────────────────────────────

type ServiceConfig struct {
	OTPLength            int     // from cfg.Rules.OTPLength (default 6)
	GPSToleranceMetres   float64 // from cfg.Rules.PODGPSToleranceMetres (default 200)
	DisputeWindowHours   int     // from cfg.Rules.DisputeWindowHours (default 24)
}

type Service struct {
	repo      *Repository
	orders    OrderReader
	updater   OrderStatusUpdater
	customers CustomerInfoReader
	notifier  PODNotifier
	storage   storage.Storage
	cfg       ServiceConfig
}

func NewService(
	repo *Repository,
	orders OrderReader,
	updater OrderStatusUpdater,
	customers CustomerInfoReader,
	notifier PODNotifier,
	storage storage.Storage,
	cfg ServiceConfig,
) *Service {
	return &Service{
		repo:      repo,
		orders:    orders,
		updater:   updater,
		customers: customers,
		notifier:  notifier,
		storage:   storage,
		cfg:       cfg,
	}
}

// ── Dispatch — called when order → out_for_delivery ───────────────────────────

// Dispatch generates an OTP, creates the POD record, and sends the OTP to the
// customer via WhatsApp. Called by orders.Service.UpdateStatus when the
// transition to out_for_delivery is applied.
//
// This satisfies the orders.PODDispatcher interface.
func (s *Service) Dispatch(ctx context.Context, orderID, customerID, customerPhone, customerName, orderRef string) error {
	code, err := otp.Generate(s.cfg.OTPLength)
	if err != nil {
		return fmt.Errorf("pod: failed to generate OTP: %w", err)
	}

	if _, err := s.repo.Create(ctx, orderID, code); err != nil {
		return fmt.Errorf("pod: failed to create POD record: %w", err)
	}

	// Send OTP via WhatsApp only — delivery person must see the code on the phone
	if s.notifier != nil {
		s.notifier.OutForDelivery(customerPhone, customerName, orderRef, code)
	}

	logger.Get().Info().
		Str("order", orderID).
		Str("order_ref", orderRef).
		Msg("pod: OTP generated and dispatched")

	return nil
}

// ── Submit — delivery person submits proof ────────────────────────────────────

// Submit validates all three POD layers and marks the order as delivered.
//
// Three-layer validation:
//  1. OTP     — must match what was sent to the customer
//  2. GPS     — delivery person must be within GPSToleranceMetres of the address
//  3. Photo   — must be uploaded successfully to Cloudinary
//
// All three must pass. If any layer fails, the delivery is rejected and the
// delivery person must try again.
func (s *Service) Submit(ctx context.Context, deliveryPersonID string, req SubmitPODRequest, photo multipart.File, photoHeader *multipart.FileHeader) (*SubmitPODResponse, error) {
	// Load the POD record
	pod, err := s.repo.GetByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("pod: no dispatch record found for this order")
	}

	if pod.OTPVerified {
		return nil, errors.New("pod: this delivery has already been submitted")
	}

	// Layer 1: OTP verification
	if req.OTP != pod.OTP {
		return nil, errors.New("incorrect OTP — please ask the customer for the code they received on WhatsApp")
	}

	// Load the order to get delivery coordinates
	info, err := s.orders.GetDeliveryInfo(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("pod: could not load order info: %w", err)
	}

	if info.DeliveryLat == 0 || info.DeliveryLng == 0 {
		return nil, errors.New("pod: this order has no delivery coordinates — cannot verify GPS")
	}

	// Layer 2: GPS proximity check
	distanceM := geo.DistanceMetres(info.DeliveryLat, info.DeliveryLng, req.Lat, req.Lng)
	if distanceM > s.cfg.GPSToleranceMetres {
		return nil, fmt.Errorf(
			"you are %.0fm away from the delivery address — please move within %.0fm and try again",
			distanceM, s.cfg.GPSToleranceMetres,
		)
	}

	// Layer 3: Photo upload to Cloudinary
	if photo == nil {
		return nil, errors.New("a delivery photo is required")
	}

	uploadResult, err := s.storage.Upload(ctx, photo, photoHeader.Filename, storage.FolderDeliveryPhotos)
	if err != nil {
		return nil, fmt.Errorf("pod: photo upload failed: %w", err)
	}

	// All three layers passed — record the submission
	if err := s.repo.Submit(ctx, req.OrderID, uploadResult.URL, uploadResult.PublicID, req.Lat, req.Lng, distanceM); err != nil {
		return nil, fmt.Errorf("pod: failed to record submission: %w", err)
	}

	// Advance order status to delivered
	if err := s.updater.MarkDelivered(ctx, req.OrderID, deliveryPersonID); err != nil {
		logger.Get().Error().Err(err).Str("order", req.OrderID).Msg("pod: failed to mark order delivered")
	}

	// Notify customer
	if s.notifier != nil && s.customers != nil {
		name, phone, email, _ := s.customers.GetCustomerInfo(ctx, info.CustomerID)
		go s.notifier.OrderDelivered(phone, email, name, info.OrderRef)
	}

	return &SubmitPODResponse{
		Message:   "Delivery confirmed successfully",
		DistanceM: distanceM,
		OrderRef:  info.OrderRef,
	}, nil
}

// ── Dispute — customer raises a dispute after delivery ────────────────────────

// RaiseDispute allows a customer to contest a delivery within the dispute window.
// An optional evidence photo can be uploaded alongside the description.
func (s *Service) RaiseDispute(ctx context.Context, customerID, orderID string, req RaiseDisputeRequest, evidence multipart.File, evidenceHeader *multipart.FileHeader) (*DisputeResponse, error) {
	// Load the POD record to get delivery time
	pod, err := s.repo.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, errors.New("no delivery record found for this order — disputes can only be raised for delivered orders")
	}

	if pod.DeliveredAt == nil {
		return nil, errors.New("this order has not been marked as delivered yet")
	}

	// Enforce dispute window
	windowEnd := pod.DeliveredAt.Add(time.Duration(s.cfg.DisputeWindowHours) * time.Hour)
	if time.Now().After(windowEnd) {
		return nil, fmt.Errorf(
			"the %d-hour dispute window has expired — disputes must be raised within %d hours of delivery",
			s.cfg.DisputeWindowHours, s.cfg.DisputeWindowHours,
		)
	}

	// Upload evidence photo if provided
	evidenceURL := ""
	evidencePublicID := ""
	if evidence != nil {
		result, err := s.storage.Upload(ctx, evidence, evidenceHeader.Filename, storage.FolderDisputeEvidence)
		if err != nil {
			return nil, fmt.Errorf("pod: evidence photo upload failed: %w", err)
		}
		evidenceURL = result.URL
		evidencePublicID = result.PublicID
	}

	dispute, err := s.repo.CreateDispute(ctx, orderID, customerID, req.Description, evidenceURL, evidencePublicID)
	if err != nil {
		return nil, fmt.Errorf("pod: failed to create dispute: %w", err)
	}

	// Notify customer and staff
	if s.notifier != nil && s.customers != nil {
		info, _ := s.orders.GetDeliveryInfo(ctx, orderID)
		if info != nil {
			name, phone, email, _ := s.customers.GetCustomerInfo(ctx, customerID)
			go s.notifier.DisputeRaised(phone, email, name, info.OrderRef)
		}
	}

	return toDisputeResponse(dispute), nil
}

// ── Staff operations ──────────────────────────────────────────────────────────

// GetPOD returns the proof of delivery record for a staff member to review.
func (s *Service) GetPOD(ctx context.Context, orderID string) (*PODResponse, error) {
	pod, err := s.repo.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return toPODResponse(pod), nil
}

// ResolveDispute closes a dispute with a staff resolution note.
func (s *Service) ResolveDispute(ctx context.Context, disputeID string, req ResolveDisputeRequest) error {
	return s.repo.ResolveDispute(ctx, disputeID, req.Status, req.Resolution)
}

// GetDispute returns the dispute for an order.
func (s *Service) GetDispute(ctx context.Context, orderID string) (*DisputeResponse, error) {
	dispute, err := s.repo.GetDisputeByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return toDisputeResponse(dispute), nil
}

// ── Mappers ───────────────────────────────────────────────────────────────────

func toPODResponse(p *ProofOfDelivery) *PODResponse {
	return &PODResponse{
		OrderID:     p.OrderID,
		OTPVerified: p.OTPVerified,
		DeliveryLat: p.DeliveryLat,
		DeliveryLng: p.DeliveryLng,
		DistanceM:   p.DistanceM,
		PhotoURL:    p.PhotoURL,
		DeliveredAt: p.DeliveredAt,
	}
}

func toDisputeResponse(d *Dispute) *DisputeResponse {
	return &DisputeResponse{
		ID:          d.ID,
		OrderID:     d.OrderID,
		Description: d.Description,
		EvidenceURL: d.EvidenceURL,
		Status:      d.Status,
		Resolution:  d.Resolution,
		CreatedAt:   d.CreatedAt,
	}
}