package services

import (
	"context"
	"fmt"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/internal/repositories"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/dbmanager"
	"neobase-ai/pkg/llm"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Used by Handler
type StreamHandler interface {
	HandleStreamEvent(userID, chatID, streamID string, response dtos.StreamResponse)
}

type ChatService interface {
	SetStreamHandler(handler StreamHandler)

	// CRUD operations
	Create(userID string, req *dtos.CreateChatRequest) (*dtos.ChatResponse, uint32, error)
	CreateWithoutConnectionPing(userID string, req *dtos.CreateChatRequest) (*dtos.ChatResponse, uint32, error)
	Update(userID, chatID string, req *dtos.UpdateChatRequest) (*dtos.ChatResponse, uint32, error)
	Delete(userID, chatID string) (uint32, error)
	GetByID(userID, chatID string) (*dtos.ChatResponse, uint32, error)
	List(userID string, page, pageSize int) (*dtos.ChatListResponse, uint32, error)
	CreateMessage(ctx context.Context, userID, chatID string, streamID string, content string) (*dtos.MessageResponse, uint16, error)
	UpdateMessage(ctx context.Context, userID, chatID, messageID string, streamID string, req *dtos.CreateMessageRequest) (*dtos.MessageResponse, uint32, error)
	DeleteMessages(userID, chatID string) (uint32, error)
	Duplicate(userID, chatID string, duplicateMessages bool) (*dtos.ChatResponse, uint32, error)
	ListMessages(userID, chatID string, page, pageSize int) (*dtos.MessageListResponse, uint32, error)
	EditQuery(ctx context.Context, userID, chatID, messageID, queryID string, query string) (*dtos.EditQueryResponse, uint32, error)
	GetDBConnectionStatus(ctx context.Context, userID, chatID string) (*dtos.ConnectionStatusResponse, uint32, error)
	HandleSchemaChange(userID, chatID, streamID string, diff *dbmanager.SchemaDiff)
	HandleDBEvent(userID, chatID, streamID string, response dtos.StreamResponse)
	GetAllTables(ctx context.Context, userID, chatID string) (*dtos.TablesResponse, uint32, error)
	GetSelectedCollections(chatID string) (string, error)

	// Execution operations
	CancelProcessing(userID, chatID, streamID string)
	ConnectDB(ctx context.Context, userID, chatID string, streamID string) (uint32, error)
	DisconnectDB(ctx context.Context, userID, chatID string, streamID string) (uint32, error)
	ExecuteQuery(ctx context.Context, userID, chatID string, req *dtos.ExecuteQueryRequest) (*dtos.QueryExecutionResponse, uint32, error)
	RollbackQuery(ctx context.Context, userID, chatID string, req *dtos.RollbackQueryRequest) (*dtos.QueryExecutionResponse, uint32, error)
	CancelQueryExecution(userID, chatID, messageID, queryID, streamID string)
	processMessage(ctx context.Context, userID, chatID string, messageID, streamID string) error
	processLLMResponseAndRunQuery(ctx context.Context, userID, chatID string, messageID, streamID string) error
	RefreshSchema(ctx context.Context, userID, chatID string, sync bool) (uint32, error)
	GetQueryResults(ctx context.Context, userID, chatID, messageID, queryID, streamID string, offset int) (*dtos.QueryResultsResponse, uint32, error)
}

type chatService struct {
	chatRepo        repositories.ChatRepository
	llmRepo         repositories.LLMMessageRepository
	dbManager       *dbmanager.Manager
	llmClient       llm.Client
	streamChans     map[string]chan dtos.StreamResponse
	streamHandler   StreamHandler
	activeProcesses map[string]context.CancelFunc // key: streamID
	processesMu     sync.RWMutex
}

func isValidDBType(dbType string) bool {
	validTypes := []string{
		constants.DatabaseTypePostgreSQL,
		constants.DatabaseTypeYugabyteDB,
		constants.DatabaseTypeMySQL,
		constants.DatabaseTypeClickhouse,
		constants.DatabaseTypeMongoDB,
		constants.DatabaseTypeRedis,
		constants.DatabaseTypeNeo4j,
	}

	for _, validType := range validTypes {
		if dbType == validType {
			return true
		}
	}

	return false
}

func (s *chatService) SetStreamHandler(handler StreamHandler) {
	s.streamHandler = handler
}

// Helper method to send stream events
func (s *chatService) sendStreamEvent(userID, chatID, streamID string, response dtos.StreamResponse) {
	log.Printf("sendStreamEvent -> userID: %s, chatID: %s, streamID: %s, response: %+v", userID, chatID, streamID, response)
	if s.streamHandler != nil {
		s.streamHandler.HandleStreamEvent(userID, chatID, streamID, response)
	} else {
		log.Printf("sendStreamEvent -> no stream handler set")
	}
}

// Add method to handle DB status events
func (s *chatService) HandleDBEvent(userID, chatID, streamID string, response dtos.StreamResponse) {
	// Send to stream handler
	log.Printf("ChatService -> HandleDBEvent -> response: %+v", response)
	if s.streamHandler != nil {
		s.streamHandler.HandleStreamEvent(userID, chatID, streamID, response)
	}
}

func NewChatService(
	chatRepo repositories.ChatRepository,
	llmRepo repositories.LLMMessageRepository,
	dbManager *dbmanager.Manager,
	llmClient llm.Client,
) ChatService {
	return &chatService{
		chatRepo:        chatRepo,
		llmRepo:         llmRepo,
		dbManager:       dbManager,
		llmClient:       llmClient,
		streamChans:     make(map[string]chan dtos.StreamResponse),
		activeProcesses: make(map[string]context.CancelFunc),
	}
}

// Create a new chat
func (s *chatService) Create(userID string, req *dtos.CreateChatRequest) (*dtos.ChatResponse, uint32, error) {
	log.Printf("Creating chat for user %s", userID)

	// If 0, means trial mode, so user cannot create more than 1 chat
	if config.Env.MaxChatsPerUser == 0 {
		// Apply check that single user cannot have more than 1 chat
		userObjID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
		}
		chats, _, err := s.chatRepo.FindByUserID(userObjID, 1, 2)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
		}
		if len(chats) > 1 {
			return nil, http.StatusBadRequest, fmt.Errorf("user cannot have more than 2 chats")
		}
	}

	// Validate database type
	if !isValidDBType(req.Connection.Type) {
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported database type: %s", req.Connection.Type)
	}

	// Test connection without creating a persistent connection
	err := s.dbManager.TestConnection(&dbmanager.ConnectionConfig{
		Type:           req.Connection.Type,
		Host:           req.Connection.Host,
		Port:           req.Connection.Port,
		Username:       &req.Connection.Username,
		Password:       req.Connection.Password,
		Database:       req.Connection.Database,
		AuthDatabase:   req.Connection.AuthDatabase,
		SSLMode:        req.Connection.SSLMode,
		UseSSL:         req.Connection.UseSSL,
		SSLCertURL:     req.Connection.SSLCertURL,
		SSLKeyURL:      req.Connection.SSLKeyURL,
		SSLRootCertURL: req.Connection.SSLRootCertURL,
	})
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("%v", err)
	}

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	// Create connection object with SSL configuration
	connection := models.Connection{
		Type:           req.Connection.Type,
		Host:           req.Connection.Host,
		Port:           req.Connection.Port,
		Username:       &req.Connection.Username,
		Password:       req.Connection.Password,
		Database:       req.Connection.Database,
		AuthDatabase:   req.Connection.AuthDatabase,
		SSLMode:        req.Connection.SSLMode,
		UseSSL:         req.Connection.UseSSL,
		SSLCertURL:     req.Connection.SSLCertURL,
		SSLKeyURL:      req.Connection.SSLKeyURL,
		SSLRootCertURL: req.Connection.SSLRootCertURL,
		Base:           models.NewBase(),
	}

	// Encrypt connection details
	if err := utils.EncryptConnection(&connection); err != nil {
		log.Printf("Warning: Failed to encrypt connection details: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to secure connection details: %v", err)
	}

	settings := models.DefaultChatSettings()
	if req.Settings.AutoExecuteQuery != nil {
		settings.AutoExecuteQuery = *req.Settings.AutoExecuteQuery
	}
	if req.Settings.ShareDataWithAI != nil {
		settings.ShareDataWithAI = *req.Settings.ShareDataWithAI
	}
	// Create chat with connection
	chat := models.NewChat(userObjID, connection, settings)
	if err := s.chatRepo.Create(chat); err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return s.buildChatResponse(chat), http.StatusCreated, nil
}

