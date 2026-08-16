package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

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
			if err != nil {
				// Echo runs HTTPErrorHandler outside the middleware chain, so
				// the response status is still unset here. Handling the error
				// now means the status logged below is the real one — and the
				// error must not be returned again, or it would be handled
				// twice.
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
