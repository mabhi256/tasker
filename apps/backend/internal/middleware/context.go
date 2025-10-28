package middleware

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/mabhi256/tasker/internal/logging"
	"github.com/mabhi256/tasker/internal/server"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/rs/zerolog"
)

// Define custom types for context keys to avoid collisions
type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UserRoleKey contextKey = "user_role"
	LoggerKey   contextKey = "logger"
)

type ContextEnhancer struct {
	server *server.Server
}

func NewContextEnhancer(s *server.Server) *ContextEnhancer {
	return &ContextEnhancer{server: s}
}

func (ce *ContextEnhancer) EnhanceContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Extract request ID
			requestID := GetRequestID(c)

			// Create enhanced logger with request context
			contextLogger := ce.server.Logger.With().
				Str("request_id", requestID).
				Str("method", c.Request().Method).
				Str("path", c.Path()).
				Str("ip", c.RealIP()).
				Logger()

			// Add trace context if available
			txn := newrelic.FromContext(c.Request().Context())
			if txn != nil {
				contextLogger = logging.WithTraceContext(contextLogger, txn)
			}

			// Extract user information from JWT token or session
			userID := ce.extractUserID(c)
			if userID != "" {
				contextLogger = contextLogger.With().Str(string(UserIDKey), userID).Logger()
			}

			userRole := ce.extractUserRole(c)
			if userRole != "" {
				contextLogger = contextLogger.With().Str(string(UserRoleKey), userRole).Logger()
			}

			// Store the enhanced logger in context
			c.Set(string(LoggerKey), &contextLogger)

			// Create a new context with the logger
			ctx := context.WithValue(c.Request().Context(), LoggerKey, &contextLogger)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

func (ce *ContextEnhancer) extractUserID(c echo.Context) string {
	return GetUserID(c)
}

func GetUserID(c echo.Context) string {
	// Check if user_id was already set by auth middleware (Clerk)
	if userID, ok := c.Get(string(UserIDKey)).(string); ok {
		return userID
	}
	return ""
}

func (ce *ContextEnhancer) extractUserRole(c echo.Context) string {
	// Check if user_role was already set by auth middleware (Clerk)
	if userRole, ok := c.Get(string(UserRoleKey)).(string); ok {
		return userRole
	}
	return ""
}

func GetLogger(c echo.Context) *zerolog.Logger {
	if logger, ok := c.Get(string(LoggerKey)).(*zerolog.Logger); ok {
		return logger
	}
	// Fallback to a basic logger if not found
	logger := zerolog.Nop()
	return &logger
}
