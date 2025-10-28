package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mabhi256/tasker/internal/config"
	"github.com/mabhi256/tasker/internal/database"
	"github.com/mabhi256/tasker/internal/lib/job"
	"github.com/mabhi256/tasker/internal/logging"
	"github.com/newrelic/go-agent/v3/integrations/nrredis-v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type Server struct {
	Config        *config.Config
	Logger        *zerolog.Logger
	LoggerService *logging.LoggerService
	DB            *database.Database
	Redis         *redis.Client
	httpServer    *http.Server
	Job           *job.JobService
}

func New(cfg *config.Config, logger *zerolog.Logger, loggerService *logging.LoggerService) (*Server, error) {
	db, err := database.New(cfg, logger, loggerService)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Address,
	})

	// Add New Relic Redis hooks if available
	if loggerService != nil && loggerService.GetApplication() != nil {
		redisClient.AddHook(nrredis.NewHook(redisClient.Options()))
	}

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = redisClient.Ping(ctx).Err()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to connect to Redis, continuing without Redis")
		// Don't fail startup if Redis is unavailable
	}

	// Job service
	jobService := job.NewJobService(cfg, logger)
	jobService.InitHandlers(cfg, logger)
	err = jobService.Start()
	if err != nil {
		return nil, err
	}

	server := &Server{
		Config:        cfg,
		Logger:        logger,
		LoggerService: loggerService,
		DB:            db,
		Redis:         redisClient,
		Job:           jobService,
	}
	// Runtime metrics are automatically collected by New Relic Go agent

	return server, nil
}

func (s *Server) SetupHttpServer(handler http.Handler) {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.Config.Server.Port),
		Handler:      handler,
		ReadTimeout:  time.Duration(s.Config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.Config.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(s.Config.Server.IdleTimeout) * time.Second,
	}
}

func (s *Server) Start() error {
	if s.httpServer == nil {
		return fmt.Errorf("http server not initialized")
	}

	s.Logger.Info().
		Str("port", fmt.Sprintf(":%d", s.Config.Server.Port)).
		Str("env", s.Config.Primary.Env).
		Msg("starting server")

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	s.DB.Close()
	if s.Job != nil {
		s.Job.Stop()
	}

	if s.Redis != nil {
		s.Redis.Close()
	}

	return nil
}
