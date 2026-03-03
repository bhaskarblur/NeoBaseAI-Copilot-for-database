package constants

import (
	"fmt"
	"log"
	"strings"
)

const (
	// SchemaCollectionName is the Qdrant collection for schema + KB + relationship vectors.
	SchemaCollectionName = "neobase_schema"

	// MessageCollectionName is the Qdrant collection for chat message vectors.
	// Separate collection gives isolation, independent scaling, and distinct tuning.
	MessageCollectionName = "neobase_messages"

	// --- Schema search defaults ---

	// DefaultTopK is the default number of schema results to retrieve.
	// Reduced from 10 → 6: most queries need 1-3 tables; 6 gives headroom
	// without wasting context tokens on irrelevant tables.
	DefaultTopK = 6
	// DefaultScoreThreshold is the minimum similarity score for the vector leg of schema search.
	// Set relatively low because RRF fusion with the keyword leg handles relevance ranking.
	// The keyword match leg has no threshold — if a table name appears in the query, it's included.
	// This ensures tables with even moderate semantic similarity are not dropped prematurely.
	DefaultScoreThreshold = 0.5

	// --- Message search defaults ---

	// MessageScoreThreshold is the minimum similarity for message retrieval.
	// Higher than schema threshold because we want highly relevant conversation context.
	MessageScoreThreshold = 0.50
	// SlidingWindowSize is the number of recent messages always included verbatim.
	SlidingWindowSize = 20
	// MessageRAGTopK is the default number of older relevant messages to retrieve.
	MessageRAGTopK = 8

	// --- Search post-processing constants ---

	// MaxChunksPerTable is the maximum number of chunks from the same table in search results.
	// Prevents large (chunk-split) tables from dominating all RAG slots.
	// Set to 2 so large tables get their first chunk (header + key columns) plus one continuation.
	MaxChunksPerTable = 2

	// SchemaSearchOverfetch is the multiplier applied to topK when fetching from Qdrant.
	// We fetch more candidates than needed so that per-table dedup and diversity filtering
	// can operate on a richer candidate set while still producing topK final results.
	SchemaSearchOverfetch = 3

	// --- Score-based filtering (post-RRF) ---

	// ScoreCliffRatio is the minimum ratio between consecutive RRF scores.
	// If score[i+1] / score[i] < this ratio, all results from i+1 onward are dropped.
	// Detects the "relevance cliff" where scores plummet from meaningful to noise.
	// Example: 0.50 → 0.12 has ratio 0.24, below 0.30, so 0.12 and below are cut.
	ScoreCliffRatio = 0.30

	// MinRelativeScore is the minimum score as a fraction of the top result's score.
	// Results scoring below top_score * MinRelativeScore are dropped.
	// Prevents very low-scoring noise from filling RAG context.
	// Example: if top score is 0.50, anything below 0.50 * 0.20 = 0.10 is dropped.
	MinRelativeScore = 0.20

	// MinSchemaResults is the minimum number of tables to always return,
	// even if score thresholds would eliminate them. Ensures the LLM always
	// has some context to work with.
	MinSchemaResults = 2
)