// Create a new chat without connection ping
func (s *chatService) CreateWithoutConnectionPing(userID string, req *dtos.CreateChatRequest) (*dtos.ChatResponse, uint32, error) {
	log.Printf("Creating chat for user %s", userID)

	// If 0, means trial mode, so user cannot create more than 1 chat
	if config.Env.MaxChatsPerUser == 0 {
		// Apply check that single user cannot have more than 1 chat
		userObjID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
		}
		chats, _, err := s.chatRepo.FindByUserID(userObjID, 1, 2)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
		}
		if len(chats) > 1 {
			return nil, http.StatusBadRequest, fmt.Errorf("user cannot have more than 2 chats")
		}
	}

	// Validate database type
	if !isValidDBType(req.Connection.Type) {
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported database type: %s", req.Connection.Type)
	}

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	// Create connection object with SSL configuration
	connection := models.Connection{
		Type:           req.Connection.Type,
		Host:           req.Connection.Host,
		Port:           req.Connection.Port,
		Username:       &req.Connection.Username,
		Password:       req.Connection.Password,
		Database:       req.Connection.Database,
		AuthDatabase:   req.Connection.AuthDatabase,
		IsExampleDB:    true, // default is true, if false, then the database is a user's own database
		UseSSL:         req.Connection.UseSSL,
		SSLMode:        req.Connection.SSLMode,
		SSLCertURL:     req.Connection.SSLCertURL,
		SSLKeyURL:      req.Connection.SSLKeyURL,
		SSLRootCertURL: req.Connection.SSLRootCertURL,
		Base:           models.NewBase(),
	}

	// Encrypt connection details
	if err := utils.EncryptConnection(&connection); err != nil {
		log.Printf("Warning: Failed to encrypt connection details: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to secure connection details: %v", err)
	}

	settings := models.DefaultChatSettings()

	if req.Settings.AutoExecuteQuery != nil {
		settings.AutoExecuteQuery = *req.Settings.AutoExecuteQuery
	}
	if req.Settings.ShareDataWithAI != nil {
		settings.ShareDataWithAI = *req.Settings.ShareDataWithAI
	}
	// Create chat with connection
	chat := models.NewChat(userObjID, connection, settings)
	if err := s.chatRepo.Create(chat); err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return s.buildChatResponse(chat), http.StatusCreated, nil
}

