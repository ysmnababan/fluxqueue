package worker

import (
	"context"
	"encoding/json"
	"fluxqueue/internal/model"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type IRedisClient interface {
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) (string, error)
	LPush(ctx context.Context, key string, value string) error
	ZAdd(ctx context.Context, key string, score float64, member string) error
}

type WorkerPool struct {
	redis             IRedisClient
	registry          *HandlerRegistry
	maxWorkers        int
	queueReady        string
	queueScheduled    string
	wg                *sync.WaitGroup
	cancelFunc        context.CancelFunc
	baseRetryInterval int
}

func NewWorkerPool(maxWorkerPool int, r IRedisClient, registry *HandlerRegistry, retryInv int) WorkerPool {
	wg := &sync.WaitGroup{}
	return WorkerPool{
		redis:             r,
		registry:          registry,
		maxWorkers:        maxWorkerPool,
		wg:                wg,
		queueReady:        "queue:ready",
		queueScheduled:    "queue:scheduled",
		baseRetryInterval: retryInv,
	}
}

func (w *WorkerPool) Start(ctx context.Context) {
	newCtx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel

	w.wg.Add(w.maxWorkers)
	log.Info().Msgf("[WORKER POOL STARTED]: %d instances", w.maxWorkers)
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
			taskStr, err := w.redis.BRPop(ctx, time.Second, w.queueReady)
			if err != nil {
				if err != redis.Nil {
					log.Error().Err(err).Msg("error fetching task from store")
				}
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
		if task.Attempts > task.MaxRetries {
			// move to DLQ for further inspection
			log.Info().Msg("add to DLQ")
		} else {
			delaySec := w.baseRetryInterval * int(math.Pow(float64(2), float64(task.Attempts-1)))
			data, _ := json.Marshal(task)
			now := time.Now().Add(time.Duration(delaySec) * time.Second).UTC()
			err := w.redis.ZAdd(ctx, w.queueScheduled, float64(now.UnixMilli()), string(data))
			if err != nil {
				log.Error().Err(err).Msg("error add to scheduled")
				return
			}
			log.Info().Msgf("retry task %s for %d seconds later", task.ID, delaySec)
		}
	}
}
