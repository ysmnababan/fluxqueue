// Package processor initializes and bootstraps the application with all its components.
package processor

import (
	"time"

	"fluxqueue/internal/config"
	"fluxqueue/internal/mail"
	"fluxqueue/internal/scheduler"
	"fluxqueue/internal/service"
	"fluxqueue/internal/storage"
	"fluxqueue/internal/store"
	"fluxqueue/internal/worker"
)

func NewProcessor(cfg *config.Config) (*Processor, error) {
	store := store.NewConsumerRedis(
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
		store,
		handlerRegistry,
		cfg.Worker.BaseRetryInterval,
		cfg.Redis.Worker)

	scheduler := scheduler.NewScheduler(
		store,
		time.Millisecond*time.Duration(cfg.Worker.SchedulerTickInterval))

	app := &Processor{
		Cfg:       cfg,
		Worker:    worker,
		Scheduler: scheduler,
		Redis:     store,
	}
	return app, nil
}
