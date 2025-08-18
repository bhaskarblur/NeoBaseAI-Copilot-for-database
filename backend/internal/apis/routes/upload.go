package routes

import (
	"log"
	"neobase-ai/internal/apis/handlers"
	"neobase-ai/internal/apis/middlewares"
	"neobase-ai/internal/di"

	"github.com/gin-gonic/gin"
)

func SetupUploadRoutes(router *gin.Engine) {
	// Get chat handler to access chat service
	chatHandler, err := di.GetChatHandler()
	if err != nil {
		log.Fatalf("Failed to get chat handler: %v", err)
	}
	
	// Create upload handler using the chat service
	uploadHandler := handlers.NewUploadHandler(chatHandler.GetChatService())

	protected := router.Group("/api/upload")
	protected.Use(middlewares.AuthMiddleware())
	{
		// File upload for spreadsheet connections
		protected.POST("/:chatID/file", uploadHandler.UploadFile)
		
		// Table data operations
		protected.GET("/:chatID/tables/:tableName", uploadHandler.GetTableData)
		protected.DELETE("/:chatID/tables/:tableName", uploadHandler.DeleteTable)
		protected.DELETE("/:chatID/tables/:tableName/rows/:rowID", uploadHandler.DeleteRow)
		protected.GET("/:chatID/tables/:tableName/download", uploadHandler.DownloadTableData)
	}
}