package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/internal/utils"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiClient struct {
	client              *genai.Client
	model               string
	maxCompletionTokens int
	temperature         float64
	DBConfigs           []LLMDBConfig
}

// safeSendMessage wraps session.SendMessage with panic recovery.
// The Gemini SDK panics with a nil pointer dereference when the API
// returns 0 candidates (it accesses Candidates[0].Content unconditionally).
func safeSendMessage(session *genai.ChatSession, ctx context.Context, parts ...genai.Part) (result *genai.GenerateContentResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Gemini safeSendMessage -> recovered from panic: %v", r)
			result = nil
			err = fmt.Errorf("gemini SendMessage returned empty candidates (recovered from SDK panic)")
		}
	}()
	return session.SendMessage(ctx, parts...)
}

func NewGeminiClient(config Config) (*GeminiClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("gemini API key is required")
	}
	// Create the Gemini SDK client using the provided API key.
	client, err := genai.NewClient(context.Background(), option.WithAPIKey(config.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %v", err)
	}
	maxCompletionTokens := config.MaxCompletionTokens
	temperature := config.Temperature
	DBConfigs := config.DBConfigs

	return &GeminiClient{
		client:              client,
		model:               config.Model,
		maxCompletionTokens: maxCompletionTokens,
		temperature:         temperature,
		DBConfigs:           DBConfigs,
	}, nil
}

func (c *GeminiClient) GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string, nonTechMode bool, modelID ...string) (string, error) {
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Use provided model if specified, otherwise use the client's default model
	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("Gemini GenerateResponse -> Using selected model: %s", model)
	}

	// Convert messages into parts for the Gemini API.
	geminiMessages := make([]*genai.Content, 0)

	// Get the system prompt with non-tech mode if enabled
	systemPrompt := constants.GetSystemPrompt(constants.Gemini, dbType, nonTechMode)
	var responseSchema *genai.Schema

	for _, dbConfig := range c.DBConfigs {
		if dbConfig.DBType == dbType {
			// Use the dynamically generated prompt instead of the stored one
			// systemPrompt = dbConfig.SystemPrompt
			responseSchema = dbConfig.Schema.(*genai.Schema)
			break
		}
	}

	// Add system message first
	geminiMessages = append(geminiMessages, &genai.Content{
		Role: "user",
		Parts: []genai.Part{
			genai.Text(systemPrompt),
		},
	})
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	// Add conversation history
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
			role := "user"
			if msg.Role == "assistant" {
				role = "model"
			}

			geminiMessages = append(geminiMessages, &genai.Content{
				Role: role,
				Parts: []genai.Part{
					genai.Text(content),
				},
			})
		}
	}

	// for _, msg := range geminiMessages {
	// 	log.Printf("GEMINI -> GenerateResponse -> msg: %v", msg)
	// }
	// Build the request with a single content bundle.
	// Call Gemini's content generation API.

	// Get the API version for the model (default to v1beta if not specified)
	apiVersion := "v1beta"
	if llmModel := constants.GetLLMModel(model); llmModel != nil && llmModel.APIVersion != "" {
		apiVersion = llmModel.APIVersion
	}

	// Construct the model name with API version (e.g., "models/gemini-3-pro" for v1beta or v1alpha)
	modelName := fmt.Sprintf("models/%s", model)
	log.Printf("Gemini GenerateResponse -> Using model: %s with API version: %s", modelName, apiVersion)

	geminiModel := c.client.GenerativeModel(modelName)
	geminiModel.MaxOutputTokens = utils.ToInt32Ptr(int32(c.maxCompletionTokens))
	geminiModel.SetTemperature(float32(c.temperature))
	geminiModel.ResponseMIMEType = "application/json"
	geminiModel.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	geminiModel.ResponseSchema = responseSchema
	geminiModel.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
	}

	// Start chat session
	session := geminiModel.StartChat()
	session.History = geminiMessages

	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	// Send empty message to get response based on history
	result, err := safeSendMessage(session, ctx, genai.Text("Please provide a response based on our conversation history."))
	if err != nil {
		log.Printf("Gemini API error: %v", err)
		return "", fmt.Errorf("gemini API error: %v", err)
	}

	log.Printf("GEMINI -> GenerateResponse -> result: %v", result)
	log.Printf("GEMINI -> GenerateResponse -> result.Candidates[0].Content.Parts[0]: %v", result.Candidates[0].Content.Parts[0])
	responseText := strings.ReplaceAll(fmt.Sprintf("%v", result.Candidates[0].Content.Parts[0]), "```json", "")
	responseText = strings.ReplaceAll(responseText, "```", "")

	var llmResponse constants.LLMResponse
	if err := json.Unmarshal([]byte(responseText), &llmResponse); err != nil {
		log.Printf("Warning: Gemini response didn't match expected JSON schema: %v", err)
		return "", fmt.Errorf("invalid JSON response: %v", err)
	}

	var mapResponse map[string]interface{}
	if err := json.Unmarshal([]byte(responseText), &mapResponse); err != nil {
		log.Printf("Warning: Gemini response didn't match expected JSON schema: %v", err)
		return "", fmt.Errorf("invalid JSON response: %v", err)
	}

	temporaryQueries := []map[string]interface{}{}
	if mapResponse["queries"] != nil {
		for _, v := range mapResponse["queries"].([]interface{}) {
			value := v.(map[string]interface{})
			log.Printf("gemini responseMap loop queries: %v", value)
			var exampleResult []map[string]interface{}
			if value["exampleResultString"] != nil && value["exampleResultString"] != "" {
				if err := json.Unmarshal([]byte(value["exampleResultString"].(string)), &exampleResult); err == nil {
					value["exampleResult"] = exampleResult
				}
			}
			temporaryQueries = append(temporaryQueries, value)
		}
	}

	mapResponse["queries"] = temporaryQueries

	convertedResponseText, err := json.Marshal(mapResponse)
	if err != nil {
		log.Printf("marshal map err: %v", err)
		return responseText, nil
	}
	return string(convertedResponseText), nil
}

