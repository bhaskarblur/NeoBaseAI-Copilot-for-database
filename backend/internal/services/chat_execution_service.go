package services

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	mathrand "math/rand"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/dbmanager"
	"neobase-ai/pkg/llm"
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

// performRAGSearch performs vector-based retrieval for a user query against the chat's vectorized schema and knowledge base.
// Returns the assembled RAG context string, the number of unique tables found, and any error.
func (s *chatService) performRAGSearch(ctx context.Context, chatID string, userQuery string) (ragContext string, tableCount int, err error) {
	if s.vectorizationSvc == nil || !s.vectorizationSvc.IsAvailable(ctx) {
		return "", 0, nil
	}

	searchQuery := userQuery
	if searchQuery == "" {
		// No user query — use a broad query to retrieve key schema highlights (e.g. for recommendations)
		searchQuery = "main tables relationships key columns important data"
	}

	topK := 10
	if userQuery == "" {
		topK = 5
	}

	ragResults, ragErr := s.vectorizationSvc.SearchSchema(ctx, chatID, searchQuery, topK)
	if ragErr != nil {
		log.Printf("performRAGSearch -> search failed: %v", ragErr)
		return "", 0, nil // non-fatal, fall back to full schema
	}

	if len(ragResults) == 0 {
		return "", 0, nil
	}

	// Count unique tables from results
	tableSet := make(map[string]struct{})
	var ragBuilder strings.Builder
	if userQuery != "" {
		ragBuilder.WriteString("\n\n--- Relevant Schema Context (from vector search) ---\n")
	} else {
		ragBuilder.WriteString("\n\n--- Key Schema Context (from vector search) ---\n")
	}
	for _, result := range ragResults {
		if content, ok := result.Payload["content"].(string); ok {
			ragBuilder.WriteString(content)
			ragBuilder.WriteString("\n---\n")
		}
		if tbl, ok := result.Payload["table_name"].(string); ok && tbl != "" {
			tableSet[tbl] = struct{}{}
		}
	}

	log.Printf("performRAGSearch -> %d results, %d unique tables, context length: %d",
		len(ragResults), len(tableSet), ragBuilder.Len())

	return ragBuilder.String(), len(tableSet), nil
}

// convertMessagesToLLMFormat converts regular messages to LLM format and prepends schema.
// ragContext is optional pre-computed RAG context to include alongside or instead of the full schema.
// useRAGOnly: when true and ragContext is non-empty, the full schema is omitted from the system message
// and only the RAG chunks are sent. This dramatically reduces token usage when the schema is
// already vectorized. A lightweight schema summary is included so the LLM knows the DB structure.
func (s *chatService) convertMessagesToLLMFormat(ctx context.Context, chat *models.Chat,
	messages []*models.Message, dbType string, ragContext string, useRAGOnly bool) ([]*models.LLMMessage, error) {
	chatIDStr := chat.ID.Hex()

	// Step 1: Get or fetch schema (skipped when RAG-only mode is active)
	var schemaStr string
	var shouldUpdateCache bool

	if useRAGOnly && ragContext != "" {
		// RAG-only mode: schema is vectorized and relevant chunks were found.
		// We include ONLY the RAG chunks and a lightweight table listing so
		// the LLM knows which tables exist without the full column details.
		log.Printf("convertMessagesToLLMFormat -> RAG-only mode: skipping full schema. Using RAG chunks instead (ragContext length: %d chars).", len(ragContext))
	} else {
		if chat.Connection.CurrentSchema != nil && *chat.Connection.CurrentSchema != "" {
			// Schema exists in cache
			schemaStr = *chat.Connection.CurrentSchema
			log.Printf("convertMessagesToLLMFormat -> Using cached schema from chat.Connection (length: %d)", len(schemaStr))
		} else {
			// Schema doesn't exist, fetch from DB Manager
			log.Printf("convertMessagesToLLMFormat -> Schema not found in chat.Connection, fetching from DB Manager")

			// Parse selected collections
			selectedCollections := []string{"ALL"}
			if chat.SelectedCollections != "" && chat.SelectedCollections != "ALL" {
				selectedCollections = strings.Split(chat.SelectedCollections, ",")
			}

			// Fetch schema with examples from DB Manager
			formattedSchema, err := s.dbManager.FormatSchemaWithExamples(ctx, chatIDStr, selectedCollections)
			if err != nil {
				// Fallback to basic schema if examples fail
				log.Printf("convertMessagesToLLMFormat -> Error getting schema with examples, trying basic schema: %v", err)

				dbConn, connErr := s.dbManager.GetConnection(chatIDStr)
				if connErr != nil {
					return nil, fmt.Errorf("failed to get database connection: %v", connErr)
				}

				connInfo, exists := s.dbManager.GetConnectionInfo(chatIDStr)
				if !exists {
					return nil, fmt.Errorf("connection info not found for chat %s", chatIDStr)
				}

				schema, schemaErr := s.dbManager.GetSchemaManager().GetSchema(ctx, chatIDStr, dbConn, connInfo.Config.Type, selectedCollections)
				if schemaErr != nil {
					return nil, fmt.Errorf("failed to get schema: %v", schemaErr)
				}

				formattedSchema = s.dbManager.GetSchemaManager().FormatSchemaForLLM(schema)
			}

			schemaStr = formattedSchema
			shouldUpdateCache = true
			log.Printf("convertMessagesToLLMFormat -> Fetched and formatted schema from DB (length: %d)", len(schemaStr))
		}
	}

	// Step 2: Create system message with schema + optional RAG context
	now := time.Now()

	systemContent := map[string]interface{}{}
	if schemaStr != "" {
		systemContent["schema_update"] = schemaStr
	}
	if ragContext != "" {
		systemContent["rag_context"] = ragContext
	}

	systemMessage := &models.LLMMessage{
		ChatID:      chat.ID,
		UserID:      chat.UserID,
		Role:        string(constants.MessageTypeSystem),
		Content:     systemContent,
		IsEdited:    false,
		NonTechMode: chat.Settings.NonTechMode,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Step 3: Convert messages to LLM format
	llmMessages := make([]*models.LLMMessage, 0, len(messages)+1)
	llmMessages = append(llmMessages, systemMessage) // Prepend schema

	for _, msg := range messages {
		var contentMap map[string]interface{}

		if string(msg.Type) == string(constants.MessageTypeUser) {
			// User message
			contentMap = map[string]interface{}{
				"user_message": msg.Content,
			}
		} else {
			// Assistant message - parse the content
			var parsedContent map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Content), &parsedContent); err != nil {
				log.Printf("Warning: Failed to parse assistant message content: %v", err)
				// Try to extract meaningful content from old-format text tool calls
				// (e.g., <ctrl42>call\nprint(default_api.generate_final_response(...)))
				extractedContent := msg.Content
				if parsed, ok := llm.TryParseTextToolCall(msg.Content); ok {
					var parsedMap map[string]interface{}
					if json.Unmarshal([]byte(parsed), &parsedMap) == nil {
						if assistantMsg, hasMsg := parsedMap["assistantMessage"].(string); hasMsg {
							extractedContent = assistantMsg
							log.Printf("convertMessagesToLLMFormat -> Extracted assistantMessage from old-format text tool call")
						}
					}
				}
				contentMap = map[string]interface{}{
					"assistant_response": extractedContent,
				}
			} else {
				// Extract response and queries
				contentMap = map[string]interface{}{
					"assistant_response": parsedContent["response"],
				}
				if queries, ok := parsedContent["query"]; ok {
					contentMap["queries"] = queries
				}
				if buttons, ok := parsedContent["button_prompts"]; ok {
					contentMap["buttons"] = buttons
				}
			}
		}

		llmMessage := &models.LLMMessage{
			MessageID:   msg.ID,
			ChatID:      msg.ChatID,
			UserID:      msg.UserID,
			Role:        string(msg.Type),
			Content:     contentMap,
			IsEdited:    msg.IsEdited,
			NonTechMode: chat.Settings.NonTechMode,
			CreatedAt:   msg.CreatedAt,
			UpdatedAt:   msg.UpdatedAt,
		}

		llmMessages = append(llmMessages, llmMessage)
	}

	// Step 4: Update cache if needed (async, don't wait)
	if shouldUpdateCache {
		go func() {
			updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Update chat document
			if err := s.chatRepo.UpdateConnectionSchema(updateCtx, chat.ID, schemaStr); err != nil {
				log.Printf("Warning: Failed to update schema in chat.Connection: %v", err)
			} else {
				log.Printf("convertMessagesToLLMFormat -> Successfully cached schema in chat.Connection")
			}
		}()
	}

	return llmMessages, nil
}

// injectToolQueriesIfMissing checks if the LLM's final response has an empty queries array
// but the tool history shows execute_read_query calls were made. In that case, it injects
// those queries into the response so the user can see and re-run them.
// This is a safety net — the prompt instructs the LLM to include queries, but models
// sometimes omit them thinking "I already executed it, user doesn't need it."
// Exploration-only queries (information_schema, SHOW TABLES, etc.) are filtered out
// because they are not useful to the user.
func (s *chatService) injectToolQueriesIfMissing(response string, toolResult *llm.ToolCallResult) string {
	if toolResult == nil || len(toolResult.ToolHistory) == 0 {
		return response
	}

	// Parse the response
	var jsonResp map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonResp); err != nil {
		return response
	}

	// Check if queries is empty or nil
	queries, _ := jsonResp["queries"].([]interface{})
	if len(queries) > 0 {
		return response // LLM already included queries, nothing to do
	}

	// Collect queries from tool history (execute_read_query calls),
	// filtering out pure exploration queries that only discover schema metadata.
	var injectedQueries []interface{}
	for _, call := range toolResult.ToolHistory {
		if call.Name != llm.ExecuteQueryToolName {
			continue
		}
		queryStr, _ := call.Arguments["query"].(string)
		explanation, _ := call.Arguments["explanation"].(string)
		if queryStr == "" {
			continue
		}

		// Skip exploration-only queries that just discover table/collection names or schema metadata.
		// These are not useful to the user as executable queries.
		upperQuery := strings.ToUpper(strings.TrimSpace(queryStr))
		if isExplorationQuery(upperQuery) {
			log.Printf("processLLMResponse -> injectToolQueriesIfMissing: skipping exploration query: %s", queryStr)
			continue
		}

		queryObj := map[string]interface{}{
			"query":       queryStr,
			"explanation": explanation,
			"queryType":   "SELECT",
		}
		injectedQueries = append(injectedQueries, queryObj)
	}

	if len(injectedQueries) == 0 {
		return response
	}

	log.Printf("processLLMResponse -> injectToolQueriesIfMissing: LLM returned empty queries but %d execute_read_query calls found in tool history, injecting", len(injectedQueries))
	jsonResp["queries"] = injectedQueries

	updatedResponse, err := json.Marshal(jsonResp)
	if err != nil {
		return response
	}
	return string(updatedResponse)
}

// isExplorationQuery returns true if the query is a pure schema exploration query
// (e.g. listing tables, describing columns) that should not be shown to the user
// as an executable query.
func isExplorationQuery(upperQuery string) bool {
	explorationPatterns := []string{
		"INFORMATION_SCHEMA",
		"SHOW TABLES",
		"SHOW DATABASES",
		"SHOW COLUMNS",
		"SHOW FULL COLUMNS",
		"SHOW CREATE TABLE",
		"\\DT",
		"\\D ",
		"\\D+",
		"DESCRIBE ",
		"DESC ",
		"SHOW COLLECTIONS",
		"DB.GETCOMMAND",
		"DB.GETCOLLECTIONNAMES",
		"SYSTEM.TABLES",
		"SYSTEM.COLUMNS",
		"SYSTEM.DATABASES",
	}
	for _, pattern := range explorationPatterns {
		if strings.Contains(upperQuery, pattern) {
			return true
		}
	}
	return false
}