// GetRagNoMatchingTablesFound returns the RAG-no-match instruction string
// customized per database type so the LLM knows the correct discovery commands.
func GetRagNoMatchingTablesFound(dbType string) string {
	// Build the DB-specific discovery step
	var discoveryStep string
	switch dbType {
	case DatabaseTypeMongoDB:
		discoveryStep = "1. Start by using execute_read_query with the query `db.getCollectionNames()` to list all available collections in the MongoDB database.\n" +
			"2. Once you identify potentially relevant collections, call get_table_info with those specific collection names to see their fields and structure.\n" +
			"3. Use execute_read_query to run further exploratory queries as needed (e.g. `db.collectionName.find({}).limit(5)` to see sample documents).\n"
	case DatabaseTypeClickhouse:
		discoveryStep = "1. Start by using execute_read_query with the query `SHOW TABLES` to list all available tables in the ClickHouse database.\n" +
			"2. Once you identify potentially relevant tables, call get_table_info with those specific table names to see their columns and structure.\n" +
			"3. Use execute_read_query to run further exploratory queries as needed to understand the data.\n"
	case DatabaseTypeMySQL:
		discoveryStep = "1. Start by using execute_read_query with the query `SHOW TABLES` to list all available tables in the MySQL database.\n" +
			"2. Once you identify potentially relevant tables, call get_table_info with those specific table names to see their columns and structure.\n" +
			"3. Use execute_read_query to run further exploratory queries as needed to understand the data.\n"
	case DatabaseTypeSpreadsheet:
		// Spreadsheet connections use a chat-specific PostgreSQL schema (conn_<chatID>),
		// not the 'public' schema. Use current_schema() which resolves to the correct one.
		discoveryStep = "1. Start by using execute_read_query with the query `SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema()` to list all available tables.\n" +
			"2. Once you identify potentially relevant tables, call get_table_info with those specific table names to see their columns and structure.\n" +
			"3. Use execute_read_query to run further exploratory queries as needed to understand the data.\n"
	default:
		// PostgreSQL, YugabyteDB, Spreadsheet, etc.
		discoveryStep = "1. Start by using execute_read_query with the query `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'` to list all available tables.\n" +
			"2. Once you identify potentially relevant tables, call get_table_info with those specific table names to see their columns and structure.\n" +
			"3. Use execute_read_query to run further exploratory queries as needed to understand the data.\n"
	}

	return "\n\nℹ️ SCHEMA SEARCH NOTE:\n" +
		"A semantic search of the database schema found no tables or collections that closely match the user's query.\n" +
		"The full database schema is NOT provided in this context to keep token usage efficient.\n\n" +
		"IMPORTANT — You MUST use tool-calling to discover the database structure:\n" +
		discoveryStep +
		"4. Do NOT guess or fabricate table/collection names — always verify by calling the tools first.\n" +
		"5. If after exploring you confirm the data truly does not exist, explain that clearly to the user.\n" +
		"6. Always include the FINAL user-facing queries in the 'queries' array of your generate_final_response. Do NOT include intermediate exploration queries (like listing tables or checking schema) that only helped you discover the database structure.\n" +
		"7. Remember: You can also see the message history to know about the past interaction and the database structure from the messages."
}

// GetRagQdrantUnavailable returns a lightweight fallback context string when Qdrant is
// temporarily unreachable but the Knowledge Base has table descriptions. Instead of
// sending the full schema (~196K chars / ~49K tokens), we send only table names and
// descriptions (~1,500-2,500 chars / ~400-600 tokens) so the LLM knows what tables
// exist and can use targeted tool-calling (get_table_info) for detailed column info.
func GetRagQdrantUnavailable(dbType string, tableListing string) string {
	// Build the DB-specific discovery step
	var discoveryStep string
	switch dbType {
	case DatabaseTypeMongoDB:
		discoveryStep = "Use get_table_info with selected collection names to see their fields and structure, or execute_read_query for further exploration."
	case DatabaseTypeClickhouse:
		discoveryStep = "Use get_table_info with selected table names to see their columns and structure, or execute_read_query for further exploration."
	default:
		discoveryStep = "Use get_table_info with selected table names to see their columns and structure, or execute_read_query for further exploration."
	}

	return "\n\nℹ️ SCHEMA CONTEXT (lightweight — vector search temporarily unavailable):\n" +
		"The semantic search service is temporarily unreachable. Below is a summary of all tables/collections " +
		"in this database with their descriptions. Use this to identify relevant tables, then call " +
		"get_table_info for detailed column information before writing queries.\n\n" +
		tableListing + "\n\n" +
		"INSTRUCTIONS:\n" +
		"1. Review the table listing above to identify tables relevant to the user's question.\n" +
		"2. " + discoveryStep + "\n" +
		"3. Do NOT guess column names — always verify with get_table_info first.\n" +
		"4. Always include the FINAL user-facing queries in the 'queries' array of your generate_final_response.\n" +
		"5. Remember: You can also see the message history to know about the past interaction and the database structure from the messages."
}

// --- Embedding Provider Constants ---

const (
	EmbeddingProviderOpenAI = "openai"
	EmbeddingProviderGemini = "gemini"
)

// SupportedEmbeddingProviders lists all valid embedding provider identifiers.
var SupportedEmbeddingProviders = []string{
	EmbeddingProviderOpenAI,
	EmbeddingProviderGemini,
}

// --- Embedding Model Definitions ---

