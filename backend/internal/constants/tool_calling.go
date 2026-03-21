package constants

// ============================================================================
// Tool-Calling Constants — Prompt, Response Schema, Tool Definitions
// ============================================================================

// Tool name constants used across the tool-calling system.
const (
	FinalResponseToolName = "generate_final_response"
	ExecuteQueryToolName  = "execute_read_query"
	GetTableInfoToolName  = "get_table_info"

	DefaultMaxToolIterations = 10
	MaxToolResultChars       = 4000 // Truncate tool results beyond this
)

// ToolCallingSystemPromptAddendum is appended to the base system prompt when
// iterative tool-calling mode is active. It instructs the LLM how to use tools.
const ToolCallingSystemPromptAddendum = `

===== TOOL-CALLING MODE =====
You have access to tools that let you explore the database before responding. Use them wisely:

WORKFLOW:
1. ANALYZE the user's request and the schema context provided.
2. If you need more details about specific tables, call "get_table_info".
3. If you want to verify a query works or explore data, call "execute_read_query".
4. You may call tools multiple times to refine your understanding.
5. When you're confident in your response, call "generate_final_response" with your structured answer.

RULES:
- ALWAYS finish by calling "generate_final_response". Never return plain text as your final answer.
- Keep tool usage efficient — don't make redundant calls. 1-3 tool calls is typical.
- "execute_read_query" is READ-ONLY. It will reject INSERT/UPDATE/DELETE/DROP operations.
- Include write queries (INSERT, UPDATE, DELETE, etc.) ONLY inside "generate_final_response" — do NOT try to execute them via tools.
- If the schema context already has enough info, go straight to "generate_final_response".
- NEVER refuse to query data based on its topic (financial, medical, legal, etc.). You are a database assistant — your job is to retrieve and present data from the user's database, not to give professional advice. Always explore the data first and present what you find. Let the user decide how to use it.

CRITICAL — QUERIES IN FINAL RESPONSE:
- In "generate_final_response", you MUST include ALL queries you used during exploration in the "queries" array.
- If you called "execute_read_query" with a query, that SAME query MUST appear in the "queries" array of your final response.
- The "queries" array is how the user sees, re-runs, and modifies queries — it is NOT just for executio, so try to include the final response query there.
- NEVER leave "queries" empty if you executed any queries during the conversation, but give the final query in the queries, not the tool call ones which helped you gather details. 
- Include the query text, explanation, referenced tables/collections, query type, and example results from what you observed.
===== END TOOL-CALLING MODE =====
`

// ToolFinalResponseSchema is the JSON Schema for the generate_final_response tool.
// This mirrors the LLMResponse structure so the tool-calling loop returns
// the same structured data as the original single-shot flow.
var ToolFinalResponseSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"queries": map[string]interface{}{
			"type":        "array",
			"description": "Array of the final database queries relevant to the user's request. MUST only include the final queries that are part of the structured response not the tool call ones which helped you gather details. Only leave empty if genuinely no queries are involved (e.g. purely informational questions about database concepts).",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The database query to execute.",
					},
					"tables": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Tables referenced by this query.",
					},
					"collection": map[string]interface{}{
						"type":        "string",
						"description": "MongoDB collection name (if applicable).",
					},
					"queryType": map[string]interface{}{
						"type":        "string",
						"description": "Type of query: SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, AGGREGATE, FIND, etc.",
					},
					"pagination": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"totalRecordsCount": map[string]interface{}{
								"type":        "integer",
								"description": "Total number of records the query would return without pagination.",
							},
							"paginatedQuery": map[string]interface{}{
								"type":        "string",
								"description": "This is the query for SUBSEQUENT PAGES (page 2, 3, etc) — NOT for the first page. The 'query' field above is used for the first page and MUST NOT contain {{cursor_value}}. The query with LIMIT/OFFSET or cursor placeholder applied for pagination.",
							},
							"countQuery": map[string]interface{}{
								"type":        "string",
								"description": "Query to count total records.",
							},
						},
					},
					"isCritical": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether this is a destructive/critical operation (DELETE, DROP, TRUNCATE, etc.).",
					},
					"canRollback": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether this operation can be rolled back.",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "Human-readable explanation of what this query does.",
					},
					"exampleResultString": map[string]interface{}{
						"type":        "string",
						"description": "JSON string showing example/expected result rows.",
					},
					"rollbackQuery": map[string]interface{}{
						"type":        "string",
						"description": "Query to undo this operation (if canRollback is true).",
					},
					"rollbackDependentQuery": map[string]interface{}{
						"type":        "string",
						"description": "Query that must run before the rollback query.",
					},
					"estimateResponseTime": map[string]interface{}{
						"type":        "number",
						"description": "Estimated execution time in seconds.",
					},
				},
				"required": []interface{}{"query"},
			},
		},
		"assistantMessage": map[string]interface{}{
			"type":        "string",
			"description": "A friendly, clear explanation or response message for the user. Always required.",
		},
		"actionButtons": map[string]interface{}{
			"type":        "array",
			"description": "Suggested follow-up actions the user can take.",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"label": map[string]interface{}{
						"type":        "string",
						"description": "Button label text.",
					},
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "The prompt to send when the user clicks this button.",
					},
					"isPrimary": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether this is a primary (highlighted) action.",
					},
				},
				"required": []interface{}{"label", "prompt"},
			},
		},
	},
	"required": []interface{}{"assistantMessage"},
}

// ExecuteQueryToolSchema is the parameter schema for the execute_read_query tool.
var ExecuteQueryToolSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "The read-only query to execute. Must be a SELECT, SHOW, DESCRIBE, EXPLAIN, or equivalent read operation as per the database.",
		},
		"explanation": map[string]interface{}{
			"type":        "string",
			"description": "Brief explanation of what this query does and why you're running it (for logging).",
		},
	},
	"required": []interface{}{"query"},
}

// GetTableInfoToolSchema is the parameter schema for the get_table_info tool.
var GetTableInfoToolSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"table_names": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Names of the tables or collections to inspect.",
		},
	},
	"required": []interface{}{"table_names"},
}
