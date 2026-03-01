package llm

import (
	"context"
	"neobase-ai/internal/models"
)

// Message represents a chat message
type Message struct {
	Role    string                 `json:"role"`
	Content string                 `json:"content"`
	Type    string                 `json:"type,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// Client defines the interface for LLM interactions
type Client interface {
	GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string, nonTechMode bool, modelID ...string) (string, error)
	GenerateRecommendations(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error)
	GenerateVisualization(ctx context.Context, systemPrompt string, visualizationPrompt string, dataRequest string, modelID ...string) (string, error)
	// GenerateRawJSON generates a response with a custom system prompt and user message,
	// without applying the standard NeoBase response schema. Used for tasks like KB generation
	// that need raw JSON output in a custom format.
	GenerateRawJSON(ctx context.Context, systemPrompt string, userMessage string, modelID ...string) (string, error)
	// GenerateWithTools runs an iterative tool-calling loop: the LLM can call tools
	// (execute_read_query, get_table_info) to explore the database, then calls
	// generate_final_response to return the structured answer.
	GenerateWithTools(ctx context.Context, messages []*models.LLMMessage, tools []ToolDefinition, executor ToolExecutorFunc, config ToolCallConfig) (*ToolCallResult, error)
	GetModelInfo() ModelInfo
	SetModel(modelID string) error
}

// ModelInfo contains information about the LLM model
type ModelInfo struct {
	Name                string
	Provider            string
	MaxCompletionTokens int
	ContextLimit        int
}

// Config holds configuration for LLM clients
type Config struct {
	Provider            string
	Model               string
	APIKey              string
	MaxCompletionTokens int
	Temperature         float64
	DBConfigs           []LLMDBConfig
}

type LLMDBConfig struct {
	DBType       string
	Schema       interface{}
	SystemPrompt string
}
