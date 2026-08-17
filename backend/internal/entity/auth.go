package entity

import (
	"github.com/siamionv/finy/internal/generated/openapi"
	"github.com/siamionv/finy/pkg/cerr"
)

var (
	ErrUsernameTooShort             = cerr.New("username is too short", cerr.Invalid)
	ErrUsernameInvalidStart         = cerr.New("username start is invalid", cerr.Invalid)
	ErrUsernameInvalidCharacters    = cerr.New("username has invalid characters", cerr.Invalid)
	ErrPasswordTooShort             = cerr.New("password is too short", cerr.Invalid)
	ErrPasswordMissingUppercase     = cerr.New("password is missing uppercase", cerr.Invalid)
	ErrPasswordMissingLowercase     = cerr.New("password is missing lowercase", cerr.Invalid)
	ErrPasswordMissingDigit         = cerr.New("password is missing digit", cerr.Invalid)
	ErrPasswordMissingSpecialSymbol = cerr.New("password is missing special digit", cerr.Invalid)
	ErrPasswordInvalidCharacters    = cerr.New("password has invalid characters", cerr.Invalid)
	ErrUserAlreadyExist             = cerr.New("user already exists", cerr.Conflict)

	ErrIncorrectPassword = cerr.New("password is incorrect", cerr.Invalid)

	ErrFailedToCreateUser = cerr.New("failed to create user")
	ErrUserNotFound       = cerr.New("user not found", cerr.NotFound)

	ErrMissingSigningKey = cerr.New("token signing key is not configured", cerr.Internal)
	ErrFailedToMintToken = cerr.New("failed to mint token", cerr.Internal)
)

type UserCredentials struct {
	Username string
	Password string
}

type CreateUser struct {
	Username     string
	PasswordHash string
}

func (c *UserCredentials) FromOpenAPI(req openapi.UserCredentialsRequest) {
	c.Username = req.Username
	c.Password = req.Password
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

func (t TokenPair) ToOpenAPI() openapi.TokenPair {
	return openapi.TokenPair{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
	}
}
