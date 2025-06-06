package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/dbmanager"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// NOTE: Service type, signatures are defined in services/chat_crud_service.go
func (s *chatService) handleError(_ context.Context, chatID string, err error) {
	log.Printf("Error processing message for chat %s: %v", chatID, err)
}

// processLLMResponse processes the LLM response updates SSE stream only if synchronous is false, allowSSEUpdates is used to send SSE updates to the client except the final ai-response event
func (s *chatService) processLLMResponse(ctx context.Context, userID, chatID, userMessageID, streamID string, synchronous bool, allowSSEUpdates bool) (*dtos.MessageResponse, error) {
	log.Printf("processLLMResponse -> userID: %s, chatID: %s, streamID: %s", userID, chatID, streamID)

	// Create cancellable context from the background context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, err
	}

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, err
	}

	userMessageObjID, err := primitive.ObjectIDFromHex(userMessageID)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, err
	}

	// Store cancel function
	s.processesMu.Lock()
	s.activeProcesses[streamID] = cancel
	s.processesMu.Unlock()

	// Cleanup when done
	defer func() {
		s.processesMu.Lock()
		delete(s.activeProcesses, streamID)
		s.processesMu.Unlock()
	}()

	if !synchronous || allowSSEUpdates {
		// Send initial processing message
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "NeoBase is analyzing your request..",
		})
	}

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		s.handleError(ctx, chatID, fmt.Errorf("connection info not found"))
		// Let's create a new connection
		_, err := s.ConnectDB(ctx, userID, chatID, streamID)
		if err != nil {
			// Send a error event to the client
			s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: "ai-response-error",
				Data:  "Error: " + err.Error(),
			})
			return nil, err
		}

	}

	// Fetch all the messages from the LLM
	messages, err := s.llmRepo.GetByChatID(chatObjID)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, err
	}

	// Filter messages up to the edited message
	filteredMessages := make([]*models.LLMMessage, 0)
	for _, msg := range messages {
		filteredMessages = append(filteredMessages, msg)
		if msg.MessageID == userMessageObjID {
			break
		}
	}

	// Helper function to check cancellation
	checkCancellation := func() bool {
		select {
		case <-ctx.Done():
			if !synchronous || allowSSEUpdates {
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "response-cancelled",
					Data:  "Operation cancelled by user",
				})
			}
			return true
		default:
			return false
		}
	}

	// Check cancellation before expensive operations
	if checkCancellation() {
		return nil, fmt.Errorf("operation cancelled")
	}

	if !synchronous || allowSSEUpdates {
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "Fetching relevant data points & structure for the query..",
		})

		// Send initial processing message
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "Generating an optimized query & results for the request..",
		})
	}
	if checkCancellation() {
		return nil, fmt.Errorf("operation cancelled")
	}

	// Generate LLM response
	response, err := s.llmClient.GenerateResponse(ctx, filteredMessages, connInfo.Config.Type)
	if err != nil {
		if !synchronous || allowSSEUpdates {
			s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: "ai-response-error",
				Data:  map[string]string{"error": "Error: " + err.Error()},
			})
		}
		return nil, fmt.Errorf("failed to generate LLM response: %v", err)
	}

	log.Printf("processLLMResponse -> response: %s", response)

	if checkCancellation() {
		return nil, fmt.Errorf("operation cancelled")
	}

	// Send initial processing message
	if !synchronous || allowSSEUpdates {
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "Analyzing the criticality of the query & if roll back is possible..",
		})
	}

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-error",
			Data:  map[string]string{"error": "Error: " + err.Error()},
		})
	}

	queries := []models.Query{}
	if jsonResponse["queries"] != nil {
		for _, query := range jsonResponse["queries"].([]interface{}) {
			queryMap := query.(map[string]interface{})
			var exampleResult *string
			log.Printf("processLLMResponse -> queryMap: %v", queryMap)
			if queryMap["exampleResult"] != nil {
				log.Printf("processLLMResponse -> queryMap[\"exampleResult\"]: %v", queryMap["exampleResult"])
				result, _ := json.Marshal(queryMap["exampleResult"].([]interface{}))
				exampleResult = utils.ToStringPtr(string(result))
				log.Printf("processLLMResponse -> saving exampleResult: %v", *exampleResult)
			} else {
				exampleResult = nil
				log.Println("processLLMResponse -> saving exampleResult: nil")
			}

			var rollbackDependentQuery *string
			if queryMap["rollbackDependentQuery"] != nil {
				rollbackDependentQuery = utils.ToStringPtr(queryMap["rollbackDependentQuery"].(string))
			} else {
				rollbackDependentQuery = nil
			}

			var estimateResponseTime *float64
			// First check if the estimateResponseTime is a string, if not string & it is float then set value
			if queryMap["estimateResponseTime"] != nil {
				switch v := queryMap["estimateResponseTime"].(type) {
				case string:
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						estimateResponseTime = &f
					} else {
						defaultVal := float64(100)
						estimateResponseTime = &defaultVal
					}
				case float64:
					estimateResponseTime = &v
				default:
					defaultVal := float64(100)
					estimateResponseTime = &defaultVal
				}
			} else {
				defaultVal := float64(100)
				estimateResponseTime = &defaultVal
			}

			log.Printf("processLLMResponse -> queryMap[\"pagination\"]: %v", queryMap["pagination"])
			pagination := &models.Pagination{}
			if queryMap["pagination"] != nil {
				if queryMap["pagination"].(map[string]interface{})["paginatedQuery"] != nil {
					pagination.PaginatedQuery = utils.ToStringPtr(queryMap["pagination"].(map[string]interface{})["paginatedQuery"].(string))
					log.Printf("processLLMResponse -> pagination.PaginatedQuery: %v", *pagination.PaginatedQuery)
				}
				if queryMap["pagination"].(map[string]interface{})["countQuery"] != nil {
					pagination.CountQuery = utils.ToStringPtr(queryMap["pagination"].(map[string]interface{})["countQuery"].(string))
					log.Printf("processLLMResponse -> pagination.CountQuery: %v", *pagination.CountQuery)
				}
			}
			var tables *string
			if queryMap["tables"] != nil {
				tables = utils.ToStringPtr(queryMap["tables"].(string))
			}

			if queryMap["collections"] != nil {
				tables = utils.ToStringPtr(queryMap["collections"].(string))
			}
			var queryType *string
			if queryMap["queryType"] != nil {
				queryType = utils.ToStringPtr(queryMap["queryType"].(string))
			}

			var rollbackQuery *string
			if queryMap["rollbackQuery"] != nil {
				rollbackQuery = utils.ToStringPtr(queryMap["rollbackQuery"].(string))
			}

			// Create the query object
			query := models.Query{
				ID:                     primitive.NewObjectID(),
				Query:                  queryMap["query"].(string),
				Description:            queryMap["explanation"].(string),
				ExecutionTime:          nil,
				ExampleExecutionTime:   int(*estimateResponseTime),
				CanRollback:            queryMap["canRollback"].(bool),
				IsCritical:             queryMap["isCritical"].(bool),
				IsExecuted:             false,
				IsRolledBack:           false,
				ExampleResult:          exampleResult,
				ExecutionResult:        nil,
				Error:                  nil,
				QueryType:              queryType,
				Tables:                 tables,
				RollbackQuery:          rollbackQuery,
				RollbackDependentQuery: rollbackDependentQuery,
				Pagination:             pagination,
			}

			// Handle ClickHouse-specific metadata
			if connInfo.Config.Type == constants.DatabaseTypeClickhouse {
				metadata := make(map[string]interface{})

				// Add ClickHouse-specific fields if they exist
				if queryMap["engineType"] != nil {
					metadata["engineType"] = queryMap["engineType"]
				}
				if queryMap["partitionKey"] != nil {
					metadata["partitionKey"] = queryMap["partitionKey"]
				}
				if queryMap["orderByKey"] != nil {
					metadata["orderByKey"] = queryMap["orderByKey"]
				}

				// Store metadata as JSON if we have any
				if len(metadata) > 0 {
					metadataJSON, err := json.Marshal(metadata)
					if err == nil {
						metadataStr := string(metadataJSON)
						query.Metadata = &metadataStr
					}
				}
			}

			queries = append(queries, query)
		}
	}

	log.Printf("processLLMResponse -> queries: %v", queries)

	// Extract action buttons from the LLM response
	var actionButtons []models.ActionButton
	if jsonResponse["actionButtons"] != nil {
		actionButtonsArray := jsonResponse["actionButtons"].([]interface{})
		if len(actionButtonsArray) > 0 {
			actionButtons = make([]models.ActionButton, 0, len(actionButtonsArray))
			for _, btn := range actionButtonsArray {
				btnMap := btn.(map[string]interface{})
				actionButton := models.ActionButton{
					ID:        primitive.NewObjectID(),
					Label:     btnMap["label"].(string),
					Action:    btnMap["action"].(string),
					IsPrimary: btnMap["isPrimary"].(bool),
				}
				actionButtons = append(actionButtons, actionButton)
			}
		}
	} else {
		actionButtons = []models.ActionButton{}
	}

	assistantMessage := ""
	if jsonResponse["assistantMessage"] != nil {
		assistantMessage = jsonResponse["assistantMessage"].(string)
	} else {
		assistantMessage = ""
	}

	// Find existing AI response message
	existingMessage, err := s.chatRepo.FindNextMessageByID(userMessageObjID)
	if err != nil && err != mongo.ErrNoDocuments {
		s.handleError(ctx, chatID, err)
		return nil, fmt.Errorf("failed to find existing AI message: %v", err)
	}

	// Convert queries and action buttons to the correct pointer type
	queriesPtr := &queries
	var actionButtonsPtr *[]models.ActionButton
	if len(actionButtons) > 0 {
		actionButtonsPtr = &actionButtons
	} else {
		// Clear action buttons
		actionButtonsPtr = &[]models.ActionButton{}
	}

	// If we found an existing AI message, update it instead of creating a new one
	if existingMessage != nil && existingMessage.Type == "assistant" {
		log.Printf("processLLMResponse -> Updating existing AI message: %v", existingMessage.ID)

		if actionButtonsPtr != nil && len(*actionButtonsPtr) > 0 {
			log.Printf("processLLMResponse -> saving existingMessage.ActionButtons: %v", *actionButtonsPtr)
		} else {
			log.Printf("processLLMResponse -> saving existingMessage.ActionButtons: nil or empty")
		}
		// Update the existing message with new content
		existingMessage.Content = assistantMessage
		existingMessage.Queries = queriesPtr // Now correctly typed as *[]models.Query
		existingMessage.ActionButtons = actionButtonsPtr
		existingMessage.IsEdited = true

		// Update the message in the database
		if err := s.chatRepo.UpdateMessage(existingMessage.ID, existingMessage); err != nil {
			s.handleError(ctx, chatID, err)
			return nil, fmt.Errorf("failed to update AI message: %v", err)
		}

		// Update the LLM message
		existingLLMMsg, err := s.llmRepo.FindMessageByChatMessageID(existingMessage.ID)
		if err != nil {
			s.handleError(ctx, chatID, err)
			return nil, fmt.Errorf("failed to fetch LLM message: %v", err)
		}

		formattedJsonResponse := map[string]interface{}{
			"assistant_response": jsonResponse,
		}
		existingLLMMsg.Content = formattedJsonResponse

		if err := s.llmRepo.UpdateMessage(existingLLMMsg.ID, existingLLMMsg); err != nil {
			s.handleError(ctx, chatID, err)
			return nil, fmt.Errorf("failed to update LLM message: %v", err)
		}

		if !synchronous {
			// Send final response
			s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: "ai-response",
				Data: &dtos.MessageResponse{
					ID:            existingMessage.ID.Hex(),
					ChatID:        existingMessage.ChatID.Hex(),
					Content:       existingMessage.Content,
					UserMessageID: utils.ToStringPtr(userMessageObjID.Hex()),
					Queries:       dtos.ToQueryDto(existingMessage.Queries),
					ActionButtons: dtos.ToActionButtonDto(existingMessage.ActionButtons),
					Type:          existingMessage.Type,
					CreatedAt:     existingMessage.CreatedAt.Format(time.RFC3339),
					UpdatedAt:     existingMessage.UpdatedAt.Format(time.RFC3339),
					IsEdited:      existingMessage.IsEdited,
				},
			})
		}

		return &dtos.MessageResponse{
			ID:            existingMessage.ID.Hex(),
			ChatID:        existingMessage.ChatID.Hex(),
			Content:       existingMessage.Content,
			UserMessageID: utils.ToStringPtr(userMessageObjID.Hex()),
			Queries:       dtos.ToQueryDto(existingMessage.Queries),
			ActionButtons: dtos.ToActionButtonDto(existingMessage.ActionButtons),
			Type:          existingMessage.Type,
			CreatedAt:     existingMessage.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     existingMessage.UpdatedAt.Format(time.RFC3339),
			IsEdited:      existingMessage.IsEdited,
		}, nil
	}

	log.Printf("processLLMResponse -> saving new message actionButtonsPtr: %v", actionButtonsPtr)
	// If no existing message found, create a new one
	// Use the messageObjID that was already defined above
	chatResponseMsg := &models.Message{
		Base:          models.NewBase(),
		UserID:        userObjID,
		ChatID:        chatObjID,
		Content:       assistantMessage,
		Type:          "assistant",
		Queries:       queriesPtr,
		ActionButtons: actionButtonsPtr,
		IsEdited:      false,
		UserMessageId: &userMessageObjID, // Set the user message ID that this AI message is responding to
	}

	if err := s.chatRepo.CreateMessage(chatResponseMsg); err != nil {
		log.Printf("processLLMResponse -> Error saving chat response message: %v", err)
		return nil, err
	}

	formattedJsonResponse := map[string]interface{}{
		"assistant_response": jsonResponse,
	}
	llmMsg := &models.LLMMessage{
		Base:      models.NewBase(),
		UserID:    userObjID,
		ChatID:    chatObjID,
		MessageID: chatResponseMsg.ID,
		Content:   formattedJsonResponse,
		Role:      string(constants.MessageTypeAssistant),
	}
	if err := s.llmRepo.CreateMessage(llmMsg); err != nil {
		log.Printf("processLLMResponse -> Error saving LLM message: %v", err)
	}

	if !synchronous {
		// Send final response
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response",
			Data: &dtos.MessageResponse{
				ID:            chatResponseMsg.ID.Hex(),
				ChatID:        chatResponseMsg.ChatID.Hex(),
				Content:       chatResponseMsg.Content,
				UserMessageID: utils.ToStringPtr(userMessageObjID.Hex()),
				Queries:       dtos.ToQueryDto(chatResponseMsg.Queries),
				ActionButtons: dtos.ToActionButtonDto(chatResponseMsg.ActionButtons),
				Type:          chatResponseMsg.Type,
				CreatedAt:     chatResponseMsg.CreatedAt.Format(time.RFC3339),
				UpdatedAt:     chatResponseMsg.UpdatedAt.Format(time.RFC3339),
			},
		})
	}
	return &dtos.MessageResponse{
		ID:            chatResponseMsg.ID.Hex(),
		ChatID:        chatResponseMsg.ChatID.Hex(),
		Content:       chatResponseMsg.Content,
		UserMessageID: utils.ToStringPtr(userMessageObjID.Hex()),
		Queries:       dtos.ToQueryDto(chatResponseMsg.Queries),
		ActionButtons: dtos.ToActionButtonDto(chatResponseMsg.ActionButtons),
		Type:          chatResponseMsg.Type,
		CreatedAt:     chatResponseMsg.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     chatResponseMsg.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// Cancels the ongoing LLM processing for the given streamID
func (s *chatService) CancelProcessing(userID, chatID, streamID string) {
	s.processesMu.Lock()
	defer s.processesMu.Unlock()

	log.Printf("CancelProcessing -> activeProcesses: %+v", s.activeProcesses)
	if cancel, exists := s.activeProcesses[streamID]; exists {
		log.Printf("CancelProcessing -> canceling LLM processing for streamID: %s", streamID)
		cancel() // Only cancels the LLM context
		delete(s.activeProcesses, streamID)

		go func() {
			chatObjID, err := primitive.ObjectIDFromHex(chatID)
			if err != nil {
				log.Printf("CancelProcessing -> error fetching chatID: %v", err)
			}

			userObjID, err := primitive.ObjectIDFromHex(userID)
			if err != nil {
				log.Printf("CancelProcessing -> error fetching userID: %v", err)
			}

			msg := &models.Message{
				Base:    models.NewBase(),
				ChatID:  chatObjID,
				UserID:  userObjID,
				Type:    string(constants.MessageTypeAssistant),
				Content: "Operation cancelled by user",
			}

			// Save cancelled event to database
			if err := s.chatRepo.CreateMessage(msg); err != nil {
				log.Printf("CancelProcessing -> error creating message: %v", err)
			}
		}()
		// Send cancelled event using stream
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "response-cancelled",
			Data:  "Operation cancelled by user",
		})
	}
}

