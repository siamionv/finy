package di

import (
	"context"
	"log/slog"
	"os"

	"github.com/siamionv/finy/internal/config"
	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Dependencies is the composition root: infrastructure on top, the assembled
// object graph below it. Callers get Services, never the repositories that back them.
type Dependencies struct {
	Logger *slog.Logger
	DB     *pgxpool.Pool

	Services Services
}

// New builds the object graph, connecting to every backing store it needs.
func New(ctx context.Context, cfg *config.Config) (Dependencies, error) {
	logger, err := newLogger(cfg.Logger)
	if err != nil {
		return Dependencies{}, err
	}

	db, err := newDB(ctx, cfg.Database)
	if err != nil {
		return Dependencies{}, err
	}

	return Dependencies{
		Logger:   logger,
		DB:       db,
		Services: newServices(cfg, newRepositories(db)),
	}, nil
}

func newDB(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN())
	if err != nil {
		return nil, cerr.New("failed to create postgres pool", err, cerr.Internal).Loc().Time()
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, cerr.New("failed to ping postgres", err, cerr.Internal).Loc().Time()
	}

	return pool, nil
}

func newLogger(cfg config.Logger) (*slog.Logger, error) {
	level, err := cfg.SlogLevel()
	if err != nil {
		return nil, err
	}

	options := &slog.HandlerOptions{AddSource: cfg.AddSource, Level: level}

	var handler slog.Handler
	switch cfg.Format {
	case config.LogFormatText:
		handler = slog.NewTextHandler(os.Stdout, options)
	case config.LogFormatJSON:
		handler = slog.NewJSONHandler(os.Stdout, options)
	default:
		return nil, entity.ErrUnknownLogFormat.Loc().Time().With("format", cfg.Format)
	}

	return slog.New(handler), nil
}

// Close releases every backing store the graph holds open.
func (d Dependencies) Close() error {
	d.DB.Close()

	return nil
}
