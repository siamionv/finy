package auth_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/internal/handler/httpapi/v1/auth"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/labstack/echo/v4"
)

// fakeAuth answers with whatever the case under test needs the business layer to
// have produced, stamped the way the real one stamps it.
type fakeAuth struct {
	user *entity.User
	pair entity.TokenPair
	err  error
}

func (f *fakeAuth) Register(
	_ context.Context,
	_ entity.UserCredentials,
) (*entity.User, error) {
	return f.user, f.err
}

func (f *fakeAuth) Login(
	_ context.Context,
	_ entity.UserCredentials,
) (entity.TokenPair, error) {
	return f.pair, f.err
}

func post(t *testing.T, svc auth.Authenticator, body string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	e.POST("/api/v1/auth/register", auth.New(auth.Deps{
		Logger:        slog.New(slog.DiscardHandler),
		Authenticator: svc,
	}).RegisterUser)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec
}

func decode[T any](t *testing.T, r io.Reader) T {
	t.Helper()

	var out T
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		t.Fatalf("response is not the expected shape: %v", err)
	}

	return out
}

const validBody = `{"username":"johndoe","password":"Correct9Horse!"}`

// The spec declares the 201 body as EnvelopedUser, and every error path here is
// already enveloped: a client switching on `status` must not break on success.
func TestRegisterUser_Created(t *testing.T) {
	created := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	rec := post(t, &fakeAuth{
		user: &entity.User{ID: 142, Username: "johndoe", CreatedAt: created},
	}, validBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201 (%s)", rec.Code, rec.Body)
	}

	got := decode[openapi.EnvelopedUser](t, rec.Body)
	if got.Status != openapi.Success {
		t.Errorf("status = %q, want %q", got.Status, openapi.Success)
	}
	if got.Data == nil {
		t.Fatal("data is absent: the user was not enveloped")
	}
	if got.Data.Id != 142 || got.Data.Username != "johndoe" {
		t.Errorf("data = %+v, want the created user", *got.Data)
	}
	if got.Data.CreatedAt != created.Format(time.RFC3339) {
		t.Errorf("created_at = %q, want RFC3339", got.Data.CreatedAt)
	}
}

// Regression: the service stamps its sentinels with Loc/Time, and the handler
// matches them with errors.Is. When stamping broke that identity, every one of
// these fell through to an opaque 500.
func TestRegisterUser_ValidationFailureIsRendered(t *testing.T) {
	tests := []struct {
		name string
		err  *cerr.Error
		code openapi.ValidationErrorCode
	}{
		{"username too short", entity.ErrUsernameTooShort, openapi.UsernameTooShort},
		{"username invalid start", entity.ErrUsernameInvalidStart, openapi.UsernameInvalidStart},
		{
			"username invalid characters",
			entity.ErrUsernameInvalidCharacters,
			openapi.UsernameInvalidCharacters,
		},
		{"password too short", entity.ErrPasswordTooShort, openapi.PasswordTooShort},
		{
			"password missing uppercase",
			entity.ErrPasswordMissingUppercase,
			openapi.PasswordMissingUppercase,
		},
		{
			"password missing lowercase",
			entity.ErrPasswordMissingLowercase,
			openapi.PasswordMissingLowercase,
		},
		{"password missing digit", entity.ErrPasswordMissingDigit, openapi.PasswordMissingDigit},
		{
			"password missing special symbol",
			entity.ErrPasswordMissingSpecialSymbol,
			openapi.PasswordMissingSpecialSymbol,
		},
		{
			"password invalid characters",
			entity.ErrPasswordInvalidCharacters,
			openapi.PasswordInvalidCharacters,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Stamped exactly as the service stamps it.
			rec := post(t, &fakeAuth{err: tc.err.Loc().Time()}, validBody)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400 (%s)", rec.Code, rec.Body)
			}

			got := decode[openapi.EnvelopedValidationError](t, rec.Body)
			if len(got.Data.Errors) != 1 {
				t.Fatalf("got %d field errors, want 1: %+v", len(got.Data.Errors), got.Data.Errors)
			}
			if got.Data.Errors[0].Code != tc.code {
				t.Errorf("code = %q, want %q", got.Data.Errors[0].Code, tc.code)
			}
			if got.Data.Errors[0].Message == "" {
				t.Error("message is empty")
			}
		})
	}
}

func TestRegisterUser_Conflict(t *testing.T) {
	rec := post(t, &fakeAuth{
		err: entity.ErrUserAlreadyExist.Loc().Time().With("username", "johndoe"),
	}, validBody)

	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d, want 409 (%s)", rec.Code, rec.Body)
	}
	if got := decode[openapi.Envelope](t, rec.Body); got.Status != openapi.Failure {
		t.Errorf("status = %q, want %q", got.Status, openapi.Failure)
	}
}

func TestRegisterUser_InternalFailure(t *testing.T) {
	rec := post(t, &fakeAuth{err: entity.ErrFailedToCreateUser}, validBody)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500 (%s)", rec.Code, rec.Body)
	}
}

func TestRegisterUser_MalformedPayload(t *testing.T) {
	rec := post(t, &fakeAuth{}, `{"username":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 (%s)", rec.Code, rec.Body)
	}

	got := decode[openapi.EnvelopedValidationError](t, rec.Body)
	if len(got.Data.Errors) != 1 || got.Data.Errors[0].Code != openapi.PayloadInvalidJson {
		t.Errorf("errors = %+v, want a single payload_invalid_json", got.Data.Errors)
	}
}
