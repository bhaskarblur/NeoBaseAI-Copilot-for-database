package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// Message exchanged with the LLM
type LLMMessage struct {
	ChatID      primitive.ObjectID     `bson:"chat_id" json:"chat_id"`
	MessageID   primitive.ObjectID     `bson:"message_id" json:"message_id"` // ID of the original message
	UserID      primitive.ObjectID     `bson:"user_id" json:"user_id"`
	Role        string                 `bson:"role" json:"role"`
	Content     map[string]interface{} `bson:"content" json:"content"`       // Can include user_message, assistant_response (with queries and action_buttons), schema_update
	IsEdited    bool                   `bson:"is_edited" json:"is_edited"`   // if the message content has been edited
	NonTechMode bool                   `bson:"non_tech_mode" json:"non_tech_mode"` // Whether this message was generated/sent in non-tech mode
	Base        `bson:",inline"`
}
