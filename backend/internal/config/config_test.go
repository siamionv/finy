package config

import (
	"testing"
	"time"
)

func TestLoad_LocalConfig(t *testing.T) {
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	cfg, err := load("../../configs/local.yaml")
	if err != nil {
		t.Fatalf("load() error = %v, want nil", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("cfg.Validate() error = %v, want nil", err)
	}

	if want := 720 * time.Hour; cfg.JWT.RefreshTokenTimeout != want {
		t.Errorf(
			"cfg.JWT.RefreshTokenTimeout = %v, want %v",
			cfg.JWT.RefreshTokenTimeout,
			want,
		)
	}
}
