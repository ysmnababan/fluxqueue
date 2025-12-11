package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

type IRedisClient interface {
	// BRPop(ctx context.Context, timeout time.Duration, keys ...string) (string, error)
	// LPush(ctx context.Context, key string, value string) error
	// ZRangeByScore(ctx context.Context, key string, min, max string) ([]string, error)
	// ZPopMin(ctx context.Context, key string) ([]model.ZItem, error)
	MoveScheduledToReady(ctx context.Context, zsetKey, readyListKey, maxScore string) (int64, error)
}

type Scheduler struct {
	queueScheduled string
	queueReady     string
	redis          IRedisClient
	cancelFunc     context.CancelFunc
	tickInterval   time.Duration
	backoffTime    [3]int
}

func NewScheduler(backoffTime [3]int, redis IRedisClient, tickInterval time.Duration) *Scheduler {
	return &Scheduler{
		queueScheduled: "queue:scheduled",
		queueReady:     "queue:ready",
		redis:          redis,
		backoffTime:    backoffTime,
		tickInterval:   tickInterval,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel
	ticker := time.NewTicker(s.tickInterval)
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Scheduler is terminating ...")
			return
		case <-ticker.C:
			// take a executable task(s)
			now := time.Now().UTC().UnixMilli()
			moved, err := s.redis.MoveScheduledToReady(ctx, s.queueScheduled, s.queueReady, fmt.Sprintf("%d", now))
			if err != nil {
				log.Error().Err(err).Msg("scheduler failed")
				continue
			}
			if moved > 0 {
				log.Info().Msgf("scheduler moved %d items to ready queue", moved)
			}
		}
	}
}

func (s *Scheduler) Stop() {
	s.cancelFunc()
}
