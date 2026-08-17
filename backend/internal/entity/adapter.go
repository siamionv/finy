package entity

import "github.com/siamionv/finy/pkg/cerr"

var ErrFailedToBuildQuery = cerr.New("failed to build query", cerr.Invalid)