// Update a chat details such as connection, selected collections, auto execute query flag
func (s *chatService) Update(userID, chatID string, req *dtos.UpdateChatRequest) (*dtos.ChatResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	// Get the chat
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, http.StatusNotFound, fmt.Errorf("chat not found")
		}
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}

	// Check if the chat belongs to the user
	if chat.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("chat does not belong to user")
	}

	// Check for connection changes
	var credentialsChanged bool
	if req.Connection != nil {
		// Validate database type
		if !isValidDBType(req.Connection.Type) {
			return nil, http.StatusBadRequest, fmt.Errorf("unsupported database type: %s", req.Connection.Type)
		}

		// Create a copy of the existing connection and decrypt it for comparison
		existingConn := chat.Connection
		utils.DecryptConnection(&existingConn)

		// Check if critical connection details have changed
		credentialsChanged = existingConn.Database != req.Connection.Database ||
			existingConn.Host != req.Connection.Host ||
			existingConn.Port != req.Connection.Port ||
			*existingConn.Username != req.Connection.Username ||
			(req.Connection.Password != nil && existingConn.Password != nil && *existingConn.Password != *req.Connection.Password)

		// Test connection without creating a persistent connection
		err = s.dbManager.TestConnection(&dbmanager.ConnectionConfig{
			Type:           req.Connection.Type,
			Host:           req.Connection.Host,
			Port:           req.Connection.Port,
			Username:       &req.Connection.Username,
			Password:       req.Connection.Password,
			Database:       req.Connection.Database,
			AuthDatabase:   req.Connection.AuthDatabase,
			UseSSL:         req.Connection.UseSSL,
			SSLMode:        req.Connection.SSLMode,
			SSLCertURL:     req.Connection.SSLCertURL,
			SSLKeyURL:      req.Connection.SSLKeyURL,
			SSLRootCertURL: req.Connection.SSLRootCertURL,
		})
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("%v", err)
		}

		// Create connection object with SSL configuration
		connection := models.Connection{
			Type:           req.Connection.Type,
			Host:           req.Connection.Host,
			Port:           req.Connection.Port,
			Username:       &req.Connection.Username,
			Password:       req.Connection.Password,
			Database:       req.Connection.Database,
			AuthDatabase:   req.Connection.AuthDatabase,
			UseSSL:         req.Connection.UseSSL,
			SSLMode:        req.Connection.SSLMode,
			SSLCertURL:     req.Connection.SSLCertURL,
			SSLKeyURL:      req.Connection.SSLKeyURL,
			SSLRootCertURL: req.Connection.SSLRootCertURL,
			Base:           models.NewBase(),
		}

		// Encrypt connection details
		if err := utils.EncryptConnection(&connection); err != nil {
			log.Printf("Warning: Failed to encrypt connection details: %v", err)
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to secure connection details: %v", err)
		}

		// If credentials changed, disconnect existing connection
		if credentialsChanged {
			log.Printf("ChatService -> Update -> Critical connection details changed, disconnecting existing connection")
			if err := s.dbManager.Disconnect(chatID, userID, true); err != nil {
				log.Printf("ChatService -> Update -> Warning: Failed to disconnect existing connection: %v", err)
				// Don't return error as we still want to update the connection details
			}
		}

		chat.Connection = connection

		// If credentials changed, reset selected collections
		if credentialsChanged {
			log.Printf("ChatService -> Update -> Resetting selected collections due to connection change")
			chat.SelectedCollections = ""
		}
	}

	// Store the old selected collections value to check for changes
	oldSelectedCollections := chat.SelectedCollections
	// Flag to track if selected collections changed
	selectedCollectionsChanged := false

	// Update selected collections if provided
	if req.SelectedCollections != nil {
		if oldSelectedCollections != *req.SelectedCollections {
			selectedCollectionsChanged = true
			log.Printf("ChatService -> Update -> Selected collections changed from '%s' to '%s'", oldSelectedCollections, *req.SelectedCollections)
		}
		chat.SelectedCollections = *req.SelectedCollections
	}

	// Update auto execute query if provided
	if req.Settings != nil {
		if req.Settings.AutoExecuteQuery != nil {
			log.Printf("ChatService -> Update -> AutoExecuteQuery: %v", *req.Settings.AutoExecuteQuery)
			chat.Settings.AutoExecuteQuery = *req.Settings.AutoExecuteQuery
		}
		if req.Settings.ShareDataWithAI != nil {
			log.Printf("ChatService -> Update -> ShareDataWithAI: %v", *req.Settings.ShareDataWithAI)
			chat.Settings.ShareDataWithAI = *req.Settings.ShareDataWithAI
		}
	}

	// Update the chat
	if err := s.chatRepo.Update(chatObjID, chat); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update chat: %v", err)
	}

	// If selected collections changed, trigger a schema refresh
	if selectedCollectionsChanged {
		log.Printf("ChatService -> Update -> Triggering schema refresh due to selected collections change")
		go func() {
			// Create a completely new context with a much longer timeout
			// This ensures it's not tied to the API request context
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
			defer cancel()

			log.Printf("ChatService -> Update -> Starting schema refresh with 60-minute timeout")
			_, err := s.RefreshSchema(ctx, userID, chatID, false)
			if err != nil {
				log.Printf("ChatService -> Update -> Error refreshing schema: %v", err)
			}
		}()
	}

	return s.buildChatResponse(chat), http.StatusOK, nil
}

// Delete a chat
func (s *chatService) Delete(userID, chatID string) (uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	// Verify ownership
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	// Delete chat and its messages
	if err := s.chatRepo.Delete(chatObjID); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete chat: %v", err)
	}

	// Delete messages
	if err := s.chatRepo.DeleteMessages(chatObjID); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete chat messages: %v", err)
	}

	// Delete LLM messages
	if err := s.llmRepo.DeleteMessagesByChatID(chatObjID, false); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete chat messages: %v", err)
	}

	go func() {
		// Delete DB connection
		if err := s.dbManager.Disconnect(chatID, userID, true); err != nil {
			log.Printf("failed to delete DB connection: %v", err)
		}
	}()

	return http.StatusOK, nil
}

// Get a chat by ID
func (s *chatService) GetByID(userID, chatID string) (*dtos.ChatResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	return s.buildChatResponse(chat), http.StatusOK, nil
}

// List all chats for a user
func (s *chatService) List(userID string, page, pageSize int) (*dtos.ChatListResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chats, total, err := s.chatRepo.FindByUserID(userObjID, page, pageSize)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chats: %v", err)
	}

	response := &dtos.ChatListResponse{
		Chats: make([]dtos.ChatResponse, len(chats)),
		Total: total,
	}

	for i, chat := range chats {
		response.Chats[i] = *s.buildChatResponse(chat)
	}

	return response, http.StatusOK, nil
}

// Create a new message
func (s *chatService) CreateMessage(ctx context.Context, userID, chatID string, streamID string, content string) (*dtos.MessageResponse, uint16, error) {
	// Validate chat exists and user has access
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}

	// Create and save the user message first
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	msg := &models.Message{
		Base:    models.NewBase(),
		UserID:  userObjID,
		ChatID:  chatObjID,
		Content: content,
		Type:    string(constants.MessageTypeUser),
	}

	if err := s.chatRepo.CreateMessage(msg); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to save message: %v", err)
	}

	// Make LLM Message
	llmMsg := &models.LLMMessage{
		Base:      models.NewBase(),
		UserID:    userObjID,
		ChatID:    chatObjID,
		MessageID: msg.ID,
		Role:      string(constants.MessageTypeUser),
		Content: map[string]interface{}{
			"user_message": content,
		},
	}
	if err := s.llmRepo.CreateMessage(llmMsg); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to save LLM message: %v", err)
	}

	log.Printf("ChatService -> CreateMessage -> AutoExecuteQuery: %v", chat.Settings.AutoExecuteQuery)
	// If auto execute query is true, we need to process LLM response & run query automatically
	if chat.Settings.AutoExecuteQuery {
		if err := s.processLLMResponseAndRunQuery(ctx, userID, chatID, msg.ID.Hex(), streamID); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to process message: %v", err)
		}
	} else {
		// Start processing the message asynchronously
		if err := s.processMessage(ctx, userID, chatID, msg.ID.Hex(), streamID); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to process message: %v", err)
		}
	}

	// Return the actual message ID
	return &dtos.MessageResponse{
		ID:        msg.ID.Hex(), // Use actual message ID
		ChatID:    chatID,
		Content:   content,
		Type:      string(constants.MessageTypeUser),
		CreatedAt: msg.CreatedAt.Format(time.RFC3339),
	}, http.StatusOK, nil
}

