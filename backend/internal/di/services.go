package di

import (
	"github.com/siamionv/finy/internal/business"
)

// Services is the use-case layer, and the only slice of the graph a transport
// is handed. Fields are concrete types; each consumer narrows them to its own
// interface at the wiring site, so adding a method here never widens what a
// handler group is able to call.
//
// It grows by one field per service.
type Services struct {
	User *business.UserService
}

// newServices is where the graph is actually assembled. Construction is pure —
// no I/O, no ordering constraints — so a new service is one field and one line.
func newServices(repos repositories) Services {
	return Services{
		User: business.NewUserService(repos.user),
	}
}
