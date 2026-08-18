package business

import (
	"strconv"
	"time"

	"github.com/siamionv/finy/internal/entity"

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
