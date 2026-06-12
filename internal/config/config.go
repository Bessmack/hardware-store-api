package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration loaded from environment variables.
// One instance is created in main.go and passed to every component that needs it.
type Config struct {
	App        AppConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	JWT        JWTConfig
	Mpesa      MpesaConfig
	Airtel     AirtelConfig
	WhatsApp   WhatsAppConfig
	Email      EmailConfig
	Geo        GeoConfig
	Security   SecurityConfig
	Cloudinary CloudinaryConfig
	Rules      RulesConfig
}

// ── Section structs ───────────────────────────────────────────────────────────

type AppConfig struct {
	Name    string
	LogoURL string
	Env     string
	Port    string
	URL     string // frontend origin — used for CORS and email redirect links
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

// JWTConfig controls token lifetimes.
//
// Two-token strategy:
//   - Access token  short-lived (30 min default), sent with every API request
//   - Refresh token long-lived (7 days default), stored in Redis, used only to
//     obtain a new access token; rotated on every use
type JWTConfig struct {
	Secret              string
	AccessExpiryMinutes int
	RefreshExpiryDays   int
}

// MpesaConfig holds Daraja API credentials.
//
// ConsumerKey/Secret  Global credentials for generating OAuth access tokens.
//                     One pair per application — not per store.
//
// Shortcode/Passkey   Default STK push credentials used when a store has not
//                     yet been configured with its own in the stores table.
//                     Once a store's own credentials are set, those take over.
//
// CallbackURL         Public HTTPS endpoint Safaricom posts payment results to.
//                     In development, use ngrok: ngrok http 8080
type MpesaConfig struct {
	ConsumerKey    string
	ConsumerSecret string
	Shortcode      string
	Passkey        string
	CallbackURL    string
	BaseURL        string
}

type AirtelConfig struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
}

// WhatsAppConfig holds Green API credentials.
// Sign up at https://green-api.com — free tier available.
//
//	APIURL      base URL for all text/message API calls
//	MediaURL    base URL for sending media (images, PDFs)
//	IDInstance  instance ID shown on the Green API dashboard
//	APIToken    authentication token (treat like a password)
//	Phone       sender number — no + prefix (e.g. 254712345678)
type WhatsAppConfig struct {
	APIURL     string
	MediaURL   string
	IDInstance string
	APIToken   string
	Phone      string
}

// EmailConfig holds SMTP credentials for transactional email.
// Works with Gmail App Passwords, Outlook, or any standard SMTP relay.
type EmailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	FromName string
}

// GeoConfig holds configuration for all three geocoding providers.
// Each provider has a distinct role:
//
//	Photon      address autocomplete / search-as-you-type  (no key required)
//	Nominatim   text address -> lat/lng                    (no key, max 1 req/sec)
//	OpenCage    lat/lng -> readable address (reverse)      (optional, 2,500/day free)
//
// Browser Geolocation API (GPS on app open) runs on the device — no config needed.
// All distance math (nearest store, delivery fees) uses Haversine — no API calls.
type GeoConfig struct {
	PhotonBaseURL      string
	NominatimBaseURL   string
	NominatimUserAgent string // required by Nominatim policy — identifies your app
	OpenCageAPIKey     string // leave empty to fall back to Nominatim for reverse geocoding
}

// SecurityConfig holds cryptographic settings.
type SecurityConfig struct {
	// EncryptionKey is a 64-character hex string (32 bytes) used for AES-256-GCM
	// encryption of sensitive database fields (M-Pesa credentials, Airtel keys).
	// Generate with: openssl rand -hex 32
	// Required — the server will not start without it.
	EncryptionKey string
}

// CloudinaryConfig holds credentials for proof-of-delivery photo storage.
// Two folders are used — configure their lifecycle rules in the Cloudinary dashboard:
//
//	delivery-photos/   auto-delete after 30 days  (Settings > Upload > Lifecycle rules)
//	dispute-evidence/  no deletion rule            (kept until manually cleared)
type CloudinaryConfig struct {
	CloudName string
	APIKey    string
	APISecret string
}

// RulesConfig holds business rules that may need tuning without code changes.
// All values have sensible defaults and can be overridden in .env.
type RulesConfig struct {
	// LocationCacheTTLHours is how long a customer's GPS location stays cached
	// in Redis before the app re-captures it on the next visit. Default: 4.
	LocationCacheTTLHours int

	// PODGPSToleranceMetres is how far the delivery person's GPS can be from
	// the delivery address and still have their POD submission accepted. Default: 200.
	PODGPSToleranceMetres int

	// OTPLength is the number of digits in the delivery confirmation OTP
	// sent to the customer when their order is dispatched. Default: 6.
	OTPLength int

	// DisputeWindowHours is how many hours after delivery a customer can
	// raise a dispute. Default: 24.
	DisputeWindowHours int
}

