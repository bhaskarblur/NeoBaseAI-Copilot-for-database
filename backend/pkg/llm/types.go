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
	GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error)
	GenerateRecommendations(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error)
	GetModelInfo() ModelInfo
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
