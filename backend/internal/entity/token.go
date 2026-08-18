package entity

import (
	"time"

	"github.com/siamionv/finy/pkg/cerr"
)

var (
	ErrMissingSigningKey = cerr.New("token signing key is not configured", cerr.Internal)
	ErrFailedToMintToken = cerr.New("failed to mint token", cerr.Internal)
)

// TokenSettings is the signing key and lifetimes the token service mints against.
type TokenSettings struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// TokenPair is what a caller receives once it has proved who it is.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}