// Update a message
func (s *chatService) UpdateMessage(ctx context.Context, userID, chatID, messageID string, streamID string, req *dtos.CreateMessageRequest) (*dtos.MessageResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	messageObjID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid message ID format")
	}

	message, err := s.chatRepo.FindMessageByID(messageObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch message: %v", err)
	}

	if message.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized access to message")
	}

	if message.ChatID != chatObjID {
		return nil, http.StatusBadRequest, fmt.Errorf("message does not belong to chat")
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}

	log.Printf("UpdateMessage -> content: %+v", req.Content)
	// Update message content, This is a user message
	message.Content = req.Content
	message.IsEdited = true
	log.Printf("UpdateMessage -> message: %+v", message)
	log.Printf("UpdateMessage -> message.Content: %+v", message.Content)
	err = s.chatRepo.UpdateMessage(message.ID, message)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update message: %v", err)
	}

	// Find the next AI message after the edited message
	nextMessage, err := s.chatRepo.FindNextMessageByID(messageObjID)
	if err == nil && nextMessage != nil && nextMessage.Type == string(constants.MessageTypeAssistant) {
		log.Printf("UpdateMessage -> Found next AI message: %v", nextMessage.ID)

		// Reset query states for the AI message
		if nextMessage.Queries != nil {
			for i := range *nextMessage.Queries {
				(*nextMessage.Queries)[i].IsExecuted = false
				(*nextMessage.Queries)[i].IsRolledBack = false
				(*nextMessage.Queries)[i].ExecutionResult = nil
				(*nextMessage.Queries)[i].ExecutionTime = nil
				(*nextMessage.Queries)[i].Error = nil
			}

			// Update the AI message with reset query states
			if err := s.chatRepo.UpdateMessage(nextMessage.ID, nextMessage); err != nil {
				log.Printf("UpdateMessage -> Failed to update AI message: %v", err)
				// Continue even if this fails, as it's not critical
			}
		}
	}

	llmMsg, err := s.llmRepo.FindMessageByChatMessageID(message.ID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch LLM message: %v", err)
	}

	log.Printf("UpdateMessage -> llmMsg: %+v", llmMsg)
	llmMsg.Content = map[string]interface{}{
		"user_message": req.Content,
	}

	if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update LLM message: %v", err)
	}

	// If auto execute query is true, we need to process LLM response & run query automatically
	if chat.Settings.AutoExecuteQuery {
		if err := s.processLLMResponseAndRunQuery(ctx, userID, chatID, messageID, streamID); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to process message: %v", err)
		}
	} else {
		// Start processing the message asynchronously
		if err := s.processMessage(ctx, userID, chatID, messageID, streamID); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to process message: %v", err)
		}
	}
	return s.buildMessageResponse(message), http.StatusOK, nil
}

// Delete messages
func (s *chatService) DeleteMessages(userID, chatID string) (uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	// Verify chat ownership
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	if err := s.chatRepo.DeleteMessages(chatObjID); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete messages: %v", err)
	}

	// Delete LLM messages
	if err := s.llmRepo.DeleteMessagesByChatID(chatObjID, true); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete LLM messages: %v", err)
	}

	return http.StatusOK, nil
}

