// Package user serves the operations tagged Users in the OpenAPI spec. Handlers
// here stay thin: read the request, delegate to the service, render.
package user

import (
	"context"
	"log/slog"

	"github.com/siamionv/finy/internal/entity"

	"github.com/labstack/echo/v4"
)

// Handler serves every Users operation: one type per tag, one file per operation.
type Handler struct {
	logger   *slog.Logger
	profiles ProfileReader
}

// Deps is what this group needs to serve its operations.
type Deps struct {
	Logger        *slog.Logger
	ProfileReader ProfileReader
}

func New(deps Deps) *Handler {
	return &Handler{
		logger:   deps.Logger,
		profiles: deps.ProfileReader,
	}
}

// ProfileReader is what this group needs of the business layer: read the account
// behind an id. Named for that capability, not for the type that provides it.
type ProfileReader interface {
	GetUser(ctx context.Context, id int) (*entity.User, error)
}

// fail renders body to the client and hands err back up to the single log site in
// the middleware chain, the same contract the other handler groups follow.
func fail(c echo.Context, status int, body any, err error) error {
	if writeErr := c.JSON(status, body); writeErr != nil {
		return writeErr
	}

	return err
}
