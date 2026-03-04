package handlers

import (
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DashboardHandler handles dashboard-related HTTP endpoints
type DashboardHandler struct {
	dashboardService services.DashboardService
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(dashboardService services.DashboardService) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
	}
}

// CreateDashboard creates a new dashboard via AI generation
// POST /api/chats/:id/dashboards
func (h *DashboardHandler) CreateDashboard(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	var req dtos.CreateDashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	log.Printf("CreateDashboard -> userID: %s, chatID: %s", userID, chatID)

	resp, statusCode, err := h.dashboardService.CreateDashboard(c, userID, chatID, &req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    resp,
	})
}

// GetDashboard retrieves a dashboard with all its widgets
// GET /api/chats/:id/dashboards/:dashboardId
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	dashboardID := c.Param("dashboardId")

	resp, statusCode, err := h.dashboardService.GetDashboard(c, userID, chatID, dashboardID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    resp,
	})
}

// ListDashboards lists all dashboards for a chat
// GET /api/chats/:id/dashboards
func (h *DashboardHandler) ListDashboards(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	items, statusCode, err := h.dashboardService.ListDashboards(c, userID, chatID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    items,
	})
}

// UpdateDashboard updates dashboard metadata (name, refresh interval, layout, etc.)
// PATCH /api/chats/:id/dashboards/:dashboardId
func (h *DashboardHandler) UpdateDashboard(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	dashboardID := c.Param("dashboardId")

	var req dtos.UpdateDashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	resp, statusCode, err := h.dashboardService.UpdateDashboard(c, userID, chatID, dashboardID, &req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    resp,
	})
}

// DeleteDashboard deletes a dashboard and all its widgets
// DELETE /api/chats/:id/dashboards/:dashboardId
func (h *DashboardHandler) DeleteDashboard(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	dashboardID := c.Param("dashboardId")

	statusCode, err := h.dashboardService.DeleteDashboard(c, userID, chatID, dashboardID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Dashboard deleted successfully",
	})
}

// AddWidget adds a new widget to a dashboard via AI prompt
// POST /api/chats/:id/dashboards/:dashboardId/widgets
func (h *DashboardHandler) AddWidget(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	dashboardID := c.Param("dashboardId")

	var req dtos.AddWidgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	resp, statusCode, err := h.dashboardService.AddWidget(c, userID, chatID, dashboardID, &req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    resp,
	})
}

// EditWidget edits an existing widget via AI prompt
// POST /api/chats/:id/dashboards/:dashboardId/widgets/:widgetId/edit
func (h *DashboardHandler) EditWidget(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	dashboardID := c.Param("dashboardId")
	widgetID := c.Param("widgetId")

	var req dtos.EditWidgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	resp, statusCode, err := h.dashboardService.EditWidget(c, userID, chatID, dashboardID, widgetID, &req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    resp,
	})
}

// DeleteWidget removes a widget from a dashboard
// DELETE /api/chats/:id/dashboards/:dashboardId/widgets/:widgetId
func (h *DashboardHandler) DeleteWidget(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	dashboardID := c.Param("dashboardId")
	widgetID := c.Param("widgetId")

	statusCode, err := h.dashboardService.DeleteWidget(c, userID, chatID, dashboardID, widgetID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Widget deleted successfully",
	})
}

// GenerateBlueprints triggers AI to generate dashboard blueprint suggestions
// POST /api/chats/:id/dashboards/suggest-templates
func (h *DashboardHandler) GenerateBlueprints(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	streamID := c.Query("stream_id")

	if streamID == "" {
		errorMsg := "stream_id query parameter is required"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	// Optional: user-provided prompt for "Create with AI" flow
	var body struct {
		Prompt string `json:"prompt"`
	}
	// Ignore bind errors — prompt is optional, body may be empty
	_ = c.ShouldBindJSON(&body)

	statusCode, err := h.dashboardService.GenerateBlueprints(c, userID, chatID, streamID, body.Prompt)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Blueprint generation started",
	})
}

// CreateFromBlueprints creates dashboards from selected blueprints
// POST /api/chats/:id/dashboards/create-from-blueprints
func (h *DashboardHandler) CreateFromBlueprints(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	streamID := c.Query("stream_id")

	var req dtos.CreateFromBlueprintsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	if streamID == "" {
		errorMsg := "stream_id query parameter is required"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	statusCode, err := h.dashboardService.CreateFromBlueprints(c, userID, chatID, streamID, &req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Dashboard creation from blueprints started",
	})
}

// RegenerateDashboard regenerates an existing dashboard
// POST /api/chats/:id/dashboards/:dashboardId/regenerate
func (h *DashboardHandler) RegenerateDashboard(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	dashboardID := c.Param("dashboardId")
	streamID := c.Query("stream_id")

	var req dtos.RegenerateDashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	if streamID == "" {
		errorMsg := "stream_id query parameter is required"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	statusCode, err := h.dashboardService.RegenerateDashboard(c, userID, chatID, dashboardID, streamID, &req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Dashboard regeneration started",
	})
}

// RefreshDashboard triggers a manual refresh of all dashboard widgets
// POST /api/chats/:id/dashboards/:dashboardId/refresh
func (h *DashboardHandler) RefreshDashboard(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	dashboardID := c.Param("dashboardId")
	streamID := c.Query("stream_id")

	if streamID == "" {
		errorMsg := "stream_id query parameter is required"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	statusCode, err := h.dashboardService.RefreshDashboard(c, userID, chatID, dashboardID, streamID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Dashboard refresh completed",
	})
}

// RefreshWidget triggers a refresh of a single widget
// POST /api/chats/:id/dashboards/:dashboardId/widgets/:widgetId/refresh
func (h *DashboardHandler) RefreshWidget(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	dashboardID := c.Param("dashboardId")
	widgetID := c.Param("widgetId")
	streamID := c.Query("stream_id")

	if streamID == "" {
		errorMsg := "stream_id query parameter is required"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	statusCode, err := h.dashboardService.RefreshWidget(c, userID, chatID, dashboardID, widgetID, streamID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Widget refresh completed",
	})
}