// ── Loader ────────────────────────────────────────────────────────────────────

// Load reads environment variables (from .env in development, injected directly
// in production) and returns a fully populated Config.
// Panics immediately if any required variable is missing so misconfigurations
// are caught at startup rather than mid-request.
func Load() (*Config, error) {
	_ = godotenv.Load() // .env is optional in production

	// Parse all integer env vars up front
	accessExpiry, _  := strconv.Atoi(getEnv("JWT_ACCESS_EXPIRY_MINUTES", "30"))
	refreshExpiry, _ := strconv.Atoi(getEnv("JWT_REFRESH_EXPIRY_DAYS", "7"))
	smtpPort, _          := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	locationTTL, _       := strconv.Atoi(getEnv("LOCATION_CACHE_TTL_HOURS", "4"))
	gpsTolerance, _      := strconv.Atoi(getEnv("POD_GPS_TOLERANCE_METRES", "200"))
	otpLength, _         := strconv.Atoi(getEnv("OTP_LENGTH", "6"))
	disputeWindow, _     := strconv.Atoi(getEnv("DISPUTE_WINDOW_HOURS", "24"))

	cfg := &Config{
		App: AppConfig{
			Name:    getEnv("APP_NAME", "Hardware Store"),
			LogoURL: getEnv("APP_LOGO_URL", "/images/logo.png"),
			Env:     getEnv("APP_ENV", "development"),
			Port:    getEnv("APP_PORT", "8080"),
			URL:     getEnv("APP_URL", "http://localhost:5173"),
		},
		Database: DatabaseConfig{
			URL: requireEnv("DATABASE_URL"),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379"),
		},
		JWT: JWTConfig{
			Secret:              requireEnv("JWT_SECRET"),
			AccessExpiryMinutes: accessExpiry,
			RefreshExpiryDays:   refreshExpiry,
		},
		Mpesa: MpesaConfig{
			ConsumerKey:    getEnv("MPESA_CONSUMER_KEY", ""),
			ConsumerSecret: getEnv("MPESA_CONSUMER_SECRET", ""),
			Shortcode:      getEnv("MPESA_SHORTCODE", ""),
			Passkey:        getEnv("MPESA_PASSKEY", ""),
			CallbackURL:    getEnv("MPESA_CALLBACK_URL", ""),
			BaseURL:        getEnv("MPESA_BASE_URL", "https://sandbox.safaricom.co.ke"),
		},
		Airtel: AirtelConfig{
			ClientID:     getEnv("AIRTEL_CLIENT_ID", ""),
			ClientSecret: getEnv("AIRTEL_CLIENT_SECRET", ""),
			BaseURL:      getEnv("AIRTEL_BASE_URL", "https://openapi.airtel.africa"),
		},
		WhatsApp: WhatsAppConfig{
			APIURL:     getEnv("GREENAPI_API_URL", "https://api.green-api.com"),
			MediaURL:   getEnv("GREENAPI_MEDIA_URL", "https://media.green-api.com"),
			IDInstance: getEnv("GREENAPI_ID_INSTANCE", ""),
			APIToken:   getEnv("GREENAPI_API_TOKEN", ""),
			Phone:      getEnv("GREENAPI_PHONE", ""),
		},
		Email: EmailConfig{
			Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
			Port:     smtpPort,
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			FromName: getEnv("SMTP_FROM_NAME", "Hardware Store"),
		},
		Geo: GeoConfig{
			PhotonBaseURL:      getEnv("PHOTON_BASE_URL", "https://photon.komoot.io"),
			NominatimBaseURL:   getEnv("NOMINATIM_BASE_URL", "https://nominatim.openstreetmap.org"),
			NominatimUserAgent: getEnv("NOMINATIM_USER_AGENT", "HardwareStoreApp/1.0"),
			OpenCageAPIKey:     getEnv("OPENCAGE_API_KEY", ""),
		},
		Security: SecurityConfig{
			EncryptionKey: requireEnv("ENCRYPTION_KEY"),
		},
		Cloudinary: CloudinaryConfig{
			CloudName: requireEnv("CLOUDINARY_CLOUD_NAME"),
			APIKey:    requireEnv("CLOUDINARY_API_KEY"),
			APISecret: requireEnv("CLOUDINARY_API_SECRET"),
		},
		Rules: RulesConfig{
			LocationCacheTTLHours: locationTTL,
			PODGPSToleranceMetres: gpsTolerance,
			OTPLength:             otpLength,
			DisputeWindowHours:    disputeWindow,
		},
	}

	return cfg, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// IsDevelopment returns true when APP_ENV=development.
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction returns true when APP_ENV=production.
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// getEnv returns the value of key or fallback if the variable is not set or empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// requireEnv returns the value of key or panics with a clear message if not set.
// Used for variables the application absolutely cannot start without.
func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set — check your .env file", key))
	}
	return v
}