package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"strings"
)

// ToolDefinition defines a tool the LLM can call during iterative tool-calling.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema object
}

// ToolCall represents a single tool invocation requested by the LLM.
type ToolCall struct {
	ID        string                 `json:"id"`        // Provider-specific call ID
	Name      string                 `json:"name"`      // Tool name
	Arguments map[string]interface{} `json:"arguments"` // Parsed arguments
}

// ToolResult represents the result of executing a tool.
type ToolResult struct {
	CallID  string `json:"call_id"` // Matches ToolCall.ID
	Name    string `json:"name"`    // Tool name
	Content string `json:"content"` // Result content (JSON or text)
	IsError bool   `json:"is_error"`
}

// ToolExecutorFunc executes a tool call and returns a result.
// The service layer provides this — it has access to dbManager, schema, etc.
type ToolExecutorFunc func(ctx context.Context, call ToolCall) (*ToolResult, error)

// ToolCallConfig configures the iterative tool-calling behavior.
type ToolCallConfig struct {
	MaxIterations int
	DBType        string
	NonTechMode   bool
	ModelID       string
	SystemPrompt  string                                 // Additional tool-calling instructions appended to the DB-specific system prompt
	OnToolCall    func(call ToolCall)                    // Callback when LLM requests a tool
	OnToolResult  func(call ToolCall, result ToolResult) // Callback when tool execution completes
	OnIteration   func(iteration int, toolCallCount int) // Callback at each iteration start
}

// ToolCallResult is the final outcome of an iterative tool-calling session.
type ToolCallResult struct {
	Response    string     `json:"response"`     // Final JSON response (LLMResponse format)
	Iterations  int        `json:"iterations"`   // Number of LLM round-trips
	TotalCalls  int        `json:"total_calls"`  // Total tool calls made
	ToolHistory []ToolCall `json:"tool_history"` // All tool calls for audit/logging
}

// Aliases for tool name constants — canonical definitions live in constants package.
const (
	FinalResponseToolName = constants.FinalResponseToolName
	ExecuteQueryToolName  = constants.ExecuteQueryToolName
	GetTableInfoToolName  = constants.GetTableInfoToolName

	DefaultMaxIterations = constants.DefaultMaxToolIterations
	MaxToolResultChars   = constants.MaxToolResultChars
)

// GetNeobaseTools returns the tool definitions for the NeoBase agent.
// These are provider-agnostic — each provider converts to its native format.
func GetNeobaseTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name: ExecuteQueryToolName,
			Description: "Execute a read-only database query (SELECT, SHOW, DESCRIBE, EXPLAIN, or MongoDB find/aggregate) " +
				"to explore data, verify query correctness, or understand table contents. " +
				"Only read operations are allowed — write operations will be rejected. " +
				"Use this to test and validate queries before including them in the final response.",
			Parameters: constants.ExecuteQueryToolSchema,
		},
		{
			Name: GetTableInfoToolName,
			Description: "Get detailed schema information for specific tables or collections, " +
				"including column names, data types, constraints, indexes, and foreign keys. " +
				"Use this when you need more detail about specific tables than what the RAG context provides.",
			Parameters: constants.GetTableInfoToolSchema,
		},
		{
			Name: FinalResponseToolName,
			Description: "Generate the final structured response to send to the user. " +
				"Call this ONLY when you have gathered enough information and are ready to respond. " +
				"Include any queries the user should execute, your explanation, and suggested follow-up actions. " +
				"This MUST be the last tool you call.",
			Parameters: GetFinalResponseSchema(),
		},
	}
}

// GetFinalResponseSchema returns the JSON Schema for the generate_final_response tool.
// This matches the LLMResponse structure used throughout the system.
func GetFinalResponseSchema() map[string]interface{} {
	return constants.ToolFinalResponseSchema
}

// GetToolCallingSystemPromptAddendum returns additional instructions appended to the
// base system prompt when tool calling mode is active.
func GetToolCallingSystemPromptAddendum() string {
	return constants.ToolCallingSystemPromptAddendum
}

