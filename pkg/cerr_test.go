package cerr_test

import (
	"errors"
	"testing"

	"github.com/infomediji/go-common/core/cerr"
)

func TestNew_DropsNilEntries(t *testing.T) {
	sentinel := errors.New("sentinel")
	err := cerr.New("m", nil, sentinel)
	unwrapped := err.Unwrap()
	if len(unwrapped) != 1 {
		t.Fatalf("got %d unwrapped errors, want 1", len(unwrapped))
	}
	if unwrapped[0] != sentinel {
		t.Errorf("got %v, want sentinel", unwrapped[0])
	}
}

func TestNew_NeverReturnsNil(t *testing.T) {
	err := cerr.New("m")
	if err == nil {
		t.Fatal("New must never return nil")
	}
	if got := err.Error(); got != "m" {
		t.Errorf("got %q, want %q", got, "m")
	}
}

func TestError_RendersMsgCauseSentinelInOrder(t *testing.T) {
	err := cerr.New("delete object", errors.New("boom"), errors.New("s3: delete"))
	want := "delete object: boom: s3: delete"
	if got := err.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestError_SkipsKinds(t *testing.T) {
	err := cerr.New("m", cerr.NotFound)
	if got := err.Error(); got != "m" {
		t.Errorf("got %q, want %q", got, "m")
	}
}

func TestError_EmptyMsgJoinsOnlyWrappedErrors(t *testing.T) {
	err := cerr.New("", errors.New("boom"))
	if got := err.Error(); got != "boom" {
		t.Errorf("got %q, want %q", got, "boom")
	}
}

func TestErrorsIs_FindsEveryWrappedError(t *testing.T) {
	sentinel := errors.New("sentinel")
	cause := errors.New("cause")
	err := cerr.New("m", cause, sentinel)
	if !errors.Is(err, sentinel) {
		t.Error("errors.Is must find sentinel")
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is must find cause")
	}
}

func TestErrorsIs_TwoLevelClassification(t *testing.T) {
	sentinel := cerr.New("s3: object not found", cerr.NotFound)
	err := cerr.New("get object", errors.New("boom"), sentinel)
	if !errors.Is(err, sentinel) {
		t.Error("errors.Is must find sentinel")
	}
	if !errors.Is(err, cerr.NotFound) {
		t.Error("errors.Is must find cerr.NotFound")
	}
	if errors.Is(err, cerr.Conflict) {
		t.Error("errors.Is must not find cerr.Conflict")
	}
}

type customError struct{ msg string }

func (e *customError) Error() string { return e.msg }

func TestErrorsAs_FindsCustomTypedCause(t *testing.T) {
	cause := &customError{msg: "custom failure"}
	err := cerr.New("m", cause)

	var target *customError
	if !errors.As(err, &target) {
		t.Fatal("errors.As must find the custom typed cause")
	}
	if target != cause {
		t.Errorf("got %v, want %v", target, cause)
	}
}

func TestWith_Copies(t *testing.T) {
	sentinel := errors.New("sentinel")
	base := cerr.New("m", sentinel)
	withA := base.With("k", "v")

	if fields := cerr.Fields(base); len(fields) != 0 {
		t.Errorf("Fields(base) = %v, want empty", fields)
	}
	fields := cerr.Fields(withA)
	if len(fields) != 2 || fields[0] != "k" || fields[1] != "v" {
		t.Errorf("Fields(withA) = %v, want [k v]", fields)
	}
	if base.Error() != withA.Error() {
		t.Errorf("base.Error() = %q, withA.Error() = %q", base.Error(), withA.Error())
	}
	if !errors.Is(withA, sentinel) {
		t.Error("errors.Is must find sentinel through withA")
	}

	withB := base.With("k2", "v2")
	fieldsA := cerr.Fields(withA)
	fieldsB := cerr.Fields(withB)
	if len(fieldsA) != 2 || fieldsA[0] != "k" || fieldsA[1] != "v" {
		t.Errorf("withA fields mutated: %v", fieldsA)
	}
	if len(fieldsB) != 2 || fieldsB[0] != "k2" || fieldsB[1] != "v2" {
		t.Errorf("withB fields = %v, want [k2 v2]", fieldsB)
	}
}

func TestWrap_AttachesCauseWithoutRestatingMessage(t *testing.T) {
	someErr := cerr.New("some error desc", cerr.NotFound)
	cause := errors.New("boom")

	err := someErr.Wrap(cause)

	if got, want := err.Error(), "boom: some error desc"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !errors.Is(err, someErr) {
		t.Error("errors.Is must find someErr")
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is must find cause")
	}
	if !errors.Is(err, cerr.NotFound) {
		t.Error("errors.Is must find cerr.NotFound through someErr")
	}
}

func TestFields_CollectsThroughNestedChain(t *testing.T) {
	sentinel := errors.New("sentinel")
	inner := cerr.New("inner", sentinel).With("a", 1)
	outer := cerr.New("outer", inner).With("b", 2)

	got := cerr.Fields(outer)
	want := []any{"b", 2, "a", 1}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestLogValue_GroupWithMsgAndFields(t *testing.T) {
	err := cerr.New("m").With("a", 1, "b", 2)
	attrs := err.LogValue().Group()

	if len(attrs) != 3 {
		t.Fatalf("got %d attrs, want 3", len(attrs))
	}
	if attrs[0].Key != "msg" || attrs[0].Value.String() != err.Error() {
		t.Errorf("attrs[0] = %v, want msg=%q", attrs[0], err.Error())
	}
	if attrs[1].Key != "a" || attrs[1].Value.Any() != int64(1) {
		t.Errorf("attrs[1] = %v, want a=1", attrs[1])
	}
	if attrs[2].Key != "b" || attrs[2].Value.Any() != int64(2) {
		t.Errorf("attrs[2] = %v, want b=2", attrs[2])
	}
}

func TestLogValue_IgnoresDanglingKeyAndNonStringKey(t *testing.T) {
	err := cerr.New("m").With("k")
	attrs := err.LogValue().Group()
	if len(attrs) != 1 {
		t.Fatalf("got %d attrs, want 1 (msg only)", len(attrs))
	}

	err2 := cerr.New("m").With(42, "v")
	attrs2 := err2.LogValue().Group()
	if len(attrs2) != 1 {
		t.Fatalf("got %d attrs, want 1 (msg only)", len(attrs2))
	}
}
