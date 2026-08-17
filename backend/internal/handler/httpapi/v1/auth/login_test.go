package auth_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/internal/handler/httpapi/v1/auth"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/labstack/echo/v4"
)

// fakeTokenService answers with the pair — or the failure — the case under test
// needs the token service to have produced.
type fakeTokenService struct {
	pair entity.TokenPair
	err  error
}

func (f *fakeTokenService) Mint(_ int) (entity.TokenPair, error) {
	return f.pair, f.err
}

func login(
	t *testing.T,
	userSvc auth.UserService,
	tokenSvc auth.TokenService,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	e.POST("/api/v1/auth/login", auth.New(auth.Deps{
		Logger:       slog.New(slog.DiscardHandler),
		UserService:  userSvc,
		TokenService: tokenSvc,
	}).LoginUser)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec
}

// The 200 body is EnvelopedTokenPair: a client switching on `status` must not
// break on the one response it actually wants.
func TestLoginUser_OK(t *testing.T) {
	rec := login(t,
		&fakeUserService{user: &entity.User{ID: 142}},
		&fakeTokenService{pair: entity.TokenPair{
			AccessToken:  "access",
			RefreshToken: "refresh",
		}},
		validBody,
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (%s)", rec.Code, rec.Body)
	}

	got := decode[openapi.EnvelopedTokenPair](t, rec.Body)
	if got.Status != openapi.Success {
		t.Errorf("status = %q, want %q", got.Status, openapi.Success)
	}
	if got.Data == nil {
		t.Fatal("data is absent: the token pair was not enveloped")
	}
	if got.Data.AccessToken != "access" || got.Data.RefreshToken != "refresh" {
		t.Errorf("data = %+v, want the minted pair", *got.Data)
	}
}

// The spec documents 404 for an unknown username.
func TestLoginUser_UserNotFound(t *testing.T) {
	rec := login(t,
		// Stamped the way the repository stamps it, wrapped the way the service
		// wraps it: the handler has to match through both.
		&fakeUserService{err: cerr.New("failed to get user by username",
			entity.ErrUserNotFound.Loc().Time().With("username", "johndoe"))},
		&fakeTokenService{},
		validBody,
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404 (%s)", rec.Code, rec.Body)
	}
	if got := decode[openapi.Envelope](t, rec.Body); got.Status != openapi.Failure {
		t.Errorf("status = %q, want %q", got.Status, openapi.Failure)
	}
}

func TestLoginUser_IncorrectPassword(t *testing.T) {
	rec := login(
		t,
		&fakeUserService{err: entity.ErrIncorrectPassword},
		&fakeTokenService{},
		validBody,
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 (%s)", rec.Code, rec.Body)
	}

	got := decode[openapi.Envelope](t, rec.Body)
	if got.Status != openapi.Failure {
		t.Errorf("status = %q, want %q", got.Status, openapi.Failure)
	}
	if got.Error == nil || *got.Error == "" {
		t.Fatal("no error message")
	}
}

// Login is not the endpoint that teaches the rules. Every rule the service can
// reject credentials with has to come back as the same opaque 400, or the
// response says which half of the pair was wrong — and, given a rejected
// password, which rule it broke.
func TestLoginUser_CredentialRulesAreNotSpelledOut(t *testing.T) {
	rules := []struct {
		name string
		err  *cerr.Error
	}{
		{"username too short", entity.ErrUsernameTooShort},
		{"username invalid start", entity.ErrUsernameInvalidStart},
		{"username invalid characters", entity.ErrUsernameInvalidCharacters},
		{"password too short", entity.ErrPasswordTooShort},
		{"password missing uppercase", entity.ErrPasswordMissingUppercase},
		{"password missing lowercase", entity.ErrPasswordMissingLowercase},
		{"password missing digit", entity.ErrPasswordMissingDigit},
		{"password missing special symbol", entity.ErrPasswordMissingSpecialSymbol},
		{"password invalid characters", entity.ErrPasswordInvalidCharacters},
	}

	for _, tc := range rules {
		t.Run(tc.name, func(t *testing.T) {
			// Stamped and wrapped exactly as the service does it.
			rec := login(t,
				&fakeUserService{err: cerr.New("failed to validate credentials",
					tc.err.Loc().Time()).With("username", "johndoe")},
				&fakeTokenService{},
				validBody,
			)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400 (%s)", rec.Code, rec.Body)
			}

			got := decode[openapi.Envelope](t, rec.Body)
			if got.Error == nil {
				t.Fatal("no error message")
			}
			if *got.Error != "invalid credentials" {
				t.Errorf("error = %q, want the same opaque message as a wrong password", *got.Error)
			}
			if got.Data != nil {
				t.Errorf(
					"data = %+v, want nothing: field-level detail belongs to register",
					*got.Data,
				)
			}
		})
	}
}

// A failure the layers below already classified as ours stays a 500 and tells
// the client nothing about it.
func TestLoginUser_InternalFailure(t *testing.T) {
	rec := login(t,
		&fakeUserService{err: cerr.New("failed to get user by username",
			cerr.New("failed to select user", cerr.Internal).Loc().Time())},
		&fakeTokenService{},
		validBody,
	)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500 (%s)", rec.Code, rec.Body)
	}
}

// A sentinel this handler has never been taught is indistinguishable to the
// client from a real failure — and must not become an accidental 200 or 400.
func TestLoginUser_UnclassifiedFailure(t *testing.T) {
	rec := login(t,
		&fakeUserService{err: cerr.New("something new happened")},
		&fakeTokenService{},
		validBody,
	)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500 (%s)", rec.Code, rec.Body)
	}
}

// The credentials were right and the tokens still could not be signed: ours to
// own, and no pair may reach the client alongside the error.
func TestLoginUser_MintFailure(t *testing.T) {
	rec := login(t,
		&fakeUserService{user: &entity.User{ID: 1}},
		&fakeTokenService{err: entity.ErrMissingSigningKey.Loc().Time()},
		validBody,
	)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500 (%s)", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "access_token") {
		t.Errorf("token fields in a failed response: %s", rec.Body)
	}
}

func TestLoginUser_MalformedPayload(t *testing.T) {
	rec := login(t, &fakeUserService{}, &fakeTokenService{}, `{"username":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 (%s)", rec.Code, rec.Body)
	}
}
