package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/siamionv/finy/pkg/cerr"
)

// recoverMiddleware turns a panic into an ordinary error and hands it back down
// the chain rather than answering the client itself. Echo's stock Recover
// writes the response and prints the stack to its own stdout logger, which
// leaves the structured logger below with a bare 500 and no cause — the panics
// most worth reading would be the ones with the least attached to them.
func recoverMiddleware() echo.MiddlewareFunc {
	return middleware.RecoverWithConfig(middleware.RecoverConfig{
		// Return the error instead of calling c.Error: loggingMiddleware wraps
		// this one, and it already renders what it logs.
		DisableErrorHandler: true,
		// Set, so echo skips its own printing — but the stack is still
		// collected, which is what DisablePrintStack actually gates.
		LogErrorFunc: func(_ echo.Context, err error, stack []byte) error {
			return cerr.New("handler panicked", err, cerr.Internal).
				Loc().
				Time().
				With("stack", string(stack))
		},
	})
}

func loggingMiddleware(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			now := time.Now()

			// RequestID writes the id to the response header, never back onto
			// the request, so this is the only place it can be read from.
			logger := log.With(
				"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
				"method", c.Request().Method,
				"route", c.Path(),
			)

			logger.Debug("request received")

			err := next(c)
			if err != nil && !c.Response().Committed {
				// Echo runs HTTPErrorHandler outside the middleware chain, so
				// for an unhandled error the response status is still unset
				// here. Handling it now means the status logged below is the
				// real one — and the error must not be returned again, or it
				// would be handled twice.
				//
				// A handler that already answered the client still returns its
				// error, so this log site can see it; that one is committed and
				// needs no rendering, only recording.
				c.Error(err)
			}

			status := c.Response().Status

			logger = logger.With(
				"status", status,
				"elapsed_ms", float64(time.Since(now).Microseconds())/1000,
			)

			switch {
			case status >= http.StatusInternalServerError:
				logger.Error("request failed", "error", err)
			case status >= http.StatusBadRequest:
				logger.Warn("request rejected", "error", err)
			default:
				logger.Info("request handled")
			}

			return nil
		}
	}
}
