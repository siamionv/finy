package user

import (
	"time"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
)

// userToResponse renders a user for the wire, timestamps included.
func userToResponse(u entity.User) openapi.User {
	return openapi.User{
		Id:        u.ID,
		Username:  u.Username,
		IconUrl:   u.IconURL,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}