// private function, processLLMResponse processes the LLM response updates SSE stream only if synchronous is false, allowSSEUpdates is used to send SSE updates to the client except the final ai-response event
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

	// Get chat to access settings
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, fmt.Errorf("failed to fetch chat: %v", err)
	}

	log.Printf("ChatService -> Execute -> Chat settings: AutoExecuteQuery=%v, ShareDataWithAI=%v, NonTechMode=%v",
		chat.Settings.AutoExecuteQuery, chat.Settings.ShareDataWithAI, chat.Settings.NonTechMode)

	// Get the user message to retrieve the selected LLM model
	userMessage, err := s.chatRepo.FindMessageByID(userMessageObjID)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, fmt.Errorf("failed to fetch user message: %v", err)
	}

	var selectedLLMModel string

	// Initialize selectedLLMModel, will be finalized after fetching messages
	// For new messages, use the model from the user message (if set)
	if userMessage != nil && !userMessage.IsEdited && userMessage.LLMModel != nil && *userMessage.LLMModel != "" {
		// New message: use the model from the user message
		selectedLLMModel = *userMessage.LLMModel
		log.Printf("processLLMResponse -> Selected LLM Model from user message: %s", selectedLLMModel)
	}

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		s.handleError(ctx, chatID, fmt.Errorf("connection info not found"))
		// Let's create a new connection
		_, err := s.ConnectDB(ctx, userID, chatID, streamID)
		if err != nil {
			// Get model display name for error response
			var llmModelName *string
			if selectedLLMModel != "" {
				displayName := s.getModelDisplayName(selectedLLMModel)
				llmModelName = &displayName
			}
			// Send a error event to the client
			s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: "ai-response-error",
				Data: map[string]interface{}{
					"error":          "Error: " + err.Error(),
					"llm_model":      selectedLLMModel,
					"llm_model_name": llmModelName,
				},
			})
			return nil, err
		}

	}

	// Fetch all regular messages from the chat
	regularMessages, _, err := s.chatRepo.FindMessagesByChat(chatObjID, 1, 1000) // Get up to 1000 messages
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, fmt.Errorf("failed to fetch messages: %v", err)
	}

	// Filter messages up to the edited message
	filteredRegularMessages := make([]*models.Message, 0)
	for _, msg := range regularMessages {
		filteredRegularMessages = append(filteredRegularMessages, msg)
		if msg.ID == userMessageObjID {
			break
		}
	}

	// --- Sliding Window + Message RAG ---
	// For long conversations, we use a hybrid approach:
	//   1. Always include the last N messages (sliding window) for recency context
	//   2. For older messages, do semantic search to find ones relevant to the current query
	// This mirrors how ChatGPT/Claude handle long conversations.
	windowSize := constants.SlidingWindowSize
	var recentMessages []*models.Message
	var olderMessagesForRAG []*models.Message

	if len(filteredRegularMessages) <= windowSize {
		// Short conversation — include everything, no RAG needed
		recentMessages = filteredRegularMessages
	} else {
		// Long conversation — split into recent window + older messages
		recentMessages = filteredRegularMessages[len(filteredRegularMessages)-windowSize:]
		olderMessagesForRAG = filteredRegularMessages[:len(filteredRegularMessages)-windowSize]
		log.Printf("processLLMResponse -> Sliding window: %d recent + %d older messages for chat %s",
			len(recentMessages), len(olderMessagesForRAG), chatID)
	}

	// Now finalize model selection for edited messages
	// Priority: 1) Chat's preferred model, 2) Last assistant message's model, 3) Default LLM model for the provider
	if userMessage != nil && userMessage.IsEdited && selectedLLMModel == "" {
		// Edited message: Priority 1) Chat's preferred model, 2) Last assistant message's model, 3) Default LLM model for the provider

		// Priority 1: Check chat's preferred model first
		if chat.PreferredLLMModel != nil && *chat.PreferredLLMModel != "" {
			selectedLLMModel = *chat.PreferredLLMModel
			log.Printf("processLLMResponse -> Edited message: Using chat's preferred model: %s", selectedLLMModel)
		} else {
			// Priority 2: Check last assistant message's model
			lastAssistantModel := ""
			// Find from recent messages directly (before LLM conversion)
			for i := len(recentMessages) - 1; i >= 0; i-- {
				if string(recentMessages[i].Type) == string(constants.MessageTypeAssistant) {
					if recentMessages[i].LLMModel != nil && *recentMessages[i].LLMModel != "" {
						lastAssistantModel = *recentMessages[i].LLMModel
						break
					}
				}
			}

			if lastAssistantModel != "" {
				// Validate that the last assistant model is still enabled and valid
				if constants.IsValidModel(lastAssistantModel) {
					selectedLLMModel = lastAssistantModel
					log.Printf("processLLMResponse -> Edited message: Using last assistant message's model: %s", selectedLLMModel)
				} else {
					log.Printf("processLLMResponse -> Last assistant model %s is no longer available, falling back to provider default", lastAssistantModel)
				}
			}

			// If last assistant model is not available, try provider defaults
			if selectedLLMModel == "" {
				// Priority 3: Get default LLM model for the provider (in provider priority order)
				// Try providers in order: OpenAI -> Gemini -> Claude -> Ollama
				providers := []string{constants.OpenAI, constants.Gemini, constants.Claude, constants.Ollama}
				for _, provider := range providers {
					if defaultModel := constants.GetDefaultModelForProvider(provider); defaultModel != nil && defaultModel.IsEnabled {
						selectedLLMModel = defaultModel.ID
						log.Printf("processLLMResponse -> Edited message: Using default model for provider: %s (%s)", selectedLLMModel, defaultModel.Provider)
						break
					}
				}
			}
		}

		// Update the user message with the selected model
		if userMessage.LLMModel == nil || *userMessage.LLMModel != selectedLLMModel {
			userMessage.LLMModel = &selectedLLMModel
			userMessageID := userMessage.ID
			if err := s.chatRepo.UpdateMessage(userMessageID, userMessage); err != nil {
				log.Printf("Warning: Failed to update user message with new model: %v", err)
			}
		}
	}

	// Handle fallback if still no model selected
	if selectedLLMModel == "" {
		// Use default LLM model for the provider (in provider priority order)
		// Try providers in order: OpenAI -> Gemini -> Claude -> Ollama
		providers := []string{constants.OpenAI, constants.Gemini, constants.Claude, constants.Ollama}
		for _, provider := range providers {
			if defaultModel := constants.GetDefaultModelForProvider(provider); defaultModel != nil && defaultModel.IsEnabled {
				// Validate that the model is properly configured
				if constants.IsValidModel(defaultModel.ID) {
					selectedLLMModel = defaultModel.ID
					log.Printf("processLLMResponse -> No model selected, using default model for provider: %s (%s)", selectedLLMModel, defaultModel.Provider)
					break
				}
			}
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
			Data:  "Fetching relevant data points & structure for the request..",
		})
	}
	if checkCancellation() {
		return nil, fmt.Errorf("operation cancelled")
	}

	// --- Schema RAG: Perform BEFORE message conversion ---
	// If schema is vectorized, we can skip sending the entire schema (~1M+ chars) to the LLM
	// and instead send only the relevant RAG chunks, saving ~95% of tokens.
	var ragContext string
	var useRAGOnly bool
	schemaVectorized := false

	if s.vectorizationSvc != nil && s.vectorizationSvc.IsAvailable(ctx) {
		if !synchronous || allowSSEUpdates {
			s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: "ai-response-step",
				Data:  "Searching knowledge base for relevant context..",
			})
		}

		// Extract the user's latest query
		userQuery := ""
		for i := len(filteredRegularMessages) - 1; i >= 0; i-- {
			if string(filteredRegularMessages[i].Type) == string(constants.MessageTypeUser) {
				userQuery = filteredRegularMessages[i].Content
				break
			}
		}

		// Check if schema vectors exist for this chat
		schemaVectorized = s.vectorizationSvc.HasSchemaVectors(ctx, chatID)

		// Auto-vectorize old chats that have a cached schema but no vectors yet.
		// This triggers vectorization in the background so subsequent messages benefit from RAG.
		// The CURRENT request will proceed without vectors (tool-calling discovery mode).
		if !schemaVectorized && chat.Connection.CurrentSchema != nil && *chat.Connection.CurrentSchema != "" {
			log.Printf("processLLMResponse -> Chat %s has cached schema but no vectors. Triggering background vectorization.", chatID)
			if !synchronous || allowSSEUpdates {
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-step",
					Data:  "Setting up knowledge base for future queries (first-time setup)...",
				})
			}
			go func() {
				bgCtx, bgCancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer bgCancel()

				// First sync KB descriptions so enriched chunks have KB data
				if *chat.Connection.CurrentSchema != "" {
					s.syncKnowledgeBase(bgCtx, chatID, *chat.Connection.CurrentSchema)
				}

				// Then vectorize schema with enriched chunks
				s.vectorizeSchemaForChat(bgCtx, chatID)
				log.Printf("processLLMResponse -> Background vectorization completed for chat %s", chatID)
			}()
		}

		ragCtx, tableCount, _ := s.performRAGSearch(ctx, chatID, userQuery)

		if !synchronous || allowSSEUpdates {
			if tableCount > 0 {
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-step",
					Data:  fmt.Sprintf("Found %d relevant tables from knowledge base", tableCount),
				})
			} else {
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-step",
					Data:  "Exploring knowledge base using tool-calling to discover relevant context..",
				})
			}
		}

		if ragCtx != "" {
			ragContext = ragCtx
			// When schema is vectorized and RAG found relevant chunks, skip sending
			// the full schema (~1M+ chars) — the RAG chunks contain everything the
			// LLM needs for this specific query. Fall back to full schema if not vectorized.
			if schemaVectorized {
				useRAGOnly = true
				log.Printf("processLLMResponse -> RAG-only mode: schema is vectorized and RAG found %d tables. Skipping full schema.", tableCount)
			}
		} else if tableCount == 0 && userQuery != "" {
			// RAG search was performed but found NO matching tables.
			// NEVER send the full schema — instead instruct the LLM to use
			// tool-calling (get_table_info, execute_read_query) to discover
			// the database structure on its own. This saves ~300-400K tokens.
			ragContext = constants.GetRagNoMatchingTablesFound(connInfo.Config.Type)
			useRAGOnly = true
			log.Printf("processLLMResponse -> No RAG tables found. Using tool-calling discovery mode (no full schema sent). dbType=%s", connInfo.Config.Type)
		}
	}

	// Convert the recent window messages to LLM format.
	// When useRAGOnly=true, the full schema is omitted and only RAG chunks are sent as context.
	filteredMessages, err := s.convertMessagesToLLMFormat(ctx, chat, recentMessages, connInfo.Config.Type, ragContext, useRAGOnly)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, fmt.Errorf("failed to convert messages to LLM format: %v", err)
	}

	// --- Message RAG: Retrieve relevant older messages ---
	if s.vectorizationSvc != nil && s.vectorizationSvc.IsAvailable(ctx) {
		userQuery := ""
		for i := len(filteredRegularMessages) - 1; i >= 0; i-- {
			if string(filteredRegularMessages[i].Type) == string(constants.MessageTypeUser) {
				userQuery = filteredRegularMessages[i].Content
				break
			}
		}

		// Only perform message RAG if there are older messages outside the sliding window
		if len(olderMessagesForRAG) > 0 && userQuery != "" {
			if !synchronous || allowSSEUpdates {
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-step",
					Data:  "Retrieving relevant earlier conversation context..",
				})
			}

			// Build exclusion set from messages already in the recent window
			excludeIDs := make([]string, 0, len(recentMessages))
			for _, msg := range recentMessages {
				excludeIDs = append(excludeIDs, msg.ID.Hex())
			}

			msgResults, msgErr := s.vectorizationSvc.SearchMessages(ctx, chatID, userQuery, constants.MessageRAGTopK, excludeIDs)
			if msgErr != nil {
				log.Printf("processLLMResponse -> Message RAG search failed (non-fatal): %v", msgErr)
			} else if len(msgResults) > 0 {
				log.Printf("processLLMResponse -> Message RAG found %d relevant older messages", len(msgResults))

				// Build retrieved messages as LLM messages and insert them AFTER the system message
				// but BEFORE the recent sliding window, with a clear separator.
				retrievedLLMMessages := make([]*models.LLMMessage, 0, len(msgResults)+1)

				// Add a separator system note
				retrievedLLMMessages = append(retrievedLLMMessages, &models.LLMMessage{
					Role: string(constants.MessageTypeSystem),
					Content: map[string]interface{}{
						"context_note": fmt.Sprintf("The following %d messages are from earlier in this conversation, retrieved because they are relevant to the current query. They are NOT the most recent messages — the recent conversation follows after.", len(msgResults)),
					},
				})

				for _, r := range msgResults {
					role, _ := r.Payload["role"].(string)
					content, _ := r.Payload["content"].(string)
					if role == "" || content == "" {
						continue
					}

					var contentMap map[string]interface{}
					if role == string(constants.MessageTypeUser) {
						contentMap = map[string]interface{}{
							"user_message": content,
						}
					} else {
						contentMap = map[string]interface{}{
							"assistant_response": content,
						}
					}

					retrievedLLMMessages = append(retrievedLLMMessages, &models.LLMMessage{
						Role:    role,
						Content: contentMap,
					})
				}

				// Insert retrieved messages after system message (index 0) but before recent window
				if len(retrievedLLMMessages) > 1 { // > 1 because we always have the separator
					newFiltered := make([]*models.LLMMessage, 0, len(filteredMessages)+len(retrievedLLMMessages))
					newFiltered = append(newFiltered, filteredMessages[0]) // system message
					newFiltered = append(newFiltered, retrievedLLMMessages...)
					newFiltered = append(newFiltered, filteredMessages[1:]...) // recent window messages
					filteredMessages = newFiltered

					if !synchronous || allowSSEUpdates {
						s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
							Event: "ai-response-step",
							Data:  fmt.Sprintf("Found %d relevant earlier messages for context", len(msgResults)),
						})
					}
				}
			}
		}
	}

	if checkCancellation() {
		return nil, fmt.Errorf("operation cancelled")
	}

	// Get the correct LLM client based on the selected model's provider
	llmClient := s.llmClient
	if s.llmManager != nil && selectedLLMModel != "" {
		selectedModel := constants.GetLLMModel(selectedLLMModel)
		if selectedModel != nil {
			providerClient, err := s.llmManager.GetClient(selectedModel.Provider)
			if err != nil {
				log.Printf("Warning: Failed to get LLM client for provider '%s': %v, will use default client", selectedModel.Provider, err)
			} else {
				llmClient = providerClient
				log.Printf("processLLMResponse -> Using LLM client for provider: %s", selectedModel.Provider)
			}
		}
	}

	// Log messages being sent to LLM (for debugging)
	log.Printf("========== LLM CONTEXT DEBUG START ==========")
	log.Printf("processLLMResponse -> Sending %d messages to LLM", len(filteredMessages))
	log.Printf("processLLMResponse -> Database Type: %s", connInfo.Config.Type)
	log.Printf("processLLMResponse -> NonTechMode: %v", chat.Settings.NonTechMode)
	log.Printf("processLLMResponse -> Selected LLM Model: %s", selectedLLMModel)
	log.Printf("processLLMResponse -> Schema Vectorized: %v, RAG-only mode: %v", schemaVectorized, useRAGOnly)
	totalContextChars := 0
	for i, msg := range filteredMessages {
		contentStr := ""
		if contentBytes, err := json.Marshal(msg.Content); err == nil {
			contentStr = string(contentBytes)
		}
		msgChars := len(contentStr)
		totalContextChars += msgChars
		approxTokens := msgChars / 4 // rough estimate: ~4 chars per token
		log.Printf("processLLMResponse -> Message[%d]: Role=%s, Chars=%d, ~Tokens=%d, MessageID=%s",
			i, msg.Role, msgChars, approxTokens, msg.MessageID.Hex())
		// Log content preview (first 500 chars to avoid too much log spam)
		if msgChars > 500 {
			log.Printf("processLLMResponse -> Message[%d] Content (first 500 chars): %s...", i, contentStr[:500])
		} else {
			log.Printf("processLLMResponse -> Message[%d] Content: %s", i, contentStr)
		}
	}
	log.Printf("processLLMResponse -> TOTAL context: %d chars, ~%d tokens (approx)", totalContextChars, totalContextChars/4)
	log.Printf("========== LLM CONTEXT DEBUG END ==========")

	// Send SSE step right before the LLM call
	if !synchronous || allowSSEUpdates {
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "Running the workflow & fetching the results..",
		})
	}

	// Generate LLM response using iterative tool-calling.
	// The LLM can call tools (execute_read_query, get_table_info) to explore the
	// database before calling generate_final_response with the structured answer.
	toolExecutor := BuildToolExecutor(s.dbManager, chatID, connInfo.Config.Type)
	tools := llm.GetNeobaseTools()

	// Build system prompt addendum for tool-calling instructions
	toolCallConfig := llm.ToolCallConfig{
		MaxIterations: llm.DefaultMaxIterations,
		DBType:        connInfo.Config.Type,
		NonTechMode:   chat.Settings.NonTechMode,
		ModelID:       selectedLLMModel,
		SystemPrompt:  llm.GetToolCallingSystemPromptAddendum(),
		OnToolCall: func(call llm.ToolCall) {
			if !synchronous || allowSSEUpdates {
				var stepMsg string
				if call.Name == "generate_final_response" {
					stepMsg = "Preparing your response..."
				} else if explanation, ok := call.Arguments["explanation"].(string); ok && explanation != "" {
					stepMsg = fmt.Sprintf("Exploring: %s", explanation)
				} else {
					stepMsg = fmt.Sprintf("Exploring database: calling %s", call.Name)
				}
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-step",
					Data:  stepMsg,
				})
			}
		},
		OnToolResult: func(call llm.ToolCall, result llm.ToolResult) {
			if !synchronous || allowSSEUpdates {
				if result.IsError {
					s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
						Event: "ai-response-step",
						Data:  fmt.Sprintf("Tool %s returned an error, retrying..", call.Name),
					})
				}
			}
		},
		OnIteration: func(iteration int, toolCallCount int) {
			log.Printf("processLLMResponse -> Tool-calling iteration %d, total calls so far: %d", iteration, toolCallCount)
		},
	}

	toolResult, err := llmClient.GenerateWithTools(ctx, filteredMessages, tools, toolExecutor, toolCallConfig)
	if err != nil {
		if !synchronous || allowSSEUpdates {
			// Get model display name for error response
			var llmModelName *string
			if selectedLLMModel != "" {
				displayName := s.getModelDisplayName(selectedLLMModel)
				llmModelName = &displayName
			}
			// Show a user-friendly message instead of raw internal errors
			userErrorMsg := "The AI model was unable to generate a complete response. Please try again or use a different model."
			log.Printf("processLLMResponse -> LLM GenerateWithTools error (raw): %v", err)
			s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: "ai-response-error",
				Data: map[string]interface{}{
					"error":          userErrorMsg,
					"llm_model":      selectedLLMModel,
					"llm_model_name": llmModelName,
				},
			})
		}
		return nil, fmt.Errorf("failed to generate LLM response: %v", err)
	}

	response := toolResult.Response
	log.Printf("processLLMResponse -> Tool-calling completed: %d iterations, %d total tool calls", toolResult.Iterations, toolResult.TotalCalls)

	// Safety net: if the LLM returned empty queries but actually executed queries via tools,
	// inject those queries into the response so the user can see and re-run them.
	response = s.injectToolQueriesIfMissing(response, toolResult)

	log.Printf("processLLMResponse -> response: %s", response)

	if checkCancellation() {
		return nil, fmt.Errorf("operation cancelled")
	}

	// Send initial processing message
	if !synchronous || allowSSEUpdates {
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "Analyzing the criticality of the request..",
		})
	}

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
		// Get model display name for error response
		var llmModelName *string
		if selectedLLMModel != "" {
			displayName := s.getModelDisplayName(selectedLLMModel)
			llmModelName = &displayName
		}
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-error",
			Data: map[string]interface{}{
				"error":          "Error: " + err.Error(),
				"llm_model":      selectedLLMModel,
				"llm_model_name": llmModelName,
			},
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
				// Use pooled buffer instead of allocating new one
				buf := utils.GetJSONBuffer()
				encoder := json.NewEncoder(buf)
				encoder.SetEscapeHTML(false)
				_ = encoder.Encode(queryMap["exampleResult"].([]interface{}))
				resultStr := buf.String()
				utils.PutJSONBuffer(buf)
				// Encrypt the example result before storage
				encryptedResult := s.encryptQueryResult(resultStr)
				exampleResult = utils.StringPtr(encryptedResult)
				log.Printf("processLLMResponse -> saving exampleResult (encrypted): %v", *exampleResult)
			} else {
				exampleResult = nil
				log.Println("processLLMResponse -> saving exampleResult: nil")
			}

			var rollbackDependentQuery *string
			if rdq, ok := queryMap["rollbackDependentQuery"].(string); ok {
				rollbackDependentQuery = utils.StringPtr(rdq)
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
				if pagMap, ok := queryMap["pagination"].(map[string]interface{}); ok {
					if pq, ok := pagMap["paginatedQuery"].(string); ok {
						pagination.PaginatedQuery = utils.StringPtr(pq)
						log.Printf("processLLMResponse -> pagination.PaginatedQuery: %v", pq)
					}
					if cq, ok := pagMap["countQuery"].(string); ok {
						pagination.CountQuery = utils.StringPtr(cq)
						log.Printf("processLLMResponse -> pagination.CountQuery: %v", cq)
					}
				}
			}
			var tables *string
			if queryMap["tables"] != nil {
				switch v := queryMap["tables"].(type) {
				case string:
					tables = utils.StringPtr(v)
				case []interface{}:
					names := make([]string, 0, len(v))
					for _, item := range v {
						if s, ok := item.(string); ok {
							names = append(names, s)
						}
					}
					if len(names) > 0 {
						tables = utils.StringPtr(strings.Join(names, ", "))
					}
				}
			}

			if queryMap["collections"] != nil {
				switch v := queryMap["collections"].(type) {
				case string:
					tables = utils.StringPtr(v)
				case []interface{}:
					names := make([]string, 0, len(v))
					for _, item := range v {
						if s, ok := item.(string); ok {
							names = append(names, s)
						}
					}
					if len(names) > 0 {
						tables = utils.StringPtr(strings.Join(names, ", "))
					}
				}
			}

			if queryMap["collection"] != nil {
				if s, ok := queryMap["collection"].(string); ok && tables == nil {
					tables = utils.StringPtr(s)
				}
			}
			var queryType *string
			if qt, ok := queryMap["queryType"].(string); ok {
				queryType = utils.StringPtr(qt)
			}

			var rollbackQuery *string
			if rq, ok := queryMap["rollbackQuery"].(string); ok {
				rollbackQuery = utils.StringPtr(rq)
			}

			// Safely extract required string fields with defaults
			queryStr, _ := queryMap["query"].(string)
			explanationStr, _ := queryMap["explanation"].(string)
			canRollback, _ := queryMap["canRollback"].(bool)
			isCritical, _ := queryMap["isCritical"].(bool)

			// Create the query object
			query := models.Query{
				ID:                     primitive.NewObjectID(),
				Query:                  queryStr,
				Description:            explanationStr,
				ExecutionTime:          nil,
				ExampleExecutionTime:   int(*estimateResponseTime),
				CanRollback:            canRollback,
				IsCritical:             isCritical,
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
				LLMModel:               selectedLLMModel,
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
					// Use pooled buffer for JSON encoding
					buf := utils.GetJSONBuffer()
					encoder := json.NewEncoder(buf)
					encoder.SetEscapeHTML(false)
					if err := encoder.Encode(metadata); err == nil {
						metadataStr := buf.String()
						query.Metadata = utils.StringPtr(metadataStr)
					}
					utils.PutJSONBuffer(buf)
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
		if selectedLLMModel != "" {
			existingMessage.LLMModel = &selectedLLMModel // Update with the LLM model used
		}

		// Update the message in the database
		if err := s.chatRepo.UpdateMessage(existingMessage.ID, existingMessage); err != nil {
			s.handleError(ctx, chatID, err)
			return nil, fmt.Errorf("failed to update AI message: %v", err)
		}

		if !synchronous {
			// Send final response
			var llmModelName *string
			if existingMessage.LLMModel != nil {
				displayName := s.getModelDisplayName(*existingMessage.LLMModel)
				llmModelName = &displayName
			}
			s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: "ai-response",
				Data: &dtos.MessageResponse{
					ID:            existingMessage.ID.Hex(),
					ChatID:        existingMessage.ChatID.Hex(),
					Content:       existingMessage.Content,
					UserMessageID: utils.StringPtr(userMessageObjID.Hex()),
					Queries:       dtos.ToQueryDtoWithDecryption(existingMessage.Queries, s.decryptQueryResult, s.visualizationRepo, ctx),
					ActionButtons: dtos.ToActionButtonDto(existingMessage.ActionButtons),
					Type:          existingMessage.Type,
					LLMModel:      existingMessage.LLMModel,
					LLMModelName:  llmModelName,
					CreatedAt:     existingMessage.CreatedAt.Format(time.RFC3339),
					UpdatedAt:     existingMessage.UpdatedAt.Format(time.RFC3339),
					IsEdited:      existingMessage.IsEdited,
				},
			})
		}

		// Compute LLM model name for the synchronous response
		var llmModelNameForResponse *string
		if existingMessage.LLMModel != nil {
			displayName := s.getModelDisplayName(*existingMessage.LLMModel)
			llmModelNameForResponse = &displayName
		}

		return &dtos.MessageResponse{
			ID:            existingMessage.ID.Hex(),
			ChatID:        existingMessage.ChatID.Hex(),
			Content:       existingMessage.Content,
			UserMessageID: utils.StringPtr(userMessageObjID.Hex()),
			Queries:       dtos.ToQueryDtoWithDecryption(existingMessage.Queries, s.decryptQueryResult, s.visualizationRepo, ctx),
			ActionButtons: dtos.ToActionButtonDto(existingMessage.ActionButtons),
			Type:          existingMessage.Type,
			LLMModel:      existingMessage.LLMModel,
			LLMModelName:  llmModelNameForResponse,
			NonTechMode:   existingMessage.NonTechMode,
			CreatedAt:     existingMessage.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     existingMessage.UpdatedAt.Format(time.RFC3339),
			IsEdited:      existingMessage.IsEdited,
		}, nil
	}

	log.Printf("processLLMResponse -> saving new message actionButtonsPtr: %v", actionButtonsPtr)
	log.Printf("processLLMResponse -> Creating assistant message with NonTechMode=%v from chat settings", chat.Settings.NonTechMode)
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
		UserMessageId: &userMessageObjID,         // Set the user message ID that this AI message is responding to
		NonTechMode:   chat.Settings.NonTechMode, // Store the non-tech mode setting with the message
	}
	if selectedLLMModel != "" {
		chatResponseMsg.LLMModel = &selectedLLMModel // Store which LLM model was used to generate this message
	}

	if err := s.chatRepo.CreateMessage(chatResponseMsg); err != nil {
		log.Printf("processLLMResponse -> Error saving chat response message: %v", err)
		return nil, err
	}

	// Vectorize assistant message in the background for conversational RAG
	go func() {
		bgCtx := context.Background()
		if s.vectorizationSvc != nil && assistantMessage != "" {
			_, total, _ := s.chatRepo.FindMessagesByChat(chatObjID, 1, 1)
			if err := s.vectorizationSvc.VectorizeMessage(bgCtx, chatID, chatResponseMsg.ID.Hex(), "assistant", assistantMessage, int(total)); err != nil {
				log.Printf("processLLMResponse -> Failed to vectorize assistant message: %v", err)
			}
		}
	}()

	if !synchronous {
		// Send final response
		var llmModelName *string
		if chatResponseMsg.LLMModel != nil {
			displayName := s.getModelDisplayName(*chatResponseMsg.LLMModel)
			llmModelName = &displayName
		}
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response",
			Data: &dtos.MessageResponse{
				ID:            chatResponseMsg.ID.Hex(),
				ChatID:        chatResponseMsg.ChatID.Hex(),
				Content:       chatResponseMsg.Content,
				UserMessageID: utils.StringPtr(userMessageObjID.Hex()),
				Queries:       dtos.ToQueryDtoWithDecryption(chatResponseMsg.Queries, s.decryptQueryResult, s.visualizationRepo, ctx),
				ActionButtons: dtos.ToActionButtonDto(chatResponseMsg.ActionButtons),
				Type:          chatResponseMsg.Type,
				LLMModel:      chatResponseMsg.LLMModel,
				LLMModelName:  llmModelName,
				NonTechMode:   chatResponseMsg.NonTechMode,
				CreatedAt:     chatResponseMsg.CreatedAt.Format(time.RFC3339),
				UpdatedAt:     chatResponseMsg.UpdatedAt.Format(time.RFC3339),
			},
		})
	}
	var llmModelName *string
	if chatResponseMsg.LLMModel != nil {
		displayName := s.getModelDisplayName(*chatResponseMsg.LLMModel)
		llmModelName = &displayName
	}
	return &dtos.MessageResponse{
		ID:            chatResponseMsg.ID.Hex(),
		ChatID:        chatResponseMsg.ChatID.Hex(),
		Content:       chatResponseMsg.Content,
		UserMessageID: utils.StringPtr(userMessageObjID.Hex()),
		Queries:       dtos.ToQueryDtoWithDecryption(chatResponseMsg.Queries, s.decryptQueryResult, s.visualizationRepo, ctx),
		ActionButtons: dtos.ToActionButtonDto(chatResponseMsg.ActionButtons),
		Type:          chatResponseMsg.Type,
		LLMModel:      chatResponseMsg.LLMModel,
		LLMModelName:  llmModelName,
		NonTechMode:   chatResponseMsg.NonTechMode,
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

	// Log connection details for debugging spreadsheet connections
	if chat.Connection.Type == constants.DatabaseTypeSpreadsheet || chat.Connection.Type == constants.DatabaseTypeGoogleSheets {
		log.Printf("ChatService -> ConnectDB -> %s connection after decrypt: Host=%s, Database=%s", chat.Connection.Type, chat.Connection.Host, chat.Connection.Database)
		if chat.Connection.Type == constants.DatabaseTypeGoogleSheets {
			log.Printf("ChatService -> ConnectDB -> Google Sheet ID: %v", chat.Connection.GoogleSheetID)
		}
	}

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

	// Determine schema name for spreadsheet connections
	schemaName := ""
	if chat.Connection.Type == constants.DatabaseTypeSpreadsheet || chat.Connection.Type == constants.DatabaseTypeGoogleSheets {
		schemaName = fmt.Sprintf("conn_%s", chatID)
	}

	// Connect to database
	err = s.dbManager.Connect(chatID, userID, streamID, dbmanager.ConnectionConfig{
		Type:               chat.Connection.Type,
		Host:               chat.Connection.Host,
		Port:               chat.Connection.Port,
		Username:           chat.Connection.Username,
		Password:           chat.Connection.Password,
		Database:           chat.Connection.Database,
		AuthDatabase:       chat.Connection.AuthDatabase, // Added AuthDatabase
		UseSSL:             chat.Connection.UseSSL,
		SSLMode:            chat.Connection.SSLMode,
		SSLCertURL:         chat.Connection.SSLCertURL,
		SSLKeyURL:          chat.Connection.SSLKeyURL,
		SSLRootCertURL:     chat.Connection.SSLRootCertURL,
		GoogleSheetID:      chat.Connection.GoogleSheetID,
		GoogleAuthToken:    chat.Connection.GoogleAuthToken,
		GoogleRefreshToken: chat.Connection.GoogleRefreshToken,
		SchemaName:         schemaName,
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

	// Safe dereference of QueryType — default to "SELECT" if nil.
	// This can happen when injectToolQueriesIfMissing creates queries without a queryType field.
	queryType := "SELECT"
	if query.QueryType != nil {
		queryType = *query.QueryType
	}

	// To find total records count, we need to execute the pagination.countQuery with findCount = true
	if query.Pagination != nil && query.Pagination.CountQuery != nil && *query.Pagination.CountQuery != "" {
		log.Printf("ChatService -> ExecuteQuery -> query.Pagination.CountQuery is present, will use it to get the total records count")
		countResult, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, *query.Pagination.CountQuery, queryType, false, true)
		if queryErr != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error executing count query: %v", queryErr)
		}
		if countResult != nil && countResult.Result != nil {
			log.Printf("ChatService -> ExecuteQuery -> countResult.Result: %+v", countResult.Result)

			// Try to extract count from different possible formats

			// First type assert Result to map
			if resultMap, ok := countResult.Result.(map[string]interface{}); ok {
				// Format 1: Direct count in the result
				if countVal, ok := resultMap["count"].(float64); ok {
					tempCount := int(countVal)
					totalRecordsCount = &tempCount
					log.Printf("ChatService -> ExecuteQuery -> Found count directly in result: %d", tempCount)
				} else if countVal, ok := resultMap["count"].(int64); ok {
					tempCount := int(countVal)
					totalRecordsCount = &tempCount
					log.Printf("ChatService -> ExecuteQuery -> Found count directly in result (int64): %d", tempCount)
				} else if countVal, ok := resultMap["count"].(int); ok {
					totalRecordsCount = &countVal
					log.Printf("ChatService -> ExecuteQuery -> Found count directly in result (int): %d", countVal)
				} else if results, ok := resultMap["results"]; ok {
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
					if resultsRaw, ok := resultMap["results"]; ok {
						log.Printf("ChatService -> ExecuteQuery -> Trying direct approach for format: map[results:[map[count:92]]]")

						// Convert to JSON and back to ensure proper type handling
						buf := utils.GetJSONBuffer()
						encoder := json.NewEncoder(buf)
						encoder.SetEscapeHTML(false)
						err := encoder.Encode(resultsRaw)
						if err == nil {
							var resultsArray []map[string]interface{}
							if err := json.Unmarshal(buf.Bytes(), &resultsArray); err == nil && len(resultsArray) > 0 {
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
						utils.PutJSONBuffer(buf) // Return buffer to pool
					}
				}
			} // Close the resultMap check

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
	result, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, queryToExecute, queryType, false, false)
	if queryErr != nil {
		// Checking if executed query was paginatedQuery, if so, let's try to execute it again with the original query
		if query.Pagination != nil && query.Pagination.PaginatedQuery != nil && *query.Pagination.PaginatedQuery != "" && queryToExecute == strings.Replace(*query.Pagination.PaginatedQuery, "offset_size", strconv.Itoa(0), 1) {
			log.Printf("ChatService -> ExecuteQuery -> query.Pagination.PaginatedQuery was executed but faced an error, will try to execute the original query")
			queryToExecute = query.Query
			result, queryErr = s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, queryToExecute, queryType, false, false)
		}
	}
	var updatedContent *string // tracks content updated by explainErrorWithLLM (for SSE)
	if queryErr != nil {
		log.Printf("ChatService -> ExecuteQuery -> queryErr: %+v", queryErr)
		if queryErr.Code == "FAILED_TO_START_TRANSACTION" || strings.Contains(queryErr.Message, "context deadline exceeded") || strings.Contains(queryErr.Message, "context canceled") {
			return nil, http.StatusRequestTimeout, fmt.Errorf("query execution timed out")
		}

		// Attempt to fix the query using LLM and retry (tool-call style retry).
		// Skip retry for structural / non-retryable errors where fixing the SQL/query text is impossible.
		isNonRetryable := queryErr.Code == "COLLECTION_NOT_FOUND" ||
			queryErr.Code == "TABLE_NOT_FOUND" ||
			strings.Contains(queryErr.Message, "does not exist") ||
			strings.Contains(queryErr.Message, "authentication failed") ||
			strings.Contains(queryErr.Message, "permission denied") ||
			strings.Contains(queryErr.Message, "access denied") ||
			strings.Contains(queryErr.Message, "connection refused")
		if isNonRetryable {
			log.Printf("ChatService -> ExecuteQuery -> Skipping LLM retry for non-retryable error: code=%s msg=%s", queryErr.Code, queryErr.Message)

			// Ask the LLM to generate a user-friendly explanation of the structural error
			// so the user sees a helpful message instead of a raw DB error.
			if chat != nil && s.llmManager != nil {
				s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
					Event: "ai-response-step",
					Data:  "Analyzing the error to provide a clear explanation..",
				})

				explanation, explainErr := s.explainErrorWithLLM(ctx, queryToExecute, queryErr.Message, chat.Connection.Type, query.LLMModel)
				if explainErr == nil && explanation != "" {
					log.Printf("ChatService -> ExecuteQuery -> LLM explanation for structural error: %s", explanation)
					// Update the assistant message content with the friendly explanation
					msg.Content = explanation
					updatedContent = &explanation
				}
			}
		}
		if chat != nil && s.llmManager != nil && !isNonRetryable {
			fixedQuery, retryErr := s.retryQueryWithLLM(ctx, userID, chatID, req.StreamID, queryToExecute, queryErr.Message, chat.Connection.Type, query.LLMModel)
			if retryErr == nil && fixedQuery != "" && fixedQuery != queryToExecute {
				log.Printf("ChatService -> ExecuteQuery -> LLM suggested fixed query: %s", fixedQuery)

				// Send SSE step about retry
				s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
					Event: "ai-response-step",
					Data:  "Query faced error, applying fix and retrying..",
				})

				// Execute the fixed query
				retryResult, retryQueryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, fixedQuery, queryType, false, false)
				if retryQueryErr == nil && retryResult != nil {
					log.Printf("ChatService -> ExecuteQuery -> Retry succeeded with fixed query")

					// Update the query in the message with the fixed version
					if msg.Queries != nil {
						for i := range *msg.Queries {
							if (*msg.Queries)[i].ID.Hex() == query.ID.Hex() {
								(*msg.Queries)[i].Query = fixedQuery
								break
							}
						}
					}

					// Use retry result instead of failing
					result = retryResult
					queryErr = nil

					s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
						Event: "ai-response-step",
						Data:  "Query fix applied successfully!",
					})
				} else {
					log.Printf("ChatService -> ExecuteQuery -> Retry also failed: %+v", retryQueryErr)
				}
			}
		}
	}
	if queryErr != nil {
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
						(*msg.Queries)[i].ActionAt = utils.StringPtr(time.Now().Format(time.RFC3339))
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
			UpdatedContent:    updatedContent,
		}, http.StatusOK, nil
	}
	// Convert Result to JSON string first
	buf := utils.GetJSONBuffer()
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result.Result); err != nil {
		log.Printf("ChatService -> ExecuteQuery -> Error marshalling result: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to marshal result: %v", err)
	}
	resultJSON := buf.Bytes()
	resultJSONStr := buf.String()
	log.Printf("ChatService -> ExecuteQuery -> resultJSON: %+v", resultJSONStr)

	var formattedResultJSON interface{}
	var resultListFormatting []interface{} = []interface{}{}
	var resultMapFormatting map[string]interface{} = map[string]interface{}{}
	if err := json.Unmarshal(resultJSON, &resultListFormatting); err != nil {
		log.Printf("ChatService -> ExecuteQuery -> Error unmarshalling result JSON: %v", err)
		if err := json.Unmarshal(resultJSON, &resultMapFormatting); err != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error unmarshalling result JSON: %v", err)
			// Try to unmarshal as a map
			err = json.Unmarshal(resultJSON, &resultMapFormatting)
			if err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error unmarshalling result JSON: %v", err)
			}
		}
	}

	utils.PutJSONBuffer(buf) // Return buffer to pool

	log.Printf("ChatService -> ExecuteQuery -> resultListFormatting: %+v", resultListFormatting)
	log.Printf("ChatService -> ExecuteQuery -> resultMapFormatting: %+v", resultMapFormatting)
	if len(resultListFormatting) > 0 {
		log.Printf("ChatService -> ExecuteQuery -> resultListFormatting: %+v", resultListFormatting)
		formattedResultJSON = resultListFormatting
		if len(resultListFormatting) > 50 {
			log.Printf("ChatService -> ExecuteQuery -> resultListFormatting length > 50")
			formattedResultJSON = resultListFormatting[:50] // Cap the result to 50 records

			// Cap the result to 50 records
			cappedBuf := utils.GetJSONBuffer()
			encoder := json.NewEncoder(cappedBuf)
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(resultListFormatting[:50]); err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error marshaling capped results: %v", err)
			} else {
				resultJSONStr = cappedBuf.String()
				result.Result = resultListFormatting[:50]
			}
			utils.PutJSONBuffer(cappedBuf)
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
				resultJSONStr = string(cappedResultsJSON)
				result.Result = cappedResults
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
	// Encrypt the execution result before storage
	encryptedResult := s.encryptQueryResult(resultJSONStr)
	query.ExecutionResult = &encryptedResult
	query.ActionAt = utils.StringPtr(time.Now().Format(time.RFC3339))
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
					(*msg.Queries)[i].ActionAt = utils.StringPtr(time.Now().Format(time.RFC3339))
					if totalRecordsCount != nil {
						if (*msg.Queries)[i].Pagination == nil {
							(*msg.Queries)[i].Pagination = &models.Pagination{}
						}
						(*msg.Queries)[i].Pagination.TotalRecordsCount = totalRecordsCount
					}
					log.Printf("ChatService -> ExecuteQuery -> resultJSONStr: %v", resultJSONStr)
					log.Printf("ChatService -> ExecuteQuery -> ExecutionResult before update: %v", (*msg.Queries)[i].ExecutionResult)
					// Encrypt the execution result before storage
					encryptedResult := s.encryptQueryResult(resultJSONStr)
					(*msg.Queries)[i].ExecutionResult = &encryptedResult
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

		var contextBuilder strings.Builder
		contextBuilder.WriteString(fmt.Sprintf("\nQuery id: %s\n", query.ID.Hex())) // This will help LLM to understand the context of the query to be rolled back
		contextBuilder.WriteString(fmt.Sprintf("\nOriginal query: %s\n", query.Query))
		// Convert Result to JSON string
		buf := utils.GetJSONBuffer()
		encoder := json.NewEncoder(buf)
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(dependentResult.Result)
		dependentResultJSONStr := buf.String()
		utils.PutJSONBuffer(buf)
		contextBuilder.WriteString(fmt.Sprintf("Dependent query result: %s\n", dependentResultJSONStr))
		contextBuilder.WriteString("\nPlease generate a rollback query that will undo the effects of the original query.")

		// Get connection info for db type
		conn, exists := s.dbManager.GetConnectionInfo(chatID)
		if !exists {
			return nil, http.StatusBadRequest, fmt.Errorf("no database connection found")
		}

		// Get recent messages and convert to LLM format for context
		recentMessages, _, err := s.chatRepo.FindLatestMessageByChat(msg.ChatID, 10, 1)
		if err != nil {
			log.Printf("ChatService -> RollbackQuery -> Error getting recent messages: %v", err)
			recentMessages = []*models.Message{} // Continue with empty context
		}

		// Convert messages to LLM format
		llmMessages, err := s.convertMessagesToLLMFormat(ctx, chat, recentMessages, conn.Config.Type, "", false)
		if err != nil {
			log.Printf("ChatService -> RollbackQuery -> Error converting messages: %v", err)
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to convert messages: %v", err)
		}

		// Get rollback query from LLM
		llmResponse, err := s.llmClient.GenerateResponse(
			ctx,
			llmMessages,               // Pass the LLM messages array
			conn.Config.Type,          // Pass the database type
			chat.Settings.NonTechMode, // Pass the non-tech mode setting
			query.LLMModel,            // Pass the selected LLM model
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
	query.ActionAt = utils.StringPtr(time.Now().Format(time.RFC3339))
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
				// Convert Result to JSON string
				buf := utils.GetJSONBuffer()
				encoder := json.NewEncoder(buf)
				encoder.SetEscapeHTML(false)
				_ = encoder.Encode(result.Result)
				resultJSONStr := buf.String()
				utils.PutJSONBuffer(buf)
				// Encrypt the execution result before storage
				encryptedResult := s.encryptQueryResult(resultJSONStr)
				(*msg.Queries)[i].ExecutionResult = &encryptedResult
				(*msg.Queries)[i].ActionAt = utils.StringPtr(time.Now().Format(time.RFC3339))
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
				// Get LLM model info from the user message
				var llmModel *string
				var llmModelName *string
				if userMsgObjID, err := primitive.ObjectIDFromHex(messageID); err == nil {
					if userMsg, err := s.chatRepo.FindMessageByID(userMsgObjID); err == nil && userMsg != nil && userMsg.LLMModel != nil {
						llmModel = userMsg.LLMModel
						displayName := s.getModelDisplayName(*userMsg.LLMModel)
						llmModelName = &displayName
					}
				}
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-error",
					Data: map[string]interface{}{
						"error":          "Error: Failed to complete the request, seems like the database connection issue, try reconnecting the database.",
						"llm_model":      llmModel,
						"llm_model_name": llmModelName,
					},
				})
			}
			log.Printf("ProcessLLMResponseAndRunQuery -> activeProcesses: %v", s.activeProcesses)
			s.processesMu.Lock()
			delete(s.activeProcesses, streamID)
			s.processesMu.Unlock()
		}()

		// Get chat settings for auto-visualization
		chatObjID, chatErr := primitive.ObjectIDFromHex(chatID)
		if chatErr != nil {
			log.Printf("ProcessLLMResponseAndRunQuery -> Invalid chat ID: %v", chatErr)
			return
		}
		chat, chatErr := s.chatRepo.FindByID(chatObjID)
		if chatErr != nil {
			log.Printf("ProcessLLMResponseAndRunQuery -> Error fetching chat: %v", chatErr)
			return
		}

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
					Data:  "Combining & structuring the response",
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

						// If ExecuteQuery updated the message content (e.g. via explainErrorWithLLM
						// for non-retryable errors), reflect it in msgResp so the SSE event
						// carries the friendly explanation instead of the original LLM output.
						if executionResult.UpdatedContent != nil {
							msgResp.Content = *executionResult.UpdatedContent
						}

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

						// AUTO-GENERATE VISUALIZATION if enabled and query succeeded
						if chat.Settings.AutoGenerateVisualization && executionResult.Error == nil && executionResult.ExecutionResult != nil {
							log.Printf("ProcessLLMResponseAndRunQuery -> Auto-generating visualization for query: %s", query.ID)

							// Send SSE step update
							s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
								Event: "ai-response-step",
								Data:  "Generating visualization for the result",
							})

							// Generate visualization (synchronous to include in response)
							vizCtx := context.Background()
							// Use the same LLM model as the message
							selectedModel := ""
							if msgResp.LLMModel != nil {
								selectedModel = *msgResp.LLMModel
							}
							visualization, vizErr := s.GenerateVisualizationForMessage(
								vizCtx,
								userID,
								chatID,
								msgResp.ID,
								query.ID,
								selectedModel,
							)

							if vizErr != nil {
								log.Printf("ProcessLLMResponseAndRunQuery -> Error auto-generating visualization: %v", vizErr)
							} else if visualization != nil {
								log.Printf("ProcessLLMResponseAndRunQuery -> Auto-generated visualization: can_visualize=%v, visualization_id=%s", visualization.CanVisualize, visualization.VisualizationID)

								// Construct VisualizationData for the response
								vizData := &dtos.VisualizationData{
									ID:           visualization.VisualizationID,
									CanVisualize: visualization.CanVisualize,
								}
								if visualization.Reason != "" {
									vizData.Reason = &visualization.Reason
								}
								if visualization.Error != "" {
									vizData.Error = &visualization.Error
								}
								if visualization.ChartConfiguration != nil {
									vizData.ChartType = &visualization.ChartConfiguration.ChartType
									vizData.Title = &visualization.ChartConfiguration.Title
									chartConfigJSON, _ := json.Marshal(visualization.ChartConfiguration)
									var chartConfigMap map[string]interface{}
									json.Unmarshal(chartConfigJSON, &chartConfigMap)
									vizData.ChartConfiguration = chartConfigMap
								}

								// Update the query with visualization data for SSE response
								query.Visualization = vizData

								// Update the message in the database with the visualization ID on the query
								// This ensures the visualization persists and is fetched in ListMessages API
								if visualization.VisualizationID != "" {
									msgObjID, _ := primitive.ObjectIDFromHex(msgResp.ID)
									queryObjID, _ := primitive.ObjectIDFromHex(query.ID)
									vizObjID, _ := primitive.ObjectIDFromHex(visualization.VisualizationID)

									// Update the query in the message with visualization ID
									updatedMsg, err := s.chatRepo.FindMessageByID(msgObjID)
									if err == nil && updatedMsg != nil {
										// Find and update the specific query in the message
										for j, q := range *updatedMsg.Queries {
											if q.ID == queryObjID {
												(*updatedMsg.Queries)[j].VisualizationID = &vizObjID
												// Save the updated message back to database
												saveErr := s.chatRepo.UpdateMessage(msgObjID, updatedMsg)
												if saveErr != nil {
													log.Printf("ProcessLLMResponseAndRunQuery -> Error updating message with visualization ID: %v", saveErr)
												} else {
													log.Printf("ProcessLLMResponseAndRunQuery -> Query updated with visualization ID in database")
												}
												break
											}
										}
									}
								}
							}
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

				// Get LLM model info from the user message
				var llmModel *string
				var llmModelName *string
				if userMsgObjID, msgErr := primitive.ObjectIDFromHex(messageID); msgErr == nil {
					if userMsg, msgErr := s.chatRepo.FindMessageByID(userMsgObjID); msgErr == nil && userMsg != nil && userMsg.LLMModel != nil {
						llmModel = userMsg.LLMModel
						displayName := s.getModelDisplayName(*userMsg.LLMModel)
						llmModelName = &displayName
					}
				}

				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-error",
					Data: map[string]interface{}{
						"error":          "Error: " + err.Error(),
						"llm_model":      llmModel,
						"llm_model_name": llmModelName,
					},
				})
			}
		}
	}()

	return nil
}

