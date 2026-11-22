package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/database"
)

// Events go to an organization channel when tenant org context is active, or a
// per-user channel otherwise, so one tenant never receives another tenant's
// request logs, credentials, or routing decisions.
const (
	userEventsChannelPrefix = "gateway:events:user:"
	orgEventsChannelPrefix  = "gateway:events:org:"
	eventHistorySuffix      = ":history"
	eventHistoryMaxLen      = 100
	eventHistoryTTL         = 5 * time.Minute
)

// ErrNoEventOwner is returned for an event published without a user or org.
var ErrNoEventOwner = errors.New("event has no owner")

// UserEventsChannel is the Redis channel carrying the given user's events.
func UserEventsChannel(userID string) string {
	return userEventsChannelPrefix + userID
}

// OrgEventsChannel is the Redis channel carrying the given organization's events.
func OrgEventsChannel(orgID string) string {
	return orgEventsChannelPrefix + orgID
}

func resolveEventsChannel(ctx context.Context, userID string) string {
	if orgID := strings.TrimSpace(database.TenantContextFrom(ctx).OrgID); orgID != "" {
		return OrgEventsChannel(orgID)
	}
	return UserEventsChannel(userID)
}

type Event struct {
	ID        string      `json:"id,omitempty"`
	OrgID     string      `json:"orgId,omitempty"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

type EventPublisher struct {
	rdb *goredis.Client
}

func NewEventPublisher(rdb *goredis.Client) *EventPublisher {
	return &EventPublisher{rdb: rdb}
}

// Publish sends an event to the active organization's stream (when orgID is
// present in ctx) or userID's stream, and records it in a short-TTL ring buffer
// for Last-Event-ID replay.
func (p *EventPublisher) Publish(ctx context.Context, userID, eventType string, data interface{}) error {
	if p == nil {
		return nil
	}
	orgID := strings.TrimSpace(database.TenantContextFrom(ctx).OrgID)
	if userID == "" && orgID == "" {
		return ErrNoEventOwner
	}
	if p.rdb == nil {
		return nil
	}
	now := time.Now().UTC()
	event := Event{
		ID:        fmt.Sprintf("%d-%s", now.UnixNano(), uuid.NewString()[:8]),
		OrgID:     orgID,
		Type:      eventType,
		Data:      data,
		Timestamp: now,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	channel := resolveEventsChannel(ctx, userID)
	histKey := channel + eventHistorySuffix
	pipe := p.rdb.Pipeline()
	pipe.LPush(ctx, histKey, payload)
	pipe.LTrim(ctx, histKey, 0, eventHistoryMaxLen-1)
	pipe.Expire(ctx, histKey, eventHistoryTTL)
	pipe.Publish(ctx, channel, payload)
	_, err = pipe.Exec(ctx)
	return err
}

// Subscribe listens to the active organization's events (when orgID is in ctx)
// or userID's events.
func (p *EventPublisher) Subscribe(ctx context.Context, userID string) *goredis.PubSub {
	if p == nil || p.rdb == nil {
		return nil
	}
	return p.rdb.Subscribe(ctx, resolveEventsChannel(ctx, userID))
}

// ReplaySince returns buffered JSON payloads published after lastEventID on the
// caller's resolved tenant event channel, in chronological order.
func (p *EventPublisher) ReplaySince(ctx context.Context, userID, lastEventID string) ([]string, error) {
	lastEventID = strings.TrimSpace(lastEventID)
	if p == nil || p.rdb == nil || lastEventID == "" {
		return nil, nil
	}
	histKey := resolveEventsChannel(ctx, userID) + eventHistorySuffix
	items, err := p.rdb.LRange(ctx, histKey, 0, eventHistoryMaxLen-1).Result()
	if err != nil || len(items) == 0 {
		return nil, err
	}
	// items are newest-first (LPUSH); find lastEventID and return everything newer in oldest-first order.
	matchIdx := -1
	for i, raw := range items {
		var evt struct {
			ID string `json:"id"`
		}
		if json.Unmarshal([]byte(raw), &evt) == nil && evt.ID == lastEventID {
			matchIdx = i
			break
		}
	}
	if matchIdx <= 0 {
		return nil, nil
	}
	out := make([]string, 0, matchIdx)
	for i := matchIdx - 1; i >= 0; i-- {
		out = append(out, items[i])
	}
	return out, nil
}
