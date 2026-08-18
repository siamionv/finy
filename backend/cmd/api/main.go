package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/siamionv/finy/internal/adapter"
	"github.com/siamionv/finy/internal/config"
	"github.com/siamionv/finy/internal/di"
	"github.com/siamionv/finy/internal/handler/httpapi"
)

func main() {
	// All the work lives in run so its defers still fire on the failure path.
	os.Exit(run())
}

func run() int {
	// SIGTERM matters as much as SIGINT: it is what Docker and Kubernetes send.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	configPath := flag.String("config", "", "path to config [required]")
	flag.Parse()

	if *configPath == "" {
		return abort("config path is required")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return abort(fmt.Sprintf("failed to load config: %v", err))
	}

	deps, err := di.New(ctx, cfg)
	if err != nil {
		return abort(fmt.Sprintf("failed to build dependencies: %v", err))
	}
	defer func() { _ = deps.Close() }()

	if err := adapter.RunMigrations(
		cfg.Database.PostgresDSN(),
		cfg.Database.MigrationsPath,
	); err != nil {
		deps.Logger.Error("migrations failed", "error", err)

		return 1
	}

	srv, err := httpapi.New(httpapi.Deps{
		Config:        cfg.HTTP,
		Logger:        deps.Logger,
		Authenticator: deps.Services.Auth,
		ProfileReader: deps.Services.Users,
		TokenVerifier: deps.Services.Tokens,
	})
	if err != nil {
		deps.Logger.Error("failed to build api", "error", err)

		return 1
	}

	if err := srv.Run(ctx); err != nil {
		deps.Logger.Error("api stopped", "error", err)

		return 1
	}

	return 0
}

// abort reports a failure that happened before the logger existed.
func abort(msg string) int {
	fmt.Fprintln(os.Stderr, msg)

	return 1
}
