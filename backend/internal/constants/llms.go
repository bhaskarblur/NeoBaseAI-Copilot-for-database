package constants

import "log"

const (
	OpenAI = "openai"
	Gemini = "gemini"
	Claude = "claude"
	Ollama = "ollama"
)

// GetLLMResponseSchema returns the appropriate response schema based on the LLM provider
// Note: Schemas are provider-specific, not database-specific
func GetLLMResponseSchema(provider string, dbType string) interface{} {
	switch provider {
	case OpenAI:
		return OpenAILLMResponseSchema
	case Gemini:
		return GeminiLLMResponseSchema
	case Claude:
		return ClaudeLLMResponseSchemaJSON
	case Ollama:
		return OllamaLLMResponseSchemaJSON
	default:
		return OpenAILLMResponseSchema
	}
}

// GetSystemPrompt returns the appropriate system prompt based on database type
// The prompt is unified per database type and works with any LLM provider
func GetSystemPrompt(provider string, dbType string, nonTechMode bool) string {
	log.Printf("GetSystemPrompt -> provider: %s, dbType: %s, nonTechMode: %v", provider, dbType, nonTechMode)

	var basePrompt string

	// Get database-type-specific prompt (unified for all providers)
	basePrompt = getDatabasePrompt(dbType)

	// Prepend non-tech mode instructions if enabled (at the beginning for stronger emphasis)
	if nonTechMode {
		log.Printf("GetSystemPrompt -> Prepending non-tech mode instructions for %s", dbType)
		nonTechInstructions := getNonTechModeInstructions(dbType)
		basePrompt = nonTechInstructions + "\n\n" + basePrompt
	}

	log.Printf("GetSystemPrompt -> Final prompt length: %d characters", len(basePrompt))
	return basePrompt
}

// getDatabasePrompt returns the database-type-specific prompt
// This is unified and not provider-specific
func getDatabasePrompt(dbType string) string {
	switch dbType {
	case DatabaseTypePostgreSQL:
		return PostgreSQLPrompt
	case DatabaseTypeMySQL:
		return MySQLPrompt
	case DatabaseTypeYugabyteDB:
		return YugabyteDBPrompt
	case DatabaseTypeClickhouse:
		return ClickhousePrompt
	case DatabaseTypeMongoDB:
		return MongoDBPrompt
	case DatabaseTypeSpreadsheet:
		return PostgreSQLPrompt // Use PostgreSQL schema since spreadsheet uses PostgreSQL internally
	default:
		return PostgreSQLPrompt // Default to PostgreSQL
	}
}

