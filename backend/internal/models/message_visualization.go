package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MessageVisualization stores the visualization configuration for a message/query
// Each Query can have its own MessageVisualization record for independent visualization management
// Relationship: Query.VisualizationID → MessageVisualization.ID
type MessageVisualization struct {
	ID                 primitive.ObjectID  `bson:"_id" json:"id"`
	MessageID          primitive.ObjectID  `bson:"message_id" json:"message_id"`
	QueryID            *primitive.ObjectID `bson:"query_id,omitempty" json:"query_id,omitempty"` // Reference to Query for per-query visualization (enables 1:1 query-visualization relationship)
	ChatID             primitive.ObjectID  `bson:"chat_id" json:"chat_id"`
	UserID             primitive.ObjectID  `bson:"user_id" json:"user_id"`
	CanVisualize       bool                `bson:"can_visualize" json:"can_visualize"`
	Reason             string              `bson:"reason,omitempty" json:"reason,omitempty"`
	ChartType          string              `bson:"chart_type,omitempty" json:"chart_type,omitempty"`
	Title              string              `bson:"title,omitempty" json:"title,omitempty"`
	Description        string              `bson:"description,omitempty" json:"description,omitempty"`
	ChartConfigJSON    string              `bson:"chart_config_json,omitempty" json:"chart_config_json,omitempty"` // Full JSON config stored as string
	OptimizedQuery     string              `bson:"optimized_query,omitempty" json:"optimized_query,omitempty"`
	QueryStrategy      string              `bson:"query_strategy,omitempty" json:"query_strategy,omitempty"`
	DataTransformation string              `bson:"data_transformation,omitempty" json:"data_transformation,omitempty"`
	ProjectedRowCount  int                 `bson:"projected_row_count,omitempty" json:"projected_row_count,omitempty"`
	ChartHeight        int                 `bson:"chart_height,omitempty" json:"chart_height,omitempty"`
	ColorScheme        string              `bson:"color_scheme,omitempty" json:"color_scheme,omitempty"`
	DataDensity        string              `bson:"data_density,omitempty" json:"data_density,omitempty"`
	XAxisLabel         string              `bson:"x_axis_label,omitempty" json:"x_axis_label,omitempty"`
	YAxisLabel         string              `bson:"y_axis_label,omitempty" json:"y_axis_label,omitempty"`
	GeneratedBy        string              `bson:"generated_by,omitempty" json:"generated_by,omitempty"` // LLM model used
	Error              string              `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt          time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt          time.Time           `bson:"updated_at" json:"updated_at"`
}

// NewMessageVisualization creates a new MessageVisualization instance
// If queryID is provided, the visualization will be linked to a specific query (per-query visualization)
// If queryID is nil, the visualization will be linked to the entire message (for backward compatibility)
// messageID can be nil if visualization is query-specific without message context
func NewMessageVisualization(
	messageID *primitive.ObjectID,
	chatID, userID primitive.ObjectID,
	queryID *primitive.ObjectID,
) *MessageVisualization {
	now := time.Now()
	msgID := primitive.NilObjectID
	if messageID != nil {
		msgID = *messageID
	}
	return &MessageVisualization{
		ID:        primitive.NewObjectID(),
		MessageID: msgID,
		QueryID:   queryID,
		ChatID:    chatID,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
