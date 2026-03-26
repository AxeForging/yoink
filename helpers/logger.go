package helpers

import (
	"os"

	"github.com/rs/zerolog"
)

// Log is the global logger instance.
var Log zerolog.Logger

// SetupLogger configures the global zerolog logger.
func SetupLogger(level string) {
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)
	Log = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05",
	}).With().Timestamp().Logger()
}
