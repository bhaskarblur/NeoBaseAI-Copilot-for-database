package routes

import (
	"log"
	"neobase-ai/internal/apis/middlewares"
	"neobase-ai/internal/di"

	"github.com/gin-gonic/gin"
)

func SetupDashboardRoutes(router *gin.Engine) {
	dashHandler, err := di.GetDashboardHandler()
	if err != nil {
		log.Fatalf("Failed to get dashboard handler: %v", err)
	}

	protected := router.Group("/api/chats")
	protected.Use(middlewares.AuthMiddleware())
	{
		// Dashboard CRUD
		protected.POST("/:id/dashboards", dashHandler.CreateDashboard)
		protected.GET("/:id/dashboards", dashHandler.ListDashboards)
		protected.GET("/:id/dashboards/:dashboardId", dashHandler.GetDashboard)
		protected.PATCH("/:id/dashboards/:dashboardId", dashHandler.UpdateDashboard)
		protected.DELETE("/:id/dashboards/:dashboardId", dashHandler.DeleteDashboard)

		// Widget CRUD
		protected.POST("/:id/dashboards/:dashboardId/widgets", dashHandler.AddWidget)
		protected.POST("/:id/dashboards/:dashboardId/widgets/:widgetId/edit", dashHandler.EditWidget)
		protected.DELETE("/:id/dashboards/:dashboardId/widgets/:widgetId", dashHandler.DeleteWidget)

		// AI Operations
		protected.POST("/:id/dashboards/suggest-templates", dashHandler.GenerateBlueprints)
		protected.POST("/:id/dashboards/create-from-blueprints", dashHandler.CreateFromBlueprints)
		protected.POST("/:id/dashboards/:dashboardId/regenerate", dashHandler.RegenerateDashboard)

		// Data Refresh
		protected.POST("/:id/dashboards/:dashboardId/refresh", dashHandler.RefreshDashboard)
		protected.POST("/:id/dashboards/:dashboardId/widgets/:widgetId/refresh", dashHandler.RefreshWidget)

		// Import/Export
		protected.GET("/:id/dashboards/:dashboardId/export", dashHandler.ExportDashboard)
		protected.POST("/:id/dashboards/import/validate", dashHandler.ValidateImport)
		protected.POST("/:id/dashboards/import", dashHandler.ImportDashboard)
	}
}
