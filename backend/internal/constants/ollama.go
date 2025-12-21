package constants

const (
	OllamaModel               = "llama3.1:latest"
	OllamaTemperature         = 1
	OllamaMaxCompletionTokens = 8192
)

// Ollama uses the same database-specific system prompts as Gemini
// These are comprehensive prompts that work well across all providers
const OllamaPostgreSQLPrompt = GeminiPostgreSQLPrompt
const OllamaMySQLPrompt = GeminiMySQLPrompt
const OllamaClickhousePrompt = GeminiClickhousePrompt
const OllamaMongoDBPrompt = GeminiMongoDBPrompt
const OllamaSpreadsheetPrompt = GeminiSpreadsheetPrompt

// Ollama Response Schema as JSON object (for format parameter)
// Ollama uses a format parameter with JSON schema similar to Gemini
const OllamaLLMResponseSchemaJSON = `{
	"type": "object",
	"properties": {
		"assistantMessage": {
			"type": "string",
			"description": "A friendly AI Response/Explanation or clarification question (Must Send this). Note: This should be Markdown formatted text"
		},
		"actionButtons": {
			"type": "array",
			"description": "Array of action buttons to suggest to the user",
			"items": {
				"type": "object",
				"properties": {
					"label": {
						"type": "string",
						"description": "Button text to display to the user (example: Refresh Knowledge Base)"
					},
					"action": {
						"type": "string",
						"description": "Action identifier (e.g., refresh_schema)"
					},
					"isPrimary": {
						"type": "boolean",
						"description": "Whether this is a primary action button"
					}
				},
				"required": ["label", "action"]
			}
		},
		"queries": {
			"type": "array",
			"description": "Array of database queries to execute",
			"items": {
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "The actual database query with no placeholders"
					},
					"queryType": {
						"type": "string",
						"description": "Type of query: SELECT, INSERT, UPDATE, DELETE, DDL, etc."
					},
					"pagination": {
						"type": "object",
						"description": "Pagination configuration for the query",
						"properties": {
							"paginatedQuery": {
								"type": "string",
								"description": "Paginated version of the query with LIMIT/OFFSET or equivalent"
							},
							"countQuery": {
								"type": "string",
								"description": "Query to get total count of records"
							}
						}
					},
					"tables": {
						"type": "string",
						"description": "Comma-separated list of tables involved in the query"
					},
					"explanation": {
						"type": "string",
						"description": "User-friendly explanation of what the query does"
					},
					"isCritical": {
						"type": "boolean",
						"description": "Whether this query modifies data or is potentially dangerous"
					},
					"canRollback": {
						"type": "boolean",
						"description": "Whether this query can be rolled back"
					},
					"rollbackDependentQuery": {
						"type": "string",
						"description": "Query to fetch data needed for rollback (if applicable)"
					},
					"rollbackQuery": {
						"type": "string",
						"description": "Query to reverse the operation (if applicable)"
					},
					"estimateResponseTime": {
						"type": "string",
						"description": "Estimated response time in milliseconds (e.g., '78')"
					},
					"exampleResultString": {
						"type": "string",
						"description": "MUST BE VALID JSON STRING with no additional text. Example: [{\"column1\":\"value1\"}] or {\"result\":\"1 row affected\"}"
					}
				},
				"required": ["query", "queryType", "explanation", "isCritical", "canRollback", "estimateResponseTime", "exampleResultString"]
			}
		}
	},
	"required": ["assistantMessage"]
}`

// Query Recommendations Schema for Ollama
const OllamaRecommendationsSchemaJSON = `{
	"type": "object",
	"properties": {
		"recommendations": {
			"type": "array",
			"description": "Array of recommended database queries",
			"items": {
				"type": "object",
				"properties": {
					"title": {
						"type": "string",
						"description": "Short title for the recommendation (max 6 words)"
					},
					"description": {
						"type": "string",
						"description": "Brief description of what this query does"
					},
					"query": {
						"type": "string",
						"description": "The natural language query the user should ask"
					},
					"category": {
						"type": "string",
						"enum": ["analytics", "data_exploration", "monitoring", "optimization", "reporting"],
						"description": "Category of the recommendation"
					}
				},
				"required": ["title", "description", "query", "category"]
			}
		}
	},
	"required": ["recommendations"]
}`

// Query Recommendations Prompt for Ollama (same as Gemini)
const OllamaRecommendationsPrompt = GeminiRecommendationsPrompt