// EmbeddingModel represents a supported embedding model configuration.
type EmbeddingModel struct {
	ID          string `json:"id"`          // Model identifier used in API calls
	Provider    string `json:"provider"`    // Provider name ("openai" or "gemini")
	DisplayName string `json:"displayName"` // Human-readable name
	Dimension   int    `json:"dimension"`   // Output embedding vector dimension
	MaxInput    int    `json:"maxInput"`    // Maximum input tokens
	IsDefault   bool   `json:"isDefault"`   // Whether this is the default model for its provider
	Description string `json:"description"` // Brief description
}

// OpenAI Embedding Models
var OpenAIEmbeddingModels = []EmbeddingModel{
	{
		ID:          "text-embedding-3-small",
		Provider:    EmbeddingProviderOpenAI,
		DisplayName: "Text Embedding 3 Small",
		Dimension:   1536,
		MaxInput:    8191,
		IsDefault:   true,
		Description: "Most cost-effective OpenAI embedding model, great balance of performance and cost",
	},
	{
		ID:          "text-embedding-3-large",
		Provider:    EmbeddingProviderOpenAI,
		DisplayName: "Text Embedding 3 Large",
		Dimension:   3072,
		MaxInput:    8191,
		IsDefault:   false,
		Description: "Highest quality OpenAI embedding model with larger dimensions",
	},
	{
		ID:          "text-embedding-ada-002",
		Provider:    EmbeddingProviderOpenAI,
		DisplayName: "Text Embedding Ada 002",
		Dimension:   1536,
		MaxInput:    8191,
		IsDefault:   false,
		Description: "Legacy OpenAI embedding model, predecessor to v3 models",
	},
}

// Gemini Embedding Models
var GeminiEmbeddingModels = []EmbeddingModel{
	{
		ID:          "gemini-embedding-001",
		Provider:    EmbeddingProviderGemini,
		DisplayName: "Gemini Embedding 001",
		Dimension:   768,
		MaxInput:    2048,
		IsDefault:   true,
		Description: "Latest Gemini embedding model (June 2025) with MRL support, flexible dimensions 128-3072, recommended: 768/1536/3072",
	},
	{
		ID:          "text-embedding-004",
		Provider:    EmbeddingProviderGemini,
		DisplayName: "Text Embedding 004",
		Dimension:   768,
		MaxInput:    2048,
		IsDefault:   false,
		Description: "Previous generation Gemini embedding model, 768 dimensions",
	},
	{
		ID:          "embedding-001",
		Provider:    EmbeddingProviderGemini,
		DisplayName: "Embedding 001",
		Dimension:   768,
		MaxInput:    2048,
		IsDefault:   false,
		Description: "Legacy Gemini embedding model (oldest)",
	},
}

// SupportedEmbeddingModels is the combined list of all supported embedding models.
var SupportedEmbeddingModels = append(
	OpenAIEmbeddingModels,
	GeminiEmbeddingModels...,
)

// --- Lookup & Validation Functions ---

// IsValidEmbeddingProvider checks if the given provider string is supported.
func IsValidEmbeddingProvider(provider string) bool {
	for _, p := range SupportedEmbeddingProviders {
		if p == provider {
			return true
		}
	}
	return false
}

// GetEmbeddingModel returns the embedding model definition by ID, or nil if not found.
func GetEmbeddingModel(modelID string) *EmbeddingModel {
	for i := range SupportedEmbeddingModels {
		if SupportedEmbeddingModels[i].ID == modelID {
			return &SupportedEmbeddingModels[i]
		}
	}
	return nil
}

// GetEmbeddingModelsByProvider returns all embedding models for a specific provider.
func GetEmbeddingModelsByProvider(provider string) []EmbeddingModel {
	var models []EmbeddingModel
	for _, m := range SupportedEmbeddingModels {
		if m.Provider == provider {
			models = append(models, m)
		}
	}
	return models
}

// GetDefaultEmbeddingModel returns the default embedding model for a given provider.
func GetDefaultEmbeddingModel(provider string) *EmbeddingModel {
	for i := range SupportedEmbeddingModels {
		if SupportedEmbeddingModels[i].Provider == provider && SupportedEmbeddingModels[i].IsDefault {
			return &SupportedEmbeddingModels[i]
		}
	}
	// Fallback: first model for this provider
	for i := range SupportedEmbeddingModels {
		if SupportedEmbeddingModels[i].Provider == provider {
			return &SupportedEmbeddingModels[i]
		}
	}
	return nil
}

