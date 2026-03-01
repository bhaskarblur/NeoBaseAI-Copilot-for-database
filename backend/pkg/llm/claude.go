package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"net/http"
	"time"
)

type ClaudeClient struct {
	apiKey              string
	model               string
	maxCompletionTokens int
	temperature         float64
	DBConfigs           []LLMDBConfig
	httpClient          *http.Client
}

// Claude API request/response structures
type claudeMessage struct {
	Role    string                 `json:"role"`
	Content []claudeMessageContent `json:"content"`
}

type claudeMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
	System      string          `json:"system,omitempty"`
	Messages    []claudeMessage `json:"messages"`
	Tools       []claudeTool    `json:"tools,omitempty"`       // For structured outputs
	ToolChoice  interface{}     `json:"tool_choice,omitempty"` // Force tool use
}

type claudeTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type claudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type  string                 `json:"type"`
		Text  string                 `json:"text,omitempty"`
		ID    string                 `json:"id,omitempty"`
		Name  string                 `json:"name,omitempty"`
		Input map[string]interface{} `json:"input,omitempty"` // For tool_use responses
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type claudeErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func NewClaudeClient(config Config) (*ClaudeClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Claude API key is required")
	}

	model := config.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022" // Default to latest Sonnet
	}

	return &ClaudeClient{
		apiKey:              config.APIKey,
		model:               model,
		maxCompletionTokens: config.MaxCompletionTokens,
		temperature:         config.Temperature,
		DBConfigs:           config.DBConfigs,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

func (c *ClaudeClient) GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string, nonTechMode bool, modelID ...string) (string, error) {
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Use provided model if specified, otherwise use the client's default model
	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("Claude GenerateResponse -> Using selected model: %s", model)
	}

	// Get the system prompt with non-tech mode if enabled
	systemPrompt := constants.GetSystemPrompt(constants.Claude, dbType, nonTechMode)
	responseSchemaJSON := ""

	for _, dbConfig := range c.DBConfigs {
		if dbConfig.DBType == dbType {
			responseSchemaJSON = dbConfig.Schema.(string)
			break
		}
	}

	// Parse the JSON schema string into a map for Claude's tool input_schema
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(responseSchemaJSON), &schemaMap); err != nil {
		return "", fmt.Errorf("failed to parse response schema: %v", err)
	}

	// Create a tool definition for structured output
	tool := claudeTool{
		Name:        "generate_database_response",
		Description: "Generate a structured response for database interaction with assistant message and optional queries",
		InputSchema: schemaMap,
	}

	// Force the model to use the tool
	toolChoice := map[string]interface{}{
		"type": "tool",
		"name": "generate_database_response",
	}

	// Convert messages to Claude format
	claudeMessages := make([]claudeMessage, 0, len(messages))

	for _, msg := range messages {
		content := ""

		switch msg.Role {
		case "user":
			if userMsg, ok := msg.Content["user_message"].(string); ok {
				content = userMsg
				// Add non-tech mode context if the mode differs from current request
				if msg.NonTechMode != nonTechMode {
					if msg.NonTechMode {
						content = "[This message was sent in NON-TECHNICAL MODE] " + content
					} else {
						content = "[This message was sent in TECHNICAL MODE] " + content
					}
				}
			}
		case "assistant":
			content = getAssistantContent(msg.Content)
			// Add non-tech mode context if the mode differs from current request
			if content != "" && msg.NonTechMode != nonTechMode {
				if msg.NonTechMode {
					content = "[This response was generated in NON-TECHNICAL MODE]\n" + content
				} else {
					content = "[This response was generated in TECHNICAL MODE]\n" + content
				}
			}
		case "system":
			if schemaUpdate, ok := msg.Content["schema_update"].(string); ok {
				content = fmt.Sprintf("Database schema update:\n%s", schemaUpdate)
			}
			// Append RAG context (relevant schema context or no-match signal) if present
			if ragCtx, ok := msg.Content["rag_context"].(string); ok && ragCtx != "" {
				if content != "" {
					content += "\n\n" + ragCtx
				} else {
					content = ragCtx
				}
			}
		}

		if content != "" {
			claudeMessages = append(claudeMessages, claudeMessage{
				Role: mapClaudeRole(msg.Role),
				Content: []claudeMessageContent{
					{
						Type: "text",
						Text: content,
					},
				},
			})
		}
	}

	// Create request
	reqBody := claudeRequest{
		Model:       model,
		MaxTokens:   c.maxCompletionTokens,
		Temperature: c.temperature,
		System:      systemPrompt,
		Messages:    claudeMessages,
		Tools:       []claudeTool{tool},
		ToolChoice:  toolChoice,
	}

	// Marshal request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Claude API error: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp claudeErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return "", fmt.Errorf("Claude API error: %s - %s", errResp.Error.Type, errResp.Error.Message)
		}
		return "", fmt.Errorf("Claude API error: status code %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	// Claude returns tool_use in content when using structured outputs
	// Find the tool_use content block
	for _, content := range claudeResp.Content {
		if content.Type == "tool_use" && content.Name == "generate_database_response" {
			// Convert the input map to JSON string
			responseJSON, err := json.Marshal(content.Input)
			if err != nil {
				return "", fmt.Errorf("failed to marshal tool input: %v", err)
			}
			return string(responseJSON), nil
		}
	}

	// Fallback to text response if no tool_use found
	for _, content := range claudeResp.Content {
		if content.Type == "text" && content.Text != "" {
			return content.Text, nil
		}
	}

	return "", fmt.Errorf("no valid response content found")
}