// GenerateRawJSON generates a response with a custom system prompt and no response schema.
// Used for tasks like KB generation that need raw JSON output.
func (c *GeminiClient) GenerateRawJSON(ctx context.Context, systemPrompt string, userMessage string, modelID ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("Gemini GenerateRawJSON -> Using selected model: %s", model)
	}

	// Get the API version for the model
	apiVersion := "v1beta"
	if llmModel := constants.GetLLMModel(model); llmModel != nil && llmModel.APIVersion != "" {
		apiVersion = llmModel.APIVersion
	}

	modelName := fmt.Sprintf("models/%s", model)
	log.Printf("Gemini GenerateRawJSON -> Using model: %s with API version: %s", modelName, apiVersion)

	geminiModel := c.client.GenerativeModel(modelName)
	geminiModel.MaxOutputTokens = utils.ToInt32Ptr(int32(c.maxCompletionTokens))
	geminiModel.SetTemperature(float32(c.temperature))
	geminiModel.ResponseMIMEType = "application/json"
	// NO ResponseSchema — we want the LLM to produce raw JSON matching the prompt's format
	geminiModel.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	geminiModel.SafetySettings = []*genai.SafetySetting{
		{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockNone},
	}

	session := geminiModel.StartChat()
	result, err := safeSendMessage(session, ctx, genai.Text(userMessage))
	if err != nil {
		log.Printf("Gemini GenerateRawJSON API error: %v", err)
		return "", fmt.Errorf("gemini API error: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini API")
	}

	responseText := strings.ReplaceAll(fmt.Sprintf("%v", result.Candidates[0].Content.Parts[0]), "```json", "")
	responseText = strings.ReplaceAll(responseText, "```", "")
	responseText = strings.TrimSpace(responseText)

	log.Printf("Gemini GenerateRawJSON -> response length: %d", len(responseText))
	return responseText, nil
}

