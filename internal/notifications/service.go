package notifications

import (
	"fmt"
	"sync"

	"github.com/Bessmack/hardware-store-api/pkg/logger"
)

// Service fans out a notification to all registered providers concurrently.
// Failures from individual providers are logged but do not fail the call —
// a WhatsApp failure should never prevent an email from being sent.
type Service struct {
	registry *Registry
}

func NewService(registry *Registry) *Service {
	return &Service{registry: registry}
}

// Send delivers a notification to all registered providers in parallel.
// It always returns immediately — send errors are logged, not propagated.
func (s *Service) Send(n Notification) {
	providers := s.registry.All()
	if len(providers) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, p := range providers {
		wg.Add(1)
		go func(provider Provider) {
			defer wg.Done()
			if err := provider.Send(n); err != nil {
				logger.Get().Error().
					Err(err).
					Str("provider", provider.Name()).
					Str("phone", n.Phone).
					Str("email", n.Email).
					Msg("notification delivery failed")
			}
		}(p)
	}
	wg.Wait()
}

// SendVia delivers a notification through a specific provider only.
// Useful when one channel is more appropriate for a given event
// (e.g. sending a delivery OTP via WhatsApp only, not email).
func (s *Service) SendVia(providerName string, n Notification) error {
	p, err := s.registry.Get(providerName)
	if err != nil {
		return err
	}
	return p.Send(n)
}

// ── Pre-built event notifications ─────────────────────────────────────────────
// Each method builds the correct Notification for a specific system event
// and sends it through all channels. Callers pass only the data they have;
// formatting is handled here so no domain package ever builds message strings.

// OrderConfirmed notifies the customer that payment was received.
func (s *Service) OrderConfirmed(phone, email, name, orderRef, storeName string, totalKES float64) {
	s.Send(Notification{
		Phone: phone,
		Email: email,
		Name:  name,
		Subject: fmt.Sprintf("Payment confirmed — Order #%s", orderRef),
		Body: fmt.Sprintf(
			"Hi %s, your payment for order #%s has been received.\n"+
				"Your order is being prepared at %s.\nTotal paid: KES %.0f",
			name, orderRef, storeName, totalKES,
		),
	})
}

// OrderStatusChanged notifies the customer of any status transition.
func (s *Service) OrderStatusChanged(phone, email, name, orderRef, statusLabel, description string) {
	s.Send(Notification{
		Phone: phone,
		Email: email,
		Name:  name,
		Subject: fmt.Sprintf("Order update — #%s: %s", orderRef, statusLabel),
		Body: fmt.Sprintf(
			"Hi %s, your order #%s is now: %s\n%s",
			name, orderRef, statusLabel, description,
		),
	})
}

// OutForDelivery notifies the customer their order is on its way
// and includes the OTP they must give to the delivery person.
// Sent via WhatsApp only — OTP must be on the customer's phone.
func (s *Service) OutForDelivery(phone, name, orderRef, otp string) {
	_ = s.SendVia("whatsapp", Notification{
		Phone: phone,
		Name:  name,
		Body: fmt.Sprintf(
			"Hi %s, your order #%s is on its way!\n\n"+
				"Give this code to the delivery person when they arrive:\n\n"+
				"*%s*\n\n"+
				"Do NOT share this code until your goods have arrived safely.",
			name, orderRef, otp,
		),
	})
}

// OrderDelivered notifies the customer that their order was successfully delivered.
func (s *Service) OrderDelivered(phone, email, name, orderRef string) {
	s.Send(Notification{
		Phone: phone,
		Email: email,
		Name:  name,
		Subject: fmt.Sprintf("Delivered — Order #%s", orderRef),
		Body: fmt.Sprintf(
			"Hi %s, your order #%s has been delivered. Thank you for shopping with us!\n"+
				"If you have any issues, please contact us within 24 hours.",
			name, orderRef,
		),
	})
}

// DisputeRaised confirms to the customer that their dispute was received.
func (s *Service) DisputeRaised(phone, email, name, orderRef string) {
	s.Send(Notification{
		Phone: phone,
		Email: email,
		Name:  name,
		Subject: fmt.Sprintf("Dispute received — Order #%s", orderRef),
		Body: fmt.Sprintf(
			"Hi %s, we've received your dispute for order #%s.\n"+
				"Our support team will review it and get back to you shortly.",
			name, orderRef,
		),
	})
}

// WelcomeCustomer sends a welcome message after successful registration.
func (s *Service) WelcomeCustomer(phone, email, name string) {
	s.Send(Notification{
		Phone:   phone,
		Email:   email,
		Name:    name,
		Subject: "Welcome!",
		Body: fmt.Sprintf(
			"Hi %s, welcome! You can now browse our products, place orders, and track deliveries.",
			name,
		),
	})
}