package auth

import (
	"errors"
	"net/http"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/labstack/echo/v4"
)

func (h *Handler) LoginUser(c echo.Context) error {
	var request openapi.UserCredentialsRequest
	if err := c.Bind(&request); err != nil {
		err := cerr.New("failed to bind login user request", err, cerr.Invalid).
			Loc().
			Time().
			With("content_type", c.Request().Header.Get(echo.HeaderContentType))

		return fail(c, http.StatusBadRequest, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to deserialize request payload"),
		}, err)
	}

	tokenPair, err := h.auth.Login(c.Request().Context(), credentialsFromRequest(request))
	if err != nil {
		return h.handleLoginUserError(c, err)
	}

	response := tokenPairToResponse(tokenPair)

	return c.JSON(http.StatusOK, openapi.EnvelopedTokenPair{
		Status: openapi.Success,
		Data:   &response,
	})
}

// handleLoginUserError answers every bad credential the same way, so this endpoint
// is not a username-enumeration oracle.
func (h *Handler) handleLoginUserError(c echo.Context, err error) error {
	if errors.Is(err, entity.ErrUserNotFound) ||
		errors.Is(err, entity.ErrIncorrectPassword) ||
		errors.Is(err, entity.ErrCredentialsRequired) ||
		isCredentialsRuleError(err) {
		return fail(c, http.StatusBadRequest, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("invalid credentials"),
		}, err)
	}

	// Already classified as ours; the chain carries the cause to the log site.
	if errors.Is(err, cerr.Internal) {
		return fail(c, http.StatusInternalServerError, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to login user"),
		}, err)
	}

	// Matched nothing this handler knows: same opaque 500, but the chain says why.
	return fail(c, http.StatusInternalServerError, openapi.Envelope{
		Status: openapi.Failure,
		Error:  new("failed to login user"),
	}, cerr.New("unclassified error from Login", err, cerr.Internal).Loc().Time())
}

// isCredentialsRuleError reports whether err is one of the credential rules. They
// are listed once, in validationRules, so a rule added for registration lands here too.
func isCredentialsRuleError(err error) bool {
	for _, rule := range validationRules {
		if errors.Is(err, rule.err) {
			return true
		}
	}

	return false
}