// Duplicate a chat
func (s *chatService) Duplicate(userID, chatID string, duplicateMessages bool) (*dtos.ChatResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	// Verify chat ownership
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	// Duplicate the chat
	newChat := &models.Chat{
		UserID:              userObjID,
		Connection:          chat.Connection,
		SelectedCollections: chat.SelectedCollections,
		Settings:            chat.Settings,
		Base:                models.NewBase(), // Create a new Base with new ID and timestamps
	}

	if err := s.chatRepo.Create(newChat); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to create duplicate chat: %v", err)
	}

	// if duplicateMessages is true, then we duplicate both regular messages and LLM messages
	if duplicateMessages {
		// Create a mapping of old message IDs to new message IDs to maintain relationships
		messageIDMap := make(map[primitive.ObjectID]primitive.ObjectID)
		messageIDMapMutex := &sync.Mutex{}

		// First, get all messages in the original chat in a single query to maintain their ordering
		allMessages, _, err := s.chatRepo.FindMessagesByChat(chatObjID, 1, 1000) // Large page size to get all
		if err != nil {
			log.Printf("Warning: failed to fetch messages: %v", err)
			// Continue without messages, at least the chat was duplicated
			return s.buildChatResponse(newChat), http.StatusOK, nil
		}

		if len(allMessages) > 0 {
			// Sort messages by created_at to ensure proper ordering
			sort.Slice(allMessages, func(i, j int) bool {
				return allMessages[i].CreatedAt.Before(allMessages[j].CreatedAt)
			})

			log.Printf("Duplicating %d messages in order", len(allMessages))

			// Process messages sequentially to ensure correct ordering
			baseTime := time.Now()
			for i, originalMsg := range allMessages {
				// Create a new message with the same content but for the new chat
				newMsg := &models.Message{
					UserID:   userObjID,
					ChatID:   newChat.ID,
					Type:     originalMsg.Type,
					Content:  originalMsg.Content,
					IsEdited: originalMsg.IsEdited,
					Base:     models.NewBase(), // Create a new Base with new ID and timestamps
				}

				// Set timestamps with precise sequential ordering
				newMsg.CreatedAt = baseTime.Add(time.Duration(i*1000) * time.Millisecond) // 1 second increment
				newMsg.UpdatedAt = newMsg.CreatedAt

				if originalMsg.UserMessageId != nil {
					messageIDMapMutex.Lock()
					if newID, exists := messageIDMap[*originalMsg.UserMessageId]; exists {
						newMsg.UserMessageId = &newID
					}
					messageIDMapMutex.Unlock()
				}

				// Copy queries if they exist
				if originalMsg.Queries != nil {
					queries := make([]models.Query, len(*originalMsg.Queries))
					for i, q := range *originalMsg.Queries {
						// Create a copy of the query with a new ID
						queries[i] = models.Query{
							ID:                     primitive.NewObjectID(),
							Query:                  q.Query,
							QueryType:              q.QueryType,
							Tables:                 q.Tables,
							Description:            q.Description,
							RollbackDependentQuery: q.RollbackDependentQuery, // Will update in second pass
							RollbackQuery:          q.RollbackQuery,
							ExecutionTime:          q.ExecutionTime,
							ExampleExecutionTime:   q.ExampleExecutionTime,
							CanRollback:            q.CanRollback,
							IsCritical:             q.IsCritical,
							IsExecuted:             false, // Reset execution state in the duplicate
							IsRolledBack:           false, // Reset rollback state
							Error:                  q.Error,
							ExampleResult:          q.ExampleResult,
							ExecutionResult:        nil, // Clear execution results
							IsEdited:               q.IsEdited,
							Metadata:               q.Metadata,
							ActionAt:               q.ActionAt,
						}

						// Copy pagination if it exists
						if q.Pagination != nil {
							queries[i].Pagination = &models.Pagination{
								TotalRecordsCount: q.Pagination.TotalRecordsCount,
								PaginatedQuery:    q.Pagination.PaginatedQuery,
								CountQuery:        q.Pagination.CountQuery,
							}
						}
					}
					newMsg.Queries = &queries
				}

				// Copy action buttons if they exist
				if originalMsg.ActionButtons != nil {
					actionButtons := make([]models.ActionButton, len(*originalMsg.ActionButtons))
					for i, btn := range *originalMsg.ActionButtons {
						actionButtons[i] = models.ActionButton{
							ID:        primitive.NewObjectID(),
							Label:     btn.Label,
							Action:    btn.Action,
							IsPrimary: btn.IsPrimary,
						}
					}
					newMsg.ActionButtons = &actionButtons
				}

				// Save the new message
				if err := s.chatRepo.CreateMessage(newMsg); err != nil {
					log.Printf("Error duplicating message: %v", err)
					continue
				}

				// Store the ID mapping
				messageIDMapMutex.Lock()
				messageIDMap[originalMsg.ID] = newMsg.ID
				messageIDMapMutex.Unlock()
			}
		}

		// Now handle LLM messages
		allLLMMessages, _, err := s.llmRepo.FindMessagesByChatID(chatObjID)
		if err != nil {
			log.Printf("Warning: failed to fetch LLM messages: %v", err)
			// Continue without LLM messages
			return s.buildChatResponse(newChat), http.StatusOK, nil
		}

		if len(allLLMMessages) > 0 {
			// Sort LLM messages by created_at to ensure proper ordering
			sort.Slice(allLLMMessages, func(i, j int) bool {
				return allLLMMessages[i].CreatedAt.Before(allLLMMessages[j].CreatedAt)
			})

			log.Printf("Duplicating %d LLM messages in order", len(allLLMMessages))

			// Process LLM messages sequentially
			baseLLMTime := time.Now().Add(time.Hour) // Use a different time base to differentiate from regular messages
			for i, llmMsg := range allLLMMessages {
				// Create a new LLM message with the same content but for the new chat
				newLLMMsg := &models.LLMMessage{
					ChatID:   newChat.ID,
					UserID:   userObjID,
					Role:     llmMsg.Role,
					Content:  llmMsg.Content, // Copy the content map
					IsEdited: llmMsg.IsEdited,
					Base:     models.NewBase(), // Create a new Base with new ID and timestamps
				}

				// Set unique timestamps
				newLLMMsg.CreatedAt = baseLLMTime.Add(time.Duration(i*1000) * time.Millisecond) // 1 second increment
				newLLMMsg.UpdatedAt = newLLMMsg.CreatedAt

				// Map the original message ID to the new one
				messageIDMapMutex.Lock()
				newID, exists := messageIDMap[llmMsg.MessageID]
				messageIDMapMutex.Unlock()

				if exists {
					newLLMMsg.MessageID = newID
					log.Printf("Mapping LLM message: original message ID %s -> new message ID %s",
						llmMsg.MessageID.Hex(), newID.Hex())
				} else {
					// If the message ID isn't mapped, create a new ID
					newLLMMsg.MessageID = primitive.NewObjectID()
					log.Printf("Warning: couldn't find mapping for message ID %s when duplicating LLM message",
						llmMsg.MessageID.Hex())
				}

				// Save the new LLM message
				if err := s.llmRepo.CreateMessage(newLLMMsg); err != nil {
					log.Printf("Error duplicating LLM message: %v", err)
					continue
				}
			}
		}

		// Second pass to update complex relationships if needed
		newMessages, _, err := s.chatRepo.FindMessagesByChat(newChat.ID, 1, 1000)
		if err == nil && len(newMessages) > 0 {
			for _, message := range newMessages {
				needsUpdate := false

				// Update query relationships if needed
				if message.Queries != nil {
					for i := range *message.Queries {
						// Update RollbackDependentQuery if it exists
						if (*message.Queries)[i].RollbackDependentQuery != nil {
							// For simplicity, set to nil
							(*message.Queries)[i].RollbackDependentQuery = nil
							needsUpdate = true
						}
					}
				}

				if needsUpdate {
					if err := s.chatRepo.UpdateMessage(message.ID, message); err != nil {
						log.Printf("Error updating duplicated message relationships: %v", err)
					}
				}
			}
		}

		log.Printf("Chat duplication completed successfully with messages. New chat ID: %s", newChat.ID.Hex())
	}

	return s.buildChatResponse(newChat), http.StatusOK, nil
}

// List messages for a chat
func (s *chatService) ListMessages(userID, chatID string, page, pageSize int) (*dtos.MessageListResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	// Verify chat ownership
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	messages, total, err := s.chatRepo.FindLatestMessageByChat(chatObjID, page, pageSize)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch messages: %v", err)
	}

	response := &dtos.MessageListResponse{
		Messages: make([]dtos.MessageResponse, len(messages)),
		Total:    total,
	}

	for i, msg := range messages {
		response.Messages[i] = *s.buildMessageResponse(msg)
	}

	return response, http.StatusOK, nil
}

