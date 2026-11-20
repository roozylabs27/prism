package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/utils"
)

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicMessagesRequest struct {
	Model         string                 `json:"model"`
	MaxTokens     int                    `json:"max_tokens"`
	Messages      []anthropicMessage     `json:"messages"`
	System        json.RawMessage        `json:"system,omitempty"`
	Tools         []interface{}          `json:"tools,omitempty"`
	ToolChoice    interface{}            `json:"tool_choice,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
	Temperature   *float64               `json:"temperature,omitempty"`
	TopP          *float64               `json:"top_p,omitempty"`
	TopK          *int                   `json:"top_k,omitempty"`
	StopSequences []string               `json:"stop_sequences,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Thinking      map[string]interface{} `json:"thinking,omitempty"`
}

func extractAnthropicSystemPrompt(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	var s string
	if err := json.Unmarshal(trimmed, &s); err == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(trimmed, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if (b.Type == "text" || b.Type == "") && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

func extractAnthropicToolResultContent(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, _ := m["type"].(string); t == "text" || t == "" {
					if txt, ok := m["text"].(string); ok {
						parts = append(parts, txt)
					}
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
	default:
		if v != nil {
			if b, err := json.Marshal(v); err == nil {
				return string(b)
			}
		}
	}
	return ""
}

func translateAnthropicToolChoiceToOpenAI(tc interface{}) interface{} {
	m, ok := tc.(map[string]interface{})
	if !ok {
		return tc
	}
	t, _ := m["type"].(string)
	switch t {
	case "auto":
		return "auto"
	case "any":
		return "required"
	case "none":
		return "none"
	case "tool":
		if name, ok := m["name"].(string); ok && name != "" {
			return map[string]interface{}{
				"type":     "function",
				"function": map[string]interface{}{"name": name},
			}
		}
	}
	return tc
}

func translateAnthropicRequestToProxy(body []byte) (*proxy.ProxyRequest, error) {
	var in anthropicMessagesRequest
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if strings.TrimSpace(in.Model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	if len(in.Messages) == 0 {
		return nil, fmt.Errorf("messages is required and must be non-empty")
	}

	var msgs []map[string]interface{}
	if sys := extractAnthropicSystemPrompt(in.System); sys != "" {
		msgs = append(msgs, map[string]interface{}{
			"role":    "system",
			"content": sys,
		})
	}

	for _, m := range in.Messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		trimmed := bytes.TrimSpace(m.Content)
		var textContent string
		if json.Unmarshal(trimmed, &textContent) == nil {
			msgs = append(msgs, map[string]interface{}{
				"role":    role,
				"content": textContent,
			})
			continue
		}

		var blocks []map[string]interface{}
		if err := json.Unmarshal(trimmed, &blocks); err != nil {
			return nil, fmt.Errorf("invalid message content for role %q", role)
		}

		switch role {
		case "assistant":
			var textParts []string
			var toolCalls []interface{}
			for i, b := range blocks {
				bType, _ := b["type"].(string)
				switch bType {
				case "text":
					if txt, ok := b["text"].(string); ok && txt != "" {
						textParts = append(textParts, txt)
					}
				case "tool_use":
					id, _ := b["id"].(string)
					if id == "" {
						id = fmt.Sprintf("call_%d", i)
					}
					name, _ := b["name"].(string)
					argsBytes, _ := json.Marshal(b["input"])
					if len(argsBytes) == 0 || string(argsBytes) == "null" {
						argsBytes = []byte("{}")
					}
					toolCalls = append(toolCalls, map[string]interface{}{
						"id":   id,
						"type": "function",
						"function": map[string]interface{}{
							"name":      name,
							"arguments": string(argsBytes),
						},
					})
				}
			}
			outMsg := map[string]interface{}{"role": "assistant"}
			joined := strings.Join(textParts, "")
			if joined != "" || len(toolCalls) == 0 {
				outMsg["content"] = joined
			}
			if len(toolCalls) > 0 {
				outMsg["tool_calls"] = toolCalls
			}
			msgs = append(msgs, outMsg)

		case "user":
			var textParts []string
			var nonTextBlocks []interface{}
			for _, b := range blocks {
				bType, _ := b["type"].(string)
				if bType == "tool_result" {
					callID, _ := b["tool_use_id"].(string)
					toolMsg := map[string]interface{}{
						"role":         "tool",
						"tool_call_id": callID,
						"content":      extractAnthropicToolResultContent(b["content"]),
					}
					if isErr, ok := b["is_error"].(bool); ok && isErr {
						toolMsg["is_error"] = true
					}
					msgs = append(msgs, toolMsg)
					continue
				}
				if bType == "text" {
					if txt, ok := b["text"].(string); ok && txt != "" {
						textParts = append(textParts, txt)
					}
					continue
				}
				nonTextBlocks = append(nonTextBlocks, b)
			}
			if len(nonTextBlocks) > 0 {
				combined := make([]interface{}, 0, len(textParts)+len(nonTextBlocks))
				for _, txt := range textParts {
					combined = append(combined, map[string]interface{}{"type": "text", "text": txt})
				}
				combined = append(combined, nonTextBlocks...)
				msgs = append(msgs, map[string]interface{}{"role": "user", "content": combined})
			} else if len(textParts) > 0 {
				msgs = append(msgs, map[string]interface{}{"role": "user", "content": strings.Join(textParts, "\n")})
			}

		default:
			msgs = append(msgs, map[string]interface{}{"role": role, "content": blocks})
		}
	}

	extra := make(map[string]interface{})
	if in.TopP != nil {
		extra["top_p"] = *in.TopP
	}
	if in.TopK != nil {
		extra["top_k"] = *in.TopK
	}
	if len(in.StopSequences) > 0 {
		extra["stop"] = in.StopSequences
	}
	if in.Thinking != nil {
		extra["thinking"] = in.Thinking
	}
	if in.Metadata != nil {
		extra["metadata"] = in.Metadata
	}

	return &proxy.ProxyRequest{
		Model:       in.Model,
		Messages:    msgs,
		Tools:       in.Tools,
		ToolChoice:  translateAnthropicToolChoiceToOpenAI(in.ToolChoice),
		Stream:      in.Stream,
		MaxTokens:   in.MaxTokens,
		Temperature: in.Temperature,
		Extra:       extra,
	}, nil
}

func mapOpenAIFinishReasonToAnthropic(reason string, hasTools bool) string {
	switch reason {
	case "tool_calls", "function_call":
		return "tool_use"
	case "length":
		return "max_tokens"
	case "stop", "":
		if hasTools {
			return "tool_use"
		}
		return "end_turn"
	default:
		return reason
	}
}

func extractToolCallsFromMessage(raw interface{}) []map[string]interface{} {
	switch list := raw.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(list))
		for _, item := range list {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]interface{}:
		return list
	default:
		return nil
	}
}