// RefreshSchema refreshes the schema of the chat & stores the latest schema in the database
// Also refreshed knowledge base descriptions via LLM and vectorizes the schema for better retrieval during question answering,
// this is a synchronous operation and can take time depending on the size of the schema, so it should be called asynchronously from the controller
func (s *chatService) RefreshSchema(ctx context.Context, userID, chatID string, sync bool) (uint32, error) {
	log.Printf("ChatService -> RefreshSchema -> Starting for chatID: %s", chatID)
	// Increase the timeout for the initial context to 60 minutes
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		return http.StatusOK, nil
	default:
		// Check if connection exists, if not create one
		_, exists := s.dbManager.GetConnectionInfo(chatID)
		if !exists {
			log.Printf("ChatService -> RefreshSchema -> Connection not found for chatID: %s, attempting to create connection", chatID)
			status, err := s.ConnectDB(ctx, userID, chatID, "") // Stream Id here is fine and can be empty, it won't affect anything in SSE communication
			if err != nil {
				log.Printf("ChatService -> RefreshSchema -> Failed to create connection: %v", err)
				return status, err
			}
			log.Printf("ChatService -> RefreshSchema -> Connection created successfully for chatID: %s", chatID)
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

			log.Println("ChatService -> RefreshSchema -> Schema refreshed successfully")

			// Save formatted schema to chat.Connection.CurrentSchema so that
			// GetQueryRecommendations (and other consumers) know the schema is ready.
			// Previously this was missing — only HandleSchemaChange saved it, but
			// RefreshSchema races with StartSchemaTracking and often the schema is
			// already stored in Redis before doSchemaCheck runs, so HandleSchemaChange
			// never fires, leaving CurrentSchema nil forever.
			if schemaMsg != "" {
				updateCtx, updateCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := s.chatRepo.UpdateConnectionSchema(updateCtx, chatObjID, schemaMsg); err != nil {
					log.Printf("ChatService -> RefreshSchema -> Warning: Failed to save CurrentSchema: %v", err)
				} else {
					log.Printf("ChatService -> RefreshSchema -> Saved CurrentSchema to chat.Connection (length: %d)", len(schemaMsg))
				}
				updateCancel()
			}

			// Sync knowledge base descriptions via LLM FIRST (auto-generate from schema)
			// so that KB descriptions are available when we build enriched schema chunks.
			if schemaMsg != "" {
				kbCtx, kbCancel := context.WithTimeout(context.Background(), 5*time.Minute)
				s.syncKnowledgeBase(kbCtx, chatID, schemaMsg)
				kbCancel()
			}

			// Vectorize schema synchronously — enriched chunks now include KB descriptions + example records
			if s.vectorizationSvc != nil {
				vecCtx, vecCancel := context.WithTimeout(context.Background(), 5*time.Minute)
				s.vectorizeSchemaForChat(vecCtx, chatID)
				vecCancel()
			}

			// Clear cached recommendations since schema is being refreshed
			if err := s.clearRecommendationsCache(context.Background(), chatID); err != nil {
				log.Printf("ChatService -> RefreshSchema -> Warning: Failed to clear recommendations cache: %v", err)
				// Don't return error as this is not critical to the operation
			}

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

	// Convert Result to JSON string
	buf := utils.GetJSONBuffer()
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(result.Result)
	resultJSONStr := buf.String()
	resultJSONBytes := []byte(resultJSONStr)
	utils.PutJSONBuffer(buf)

	var formattedResultJSON interface{}
	var resultListFormatting []interface{} = []interface{}{}
	var resultMapFormatting map[string]interface{} = map[string]interface{}{}
	if err := json.Unmarshal([]byte(resultJSONStr), &resultListFormatting); err != nil {
		if err := json.Unmarshal([]byte(resultJSONStr), &resultMapFormatting); err != nil {
			log.Printf("ChatService -> GetQueryResults -> Error unmarshalling result JSON: %v", err)
			// Try to unmarshal as a map
			err = json.Unmarshal(resultJSONBytes, &resultMapFormatting)
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

// GetQueryRecommendations generates 4 random query recommendations with Redis caching
func (s *chatService) GetQueryRecommendations(ctx context.Context, userID, chatID string, streamID string) (*dtos.QueryRecommendationsResponse, uint32, error) {
	log.Printf("ChatService -> GetQueryRecommendations -> userID: %s, chatID: %s, streamID: %s", userID, chatID, streamID)

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		log.Printf("ChatService -> GetQueryRecommendations -> Connection not found, creating new connection for chatID: %s", chatID)
		// Create a new connection instead of returning an error
		// Use provided streamID or generate a temporary one
		if streamID == "" {
			streamID = fmt.Sprintf("recommendations-%s", chatID)
		}
		_, err := s.ConnectDB(ctx, userID, chatID, streamID)
		if err != nil {
			log.Printf("ChatService -> GetQueryRecommendations -> Failed to create connection: %v", err)
			return nil, http.StatusBadRequest, fmt.Errorf("failed to connect to database: %v", err)
		}

		// Get connection info again after creating connection
		connInfo, exists = s.dbManager.GetConnectionInfo(chatID)
		if !exists {
			return nil, http.StatusInternalServerError, fmt.Errorf("connection created but not found in manager")
		}
		log.Printf("ChatService -> GetQueryRecommendations -> Successfully created connection for chatID: %s", chatID)
	}

	// Get chat to access settings and context
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	_, err = s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}

	// Try to get cached recommendations first
	cacheKey := fmt.Sprintf("recommendations:%s", chatID)
	cachedData, err := s.redisRepo.GetCompressed(cacheKey, ctx)
	if err == nil && len(cachedData) > 0 {
		log.Printf("ChatService -> GetQueryRecommendations -> Found cached recommendations (compressed)")

		// Parse cached recommendations
		var cachedRecs dtos.CachedQueryRecommendations
		if err := json.Unmarshal(cachedData, &cachedRecs); err == nil {
			// Select 4 random recommendations from cache
			selectedRecs, err := s.selectAndMarkRecommendations(&cachedRecs)
			if err != nil {
				log.Printf("ChatService -> GetQueryRecommendations -> Error selecting from cache: %v", err)
			} else {
				// Update cache with marked recommendations (compressed)
				updatedCacheData, _ := json.Marshal(cachedRecs)
				s.redisRepo.SetCompressed(cacheKey, updatedCacheData, 24*time.Hour, ctx)

				return &dtos.QueryRecommendationsResponse{
					Recommendations: selectedRecs,
				}, http.StatusOK, nil
			}
		}
	}

	log.Printf("ChatService -> GetQueryRecommendations -> No cache found, generating new recommendations")

	// Get chat to access connection and settings
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		log.Printf("ChatService -> GetQueryRecommendations -> Error getting chat: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get chat: %v", err)
	}

	// Check if schema is ready (connection has CurrentSchema).
	// Fallback: if CurrentSchema is empty but a KnowledgeBase exists for this chat,
	// format KB table descriptions into a lightweight schema context. The KB contains
	// the same table/field information that the schema would have.
	schemaContext := ""
	if chat.Connection.CurrentSchema != nil && *chat.Connection.CurrentSchema != "" {
		schemaContext = *chat.Connection.CurrentSchema
	} else if s.kbRepo != nil {
		log.Printf("ChatService -> GetQueryRecommendations -> CurrentSchema not ready, checking knowledge base fallback")
		kb, kbErr := s.kbRepo.FindByChatID(ctx, chatObjID)
		if kbErr == nil && kb != nil && len(kb.TableDescriptions) > 0 {
			// Format KB table descriptions into a schema-like string
			var sb strings.Builder
			sb.WriteString("Database Schema (from Knowledge Base):\n\n")
			for _, td := range kb.TableDescriptions {
				sb.WriteString(fmt.Sprintf("Table: %s\n", td.TableName))
				if td.Description != "" {
					sb.WriteString(fmt.Sprintf("  Description: %s\n", td.Description))
				}
				for _, fd := range td.FieldDescriptions {
					sb.WriteString(fmt.Sprintf("  - %s", fd.FieldName))
					if fd.Description != "" {
						sb.WriteString(fmt.Sprintf(": %s", fd.Description))
					}
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}
			schemaContext = sb.String()
			log.Printf("ChatService -> GetQueryRecommendations -> Using KB fallback as schema context (%d tables, %d chars)", len(kb.TableDescriptions), len(schemaContext))
		} else {
			log.Printf("ChatService -> GetQueryRecommendations -> No CurrentSchema and no KB found, skipping recommendation generation")
			return &dtos.QueryRecommendationsResponse{
				Recommendations: []dtos.QueryRecommendation{},
			}, http.StatusOK, nil
		}
	} else {
		log.Printf("ChatService -> GetQueryRecommendations -> Schema not ready and KB repo unavailable, skipping recommendation generation")
		return &dtos.QueryRecommendationsResponse{
			Recommendations: []dtos.QueryRecommendation{},
		}, http.StatusOK, nil
	}

	// Get recent messages and convert to LLM format for context
	recentMessages, _, err := s.chatRepo.FindLatestMessageByChat(chatObjID, 10, 1)
	if err != nil {
		log.Printf("ChatService -> GetQueryRecommendations -> Error getting messages: %v", err)
		recentMessages = []*models.Message{} // Continue with just schema
	}

	// Convert messages to LLM format (includes schema as system message)
	var llmMessages []*models.LLMMessage

	// Perform RAG search for recommendations (uses last user query or broad search)
	var recoRAGContext string
	if s.vectorizationSvc != nil && s.vectorizationSvc.IsAvailable(ctx) {
		userQuery := ""
		for i := len(recentMessages) - 1; i >= 0; i-- {
			if string(recentMessages[i].Type) == string(constants.MessageTypeUser) {
				userQuery = recentMessages[i].Content
				break
			}
		}
		recoRAGContext, _, _ = s.performRAGSearch(ctx, chatID, userQuery)
	}

	// If CurrentSchema is empty (KB fallback path), inject the KB-derived schemaContext
	// as RAG context so convertMessagesToLLMFormat includes it in the system message.
	useRAGOnlyForReco := false
	if (chat.Connection.CurrentSchema == nil || *chat.Connection.CurrentSchema == "") && schemaContext != "" {
		if recoRAGContext == "" {
			recoRAGContext = schemaContext
		} else {
			recoRAGContext = schemaContext + "\n\n" + recoRAGContext
		}
		useRAGOnlyForReco = true
		log.Printf("ChatService -> GetQueryRecommendations -> Injecting KB-derived schema as RAG context for recommendations")
	}

	llmMessages, err = s.convertMessagesToLLMFormat(ctx, chat, recentMessages, connInfo.Config.Type, recoRAGContext, useRAGOnlyForReco)
	if err != nil {
		log.Printf("ChatService -> GetQueryRecommendations -> Error converting messages: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to convert messages: %v", err)
	}

	log.Printf("ChatService -> GetQueryRecommendations -> Found %d LLM messages, using conversation and schema context", len(llmMessages))

	// Determine which LLM model to use for recommendations
	// Priority: 1) Chat's preferred model, 2) Last assistant message's model, 3) Default model
	selectedLLMModel := ""

	// Check chat's preferred model first
	if chat.PreferredLLMModel != nil && *chat.PreferredLLMModel != "" {
		selectedLLMModel = *chat.PreferredLLMModel
		log.Printf("ChatService -> GetQueryRecommendations -> Using chat's preferred model: %s", selectedLLMModel)
	} else {
		// Fallback to the last assistant message's model (what LLM model was used for the conversation)
		chatMessages, _, err := s.chatRepo.FindLatestMessageByChat(chatObjID, 20, 1) // Get up to 20 recent messages
		if err == nil && len(chatMessages) > 0 {
			// Find the last assistant/AI message
			for _, msg := range chatMessages {
				if msg.Type == string(constants.MessageTypeAssistant) && msg.LLMModel != nil && *msg.LLMModel != "" {
					selectedLLMModel = *msg.LLMModel
					log.Printf("ChatService -> GetQueryRecommendations -> Using last assistant message's model: %s", selectedLLMModel)
					break
				}
			}
		}
	}

	// If still no model selected, use first available model from any provider
	if selectedLLMModel == "" {
		defaultModel := constants.GetFirstAvailableModel()
		if defaultModel != nil {
			selectedLLMModel = defaultModel.ID
			log.Printf("ChatService -> GetQueryRecommendations -> Using first available model: %s (%s)", selectedLLMModel, defaultModel.Provider)
		}
	}

	// Get the correct LLM client based on the selected model's provider
	llmClient := s.llmClient
	if s.llmManager != nil && selectedLLMModel != "" {
		selectedModel := constants.GetLLMModel(selectedLLMModel)
		if selectedModel != nil {
			providerClient, err := s.llmManager.GetClient(selectedModel.Provider)
			if err != nil {
				log.Printf("Warning: Failed to get LLM client for provider '%s': %v, using default client", selectedModel.Provider, err)
			} else {
				llmClient = providerClient
				log.Printf("ChatService -> GetQueryRecommendations -> Using LLM client for provider: %s (model: %s)",
					selectedModel.Provider, selectedModel.DisplayName)
			}
		}
	}

	// Generate recommendations using the selected LLM client and model
	response, err := llmClient.GenerateRecommendations(ctx, llmMessages, connInfo.Config.Type)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to generate recommendations: %v", err)
	}

	log.Printf("ChatService -> GetQueryRecommendations -> LLM response: %s", response)

	// Parse the LLM response
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to parse LLM response: %v", err)
	}

	// Extract recommendations
	var recommendations []dtos.CachedQueryRecommendation
	if recsInterface, ok := jsonResponse["recommendations"]; ok {
		if recsArray, ok := recsInterface.([]interface{}); ok {
			for _, recInterface := range recsArray {
				if recMap, ok := recInterface.(map[string]interface{}); ok {
					if text, ok := recMap["text"].(string); ok && text != "" {
						recommendations = append(recommendations, dtos.CachedQueryRecommendation{
							Text:   text,
							Picked: false,
						})
					}
				}
			}
		}
	}

	// Fallback recommendations if LLM didn't provide proper format
	if len(recommendations) == 0 {
		log.Printf("ChatService -> GetQueryRecommendations -> No valid recommendations from LLM")
		// Add basic fallback recommendations
		fallbackRecommendations := []string{
			"What kind of data do I have in this database?",
			"Show me the main tables and what they contain",
			"How can I explore my data effectively?",
			"What insights can I get from my database?",
		}
		for _, text := range fallbackRecommendations {
			recommendations = append(recommendations, dtos.CachedQueryRecommendation{
				Text:   text,
				Picked: false,
			})
		}
	}

	log.Printf("ChatService -> GetQueryRecommendations -> Generated %d recommendations for caching", len(recommendations))

	// Create cached recommendations structure
	cachedRecommendations := dtos.CachedQueryRecommendations{
		Recommendations: recommendations,
		CreatedAt:       time.Now().Unix(),
	}

	// Cache the recommendations for 3 days (compressed)
	cacheData, _ := json.Marshal(cachedRecommendations)
	if err := s.redisRepo.SetCompressed(cacheKey, cacheData, 3*24*time.Hour, ctx); err != nil {
		log.Printf("ChatService -> GetQueryRecommendations -> Warning: Failed to cache recommendations: %v", err)
	} else {
		log.Printf("ChatService -> GetQueryRecommendations -> Successfully cached %d recommendations (compressed)", len(recommendations))
	}

	// Select 4 random recommendations and mark them as picked
	selectedRecs, err := s.selectAndMarkRecommendations(&cachedRecommendations)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to select recommendations: %v", err)
	}

	// Update cache with marked recommendations (compressed)
	updatedCacheData, _ := json.Marshal(cachedRecommendations)
	s.redisRepo.SetCompressed(cacheKey, updatedCacheData, 3*24*time.Hour, ctx)

	return &dtos.QueryRecommendationsResponse{
		Recommendations: selectedRecs,
	}, http.StatusOK, nil
}

