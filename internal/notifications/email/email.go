package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
)

// Provider implements the notifications.Provider interface using plain SMTP.
// Works with Gmail (App Password), Outlook, or any SMTP relay.
type Provider struct {
	host     string
	port     int
	user     string
	password string
	fromName string
}

// Config mirrors config.EmailConfig — passed in from main.go.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	FromName string
}

// New creates a new SMTP email provider.
func New(cfg Config) *Provider {
	return &Provider{
		host:     cfg.Host,
		port:     cfg.Port,
		user:     cfg.User,
		password: cfg.Password,
		fromName: cfg.FromName,
	}
}

// Name satisfies the notifications.Provider interface.
func (p *Provider) Name() string {
	return "email"
}

// Send delivers an email to the given address.
// subject and htmlBody are both required.
func (p *Provider) Send(to, subject, htmlBody string) error {
	auth := smtp.PlainAuth("", p.user, p.password, p.host)

	from := fmt.Sprintf("%s <%s>", p.fromName, p.user)

	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n",
		from, to, subject,
	)

	message := []byte(headers + htmlBody)

	addr := fmt.Sprintf("%s:%d", p.host, p.port)

	if err := smtp.SendMail(addr, auth, p.user, []string{to}, message); err != nil {
		return fmt.Errorf("email: failed to send to %s: %w", to, err)
	}

	return nil
}

// ── Email templates ───────────────────────────────────────────────────────────
// Each template function builds a self-contained HTML email body.
// Inline styles are used intentionally — most email clients strip <style> blocks.

// OrderConfirmedBody returns the HTML body for a payment-confirmed email.
func OrderConfirmedBody(data OrderConfirmedData) (string, error) {
	return render(orderConfirmedTpl, data)
}

// OrderStatusUpdateBody returns the HTML body for a status-change email.
func OrderStatusUpdateBody(data OrderStatusData) (string, error) {
	return render(orderStatusTpl, data)
}

// OrderDeliveredBody returns the HTML body for a delivery-complete email.
func OrderDeliveredBody(data OrderDeliveredData) (string, error) {
	return render(orderDeliveredTpl, data)
}

// ── Template data structs ─────────────────────────────────────────────────────

type OrderConfirmedData struct {
	CustomerName string
	OrderRef     string
	StoreName    string
	TotalKES     float64
	AppName      string
	LogoURL      string
}

type OrderStatusData struct {
	CustomerName string
	OrderRef     string
	StatusLabel  string
	Description  string
	AppName      string
	LogoURL      string
}

type OrderDeliveredData struct {
	CustomerName string
	OrderRef     string
	AppName      string
	LogoURL      string
}

// ── Template renderer ─────────────────────────────────────────────────────────

func render(tpl string, data interface{}) (string, error) {
	t, err := template.New("email").Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("email: template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("email: template render error: %w", err)
	}

	return buf.String(), nil
}

// ── HTML templates ────────────────────────────────────────────────────────────

const orderConfirmedTpl = `
<!DOCTYPE html>
<html>
<body style="font-family:Arial,sans-serif;background:#f5f5f5;margin:0;padding:20px;">
  <div style="max-width:600px;margin:0 auto;background:#ffffff;border-radius:8px;overflow:hidden;">

    <div style="background:#2E7D32;padding:24px;text-align:center;">
      <img src="{{.LogoURL}}" alt="{{.AppName}}" style="height:48px;" />
    </div>

    <div style="padding:32px;">
      <h2 style="color:#212121;margin-top:0;">Payment Confirmed ✅</h2>
      <p style="color:#616161;">Hi {{.CustomerName}},</p>
      <p style="color:#616161;">
        Your payment for order <strong>#{{.OrderRef}}</strong> has been received.
        Your order is now being prepared at <strong>{{.StoreName}}</strong>.
      </p>

      <div style="background:#f5f5f5;border-radius:6px;padding:16px;margin:24px 0;">
        <p style="margin:0;color:#212121;"><strong>Order:</strong> #{{.OrderRef}}</p>
        <p style="margin:8px 0 0;color:#212121;"><strong>Total Paid:</strong> KES {{.TotalKES}}</p>
        <p style="margin:8px 0 0;color:#212121;"><strong>Store:</strong> {{.StoreName}}</p>
      </div>

      <p style="color:#616161;">We will notify you as your order progresses.</p>
    </div>

    <div style="background:#f5f5f5;padding:16px;text-align:center;">
      <p style="color:#9E9E9E;font-size:12px;margin:0;">
        © {{.AppName}}. If you did not place this order, contact us immediately.
      </p>
    </div>

  </div>
</body>
</html>`

const orderStatusTpl = `
<!DOCTYPE html>
<html>
<body style="font-family:Arial,sans-serif;background:#f5f5f5;margin:0;padding:20px;">
  <div style="max-width:600px;margin:0 auto;background:#ffffff;border-radius:8px;overflow:hidden;">

    <div style="background:#2E7D32;padding:24px;text-align:center;">
      <img src="{{.LogoURL}}" alt="{{.AppName}}" style="height:48px;" />
    </div>

    <div style="padding:32px;">
      <h2 style="color:#212121;margin-top:0;">Order Update</h2>
      <p style="color:#616161;">Hi {{.CustomerName}},</p>
      <p style="color:#616161;">
        Your order <strong>#{{.OrderRef}}</strong> status has been updated.
      </p>

      <div style="background:#E8F5E9;border-left:4px solid #2E7D32;border-radius:4px;padding:16px;margin:24px 0;">
        <p style="margin:0;color:#1B5E20;font-weight:bold;">{{.StatusLabel}}</p>
        <p style="margin:8px 0 0;color:#2E7D32;">{{.Description}}</p>
      </div>
    </div>

    <div style="background:#f5f5f5;padding:16px;text-align:center;">
      <p style="color:#9E9E9E;font-size:12px;margin:0;">© {{.AppName}}</p>
    </div>

  </div>
</body>
</html>`

const orderDeliveredTpl = `
<!DOCTYPE html>
<html>
<body style="font-family:Arial,sans-serif;background:#f5f5f5;margin:0;padding:20px;">
  <div style="max-width:600px;margin:0 auto;background:#ffffff;border-radius:8px;overflow:hidden;">

    <div style="background:#2E7D32;padding:24px;text-align:center;">
      <img src="{{.LogoURL}}" alt="{{.AppName}}" style="height:48px;" />
    </div>

    <div style="padding:32px;text-align:center;">
      <div style="font-size:48px;">✅</div>
      <h2 style="color:#212121;">Order Delivered!</h2>
      <p style="color:#616161;">Hi {{.CustomerName}},</p>
      <p style="color:#616161;">
        Your order <strong>#{{.OrderRef}}</strong> has been successfully delivered.
        Thank you for shopping with us!
      </p>
      <p style="color:#9E9E9E;font-size:13px;">
        If you have any issues with your order, please contact us within 24 hours.
      </p>
    </div>

    <div style="background:#f5f5f5;padding:16px;text-align:center;">
      <p style="color:#9E9E9E;font-size:12px;margin:0;">© {{.AppName}}</p>
    </div>

  </div>
</body>
</html>`