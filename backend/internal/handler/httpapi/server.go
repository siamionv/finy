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
	"github.com/siamionv/finy/internal/handler/httpapi/authn"
	"github.com/siamionv/finy/internal/handler/httpapi/docs"
	"github.com/siamionv/finy/internal/handler/httpapi/v1/auth"
	"github.com/siamionv/finy/internal/handler/httpapi/v1/user"
)

// Deps is everything the HTTP layer needs, declared by the HTTP layer itself.
// Service fields are typed as the owning group's interface, so httpapi never
// imports business.
type Deps struct {
	Config config.HTTP
	Logger *slog.Logger

	Authenticator auth.Authenticator
	ProfileReader user.ProfileReader
	TokenVerifier authn.TokenVerifier
}

// Server owns the echo instance and its lifecycle. Echo does not escape this package.
type Server struct {
	echo          *echo.Echo
	logger        *slog.Logger
	config        config.HTTP
	abortInflight context.CancelFunc
}

// New assembles the router. It fails only on a defect in the embedded spec,
// which must stop the process: the alternative is serving every guarded route
// wide open.
func New(deps Deps) (*Server, error) {
	// Parent of every in-flight request context. Deliberately not derived from the
	// signal context: cancelling it is the last resort, after the drain window.
	baseCtx, abortInflight := context.WithCancel(context.Background())

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Server.BaseContext = func(net.Listener) context.Context { return baseCtx }
	e.Server.ReadHeaderTimeout = deps.Config.ReadHeaderTimeout
	e.Server.ReadTimeout = deps.Config.ReadTimeout
	e.Server.WriteTimeout = deps.Config.WriteTimeout
	e.Server.IdleTimeout = deps.Config.IdleTimeout

	// Order is load-bearing: RequestID sets the id loggingMiddleware reads, and
	// loggingMiddleware must stay outside Recover and BodyLimit to see what they answer.
	e.Use(middleware.RequestID())
	e.Use(loggingMiddleware(deps.Logger))
	e.Use(recoverMiddleware())
	e.Use(middleware.BodyLimit(deps.Config.MaxBodySize))

	guarded, err := guardedOperations(authn.Middleware(deps.TokenVerifier))
	if err != nil {
		abortInflight()

		return nil, err
	}

	openapi.RegisterHandlersWithOptions(e, newHandlers(deps), openapi.RegisterHandlersOptions{
		OperationMiddlewares: guarded,
	})

	docsHandler := docs.New()
	e.GET("/docs", docsHandler.Index)
	e.GET("/docs/openapi.json", docsHandler.Spec)

	return &Server{
		echo:          e,
		logger:        deps.Logger,
		config:        deps.Config,
		abortInflight: abortInflight,
	}, nil
}

// Run serves until ctx is cancelled, then drains. It blocks.
func (s *Server) Run(ctx context.Context) error {
	// Covers the errCh path below, which returns without draining. Idempotent.
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

// Handler exposes the router for httptest-based tests.
func (s *Server) Handler() http.Handler { return s.echo }

// shutdown stops accepting new requests, gives the in-flight ones DrainTimeout to
// finish, then cancels their contexts and force-closes what is left.
//
//nolint:contextcheck // the drain deadline must outlive the cancelled signal context
func (s *Server) shutdown() error {
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
