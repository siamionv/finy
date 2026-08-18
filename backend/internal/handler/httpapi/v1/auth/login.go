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

	var credentials entity.UserCredentials
	credentials.FromOpenAPI(request)

	userID, err := h.userSvc.GetUserIDByCreds(c.Request().Context(), credentials)
	if err != nil {
		return h.handleLoginUserError(c, err)
	}

	tokenPair, err := h.tokenSvc.Mint(userID)
	if err != nil {
		// The credentials were right — only our own signing failed — so this is
		// ours to own with a 500 rather than anything the client can act on.
		return fail(c, http.StatusInternalServerError, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to issue tokens"),
		}, err)
	}

	response := tokenPair.ToOpenAPI()

	return c.JSON(http.StatusOK, openapi.EnvelopedTokenPair{
		Status: openapi.Success,
		Data:   &response,
	})
}

// handleLoginUserError renders what a failed login is allowed to say. Unlike
// registration, which tells the client exactly which rule its input broke,
// this endpoint answers every bad credential the same way: an unknown
// username, a wrong password, and a malformed credential are indistinguishable
// to the caller, so this is not a username-enumeration oracle.
func (h *Handler) handleLoginUserError(c echo.Context, err error) error {
	// An unknown username, a wrong password, and credentials that never
	// satisfied the rules are one answer: none of them authenticate anyone,
	// and none may be more informative than another.
	if errors.Is(err, entity.ErrUserNotFound) ||
		errors.Is(err, entity.ErrIncorrectPassword) ||
		errors.Is(err, entity.ErrCredentialsRequired) ||
		isCredentialsRuleError(err) {
		return fail(c, http.StatusBadRequest, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("invalid credentials"),
		}, err)
	}

	// Already classified as ours — the database was unreachable, a query would
	// not build. Nothing to add: the chain carries the cause to the log site.
	if errors.Is(err, cerr.Internal) {
		return fail(c, http.StatusInternalServerError, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to login user"),
		}, err)
	}

	// Matched nothing this handler knows. The client gets the same opaque 500
	// as a real failure, and the chain says why: a sentinel reached the
	// transport that nobody taught it how to answer.
	return fail(c, http.StatusInternalServerError, openapi.Envelope{
		Status: openapi.Failure,
		Error:  new("failed to login user"),
	}, cerr.New("unclassified error from GetUserIDByCreds", err, cerr.Internal))
}

// isCredentialsRuleError reports whether err is one of the credential rules the
// service validates before it looks anything up. The rules are listed once, in
// validationRules, so a rule added for registration is answered here too.
func isCredentialsRuleError(err error) bool {
	for _, rule := range validationRules {
		if errors.Is(err, rule.err) {
			return true
		}
	}

	return false
}
