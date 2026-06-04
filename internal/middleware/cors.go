package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORSConfig holds the values needed to build the CORS middleware.
// Populated from config in main.go.
type CORSConfig struct {
	// AppURL is the frontend origin (e.g. "http://localhost:5173" or "https://yourstore.co.ke").
	// This is the only allowed origin in production.
	AppURL string

	// IsDevelopment enables looser CORS rules for local development:
	//   - Allows all localhost ports (for Vite, Storybook, etc.)
	//   - Allows the wildcard origin during initial setup
	IsDevelopment bool
}

// CORS returns a chi-compatible CORS middleware configured for the hardware
// store frontend.
//
// Headers always allowed:
//   - Authorization      — JWT access token (Bearer scheme)
//   - Content-Type       — JSON request bodies
//   - X-Session-ID       — guest cart and location cache identification
//   - X-Request-ID       — optional client-side request tracing
//
// Credentials are enabled so the Authorization header is accepted.
//
// In development, all localhost origins are permitted so the frontend dev
// server (typically Vite on :5173) can talk to the API without reconfiguring.
//
// In production, only the exact APP_URL origin is allowed. Any other origin
// receives a CORS rejection before the request reaches any handler.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedOrigins := []string{cfg.AppURL}

	if cfg.IsDevelopment {
		// Allow any localhost port so engineers can run multiple dev servers
		// (Vite, Storybook, Swagger UI, etc.) without editing the config.
		allowedOrigins = append(allowedOrigins,
			"http://localhost:3000",
			"http://localhost:4000",
			"http://localhost:5173", // Vite default
			"http://localhost:5174",
			"http://localhost:8080",
			"http://127.0.0.1:5174",
			"http://127.0.0.1:5173",
		)
	}

	return cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,

		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions, // preflight
		},

		AllowedHeaders: []string{
			"Authorization",  // JWT Bearer token
			"Content-Type",   // application/json
			"X-Session-ID",   // guest cart and location cache key
			"X-Request-ID",   // optional request tracing
			"Accept",
			"Origin",
		},

		// Headers the browser is allowed to read from the response.
		// X-RateLimit-* lets the frontend know when it is being throttled.
		ExposedHeaders: []string{
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-RateLimit-Reset",
			"Retry-After",
		},

		// Required for Authorization header to be sent by the browser.
		AllowCredentials: true,

		// Cache preflight responses for 5 minutes — reduces OPTIONS requests
		// on repeat calls to the same endpoint.
		MaxAge: 300,

		// Log CORS rejections in development to help diagnose origin mismatches.
		Debug: cfg.IsDevelopment,
	})
}