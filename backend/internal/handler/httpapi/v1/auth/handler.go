// Package auth serves the operations tagged Auth in the OpenAPI spec. Handlers
// here stay thin: bind, delegate to the service, render.
package auth

import (
	"context"
	"log/slog"

	"github.com/siamionv/finy/internal/entity"

	"github.com/labstack/echo/v4"
)

// Handler serves every Auth operation: one type per tag, one file per operation.
type Handler struct {
	logger *slog.Logger
	auth   Authenticator
}

// Deps is what this group needs to serve its operations.
type Deps struct {
	Logger        *slog.Logger
	Authenticator Authenticator
}

func New(deps Deps) *Handler {
	return &Handler{
		logger: deps.Logger,
		auth:   deps.Authenticator,
	}
}

// Authenticator is what this group needs of the business layer: one method per
// operation it serves. Named for the capability required, not for the type that
// happens to provide it, so the business layer can be reshaped without touching
// this package.
type Authenticator interface {
	Register(ctx context.Context, creds entity.UserCredentials) (*entity.User, error)
	Login(ctx context.Context, creds entity.UserCredentials) (entity.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (entity.TokenPair, error)
}

// fail renders body to the client and hands err back up to the single log site in
// the middleware chain. Echo's error handler returns early once the response is
// committed, so the returned error changes what gets logged, never what gets sent.
func fail(c echo.Context, status int, body any, err error) error {
	if writeErr := c.JSON(status, body); writeErr != nil {
		return writeErr
	}

	return err
}
