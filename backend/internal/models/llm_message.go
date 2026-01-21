package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LLMMessage is used ONLY for structuring messages sent to/from LLM.
// It is NOT stored in the database - messages are converted on-the-fly from regular Message models.
// Before: Schema was stored in LLM messages and LLM messages were sent to LLM API.
// After: Schema is stored in Chat model and LLM messages are converted on-the-fly when needed.
type LLMMessage struct {
	ChatID      primitive.ObjectID     `json:"chat_id"`
	MessageID   primitive.ObjectID     `json:"message_id"` // References the original Message.ID
	UserID      primitive.ObjectID     `json:"user_id"`
	Role        string                 `json:"role"`          // "user", "assistant", or "system"
	Content     map[string]interface{} `json:"content"`       // user_message, assistant_response, or schema_update
	IsEdited    bool                   `json:"is_edited"`     // Whether message content has been edited
	NonTechMode bool                   `json:"non_tech_mode"` // Whether generated in non-tech mode
	CreatedAt   time.Time              `json:"created_at"`    // Timestamp for ordering
	UpdatedAt   time.Time              `json:"updated_at"`    // Last update timestamp
}
