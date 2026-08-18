// Package authn guards the operations the OpenAPI spec marks as secured: it
// turns a bearer access token into the id of whoever it belongs to, and hands
// that id to the handler behind it.
package authn

import (
	"errors"
	"net/http"
	"strings"

	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/labstack/echo/v4"
)

// bearerScheme is the only authorization scheme this API answers to; the header
// it is matched against is case-insensitive, the challenge it is sent back in is
// spelled the canonical way.
const (
	bearerScheme    = "bearer"
	bearerChallenge = "Bearer"
)

// userIDKey is unexported, so this package is the only thing in the process that
// can stamp an identity onto a request.
const userIDKey = "authn.user_id"

// TokenVerifier is what the guard needs of the business layer: turn an access
// token into the user it identifies. Named for that capability, not for the
// service that happens to provide it.
type TokenVerifier interface {
	Authenticate(accessToken string) (int, error)
}

// Middleware rejects any request that does not carry a valid access token, and
// stamps the id behind a valid one onto the request for the handler to read.
func Middleware(verifier TokenVerifier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, err := bearerToken(c.Request().Header.Get(echo.HeaderAuthorization))
			if err != nil {
				return reject(c, err)
			}

			userID, err := verifier.Authenticate(token)
			if err != nil {
				return reject(c, err)
			}

			c.Set(userIDKey, userID)

			return next(c)
		}
	}
}

// UserID returns the id the guard stamped on the request. It fails only when the
// route it is called from was never guarded: a wiring bug, not a client one, and
// one that must surface rather than quietly serve whoever user 0 is.
func UserID(c echo.Context) (int, error) {
	userID, ok := c.Get(userIDKey).(int)
	if !ok {
		return 0, cerr.New("request is not authenticated", cerr.Internal).
			Loc().
			Time().
			With("route", c.Path())
	}

	return userID, nil
}

// bearerToken pulls the credential out of an Authorization header. The scheme is
// matched case-insensitively because RFC 7235 says it is case-insensitive.
func bearerToken(header string) (string, error) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, bearerScheme) {
		return "", cerr.New("authorization header is missing or not bearer", cerr.Unauthorized).
			Loc().
			Time()
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", cerr.New("bearer token is empty", cerr.Unauthorized).Loc().Time()
	}

	return token, nil
}

// reject answers the client and hands err back up to the single log site in the
// middleware chain, the same contract the handler groups follow. The body is
// deliberately uniform: the client learns that it is unauthenticated, never
// whether the token was missing, malformed, expired or forged. The error chain
// carries all of that to the log.
func reject(c echo.Context, err error) error {
	status := http.StatusUnauthorized
	message := "unauthorized"

	if errors.Is(err, cerr.Internal) {
		status = http.StatusInternalServerError
		message = "failed to authenticate request"
	} else {
		// RFC 6750: a 401 has to name the scheme the client should retry with.
		c.Response().Header().Set(echo.HeaderWWWAuthenticate, bearerChallenge)
	}

	if writeErr := c.JSON(status, openapi.Envelope{
		Status: openapi.Failure,
		Error:  new(message),
	}); writeErr != nil {
		return writeErr
	}

	return err
}
