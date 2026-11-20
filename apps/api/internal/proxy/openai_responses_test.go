package proxy

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestOpenAIResponsesAdapter_ParseResponse(t *testing.T) {
	adapter := NewOpenAIResponsesAdapter()

	tax := `{
		"id": "resp_abc123",
		"model": "gpt-5.6-luna",
		"output": [
			{
				"type": "message",
				"content": [
					{
						"type": "output_text",
						"text": "Hello! How can I help you?"
					}
				]
			}
		],
		"usage": {
			"input_tokens": 12,
			"output_tokens": 8
		}
	}`

	resp, err := adapter.ParseResponse(strings.NewReader(tax))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "resp_abc123" {
		t.Errorf("expected ID resp_abc123, got %s", resp.ID)
	}
	if resp.Model != "gpt-5.6-luna" {
		t.Errorf("expected model gpt-5.6-luna, got %s", resp.Model)
	}
	if resp.Usage.PromptTokens != 12 {
		t.Errorf("expected 12 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 8 {
		t.Errorf("expected 8 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message["content"] != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %v", resp.Choices[0].Message["content"])
	}
}

func TestOpenAIResponsesAdapter_ParseResponse_Error(t *testing.T) {
	adapter := NewOpenAIResponsesAdapter()

	tax := `{
		"error": {
			"message": "Invalid API key",
			"type": "authentication_error",
			"code": "invalid_api_key"
		}
	}`

	resp, err := adapter.ParseResponse(strings.NewReader(tax))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Error.Message != "Invalid API key" {
		t.Errorf("expected error message 'Invalid API key', got %s", resp.Error.Message)
	}
}

func TestOpenAIResponsesAdapter_ParseStreamChunk(t *testing.T) {
	adapter := NewOpenAIResponsesAdapter()

	tax := []byte(`data: {"type":"response.output_text.delta","delta":{"content_index":0,"delta":"Hello"}}`)
	resp, done := adapter.ParseStreamChunk(tax)
	if done {
		t.Fatal("expected not done")
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Delta["content"] != "Hello" {
		t.Errorf("expected delta content 'Hello', got %v", resp.Choices[0].Delta["content"])
	}
}

func TestOpenAIResponsesAdapter_ParseStreamChunk_Done(t *testing.T) {
	adapter := NewOpenAIResponsesAdapter()

	_, done := adapter.ParseStreamChunk([]byte("data: [DONE]"))
	if !done {
		t.Fatal("expected done=true")
	}
}

func TestOpenAIResponsesAdapter_BuildRequest(t *testing.T) {
	adapter := NewOpenAIResponsesAdapter()

	req := &ProviderRequest{
		Model: "gpt-5.6-luna",
		Messages: []map[string]interface{}{
			{"role": "user", "content": "Hi"},
		},
		Stream: false,
	}

	httpReq, err := adapter.BuildRequest("https://opencode.ai/zen", "sk-test", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if httpReq.Method != "POST" {
		t.Errorf("expected POST, got %s", httpReq.Method)
	}
	if !strings.HasSuffix(httpReq.URL.Path, "/v1/responses") {
		t.Errorf("expected path ending with /v1/responses, got %s", httpReq.URL.Path)
	}
	if httpReq.Header.Get("Authorization") != "Bearer sk-test" {
		t.Errorf("expected Bearer auth, got %s", httpReq.Header.Get("Authorization"))
	}
}

func TestOpenCodeAdapter_DetectAdapter(t *testing.T) {
	adapter := NewOpenCodeAdapter()

	tests := []struct {
		model    string
		baseURL  string
		expected string
	}{
		{"gpt-5.6-luna", "https://opencode.ai/zen", "*OpenAIResponsesAdapter"},
		{"grok-4.5", "https://opencode.ai/zen", "*OpenAIResponsesAdapter"},
		{"muse-spark-1.2", "https://opencode.ai/zen", "*OpenAIResponsesAdapter"},
		{"claude-opus-5", "https://opencode.ai/zen", "*AnthropicAdapter"},
		{"qwen3.7-max", "https://opencode.ai/zen", "*AnthropicAdapter"},
		{"deepseek-v4-pro", "https://opencode.ai/zen", "*OpenAIAdapter"},
		{"glm-5.2", "https://opencode.ai/zen", "*OpenAIAdapter"},
		{"kimi-k3", "https://opencode.ai/zen", "*OpenAIAdapter"},
		{"minimax-m3", "https://opencode.ai/zen", "*OpenAIAdapter"},
		{"minimax-m3", "https://opencode.ai/zen/go", "*AnthropicAdapter"},
	}

	for _, tt := range tests {
		a := adapter.detectAdapter(tt.model, tt.baseURL)
		got := typeOfAdapter(a)
		if got != tt.expected {
			t.Errorf("model=%s baseURL=%s: expected %s, got %s", tt.model, tt.baseURL, tt.expected, got)
		}
	}
}

func typeOfAdapter(a ProviderAdapter) string {
	switch a.(type) {
	case *OpenAIResponsesAdapter:
		return "*OpenAIResponsesAdapter"
	case *AnthropicAdapter:
		return "*AnthropicAdapter"
	case *OpenAIAdapter:
		return "*OpenAIAdapter"
	default:
		return "unknown"
	}
}

func TestOpenAIResponsesAdapter_BuildRequestToolsTempAndHistory(t *testing.T) {
	adapter := NewOpenAIResponsesAdapter()
	zero := 0.0
	httpReq, err := adapter.BuildRequest("https://api.openai.com", "sk-test", &ProviderRequest{
		Model:       "gpt-5",
		MaxTokens:   512,
		Temperature: &zero,
		ToolChoice:  "auto",
		Tools: []interface{}{
			map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "search_docs",
					"description": "Search knowledge base",
					"parameters":  map[string]interface{}{"type": "object"},
				},
			},
		},
		Messages: []map[string]interface{}{
			{"role": "system", "content": "Be exact."},
			{"role": "user", "content": "Find Q3 docs"},
			{
				"role": "assistant",
				"tool_calls": []interface{}{
					map[string]interface{}{
						"id":       "call_resp_1",
						"type":     "function",
						"function": map[string]interface{}{"name": "search_docs", "arguments": `{"q":"Q3"}`},
					},
				},
			},
			{"role": "tool", "tool_call_id": "call_resp_1", "content": "Found 2 docs"},
		},
	})
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}

	raw, _ := io.ReadAll(httpReq.Body)
	var body map[string]interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body["instructions"] != "Be exact." {
		t.Fatalf("instructions=%v", body["instructions"])
	}
	if body["max_output_tokens"] != float64(512) {
		t.Fatalf("max_output_tokens=%v", body["max_output_tokens"])
	}
	if temp, ok := body["temperature"].(float64); !ok || temp != 0.0 {
		t.Fatalf("expected temperature=0.0, got %v", body["temperature"])
	}
	tools := body["tools"].([]interface{})
	if len(tools) != 1 || tools[0].(map[string]interface{})["name"] != "search_docs" {
		t.Fatalf("unexpected tools translation: %v", tools)
	}
	inputItems := body["input"].([]interface{})
	if len(inputItems) != 3 {
		t.Fatalf("expected 3 input items (user, function_call, function_call_output), got %d: %v", len(inputItems), inputItems)
	}
	if inputItems[1].(map[string]interface{})["type"] != "function_call" || inputItems[2].(map[string]interface{})["type"] != "function_call_output" {
		t.Fatalf("unexpected input items: %v", inputItems)
	}
}

