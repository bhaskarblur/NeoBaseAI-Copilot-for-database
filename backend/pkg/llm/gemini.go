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

func (c *GeminiClient) GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string, nonTechMode bool) (string, error) {
	// Check if the context is cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
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
			if assistantMsg, ok := msg.Content["assistant_response"].(map[string]interface{}); ok {
				content = formatAssistantResponse(assistantMsg)
				// Add non-tech mode context if the mode differs from current request
				if msg.NonTechMode != nonTechMode {
					if msg.NonTechMode {
						content = "[This response was generated in NON-TECHNICAL MODE]\n" + content
					} else {
						content = "[This response was generated in TECHNICAL MODE]\n" + content
					}
				}
			}
		case "system":
			if schemaUpdate, ok := msg.Content["schema_update"].(string); ok {
				content = fmt.Sprintf("Database schema update:\n%s", schemaUpdate)
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
	result, err := session.SendMessage(ctx, genai.Text("Please provide a response based on our conversation history."))
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
			if assistantMsg, ok := msg.Content["assistant_response"].(map[string]interface{}); ok {
				content = formatAssistantResponse(assistantMsg)
			}
		case "system":
			if schemaUpdate, ok := msg.Content["schema_update"].(string); ok {
				content = fmt.Sprintf("Database schema update:\n%s", schemaUpdate)
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
	result, err := session.SendMessage(ctx, genai.Text("Please provide query recommendations based on our conversation history."))
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

// GetModelInfo returns information about the Gemini model.
func (c *GeminiClient) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:                c.model,
		Provider:            "gemini",
		MaxCompletionTokens: c.maxCompletionTokens,
	}
}
