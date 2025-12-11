package worker

import (
	"context"
	"encoding/json"
	"fluxqueue/internal/model"
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
	maxWorkers int
	queueReady string
	wg         *sync.WaitGroup
	cancelFunc context.CancelFunc
}

func NewWorkerPool(maxWorkerPool int, r IRedisClient, registry *HandlerRegistry) WorkerPool {
	wg := &sync.WaitGroup{}
	return WorkerPool{
		redis:      r,
		registry:   registry,
		maxWorkers: maxWorkerPool,
		wg:         wg,
		queueReady: "queue:ready",
	}
}

func (w *WorkerPool) Start(ctx context.Context) {
	newCtx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel

	w.wg.Add(w.maxWorkers)
	for i := range w.maxWorkers {
		idx := i
		go w.workerLoop(newCtx, idx)
	}
}

func (w *WorkerPool) workerLoop(ctx context.Context, workerId int) {
	for {
		select {
		case <-ctx.Done():
			w.wg.Done()
			log.Info().Msgf("worker %d is stopping\n", workerId)
			return
		default:
			taskStr, err := w.redis.BRPop(ctx, 0, w.queueReady)
			if err != nil {
				log.Error().Err(err).Msg("error fetching task from store")
				continue
			}

			// unmarshall the Task
			task := &model.Task{}
			err = json.Unmarshal([]byte(taskStr), task)
			if err != nil {
				log.Error().Err(err).Msg("invalid task JSON")
				continue
			}
			w.processTask(ctx, workerId, task)
		}
	}
}

func (w *WorkerPool) Stop() {
	w.cancelFunc()
	w.wg.Wait()
}

func (w *WorkerPool) processTask(ctx context.Context, workerId int, task *model.Task) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Msgf("worker %d panic: %v", workerId, r)
		}
	}()

	// get the registry
	handler, ok := w.registry.Get(task.Type)
	if !ok {
		log.Error().Msgf("no handler found for the task: %s", task.Type)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := handler(ctx, &model.Task{})
	task.Attempts++
	if err != nil {
		log.Error().Err(err).Msg("error handling task")
		// TODO: add process of readding to queue
		// there must be another storage to know how many times each task is retried
		// maybe use Lpush or Zadd with incremental time retry
		if task.Attempts >= task.MaxRetries {
			// move to DLQ for further inspection
			log.Info().Msg("add to DLQ")
		} else {
			// add to queue again with
			log.Info().Msg("retry x seconds later")
		}
	}
}