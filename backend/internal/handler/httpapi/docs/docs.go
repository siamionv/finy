// Package docs serves the Swagger UI and the OpenAPI document it renders.
package docs

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/pkg/cerr"
)

//go:embed index.html
var indexHTML []byte

// Handler serves the API documentation UI and the spec it renders.
type Handler struct{}

func New() *Handler {
	return &Handler{}
}

// Index renders the Swagger UI page.
func (h *Handler) Index(c echo.Context) error {
	return c.HTMLBlob(http.StatusOK, indexHTML)
}

// Spec answers with the OpenAPI document embedded in the binary at generate time.
func (h *Handler) Spec(c echo.Context) error {
	spec, err := openapi.GetSpecJSON()
	if err != nil {
		return cerr.New("failed to load embedded openapi spec", err, cerr.Internal).Loc().Time()
	}

	return c.Blob(http.StatusOK, "application/json", spec)
}
