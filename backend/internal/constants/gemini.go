package constants

import "github.com/google/generative-ai-go/genai"

var GeminiLLMModels = []LLMModel{
	// Gemini 3 Series (Latest & Most Powerful)
	{
		ID:                  "gemini-3-pro",
		Provider:            Gemini,
		DisplayName:         "Gemini 3 Pro (Most Intelligent)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 65536,
		Temperature:         1,
		InputTokenLimit:     1048576,
		Description:         "Most intelligent Gemini model with breakthrough reasoning capabilities. Best for complex coding, analysis, and agentic workflows",
	},
	{
		ID:                  "gemini-3-flash",
		Provider:            Gemini,
		DisplayName:         "Gemini 3 Flash (Frontier Speed)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 65536,
		Temperature:         1,
		InputTokenLimit:     1048576,
		Description:         "Fastest Gemini 3 model with exceptional multimodal understanding. Best for high-throughput tasks requiring speed and intelligence",
	},
	{
		ID:                  "gemini-3.1-pro-preview",
		Provider:            Gemini,
		DisplayName:         "Gemini 3.1 Pro Preview (Experimental)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 65536,
		Temperature:         1,
		InputTokenLimit:     1048576,
		Description:         "Experimental preview of Gemini 3.1 with improved thinking and token efficiency. Early access to next-generation features",
	},
	{
		ID:                  "gemini-3-flash-preview",
		Provider:            Gemini,
		DisplayName:         "Gemini 3 Flash Preview (Experimental)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 65536,
		Temperature:         1,
		InputTokenLimit:     1048576,
		Description:         "Preview version of Gemini 3 Flash with latest experimental features and performance optimizations",
	},
	// Gemini 2.5 Series (Advanced)
	{
		ID:                  "gemini-2.5-pro",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.5 Pro (Advanced Reasoning)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 65536,
		Temperature:         1,
		InputTokenLimit:     1048576,
		Description:         "State-of-the-art thinking model capable of reasoning over complex problems in code, math, and STEM",
	},
	{
		ID:                  "gemini-2.5-flash",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.5 Flash (Best Price-Performance)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		Default:             ptrBool(true), // Default model for this provider
		MaxCompletionTokens: 65536,
		Temperature:         1,
		InputTokenLimit:     1048576,
		Description:         "Best model for price-performance with well-rounded capabilities, ideal for large-scale processing and agentic tasks",
	},
	{
		ID:                  "gemini-2.5-flash-lite",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.5 Flash-Lite (Ultra-Fast)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 50000,
		Temperature:         1,
		InputTokenLimit:     1048576,
		Description:         "Fastest flash model optimized for cost-efficiency and high throughput on repetitive tasks",
	},
	// Gemini 2.0 Series (Deprecated)
	{
		ID:                  "gemini-2.0-flash",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.0 Flash (Deprecated)",
		IsEnabled:           false,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 30000,
		Temperature:         1,
		InputTokenLimit:     1048576,
		Description:         "Deprecated second generation workhorse model. Migrate to Gemini 2.5 Flash",
	},
	{
		ID:                  "gemini-2.0-flash-lite",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.0 Flash-Lite (Deprecated)",
		IsEnabled:           false,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 20000,
		Temperature:         1,
		InputTokenLimit:     1048576,
		Description:         "Deprecated second generation small workhorse model. Migrate to Gemini 2.5 Flash-Lite",
	},
}