func translateProxyResponseToAnthropic(resp *proxy.ProviderResponse, reqModel, requestID string) map[string]interface{} {
	msgID := "msg_" + strings.ReplaceAll(requestID, "-", "")
	model := reqModel
	var contentBlocks []map[string]interface{}
	finishReason := ""

	if resp != nil {
		if resp.ID != "" {
			msgID = resp.ID
		}
		if resp.Model != "" {
			model = resp.Model
		}
		if len(resp.Choices) > 0 {
			ch := resp.Choices[0]
			finishReason = ch.FinishReason
			if ch.Message != nil {
				if txt, ok := ch.Message["content"].(string); ok && txt != "" {
					contentBlocks = append(contentBlocks, map[string]interface{}{
						"type": "text",
						"text": txt,
					})
				}
				for i, tc := range extractToolCallsFromMessage(ch.Message["tool_calls"]) {
					fn, _ := tc["function"].(map[string]interface{})
					if fn == nil {
						continue
					}
					id, _ := tc["id"].(string)
					if id == "" {
						id = fmt.Sprintf("toolu_%d", i)
					}
					name, _ := fn["name"].(string)
					inputObj := map[string]interface{}{}
					if argStr, ok := fn["arguments"].(string); ok && strings.TrimSpace(argStr) != "" {
						_ = json.Unmarshal([]byte(argStr), &inputObj)
					}
					contentBlocks = append(contentBlocks, map[string]interface{}{
						"type":  "tool_use",
						"id":    id,
						"name":  name,
						"input": inputObj,
					})
				}
			}
		}
	}

	hasTools := false
	for _, b := range contentBlocks {
		if b["type"] == "tool_use" {
			hasTools = true
			break
		}
	}
	if len(contentBlocks) == 0 {
		contentBlocks = []map[string]interface{}{{"type": "text", "text": ""}}
	}

	var inTokens, outTokens int
	if resp != nil {
		inTokens = resp.Usage.PromptTokens
		outTokens = resp.Usage.CompletionTokens
	}

	return map[string]interface{}{
		"id":            msgID,
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       contentBlocks,
		"stop_reason":   mapOpenAIFinishReasonToAnthropic(finishReason, hasTools),
		"stop_sequence": nil,
		"usage": map[string]interface{}{
			"input_tokens":  inTokens,
			"output_tokens": outTokens,
		},
	}
}

