package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/mabhi256/tasker/internal/handler"
	"github.com/mabhi256/tasker/internal/middleware"
)

func RegisterV1Routes(router *echo.Group, handlers *handler.Handlers, middleware *middleware.Middlewares) {
	// Register todo routes
	registerTodoRoutes(router, handlers.Todo, handlers.Comment, middleware.Auth)

	// Register category routes
	registerCategoryRoutes(router, handlers.Category, middleware.Auth)

	// Register comment routes
	registerCommentRoutes(router, handlers.Comment, middleware.Auth)
}
