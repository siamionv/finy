package business

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/pkg/cerr"
)

const (
	usernameMinLength = 3
	passwordMinLength = 8
)

const passwordSpecialSymbols = "!@#$%^&*()_+-=[]{}|;:,.<>?"

type UserService struct {
	userRepo UserRepository
}

func NewUserService(userRepo UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

// UserRepository is declared here, by the consumer, and kept to the methods
// this service actually calls. The adapter satisfies it implicitly, so the
// dependency arrow points inwards and tests can substitute a fake.
type UserRepository interface {
	InsertUser(ctx context.Context, dto entity.CreateUser) (*entity.User, error)
}

func (s *UserService) CreateUser(
	ctx context.Context,
	creds entity.UserCredentials,
) (*entity.User, error) {
	if err := s.ValidateCredentials(creds); err != nil {
		// No Loc/Time here: the rule that failed already stamped both, and this
		// layer only adds the subject they could not see. Never the password —
		// the sentinel says which rule broke, which is all a log may know.
		return nil, cerr.New("failed to validate credentials", err).
			With("username", creds.Username)
	}

	createUserDTO := entity.CreateUser{
		Username:     creds.Username,
		PasswordHash: creds.Password,
	}

	user, err := s.userRepo.InsertUser(ctx, createUserDTO)
	if err != nil {
		if errors.Is(err, entity.ErrUserAlreadyExist) {
			return nil, err
		}

		return nil, entity.ErrFailedToCreateUser.Wrap(err)
	}

	return user, nil
}

func (s *UserService) ValidateCredentials(creds entity.UserCredentials) error {
	if err := validateUsername(creds.Username); err != nil {
		return err
	}

	if err := validatePassword(creds.Password); err != nil {
		return err
	}

	return nil
}

func validateUsername(username string) error {
	if len(username) < usernameMinLength {
		return entity.ErrUsernameTooShort.Loc().Time()
	}

	first := rune(username[0])
	if !unicode.IsLetter(first) {
		return entity.ErrUsernameInvalidStart.Loc().Time()
	}

	for _, r := range username {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return entity.ErrUsernameInvalidCharacters.Loc().Time()
		}
	}

	return nil
}

func validatePassword(password string) error {
	if len(password) < passwordMinLength {
		return entity.ErrPasswordTooShort.Loc().Time()
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case strings.ContainsRune(passwordSpecialSymbols, r):
			hasSpecial = true
		default:
			return entity.ErrPasswordInvalidCharacters.Loc().Time()
		}
	}

	if !hasUpper {
		return entity.ErrPasswordMissingUppercase.Loc().Time()
	}

	if !hasLower {
		return entity.ErrPasswordMissingLowercase.Loc().Time()
	}

	if !hasDigit {
		return entity.ErrPasswordMissingDigit.Loc().Time()
	}

	if !hasSpecial {
		return entity.ErrPasswordMissingSpecialSymbol.Loc().Time()
	}

	return nil
}
