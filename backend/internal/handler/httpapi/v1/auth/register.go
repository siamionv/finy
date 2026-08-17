package auth

import (
	"errors"
	"net/http"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/labstack/echo/v4"
)

func (h *Handler) RegisterUser(c echo.Context) error {
	var request openapi.RegisterUserRequest
	if err := c.Bind(&request); err != nil {
		return h.handleRegisterUserInvalidPayload(c, err)
	}

	var credentials entity.UserCredentials
	credentials.FromOpenAPI(request)

	user, err := h.userSvc.CreateUser(c.Request().Context(), credentials)
	if err != nil {
		return h.handleCreateUserError(c, err)
	}

	response := user.ToOpenAPI()

	// Enveloped like every other response from this endpoint: a client that
	// switches on `status` must not have to special-case the success path.
	return c.JSON(http.StatusCreated, openapi.EnvelopedUser{
		Status: openapi.Success,
		Data:   &response,
	})
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
		return fail(c, http.StatusConflict, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("user already exists"),
		}, err)
	}
	if errors.Is(err, entity.ErrFailedToCreateUser) {
		return fail(c, http.StatusInternalServerError, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to create user"),
		}, err)
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
		// The error matched no sentinel this handler knows, so the client gets
		// the same opaque 500 as a real failure. Naming that in the chain is
		// what separates "the database was down" from "a new sentinel reached
		// the transport and nobody taught it how to answer".
		return fail(c, http.StatusInternalServerError, openapi.Envelope{
			Status: openapi.Failure,
			Error:  new("failed to create user"),
		}, cerr.New("unclassified error from CreateUser", err, cerr.Internal))
	}

	return fail(c, http.StatusBadRequest, openapi.EnvelopedValidationError{
		Status: openapi.Failure,
		Error:  "invalid registration input",
		Data: openapi.ValidationError{
			Errors: fields,
		},
	}, err)
}

func (h *Handler) handleRegisterUserInvalidPayload(c echo.Context, cause error) error {
	// Root error: c.Bind failed before any layer of ours ran, so this is the
	// only place that can say where and on what.
	err := cerr.New("failed to bind register user request", cause, cerr.Invalid).
		Loc().
		Time().
		With("content_type", c.Request().Header.Get(echo.HeaderContentType))

	return fail(c, http.StatusBadRequest, openapi.EnvelopedValidationError{
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
	}, err)
}
