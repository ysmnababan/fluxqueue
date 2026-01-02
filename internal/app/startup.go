// Package app initializes and bootstraps the application with all its components.
package app

import (
	"fluxqueue/internal/api/http"
	"fluxqueue/internal/config"
	"fluxqueue/internal/logging"
	"fluxqueue/internal/mail"
	"fluxqueue/internal/scheduler"
	"fluxqueue/internal/service"
	"fluxqueue/internal/storage"
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

	producerRedis := store.NewProducerRedis(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	consumerRedis := store.NewConsumerRedis(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Redis.Worker,
	)
	storage := storage.NewStorage(cfg.Storage.Endpoint,
		cfg.Storage.AccessKey,
		cfg.Storage.SecretAccessKey,
		false)
	emailService := mail.NewEmailService(cfg.Email)

	handlerRegistry := worker.NewRegistry()

	svc := service.NewTaskService(storage, emailService)
	handlerRegistry.Register("email.send", svc.SendEmail)
	handlerRegistry.Register("report.generate", svc.GenerateExcelReport)

	worker := worker.NewWorkerPool(
		cfg.Worker.WorkerCount,
		consumerRedis,
		handlerRegistry,
		cfg.Worker.BaseRetryInterval,
		cfg.Redis.Worker)

	scheduler := scheduler.NewScheduler(
		consumerRedis,
		time.Millisecond*time.Duration(cfg.Worker.SchedulerTickInterval))

	e := http.InitServer()
	app := &App{
		Cfg:        cfg,
		Log:        logger,
		httpServer: e,
		Worker:     worker,
		Scheduler:  scheduler,
		Redis:      producerRedis,
	}
	app.registerRoute()
	return app, nil
}
