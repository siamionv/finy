package auth

import (
	"errors"
	"net/http"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"

	"github.com/labstack/echo/v4"
)

// TODO: add logging
func (h *Handler) RegisterUser(c echo.Context) error {
	var request openapi.RegisterUserRequest
	if err := c.Bind(&request); err != nil {
		return h.handleRegisterUserInvalidPayload(c)
	}

	var credentials entity.UserCredentials
	credentials.FromOpenAPI(request)

	publicUser, err := h.userSvc.CreateUser(c.Request().Context(), credentials)
	if err != nil {
		return h.handleCreateUserError(c, err)
	}

	response := publicUser.ToOpenAPI()

	return c.JSON(http.StatusCreated, response)
}

var validationRules = []struct {
	err     error
	field   openapi.ValidationErrorFieldName
	code    openapi.ValidationErrorCode
	message string
}{
	{
		entity.ErrUsernameTooShort,
		openapi.Username,
		openapi.UsernameTooShort,
		"username must be at least 3 characters long",
	},
	{
		entity.ErrUsernameInvalidStart,
		openapi.Username,
		openapi.UsernameInvalidStart,
		"username must start with a latin letter",
	},
	{
		entity.ErrUsernameInvalidCharacters,
		openapi.Username,
		openapi.UsernameInvalidCharacters,
		"username must contain only latin letters, digits, '_' and '-'",
	},
	{
		entity.ErrPasswordTooShort,
		openapi.Password,
		openapi.PasswordTooShort,
		"password must be at least 8 characters long",
	},
	{
		entity.ErrPasswordMissingUppercase,
		openapi.Password,
		openapi.PasswordMissingUppercase,
		"password must include at least one uppercase letter",
	},
	{
		entity.ErrPasswordMissingLowercase,
		openapi.Password,
		openapi.PasswordMissingLowercase,
		"password must include at least one lowercase letter",
	},
	{
		entity.ErrPasswordMissingDigit,
		openapi.Password,
		openapi.PasswordMissingDigit,
		"password must include at least one digit",
	},
	{
		entity.ErrPasswordMissingSpecialSymbol,
		openapi.Password,
		openapi.PasswordMissingSpecialSymbol,
		"password must include at least one special symbol",
	},
	{
		entity.ErrPasswordInvalidCharacters,
		openapi.Password,
		openapi.PasswordInvalidCharacters,
		"password contains invalid characters",
	},
}

func (h *Handler) handleCreateUserError(c echo.Context, err error) error {
	// Critical errors are exclusive of validation errors, so they can't collide.
	if errors.Is(err, entity.ErrUserAlreadyExist) {
		return c.JSON(http.StatusConflict, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("user already exists"),
		})
	}
	if errors.Is(err, entity.ErrFailedToCreateUser) {
		return c.JSON(http.StatusInternalServerError, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to create user"),
		})
	}

	var fields []openapi.ValidationErrorField
	for _, rule := range validationRules {
		if errors.Is(err, rule.err) {
			fields = append(fields, openapi.ValidationErrorField{
				Field:   rule.field,
				Code:    rule.code,
				Message: rule.message,
			})
		}
	}

	if len(fields) == 0 {
		return c.JSON(http.StatusInternalServerError, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to create user"),
		})
	}

	return c.JSON(http.StatusBadRequest, openapi.EnvelopedValidationError{
		Status: openapi.Failure,
		Error:  "invalid registration input",
		Data: openapi.ValidationError{
			Errors: fields,
		},
	})
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