// selectAndMarkRecommendations selects 4 random recommendations and marks them as picked
func (s *chatService) selectAndMarkRecommendations(cachedRecs *dtos.CachedQueryRecommendations) ([]dtos.QueryRecommendation, error) {
	// Check if we have any unpicked recommendations
	unpicked := []int{}
	for i, rec := range cachedRecs.Recommendations {
		if !rec.Picked {
			unpicked = append(unpicked, i)
		}
	}

	// If all are picked, reset all to unpicked and use all
	if len(unpicked) == 0 {
		log.Printf("ChatService -> selectAndMarkRecommendations -> All recommendations picked, resetting")
		for i := range cachedRecs.Recommendations {
			cachedRecs.Recommendations[i].Picked = false
			unpicked = append(unpicked, i)
		}
	}

	// Select up to 4 random recommendations
	selectionCount := 4
	if len(unpicked) < 4 {
		selectionCount = len(unpicked)
	}

	selectedIndices := make([]int, 0, selectionCount)
	for i := 0; i < selectionCount; i++ {
		// Generate cryptographically secure random index
		randomIndex, err := s.secureRandomInt(len(unpicked))
		if err != nil {
			// Fallback to math/rand if crypto/rand fails
			randomIndex = mathrand.Intn(len(unpicked))
		}

		selectedIdx := unpicked[randomIndex]
		selectedIndices = append(selectedIndices, selectedIdx)

		// Remove the selected index from unpicked slice
		unpicked = append(unpicked[:randomIndex], unpicked[randomIndex+1:]...)
	}

	// Mark selected recommendations as picked and prepare response
	var result []dtos.QueryRecommendation
	for _, idx := range selectedIndices {
		cachedRecs.Recommendations[idx].Picked = true
		result = append(result, dtos.QueryRecommendation{
			Text: cachedRecs.Recommendations[idx].Text,
		})
	}

	log.Printf("ChatService -> selectAndMarkRecommendations -> Selected %d recommendations", len(result))
	return result, nil
}

