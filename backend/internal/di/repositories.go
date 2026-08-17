package di

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/siamionv/finy/internal/adapter"
)

// repositories are the persistence adapters. The type is deliberately
// unexported: only newServices ever holds one, so "no transport talks to the
// database directly" is enforced by the compiler rather than by review.
//
// It grows by one field per repository.
type repositories struct {
	user *adapter.UserRepository
}

func newRepositories(db *pgxpool.Pool) repositories {
	return repositories{
		user: adapter.NewUserRepository(db),
	}
}
