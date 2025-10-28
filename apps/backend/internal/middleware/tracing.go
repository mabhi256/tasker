package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/mabhi256/tasker/internal/server"
	"github.com/newrelic/go-agent/v3/integrations/nrecho-v4"
	"github.com/newrelic/go-agent/v3/integrations/nrpkgerrors"
	"github.com/newrelic/go-agent/v3/newrelic"
)

type TracingMiddleware struct {
	server *server.Server
	nrApp  *newrelic.Application
}

func NewTracingMiddleware(s *server.Server, nrApp *newrelic.Application) *TracingMiddleware {
	return &TracingMiddleware{
		server: s,
		nrApp:  nrApp,
	}
}

// NewRelicMiddleware returns the New Relic middleware for Echo
func (tm *TracingMiddleware) NewRelicMiddleware() echo.MiddlewareFunc {
	if tm.nrApp == nil {
		// Return a no-op middleware if New Relic is not initialized
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}
	return nrecho.Middleware(tm.nrApp)
}

// EnhanceTracing adds custom attributes to New Relic transactions
func (tm *TracingMiddleware) EnhanceTracing() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get New Relic transaction from context
			txn := newrelic.FromContext(c.Request().Context())
			if txn == nil {
				return next(c)
			}

			// service.name and service.environment are already set in logger and New Relic config
			txn.AddAttribute("http.real_ip", c.RealIP())
			txn.AddAttribute("http.user_agent", c.Request().UserAgent())

			// Add request ID if available
			if reqID := GetRequestID(c); reqID != "" {
				txn.AddAttribute("request.id", reqID)
			}

			// Add user context if available
			if userID := GetUserID(c); userID != "" {
				txn.AddAttribute("user.id", userID)
			}

			// Execute next handler
			err := next(c)
			// Record error if any with enhanced stack traces
			if err != nil {
				txn.NoticeError(nrpkgerrors.Wrap(err))
			}

			// Add response status
			txn.AddAttribute("http.status_code", c.Response().Status)

			return err
		}
	}
}