// secureRandomInt generates a cryptographically secure random integer between 0 and max-1
func (s *chatService) secureRandomInt(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be positive")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}

	return int(n.Int64()), nil
}

// clearRecommendationsCache clears the cached query recommendations for a chat
func (s *chatService) clearRecommendationsCache(ctx context.Context, chatID string) error {
	cacheKey := fmt.Sprintf("recommendations:%s", chatID)
	err := s.redisRepo.Del(cacheKey, ctx)
	if err != nil {
		log.Printf("ChatService -> clearRecommendationsCache -> Warning: Failed to clear recommendations cache for chatID %s: %v", chatID, err)
		// Don't return error as this is not critical to the operation
		return nil
	}
	log.Printf("ChatService -> clearRecommendationsCache -> Successfully cleared recommendations cache for chatID %s", chatID)
	return nil
}

// vectorizeSchemaForChat fetches the stored schema and vectorizes it into Qdrant.
// This is called as a background task after schema refresh.
func (s *chatService) vectorizeSchemaForChat(ctx context.Context, chatID string) {
	if s.vectorizationSvc == nil {
		return
	}

	if !s.vectorizationSvc.IsAvailable(ctx) {
		log.Printf("ChatService -> vectorizeSchemaForChat -> Vectorization not available, skipping for chatID: %s", chatID)
		return
	}

	log.Printf("ChatService -> vectorizeSchemaForChat -> Starting enriched schema vectorization for chatID: %s", chatID)

	// 1. Get the stored schema (FullSchema for structure)
	schemaInfo, err := s.dbManager.GetSchemaManager().GetStoredSchemaInfo(ctx, chatID)
	if err != nil {
		log.Printf("ChatService -> vectorizeSchemaForChat -> Error getting stored schema: %v", err)
		return
	}

	// 2. Get connection info for db_type
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	dbType := "unknown"
	if exists {
		dbType = connInfo.Config.Type
	}

	// 3. Get SchemaStorage for example records from LLMSchema
	schemaStorage, err := s.dbManager.GetSchemaManager().GetStoredSchemaStorage(ctx, chatID)
	var examplesByTable map[string][]map[string]interface{}
	if err == nil && schemaStorage != nil && schemaStorage.LLMSchema != nil {
		examplesByTable = make(map[string][]map[string]interface{}, len(schemaStorage.LLMSchema.Tables))
		for tName, tInfo := range schemaStorage.LLMSchema.Tables {
			if len(tInfo.ExampleRecords) > 0 {
				examplesByTable[tName] = tInfo.ExampleRecords
			}
		}
	} else if err != nil {
		log.Printf("ChatService -> vectorizeSchemaForChat -> Could not fetch schema storage for examples: %v (continuing without examples)", err)
	}

	// 4. Get KB descriptions from MongoDB (best-effort — nil if unavailable)
	var kbDescsByTable map[string]*models.TableDescription
	if s.kbRepo != nil {
		chatObjID, parseErr := primitive.ObjectIDFromHex(chatID)
		if parseErr == nil {
			kb, kbErr := s.kbRepo.FindByChatID(ctx, chatObjID)
			if kbErr == nil && kb != nil {
				kbDescsByTable = make(map[string]*models.TableDescription, len(kb.TableDescriptions))
				for i := range kb.TableDescriptions {
					td := &kb.TableDescriptions[i]
					if td.Description != "" || len(td.FieldDescriptions) > 0 {
						kbDescsByTable[td.TableName] = td
					}
				}
			} else if kbErr != nil {
				log.Printf("ChatService -> vectorizeSchemaForChat -> Could not fetch KB: %v (continuing without KB descriptions)", kbErr)
			}
		}
	}

	// 5. Build enrichment map combining KB descriptions + example records
	enrichments := make(map[string]*TableEnrichment)
	allTableNames := make(map[string]bool)
	for t := range schemaInfo.Tables {
		allTableNames[t] = true
	}
	for t := range kbDescsByTable {
		allTableNames[t] = true
	}
	for t := range examplesByTable {
		allTableNames[t] = true
	}

	for tName := range allTableNames {
		enrichment := &TableEnrichment{
			FieldDescriptions: make(map[string]string),
		}
		hasData := false

		// KB descriptions
		if td, ok := kbDescsByTable[tName]; ok {
			enrichment.TableDescription = td.Description
			for _, fd := range td.FieldDescriptions {
				if fd.Description != "" {
					enrichment.FieldDescriptions[fd.FieldName] = fd.Description
				}
			}
			hasData = true
		}

		// Example records
		if examples, ok := examplesByTable[tName]; ok && len(examples) > 0 {
			enrichment.ExampleRecords = examples
			hasData = true
		}

		if hasData {
			enrichments[tName] = enrichment
		}
	}

	// 6. Convert SchemaInfo to SchemaTable map (same as before)
	tables := make(map[string]SchemaTable)
	for tableName, ts := range schemaInfo.Tables {
		columns := make([]SchemaColumn, 0, len(ts.Columns))
		for colName, col := range ts.Columns {
			isPK := false
			for _, c := range ts.Constraints {
				if c.Type == "PRIMARY KEY" {
					for _, cCol := range c.Columns {
						if cCol == colName {
							isPK = true
							break
						}
					}
				}
			}
			columns = append(columns, SchemaColumn{
				Name:         colName,
				Type:         col.Type,
				IsNullable:   col.IsNullable,
				IsPrimaryKey: isPK,
				DefaultValue: col.DefaultValue,
				Comment:      col.Comment,
			})
		}

		indexes := make([]SchemaIndex, 0, len(ts.Indexes))
		for _, idx := range ts.Indexes {
			indexes = append(indexes, SchemaIndex{
				Name:     idx.Name,
				Columns:  idx.Columns,
				IsUnique: idx.IsUnique,
			})
		}

		fks := make([]SchemaFK, 0, len(ts.ForeignKeys))
		for _, fk := range ts.ForeignKeys {
			fks = append(fks, SchemaFK{
				ColumnName: fk.ColumnName,
				RefTable:   fk.RefTable,
				RefColumn:  fk.RefColumn,
			})
		}

		tables[tableName] = SchemaTable{
			Name:        tableName,
			Comment:     ts.Comment,
			RowCount:    ts.RowCount,
			Columns:     columns,
			Indexes:     indexes,
			ForeignKeys: fks,
		}
	}

	// 7. Build enriched chunks and vectorize
	chunks := BuildEnrichedSchemaChunks(tables, dbType, enrichments)
	if len(chunks) == 0 {
		log.Printf("ChatService -> vectorizeSchemaForChat -> No schema chunks to vectorize for chatID: %s", chatID)
		return
	}

	if err := s.vectorizationSvc.VectorizeSchema(ctx, chatID, chunks); err != nil {
		log.Printf("ChatService -> vectorizeSchemaForChat -> Error vectorizing schema: %v", err)
		return
	}

	enrichedCount := len(enrichments)
	log.Printf("ChatService -> vectorizeSchemaForChat -> Enriched schema vectorization completed for chatID: %s (%d tables, %d enriched with KB/examples)", chatID, len(chunks), enrichedCount)
}

