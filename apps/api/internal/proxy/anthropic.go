package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const defaultAnthropicMaxTokens = 4096

type AnthropicAdapter struct {
	blockToToolIndex map[int]int
	nextToolIndex    int
}

func NewAnthropicAdapter() *AnthropicAdapter {
	return &AnthropicAdapter{
		blockToToolIndex: make(map[int]int),
	}
}

func extractTextFromContent(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		return v
	case []interface{}:
		var sb strings.Builder
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, _ := m["type"].(string); t == "text" {
					if txt, ok := m["text"].(string); ok {
						if sb.Len() > 0 {
							sb.WriteString("\n")
						}
						sb.WriteString(txt)
					}
				}
			}
		}
		return sb.String()
	case []map[string]interface{}:
		var sb strings.Builder
		for _, m := range v {
			if t, _ := m["type"].(string); t == "text" {
				if txt, ok := m["text"].(string); ok {
					if sb.Len() > 0 {
						sb.WriteString("\n")
					}
					sb.WriteString(txt)
				}
			}
		}
		return sb.String()
	default:
		return ""
	}
}

func stripGoogleSignatures(raw interface{}) interface{} {
	switch v := raw.(type) {
	case map[string]interface{}:
		cleaned := make(map[string]interface{}, len(v))
		for k, val := range v {
			if k == "thought_signature" || k == "thoughtSignature" {
				continue
			}
			cleaned[k] = stripGoogleSignatures(val)
		}
		return cleaned
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = stripGoogleSignatures(item)
		}
		return out
	case []map[string]interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = stripGoogleSignatures(item)
		}
		return out
	default:
		return raw
	}
}

func toAnthropicContentBlocks(content interface{}) []interface{} {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []interface{}{map[string]interface{}{"type": "text", "text": v}}
	case []interface{}:
		out := make([]interface{}, 0, len(v))
		for _, item := range v {
			out = append(out, stripGoogleSignatures(item))
		}
		return out
	case []map[string]interface{}:
		out := make([]interface{}, 0, len(v))
		for _, item := range v {
			out = append(out, stripGoogleSignatures(item))
		}
		return out
	default:
		return nil
	}
}

func appendUserBlocks(messages []map[string]interface{}, blocks []interface{}) []map[string]interface{} {
	if len(blocks) == 0 {
		return messages
	}
	if n := len(messages); n > 0 && messages[n-1]["role"] == "user" {
		existing := toAnthropicContentBlocks(messages[n-1]["content"])
		existing = append(existing, blocks...)
		messages[n-1]["content"] = existing
		return messages
	}
	return append(messages, map[string]interface{}{
		"role":    "user",
		"content": blocks,
	})
}

func normalizeToolCallsList(raw interface{}) []map[string]interface{} {
	switch tcs := raw.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(tcs))
		for _, item := range tcs {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]interface{}:
		return tcs
	default:
		return nil
	}
}

func convertMessagesToAnthropic(rawMsgs []map[string]interface{}) (string, []map[string]interface{}) {
	var systemParts []string
	var out []map[string]interface{}

	for _, msg := range rawMsgs {
		role, _ := msg["role"].(string)
		switch role {
		case "system":
			if txt := strings.TrimSpace(extractTextFromContent(msg["content"])); txt != "" {
				systemParts = append(systemParts, txt)
			}

		case "tool":
			callID, _ := msg["tool_call_id"].(string)
			block := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": callID,
				"content":     msg["content"],
			}
			if isErr, ok := msg["is_error"].(bool); ok && isErr {
				block["is_error"] = true
			}
			out = appendUserBlocks(out, []interface{}{block})

		case "assistant":
			toolCalls := normalizeToolCallsList(msg["tool_calls"])
			if len(toolCalls) > 0 {
				var blocks []interface{}
				if txt := extractTextFromContent(msg["content"]); txt != "" {
					blocks = append(blocks, map[string]interface{}{
						"type": "text",
						"text": txt,
					})
				}
				for _, tc := range toolCalls {
					fn, _ := tc["function"].(map[string]interface{})
					if fn == nil {
						continue
					}
					id, _ := tc["id"].(string)
					name, _ := fn["name"].(string)
					inputObj := map[string]interface{}{}
					switch args := fn["arguments"].(type) {
					case string:
						if strings.TrimSpace(args) != "" {
							_ = json.Unmarshal([]byte(args), &inputObj)
						}
					case map[string]interface{}:
						inputObj = args
					}
					blocks = append(blocks, map[string]interface{}{
						"type":  "tool_use",
						"id":    id,
						"name":  name,
						"input": inputObj,
					})
				}
				if len(blocks) > 0 {
					out = append(out, map[string]interface{}{
						"role":    "assistant",
						"content": blocks,
					})
				}
				continue
			}

			out = append(out, map[string]interface{}{
				"role":    "assistant",
				"content": stripGoogleSignatures(msg["content"]),
			})

		default:
			if role == "" {
				role = "user"
			}
			if role == "user" && len(out) > 0 && out[len(out)-1]["role"] == "user" {
				out = appendUserBlocks(out, toAnthropicContentBlocks(msg["content"]))
				continue
			}
			out = append(out, map[string]interface{}{
				"role":    role,
				"content": stripGoogleSignatures(msg["content"]),
			})
		}
	}

	return strings.Join(systemParts, "\n\n"), out
}

