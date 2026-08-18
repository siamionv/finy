package config

import (
	"log/slog"
	"strings"

	"github.com/siamionv/finy/internal/entity"
)

// Log formats the service knows how to build a handler for.
const (
	LogFormatText = "text"
	LogFormatJSON = "json"
)

type Logger struct {
	Level     string `env:"LOGGER_LEVEL"      mapstructure:"level"      validate:"required,oneof=debug info warn error"`
	Format    string `env:"LOGGER_FORMAT"     mapstructure:"format"     validate:"required,oneof=json text"`
	AddSource bool   `env:"LOGGER_ADD_SOURCE" mapstructure:"add_source"`
}

// SlogLevel maps the configured level onto slog's vocabulary.
func (l Logger) SlogLevel() (slog.Level, error) {
	switch strings.ToLower(l.Level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, entity.ErrUnknownLogLevel.Loc().Time().With("level", l.Level)
	}
}
