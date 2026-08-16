package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) RegisterUser(_ echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented)
}
