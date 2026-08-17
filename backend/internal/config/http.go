package config

import "time"

type HTTP struct {
	Addr        string `env:"HTTP_ADDR"          mapstructure:"addr"          validate:"required"`
	MaxBodySize string `env:"HTTP_MAX_BODY_SIZE" mapstructure:"max_body_size" validate:"required"`

	// gte=0 without required: zero is the documented way to disable an
	// http.Server timeout, and required rejects exactly that value.
	ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" mapstructure:"read_header_timeout" validate:"gte=0"`
	ReadTimeout       time.Duration `env:"HTTP_READ_TIMEOUT"        mapstructure:"read_timeout"        validate:"gte=0"`
	WriteTimeout      time.Duration `env:"HTTP_WRITE_TIMEOUT"       mapstructure:"write_timeout"       validate:"gte=0"`
	IdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT"        mapstructure:"idle_timeout"        validate:"gte=0"`

	// DrainTimeout is how long in-flight requests get to finish after the
	// shutdown signal, before their contexts are cancelled and the remaining
	// connections are force-closed. Unlike the timeouts above, zero is not a
	// meaningful setting here — it would abort every in-flight request the
	// instant the signal arrives — so this one stays required.
	DrainTimeout time.Duration `env:"HTTP_DRAIN_TIMEOUT" mapstructure:"drain_timeout" validate:"required,gt=0"`
}
