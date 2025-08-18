package models

import (
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	UserID        primitive.ObjectID  `bson:"user_id" json:"user_id"`
	ChatID        primitive.ObjectID  `bson:"chat_id" json:"chat_id"`
	UserMessageId *primitive.ObjectID `bson:"user_message_id,omitempty" json:"user_message_id,omitempty"` // Holds id of user message that was sent before this message, only applicable for Type assistant
	Type          string              `bson:"type" json:"type"`                                           // 'user' or 'assistant'
	Content       string              `bson:"content" json:"content"`
	IsEdited      bool                `bson:"is_edited" json:"is_edited"` // if the message content has been edited, only for user messages
	Queries       *[]Query            `bson:"queries,omitempty" json:"queries,omitempty"`
	ActionButtons *[]ActionButton     `bson:"action_buttons,omitempty" json:"action_buttons,omitempty"` // UI action buttons suggested by the LLM
	NonTechMode   bool                `bson:"non_tech_mode" json:"non_tech_mode"`                       // Whether this message was generated in non-tech mode
	IsPinned      bool                `bson:"is_pinned" json:"is_pinned"`                               // Whether this message is pinned
	PinnedAt      *time.Time          `bson:"pinned_at,omitempty" json:"pinned_at,omitempty"`           // When the message was pinned
	Base          `bson:",inline"`
}

// ActionButton represents a UI action button that can be suggested by the LLM
type ActionButton struct {
	ID        primitive.ObjectID `bson:"id" json:"id"`
	Label     string             `bson:"label" json:"label"`          // Display text for the button
	Action    string             `bson:"action" json:"action"`        // Action identifier (e.g., "refresh_schema", "show_tables")
	IsPrimary bool               `bson:"is_primary" json:"isPrimary"` // Whether this is a primary (highlighted) action
}

type Query struct {
	ID                     primitive.ObjectID `bson:"id" json:"id"`
	Query                  string             `bson:"query" json:"query"`
	QueryType              *string            `bson:"query_type" json:"query_type"` // SELECT, INSERT, UPDATE, DELETE...
	Pagination             *Pagination        `bson:"pagination,omitempty" json:"pagination,omitempty"`
	Tables                 *string            `bson:"tables" json:"tables"` // comma separated table names involved in the query
	Description            string             `bson:"description" json:"description"`
	RollbackDependentQuery *string            `bson:"rollback_dependent_query,omitempty" json:"rollback_dependent_query,omitempty"` // ID of the query that this query depends on
	RollbackQuery          *string            `bson:"rollback_query,omitempty" json:"rollback_query,omitempty"`                     // the query to rollback the query
	ExecutionTime          *int               `bson:"execution_time" json:"execution_time"`                                         // in milliseconds, same for execution & rollback query
	ExampleExecutionTime   int                `bson:"example_execution_time" json:"example_execution_time"`                         // in milliseconds
	CanRollback            bool               `bson:"can_rollback" json:"can_rollback"`
	IsCritical             bool               `bson:"is_critical" json:"is_critical"`
	IsExecuted             bool               `bson:"is_executed" json:"is_executed"`       // if the query has been executed
	IsRolledBack           bool               `bson:"is_rolled_back" json:"is_rolled_back"` // if the query has been rolled back
	Error                  *QueryError        `bson:"error,omitempty" json:"error,omitempty"`
	ExampleResult          *string            `bson:"example_result,omitempty" json:"example_result,omitempty"`     // JSON string
	ExecutionResult        *string            `bson:"execution_result,omitempty" json:"execution_result,omitempty"` // JSON string
	IsEdited               bool               `bson:"is_edited" json:"is_edited"`                                   // if the query has been edited
	Metadata               *string            `bson:"metadata,omitempty" json:"metadata,omitempty"`                 // JSON string for database-specific metadata (e.g., ClickHouse engine type)
	ActionAt               *string            `bson:"action_at,omitempty" json:"action_at,omitempty"`               // The timestamp when the action was taken
}

type QueryError struct {
	Code    string `bson:"code" json:"code"`
	Message string `bson:"message" json:"message"`
	Details string `bson:"details" json:"details"`
}

type Pagination struct {
	TotalRecordsCount *int    `bson:"total_records_count" json:"total_records_count"`
	PaginatedQuery    *string `bson:"paginated_query" json:"paginated_query"`
	CountQuery        *string `bson:"count_query" json:"count_query"`
}

func NewMessage(userID, chatID primitive.ObjectID, msgType, content string, queries *[]Query, userMessageId *primitive.ObjectID) *Message {
	log.Printf("NewMessage -> queries: %v", queries)
	return &Message{
		UserID:        userID,
		ChatID:        chatID,
		Type:          msgType,
		UserMessageId: userMessageId,
		Content:       content,
		IsEdited:      false,
		Queries:       queries,
		Base:          NewBase(),
	}
}

// NewMessageWithActionButtons creates a new message with action buttons
func NewMessageWithActionButtons(userID, chatID primitive.ObjectID, msgType, content string, queries *[]Query, actionButtons *[]ActionButton, userMessageId *primitive.ObjectID) *Message {
	log.Printf("NewMessageWithActionButtons -> queries: %v, actionButtons: %v", queries, actionButtons)
	return &Message{
		UserID:        userID,
		ChatID:        chatID,
		Type:          msgType,
		UserMessageId: userMessageId,
		Content:       content,
		IsEdited:      false,
		Queries:       queries,
		ActionButtons: actionButtons,
		Base:          NewBase(),
	}
}
