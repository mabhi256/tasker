package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5/tracelog"
	"github.com/mabhi256/go-boilerplate-echo-pgx-newrelic/internal/config"
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/zerologWriter"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

type LoggerService struct {
	nrApp *newrelic.Application
}

func NewLoggerService(cfg *config.ObservabilityConfig) *LoggerService {
	service := &LoggerService{}

	if cfg.NewRelic.LicenseKey == "" {
		fmt.Println("New Relic license key not provided, skipping initialization")
	}

	configOptions := []newrelic.ConfigOption{
		newrelic.ConfigAppName(cfg.ServiceName),
		newrelic.ConfigLicense(cfg.NewRelic.LicenseKey),
		newrelic.ConfigAppLogForwardingEnabled(cfg.NewRelic.AppLogForwardingEnabled),
		newrelic.ConfigDistributedTracerEnabled(cfg.NewRelic.DistributedTracingEnabled),
	}

	if cfg.NewRelic.DebugLogging {
		configOptions = append(configOptions, newrelic.ConfigDebugLogger(os.Stdout))
	}

	app, err := newrelic.NewApplication(configOptions...)
	if err != nil {
		fmt.Printf("Failed to initialize New Relic: %v\n", err)
		return service
	}

	service.nrApp = app
	fmt.Printf("New Relic initialized for app: %s\n", cfg.ServiceName)
	return service
}

func (ls *LoggerService) Shutdown() {
	if ls.nrApp != nil {
		ls.nrApp.Shutdown(10 * time.Second)
	}
}

func (ls *LoggerService) GetApplication() *newrelic.Application {
	return ls.nrApp
}

func NewLogger(level string, isProd bool) zerolog.Logger {
	env := "dev"
	if isProd {
		env = "prod"
	}

	config := &config.ObservabilityConfig{
		Logging: config.LoggingConfig{
			Level: level,
		},
		Environment: env,
	}

	return NewLoggerWithService(config, nil)
}

func NewLoggerWithService(cfg *config.ObservabilityConfig, loggerService *LoggerService) zerolog.Logger {
	var logLevel zerolog.Level
	switch cfg.GetLogLevel() {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	default:
		logLevel = zerolog.InfoLevel
	}

	zerolog.TimeFieldFormat = "1000-01-01 10:00:00"
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	var writer io.Writer
	var baseWriter io.Writer
	if cfg.IsProduction() && cfg.Logging.Format == "json" {
		// In production, write to stdout
		baseWriter = os.Stdout

		// Wrap with New Relic zerologWriter for log forwarding in production
		if loggerService != nil && loggerService.nrApp != nil {
			writer = zerologWriter.New(baseWriter, loggerService.nrApp)
		} else {
			writer = baseWriter
		}
	} else {
		// Dev mode - use console writer
		writer = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "1000-01-01 10:00:00"}
	}

	// Note: New Relic log forwarding is now handled automatically by zerologWriter integration
	logger := zerolog.New(writer).
		Level(logLevel).
		With().Timestamp().
		Str("service", cfg.ServiceName).
		Str("environment", cfg.Environment).
		Logger()

	// Include stack traces for errors in development
	if !cfg.IsProduction() {
		logger = logger.With().Stack().Logger()
	}

	return logger
}

// WithTraceContext adds New Relic transaction context to logger
func WithTraceContext(logger zerolog.Logger, txn *newrelic.Transaction) zerolog.Logger {
	if txn == nil {
		return logger
	}

	metadata := txn.GetTraceMetadata()

	return logger.With().
		Str("trace.id", metadata.TraceID).
		Str("span.id", metadata.SpanID).
		Logger()
}

func NewPgxLogger(level zerolog.Level) zerolog.Logger {
	writer := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "1000-01-01 10:00:00",
		FormatFieldValue: func(i any) string {
			switch v := i.(type) {
			case string:
				// Clean and format SQL for better readability
				if len(v) > 200 {
					// Truncate very long SQL statements
					return v[:200] + "..."
				}
				return v
			case []byte:
				var obj any
				if err := json.Unmarshal(v, &obj); err == nil {
					pretty, _ := json.MarshalIndent(obj, "", "    ")
					return "\n" + string(pretty)
				}
				return string(v)
			default:
				return fmt.Sprintf("%v", v)
			}
		},
	}

	return zerolog.New(writer).
		Level(level).
		With().Timestamp().
		Str("component", "database").
		Logger()
}

func GetPgxTraceLogLevel(level zerolog.Level) tracelog.LogLevel {
	switch level {
	case zerolog.DebugLevel:
		return tracelog.LogLevelDebug
	case zerolog.InfoLevel:
		return tracelog.LogLevelInfo
	case zerolog.WarnLevel:
		return tracelog.LogLevelWarn
	case zerolog.ErrorLevel:
		return tracelog.LogLevelError
	default:
		return tracelog.LogLevelNone
	}
}
