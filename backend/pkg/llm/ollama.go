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

type OllamaClient struct {
	baseURL             string
	model               string
	maxCompletionTokens int
	temperature         float64
	DBConfigs           []LLMDBConfig
	httpClient          *http.Client
}

// Ollama API request/response structures
type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options,omitempty"`
	Format   interface{}     `json:"format,omitempty"` // Can be "json" string or JSON schema object
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"` // max tokens
}

type ollamaResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            ollamaMessage `json:"message"`
	Done               bool          `json:"done"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
}

type ollamaErrorResponse struct {
	Error string `json:"error"`
}

func NewOllamaClient(config Config) (*OllamaClient, error) {
	baseURL := config.APIKey // For Ollama, we use APIKey field to store base URL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := config.Model
	if model == "" {
		model = "llama3.1:latest" // Default model
	}

	return &OllamaClient{
		baseURL:             baseURL,
		model:               model,
		maxCompletionTokens: config.MaxCompletionTokens,
		temperature:         config.Temperature,
		DBConfigs:           config.DBConfigs,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // Ollama can be slower on CPU
		},
	}, nil
}

func (c *OllamaClient) GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string, nonTechMode bool, modelID ...string) (string, error) {
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Use provided model if specified, otherwise use the client's default model
	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("Ollama GenerateResponse -> Using selected model: %s", model)
	}

	// Get the system prompt with non-tech mode if enabled
	systemPrompt := constants.GetSystemPrompt(constants.Ollama, dbType, nonTechMode)
	responseSchemaJSON := ""

	for _, dbConfig := range c.DBConfigs {
		if dbConfig.DBType == dbType {
			responseSchemaJSON = dbConfig.Schema.(string)
			break
		}
	}

	// Parse the JSON schema string into a map for Ollama's format parameter
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(responseSchemaJSON), &schemaMap); err != nil {
		return "", fmt.Errorf("failed to parse response schema: %v", err)
	}

	// Convert messages to Ollama format
	ollamaMessages := make([]ollamaMessage, 0, len(messages)+1)

	// Add system message first
	ollamaMessages = append(ollamaMessages, ollamaMessage{
		Role:    "system",
		Content: systemPrompt,
	})

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
			ollamaMessages = append(ollamaMessages, ollamaMessage{
				Role:    mapOllamaRole(msg.Role),
				Content: content,
			})
		}
	}

	// Create request
	reqBody := ollamaRequest{
		Model:    model,
		Messages: ollamaMessages,
		Stream:   false,
		Format:   schemaMap, // Use JSON schema object for structured outputs
		Options: ollamaOptions{
			Temperature: c.temperature,
			NumPredict:  c.maxCompletionTokens,
		},
	}

	// Marshal request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama API error: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp ollamaErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return "", fmt.Errorf("Ollama API error: %s", errResp.Error)
		}
		return "", fmt.Errorf("Ollama API error: status code %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if ollamaResp.Message.Content == "" {
		return "", fmt.Errorf("no content in response")
	}

	return ollamaResp.Message.Content, nil
}

func (c *OllamaClient) GenerateRecommendations(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error) {
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Get recommendations-specific prompt and schema
	systemPrompt := constants.GetRecommendationsPrompt(constants.Ollama)
	responseSchemaJSON := constants.GetRecommendationsSchema(constants.Ollama).(string)

	// Parse the JSON schema string into a map for Ollama's format parameter
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(responseSchemaJSON), &schemaMap); err != nil {
		return "", fmt.Errorf("failed to parse recommendations schema: %v", err)
	}

	// Convert messages to Ollama format
	ollamaMessages := make([]ollamaMessage, 0, len(messages)+1)

	// Add system message first
	ollamaMessages = append(ollamaMessages, ollamaMessage{
		Role:    "system",
		Content: systemPrompt,
	})

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
			ollamaMessages = append(ollamaMessages, ollamaMessage{
				Role:    mapOllamaRole(msg.Role),
				Content: content,
			})
		}
	}

	// Create request
	reqBody := ollamaRequest{
		Model:    c.model,
		Messages: ollamaMessages,
		Stream:   false,
		Format:   schemaMap, // Use JSON schema object for structured recommendations
		Options: ollamaOptions{
			Temperature: c.temperature,
			NumPredict:  c.maxCompletionTokens,
		},
	}

	// Marshal request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama API error: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp ollamaErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return "", fmt.Errorf("Ollama API error: %s", errResp.Error)
		}
		return "", fmt.Errorf("Ollama API error: status code %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if ollamaResp.Message.Content == "" {
		return "", fmt.Errorf("no content in response")
	}

	return ollamaResp.Message.Content, nil
}

// GenerateVisualization generates a visualization configuration for query results
// This method uses a dedicated visualization system prompt and enforces JSON response format
func (c *OllamaClient) GenerateVisualization(ctx context.Context, systemPrompt string, visualizationPrompt string, dataRequest string, modelID ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("Ollama GenerateVisualization -> Using selected model: %s", model)
	}

	// Build the messages for visualization
	messages := make([]ollamaMessage, 0)

	// Add system prompt
	messages = append(messages, ollamaMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add visualization prompt
	messages = append(messages, ollamaMessage{
		Role:    "user",
		Content: visualizationPrompt,
	})

	// Add data request
	messages = append(messages, ollamaMessage{
		Role:    "user",
		Content: dataRequest,
	})

	// Create request with JSON format enforcement
	req := ollamaRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Options: ollamaOptions{
			Temperature: c.temperature,
			NumPredict:  c.maxCompletionTokens,
		},
		Format: "json",
	}

	// Prepare request body
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	log.Printf("Ollama GenerateVisualization -> Request: %s", string(reqBody))

	// Make API request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/chat", c.baseURL), bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Add timeout
	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(reqCtx)

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

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp ollamaErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return "", fmt.Errorf("Ollama API error: %s", errResp.Error)
		}
		return "", fmt.Errorf("Ollama API error: status code %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if ollamaResp.Message.Content == "" {
		return "", fmt.Errorf("no content in response")
	}

	responseText := ollamaResp.Message.Content
	log.Printf("OLLAMA -> GenerateVisualization -> responseText: %s", responseText)

	// Validate JSON response
	var visualizationResponse map[string]interface{}
	if err := json.Unmarshal([]byte(responseText), &visualizationResponse); err != nil {
		log.Printf("Error: Ollama visualization response is not valid JSON: %v", err)
		return "", fmt.Errorf("invalid JSON response from Ollama: %v", err)
	}

	return responseText, nil
}

func (c *OllamaClient) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:                c.model,
		Provider:            constants.Ollama,
		MaxCompletionTokens: c.maxCompletionTokens,
		ContextLimit:        128000, // Most Ollama models support 128K context
	}
}

func (c *OllamaClient) SetModel(modelID string) error {
	c.model = modelID
	return nil
}

// Helper function to map roles to Ollama format
func mapOllamaRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "user":
		return "user"
	case "system":
		return "system"
	default:
		return "user"
	}
}
