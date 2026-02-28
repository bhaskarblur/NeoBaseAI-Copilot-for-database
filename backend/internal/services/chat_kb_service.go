package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetKnowledgeBase retrieves the knowledge base for a chat.
func (s *chatService) GetKnowledgeBase(ctx context.Context, userID, chatID string) (*models.KnowledgeBase, uint32, error) {
	log.Printf("ChatService -> GetKnowledgeBase -> chatID: %s", chatID)

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID")
	}

	// Verify user owns the chat
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil || chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil || chat.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized")
	}

	if s.kbRepo == nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("knowledge base not available")
	}

	kb, err := s.kbRepo.FindByChatID(ctx, chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch knowledge base: %v", err)
	}

	// Return empty KB if none exists yet
	if kb == nil {
		kb = models.NewKnowledgeBase(chatObjID)
	}

	return kb, http.StatusOK, nil
}

// UpdateKnowledgeBase saves the knowledge base and triggers vectorization.
func (s *chatService) UpdateKnowledgeBase(ctx context.Context, userID, chatID string, tableDescs []models.TableDescription) (*models.KnowledgeBase, uint32, error) {
	log.Printf("ChatService -> UpdateKnowledgeBase -> chatID: %s, tables: %d", chatID, len(tableDescs))

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID")
	}

	// Verify user owns the chat
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil || chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil || chat.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized")
	}

	if s.kbRepo == nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("knowledge base not available")
	}

	// Get or create KB
	kb, err := s.kbRepo.FindByChatID(ctx, chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch knowledge base: %v", err)
	}
	if kb == nil {
		kb = models.NewKnowledgeBase(chatObjID)
	}

	// Set user ID and update table descriptions
	kb.UserID = userObjID
	kb.TableDescriptions = tableDescs

	// Save to MongoDB
	if err := s.kbRepo.Upsert(ctx, kb); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to save knowledge base: %v", err)
	}

	log.Printf("ChatService -> UpdateKnowledgeBase -> Saved KB with %d table descriptions", len(tableDescs))

	// Re-vectorize schema in background — enriched chunks now include KB descriptions
	go func() {
		vecCtx, vecCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer vecCancel()
		s.vectorizeSchemaForChat(vecCtx, chatID)
	}()

	return kb, http.StatusOK, nil
}

// syncKnowledgeBase generates/updates the knowledge base descriptions using the LLM
// and the formatted schema (with examples). Called from RefreshSchema and HandleSchemaChange.
// This is a background operation — errors are logged but don't block the caller.
func (s *chatService) syncKnowledgeBase(ctx context.Context, chatID string, formattedSchema string) {
	if s.kbRepo == nil {
		return
	}

	log.Printf("ChatService -> syncKnowledgeBase -> Starting KB sync for chatID: %s", chatID)

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		log.Printf("ChatService -> syncKnowledgeBase -> Invalid chatID: %v", err)
		return
	}

	// Get the chat to obtain userID
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil || chat == nil {
		log.Printf("ChatService -> syncKnowledgeBase -> Chat not found: %v", err)
		return
	}

	// Generate descriptions via LLM
	descriptions, err := s.generateKBDescriptionsViaLLM(ctx, formattedSchema, chat.PreferredLLMModel)
	if err != nil {
		log.Printf("ChatService -> syncKnowledgeBase -> LLM description generation failed: %v", err)
		return
	}

	if len(descriptions) == 0 {
		log.Printf("ChatService -> syncKnowledgeBase -> LLM returned no descriptions, skipping")
		return
	}

	// Get or create KB
	kb, err := s.kbRepo.FindByChatID(ctx, chatObjID)
	if err != nil {
		log.Printf("ChatService -> syncKnowledgeBase -> Error fetching KB: %v", err)
		return
	}
	if kb == nil {
		kb = models.NewKnowledgeBase(chatObjID)
	}
	kb.UserID = chat.UserID

	// Merge: keep any user-edited descriptions, fill in LLM-generated ones for empty fields
	existingMap := make(map[string]*models.TableDescription)
	for i := range kb.TableDescriptions {
		existingMap[kb.TableDescriptions[i].TableName] = &kb.TableDescriptions[i]
	}

	for _, genTD := range descriptions {
		existing, has := existingMap[genTD.TableName]
		if !has {
			// New table — use LLM description as-is
			kb.TableDescriptions = append(kb.TableDescriptions, genTD)
			continue
		}
		// Table exists — only fill in empty descriptions (don't overwrite user edits)
		if existing.Description == "" {
			existing.Description = genTD.Description
		}
		// Merge field descriptions
		existingFieldMap := make(map[string]*models.FieldDescription)
		for j := range existing.FieldDescriptions {
			existingFieldMap[existing.FieldDescriptions[j].FieldName] = &existing.FieldDescriptions[j]
		}
		for _, genFD := range genTD.FieldDescriptions {
			if existingFD, ok := existingFieldMap[genFD.FieldName]; ok {
				if existingFD.Description == "" {
					existingFD.Description = genFD.Description
				}
			} else {
				existing.FieldDescriptions = append(existing.FieldDescriptions, genFD)
			}
		}
	}

	// Save to MongoDB
	if err := s.kbRepo.Upsert(ctx, kb); err != nil {
		log.Printf("ChatService -> syncKnowledgeBase -> Error saving KB: %v", err)
		return
	}

	log.Printf("ChatService -> syncKnowledgeBase -> KB sync completed for chatID: %s (%d table descriptions saved)", chatID, len(kb.TableDescriptions))
}

