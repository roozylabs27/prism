package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	budgetInFlightPrefix = "prism:budget:inflight:"
	budgetExpPrefix      = "prism:budget:exp:"
	budgetSettledPrefix  = "prism:budget:settled:"
	budgetRevPrefix      = "prism:budget:rev:"

	defaultReservationTTL = 10 * time.Minute
	defaultSettledTTL     = 5 * time.Second
)

// ErrDistributedBudgetUnavailable is returned when Redis is unreachable or not configured.
var ErrDistributedBudgetUnavailable = errors.New("distributed budget store unavailable")

// BudgetReserveInput describes a multi-replica atomic budget reservation request.
type BudgetReserveInput struct {
	ScopeKey      string
	ReservationID string
	EstimatedCost float64
	MonthlySpent  float64
	DailySpent    float64
	MonthlyLimit  float64
	DailyLimit    float64
	HardStop      bool
	TTL           time.Duration
}

// BudgetReserveResult holds the outcome of an atomic Lua budget reservation.
type BudgetReserveResult struct {
	Allowed        bool
	InFlight       float64
	RecentSettled  float64
	Revision       int64
	ReservationID  string
	ExceededPeriod string
}

// BudgetSnapshot returns current distributed in-flight total, short-TTL settled delta, and revision.
type BudgetSnapshot struct {
	InFlight      float64
	RecentSettled float64
	Revision      int64
}

// BudgetReservationStore coordinates in-flight and short-TTL settled budget reservations across
// multiple API replicas using atomic Redis Lua scripts.
type BudgetReservationStore struct {
	rdb        *goredis.Client
	settledTTL time.Duration
}

// NewBudgetReservationStore constructs a Redis-backed distributed budget reservation store.
func NewBudgetReservationStore(rdb *goredis.Client) *BudgetReservationStore {
	return &BudgetReservationStore{
		rdb:        rdb,
		settledTTL: defaultSettledTTL,
	}
}

func budgetKeys(scopeKey string) []string {
	return []string{
		budgetInFlightPrefix + scopeKey,
		budgetExpPrefix + scopeKey,
		budgetSettledPrefix + scopeKey,
		budgetRevPrefix + scopeKey,
	}
}

func newReservationID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// KEYS[1] = inflight hash (resID -> floatStr)
// KEYS[2] = inflight expiry zset (resID -> expireUnixMs)
// KEYS[3] = recent settled zset ("resID:floatStr" -> expireUnixMs)
// KEYS[4] = revision integer key
//
// ARGV[1] = resID
// ARGV[2] = estCost (float)
// ARGV[3] = nowMs (int)
// ARGV[4] = expMs (int)
// ARGV[5] = monthlySpent
// ARGV[6] = dailySpent
// ARGV[7] = monthlyLimit
// ARGV[8] = dailyLimit
// ARGV[9] = hardStop ("1" or "0")
// ARGV[10] = keyTTLMs (int)
//
// Returns {allowed (1|0), inflightStr, settledStr, revInt, reasonStr}.
var reserveSpendScript = goredis.NewScript(`
local hashKey = KEYS[1]
local expKey = KEYS[2]
local settledKey = KEYS[3]
local revKey = KEYS[4]

local resID = ARGV[1]
local estCost = tonumber(ARGV[2]) or 0
local nowMs = tonumber(ARGV[3]) or 0
local expMs = tonumber(ARGV[4]) or 0
local monthlySpent = tonumber(ARGV[5]) or 0
local dailySpent = tonumber(ARGV[6]) or 0
local monthlyLimit = tonumber(ARGV[7]) or 0
local dailyLimit = tonumber(ARGV[8]) or 0
local hardStop = ARGV[9] == "1"
local keyTTLMs = tonumber(ARGV[10]) or 600000

local expired = redis.call("ZRANGEBYSCORE", expKey, "-inf", nowMs)
for _, id in ipairs(expired) do
	redis.call("HDEL", hashKey, id)
	redis.call("ZREM", expKey, id)
end

redis.call("ZREMRANGEBYSCORE", settledKey, "-inf", nowMs)

local inflight = 0
local vals = redis.call("HVALS", hashKey)
for _, v in ipairs(vals) do
	local n = tonumber(v) or 0
	if n > 0 then
		inflight = inflight + n
	end
end

local settled = 0
local smembers = redis.call("ZRANGE", settledKey, 0, -1)
for _, m in ipairs(smembers) do
	local idx = string.find(m, ":")
	if idx then
		local n = tonumber(string.sub(m, idx + 1)) or 0
		if n > 0 then
			settled = settled + n
		end
	end
end

local rev = tonumber(redis.call("GET", revKey) or "0") or 0

if hardStop then
	local currentMonthly = monthlySpent + settled + inflight
	local projectedMonthly = currentMonthly + estCost
	if monthlyLimit > 0 and (currentMonthly >= monthlyLimit or (estCost > 0 and projectedMonthly > monthlyLimit + 1e-12)) then
		return {0, string.format("%.8f", inflight), string.format("%.8f", settled), rev, "monthly"}
	end
	local currentDaily = dailySpent + settled + inflight
	local projectedDaily = currentDaily + estCost
	if dailyLimit > 0 and (currentDaily >= dailyLimit or (estCost > 0 and projectedDaily > dailyLimit + 1e-12)) then
		return {0, string.format("%.8f", inflight), string.format("%.8f", settled), rev, "daily"}
	end
end

if estCost > 0 then
	redis.call("HSET", hashKey, resID, ARGV[2])
	redis.call("ZADD", expKey, expMs, resID)
	redis.call("PEXPIRE", hashKey, keyTTLMs)
	redis.call("PEXPIRE", expKey, keyTTLMs)
	inflight = inflight + estCost
end

return {1, string.format("%.8f", inflight), string.format("%.8f", settled), rev, ""}
`)

