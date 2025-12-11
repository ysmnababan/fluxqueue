package store

import (
	"context"
	"fluxqueue/internal/model"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStore is a thin wrapper around go-redis for queue operations.
type redisStore struct {
	client *redis.Client
}

// NewRedisClient creates a new redisStore.
func NewRedisClient(addr, pwd string, db int) *redisStore {
	rdb := redis.NewClient(
		&redis.Options{
			Addr:     addr,
			Password: pwd,
			DB:       db,
		},
	)
	return &redisStore{
		client: rdb,
	}
}

// Ping tests the connection.
func (r *redisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the underlying redis client.
func (r *redisStore) Close() error {
	return r.client.Close()
}

// ---------------------- LIST (queue) operations -------------------------

// LPush pushes a value to the head (left) of the list.
func (r *redisStore) LPush(ctx context.Context, key string, value string) error {
	return r.client.LPush(ctx, key, value).Err()
}

// RPush pushes a value to the tail (right) of the list.
func (r *redisStore) RPush(ctx context.Context, key string, value string) error {
	return r.client.RPush(ctx, key, value).Err()
}

// LLen returns the length of the list.
func (r *redisStore) LLen(ctx context.Context, key string) (int64, error) {
	return r.client.LLen(ctx, key).Result()
}

// BRPop performs a blocking right pop with timeout. It returns the popped value (not the key).
// If timeout is 0, call will block indefinitely until an element is available.
func (r *redisStore) BRPop(ctx context.Context, timeout time.Duration, keys ...string) (string, error) {
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
func (r *redisStore) ZPopMin(ctx context.Context, key string) ([]model.ZItem, error) {
	res, err := r.client.ZPopMin(ctx, key, 1).Result()
	if err != nil {
		return nil, err
	}

	items := make([]model.ZItem, len(res))
	for i, v := range res {
		memberStr, _ := v.Member.(string)
		items[i] = model.ZItem{
			Member: memberStr,
			Score:  v.Score,
		}
	}

	return items, nil
}

// ---------------------- ZSET (scheduled) operations --------------------

// ZAdd adds a member with score to a sorted set.
func (r *redisStore) ZAdd(ctx context.Context, key string, score float64, member string) error {
	z := redis.Z{Score: score, Member: member}
	return r.client.ZAdd(ctx, key, z).Err()
}

// ZRangeByScore returns members in the zset with score between min and max (inclusive).
func (r *redisStore) ZRangeByScore(ctx context.Context, key string, min, max string) ([]string, error) {
	opt := &redis.ZRangeBy{
		Min: min,
		Max: max,
	}
	return r.client.ZRangeByScore(ctx, key, opt).Result()
}

// ZRem removes members from the sorted set.
func (r *redisStore) ZRem(ctx context.Context, key string, members ...string) (int64, error) {
	return r.client.ZRem(ctx, key, members).Result()
}

// ZCount returns number of members in score range
func (r *redisStore) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return r.client.ZCount(ctx, key, min, max).Result()
}

// ---------------------- Atomic move: scheduled -> ready ----------------

// MoveScheduledToReady atomically moves all members with score <= maxScore from the scheduled zset to the ready list.
// It returns the number of moved items.
// Note: maxScore should be a string representing the score (e.g. "1700000000000" as unix milliseconds).
func (r *redisStore) MoveScheduledToReady(ctx context.Context, zsetKey, readyListKey, maxScore string) (int64, error) {
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
func (r *redisStore) MoveOneScheduledToReady(ctx context.Context, zsetKey, readyListKey, maxScore string, limit int64) ([]string, error) {
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
func (r *redisStore) PushToDLQ(ctx context.Context, dlqKey, taskJSON string) error {
	return r.client.LPush(ctx, dlqKey, taskJSON).Err()
}

// PopFromDLQ pops one item from the DLQ (non-blocking). Returns "" if none.
func (r *redisStore) PopFromDLQ(ctx context.Context, dlqKey string) (string, error) {
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
func (r *redisStore) SetNX(ctx context.Context, key string, val string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, val, ttl).Result()
}

// Get returns string value for key.
func (r *redisStore) Get(ctx context.Context, key string) (string, error) {
	res, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return res, err
}

// Del deletes keys.
func (r *redisStore) Del(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Del(ctx, keys...).Result()
}

// SetProcessing creates a processing marker for a task (e.g. processing:<taskID>) with TTL.
// Returns true if set.
func (r *redisStore) SetProcessing(ctx context.Context, processingKey, value string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, processingKey, value, ttl).Result()
}

// ExtendProcessingTTL extends TTL for a processing key.
func (r *redisStore) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.client.Expire(ctx, key, ttl).Result()
}

// ---------------------- HASH helpers (metadata) -------------------------

// HSet sets multiple fields in a hash.
func (r *redisStore) HSet(ctx context.Context, key string, values map[string]interface{}) error {
	return r.client.HSet(ctx, key, values).Err()
}

// HGetAll returns all fields in a hash.
func (r *redisStore) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// ---------------------- Utility helpers --------------------------------

// ZScore returns score of a member.
func (r *redisStore) ZScore(ctx context.Context, key string, member string) (float64, error) {
	return r.client.ZScore(ctx, key, member).Result()
}