// ConnectDB connects to a database for the chat
func (s *chatService) ConnectDB(ctx context.Context, userID, chatID string, streamID string) (uint32, error) {
	// Get chat
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return http.StatusNotFound, fmt.Errorf("chat not found")
		}
		return http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}

	// Check if chat belongs to user
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	if chat.UserID != userObjID {
		return http.StatusForbidden, fmt.Errorf("chat does not belong to user")
	}

	// Check if connection details are present
	if chat.Connection.Host == "" || chat.Connection.Database == "" {
		return http.StatusBadRequest, fmt.Errorf("connection details are incomplete")
	}

	// Decrypt connection details
	utils.DecryptConnection(&chat.Connection)

	// Ensure port has a default value if empty
	if chat.Connection.Port == nil || *chat.Connection.Port == "" {
		var defaultPort string
		switch chat.Connection.Type {
		case constants.DatabaseTypePostgreSQL:
			defaultPort = "5432"
		case constants.DatabaseTypeYugabyteDB:
			defaultPort = "5433"
		case constants.DatabaseTypeMySQL:
			defaultPort = "3306"
		case constants.DatabaseTypeClickhouse:
			defaultPort = "9000"
		case constants.DatabaseTypeMongoDB:
			defaultPort = "27017"
		}
		chat.Connection.Port = &defaultPort
	}

	// Connect to database
	err = s.dbManager.Connect(chatID, userID, streamID, dbmanager.ConnectionConfig{
		Type:           chat.Connection.Type,
		Host:           chat.Connection.Host,
		Port:           chat.Connection.Port,
		Username:       chat.Connection.Username,
		Password:       chat.Connection.Password,
		Database:       chat.Connection.Database,
		AuthDatabase:   chat.Connection.AuthDatabase, // Added AuthDatabase
		UseSSL:         chat.Connection.UseSSL,
		SSLMode:        chat.Connection.SSLMode,
		SSLCertURL:     chat.Connection.SSLCertURL,
		SSLKeyURL:      chat.Connection.SSLKeyURL,
		SSLRootCertURL: chat.Connection.SSLRootCertURL,
	})

	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Printf("ChatService -> ConnectDB -> Database already connected, skipping connection")
		} else {
			return http.StatusBadRequest, fmt.Errorf("failed to connect: %v", err)
		}
	}

	return http.StatusOK, nil
}

// DisconnectDB disconnects from a database for the chat
func (s *chatService) DisconnectDB(ctx context.Context, userID, chatID string, streamID string) (uint32, error) {
	log.Printf("ChatService -> DisconnectDB -> Starting for chatID: %s", chatID)

	// Subscribe to connection status updates before disconnecting
	s.dbManager.Subscribe(chatID, streamID)
	log.Printf("ChatService -> DisconnectDB -> Subscribed to updates with streamID: %s", streamID)

	if err := s.dbManager.Disconnect(chatID, userID, false); err != nil {
		log.Printf("ChatService -> DisconnectDB -> failed to disconnect: %v", err)
		return http.StatusBadRequest, fmt.Errorf("failed to disconnect: %v", err)
	}

	log.Printf("ChatService -> DisconnectDB -> disconnected from chat: %s", chatID)
	return http.StatusOK, nil
}

