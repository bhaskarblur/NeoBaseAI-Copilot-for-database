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
	DefaultTopK = 10
	// DefaultScoreThreshold is the minimum similarity score for schema/KB search.
	DefaultScoreThreshold = 0.65

	// --- Message search defaults ---

	// MessageScoreThreshold is the minimum similarity for message retrieval.
	// Higher than schema threshold because we want highly relevant conversation context.
	MessageScoreThreshold = 0.65
	// SlidingWindowSize is the number of recent messages always included verbatim.
	SlidingWindowSize = 20
	// MessageRAGTopK is the default number of older relevant messages to retrieve.
	MessageRAGTopK = 8
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
