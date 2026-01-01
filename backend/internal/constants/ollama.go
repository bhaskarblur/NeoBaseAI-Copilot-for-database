package constants

var OllamaLLMModels = []LLMModel{

	// === REASONING MODELS ===
	// DeepSeek R1 Series (74.6M+ pulls) - State-of-the-art reasoning
	{
		ID:                  "deepseek-r1:latest",
		Provider:            Ollama,
		DisplayName:         "DeepSeek R1 (Top Reasoning)",
		IsEnabled:           true,
		Default:             ptrBool(true),
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "State-of-the-art reasoning model rivaling GPT-4, excellent for complex problem-solving and analysis",
	},
	{
		ID:                  "deepseek-r1:70b",
		Provider:            Ollama,
		DisplayName:         "DeepSeek R1 70B (Powerful Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "70B parameter reasoning model for demanding analytical tasks",
	},
	{
		ID:                  "deepseek-r1:8b",
		Provider:            Ollama,
		DisplayName:         "DeepSeek R1 8B (Fast Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Lightweight reasoning model running efficiently on consumer hardware",
	},
	{
		ID:                  "qwq:32b",
		Provider:            Ollama,
		DisplayName:         "QwQ 32B (Qwen Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Alibaba's reasoning model from Qwen series, 1.9M+ pulls, strong math and logic",
	},

	// === META LLAMA FAMILY ===
	// Llama 3.1 Series (107.8M+ pulls) - Most popular
	{
		ID:                  "llama3.1:latest",
		Provider:            Ollama,
		DisplayName:         "Llama 3.1 8B (Most Popular)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Most downloaded model on Ollama (107M+ pulls), excellent all-around performance",
	},
	{
		ID:                  "llama3.1:70b",
		Provider:            Ollama,
		DisplayName:         "Llama 3.1 70B (Flagship)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Meta's flagship model with best quality, competitive with GPT-4",
	},
	{
		ID:                  "llama3.1:405b",
		Provider:            Ollama,
		DisplayName:         "Llama 3.1 405B (Largest Open)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Largest open-source model, requires high-end hardware but exceptional quality",
	},
	{
		ID:                  "llama3.3:70b",
		Provider:            Ollama,
		DisplayName:         "Llama 3.3 70B (Latest Quality)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Newest Llama 3.3 with quality matching 405B model (2.8M+ pulls)",
	},
	{
		ID:                  "llama3.2:latest",
		Provider:            Ollama,
		DisplayName:         "Llama 3.2 (Lightweight)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Small 1B-3B models for edge devices and rapid inference (50M+ pulls)",
	},

	// === QWEN FAMILY (ALIBABA) ===
	// Qwen 2.5 Series (18.3M+ pulls)
	{
		ID:                  "qwen2.5-coder:latest",
		Provider:            Ollama,
		DisplayName:         "Qwen 2.5 Coder (Top Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Best open-source coding model (9.3M+ pulls), excels at code generation and debugging",
	},
	{
		ID:                  "qwen2.5:32b",
		Provider:            Ollama,
		DisplayName:         "Qwen 2.5 32B (Powerful General)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "32B general purpose with 128K context, excellent multilingual support",
	},
	{
		ID:                  "qwen2.5:72b",
		Provider:            Ollama,
		DisplayName:         "Qwen 2.5 72B (Flagship)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Largest Qwen 2.5 model with best performance across all tasks",
	},
	// Qwen 3 Series (15.4M+ pulls) - Latest
	{
		ID:                  "qwen3:latest",
		Provider:            Ollama,
		DisplayName:         "Qwen 3 (Latest Generation)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Latest Qwen with MoE architecture and improved reasoning capabilities",
	},
	{
		ID:                  "qwen3-coder:latest",
		Provider:            Ollama,
		DisplayName:         "Qwen 3 Coder (Latest Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Latest coding model from Qwen 3 series (1.4M+ pulls)",
	},

	// === DEEPSEEK CODING ===
	{
		ID:                  "deepseek-coder-v2:latest",
		Provider:            Ollama,
		DisplayName:         "DeepSeek Coder V2 (Strong Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "GPT-4 level coding model with MoE architecture (1.3M+ pulls)",
	},
	{
		ID:                  "deepseek-v3:latest",
		Provider:            Ollama,
		DisplayName:         "DeepSeek V3 (MoE 671B)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Massive MoE model with 671B parameters, 37B active (3M+ pulls)",
	},

	// === GOOGLE GEMMA ===
	{
		ID:                  "gemma3:latest",
		Provider:            Ollama,
		DisplayName:         "Gemma 3 (Google Latest)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Latest Google model with vision support (28.5M+ pulls)",
	},
	{
		ID:                  "gemma2:27b",
		Provider:            Ollama,
		DisplayName:         "Gemma 2 27B (Powerful)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Largest Gemma 2 model, excellent performance (11.6M+ pulls)",
	},
	{
		ID:                  "gemma2:9b",
		Provider:            Ollama,
		DisplayName:         "Gemma 2 9B (Balanced)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Balanced 9B model with good quality and speed",
	},

	// === MISTRAL FAMILY ===
	{
		ID:                  "mistral:latest",
		Provider:            Ollama,
		DisplayName:         "Mistral 7B (Efficient)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     32000,
		Description:         "Fast and efficient 7B model (23.3M+ pulls), great balance of speed and quality",
	},
	{
		ID:                  "mistral-nemo:12b",
		Provider:            Ollama,
		DisplayName:         "Mistral Nemo 12B (128K Context)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "12B model with 128K context by Mistral + NVIDIA (3.1M+ pulls)",
	},
	{
		ID:                  "mistral-large:123b",
		Provider:            Ollama,
		DisplayName:         "Mistral Large 123B (Flagship)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Mistral's flagship with 128K context, top-tier quality (299K+ pulls)",
	},
	{
		ID:                  "mixtral:8x7b",
		Provider:            Ollama,
		DisplayName:         "Mixtral 8x7B (MoE)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     32000,
		Description:         "Mixture of Experts model with 47B params, 13B active (1.5M+ pulls)",
	},
	{
		ID:                  "mixtral:8x22b",
		Provider:            Ollama,
		DisplayName:         "Mixtral 8x22B (Large MoE)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     64000,
		Description:         "Larger MoE with 141B params, 39B active per token",
	},

	// === PHI FAMILY (MICROSOFT) ===
	{
		ID:                  "phi4:latest",
		Provider:            Ollama,
		DisplayName:         "Phi 4 14B (Microsoft Latest)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     16000,
		Description:         "Microsoft's latest small model with strong reasoning (6.6M+ pulls)",
	},
	{
		ID:                  "phi3:14b",
		Provider:            Ollama,
		DisplayName:         "Phi 3 14B (Efficient)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Lightweight 14B model with excellent quality (15.2M+ pulls)",
	},

	// === CODING SPECIALISTS ===
	{
		ID:                  "codellama:latest",
		Provider:            Ollama,
		DisplayName:         "Code Llama 7B (Meta Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     16000,
		Description:         "Meta's specialized coding model, reliable and fast (3.7M+ pulls)",
	},
	{
		ID:                  "codellama:34b",
		Provider:            Ollama,
		DisplayName:         "Code Llama 34B (Powerful Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     16000,
		Description:         "Larger Code Llama for complex coding tasks",
	},
	{
		ID:                  "starcoder2:15b",
		Provider:            Ollama,
		DisplayName:         "StarCoder2 15B (Code Generation)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     16000,
		Description:         "Open code generation model trained on 600+ languages (1.6M+ pulls)",
	},
	{
		ID:                  "granite-code:20b",
		Provider:            Ollama,
		DisplayName:         "Granite Code 20B (IBM Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     8000,
		Description:         "IBM's code intelligence model (397K+ pulls)",
	},
}

// Initial LLM Response Schema for Ollama
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