// ExecuteQuery executes a query, runs realtime query to connected database, stores the result in execution_result etc...
func (s *chatService) ExecuteQuery(ctx context.Context, userID, chatID string, req *dtos.ExecuteQueryRequest) (*dtos.QueryExecutionResponse, uint32, error) {
	// Verify message and query ownership
	chat, msg, query, err := s.verifyQueryOwnership(userID, chatID, req.MessageID, req.QueryID)
	if err != nil {
		return nil, http.StatusForbidden, err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, http.StatusRequestTimeout, fmt.Errorf("query execution cancelled or timed out")
	default:
		log.Printf("ChatService -> ExecuteQuery -> msg: %+v", msg)
	}

	// Check connection status and connect if needed
	if !s.dbManager.IsConnected(chatID) {
		log.Printf("ChatService -> ExecuteQuery -> Database not connected, initiating connection")
		status, err := s.ConnectDB(ctx, userID, chatID, req.StreamID)
		if err != nil {
			return nil, status, err
		}
		// Give a small delay for connection to stabilize
		time.Sleep(1 * time.Second)
	}

	var totalRecordsCount *int

	// To find total records count, we need to execute the pagination.countQuery with findCount = true
	if query.Pagination != nil && query.Pagination.CountQuery != nil && *query.Pagination.CountQuery != "" {
		log.Printf("ChatService -> ExecuteQuery -> query.Pagination.CountQuery is present, will use it to get the total records count")
		countResult, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, *query.Pagination.CountQuery, *query.QueryType, false, true)
		if queryErr != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error executing count query: %v", queryErr)
		}
		if countResult != nil && countResult.Result != nil {
			log.Printf("ChatService -> ExecuteQuery -> countResult.Result: %+v", countResult.Result)

			// Try to extract count from different possible formats

			// Format 1: Direct count in the result
			if countVal, ok := countResult.Result["count"].(float64); ok {
				tempCount := int(countVal)
				totalRecordsCount = &tempCount
				log.Printf("ChatService -> ExecuteQuery -> Found count directly in result: %d", tempCount)
			} else if countVal, ok := countResult.Result["count"].(int64); ok {
				tempCount := int(countVal)
				totalRecordsCount = &tempCount
				log.Printf("ChatService -> ExecuteQuery -> Found count directly in result (int64): %d", tempCount)
			} else if countVal, ok := countResult.Result["count"].(int); ok {
				totalRecordsCount = &countVal
				log.Printf("ChatService -> ExecuteQuery -> Found count directly in result (int): %d", countVal)
			} else if results, ok := countResult.Result["results"]; ok {
				// Format 2: Results is an array of objects with count
				if resultsList, ok := results.([]interface{}); ok && len(resultsList) > 0 {
					log.Printf("ChatService -> ExecuteQuery -> Results is a list with %d items", len(resultsList))

					// Try to get count from the first item
					if countObj, ok := resultsList[0].(map[string]interface{}); ok {
						if countVal, ok := countObj["count"].(float64); ok {
							tempCount := int(countVal)
							totalRecordsCount = &tempCount
							log.Printf("ChatService -> ExecuteQuery -> Found count in first result item: %d", tempCount)
						} else if countVal, ok := countObj["count"].(int64); ok {
							tempCount := int(countVal)
							totalRecordsCount = &tempCount
							log.Printf("ChatService -> ExecuteQuery -> Found count in first result item (int64): %d", tempCount)
						} else if countVal, ok := countObj["count"].(int); ok {
							totalRecordsCount = &countVal
							log.Printf("ChatService -> ExecuteQuery -> Found count in first result item (int): %d", countVal)
						} else {
							// For PostgreSQL, the count might be in a column named 'count'
							for key, value := range countObj {
								if strings.ToLower(key) == "count" {
									if countVal, ok := value.(float64); ok {
										tempCount := int(countVal)
										totalRecordsCount = &tempCount
										log.Printf("ChatService -> ExecuteQuery -> Found count in column '%s': %d", key, tempCount)
										break
									} else if countVal, ok := value.(int64); ok {
										tempCount := int(countVal)
										totalRecordsCount = &tempCount
										log.Printf("ChatService -> ExecuteQuery -> Found count in column '%s' (int64): %d", key, tempCount)
										break
									} else if countVal, ok := value.(int); ok {
										totalRecordsCount = &countVal
										log.Printf("ChatService -> ExecuteQuery -> Found count in column '%s' (int): %d", key, countVal)
										break
									} else if countStr, ok := value.(string); ok {
										// Handle case where count is returned as string
										if countVal, err := strconv.Atoi(countStr); err == nil {
											totalRecordsCount = &countVal
											log.Printf("ChatService -> ExecuteQuery -> Found count in column '%s' (string): %d", key, countVal)
											break
										}
									}
								}
							}
						}
					} else {
						// Handle case where the array element is not a map
						log.Printf("ChatService -> ExecuteQuery -> First item in results list is not a map: %T", resultsList[0])
					}
				} else if resultsMap, ok := results.(map[string]interface{}); ok {
					// Format 3: Results is a map with count
					log.Printf("ChatService -> ExecuteQuery -> Results is a map")
					if countVal, ok := resultsMap["count"].(float64); ok {
						tempCount := int(countVal)
						totalRecordsCount = &tempCount
						log.Printf("ChatService -> ExecuteQuery -> Found count in results map: %d", tempCount)
					} else if countVal, ok := resultsMap["count"].(int64); ok {
						tempCount := int(countVal)
						totalRecordsCount = &tempCount
						log.Printf("ChatService -> ExecuteQuery -> Found count in results map (int64): %d", tempCount)
					} else if countVal, ok := resultsMap["count"].(int); ok {
						totalRecordsCount = &countVal
						log.Printf("ChatService -> ExecuteQuery -> Found count in results map (int): %d", countVal)
					}
				} else if countVal, ok := results.(float64); ok {
					// Format 4: Results is directly a number
					tempCount := int(countVal)
					totalRecordsCount = &tempCount
					log.Printf("ChatService -> ExecuteQuery -> Results is a number: %d", tempCount)
				} else if countVal, ok := results.(int64); ok {
					tempCount := int(countVal)
					totalRecordsCount = &tempCount
					log.Printf("ChatService -> ExecuteQuery -> Results is a number (int64): %d", tempCount)
				} else if countVal, ok := results.(int); ok {
					totalRecordsCount = &countVal
					log.Printf("ChatService -> ExecuteQuery -> Results is a number (int): %d", countVal)
				} else {
					// Log the actual type for debugging
					log.Printf("ChatService -> ExecuteQuery -> Results has unexpected type: %T", results)
				}
			}

			// If we still couldn't extract the count, try a more direct approach for the specific format
			if totalRecordsCount == nil {
				// Try to handle the specific format: map[results:[map[count:92]]]
				if resultsRaw, ok := countResult.Result["results"]; ok {
					log.Printf("ChatService -> ExecuteQuery -> Trying direct approach for format: map[results:[map[count:92]]]")

					// Convert to JSON and back to ensure proper type handling
					jsonBytes, err := json.Marshal(resultsRaw)
					if err == nil {
						var resultsArray []map[string]interface{}
						if err := json.Unmarshal(jsonBytes, &resultsArray); err == nil && len(resultsArray) > 0 {
							if countVal, ok := resultsArray[0]["count"]; ok {
								// Try to convert to int
								switch v := countVal.(type) {
								case float64:
									tempCount := int(v)
									totalRecordsCount = &tempCount
									log.Printf("ChatService -> ExecuteQuery -> Found count using direct approach: %d", tempCount)
								case int64:
									tempCount := int(v)
									totalRecordsCount = &tempCount
									log.Printf("ChatService -> ExecuteQuery -> Found count using direct approach (int64): %d", tempCount)
								case int:
									totalRecordsCount = &v
									log.Printf("ChatService -> ExecuteQuery -> Found count using direct approach (int): %d", v)
								case string:
									if countInt, err := strconv.Atoi(v); err == nil {
										totalRecordsCount = &countInt
										log.Printf("ChatService -> ExecuteQuery -> Found count using direct approach (string): %d", countInt)
									}
								default:
									log.Printf("ChatService -> ExecuteQuery -> Count value has unexpected type: %T", v)
								}
							}
						}
					}
				}
			}

			if totalRecordsCount == nil {
				log.Printf("ChatService -> ExecuteQuery -> Could not extract count from result: %+v", countResult.Result)
			} else {
				log.Printf("ChatService -> ExecuteQuery -> Successfully extracted count: %d", *totalRecordsCount)
			}
		}
	}

	if totalRecordsCount != nil {
		log.Printf("ChatService -> ExecuteQuery -> totalRecordsCount: %+v", *totalRecordsCount)
	}
	queryToExecute := query.Query

	if query.Pagination != nil && query.Pagination.PaginatedQuery != nil && *query.Pagination.PaginatedQuery != "" {
		log.Printf("ChatService -> ExecuteQuery -> query.Pagination.PaginatedQuery is present, will use it to cap the result to 50 records. query.Pagination.PaginatedQuery: %+v", *query.Pagination.PaginatedQuery)
		// Capping the result to 50 records by default and skipping 0 records, we do not need to run the query.Query as we have better paginated query & already have the total records count

		queryToExecute = strings.Replace(*query.Pagination.PaginatedQuery, "offset_size", strconv.Itoa(0), 1)
	}

	log.Printf("ChatService -> ExecuteQuery -> queryToExecute: %+v", queryToExecute)
	// Execute query, we will be executing the pagination.paginatedQuery if it exists, else the query.Query
	result, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, queryToExecute, *query.QueryType, false, false)
	if queryErr != nil {
		// Checking if executed query was paginatedQuery, if so, let's try to execute it again with the original query
		if query.Pagination != nil && query.Pagination.PaginatedQuery != nil && *query.Pagination.PaginatedQuery != "" && queryToExecute == strings.Replace(*query.Pagination.PaginatedQuery, "offset_size", strconv.Itoa(0), 1) {
			log.Printf("ChatService -> ExecuteQuery -> query.Pagination.PaginatedQuery was executed but faced an error, will try to execute the original query")
			queryToExecute = query.Query
			result, queryErr = s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, queryToExecute, *query.QueryType, false, false)
		}
	}
	if queryErr != nil {
		log.Printf("ChatService -> ExecuteQuery -> queryErr: %+v", queryErr)
		if queryErr.Code == "FAILED_TO_START_TRANSACTION" || strings.Contains(queryErr.Message, "context deadline exceeded") || strings.Contains(queryErr.Message, "context canceled") {
			return nil, http.StatusRequestTimeout, fmt.Errorf("query execution timed out")
		}

		processCompleted := make(chan bool)
		go func() {
			log.Printf("ChatService -> ExecuteQuery -> Updating message")

			// Update query status in message
			if msg.Queries != nil {
				log.Printf("ChatService -> ExecuteQuery -> msg queries %v", *msg.Queries)
				for i := range *msg.Queries {
					// Convert ObjectID to hex string for comparison
					queryIDHex := query.ID.Hex()
					msgQueryIDHex := (*msg.Queries)[i].ID.Hex()

					if msgQueryIDHex == queryIDHex {
						(*msg.Queries)[i].IsRolledBack = false
						(*msg.Queries)[i].IsExecuted = true
						(*msg.Queries)[i].ExecutionTime = nil
						(*msg.Queries)[i].Error = &models.QueryError{
							Code:    queryErr.Code,
							Message: queryErr.Message,
							Details: queryErr.Details,
						}
						(*msg.Queries)[i].ActionAt = utils.ToStringPtr(time.Now().Format(time.RFC3339))
						break
					}
				}
			} else {
				log.Println("ChatService -> ExecuteQuery -> msg queries is null")
				return
			}

			// Add "Fix Error" action button to the Message & LLM content if there's an error
			if queryErr != nil {
				s.addFixErrorButton(msg)
			} else {
				s.removeFixErrorButton(msg)
			}

			if msg.ActionButtons != nil {
				log.Printf("ChatService -> ExecuteQuery -> queryError, msg.ActionButtons: %+v", *msg.ActionButtons)
			} else {
				log.Printf("ChatService -> ExecuteQuery -> queryError, msg.ActionButtons: nil")
			}

			// We want to wait for the message to be updated but not save it to DB before sending the response
			processCompleted <- true

			// Save updated message
			if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error updating message: %v", err)
			}

			// Update LLM message with query execution results
			llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
			if err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error finding LLM message: %v", err)
			} else if llmMsg != nil {
				log.Printf("ChatService -> ExecuteQuery -> llmMsg: %+v", llmMsg)

				content := llmMsg.Content
				if content == nil {
					content = make(map[string]interface{})
				}

				if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
					log.Printf("ChatService -> ExecuteQuery -> assistantResponse: %+v", assistantResponse)
					log.Printf("ChatService -> ExecuteQuery -> queries type: %T", assistantResponse["queries"])

					// Handle primitive.A (BSON array) type
					switch queriesVal := assistantResponse["queries"].(type) {
					case primitive.A:
						log.Printf("ChatService -> ExecuteQuery -> queries is primitive.A")
						// Convert primitive.A to []interface{}
						queries := make([]interface{}, len(queriesVal))
						for i, q := range queriesVal {
							log.Printf("ChatService -> ExecuteQuery -> q: %+v", q)
							if queryMap, ok := q.(map[string]interface{}); ok {
								// Compare hex strings of ObjectIDs
								if queryMap["query"] == query.Query && queryMap["queryType"] == *query.QueryType && queryMap["explanation"] == query.Description {
									queryMap["isRolledBack"] = false
									queryMap["executionTime"] = nil
									queryMap["error"] = map[string]interface{}{
										"code":    queryErr.Code,
										"message": queryErr.Message,
										"details": queryErr.Details,
									}
									queryMap["actionAt"] = utils.ToStringPtr(time.Now().Format(time.RFC3339))
								}
								queries[i] = queryMap
							} else {
								log.Printf("ChatService -> ExecuteQuery -> queryMap is not a map[string]interface{}")
								queries[i] = q
							}
							log.Printf("ChatService -> ExecuteQuery -> updated queries[%d]: %+v", i, queries[i])
						}
						assistantResponse["queries"] = queries

					case []interface{}:
						log.Printf("ChatService -> ExecuteQuery -> queries is []interface{}")
						for i, q := range queriesVal {
							if queryMap, ok := q.(map[string]interface{}); ok {
								if queryMap["query"] == query.Query && queryMap["queryType"] == *query.QueryType && queryMap["explanation"] == query.Description {
									queryMap["isRolledBack"] = false
									queryMap["executionTime"] = query.ExecutionTime
									queryMap["error"] = map[string]interface{}{
										"code":    queryErr.Code,
										"message": queryErr.Message,
										"details": queryErr.Details,
									}
									queriesVal[i] = queryMap
									queryMap["actionAt"] = utils.ToStringPtr(time.Now().Format(time.RFC3339))
								}
							} else {
								log.Printf("ChatService -> ExecuteQuery -> queryMap is not a map[string]interface{}")
								queriesVal[i] = q
							}

						}
						assistantResponse["queries"] = queriesVal
					}

					content["assistant_response"] = assistantResponse
				}

				llmMsg.Content = content
				if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
					log.Printf("ChatService -> ExecuteQuery -> Error updating LLM message: %v", err)
				}
			}
		}()

		<-processCompleted
		return &dtos.QueryExecutionResponse{
			ChatID:            chatID,
			MessageID:         msg.ID.Hex(),
			QueryID:           query.ID.Hex(),
			IsExecuted:        false,
			IsRolledBack:      false,
			ExecutionTime:     query.ExecutionTime,
			ExecutionResult:   nil,
			Error:             queryErr,
			TotalRecordsCount: nil,
			ActionButtons:     dtos.ToActionButtonDto(msg.ActionButtons),
			ActionAt:          query.ActionAt,
		}, http.StatusOK, nil
	}

	// Checking if the result record is a list with > 50 records, then cap it to 50 records.
	// Then we need to save capped 50 results in DB
	log.Printf("ChatService -> ExecuteQuery -> result: %+v", result)
	log.Printf("ChatService -> ExecuteQuery -> result.ResultJSON: %+v", result.ResultJSON)

	var formattedResultJSON interface{}
	var resultListFormatting []interface{} = []interface{}{}
	var resultMapFormatting map[string]interface{} = map[string]interface{}{}
	if err := json.Unmarshal([]byte(result.ResultJSON), &resultListFormatting); err != nil {
		log.Printf("ChatService -> ExecuteQuery -> Error unmarshalling result JSON: %v", err)
		if err := json.Unmarshal([]byte(result.ResultJSON), &resultMapFormatting); err != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error unmarshalling result JSON: %v", err)
			// Try to unmarshal as a map
			err = json.Unmarshal([]byte(result.ResultJSON), &resultMapFormatting)
			if err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error unmarshalling result JSON: %v", err)
			}
		}
	}

	log.Printf("ChatService -> ExecuteQuery -> resultListFormatting: %+v", resultListFormatting)
	log.Printf("ChatService -> ExecuteQuery -> resultMapFormatting: %+v", resultMapFormatting)
	if len(resultListFormatting) > 0 {
		log.Printf("ChatService -> ExecuteQuery -> resultListFormatting: %+v", resultListFormatting)
		formattedResultJSON = resultListFormatting
		if len(resultListFormatting) > 50 {
			log.Printf("ChatService -> ExecuteQuery -> resultListFormatting length > 50")
			formattedResultJSON = resultListFormatting[:50] // Cap the result to 50 records

			// Cap the result.ResultJSON to 50 records
			cappedResults, err := json.Marshal(resultListFormatting[:50])
			if err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error marshaling capped results: %v", err)
			} else {
				result.ResultJSON = string(cappedResults)
			}
		}
	} else if resultMapFormatting != nil && resultMapFormatting["results"] != nil && len(resultMapFormatting["results"].([]interface{})) > 0 {
		log.Printf("ChatService -> ExecuteQuery -> resultMapFormatting: %+v", resultMapFormatting)
		if len(resultMapFormatting["results"].([]interface{})) > 50 {
			formattedResultJSON = map[string]interface{}{
				"results": resultMapFormatting["results"].([]interface{})[:50],
			}
			cappedResults := map[string]interface{}{
				"results": resultMapFormatting["results"].([]interface{})[:50],
			}
			cappedResultsJSON, err := json.Marshal(cappedResults)
			if err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error marshaling capped results: %v", err)
			} else {
				result.ResultJSON = string(cappedResultsJSON)
			}
		} else {
			formattedResultJSON = map[string]interface{}{
				"results": resultMapFormatting["results"].([]interface{}),
			}
		}
	} else {
		formattedResultJSON = resultMapFormatting
	}

	log.Printf("ChatService -> ExecuteQuery -> totalRecordsCount: %+v", totalRecordsCount)
	log.Printf("ChatService -> ExecuteQuery -> formattedResultJSON: %+v", formattedResultJSON)

	query.IsExecuted = true
	query.IsRolledBack = false
	query.ExecutionTime = &result.ExecutionTime
	query.ExecutionResult = &result.ResultJSON
	query.ActionAt = utils.ToStringPtr(time.Now().Format(time.RFC3339))
	if totalRecordsCount != nil {
		if query.Pagination == nil {
			query.Pagination = &models.Pagination{}
		}
		query.Pagination.TotalRecordsCount = totalRecordsCount
	}
	if result.Error != nil {
		query.Error = &models.QueryError{
			Code:    result.Error.Code,
			Message: result.Error.Message,
			Details: result.Error.Details,
		}
	} else {
		query.Error = nil
	}

	processCompleted := make(chan bool)
	go func() {
		// Update query status in message
		if msg.Queries != nil {
			for i := range *msg.Queries {
				if (*msg.Queries)[i].ID == query.ID {
					(*msg.Queries)[i].IsRolledBack = false
					(*msg.Queries)[i].IsExecuted = true
					(*msg.Queries)[i].ExecutionTime = &result.ExecutionTime
					(*msg.Queries)[i].ActionAt = utils.ToStringPtr(time.Now().Format(time.RFC3339))
					if totalRecordsCount != nil {
						if (*msg.Queries)[i].Pagination == nil {
							(*msg.Queries)[i].Pagination = &models.Pagination{}
						}
						(*msg.Queries)[i].Pagination.TotalRecordsCount = totalRecordsCount
					}
					log.Printf("ChatService -> ExecuteQuery -> result.ResultJSON: %v", result.ResultJSON)
					log.Printf("ChatService -> ExecuteQuery -> ExecutionResult before update: %v", (*msg.Queries)[i].ExecutionResult)
					(*msg.Queries)[i].ExecutionResult = &result.ResultJSON
					log.Printf("ChatService -> ExecuteQuery -> ExecutionResult after update: %v", (*msg.Queries)[i].ExecutionResult)
					if result.Error != nil {
						(*msg.Queries)[i].Error = &models.QueryError{
							Code:    result.Error.Code,
							Message: result.Error.Message,
							Details: result.Error.Details,
						}
					} else {
						(*msg.Queries)[i].Error = nil
					}
					break
				}
			}
		}

		log.Printf("ChatService -> ExecuteQuery -> Updating message %v", msg)
		if msg.Queries != nil {
			for _, query := range *msg.Queries {
				log.Printf("ChatService -> ExecuteQuery -> updated query: %v", query)
			}
		}
		// Add "Fix Error" action button to the Message & LLM content if there's an error
		if result.Error != nil {
			s.addFixErrorButton(msg)
		} else {
			s.removeFixErrorButton(msg)
		}
		// Save updated message
		if msg.ActionButtons != nil {
			log.Printf("ChatService -> ExecuteQuery -> msg.ActionButtons: %+v", *msg.ActionButtons)
		} else {
			log.Printf("ChatService -> ExecuteQuery -> msg.ActionButtons: nil")
		}

		// We want to wait for the message to be updated but not save it to DB before sending the response
		processCompleted <- true

		if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error updating message: %v", err)
		}

		// Update LLM message with query execution results
		llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
		if err != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error finding LLM message: %v", err)
		} else if llmMsg != nil {
			// Get the existing content
			content := llmMsg.Content
			if content == nil {
				content = make(map[string]interface{})
			}

			if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
				log.Printf("ChatService -> ExecuteQuery -> assistantResponse: %+v", assistantResponse)
				log.Printf("ChatService -> ExecuteQuery -> queries type: %T", assistantResponse["queries"])

				// Handle primitive.A (BSON array) type
				switch queriesVal := assistantResponse["queries"].(type) {
				case primitive.A:
					log.Printf("ChatService -> ExecuteQuery -> queries is primitive.A")
					// Convert primitive.A to []interface{}
					queries := make([]interface{}, len(queriesVal))
					for i, q := range queriesVal {
						if queryMap, ok := q.(map[string]interface{}); ok {
							// Compare hex strings of ObjectIDs
							if queryMap["query"] == query.Query && queryMap["queryType"] == *query.QueryType && queryMap["explanation"] == query.Description {
								queryMap["isExecuted"] = true
								queryMap["isRolledBack"] = false
								queryMap["executionTime"] = result.ExecutionTime
								queryMap["actionAt"] = utils.ToStringPtr(time.Now().Format(time.RFC3339))
								// If share data with AI is true, then we need to share the result with AI
								if chat.Settings.ShareDataWithAI {
									queryMap["executionResult"] = map[string]interface{}{
										"result": result.ResultJSON,
									}
								} else {
									queryMap["executionResult"] = map[string]interface{}{
										"result": "Query executed successfully",
									}
								}
								if result.Error != nil {
									queryMap["error"] = map[string]interface{}{
										"code":    result.Error.Code,
										"message": result.Error.Message,
										"details": result.Error.Details,
									}
								} else {
									queryMap["error"] = nil
								}
							}
							queries[i] = queryMap
						} else {
							queries[i] = q
						}
					}
					assistantResponse["queries"] = queries

				case []interface{}:
					log.Printf("ChatService -> ExecuteQuery -> queries is []interface{}")
					for i, q := range queriesVal {
						if queryMap, ok := q.(map[string]interface{}); ok {
							if queryMap["query"] == query.Query && queryMap["queryType"] == *query.QueryType && queryMap["explanation"] == query.Description {
								queryMap["isExecuted"] = true
								queryMap["isRolledBack"] = false
								queryMap["executionTime"] = result.ExecutionTime
								queryMap["actionAt"] = utils.ToStringPtr(time.Now().Format(time.RFC3339))
								// If share data with AI is true, then we need to share the result with AI
								if chat.Settings.ShareDataWithAI {
									queryMap["executionResult"] = map[string]interface{}{
										"result": result.ResultJSON,
									}
								} else {
									queryMap["executionResult"] = map[string]interface{}{
										"result": "Query executed successfully",
									}
								}
								if result.Error != nil {
									queryMap["error"] = map[string]interface{}{
										"code":    result.Error.Code,
										"message": result.Error.Message,
										"details": result.Error.Details,
									}
								} else {
									queryMap["error"] = nil
								}
								queriesVal[i] = queryMap
							}
						}
					}
					assistantResponse["queries"] = queriesVal
				}

				content["assistant_response"] = assistantResponse
			}

			// Save updated LLM message
			llmMsg.Content = content
			if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error updating LLM message: %v", err)
			}
		}
	}()

	<-processCompleted
	return &dtos.QueryExecutionResponse{
		ChatID:            chatID,
		MessageID:         msg.ID.Hex(),
		QueryID:           query.ID.Hex(),
		IsExecuted:        query.IsExecuted,
		IsRolledBack:      query.IsRolledBack,
		ExecutionTime:     query.ExecutionTime,
		ExecutionResult:   formattedResultJSON,
		Error:             result.Error,
		TotalRecordsCount: totalRecordsCount,
		ActionButtons:     dtos.ToActionButtonDto(msg.ActionButtons),
		ActionAt:          query.ActionAt,
	}, http.StatusOK, nil
}

