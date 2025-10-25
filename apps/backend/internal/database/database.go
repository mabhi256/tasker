package database

import (
	"context"
	"fmt"
	"net/url"
	"time"

	pgxzero "github.com/jackc/pgx-zerolog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/mabhi256/go-boilerplate-echo-pgx-newrelic/internal/config"
	"github.com/mabhi256/go-boilerplate-echo-pgx-newrelic/internal/logging"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
	"github.com/rs/zerolog"
)

// todo: use bob for typesafe query building
type Database struct {
	Pool *pgxpool.Pool
	log  *zerolog.Logger
}

// multiTracer allows chaining multiple tracers
type multiTracer struct {
	tracers []pgx.QueryTracer
}

// TraceQueryStart implements pgx tracer interface
func (mt *multiTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	for _, tracer := range mt.tracers {
		ctx = tracer.TraceQueryStart(ctx, conn, data)
	}
	return ctx
}

func (mt *multiTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	for _, tracer := range mt.tracers {
		tracer.TraceQueryEnd(ctx, conn, data)
	}
}

const DbPingTimeout = 10

func New(cfg *config.Config, logger *zerolog.Logger, loggerService *logging.LoggerService) (*Database, error) {
	encodedPassword := url.QueryEscape(cfg.Database.Password)
	// "postgres://username:password@localhost:5432/database_name?sslmode=false"
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User, encodedPassword,
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Name,
		cfg.Database.SSLMode)

	pgxPoolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pgx pool config: %w", err)
	}

	// Add New Relic PostgreSQL instrumentation
	if loggerService != nil && loggerService.GetApplication() != nil {
		pgxPoolConfig.ConnConfig.Tracer = nrpgx5.NewTracer()
	}

	if cfg.Primary.Env == "local" {
		globalLevel := logger.GetLevel()
		pgxLogger := logging.NewPgxLogger(globalLevel)

		localTracer := &tracelog.TraceLog{
			Logger:   pgxzero.NewLogger(pgxLogger),
			LogLevel: logging.GetPgxTraceLogLevel(globalLevel),
		}
		// Chain tracers - New Relic first, then local logging
		if pgxPoolConfig.ConnConfig.Tracer != nil {
			// If New Relic tracer exists, create a multi-tracer
			pgxPoolConfig.ConnConfig.Tracer = &multiTracer{
				tracers: []pgx.QueryTracer{pgxPoolConfig.ConnConfig.Tracer, localTracer},
			}
		} else {
			pgxPoolConfig.ConnConfig.Tracer = localTracer
		}
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxPoolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), DbPingTimeout*time.Second)
	defer cancel()

	if err = pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info().Msg("connected to the database")

	database := &Database{
		Pool: pool,
		log:  logger,
	}

	return database, nil
}

func (db *Database) Close() {
	db.log.Info().Msg("closing database connection pool")
	db.Pool.Close()
}
