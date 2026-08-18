package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/siamionv/finy/pkg/cerr"
)

// recoverMiddleware turns a panic into an ordinary error carrying its stack, so
// the structured logger below sees the cause instead of a bare 500.
func recoverMiddleware() echo.MiddlewareFunc {
	return middleware.RecoverWithConfig(middleware.RecoverConfig{
		// loggingMiddleware wraps this one and already renders what it logs.
		DisableErrorHandler: true,
		// Set so echo skips its own printing; the stack is still collected.
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
				// Echo runs HTTPErrorHandler outside the chain, so an unhandled
				// error leaves the status unset. Handling it here makes the status
				// logged below the real one; returning it again would double-handle it.
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
