package di

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/siamionv/finy/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	loggerLevelDebug = "debug"
	loggerLevelWarn  = "warn"
	loggerLevelInfo  = "info"
	loggerLevelError = "error"
)

const (
	loggerFormatText = "text"
	loggerFormatJSON = "json"
)

type Dependencies struct {
	Config *config.Config
	Logger *slog.Logger
	DB     *pgxpool.Pool
}

func MustDependencies(ctx context.Context, config *config.Config) Dependencies {
	return Dependencies{
		Config: config,
		Logger: mustLogger(ctx, config.Logger),
		DB:     mustDB(ctx, config.Database),
	}
}

func mustDB(ctx context.Context, config config.Database) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, config.PostgresDSN())
	if err != nil {
		panic(fmt.Sprintf("create postgres pool: %v", err))
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		panic(fmt.Sprintf("ping postgres: %v", err))
	}

	return pool
}

func mustLogger(_ context.Context, config config.Logger) *slog.Logger {
	output := os.Stdout

	var logLevel slog.Level
	switch config.Level {
	case loggerLevelDebug:
		logLevel = slog.LevelDebug
	case loggerLevelWarn:
		logLevel = slog.LevelWarn
	case loggerLevelInfo:
		logLevel = slog.LevelInfo
	case loggerLevelError:
		logLevel = slog.LevelError
	default:
		panic(fmt.Sprintf("unknown logger level: %v", config.Level))
	}

	options := &slog.HandlerOptions{
		AddSource: config.AddSource,
		Level:     logLevel,
	}

	var handler slog.Handler
	switch config.Format {
	case loggerFormatText:
		handler = slog.NewTextHandler(output, options)
	case loggerFormatJSON:
		handler = slog.NewJSONHandler(output, options)
	default:
		panic(fmt.Sprintf("unknown logger format: %v", config.Format))
	}

	logger := slog.New(handler)

	return logger
}

func (d Dependencies) Close() error {
	d.DB.Close()

	return nil
}
