package config

import "fmt"

type Database struct {
	Username       string `env:"DB_USER"         mapstructure:"username"   validate:"required"`
	Password       string `env:"DB_PASS"         mapstructure:"password"   validate:"required"`
	Host           string `env:"DB_HOST"         mapstructure:"host"       validate:"required"`
	Port           int    `env:"DB_PORT"         mapstructure:"port"       validate:"required,gte=0,lte=65536"`
	Name           string `env:"DB_NAME"         mapstructure:"name"       validate:"required"`
	MigrationsPath string `env:"MIGRATIONS_PATH" mapstructure:"migrations" validate:"required"`
}

func (d Database) PostgresDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		d.Username,
		d.Password,
		d.Host,
		d.Port,
		d.Name,
	)
}
