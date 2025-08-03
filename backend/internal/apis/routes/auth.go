package routes

import (
	"log"
	"neobase-ai/internal/apis/middlewares"
	"neobase-ai/internal/di"

	"github.com/gin-gonic/gin"
)

func SetupAuthRoutes(router *gin.Engine) {
	authHandler, err := di.GetAuthHandler()
	if err != nil {
		log.Fatalf("Failed to get auth handler: %v", err)
	}

	// Auth routes
	auth := router.Group("/api/auth")
	{
		auth.POST("/signup", authHandler.Signup)
		auth.POST("/login", authHandler.Login)
		auth.POST("/generate-signup-secret", authHandler.GenerateUserSignupSecret)
		auth.POST("/forgot-password", authHandler.ForgotPassword)
		auth.POST("/reset-password", authHandler.ResetPassword)
	}

	protected := router.Group("/api/auth")
	protected.Use(middlewares.AuthMiddleware())
	{
		protected.GET("/", authHandler.GetUser)
		protected.POST("/logout", authHandler.Logout)
		protected.GET("/refresh-token", authHandler.RefreshToken)
	}
}
