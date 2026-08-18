package business_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/siamionv/finy/internal/business"
	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/pkg/cerr"

	"golang.org/x/crypto/bcrypt"
)

type fakeUserRepo struct {
	got           entity.CreateUser
	err           error
	byUsername    *entity.UserDB
	byUsernameErr error
}

func (f *fakeUserRepo) InsertUser(
	_ context.Context,
	dto entity.CreateUser,
) (*entity.UserDB, error) {
	f.got = dto
	if f.err != nil {
		return nil, f.err
	}

	return &entity.UserDB{ID: 1, Username: dto.Username}, nil
}

func (f *fakeUserRepo) GetUserByUsername(
	_ context.Context,
	username string,
) (*entity.UserDB, error) {
	if f.byUsernameErr != nil {
		return nil, f.byUsernameErr
	}

	return f.byUsername, nil
}

// fakeMinter records the subject Login asked a pair for.
type fakeMinter struct {
	gotSub int
	pair   entity.TokenPair
	err    error
}

func (f *fakeMinter) Mint(sub int) (entity.TokenPair, error) {
	f.gotSub = sub

	return f.pair, f.err
}

func (f *fakeMinter) Refresh(_ string) (entity.TokenPair, error) {
	return f.pair, f.err
}

// newAuth builds the service under test with a minter the case can inspect.
func newAuth(repo business.UserRepository) (*business.AuthService, *fakeMinter) {
	minter := &fakeMinter{pair: entity.TokenPair{AccessToken: "access", RefreshToken: "refresh"}}

	return business.NewAuthService(repo, minter), minter
}

// mustAuth is newAuth for cases that never inspect the minter.
func mustAuth(repo business.UserRepository) *business.AuthService {
	svc, _ := newAuth(repo)

	return svc
}

