package di

import (
	"github.com/siamionv/finy/internal/business"
	"github.com/siamionv/finy/internal/config"
	"github.com/siamionv/finy/internal/entity"
)

// Services is the use-case layer, and the only slice of the graph a transport is
// handed. Each consumer narrows these concrete types to its own interface.
type Services struct {
	User  *business.UserService
	Token *business.TokenService
}

// newServices assembles the graph. Construction is pure: no I/O, no ordering.
func newServices(config *config.Config, repos repositories) Services {
	return Services{
		User: business.NewUserService(repos.user),
		Token: business.NewTokenService(entity.TokenSettings{
			Secret:     config.JWT.Secret,
			AccessTTL:  config.JWT.AccessTokenTimeout,
			RefreshTTL: config.JWT.RefreshTokenTimeout,
		}),
	}
}