// GenerateRawJSON generates a response with a custom system prompt and no response schema.
// Used for tasks like KB generation that need raw JSON output.
func (c *ClaudeClient) GenerateRawJSON(ctx context.Context, systemPrompt string, userMessage string, modelID ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("Claude GenerateRawJSON -> Using selected model: %s", model)
	}

	reqBody := claudeRequest{
		Model:       model,
		MaxTokens:   c.maxCompletionTokens,
		Temperature: c.temperature,
		System:      systemPrompt,
		Messages: []claudeMessage{
			{Role: "user", Content: []claudeMessageContent{{Type: "text", Text: userMessage}}},
		},
		// No tools / toolChoice — raw text response
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Claude API error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp claudeErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return "", fmt.Errorf("Claude API error: %s - %s", errResp.Error.Type, errResp.Error.Message)
		}
		return "", fmt.Errorf("Claude API error: status code %d", resp.StatusCode)
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	for _, content := range claudeResp.Content {
		if content.Type == "text" && content.Text != "" {
			log.Printf("Claude GenerateRawJSON -> response length: %d", len(content.Text))
			return content.Text, nil
		}
	}

	return "", fmt.Errorf("no text content in Claude response")
}

func (c *ClaudeClient) GenerateRecommendations(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error) {
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Get recommendations-specific prompt and schema
	systemPrompt := constants.GetRecommendationsPrompt(constants.Claude)
	responseSchemaJSON := constants.GetRecommendationsSchema(constants.Claude).(string)

	// Parse the JSON schema string into a map for Claude's tool input_schema
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(responseSchemaJSON), &schemaMap); err != nil {
		return "", fmt.Errorf("failed to parse recommendations schema: %v", err)
	}

	// Create a tool definition for structured recommendations output
	tool := claudeTool{
		Name:        "generate_recommendations",
		Description: "Generate structured query recommendations for the database",
		InputSchema: schemaMap,
	}

	// Force the model to use the tool
	toolChoice := map[string]interface{}{
		"type": "tool",
		"name": "generate_recommendations",
	}

	// Convert messages to Claude format
	claudeMessages := make([]claudeMessage, 0, len(messages))

	for _, msg := range messages {
		content := ""

		switch msg.Role {
		case "user":
			if userMsg, ok := msg.Content["user_message"].(string); ok {
				content = userMsg
			}
		case "assistant":
			content = getAssistantContent(msg.Content)
		case "system":
			if schemaUpdate, ok := msg.Content["schema_update"].(string); ok {
				content = fmt.Sprintf("Database schema update:\n%s", schemaUpdate)
			}
			// Append RAG context (relevant schema context or no-match signal) if present
			if ragCtx, ok := msg.Content["rag_context"].(string); ok && ragCtx != "" {
				if content != "" {
					content += "\n\n" + ragCtx
				} else {
					content = ragCtx
				}
			}
		}

		if content != "" {
			claudeMessages = append(claudeMessages, claudeMessage{
				Role: mapClaudeRole(msg.Role),
				Content: []claudeMessageContent{
					{
						Type: "text",
						Text: content,
					},
				},
			})
		}
	}

	// Create request
	reqBody := claudeRequest{
		Model:       c.model,
		MaxTokens:   c.maxCompletionTokens,
		Temperature: c.temperature,
		System:      systemPrompt,
		Messages:    claudeMessages,
		Tools:       []claudeTool{tool},
		ToolChoice:  toolChoice,
	}

	// Marshal request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Claude API error: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp claudeErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return "", fmt.Errorf("Claude API error: %s - %s", errResp.Error.Type, errResp.Error.Message)
		}
		return "", fmt.Errorf("Claude API error: status code %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	// Find the tool_use content block
	for _, content := range claudeResp.Content {
		if content.Type == "tool_use" && content.Name == "generate_recommendations" {
			// Convert the input map to JSON string
			responseJSON, err := json.Marshal(content.Input)
			if err != nil {
				return "", fmt.Errorf("failed to marshal tool input: %v", err)
			}
			return string(responseJSON), nil
		}
	}

	// Fallback to text response if no tool_use found
	for _, content := range claudeResp.Content {
		if content.Type == "text" && content.Text != "" {
			return content.Text, nil
		}
	}

	return "", fmt.Errorf("no valid response content found")
}

