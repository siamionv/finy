package entity

import "github.com/siamionv/finy/pkg/cerr"

// Credential rules, one sentinel per rule so a transport can name the rule that broke.
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

	ErrIncorrectPassword   = cerr.New("password is incorrect", cerr.Invalid)
	ErrCredentialsRequired = cerr.New("credentials are required", cerr.Invalid)
)

// UserCredentials is what a caller proves an identity with.
type UserCredentials struct {
	Username string
	Password string
}