type anthropicSSETranslatingWriter struct {
	gin.ResponseWriter
	mu            sync.Mutex
	lineBuf       bytes.Buffer
	messageID     string
	model         string
	startedMsg    bool
	openBlockIdx  int
	openBlockType string
	nextBlockIdx  int
	activeToolIdx int
	activeToolID  string
	sawToolUse    bool
	finishReason  string
	inputTokens   int
	outputTokens  int
	finished      bool
}

func newAnthropicSSETranslatingWriter(w gin.ResponseWriter, requestID, model string) *anthropicSSETranslatingWriter {
	return &anthropicSSETranslatingWriter{
		ResponseWriter: w,
		messageID:      "msg_" + strings.ReplaceAll(requestID, "-", ""),
		model:          model,
		activeToolIdx:  -1,
	}
}

func (w *anthropicSSETranslatingWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *anthropicSSETranslatingWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *anthropicSSETranslatingWriter) Flush() {
	w.ResponseWriter.Flush()
}

func (w *anthropicSSETranslatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.lineBuf.Write(p); err != nil {
		return 0, err
	}
	for {
		lineBytes, err := w.lineBuf.ReadBytes('\n')
		if err != nil {
			// Put back incomplete trailing line until next chunk arrives.
			w.lineBuf.Reset()
			_, _ = w.lineBuf.Write(lineBytes)
			break
		}
		line := strings.TrimSpace(string(lineBytes))
		if line == "" {
			continue
		}
		if writeErr := w.handleSSELine(line); writeErr != nil {
			return len(p), writeErr
		}
	}
	return len(p), nil
}

func (w *anthropicSSETranslatingWriter) writeSSEEvent(eventName string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventName, data)
	_, err = w.ResponseWriter.Write([]byte(msg))
	return err
}

func (w *anthropicSSETranslatingWriter) ensureMessageStart() error {
	if w.startedMsg {
		return nil
	}
	w.startedMsg = true
	return w.writeSSEEvent("message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            w.messageID,
			"type":          "message",
			"role":          "assistant",
			"model":         w.model,
			"content":       []interface{}{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]interface{}{
				"input_tokens":  w.inputTokens,
				"output_tokens": 0,
			},
		},
	})
}

func (w *anthropicSSETranslatingWriter) closeOpenBlock() error {
	if w.openBlockType == "" {
		return nil
	}
	err := w.writeSSEEvent("content_block_stop", map[string]interface{}{
		"type":  "content_block_stop",
		"index": w.openBlockIdx,
	})
	w.openBlockType = ""
	w.activeToolIdx = -1
	w.activeToolID = ""
	return err
}

