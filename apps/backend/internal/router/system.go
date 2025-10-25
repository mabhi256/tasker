package router

import (
	"github.com/labstack/echo/v4"
	"github.com/mabhi256/go-boilerplate-echo-pgx-newrelic/internal/handler"
)

func registerSystemRoutes(r *echo.Echo, h *handler.Handlers) {
	r.GET("/status", h.Health.CheckHealth)

	r.Static("/static", "static")

	r.GET("/docs", h.OpenAPI.ServeOpenAPIUI)
}