func (s *chatService) RollbackQuery(ctx context.Context, userID, chatID string, req *dtos.RollbackQueryRequest) (*dtos.QueryExecutionResponse, uint32, error) {
	// Verify message and query ownership
	chat, msg, query, err := s.verifyQueryOwnership(userID, chatID, req.MessageID, req.QueryID)
	if err != nil {
		return nil, http.StatusForbidden, err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, http.StatusRequestTimeout, fmt.Errorf("query rollback cancelled or timed out")
	default:
		log.Printf("ChatService -> RollbackQuery -> msg: %+v", msg)
		log.Printf("ChatService -> RollbackQuery -> query: %+v", query)
	}

	// Validate query state
	if !query.IsExecuted {
		return nil, http.StatusBadRequest, fmt.Errorf("cannot rollback a query that hasn't been executed")
	}
	if query.IsRolledBack {
		return nil, http.StatusBadRequest, fmt.Errorf("query already rolled back")
	}

	if !query.CanRollback {
		return nil, http.StatusBadRequest, fmt.Errorf("query cannot be rolled back")
	}
	// Check if we need to generate rollback query
	if query.RollbackQuery == nil || *query.RollbackQuery == "" {
		// First execute the dependent query to get context
		if query.RollbackDependentQuery == nil {
			return nil, http.StatusBadRequest, fmt.Errorf("rollback dependent query is required but not provided")
		}

		log.Printf("ChatService -> RollbackQuery -> Executing dependent query: %s", *query.RollbackDependentQuery)

		// Check connection status and connect if needed
		if !s.dbManager.IsConnected(chatID) {
			log.Printf("ChatService -> RollbackQuery -> Database not connected, initiating connection")
			status, err := s.ConnectDB(ctx, userID, chatID, req.StreamID)
			if err != nil {
				return nil, status, err
			}
			time.Sleep(1 * time.Second)
		}

		// Execute dependent query
		dependentResult, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, *query.RollbackDependentQuery, *query.QueryType, false, false)
		if queryErr != nil {
			log.Printf("ChatService -> RollbackQuery -> queryErr: %+v", queryErr)
			if queryErr.Code == "FAILED_TO_START_TRANSACTION" || strings.Contains(queryErr.Message, "context deadline exceeded") || strings.Contains(queryErr.Message, "context canceled") {
				return nil, http.StatusRequestTimeout, fmt.Errorf("query execution timed out")
			}
			// Update query status in message
			go func() {
				if msg.Queries != nil {
					for i := range *msg.Queries {
						if (*msg.Queries)[i].ID == query.ID {
							(*msg.Queries)[i].IsExecuted = true
							(*msg.Queries)[i].IsRolledBack = false
							(*msg.Queries)[i].Error = &models.QueryError{
								Code:    queryErr.Code,
								Message: queryErr.Message,
								Details: queryErr.Details,
							}
						}
					}
				}
				if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
					log.Printf("ChatService -> RollbackQuery -> Error updating message: %v", err)
				}

				// Update LLM message with query execution results
				llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
				if err != nil {
					log.Printf("ChatService -> RollbackQuery -> Error finding LLM message: %v", err)
				} else if llmMsg != nil {
					content := llmMsg.Content
					if content == nil {
						content = make(map[string]interface{})
					}
					if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
						if queries, ok := assistantResponse["queries"].([]interface{}); ok {
							for _, q := range queries {
								if queryMap, ok := q.(map[string]interface{}); ok {
									if queryMap["query"] == query.Query && queryMap["queryType"] == *query.QueryType && queryMap["explanation"] == query.Description {
										queryMap["isExecuted"] = true
										queryMap["isRolledBack"] = false
										queryMap["error"] = &models.QueryError{
											Code:    queryErr.Code,
											Message: queryErr.Message,
											Details: queryErr.Details,
										}
									}
								}
							}
						}
					}

					llmMsg.Content = content
					if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
						log.Printf("ChatService -> RollbackQuery -> Error updating LLM message: %v", err)
					}
				}
			}()

			// Send event about dependent query failure
			s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
				Event: "rollback-query-failed",
				Data: map[string]interface{}{
					"chat_id":    chatID,
					"message_id": msg.ID.Hex(),
					"query_id":   query.ID.Hex(),
					"error":      queryErr,
				},
			})
			// Add "Fix Error" action button to the Message & LLM content if there's an error
			if queryErr != nil {
				s.addFixErrorButton(msg)
			} else {
				s.removeFixErrorButton(msg)
			}

			return &dtos.QueryExecutionResponse{
				ChatID:            chatID,
				MessageID:         msg.ID.Hex(),
				QueryID:           query.ID.Hex(),
				IsExecuted:        true,
				IsRolledBack:      false,
				ExecutionTime:     query.ExecutionTime,
				ExecutionResult:   nil,
				Error:             queryErr,
				TotalRecordsCount: nil,
				ActionButtons:     dtos.ToActionButtonDto(msg.ActionButtons),
			}, http.StatusOK, nil
		}

		// Get LLM context from previous messages
		llmMsgs, err := s.llmRepo.GetByChatID(msg.ChatID)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to get LLM context: %v", err)
		}

		// Build context for LLM
		var contextBuilder strings.Builder
		contextBuilder.WriteString("Previous messages:\n")
		for _, llmMsg := range llmMsgs {
			if content, ok := llmMsg.Content["assistant_response"].(map[string]interface{}); ok {
				contextBuilder.WriteString(fmt.Sprintf("Assistant: %v\n", content["content"]))
			}
			if content, ok := llmMsg.Content["user_message"].(string); ok {
				contextBuilder.WriteString(fmt.Sprintf("User: %s\n", content))
			}
		}
		contextBuilder.WriteString(fmt.Sprintf("\nQuery id: %s\n", query.ID.Hex())) // This will help LLM to understand the context of the query to be rolled back
		contextBuilder.WriteString(fmt.Sprintf("\nOriginal query: %s\n", query.Query))
		contextBuilder.WriteString(fmt.Sprintf("Dependent query result: %s\n", dependentResult.ResultJSON))
		contextBuilder.WriteString("\nPlease generate a rollback query that will undo the effects of the original query.")

		// Get connection info for db type
		conn, exists := s.dbManager.GetConnectionInfo(chatID)
		if !exists {
			return nil, http.StatusBadRequest, fmt.Errorf("no database connection found")
		}

		// Convert LLM messages to expected format
		llmMessages := make([]*models.LLMMessage, len(llmMsgs))
		// Use copy to avoid modifying original messages
		copy(llmMessages, llmMsgs)

		// Get rollback query from LLM
		llmResponse, err := s.llmClient.GenerateResponse(
			ctx,
			llmMessages,      // Pass the LLM messages array
			conn.Config.Type, // Pass the database type
		)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to generate rollback query: %v", err)
		}

		// Parse LLM response to get rollback query
		var rollbackQuery string
		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(llmResponse), &jsonResponse); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to parse LLM response: %v", err)
		}

		if msg.Queries != nil {
			for i := range *msg.Queries {
				if (*msg.Queries)[i].ID == query.ID {
					(*msg.Queries)[i].IsExecuted = true
					(*msg.Queries)[i].IsRolledBack = false
					(*msg.Queries)[i].RollbackQuery = &rollbackQuery
				}
			}
		}
		if queryErr != nil {
			s.addFixErrorButton(msg)
		} else {
			s.removeFixErrorButton(msg)
		}
		if msg.ActionButtons != nil {
			log.Printf("ChatService -> RollbackQuery -> msg.ActionButtons: %+v", *msg.ActionButtons)
		} else {
			log.Printf("ChatService -> RollbackQuery -> msg.ActionButtons: nil")
		}
		if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
			log.Printf("ChatService -> RollbackQuery -> Error updating message: %v", err)
		}

		if assistantResponse, ok := jsonResponse["assistant_response"].(map[string]interface{}); ok {
			switch v := assistantResponse["queries"].(type) {
			case primitive.A:
				for i, q := range v {
					if qMap, ok := q.(map[string]interface{}); ok {
						if strings.Replace(qMap["query"].(string), "EDITED by user: ", "", 1) == query.Query && qMap["queryType"] == *query.QueryType && qMap["explanation"] == query.Description {
							rollbackQuery = qMap["rollback_query"].(string)
							// Update the query map with rollback info
							qMap["rollback_query"] = rollbackQuery
							v[i] = qMap
						}
					}
				}
				// Update the queries in assistant response
				assistantResponse["queries"] = v
			case []interface{}:
				for i, q := range v {
					if qMap, ok := q.(map[string]interface{}); ok {
						if strings.Replace(qMap["query"].(string), "EDITED by user: ", "", 1) == query.Query && qMap["queryType"] == *query.QueryType && qMap["explanation"] == query.Description {
							rollbackQuery = qMap["rollback_query"].(string)
							// Update the query map with rollback info
							qMap["rollback_query"] = rollbackQuery
							v[i] = qMap
						}
					}
				}
				// Update the queries in assistant response
				assistantResponse["queries"] = v
			}
		}

		if rollbackQuery == "" {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to generate valid rollback query")
		}

		// Update query with rollback query
		query.RollbackQuery = &rollbackQuery

		// Update query status in message
		if msg.Queries != nil {
			for i := range *msg.Queries {
				if (*msg.Queries)[i].ID == query.ID {
					(*msg.Queries)[i].RollbackQuery = &rollbackQuery
					(*msg.Queries)[i].IsRolledBack = false
					(*msg.Queries)[i].IsExecuted = true
				}
			}
		}
		// Update message in DB
		if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to update message with rollback query: %v", err)
		}

		// Update existing LLM message
		llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
		if err != nil {
			log.Printf("ChatService -> RollbackQuery -> Error finding LLM message: %v", err)
		} else if llmMsg != nil {
			content := llmMsg.Content
			if content == nil {
				content = make(map[string]interface{})
			}

			if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
				// Update the assistant response with new queries
				switch v := assistantResponse["queries"].(type) {
				case primitive.A:
					for i, q := range v {
						if qMap, ok := q.(map[string]interface{}); ok {
							if strings.Replace(qMap["query"].(string), "EDITED by user: ", "", 1) == query.Query && qMap["queryType"] == *query.QueryType && qMap["explanation"] == query.Description {
								qMap["isRolledBack"] = true
								qMap["rollback_query"] = rollbackQuery
								v[i] = qMap
							}
						}
					}
				case []interface{}:
					for i, q := range v {
						if qMap, ok := q.(map[string]interface{}); ok {
							if strings.Replace(qMap["query"].(string), "EDITED by user: ", "", 1) == query.Query && qMap["queryType"] == *query.QueryType && qMap["explanation"] == query.Description {
								qMap["rollback_query"] = rollbackQuery
								v[i] = qMap
							}
						}
					}
					assistantResponse["queries"] = v
				}

				content["assistant_response"] = assistantResponse
			}

			llmMsg.Content = content
			if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
				log.Printf("ChatService -> RollbackQuery -> Error updating LLM message: %v", err)
			}
		}
	}

	// Now execute the rollback query
	if query.RollbackQuery == nil || *query.RollbackQuery == "" {
		// Send event about rollback query failure
		s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
			Event: "rollback-query-failed",
			Data: map[string]interface{}{
				"chat_id":    chatID,
				"query_id":   query.ID.Hex(),
				"message_id": msg.ID.Hex(),
				"error":      "No rollback query available",
			},
		})
		return nil, http.StatusBadRequest, fmt.Errorf("no rollback query available")
	}

	// Check connection status and connect if needed
	if !s.dbManager.IsConnected(chatID) {
		log.Printf("ChatService -> RollbackQuery -> Database not connected, initiating connection")
		status, err := s.ConnectDB(ctx, userID, chatID, req.StreamID)
		if err != nil {
			return nil, status, err
		}
		time.Sleep(1 * time.Second)
	}

	// Execute rollback query
	result, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, *query.RollbackQuery, *query.QueryType, true, false)
	if queryErr != nil {
		log.Printf("ChatService -> RollbackQuery -> queryErr: %+v", queryErr)
		if queryErr.Code == "FAILED_TO_START_TRANSACTION" || strings.Contains(queryErr.Message, "context deadline exceeded") || strings.Contains(queryErr.Message, "context canceled") {
			return nil, http.StatusRequestTimeout, fmt.Errorf("query execution timed out")
		}
		// Update query status in message
		go func() {
			if msg.Queries != nil {
				for i := range *msg.Queries {
					if (*msg.Queries)[i].ID == query.ID {
						(*msg.Queries)[i].IsExecuted = true
						(*msg.Queries)[i].IsRolledBack = false
					}
				}

				if msg.ActionButtons != nil {
					log.Printf("ChatService -> RollbackQuery -> msg.ActionButtons: %+v", *msg.ActionButtons)
				} else {
					log.Printf("ChatService -> RollbackQuery -> msg.ActionButtons: nil")
				}

				if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
					log.Printf("ChatService -> RollbackQuery -> Error updating message: %v", err)
				}
				// Update LLM message with query execution results
				llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
				if err != nil {
					log.Printf("ChatService -> RollbackQuery -> Error finding LLM message: %v", err)
				} else if llmMsg != nil {
					content := llmMsg.Content
					if content == nil {
						content = make(map[string]interface{})
					}
					if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
						switch v := assistantResponse["queries"].(type) {
						case primitive.A:
							for _, q := range v {
								if qMap, ok := q.(map[string]interface{}); ok {
									if strings.Replace(qMap["query"].(string), "EDITED by user: ", "", 1) == query.Query && qMap["queryType"] == *query.QueryType && qMap["explanation"] == query.Description {
										qMap["isExecuted"] = true
										qMap["isRolledBack"] = false
									}
								}
							}
							assistantResponse["queries"] = v
						case []interface{}:
							for _, q := range v {
								if qMap, ok := q.(map[string]interface{}); ok {
									if strings.Replace(qMap["query"].(string), "EDITED by user: ", "", 1) == query.Query && qMap["queryType"] == *query.QueryType && qMap["explanation"] == query.Description {
										qMap["isExecuted"] = true
										qMap["isRolledBack"] = false
									}
								}
							}
							assistantResponse["queries"] = v
						}
					}
					llmMsg.Content = content
					if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
						log.Printf("ChatService -> RollbackQuery -> Error updating LLM message: %v", err)
					}
				}
			}
		}()

		// Send event about rollback query failure
		s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
			Event: "rollback-query-failed",
			Data: map[string]interface{}{
				"chat_id":    chatID,
				"query_id":   query.ID.Hex(),
				"message_id": msg.ID.Hex(),
				"error":      queryErr,
			},
		})

		tempMessage := *msg
		// Add "Fix Rollback Error" action button temporarily to response so that user can fix the error
		s.addFixRollbackErrorButton(&tempMessage)

		return &dtos.QueryExecutionResponse{
			ChatID:            chatID,
			MessageID:         msg.ID.Hex(),
			QueryID:           query.ID.Hex(),
			IsExecuted:        true,
			IsRolledBack:      false,
			ExecutionTime:     query.ExecutionTime,
			ExecutionResult:   nil,
			Error:             queryErr,
			TotalRecordsCount: nil,
			ActionButtons:     dtos.ToActionButtonDto(tempMessage.ActionButtons),
		}, http.StatusOK, nil
	}

	log.Printf("ChatService -> RollbackQuery -> result: %+v", result)

	// Update query status
	// We're using same execution time for the rollback as the original query
	query.IsRolledBack = true
	query.ExecutionTime = &result.ExecutionTime
	query.ActionAt = utils.ToStringPtr(time.Now().Format(time.RFC3339))
	if result.Error != nil {
		query.Error = &models.QueryError{
			Code:    result.Error.Code,
			Message: result.Error.Message,
			Details: result.Error.Details,
		}
	} else {
		query.Error = nil
	}

	// Update query status in message
	if msg.Queries != nil {
		for i := range *msg.Queries {
			if (*msg.Queries)[i].ID == query.ID {
				(*msg.Queries)[i].IsRolledBack = true
				(*msg.Queries)[i].IsExecuted = true
				(*msg.Queries)[i].ExecutionTime = &result.ExecutionTime
				(*msg.Queries)[i].ExecutionResult = &result.ResultJSON
				(*msg.Queries)[i].ActionAt = utils.ToStringPtr(time.Now().Format(time.RFC3339))
				if result.Error != nil {
					(*msg.Queries)[i].Error = &models.QueryError{
						Code:    result.Error.Code,
						Message: result.Error.Message,
						Details: result.Error.Details,
					}
				} else {
					(*msg.Queries)[i].Error = nil
				}
			}
		}
	}

	if msg.ActionButtons != nil {
		log.Printf("ChatService -> RollbackQuery -> msg.ActionButtons: %+v", *msg.ActionButtons)
	} else {
		log.Printf("ChatService -> RollbackQuery -> msg.ActionButtons: nil")
	}
	// Save updated message
	if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update message with rollback results: %v", err)
	}

	// Update LLM message with rollback results
	llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
	if err != nil {
		log.Printf("ChatService -> RollbackQuery -> Error finding LLM message: %v", err)
	} else if llmMsg != nil {
		content := llmMsg.Content
		if content == nil {
			content = make(map[string]interface{})
		}

		if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
			log.Printf("ChatService -> RollbackQuery -> assistantResponse: %+v", assistantResponse)
			log.Printf("ChatService -> RollbackQuery -> queries type: %T", assistantResponse["queries"])

			// Handle primitive.A (BSON array) type
			switch queriesVal := assistantResponse["queries"].(type) {
			case primitive.A:
				log.Printf("ChatService -> RollbackQuery -> queries is primitive.A")
				// Convert primitive.A to []interface{}
				queries := make([]interface{}, len(queriesVal))
				for i, q := range queriesVal {
					if queryMap, ok := q.(map[string]interface{}); ok {
						// Compare hex strings of ObjectIDs
						if queryMap["query"] == query.Query && queryMap["queryType"] == *query.QueryType && queryMap["explanation"] == query.Description {
							queryMap["isExecuted"] = true
							queryMap["isRolledBack"] = true
							queryMap["executionTime"] = result.ExecutionTime
							queryMap["actionAt"] = utils.ToStringPtr(time.Now().Format(time.RFC3339))
							// If share data with AI is true, then we need to share the result with AI
							if chat.Settings.ShareDataWithAI {
								queryMap["executionResult"] = map[string]interface{}{
									"result": result.ResultJSON,
								}
							} else {
								queryMap["executionResult"] = map[string]interface{}{
									"result": "Rolled back successfully",
								}
							}
							if result.Error != nil {
								queryMap["error"] = map[string]interface{}{
									"code":    result.Error.Code,
									"message": result.Error.Message,
									"details": result.Error.Details,
								}
							} else {
								queryMap["error"] = nil
							}
						}
						queries[i] = queryMap
					} else {
						queries[i] = q
					}
				}
				assistantResponse["queries"] = queries

			case []interface{}:
				log.Printf("ChatService -> RollbackQuery -> queries is []interface{}")
				for i, q := range queriesVal {
					if queryMap, ok := q.(map[string]interface{}); ok {
						if queryMap["query"] == query.Query && queryMap["queryType"] == *query.QueryType && queryMap["explanation"] == query.Description {
							queryMap["isExecuted"] = true
							queryMap["isRolledBack"] = true
							queryMap["executionTime"] = result.ExecutionTime
							queryMap["actionAt"] = utils.ToStringPtr(time.Now().Format(time.RFC3339))
							// If share data with AI is true, then we need to share the result with AI
							if chat.Settings.ShareDataWithAI {
								queryMap["executionResult"] = map[string]interface{}{
									"result": result.ResultJSON,
								}
							} else {
								queryMap["executionResult"] = map[string]interface{}{
									"result": "Rolled back successfully",
								}
							}
							if result.Error != nil {
								queryMap["error"] = map[string]interface{}{
									"code":    result.Error.Code,
									"message": result.Error.Message,
									"details": result.Error.Details,
								}
							} else {
								queryMap["error"] = nil
							}
							queriesVal[i] = queryMap
						}
					}
				}
				assistantResponse["queries"] = queriesVal
			}

			content["assistant_response"] = assistantResponse
		}

		// Save updated LLM message
		llmMsg.Content = content
		if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error updating LLM message: %v", err)
		}
	}

	// Send stream event
	s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
		Event: "rollback-executed",
		Data: map[string]interface{}{
			"chat_id":          chatID,
			"message_id":       msg.ID.Hex(),
			"query_id":         query.ID.Hex(),
			"is_executed":      query.IsExecuted,
			"is_rolled_back":   query.IsRolledBack,
			"execution_time":   query.ExecutionTime,
			"execution_result": result.Result,
			"error":            query.Error,
			"action_buttons":   dtos.ToActionButtonDto(msg.ActionButtons),
			"action_at":        query.ActionAt,
		},
	})

	return &dtos.QueryExecutionResponse{
		ChatID:          chatID,
		MessageID:       msg.ID.Hex(),
		QueryID:         query.ID.Hex(),
		IsExecuted:      query.IsExecuted,
		IsRolledBack:    query.IsRolledBack,
		ExecutionTime:   query.ExecutionTime,
		ExecutionResult: result.Result,
		Error:           result.Error,
		ActionButtons:   dtos.ToActionButtonDto(msg.ActionButtons),
		ActionAt:        query.ActionAt,
	}, http.StatusOK, nil
}

