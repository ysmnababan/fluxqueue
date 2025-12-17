// Package store provides Redis-backed queue operations for task management, scheduling, and DLQ handling.
package store

import (
	"context"
	"errors"
	"fluxqueue/internal/model"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a thin wrapper around go-redis for queue operations.
type RedisStore struct {
	client *redis.Client
}

// NewRedisClient creates a new RedisStore.
func NewRedisClient(addr, pwd string, db int) *RedisStore {
	rdb := redis.NewClient(
		&redis.Options{
			Addr:     addr,
			Password: pwd,
			DB:       db,
		},
	)
	return &RedisStore{
		client: rdb,
	}
}

// Ping tests the connection.
func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the underlying redis client.
func (r *RedisStore) Close() error {
	return r.client.Close()
}

// ---------------------- LIST (queue) operations -------------------------

// LPush pushes a value to the head (left) of the list.
func (r *RedisStore) LPush(ctx context.Context, key string, value string) error {
	return r.client.LPush(ctx, key, value).Err()
}

// RPush pushes a value to the tail (right) of the list.
func (r *RedisStore) RPush(ctx context.Context, key string, value string) error {
	return r.client.RPush(ctx, key, value).Err()
}

// LLen returns the length of the list.
func (r *RedisStore) LLen(ctx context.Context, key string) (int64, error) {
	return r.client.LLen(ctx, key).Result()
}

// BRPop performs a blocking right pop with timeout. It returns the popped value (not the key).
// If timeout is 0, call will block indefinitely until an element is available.
func (r *RedisStore) BRPop(ctx context.Context, timeout time.Duration, keys ...string) (string, error) {
	res, err := r.client.BRPop(ctx, timeout, keys...).Result()
	if err != nil {
		return "", err
	}
	// BRPop returns a two-element slice: [key, value]
	if len(res) < 2 {
		return "", fmt.Errorf("unexpected brpop response: %#v", res)
	}
	return res[1], nil
}

// ZPopMin removes and returns the element with the lowest score.
// Count = 1 means pop a single item.
func (r *RedisStore) ZPopMin(ctx context.Context, key string) ([]model.ZItem, error) {
	res, err := r.client.ZPopMin(ctx, key, 1).Result()
	if err != nil {
		return nil, err
	}

	items := make([]model.ZItem, len(res))
	for i, v := range res {
		memberStr, ok := v.Member.(string)
		if !ok {
			return nil, errors.New("field is not string")
		}
		items[i] = model.ZItem{
			Member: memberStr,
			Score:  v.Score,
		}
	}

	return items, nil
}

// ---------------------- ZSET (scheduled) operations --------------------

// ZAdd adds a member with score to a sorted set.
func (r *RedisStore) ZAdd(ctx context.Context, key string, score float64, member string) error {
	z := redis.Z{Score: score, Member: member}
	return r.client.ZAdd(ctx, key, z).Err()
}

// ZRangeByScore returns members in the zset with score between min and max (inclusive).
func (r *RedisStore) ZRangeByScore(ctx context.Context, key string, min, max string) ([]string, error) {
	opt := &redis.ZRangeBy{
		Min: min,
		Max: max,
	}
	return r.client.ZRangeByScore(ctx, key, opt).Result()
}

// ZRem removes members from the sorted set.
func (r *RedisStore) ZRem(ctx context.Context, key string, members ...string) (int64, error) {
	return r.client.ZRem(ctx, key, members).Result()
}

// ZCount returns number of members in score range
func (r *RedisStore) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return r.client.ZCount(ctx, key, min, max).Result()
}

// ---------------------- Atomic move: scheduled -> ready ----------------

