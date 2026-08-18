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

// Dependencies is the composition root: infrastructure on top, the assembled
// object graph below it. Callers get Services, never the repositories that
// back them.
type Dependencies struct {
	Config *config.Config
	Logger *slog.Logger
	DB     *pgxpool.Pool

	Services Services
}

func MustDependencies(ctx context.Context, config *config.Config) Dependencies {
	db := mustDB(ctx, config.Database)

	return Dependencies{
		Config:   config,
		Logger:   mustLogger(ctx, config.Logger),
		DB:       db,
		Services: newServices(config, newRepositories(db)),
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