// Cancels the ongoing query & rollback execution for the given streamID
func (s *chatService) CancelQueryExecution(userID, chatID, messageID, queryID, streamID string) {
	log.Printf("ChatService -> CancelQueryExecution -> Cancelling query for streamID: %s", streamID)

	// 1. Cancel the query execution in dbManager
	s.dbManager.CancelQueryExecution(streamID)

	// 2. Send cancellation event to client
	s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
		Event: "query-cancelled",
		Data: map[string]interface{}{
			"chat_id":    chatID,
			"message_id": messageID,
			"query_id":   queryID,
			"stream_id":  streamID,
			"error": map[string]string{
				"code":    "QUERY_EXECUTION_CANCELLED",
				"message": "Query execution was cancelled by user",
			},
		},
	})

	log.Printf("ChatService -> CancelQueryExecution -> Query cancelled successfully for streamID: %s", streamID)
}

// ProcessLLMResponseAndRunQuery processes the LLM response & runs the query automatically, updates SSE stream
func (s *chatService) processLLMResponseAndRunQuery(ctx context.Context, userID, chatID string, messageID, streamID string) error {
	msgCtx, cancel := context.WithCancel(context.Background())

	log.Printf("ProcessLLMResponseAndRunQuery -> userID: %s, chatID: %s, streamID: %s", userID, chatID, streamID)

	s.processesMu.Lock()
	s.activeProcesses[streamID] = cancel
	s.processesMu.Unlock()

	// Use the parent context (ctx) for SSE connection
	// Use llmCtx for LLM processing
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("ProcessLLMResponseAndRunQuery -> recovered from panic: %v", r)
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-error",
					Data:  "Error: Failed to complete the request, seems like the database connection issue, try reconnecting the database.",
				})
			}
			log.Printf("ProcessLLMResponseAndRunQuery -> activeProcesses: %v", s.activeProcesses)
			s.processesMu.Lock()
			delete(s.activeProcesses, streamID)
			s.processesMu.Unlock()
		}()

		msgResp, err := s.processLLMResponse(msgCtx, userID, chatID, messageID, streamID, true, true)
		if err != nil {
			log.Printf("Error processing LLM response: %v", err)
			return
		}
		log.Printf("ProcessLLMResponseAndRunQuery -> msgResp: %v", msgResp)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		select {
		case <-ctx.Done():
			log.Printf("Query execution timed out")
			return
		default:
			log.Printf("ProcessLLMResponseAndRunQuery -> msgResp.Queries: %v", msgResp.Queries)
			if msgResp.Queries != nil {
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-step",
					Data:  "Executing the needful query now.",
				})
				tempQueries := make([]dtos.Query, len(*msgResp.Queries))
				for i, query := range *msgResp.Queries {
					if query.Query != "" && !query.IsCritical {
						executionResult, _, queryErr := s.ExecuteQuery(ctx, userID, chatID, &dtos.ExecuteQueryRequest{
							MessageID: msgResp.ID,
							QueryID:   query.ID,
							StreamID:  streamID,
						})
						if queryErr != nil {
							log.Printf("Error executing query: %v", queryErr)
							// Send existing msgResp so far
							s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
								Event: "ai-response",
								Data:  msgResp,
							})
							return
						}
						log.Printf("ProcessLLMResponseAndRunQuery -> Query executed successfully: %v", executionResult)

						query.IsExecuted = true
						query.ExecutionTime = executionResult.ExecutionTime
						query.ActionAt = executionResult.ActionAt
						// Handle different result types (MongoDB returns array, SQL databases return map)
						switch resultType := executionResult.ExecutionResult.(type) {
						case map[string]interface{}:
							// For SQL databases (PostgreSQL, MySQL, etc.)
							query.ExecutionResult = resultType
						case []interface{}:
							// For MongoDB which returns array results
							query.ExecutionResult = map[string]interface{}{
								"results": resultType,
							}
						default:
							// For any other type, wrap it in a map
							query.ExecutionResult = map[string]interface{}{
								"result": executionResult.ExecutionResult,
							}
						}

						if executionResult.ActionButtons != nil {
							msgResp.ActionButtons = executionResult.ActionButtons
						} else {
							msgResp.ActionButtons = nil
						}
						query.Error = executionResult.Error
						if query.Pagination != nil && executionResult.TotalRecordsCount != nil {
							query.Pagination.TotalRecordsCount = *executionResult.TotalRecordsCount
						}
					}
					tempQueries[i] = query
				}

				msgResp.Queries = &tempQueries
				log.Printf("ProcessLLMResponseAndRunQuery -> Queries updated in LLM response: %v", msgResp.Queries)
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response",
					Data:  msgResp,
				})
				return
			} else {
				log.Printf("No queries found in LLM response, returning ai response")
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response",
					Data:  msgResp,
				})
				return
			}
		}
	}()
	return nil
}

