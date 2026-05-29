package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration loaded from environment variables.
// Access it everywhere via a single instance initialised in main.go.
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
	Cloudinary CloudinaryConfig
	Dispute    DisputeConfig
}

type AppConfig struct {
	Name    string
	LogoURL string
	Env     string
	Port    string
	URL     string // frontend URL — used for CORS and email redirect links
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type JWTConfig struct {
	Secret      string
	ExpiryHours int
}

// MpesaConfig holds Daraja API credentials.
//
// ConsumerKey / ConsumerSecret: used to generate short-lived access tokens
// from Safaricom's OAuth endpoint. These are global — one pair per application.
//
// Shortcode / Passkey: default credentials used when a store has not yet been
// configured with its own in the stores table. Once a store's own shortcode
// and passkey are set, those take precedence over these defaults.
//
// CallbackURL: the public HTTPS endpoint that Safaricom posts payment results to.
// In development, expose your local server with ngrok.
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
// Fields:
//   - APIURL:      base URL for all API calls
//   - MediaURL:    base URL for sending media (images, PDFs)
//   - IDInstance:  your instance ID shown on the Green API dashboard
//   - APIToken:    authentication token
//   - Phone:       the WhatsApp number messages are sent from (no + prefix)
type WhatsAppConfig struct {
	APIURL     string
	MediaURL   string
	IDInstance string
	APIToken   string
	Phone      string
}

// EmailConfig holds SMTP credentials for transactional email.
// Works with Gmail App Passwords, Outlook, or any SMTP relay.
type EmailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	FromName string
}

// GeoConfig holds configuration for geocoding delivery addresses.
// The system uses Nominatim (OpenStreetMap) as the primary geocoder — free,
// no API key required. OpenCage is an optional fallback for higher volume.
//
// Nominatim rules:
//   - Max 1 request/second (enforced by rate limiter in the geo package)
//   - UserAgent must identify your application (required by Nominatim policy)
//   - Free to use in production
//
// Distance calculation (store routing, delivery fees) uses the Haversine
// formula internally — no API calls needed.
type GeoConfig struct {
	NominatimBaseURL  string
	NominatimUserAgent string
	OpenCageAPIKey    string // optional fallback — leave empty to skip
}

// CloudinaryConfig holds credentials for POD photo storage.
// Two folders are used:
//   - delivery-photos/  → configure a 30-day auto-delete rule in Cloudinary dashboard
//   - dispute-evidence/ → permanent, no deletion rule
type CloudinaryConfig struct {
	CloudName string
	APIKey    string
	APISecret string
}

type DisputeConfig struct {
	WindowHours int
}

// Load reads environment variables (from .env in development, directly in production)
// and returns a fully populated Config. Panics if any required variable is missing.
func Load() (*Config, error) {
	// In production env vars are injected directly — .env file is optional
	_ = godotenv.Load()

	jwtExpiry, _ := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "24"))
	disputeWindow, _ := strconv.Atoi(getEnv("DISPUTE_WINDOW_HOURS", "24"))
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

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
			Secret:      requireEnv("JWT_SECRET"),
			ExpiryHours: jwtExpiry,
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
			NominatimBaseURL:   getEnv("NOMINATIM_BASE_URL", "https://nominatim.openstreetmap.org"),
			NominatimUserAgent: getEnv("NOMINATIM_USER_AGENT", "HardwareStoreApp/1.0"),
			OpenCageAPIKey:     getEnv("OPENCAGE_API_KEY", ""),
		},
		Cloudinary: CloudinaryConfig{
			CloudName: requireEnv("CLOUDINARY_CLOUD_NAME"),
			APIKey:    requireEnv("CLOUDINARY_API_KEY"),
			APISecret: requireEnv("CLOUDINARY_API_SECRET"),
		},
		Dispute: DisputeConfig{
			WindowHours: disputeWindow,
		},
	}

	return cfg, nil
}

// IsDevelopment returns true when running in the development environment.
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction returns true when running in the production environment.
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// getEnv returns the value of key or fallback if not set.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// requireEnv returns the value of key or panics with a clear message if not set.
// Use for variables that the application cannot function without.
func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set — check your .env file", key))
	}
	return v
}