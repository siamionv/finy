package httpapi

import (
	"github.com/labstack/echo/v4"

	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/internal/handler/httpapi/v1/auth"
)

// handlers implements the generated openapi.ServerInterface. It holds no logic:
// every operation delegates to the handler group that owns its OpenAPI tag.
type handlers struct {
	auth *auth.Handler
}

// Fails the build the moment codegen adds an operation nobody delegates.
var _ openapi.ServerInterface = (*handlers)(nil)

func newHandlers(deps Deps) *handlers {
	return &handlers{
		auth: auth.New(auth.Deps{
			Logger:        deps.Logger,
			Authenticator: deps.Authenticator,
		}),
	}
}

// --- Auth ---
func (h *handlers) RegisterUser(c echo.Context) error  { return h.auth.RegisterUser(c) }
func (h *handlers) LoginUser(c echo.Context) error     { return h.auth.LoginUser(c) }
func (h *handlers) RefreshTokens(c echo.Context) error { return h.auth.RefreshTokens(c) }
