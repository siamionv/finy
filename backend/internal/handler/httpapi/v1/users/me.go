package users

import (
	"errors"
	"net/http"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/internal/handler/httpapi/authn"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/labstack/echo/v4"
)

// GetCurrentUser answers with the account the presented access token belongs to.
// There is no payload to bind: the guard already turned the token into an id.
func (h *Handler) GetCurrentUser(c echo.Context) error {
	userID, err := authn.UserID(c)
	if err != nil {
		return h.handleGetCurrentUserError(c, err)
	}

	user, err := h.profiles.GetUser(c.Request().Context(), userID)
	if err != nil {
		return h.handleGetCurrentUserError(c, err)
	}

	response := userToResponse(*user)

	return c.JSON(http.StatusOK, openapi.EnvelopedUser{
		Status: openapi.Success,
		Data:   &response,
	})
}

func (h *Handler) handleGetCurrentUserError(c echo.Context, err error) error {
	// The token verified, so the row was deleted between minting and now.
	if errors.Is(err, entity.ErrUserNotFound) {
		return fail(c, http.StatusNotFound, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("user not found"),
		}, err)
	}

	// Already classified as ours; the chain carries the cause to the log site.
	// An unguarded route lands here too: authn.UserID says so as cerr.Internal.
	if errors.Is(err, cerr.Internal) {
		return fail(c, http.StatusInternalServerError, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to get current user"),
		}, err)
	}

	// Matched nothing this handler knows: same opaque 500, but the chain says why.
	return fail(c, http.StatusInternalServerError, openapi.Envelope{
		Status: openapi.Failure,
		Error:  new("failed to get current user"),
	}, cerr.New("unclassified error from GetUser", err, cerr.Internal).Loc().Time())
}
