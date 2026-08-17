package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/siamionv/finy/pkg/cerr"
)

// run drives handler through the logging middleware and returns the records it
// emitted, decoded.
func run(t *testing.T, handler echo.HandlerFunc) []map[string]any {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	e := echo.New()
	e.Use(loggingMiddleware(logger))
	e.POST("/v1/auth/register", handler)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/auth/register", nil))

	var records []map[string]any
	for line := range bytes.Lines(buf.Bytes()) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("log line is not json: %v (%q)", err, line)
		}
		records = append(records, record)
	}

	return records
}

// The contract handlers rely on: answer the client, then return the error so
// this single log site can record it. The response must survive untouched and
// the error must reach the log.
func TestLoggingMiddleware_LogsErrorOfAnAlreadyAnsweredRequest(t *testing.T) {
	err := cerr.New("failed to insert user", cerr.Internal).
		Loc().
		Time().
		With("username", "kate")

	records := run(t, func(c echo.Context) error {
		if writeErr := c.JSON(
			http.StatusInternalServerError,
			map[string]string{"error": "nope"},
		); writeErr != nil {
			return writeErr
		}

		return err
	})

	last := records[len(records)-1]
	if last["msg"] != "request failed" {
		t.Errorf("msg = %v, want %q", last["msg"], "request failed")
	}
	if last["status"] != float64(http.StatusInternalServerError) {
		t.Errorf("status = %v, want 500", last["status"])
	}

	logged, ok := last["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %v (%T), want a group of attributes", last["error"], last["error"])
	}
	if logged["msg"] != "failed to insert user" {
		t.Errorf("error.msg = %v, want %q", logged["msg"], "failed to insert user")
	}
	if logged["username"] != "kate" {
		t.Errorf("error.username = %v, want %q", logged["username"], "kate")
	}
	if _, ok := logged["cerr.location"]; !ok {
		t.Error("error.cerr.location is missing")
	}
	// The regression this guards: a func value here marshals to "!ERROR:...".
	ts, ok := logged["cerr.timestamp"].(string)
	if !ok {
		t.Fatalf(
			"error.cerr.timestamp = %v (%T), want an RFC3339 string",
			logged["cerr.timestamp"],
			logged["cerr.timestamp"],
		)
	}
	if len(ts) == 0 {
		t.Error("error.cerr.timestamp is empty")
	}
}

// A 4xx the handler already answered is the client's fault, not ours: it must
// be recorded, but never at error level.
func TestLoggingMiddleware_RejectionsLogAtWarn(t *testing.T) {
	records := run(t, func(c echo.Context) error {
		if writeErr := c.JSON(
			http.StatusBadRequest,
			map[string]string{"error": "nope"},
		); writeErr != nil {
			return writeErr
		}

		return cerr.New("failed to bind register user request", cerr.Invalid).Loc().Time()
	})

	last := records[len(records)-1]
	if last["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", last["level"])
	}
	if last["msg"] != "request rejected" {
		t.Errorf("msg = %v, want %q", last["msg"], "request rejected")
	}
	if last["status"] != float64(http.StatusBadRequest) {
		t.Errorf("status = %v, want 400", last["status"])
	}
}

// An error returned before anything was written still has to be rendered, and
// the status logged has to be the rendered one rather than the unset zero.
func TestLoggingMiddleware_RendersUnhandledError(t *testing.T) {
	records := run(t, func(_ echo.Context) error {
		return echo.NewHTTPError(http.StatusTeapot, "short and stout")
	})

	last := records[len(records)-1]
	if last["status"] != float64(http.StatusTeapot) {
		t.Errorf("status = %v, want 418", last["status"])
	}
	if last["msg"] != "request rejected" {
		t.Errorf("msg = %v, want %q", last["msg"], "request rejected")
	}
}
