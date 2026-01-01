package routes

import (
	"log"
	"neobase-ai/internal/apis/handlers"
	"neobase-ai/internal/apis/middlewares"

	"github.com/gin-gonic/gin"
)

// SetupLLMModelsRoutes sets up routes for LLM model management
func SetupLLMModelsRoutes(router *gin.Engine) {
	llmHandler := handlers.NewLLMModelsHandler()

	// Public routes - no auth required to view available models
	public := router.Group("/api/llm-models")
	{
		// Get all enabled models
		public.GET("", llmHandler.GetSupportedModels)

		// Get models by provider
		public.GET("/provider/:provider", llmHandler.GetModelsByProvider)

		// Get specific model details
		public.GET("/:modelId", llmHandler.GetModelDetails)

		// Get default model configuration
		public.GET("/default/config", llmHandler.GetDefaultModel)
	}

	// Protected routes - require authentication
	protected := router.Group("/api/llm-models")
	protected.Use(middlewares.AuthMiddleware())
	{
		// User-specific LLM model selection could go here if needed
		// For now, the main routes are public since all authenticated users
		// should be able to see available models
	}

	log.Println("LLM Models routes set up successfully")
}
