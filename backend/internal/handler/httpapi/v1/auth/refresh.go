package auth

import (
	"errors"
	"net/http"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/labstack/echo/v4"
)

func (h *Handler) RefreshTokens(c echo.Context) error {
	var request openapi.RefreshTokenRequest
	if err := c.Bind(&request); err != nil {
		err := cerr.New("failed to bind refresh tokens request", err, cerr.Invalid).
			Loc().
			Time().
			With("content_type", c.Request().Header.Get(echo.HeaderContentType))

		return fail(c, http.StatusBadRequest, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to deserialize request payload"),
		}, err)
	}

	tokenPair, err := h.auth.Refresh(c.Request().Context(), request.RefreshToken)
	if err != nil {
		return h.handleRefreshTokensError(c, err)
	}

	response := tokenPairToResponse(tokenPair)

	return c.JSON(http.StatusOK, openapi.EnvelopedTokenPair{
		Status: openapi.Success,
		Data:   &response,
	})
}

func (h *Handler) handleRefreshTokensError(c echo.Context, err error) error {
	if errors.Is(err, entity.ErrInvalidRefreshToken) {
		return fail(c, http.StatusBadRequest, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("invalid refresh token"),
		}, err)
	}

	if errors.Is(err, cerr.Internal) {
		return fail(c, http.StatusInternalServerError, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to refresh tokens"),
		}, err)
	}

	return fail(c, http.StatusInternalServerError, openapi.Envelope{
		Status: openapi.Failure,
		Error:  new("failed to refresh tokens"),
	}, cerr.New("unclassified error from Refresh", err, cerr.Internal).Loc().Time())
}
