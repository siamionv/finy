package config

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/pkg/cerr"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config is the whole of what the service is configured with.
type Config struct {
	Logger   Logger   `mapstructure:"logger"   validate:"required"`
	Database Database `mapstructure:"database" validate:"required"`
	HTTP     HTTP     `mapstructure:"http"     validate:"required"`
	JWT      JWT      `mapstructure:"jwt"      validate:"required"`
}

// Validate checks if the configuration is valid.
func (c Config) Validate() error {
	if err := validator.New().Struct(&c); err != nil {
		return entity.ErrInvalidConfig.Wrap(err).Loc().Time()
	}

	return nil
}

// String renders the config for a startup log line. Secrets tagged json:"-" stay out.
func (c Config) String() string {
	b, _ := json.Marshal(c)

	return strings.ReplaceAll(string(b), `"`, `'`)
}

// Load reads configPath and the environment, merging them into a validated Config.
func Load(configPath string) (*Config, error) {
	loadEnvFiles(configPath)

	v := viper.New()

	v.SetConfigType("yaml")
	v.AddConfigPath(setConfigName(v, configPath))

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, entity.ErrInvalidConfig.Wrap(err).Loc().Time().With("path", configPath)
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := bindEnvVars(v); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, cerr.New("unable to decode config into struct", err, cerr.Internal).Loc().Time()
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// loadEnvFiles fills the environment from the first .env found. Missing files are
// not an error: the environment may already carry everything the config needs.
func loadEnvFiles(configPath string) {
	envPaths := []string{
		".env",
		".env.local",
		"/srv/.env",
		"/srv/configs/.env",
		filepath.Join(filepath.Dir(configPath), ".env"),
		filepath.Join(filepath.Dir(configPath), ".env.local"),
	}

	for _, envPath := range envPaths {
		if err := godotenv.Load(envPath); err == nil {
			return
		}
	}
}

func setConfigName(v *viper.Viper, configPath string) string {
	configFileInfo, err := filepath.Abs(configPath)
	if err == nil && filepath.Ext(configFileInfo) != "" {
		configFileName := strings.TrimSuffix(
			filepath.Base(configFileInfo),
			filepath.Ext(configFileInfo),
		)
		v.SetConfigName(configFileName)

		return filepath.Dir(configFileInfo)
	}

	v.SetConfigName("config")

	return configPath
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.format", "json")
	v.SetDefault("logger.add_source", false)

	v.SetDefault("http.addr", ":8080")
	v.SetDefault("http.max_body_size", "1M")
	v.SetDefault("http.read_header_timeout", "5s")
	v.SetDefault("http.read_timeout", "15s")
	v.SetDefault("http.write_timeout", "15s")
	v.SetDefault("http.idle_timeout", "60s")
	v.SetDefault("http.drain_timeout", "15s")
}

// bindEnvVars binds every `env`-tagged field of Config to its variable.
func bindEnvVars(v *viper.Viper) error {
	mappings := make(map[string]string)
	if err := collectEnvMappings(reflect.TypeOf(Config{}), "", mappings); err != nil {
		return err
	}

	for configKey, envVar := range mappings {
		if err := v.BindEnv(configKey, envVar); err != nil {
			return cerr.New("failed to bind env var", err, cerr.Internal).
				Loc().
				Time().
				With("env", envVar, "key", configKey)
		}
	}

	return nil
}

// collectEnvMappings walks reflectedType and fills out with "config.key" -> "ENV_VAR",
// descending into nested structs under their mapstructure prefix.
func collectEnvMappings(reflectedType reflect.Type, prefix string, out map[string]string) error {
	for i := range reflectedType.NumField() {
		field := reflectedType.Field(i)

		if skipField(field) {
			continue
		}

		name := field.Tag.Get("mapstructure")
		if name == "" {
			return entity.ErrMissingMapstructureTag.
				Loc().
				Time().
				With("field", field.Name, "type", field.Type.String())
		}

		key := name
		if prefix != "" {
			key = strings.Join([]string{prefix, name}, ".")
		}

		if field.Type.Kind() == reflect.Struct {
			if err := collectEnvMappings(field.Type, key, out); err != nil {
				return err
			}

			continue
		}

		// "-" opts a field out of env binding entirely.
		if envTag := field.Tag.Get("env"); envTag != "" && envTag != "-" {
			out[key] = envTag
		}
	}

	return nil
}

// skipField reports whether a field carries no config value of its own: an
// untagged embedded struct, or a sync primitive.
func skipField(field reflect.StructField) bool {
	if field.Anonymous {
		tag := field.Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			return true
		}
	}

	return strings.HasPrefix(field.Type.String(), "sync.")
}