// getNonTechModeInstructions returns database-specific instructions for non-technical mode
func getNonTechModeInstructions(dbType string) string {
	baseInstructions := `
===== CRITICAL: NON-TECHNICAL MODE ACTIVE =====
**YOU MUST FOLLOW THESE NON-TECHNICAL MODE INSTRUCTIONS - THEY OVERRIDE ALL OTHER INSTRUCTIONS**

**TODAY'S DATE CONTEXT**: Always check the current timestamp in the logs or system time to determine "today". 
For relative dates like "yesterday", "last week", etc., calculate from the actual current date.

⚠️ **MOST IMPORTANT RULE**: The assistantMessage field MUST be completely non-technical!
- NO database terminology (query, fetch, limit, order, sort, join, table, collection)
- NO technical explanations of what you're doing
- Just simple, direct statements like "Here's your latest feedback:"

You are in NON-TECHNICAL MODE. This mode is designed for business users who need insights without technical complexity.

**UNIVERSAL RULES FOR ALL DATABASES**:
1. Hide ALL technical fields (IDs, timestamps, version fields)
2. Replace ALL ID references with meaningful data via JOINs/lookups
3. Format ALL dates to human-readable format (e.g., "January 15, 2024")
4. Show ONLY fields that provide business value
5. Use simple, conversational language in assistantMessage
6. NEVER mention technical terms like "query", "database", "table", "collection"
7. Focus on WHAT the data shows and explain the result in a way that is easy to understand, not HOW the query works
8. **CRITICAL DATE RANGE RULES**:
   - When user asks for data "on" a specific date, use THAT date (not the day before)
   - "Yesterday" means the day before today, NOT two days ago
   - Example: If today is August 10, "yesterday" = August 9 (NOT August 8)
   - Date ranges: start of requested date to start of NEXT day
   - "on August 9" = >= "2025-08-09T00:00:00" AND < "2025-08-10T00:00:00"

**CRITICAL assistantMessage RULES - ABSOLUTELY NO TECHNICAL LANGUAGE**:
- ❌ NEVER say: "Here's the query to fetch...", "I'm fetching...", "ordered by...", "limit the results"
- ❌ NEVER say: "The query will...", "Including details about...", "submission date"
- ❌ NEVER use database terms: query, fetch, limit, order, sort, filter, database, table, collection
- ✅ ALWAYS use simple phrases: "Here's your latest...", "I found...", "This shows..."
- ✅ Keep it SHORT and SIMPLE - one sentence is often enough

Examples of CORRECT assistantMessage:
- "Here's your latest feedback:"
- "I found your most recent order:"
- "This shows your top customers:"
- "Here are your sales for January:" 
`

	// Add database-specific instructions
	switch dbType {
	case DatabaseTypeMongoDB:
		return baseInstructions + getMongoDBNonTechInstructions()
	case DatabaseTypePostgreSQL, DatabaseTypeYugabyteDB:
		return baseInstructions + getPostgreSQLNonTechInstructions()
	case DatabaseTypeMySQL:
		return baseInstructions + getMySQLNonTechInstructions()
	case DatabaseTypeClickhouse:
		return baseInstructions + getClickhouseNonTechInstructions()
	default:
		return baseInstructions + getPostgreSQLNonTechInstructions()
	}
}

// GetRecommendationsSchema returns the appropriate recommendations schema based on provider
func GetRecommendationsSchema(provider string) interface{} {
	switch provider {
	case OpenAI:
		return OpenAIRecommendationsResponseSchema
	case Gemini:
		return GeminiRecommendationsResponseSchema
	case Claude:
		return ClaudeRecommendationsSchemaJSON
	case Ollama:
		return OllamaRecommendationsSchemaJSON
	default:
		return OpenAIRecommendationsResponseSchema // Default to OpenAI
	}
}

// GetRecommendationsPrompt returns the recommendations prompt
func GetRecommendationsPrompt(provider string) string {
	return RecommendationsPrompt
}

func GetVisualizationPrompt(dbType string) string {
	switch dbType {
	case DatabaseTypePostgreSQL:
		return PostgreSQLVisualizationPrompt
	case DatabaseTypeMySQL:
		return MySQLVisualizationPrompt
	case DatabaseTypeYugabyteDB:
		return YugabyteVisualizationPrompt
	case DatabaseTypeClickhouse:
		return ClickhouseVisualizationPrompt
	case DatabaseTypeMongoDB:
		return MongoDBVisualizationPrompt
	case DatabaseTypeSpreadsheet:
		return PostgreSQLVisualizationPrompt // Use PostgreSQL prompt for spreadsheets
	default:
		return PostgreSQLVisualizationPrompt // Default to PostgreSQL
	}
}

// Recommendations Prompt for all LLMs
const RecommendationsPrompt = `You are NeoBase AI, a database assistant. Your task is to generate 60 diverse and practical question recommendations that users can ask about their database.

Generate exactly 60 different question recommendations. If you cannot generate 60, you MUST provide at least 40 recommendations at any cost.

The recommendations should be:
- Diverse (data exploration, analytics, insights, reporting, monitoring, administration, etc.)
- Practical and commonly useful for data analysis
- Natural language questions that users would ask
- Relevant to the database type and schema
- Helpful that would allow user to deeply explore their data & potentially what could be done with the data
- Concise and clear
- User-Friendly & Meaningful that user should understand
Consider the database type, the schema and any recent conversation context when generating recommendations.

Response format should be JSON with this structure:
{
  "recommendations": [
    {
      "text": "Show me the most recent orders"
    },
    {
      "text": "What are the top selling products?"
    },
    {
      "text": "How many users registered this month?"
    }
    // ... continue with more recommendations to reach 60 total
  ]
}`
