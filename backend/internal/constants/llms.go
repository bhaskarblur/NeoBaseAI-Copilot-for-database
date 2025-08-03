package constants

const (
	OpenAI = "openai"
	Gemini = "gemini"
)

func GetLLMResponseSchema(provider string, dbType string) interface{} {
	switch provider {
	case OpenAI:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return OpenAIPostgresLLMResponseSchema
		case DatabaseTypeYugabyteDB:
			return OpenAIYugabyteDBLLMResponseSchema
		case DatabaseTypeMySQL:
			return OpenAIMySQLLLMResponseSchema
		case DatabaseTypeClickhouse:
			return OpenAIClickhouseLLMResponseSchema
		case DatabaseTypeMongoDB:
			return OpenAIMongoDBLLMResponseSchema
		default:
			return OpenAIPostgresLLMResponseSchema
		}
	case Gemini:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return GeminiPostgresLLMResponseSchema
		case DatabaseTypeYugabyteDB:
			return GeminiYugabyteDBLLMResponseSchema
		case DatabaseTypeMySQL:
			return GeminiMySQLLLMResponseSchema
		case DatabaseTypeClickhouse:
			return GeminiClickhouseLLMResponseSchema
		case DatabaseTypeMongoDB:
			return GeminiMongoDBLLMResponseSchema
		default:
			return GeminiPostgresLLMResponseSchema
		}
	}
	return ""
}

// GetSystemPrompt returns the appropriate system prompt based on database type
func GetSystemPrompt(provider string, dbType string) string {
	switch provider {
	case OpenAI:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return OpenAIPostgreSQLPrompt
		case DatabaseTypeMySQL:
			return OpenAIMySQLPrompt
		case DatabaseTypeYugabyteDB:
			return OpenAIYugabyteDBPrompt
		case DatabaseTypeClickhouse:
			return OpenAIClickhousePrompt
		case DatabaseTypeMongoDB:
			return OpenAIMongoDBPrompt
		default:
			return OpenAIPostgreSQLPrompt // Default to PostgreSQL
		}
	case Gemini:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return GeminiPostgreSQLPrompt
		case DatabaseTypeYugabyteDB:
			return GeminiYugabyteDBPrompt
		case DatabaseTypeMySQL:
			return GeminiMySQLPrompt
		case DatabaseTypeClickhouse:
			return GeminiClickhousePrompt
		case DatabaseTypeMongoDB:
			return GeminiMongoDBPrompt
		default:
			return GeminiPostgreSQLPrompt // Default to PostgreSQL
		}
	}
	return ""
}

// GetRecommendationsPrompt returns the appropriate recommendations prompt based on provider
func GetRecommendationsPrompt(provider string) string {
	switch provider {
	case OpenAI:
		return OpenAIRecommendationsPrompt
	case Gemini:
		return GeminiRecommendationsPrompt
	default:
		return OpenAIRecommendationsPrompt // Default to OpenAI
	}
}

// GetRecommendationsSchema returns the appropriate recommendations schema based on provider
func GetRecommendationsSchema(provider string) interface{} {
	switch provider {
	case OpenAI:
		return OpenAIRecommendationsResponseSchema
	case Gemini:
		return GeminiRecommendationsResponseSchema
	default:
		return OpenAIRecommendationsResponseSchema // Default to OpenAI
	}
}
