package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type GoogleAdapter struct {
	openAI OpenAIAdapter
}

func NewGoogleAdapter() *GoogleAdapter {
	return &GoogleAdapter{
		openAI: OpenAIAdapter{},
	}
}

// MapGoogleUpstreamModel maps internal, virtual, or alias model names to valid Google AI Studio model IDs.
func MapGoogleUpstreamModel(model string) string {
	m := strings.TrimSpace(strings.ToLower(model))
	m = strings.TrimPrefix(m, "models/")

	switch m {
	case "", "prism-auto", "roozy-auto", "auto":
		return "gemini-3.5-flash"
	case "gemini-3.5-flash":
		return "gemini-3.5-flash"
	case "gemini-3.5-flash-lite":
		return "gemini-3.5-flash-lite"
	case "gemini-3.6-flash":
		return "gemini-3.6-flash"
	case "gemini-3.7-flash":
		return "gemini-3.7-flash"
	case "gemini-3.8-flash":
		return "gemini-3.8-flash"
	case "gemma-4-26b-a4b-it":
		return "gemma-4-26b-a4b-it"
	case "gemma-4-31b-it":
		return "gemma-4-31b-it"
	case "gemini-2.0-flash", "gemini-2.0-flash-001", "gemini-1.5-flash", "gemini-1.5-flash-001", "gemini-flash":
		return "gemini-3.5-flash"
	case "gemini-2.0-flash-lite", "gemini-2.0-flash-lite-preview", "gemini-1.5-flash-8b":
		return "gemini-3.5-flash-lite"
	case "gemini-1.5-pro", "gemini-2.0-pro", "gemini-2.5-pro", "gemini-pro":
		return "gemini-3.5-flash"
	default:
		// Virtual orchestrators and heavy next-gen aliases map gracefully to gemini-3.5-flash
		if strings.HasPrefix(m, "orchestrator-") ||
			strings.HasPrefix(m, "gemini-4-") {
			return "gemini-3.5-flash"
		}
		return model
	}
}

func (a *GoogleAdapter) BuildRequest(baseURL, apiKey string, req *ProviderRequest) (*http.Request, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Ensure path routes to /v1beta/openai/chat/completions
	if !strings.Contains(u.Path, "/v1beta/openai") {
		u.Path = path.Join(u.Path, "v1beta", "openai", "chat", "completions")
	} else if !strings.HasSuffix(u.Path, "/chat/completions") {
		u.Path = path.Join(u.Path, "chat", "completions")
	}

	targetModel := MapGoogleUpstreamModel(req.Model)

	body := map[string]interface{}{
		"model":    targetModel,
		"messages": SanitizeMessagesForGoogle(req.Messages),
		"stream":   req.Stream,
	}
	if len(req.Tools) > 0 {
		body["tools"] = ConvertGenericToolsToOpenAI(req.Tools)
	}
	if req.Stream {
		body["stream_options"] = map[string]interface{}{
			"include_usage": true,
		}
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	for k, v := range req.Extra {
		body[k] = v
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// For API Keys (e.g. starting with AIzaSy or AQ.), append key query parameter & x-goog-api-key
	if strings.HasPrefix(apiKey, "AIzaSy") || strings.HasPrefix(apiKey, "AQ.") || !strings.Contains(apiKey, ".") {
		q := u.Query()
		q.Set("key", apiKey)
		u.RawQuery = q.Encode()
	}

	httpReq, err := http.NewRequest("POST", u.String(), bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("x-goog-api-key", apiKey)
	return httpReq, nil
}

func (a *GoogleAdapter) ParseResponse(body io.Reader) (*ProviderResponse, error) {
	return a.openAI.ParseResponse(body)
}

func (a *GoogleAdapter) ParseStreamChunk(line []byte) (*ProviderResponse, bool) {
	return a.openAI.ParseStreamChunk(line)
}

func (a *GoogleAdapter) SupportsStreaming() bool {
	return true
}

const SentinelThoughtSignature = "skip_thought_signature_validator"

func isInvalidSignature(v interface{}) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	if !ok {
		return true
	}
	s = strings.TrimSpace(s)
	if s == "" || s == "skip" || s == "none" || s == "null" || s == "undefined" {
		return true
	}
	return false
}

func ensureSignature(m map[string]interface{}) {
	var validSig interface{}
	if sig, hasSig := m["thought_signature"]; hasSig && !isInvalidSignature(sig) {
		validSig = sig
	} else if sig, hasSig := m["thoughtSignature"]; hasSig && !isInvalidSignature(sig) {
		validSig = sig
	}

	if validSig != nil {
		m["thought_signature"] = validSig
		m["thoughtSignature"] = validSig
	} else {
		m["thought_signature"] = SentinelThoughtSignature
		m["thoughtSignature"] = SentinelThoughtSignature
	}
}

// SanitizeValueRecursively recursively inspects and sanitizes any map or slice in the message payload.
func SanitizeValueRecursively(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]interface{}:
		return sanitizeMapRecursively(val)
	case []interface{}:
		newSlice := make([]interface{}, len(val))
		for i, item := range val {
			newSlice[i] = SanitizeValueRecursively(item)
		}
		return newSlice
	case []map[string]interface{}:
		newSlice := make([]interface{}, len(val))
		for i, item := range val {
			newSlice[i] = sanitizeMapRecursively(item)
		}
		return newSlice
	default:
		return v
	}
}