// TryParseTextToolCall attempts to detect and extract a valid final response from text
// that looks like a tool-call attempt (e.g., models that emit Python-style function calls
// as text instead of using native function calling). Returns the extracted JSON string
// and true if a valid response was found, or ("", false) if not.
//
// Common patterns detected:
//   - generate_final_response(assistantMessage='...', queries=[...])
//   - {"assistantMessage": "...", "queries": [...]} embedded in other text
//   - <ctrl42>call print(default_api.generate_final_response(...))
func TryParseTextToolCall(textContent string) (string, bool) {
	// Fast path: if it looks like valid JSON with assistantMessage, use it directly
	trimmed := strings.TrimSpace(textContent)
	if strings.HasPrefix(trimmed, "{") {
		var testJSON map[string]interface{}
		if json.Unmarshal([]byte(trimmed), &testJSON) == nil {
			if _, hasMsg := testJSON["assistantMessage"]; hasMsg {
				return trimmed, true
			}
		}
	}

	// Pattern 1: Extract JSON object embedded in text (look for {"assistantMessage": ...})
	if idx := strings.Index(textContent, `{"assistantMessage"`); idx >= 0 {
		candidate := textContent[idx:]
		// Try to find the matching closing brace
		if extracted := extractJSONObject(candidate); extracted != "" {
			var testJSON map[string]interface{}
			if json.Unmarshal([]byte(extracted), &testJSON) == nil {
				return extracted, true
			}
		}
	}

	// Pattern 1b: Extract JSON from markdown code fences (```json ... ``` or ``` ... ```)
	// LLMs sometimes emit the tool call as a markdown code block instead of native function calling.
	if idx := indexOfCodeFenceJSON(textContent); idx >= 0 {
		// Find the end of the code fence
		endFence := strings.Index(textContent[idx:], "\n```")
		if endFence < 0 {
			endFence = strings.Index(textContent[idx:], "\r\n```")
		}
		if endFence > 0 {
			jsonCandidate := strings.TrimSpace(textContent[idx : idx+endFence])
			var testJSON map[string]interface{}
			if json.Unmarshal([]byte(jsonCandidate), &testJSON) == nil {
				if _, hasMsg := testJSON["assistantMessage"]; hasMsg {
					log.Printf("TryParseTextToolCall -> Extracted response from markdown code fence")
					responseJSON, err := json.Marshal(testJSON)
					if err == nil {
						return string(responseJSON), true
					}
				}
			}
		}
	}

	// Pattern 1c: Pretty-printed JSON with whitespace between { and "assistantMessage"
	// e.g., "{\n  \"assistantMessage\": ..."
	if idx := strings.Index(textContent, `"assistantMessage"`); idx > 0 {
		// Walk backwards from "assistantMessage" to find the opening {
		for i := idx - 1; i >= 0; i-- {
			ch := textContent[i]
			if ch == '{' {
				candidate := textContent[i:]
				if extracted := extractJSONObject(candidate); extracted != "" {
					var testJSON map[string]interface{}
					if json.Unmarshal([]byte(extracted), &testJSON) == nil {
						if _, hasMsg := testJSON["assistantMessage"]; hasMsg {
							log.Printf("TryParseTextToolCall -> Extracted response from pretty-printed JSON")
							return extracted, true
						}
					}
				}
				break
			}
			if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
				break // Non-whitespace before "assistantMessage" means { isn't immediately preceding
			}
		}
	}

	// Pattern 2: Python-style generate_final_response(...) call
	// Extract the keyword arguments and convert to JSON
	fnName := "generate_final_response("
	if idx := strings.Index(textContent, fnName); idx >= 0 {
		// Find the arguments portion after generate_final_response(
		argsStart := idx + len(fnName)
		rest := textContent[argsStart:]

		// Try to extract assistantMessage from the kwargs
		assistantMsg := extractPythonStringArg(rest, "assistantMessage")
		if assistantMsg != "" {
			// Build a minimal valid response
			response := map[string]interface{}{
				"assistantMessage": assistantMsg,
				"queries":          []interface{}{},
				"actionButtons":    []interface{}{},
			}

			// Try to extract queries array if present
			if queriesStr := extractPythonListArg(rest, "queries"); queriesStr != "" {
				// Try parsing as JSON (Python-style lists with single quotes won't parse, but try)
				queriesJSON := strings.ReplaceAll(queriesStr, "'", "\"")
				queriesJSON = strings.ReplaceAll(queriesJSON, "True", "true")
				queriesJSON = strings.ReplaceAll(queriesJSON, "False", "false")
				queriesJSON = strings.ReplaceAll(queriesJSON, "None", "null")
				var queries []interface{}
				if json.Unmarshal([]byte(queriesJSON), &queries) == nil {
					response["queries"] = queries
				}
			}

			responseJSON, err := json.Marshal(response)
			if err == nil {
				log.Printf("TryParseTextToolCall -> Extracted response from Python-style tool call")
				return string(responseJSON), true
			}
		}
	}

	return "", false
}

