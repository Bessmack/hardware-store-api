package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Provider implements the notifications.Provider interface using Green API.
// Sign up at https://green-api.com — free tier available.
// Docs: https://green-api.com/en/docs/api/sending/SendMessage/
type Provider struct {
	apiURL     string
	mediaURL   string
	idInstance string
	apiToken   string
	phone      string
	httpClient *http.Client
}

// Config mirrors config.WhatsAppConfig — passed in from main.go.
type Config struct {
	APIURL     string
	MediaURL   string
	IDInstance string
	APIToken   string
	Phone      string
}

// New creates a new Green API WhatsApp provider.
func New(cfg Config) *Provider {
	return &Provider{
		apiURL:     cfg.APIURL,
		mediaURL:   cfg.MediaURL,
		idInstance: cfg.IDInstance,
		apiToken:   cfg.APIToken,
		phone:      cfg.Phone,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name satisfies the notifications.Provider interface.
func (p *Provider) Name() string {
	return "whatsapp"
}

// SendText sends a plain text WhatsApp message to the given phone number.
// phone must be in international format without the + sign (e.g. 254712345678).
func (p *Provider) SendText(phone, message string) error {
	endpoint := fmt.Sprintf(
		"%s/waInstance%s/sendMessage/%s",
		p.apiURL, p.idInstance, p.apiToken,
	)

	// Green API expects the chat ID in the format: <phone>@c.us
	payload := map[string]interface{}{
		"chatId":  fmt.Sprintf("%s@c.us", phone),
		"message": message,
	}

	return p.post(endpoint, payload)
}

// SendFile sends a media file (e.g. a PDF invoice) to the given phone number.
// fileURL must be publicly accessible — Cloudinary URLs work perfectly here.
// caption is the text shown below the file (can be empty).
func (p *Provider) SendFile(phone, fileURL, filename, caption string) error {
	endpoint := fmt.Sprintf(
		"%s/waInstance%s/sendFileByUrl/%s",
		p.mediaURL, p.idInstance, p.apiToken,
	)

	payload := map[string]interface{}{
		"chatId":   fmt.Sprintf("%s@c.us", phone),
		"urlFile":  fileURL,
		"fileName": filename,
		"caption":  caption,
	}

	return p.post(endpoint, payload)
}

// ── Pre-built message builders ────────────────────────────────────────────────
// Each function returns the formatted message string for a specific event.
// Call SendText() with the result.

// OrderConfirmedMessage returns the message sent when payment is confirmed.
func OrderConfirmedMessage(orderRef, storeName string, totalKES float64) string {
	return fmt.Sprintf(
		"✅ *Payment Confirmed*\n\n"+
			"Your payment for order *#%s* has been received.\n"+
			"Your order is being prepared at *%s*.\n\n"+
			"*Total paid:* KES %.0f\n\n"+
			"We will notify you as your order progresses.",
		orderRef, storeName, totalKES,
	)
}

// OrderStatusMessage returns the message sent on any status change.
func OrderStatusMessage(orderRef, statusLabel, description string) string {
	return fmt.Sprintf(
		"📦 *Order Update*\n\n"+
			"Order *#%s* status: *%s*\n"+
			"%s",
		orderRef, statusLabel, description,
	)
}

// OutForDeliveryMessage returns the message sent when the rider has dispatched.
// Includes the OTP the customer must give to the delivery person.
func OutForDeliveryMessage(orderRef, otp string) string {
	return fmt.Sprintf(
		"🚚 *Your delivery is on its way!*\n\n"+
			"Order *#%s* is heading to you.\n\n"+
			"When the delivery person arrives, give them this code:\n\n"+
			"*%s*\n\n"+
			"⚠️ Do not share this code until your goods have arrived safely.",
		orderRef, otp,
	)
}

// OrderDeliveredMessage returns the message sent after successful delivery.
func OrderDeliveredMessage(orderRef string) string {
	return fmt.Sprintf(
		"🎉 *Order Delivered!*\n\n"+
			"Order *#%s* has been successfully delivered.\n"+
			"Thank you for shopping with us!\n\n"+
			"If you have any issues, please contact us within 24 hours.",
		orderRef,
	)
}

// DisputeRaisedMessage confirms to the customer that their dispute was received.
func DisputeRaisedMessage(orderRef string) string {
	return fmt.Sprintf(
		"📋 *Dispute Received*\n\n"+
			"We have received your dispute for order *#%s*.\n"+
			"Our support team will review it and get back to you shortly.",
		orderRef,
	)
}

// ── Internal helper ───────────────────────────────────────────────────────────

func (p *Provider) post(endpoint string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("whatsapp: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("whatsapp: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("whatsapp: unexpected status %d from Green API", resp.StatusCode)
	}

	return nil
}