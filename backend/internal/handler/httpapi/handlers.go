package httpapi

import (
	"github.com/labstack/echo/v4"

	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/internal/handler/httpapi/v1/auth"
)

// handlers implements the generated openapi.ServerInterface. It holds no logic:
// every operation delegates to the handler group that owns its OpenAPI tag.
// Centralising the delegation keeps "which type serves this operation" a single
// grep, and makes a newly generated operation fail to compile here — next to
// the fix — rather than at the RegisterHandlers call site.
type handlers struct {
	auth *auth.Handler
}

// Fails the build the moment codegen adds an operation nobody delegates.
var _ openapi.ServerInterface = (*handlers)(nil)

func newHandlers(deps Deps) *handlers {
	return &handlers{
		auth: auth.New(auth.Deps{
			Logger:      deps.Logger,
			UserService: deps.UserService,
		}),
	}
}

// --- Auth ---

func (h *handlers) RegisterUser(c echo.Context) error { return h.auth.RegisterUser(c) }
