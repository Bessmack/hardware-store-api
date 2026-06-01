package notifications

// Notification is the data passed to every provider when sending a message.
// Not every field is used by every provider — WhatsApp uses Phone,
// email uses Email; a provider ignores fields it does not need.
type Notification struct {
	// Recipient identity
	Phone string // international format, no + (e.g. 254712345678)
	Email string // recipient email address
	Name  string // recipient display name — used in email greetings

	// Content
	Subject  string // email subject line (ignored by WhatsApp)
	Body     string // plain-text / WhatsApp message body
	HTMLBody string // HTML email body (when set, overrides Body for email)
}

// Provider is the interface every notification channel must implement.
// Adding a new channel (e.g. SMS, push notification) means creating a new
// file that satisfies this interface and registering it — nothing else changes.
type Provider interface {
	// Name returns the unique identifier for this provider (e.g. "whatsapp", "email").
	Name() string

	// Send delivers a notification. Implementations must be safe to call
	// concurrently — the NotificationService fans out to all providers in parallel.
	Send(n Notification) error
}