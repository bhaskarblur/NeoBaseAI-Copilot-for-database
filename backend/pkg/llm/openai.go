package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"

	"github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	client              *openai.Client
	model               string
	maxCompletionTokens int
	temperature         float64
	DBConfigs           []LLMDBConfig
}

func NewOpenAIClient(config Config) (*OpenAIClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(config.APIKey)
	model := config.Model
	if model == "" {
		model = openai.GPT4o
	}

	return &OpenAIClient{
		client:              client,
		model:               model,
		maxCompletionTokens: config.MaxCompletionTokens,
		temperature:         config.Temperature,
		DBConfigs:           config.DBConfigs,
	}, nil
}

func (c *OpenAIClient) GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string, nonTechMode bool, modelID ...string) (string, error) {
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Use provided model if specified, otherwise use the client's default model
	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("OpenAI GenerateResponse -> Using selected model: %s", model)
	}

	// Convert messages to OpenAI format
	openAIMessages := make([]openai.ChatCompletionMessage, 0, len(messages))

	// Get the system prompt with non-tech mode if enabled
	log.Printf("OpenAI GenerateResponse -> nonTechMode parameter: %v", nonTechMode)
	systemPrompt := constants.GetSystemPrompt(constants.OpenAI, dbType, nonTechMode)
	responseSchema := ""

	for _, dbConfig := range c.DBConfigs {
		if dbConfig.DBType == dbType {
			// Use the dynamically generated prompt instead of the stored one
			// systemPrompt = dbConfig.SystemPrompt
			responseSchema = dbConfig.Schema.(string)
			break
		}
	}

	// Add system message with database-specific prompt only
	openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// log.Printf("OPENAI -> GenerateResponse -> messages: %v", messages)

	for _, msg := range messages {
		content := ""

		// Handle different message types
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
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:    mapRole(msg.Role),
				Content: content,
			})
		}
	}

	// Create completion request with JSON schema
	req := openai.ChatCompletionRequest{
		Model:               model,
		Messages:            openAIMessages,
		MaxCompletionTokens: c.maxCompletionTokens,
		Temperature:         float32(c.temperature),
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:        "neobase-response",
				Description: "A friendly AI Response/Explanation or clarification question (Must Send this)",
				Schema:      json.RawMessage(responseSchema),
				Strict:      false,
			},
		},
	}

	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Call OpenAI API
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("GenerateResponse -> err: %v", err)
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	log.Printf("OPENAI -> GenerateResponse -> resp: %v", resp)
	// Validate response against schema
	var llmResponse constants.LLMResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &llmResponse); err != nil {
		return "", fmt.Errorf("invalid response format: %v", err)
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateRawJSON generates a response with a custom system prompt and no response schema.
// Used for tasks like KB generation that need raw JSON output.
func (c *OpenAIClient) GenerateRawJSON(ctx context.Context, systemPrompt string, userMessage string, modelID ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("OpenAI GenerateRawJSON -> Using selected model: %s", model)
	}

	openAIMessages := []openai.ChatCompletionMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	req := openai.ChatCompletionRequest{
		Model:               model,
		Messages:            openAIMessages,
		MaxCompletionTokens: c.maxCompletionTokens,
		Temperature:         float32(c.temperature),
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("OpenAI GenerateRawJSON error: %v", err)
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	responseText := resp.Choices[0].Message.Content
	log.Printf("OpenAI GenerateRawJSON -> response length: %d", len(responseText))
	return responseText, nil
}

// GenerateRecommendations generates query recommendations using a different prompt and schema
func (c *OpenAIClient) GenerateRecommendations(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error) {
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Convert messages to OpenAI format
	openAIMessages := make([]openai.ChatCompletionMessage, 0, len(messages))

	systemPrompt := constants.GetRecommendationsPrompt(constants.OpenAI)
	responseSchema := constants.GetRecommendationsSchema(constants.OpenAI).(string)

	// Add system message with recommendations-specific prompt
	openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	for _, msg := range messages {
		content := ""

		// Handle different message types
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
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:    mapRole(msg.Role),
				Content: content,
			})
		}
	}

	// Create completion request with JSON schema for recommendations
	req := openai.ChatCompletionRequest{
		Model:               c.model,
		Messages:            openAIMessages,
		MaxCompletionTokens: c.maxCompletionTokens,
		Temperature:         float32(c.temperature),
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:        "recommendations-response",
				Description: "Query recommendations response",
				Schema:      json.RawMessage(responseSchema),
				Strict:      false,
			},
		},
	}

	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Call OpenAI API
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("GenerateRecommendations -> err: %v", err)
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	log.Printf("OPENAI -> GenerateRecommendations -> resp: %v", resp)
	return resp.Choices[0].Message.Content, nil
}

