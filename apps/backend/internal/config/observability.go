package config

import (
	"fmt"
	"slices"
	"time"
)

// todo: use lgtm for observability
type ObservabilityConfig struct {
	ServiceName string            `koanf:"service_name" validate:"required"`
	Environment string            `koanf:"environment" validate:"required"`
	Logging     LoggingConfig     `koanf:"logging" validate:"required"`
	NewRelic    NewRelicConfig    `koanf:"new_relic" validate:"required"`
	HealthCheck HealthCheckConfig `koanf:"health_check" validate:"required"`
}

type LoggingConfig struct {
	Level              string        `koanf:"level" validate:"required"`
	Format             string        `koanf:"format" validate:"required"`
	SlowQueryThreshold time.Duration `koanf:"slow_query_threshold"`
}

type NewRelicConfig struct {
	LicenseKey                string `koanf:"license_key" validate:"required"`
	AppLogForwardingEnabled   bool   `koanf:"app_log_forwarding_enabled"`
	DistributedTracingEnabled bool   `koanf:"distributed_tracing_enabled"`
	DebugLogging              bool   `koanf:"debug_logging"`
}

type HealthCheckConfig struct {
	Enabled  bool          `koanf:"enabled"`
	Interval time.Duration `koanf:"interval" validate:"min=1s"`
	Timeout  time.Duration `koanf:"timeout" validate:"min=1s"`
	Checks   []string      `koanf:"checks"`
}

func DefaultObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		ServiceName: "tasker",
		Environment: "dev",
		Logging: LoggingConfig{
			Level:              "info",
			Format:             "json",
			SlowQueryThreshold: 100 * time.Millisecond,
		},
		NewRelic: NewRelicConfig{
			LicenseKey:                "",
			AppLogForwardingEnabled:   true,
			DistributedTracingEnabled: true,
			DebugLogging:              false, // Disabled by default to avoid mixed log formats
		},
		HealthCheck: HealthCheckConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
			Timeout:  5 * time.Second,
			Checks:   []string{"database", "redis"},
		},
	}
}

func (oc *ObservabilityConfig) Validate() error {
	if oc.ServiceName == "" {
		return fmt.Errorf("service_name is required")
	}

	validLevels := []string{"debug", "info", "warn", "error"}
	if !slices.Contains(validLevels, oc.Logging.Level) {
		return fmt.Errorf("invalid logging level: %s (must be one of: debug, info, warn, error)", oc.Logging.Level)
	}

	if oc.Logging.SlowQueryThreshold < 0 {
		return fmt.Errorf("logging slow_query_threshold must be non-negative")
	}

	return nil
}

func (oc *ObservabilityConfig) GetLogLevel() string {
	if oc.Logging.Level == "" {
		switch oc.Environment {
		case "prod":
			return "info"
		case "dev":
			return "debug"
		}
	}

	return oc.Logging.Level
}

func (oc *ObservabilityConfig) IsProduction() bool {
	return oc.Environment == "prod"
}