func translateToolChoiceToAnthropic(tc interface{}) interface{} {
	switch v := tc.(type) {
	case string:
		switch v {
		case "auto":
			return map[string]interface{}{"type": "auto"}
		case "required", "any":
			return map[string]interface{}{"type": "any"}
		case "none":
			return map[string]interface{}{"type": "none"}
		}
	case map[string]interface{}:
		if t, _ := v["type"].(string); t == "auto" || t == "any" || t == "none" || t == "tool" {
			return v
		}
		if t, _ := v["type"].(string); t == "function" {
			if fn, ok := v["function"].(map[string]interface{}); ok {
				if name, ok := fn["name"].(string); ok && name != "" {
					return map[string]interface{}{"type": "tool", "name": name}
				}
			}
		}
	}
	return nil
}

func (a *AnthropicAdapter) BuildRequest(baseURL, apiKey string, req *ProviderRequest) (*http.Request, error) {
	systemPrompt, messages := convertMessagesToAnthropic(req.Messages)

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultAnthropicMaxTokens
	}

	body := map[string]interface{}{
		"model":      req.Model,
		"messages":   messages,
		"max_tokens": maxTokens,
		"stream":     req.Stream,
	}
	if len(req.Tools) > 0 {
		openAiTools := ConvertGenericToolsToOpenAI(req.Tools)
		body["tools"] = ConvertOpenAIToolsToAnthropic(openAiTools)
	}
	if req.ToolChoice != nil {
		if converted := translateToolChoiceToAnthropic(req.ToolChoice); converted != nil {
			body["tool_choice"] = converted
		}
	}
	if systemPrompt != "" {
		body["system"] = systemPrompt
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	for k, v := range req.Extra {
		switch k {
		case "stream_options", "response_format", "n", "frequency_penalty", "presence_penalty", "logprobs", "top_logprobs", "seed", "user":
			// Skip OpenAI-only parameters that Anthropic rejects as unknown fields.
			continue
		default:
			if k == "system" && systemPrompt != "" {
				continue
			}
			body[k] = v
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	return httpReq, nil
}

func (a *AnthropicAdapter) ParseResponse(body io.Reader) (*ProviderResponse, error) {
	var raw struct {
		ID         string        `json:"id"`
		Model      string        `json:"model"`
		Content    []interface{} `json:"content"`
		StopReason string        `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, err
	}

	resp := &ProviderResponse{
		ID:    raw.ID,
		Model: raw.Model,
		Usage: Usage{
			PromptTokens:     raw.Usage.InputTokens,
			CompletionTokens: raw.Usage.OutputTokens,
			TotalTokens:      raw.Usage.InputTokens + raw.Usage.OutputTokens,
		},
	}

	if raw.Error != nil {
		resp.Error = &ProviderError{
			Type:    raw.Error.Type,
			Message: raw.Error.Message,
		}
		return resp, nil
	}

	var textContent string
	for _, block := range raw.Content {
		if blockMap, ok := block.(map[string]interface{}); ok {
			if blockType, _ := blockMap["type"].(string); blockType == "text" {
				if t, ok := blockMap["text"].(string); ok {
					textContent += t
				}
			}
		}
	}

	toolCalls := NormalizeAnthropicToolUse(raw.Content)

	msgMap := map[string]interface{}{
		"role":    "assistant",
		"content": textContent,
	}
	if len(toolCalls) > 0 {
		msgMap["tool_calls"] = toolCalls
	}

	finishReason := mapAnthropicStopReasonToOpenAI(raw.StopReason)
	if len(toolCalls) > 0 && (finishReason == "" || finishReason == "stop") {
		finishReason = "tool_calls"
	}

	resp.Choices = append(resp.Choices, Choice{
		Index:        0,
		Message:      msgMap,
		FinishReason: finishReason,
	})

	return resp, nil
}

func mapAnthropicStopReasonToOpenAI(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		if reason != "" {
			return reason
		}
		return "stop"
	}
}

func (a *AnthropicAdapter) ParseStreamChunk(line []byte) (*ProviderResponse, bool) {
	lineStr := strings.TrimSpace(string(line))
	if !strings.HasPrefix(lineStr, "data:") {
		return nil, false
	}
	data := strings.TrimSpace(strings.TrimPrefix(lineStr, "data:"))

	var event struct {
		Type         string          `json:"type"`
		Index        int             `json:"index"`
		Delta        json.RawMessage `json:"delta"`
		ContentBlock *struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"content_block"`
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage struct {
				InputTokens int `json:"input_tokens"`
			} `json:"usage"`
		} `json:"message"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil, false
	}

	switch event.Type {
	case "message_start":
		return &ProviderResponse{
			ID:    event.Message.ID,
			Model: event.Message.Model,
			Usage: Usage{PromptTokens: event.Message.Usage.InputTokens},
		}, false

	case "error":
		msg := event.Error.Message
		if msg == "" {
			msg = "upstream reported an error"
		}
		return &ProviderResponse{
			Error: &ProviderError{Type: event.Error.Type, Message: msg},
		}, false

	case "content_block_start":
		if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
			if a.blockToToolIndex == nil {
				a.blockToToolIndex = make(map[int]int)
			}
			toolIdx := a.nextToolIndex
			a.nextToolIndex++
			a.blockToToolIndex[event.Index] = toolIdx
			return &ProviderResponse{
				Choices: []Choice{{
					Index: 0,
					Delta: map[string]interface{}{
						"role": "assistant",
						"tool_calls": []interface{}{
							map[string]interface{}{
								"index": float64(toolIdx),
								"id":    event.ContentBlock.ID,
								"type":  "function",
								"function": map[string]interface{}{
									"name":      event.ContentBlock.Name,
									"arguments": "",
								},
							},
						},
					},
				}},
			}, false
		}
		return nil, false

	case "content_block_delta":
		var delta struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			Thinking    string `json:"thinking"`
			PartialJSON string `json:"partial_json"`
		}
		_ = json.Unmarshal(event.Delta, &delta)
		switch delta.Type {
		case "thinking_delta":
			return &ProviderResponse{
				Choices: []Choice{{
					Index: 0,
					Delta: map[string]interface{}{
						"role":              "assistant",
						"reasoning_content": delta.Thinking,
					},
				}},
			}, false
		case "input_json_delta":
			toolIdx := event.Index
			if a.blockToToolIndex != nil {
				if mapped, ok := a.blockToToolIndex[event.Index]; ok {
					toolIdx = mapped
				}
			}
			return &ProviderResponse{
				Choices: []Choice{{
					Index: 0,
					Delta: map[string]interface{}{
						"tool_calls": []interface{}{
							map[string]interface{}{
								"index": float64(toolIdx),
								"function": map[string]interface{}{
									"arguments": delta.PartialJSON,
								},
							},
						},
					},
				}},
			}, false
		default:
			return &ProviderResponse{
				Choices: []Choice{{
					Index: 0,
					Delta: map[string]interface{}{
						"role":    "assistant",
						"content": delta.Text,
					},
				}},
			}, false
		}

	case "message_delta":
		var delta struct {
			StopReason string `json:"stop_reason"`
		}
		_ = json.Unmarshal(event.Delta, &delta)
		return &ProviderResponse{
			Choices: []Choice{{
				Index:        0,
				FinishReason: mapAnthropicStopReasonToOpenAI(delta.StopReason),
			}},
			Usage: Usage{PromptTokens: event.Usage.InputTokens, CompletionTokens: event.Usage.OutputTokens},
		}, false

	case "message_stop":
		return nil, true

	default:
		return nil, false
	}
}

func (a *AnthropicAdapter) SupportsStreaming() bool {
	return true
}
