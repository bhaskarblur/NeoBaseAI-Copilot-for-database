package handlers

import (
	"neobase-ai/config"
	"neobase-ai/internal/constants"
	"net/http"

	"github.com/gin-gonic/gin"
)

// LLMModelsHandler handles LLM model-related requests
type LLMModelsHandler struct{}

// NewLLMModelsHandler creates a new LLM models handler
func NewLLMModelsHandler() *LLMModelsHandler {
	return &LLMModelsHandler{}
}

// GetSupportedModels returns all enabled LLM models available for use
// Filters models based on which API keys are configured
func (h *LLMModelsHandler) GetSupportedModels(c *gin.Context) {
	// Get available models based on configured API keys
	models := constants.GetAvailableModelsByAPIKeys(config.Env.OpenAIAPIKey, config.Env.GeminiAPIKey, config.Env.ClaudeAPIKey, config.Env.OllamaBaseURL)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"models": models,
			"count":  len(models),
		},
	})
}

// GetModelsByProvider returns all enabled LLM models for a specific provider
func (h *LLMModelsHandler) GetModelsByProvider(c *gin.Context) {
	provider := c.Param("provider")

	// Validate provider
	if provider != constants.OpenAI && provider != constants.Gemini {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid provider. Supported: openai, gemini",
		})
		return
	}

	// Check if API key is configured for this provider
	var apiKey string
	switch provider {
	case constants.OpenAI:
		apiKey = config.Env.OpenAIAPIKey
	case constants.Gemini:
		apiKey = config.Env.GeminiAPIKey
	}

	if apiKey == "" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Provider not configured. No API key found for " + provider,
			"message": "Please configure the API key for this provider to use its models",
		})
		return
	}

	models := constants.GetLLMModelsByProvider(provider)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"provider": provider,
			"models":   models,
			"count":    len(models),
		},
	})
}

// GetModelDetails returns details for a specific LLM model
func (h *LLMModelsHandler) GetModelDetails(c *gin.Context) {
	modelID := c.Param("modelId")

	model := constants.GetLLMModel(modelID)
	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Model not found",
		})
		return
	}

	if !model.IsEnabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Model is disabled",
		})
		return
	}

	// Check if API key is configured for this model's provider
	var apiKey string
	switch model.Provider {
	case constants.OpenAI:
		apiKey = config.Env.OpenAIAPIKey
	case constants.Gemini:
		apiKey = config.Env.GeminiAPIKey
	}

	if apiKey == "" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Provider not configured",
			"message": "API key not configured for " + model.Provider,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    model,
	})
}

// GetDefaultModel returns the default LLM model configured for the instance
func (h *LLMModelsHandler) GetDefaultModel(c *gin.Context) {
	// Get the default model from config
	defaultModelID := config.Env.DefaultLLMModel

	model := constants.GetLLMModel(defaultModelID)
	if model == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Default model not found in configuration",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"default_model": defaultModelID,
			"model_details": model,
		},
	})
}
