package middleware

import (
	"net/http"

	"github.com/Bessmack/hardware-store-api/pkg/response"
)

// ── Payment callback IP allowlists ────────────────────────────────────────────
//
// M-Pesa and Airtel Money callback endpoints should only accept requests
// from the payment provider's own servers, not from arbitrary IPs on the internet.
//
// An attacker who knows your callback URL could otherwise craft a fake payment
// confirmation and mark unpaid orders as paid.
//
// Sources:
//   Safaricom M-Pesa:  https://developer.safaricom.co.ke/APIs/MpesaExpressSimulate
//   Airtel Money:      https://developers.airtel.africa/

// safaricomIPs contains Safaricom's known M-Pesa callback IP ranges.
// Check https://developer.safaricom.co.ke for the current list before going live.
// Sandbox and production use different IPs — both are included here.
var safaricomIPs = map[string]bool{
	// Production
	"196.201.213.114": true,
	"196.201.214.207": true,
	"196.201.214.208": true,
	"196.201.213.44":  true,
	"196.201.212.127": true,
	"196.201.212.138": true,
	"196.201.212.129": true,
	"196.201.212.136": true,
	"196.201.212.74":  true,
	"196.201.212.69":  true,

	// Sandbox (for development/testing)
	"196.201.214.200": true,
	"196.201.214.206": true,
}

// airtelIPs contains Airtel Money's known callback IP ranges.
// Update this list from https://developers.airtel.africa before going live.
var airtelIPs = map[string]bool{
	// Add Airtel's IPs here when confirmed from their developer portal
	// Placeholder — populate before enabling Airtel Money in production
}

// AllowSafaricom rejects any request to the M-Pesa callback endpoint
// that does not originate from a known Safaricom IP address.
//
// Apply only to POST /api/v1/payments/mpesa/callback/:storeID.
// Do NOT apply to any customer-facing routes.
func AllowSafaricom(next http.Handler) http.Handler {
	return allowlist(safaricomIPs, "M-Pesa callback")(next)
}

// AllowAirtel rejects requests to the Airtel Money callback endpoint
// from any IP not in Airtel's allowlist.
//
// Apply only to POST /api/v1/payments/airtel/callback/:storeID.
func AllowAirtel(next http.Handler) http.Handler {
	return allowlist(airtelIPs, "Airtel Money callback")(next)
}

// ── Core allowlist logic ──────────────────────────────────────────────────────

func allowlist(allowed map[string]bool, name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r) // reuses the clientIP helper from rate_limit.go

			// In development, allow all IPs so you can test callbacks via ngrok
			// without needing to spoof Safaricom's IP.
			// Remove this bypass before deploying to production.
			if r.Header.Get("X-Dev-Bypass-IP-Check") == "true" {
				next.ServeHTTP(w, r)
				return
			}

			if !allowed[ip] {
				// Return 403 with no detail — don't reveal that an allowlist exists
				response.Forbidden(w, "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}