func TestOpenAIResponsesAdapter_ParseStreamStringDeltaAndCompletedUsage(t *testing.T) {
	adapter := NewOpenAIResponsesAdapter()

	textChunk, done := adapter.ParseStreamChunk([]byte(`data: {"type":"response.output_text.delta","delta":"Hello OpenCode"}`))
	if done || textChunk == nil || textChunk.Choices[0].Delta["content"] != "Hello OpenCode" {
		t.Fatalf("expected string delta parsed cleanly, got %+v done=%v", textChunk, done)
	}

	completed, done := adapter.ParseStreamChunk([]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-5","usage":{"input_tokens":19,"output_tokens":7,"total_tokens":26}}}`))
	if done || completed == nil {
		t.Fatalf("expected completed usage event")
	}
	if completed.Usage.PromptTokens != 19 || completed.Usage.CompletionTokens != 7 || completed.Usage.TotalTokens != 26 {
		t.Fatalf("unexpected usage: %+v", completed.Usage)
	}

	acc := NewStreamToolAccumulator()
	added, _ := adapter.ParseStreamChunk([]byte(`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_fc_1","name":"get_weather"}}`))
	if added == nil {
		t.Fatal("expected tool added delta")
	}
	acc.Observe(added.Choices[0].Delta)

	argDelta, _ := adapter.ParseStreamChunk([]byte(`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"city\":\"Jakarta\"}"}`))
	if argDelta == nil {
		t.Fatal("expected tool arg delta")
	}
	acc.Observe(argDelta.Choices[0].Delta)

	recs := acc.Finish()
	if len(recs) != 1 || recs[0].CallID != "call_fc_1" || recs[0].Name != "get_weather" {
		t.Fatalf("unexpected streamed tool calls: %+v", recs)
	}
}

func TestOpenAIResponsesAdapter_ParseResponseFunctionCall(t *testing.T) {
	adapter := NewOpenAIResponsesAdapter()
	raw := `{
		"id": "resp_tool_1",
		"model": "gpt-5",
		"output": [
			{"type":"function_call","call_id":"call_42","name":"run_sql","arguments":"{\"limit\":5}"}
		],
		"usage": {"input_tokens": 10, "output_tokens": 4}
	}`
	resp, err := adapter.ParseResponse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("expected finishReason=tool_calls, got %s", resp.Choices[0].FinishReason)
	}
	recs := ExtractToolCallsFromResponse(resp)
	if len(recs) != 1 || recs[0].CallID != "call_42" || recs[0].Name != "run_sql" {
		t.Fatalf("unexpected extracted tools: %+v", recs)
	}
}
