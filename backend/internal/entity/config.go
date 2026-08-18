package entity

import "github.com/siamionv/finy/pkg/cerr"

var (
	ErrInvalidConfig          = cerr.New("invalid configuration", cerr.Internal)
	ErrMissingMapstructureTag = cerr.New("mapstructure tag is required", cerr.Internal)
	ErrUnknownLogLevel        = cerr.New("unknown log level", cerr.Internal)
	ErrUnknownLogFormat       = cerr.New("unknown log format", cerr.Internal)
)