// Edit a query, this can be done only before the query is executed
func (s *chatService) EditQuery(ctx context.Context, userID, chatID, messageID, queryID string, query string) (*dtos.EditQueryResponse, uint32, error) {
	log.Printf("ChatService -> EditQuery -> userID: %s, chatID: %s, messageID: %s, queryID: %s, query: %s", userID, chatID, messageID, queryID, query)

	_, message, queryData, err := s.verifyQueryOwnership(userID, chatID, messageID, queryID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	if queryData.IsExecuted || queryData.IsRolledBack {
		return nil, http.StatusBadRequest, fmt.Errorf("query has already been executed, cannot edit")
	}

	originalQuery := queryData.Query
	// Fix the query update logic
	for i := range *message.Queries {
		if (*message.Queries)[i].ID == queryData.ID {
			(*message.Queries)[i].Query = query
			(*message.Queries)[i].IsEdited = true
			if (*message.Queries)[i].Pagination != nil && (*message.Queries)[i].Pagination.PaginatedQuery != nil {
				(*message.Queries)[i].Pagination.PaginatedQuery = utils.ToStringPtr(strings.Replace(*(*message.Queries)[i].Pagination.PaginatedQuery, originalQuery, query, 1))
			}
		}
	}

	message.IsEdited = true
	if err := s.chatRepo.UpdateMessage(message.ID, message); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to update message: %v", err)
	}

	// Update the query in LLM messages too
	llmMsg, err := s.llmRepo.FindMessageByChatMessageID(message.ID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to find LLM message: %v", err)
	}

	if assistantResponse, ok := llmMsg.Content["assistant_response"].(map[string]interface{}); ok {
		log.Printf("ChatService -> EditQuery -> assistantResponse: %+v", assistantResponse)
		log.Printf("ChatService -> EditQuery -> queries type: %T", assistantResponse["queries"])

		llmMsg.IsEdited = true
		queries := assistantResponse["queries"]
		// Handle primitive.A (BSON array) type
		switch queriesVal := queries.(type) {
		case primitive.A:
			for i, q := range queriesVal {
				qMap, ok := q.(map[string]interface{})
				if !ok {
					continue
				}
				if strings.Replace(qMap["query"].(string), "EDITED by user: ", "", 1) == queryData.Query && qMap["queryType"] == *queryData.QueryType && qMap["explanation"] == queryData.Description {
					qMap["query"] = "EDITED by user: " + query // Telling the LLM that the query has been edited
					qMap["is_edited"] = true
					qMap["is_executed"] = false
					if qMap["pagination"] != nil {
						if qMap["pagination"].(map[string]interface{})["paginated_query"] != nil {
							currentPaginatedQuery := qMap["pagination"].(map[string]interface{})["paginated_query"].(string)
							qMap["pagination"].(map[string]interface{})["paginated_query"] = utils.ToStringPtr(strings.Replace(currentPaginatedQuery, originalQuery, query, 1))
						}
					}
					queriesVal[i] = qMap
					break
				}
			}
			assistantResponse["queries"] = queriesVal
		case []interface{}:
			for i, q := range queriesVal {
				qMap, ok := q.(map[string]interface{})
				if !ok {
					continue
				}
				if qMap["id"] == queryData.ID {
					qMap["query"] = "EDITED by user: " + query // Telling the LLM that the query has been edited
					qMap["is_edited"] = true
					qMap["is_executed"] = false
					if qMap["pagination"] != nil {
						currentPaginatedQuery := qMap["pagination"].(map[string]interface{})["paginated_query"].(string)
						qMap["pagination"].(map[string]interface{})["paginated_query"] = utils.ToStringPtr(strings.Replace(currentPaginatedQuery, originalQuery, query, 1))
					}
					queriesVal[i] = qMap
					break
				}
			}
			assistantResponse["queries"] = queriesVal
		}
	}

	if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to update LLM message: %v", err)
	}

	return &dtos.EditQueryResponse{
		ChatID:    chatID,
		MessageID: messageID,
		QueryID:   queryID,
		Query:     query,
		IsEdited:  true,
	}, http.StatusOK, nil
}

// Get the DB connection status for current chat
func (s *chatService) GetDBConnectionStatus(ctx context.Context, userID, chatID string) (*dtos.ConnectionStatusResponse, uint32, error) {
	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return nil, http.StatusNotFound, fmt.Errorf("no connection found")
	}

	// Check if connection is active
	isConnected := s.dbManager.IsConnected(chatID)

	// Convert port string to int
	var port *int
	if connInfo.Config.Port != nil {
		portVal, err := strconv.Atoi(*connInfo.Config.Port)
		if err != nil {
			defaultPort := 0
			port = &defaultPort // Default value if conversion fails
		} else {
			port = &portVal
		}
	}

	return &dtos.ConnectionStatusResponse{
		IsConnected: isConnected,
		Type:        connInfo.Config.Type,
		Host:        connInfo.Config.Host,
		Port:        port,
		Database:    connInfo.Config.Database,
		Username:    *connInfo.Config.Username,
	}, http.StatusOK, nil
}

