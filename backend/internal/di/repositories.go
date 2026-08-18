package di

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/siamionv/finy/internal/adapter"
)

// repositories are the persistence adapters. Unexported on purpose: only
// newServices holds one, so the compiler keeps transports away from the database.
type repositories struct {
	user *adapter.UserRepository
}

func newRepositories(db *pgxpool.Pool) repositories {
	return repositories{
		user: adapter.NewUserRepository(db),
	}
}
