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

// GenerateRawJSON generates a response with a custom system prompt and no response schema.
// Used for tasks like KB generation that need raw JSON output.
func (c *OllamaClient) GenerateRawJSON(ctx context.Context, systemPrompt string, userMessage string, modelID ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("Ollama GenerateRawJSON -> Using selected model: %s", model)
	}

	ollamaMessages := []ollamaMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	reqBody := ollamaRequest{
		Model:    model,
		Messages: ollamaMessages,
		Stream:   false,
		Format:   "json", // Request JSON output without enforcing a specific schema
		Options: ollamaOptions{
			Temperature: c.temperature,
			NumPredict:  c.maxCompletionTokens,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama API error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ollamaErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return "", fmt.Errorf("Ollama API error: %s", errResp.Error)
		}
		return "", fmt.Errorf("Ollama API error: status code %d", resp.StatusCode)
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	log.Printf("Ollama GenerateRawJSON -> response length: %d", len(ollamaResp.Message.Content))
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

// Ollama tool-calling request/response structures
type ollamaToolCallRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaToolMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  ollamaOptions       `json:"options,omitempty"`
	Tools    []ollamaToolDef     `json:"tools,omitempty"`
}

type ollamaToolMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolDef struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ollamaToolCall struct {
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaToolCallFunction struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ollamaToolCallResponse struct {
	Model           string            `json:"model"`
	CreatedAt       string            `json:"created_at"`
	Message         ollamaToolMessage `json:"message"`
	Done            bool              `json:"done"`
	TotalDuration   int64             `json:"total_duration,omitempty"`
	PromptEvalCount int               `json:"prompt_eval_count,omitempty"`
	EvalCount       int               `json:"eval_count,omitempty"`
}

// GenerateWithTools implements iterative tool-calling using Ollama's tool support.
// Note: Not all Ollama models support tool calling. Falls back to structured output if tools don't work.
func (c *OllamaClient) GenerateWithTools(ctx context.Context, messages []*models.LLMMessage, tools []ToolDefinition, executor ToolExecutorFunc, config ToolCallConfig) (*ToolCallResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	model := c.model
	if config.ModelID != "" {
		model = config.ModelID
		log.Printf("Ollama GenerateWithTools -> Using selected model: %s", model)
	}

	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = DefaultMaxIterations
	}

	// Build system prompt: always include DB-specific prompt, then append tool-calling addendum
	systemPrompt := constants.GetSystemPrompt(constants.Ollama, config.DBType, config.NonTechMode)
	if config.SystemPrompt != "" {
		systemPrompt = systemPrompt + "\n\n" + config.SystemPrompt
	}

	// Convert tool definitions to Ollama format
	ollamaTools := make([]ollamaToolDef, 0, len(tools))
	for _, tool := range tools {
		ollamaTools = append(ollamaTools, ollamaToolDef{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	// Build initial messages
	ollamaMessages := make([]ollamaToolMessage, 0, len(messages)+1)
	ollamaMessages = append(ollamaMessages, ollamaToolMessage{
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
			ollamaMessages = append(ollamaMessages, ollamaToolMessage{
				Role:    mapOllamaRole(msg.Role),
				Content: content,
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

		reqBody := ollamaToolCallRequest{
			Model:    model,
			Messages: ollamaMessages,
			Stream:   false,
			Options: ollamaOptions{
				Temperature: c.temperature,
				NumPredict:  c.maxCompletionTokens,
			},
			Tools: ollamaTools,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Ollama request: %v", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create Ollama request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Ollama tool-calling API error at iteration %d: %v", iteration, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read Ollama response: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Ollama API error: status %d, body: %s", resp.StatusCode, string(body))
		}

		var ollamaResp ollamaToolCallResponse
		if err := json.Unmarshal(body, &ollamaResp); err != nil {
			return nil, fmt.Errorf("failed to parse Ollama response: %v", err)
		}

		// No tool calls — return text content
		if len(ollamaResp.Message.ToolCalls) == 0 {
			log.Printf("Ollama GenerateWithTools -> Iteration %d: No tool calls, got text response", iteration)
			content := ollamaResp.Message.Content
			if content != "" {
				// Try to detect and parse text that looks like a tool call attempt
				if parsed, ok := TryParseTextToolCall(content); ok {
					log.Printf("Ollama GenerateWithTools -> Extracted structured response from text tool-call")
					return &ToolCallResult{
						Response:    parsed,
						Iterations:  iteration + 1,
						TotalCalls:  totalCalls,
						ToolHistory: toolHistory,
					}, nil
				}
				// Try raw JSON
				var testJSON map[string]interface{}
				if json.Unmarshal([]byte(content), &testJSON) == nil {
					return &ToolCallResult{
						Response:    content,
						Iterations:  iteration + 1,
						TotalCalls:  totalCalls,
						ToolHistory: toolHistory,
					}, nil
				}
				// Plain text instead of tool call — nudge LLM to use generate_final_response
				emptyRetries++
				if emptyRetries > maxEmptyRetries {
					log.Printf("Ollama GenerateWithTools -> Plain text after %d retries, wrapping as assistantMessage", maxEmptyRetries)
					wrappedResponse, _ := json.Marshal(map[string]interface{}{
						"assistantMessage": content,
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
				log.Printf("Ollama GenerateWithTools -> Plain text at iteration %d, nudging to use generate_final_response (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
				ollamaMessages = append(ollamaMessages, ollamaToolMessage{
					Role:    "assistant",
					Content: content,
				})
				ollamaMessages = append(ollamaMessages, ollamaToolMessage{
					Role:    "user",
					Content: "You returned a plain text response instead of calling the generate_final_response tool. You MUST call generate_final_response with your complete answer including any SQL queries in the 'queries' array. Do not respond with plain text.",
				})
				continue
			}
			// Empty text + no tool calls — nudge the LLM to retry
			emptyRetries++
			if emptyRetries > maxEmptyRetries {
				return nil, fmt.Errorf("empty response from Ollama after %d retries at iteration %d", maxEmptyRetries, iteration)
			}
			log.Printf("Ollama GenerateWithTools -> Empty text at iteration %d, retrying (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
			ollamaMessages = append(ollamaMessages, ollamaToolMessage{
				Role:    "assistant",
				Content: "I need to continue.",
			})
			ollamaMessages = append(ollamaMessages, ollamaToolMessage{
				Role:    "user",
				Content: "Your previous response was empty. Please call generate_final_response with your complete answer, or call an appropriate tool if you need more information.",
			})
			continue
		}

		// Add assistant message with tool calls to conversation
		ollamaMessages = append(ollamaMessages, ollamaResp.Message)

		// Process tool calls
		for _, tc := range ollamaResp.Message.ToolCalls {
			totalCalls++
			call := ToolCall{
				ID:        fmt.Sprintf("ollama-%d-%d", iteration, totalCalls),
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
			toolHistory = append(toolHistory, call)

			if config.OnToolCall != nil {
				config.OnToolCall(call)
			}

			// Check if this is the final response tool
			if tc.Function.Name == FinalResponseToolName {
				log.Printf("Ollama GenerateWithTools -> Final response tool called at iteration %d", iteration)
				responseJSON, err := json.Marshal(tc.Function.Arguments)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal final response args: %v", err)
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
				log.Printf("Ollama GenerateWithTools -> Tool %s execution error: %v", tc.Function.Name, err)
				toolResult = &ToolResult{
					CallID:  call.ID,
					Name:    call.Name,
					Content: fmt.Sprintf("Error executing tool: %v", err),
					IsError: true,
				}
			}

			if config.OnToolResult != nil {
				config.OnToolResult(call, *toolResult)
			}

			// Add tool result as a tool message
			ollamaMessages = append(ollamaMessages, ollamaToolMessage{
				Role:    "tool",
				Content: toolResult.Content,
			})
		}
	}

	// Max iterations reached
	log.Printf("Ollama GenerateWithTools -> Max iterations (%d) reached", maxIterations)
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
