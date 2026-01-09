// Package processor initializes and bootstraps the application with all its components.
package processor

import (
	"context"
	"fmt"
	"net/http"

	"fluxqueue/internal/config"
	"fluxqueue/internal/scheduler"
	"fluxqueue/internal/store"
	"fluxqueue/internal/worker"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

type Processor struct {
	Cfg *config.Config

	Redis     *store.RedisStore
	Worker    *worker.WorkerPool
	Scheduler scheduler.Scheduler
}

func (a *Processor) Start(ctx context.Context) {
	a.Worker.Start(ctx)
	a.Scheduler.Start(ctx)
	log.Info().Str("env", a.Cfg.Server.Env).Msg("config loaded")

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		port := fmt.Sprintf(":%d", a.Cfg.Worker.Port)
		log.Info().Msg("/metrics endpoint running...")
		err := http.ListenAndServe(port, nil)
		if err != nil {
			fmt.Println("err", err)
			panic(err)
		}
	}()
}

func (a *Processor) Shutdown(ctx context.Context) error {
	a.Scheduler.Stop()
	a.Worker.Stop()
	err := a.Redis.Close()
	if err != nil {
		log.Error().Err(err)
	}
	return nil
}
