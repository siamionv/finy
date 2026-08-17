package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/siamionv/finy/internal/config"
	"github.com/siamionv/finy/internal/entity"
)

// panickingUserService stands in for a handler dependency that blows up, so the
// panic originates inside the real middleware chain rather than beside it.
type panickingUserService struct{}

func (panickingUserService) CreateUser(
	_ context.Context,
	_ entity.UserCredentials,
) (*entity.User, error) {
	panic("boom")
}

// serve drives a request through the chain New assembles — the ordering is the
// thing under test, so it must not be restated here.
func serve(t *testing.T, req *http.Request) (*httptest.ResponseRecorder, []map[string]any) {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	server := New(Deps{
		Config: config.HTTP{
			Addr:         "127.0.0.1:0",
			MaxBodySize:  "1K",
			DrainTimeout: time.Second,
		},
		Logger:      logger,
		UserService: panickingUserService{},
	})

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	var records []map[string]any
	for line := range bytes.Lines(buf.Bytes()) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("log line is not json: %v (%q)", err, line)
		}
		records = append(records, record)
	}

	return rec, records
}

func registerRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	return req
}

// Regression: Recover sat outside loggingMiddleware, so a recovered panic
// unwound past the log site and the 500s most worth seeing produced no record
// from the structured logger at all.
func TestServer_LogsPanickingRequests(t *testing.T) {
	rec, records := serve(t, registerRequest(`{"username":"johndoe","password":"Correct9Horse!"}`))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", rec.Code)
	}

	last := records[len(records)-1]
	if last["msg"] != "request failed" {
		t.Fatalf("last record = %v, want a %q record for the panic", last, "request failed")
	}
	if last["status"] != float64(http.StatusInternalServerError) {
		t.Errorf("status = %v, want 500", last["status"])
	}
	if last["request_id"] == "" || last["request_id"] == nil {
		t.Error("request_id is missing: RequestID must still run before the log site")
	}

	// A 500 with nothing attached is the half-fix: the cause has to travel with
	// the record, not to echo's stdout logger.
	logged, ok := last["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %v (%T), want a group of attributes", last["error"], last["error"])
	}
	if !strings.Contains(logged["msg"].(string), "boom") {
		t.Errorf("error.msg = %v, want the panic value", logged["msg"])
	}
	if stack, _ := logged["stack"].(string); !strings.Contains(stack, "CreateUser") {
		t.Error("error.stack does not reach the panicking frame")
	}
}

// Same ordering, other victim: BodyLimit answers 413 by itself.
func TestServer_LogsOversizedBodies(t *testing.T) {
	rec, records := serve(t, registerRequest(
		`{"username":"johndoe","password":"`+strings.Repeat("a", 2048)+`"}`,
	))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d, want 413", rec.Code)
	}

	last := records[len(records)-1]
	if last["status"] != float64(http.StatusRequestEntityTooLarge) {
		t.Errorf("status = %v, want 413 — the rejection never reached the log site", last["status"])
	}
}
