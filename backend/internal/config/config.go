package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Logger   Logger   `mapstructure:"logger"   validate:"required"`
	Database Database `mapstructure:"database" validate:"required"`
	HTTP     HTTP     `mapstructure:"http"     validate:"required"`
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	validate := validator.New()

	return validate.Struct(&c)
}

func (c Config) String() string {
	b, _ := json.Marshal(c)

	return strings.ReplaceAll(string(b), `"`, `'`)
}

func MustConfig() *Config {
	configPath := flag.String("config", "", "path to config [required]")
	flag.Parse()

	if *configPath == "" {
		panic("fail to start without config path")
	}

	// Try loading .env file from multiple locations
	envPaths := []string{
		".env",              // Current directory
		"/srv/.env",         // Docker container root
		"/srv/configs/.env", // Config directory in Docker
		filepath.Join(filepath.Dir(*configPath), ".env"), // Near config file
	}

	envLoaded := false
	for _, envPath := range envPaths {
		if err := godotenv.Load(envPath); err == nil {
			log.Printf("Successfully loaded .env file from: %s", envPath)
			envLoaded = true
			break
		}
	}

	if !envLoaded {
		log.Printf("Warning: Could not load .env file from any of the tried paths: %v", envPaths)
	}

	v := viper.New()

	configDir := setConfigName(v, *configPath)

	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		log.Printf(
			"Warning reading config file: %s, will use defaults and environment variables\n",
			err,
		)
	}

	// Убираем префикс для совместимости с существующими переменными
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnvVars(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("Unable to decode config into struct: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	return &cfg
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

func bindEnvVars(v *viper.Viper) {
	envMappings, err := createEnvMappings(reflect.TypeOf(Config{}))
	if err != nil {
		log.Fatalf("Error creating env mappings: %v", err)
	}

	for configKey, envVar := range envMappings {
		bindEnv(v, configKey, envVar)
	}
}

func createEnvMappings(reflectedType reflect.Type, args ...any) (map[string]string, error) {
	var envMappings map[string]string
	if len(args) > 0 {
		envMappings = args[0].(map[string]string)
	} else {
		envMappings = make(map[string]string)
	}

	var prefixReference string
	if len(args) > 1 {
		prefixReference = args[1].(string)
	}

	for i := 0; i < reflectedType.NumField(); i++ {
		field := reflectedType.Field(i)

		// Skip embedded fields without mapstructure tags
		if field.Anonymous {
			mapstructureTag := field.Tag.Get("mapstructure")
			if mapstructureTag == "" || mapstructureTag == "-" {
				continue
			}
		}

		// Skip sync package types
		if strings.HasPrefix(field.Type.String(), "sync.") {
			continue
		}

		if field.Type.Kind() != reflect.Struct {
			if err := mapStructField(field, envMappings, prefixReference); err != nil {
				return nil, err
			}

			continue
		}

		name := field.Tag.Get("mapstructure")
		if name == "" {
			return nil, fmt.Errorf(
				"mapstructure tag is required for struct field %s (type: %s, anonymous: %v)",
				field.Name,
				field.Type.String(),
				field.Anonymous,
			)
		}

		prefix := name
		if prefixReference != "" {
			prefix = strings.Join([]string{prefixReference, name}, ".")
		}

		if _, err := createEnvMappings(field.Type, envMappings, prefix); err != nil {
			return nil, err
		}
	}

	return envMappings, nil
}

func mapStructField(field reflect.StructField, envMappings map[string]string, prefix string) error {
	// Skip embedded fields without mapstructure tags
	if field.Anonymous {
		mapstructureTag := field.Tag.Get("mapstructure")
		if mapstructureTag == "" || mapstructureTag == "-" {
			return nil
		}
	}

	mapstructureTag := field.Tag.Get("mapstructure")
	if mapstructureTag == "" {
		return fmt.Errorf(
			"mapstructure tag is required for struct field %s (type: %s, anonymous: %v)",
			field.Name,
			field.Type.String(),
			field.Anonymous,
		)
	}

	envTag := field.Tag.Get("env")
	if envTag == "" || envTag == "-" { // "-" means that the field is not binded to an env var
		return nil
	}

	envMappings[strings.Join([]string{prefix, mapstructureTag}, ".")] = envTag

	return nil
}

func bindEnv(v *viper.Viper, configKey, envVar string) {
	if err := v.BindEnv(configKey, envVar); err != nil {
		log.Printf("Error binding env var %s: %s\n", envVar, err)
	}
}
