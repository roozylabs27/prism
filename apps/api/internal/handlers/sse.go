package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	goredis "github.com/roozylabs/prism/internal/redis"
)

type SSEHandler struct {
	publisher *goredis.EventPublisher
	closing   chan struct{}
	closeOnce sync.Once
}

func NewSSEHandler(publisher *goredis.EventPublisher) *SSEHandler {
	return &SSEHandler{publisher: publisher, closing: make(chan struct{})}
}

// Shutdown ends every open event stream. Streams never finish on their own,
// so the server calls this when it starts shutting down; otherwise a graceful
// shutdown would wait on dashboard tabs until it times out. Browsers'
// EventSource reconnects to the new instance.
func (h *SSEHandler) Shutdown() {
	h.closeOnce.Do(func() { close(h.closing) })
}

// Stream godoc
// @Summary      SSE event stream
// @Description  Real-time Server-Sent Events stream for live dashboard updates
// @Tags         sse
// @Security     BearerAuth
// @Success      200 {string} string "SSE stream"
// @Router       /api/sse [get]
func (h *SSEHandler) Stream(c *gin.Context) {
	// Each stream carries only its user's events; without a user there is
	// nothing it may receive.
	userID := c.GetString("userId")
	if userID == "" {
		httputil.RespondUnauthorized(c, "Missing user for event stream", nil, "AUTH_USER_MISSING")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Transfer-Encoding", "chunked")
	c.Status(http.StatusOK)

	// Send initial connected event immediately
	if _, err := fmt.Fprint(c.Writer, "event: connected\ndata: {\"status\":\"ok\"}\n\n"); err != nil {
		return
	}
	c.Writer.Flush()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	ctx := middleware.ContextWithTenant(c)
	lastEventID := strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	if lastEventID == "" {
		lastEventID = strings.TrimSpace(c.Query("lastEventId"))
	}

	var pubsub *redis.PubSub
	var ch <-chan *redis.Message
	if h.publisher != nil {
		pubsub = h.publisher.Subscribe(ctx, userID)
		if pubsub != nil {
			defer func() { _ = pubsub.Close() }()
			ch = pubsub.Channel()
		}
		if lastEventID != "" {
			if missed, err := h.publisher.ReplaySince(ctx, userID, lastEventID); err == nil {
				for _, payload := range missed {
					if !writeSSEPayload(c, payload) {
						return
					}
				}
			}
		}
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.closing:
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(c.Writer, ": ping\nevent: ping\ndata: {\"status\":\"ok\"}\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case msg, ok := <-ch:
			if !ok {
				// Redis subscription channel was closed or disconnected; wait briefly without blocking shutdown/cancellation.
				if pubsub != nil {
					_ = pubsub.Close()
				}
				select {
				case <-c.Request.Context().Done():
					return
				case <-h.closing:
					return
				case <-time.After(1 * time.Second):
				}
				if h.publisher != nil {
					pubsub = h.publisher.Subscribe(ctx, userID)
					if pubsub != nil {
						ch = pubsub.Channel()
					}
				}
				continue
			}
			if !writeSSEPayload(c, msg.Payload) {
				return
			}
		}
	}
}

func writeSSEPayload(c *gin.Context, payload string) bool {
	var meta struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal([]byte(payload), &meta)
	var err error
	if meta.ID != "" {
		_, err = fmt.Fprintf(c.Writer, "id: %s\nevent: message\ndata: %s\n\n", meta.ID, payload)
	} else {
		_, err = fmt.Fprintf(c.Writer, "event: message\ndata: %s\n\n", payload)
	}
	if err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}