func sanitizeMapRecursively(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}

	mCopy := make(map[string]interface{}, len(m))
	for k, v := range m {
		// Preserve tool argument payloads (args/arguments/input) verbatim without injecting thought_signature into tool parameters
		if k == "args" || k == "arguments" || k == "input" {
			mCopy[k] = v
		} else {
			mCopy[k] = SanitizeValueRecursively(v)
		}
	}

	// Ensure signature ONLY on message, part, tool_call, or function_call containers
	isMessageOrToolCall := false

	if role, ok := mCopy["role"].(string); ok && (role == "assistant" || role == "model" || role == "tool" || role == "function") {
		isMessageOrToolCall = true
	}
	if _, hasName := mCopy["name"]; hasName {
		isMessageOrToolCall = true
	}
	if _, hasFnCall := mCopy["functionCall"]; hasFnCall {
		isMessageOrToolCall = true
	}
	if _, hasFnCall := mCopy["function_call"]; hasFnCall {
		isMessageOrToolCall = true
	}
	if _, hasFn := mCopy["function"]; hasFn {
		isMessageOrToolCall = true
	}
	if _, hasFnResp := mCopy["functionResponse"]; hasFnResp {
		isMessageOrToolCall = true
	}
	if _, hasFnResp := mCopy["function_response"]; hasFnResp {
		isMessageOrToolCall = true
	}
	if _, hasParts := mCopy["parts"]; hasParts {
		isMessageOrToolCall = true
	}
	if _, hasToolCalls := mCopy["tool_calls"]; hasToolCalls {
		isMessageOrToolCall = true
	}
	if _, hasToolCalls := mCopy["toolCalls"]; hasToolCalls {
		isMessageOrToolCall = true
	}
	if typeVal, ok := mCopy["type"].(string); ok && (typeVal == "function" || typeVal == "tool_calls" || typeVal == "tool_use" || typeVal == "function_call" || typeVal == "functionCall") {
		isMessageOrToolCall = true
	}

	if isMessageOrToolCall {
		ensureSignature(mCopy)
	}

	return mCopy
}

// SanitizeMessagesForGoogle ensures all tool calls across all messages contain thought_signature fields.
func SanitizeMessagesForGoogle(messages []map[string]interface{}) []map[string]interface{} {
	if len(messages) == 0 {
		return messages
	}

	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		sanitized := SanitizeValueRecursively(msg)
		if msgMap, ok := sanitized.(map[string]interface{}); ok {
			result = append(result, msgMap)
		} else {
			result = append(result, msg)
		}
	}
	return result
}
