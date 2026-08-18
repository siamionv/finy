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

func refresh(t *testing.T, svc auth.Authenticator, body string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	e.POST("/api/v1/auth/refresh", auth.New(auth.Deps{
		Logger:        slog.New(slog.DiscardHandler),
		Authenticator: svc,
	}).RefreshTokens)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec
}

const validRefreshBody = `{"refresh_token":"dGhpc19pc19hX3JlZnJlc2hfdG9rZW4"}`

func TestRefreshTokens_OK(t *testing.T) {
	rec := refresh(t, &fakeAuth{pair: entity.TokenPair{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
	}}, validRefreshBody)

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
	if got.Data.AccessToken != "new-access" || got.Data.RefreshToken != "new-refresh" {
		t.Errorf("data = %+v, want the minted pair", *got.Data)
	}
}

func TestRefreshTokens_InvalidToken(t *testing.T) {
	rec := refresh(t, &fakeAuth{err: entity.ErrInvalidRefreshToken.Loc().Time()}, validRefreshBody)

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

func TestRefreshTokens_InternalFailure(t *testing.T) {
	rec := refresh(t, &fakeAuth{err: cerr.New("failed to refresh", cerr.Internal).Loc().Time()},
		validRefreshBody)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500 (%s)", rec.Code, rec.Body)
	}
}

func TestRefreshTokens_UnclassifiedFailure(t *testing.T) {
	rec := refresh(t, &fakeAuth{err: cerr.New("something new happened")}, validRefreshBody)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500 (%s)", rec.Code, rec.Body)
	}
}

func TestRefreshTokens_MalformedPayload(t *testing.T) {
	rec := refresh(t, &fakeAuth{}, `{"refresh_token":`)

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