func (w *anthropicSSETranslatingWriter) handleSSELine(line string) error {
	if strings.HasPrefix(line, ":") {
		if err := w.ensureMessageStart(); err != nil {
			return err
		}
		return w.writeSSEEvent("ping", map[string]interface{}{"type": "ping"})
	}
	if !strings.HasPrefix(line, "data:") {
		return nil
	}
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data == "[DONE]" {
		if w.finished {
			return nil
		}
		w.finished = true
		if err := w.ensureMessageStart(); err != nil {
			return err
		}
		if err := w.closeOpenBlock(); err != nil {
			return err
		}
		if err := w.writeSSEEvent("message_delta", map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason":   mapOpenAIFinishReasonToAnthropic(w.finishReason, w.sawToolUse),
				"stop_sequence": nil,
			},
			"usage": map[string]interface{}{
				"output_tokens": w.outputTokens,
			},
		}); err != nil {
			return err
		}
		return w.writeSSEEvent("message_stop", map[string]interface{}{"type": "message_stop"})
	}

	var chunk proxy.ProviderResponse
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil
	}

	if chunk.Model != "" {
		w.model = chunk.Model
	}
	if chunk.ID != "" && !w.startedMsg {
		w.messageID = chunk.ID
	}
	if chunk.Usage.PromptTokens > 0 {
		w.inputTokens = chunk.Usage.PromptTokens
	}
	if chunk.Usage.CompletionTokens > 0 {
		w.outputTokens = chunk.Usage.CompletionTokens
	}

	if chunk.Error != nil {
		errType := chunk.Error.Type
		if errType == "" {
			errType = "api_error"
		}
		w.finished = true
		return w.writeSSEEvent("error", map[string]interface{}{
			"type": "error",
			"error": map[string]interface{}{
				"type":    errType,
				"message": chunk.Error.Message,
			},
		})
	}

	if err := w.ensureMessageStart(); err != nil {
		return err
	}

	for _, ch := range chunk.Choices {
		if ch.FinishReason != "" {
			w.finishReason = ch.FinishReason
		}
		if ch.Delta == nil {
			continue
		}
		if thinking, ok := ch.Delta["reasoning_content"].(string); ok && thinking != "" {
			if w.openBlockType != "thinking" {
				if err := w.closeOpenBlock(); err != nil {
					return err
				}
				w.openBlockIdx = w.nextBlockIdx
				w.nextBlockIdx++
				w.openBlockType = "thinking"
				if err := w.writeSSEEvent("content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": w.openBlockIdx,
					"content_block": map[string]interface{}{
						"type":     "thinking",
						"thinking": "",
					},
				}); err != nil {
					return err
				}
			}
			if err := w.writeSSEEvent("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": w.openBlockIdx,
				"delta": map[string]interface{}{
					"type":     "thinking_delta",
					"thinking": thinking,
				},
			}); err != nil {
				return err
			}
		}

		if content, ok := ch.Delta["content"].(string); ok && content != "" {
			if w.openBlockType != "text" {
				if err := w.closeOpenBlock(); err != nil {
					return err
				}
				w.openBlockIdx = w.nextBlockIdx
				w.nextBlockIdx++
				w.openBlockType = "text"
				if err := w.writeSSEEvent("content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": w.openBlockIdx,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				}); err != nil {
					return err
				}
			}
			if err := w.writeSSEEvent("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": w.openBlockIdx,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": content,
				},
			}); err != nil {
				return err
			}
		}

		for _, tc := range extractToolCallsFromMessage(ch.Delta["tool_calls"]) {
			idx := 0
			if idxF, ok := tc["index"].(float64); ok {
				idx = int(idxF)
			}
			id, _ := tc["id"].(string)
			fn, _ := tc["function"].(map[string]interface{})
			name := ""
			args := ""
			if fn != nil {
				name, _ = fn["name"].(string)
				args, _ = fn["arguments"].(string)
			}

			isNewTool := w.openBlockType != "tool_use" || idx != w.activeToolIdx || (id != "" && id != w.activeToolID)
			if isNewTool {
				if err := w.closeOpenBlock(); err != nil {
					return err
				}
				w.openBlockIdx = w.nextBlockIdx
				w.nextBlockIdx++
				w.openBlockType = "tool_use"
				w.activeToolIdx = idx
				w.activeToolID = id
				w.sawToolUse = true
				if id == "" {
					id = fmt.Sprintf("toolu_%d", w.openBlockIdx)
				}
				if err := w.writeSSEEvent("content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": w.openBlockIdx,
					"content_block": map[string]interface{}{
						"type":  "tool_use",
						"id":    id,
						"name":  name,
						"input": map[string]interface{}{},
					},
				}); err != nil {
					return err
				}
			}

			if args != "" {
				if err := w.writeSSEEvent("content_block_delta", map[string]interface{}{
					"type":  "content_block_delta",
					"index": w.openBlockIdx,
					"delta": map[string]interface{}{
						"type":         "input_json_delta",
						"partial_json": args,
					},
				}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func respondAnthropicError(c *gin.Context, status int, errType, message string) {
	if errType == "" {
		errType = "invalid_request_error"
	}
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// Messages handles Anthropic Messages API requests (`POST /v1/messages`) for
// native Anthropic SDK and Claude Code integrations.
func (h *GatewayHandler) Messages(c *gin.Context) {
	start := time.Now()
	gatewayKey := c.MustGet("gatewayKey").(*models.GatewayAPIKey)

	requestID := middleware.GetRequestID(c)
	if _, err := uuid.Parse(requestID); err != nil {
		requestID = uuid.New().String()
	}
	c.Set("requestID", requestID)

	if _, hasIdem := c.Get("_idem_key"); hasIdem {
		proxy.WrapBufferedWriter(c)
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		if middleware.IsBodyTooLarge(err) {
			respondAnthropicError(c, http.StatusRequestEntityTooLarge, "invalid_request_error", "Request body is too large")
			return
		}
		respondAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	reqPtr, err := translateAnthropicRequestToProxy(body)
	if err != nil {
		respondAnthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	req := *reqPtr

	if c.GetString("routingPolicyID") == "" && strings.TrimSpace(c.GetHeader("X-Routing-Policy")) != "" {
		if !h.resolveSandboxPolicy(c, gatewayKey) {
			return
		}
	}
	req.RoutingPolicyID = c.GetString("routingPolicyID")

	tenantCtx, err := middleware.ResolveCanonicalTenantContext(c, gatewayKey)
	if err != nil {
		respondAnthropicError(c, http.StatusForbidden, "permission_error", err.Error())
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	clientApp := parseClientApp(userAgent)
	agent := middleware.AgentFromContext(c)
	var agentID, agentName string
	if agent != nil {
		agentID, agentName = agent.ID, agent.Name
	}
	userRole, err := h.resolveMemberRole(c.Request.Context(), gatewayKey, tenantCtx)
	if err != nil {
		respondAnthropicError(c, http.StatusForbidden, "permission_error", err.Error())
		return
	}

	h.applyAgentBoundary(c.Request.Context(), &req, agent, gatewayKey)

	if req.Stream {
		c.Writer = newAnthropicSSETranslatingWriter(c.Writer, requestID, req.Model)
	}

	if h.orchestrator != nil {
		res, err := h.orchestrator.ExecuteChatCompletions(
			c,
			c.Request.Context(),
			&req,
			gatewayKey,
			tenantCtx,
			agentID,
			agentName,
			userRole,
			requestID,
			clientIP,
			userAgent,
			clientApp,
		)
		if err != nil {
			latencyMs := int(time.Since(start).Milliseconds())
			status, errType, _, cleanMsg := utils.CleanUpstreamError(err)
			if !c.Writer.Written() {
				respondAnthropicError(c, status, errType, cleanMsg)
			}
			h.handleProxyError(c, err, gatewayKey, tenantCtx, agentID, &req, requestID, clientIP, userAgent, clientApp, req.Stream, latencyMs)
			return
		}

		if res.Denied {
			errType := "permission_error"
			switch res.HTTPStatus {
			case http.StatusBadRequest:
				errType = "invalid_request_error"
			case http.StatusTooManyRequests:
				errType = "rate_limit_error"
			}
			respondAnthropicError(c, res.HTTPStatus, errType, res.ErrorMessage)
			return
		}

		if req.Stream {
			proxy.SaveIdempotencyResult(c, h.idemStore)
			return
		}

		if res.RequestLog != nil {
			c.Header("X-Request-ID", requestID)
			c.Header("X-Prism-Model", res.RequestLog.Model)
			c.Header("X-Prism-Provider", res.RequestLog.ProviderType)
			c.Header("X-Roozy-Model", res.RequestLog.Model)
			c.Header("X-Roozy-Provider", res.RequestLog.ProviderType)
		}

		if res.Response != nil && res.Response.Error != nil {
			respondAnthropicError(c, http.StatusBadGateway, res.Response.Error.Type, res.Response.Error.Message)
			return
		}

		c.JSON(http.StatusOK, translateProxyResponseToAnthropic(res.Response, req.Model, requestID))
		proxy.SaveIdempotencyResult(c, h.idemStore)
	}
}