// generateKBDescriptionsViaLLM calls the LLM with the formatted schema to generate
// table and field descriptions for the knowledge base.
func (s *chatService) generateKBDescriptionsViaLLM(ctx context.Context, formattedSchema string, preferredModel *string) ([]models.TableDescription, error) {
	// Resolve LLM client
	llmClient := s.llmClient
	modelID := ""
	if preferredModel != nil && *preferredModel != "" {
		modelID = *preferredModel
		if s.llmManager != nil {
			selectedModel := constants.GetLLMModel(modelID)
			if selectedModel != nil {
				if providerClient, err := s.llmManager.GetClient(selectedModel.Provider); err == nil {
					llmClient = providerClient
				}
			}
		}
	}

	if llmClient == nil {
		return nil, fmt.Errorf("no LLM client available")
	}

	// Build messages: system prompt + schema as user message
	messages := []*models.LLMMessage{
		{
			Role: "user",
			Content: map[string]interface{}{
				"user_message": constants.KBDescriptionGenerationPrompt + "\n\nHere is the database schema:\n\n" + formattedSchema,
			},
		},
	}

	// Use GenerateResponse (single-shot, not tool-calling) — this is a simple generation task
	// We use "postgresql" as dbType since we want raw JSON back, not DB-specific queries
	response, err := llmClient.GenerateResponse(ctx, messages, "postgresql", false, modelID)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %v", err)
	}

	log.Printf("ChatService -> generateKBDescriptionsViaLLM -> Raw response length: %d", len(response))

	// The response is wrapped in the LLMResponse schema with assistantMessage.
	// Try to extract the JSON from assistantMessage first
	var llmResp map[string]interface{}
	if err := json.Unmarshal([]byte(response), &llmResp); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %v", err)
	}

	// The KB JSON might be in assistantMessage or directly in the response
	var kbJSON string
	if msg, ok := llmResp["assistantMessage"].(string); ok && msg != "" {
		kbJSON = msg
	} else {
		// Try the raw response
		kbJSON = response
	}

	// Try to extract JSON from the text (may contain markdown code blocks)
	kbJSON = extractJSONFromText(kbJSON)

	// Parse the KB descriptions
	var kbResp struct {
		Tables []struct {
			TableName         string `json:"table_name"`
			Description       string `json:"description"`
			FieldDescriptions []struct {
				FieldName   string `json:"field_name"`
				Description string `json:"description"`
			} `json:"field_descriptions"`
		} `json:"tables"`
	}

	if err := json.Unmarshal([]byte(kbJSON), &kbResp); err != nil {
		return nil, fmt.Errorf("failed to parse KB descriptions JSON: %v", err)
	}

	// Convert to models
	result := make([]models.TableDescription, 0, len(kbResp.Tables))
	for _, t := range kbResp.Tables {
		td := models.TableDescription{
			TableName:   t.TableName,
			Description: t.Description,
		}
		for _, f := range t.FieldDescriptions {
			td.FieldDescriptions = append(td.FieldDescriptions, models.FieldDescription{
				FieldName:   f.FieldName,
				Description: f.Description,
			})
		}
		result = append(result, td)
	}

	log.Printf("ChatService -> generateKBDescriptionsViaLLM -> Generated descriptions for %d tables", len(result))
	return result, nil
}

// extractJSONFromText tries to extract a JSON object from text that may contain
// markdown code blocks or other wrapping.
func extractJSONFromText(text string) string {
	// Try as-is first
	text = strings.TrimSpace(text)
	if len(text) > 0 && text[0] == '{' {
		return text
	}

	// Look for ```json ... ``` blocks
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + 7
		if end := strings.Index(text[start:], "```"); end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Look for ``` ... ``` blocks
	if idx := strings.Index(text, "```"); idx >= 0 {
		start := idx + 3
		// Skip to the next line
		for start < len(text) && text[start] != '\n' {
			start++
		}
		if start < len(text) {
			start++
		}
		if end := strings.Index(text[start:], "```"); end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Look for first { and try to extract balanced JSON
	if idx := strings.Index(text, "{"); idx >= 0 {
		return text[idx:]
	}

	return text
}
