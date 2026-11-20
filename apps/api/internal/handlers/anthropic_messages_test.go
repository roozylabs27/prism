package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubGatewayKeyFinder struct {
	key *models.GatewayAPIKey
}

func (s *stubGatewayKeyFinder) FindByKeyHash(_ context.Context, _ string) (*models.GatewayAPIKey, error) {
	return s.key, nil
}

func TestGatewayAuthMiddleware_AcceptsXApiKeyHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.GatewayAuthMiddleware(&stubGatewayKeyFinder{
		key: &models.GatewayAPIKey{ID: "gk-ant", UserID: "user-1"},
	}))
	r.POST("/v1/messages", func(c *gin.Context) {
		k := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
		c.JSON(http.StatusOK, gin.H{"id": k.ID})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", "prism_sk_test_1234567890")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "gk-ant")
}

func TestTranslateAnthropicRequestToProxy(t *testing.T) {
	rawBody := `{
		"model": "claude-3-7-sonnet",
		"max_tokens": 1024,
		"temperature": 0,
		"system": [{"type": "text", "text": "You are Claude Code."}],
		"tool_choice": {"type": "tool", "name": "read_file"},
		"tools": [{"name": "read_file", "description": "Read file", "input_schema": {"type": "object"}}],
		"messages": [
			{"role": "user", "content": "Read main.go"},
			{"role": "assistant", "content": [
				{"type": "text", "text": "Opening file."},
				{"type": "tool_use", "id": "toolu_1", "name": "read_file", "input": {"path": "main.go"}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_1", "content": [{"type": "text", "text": "package main"}]},
				{"type": "text", "text": "Now summarize it"}
			]}
		]
	}`

	req, err := translateAnthropicRequestToProxy([]byte(rawBody))
	require.NoError(t, err)
	assert.Equal(t, "claude-3-7-sonnet", req.Model)
	require.NotNil(t, req.Temperature)
	assert.Equal(t, 0.0, *req.Temperature)
	assert.Equal(t, map[string]interface{}{
		"type":     "function",
		"function": map[string]interface{}{"name": "read_file"},
	}, req.ToolChoice)

	require.Len(t, req.Messages, 5)
	assert.Equal(t, "system", req.Messages[0]["role"])
	assert.Equal(t, "You are Claude Code.", req.Messages[0]["content"])
	assert.Equal(t, "user", req.Messages[1]["role"])
	assert.Equal(t, "assistant", req.Messages[2]["role"])
	tcs := req.Messages[2]["tool_calls"].([]interface{})
	require.Len(t, tcs, 1)
	assert.Equal(t, "tool", req.Messages[3]["role"])
	assert.Equal(t, "toolu_1", req.Messages[3]["tool_call_id"])
	assert.Equal(t, "package main", req.Messages[3]["content"])
	assert.Equal(t, "user", req.Messages[4]["role"])
	assert.Equal(t, "Now summarize it", req.Messages[4]["content"])
}

func TestTranslateProxyResponseToAnthropic(t *testing.T) {
	resp := &proxy.ProviderResponse{
		ID:    "chatcmpl-99",
		Model: "claude-3-7-sonnet",
		Choices: []proxy.Choice{{
			Index:        0,
			FinishReason: "tool_calls",
			Message: map[string]interface{}{
				"role":    "assistant",
				"content": "Let me check.",
				"tool_calls": []interface{}{
					map[string]interface{}{
						"id":   "call_77",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "bash",
							"arguments": `{"cmd":"ls"}`,
						},
					},
				},
			},
		}},
		Usage: proxy.Usage{PromptTokens: 40, CompletionTokens: 15, TotalTokens: 55},
	}

	out := translateProxyResponseToAnthropic(resp, "claude-3-7-sonnet", "req-1")
	assert.Equal(t, "message", out["type"])
	assert.Equal(t, "tool_use", out["stop_reason"])

	blocks := out["content"].([]map[string]interface{})
	require.Len(t, blocks, 2)
	assert.Equal(t, "text", blocks[0]["type"])
	assert.Equal(t, "Let me check.", blocks[0]["text"])
	assert.Equal(t, "tool_use", blocks[1]["type"])
	assert.Equal(t, "call_77", blocks[1]["id"])
	assert.Equal(t, "bash", blocks[1]["name"])
	assert.Equal(t, map[string]interface{}{"cmd": "ls"}, blocks[1]["input"])
}

func TestAnthropicSSETranslatingWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	w := newAnthropicSSETranslatingWriter(c.Writer, "req-stream-1", "claude-3-7-sonnet")

	chunks := []string{
		": ping\n\n",
		`data: {"id":"1","model":"claude-3-7-sonnet","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"Step 1"}}],"usage":{"prompt_tokens":18}}` + "\n\n",
		`data: {"id":"1","choices":[{"index":0,"delta":{"content":"Hello "}}]}` + "\n\n",
		`data: {"id":"1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"grep","arguments":"{\"q\":"}}]}}]}` + "\n\n",
		`data: {"id":"1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"prism\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"completion_tokens":33}}` + "\n\n",
		"data: [DONE]\n\n",
	}

	for _, ch := range chunks {
		_, err := w.Write([]byte(ch))
		require.NoError(t, err)
	}

	out := rec.Body.String()
	for _, want := range []string{
		"event: message_start",
		"event: ping",
		`"thinking_delta"`,
		`"text_delta"`,
		`"type":"tool_use"`,
		`"input_json_delta"`,
		`\"prism\"`,
		`"stop_reason":"tool_use"`,
		`"output_tokens":33`,
		"event: message_stop",
	} {
		assert.Contains(t, out, want)
	}
	assert.Equal(t, 1, strings.Count(out, "event: message_start"))
	assert.Equal(t, 1, strings.Count(out, "event: message_stop"))

	// Ensure every payload line still unmarshals cleanly after event: prefix.
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "data: ") {
			var parsed map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(l, "data: ")), &parsed), l)
		}
	}
}
