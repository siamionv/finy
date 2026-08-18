package di

import (
	"github.com/siamionv/finy/internal/business"
	"github.com/siamionv/finy/internal/config"
	"github.com/siamionv/finy/internal/entity"
)

// Services is the use-case layer, and the only slice of the graph a transport is
// handed. Each consumer narrows these concrete types to its own interface.
type Services struct {
	Auth   *business.AuthService
	Users  *business.UserService
	Tokens *business.TokenService
}

// newServices assembles the graph. Construction is pure: no I/O, no ordering.
func newServices(config *config.Config, repos repositories) Services {
	tokens := business.NewTokenService(entity.TokenSettings{
		Secret:     config.JWT.Secret,
		AccessTTL:  config.JWT.AccessTokenTimeout,
		RefreshTTL: config.JWT.RefreshTokenTimeout,
	})

	return Services{
		Auth:   business.NewAuthService(repos.user, tokens),
		Users:  business.NewUserService(repos.user),
		Tokens: tokens,
	}
}