// extractJSONObject attempts to extract a balanced JSON object starting from the beginning of s.
func extractJSONObject(s string) string {
	if len(s) == 0 || s[0] != '{' {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i, ch := range s {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return ""
}

// indexOfCodeFenceJSON finds the start of JSON content inside a markdown code fence.
// Returns the index of the JSON content (after the fence opener), or -1 if not found.
func indexOfCodeFenceJSON(text string) int {
	// Look for ```json\n{ or ```\n{
	patterns := []string{"```json\n", "```json\r\n", "```\n", "```\r\n"}
	for _, pat := range patterns {
		idx := strings.Index(text, pat)
		if idx >= 0 {
			contentStart := idx + len(pat)
			// Skip leading whitespace to find the JSON object start
			for contentStart < len(text) && (text[contentStart] == ' ' || text[contentStart] == '\t' || text[contentStart] == '\n' || text[contentStart] == '\r') {
				contentStart++
			}
			if contentStart < len(text) && text[contentStart] == '{' {
				return contentStart
			}
		}
	}
	return -1
}

// extractPythonStringArg extracts the value of a keyword argument like assistantMessage='...'
func extractPythonStringArg(text, argName string) string {
	// Look for argName='...' or argName="..."
	for _, quote := range []string{"'", "\""} {
		pattern := argName + "=" + quote
		idx := strings.Index(text, pattern)
		if idx < 0 {
			continue
		}
		start := idx + len(pattern)
		// Find matching close quote (handle escaped quotes)
		end := -1
		for i := start; i < len(text); i++ {
			if text[i] == '\\' {
				i++ // skip escaped char
				continue
			}
			if string(text[i]) == quote {
				end = i
				break
			}
		}
		if end > start {
			return text[start:end]
		}
	}
	return ""
}

// extractPythonListArg extracts the value of a keyword argument like queries=[...]
func extractPythonListArg(text, argName string) string {
	pattern := argName + "=["
	idx := strings.Index(text, pattern)
	if idx < 0 {
		return ""
	}
	start := idx + len(pattern) - 1 // include the opening [
	depth := 0
	for i := start; i < len(text); i++ {
		if text[i] == '[' {
			depth++
		} else if text[i] == ']' {
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

// TruncateToolResult truncates a tool result to MaxToolResultChars with a note.
func TruncateToolResult(content string) string {
	if len(content) <= MaxToolResultChars {
		return content
	}
	return content[:MaxToolResultChars] + fmt.Sprintf(
		"\n... [truncated, showing first %d chars of %d total]",
		MaxToolResultChars, len(content),
	)
}

// ConvertLLMMessagesToGenericHistory converts []*models.LLMMessage to a simpler format
// for providers that need to rebuild history during tool-calling iterations.
func ConvertLLMMessagesToGenericHistory(messages []*models.LLMMessage) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		result = append(result, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	return result
}