// ProcessMessage processes the message, updates SSE stream only if allowSSEUpdates is true, allowSSEUpdates is used to send SSE updates to the client except the final ai-response event
func (s *chatService) processMessage(_ context.Context, userID, chatID, messageID, streamID string) error {
	// Create a new context specifically for LLM processing
	// Use context.Background() to avoid cancellation of the parent context
	msgCtx, cancel := context.WithCancel(context.Background())

	log.Printf("ProcessMessage -> userID: %s, chatID: %s, streamID: %s", userID, chatID, streamID)

	s.processesMu.Lock()
	s.activeProcesses[streamID] = cancel
	s.processesMu.Unlock()

	// Use the parent context (ctx) for SSE connection
	// Use llmCtx for LLM processing
	go func() {
		defer func() {
			s.processesMu.Lock()
			delete(s.activeProcesses, streamID)
			s.processesMu.Unlock()
		}()

		if _, err := s.processLLMResponse(msgCtx, userID, chatID, messageID, streamID, false, true); err != nil {
			log.Printf("Error processing message: %v", err)
			// Use parent context for sending stream events
			select {
			case <-msgCtx.Done():
				return
			default:
				go func() {
					// Get user and chat IDs
					userObjID, cErr := primitive.ObjectIDFromHex(userID)
					if cErr != nil {
						log.Printf("Error processing message: %v", cErr)
						return
					}

					chatObjID, cErr := primitive.ObjectIDFromHex(chatID)
					if cErr != nil {
						log.Printf("Error processing message: %v", err)
						return
					}

					// Create a new error message
					errorMsg := &models.Message{
						Base:    models.NewBase(),
						UserID:  userObjID,
						ChatID:  chatObjID,
						Queries: nil,
						Content: "Error: " + err.Error(),
						Type:    string(constants.MessageTypeAssistant),
					}

					if err := s.chatRepo.CreateMessage(errorMsg); err != nil {
						log.Printf("Error processing message: %v", err)
						return
					}
				}()

				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-error",
					Data:  "Error: " + err.Error(),
				})
			}
		}
	}()

	return nil
}

