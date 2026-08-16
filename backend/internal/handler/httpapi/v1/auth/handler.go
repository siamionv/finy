// Package auth serves the operations tagged Auth in the OpenAPI spec.
//
// Handlers here stay thin: bind, delegate to the service, render. Anything
// that survives a change of transport belongs in the service layer instead.
package auth

import (
	"log/slog"

	"github.com/siamionv/finy/internal/entity"
)

// Handler serves every Auth operation. One type per tag, one file per
// operation — the type is the unit of wiring, the file is the unit of reading.
type Handler struct {
	logger  *slog.Logger
	userSvc UserService
}

func New(logger *slog.Logger) *Handler {
	return &Handler{logger: logger}
}

type UserService interface {
	CreateUser(creds entity.UserCredentials) (entity.PublicUser, error)
}