// ValidateEmbeddingConfig validates the embedding provider and model configuration.
// Returns the resolved (provider, model, error).
// If provider is empty, it auto-detects from available API keys (OpenAI > Gemini).
// If model is empty, it uses the default for that provider.
func ValidateEmbeddingConfig(provider, model, openAIKey, geminiKey string) (string, string, error) {
	// --- Auto-detect provider if not set ---
	if provider == "" {
		if openAIKey != "" {
			provider = EmbeddingProviderOpenAI
		} else if geminiKey != "" {
			provider = EmbeddingProviderGemini
		} else {
			// No API keys at all — embeddings disabled, not an error
			return "", "", nil
		}
	}

	// --- Validate provider ---
	if !IsValidEmbeddingProvider(provider) {
		return "", "", fmt.Errorf(
			"unsupported EMBEDDING_PROVIDER: %q. Supported providers: %s",
			provider, strings.Join(SupportedEmbeddingProviders, ", "),
		)
	}

	// --- Validate API key for selected provider ---
	switch provider {
	case EmbeddingProviderOpenAI:
		if openAIKey == "" {
			return "", "", fmt.Errorf("EMBEDDING_PROVIDER is %q but OPENAI_API_KEY is not configured", provider)
		}
	case EmbeddingProviderGemini:
		if geminiKey == "" {
			return "", "", fmt.Errorf("EMBEDDING_PROVIDER is %q but GEMINI_API_KEY is not configured", provider)
		}
	}

	// --- Resolve model ---
	if model == "" {
		defaultModel := GetDefaultEmbeddingModel(provider)
		if defaultModel == nil {
			return "", "", fmt.Errorf("no default embedding model found for provider: %s", provider)
		}
		model = defaultModel.ID
	}

	// --- Validate model ---
	embModel := GetEmbeddingModel(model)
	if embModel == nil {
		validModels := GetEmbeddingModelsByProvider(provider)
		ids := make([]string, len(validModels))
		for i, m := range validModels {
			ids[i] = m.ID
		}
		return "", "", fmt.Errorf(
			"unsupported EMBEDDING_MODEL: %q for provider %q. Supported models: %s",
			model, provider, strings.Join(ids, ", "),
		)
	}

	// --- Validate model belongs to chosen provider ---
	if embModel.Provider != provider {
		return "", "", fmt.Errorf(
			"EMBEDDING_MODEL %q belongs to provider %q, but EMBEDDING_PROVIDER is set to %q",
			model, embModel.Provider, provider,
		)
	}

	return provider, model, nil
}

// LogEmbeddingInitialization logs the resolved embedding configuration at startup.
func LogEmbeddingInitialization(provider, model, openAIKey, geminiKey string) {
	separator := strings.Repeat("=", 80)
	log.Println("\n" + separator)
	log.Println("🧠 EMBEDDING MODEL INITIALIZATION REPORT")
	log.Println(separator)

	if provider == "" {
		log.Println("\n  ⚠️  No embedding provider configured. RAG pipeline is DISABLED.")
		log.Println("   To enable, configure one of the following:")
		log.Println("   - Set OPENAI_API_KEY (auto-detects OpenAI embeddings)")
		log.Println("   - Set GEMINI_API_KEY (auto-detects Gemini embeddings)")
		log.Println("   - Or explicitly set EMBEDDING_PROVIDER and EMBEDDING_MODEL")
	} else {
		embModel := GetEmbeddingModel(model)
		icon := "📘"
		if provider == EmbeddingProviderGemini {
			icon = "🔵"
		}

		log.Printf("\n%s EMBEDDING PROVIDER: %s", icon, strings.ToUpper(provider))
		if embModel != nil {
			log.Printf("  ✅ Model: %s (%s)", embModel.DisplayName, embModel.ID)
			log.Printf("  📐 Dimension: %d", embModel.Dimension)
			log.Printf("  📝 Max Input Tokens: %d", embModel.MaxInput)
			log.Printf("  💡 %s", embModel.Description)
		} else {
			log.Printf("  ✅ Model: %s", model)
		}

		// Show available models for the provider
		log.Printf("\n  📊 Available Embedding Models for %s:", provider)
		for _, m := range GetEmbeddingModelsByProvider(provider) {
			defaultTag := ""
			if m.IsDefault {
				defaultTag = " (default)"
			}
			activeTag := ""
			if m.ID == model {
				activeTag = " ← active"
			}
			log.Printf("     • %s — %dd%s%s", m.ID, m.Dimension, defaultTag, activeTag)
		}
	}

	log.Println(separator + "\n")
}
