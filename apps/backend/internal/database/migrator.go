package database

import (
	"context"
	"embed"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5"
	tern "github.com/jackc/tern/v2/migrate"
	"github.com/mabhi256/tasker/internal/config"
	"github.com/rs/zerolog"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Migrate(ctx context.Context, logger *zerolog.Logger, cfg *config.Config) error {
	encodedPassword := url.QueryEscape(cfg.Database.Password)

	// "postgres://username:password@localhost:5432/database_name?sslmode=false"
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User, encodedPassword,
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Name,
		cfg.Database.SSLMode)

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	m, err := tern.NewMigrator(ctx, conn, "schema_version")
	if err != nil {
		return fmt.Errorf("constructing database migrator: %w", err)
	}

	// subtree, err := fs.Sub(migrations, "migrations")
	// if err != nil {
	// 	return fmt.Errorf("retrieving database migrations subtree: %w", err)
	// }

	if err := m.LoadMigrations(migrations); err != nil {
		return fmt.Errorf("loading database migrations: %w", err)
	}

	from, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("retreiving current database migration version")
	}

	if err := m.Migrate(ctx); err != nil {
		return err
	}

	if from == int32(len(m.Migrations)) {
		logger.Info().Msgf("database schema up to date, version %d", len(m.Migrations))
	} else {
		logger.Info().Msgf("migrated database schema, from %d to %d", from, len(m.Migrations))
	}
	return nil
}

// in cli
// task migrations:new name=setup

// todo: check if goose is better
// db, err := goose.OpenDBWithDriver("pgx", dsn)
// if err != nil {
// 	return err
// }
// defer db.Close()

// goose.SetBaseFS(migrations)

// if err := goose.Up(db, "migrations"); err != nil {
// 	return err
// }
