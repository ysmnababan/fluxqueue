package app

import (
	"fluxqueue/internal/api/http"
	"fluxqueue/internal/config"
	"fluxqueue/internal/logging"
	"fluxqueue/internal/store"
	"fluxqueue/internal/worker"

	"github.com/rs/zerolog"
)

func NewApp(cfg *config.Config) (*App, error) {
	logger := logging.InitLogger(cfg.Server.Env,
		zerolog.InfoLevel,
		cfg.Server.ServiceName,
		cfg.Server.Version)

	redis := store.NewRedisClient(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	handlerRegistry := worker.NewRegistry()
	worker := worker.NewWorkerPool(cfg.Worker.WorkerCount, redis, handlerRegistry)
	// queue := store.NewQueue(redis, logger)
	// svc := service.NewTaskService(queue, logger)

	e := http.InitServer()
	app := &App{
		Cfg:        cfg,
		Log:        logger,
		httpServer: e,
		Worker:     worker,
	}
	app.registerRoute()
	return app, nil
}
