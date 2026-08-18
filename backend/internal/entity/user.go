package entity

import (
	"time"

	"github.com/siamionv/finy/internal/generated/openapi"
)

type User struct {
	ID        int
	Username  string
	IconURL   *string
	CreatedAt time.Time
}

func (u User) ToOpenAPI() openapi.User {
	return openapi.User{
		Id:        u.ID,
		Username:  u.Username,
		IconUrl:   u.IconURL,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

type UserDB struct {
	ID           int
	Username     string
	PasswordHash string
	IconURL      *string
	CreatedAt    time.Time
}

func (u UserDB) IntoUser() User {
	return User{
		ID:        u.ID,
		Username:  u.Username,
		IconURL:   u.IconURL,
		CreatedAt: u.CreatedAt,
	}
}
