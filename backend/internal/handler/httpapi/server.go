package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/siamionv/finy/internal/config"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/internal/handler/httpapi/v1/auth"
)

// Deps is everything the HTTP layer needs, declared by the HTTP layer itself.
// Handler groups get their service as a narrow interface, so nothing under
// this package can reach past a service into the database.
//
// Service fields are typed as the owning group's interface, not as the
// concrete business type: httpapi never imports business, and the composition
// root is the only place the two sides meet.
//
// It grows by one field per service.
type Deps struct {
	Config config.HTTP
	Logger *slog.Logger

	UserService auth.UserService
}

// Server owns the echo instance and its lifecycle. Echo does not escape this
// package: callers get Run and Handler, not a framework object.
type Server struct {
	echo          *echo.Echo
	logger        *slog.Logger
	config        config.HTTP
	abortInflight context.CancelFunc
}

func New(deps Deps) *Server {
	// Parent of every in-flight request context. Deliberately not derived from
	// the signal context: cancelling this is the last resort, after the drain
	// window expires, not the first thing that happens on SIGINT.
	baseCtx, abortInflight := context.WithCancel(context.Background())

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Server.BaseContext = func(net.Listener) context.Context { return baseCtx }
	e.Server.ReadHeaderTimeout = deps.Config.ReadHeaderTimeout
	e.Server.ReadTimeout = deps.Config.ReadTimeout
	e.Server.WriteTimeout = deps.Config.WriteTimeout
	e.Server.IdleTimeout = deps.Config.IdleTimeout

	// Order is load-bearing. RequestID first, because loggingMiddleware reads
	// the id it sets. loggingMiddleware second, so it stays outside Recover and
	// BodyLimit: it has no defer, so anything those two answer on their own —
	// a recovered panic, a 413 — would otherwise unwind past its post-handler
	// block and never reach the structured logger at all.
	e.Use(middleware.RequestID())
	e.Use(loggingMiddleware(deps.Logger))
	e.Use(recoverMiddleware())
	e.Use(middleware.BodyLimit(deps.Config.MaxBodySize))

	openapi.RegisterHandlers(e, newHandlers(deps))

	return &Server{
		echo:          e,
		logger:        deps.Logger,
		config:        deps.Config,
		abortInflight: abortInflight,
	}
}

// Run serves until ctx is cancelled, then drains. It blocks.
func (s *Server) Run(ctx context.Context) error {
	// Covers the errCh path below, which returns without draining: the base
	// context outlives a server that never bound, and Handler() means a test
	// can construct-and-fail repeatedly. Idempotent, so the drain path calling
	// it too is harmless.
	defer s.abortInflight()

	errCh := make(chan error, 1)

	go func() {
		err := s.echo.Start(s.config.Addr)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		errCh <- err
	}()

	s.logger.Info("api listening", "addr", s.config.Addr)

	select {
	case err := <-errCh:
		// Failed to bind, or stopped without anyone asking it to.
		return err
	case <-ctx.Done():
		s.logger.Info("shutdown signal received, draining", "timeout", s.config.DrainTimeout)
	}

	return s.shutdown()
}

// Handler exposes the router for httptest-based tests, without leaking echo
// into production callers.
func (s *Server) Handler() http.Handler { return s.echo }

// shutdown stops accepting new requests, gives the in-flight ones DrainTimeout
// to finish, and only then cancels their contexts and force-closes what is
// left. Shutdown returning on timeout does not stop the handlers — cancelling
// the base context is what actually unwinds them.
//
//nolint:contextcheck // the drain deadline must outlive the cancelled signal context
func (s *Server) shutdown() error {
	// Success path: everything finished, release the base context.
	defer s.abortInflight()

	drainCtx, cancel := context.WithTimeout(context.Background(), s.config.DrainTimeout)
	defer cancel()

	start := time.Now()

	if err := s.echo.Shutdown(drainCtx); err != nil {
		s.logger.Warn("drain timed out, aborting in-flight requests", "error", err)
		s.abortInflight()

		return s.echo.Close()
	}

	s.logger.Info("drained cleanly", "elapsed", time.Since(start).String())

	return nil
}
