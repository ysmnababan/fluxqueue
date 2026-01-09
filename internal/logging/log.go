// Package logging provides structured logging setup using zerolog with environment-aware configuration.
package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func InitLogger(env string, level zerolog.Level, serviceName, version string) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339

	// base logger with common fields
	base := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", serviceName).
		Str("version", version).
		Str("env", env).
		Logger().
		Level(level)

	if env == "DEVELOPMENT" {
		// pretty console for dev: preserve base fields by printing with ConsoleWriter
		cw := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.Kitchen}
		l := zerolog.New(cw).With().
			Timestamp().
			Str("service", serviceName).
			Str("version", version).
			Str("env", env).
			Logger().
			Level(level)
		log.Logger = l // optional: set global
		return l
	}

	log.Logger = base // optional: set global
	return base
}