// GenerateVisualization generates a visualization configuration for query results
// This method uses a dedicated visualization system prompt and enforces JSON response format
func (c *OpenAIClient) GenerateVisualization(ctx context.Context, systemPrompt string, visualizationPrompt string, dataRequest string, modelID ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("OpenAI GenerateVisualization -> Using selected model: %s", model)
	}

	// Create messages for visualization request
	messages := make([]openai.ChatCompletionMessage, 0)

	// Add system message with visualization prompt
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add visualization prompt
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    "user",
		Content: visualizationPrompt,
	})

	// Add data request
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    "user",
		Content: dataRequest,
	})

	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Call OpenAI API with JSON response format enforcement
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   c.maxCompletionTokens,
		Temperature: float32(c.temperature),
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		log.Printf("GenerateVisualization -> OpenAI API error: %v", err)
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	responseText := resp.Choices[0].Message.Content
	log.Printf("OPENAI -> GenerateVisualization -> responseText: %s", responseText)

	// Validate JSON response
	var visualizationResponse map[string]interface{}
	if err := json.Unmarshal([]byte(responseText), &visualizationResponse); err != nil {
		log.Printf("Error: OpenAI visualization response is not valid JSON: %v", err)
		return "", fmt.Errorf("invalid JSON response from OpenAI: %v", err)
	}

	return responseText, nil
}

func (c *OpenAIClient) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:                c.model,
		Provider:            "openai",
		MaxCompletionTokens: c.maxCompletionTokens,
	}
}

// SetModel updates the model used by the client
func (c *OpenAIClient) SetModel(modelID string) error {
	if modelID == "" {
		return fmt.Errorf("model ID cannot be empty")
	}
	c.model = modelID
	log.Printf("OpenAI client model updated to: %s", modelID)
	return nil
}

