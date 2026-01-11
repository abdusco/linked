package logger

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	// Set global log level from environment, default to info
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)

	// Pretty print in development, JSON in production
	if os.Getenv("ENV") != "production" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
}

// Get returns the global zerolog logger
func Get() zerolog.Logger {
	return log.Logger
}

// With returns a logger with additional fields
func With(fields ...any) zerolog.Logger {
	return log.Logger.With().Fields(fields).Logger()
}

