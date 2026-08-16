package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/siamionv/finy/internal/config"
	"github.com/siamionv/finy/internal/di"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/pkg/cerr"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pressly/goose/v3"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	config := config.MustConfig()
	deps := di.MustDependencies(ctx, config)

	runApp(ctx, deps)
}

func runApp(ctx context.Context, deps di.Dependencies) {
	if err := runMigrations(deps); err != nil {
		panic(err)
	}

	e := createRouter(deps)

	runAPI(ctx, e)
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

func createRouter(deps di.Dependencies) *echo.Echo {
	// api := handler.New(users, health)
	_ = deps

	e := echo.New()
	e.Use(middleware.Recover())

	openapi.RegisterHandlers(e, nil)

	return e
}

func runAPI(ctx context.Context, e *echo.Echo) {
	go func() {
		if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("shutting down the server: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//nolint:contextcheck
	if err := e.Shutdown(shutdownCtx); err != nil {
		panic(err)
	}
}
