package business

import (
	"strconv"
	"time"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/golang-jwt/jwt/v5"
)

// Token types. Both tokens share a signing key, so this claim is the only thing
// stopping a refresh token from being accepted as an access token.
const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// signingMethod is pinned to HMAC-SHA256, and checked again at verification time,
// so a token arriving with "alg": "none" is never trusted.
var signingMethod = jwt.SigningMethodHS256

type TokenService struct {
	settings entity.TokenSettings
}

func NewTokenService(settings entity.TokenSettings) *TokenService {
	return &TokenService{
		settings: settings,
	}
}

// tokenClaims is the payload of both tokens.
type tokenClaims struct {
	TokenType string `json:"token_type"`

	jwt.RegisteredClaims
}

// Mint issues the pair handed to a client that has just proved who they are. Both
// tokens are stamped from a single `now`, so their lifetimes start together.
func (s *TokenService) Mint(sub int) (entity.TokenPair, error) {
	// An empty key still yields a well-formed HMAC signature — one anybody can
	// reproduce — so refusing to sign is the only safe answer.
	if s.settings.Secret == "" {
		return entity.TokenPair{}, entity.ErrMissingSigningKey.Loc().Time()
	}

	now := time.Now()

	accessToken, err := s.sign(sub, tokenTypeAccess, now, s.settings.AccessTTL)
	if err != nil {
		return entity.TokenPair{}, err
	}

	refreshToken, err := s.sign(sub, tokenTypeRefresh, now, s.settings.RefreshTTL)
	if err != nil {
		return entity.TokenPair{}, err
	}

	return entity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// Refresh verifies a refresh token and mints a fresh pair in its place. The
// presented token is never reissued: every refresh starts both lifetimes over.
func (s *TokenService) Refresh(refreshToken string) (entity.TokenPair, error) {
	if s.settings.Secret == "" {
		return entity.TokenPair{}, entity.ErrMissingSigningKey.Loc().Time()
	}

	sub, err := s.verify(refreshToken, tokenTypeRefresh)
	if err != nil {
		return entity.TokenPair{}, entity.ErrInvalidRefreshToken.Wrap(err).Loc().Time()
	}

	return s.Mint(sub)
}

// Authenticate verifies an access token and reports whose it is. It is the way
// in for a guarded request, the counterpart of the Mint a login ends with.
func (s *TokenService) Authenticate(accessToken string) (int, error) {
	// An empty key verifies every signature an attacker cares to compute, so the
	// guard matters more on this side than it does on the minting one.
	if s.settings.Secret == "" {
		return 0, entity.ErrMissingSigningKey.Loc().Time()
	}

	sub, err := s.verify(accessToken, tokenTypeAccess)
	if err != nil {
		return 0, entity.ErrInvalidAccessToken.Wrap(err).Loc().Time()
	}

	return sub, nil
}

// verify parses and validates a token, and checks it carries the expected type,
// so a refresh token can never be replayed as an access token or vice versa.
// Failures are described here, not classified: only the caller knows which token
// it asked about, so only the caller can name the sentinel that fits.
func (s *TokenService) verify(token string, wantType string) (int, error) {
	claims := tokenClaims{}

	parsed, err := jwt.ParseWithClaims(
		token,
		&claims,
		func(t *jwt.Token) (any, error) { return []byte(s.settings.Secret), nil },
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
	)
	if err != nil {
		return 0, cerr.New("failed to parse token", err).Loc().Time()
	}

	if !parsed.Valid {
		return 0, cerr.New("token did not validate").Loc().Time()
	}

	if claims.TokenType != wantType {
		return 0, cerr.New("unexpected token type").
			With("want", wantType, "got", claims.TokenType).
			Loc().
			Time()
	}

	sub, err := strconv.Atoi(claims.Subject)
	if err != nil {
		return 0, cerr.New("token subject is not a user id", err).Loc().Time()
	}

	return sub, nil
}

// sign builds one token and returns it serialized. issuedAt is passed in so both
// tokens of a pair agree on when the pair was minted.
func (s *TokenService) sign(
	sub int,
	tokenType string,
	issuedAt time.Time,
	timeout time.Duration,
) (string, error) {
	claims := tokenClaims{
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(sub),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(timeout)),
		},
	}

	signed, err := jwt.NewWithClaims(signingMethod, claims).
		SignedString([]byte(s.settings.Secret))
	if err != nil {
		// No key material in the fields.
		return "", entity.ErrFailedToMintToken.Wrap(err).
			With("token_type", tokenType, "sub", sub).
			Loc().
			Time()
	}

	return signed, nil
}
