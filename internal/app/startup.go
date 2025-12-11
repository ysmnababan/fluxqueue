package app

import (
	"fluxqueue/internal/api/http"
	"fluxqueue/internal/config"
	"fluxqueue/internal/logging"
	"fluxqueue/internal/scheduler"
	"fluxqueue/internal/store"
	"fluxqueue/internal/worker"
	"time"

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
	scheduler := scheduler.NewScheduler(
		redis,
		time.Millisecond*time.Duration(cfg.Worker.SchedulerTickInterval))

	// svc := service.NewTaskService(queue, logger)

	e := http.InitServer()
	app := &App{
		Cfg:        cfg,
		Log:        logger,
		httpServer: e,
		Worker:     worker,
		Scheduler:  scheduler,
		Redis:      redis,
	}
	app.registerRoute()
	return app, nil
}
