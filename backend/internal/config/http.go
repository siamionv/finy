package config

import "time"

type HTTP struct {
	Addr        string `env:"HTTP_ADDR"          mapstructure:"addr"          validate:"required"`
	MaxBodySize string `env:"HTTP_MAX_BODY_SIZE" mapstructure:"max_body_size" validate:"required"`

	ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" mapstructure:"read_header_timeout" validate:"required,gte=0"`
	ReadTimeout       time.Duration `env:"HTTP_READ_TIMEOUT"        mapstructure:"read_timeout"        validate:"required,gte=0"`
	WriteTimeout      time.Duration `env:"HTTP_WRITE_TIMEOUT"       mapstructure:"write_timeout"       validate:"required,gte=0"`
	IdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT"        mapstructure:"idle_timeout"        validate:"required,gte=0"`

	// DrainTimeout is how long in-flight requests get to finish after the
	// shutdown signal, before their contexts are cancelled and the remaining
	// connections are force-closed.
	DrainTimeout time.Duration `env:"HTTP_DRAIN_TIMEOUT" mapstructure:"drain_timeout" validate:"required,gte=0"`
}
