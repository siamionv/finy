package auth

import (
	"net/http"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"

	"github.com/labstack/echo/v4"
)

// TODO: add logging
// TODO: add handle of error statuses
// TODO: implement service layer for RegisterUSer
func (h *Handler) RegisterUser(c echo.Context) error {
	var request openapi.RegisterUserRequest
	if err := c.Bind(&request); err != nil {
		return h.handleRegisterUserInvalidPayload(c)
	}

	var credentials entity.UserCredentials
	credentials.FromOpenAPI(request)

	publicUser, err := h.userSvc.CreateUser(credentials)
	if err != nil {
		return h.handleCreateUserError(c, err)
	}

	response := publicUser.ToOpenAPI()

	return c.JSON(http.StatusCreated, response)
}

func (h *Handler) handleCreateUserError(c echo.Context, err error) error {
	panic("implement me")
}

func (h *Handler) handleRegisterUserInvalidPayload(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, openapi.EnvelopedValidationError{
		Status: openapi.Failure,
		Error:  "failed to deserialize request payload",
		Data: openapi.ValidationError{
			Errors: []openapi.ValidationErrorField{
				{
					Field:   openapi.Payload,
					Code:    openapi.PayloadInvalidJson,
					Message: "invalid json",
				},
			},
		},
	})
}
