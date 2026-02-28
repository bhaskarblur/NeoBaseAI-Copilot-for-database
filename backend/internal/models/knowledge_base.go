package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FieldDescription stores the user-provided (or AI-generated) description for a single field/column.
type FieldDescription struct {
	FieldName   string `bson:"field_name" json:"field_name"`
	Description string `bson:"description" json:"description"`
}

// TableDescription stores the description for a table/collection and its fields.
type TableDescription struct {
	TableName         string             `bson:"table_name" json:"table_name"`
	Description       string             `bson:"description" json:"description"`
	FieldDescriptions []FieldDescription `bson:"field_descriptions" json:"field_descriptions"`
}

// KnowledgeBase stores the per-chat knowledge base consisting of table and field descriptions.
// One document per chat in the `knowledge_bases` MongoDB collection.
type KnowledgeBase struct {
	ChatID            primitive.ObjectID `bson:"chat_id" json:"chat_id"`
	UserID            primitive.ObjectID `bson:"user_id" json:"user_id"`
	TableDescriptions []TableDescription `bson:"table_descriptions" json:"table_descriptions"`
	IsVectorized      bool               `bson:"is_vectorized" json:"is_vectorized"`
	VectorizedAt      *time.Time         `bson:"vectorized_at,omitempty" json:"vectorized_at,omitempty"`
	Base              `bson:",inline"`
}

// NewKnowledgeBase creates a new KnowledgeBase instance for a chat.
func NewKnowledgeBase(chatID primitive.ObjectID) *KnowledgeBase {
	return &KnowledgeBase{
		ChatID:            chatID,
		TableDescriptions: []TableDescription{},
		Base:              NewBase(),
	}
}

// GetTableDescription returns the description for a specific table, or nil if not found.
func (kb *KnowledgeBase) GetTableDescription(tableName string) *TableDescription {
	for i := range kb.TableDescriptions {
		if kb.TableDescriptions[i].TableName == tableName {
			return &kb.TableDescriptions[i]
		}
	}
	return nil
}

// GetTablesWithDescriptions returns only tables that have a non-empty description or at least one field with a description.
func (kb *KnowledgeBase) GetTablesWithDescriptions() []TableDescription {
	result := make([]TableDescription, 0)
	for _, td := range kb.TableDescriptions {
		if td.Description != "" {
			result = append(result, td)
			continue
		}
		for _, fd := range td.FieldDescriptions {
			if fd.Description != "" {
				result = append(result, td)
				break
			}
		}
	}
	return result
}