// KEYS[1] = inflight hash
// KEYS[2] = inflight expiry zset
// KEYS[3] = recent settled zset
// KEYS[4] = revision integer key
//
// ARGV[1] = resID
// ARGV[2] = actualCost
// ARGV[3] = settledExpMs
// ARGV[4] = settledTTLMs
// ARGV[5] = nowMs
//
// Returns new revision integer.
var releaseSpendScript = goredis.NewScript(`
local hashKey = KEYS[1]
local expKey = KEYS[2]
local settledKey = KEYS[3]
local revKey = KEYS[4]

local resID = ARGV[1]
local actualCost = tonumber(ARGV[2]) or 0
local settledExpMs = tonumber(ARGV[3]) or 0
local settledTTLMs = tonumber(ARGV[4]) or 0
local nowMs = tonumber(ARGV[5]) or 0

redis.call("HDEL", hashKey, resID)
redis.call("ZREM", expKey, resID)
redis.call("ZREMRANGEBYSCORE", settledKey, "-inf", nowMs)

if actualCost > 0 and settledTTLMs > 0 then
	local member = resID .. ":" .. string.format("%.8f", actualCost)
	redis.call("ZADD", settledKey, settledExpMs, member)
	redis.call("PEXPIRE", settledKey, settledTTLMs + 5000)
end

local rev = redis.call("INCR", revKey)
redis.call("EXPIRE", revKey, 3600)
return rev
`)

// KEYS[1] = inflight hash
// KEYS[2] = inflight expiry zset
// KEYS[3] = recent settled zset
// KEYS[4] = revision integer key
//
// ARGV[1] = nowMs
//
// Returns {inflightStr, settledStr, revInt}.
var inFlightGetScript = goredis.NewScript(`
local hashKey = KEYS[1]
local expKey = KEYS[2]
local settledKey = KEYS[3]
local revKey = KEYS[4]
local nowMs = tonumber(ARGV[1]) or 0

local expired = redis.call("ZRANGEBYSCORE", expKey, "-inf", nowMs)
for _, id in ipairs(expired) do
	redis.call("HDEL", hashKey, id)
	redis.call("ZREM", expKey, id)
end

redis.call("ZREMRANGEBYSCORE", settledKey, "-inf", nowMs)

local inflight = 0
local vals = redis.call("HVALS", hashKey)
for _, v in ipairs(vals) do
	local n = tonumber(v) or 0
	if n > 0 then
		inflight = inflight + n
	end
end

local settled = 0
local smembers = redis.call("ZRANGE", settledKey, 0, -1)
for _, m in ipairs(smembers) do
	local idx = string.find(m, ":")
	if idx then
		local n = tonumber(string.sub(m, idx + 1)) or 0
		if n > 0 then
			settled = settled + n
		end
	end
end

local rev = tonumber(redis.call("GET", revKey) or "0") or 0
return {string.format("%.8f", inflight), string.format("%.8f", settled), rev}
`)

