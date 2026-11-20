package proxy

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicStreamUsageComesFromMessageStartAndMessageDelta(t *testing.T) {
	a := &AnthropicAdapter{}

	start, done := a.ParseStreamChunk([]byte(`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":25,"output_tokens":1}}}`))
	require.False(t, done)
	require.NotNil(t, start)
	assert.Equal(t, 25, start.Usage.PromptTokens)

	text, _ := a.ParseStreamChunk([]byte(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`))
	require.NotNil(t, text)
	assert.Equal(t, "Hi", text.Choices[0].Delta["content"])

	delta, done := a.ParseStreamChunk([]byte(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":42}}`))
	require.False(t, done)
	require.NotNil(t, delta)
	assert.Equal(t, 42, delta.Usage.CompletionTokens)
	assert.Equal(t, "stop", delta.Choices[0].FinishReason)

	_, done = a.ParseStreamChunk([]byte(`data: {"type":"message_stop"}`))
	assert.True(t, done)
}

func TestAnthropicStreamErrorEventIsReported(t *testing.T) {
	a := &AnthropicAdapter{}

	chunk, done := a.ParseStreamChunk([]byte(`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`))
	require.False(t, done)
	require.NotNil(t, chunk)
	require.NotNil(t, chunk.Error)
	assert.Equal(t, "overloaded_error", chunk.Error.Type)
	assert.Equal(t, "Overloaded", chunk.Error.Message)
}

func TestAnthropicStreamIgnoresPingsAndEventLines(t *testing.T) {
	a := &AnthropicAdapter{}

	chunk, done := a.ParseStreamChunk([]byte(`event: ping`))
	assert.Nil(t, chunk)
	assert.False(t, done)
	chunk, done = a.ParseStreamChunk([]byte(`data: {"type":"ping"}`))
	assert.Nil(t, chunk)
	assert.False(t, done)
}

func TestOpenAIStreamErrorObjectIsReported(t *testing.T) {
	a := &OpenAIAdapter{}

	chunk, done := a.ParseStreamChunk([]byte(`data: {"error":{"message":"The server had an error","type":"server_error","code":"overloaded"}}`))
	require.False(t, done)
	require.NotNil(t, chunk)
	require.NotNil(t, chunk.Error)
	assert.Equal(t, "server_error", chunk.Error.Type)
	assert.Equal(t, "overloaded", chunk.Error.Code)

	// A normal chunk is unaffected.
	chunk, _ = a.ParseStreamChunk([]byte(`data: {"id":"1","choices":[{"index":0,"delta":{"content":"hi"}}]}`))
	require.NotNil(t, chunk)
	assert.Nil(t, chunk.Error)
	assert.Equal(t, "hi", chunk.Choices[0].Delta["content"])
}

func TestAnthropicBuildRequest_DefaultsToolResultsAndTemperatureZero(t *testing.T) {
	a := NewAnthropicAdapter()
	tempZero := 0.0
	httpReq, err := a.BuildRequest("https://api.anthropic.com", "ant-key", &ProviderRequest{
		Model:       "claude-3-7-sonnet",
		Temperature: &tempZero,
		ToolChoice:  "required",
		Messages: []map[string]interface{}{
			{"role": "system", "content": "Be concise."},
			{"role": "user", "content": "Check SF weather and time"},
			{
				"role":    "assistant",
				"content": "Checking both.",
				"tool_calls": []interface{}{
					map[string]interface{}{
						"id":       "call_1",
						"type":     "function",
						"function": map[string]interface{}{"name": "get_weather", "arguments": `{"city":"SF"}`},
					},
					map[string]interface{}{
						"id":       "call_2",
						"type":     "function",
						"function": map[string]interface{}{"name": "get_time", "arguments": `{"tz":"PST"}`},
					},
				},
			},
			{"role": "tool", "tool_call_id": "call_1", "content": "72F"},
			{"role": "tool", "tool_call_id": "call_2", "content": "12:00"},
		},
		Extra: map[string]interface{}{
			"stream_options": map[string]interface{}{"include_usage": true},
			"top_p":          0.9,
		},
	})
	require.NoError(t, err)

	raw, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &body))

	assert.Equal(t, float64(defaultAnthropicMaxTokens), body["max_tokens"])
	assert.Equal(t, 0.0, body["temperature"])
	assert.Equal(t, "Be concise.", body["system"])
	assert.Equal(t, map[string]interface{}{"type": "any"}, body["tool_choice"])
	assert.Nil(t, body["stream_options"])
	assert.Equal(t, 0.9, body["top_p"])

	msgs, ok := body["messages"].([]interface{})
	require.True(t, ok)
	// user -> assistant (tool_use) -> single merged user (two tool_result blocks)
	require.Len(t, msgs, 3)

	assistantContent := msgs[1].(map[string]interface{})["content"].([]interface{})
	require.Len(t, assistantContent, 3)
	assert.Equal(t, "tool_use", assistantContent[1].(map[string]interface{})["type"])

	toolResultUser := msgs[2].(map[string]interface{})
	assert.Equal(t, "user", toolResultUser["role"])
	mergedResults := toolResultUser["content"].([]interface{})
	require.Len(t, mergedResults, 2)
	assert.Equal(t, "tool_result", mergedResults[0].(map[string]interface{})["type"])
	assert.Equal(t, "call_1", mergedResults[0].(map[string]interface{})["tool_use_id"])
	assert.Equal(t, "call_2", mergedResults[1].(map[string]interface{})["tool_use_id"])
}

func TestAnthropicStreamToolUseAndThinkingChunks(t *testing.T) {
	a := NewAnthropicAdapter()
	acc := NewStreamToolAccumulator()

	thinkChunk, done := a.ParseStreamChunk([]byte(`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Thinking..."}}`))
	require.False(t, done)
	require.NotNil(t, thinkChunk)
	assert.Equal(t, "Thinking...", thinkChunk.Choices[0].Delta["reasoning_content"])

	// content block index 1 is a tool_use block -> remapped cleanly to tool_call index 0
	startTool, _ := a.ParseStreamChunk([]byte(`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_9","name":"linear__list","input":{}}}`))
	require.NotNil(t, startTool)
	acc.Observe(startTool.Choices[0].Delta)

	argPart1, _ := a.ParseStreamChunk([]byte(`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"team\":"}}`))
	require.NotNil(t, argPart1)
	acc.Observe(argPart1.Choices[0].Delta)

	argPart2, _ := a.ParseStreamChunk([]byte(`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"ENG\"}"}}`))
	require.NotNil(t, argPart2)
	acc.Observe(argPart2.Choices[0].Delta)

	recs := acc.Finish()
	require.Len(t, recs, 1)
	assert.Equal(t, "toolu_9", recs[0].CallID)
	assert.Equal(t, "linear__list", recs[0].Name)
	assert.Equal(t, json.RawMessage(`{"team":"ENG"}`), recs[0].Arguments)
}
