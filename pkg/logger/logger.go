package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init configures the global zerolog logger.
// Development: human-readable coloured console output.
// Production:  structured JSON — ready for log aggregators (Datadog, Loki, etc.)
func Init(env string) {
	zerolog.TimeFieldFormat = time.RFC3339

	if env == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "15:04:05",
		})
	} else {
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}
}

// Get returns the global logger.
// Usage: logger.Get().Info().Str("order_id", id).Msg("order placed")
func Get() *zerolog.Logger {
	return &log.Logger
}

// WithField returns a logger with a persistent key-value field attached.
// Useful for adding a consistent "service" or "store_id" field to a group of log lines.
func WithField(key, value string) zerolog.Logger {
	return log.With().Str(key, value).Logger()
}