// TryReserve atomically validates limits across active distributed in-flight and
// recent settled reservations and records the new reservation when allowed.
func (s *BudgetReservationStore) TryReserve(ctx context.Context, in BudgetReserveInput) (*BudgetReserveResult, error) {
	if s == nil || s.rdb == nil {
		return nil, ErrDistributedBudgetUnavailable
	}
	if strings.TrimSpace(in.ScopeKey) == "" {
		return nil, fmt.Errorf("scopeKey is required")
	}

	resID := in.ReservationID
	if resID == "" {
		resID = newReservationID()
	}
	ttl := in.TTL
	if ttl <= 0 {
		ttl = defaultReservationTTL
	}
	nowMs := time.Now().UnixMilli()
	expMs := nowMs + ttl.Milliseconds()
	keyTTLMs := (ttl + time.Minute).Milliseconds()

	hardStopArg := "0"
	if in.HardStop {
		hardStopArg = "1"
	}

	res, err := reserveSpendScript.Run(
		ctx,
		s.rdb,
		budgetKeys(in.ScopeKey),
		resID,
		strconv.FormatFloat(in.EstimatedCost, 'f', 8, 64),
		strconv.FormatInt(nowMs, 10),
		strconv.FormatInt(expMs, 10),
		strconv.FormatFloat(in.MonthlySpent, 'f', 8, 64),
		strconv.FormatFloat(in.DailySpent, 'f', 8, 64),
		strconv.FormatFloat(in.MonthlyLimit, 'f', 8, 64),
		strconv.FormatFloat(in.DailyLimit, 'f', 8, 64),
		hardStopArg,
		strconv.FormatInt(keyTTLMs, 10),
	).Slice()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDistributedBudgetUnavailable, err)
	}
	if len(res) < 5 {
		return nil, fmt.Errorf("unexpected lua reserve result length %d", len(res))
	}

	allowed := toInt64(res[0]) == 1
	inflight, _ := strconv.ParseFloat(fmt.Sprint(res[1]), 64)
	settled, _ := strconv.ParseFloat(fmt.Sprint(res[2]), 64)
	rev := toInt64(res[3])
	reason := fmt.Sprint(res[4])

	return &BudgetReserveResult{
		Allowed:        allowed,
		InFlight:       inflight,
		RecentSettled:  settled,
		Revision:       rev,
		ReservationID:  resID,
		ExceededPeriod: reason,
	}, nil
}

// Release removes an active reservation from Redis, records short-lived optimistic
// settled cost so concurrent replicas see it until Postgres flush completes,
// and bumps the scope revision counter.
func (s *BudgetReservationStore) Release(ctx context.Context, scopeKey, reservationID string, actualCost float64) (int64, error) {
	if s == nil || s.rdb == nil {
		return 0, ErrDistributedBudgetUnavailable
	}
	if strings.TrimSpace(scopeKey) == "" || strings.TrimSpace(reservationID) == "" {
		return 0, nil
	}

	nowMs := time.Now().UnixMilli()
	settledTTLMs := s.settledTTL.Milliseconds()
	settledExpMs := nowMs + settledTTLMs

	rev, err := releaseSpendScript.Run(
		ctx,
		s.rdb,
		budgetKeys(scopeKey),
		reservationID,
		strconv.FormatFloat(actualCost, 'f', 8, 64),
		strconv.FormatInt(settledExpMs, 10),
		strconv.FormatInt(settledTTLMs, 10),
		strconv.FormatInt(nowMs, 10),
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrDistributedBudgetUnavailable, err)
	}
	return rev, nil
}

// InFlightAndOptimistic returns the current distributed in-flight sum,
// short-TTL settled sum, and monotonically increasing revision counter.
func (s *BudgetReservationStore) InFlightAndOptimistic(ctx context.Context, scopeKey string) (*BudgetSnapshot, error) {
	if s == nil || s.rdb == nil {
		return nil, ErrDistributedBudgetUnavailable
	}
	if strings.TrimSpace(scopeKey) == "" {
		return &BudgetSnapshot{}, nil
	}

	nowMs := time.Now().UnixMilli()
	res, err := inFlightGetScript.Run(
		ctx,
		s.rdb,
		budgetKeys(scopeKey),
		strconv.FormatInt(nowMs, 10),
	).Slice()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDistributedBudgetUnavailable, err)
	}
	if len(res) < 3 {
		return nil, fmt.Errorf("unexpected lua snapshot result length %d", len(res))
	}

	inflight, _ := strconv.ParseFloat(fmt.Sprint(res[0]), 64)
	settled, _ := strconv.ParseFloat(fmt.Sprint(res[1]), 64)
	rev := toInt64(res[2])

	return &BudgetSnapshot{
		InFlight:      inflight,
		RecentSettled: settled,
		Revision:      rev,
	}, nil
}

func toInt64(v interface{}) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		n, _ := strconv.ParseInt(fmt.Sprint(x), 10, 64)
		return n
	}
}
