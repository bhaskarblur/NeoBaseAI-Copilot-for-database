package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"
	"neobase-ai/internal/utils"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService services.ChatService
	streamMutex sync.RWMutex
	streams     map[string]chan dtos.StreamResponse // key: userID:chatID:streamID
}

func NewChatHandler(chatService services.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		streamMutex: sync.RWMutex{},
		streams:     make(map[string]chan dtos.StreamResponse),
	}
}

// @Summary Create a new chat
// @Description Create a new chat
// @Accept json
// @Produce json
// @Param createChatRequest body dtos.CreateChatRequest true "Create chat request"
// @Success 200 {object} dtos.Response

func (h *ChatHandler) Create(c *gin.Context) {
	var req dtos.CreateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	// Custom validation based on connection type
	if req.Connection.Type != "spreadsheet" && req.Connection.Type != "google_sheets" {
		// For database connections, validate required fields
		if req.Connection.Host == "" {
			errorMsg := "Host is required for database connections"
			c.JSON(http.StatusBadRequest, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			return
		}
		if req.Connection.Username == "" {
			errorMsg := "Username is required for database connections"
			c.JSON(http.StatusBadRequest, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			return
		}
		if req.Connection.Database == "" {
			errorMsg := "Database is required for database connections"
			c.JSON(http.StatusBadRequest, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			return
		}
	} else if req.Connection.Type == "google_sheets" {
		// For Google Sheets, validate specific fields
		if req.Connection.GoogleSheetID == nil || *req.Connection.GoogleSheetID == "" {
			errorMsg := "Google Sheet ID is required"
			c.JSON(http.StatusBadRequest, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			return
		}
		if req.Connection.GoogleAuthToken == nil || *req.Connection.GoogleAuthToken == "" {
			errorMsg := "Google authentication token is required"
			c.JSON(http.StatusBadRequest, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			return
		}
		if req.Connection.GoogleRefreshToken == nil || *req.Connection.GoogleRefreshToken == "" {
			errorMsg := "Google refresh token is required"
			c.JSON(http.StatusBadRequest, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			return
		}
	}

	userID := c.GetString("userID")
	response, statusCode, err := h.chatService.Create(userID, &req)
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
		Data:    response,
	})
}

// @Summary List chats
// @Description List all chats
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)

func (h *ChatHandler) List(c *gin.Context) {
	userID := c.GetString("userID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	response, statusCode, err := h.chatService.List(userID, page, pageSize)
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
		Data:    response,
	})
}

// @Summary Get chat by ID
// @Description Get a chat by its ID
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) GetByID(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	response, statusCode, err := h.chatService.GetByID(userID, chatID)
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
		Data:    response,
	})
}

// @Summary Update a chat
// @Description Update a chat
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) Update(c *gin.Context) {
	var req dtos.UpdateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	userID := c.GetString("userID")
	chatID := c.Param("id")

	response, statusCode, err := h.chatService.Update(userID, chatID, &req)
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
		Data:    response,
	})
}

// @Summary Delete a chat
// @Description Delete a chat
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) Delete(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	statusCode, err := h.chatService.Delete(userID, chatID)
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
		Data:    "Chat deleted successfully",
	})
}

// @Summary Duplicate a chat
// @Description Duplicate a chat
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"
// @Param duplicate_messages query bool false "Duplicate messages" default(false)

func (h *ChatHandler) Duplicate(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	duplicateMessages := c.Query("duplicate_messages") == "true"

	response, statusCode, err := h.chatService.Duplicate(userID, chatID, duplicateMessages)
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
		Data:    response,
	})
}

// @Summary List messages
// @Description List all messages for a chat
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) ListMessages(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	response, statusCode, err := h.chatService.ListMessages(userID, chatID, page, pageSize)
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
		Data:    response,
	})
}

// @Summary Create a new message
// @Description Create a new message
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) CreateMessage(c *gin.Context) {
	var req dtos.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	userID := c.GetString("userID")
	chatID := c.Param("id")

	response, statusCode, err := h.chatService.CreateMessage(c.Request.Context(), userID, chatID, req.StreamID, req.Content)
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
		Data:    response,
	})
}

// @Summary Update a message
// @Description Update a message
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) UpdateMessage(c *gin.Context) {
	var req dtos.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	userID := c.GetString("userID")
	chatID := c.Param("id")
	messageID := c.Param("messageId")

	response, statusCode, err := h.chatService.UpdateMessage(c.Request.Context(), userID, chatID, messageID, req.StreamID, &req)
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
		Data:    response,
	})
}

// @Summary Delete messages
// @Description Delete messages
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) DeleteMessages(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	statusCode, err := h.chatService.DeleteMessages(userID, chatID)
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
		Data:    "Messages deleted successfully",
	})
}

// @Summary Pin a message
// @Description Pin a message and its related message (user-AI cluster)
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"
// @Param messageId path string true "Message ID"

func (h *ChatHandler) PinMessage(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	messageID := c.Param("messageId")

	response, statusCode, err := h.chatService.PinMessage(userID, chatID, messageID)
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
		Data:    response,
	})
}

