package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fluxqueue/internal/model"
	"fluxqueue/internal/store"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
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
	redis.EXPECT().
		LPush(mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
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

	redis.EXPECT().BRPop(mock.Anything, mock.Anything, mock.Anything).Return("", nil)
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	cancel()
	w.Stop()
}

func TestGracefulShutdown_NoTaskLoss(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Setup
	redisMock := NewMockIRedisClient(t)
	reg := NewRegistry()

	// Track processed tasks
	var processedTasks []string
	var mu sync.Mutex

	// Slow handler to ensure tasks are in flight during shutdown
	reg.Register("slow_task", func(ctx context.Context, task *model.Task) error {
		time.Sleep(100 * time.Millisecond) // Simulate slow processing
		mu.Lock()
		processedTasks = append(processedTasks, task.ID)
		mu.Unlock()
		return nil
	})

	w := NewWorkerPool(3, redisMock, reg, 1, 2) // 3 workers, 2 consumers

	// Queue of tasks to return from BRPop
	taskQueue := []string{
		`{"id":"task1","type":"slow_task","idempotency_key":"ik1"}`,
		`{"id":"task2","type":"slow_task","idempotency_key":"ik2"}`,
		`{"id":"task3","type":"slow_task","idempotency_key":"ik3"}`,
		`{"id":"task4","type":"slow_task","idempotency_key":"ik4"}`,
		`{"id":"task5","type":"slow_task","idempotency_key":"ik5"}`,
	}

	// Use atomic counter for thread-safe tracking
	var callCount int32

	// Mock BRPop to return tasks one by one
	redisMock.EXPECT().BRPop(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, timeout time.Duration, keys ...string) (string, error) {
			// Atomically increment and get the new value
			cnt := atomic.AddInt32(&callCount, 1)

			if cnt <= int32(len(taskQueue)) {
				return taskQueue[cnt-1], nil
			}
			return "", redis.Nil
		}).
		Maybe()

	// Mock other operations
	redisMock.EXPECT().SetNX(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()
	redisMock.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()
	redisMock.EXPECT().Del(mock.Anything, mock.Anything).
		Return(int64(1), nil).Maybe()
	redisMock.EXPECT().LPush(mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()

	// Start worker pool
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	// Let some tasks start processing
	time.Sleep(150 * time.Millisecond)

	// Cancel context while tasks are in flight
	log.Info().Msg("=== CANCELLING CONTEXT ===")
	cancel()

	// Stop should complete without hanging
	log.Info().Msg("=== CALLING STOP ===")
	w.Stop()
	log.Info().Msg("=== SHUTDOWN COMPLETE ===")

	// Verify results with proper locking
	mu.Lock()
	processedCount := len(processedTasks)
	mu.Unlock()

	t.Logf("Processed tasks: %d / %d", processedCount, len(taskQueue))

	// With graceful shutdown, some tasks should be processed
	assert.GreaterOrEqual(t, processedCount, 1,
		"At least some tasks should be processed before shutdown")
}

func TestGracefuShutdown_RealRedis_NoTaskLoss(t *testing.T) {
	// Skip if Redis not available
	redisStore := store.NewConsumerRedis("localhost:6379", "", 0, 2)
	err := redisStore.Ping(context.Background())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisStore.Close()

	// Use CONSISTENT queue names - don't override after creation
	queueReady := "test:queue:ready:integration"
	queueScheduled := "test:queue:scheduled:integration"
	queueDead := "test:queue:dead:integration"

	// Clean up test queues
	ctx := context.Background()
	redisStore.Del(ctx, queueReady, queueScheduled, queueDead)

	// Register handler
	reg := NewRegistry()
	var processed int32
	var processedMu sync.Mutex
	var processedTasks []string

	reg.Register("test_task", func(ctx context.Context, task *model.Task) error {
		t.Logf("Handler executing for task: %s", task.ID)
		time.Sleep(150 * time.Millisecond) // Simulate work
		atomic.AddInt32(&processed, 1)

		processedMu.Lock()
		processedTasks = append(processedTasks, task.ID)
		processedMu.Unlock()

		t.Logf("Handler completed for task: %s", task.ID)
		return nil
	})

	// Create WorkerPool BEFORE setting queue names
	wp := NewWorkerPool(3, redisStore, reg, 1, 2)

	// IMPORTANT: Override queue names BEFORE Start()
	wp.queueReady = queueReady
	wp.queueScheduled = queueScheduled
	wp.queueDead = queueDead

	// Enqueue 10 tasks
	totalTasks := 10
	t.Logf("Enqueueing %d tasks to %s", totalTasks, queueReady)

	for i := 1; i <= totalTasks; i++ {
		task := &model.Task{
			ID:             fmt.Sprintf("task-%d", i),
			Type:           "test_task",
			IdempotencyKey: fmt.Sprintf("ik-%d", i),
			Attempts:       0,
			MaxRetries:     3,
		}
		data, err := json.Marshal(task)
		require.NoError(t, err)

		err = redisStore.LPush(ctx, queueReady, string(data))
		require.NoError(t, err)
		t.Logf("  Enqueued: %s", task.ID)
	}

	// Verify enqueued
	initialLen, err := redisStore.LLen(ctx, queueReady)
	require.NoError(t, err)
	t.Logf("Verified enqueued: %d tasks in queue '%s'", initialLen, queueReady)
	assert.Equal(t, int64(totalTasks), initialLen)

	// Start processing
	t.Log("Starting worker pool...")
	ctxRun, cancel := context.WithCancel(context.Background())
	wp.Start(ctxRun)

	// Let some tasks be processed
	// With 3 workers and 150ms per task, we should process ~6 tasks in 300ms
	t.Log("Waiting 300ms for processing...")
	time.Sleep(300 * time.Millisecond)

	processed1 := atomic.LoadInt32(&processed)
	t.Logf("Status after 300ms: %d tasks processed", processed1)

	// Check queue status before cancellation
	beforeCancelLen, _ := redisStore.LLen(ctx, queueReady)
	t.Logf("Queue length before cancel: %d", beforeCancelLen)

	// Cancel context while tasks are in flight
	t.Log("=== CANCELLING CONTEXT ===")
	cancel()

	// Graceful shutdown
	t.Log("=== CALLING STOP ===")
	shutdownStart := time.Now()
	wp.Stop()
	shutdownDuration := time.Since(shutdownStart)
	t.Logf("Shutdown took %v", shutdownDuration)
	t.Log("=== SHUTDOWN COMPLETE ===")

	// Check final state
	processedFinal := atomic.LoadInt32(&processed)

	// Check all possible locations where tasks could be
	remainingInQueue, err := redisStore.LLen(ctx, queueReady)
	require.NoError(t, err)

	inScheduled, err := redisStore.ZCount(ctx, queueScheduled, "-inf", "+inf")
	require.NoError(t, err)

	inDLQ, err := redisStore.LLen(ctx, queueDead)
	require.NoError(t, err)

	processedMu.Lock()
	processedCount := len(processedTasks)
	processedMu.Unlock()
	fmt.Println(processedCount)
	t.Logf("\n=== FINAL STATE ===")
	t.Logf("Processed: %d", processedFinal)
	t.Logf("Remaining in queue '%s': %d", queueReady, remainingInQueue)
	t.Logf("In scheduled queue '%s': %d", queueScheduled, inScheduled)
	t.Logf("In DLQ '%s': %d", queueDead, inDLQ)
	t.Logf("Total accounted for: %d", processedFinal+int32(remainingInQueue)+int32(inScheduled)+int32(inDLQ))
	t.Logf("Total enqueued: %d", totalTasks)

	processedMu.Lock()
	if len(processedTasks) > 0 {
		t.Logf("Processed task IDs: %v", processedTasks)
	}
	processedMu.Unlock()

	// Verify no task loss
	totalAccountedFor := processedFinal + int32(remainingInQueue) + int32(inScheduled) + int32(inDLQ)

	if totalAccountedFor != int32(totalTasks) {
		t.Errorf("TASK LOSS DETECTED! Enqueued: %d, Accounted for: %d, Missing: %d",
			totalTasks, totalAccountedFor, int32(totalTasks)-totalAccountedFor)
	}

	assert.Equal(t, int32(totalTasks), totalAccountedFor,
		"Task loss detected! Some tasks were neither processed, in queue, scheduled, nor in DLQ")

	// Shutdown should be reasonably fast
	assert.Less(t, shutdownDuration, 30*time.Second,
		"Shutdown took too long - possible deadlock")
}

func TestGracefulShutdown_RealRedis_NoTaskLoss(t *testing.T) {
	// Skip if Redis not available
	redisStore := store.NewConsumerRedis("localhost:6379", "", 0, 2)
	err := redisStore.Ping(context.Background())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisStore.Close()

	// Clean up test queues
	ctx := context.Background()
	redisStore.Del(ctx, "test:queue:ready", "test:queue:scheduled", "test:queue:dead")

	// Register handler
	reg := NewRegistry()
	var processed int32
	var processedMu sync.Mutex
	var processedTasks []string

	reg.Register("test_task", func(ctx context.Context, task *model.Task) error {
		time.Sleep(200 * time.Millisecond) // Simulate slow task
		atomic.AddInt32(&processed, 1)

		processedMu.Lock()
		processedTasks = append(processedTasks, task.ID)
		processedMu.Unlock()
		return nil
	})

	// Create custom WorkerPool with test queue names
	wp := NewWorkerPool(3, redisStore, reg, 1, 2)
	wp.queueReady = "test:queue:ready"
	wp.queueScheduled = "test:queue:scheduled"
	wp.queueDead = "test:queue:dead"

	// Enqueue 10 tasks
	totalTasks := 10
	for i := 1; i <= totalTasks; i++ {
		task := &model.Task{
			ID:             fmt.Sprintf("task-%d", i),
			Type:           "test_task",
			IdempotencyKey: fmt.Sprintf("ik-%d", i),
			Attempts:       0,
			MaxRetries:     1,
		}
		data, err := json.Marshal(task)
		require.NoError(t, err)

		err = redisStore.LPush(ctx, wp.queueReady, string(data))
		require.NoError(t, err)
	}

	// Verify enqueued
	initialLen, err := redisStore.LLen(ctx, wp.queueReady)
	require.NoError(t, err)
	t.Logf("Enqueued %d tasks (queue length: %d)", totalTasks, initialLen)
	assert.Equal(t, int64(totalTasks), initialLen)

	// Start processing
	ctxRun, cancel := context.WithCancel(context.Background())
	wp.Start(ctxRun)

	// Let some tasks be processed
	time.Sleep(1000 * time.Millisecond)

	processed1 := atomic.LoadInt32(&processed)
	t.Logf("After 300ms: %d tasks processed", processed1)

	// Cancel during processing
	t.Log("=== CANCELLING ===")
	cancel()

	// Graceful shutdown
	shutdownStart := time.Now()
	wp.Stop()
	shutdownDuration := time.Since(shutdownStart)
	t.Logf("Shutdown took %v", shutdownDuration)

	// Check final state
	processedFinal := atomic.LoadInt32(&processed)
	remainingInQueue, err := redisStore.LLen(ctx, wp.queueReady)
	require.NoError(t, err)

	t.Logf("Final state:")
	t.Logf("  - Processed: %d", processedFinal)
	t.Logf("  - Remaining in queue: %d", remainingInQueue)

	processedMu.Lock()
	t.Logf("  - Processed task IDs: %v", processedTasks)
	processedMu.Unlock()

	// Verify no task loss
	totalAccountedFor := processedFinal + int32(remainingInQueue)
	t.Logf("Accounted for: %d / %d", totalAccountedFor, totalTasks)

	// All tasks should be either processed or back in queue
	assert.Equal(t, int32(totalTasks), totalAccountedFor,
		"Task loss detected! Some tasks were neither processed nor returned to queue")

	// Shutdown should be reasonably fast (not hang)
	assert.Less(t, shutdownDuration, 30*time.Second,
		"Shutdown took too long - possible deadlock")
}

func TestGracefulShutdown_RealRedis_HighLoad_NoTaskLoss(t *testing.T) {
	// Test with high volume of tasks
	redisStore := store.NewConsumerRedis("localhost:6379", "", 0, 5)
	err := redisStore.Ping(context.Background())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisStore.Close()

	ctx := context.Background()
	redisStore.Del(ctx, "test:queue:high", "test:queue:scheduled", "test:queue:dead")

	reg := NewRegistry()
	var processed int32

	reg.Register("high_load_task", func(ctx context.Context, task *model.Task) error {
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&processed, 1)
		return nil
	})

	wp := NewWorkerPool(10, redisStore, reg, 1, 5)
	wp.queueReady = "test:queue:high"
	wp.queueScheduled = "test:queue:scheduled"
	wp.queueDead = "test:queue:dead"

	// Enqueue 100 tasks
	totalTasks := 100
	for i := 1; i <= totalTasks; i++ {
		task := &model.Task{
			ID:             fmt.Sprintf("task-%d", i),
			Type:           "high_load_task",
			IdempotencyKey: fmt.Sprintf("ik-%d", i),
			Attempts:       0,
			MaxRetries:     3,
		}
		data, _ := json.Marshal(task)
		redisStore.LPush(ctx, wp.queueReady, string(data))
	}

	t.Logf("Enqueued %d tasks", totalTasks)

	// Start processing
	ctxRun, cancel := context.WithCancel(context.Background())
	wp.Start(ctxRun)

	// Let process for a bit
	time.Sleep(400 * time.Millisecond)
	processed1 := atomic.LoadInt32(&processed)
	t.Logf("After 400ms: %d tasks processed", processed1)

	// Cancel
	cancel()

	// Shutdown
	shutdownStart := time.Now()
	wp.Stop()
	shutdownDuration := time.Since(shutdownStart)

	// Verify
	processedFinal := atomic.LoadInt32(&processed)
	remaining, _ := redisStore.LLen(ctx, wp.queueReady)

	t.Logf("Final: %d processed, %d remaining", processedFinal, remaining)
	t.Logf("Shutdown duration: %v", shutdownDuration)

	totalAccountedFor := processedFinal + int32(remaining)
	assert.Equal(t, int32(totalTasks), totalAccountedFor,
		"High load test: Task loss detected")
	assert.Less(t, shutdownDuration, 60*time.Second,
		"High load shutdown took too long")
}

