package httpapi

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"

	"github.com/siamionv/finy/internal/generated/openapi"
)

// operation builds a spec operation with the given security stance: nil for
// "says nothing", a non-nil empty slice for an explicit opt-out.
func operation(id string, security *openapi3.SecurityRequirements) *openapi3.Operation {
	return &openapi3.Operation{OperationID: id, Security: security}
}

func requirements(names ...string) *openapi3.SecurityRequirements {
	reqs := make(openapi3.SecurityRequirements, 0, len(names))
	for _, name := range names {
		reqs = append(reqs, openapi3.SecurityRequirement{name: []string{}})
	}

	return &reqs
}

// The spec is the one place that says which endpoints are public, so every
// stance it can express has to be read the way the OpenAPI standard reads it.
func TestSecuredOperationIDs_ReadsEachSecurityStance(t *testing.T) {
	tests := []struct {
		name string
		root openapi3.SecurityRequirements
		ops  map[string]*openapi3.Operation
		want []string
	}{
		{
			name: "an operation that asks for credentials is guarded",
			ops: map[string]*openapi3.Operation{
				"/guarded": operation("guarded-op", requirements("jwt-auth")),
			},
			want: []string{"guarded-op"},
		},
		{
			name: "an operation that says nothing stays public",
			ops: map[string]*openapi3.Operation{
				"/public": operation("public-op", nil),
			},
			want: nil,
		},
		{
			name: "a spec-wide requirement reaches an operation that says nothing",
			root: *requirements("jwt-auth"),
			ops: map[string]*openapi3.Operation{
				"/inherits": operation("inheriting-op", nil),
			},
			want: []string{"inheriting-op"},
		},
		{
			name: "an empty security list opts back out of the spec-wide one",
			root: *requirements("jwt-auth"),
			ops: map[string]*openapi3.Operation{
				"/opted-out": operation("opted-out-op", requirements()),
			},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			paths := openapi3.NewPaths()
			for path, op := range tc.ops {
				paths.Set(path, &openapi3.PathItem{Get: op})
			}

			got := securedOperationIDs(&openapi3.T{Security: tc.root, Paths: paths})
			slices.Sort(got)

			if !slices.Equal(got, tc.want) {
				t.Errorf("securedOperationIDs() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Run against the spec the binary actually embeds, because the way to get a
// token must never itself need one: guarding login locks every client out with
// no way back in.
func TestGuardedOperations_LeavesTheWayInOpen(t *testing.T) {
	noop := func(next echo.HandlerFunc) echo.HandlerFunc { return next }

	guarded, err := guardedOperations(noop)
	if err != nil {
		t.Fatalf("guardedOperations: %v", err)
	}

	for _, id := range []string{"register-user", "login-user", "refresh-tokens"} {
		if _, found := guarded[id]; found {
			t.Errorf("%q is guarded, but it is how a client obtains a token", id)
		}
	}
}

// The other half of the stance above: an operation that says `security` in the
// spec must come back guarded, or it serves unauthenticated with nothing to say so.
func TestGuardedOperations_GuardsWhatTheSpecMarks(t *testing.T) {
	noop := func(next echo.HandlerFunc) echo.HandlerFunc { return next }

	guarded, err := guardedOperations(noop)
	if err != nil {
		t.Fatalf("guardedOperations: %v", err)
	}

	if _, found := guarded["get-current-user"]; !found {
		t.Error("get-current-user is not guarded, but the spec marks it jwt-auth")
	}
}

// The bug this guards against is silent and total: codegen normalizes
// operationIds ("login-user" into "LoginUser") and by default writes that back
// into the embedded spec, while keying OperationMiddlewares by the raw id. The
// two key spaces then never meet, guardedOperations produces keys registration
// never looks up, and every guarded route serves unauthenticated with nothing
// in the logs to say so. Prove the spec's ids are the ids registration uses by
// guarding every operation in the spec and watching each one intercept.
func TestGuardedOperations_KeysMatchWhatRegistrationLooksUp(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}

	intercepted := map[string]bool{}
	middlewares := map[string][]echo.MiddlewareFunc{}

	type route struct{ method, path string }

	var routes []route

	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			routes = append(routes, route{method: method, path: path})

			middlewares[op.OperationID] = []echo.MiddlewareFunc{
				func(_ echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						intercepted[op.OperationID] = true

						return c.NoContent(http.StatusOK)
					}
				},
			}
		}
	}

	e := echo.New()
	openapi.RegisterHandlersWithOptions(e, nil, openapi.RegisterHandlersOptions{
		OperationMiddlewares: middlewares,
	})

	for _, r := range routes {
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(r.method, r.path, nil))
	}

	for id := range middlewares {
		if !intercepted[id] {
			t.Errorf("%q is an operationId the spec knows but registration does not", id)
		}
	}
}

// An operation with no id cannot be keyed, and every one of them would key the
// same empty string.
func TestSecuredOperationIDs_SkipsAnOperationWithoutAnID(t *testing.T) {
	paths := openapi3.NewPaths()
	paths.Set("/anonymous", &openapi3.PathItem{Get: operation("", requirements("jwt-auth"))})

	if got := securedOperationIDs(&openapi3.T{Paths: paths}); len(got) != 0 {
		t.Errorf("securedOperationIDs() = %v, want nothing keyable", got)
	}
}
