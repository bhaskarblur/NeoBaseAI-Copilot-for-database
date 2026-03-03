package embedding

import "context"

// Provider defines the interface for text embedding services.
// Implementations exist for OpenAI and Gemini.
type Provider interface {
	// Embed generates a vector embedding for a single text input (document/storage context).
	// For indexing documents — uses RETRIEVAL_DOCUMENT task type where supported.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates vector embeddings for multiple texts in a single API call (document/storage context).
	// More efficient than calling Embed in a loop. Uses RETRIEVAL_DOCUMENT task type where supported.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// EmbedQuery generates a vector embedding optimized for search/retrieval queries.
	// Uses RETRIEVAL_QUERY task type where supported (e.g., Gemini).
	// For providers that don't distinguish (e.g., OpenAI), this is identical to Embed.
	EmbedQuery(ctx context.Context, text string) ([]float32, error)

	// GetDimension returns the dimensionality of embedding vectors produced by this provider.
	GetDimension() int

	// GetProviderName returns the provider identifier (e.g., "openai", "gemini").
	GetProviderName() string

	// GetModelName returns the embedding model being used.
	GetModelName() string
}

// Config holds configuration for initializing an embedding provider.
type Config struct {
	Provider string // "openai" or "gemini"
	Model    string // e.g., "text-embedding-3-small" or "text-embedding-004"
	APIKey   string
}

// DefaultOpenAIModel is the default embedding model for OpenAI.
const DefaultOpenAIModel = "text-embedding-3-small"

// DefaultGeminiModel is the default embedding model for Gemini.
const DefaultGeminiModel = "gemini-embedding-001"

// OpenAIDimension is the embedding dimension for text-embedding-3-small.
const OpenAIDimension = 1536

// GeminiDimension is the default embedding dimension for gemini-embedding-001 (supports 128-3072 via MRL, we use 768).
const GeminiDimension = 768