func TestGracefulShutdown_RealRedis_FastCancellation(t *testing.T) {
	// Test cancellation before tasks even reach workers
	redisStore := store.NewConsumerRedis("localhost:6379", "", 0, 2)
	err := redisStore.Ping(context.Background())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisStore.Close()

	ctx := context.Background()
	redisStore.Del(ctx, "test:queue:fast", "test:queue:scheduled", "test:queue:dead")

	reg := NewRegistry()
	var processed int32

	reg.Register("fast_cancel_task", func(ctx context.Context, task *model.Task) error {
		atomic.AddInt32(&processed, 1)
		return nil
	})

	wp := NewWorkerPool(2, redisStore, reg, 1, 1)
	wp.queueReady = "test:queue:fast"
	wp.queueScheduled = "test:queue:scheduled"
	wp.queueDead = "test:queue:dead"

	// Enqueue 5 tasks
	totalTasks := 5
	for i := 1; i <= totalTasks; i++ {
		task := &model.Task{
			ID:             fmt.Sprintf("task-%d", i),
			Type:           "fast_cancel_task",
			IdempotencyKey: fmt.Sprintf("ik-%d", i),
		}
		data, _ := json.Marshal(task)
		redisStore.LPush(ctx, wp.queueReady, string(data))
	}

	// Start and immediately cancel
	ctxRun, cancel := context.WithCancel(context.Background())
	wp.Start(ctxRun)

	// Cancel almost immediately (before processing completes)
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Shutdown
	wp.Stop()

	// Verify
	processedFinal := atomic.LoadInt32(&processed)
	remaining, _ := redisStore.LLen(ctx, wp.queueReady)
	totalAccountedFor := processedFinal + int32(remaining)

	t.Logf("Fast cancel: %d processed, %d remaining", processedFinal, remaining)

	assert.Equal(t, int32(totalTasks), totalAccountedFor,
		"Fast cancellation test: Task loss detected")
}

