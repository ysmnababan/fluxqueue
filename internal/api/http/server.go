// Package http sets up the Echo HTTP server with routes, middleware, and validators.
package http

import (
	"net/http"
	"net/http/pprof"

	"fluxqueue/internal/api/http/handler"
	"fluxqueue/internal/api/http/middleware"
	"fluxqueue/internal/config"
	metric "fluxqueue/internal/metrics"
	"fluxqueue/internal/store"
	"fluxqueue/utils/validator"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func InitServer() *echo.Echo {
	e := echo.New()
	middleware.Setup(e)
	e.Validator = validator.NewCustomValidator()
	e.Use(metric.TrackMetrics)
	return e
}

func RegisterRoute(e *echo.Echo, cfg config.Config) {
	store := store.NewProducerRedis(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	h := handler.NewHandler(store)

	// register prometheus handler
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	registerPprof(e)
	api := e.Group("/api")
	v1 := api.Group("/v1")

	v1.POST("/enqueue", h.Enqueue)
	v1.POST("/schedule", h.Schedule)
}

func registerPprof(e *echo.Echo) {
	e.GET("/debug/pprof", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
	e.GET("/debug/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
	e.GET("/debug/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	e.GET("/debug/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))
}
