package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type OpenAIResponsesAdapter struct {
	itemToToolIndex map[string]int
	nextToolIndex   int
}

func NewOpenAIResponsesAdapter() *OpenAIResponsesAdapter {
	return &OpenAIResponsesAdapter{
		itemToToolIndex: make(map[string]int),
	}
}

func convertToolsToOpenAIResponses(rawTools []interface{}) []map[string]interface{} {
	openAITools := ConvertGenericToolsToOpenAI(rawTools)
	if len(openAITools) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(openAITools))
	for _, t := range openAITools {
		fn, _ := t["function"].(map[string]interface{})
		if fn == nil {
			out = append(out, t)
			continue
		}
		item := map[string]interface{}{
			"type": "function",
			"name": fn["name"],
		}
		if desc, _ := fn["description"].(string); desc != "" {
			item["description"] = desc
		}
		if params := fn["parameters"]; params != nil {
			item["parameters"] = params
		}
		out = append(out, item)
	}
	return out
}

func convertMessagesToResponsesInput(rawMsgs []map[string]interface{}) (string, interface{}) {
	var instrParts []string
	var nonSystem []map[string]interface{}

	for _, msg := range rawMsgs {
		role, _ := msg["role"].(string)
		if role == "system" {
			if txt := strings.TrimSpace(extractTextFromContent(msg["content"])); txt != "" {
				instrParts = append(instrParts, txt)
			}
			continue
		}
		nonSystem = append(nonSystem, msg)
	}

	instructions := strings.Join(instrParts, "\n\n")

	if len(nonSystem) == 1 && nonSystem[0]["tool_calls"] == nil {
		if role, _ := nonSystem[0]["role"].(string); role == "user" || role == "" {
			if content, ok := nonSystem[0]["content"].(string); ok {
				return instructions, content
			}
		}
	}

	items := make([]map[string]interface{}, 0, len(nonSystem))
	for _, msg := range nonSystem {
		role, _ := msg["role"].(string)
		switch role {
		case "tool":
			callID, _ := msg["tool_call_id"].(string)
			output := extractTextFromContent(msg["content"])
			if output == "" && msg["content"] != nil {
				if b, err := json.Marshal(msg["content"]); err == nil {
					output = string(b)
				}
			}
			items = append(items, map[string]interface{}{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  output,
			})

		case "assistant":
			toolCalls := normalizeToolCallsList(msg["tool_calls"])
			if txt := extractTextFromContent(msg["content"]); txt != "" || len(toolCalls) == 0 {
				items = append(items, map[string]interface{}{
					"role":    "assistant",
					"content": msg["content"],
				})
			}
			for _, tc := range toolCalls {
				fn, _ := tc["function"].(map[string]interface{})
				if fn == nil {
					continue
				}
				callID, _ := tc["id"].(string)
				name, _ := fn["name"].(string)
				args := "{}"
				switch a := fn["arguments"].(type) {
				case string:
					if a != "" {
						args = a
					}
				case map[string]interface{}:
					if b, err := json.Marshal(a); err == nil {
						args = string(b)
					}
				}
				items = append(items, map[string]interface{}{
					"type":      "function_call",
					"call_id":   callID,
					"name":      name,
					"arguments": args,
				})
			}

		default:
			if role == "" {
				role = "user"
			}
			items = append(items, map[string]interface{}{
				"role":    role,
				"content": msg["content"],
			})
		}
	}

	return instructions, items
}

