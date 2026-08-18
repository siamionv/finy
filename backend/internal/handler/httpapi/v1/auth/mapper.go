package auth

import (
	"time"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/internal/generated/openapi"
)

// credentialsFromRequest adapts the wire payload into the business model.
func credentialsFromRequest(req openapi.UserCredentialsRequest) entity.UserCredentials {
	return entity.UserCredentials{
		Username: req.Username,
		Password: req.Password,
	}
}

// userToResponse renders a user for the wire, timestamps included.
func userToResponse(u entity.User) openapi.User {
	return openapi.User{
		Id:        u.ID,
		Username:  u.Username,
		IconUrl:   u.IconURL,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

// tokenPairToResponse renders a minted token pair for the wire.
func tokenPairToResponse(p entity.TokenPair) openapi.TokenPair {
	return openapi.TokenPair{
		AccessToken:  p.AccessToken,
		RefreshToken: p.RefreshToken,
	}
}
