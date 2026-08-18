package business_test

import (
	"errors"
	"testing"
	"time"

	"github.com/siamionv/finy/internal/business"
	"github.com/siamionv/finy/internal/entity"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-that-is-long-enough-32"

func testTokenSettings() entity.TokenSettings {
	return entity.TokenSettings{
		Secret:     testSecret,
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 30 * 24 * time.Hour,
	}
}

// parseWith verifies token against secret and returns its claims. Only HS256 is
// accepted: a test that let the library pick the method from the token's own
// header would still pass if Mint started signing with "none".
func parseWith(t *testing.T, token, secret string) jwt.MapClaims {
	t.Helper()

	claims := jwt.MapClaims{}

	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	return claims
}

func claimTime(t *testing.T, claims jwt.MapClaims, name string) time.Time {
	t.Helper()

	seconds, ok := claims[name].(float64)
	if !ok {
		t.Fatalf("claim %q is %T, want a numeric date", name, claims[name])
	}

	return time.Unix(int64(seconds), 0)
}

// The pair is what the client is handed after proving who they are: both halves
// have to verify under the configured key and name the user they were minted
// for, or the login is worthless.
func TestMint_SignsBothTokensForTheSubject(t *testing.T) {
	pair, err := business.NewTokenService(testTokenSettings()).Mint(42)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	for name, token := range map[string]string{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
	} {
		claims := parseWith(t, token, testSecret)

		sub, err := claims.GetSubject()
		if err != nil {
			t.Fatalf("%s: subject: %v", name, err)
		}
		if sub != "42" {
			t.Errorf("%s: sub = %q, want %q", name, sub, "42")
		}
	}
}

// The two tokens are signed with one key, so only the claim separates them. If
// they were interchangeable, the 30-day refresh token would authenticate
// requests for a month — exactly what the 15-minute access token exists to
// prevent.
func TestMint_MarksTheTwoTokensApart(t *testing.T) {
	pair, err := business.NewTokenService(testTokenSettings()).Mint(1)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	if pair.AccessToken == pair.RefreshToken {
		t.Fatal("access and refresh tokens are identical")
	}

	access := parseWith(t, pair.AccessToken, testSecret)
	refresh := parseWith(t, pair.RefreshToken, testSecret)

	if got := access["token_type"]; got != "access" {
		t.Errorf("access token_type = %v, want %q", got, "access")
	}
	if got := refresh["token_type"]; got != "refresh" {
		t.Errorf("refresh token_type = %v, want %q", got, "refresh")
	}
}

// The lifetimes are the security boundary the config is there to set, and the
// spec documents them to clients. Measured as exp-iat so the assertion does not
// race the clock.
func TestMint_HonoursConfiguredLifetimes(t *testing.T) {
	cfg := entity.TokenSettings{
		Secret:     testSecret,
		AccessTTL:  time.Minute,
		RefreshTTL: 72 * time.Hour,
	}

	pair, err := business.NewTokenService(cfg).Mint(7)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
		want  time.Duration
	}{
		{"access", pair.AccessToken, cfg.AccessTTL},
		{"refresh", pair.RefreshToken, cfg.RefreshTTL},
	} {
		claims := parseWith(t, tc.token, testSecret)

		issuedAt := claimTime(t, claims, "iat")
		expiresAt := claimTime(t, claims, "exp")

		if got := expiresAt.Sub(issuedAt); got != tc.want {
			t.Errorf("%s token lifetime = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// One `now` for the pair: if each token read the clock itself, a pair minted
// across a second boundary would disagree about when the session began.
func TestMint_StampsBothTokensAtTheSameInstant(t *testing.T) {
	before := time.Now()

	pair, err := business.NewTokenService(testTokenSettings()).Mint(3)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	after := time.Now()

	accessIssuedAt := claimTime(t, parseWith(t, pair.AccessToken, testSecret), "iat")
	refreshIssuedAt := claimTime(t, parseWith(t, pair.RefreshToken, testSecret), "iat")

	if !accessIssuedAt.Equal(refreshIssuedAt) {
		t.Errorf("iat differs: access %v, refresh %v", accessIssuedAt, refreshIssuedAt)
	}
	// Truncated to the second by the JWT numeric date format, so compare against
	// the same resolution rather than the raw bounds.
	if accessIssuedAt.Before(before.Truncate(time.Second)) || accessIssuedAt.After(after) {
		t.Errorf("iat = %v, outside [%v, %v]", accessIssuedAt, before, after)
	}
}

// The signature is only worth anything if it is bound to the configured key.
// This is the test that fails if Mint ever signs with a constant or an empty
// key, which would let anyone mint tokens for any user.
func TestMint_BindsSignatureToTheConfiguredSecret(t *testing.T) {
	pair, err := business.NewTokenService(testTokenSettings()).Mint(5)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	for name, token := range map[string]string{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
	} {
		_, err := jwt.Parse(token, func(*jwt.Token) (any, error) {
			return []byte("another-secret-that-is-long-enough"), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

		if !errors.Is(err, jwt.ErrSignatureInvalid) {
			t.Errorf("%s verified under a foreign key: err = %v", name, err)
		}
	}
}

// Different users must not receive the same token, whatever the timing.
func TestMint_SeparatesSubjects(t *testing.T) {
	svc := business.NewTokenService(testTokenSettings())

	first, err := svc.Mint(1)
	if err != nil {
		t.Fatalf("Mint(1): %v", err)
	}

	second, err := svc.Mint(2)
	if err != nil {
		t.Fatalf("Mint(2): %v", err)
	}

	if first.AccessToken == second.AccessToken {
		t.Error("two subjects received the same access token")
	}
	if first.RefreshToken == second.RefreshToken {
		t.Error("two subjects received the same refresh token")
	}
}

// HMAC over an empty key produces a signature anyone can forge, and the library
// signs one happily. Refusing beats issuing a token that verifies for everybody.
func TestMint_RefusesToSignWithoutAKey(t *testing.T) {
	cfg := testTokenSettings()
	cfg.Secret = ""

	pair, err := business.NewTokenService(cfg).Mint(1)
	if !errors.Is(err, entity.ErrMissingSigningKey) {
		t.Fatalf("err = %v, want %v", err, entity.ErrMissingSigningKey)
	}

	if pair != (entity.TokenPair{}) {
		t.Errorf("tokens issued alongside the error: %+v", pair)
	}
}
