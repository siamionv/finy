package entity

import (
	"time"

	"github.com/siamionv/finy/pkg/cerr"
)

var (
	ErrUserAlreadyExist   = cerr.New("user already exists", cerr.Conflict)
	ErrUserNotFound       = cerr.New("user not found", cerr.NotFound)
	ErrFailedToCreateUser = cerr.New("failed to create user", cerr.Internal)
)

// User is the account as the rest of the service sees it: never the password hash.
type User struct {
	ID        int
	Username  string
	IconURL   *string
	CreatedAt time.Time
}

// CreateUser is what the user repository needs to insert a new account.
type CreateUser struct {
	Username     string
	PasswordHash string
}

// UserDB is the stored row, hash included.
type UserDB struct {
	ID           int
	Username     string
	PasswordHash string
	IconURL      *string
	CreatedAt    time.Time
}

// IntoUser drops the hash, leaving what may travel past the business layer.
func (u UserDB) IntoUser() User {
	return User{
		ID:        u.ID,
		Username:  u.Username,
		IconURL:   u.IconURL,
		CreatedAt: u.CreatedAt,
	}
}
