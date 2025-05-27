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
	"regexp"
	"strings"
)

type OllamaClient struct {
	baseURL             string
	model               string
	maxCompletionTokens int
	temperature         float64
	DBConfigs           []LLMDBConfig
}

type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  OllamaOptions   `json:"options"`
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaOptions struct {
	Temperature float64 `json:"temperature"`
	NumPredict  int     `json:"num_predict"`
}

type OllamaResponse struct {
	Model   string        `json:"model"`
	Message OllamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

func NewOllamaClient(config Config) (*OllamaClient, error) {
	baseURL := "http://localhost:11434" // Default Ollama API endpoint
	if config.APIKey != "" {
		baseURL = config.APIKey // We'll use APIKey field to store custom Ollama URL
	}

	return &OllamaClient{
		baseURL:             baseURL,
		model:               config.Model,
		maxCompletionTokens: config.MaxCompletionTokens,
		temperature:         config.Temperature,
		DBConfigs:           config.DBConfigs,
	}, nil
}

func (c *OllamaClient) GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Get system prompt and schema for the database type
	systemPrompt := ""

	for _, dbConfig := range c.DBConfigs {
		if dbConfig.DBType == dbType {
			systemPrompt = dbConfig.SystemPrompt
			break
		}
	}

	// Convert messages to Ollama format
	ollamaMessages := make([]OllamaMessage, 0)

	// Add system message first with explicit JSON formatting instruction
	systemPrompt = systemPrompt + "\n\nCRITICAL INSTRUCTION: You MUST respond with ONLY a valid JSON object that strictly follows the schema above. Your response MUST include all required fields: assistantMessage, queries (array), and optionally actionButtons. Do not include any other text, markdown, or HTML in your response. Your entire response must be a single JSON object starting with { and ending with }. Do not include any explanations or additional text."
	ollamaMessages = append(ollamaMessages, OllamaMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add conversation history
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
			ollamaMessages = append(ollamaMessages, OllamaMessage{
				Role:    mapRole(msg.Role),
				Content: content,
			})
		}
	}

	// Add a final instruction message to reinforce JSON formatting
	ollamaMessages = append(ollamaMessages, OllamaMessage{
		Role:    "system",
		Content: "Remember: Your response must be ONLY a valid JSON object with all required fields: assistantMessage, queries (array), and optionally actionButtons. Do not include any other text or explanations.",
	})

	// Create request
	req := OllamaRequest{
		Model:    c.model,
		Messages: ollamaMessages,
		Stream:   false,
		Options: OllamaOptions{
			Temperature: c.temperature,
			NumPredict:  c.maxCompletionTokens,
		},
	}

	// Convert request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/chat", c.baseURL), bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Log raw response for debugging
	log.Printf("Ollama raw response: %s", string(body))

	// Parse response
	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode Ollama response: %v", err)
	}

	// Validate that we got a response
	if ollamaResp.Message.Content == "" {
		return "", fmt.Errorf("empty response from Ollama")
	}

	// Try to parse the content as JSON
	var llmResponse constants.LLMResponse
	content := ollamaResp.Message.Content

	// First, try to clean the response
	cleanedContent := cleanJSONResponse(content)

	// Try parsing the cleaned content
	if err := json.Unmarshal([]byte(cleanedContent), &llmResponse); err != nil {
		// If that fails, try to extract JSON from the response
		jsonStart := strings.Index(content, "{")
		jsonEnd := strings.LastIndex(content, "}")

		if jsonStart >= 0 && jsonEnd > jsonStart {
			extractedJSON := content[jsonStart : jsonEnd+1]
			if err := json.Unmarshal([]byte(extractedJSON), &llmResponse); err != nil {
				// If we still can't parse it, try to fix the queries format
				var rawResponse map[string]interface{}
				if err := json.Unmarshal([]byte(extractedJSON), &rawResponse); err == nil {
					if queries, ok := rawResponse["queries"].([]interface{}); ok {
						// Convert string queries to proper QueryInfo objects
						queryInfos := make([]constants.QueryInfo, 0)
						for _, q := range queries {
							if queryStr, ok := q.(string); ok {
								queryInfos = append(queryInfos, constants.QueryInfo{
									Query:                queryStr,
									QueryType:            "SELECT",
									Explanation:          "Query generated from user request",
									IsCritical:           false,
									CanRollback:          false,
									EstimateResponseTime: 100,
								})
							}
						}
						rawResponse["queries"] = queryInfos
						if fixedJSON, err := json.Marshal(rawResponse); err == nil {
							if err := json.Unmarshal(fixedJSON, &llmResponse); err == nil {
								return string(fixedJSON), nil
							}
						}
					}
				}
				return "", fmt.Errorf("invalid response format: %v. Raw response: %s", err, content)
			}
			return extractedJSON, nil
		}

		return "", fmt.Errorf("invalid response format: %v. Raw response: %s", err, content)
	}

	// Validate required fields
	if llmResponse.AssistantMessage == "" {
		llmResponse.AssistantMessage = "I'll help you with your database query."
	}
	if llmResponse.Queries == nil {
		llmResponse.Queries = make([]constants.QueryInfo, 0)
	}

	// Convert back to JSON to ensure all fields are present
	finalJSON, err := json.Marshal(llmResponse)
	if err != nil {
		return "", fmt.Errorf("failed to marshal final response: %v", err)
	}

	return string(finalJSON), nil
}

// cleanJSONResponse attempts to clean and fix common JSON formatting issues
func cleanJSONResponse(content string) string {
	// Remove any leading/trailing whitespace
	content = strings.TrimSpace(content)

	// If the response starts with markdown code block, remove it
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")

	// Remove any HTML-like tags
	content = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(content, "")

	// Remove any non-JSON text before the first {
	firstBrace := strings.Index(content, "{")
	if firstBrace > 0 {
		content = content[firstBrace:]
	}

	// Remove any non-JSON text after the last }
	lastBrace := strings.LastIndex(content, "}")
	if lastBrace > 0 && lastBrace < len(content)-1 {
		content = content[:lastBrace+1]
	}

	// Remove any explanatory text that might be in the JSON
	content = regexp.MustCompile(`(?m)^[^{]*`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`(?m)[^}]*$`).ReplaceAllString(content, "")

	return content
}

func (c *OllamaClient) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:                c.model,
		Provider:            "ollama",
		MaxCompletionTokens: c.maxCompletionTokens,
	}
}
