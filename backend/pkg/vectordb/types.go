package vectordb

import "context"

// PointID is a string identifier for a vector point (e.g., "schema_{chatID}_{tableName}").
type PointID = string

// VectorPoint represents a single vector to upsert into the vector database.
type VectorPoint struct {
	ID      PointID                `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

// SearchRequest defines parameters for a vector similarity search.
type SearchRequest struct {
	Vector         []float32         `json:"vector"`
	Filter         map[string]string `json:"filter"`          // Key-value payload filter (AND logic)
	TopK           int               `json:"top_k"`           // Max results to return
	ScoreThreshold float32           `json:"score_threshold"` // Minimum similarity score (0-1)
}

// HybridSearchRequest defines parameters for a hybrid (vector + full-text) search.
// Qdrant executes both legs server-side and fuses results via Reciprocal Rank Fusion.
type HybridSearchRequest struct {
	Vector         []float32         `json:"vector"`
	Filter         map[string]string `json:"filter"`          // Key-value payload filter applied to BOTH legs
	TopK           int               `json:"top_k"`           // Max results to return after fusion
	ScoreThreshold float32           `json:"score_threshold"` // Minimum similarity score for vector leg
	TextQuery      string            `json:"text_query"`      // Full-text search query (matched against text-indexed payload fields)
	TextField      string            `json:"text_field"`      // Payload field to run full-text match on (must have text index)
}

// SearchResult represents a single search result from the vector database.
type SearchResult struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// FilterCondition represents a key-value condition for filtering vectors.
type FilterCondition struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Client defines the interface for vector database operations.
type Client interface {
	// IsHealthy checks if the vector database is reachable.
	IsHealthy(ctx context.Context) bool

	// EnsureCollection creates a collection if it doesn't already exist (generic, no indexes).
	EnsureCollection(ctx context.Context, collection string, dimension int) error

	// EnsureSchemaCollection creates the schema collection with schema-specific indexes.
	EnsureSchemaCollection(ctx context.Context, dimension int) error

	// EnsureMessageCollection creates the message collection with message-specific indexes.
	EnsureMessageCollection(ctx context.Context, dimension int) error

	// Upsert inserts or updates vector points in a collection.
	Upsert(ctx context.Context, collection string, points []VectorPoint) error

	// Search performs similarity search within a collection.
	Search(ctx context.Context, collection string, req SearchRequest) ([]SearchResult, error)

	// HybridSearch performs a hybrid search combining dense vector similarity and full-text keyword matching.
	// Uses Qdrant's Query API with two prefetch legs fused via Reciprocal Rank Fusion (RRF).
	// All computation happens server-side in Qdrant — no application-side post-processing.
	HybridSearch(ctx context.Context, collection string, req HybridSearchRequest) ([]SearchResult, error)

	// Delete removes points by their IDs from a collection.
	Delete(ctx context.Context, collection string, ids []PointID) error

	// DeleteByFilter removes all points matching the filter conditions.
	DeleteByFilter(ctx context.Context, collection string, filters []FilterCondition) error

	// Count returns the number of points matching the filter in a collection.
	Count(ctx context.Context, collection string, filters []FilterCondition) (int64, error)

	// ScrollByFilter retrieves all points matching the filter, including their vectors.
	// Used for copying vectors between chats (e.g., chat duplication).
	ScrollByFilter(ctx context.Context, collection string, filters []FilterCondition, withVectors bool) ([]VectorPoint, error)

	// Close cleans up the connection.
	Close() error
}