func (a *OpenAIResponsesAdapter) BuildRequest(baseURL, apiKey string, req *ProviderRequest) (*http.Request, error) {
	instructions, input := convertMessagesToResponsesInput(req.Messages)

	body := map[string]interface{}{
		"model":  req.Model,
		"input":  input,
		"stream": req.Stream,
	}
	if instructions != "" {
		body["instructions"] = instructions
	}
	if len(req.Tools) > 0 {
		if converted := convertToolsToOpenAIResponses(req.Tools); len(converted) > 0 {
			body["tools"] = converted
		}
	}
	if req.ToolChoice != nil {
		body["tool_choice"] = req.ToolChoice
	}
	if req.MaxTokens > 0 {
		body["max_output_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	for k, v := range req.Extra {
		if k == "stream_options" {
			continue
		}
		body[k] = v
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/responses"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	return httpReq, nil
}

func (a *OpenAIResponsesAdapter) ParseResponse(body io.Reader) (*ProviderResponse, error) {
	var raw struct {
		ID     string `json:"id"`
		Model  string `json:"model"`
		Output []struct {
			Type      string `json:"type"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, err
	}

	total := raw.Usage.TotalTokens
	if total == 0 {
		total = raw.Usage.InputTokens + raw.Usage.OutputTokens
	}

	resp := &ProviderResponse{
		ID:    raw.ID,
		Model: raw.Model,
		Usage: Usage{
			PromptTokens:     raw.Usage.InputTokens,
			CompletionTokens: raw.Usage.OutputTokens,
			TotalTokens:      total,
		},
	}

	if raw.Error != nil {
		resp.Error = &ProviderError{
			Type:    raw.Error.Type,
			Message: raw.Error.Message,
			Code:    raw.Error.Code,
		}
		return resp, nil
	}

	var textContent string
	var toolCalls []map[string]interface{}
	for _, item := range raw.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" || c.Type == "text" {
					textContent += c.Text
				}
			}
		case "function_call":
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   item.CallID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      item.Name,
					"arguments": item.Arguments,
				},
			})
		}
	}

	msg := map[string]interface{}{
		"role":    "assistant",
		"content": textContent,
	}
	finishReason := "stop"
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
		finishReason = "tool_calls"
	}

	resp.Choices = append(resp.Choices, Choice{
		Index:        0,
		Message:      msg,
		FinishReason: finishReason,
	})

	return resp, nil
}

func decodeResponsesDeltaString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		Delta string `json:"delta"`
	}
	_ = json.Unmarshal(raw, &obj)
	return obj.Delta
}

func (a *OpenAIResponsesAdapter) ParseStreamChunk(line []byte) (*ProviderResponse, bool) {
	lineStr := strings.TrimSpace(string(line))
	if !strings.HasPrefix(lineStr, "data:") {
		return nil, false
	}
	data := strings.TrimSpace(strings.TrimPrefix(lineStr, "data:"))
	if data == "[DONE]" {
		return nil, true
	}

	var event struct {
		Type        string          `json:"type"`
		Delta       json.RawMessage `json:"delta"`
		ItemID      string          `json:"item_id"`
		OutputIndex int             `json:"output_index"`
		Item        *struct {
			Type   string `json:"type"`
			ID     string `json:"id"`
			CallID string `json:"call_id"`
			Name   string `json:"name"`
		} `json:"item"`
		Response *struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
				TotalTokens  int `json:"total_tokens"`
			} `json:"usage"`
			Error *struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		} `json:"response"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil, false
	}

	switch event.Type {
	case "response.output_text.delta":
		return &ProviderResponse{
			Choices: []Choice{{
				Index: 0,
				Delta: map[string]interface{}{
					"role":    "assistant",
					"content": decodeResponsesDeltaString(event.Delta),
				},
			}},
		}, false

	case "response.output_item.added":
		if event.Item != nil && event.Item.Type == "function_call" {
			if a.itemToToolIndex == nil {
				a.itemToToolIndex = make(map[string]int)
			}
			toolIdx := a.nextToolIndex
			a.nextToolIndex++
			if event.Item.ID != "" {
				a.itemToToolIndex[event.Item.ID] = toolIdx
			}
			callID := event.Item.CallID
			if callID == "" {
				callID = event.Item.ID
			}
			return &ProviderResponse{
				Choices: []Choice{{
					Index: 0,
					Delta: map[string]interface{}{
						"role": "assistant",
						"tool_calls": []interface{}{
							map[string]interface{}{
								"index": float64(toolIdx),
								"id":    callID,
								"type":  "function",
								"function": map[string]interface{}{
									"name":      event.Item.Name,
									"arguments": "",
								},
							},
						},
					},
				}},
			}, false
		}
		return nil, false

	case "response.function_call_arguments.delta":
		toolIdx := event.OutputIndex
		if event.ItemID != "" && a.itemToToolIndex != nil {
			if mapped, ok := a.itemToToolIndex[event.ItemID]; ok {
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
								"arguments": decodeResponsesDeltaString(event.Delta),
							},
						},
					},
				},
			}},
		}, false

	case "response.completed":
		var inTok, outTok, totTok int
		var respID, respModel string
		if event.Response != nil {
			respID = event.Response.ID
			respModel = event.Response.Model
			inTok = event.Response.Usage.InputTokens
			outTok = event.Response.Usage.OutputTokens
			totTok = event.Response.Usage.TotalTokens
		} else {
			var completed struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			_ = json.Unmarshal(event.Delta, &completed)
			inTok = completed.Usage.InputTokens
			outTok = completed.Usage.OutputTokens
		}
		if totTok == 0 {
			totTok = inTok + outTok
		}
		return &ProviderResponse{
			ID:    respID,
			Model: respModel,
			Usage: Usage{
				PromptTokens:     inTok,
				CompletionTokens: outTok,
				TotalTokens:      totTok,
			},
		}, false

	case "response.failed", "error":
		var errType, errMsg, errCode string
		if event.Error != nil {
			errType = event.Error.Type
			errMsg = event.Error.Message
			errCode = event.Error.Code
		} else if event.Response != nil && event.Response.Error != nil {
			errType = event.Response.Error.Type
			errMsg = event.Response.Error.Message
			errCode = event.Response.Error.Code
		}
		if errMsg == "" {
			errMsg = "upstream responses stream failed"
		}
		return &ProviderResponse{
			Error: &ProviderError{Type: errType, Message: errMsg, Code: errCode},
		}, false

	default:
		return nil, false
	}
}

func (a *OpenAIResponsesAdapter) SupportsStreaming() bool {
	return true
}
