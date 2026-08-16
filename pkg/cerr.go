// Package cerr provides the single error constructor used across go-common:
// a message, the errors it wraps, and structured fields for logs and traces.
package cerr

import (
	"log/slog"
	"strings"
)

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
		if _, ok := err.(kind); ok {
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
	if e, ok := err.(*Error); ok {
		*out = append(*out, e.fields...)
	}
	switch u := err.(type) {
	case interface{ Unwrap() error }:
		collect(u.Unwrap(), out)
	case interface{ Unwrap() []error }:
		for _, w := range u.Unwrap() {
			collect(w, out)
		}
	}
}
