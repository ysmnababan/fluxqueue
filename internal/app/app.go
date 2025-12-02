package app

import (
	"context"
	"fluxqueue/internal/api/http/handler"
	"fluxqueue/internal/config"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type App struct {
	Cfg *config.Config
	Log zerolog.Logger

	// Redis *store.RedisClient
	// Queue *store.Queue
	// Tasks *service.TaskService

	httpServer *echo.Echo
	// Worker *worker.Pool
}

func (a *App) Start() error {
	// go a.Worker.Start(context.Background())
	log.Info().Str("env", a.Cfg.Server.Env).Msg("config loaded")
	return a.httpServer.Start(fmt.Sprintf(":%d", a.Cfg.Server.HTTPPort))
}

func (a *App) Shutdown(ctx context.Context) error {
	// a.Worker.Stop(ctx)
	// return a.Redis.Close()

	return a.httpServer.Shutdown(ctx)
}

func (a *App) registerRoute() {
	h := handler.NewHandler()
	api := a.httpServer.Group("/api")
	v1 := api.Group("/v1")

	v1.POST("/enqueue", h.Enqueue)
}
