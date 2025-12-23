// Package http sets up the Echo HTTP server with routes, middleware, and validators.
package http

import (
	"fluxqueue/internal/api/http/middleware"
	metric "fluxqueue/internal/metrics"
	"fluxqueue/utils/validator"

	"github.com/labstack/echo/v4"
)

func InitServer() *echo.Echo {
	e := echo.New()
	middleware.Setup(e)
	e.Validator = validator.NewCustomValidator()
	e.Use(metric.TrackMetrics)
	return e
}