// HandleSchemaChange handles schema changes
func (s *chatService) HandleSchemaChange(userID, chatID, streamID string, diff *dbmanager.SchemaDiff) {
	log.Printf("ChatService -> HandleSchemaChange -> Starting for chatID: %s", chatID)

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		log.Printf("ChatService -> HandleSchemaChange -> Connection not found for chat ID: %s", chatID)
		return
	}

	// Get database connection
	dbConn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Failed to get database connection: %v", err)
		return
	}

	// Get chat to get selected collections
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Error getting chatID: %v", err)
		return
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Error finding chat: %v", err)
		return
	}

	if chat == nil {
		log.Printf("ChatService -> HandleSchemaChange -> Chat not found for chatID: %s", chatID)
		return
	}

	// Convert the selectedCollections string to a slice
	var selectedCollectionsSlice []string
	if chat.SelectedCollections != "ALL" && chat.SelectedCollections != "" {
		selectedCollectionsSlice = strings.Split(chat.SelectedCollections, ",")
	}
	log.Printf("ChatService -> HandleSchemaChange -> Selected collections: %v", selectedCollectionsSlice)

	// Convert to ObjectID
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Invalid user ID format: %v", err)
		return
	}

	// Convert chat ID to ObjectID
	chatObjID, err = primitive.ObjectIDFromHex(chatID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Invalid chat ID format: %v", err)
		return
	}

	// Clear previous system message from LLM
	if err := s.llmRepo.DeleteMessagesByRole(chatObjID, string(constants.MessageTypeSystem)); err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Error deleting system message: %v", err)
	}

	// Format the schema changes for LLM
	if diff != nil {
		log.Printf("ChatService -> HandleSchemaChange -> diff: %+v", diff)

		// Need to update the chat LLM messages with the new schema
		// Only do full schema comparison if changes detected
		ctx := context.Background()
		var schemaMsg string
		if diff.IsFirstTime {
			// For first time, format the full schema with examples
			schemaMsg, err = s.dbManager.FormatSchemaWithExamples(ctx, chatID, selectedCollectionsSlice)
			if err != nil {
				log.Printf("ChatService -> HandleSchemaChange -> Error formatting schema with examples: %v", err)
				// Fall back to the old method if there's an error
				schemaMsg = s.dbManager.GetSchemaManager().FormatSchemaForLLM(diff.FullSchema)
			}
		} else {
			// For subsequent changes, get current schema with examples and show changes
			schemaMsg, err = s.dbManager.FormatSchemaWithExamples(ctx, chatID, selectedCollectionsSlice)
			if err != nil {
				log.Printf("ChatService -> HandleSchemaChange -> Error formatting schema with examples: %v", err)
				// Fall back to the old method if there's an error, but still use selected collections
				schema, schemaErr := s.dbManager.GetSchemaManager().GetSchema(ctx, chatID, dbConn, connInfo.Config.Type, selectedCollectionsSlice)
				if schemaErr != nil {
					log.Printf("ChatService -> HandleSchemaChange -> Error getting schema: %v", schemaErr)
					return
				}
				schemaMsg = s.dbManager.GetSchemaManager().FormatSchemaForLLM(schema)
			}
		}

		// Create LLM message with schema
		llmMsg := &models.LLMMessage{
			Base:   models.NewBase(),
			UserID: userObjID,
			ChatID: chatObjID,
			Role:   string(constants.MessageTypeSystem),
			Content: map[string]interface{}{
				"schema_update": schemaMsg,
			},
		}

		// Save LLM message
		if err := s.llmRepo.CreateMessage(llmMsg); err != nil {
			log.Printf("ChatService -> HandleSchemaChange -> Error saving LLM message: %v", err)
			return
		}

		log.Printf("ChatService -> HandleSchemaChange -> Schema update message saved")
	}
}

// Helper methods for building responses

func (s *chatService) buildChatResponse(chat *models.Chat) *dtos.ChatResponse {
	// Create a copy of the connection to avoid modifying the original
	connectionCopy := chat.Connection

	// Decrypt connection details for the response
	utils.DecryptConnection(&connectionCopy)

	return &dtos.ChatResponse{
		ID:     chat.ID.Hex(),
		UserID: chat.UserID.Hex(),
		Connection: dtos.ConnectionResponse{
			ID:             chat.ID.Hex(),
			Type:           connectionCopy.Type,
			Host:           connectionCopy.Host,
			Port:           connectionCopy.Port,
			Username:       *connectionCopy.Username,
			Database:       connectionCopy.Database,
			IsExampleDB:    connectionCopy.IsExampleDB,
			UseSSL:         connectionCopy.UseSSL,
			SSLMode:        connectionCopy.SSLMode,
			SSLCertURL:     connectionCopy.SSLCertURL,
			SSLKeyURL:      connectionCopy.SSLKeyURL,
			SSLRootCertURL: connectionCopy.SSLRootCertURL,
		},
		SelectedCollections: chat.SelectedCollections,
		CreatedAt:           chat.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           chat.UpdatedAt.Format(time.RFC3339),
		Settings: dtos.ChatSettingsResponse{
			AutoExecuteQuery: chat.Settings.AutoExecuteQuery,
			ShareDataWithAI:  chat.Settings.ShareDataWithAI,
		},
	}
}

func (s *chatService) buildMessageResponse(msg *models.Message) *dtos.MessageResponse {
	var userMessageID *string
	if msg.UserMessageId != nil {
		id := msg.UserMessageId.Hex()
		userMessageID = &id
	}

	queriesDto := dtos.ToQueryDto(msg.Queries)
	actionButtonsDto := dtos.ToActionButtonDto(msg.ActionButtons)

	return &dtos.MessageResponse{
		ID:            msg.ID.Hex(),
		ChatID:        msg.ChatID.Hex(),
		UserMessageID: userMessageID,
		Type:          msg.Type,
		Content:       msg.Content,
		Queries:       queriesDto,
		ActionButtons: actionButtonsDto,
		IsEdited:      msg.IsEdited,
		CreatedAt:     msg.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     msg.UpdatedAt.Format(time.RFC3339),
	}
}

// Verify query ownership checks if the query belongs to the message and the message belongs to the chat
func (s *chatService) verifyQueryOwnership(_, chatID, messageID, queryID string) (*models.Chat, *models.Message, *models.Query, error) {

	// Get chat
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid chat ID format")
	}
	chat, err := s.chatRepo.FindByID(chatObjID)

	// Convert IDs to ObjectIDs
	msgObjID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid message ID format")
	}

	queryObjID, err := primitive.ObjectIDFromHex(queryID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid query ID format")
	}

	// Get message
	msg, err := s.chatRepo.FindMessageByID(msgObjID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch message: %v", err)
	}
	if msg == nil {
		return nil, nil, nil, fmt.Errorf("message not found")
	}

	// Verify chat ownership
	if msg.ChatID.Hex() != chatID {
		return nil, nil, nil, fmt.Errorf("message does not belong to this chat")
	}

	log.Printf("ChatService -> verifyQueryOwnership -> msgObjID: %+v", msgObjID)
	log.Printf("ChatService -> verifyQueryOwnership -> queryObjID: %+v", queryObjID)
	log.Printf("ChatService -> verifyQueryOwnership -> msg.ChatID: %+v", msg.ChatID)

	log.Printf("ChatService -> verifyQueryOwnership -> msg: %+v", msg)
	// Find query in message
	var targetQuery *models.Query
	if msg.Queries != nil {
		for _, q := range *msg.Queries {
			if q.ID == queryObjID {
				targetQuery = &q
				break
			}
		}
	}
	if targetQuery == nil {
		return nil, nil, nil, fmt.Errorf("query not found in message")
	}

	return chat, msg, targetQuery, nil
}