// GenerateWithTools implements iterative tool-calling using OpenAI's native function calling.
func (c *OpenAIClient) GenerateWithTools(ctx context.Context, messages []*models.LLMMessage, tools []ToolDefinition, executor ToolExecutorFunc, config ToolCallConfig) (*ToolCallResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	model := c.model
	if config.ModelID != "" {
		model = config.ModelID
		log.Printf("OpenAI GenerateWithTools -> Using selected model: %s", model)
	}

	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = DefaultMaxIterations
	}

	// Build system prompt: always include DB-specific prompt, then append tool-calling addendum
	systemPrompt := constants.GetSystemPrompt(constants.OpenAI, config.DBType, config.NonTechMode)
	if config.SystemPrompt != "" {
		systemPrompt = systemPrompt + "\n\n" + config.SystemPrompt
	}

	// Convert tool definitions to OpenAI tools
	openAITools := make([]openai.Tool, 0, len(tools))
	for _, tool := range tools {
		paramsJSON, _ := json.Marshal(tool.Parameters)
		openAITools = append(openAITools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  json.RawMessage(paramsJSON),
			},
		})
	}

	// Build initial messages
	openAIMessages := make([]openai.ChatCompletionMessage, 0, len(messages)+1)
	openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
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
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:    mapRole(msg.Role),
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

		req := openai.ChatCompletionRequest{
			Model:               model,
			Messages:            openAIMessages,
			MaxCompletionTokens: c.maxCompletionTokens,
			Temperature:         float32(c.temperature),
			Tools:               openAITools,
		}

		resp, err := c.client.CreateChatCompletion(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("OpenAI tool-calling API error at iteration %d: %v", iteration, err)
		}

		if len(resp.Choices) == 0 {
			emptyRetries++
			if emptyRetries > maxEmptyRetries {
				return nil, fmt.Errorf("no response from OpenAI after %d retries at iteration %d", maxEmptyRetries, iteration)
			}
			log.Printf("OpenAI GenerateWithTools -> Empty response at iteration %d, retrying (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "Your previous response was empty. Please continue — either call the appropriate tool or call generate_final_response with your answer.",
			})
			continue
		}

		choice := resp.Choices[0]

		// No tool calls — LLM returned final content
		if len(choice.Message.ToolCalls) == 0 {
			log.Printf("OpenAI GenerateWithTools -> Iteration %d: No tool calls, got text response", iteration)
			content := choice.Message.Content
			if content != "" {
				// Try to detect and parse text that looks like a tool call attempt
				if parsed, ok := TryParseTextToolCall(content); ok {
					log.Printf("OpenAI GenerateWithTools -> Extracted structured response from text tool-call")
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
					log.Printf("OpenAI GenerateWithTools -> Plain text after %d retries, wrapping as assistantMessage", maxEmptyRetries)
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
				log.Printf("OpenAI GenerateWithTools -> Plain text at iteration %d, nudging to use generate_final_response (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
				openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: content,
				})
				openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: "You returned a plain text response instead of calling the generate_final_response tool. You MUST call generate_final_response with your complete answer including any SQL queries in the 'queries' array. Do not respond with plain text.",
				})
				continue
			}
			// Empty text + no tool calls — nudge the LLM to retry
			emptyRetries++
			if emptyRetries > maxEmptyRetries {
				return nil, fmt.Errorf("empty response from OpenAI after %d retries at iteration %d", maxEmptyRetries, iteration)
			}
			log.Printf("OpenAI GenerateWithTools -> Empty text at iteration %d, retrying (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "",
			})
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "Your previous response was empty. Please call generate_final_response with your complete answer, or call an appropriate tool if you need more information.",
			})
			continue
		}

		// Add the assistant message (with tool calls) to conversation
		openAIMessages = append(openAIMessages, choice.Message)

		// Process tool calls
		for _, tc := range choice.Message.ToolCalls {
			totalCalls++

			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]interface{}{"raw": tc.Function.Arguments}
			}

			call := ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			}
			toolHistory = append(toolHistory, call)

			if config.OnToolCall != nil {
				config.OnToolCall(call)
			}

			// Check if this is the final response tool
			if tc.Function.Name == FinalResponseToolName {
				log.Printf("OpenAI GenerateWithTools -> Final response tool called at iteration %d", iteration)
				return &ToolCallResult{
					Response:    tc.Function.Arguments,
					Iterations:  iteration + 1,
					TotalCalls:  totalCalls,
					ToolHistory: toolHistory,
				}, nil
			}

			// Execute the tool
			toolResult, err := executor(ctx, call)
			if err != nil {
				log.Printf("OpenAI GenerateWithTools -> Tool %s execution error: %v", tc.Function.Name, err)
				toolResult = &ToolResult{
					CallID:  tc.ID,
					Name:    call.Name,
					Content: fmt.Sprintf("Error executing tool: %v", err),
					IsError: true,
				}
			}

			if config.OnToolResult != nil {
				config.OnToolResult(call, *toolResult)
			}

			// Add tool result to conversation
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    toolResult.Content,
			})
		}

		// Reset empty-retry budget after successful tool execution so that
		// each phase (exploration vs. final-response) gets its own retries.
		emptyRetries = 0
	}

	// Max iterations reached
	log.Printf("OpenAI GenerateWithTools -> Max iterations (%d) reached", maxIterations)
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
