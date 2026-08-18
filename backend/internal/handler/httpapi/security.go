package httpapi

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"

	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/pkg/cerr"
)

// guardedOperations keys guard by the id of every operation the spec requires
// credentials for, ready for RegisterHandlersWithOptions. Marking an endpoint
// `security: [jwt-auth: []]` in openapi.yaml is the whole of protecting it: the
// spec stays the one place that says which endpoints are public.
func guardedOperations(guard echo.MiddlewareFunc) (map[string][]echo.MiddlewareFunc, error) {
	spec, err := openapi.GetSpec()
	if err != nil {
		return nil, cerr.New("failed to load the embedded openapi spec", err, cerr.Internal).
			Loc().
			Time()
	}

	ids := securedOperationIDs(spec)

	guarded := make(map[string][]echo.MiddlewareFunc, len(ids))
	for _, id := range ids {
		guarded[id] = []echo.MiddlewareFunc{guard}
	}

	return guarded, nil
}

// securedOperationIDs returns the id of every operation that demands credentials.
// Split from the loader above so it can be driven by a spec built in a test.
func securedOperationIDs(spec *openapi3.T) []string {
	var ids []string

	for _, item := range spec.Paths.Map() {
		for _, operation := range item.Operations() {
			// Nothing to key the guard by, and an empty key would collide with
			// the next such operation.
			if operation.OperationID == "" {
				continue
			}

			// An operation-level `security` wins outright, `security: []`
			// included: that empty list is how one endpoint opts back out of a
			// spec-wide requirement.
			requirements := spec.Security
			if operation.Security != nil {
				requirements = *operation.Security
			}

			if len(requirements) > 0 {
				ids = append(ids, operation.OperationID)
			}
		}
	}

	return ids
}
