package handlers

import (
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// VisualizationHandler handles visualization-related endpoints
type VisualizationHandler struct {
	chatService services.ChatService
}

// NewVisualizationHandler creates a new visualization handler
func NewVisualizationHandler(chatService services.ChatService) *VisualizationHandler {
	return &VisualizationHandler{
		chatService: chatService,
	}
}

// GenerateVisualization generates a chart visualization for query results
// POST /api/chats/:id/messages/:messageId/visualizations
func (h *VisualizationHandler) GenerateVisualization(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	messageID := c.Param("messageId")

	// Parse request
	var req dtos.GenerateVisualizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	log.Printf("GenerateVisualization -> userID: %s, chatID: %s, messageID: %s, queryID: %s", userID, chatID, messageID, req.QueryID)

	// Get the LLM model to use - fetch from chat's preferred model
	// The service will handle fallback logic if needed
	selectedLLMModel := ""

	visualization, err := h.chatService.GenerateVisualizationForMessage(
		c,
		userID,
		chatID,
		messageID,
		req.QueryID,
		selectedLLMModel, // Service will use chat's preferred model or first available
	)

	if err != nil {
		log.Printf("GenerateVisualization -> Error: %v", err)
		errorMsg := fmt.Sprintf("failed to generate visualization: %v", err)
		c.JSON(http.StatusInternalServerError, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	if visualization == nil {
		visualization = &dtos.VisualizationResponse{
			CanVisualize: false,
			Reason:       "Could not generate visualization for this data",
		}
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    visualization,
	})
}

// ExecuteChartQuery executes a query specifically for chart data fetching
// POST /api/v1/chat/:chatId/execute-chart
func (h *VisualizationHandler) ExecuteChartQuery(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	// Parse request
	var req struct {
		ChartConfiguration *dtos.ChartConfiguration `json:"chart_configuration" binding:"required"`
		Limit              int                      `json:"limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	if req.Limit == 0 {
		req.Limit = 500 // Default limit
	}

	log.Printf("ExecuteChartQuery -> userID: %s, chatID: %s, limit: %d", userID, chatID, req.Limit)

	// Execute the chart query
	results, err := h.chatService.ExecuteChartQuery(c, userID, chatID, req.ChartConfiguration, req.Limit)
	if err != nil {
		log.Printf("ExecuteChartQuery -> Error executing chart query: %v", err)
		errorMsg := fmt.Sprintf("failed to execute chart query: %v", err)
		c.JSON(http.StatusInternalServerError, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data: gin.H{
			"data":        results,
			"total_rows":  len(results),
			"chart_title": req.ChartConfiguration.Title,
		},
	})
}

// GetVisualizationData fetches chart data for a specific query on-demand
// POST /api/chats/:id/visualization-data
func (h *VisualizationHandler) GetVisualizationData(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	// Parse request
	var req struct {
		MessageID string `json:"message_id" binding:"required"`
		QueryID   string `json:"query_id" binding:"required"`
		Limit     int    `json:"limit"`
		Offset    int    `json:"offset"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 500
	}
	if req.Offset == 0 {
		req.Offset = 0
	}

	log.Printf("GetVisualizationData -> userID: %s, chatID: %s, messageID: %s, queryID: %s, limit: %d, offset: %d",
		userID, chatID, req.MessageID, req.QueryID, req.Limit, req.Offset)

	// Fetch visualization data
	data, err := h.chatService.GetVisualizationData(c, userID, chatID, req.MessageID, req.QueryID, req.Limit, req.Offset)
	if err != nil {
		log.Printf("GetVisualizationData -> Error: %v", err)
		errorMsg := fmt.Sprintf("failed to fetch visualization data: %v", err)
		c.JSON(http.StatusInternalServerError, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    data,
	})
}
