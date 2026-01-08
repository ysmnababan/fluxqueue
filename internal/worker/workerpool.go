// Package worker implements a worker pool for processing tasks concurrently with retries and backoff.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"

	metric "fluxqueue/internal/metrics"
	"fluxqueue/internal/model"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var (
	errMaxRetriesExceeded   = errors.New("max retries exceeded")
	errHandlerNotRegistered = errors.New("handler not registered")
)

type IRedisClient interface {
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) (string, error)
	LPush(ctx context.Context, key string, value string) error
	ZAdd(ctx context.Context, key string, score float64, member string) error
	SetNX(ctx context.Context, key string, val string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key string, val string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) (int64, error)
}

type WorkerPool struct {
	redis             IRedisClient
	registry          *HandlerRegistry
	redisConsumer     int
	maxWorkers        int
	queueReady        string
	queueScheduled    string
	queueDead         string
	idempKey          string
	wg                *sync.WaitGroup
	consumerWg        *sync.WaitGroup
	cancelFunc        context.CancelFunc
	baseRetryInterval int
	taskChan          chan *model.Task
	moveToDLQ         func(ctx context.Context, task *model.Task, err error)
}

func NewWorkerPool(maxWorkerPool int, r IRedisClient, registry *HandlerRegistry, retryInv, consumer int) WorkerPool {
	wg := &sync.WaitGroup{}
	consumerWg := &sync.WaitGroup{}
	wp := WorkerPool{
		redis:             r,
		registry:          registry,
		maxWorkers:        maxWorkerPool,
		wg:                wg,
		consumerWg:        consumerWg,
		queueReady:        "queue:ready",
		queueScheduled:    "queue:scheduled",
		queueDead:         "queue:dead",
		idempKey:          "idempotent:",
		baseRetryInterval: retryInv,
		redisConsumer:     consumer,
		taskChan:          make(chan *model.Task, 2*maxWorkerPool),
	}
	wp.moveToDLQ = wp.moveToDLQImpl
	return wp
}

func (w *WorkerPool) Start(ctx context.Context) {
	controlCtx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel
	taskCtx := context.Background()

	w.wg.Add(w.maxWorkers)
	w.consumerWg.Add(w.redisConsumer)
	log.Info().Msgf("[CONSUMER POOL STARTED]: %d instances", w.redisConsumer)
	for range w.redisConsumer {
		go w.consumeTaskFromStore(controlCtx)
	}
	log.Info().Msgf("[WORKER POOL STARTED]: %d instances", w.maxWorkers)
	for i := range w.maxWorkers {
		idx := i
		go w.workerLoop(taskCtx, idx)
	}
}

func (w *WorkerPool) consumeTaskFromStore(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.consumerWg.Done()
			log.Info().Msg("close consumer")
			return
		default:
		}
		taskStr, err := w.redis.BRPop(ctx, time.Second, w.queueReady)
		if err != nil {
			if err != redis.Nil {
				log.Error().Err(err).Msg("error fetching task from store")
			}
			continue
		}

		task := &model.Task{}
		err = json.Unmarshal([]byte(taskStr), task)
		if err != nil {
			log.Error().Err(err).Msg("error marshalling")
			continue
		}

		w.taskChan <- task
	}
}

func (w *WorkerPool) workerLoop(ctx context.Context, workerID int) {
	defer func() {
		log.Info().Msgf("worker %d stopping", workerID)
		w.wg.Done()
	}()

	for task := range w.taskChan {
		w.processTask(ctx, workerID, task)
	}
}

func (w *WorkerPool) Stop() {
	log.Info().Msg("Worker is terminating ...")
	w.cancelFunc()
	w.consumerWg.Wait()
	close(w.taskChan)
	w.wg.Wait()
}

func (w *WorkerPool) processTask(ctx context.Context, workerID int, task *model.Task) {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			log.Error().Msgf("worker %d panic: %v", workerID, r)
		}
	}()

	defer func() {
		elapsed := time.Since(start).Seconds()
		metric.TaskProcessingDuration.
			WithLabelValues(task.Type).Observe(float64(elapsed))
	}()

	// check idempotency key
	key := w.idempKey + task.IdempotencyKey
	ok, err := w.redis.SetNX(ctx, key, "processing", 10*time.Minute)
	if err != nil {
		log.Error().Err(err).Msg("error redis")
		return
	}
	if !ok {
		log.Info().Msgf("duplicate request with idempKey: %s", key)
		return
	}

	// get the registry
	handler, ok := w.registry.Get(task.Type)
	if !ok {
		metric.TaskFailedTotal.WithLabelValues(task.Type).Inc()
		log.Error().Msgf("no handler found for the task: %s", task.Type)
		w.moveToDLQ(ctx, task, errHandlerNotRegistered)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err = handler(ctx, task)
	if err == nil {
		metric.TaskProcessedTotal.WithLabelValues(task.Type).Inc()
		err = w.redis.Set(ctx, key, "done", time.Hour)
		if err != nil {
			log.Error().Err(err)
		}
		return
	}

	_, errRedis := w.redis.Del(ctx, key)
	if errRedis != nil {
		log.Error().Err(err)
	}
	task.Attempts++
	if task.Attempts > task.MaxRetries {
		// move to DLQ for further inspection
		metric.TaskFailedTotal.WithLabelValues(task.Type).Inc()
		log.Info().Msg("add to DLQ")
		w.moveToDLQ(ctx, task, err)
	} else {
		delaySec := w.baseRetryInterval * int(math.Pow(float64(2), float64(task.Attempts-1)))
		data, err := json.Marshal(task)
		if err != nil {
			log.Error().Err(err)
		}
		now := time.Now().Add(time.Duration(delaySec) * time.Second).UTC()
		err = w.redis.ZAdd(ctx, w.queueScheduled, float64(now.UnixMilli()), string(data))
		if err != nil {
			log.Error().Err(err).Msg("error add to scheduled")
			return
		}
		log.Info().Msgf("retry task %s for %d seconds later", task.ID, delaySec)
		metric.TaskRetriedTotal.WithLabelValues(task.Type).Inc()
	}
}

func (w *WorkerPool) moveToDLQImpl(ctx context.Context, task *model.Task, err error) {
	now := time.Now().UTC()
	task.FailedAt = &now
	task.LastError = err.Error()
	data, err := json.Marshal(task)
	if err != nil {
		log.Error().Err(err)
	}
	err = w.redis.LPush(ctx, w.queueDead, string(data))
	if err != nil {
		log.Error().Err(err)
	}
}
