package business_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/siamionv/finy/internal/business"
	"github.com/siamionv/finy/internal/entity"

	"golang.org/x/crypto/bcrypt"
)

type fakeUserRepo struct {
	got entity.CreateUser
	err error
}

func (f *fakeUserRepo) InsertUser(_ context.Context, dto entity.CreateUser) (*entity.User, error) {
	f.got = dto
	if f.err != nil {
		return nil, f.err
	}

	return &entity.User{ID: 1, Username: dto.Username}, nil
}

// The service is the only place that knows the plaintext, and the column it
// writes to is called password_hash. Storing the password verbatim there is the
// bug this guards.
func TestCreateUser_StoresADigestNotThePassword(t *testing.T) {
	repo := &fakeUserRepo{}
	svc := business.NewUserService(repo)

	const password = "Correct9Horse!"

	if _, err := svc.CreateUser(t.Context(), entity.UserCredentials{
		Username: "johndoe",
		Password: password,
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if repo.got.PasswordHash == password {
		t.Fatal("password reached the repository in plaintext")
	}
	if _, err := bcrypt.Cost([]byte(repo.got.PasswordHash)); err != nil {
		t.Fatalf("stored value is not a bcrypt hash: %v", err)
	}
}

// Two registrations of the same password must not produce the same stored
// value, or the column leaks which accounts share one.
func TestCreateUser_SaltsEachHash(t *testing.T) {
	hashOnce := func() string {
		t.Helper()

		repo := &fakeUserRepo{}
		if _, err := business.NewUserService(repo).CreateUser(t.Context(), entity.UserCredentials{
			Username: "johndoe",
			Password: "Correct9Horse!",
		}); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}

		return repo.got.PasswordHash
	}

	first, second := hashOnce(), hashOnce()
	if first == second {
		t.Error("identical passwords produced identical hashes")
	}
}

// bcrypt refuses inputs over 72 bytes, and the spec sets no maximum length.
// Without the sha256 pre-pass this is a 500 on a perfectly valid password.
func TestCreateUser_AcceptsAPasswordLongerThanBcryptsLimit(t *testing.T) {
	prefix := "Correct9Horse!" + strings.Repeat("a", 72)

	for _, password := range []string{prefix + "one", prefix + "two"} {
		repo := &fakeUserRepo{}
		if _, err := business.NewUserService(repo).CreateUser(t.Context(), entity.UserCredentials{
			Username: "johndoe",
			Password: password,
		}); err != nil {
			t.Fatalf("CreateUser(%d-byte password): %v", len(password), err)
		}
	}
}

func TestCreateUser_PropagatesAlreadyExists(t *testing.T) {
	repo := &fakeUserRepo{err: entity.ErrUserAlreadyExist.Loc().Time().With("username", "johndoe")}

	_, err := business.NewUserService(repo).CreateUser(t.Context(), entity.UserCredentials{
		Username: "johndoe",
		Password: "Correct9Horse!",
	})

	if !errors.Is(err, entity.ErrUserAlreadyExist) {
		t.Errorf("got %v, want ErrUserAlreadyExist", err)
	}
	if errors.Is(err, entity.ErrFailedToCreateUser) {
		t.Error("a duplicate username must not be reported as an internal failure")
	}
}

// The service's rules and docs/openapi.yaml's patterns are one contract stated
// twice. These cases are the ones where the two used to disagree.
func TestValidateCredentials(t *testing.T) {
	const validPassword = "Correct9Horse!"
	const validUsername = "johndoe"

	tests := []struct {
		name     string
		username string
		password string
		want     error
	}{
		{"valid", validUsername, validPassword, nil},
		{"username with a hyphen is spec-valid", "john-doe", validPassword, nil},
		{"username with an underscore", "john_doe", validPassword, nil},
		{"username with digits", "john99", validPassword, nil},
		{"username too short", "jo", validPassword, entity.ErrUsernameTooShort},
		{
			"multi-byte username shorter than the minimum",
			"жж",
			validPassword,
			entity.ErrUsernameTooShort,
		},
		{
			"non-latin username violates the documented pattern",
			"пётр",
			validPassword,
			entity.ErrUsernameInvalidStart,
		},
		{
			"username starting with a digit",
			"9johndoe",
			validPassword,
			entity.ErrUsernameInvalidStart,
		},
		{
			"username starting with a hyphen",
			"-johndoe",
			validPassword,
			entity.ErrUsernameInvalidStart,
		},
		{"username with a space", "john doe", validPassword, entity.ErrUsernameInvalidCharacters},
		{"password with a spec-allowed tilde", validUsername, "Correct9Horse~", nil},
		{"password with a spec-allowed quote", validUsername, `Correct9Horse"`, nil},
		{"password with a spec-allowed backslash", validUsername, `Correct9Horse\`, nil},
		{"password with a spec-allowed slash", validUsername, "Correct9Horse/", nil},
		{"password with a spec-allowed apostrophe", validUsername, "Correct9Horse'", nil},
		{"password with a spec-allowed backtick", validUsername, "Correct9Horse`", nil},
		{"password too short", validUsername, "Cor9!", entity.ErrPasswordTooShort},
		{
			"password without uppercase",
			validUsername,
			"correct9horse!",
			entity.ErrPasswordMissingUppercase,
		},
		{
			"password without lowercase",
			validUsername,
			"CORRECT9HORSE!",
			entity.ErrPasswordMissingLowercase,
		},
		{
			"password without a digit",
			validUsername,
			"CorrectHorse!",
			entity.ErrPasswordMissingDigit,
		},
		{
			"password without a special symbol",
			validUsername,
			"Correct9Horse",
			entity.ErrPasswordMissingSpecialSymbol,
		},
		{
			"password with a space",
			validUsername,
			"Correct9 Horse!",
			entity.ErrPasswordInvalidCharacters,
		},
		{"non-latin password", validUsername, "Пароль9Хорс!", entity.ErrPasswordInvalidCharacters},
	}

	svc := business.NewUserService(&fakeUserRepo{})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.ValidateCredentials(entity.UserCredentials{
				Username: tc.username,
				Password: tc.password,
			})

			if tc.want == nil {
				if err != nil {
					t.Fatalf("got %v, want nil", err)
				}

				return
			}

			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}
