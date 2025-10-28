package config

import (
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	_ "github.com/joho/godotenv/autoload"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog"
)

type Config struct {
	Primary       Primary              `koanf:"primary" validate:"required"`
	Server        ServerConfig         `koanf:"server" validate:"required"`
	Database      DatabaseConfig       `koanf:"database" validate:"required"`
	Redis         RedisConfig          `koanf:"redis" validate:"required"`
	Auth          AuthConfig           `koanf:"auth" validate:"required"`
	Email         EmailConfig          `koanf:"email" validate:"required"`
	AWS           AWSConfig            `koanf:"aws" validate:"required"`
	Cron          *CronConfig          `koanf:"cron"`
	Observability *ObservabilityConfig `koanf:"observability"`
}

type Primary struct {
	Env string `koanf:"env" validate:"required"`
}

type ServerConfig struct {
	Port               int      `koanf:"port" validate:"required"`
	ReadTimeout        int      `koanf:"read_timeout" validate:"required"`
	WriteTimeout       int      `koanf:"write_timeout" validate:"required"`
	IdleTimeout        int      `koanf:"idle_timeout" validate:"required"`
	CorsAllowedOrigins []string `koanf:"cors_allowed_origins" validate:"required"`
}

type DatabaseConfig struct {
	Host            string `koanf:"host" validate:"required"`
	Port            int    `koanf:"port" validate:"required"`
	User            string `koanf:"user" validate:"required"`
	Password        string `koanf:"password" validate:"required"`
	Name            string `koanf:"name" validate:"required"`
	SSLMode         string `koanf:"ssl_mode" validate:"required"`
	MaxOpenConns    int    `koanf:"max_open_conns" validate:"required"`
	MaxIdleConns    int    `koanf:"max_idle_conns" validate:"required"`
	ConnMaxLifetime int    `koanf:"conn_max_lifetime" validate:"required"`
	ConnMaxIdleTime int    `koanf:"conn_max_idle_time" validate:"required"`
}

type RedisConfig struct {
	Address string `koanf:"address" validate:"required"`
}

// todo: use keycloak for auth
type AuthConfig struct {
	SecretKey string `koanf:"secret_key" validate:"required"`
}

type EmailConfig struct {
	ResendAPIKey string `koanf:"resend_api_key" validate:"required"`
}

type AWSConfig struct {
	Region          string `koanf:"region" validate:"required"`
	AccessKeyID     string `koanf:"access_key_id" validate:"required"`
	SecretAccessKey string `koanf:"secret_access_key" validate:"required"`
	UploadBucket    string `koanf:"upload_bucket" validate:"required"`
	EndpointURL     string `koanf:"endpoint_url"`
}

type CronConfig struct {
	ArchiveDaysThreshold        int `koanf:"archive_days_threshold"`
	BatchSize                   int `koanf:"batch_size"`
	ReminderHours               int `koanf:"reminder_hours"`
	MaxTodosPerUserNotification int `koanf:"max_todos_per_user_notification"`
}

func DefaultCronConfig() *CronConfig {
	return &CronConfig{
		ArchiveDaysThreshold:        30,
		BatchSize:                   100,
		ReminderHours:               24,
		MaxTodosPerUserNotification: 10,
	}
}

func LoadConfig() (*Config, error) {
	errLogger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	k := koanf.New(".")
	provider := env.Provider("TASKER_", ".", func(s string) string {
		return strings.ToLower(strings.TrimPrefix(s, "TASKER_"))
	})

	err := k.Load(provider, nil)
	if err != nil {
		errLogger.Fatal().Err(err).Msg("could not load initial env variables")
	}

	mainConfig := &Config{}
	err = k.Unmarshal("", mainConfig)
	if err != nil {
		errLogger.Fatal().Err(err).Msg("could not unmarshal main config")
	}

	// DEBUG: Print loaded config
	errLogger.Info().
		Str("db_host", mainConfig.Database.Host).
		Int("db_port", mainConfig.Database.Port).
		Str("db_user", mainConfig.Database.User).
		Str("db_name", mainConfig.Database.Name).
		Str("db_ssl", mainConfig.Database.SSLMode).
		Str("redis", mainConfig.Redis.Address).
		Msg("DEBUG: Loaded config values")

	validate := validator.New()
	err = validate.Struct(mainConfig)
	if err != nil {
		errLogger.Fatal().Err(err).Msg("could not validate main config")
	}

	if mainConfig.Observability == nil {
		mainConfig.Observability = DefaultObservabilityConfig()
	}
	mainConfig.Observability.ServiceName = "tasker"
	mainConfig.Observability.Environment = mainConfig.Primary.Env

	if err := mainConfig.Observability.Validate(); err != nil {
		errLogger.Fatal().Err(err).Msg("invalid observability config")
	}

	// Set default cron config if not provided
	if mainConfig.Cron == nil {
		mainConfig.Cron = DefaultCronConfig()
	}

	return mainConfig, nil
}
