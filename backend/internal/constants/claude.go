package constants

var ClaudeLLMModels = []LLMModel{

	{
		ID:                  "claude-opus-4-5-20251101",
		Provider:            Claude,
		DisplayName:         "Claude Opus 4.5 (Best in World)",
		IsEnabled:           true,
		MaxCompletionTokens: 16384,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "World's best model for coding, agents, and computer use. State-of-the-art across all domains with 2x token efficiency",
	},
	{
		ID:                  "claude-sonnet-4-5",
		Provider:            Claude,
		DisplayName:         "Claude Sonnet 4.5 (Frontier Intelligence)",
		IsEnabled:           true,
		MaxCompletionTokens: 16384,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Best coding model in the world. State-of-the-art on SWE-bench, strongest for complex agents, most aligned model",
	},
	{
		ID:                  "claude-haiku-4-5",
		Provider:            Claude,
		DisplayName:         "Claude Haiku 4.5 (Fast Intelligence)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "State-of-the-art coding with unprecedented speed and cost-efficiency. Matches older frontier models at fraction of cost",
	},

	// Claude 4 Series (Reliable Production)
	{
		ID:                  "claude-sonnet-4",
		Provider:            Claude,
		DisplayName:         "Claude Sonnet 4 (Production Workhorse)",
		IsEnabled:           true,
		Default:             ptrBool(true),
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Reliable production model with frontier performance, improved coding over 3.7. Practical for most AI use cases and high-volume tasks",
	},

	// Claude 3.5 Series (Production Ready)
	{
		ID:                  "claude-3-5-sonnet-20241022",
		Provider:            Claude,
		DisplayName:         "Claude 3.5 Sonnet v2 (Oct 2024)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Previous generation flagship with excellent coding and reasoning, reliable for production use",
	},
	{
		ID:                  "claude-3-5-sonnet-20240620",
		Provider:            Claude,
		DisplayName:         "Claude 3.5 Sonnet v1 (June 2024)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "First version of Claude 3.5 Sonnet, highly capable for most enterprise tasks",
	},
	{
		ID:                  "claude-3-5-haiku-20241022",
		Provider:            Claude,
		DisplayName:         "Claude 3.5 Haiku (Fast)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Fast and cost-effective 3.5 model for high-volume production tasks",
	},

	// Claude 3 Series (Stable Legacy)
	{
		ID:                  "claude-3-opus-20240229",
		Provider:            Claude,
		DisplayName:         "Claude 3 Opus (Legacy Powerful)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Legacy Claude 3 model for complex tasks, superseded by 4.5 series",
	},
	{
		ID:                  "claude-3-sonnet-20240229",
		Provider:            Claude,
		DisplayName:         "Claude 3 Sonnet (Legacy Balanced)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Legacy balanced model for standard workloads, superseded by 4.5 series",
	},
	{
		ID:                  "claude-3-haiku-20240307",
		Provider:            Claude,
		DisplayName:         "Claude 3 Haiku (Legacy Fast)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Legacy fast model for simple tasks, superseded by Haiku 4.5",
	},
}

// Initial LLM Response Schema for Claude
const ClaudeLLMResponseSchemaJSON = `{
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

// Query Recommendations Schema for Claude
const ClaudeRecommendationsSchemaJSON = `{
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