// RefreshSchema refreshes the schema of the chat & stores the latest schema in the database
func (s *chatService) RefreshSchema(ctx context.Context, userID, chatID string, sync bool) (uint32, error) {
	log.Printf("ChatService -> RefreshSchema -> Starting for chatID: %s", chatID)

	// Increase the timeout for the initial context to 60 minutes
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		return http.StatusOK, nil
	default:
		// Check if connection exists
		_, exists := s.dbManager.GetConnectionInfo(chatID)
		if !exists {
			log.Printf("ChatService -> RefreshSchema -> Connection not found for chatID: %s", chatID)
			return http.StatusNotFound, fmt.Errorf("connection not found")
		}

		// Get chat to get selected collections
		chatObjID, err := primitive.ObjectIDFromHex(chatID)
		if err != nil {
			log.Printf("ChatService -> RefreshSchema -> Error getting chatID: %v", err)
			return http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
		}

		chat, err := s.chatRepo.FindByID(chatObjID)
		if err != nil {
			log.Printf("ChatService -> RefreshSchema -> Error finding chat: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
		}

		if chat == nil {
			log.Printf("ChatService -> RefreshSchema -> Chat not found for chatID: %s", chatID)
			return http.StatusNotFound, fmt.Errorf("chat not found")
		}

		// Convert the selectedCollections string to a slice
		var selectedCollectionsSlice []string
		if chat.SelectedCollections != "ALL" && chat.SelectedCollections != "" {
			selectedCollectionsSlice = strings.Split(chat.SelectedCollections, ",")
		}
		log.Printf("ChatService -> RefreshSchema -> Selected collections: %v", selectedCollectionsSlice)

		dataChan := make(chan error, 1)
		go func() {
			// Create a new context with a longer timeout specifically for the schema refresh operation
			// Increase to 90 minutes to handle large schemas or slow database responses
			schemaCtx, schemaCancel := context.WithTimeout(context.Background(), 90*time.Minute)
			defer schemaCancel()

			userObjID, err := primitive.ObjectIDFromHex(userID)
			if err != nil {
				log.Printf("ChatService -> RefreshSchema -> Error getting userID: %v", err)
				dataChan <- err
				return
			}

			// Force a fresh schema fetch by using a new context with a longer timeout
			log.Printf("ChatService -> RefreshSchema -> Forcing fresh schema fetch for chatID: %s with 90-minute timeout", chatID)

			// Use the method to get schema with examples and pass selected collections
			schemaMsg, err := s.dbManager.RefreshSchemaWithExamples(schemaCtx, chatID, selectedCollectionsSlice)
			if err != nil {
				log.Printf("ChatService -> RefreshSchema -> Error refreshing schema with examples: %v", err)
				dataChan <- err
				return
			}

			if schemaMsg == "" {
				log.Printf("ChatService -> RefreshSchema -> Warning: Empty schema message returned")
				schemaMsg = "Schema refresh completed, but no schema information was returned. Please check your database connection and selected tables."
			}

			log.Printf("ChatService -> RefreshSchema -> schemaMsg length: %d", len(schemaMsg))
			llmMsg := &models.LLMMessage{
				Base:   models.NewBase(),
				UserID: userObjID,
				ChatID: chatObjID,
				Role:   string(constants.MessageTypeSystem),
				Content: map[string]interface{}{
					"schema_update": schemaMsg,
				},
			}

			// Clear previous system message from LLM
			if err := s.llmRepo.DeleteMessagesByRole(chatObjID, string(constants.MessageTypeSystem)); err != nil {
				log.Printf("ChatService -> RefreshSchema -> Error deleting system message: %v", err)
			}

			if err := s.llmRepo.CreateMessage(llmMsg); err != nil {
				log.Printf("ChatService -> RefreshSchema -> Error saving LLM message: %v", err)
			}
			log.Println("ChatService -> RefreshSchema -> Schema refreshed successfully")
			dataChan <- nil // Will be used to Synchronous refresh
		}()

		if sync {
			log.Println("ChatService -> RefreshSchema -> Waiting for Synchronous refresh to complete")
			<-dataChan
			log.Println("ChatService -> RefreshSchema -> Synchronous refresh completed")
		}
		return http.StatusOK, nil
	}
}

