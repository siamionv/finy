// Package auth serves the operations tagged Auth in the OpenAPI spec.
//
// Handlers here stay thin: bind, delegate to the service, render. Anything
// that survives a change of transport belongs in the service layer instead.
package auth

import (
	"context"
	"log/slog"

	"github.com/siamionv/finy/internal/entity"

	"github.com/labstack/echo/v4"
)

// Handler serves every Auth operation. One type per tag, one file per
// operation — the type is the unit of wiring, the file is the unit of reading.
type Handler struct {
	logger  *slog.Logger
	userSvc UserService
}

// Deps is what this group needs to serve its operations. A struct rather than
// positional arguments: groups gain dependencies over time, and named fields
// keep the wiring site readable — and a forgotten field a nil panic on the
// first request rather than a silently misordered argument.
type Deps struct {
	Logger      *slog.Logger
	UserService UserService
}

func New(deps Deps) *Handler {
	return &Handler{
		logger:  deps.Logger,
		userSvc: deps.UserService,
	}
}

// UserService is the slice of the service layer this group uses — declared
// here, so the group depends on a shape it owns rather than on business.
type UserService interface {
	CreateUser(ctx context.Context, creds entity.UserCredentials) (*entity.User, error)
}

// fail renders body to the client and hands err back up the middleware chain,
// where the single log site lives. The two are separate concerns: body is what
// the caller is allowed to know, err is what we need to debug it, and only the
// handler knows both.
//
// Returning a non-nil error after writing is safe by design — echo's error
// handler returns early once the response is committed — so this changes what
// gets logged, never what gets sent. If the write itself fails, that error wins:
// nothing reached the client, and the cause of the empty response is the more
// urgent of the two.
func fail(c echo.Context, status int, body any, err error) error {
	if writeErr := c.JSON(status, body); writeErr != nil {
		return writeErr
	}

	return err
}
