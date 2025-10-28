package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/mabhi256/tasker/internal/config"
	"github.com/mabhi256/tasker/internal/database"
	"github.com/mabhi256/tasker/internal/handler"
	"github.com/mabhi256/tasker/internal/logging"
	"github.com/mabhi256/tasker/internal/repository"
	"github.com/mabhi256/tasker/internal/router"
	"github.com/mabhi256/tasker/internal/server"
	"github.com/mabhi256/tasker/internal/service"
)

const DefaultContextTimeout = 30

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// Initialize New Relic logger service
	loggerService := logging.NewLoggerService(cfg.Observability)
	defer loggerService.Shutdown()

	log := logging.NewLoggerWithService(cfg.Observability, loggerService)

	if cfg.Primary.Env != "local" {
		if err := database.Migrate(context.Background(), &log, cfg); err != nil {
			log.Fatal().Err(err).Msg("failed to migrate database")
		}
	}

	// Initialize server
	srv, err := server.New(cfg, &log, loggerService)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize server")
	}

	// Initialize repositories, services, and handlers
	repos := repository.NewRepositories(srv)
	services, serviceErr := service.NewServices(srv, repos)
	if serviceErr != nil {
		log.Fatal().Err(serviceErr).Msg("could not create services")
	}
	handlers := handler.NewHandlers(srv, services)

	// Initialize router
	r := router.NewRouter(srv, handlers, services)

	// Setup HTTP server
	srv.SetupHttpServer(r)
	go func() {
		if err = srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("failed to start server")
		}
	}()

	// Wait for interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	<-ctx.Done()

	// Create shutdown timeout to gracefully shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout*time.Second)
	if err = srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}
	stop()   // Release signal notification resources
	cancel() // Release timeout context resources

	log.Info().Msg("server exited properly")
}
