package app

import (
	"fluxqueue/internal/api/http"
	"fluxqueue/internal/config"
	"fluxqueue/internal/logging"

	"github.com/rs/zerolog"
)

func NewApp(cfg *config.Config) (*App, error) {
	logger := logging.InitLogger(cfg.Server.Env,
		zerolog.InfoLevel,
		cfg.Server.ServiceName,
		cfg.Server.Version)

	// redis, err := store.NewRedisClient(cfg.Redis, logger)
	// if err != nil {
	// 	return nil, err
	// }

	// queue := store.NewQueue(redis, logger)
	// svc := service.NewTaskService(queue, logger)

	// httpServer := http.NewServer(cfg.HTTP, svc, logger)
	// pool := worker.NewPool(svc, logger, cfg.Worker)

	e := http.InitServer()
	app := &App{
		Cfg:        cfg,
		Log:        logger,
		httpServer: e,
	}
	app.registerRoute()
	return app, nil
}
