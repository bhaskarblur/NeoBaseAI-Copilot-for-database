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
			content = formatAssistantResponse(msg.Content["assistant_response"].(map[string]interface{}))
			// Add non-tech mode context if the mode differs from current request
			if msg.NonTechMode != nonTechMode {
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
			content = formatAssistantResponse(msg.Content["assistant_response"].(map[string]interface{}))
		case "system":
			if schemaUpdate, ok := msg.Content["schema_update"].(string); ok {
				content = fmt.Sprintf("Database schema update:\n%s", schemaUpdate)
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