func TestGracefulShutdown_RealRedis_WithFailures_NoTaskLoss(t *testing.T) {
	// Test with failing tasks that get retried
	redisStore := store.NewConsumerRedis("localhost:6379", "", 0, 2)
	err := redisStore.Ping(context.Background())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisStore.Close()

	ctx := context.Background()
	redisStore.Del(ctx, "test:queue:fail", "test:queue:scheduled", "test:queue:dead")

	reg := NewRegistry()
	var processed int32
	var failCount int32

	reg.Register("fail_task", func(ctx context.Context, task *model.Task) error {
		if atomic.AddInt32(&failCount, 1) <= 2 {
			return errors.New("simulated failure")
		}
		atomic.AddInt32(&processed, 1)
		return nil
	})

	wp := NewWorkerPool(3, redisStore, reg, 1, 2)
	wp.queueReady = "test:queue:fail"
	wp.queueScheduled = "test:queue:scheduled"
	wp.queueDead = "test:queue:dead"

	// Enqueue 5 tasks
	totalTasks := 5
	for i := 1; i <= totalTasks; i++ {
		task := &model.Task{
			ID:             fmt.Sprintf("task-%d", i),
			Type:           "fail_task",
			IdempotencyKey: fmt.Sprintf("ik-%d", i),
			Attempts:       0,
			MaxRetries:     3,
		}
		data, _ := json.Marshal(task)
		redisStore.LPush(ctx, wp.queueReady, string(data))
	}

	ctxRun, cancel := context.WithCancel(context.Background())
	wp.Start(ctxRun)

	time.Sleep(300 * time.Millisecond)

	cancel()
	wp.Stop()

	processedFinal := atomic.LoadInt32(&processed)
	remaining, _ := redisStore.LLen(ctx, wp.queueReady)
	inScheduled, _ := redisStore.ZCount(ctx, wp.queueScheduled, "-inf", "+inf")
	inDLQ, _ := redisStore.LLen(ctx, wp.queueDead)

	t.Logf("Failed task scenario:")
	t.Logf("  - Processed successfully: %d", processedFinal)
	t.Logf("  - Remaining in ready queue: %d", remaining)
	t.Logf("  - In scheduled (retry): %d", inScheduled)
	t.Logf("  - In DLQ: %d", inDLQ)

	totalAccountedFor := int64(processedFinal) + remaining + inScheduled + inDLQ
	assert.Equal(t, int32(totalTasks), totalAccountedFor,
		"Failed task scenario: Task loss detected")
}
