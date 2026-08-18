package business

import (
	"strconv"
	"time"

	"github.com/siamionv/finy/internal/config"
	"github.com/siamionv/finy/internal/entity"

	"github.com/golang-jwt/jwt/v5"
)

// Token types. Both tokens are signed with the same key, so the claim is what
// keeps them apart: without it a refresh token — the long-lived one — would be
// a valid access token for the whole of its lifetime.
const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// signingMethod is HMAC-SHA256 because the only party that verifies these
// tokens is the party that mints them. Pinned here, and checked again at
// verification time, so a token arriving with "alg": "none" is never trusted.
var signingMethod = jwt.SigningMethodHS256

type TokenService struct {
	config config.JWT
}

func NewTokenService(config config.JWT) *TokenService {
	return &TokenService{
		config: config,
	}
}

// tokenClaims is the payload of both tokens: the registered claims carry
// identity and lifetime, TokenType carries which of the two this is.
type tokenClaims struct {
	TokenType string `json:"token_type"`

	jwt.RegisteredClaims
}

// Mint issues the pair handed to a client that has just proved who they are.
// sub is the user id the tokens speak for; both are stamped from a single
// `now`, so the pair's lifetimes start together whatever the clock does
// between the two signatures.
func (s *TokenService) Mint(sub int) (entity.TokenPair, error) {
	// An empty key still produces a perfectly well-formed HMAC signature — one
	// anybody can reproduce. Config validation already requires the key, so
	// reaching here means a service constructed around that check; refusing to
	// sign is the only safe answer.
	if s.config.Secret == "" {
		return entity.TokenPair{}, entity.ErrMissingSigningKey.Loc().Time()
	}

	now := time.Now()

	accessToken, err := s.sign(sub, tokenTypeAccess, now, s.config.AccessTokenTimeout)
	if err != nil {
		return entity.TokenPair{}, err
	}

	refreshToken, err := s.sign(sub, tokenTypeRefresh, now, s.config.RefreshTokenTimeout)
	if err != nil {
		return entity.TokenPair{}, err
	}

	return entity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// sign builds one token and returns it serialized. issuedAt is passed in rather
// than read here so both tokens of a pair agree on when the pair was minted.
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
		SignedString([]byte(s.config.Secret))
	if err != nil {
		// No key material in the fields: token_type and subject are what a log
		// needs to tell which of the two signatures failed and for whom.
		return "", entity.ErrFailedToMintToken.Wrap(err).
			With("token_type", tokenType, "sub", sub).
			Loc().
			Time()
	}

	return signed, nil
}