// retryQueryWithLLM sends the failed query and its error to the LLM, asking it to produce a corrected query.
// It uses the same GenerateResponse flow as normal chat — including the DB-specific system prompt
// and structured JSON response schema — so that the corrected query comes back in the standard
// queries[].query format. The prompt template lives in constants/query_retry.go for reusability.
func (s *chatService) retryQueryWithLLM(ctx context.Context,
	userID, chatID, streamID, failedQuery, errorMessage, dbType, llmModelID string) (string, error) {
	log.Printf("ChatService -> retryQueryWithLLM -> Attempting LLM-based fix for query error. dbType=%s, model=%s", dbType, llmModelID)
	log.Printf("ChatService -> retryQueryWithLLM -> Failed query: %s", failedQuery)
	log.Printf("ChatService -> retryQueryWithLLM -> Error: %s", errorMessage)

	// Resolve the LLM client for the model's provider
	llmClient := s.llmClient // fallback to default
	if llmModelID != "" && s.llmManager != nil {
		selectedModel := constants.GetLLMModel(llmModelID)
		if selectedModel != nil {
			providerClient, err := s.llmManager.GetClient(selectedModel.Provider)
			if err != nil {
				log.Printf("ChatService -> retryQueryWithLLM -> Failed to get provider client for '%s': %v, using default", selectedModel.Provider, err)
			} else {
				llmClient = providerClient
			}
		}
	}

	if llmClient == nil {
		return "", fmt.Errorf("no LLM client available for retry")
	}

	// Build a user message using the standard LLMMessage format.
	// GenerateResponse will automatically prepend the DB-specific system prompt
	// and enforce the structured JSON response schema — exactly like a normal chat call.
	retryPrompt := constants.GetQueryRetryPrompt(dbType, failedQuery, errorMessage)

	messages := []*models.LLMMessage{
		{
			Role: "user",
			Content: map[string]interface{}{
				"user_message": retryPrompt,
			},
		},
	}

	// Call the LLM using the same flow as processLLMResponse
	response, err := llmClient.GenerateResponse(ctx, messages, dbType, false, llmModelID)
	if err != nil {
		log.Printf("ChatService -> retryQueryWithLLM -> LLM call failed: %v", err)
		return "", fmt.Errorf("LLM retry call failed: %v", err)
	}

	log.Printf("ChatService -> retryQueryWithLLM -> Raw LLM response: %s", response)

	// Parse the structured JSON response — same schema as normal LLM responses
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
		log.Printf("ChatService -> retryQueryWithLLM -> Failed to parse JSON response: %v", err)
		return "", fmt.Errorf("failed to parse LLM retry response: %v", err)
	}

	// Extract corrected query from queries[0].query
	if queriesRaw, ok := jsonResponse["queries"]; ok {
		if queriesArr, ok := queriesRaw.([]interface{}); ok && len(queriesArr) > 0 {
			if queryMap, ok := queriesArr[0].(map[string]interface{}); ok {
				if queryStr, ok := queryMap["query"].(string); ok && queryStr != "" {
					fixedQuery := strings.TrimSpace(queryStr)
					log.Printf("ChatService -> retryQueryWithLLM -> Extracted fixed query from structured response: %s", fixedQuery)
					return fixedQuery, nil
				}
			}
		}
	}

	log.Printf("ChatService -> retryQueryWithLLM -> No corrected query found in LLM response")
	return "", fmt.Errorf("LLM response did not contain a corrected query")
}

