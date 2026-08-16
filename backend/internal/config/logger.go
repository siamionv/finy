package config

import (
	"fmt"
	"log/slog"
	"strings"
)

type Logger struct {
	Level     string `env:"LOGGER_LEVEL"      mapstructure:"level"      validate:"required,oneof=debug info warn error"`
	Format    string `env:"LOGGER_FORMAT"     mapstructure:"format"     validate:"required,oneof=json text"`
	AddSource bool   `env:"LOGGER_ADD_SOURCE" mapstructure:"add_source"`
}

func (l Logger) ParseSlogLogLevel() (slog.Level, error) {
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
		return slog.LevelInfo, fmt.Errorf("unknown log level: %s", l.Level)
	}
}