// @Summary Unpin a message
// @Description Unpin a message and its related message (user-AI cluster)
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"
// @Param messageId path string true "Message ID"

func (h *ChatHandler) UnpinMessage(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	messageID := c.Param("messageId")

	response, statusCode, err := h.chatService.UnpinMessage(userID, chatID, messageID)
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
		Data:    response,
	})
}

// @Summary List pinned messages
// @Description List all pinned messages for a chat
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) ListPinnedMessages(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	response, statusCode, err := h.chatService.ListPinnedMessages(userID, chatID)
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
		Data:    response,
	})
}

// @Summary Handle stream event
// @Description Handle stream event
// @Accept json
// @Produce json
// @Param userID path string true "User ID"
// @Param chatID path string true "Chat ID"

// HandleStreamEvent implements the StreamHandler interface
func (h *ChatHandler) HandleStreamEvent(userID, chatID, streamID string, response dtos.StreamResponse) {
	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)

	h.streamMutex.RLock()
	streamChan, exists := h.streams[streamKey]
	h.streamMutex.RUnlock()

	if !exists {
		log.Printf("No stream found for key: %s", streamKey)
		return
	}

	// Try to send with timeout
	select {
	case streamChan <- response:
		log.Printf("Successfully sent event to stream: %s, event: %s", streamKey, response.Event)
	case <-time.After(100 * time.Millisecond):
		log.Printf("Timeout sending event to stream: %s", streamKey)
	}
}

// @Summary Stream chat
// @Description Stream chat
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

// StreamChat handles SSE endpoint
func (h *ChatHandler) StreamChat(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	streamID := c.Query("stream_id")

	if streamID == "" {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr("stream_id is required"),
		})
		return
	}

	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)
	log.Printf("Starting stream for key: %s", streamKey)

	// Create buffered channel
	h.streamMutex.Lock()
	streamChan := make(chan dtos.StreamResponse, 100)
	h.streams[streamKey] = streamChan
	h.streamMutex.Unlock()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Send connection event
	ctx := c.Request.Context()
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Cleanup on exit
	defer func() {
		h.streamMutex.Lock()
		if ch, exists := h.streams[streamKey]; exists {
			close(ch)
			delete(h.streams, streamKey)
			log.Printf("Cleaned up stream for key: %s", streamKey)
		}
		h.streamMutex.Unlock()
	}()

	log.Printf("Sending initial connection event for stream key: %s", streamKey)
	// Send initial connection event
	data, _ := json.Marshal(dtos.StreamResponse{
		Event: "connected",
		Data:  "Stream established",
	})
	c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
	c.Writer.Flush()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Client disconnected for stream key: %s", streamKey)
			return

		case <-heartbeatTicker.C:
			data, _ := json.Marshal(dtos.StreamResponse{
				Event: "heartbeat",
				Data:  "ping",
			})
			c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
			c.Writer.Flush()

		case msg, ok := <-streamChan:
			if !ok {
				log.Printf("Stream channel closed for key: %s", streamKey)
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Error marshaling message: %v", err)
				continue
			}
			log.Printf("Sending stream event -> key: %s, event: %s", streamKey, msg.Event)
			c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
			c.Writer.Flush()
		}
	}
}

// @Summary Cancel stream
// @Description Cancel currently streaming response
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

// CancelStream cancels currently streaming response
func (h *ChatHandler) CancelStream(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	streamID := c.Query("stream_id")

	if streamID == "" {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr("stream_id is required"),
		})
		return
	}

	// Create stream key
	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)

	// First cancel the processing
	h.chatService.CancelProcessing(userID, chatID, streamID)

	// Then cleanup the stream
	h.streamMutex.Lock()
	if streamChan, ok := h.streams[streamKey]; ok {
		close(streamChan)
		delete(h.streams, streamKey)
	}
	h.streamMutex.Unlock()

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    "Operation cancelled successfully",
	})
}

// @Summary Connect DB
// @Description Connect to a database
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

// ConnectDB establishes a database connection
func (h *ChatHandler) ConnectDB(c *gin.Context) {

	var req dtos.ConnectDBRequest
	userID := c.GetString("userID")
	chatID := c.Param("id")

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(fmt.Sprintf("Invalid request: %v", err)),
		})
		return
	}

	statusCode, err := h.chatService.ConnectDB(c.Request.Context(), userID, chatID, req.StreamID)
	if err != nil {
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    "Database connected successfully",
	})
}

// @Summary Disconnect DB
// @Description Disconnect from a database
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

// DisconnectDB closes a database connection
func (h *ChatHandler) DisconnectDB(c *gin.Context) {
	var req dtos.DisconnectDBRequest
	userID := c.GetString("userID")
	chatID := c.Param("id")
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(fmt.Sprintf("Invalid request: %v", err)),
		})
		return
	}

	statusCode, err := h.chatService.DisconnectDB(c.Request.Context(), userID, chatID, req.StreamID)
	if err != nil {
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    "Database disconnected successfully",
	})
}

// @Summary Get DB Connection Status
// @Description Get the current connection status of a database
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

