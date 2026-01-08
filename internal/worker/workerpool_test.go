package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"fluxqueue/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestConsumeTaskFromStore_Success(t *testing.T) {
	redis := NewMockIRedisClient(t)
	r := NewRegistry()
	w := NewWorkerPool(10, redis, r, 2, 1)

	ctx, cancel := context.WithCancel(context.Background())
	task := new(model.Task)
	taskStr, err := json.Marshal(task)
	require.NoError(t, err)
	w.consumerWg.Add(1)
	redis.EXPECT().
		BRPop(mock.Anything, mock.Anything, mock.Anything).
		Return(string(taskStr), nil)

	go w.consumeTaskFromStore(ctx)

	select {
	case got := <-w.taskChan:
		require.NotNil(t, got)
		cancel()
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no task is received")
	}
	w.consumerWg.Wait()
}

func TestConsumeTaskFromStore_ErrorRedis(t *testing.T) {
	redis := NewMockIRedisClient(t)
	r := NewRegistry()
	w := NewWorkerPool(10, redis, r, 2, 1)

	ctx, cancel := context.WithCancel(context.Background())
	w.consumerWg.Add(1)
	redis.EXPECT().
		BRPop(mock.Anything, mock.Anything, mock.Anything).
		Return("", errors.New("some error")).Maybe()

	go w.consumeTaskFromStore(ctx)

	select {
	case <-w.taskChan:
		t.Fatal("task should not send any task to the channel")
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	w.consumerWg.Wait()
}

func TestConsumeTaskFromStore_ErrorMarshal(t *testing.T) {
	redis := NewMockIRedisClient(t)
	r := NewRegistry()
	w := NewWorkerPool(10, redis, r, 2, 1)

	ctx, cancel := context.WithCancel(context.Background())
	w.consumerWg.Add(1)
	redis.EXPECT().
		BRPop(mock.Anything, mock.Anything, mock.Anything).
		Return("some text", nil).Maybe()

	go w.consumeTaskFromStore(ctx)

	select {
	case <-w.taskChan:
		t.Fatal("task should not send any task to the channel")
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	w.consumerWg.Wait()
}

func TestMoveToDLQ_Success(t *testing.T) {
	redis := NewMockIRedisClient(t)
	r := NewRegistry()
	w := NewWorkerPool(10, redis, r, 2, 1)
	task := new(model.Task)

	redis.EXPECT().
		LPush(mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	w.moveToDLQ(context.Background(), task, errors.New("someerror"))
}

func TestMoveToDLQ_ErrorRedis(t *testing.T) {
	redis := NewMockIRedisClient(t)
	r := NewRegistry()
	w := NewWorkerPool(10, redis, r, 2, 1)
	task := new(model.Task)

	redis.EXPECT().
		LPush(mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("some error")).
		Once()

	w.moveToDLQ(context.Background(), task, errors.New("someerror"))
}

func TestProcessTask_Success(t *testing.T) {
	redis := NewMockIRedisClient(t)
	reg := NewRegistry()

	called := false
	reg.Register("email", func(ctx context.Context, task *model.Task) error {
		called = true
		return nil
	})

	w := NewWorkerPool(1, redis, reg, 2, 1)

	task := &model.Task{
		Type:           "email",
		IdempotencyKey: "abc",
	}

	redis.EXPECT().
		SetNX(mock.Anything, mock.Anything, "processing", mock.Anything).
		Return(true, nil)

	redis.EXPECT().
		Set(mock.Anything, mock.Anything, "done", mock.Anything).
		Return(nil)

	w.processTask(context.Background(), 1, task)

	assert.True(t, called)
}

func TestProcessTask_Retry(t *testing.T) {
	redis := NewMockIRedisClient(t)
	reg := NewRegistry()

	reg.Register("email", func(ctx context.Context, task *model.Task) error {
		return errors.New("fail")
	})

	w := NewWorkerPool(1, redis, reg, 2, 1)

	task := &model.Task{
		Type:           "email",
		IdempotencyKey: "abc",
		Attempts:       0,
		MaxRetries:     3,
	}

	redis.EXPECT().
		SetNX(mock.Anything, mock.Anything, "processing", mock.Anything).
		Return(true, nil)

	redis.EXPECT().
		Del(mock.Anything, mock.Anything).
		Return(int64(1), nil)

	redis.EXPECT().
		ZAdd(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	w.processTask(context.Background(), 1, task)

	assert.Equal(t, 1, task.Attempts)
}

func TestProcessTask_PanicInHandler(t *testing.T) {
	redis := NewMockIRedisClient(t)
	reg := NewRegistry()

	reg.Register("panic", func(ctx context.Context, task *model.Task) error {
		panic("boom")
	})

	w := NewWorkerPool(1, redis, reg, 2, 1)

	task := &model.Task{
		Type:           "panic",
		IdempotencyKey: "x",
	}

	redis.EXPECT().
		SetNX(mock.Anything, mock.Anything, "processing", mock.Anything).
		Return(true, nil)

	assert.NotPanics(t, func() {
		w.processTask(context.Background(), 1, task)
	})
}

func TestProcessTask_NoHandler_MoveToDLQ(t *testing.T) {
	redis := NewMockIRedisClient(t)
	reg := NewRegistry()

	var dlqCalled bool
	var dlqTask *model.Task

	w := NewWorkerPool(1, redis, reg, 2, 1)
	w.moveToDLQ = func(ctx context.Context, task *model.Task, err error) {
		dlqCalled = true
		dlqTask = task
	}

	task := &model.Task{
		Type:           "unknown",
		IdempotencyKey: "abc",
	}

	redis.EXPECT().
		SetNX(mock.Anything, mock.Anything, "processing", mock.Anything).
		Return(true, nil)

	w.processTask(context.Background(), 1, task)

	assert.True(t, dlqCalled)
	assert.Equal(t, task, dlqTask)
}

func TestProcessTask_RetriesExceeded_MoveToDLQ(t *testing.T) {
	redis := NewMockIRedisClient(t)
	reg := NewRegistry()

	reg.Register("email", func(ctx context.Context, task *model.Task) error {
		return errors.New("fail")
	})

	var dlqCalled bool

	w := NewWorkerPool(2, redis, reg, 2, 1)
	w.moveToDLQ = func(ctx context.Context, task *model.Task, err error) {
		dlqCalled = true
	}

	task := &model.Task{
		Type:           "email",
		IdempotencyKey: "abc",
		Attempts:       3,
		MaxRetries:     3,
	}

	redis.EXPECT().
		SetNX(mock.Anything, mock.Anything, "processing", mock.Anything).
		Return(true, nil)

	redis.EXPECT().
		Del(mock.Anything, mock.Anything).
		Return(int64(1), nil)

	w.processTask(context.Background(), 1, task)

	assert.True(t, dlqCalled)
}

func TestWorkerPool_StartStop_NoLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	redis := NewMockIRedisClient(t)
	reg := NewRegistry()
	w := NewWorkerPool(5, redis, reg, 1, 1)

	// ctx, cancel := context.WithCancel(context.Background())
	w.Start(context.Background())
	// cancel()
	w.Stop()
}
