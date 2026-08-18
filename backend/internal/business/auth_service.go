package business

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/pkg/cerr"

	"golang.org/x/crypto/bcrypt"
)

const (
	usernameMinLength = 3
	passwordMinLength = 8
)

// passwordSpecialSymbols spells out the OpenAPI password pattern's character
// class; it has to be edited together with that pattern.
const passwordSpecialSymbols = "^$*.[]{}()?\"!@#%&/\\,><':;|_~`+=-"

// AuthService owns the two account use cases: establishing an identity, and
// proving one. Both are single calls, so no transport has to sequence them.
type AuthService struct {
	userRepo UserRepository
	tokens   TokenIssuer
}

func NewAuthService(userRepo UserRepository, tokens TokenIssuer) *AuthService {
	return &AuthService{userRepo: userRepo, tokens: tokens}
}

// UserRepository is declared by the consumer and kept to the methods this
// service calls, so the dependency arrow points inwards.
type UserRepository interface {
	InsertUser(ctx context.Context, dto entity.CreateUser) (*entity.UserDB, error)
	GetUserByUsername(ctx context.Context, username string) (*entity.UserDB, error)
}

// TokenIssuer is the slice of the token service Login needs. Declared here for
// the same reason UserRepository is: the consumer owns the shape it depends on,
// even when the provider is another business object.
type TokenIssuer interface {
	Mint(sub int) (entity.TokenPair, error)
	Refresh(refreshToken string) (entity.TokenPair, error)
}

// Register validates the credentials and creates the account they describe.
func (s *AuthService) Register(
	ctx context.Context,
	creds entity.UserCredentials,
) (*entity.User, error) {
	if err := s.ValidateCredentials(creds); err != nil {
		// Never the password: the sentinel already says which rule broke.
		return nil, cerr.New("failed to validate credentials", err).
			Loc().
			Time().
			With("username", creds.Username)
	}

	passwordHash, err := hashPassword(creds.Password)
	if err != nil {
		return nil, entity.ErrFailedToCreateUser.Wrap(err)
	}

	createUserDTO := entity.CreateUser{
		Username:     creds.Username,
		PasswordHash: passwordHash,
	}

	user, err := s.userRepo.InsertUser(ctx, createUserDTO)
	if err != nil {
		if errors.Is(err, entity.ErrUserAlreadyExist) {
			return nil, err
		}

		return nil, entity.ErrFailedToCreateUser.Wrap(err)
	}

	userDTO := user.IntoUser()

	return &userDTO, nil
}

// Login verifies the credentials and mints the pair they earn. Sequencing the
// two steps is this layer's job: a transport gets one call, not two.
func (s *AuthService) Login(
	ctx context.Context,
	creds entity.UserCredentials,
) (entity.TokenPair, error) {
	userID, err := s.userIDByCreds(ctx, creds)
	if err != nil {
		return entity.TokenPair{}, err
	}

	return s.tokens.Mint(userID)
}

// Refresh exchanges a presented refresh token for a brand new pair.
func (s *AuthService) Refresh(
	_ context.Context,
	refreshToken string,
) (entity.TokenPair, error) {
	return s.tokens.Refresh(refreshToken)
}

// userIDByCreds only checks that both fields are present: gating login on the
// registration ruleset would lock out existing users the day that ruleset tightens.
func (s *AuthService) userIDByCreds(
	ctx context.Context,
	creds entity.UserCredentials,
) (int, error) {
	if creds.Username == "" || creds.Password == "" {
		return 0, entity.ErrCredentialsRequired.Loc().Time()
	}

	user, err := s.userRepo.GetUserByUsername(ctx, creds.Username)
	if err != nil {
		return 0, cerr.New("failed to get user by username", err).Loc().Time()
	}

	if err := comparePassword(user.PasswordHash, creds.Password); err != nil {
		return 0, entity.ErrIncorrectPassword.Loc().Time().With("username", creds.Username)
	}

	return user.ID, nil
}

func (s *AuthService) ValidateCredentials(creds entity.UserCredentials) error {
	var errs []error

	if err := validateUsername(creds.Username); err != nil {
		errs = append(errs, err)
	}
	if err := validatePassword(creds.Password); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return cerr.New("invalid credentials", errs...).Loc().Time()
}

// isLatinLetter is narrower than unicode.IsLetter on purpose: the spec's
// patterns are latin-only.
func isLatinLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func validateUsername(username string) error {
	var errs []error

	// Runes, not bytes: the length the client is told about is the one it counts.
	if utf8.RuneCountInString(username) < usernameMinLength {
		errs = append(errs, entity.ErrUsernameTooShort)
	}

	var invalidStart, invalidChars bool

	// i is a byte offset, so i == 0 is the first rune whatever its width.
	for i, r := range username {
		if i == 0 && !isLatinLetter(r) {
			invalidStart = true
		}

		if !isLatinLetter(r) && !isDigit(r) && r != '_' && r != '-' {
			invalidChars = true
		}
	}

	if invalidStart {
		errs = append(errs, entity.ErrUsernameInvalidStart)
	}
	if invalidChars {
		errs = append(errs, entity.ErrUsernameInvalidCharacters)
	}

	if len(errs) == 0 {
		return nil
	}

	return cerr.New("invalid username", errs...).Loc().Time()
}

func validatePassword(password string) error {
	var errs []error

	if utf8.RuneCountInString(password) < passwordMinLength {
		errs = append(errs, entity.ErrPasswordTooShort)
	}

	var hasUpper, hasLower, hasDigit, hasSpecial, hasInvalid bool

	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case isDigit(r):
			hasDigit = true
		case strings.ContainsRune(passwordSpecialSymbols, r):
			hasSpecial = true
		default:
			hasInvalid = true
		}
	}

	if hasInvalid {
		errs = append(errs, entity.ErrPasswordInvalidCharacters)
	}
	if !hasUpper {
		errs = append(errs, entity.ErrPasswordMissingUppercase)
	}
	if !hasLower {
		errs = append(errs, entity.ErrPasswordMissingLowercase)
	}
	if !hasDigit {
		errs = append(errs, entity.ErrPasswordMissingDigit)
	}
	if !hasSpecial {
		errs = append(errs, entity.ErrPasswordMissingSpecialSymbol)
	}

	if len(errs) == 0 {
		return nil
	}

	return cerr.New("invalid password", errs...).Loc().Time()
}

// hashPassword derives what goes to the database; only the digest is ever stored.
// The SHA-256 pre-pass gives bcrypt a fixed-width input, because bcrypt truncates
// at 72 bytes; base64 rather than the raw digest, because bcrypt stops at a NUL.
func hashPassword(password string) (string, error) {
	digest := sha256.Sum256([]byte(password))
	encoded := base64.RawStdEncoding.EncodeToString(digest[:])

	hash, err := bcrypt.GenerateFromPassword([]byte(encoded), bcrypt.DefaultCost)
	if err != nil {
		// No fields: everything this call saw is the password itself.
		return "", cerr.New("failed to hash password", err, cerr.Internal).
			Loc().
			Time()
	}

	return string(hash), nil
}

// comparePassword mirrors hashPassword's pre-pass so bcrypt sees the same input
// it was given at hash time.
func comparePassword(hash, password string) error {
	digest := sha256.Sum256([]byte(password))
	encoded := base64.RawStdEncoding.EncodeToString(digest[:])

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(encoded))
}