// MoveScheduledToReady atomically moves all members with score <= maxScore from the scheduled zset to the ready list.
// It returns the number of moved items.
// Note: maxScore should be a string representing the score (e.g. "1700000000000" as unix milliseconds).
func (r *RedisStore) MoveScheduledToReady(ctx context.Context, zsetKey, readyListKey, maxScore string) (int64, error) {
	// Lua script: get items, remove from zset, push to list (LPUSH)
	// It returns the number of moved items.
	const lua = `
local items = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
if #items == 0 then
  return 0
end
for i=1,#items do
  redis.call('ZREM', KEYS[1], items[i])
  redis.call('LPUSH', KEYS[2], items[i])
end
return #items
`
	res, err := r.client.Eval(ctx, lua, []string{zsetKey, readyListKey}, maxScore).Result()
	if err != nil {
		return 0, err
	}
	switch v := res.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unexpected result type from lua script: %T %#v", res, res)
	}
}

// MoveOneScheduledToReady atomically moves up to `limit` items with score <= maxScore from zset -> list and returns moved members.
// Useful to limit how many tasks scheduler moves per iteration.
func (r *RedisStore) MoveOneScheduledToReady(ctx context.Context, zsetKey, readyListKey, maxScore string, limit int64) ([]string, error) {
	// This variant fetches up to `limit` members, removes them and pushes to list.
	const lua = `
local items = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, tonumber(ARGV[2]))
if #items == 0 then
  return {}
end
for i=1,#items do
  redis.call('ZREM', KEYS[1], items[i])
  redis.call('LPUSH', KEYS[2], items[i])
end
return items
`
	res, err := r.client.Eval(ctx, lua, []string{zsetKey, readyListKey}, maxScore, fmt.Sprintf("%d", limit)).Result()
	if err != nil {
		return nil, err
	}
	// result should be []interface{} convertible to []string
	if arr, ok := res.([]interface{}); ok {
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("unexpected result type from lua script: %T %#v", res, res)
}

// ---------------------- DLQ helpers ------------------------------------

// PushToDLQ pushes the given task string to the DLQ list.
func (r *RedisStore) PushToDLQ(ctx context.Context, dlqKey, taskJSON string) error {
	return r.client.LPush(ctx, dlqKey, taskJSON).Err()
}

// PopFromDLQ pops one item from the DLQ (non-blocking). Returns "" if none.
func (r *RedisStore) PopFromDLQ(ctx context.Context, dlqKey string) (string, error) {
	res, err := r.client.RPop(ctx, dlqKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return res, nil
}

// ---------------------- Idempotency & processing marker ----------------

// SetNX sets key if not exists with TTL. Returns true if set.
func (r *RedisStore) SetNX(ctx context.Context, key string, val string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, val, ttl).Result()
}

// Get returns string value for key.
func (r *RedisStore) Get(ctx context.Context, key string) (string, error) {
	res, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return res, err
}

// Set sets key with TTL.
func (r *RedisStore) Set(ctx context.Context, key string, val string, ttl time.Duration) error {
	return r.client.Set(ctx, key, val, ttl).Err()
}

// Del deletes keys.
func (r *RedisStore) Del(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Del(ctx, keys...).Result()
}

// SetProcessing creates a processing marker for a task (e.g. processing:<taskID>) with TTL.
// Returns true if set.
func (r *RedisStore) SetProcessing(ctx context.Context, processingKey, value string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, processingKey, value, ttl).Result()
}

// ExtendProcessingTTL extends TTL for a processing key.
func (r *RedisStore) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.client.Expire(ctx, key, ttl).Result()
}

// ---------------------- HASH helpers (metadata) -------------------------

// HSet sets multiple fields in a hash.
func (r *RedisStore) HSet(ctx context.Context, key string, values map[string]interface{}) error {
	return r.client.HSet(ctx, key, values).Err()
}

// HGetAll returns all fields in a hash.
func (r *RedisStore) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// ---------------------- Utility helpers --------------------------------

// ZScore returns score of a member.
func (r *RedisStore) ZScore(ctx context.Context, key string, member string) (float64, error) {
	return r.client.ZScore(ctx, key, member).Result()
}
