package routes

import (
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/di"
	"neobase-ai/internal/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupDefaultRoutes(router *gin.Engine) {
	// Add recovery middleware
	router.Use(middleware.CustomRecoveryMiddleware())

	// Health check route
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, dtos.Response{
			Success: true,
			Data:    "Server is healthy!",
		})
	})

	githubHandler, err := di.GetGitHubHandler()
	if err != nil {
		log.Fatalf("Failed to get github handler: %v", err)
	}
	// Github repository statistics route
	router.GET("/api/github/stats", githubHandler.GetGitHubStats)

	// Setup all route groups
	SetupAuthRoutes(router)
	SetupChatRoutes(router)
	SetupVisualizationRoutes(router)
	SetupWaitlistRoutes(router)
	SetupUploadRoutes(router)
	SetupGoogleOAuthRoutes(router)
	SetupLLMModelsRoutes(router)
}
