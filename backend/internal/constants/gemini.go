package constants

import "github.com/google/generative-ai-go/genai"

var GeminiLLMModels = []LLMModel{
	{
		ID:                  "gemini-3-pro-preview",
		Provider:            Gemini,
		DisplayName:         "Gemini 3 Pro (Most Intelligent)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Best model in the world for multimodal understanding with state-of-the-art reasoning and agentic capabilities",
	},
	{
		ID:                  "gemini-3-flash-preview",
		Provider:            Gemini,
		DisplayName:         "Gemini 3 Flash (Frontier Speed)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Most intelligent model built for speed, combining frontier intelligence with superior search and grounding",
	},
	// Gemini 2.5 Series (Advanced)
	{
		ID:                  "gemini-2.5-pro",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.5 Pro (Advanced Reasoning)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "State-of-the-art thinking model capable of reasoning over complex problems in code, math, and STEM",
	},
	{
		ID:                  "gemini-2.5-flash",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.5 Flash (Best Price-Performance)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     1000000,
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
		InputTokenLimit:     1000000,
		Description:         "Fastest flash model optimized for cost-efficiency and high throughput on repetitive tasks",
	},
	// Gemini 2.0 Series (Previous Workhorse)
	{
		ID:                  "gemini-2.0-flash",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.0 Flash (Workhorse)",
		IsEnabled:           true,
		Default:             ptrBool(true),
		APIVersion:          "v1beta",
		MaxCompletionTokens: 30000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Second generation workhorse model with 1M token context window for large document processing",
	},
	{
		ID:                  "gemini-2.0-flash-lite",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.0 Flash-Lite (Previous Fast)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 20000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Second generation small workhorse model with 1M token context, lightweight version",
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
								Type: genai.TypeString,
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
