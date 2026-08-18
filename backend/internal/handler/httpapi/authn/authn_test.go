package authn_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/internal/handler/httpapi/authn"
	"github.com/siamionv/finy/pkg/cerr"
)

// fakeVerifier answers with whatever the case under test needs, and records the
// token it was handed so the header parsing can be asserted on.
type fakeVerifier struct {
	userID int
	err    error
	seen   string
}

func (f *fakeVerifier) Authenticate(accessToken string) (int, error) {
	f.seen = accessToken

	return f.userID, f.err
}

func decode[T any](t *testing.T, r io.Reader) T {
	t.Helper()

	var value T
	if err := json.NewDecoder(r).Decode(&value); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return value
}

// guard drives one request through the middleware and reports what the handler
// behind it saw, which is nothing at all when the request was rejected.
type guarded struct {
	rec    *httptest.ResponseRecorder
	err    error
	ran    bool
	userID int
}

func guard(t *testing.T, verifier authn.TokenVerifier, header string) guarded {
	t.Helper()

	result := guarded{rec: httptest.NewRecorder()}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	if header != "" {
		req.Header.Set(echo.HeaderAuthorization, header)
	}

	e := echo.New()
	c := e.NewContext(req, result.rec)
	c.SetPath("/api/v1/protected")

	next := func(c echo.Context) error {
		result.ran = true

		userID, err := authn.UserID(c)
		if err != nil {
			return err
		}

		result.userID = userID

		return c.NoContent(http.StatusOK)
	}

	result.err = authn.Middleware(verifier)(next)(c)

	return result
}

// The happy path is the whole point: the id behind the token has to reach the
// handler, and it has to arrive as an id rather than as a token to re-parse.
func TestMiddleware_HandsTheSubjectToTheHandler(t *testing.T) {
	verifier := &fakeVerifier{userID: 42}

	result := guard(t, verifier, "Bearer good-token")

	if result.err != nil {
		t.Fatalf("middleware returned %v, want nil", result.err)
	}
	if !result.ran {
		t.Fatal("the handler behind the guard never ran")
	}
	if result.userID != 42 {
		t.Errorf("user id = %d, want 42", result.userID)
	}
	if verifier.seen != "good-token" {
		t.Errorf("verified %q, want the credential without its scheme", verifier.seen)
	}
	if result.rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", result.rec.Code)
	}
}

// RFC 7235 makes the scheme case-insensitive, so a client that sends it lowercase
// is a correct client.
func TestMiddleware_MatchesTheSchemeCaseInsensitively(t *testing.T) {
	verifier := &fakeVerifier{userID: 7}

	result := guard(t, verifier, "bEaReR good-token")

	if result.err != nil {
		t.Fatalf("middleware returned %v, want nil", result.err)
	}
	if result.userID != 7 {
		t.Errorf("user id = %d, want 7", result.userID)
	}
}

// Every way of failing to present a usable credential ends the same way: the
// handler never runs, the client gets one uniform 401, and the reason travels
// upward to the single log site instead of out to the client.
func TestMiddleware_RejectsWhatItCannotAuthenticate(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		verifier *fakeVerifier
	}{
		{
			name:     "no authorization header",
			header:   "",
			verifier: &fakeVerifier{userID: 1},
		},
		{
			name:     "credential without a scheme",
			header:   "some-token",
			verifier: &fakeVerifier{userID: 1},
		},
		{
			name:     "a scheme this api does not speak",
			header:   "Basic dXNlcjpwYXNz",
			verifier: &fakeVerifier{userID: 1},
		},
		{
			name:     "bearer with nothing behind it",
			header:   "Bearer    ",
			verifier: &fakeVerifier{userID: 1},
		},
		{
			name:     "a token the verifier turns down",
			header:   "Bearer stale-token",
			verifier: &fakeVerifier{err: entity.ErrInvalidAccessToken.Loc().Time()},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := guard(t, tc.verifier, tc.header)

			if result.ran {
				t.Error("the handler behind the guard ran on a rejected request")
			}
			if result.rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", result.rec.Code)
			}
			if got := result.rec.Header().Get(echo.HeaderWWWAuthenticate); got != "Bearer" {
				t.Errorf("WWW-Authenticate = %q, want %q", got, "Bearer")
			}
			if result.err == nil {
				t.Fatal("middleware returned nil, want the error the log site records")
			}
			if !errors.Is(result.err, cerr.Unauthorized) {
				t.Errorf("err = %v, want it to classify as unauthorized", result.err)
			}

			body := decode[openapi.Envelope](t, result.rec.Body)
			if body.Status != openapi.Failure {
				t.Errorf("status = %v, want %v", body.Status, openapi.Failure)
			}
			if body.Error == nil || *body.Error != "unauthorized" {
				t.Errorf("error = %v, want a uniform %q", body.Error, "unauthorized")
			}
		})
	}
}

// A signing key we failed to configure is our fault, not the caller's: answering
// 401 would tell them to go and fetch a token that could never work.
func TestMiddleware_AnswersOurOwnFailuresWithA500(t *testing.T) {
	verifier := &fakeVerifier{err: entity.ErrMissingSigningKey.Loc().Time()}

	result := guard(t, verifier, "Bearer good-token")

	if result.rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", result.rec.Code)
	}
	if got := result.rec.Header().Get(echo.HeaderWWWAuthenticate); got != "" {
		t.Errorf("WWW-Authenticate = %q, want it unset on a 500", got)
	}
	if !errors.Is(result.err, entity.ErrMissingSigningKey) {
		t.Errorf("err = %v, want %v", result.err, entity.ErrMissingSigningKey)
	}
}

// Reading an identity off a route nobody guarded must fail loudly. Returning a
// zero id instead would serve every such request as whoever user 0 turns out to be.
func TestUserID_FailsOnAnUnguardedRoute(t *testing.T) {
	e := echo.New()
	c := e.NewContext(
		httptest.NewRequest(http.MethodGet, "/api/v1/public", nil),
		httptest.NewRecorder(),
	)

	_, err := authn.UserID(c)
	if !errors.Is(err, cerr.Internal) {
		t.Fatalf("err = %v, want it to classify as internal", err)
	}
}
