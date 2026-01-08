package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

func TestScheduler_Success(t *testing.T) {
	redis := NewMockIRedisClient(t)
	scheduler := NewScheduler(redis, 10*time.Millisecond)
	ctx := context.Background()
	redis.EXPECT().MoveScheduledToReady(
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(2, nil)

	scheduler.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	scheduler.Stop()
}

func TestScheduler_Error(t *testing.T) {
	redis := NewMockIRedisClient(t)
	scheduler := NewScheduler(redis, 10*time.Millisecond)
	ctx := context.Background()
	redis.EXPECT().MoveScheduledToReady(
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(0, errors.New("some error"))

	scheduler.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	scheduler.Stop()
}
