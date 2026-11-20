package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	scopeStreamKeyPrefix   = "prism:streams:scope:"
	defaultScopeStreamTTL  = 10 * time.Minute
)

func scopeStreamKey(scopeKey string) string {
	return scopeStreamKeyPrefix + scopeKey
}

// KEYS[1] = scope stream zset (reqID -> expireUnixMs)
// ARGV[1] = nowMs
// ARGV[2] = reqID
// ARGV[3] = limit
// ARGV[4] = expMs
// ARGV[5] = keyTTLMs
// Returns {allowed (1|0), activeCount}
var acquireScopeStreamScript = goredis.NewScript(`
local key = KEYS[1]
local nowMs = tonumber(ARGV[1]) or 0
local reqID = ARGV[2]
local limit = tonumber(ARGV[3]) or 0
local expMs = tonumber(ARGV[4]) or 0
local keyTTLMs = tonumber(ARGV[5]) or 600000

redis.call("ZREMRANGEBYSCORE", key, "-inf", nowMs)

if redis.call("ZSCORE", key, reqID) then
	redis.call("ZADD", key, expMs, reqID)
	redis.call("PEXPIRE", key, keyTTLMs)
	return {1, redis.call("ZCARD", key)}
end

local active = redis.call("ZCARD", key)
if limit > 0 and active >= limit then
	return {0, active}
end

redis.call("ZADD", key, expMs, reqID)
redis.call("PEXPIRE", key, keyTTLMs)
return {1, active + 1}
`)

// AcquireScopeStream enforces an atomic concurrent stream cap across replicas for a quota scope
// using an auto-expiring ZSET lease per request ID.
func (s *CooldownStore) AcquireScopeStream(ctx context.Context, scopeKey, reqID string, limit int, ttl time.Duration) (bool, int64, error) {
	if s == nil || s.rdb == nil {
		return false, 0, fmt.Errorf("redis client unavailable")
	}
	if strings.TrimSpace(scopeKey) == "" || strings.TrimSpace(reqID) == "" || limit <= 0 {
		return true, 0, nil
	}
	if ttl <= 0 {
		ttl = defaultScopeStreamTTL
	}
	nowMs := time.Now().UnixMilli()
	expMs := nowMs + ttl.Milliseconds()
	keyTTLMs := (ttl + time.Minute).Milliseconds()

	res, err := acquireScopeStreamScript.Run(
		ctx,
		s.rdb,
		[]string{scopeStreamKey(scopeKey)},
		strconv.FormatInt(nowMs, 10),
		reqID,
		strconv.Itoa(limit),
		strconv.FormatInt(expMs, 10),
		strconv.FormatInt(keyTTLMs, 10),
	).Slice()
	if err != nil {
		return false, 0, err
	}
	if len(res) < 2 {
		return false, 0, fmt.Errorf("unexpected acquire stream response length %d", len(res))
	}
	return toInt64(res[0]) == 1, toInt64(res[1]), nil
}

// ReleaseScopeStream removes reqID from the distributed scope stream ZSET.
func (s *CooldownStore) ReleaseScopeStream(ctx context.Context, scopeKey, reqID string) error {
	if s == nil || s.rdb == nil || strings.TrimSpace(scopeKey) == "" || strings.TrimSpace(reqID) == "" {
		return nil
	}
	return s.rdb.ZRem(ctx, scopeStreamKey(scopeKey), reqID).Err()
}
