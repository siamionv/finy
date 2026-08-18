package user_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/internal/handler/httpapi/authn"
	"github.com/siamionv/finy/internal/handler/httpapi/v1/user"
	"github.com/siamionv/finy/pkg/cerr"
)

// fakeProfiles answers with whatever the case under test needs, and records the
// id the handler read off the guarded request.
type fakeProfiles struct {
	user *entity.User
	err  error
	seen int
}

func (f *fakeProfiles) GetUser(_ context.Context, id int) (*entity.User, error) {
	f.seen = id

	if f.err != nil {
		return nil, f.err
	}

	return f.user, nil
}

func decode[T any](t *testing.T, r io.Reader) T {
	t.Helper()

	var value T
	if err := json.NewDecoder(r).Decode(&value); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return value
}

// me drives one request through the handler. A userID of 0 stands for a route
// that was never guarded, so nothing stamps an identity onto the request.
func me(t *testing.T, profiles user.ProfileReader, userID int) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)

	handler := user.New(user.Deps{
		Logger:        slog.New(slog.DiscardHandler),
		ProfileReader: profiles,
	}).GetCurrentUser

	e := echo.New()
	if userID != 0 {
		// The real guard, not a stand-in: the key it stamps under is unexported,
		// so this is also the only way to prove the handler reads what it writes.
		req.Header.Set(echo.HeaderAuthorization, "Bearer any-token")
		e.GET("/api/v1/users/me", handler, authn.Middleware(constVerifier(userID)))
	} else {
		e.GET("/api/v1/users/me", handler)
	}

	e.ServeHTTP(rec, req)

	return rec
}

// constVerifier accepts any token and always names the same user.
type constVerifier int

func (v constVerifier) Authenticate(string) (int, error) { return int(v), nil }

func TestGetCurrentUserReturnsTheGuardedIdentity(t *testing.T) {
	iconURL := "https://finy.by/icons/default.png"
	createdAt := time.Date(2026, time.August, 18, 10, 0, 0, 0, time.UTC)

	profiles := &fakeProfiles{user: &entity.User{
		ID:        42,
		Username:  "johndoe",
		IconURL:   &iconURL,
		CreatedAt: createdAt,
	}}

	rec := me(t, profiles, 42)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}

	if profiles.seen != 42 {
		t.Errorf("service saw id %d, want the id the guard stamped (42)", profiles.seen)
	}

	body := decode[openapi.EnvelopedUser](t, rec.Body)
	if body.Status != openapi.Success {
		t.Errorf("status = %q, want success", body.Status)
	}
	if body.Data == nil {
		t.Fatal("expected a user in the envelope")
	}
	if body.Data.Id != 42 || body.Data.Username != "johndoe" {
		t.Errorf("got %+v, want id 42 / johndoe", *body.Data)
	}
	if body.Data.CreatedAt != createdAt.Format(time.RFC3339) {
		t.Errorf("created_at = %q, want RFC3339", body.Data.CreatedAt)
	}
}

func TestGetCurrentUserNotFound(t *testing.T) {
	rec := me(t, &fakeProfiles{err: entity.ErrUserNotFound}, 42)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body)
	}

	assertFailure(t, rec, "user not found")
}

func TestGetCurrentUserInternalError(t *testing.T) {
	profiles := &fakeProfiles{err: cerr.New("boom", cerr.Internal)}

	rec := me(t, profiles, 42)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body)
	}

	assertFailure(t, rec, "failed to get current user")
}

// An unguarded route is a wiring bug: the handler must refuse rather than serve
// whoever user 0 is.
func TestGetCurrentUserUnguardedRouteIs500(t *testing.T) {
	profiles := &fakeProfiles{user: &entity.User{ID: 1}}

	rec := me(t, profiles, 0)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body)
	}
	if profiles.seen != 0 {
		t.Errorf("service was called with id %d; it must not be called at all", profiles.seen)
	}
}

func assertFailure(t *testing.T, rec *httptest.ResponseRecorder, wantError string) {
	t.Helper()

	body := decode[openapi.Envelope](t, rec.Body)
	if body.Status != openapi.Failure {
		t.Errorf("status = %q, want failure", body.Status)
	}
	if body.Error == nil || *body.Error != wantError {
		t.Errorf("error = %v, want %q", body.Error, wantError)
	}
}