// The service is the only place that knows the plaintext, and the column it
// writes to is called password_hash. Storing the password verbatim there is the
// bug this guards.
func TestCreateUser_StoresADigestNotThePassword(t *testing.T) {
	repo := &fakeUserRepo{}
	svc, _ := newAuth(repo)

	const password = "Correct9Horse!"

	if _, err := svc.Register(t.Context(), entity.UserCredentials{
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
		if _, err := mustAuth(repo).
			Register(t.Context(), entity.UserCredentials{
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
		if _, err := mustAuth(repo).
			Register(t.Context(), entity.UserCredentials{
				Username: "johndoe",
				Password: password,
			}); err != nil {
			t.Fatalf("CreateUser(%d-byte password): %v", len(password), err)
		}
	}
}

func TestCreateUser_PropagatesAlreadyExists(t *testing.T) {
	repo := &fakeUserRepo{err: entity.ErrUserAlreadyExist.Loc().Time().With("username", "johndoe")}

	_, err := mustAuth(repo).Register(t.Context(), entity.UserCredentials{
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

// hashForTest reproduces the service's sha256-then-bcrypt scheme so tests can
// seed a repo with a password hash the service will actually recognize.
func hashForTest(t *testing.T, password string) string {
	t.Helper()

	digest := sha256.Sum256([]byte(password))
	encoded := base64.RawStdEncoding.EncodeToString(digest[:])

	hash, err := bcrypt.GenerateFromPassword([]byte(encoded), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hashForTest: %v", err)
	}

	return string(hash)
}

// fieldValue looks up key in a cerr.Fields key/value slice.
func fieldValue(fields []any, key string) (any, bool) {
	for i := 0; i+1 < len(fields); i += 2 {
		if k, ok := fields[i].(string); ok && k == key {
			return fields[i+1], true
		}
	}

	return nil, false
}

// Login is one call, so the id the credentials resolve to must reach the minter
// without any caller sequencing the two steps.
func TestLogin_MintsForTheMatchedUser(t *testing.T) {
	const password = "Correct9Horse!"

	repo := &fakeUserRepo{byUsername: &entity.UserDB{
		ID:           42,
		Username:     "johndoe",
		PasswordHash: hashForTest(t, password),
	}}

	svc, minter := newAuth(repo)

	pair, err := svc.Login(t.Context(), entity.UserCredentials{
		Username: "johndoe",
		Password: password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if minter.gotSub != 42 {
		t.Errorf("minted for sub %d, want 42", minter.gotSub)
	}
	if pair.AccessToken != "access" || pair.RefreshToken != "refresh" {
		t.Errorf("got %+v, want the minted pair", pair)
	}
}

// A minting failure is the caller's answer, not a partial success: no pair may
// come back beside it.
func TestLogin_PropagatesMintFailure(t *testing.T) {
	const password = "Correct9Horse!"

	repo := &fakeUserRepo{byUsername: &entity.UserDB{
		ID:           1,
		Username:     "johndoe",
		PasswordHash: hashForTest(t, password),
	}}

	minter := &fakeMinter{err: entity.ErrMissingSigningKey}

	pair, err := business.NewAuthService(repo, minter).Login(t.Context(), entity.UserCredentials{
		Username: "johndoe",
		Password: password,
	})
	if !errors.Is(err, entity.ErrMissingSigningKey) {
		t.Errorf("got %v, want ErrMissingSigningKey", err)
	}
	if pair != (entity.TokenPair{}) {
		t.Errorf("got pair %+v alongside an error, want the zero value", pair)
	}
}

// Two logins with the same password must both succeed even though hashPassword
// salts each hash differently — this is the case a naive hash-equality check
// against the stored digest gets wrong.
func TestLogin_MatchesDespiteDifferentSaltPerHash(t *testing.T) {
	const password = "Correct9Horse!"

	repo := &fakeUserRepo{byUsername: &entity.UserDB{
		ID:           1,
		Username:     "johndoe",
		PasswordHash: hashForTest(t, password),
	}}
	svc, _ := newAuth(repo)

	for i := range 2 {
		if _, err := svc.Login(t.Context(), entity.UserCredentials{
			Username: "johndoe",
			Password: password,
		}); err != nil {
			t.Fatalf("attempt %d: Login: %v", i, err)
		}
	}
}

// A wrong password must never reach the minter.
func TestLogin_WrongPassword(t *testing.T) {
	repo := &fakeUserRepo{byUsername: &entity.UserDB{
		ID:           1,
		Username:     "johndoe",
		PasswordHash: hashForTest(t, "Correct9Horse!"),
	}}

	svc, minter := newAuth(repo)

	_, err := svc.Login(t.Context(), entity.UserCredentials{
		Username: "johndoe",
		Password: "WrongPassword9!",
	})
	if !errors.Is(err, entity.ErrIncorrectPassword) {
		t.Errorf("got %v, want ErrIncorrectPassword", err)
	}
	if minter.gotSub != 0 {
		t.Errorf("minted for sub %d, want no mint at all", minter.gotSub)
	}

	fields := cerr.Fields(err)
	username, ok := fieldValue(fields, "username")
	if !ok || username != "johndoe" {
		t.Errorf("got username field %v (ok=%v), want %q", username, ok, "johndoe")
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := &fakeUserRepo{
		byUsernameErr: entity.ErrUserAlreadyExist,
	} // any repo error stands in for not-found

	_, err := mustAuth(repo).Login(t.Context(), entity.UserCredentials{
		Username: "johndoe",
		Password: "Correct9Horse!",
	})
	if !errors.Is(err, entity.ErrUserAlreadyExist) {
		t.Errorf("got %v, want it to wrap the repository's error", err)
	}
}

// Login must not enforce the registration ruleset: a user whose stored password
// predates a since-tightened policy still has to reach the repository and sign in.
func TestLogin_ReachesRepoForAWeakButPresentPassword(t *testing.T) {
	const weakPassword = "weak"

	repo := &fakeUserRepo{byUsername: &entity.UserDB{
		ID:           7,
		Username:     "jo",
		PasswordHash: hashForTest(t, weakPassword),
	}}

	svc, minter := newAuth(repo)

	if _, err := svc.Login(t.Context(), entity.UserCredentials{
		Username: "jo",
		Password: weakPassword,
	}); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if minter.gotSub != 7 {
		t.Errorf("minted for sub %d, want 7", minter.gotSub)
	}
}

func TestLogin_EmptyUsername(t *testing.T) {
	_, err := mustAuth(&fakeUserRepo{}).Login(t.Context(), entity.UserCredentials{
		Username: "",
		Password: "Correct9Horse!",
	})
	if !errors.Is(err, entity.ErrCredentialsRequired) {
		t.Errorf("got %v, want ErrCredentialsRequired", err)
	}
}

func TestLogin_EmptyPassword(t *testing.T) {
	_, err := mustAuth(&fakeUserRepo{}).Login(t.Context(), entity.UserCredentials{
		Username: "johndoe",
		Password: "",
	})
	if !errors.Is(err, entity.ErrCredentialsRequired) {
		t.Errorf("got %v, want ErrCredentialsRequired", err)
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

	svc := mustAuth(&fakeUserRepo{})

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