// GenerateRecommendations generates query recommendations using a prompt and schema
func (c *GeminiClient) GenerateRecommendations(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error) {
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Convert messages into parts for the Gemini API.
	geminiMessages := make([]*genai.Content, 0)

	// Add system prompt for recommendations
	systemPrompt := constants.GetRecommendationsPrompt(constants.Gemini)
	responseSchema := constants.GetRecommendationsSchema(constants.Gemini).(*genai.Schema)

	// Add system message first
	geminiMessages = append(geminiMessages, &genai.Content{
		Role: "user",
		Parts: []genai.Part{
			genai.Text(systemPrompt),
		},
	})

	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Add conversation history
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
			role := "user"
			if msg.Role == "assistant" {
				role = "model"
			}

			geminiMessages = append(geminiMessages, &genai.Content{
				Role: role,
				Parts: []genai.Part{
					genai.Text(content),
				},
			})
		}
	}

	// Build the request with a single content bundle.
	// Call Gemini's content generation API.
	model := c.client.GenerativeModel(c.model)
	model.MaxOutputTokens = utils.ToInt32Ptr(int32(c.maxCompletionTokens))
	model.SetTemperature(float32(c.temperature))
	model.ResponseMIMEType = "application/json"
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	model.ResponseSchema = responseSchema
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
	}

	// Start chat session
	session := model.StartChat()
	session.History = geminiMessages

	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	// Send empty message to get response based on history
	result, err := safeSendMessage(session, ctx, genai.Text("Please provide query recommendations based on our conversation history."))
	if err != nil {
		log.Printf("Gemini API error: %v", err)
		return "", fmt.Errorf("gemini API error: %v", err)
	}

	log.Printf("GEMINI -> GenerateRecommendations -> result: %v", result)
	log.Printf("GEMINI -> GenerateRecommendations -> result.Candidates[0].Content.Parts[0]: %v", result.Candidates[0].Content.Parts[0])
	responseText := strings.ReplaceAll(fmt.Sprintf("%v", result.Candidates[0].Content.Parts[0]), "```json", "")
	responseText = strings.ReplaceAll(responseText, "```", "")

	return responseText, nil
}

// GenerateVisualization generates a visualization configuration for query results
// This method uses a dedicated visualization system prompt and enforces JSON response format
func (c *GeminiClient) GenerateVisualization(ctx context.Context, systemPrompt string, visualizationPrompt string, dataRequest string, modelID ...string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	model := c.model
	if len(modelID) > 0 && modelID[0] != "" {
		model = modelID[0]
		log.Printf("Gemini GenerateVisualization -> Using selected model: %s", model)
	}

	// Create messages for visualization request - simpler than chat flow
	geminiMessages := make([]*genai.Content, 0)

	// Add system message
	geminiMessages = append(geminiMessages, &genai.Content{
		Role: "user",
		Parts: []genai.Part{
			genai.Text(systemPrompt),
		},
	})

	// Add visualization prompt and data request
	geminiMessages = append(geminiMessages, &genai.Content{
		Role: "user",
		Parts: []genai.Part{
			genai.Text(visualizationPrompt),
		},
	})

	geminiMessages = append(geminiMessages, &genai.Content{
		Role: "user",
		Parts: []genai.Part{
			genai.Text(dataRequest),
		},
	})

	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Create model with JSON response enforcement
	geminiModel := c.client.GenerativeModel(fmt.Sprintf("models/%s", model))
	geminiModel.MaxOutputTokens = utils.ToInt32Ptr(int32(c.maxCompletionTokens))
	geminiModel.SetTemperature(float32(c.temperature))
	geminiModel.ResponseMIMEType = "application/json"
	geminiModel.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	geminiModel.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
	}

	// Start chat session for visualization
	session := geminiModel.StartChat()
	session.History = geminiMessages

	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Send request for visualization
	result, err := safeSendMessage(session, ctx, genai.Text("Generate the visualization configuration as JSON."))
	if err != nil {
		log.Printf("Gemini API error in GenerateVisualization: %v", err)
		return "", fmt.Errorf("gemini API error: %v", err)
	}

	log.Printf("GEMINI -> GenerateVisualization -> result: %v", result)
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini API")
	}

	responseText := strings.ReplaceAll(fmt.Sprintf("%v", result.Candidates[0].Content.Parts[0]), "```json", "")
	responseText = strings.ReplaceAll(responseText, "```", "")

	log.Printf("GEMINI -> GenerateVisualization -> responseText: %s", responseText)

	// Validate JSON response
	var visualizationResponse map[string]interface{}
	if err := json.Unmarshal([]byte(responseText), &visualizationResponse); err != nil {
		log.Printf("Error: Gemini visualization response is not valid JSON: %v", err)
		return "", fmt.Errorf("invalid JSON response from Gemini: %v", err)
	}

	return responseText, nil
}

// GetModelInfo returns information about the Gemini model.
func (c *GeminiClient) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:                c.model,
		Provider:            "gemini",
		MaxCompletionTokens: c.maxCompletionTokens,
	}
}

// SetModel updates the model used by the client
func (c *GeminiClient) SetModel(modelID string) error {
	if modelID == "" {
		return fmt.Errorf("model ID cannot be empty")
	}
	c.model = modelID
	log.Printf("Gemini client model updated to: %s", modelID)
	return nil
}

