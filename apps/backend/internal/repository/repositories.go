package repository

import "github.com/mabhi256/go-boilerplate-echo-pgx-newrelic/internal/server"

type Repositories struct{}

func NewRepositories(s *server.Server) *Repositories {
	return &Repositories{}
}
