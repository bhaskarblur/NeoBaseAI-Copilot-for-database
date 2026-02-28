package dtos

import "neobase-ai/internal/models"

// UpdateKnowledgeBaseRequest is the request body for updating KB descriptions.
type UpdateKnowledgeBaseRequest struct {
	TableDescriptions []models.TableDescription `json:"table_descriptions" binding:"required"`
}

// KnowledgeBaseResponse is the response for KB endpoints.
type KnowledgeBaseResponse struct {
	ChatID            string                    `json:"chat_id"`
	TableDescriptions []models.TableDescription `json:"table_descriptions"`
	CreatedAt         string                    `json:"created_at"`
	UpdatedAt         string                    `json:"updated_at"`
}
