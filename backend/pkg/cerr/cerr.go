// Package cerr provides the single error constructor used across go-common:
// a message, the errors it wraps, and structured fields for logs and traces.
package cerr

import (
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// basePath is the absolute prefix to strip so paths become project-relative.
var basePath string

func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	// file looks like /Users/.../finy/backend/pkg/cerr/cerr.go
	// this package's known path relative to repo root:
	const knownSuffix = "pkg/cerr/cerr.go"
	basePath = strings.TrimSuffix(file, knownSuffix)
}

// Error carries a human message, the errors it wraps (sentinels, kinds and
// causes — all equal for identity), and key/value fields for logs and traces.
type Error struct {
	msg    string
	errs   []error
	fields []any
}

// New builds an *Error wrapping errs. nil entries are dropped. New never
// returns nil, so the result is always safe to chain with With.
func New(msg string, errs ...error) *Error {
	e := &Error{msg: msg}
	for _, err := range errs {
		if err != nil {
			e.errs = append(e.errs, err)
		}
	}
	return e
}

// With returns a copy of e with kv appended to its fields. It never mutates the
// receiver: predefined errors are shared package-level values, and mutating
// them would pollute every user of the package.
func (e *Error) With(kv ...any) *Error {
	c := &Error{msg: e.msg}
	if len(e.errs) > 0 {
		c.errs = make([]error, len(e.errs))
		copy(c.errs, e.errs)
	}
	if n := len(e.fields) + len(kv); n > 0 {
		c.fields = make([]any, 0, n)
		c.fields = append(c.fields, e.fields...)
		c.fields = append(c.fields, kv...)
	}
	return c
}

// Wrap returns a new error that attaches cause to e without restating e's
// message: msg is left empty, so Error() renders "<cause>: <e's message>"
// with no duplication, while e itself stays in the chain so errors.Is(result, e)
// still holds. Use this when a predefined error already carries the message
// you want and the only thing left to add is the cause.
func (e *Error) Wrap(cause error) *Error {
	return New("", cause, e)
}

// Error renders msg followed by the message of each wrapped error, in the order
// given, joined with ": ". kind values are skipped — they classify, they are
// not prose. Fields are deliberately absent: they belong to LogValue/Fields.
func (e *Error) Error() string {
	parts := make([]string, 0, len(e.errs)+1)
	if e.msg != "" {
		parts = append(parts, e.msg)
	}
	for _, err := range e.errs {
		// Direct assertion, not errors.As: this must match only when err
		// itself is a kind value, not when a wrapped *Error happens to
		// carry one deeper in its own chain.
		if _, ok := err.(kind); ok { //nolint:errorlint
			continue
		}
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, ": ")
}

// Unwrap exposes every wrapped error so errors.Is/As walk the whole tree.
func (e *Error) Unwrap() []error { return e.errs }

// LogValue implements slog.LogValuer: the rendered message plus every field
// collected from the chain, so `logger.Error("...", "error", err)` emits
// structured attributes instead of a flat string.
func (e *Error) LogValue() slog.Value {
	fields := Fields(e)
	attrs := make([]slog.Attr, 0, 1+len(fields)/2)
	attrs = append(attrs, slog.String("msg", e.Error()))
	for i := 0; i+1 < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		attrs = append(attrs, slog.Any(key, fields[i+1]))
	}
	return slog.GroupValue(attrs...)
}

func (e *Error) Loc() *Error {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return e
	}

	file = strings.TrimPrefix(file, basePath)
	lineStr := strconv.Itoa(line)

	sb := strings.Builder{}
	sb.Grow(len(file) + 1 + 4)

	sb.WriteString(file)
	sb.WriteByte(':')
	sb.WriteString(lineStr)

	loc := sb.String()

	return e.With("cerr.location", loc)
}

func (e *Error) Time() *Error {
	timestamp := time.Now

	return e.With("cerr.timestamp", timestamp)
}

// Fields collects the key/value pairs attached with With from err and every
// error it wraps, outermost first.
func Fields(err error) []any {
	var out []any
	collect(err, &out)
	return out
}

func collect(err error, out *[]any) {
	if err == nil {
		return
	}
	// Direct assertion, not errors.As: recursion below already walks the
	// Unwrap chain, so matching *Error at any depth here would double-count
	// fields from nested *Error values.
	if e, ok := err.(*Error); ok { //nolint:errorlint
		*out = append(*out, e.fields...)
	}
	// Checking for the Unwrap capability, not a specific error type, so
	// errors.As doesn't apply here.
	switch u := err.(type) { //nolint:errorlint
	case interface{ Unwrap() error }:
		collect(u.Unwrap(), out)
	case interface{ Unwrap() []error }:
		for _, w := range u.Unwrap() {
			collect(w, out)
		}
	}
}
