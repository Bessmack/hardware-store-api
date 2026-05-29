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
	Maps       MapsConfig
	Cloudinary CloudinaryConfig
	Dispute    DisputeConfig
}

type AppConfig struct {
	Name    string
	LogoURL string
	Env     string
	Port    string
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

// MpesaConfig holds global Daraja API credentials for generating access tokens.
// Each store's own shortcode and passkey live in the stores table, not here.
type MpesaConfig struct {
	ConsumerKey    string
	ConsumerSecret string
	BaseURL        string
}

type AirtelConfig struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
}

type WhatsAppConfig struct {
	PhoneNumberID string
	AccessToken   string
	APIVersion    string
}

// EmailConfig holds SMTP credentials for transactional email.
// Works with Gmail (use an App Password, not your account password),
// or any other SMTP provider — just swap the host/port.
type EmailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	FromName string // display name shown to email recipients
}

type MapsConfig struct {
	APIKey string
}

// CloudinaryConfig holds credentials for POD photo storage.
// Two folders are used by the system:
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
			BaseURL:        getEnv("MPESA_BASE_URL", "https://sandbox.safaricom.co.ke"),
		},
		Airtel: AirtelConfig{
			ClientID:     getEnv("AIRTEL_CLIENT_ID", ""),
			ClientSecret: getEnv("AIRTEL_CLIENT_SECRET", ""),
			BaseURL:      getEnv("AIRTEL_BASE_URL", "https://openapi.airtel.africa"),
		},
		WhatsApp: WhatsAppConfig{
			PhoneNumberID: getEnv("WHATSAPP_PHONE_NUMBER_ID", ""),
			AccessToken:   getEnv("WHATSAPP_ACCESS_TOKEN", ""),
			APIVersion:    getEnv("WHATSAPP_API_VERSION", "v19.0"),
		},
		Email: EmailConfig{
			Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
			Port:     smtpPort,
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			FromName: getEnv("SMTP_FROM_NAME", "Hardware Store"),
		},
		Maps: MapsConfig{
			APIKey: getEnv("GOOGLE_MAPS_API_KEY", ""),
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