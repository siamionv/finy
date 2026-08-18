package config

import "time"

type JWT struct {
	// Secret is the HMAC key both tokens are signed with. json:"-" keeps it out
	// of Config.String, which is logged at startup: a signing key in a log line
	// is a signing key an attacker can mint tokens with.
	Secret              string        `env:"JWT_SECRET"                     json:"-"        mapstructure:"secret" validate:"required,min=32"`
	AccessTokenTimeout  time.Duration `mapstructure:"access_token_timeout"  validate:"gt=0"`
	RefreshTokenTimeout time.Duration `mapstructure:"refresh_token_timeout" validate:"gt=0"`
}