// GetSelectedCollections retrieves the selected collections for a chat
// NOTE: This is used for UI display
func (s *chatService) GetSelectedCollections(chatID string) (string, error) {
	log.Printf("ChatService -> GetSelectedCollections -> Starting for chatID: %s", chatID)

	// Convert to ObjectID
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		log.Printf("ChatService -> GetSelectedCollections -> Error getting chatID: %v", err)
		return "ALL", fmt.Errorf("invalid chat ID format: %v", err)
	}

	// Get chat to get selected collections
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		log.Printf("ChatService -> GetSelectedCollections -> Error finding chat: %v", err)
		return "ALL", fmt.Errorf("failed to fetch chat: %v", err)
	}

	if chat == nil {
		log.Printf("ChatService -> GetSelectedCollections -> Chat not found for chatID: %s", chatID)
		return "ALL", fmt.Errorf("chat not found")
	}

	log.Printf("ChatService -> GetSelectedCollections -> Selected collections for chatID %s: %s", chatID, chat.SelectedCollections)

	// If SelectedCollections is empty, return "ALL"
	if chat.SelectedCollections == "" {
		return "ALL", nil
	}

	return chat.SelectedCollections, nil
}

// Fetch all tables for a chat
// NOTE: This is used for UI display
func (s *chatService) GetAllTables(ctx context.Context, userID, chatID string) (*dtos.TablesResponse, uint32, error) {
	log.Printf("ChatService -> GetAllTables -> Starting for chatID: %s", chatID)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, http.StatusRequestTimeout, fmt.Errorf("request timed out")
	default:
		// Get chat details first
		chatObjID, err := primitive.ObjectIDFromHex(chatID)
		if err != nil {
			log.Printf("ChatService -> GetAllTables -> Error getting chatID: %v", err)
			return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
		}

		chat, err := s.chatRepo.FindByID(chatObjID)
		if err != nil {
			log.Printf("ChatService -> GetAllTables -> Error finding chat: %v", err)
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
		}

		if chat != nil {
			// Try to decrypt the connection details
			utils.DecryptConnection(&chat.Connection)
		}

		if chat == nil {
			log.Printf("ChatService -> GetAllTables -> Chat not found for chatID: %s", chatID)
			return nil, http.StatusNotFound, fmt.Errorf("chat not found")
		}

		// Get database connection
		dbConn, err := s.dbManager.GetConnection(chatID)
		if err != nil {
			log.Printf("ChatService -> GetAllTables -> Connection not found, attempting to connect: %v", err)

			// Connection not found, try to connect with proper config
			connectErr := s.dbManager.Connect(chatID, userID, "", dbmanager.ConnectionConfig{
				Type:         chat.Connection.Type,
				Host:         chat.Connection.Host,
				Port:         chat.Connection.Port,
				Username:     chat.Connection.Username,
				Password:     chat.Connection.Password,
				Database:     chat.Connection.Database,
				AuthDatabase: chat.Connection.AuthDatabase,
			})
			if connectErr != nil {
				log.Printf("ChatService -> GetAllTables -> Failed to connect: %v", connectErr)
				return nil, http.StatusNotFound, fmt.Errorf("failed to establish database connection: %v", connectErr)
			}

			// Try to get connection again after connecting
			dbConn, err = s.dbManager.GetConnection(chatID)
			if err != nil {
				log.Printf("ChatService -> GetAllTables -> Still failed to get connection after connect: %v", err)
				return nil, http.StatusNotFound, fmt.Errorf("connection established but not ready yet: %v", err)
			}
		}

		// Get connection info
		connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
		if !exists {
			log.Printf("ChatService -> GetAllTables -> Connection info not found")
			return nil, http.StatusNotFound, fmt.Errorf("connection info not found")
		}

		// Convert the selectedCollections string to a slice
		var selectedCollectionsSlice []string
		if chat.SelectedCollections != "ALL" && chat.SelectedCollections != "" {
			selectedCollectionsSlice = strings.Split(chat.SelectedCollections, ",")
		}
		log.Printf("ChatService -> GetAllTables -> Selected collections: %v", selectedCollectionsSlice)

		// Create a map for quick lookup of selected tables
		selectedTablesMap := make(map[string]bool)
		for _, tableName := range selectedCollectionsSlice {
			selectedTablesMap[tableName] = true
		}
		isAllSelected := chat.SelectedCollections == "ALL" || chat.SelectedCollections == ""

		// Get schema manager
		schemaManager := s.dbManager.GetSchemaManager()

		log.Printf("ChatService -> GetAllTables -> Getting schema for chatID -> Database Host, Name, Type: %+v, %+v, %+v", connInfo.Config.Host, connInfo.Config.Database, connInfo.Config.Type)
		// Get schema from database - pass empty slice to get ALL tables
		schema, err := schemaManager.GetSchema(ctx, chatID, dbConn, connInfo.Config.Type, []string{})
		if err != nil {
			log.Printf("ChatService -> GetAllTables -> Error getting schema: %v", err)
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to get schema: %v", err)
		}

		// Convert schema tables to TableInfo objects
		var tables []dtos.TableInfo
		for tableName, tableSchema := range schema.Tables {
			tableInfo := dtos.TableInfo{
				Name:       tableName,
				Columns:    make([]dtos.ColumnInfo, 0, len(tableSchema.Columns)),
				IsSelected: isAllSelected || selectedTablesMap[tableName],
			}

			for columnName, columnInfo := range tableSchema.Columns {
				tableInfo.Columns = append(tableInfo.Columns, dtos.ColumnInfo{
					Name:       columnName,
					Type:       columnInfo.Type,
					IsNullable: columnInfo.IsNullable,
				})
			}

			tables = append(tables, tableInfo)
		}

		// Sort tables by name for consistent output
		sort.Slice(tables, func(i, j int) bool {
			return tables[i].Name < tables[j].Name
		})

		return &dtos.TablesResponse{
			Tables: tables,
		}, http.StatusOK, nil
	}
}
