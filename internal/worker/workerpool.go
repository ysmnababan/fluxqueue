package worker

import (
	"context"
	"fluxqueue/internal/model"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type IRedisClient interface {
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) (string, error)
	LPush(ctx context.Context, key string, value string) error
}

type WorkerPool struct {
	redis      IRedisClient
	registry   *HandlerRegistry
	sem        chan struct{} // semaphore
	queueReady string
	wg         *sync.WaitGroup
	cancelFunc context.CancelFunc
}

func NewWorkerPool(maxWorkerPool int, r IRedisClient, registry *HandlerRegistry) WorkerPool {
	sem := make(chan struct{}, maxWorkerPool)
	wg := &sync.WaitGroup{}
	return WorkerPool{
		redis:      r,
		registry:   registry,
		sem:        sem,
		wg:         wg,
		queueReady: "queue:ready",
	}
}

func (w *WorkerPool) Start(ctx context.Context) {
	newCtx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel

	go func() {
		for {
			select {
			case <-newCtx.Done():
				// cancellation requested
				log.Info().Msg("Worker pool is stopped, wait for graceful shutdown")
				w.wg.Wait()
				return
			case w.sem <- struct{}{}:
				// process each task
				task, err := w.redis.BRPop(newCtx, time.Second, w.queueReady)
				if err != nil {
					log.Error().Err(err).Msg("error fetching task from store")
					continue
				}

				w.wg.Add(1)
				go func() {
					defer func() {
						<-w.sem // return the semaphore
					}()
					defer w.wg.Done()
					err := w.processTask(newCtx, task)
					if err != nil {
						log.Error().Err(err).Msg("error processing Task")
						// TODO: add process of readding to queue
						// there must be another storage to know how many times each task is retried
						// maybe use Lpush or Zadd with incremental time retry
						
					}
					log.Info().Msg("task is executed successfuly")
				}()
			}
		}
	}()
}

func (w *WorkerPool) Stop() {
	w.cancelFunc()
}

func (w *WorkerPool) processTask(ctx context.Context, task string) error {
	// process the 'task' string into Task model
	_ = task
	taskType := task

	// get the registry
	handler, ok := w.registry.Get(taskType)
	if !ok {
		return fmt.Errorf("no handler registry found for %s", taskType)
	}
	err := handler(ctx, &model.Task{})
	if err != nil {
		return err
	}
	return nil
}
