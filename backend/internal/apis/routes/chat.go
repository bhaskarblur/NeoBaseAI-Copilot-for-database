package routes

import (
	"log"
	"neobase-ai/internal/apis/middlewares"
	"neobase-ai/internal/di"

	"github.com/gin-gonic/gin"
)

func SetupChatRoutes(router *gin.Engine) {
	chatHandler, err := di.GetChatHandler()
	if err != nil {
		log.Fatalf("Failed to get chat handler: %v", err)
	}

	protected := router.Group("/api/chats")
	protected.Use(middlewares.AuthMiddleware())
	{
		// Chat CRUD
		protected.POST("", chatHandler.Create)
		protected.GET("", chatHandler.List)
		protected.GET("/:id", chatHandler.GetByID)
		protected.PATCH("/:id", chatHandler.Update)
		protected.DELETE("/:id", chatHandler.Delete)
		protected.POST("/:id/duplicate", chatHandler.Duplicate) // Has query param "duplicate_messages"

		// Messages within a chat
		protected.GET("/:id/messages", chatHandler.ListMessages)
		protected.POST("/:id/messages", chatHandler.CreateMessage)
		protected.PATCH("/:id/messages/:messageId", chatHandler.UpdateMessage)
		protected.DELETE("/:id/messages", chatHandler.DeleteMessages)
		
		// Message pinning
		protected.POST("/:id/messages/:messageId/pin", chatHandler.PinMessage)
		protected.DELETE("/:id/messages/:messageId/pin", chatHandler.UnpinMessage)
		protected.GET("/:id/messages/pinned", chatHandler.ListPinnedMessages)

		// Database connection routes
		protected.POST("/:id/connect", chatHandler.ConnectDB)
		protected.POST("/:id/disconnect", chatHandler.DisconnectDB)
		protected.GET("/:id/connection-status", chatHandler.GetDBConnectionStatus)
		protected.POST("/:id/refresh-schema", chatHandler.RefreshSchema)
		protected.GET("/:id/tables", chatHandler.GetTables)

		// SSE endpoints for streaming
		protected.GET("/:id/stream", chatHandler.StreamChat)
		protected.POST("/:id/stream/cancel", chatHandler.CancelStream)

		// Query execution routes
		protected.POST("/:id/queries/execute", chatHandler.ExecuteQuery)
		protected.POST("/:id/queries/rollback", chatHandler.RollbackQuery)
		protected.POST("/:id/queries/cancel", chatHandler.CancelQueryExecution)
		protected.POST("/:id/queries/results", chatHandler.GetQueryResults)
		protected.PATCH("/:id/queries/edit", chatHandler.EditQuery)

		// Query recommendations
		protected.GET("/:id/recommendations", chatHandler.GetQueryRecommendations)
	}
}
