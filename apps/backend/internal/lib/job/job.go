package job

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/mabhi256/tasker/internal/config"
	"github.com/mabhi256/tasker/internal/lib/email"
	"github.com/rs/zerolog"
)

type JobService struct {
	Client      *asynq.Client
	server      *asynq.Server
	logger      *zerolog.Logger
	authService AuthServiceInterface
	emailClient *email.Client
}

type AuthServiceInterface interface {
	GetUserEmail(ctx context.Context, userID string) (string, error)
}

func NewJobService(cfg *config.Config, logger *zerolog.Logger) *JobService {
	redisAddr := cfg.Redis.Address

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr: redisAddr,
	})

	server := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6, // Higher priority queue for important emails
				"default":  3, // Default priority for most emails
				"low":      1, // Lower priority for non-urgent emails
			},
		},
	)

	return &JobService{
		Client: client,
		server: server,
		logger: logger,
	}
}

func (j *JobService) SetAuthService(authService AuthServiceInterface) {
	j.authService = authService
}

func (j *JobService) Start() error {
	// Register task handlers
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskWelcome, j.handleWelcomeEmailTask)
	mux.HandleFunc(TaskReminderEmail, j.handleReminderEmailTask)
	mux.HandleFunc(TaskWeeklyReportEmail, j.handleWeeklyReportEmailTask)

	j.logger.Info().Msg("Starting background job server")
	err := j.server.Start(mux)
	if err != nil {
		return err
	}

	return nil
}

func (j *JobService) Stop() {
	j.logger.Info().Msg("Stopping background job server")
	j.server.Shutdown()
	j.Client.Close()
}