// GetDBConnectionStatus checks the current connection status
func (h *ChatHandler) GetDBConnectionStatus(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	status, statusCode, err := h.chatService.GetDBConnectionStatus(c.Request.Context(), userID, chatID)
	if err != nil {
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    status,
	})
}

// @Summary Refresh Schema
// @Description Refresh the schema of a database
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) RefreshSchema(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	statusCode, err := h.chatService.RefreshSchema(c.Request.Context(), userID, chatID, true)
	if err != nil {
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    "Schema refreshed successfully",
	})
}

// @Summary Execute query
// @Description Execute a query
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

// Add query execution methods
func (h *ChatHandler) ExecuteQuery(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	var req dtos.ExecuteQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	// Execute query
	response, status, err := h.chatService.ExecuteQuery(c.Request.Context(), userID, chatID, &req)
	if err != nil {
		c.JSON(int(status), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(int(status), dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Rollback query
// @Description Rollback a query
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) RollbackQuery(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	var req dtos.RollbackQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	// Execute rollback
	response, status, err := h.chatService.RollbackQuery(c.Request.Context(), userID, chatID, &req)
	if err != nil {
		c.JSON(int(status), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(int(status), dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Cancel query execution
// @Description Cancel a query execution
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) CancelQueryExecution(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	var req dtos.CancelQueryExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Cancel execution
	h.chatService.CancelQueryExecution(userID, chatID, req.MessageID, req.QueryID, req.StreamID)
	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    "Query execution cancelled successfully",
	})
}

// Update the stream handling
func (h *ChatHandler) HandleStream(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	streamID := c.Query("stream_id")

	if streamID == "" {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr("stream_id is required"),
		})
		return
	}

	// Create stream key
	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)

	// Check if stream already exists
	h.streamMutex.Lock()
	if existingChan, exists := h.streams[streamKey]; exists {
		log.Printf("Stream already exists: %s, closing old stream", streamKey)
		close(existingChan)
		delete(h.streams, streamKey)
	}

	// Create new stream channel
	streamChan := make(chan dtos.StreamResponse, 100)
	h.streams[streamKey] = streamChan
	h.streamMutex.Unlock()

	log.Printf("Created new stream: %s", streamKey)

	// Set headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")

	// Send initial connection event
	c.SSEvent("message", dtos.StreamResponse{
		Event: "connected",
		Data:  "Stream established",
	})
	c.Writer.Flush()

	// Setup context and ticker
	ctx := c.Request.Context()
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Cleanup on exit
	defer func() {
		h.streamMutex.Lock()
		if ch, exists := h.streams[streamKey]; exists {
			log.Printf("Closing stream: %s", streamKey)
			close(ch)
			delete(h.streams, streamKey)
		}
		h.streamMutex.Unlock()
	}()

	// Stream handling loop
	for {
		select {
		case <-ctx.Done():
			log.Printf("Context done for stream: %s", streamKey)
			return

		case <-heartbeatTicker.C:
			if f, ok := c.Writer.(http.Flusher); ok {
				c.SSEvent("message", dtos.StreamResponse{
					Event: "heartbeat",
					Data:  "ping",
				})
				f.Flush()
			}

		case msg, ok := <-streamChan:
			if !ok {
				log.Printf("Stream channel closed: %s", streamKey)
				return
			}
			if f, ok := c.Writer.(http.Flusher); ok {
				c.SSEvent("message", msg)
				f.Flush()
			}
		}
	}
}

// @Summary Get query results
// @Description Get the results of a query
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) GetQueryResults(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	var req dtos.QueryResultsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	response, status, err := h.chatService.GetQueryResults(c.Request.Context(), userID, chatID, req.MessageID, req.QueryID, req.StreamID, req.Offset)
	if err != nil {
		c.JSON(int(status), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(int(status), dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Edit query
// @Description Edit a query
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) EditQuery(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	var req dtos.EditQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	response, status, err := h.chatService.EditQuery(c.Request.Context(), userID, chatID, req.MessageID, req.QueryID, req.Query)
	if err != nil {
		c.JSON(int(status), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(int(status), dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Get tables
// @Description Get all tables with their columns for a specific chat, marking which ones are selected
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"

func (h *ChatHandler) GetTables(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	response, statusCode, err := h.chatService.GetAllTables(c.Request.Context(), userID, chatID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    response,
	})
}

// @Summary Get query recommendations
// @Description Get 3 AI-generated query recommendations based on database schema and context
// @Produce json
// @Param id path string true "Chat ID"
// @Success 200 {object} dtos.Response{data=dtos.QueryRecommendationsResponse}
// @Router /api/chats/{id}/recommendations [get]
func (h *ChatHandler) GetQueryRecommendations(c *gin.Context) {
	chatID := c.Param("id")
	userID := c.GetString("userID")

	recommendations, status, err := h.chatService.GetQueryRecommendations(c.Request.Context(), userID, chatID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(status), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    recommendations,
	})
}

// GetChatService returns the chat service instance
func (h *ChatHandler) GetChatService() services.ChatService {
	return h.chatService
}
