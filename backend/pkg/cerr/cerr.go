// Package cerr provides the single error constructor used across finy: a message,
// the errors it wraps, and structured fields for logs and traces.
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
	const knownSuffix = "pkg/cerr/cerr.go"
	basePath = strings.TrimSuffix(file, knownSuffix)
}

// Error carries a human message, the errors it wraps (sentinels, kinds and causes,
// all equal for identity), and key/value fields for logs and traces.
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

// With returns a new error carrying kv that wraps e. It never mutates the receiver:
// sentinels are shared package-level values. e is wrapped rather than copied, so
// errors.Is(Sentinel.With(...), Sentinel) still holds.
func (e *Error) With(kv ...any) *Error {
	c := &Error{errs: []error{e}}
	if len(kv) > 0 {
		c.fields = make([]any, len(kv))
		copy(c.fields, kv)
	}
	return c
}

// Wrap attaches cause to e without restating e's message. Use it when a predefined
// error already carries the message and the only thing left to add is the cause.
func (e *Error) Wrap(cause error) *Error {
	return New("", cause, e)
}

// Join returns a new error wrapping e plus errs (typically kind values). nil
// entries are dropped, and errors.Is still matches e alongside every joined error.
func (e *Error) Join(errs ...error) *Error {
	newErrs := make([]error, 0, len(errs)+1)
	newErrs = append(newErrs, e)
	for _, err := range errs {
		if err != nil {
			newErrs = append(newErrs, err)
		}
	}
	return &Error{errs: newErrs}
}

// Error renders msg followed by each wrapped error's message, joined with ": ".
// kind values are skipped: they classify, they are not prose.
func (e *Error) Error() string {
	parts := make([]string, 0, len(e.errs)+1)
	if e.msg != "" {
		parts = append(parts, e.msg)
	}
	for _, err := range e.errs {
		//nolint:errorlint // must match only a top-level kind, not one nested deeper
		if _, ok := err.(kind); ok {
			continue
		}
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, ": ")
}

// Unwrap exposes every wrapped error so errors.Is/As walk the whole tree.
func (e *Error) Unwrap() []error { return e.errs }

// LogValue implements slog.LogValuer: the rendered message plus every field in the
// chain, so logging an *Error emits structured attributes instead of a flat string.
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

// Loc stamps e with the file:line of its caller. Call it only where the error is
// minted: a second Loc further up adds an attribute rather than replacing the first.
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

// Time stamps e with the moment it was minted. Call it where the error is created,
// not where it is handled.
func (e *Error) Time() *Error {
	return e.With("cerr.timestamp", time.Now())
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
	//nolint:errorlint // recursion below walks the chain; matching at depth double-counts
	if e, ok := err.(*Error); ok {
		*out = append(*out, e.fields...)
	}
	//nolint:errorlint // checking for the Unwrap capability, not a concrete type
	switch u := err.(type) {
	case interface{ Unwrap() error }:
		collect(u.Unwrap(), out)
	case interface{ Unwrap() []error }:
		for _, w := range u.Unwrap() {
			collect(w, out)
		}
	}
}
