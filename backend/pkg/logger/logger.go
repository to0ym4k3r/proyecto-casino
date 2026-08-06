// Package logger provides a preconfigured zerolog.Logger for structured logging.
//
// It writes human-readable output to stdout with RFC3339 timestamps and a
// service name field attached to every log entry.
package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New creates a zerolog.Logger that writes colorized, human-readable output to
// stdout with RFC3339 timestamps. Every log entry automatically includes a
// "service" field set to the provided name, making it easy to filter logs in
// multi-service deployments.
func New(service string) zerolog.Logger {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	return zerolog.New(output).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Str("service", service).
		Logger()
}