// GenerateVisualization generates a visualization configuration for query results
// This method uses a dedicated visualization system prompt and enforces JSON response format
func (c *ClaudeClient) GenerateVisualization(ctx context.Context, systemPrompt string, visualizationPrompt string, dataRequest string, modelID ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("Claude GenerateVisualization -> Using selected model: %s", model)
	}

	// Build the messages for visualization
	messages := make([]claudeMessage, 0)

	// Add visualization prompt
	messages = append(messages, claudeMessage{
		Role: "user",
		Content: []claudeMessageContent{
			{
				Type: "text",
				Text: visualizationPrompt,
			},
		},
	})

	// Add data request
	messages = append(messages, claudeMessage{
		Role: "user",
		Content: []claudeMessageContent{
			{
				Type: "text",
				Text: dataRequest,
			},
		},
	})

	// Create request
	req := claudeRequest{
		Model:       model,
		MaxTokens:   c.maxCompletionTokens,
		Temperature: c.temperature,
		System:      systemPrompt,
		Messages:    messages,
	}

	// Prepare request body
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Make API request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	// Extract text response
	for _, content := range claudeResp.Content {
		if content.Type == "text" && content.Text != "" {
			log.Printf("CLAUDE -> GenerateVisualization -> responseText: %s", content.Text)

			// Validate JSON response
			var visualizationResponse map[string]interface{}
			if err := json.Unmarshal([]byte(content.Text), &visualizationResponse); err != nil {
				log.Printf("Error: Claude visualization response is not valid JSON: %v", err)
				return "", fmt.Errorf("invalid JSON response from Claude: %v", err)
			}

			return content.Text, nil
		}
	}

	return "", fmt.Errorf("no valid text response found")
}

func (c *ClaudeClient) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:                c.model,
		Provider:            constants.Claude,
		MaxCompletionTokens: c.maxCompletionTokens,
		ContextLimit:        200000, // Claude 3.5 Sonnet has 200K context
	}
}