// GenerateWithTools implements iterative tool-calling using Gemini's native function calling.
func (c *GeminiClient) GenerateWithTools(ctx context.Context, messages []*models.LLMMessage, tools []ToolDefinition, executor ToolExecutorFunc, config ToolCallConfig) (*ToolCallResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	model := c.model
	if config.ModelID != "" {
		model = config.ModelID
		log.Printf("Gemini GenerateWithTools -> Using selected model: %s", model)
	}

	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = DefaultMaxIterations
	}

	// Convert tool definitions to Gemini FunctionDeclarations
	geminiFuncDecls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		geminiFuncDecls = append(geminiFuncDecls, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  convertToGenaiSchema(tool.Parameters),
		})
	}

	// Build system prompt: always include DB-specific prompt, then append tool-calling addendum
	systemPrompt := constants.GetSystemPrompt(constants.Gemini, config.DBType, config.NonTechMode)
	if config.SystemPrompt != "" {
		systemPrompt = systemPrompt + "\n\n" + config.SystemPrompt
	}

	// Build conversation history from messages
	geminiMessages := make([]*genai.Content, 0)
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
			role := "user"
			if msg.Role == "assistant" {
				role = "model"
			}
			geminiMessages = append(geminiMessages, &genai.Content{
				Role:  role,
				Parts: []genai.Part{genai.Text(content)},
			})
		}
	}

	// Set up the model with tools
	apiVersion := "v1beta"
	if llmModel := constants.GetLLMModel(model); llmModel != nil && llmModel.APIVersion != "" {
		apiVersion = llmModel.APIVersion
	}
	modelName := fmt.Sprintf("models/%s", model)
	log.Printf("Gemini GenerateWithTools -> Using model: %s with API version: %s", modelName, apiVersion)

	geminiModel := c.client.GenerativeModel(modelName)
	geminiModel.MaxOutputTokens = utils.ToInt32Ptr(int32(c.maxCompletionTokens))
	geminiModel.SetTemperature(float32(c.temperature))
	geminiModel.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	geminiModel.Tools = []*genai.Tool{{FunctionDeclarations: geminiFuncDecls}}
	geminiModel.SafetySettings = []*genai.SafetySetting{
		{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockNone},
	}

	// Start chat session with history
	session := geminiModel.StartChat()
	session.History = geminiMessages

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Initial prompt to kick off tool use
	result, err := safeSendMessage(session, ctx, genai.Text("Please analyze the request and use the available tools to provide an accurate response. Start by examining what you need, then call generate_final_response when ready."))
	if err != nil {
		return nil, fmt.Errorf("gemini tool-calling API error: %v", err)
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

		if result == nil || len(result.Candidates) == 0 || result.Candidates[0].Content == nil {
			emptyRetries++
			if emptyRetries > maxEmptyRetries {
				return nil, fmt.Errorf("empty response from Gemini after %d retries in tool-calling iteration %d", maxEmptyRetries, iteration)
			}
			log.Printf("Gemini GenerateWithTools -> Empty response at iteration %d, retrying (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
			var retryErr error
			result, retryErr = safeSendMessage(session, ctx, genai.Text("Your previous response was empty. Please continue — either call the appropriate tool or call generate_final_response with your answer."))
			if retryErr != nil {
				return nil, fmt.Errorf("gemini retry after empty response failed: %v", retryErr)
			}
			continue
		}

		// Scan response parts for function calls
		var functionCalls []genai.FunctionCall
		var textContent string
		for _, part := range result.Candidates[0].Content.Parts {
			switch p := part.(type) {
			case genai.FunctionCall:
				functionCalls = append(functionCalls, p)
			case genai.Text:
				textContent += string(p)
			}
		}

		// No function calls — LLM returned text directly
		if len(functionCalls) == 0 {
			log.Printf("Gemini GenerateWithTools -> Iteration %d: No function calls, got text response", iteration)
			if textContent != "" {
				// Try to detect and parse text that looks like a tool call attempt
				if parsed, ok := TryParseTextToolCall(textContent); ok {
					log.Printf("Gemini GenerateWithTools -> Extracted structured response from text tool-call")
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
					// Exhausted retries — accept the plain text as-is
					log.Printf("Gemini GenerateWithTools -> Plain text after %d retries, wrapping as assistantMessage", maxEmptyRetries)
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
				log.Printf("Gemini GenerateWithTools -> Plain text at iteration %d, nudging to use generate_final_response (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
				var retryErr error
				result, retryErr = safeSendMessage(session, ctx, genai.Text("You returned a plain text response instead of calling the generate_final_response tool. You MUST call generate_final_response with your complete answer including any SQL queries in the 'queries' array. Do not respond with plain text."))
				if retryErr != nil {
					return nil, fmt.Errorf("gemini retry after plain text failed: %v", retryErr)
				}
				continue
			}
			// Empty text + no function calls — nudge the LLM to retry
			emptyRetries++
			if emptyRetries > maxEmptyRetries {
				return nil, fmt.Errorf("empty text response from Gemini after %d retries in tool-calling iteration %d", maxEmptyRetries, iteration)
			}
			log.Printf("Gemini GenerateWithTools -> Empty text at iteration %d, retrying (%d/%d)", iteration, emptyRetries, maxEmptyRetries)
			var retryErr error
			result, retryErr = safeSendMessage(session, ctx, genai.Text("Your previous response was empty. Please call generate_final_response with your complete answer, or call an appropriate tool if you need more information."))
			if retryErr != nil {
				return nil, fmt.Errorf("gemini retry after empty text failed: %v", retryErr)
			}
			continue
		}

		// Process function calls
		var responseParts []genai.Part
		for _, fc := range functionCalls {
			totalCalls++
			call := ToolCall{
				ID:        fmt.Sprintf("gemini-%d-%d", iteration, totalCalls),
				Name:      fc.Name,
				Arguments: fc.Args,
			}
			toolHistory = append(toolHistory, call)

			if config.OnToolCall != nil {
				config.OnToolCall(call)
			}

			// Check if this is the final response tool
			if fc.Name == FinalResponseToolName {
				log.Printf("Gemini GenerateWithTools -> Final response tool called at iteration %d", iteration)
				responseJSON, err := json.Marshal(fc.Args)
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
				log.Printf("Gemini GenerateWithTools -> Tool %s execution error: %v", fc.Name, err)
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

			// Build FunctionResponse
			responseParts = append(responseParts, genai.FunctionResponse{
				Name:     fc.Name,
				Response: map[string]any{"result": toolResult.Content, "is_error": toolResult.IsError},
			})
		}

		// Reset empty-retry budget after successful tool execution so that
		// each phase (exploration vs. final-response) gets its own retries.
		emptyRetries = 0

		// Send tool results back to the session
		result, err = safeSendMessage(session, ctx, responseParts...)
		if err != nil {
			return nil, fmt.Errorf("gemini tool-calling API error at iteration %d: %v", iteration, err)
		}
	}

	// Max iterations reached — force a response
	log.Printf("Gemini GenerateWithTools -> Max iterations (%d) reached, forcing response", maxIterations)
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

// convertToGenaiSchema converts a generic JSON Schema map to Gemini's *genai.Schema.
func convertToGenaiSchema(schema map[string]interface{}) *genai.Schema {
	if schema == nil {
		return nil
	}

	s := &genai.Schema{}

	if t, ok := schema["type"].(string); ok {
		switch t {
		case "string":
			s.Type = genai.TypeString
		case "number":
			s.Type = genai.TypeNumber
		case "integer":
			s.Type = genai.TypeInteger
		case "boolean":
			s.Type = genai.TypeBoolean
		case "array":
			s.Type = genai.TypeArray
		case "object":
			s.Type = genai.TypeObject
		}
	}

	if desc, ok := schema["description"].(string); ok {
		s.Description = desc
	}

	if enum, ok := schema["enum"].([]interface{}); ok {
		for _, v := range enum {
			if str, ok := v.(string); ok {
				s.Enum = append(s.Enum, str)
			}
		}
	}

	// Handle object properties
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		s.Properties = make(map[string]*genai.Schema)
		for key, val := range props {
			if propMap, ok := val.(map[string]interface{}); ok {
				s.Properties[key] = convertToGenaiSchema(propMap)
			}
		}
	}

	// Handle required fields
	if required, ok := schema["required"].([]interface{}); ok {
		for _, r := range required {
			if str, ok := r.(string); ok {
				s.Required = append(s.Required, str)
			}
		}
	}

	// Handle array items
	if items, ok := schema["items"].(map[string]interface{}); ok {
		s.Items = convertToGenaiSchema(items)
	}

	return s
}