// Fetches paginated results for a query, default first 50 records of a large result are stored in execution_result so it fetches records after first 50 recordds
func (s *chatService) GetQueryResults(ctx context.Context, userID, chatID, messageID, queryID, streamID string, offset int) (*dtos.QueryResultsResponse, uint32, error) {
	log.Printf("ChatService -> GetQueryResults -> userID: %s, chatID: %s, messageID: %s, queryID: %s, streamID: %s, offset: %d", userID, chatID, messageID, queryID, streamID, offset)
	_, _, query, err := s.verifyQueryOwnership(userID, chatID, messageID, queryID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	if query.Pagination == nil {
		return nil, http.StatusBadRequest, fmt.Errorf("query does not support pagination")
	}
	if query.Pagination.PaginatedQuery == nil {
		return nil, http.StatusBadRequest, fmt.Errorf("query does not support pagination")
	}

	// Check the connection status and connect if needed
	if !s.dbManager.IsConnected(chatID) {
		status, err := s.ConnectDB(ctx, userID, chatID, streamID)
		if err != nil {
			return nil, status, err
		}
	}
	log.Printf("ChatService -> GetQueryResults -> query.Pagination.PaginatedQuery: %+v", query.Pagination.PaginatedQuery)
	offSettPaginatedQuery := strings.Replace(*query.Pagination.PaginatedQuery, "offset_size", strconv.Itoa(offset), 1)
	log.Printf("ChatService -> GetQueryResults -> offSettPaginatedQuery: %+v", offSettPaginatedQuery)
	result, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, messageID, queryID, streamID, offSettPaginatedQuery, *query.QueryType, false, false)
	if queryErr != nil {
		log.Printf("ChatService -> GetQueryResults -> queryErr: %+v", queryErr)
		return nil, http.StatusBadRequest, fmt.Errorf(queryErr.Message)
	}

	var formattedResultJSON interface{}
	var resultListFormatting []interface{} = []interface{}{}
	var resultMapFormatting map[string]interface{} = map[string]interface{}{}
	if err := json.Unmarshal([]byte(result.ResultJSON), &resultListFormatting); err != nil {
		if err := json.Unmarshal([]byte(result.ResultJSON), &resultMapFormatting); err != nil {
			log.Printf("ChatService -> GetQueryResults -> Error unmarshalling result JSON: %v", err)
			// Try to unmarshal as a map
			err = json.Unmarshal([]byte(result.ResultJSON), &resultMapFormatting)
			if err != nil {
				log.Printf("ChatService -> GetQueryResults -> Error unmarshalling result JSON: %v", err)
			}
		}
	}

	if len(resultListFormatting) > 0 {
		formattedResultJSON = resultListFormatting
	} else {
		formattedResultJSON = resultMapFormatting
	}

	// log.Printf("ChatService -> GetQueryResults -> formattedResultJSON: %+v", formattedResultJSON)

	s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
		Event: "query-paginated-results",
		Data: map[string]interface{}{
			"chat_id":             chatID,
			"message_id":          messageID,
			"query_id":            queryID,
			"execution_result":    formattedResultJSON,
			"error":               queryErr,
			"total_records_count": query.Pagination.TotalRecordsCount,
		},
	})
	return &dtos.QueryResultsResponse{
		ChatID:            chatID,
		MessageID:         messageID,
		QueryID:           queryID,
		ExecutionResult:   formattedResultJSON,
		Error:             queryErr,
		TotalRecordsCount: query.Pagination.TotalRecordsCount,
	}, http.StatusOK, nil
}

// Helper function to add a "Fix Rollback Error" button to a message
func (s *chatService) addFixRollbackErrorButton(msg *models.Message) {
	log.Printf("ChatService -> addFixRollbackErrorButton -> msg.id: %s", msg.ID)

	// Check if message already has a "Fix Rollback Error" button
	hasFixRollbackErrorButton := false
	for _, button := range *msg.ActionButtons {
		if button.Action == "fix_rollback_error" {
			hasFixRollbackErrorButton = true
			break
		}
	}

	if !hasFixRollbackErrorButton {
		fixRollbackErrorButton := models.ActionButton{
			ID:     primitive.NewObjectID(),
			Label:  "Fix Rollback Error",
			Action: "fix_rollback_error",
		}
		actionButtons := append(*msg.ActionButtons, fixRollbackErrorButton)
		msg.ActionButtons = &actionButtons
		log.Printf("ChatService -> addFixRollbackErrorButton -> Added fix_rollback_error button to existing array")
	}
}

// Helper function to add a "Fix Error" button to a message
func (s *chatService) addFixErrorButton(msg *models.Message) {
	log.Printf("ChatService -> addFixErrorButton -> msg.id: %s", msg.ID)

	// Check if any query has an error
	hasError := false
	if msg.Queries != nil {
		for _, query := range *msg.Queries {
			if query.Error != nil {
				hasError = true
				log.Printf("ChatService -> addFixErrorButton -> Found error in query: %s", query.ID.Hex())
				break
			}
		}
	} else {
		log.Printf("ChatService -> addFixErrorButton -> msg.Queries: nil")
		hasError = false
	}

	// Only add the button if at least one query has an error
	if !hasError {
		log.Printf("ChatService -> addFixErrorButton -> No errors found in queries, not adding button")
		return
	}

	// Create a new "Fix Error" action button
	fixErrorButton := models.ActionButton{
		ID:        primitive.NewObjectID(),
		Label:     "Fix Error",
		Action:    "fix_error",
		IsPrimary: true,
	}

	// Initialize action buttons array if it doesn't exist
	if msg.ActionButtons == nil {
		actionButtons := []models.ActionButton{fixErrorButton}
		msg.ActionButtons = &actionButtons
		log.Printf("ChatService -> addFixErrorButton -> Created new action buttons array")
	} else {
		// Check if a fix_error button already exists
		hasFixErrorButton := false
		for _, button := range *msg.ActionButtons {
			if button.Action == "fix_error" {
				hasFixErrorButton = true
				break
			}
		}

		// Add the button if it doesn't exist
		if !hasFixErrorButton {
			actionButtons := append(*msg.ActionButtons, fixErrorButton)
			msg.ActionButtons = &actionButtons
			log.Printf("ChatService -> addFixErrorButton -> Added fix_error button to existing array")
		} else {
			log.Printf("ChatService -> addFixErrorButton -> fix_error button already exists")
		}
	}

	if msg.ActionButtons != nil {
		log.Printf("ChatService -> addFixErrorButton -> msg.ActionButtons: %+v", *msg.ActionButtons)
	} else {
		log.Printf("ChatService -> addFixErrorButton -> msg.ActionButtons: nil")
	}
}

// Helper function to remove the "Fix Error" button from a message
func (s *chatService) removeFixErrorButton(msg *models.Message) {
	log.Printf("ChatService -> removeFixErrorButton -> msg.id: %s", msg.ID)
	if msg.ActionButtons == nil {
		log.Printf("ChatService -> removeFixErrorButton -> No action buttons to remove")
		return
	}

	// Check if any query has an error
	hasError := false
	if msg.Queries != nil {
		for _, query := range *msg.Queries {
			if query.Error != nil {
				hasError = true
				log.Printf("ChatService -> removeFixErrorButton -> Found error in query: %s", query.ID.Hex())
				break
			}
		}
	}

	// Only remove the button if there are no errors
	if !hasError {
		log.Printf("ChatService -> removeFixErrorButton -> No errors found, removing fix_error button")
		// Filter out the "Fix Error" button
		var filteredButtons []models.ActionButton
		for _, button := range *msg.ActionButtons {
			if button.Action != "fix_error" {
				filteredButtons = append(filteredButtons, button)
			}
		}

		// Update the message's action buttons
		if len(filteredButtons) > 0 {
			msg.ActionButtons = &filteredButtons
			log.Printf("ChatService -> removeFixErrorButton -> Updated action buttons array")
		} else {
			msg.ActionButtons = nil
			log.Printf("ChatService -> removeFixErrorButton -> Removed all action buttons")
		}
	} else {
		log.Printf("ChatService -> removeFixErrorButton -> Errors still exist, keeping fix_error button")
	}

	if msg.ActionButtons != nil {
		log.Printf("ChatService -> removeFixErrorButton -> msg.ActionButtons: %+v", *msg.ActionButtons)
	} else {
		log.Printf("ChatService -> removeFixErrorButton -> msg.ActionButtons: nil")
	}
}