// explainErrorWithLLM asks the LLM to generate a user-friendly explanation for a non-retryable
// structural error (e.g., table doesn't exist, permission denied). Returns the assistantMessage
// text from the LLM response. This is used to replace the raw error with a helpful user message.
func (s *chatService) explainErrorWithLLM(ctx context.Context,
	failedQuery, errorMessage, dbType, llmModelID string) (string, error) {
	log.Printf("ChatService -> explainErrorWithLLM -> Generating friendly explanation. dbType=%s, model=%s", dbType, llmModelID)

	// Resolve the LLM client
	llmClient := s.llmClient
	if llmModelID != "" && s.llmManager != nil {
		selectedModel := constants.GetLLMModel(llmModelID)
		if selectedModel != nil {
			if providerClient, err := s.llmManager.GetClient(selectedModel.Provider); err == nil {
				llmClient = providerClient
			}
		}
	}

	if llmClient == nil {
		return "", fmt.Errorf("no LLM client available for error explanation")
	}

	prompt := constants.GetStructuralErrorPrompt(dbType, failedQuery, errorMessage)

	messages := []*models.LLMMessage{
		{
			Role: "user",
			Content: map[string]interface{}{
				"user_message": prompt,
			},
		},
	}

	response, err := llmClient.GenerateResponse(ctx, messages, dbType, false, llmModelID)
	if err != nil {
		log.Printf("ChatService -> explainErrorWithLLM -> LLM call failed: %v", err)
		return "", err
	}

	log.Printf("ChatService -> explainErrorWithLLM -> Raw LLM response: %s", response)

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
		return "", fmt.Errorf("failed to parse error explanation response: %v", err)
	}

	if msg, ok := jsonResponse["assistantMessage"].(string); ok && msg != "" {
		return msg, nil
	}

	return "", fmt.Errorf("no assistantMessage found in error explanation response")
}
