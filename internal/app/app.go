// Package app initializes and bootstraps the application with all its components.
package app

import (
	"context"
	"fluxqueue/internal/api/http/handler"
	"fluxqueue/internal/config"
	"fluxqueue/internal/scheduler"
	"fluxqueue/internal/store"
	"fluxqueue/internal/worker"
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type App struct {
	Cfg *config.Config
	Log zerolog.Logger

	Redis      *store.RedisStore
	httpServer *echo.Echo
	Worker     worker.WorkerPool
	Scheduler  scheduler.Scheduler
}

func (a *App) Start(ctx context.Context) error {
	a.Worker.Start(ctx)
	a.Scheduler.Start(ctx)
	log.Info().Str("env", a.Cfg.Server.Env).Msg("config loaded")
	return a.httpServer.Start(fmt.Sprintf(":%d", a.Cfg.Server.HTTPPort))
}

func (a *App) Shutdown(ctx context.Context) error {
	a.Scheduler.Stop()
	a.Worker.Stop()
	err := a.Redis.Close()
	if err != nil {
		log.Error().Err(err)
	}
	return a.httpServer.Shutdown(ctx)
}

func (a *App) registerRoute() {
	h := handler.NewHandler(a.Redis)
	registerPprof(a.httpServer)
	api := a.httpServer.Group("/api")
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