// Initial LLM Response Schema for Gemini
var GeminiLLMResponseSchema = &genai.Schema{
	Type:     genai.TypeObject,
	Enum:     []string{},
	Required: []string{"assistantMessage"},
	Properties: map[string]*genai.Schema{
		"queries": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "An array of DB queries that the AI has generated. Return queries only when it makes sense to return a query, otherwise return empty array.",
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"query", "queryType", "isCritical", "canRollback", "explanation", "estimateResponseTime", "pagination", "exampleResultString"},
				Properties: map[string]*genai.Schema{
					"query": &genai.Schema{
						Type: genai.TypeString,
					},
					"tables": &genai.Schema{
						Type: genai.TypeString,
					},
					"queryType": &genai.Schema{
						Type: genai.TypeString,
					},
					"pagination": &genai.Schema{
						Type:     genai.TypeObject,
						Enum:     []string{},
						Required: []string{"paginatedQuery", "countQuery"},
						Properties: map[string]*genai.Schema{
							"paginatedQuery": &genai.Schema{
								Type:        genai.TypeString,
								Description: "CURSOR-BASED (preferred for SELECT/find on large data): use '{{cursor_value}}' placeholder. cursor_field MUST appear in SELECT/projection. SQL: SELECT id,name FROM users WHERE id > '{{cursor_value}}' ORDER BY id ASC LIMIT 50. MongoDB: db.users.find({createdAt:{$gt:'{{cursor_value}}'}},{name:1,createdAt:1}).sort({createdAt:1}).limit(50). OFFSET-BASED (fallback for aggregations): OFFSET offset_size LIMIT 50 or .skip(offset_size).limit(50). Set cursor_field empty for offset. EMPTY STRING when user requests < 50 records.",
							},
							"cursor_field": &genai.Schema{
								Type:        genai.TypeString,
								Description: "Field/column used as the pagination cursor (e.g. 'id', 'created_at', 'createdAt'). Must be present in SELECT/projection result. Leave EMPTY STRING for offset-based pagination.",
							},
							"page_size": &genai.Schema{
								Type:        genai.TypeNumber,
								Description: "Number of records per page. Use 50.",
							},
							"countQuery": &genai.Schema{
								Type:        genai.TypeString,
								Description: "(Only applicable for Fetching, Getting data) RULES FOR countQuery:\n1. IF the original query has a LIMIT OR the user explicitly requests a specific number of records → countQuery MUST BE EMPTY STRING\n3. OTHERWISE → provide a COUNT query with EXACTLY THE SAME filter conditions\n\nEXAMPLES:\n- Original: \"SELECT * FROM users LIMIT 5\" → countQuery: \"\"\n- Original: \"SELECT * FROM users ORDER BY created_at DESC LIMIT 10\" → countQuery: \"\"\n- Original: \"SELECT * FROM users WHERE status = 'active'\" → countQuery: \"SELECT COUNT(*) FROM users WHERE status = 'active'\"\n- Original: \"SELECT * FROM users WHERE created_at > '2023-01-01'\" → countQuery: \"SELECT COUNT(*) FROM users WHERE created_at > '2023-01-01'\"\n\nREMEMBER: The purpose of countQuery is ONLY to support pagination for large result sets. Never include OFFSET in countQuery.",
							},
						},
					},
					"isCritical": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"canRollback": &genai.Schema{
						Type: genai.TypeBoolean,
					},
					"explanation": &genai.Schema{
						Type: genai.TypeString,
					},
					"rollbackQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"estimateResponseTime": &genai.Schema{
						Type: genai.TypeNumber,
					},
					"rollbackDependentQuery": &genai.Schema{
						Type: genai.TypeString,
					},
					"exampleResultString": &genai.Schema{
						Type:        genai.TypeString,
						Description: "MUST BE VALID JSON STRING with no additional text. [{\"column1\":\"value1\",\"column2\":\"value2\"}] or {\"result\":\"1 row affected\"}. Avoid giving too much data in the exampleResultString, just give 1-2 rows of data or if there is too much data, then give only limited fields of data, if a field contains too much data, then give less data from that field",
					},
				},
			},
		},
		"actionButtons": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "List of action buttons to display to the user. Use these to suggest helpful actions like refreshing schema when schema issues are detected.",
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"label", "action", "isPrimary"},
				Properties: map[string]*genai.Schema{
					"label": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Display text for the button that the user will see.",
					},
					"action": &genai.Schema{
						Type:        genai.TypeString,
						Description: "Action identifier that will be processed by the frontend. Common actions: refresh_schema etc.",
					},
					"isPrimary": &genai.Schema{
						Type:        genai.TypeBoolean,
						Description: "Whether this is a primary (highlighted) action button.",
					},
				},
			},
		},
		"assistantMessage": &genai.Schema{
			Type: genai.TypeString,
		},
	},
}

// Recommendations Response Schema for Gemini
var GeminiRecommendationsResponseSchema = &genai.Schema{
	Type:     genai.TypeObject,
	Enum:     []string{},
	Required: []string{"recommendations"},
	Properties: map[string]*genai.Schema{
		"recommendations": &genai.Schema{
			Type:        genai.TypeArray,
			Description: "An array of 60 query recommendations (minimum 40 if 60 not possible)",
			Items: &genai.Schema{
				Type:     genai.TypeObject,
				Enum:     []string{},
				Required: []string{"text"},
				Properties: map[string]*genai.Schema{
					"text": &genai.Schema{
						Type:        genai.TypeString,
						Description: "The recommendation text that users can ask",
					},
				},
			},
		},
	},
}
