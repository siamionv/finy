package business

import (
	"context"
	"errors"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/pkg/cerr"
)

// UserService owns the account use cases that are about the account itself
// rather than about proving who holds it.
type UserService struct {
	userRepo UserReader
}

func NewUserService(userRepo UserReader) *UserService {
	return &UserService{userRepo: userRepo}
}

// UserReader is the slice of persistence this service calls. Narrower than
// AuthService's UserRepository on purpose: each consumer owns the shape it needs.
type UserReader interface {
	GetUserByID(ctx context.Context, id int) (*entity.UserDB, error)
}

// GetUser returns the account behind id, hash dropped.
func (s *UserService) GetUser(ctx context.Context, id int) (*entity.User, error) {
	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, entity.ErrUserNotFound) {
			return nil, err
		}

		return nil, cerr.New("failed to get user by id", err).Loc().Time().With("user_id", id)
	}

	userDTO := user.IntoUser()

	return &userDTO, nil
}
