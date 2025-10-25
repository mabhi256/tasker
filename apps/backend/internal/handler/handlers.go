package handler

import (
	"github.com/mabhi256/go-boilerplate-echo-pgx-newrelic/internal/server"
	"github.com/mabhi256/go-boilerplate-echo-pgx-newrelic/internal/service"
)

type Handlers struct {
	Health  *HealthHandler
	OpenAPI *OpenAPIHandler
}

func NewHandlers(s *server.Server, services *service.Services) *Handlers {
	return &Handlers{
		Health:  NewHealthHandler(s),
		OpenAPI: NewOpenAPIHandler(s),
	}
}