func (c *ClaudeClient) SetModel(modelID string) error {
	c.model = modelID
	return nil
}

// claudeToolResultContent represents content in a tool_result block
type claudeToolResultContent struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
}

// claudeToolResultBlock represents a tool_result block sent back to Claude
type claudeToolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// claudeRawMessage is like claudeMessage but with flexible content for tool-use multi-turn
type claudeRawMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // can be string, []claudeMessageContent, or []interface{} for tool blocks
}

// claudeRawRequest is like claudeRequest but uses raw messages for tool-calling
type claudeRawRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
	System      string             `json:"system,omitempty"`
	Messages    []claudeRawMessage `json:"messages"`
	Tools       []claudeTool       `json:"tools,omitempty"`
}

// GenerateWithTools implements iterative tool-calling using Claude's native tool use.
func (c *ClaudeClient) GenerateWithTools(ctx context.Context, messages []*models.LLMMessage, tools []ToolDefinition, executor ToolExecutorFunc, config ToolCallConfig) (*ToolCallResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	model := c.model
	if config.ModelID != "" {
		model = config.ModelID
		log.Printf("Claude GenerateWithTools -> Using selected model: %s", model)
	}

	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = DefaultMaxIterations
	}

	// Build system prompt: always include DB-specific prompt, then append tool-calling addendum
	systemPrompt := constants.GetSystemPrompt(constants.Claude, config.DBType, config.NonTechMode)
	if config.SystemPrompt != "" {
		systemPrompt = systemPrompt + "\n\n" + config.SystemPrompt
	}

	// Convert tool definitions to Claude tools
	claudeTools := make([]claudeTool, 0, len(tools))
	for _, tool := range tools {
		claudeTools = append(claudeTools, claudeTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.Parameters,
		})
	}

	// Build initial messages using raw messages (supports both text and tool blocks)
	rawMessages := make([]claudeRawMessage, 0, len(messages))
	for _, msg := range messages {
		content := ""
		switch msg.Role {
		case "user":
			if userMsg, ok := msg.Content["user_message"].(string); ok {
				content = userMsg
			}
		case "assistant":
			content = getAssistantContent(msg.Content)
		case "system":
			if schemaUpdate, ok := msg.Content["schema_update"].(string); ok {
				content = fmt.Sprintf("Database schema update:\n%s", schemaUpdate)
			}
			if ragCtx, ok := msg.Content["rag_context"].(string); ok && ragCtx != "" {
				if content != "" {
					content += "\n\n" + ragCtx
				} else {
					content = ragCtx
				}
			}
		}
		if content != "" {
			rawMessages = append(rawMessages, claudeRawMessage{
				Role: mapClaudeRole(msg.Role),
				Content: []map[string]interface{}{
					{"type": "text", "text": content},
				},
			})
		}
	}

	totalCalls := 0
	var toolHistory []ToolCall
	emptyRetries := 0
	const maxEmptyRetries = 2

	// Iterative tool-calling loop
	for iteration := 0; iteration < maxIterations; iteration++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if config.OnIteration != nil {
			config.OnIteration(iteration, totalCalls)
		}

		// Build request using raw messages
		reqBody := claudeRawRequest{
			Model:       model,
			MaxTokens:   c.maxCompletionTokens,
			Temperature: c.temperature,
			System:      systemPrompt,
			Messages:    rawMessages,
			Tools:       claudeTools,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Claude request: %v", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create Claude request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Claude tool-calling API error at iteration %d: %v", iteration, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read Claude response: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			var errResp claudeErrorResponse
			if err := json.Unmarshal(body, &errResp); err == nil {
				return nil, fmt.Errorf("Claude API error: %s - %s", errResp.Error.Type, errResp.Error.Message)
			}
			return nil, fmt.Errorf("Claude API error: status %d, body: %s", resp.StatusCode, string(body))
		}

		var claudeResp claudeResponse
		if err := json.Unmarshal(body, &claudeResp); err != nil {
			return nil, fmt.Errorf("failed to parse Claude response: %v", err)
		}

		if len(claudeResp.Content) == 0 {
			emptyRetries++
			if emptyRetries > maxEmptyRetries {
				return nil, fmt.Errorf("no content in Claude response after %d retries at iteration %d", maxEmptyRetries, iteration)
			}
			log.Printf("Claude GenerateWithTools -> Empty content at iteration %d, retrying (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
			rawMessages = append(rawMessages, claudeRawMessage{
				Role:    "assistant",
				Content: "I need to continue.",
			})
			rawMessages = append(rawMessages, claudeRawMessage{
				Role:    "user",
				Content: "Your previous response was empty. Please continue \u2014 either call the appropriate tool or call generate_final_response with your answer.",
			})
			continue
		}

		// Collect tool_use blocks and text blocks
		var toolUseBlocks []struct {
			ID    string
			Name  string
			Input map[string]interface{}
		}
		var textContent string

		for _, block := range claudeResp.Content {
			switch block.Type {
			case "tool_use":
				toolUseBlocks = append(toolUseBlocks, struct {
					ID    string
					Name  string
					Input map[string]interface{}
				}{ID: block.ID, Name: block.Name, Input: block.Input})
			case "text":
				textContent += block.Text
			}
		}

		// No tool calls — LLM returned text
		if len(toolUseBlocks) == 0 || claudeResp.StopReason != "tool_use" {
			log.Printf("Claude GenerateWithTools -> Iteration %d: No tool calls (stop_reason=%s)", iteration, claudeResp.StopReason)
			if textContent != "" {
				// Try to detect and parse text that looks like a tool call attempt
				if parsed, ok := TryParseTextToolCall(textContent); ok {
					log.Printf("Claude GenerateWithTools -> Extracted structured response from text tool-call")
					return &ToolCallResult{
						Response:    parsed,
						Iterations:  iteration + 1,
						TotalCalls:  totalCalls,
						ToolHistory: toolHistory,
					}, nil
				}
				// Try raw JSON
				var testJSON map[string]interface{}
				if json.Unmarshal([]byte(textContent), &testJSON) == nil {
					return &ToolCallResult{
						Response:    textContent,
						Iterations:  iteration + 1,
						TotalCalls:  totalCalls,
						ToolHistory: toolHistory,
					}, nil
				}
				// Plain text instead of tool call — nudge LLM to use generate_final_response
				emptyRetries++
				if emptyRetries > maxEmptyRetries {
					log.Printf("Claude GenerateWithTools -> Plain text after %d retries, wrapping as assistantMessage", maxEmptyRetries)
					wrappedResponse, _ := json.Marshal(map[string]interface{}{
						"assistantMessage": textContent,
						"queries":          []interface{}{},
						"actionButtons":    []interface{}{},
					})
					return &ToolCallResult{
						Response:    string(wrappedResponse),
						Iterations:  iteration + 1,
						TotalCalls:  totalCalls,
						ToolHistory: toolHistory,
					}, nil
				}
				log.Printf("Claude GenerateWithTools -> Plain text at iteration %d, nudging to use generate_final_response (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
				rawMessages = append(rawMessages, claudeRawMessage{
					Role:    "assistant",
					Content: textContent,
				})
				rawMessages = append(rawMessages, claudeRawMessage{
					Role:    "user",
					Content: "You returned a plain text response instead of calling the generate_final_response tool. You MUST call generate_final_response with your complete answer including any SQL queries in the 'queries' array. Do not respond with plain text.",
				})
				continue
			}
			// Empty text + no tool calls — nudge the LLM to retry
			emptyRetries++
			if emptyRetries > maxEmptyRetries {
				return nil, fmt.Errorf("empty response from Claude after %d retries at iteration %d", maxEmptyRetries, iteration)
			}
			log.Printf("Claude GenerateWithTools -> Empty text at iteration %d, retrying (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
			// Add assistant + user nudge messages
			rawMessages = append(rawMessages, claudeRawMessage{
				Role:    "assistant",
				Content: "I need to continue.",
			})
			rawMessages = append(rawMessages, claudeRawMessage{
				Role:    "user",
				Content: "Your previous response was empty. Please call generate_final_response with your complete answer, or call an appropriate tool if you need more information.",
			})
			continue
		}

		// Build the assistant content blocks (preserving tool_use blocks exactly as returned)
		assistantContentBlocks := make([]interface{}, 0, len(claudeResp.Content))
		for _, block := range claudeResp.Content {
			blockMap := map[string]interface{}{"type": block.Type}
			if block.Text != "" {
				blockMap["text"] = block.Text
			}
			if block.ID != "" {
				blockMap["id"] = block.ID
			}
			if block.Name != "" {
				blockMap["name"] = block.Name
			}
			if block.Input != nil {
				blockMap["input"] = block.Input
			}
			assistantContentBlocks = append(assistantContentBlocks, blockMap)
		}

		// Add assistant message with tool_use blocks to conversation
		rawMessages = append(rawMessages, claudeRawMessage{
			Role:    "assistant",
			Content: assistantContentBlocks,
		})

		// Process tool calls and build tool_result blocks
		toolResultBlocks := make([]interface{}, 0, len(toolUseBlocks))
		for _, tu := range toolUseBlocks {
			totalCalls++
			call := ToolCall{
				ID:        tu.ID,
				Name:      tu.Name,
				Arguments: tu.Input,
			}
			toolHistory = append(toolHistory, call)

			if config.OnToolCall != nil {
				config.OnToolCall(call)
			}

			// Check if this is the final response tool
			if tu.Name == FinalResponseToolName {
				log.Printf("Claude GenerateWithTools -> Final response tool called at iteration %d", iteration)
				responseJSON, err := json.Marshal(tu.Input)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal final response input: %v", err)
				}
				return &ToolCallResult{
					Response:    string(responseJSON),
					Iterations:  iteration + 1,
					TotalCalls:  totalCalls,
					ToolHistory: toolHistory,
				}, nil
			}

			// Execute the tool
			toolResult, err := executor(ctx, call)
			if err != nil {
				log.Printf("Claude GenerateWithTools -> Tool %s execution error: %v", tu.Name, err)
				toolResult = &ToolResult{
					CallID:  tu.ID,
					Name:    call.Name,
					Content: fmt.Sprintf("Error executing tool: %v", err),
					IsError: true,
				}
			}

			if config.OnToolResult != nil {
				config.OnToolResult(call, *toolResult)
			}

			toolResultBlock := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": tu.ID,
				"content":     toolResult.Content,
			}
			if toolResult.IsError {
				toolResultBlock["is_error"] = true
			}
			toolResultBlocks = append(toolResultBlocks, toolResultBlock)
		}

		// Add user message with tool_result blocks
		rawMessages = append(rawMessages, claudeRawMessage{
			Role:    "user",
			Content: toolResultBlocks,
		})
	}

	// Max iterations reached
	log.Printf("Claude GenerateWithTools -> Max iterations (%d) reached", maxIterations)
	wrappedResponse, _ := json.Marshal(map[string]interface{}{
		"assistantMessage": "I explored the database but reached the maximum number of steps. Please try a more specific question.",
		"queries":          []interface{}{},
		"actionButtons":    []interface{}{},
	})
	return &ToolCallResult{
		Response:    string(wrappedResponse),
		Iterations:  maxIterations,
		TotalCalls:  totalCalls,
		ToolHistory: toolHistory,
	}, nil
}

// Helper function to map roles to Claude format
func mapClaudeRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "user", "system":
		return "user"
	default:
		return "user"
	}
}
