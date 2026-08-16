package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"

	"github.com/siamionv/finy/internal/config"
	"github.com/siamionv/finy/internal/di"
	"github.com/siamionv/finy/internal/handler/httpapi"
	"github.com/siamionv/finy/pkg/cerr"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	// All the work lives in run so its defers still fire on the failure path;
	// os.Exit from main would skip them.
	os.Exit(run())
}

func run() int {
	// SIGTERM matters as much as SIGINT: it is what Docker and Kubernetes send,
	// and without it containers are killed outright and never drain.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.MustConfig()

	deps := di.MustDependencies(ctx, cfg)
	defer func() { _ = deps.Close() }()

	if err := runMigrations(deps); err != nil {
		deps.Logger.Error("migrations failed", "error", err)

		return 1
	}

	srv := httpapi.New(httpapi.Deps{
		Config: cfg.HTTP,
		Logger: deps.Logger,
	})

	if err := srv.Run(ctx); err != nil {
		deps.Logger.Error("api stopped", "error", err)

		return 1
	}

	return 0
}

func runMigrations(deps di.Dependencies) error {
	db, err := sql.Open("pgx", deps.Config.Database.PostgresDSN())
	if err != nil {
		return cerr.New("fail to open sql connection", err).Loc().Time()
	}
	defer func() { _ = db.Close() }()

	if err := goose.Up(db, deps.Config.Database.MigrationsPath); err != nil {
		return cerr.New("fail to run migration", err).Loc().Time()
	}

	return nil
}
