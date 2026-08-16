// Package auth serves the operations tagged Auth in the OpenAPI spec.
//
// Handlers here stay thin: bind, delegate to the service, render. Anything
// that survives a change of transport belongs in the service layer instead.
package auth

import (
	"context"
	"log/slog"

	"github.com/siamionv/finy/internal/entity"
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
