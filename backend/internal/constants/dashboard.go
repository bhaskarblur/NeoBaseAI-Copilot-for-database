package constants

import "time"

// ============================================================================
// Dashboard Tool-Calling Constants — Prompts, Response Schemas, Tool Definitions
// ============================================================================

// Dashboard tool name constants
const (
	DashboardFinalResponseToolName    = "generate_final_response"
	DashboardExecuteQueryToolName     = "execute_dashboard_query" // Read-only queries for widget data validation
	DashboardGetTableInfoToolName     = "get_table_info"          // Reuse existing tool
	DashboardGetKnowledgeBaseToolName = "get_knowledge_base"      // Get KB table/field descriptions

	DashboardMaxToolIterations  = 25 // Dashboard generation may need more iterations
	DashboardMaxToolResultChars = 4000
)

// Dashboard limits and configuration constants
const (
	MaxWidgetsPerDashboard    = 12 // Max widgets allowed per dashboard
	MaxDashboardsPerChatFree  = 5  // Max dashboards per chat for free tier
	MaxDashboardsPerChatPaid  = 10 // Max dashboards per chat for paid tier
	MaxBlueprintSuggestions   = 4  // Max blueprint suggestions generated
	WidgetQueryTimeoutSeconds = 5  // Per-widget query timeout in seconds
)

// Dashboard generation modes
const (
	DashboardModeGenerate   = "generate"    // Full dashboard generation from prompt
	DashboardModeBlueprint  = "blueprint"   // Lightweight blueprint suggestions (no queries)
	DashboardModeRegenerate = "regenerate"  // Regenerate existing dashboard
	DashboardModeAddWidget  = "add_widget"  // Add a new widget to existing dashboard
	DashboardModeEditWidget = "edit_widget" // Edit an existing widget
)

// Regeneration reasons
const (
	RegenerateReasonVariant      = "try_another_variant"
	RegenerateReasonSchemaChange = "schema_changed"
)

// DashboardBlueprintSystemPrompt instructs the LLM to generate lightweight dashboard blueprints
const DashboardBlueprintSystemPrompt = `
===== DASHBOARD BLUEPRINT MODE =====
You are an AI dashboard planner. Your job is to discover the database structure and suggest dashboard blueprints.

IMPORTANT: The database schema is NOT provided upfront. You MUST use tool calls to discover it.

WORKFLOW:
1. Use "execute_dashboard_query" to list all tables/collections (e.g., SHOW TABLES, SELECT table_name FROM information_schema.tables, db.getCollectionNames(), etc. — adapt the query for the DB type).
2. Call "get_knowledge_base" to get human-readable descriptions of tables and fields (if available).
3. Call "get_table_info" for the most important/interesting tables to understand their columns and types.
4. Based on what you discover, call "generate_final_response" with your blueprint suggestions.

RULES:
- ALWAYS start by discovering tables using "execute_dashboard_query" — do NOT guess table names
- Optionally call "get_knowledge_base" to understand what the tables represent
- Suggest up to 4 DIFFERENT dashboard concepts (e.g., "Sales Overview", "User Analytics", "Data Health")
- Each blueprint should have a unique focus area — do NOT create overlapping dashboards
- For each blueprint, provide: name, description, and a list of proposed widget titles + types
- Do NOT write actual widget queries — this is a planning phase only
- Each blueprint should have 4-8 proposed widgets
- Always include 2-4 stat cards at the top for immediate KPIs
- Include at least one time-series chart if temporal data exists
- If the schema is very simple, suggest fewer blueprints (even just 1-2 is fine)
- Widget types: "stat", "line", "bar", "area", "pie", "table", "combo", "gauge", "bar_gauge", "heatmap", "histogram"
  * stat: Single KPI value card, optionally with comparison (e.g., Total Revenue: $45K, +12% vs yesterday)
  * line/area: Time-series trends etc
  * bar: Categorical comparisons, top-N, distributions
  * pie: Proportions/distributions etc
  * table: Detailed records, drill-downs etc
  * gauge: Radial gauge (speedometer) for % or ratio metrics etc
  * bar_gauge: Horizontal/vertical progress bars for thresholds etc
  * heatmap: 2D patterns over time/categories (activity, errors by hour) etc
  * histogram: Value distribution (age ranges, price buckets, latency distribution) etc
===== END DASHBOARD BLUEPRINT MODE =====`

// DashboardGenerationSystemPrompt instructs the LLM to generate a full dashboard
const DashboardGenerationSystemPrompt = `
===== DASHBOARD BUILDER MODE =====
You are an AI dashboard builder. You create data dashboards with widgets that display KPIs, charts, and tables from the user's database.

IMPORTANT: The database schema is NOT provided upfront. You MUST use tool calls to discover it and test queries.

WORKFLOW:
1. Use "execute_dashboard_query" to list all tables/collections (e.g., SHOW TABLES, SELECT table_name FROM information_schema.tables, db.getCollectionNames(), etc. — adapt the query for the DB type).
2. Call "get_knowledge_base" to understand table purposes and field meanings (if available).
3. Call "get_table_info" for tables relevant to the dashboard being built.
4. Write queries and test them with "execute_dashboard_query". If a query fails, fix it and retry.
5. Once all queries are tested and working, call "generate_final_response" with the complete config.

WIDGET TYPES:
- "stat": Single KPI value with optional comparison (e.g., Total Revenue: $45K, +12%)
- "line": Time-series trend chart (requires x-axis date/time column)
- "bar": Categorical comparison chart (top-N, distribution)
- "area": Time-series with filled area (similar to line but with fill)
- "pie": Distribution/proportion chart (max 8-10 slices)
- "table": Tabular data display (recent records, top-N lists)
- "combo": Mixed chart types (bar + line overlay)
- "gauge": Radial gauge (speedometer-style) showing value relative to min/max
  * Perfect for: % completion, capacity utilization, success rate, health score
  * Requires: Single numeric value between min (0) and max (100 default)
  * Config: min, max, thresholds for color zones, decimal_places, unit
- "bar_gauge": Horizontal or vertical bar showing progress toward threshold
  * Perfect for: Progress bars, multi-series comparisons, quota tracking
  * Requires: Single or multiple numeric values with min/max range
  * Config: orientation (horizontal/vertical), display_mode (basic/lcd/gradient), thresholds
- "heatmap": 2D visualization showing magnitude/intensity over time or categories
  * Perfect for: Activity patterns by hour/day, error rates over time, user engagement heatmap
  * Requires: 3 columns: x-axis (time/category), y-axis (category), value (metric)
  * Config: x_axis_column, y_axis_column, value_column, color_scheme, bucket_size
- "histogram": Distribution visualization showing how values are distributed across ranges
  * Perfect for: Age distribution, price ranges, response time distribution, data quality checks
  * Requires: Single numeric column to segment into buckets
  * Config: value_column, bucket_count (num of bins), show_mean, show_median

RULES:
- ALWAYS start by discovering tables using "execute_dashboard_query" — do NOT guess table/column names
- Create 4-8 widgets for a balanced dashboard
- Always include 2-4 stat cards or gauges at the top for immediate KPIs
- Use gauges for percentage/ratio metrics (completion %, utilization %, success rate)
- Use bar gauges for progress tracking or threshold monitoring
- Use heatmaps when you have time-based patterns or 2D categorical data
- Use histograms to show data distribution and detect outliers
- Include at least one time-series chart (line/area) if temporal data exists
- Include at least one table widget for detail/drill-down
- All queries MUST be READ-ONLY (SELECT, FIND, AGGREGATE only)
- Optimize queries for performance (use LIMIT, appropriate indexes)
- ALWAYS test your queries with "execute_dashboard_query" before including them
- If a query fails, FIX IT and test again — do not include broken queries
- For stat cards, provide separate value_query and comparison_query
- For chart widgets, ensure queries return data suitable for the chart type
- For table widgets:
  * Use CURSOR-BASED pagination (more efficient than OFFSET for large datasets)
  * Identify a suitable cursor field: primary key (id, uuid) or timestamp (created_at, updated_at)
  * Order results by the cursor field for consistent pagination
  * Set page_size (default 25, max 100)
  * Example PostgreSQL: "SELECT * FROM users WHERE id > {{cursor_value}} ORDER BY id ASC LIMIT 25"
  * Example MongoDB: db.users.find({_id: {$gt: ObjectId("{{cursor_value}}")}}).sort({_id: 1}).limit(25)
  * Example MySQL: "SELECT * FROM orders WHERE created_at > '{{cursor_value}}' ORDER BY created_at ASC LIMIT 25"
  * The {{cursor_value}} placeholder will be replaced at runtime for pagination

LAYOUT GUIDELINES:
- Stat cards: w=3, h=2 (4 cards in a row on 12-column grid)
- Line/Area/Combo charts: w=12 or w=6, h=4 (full or half width)
- Bar/Pie charts: w=6, h=4 (half width)
- Table widgets: w=12 or w=6, h=4 (full or half width)
===== END DASHBOARD BUILDER MODE =====
`

// DashboardWidgetEditSystemPrompt is used when editing a single widget via AI
const DashboardWidgetEditSystemPrompt = `
===== WIDGET EDIT MODE =====
You are editing a single dashboard widget. The user wants to modify this widget based on their instruction.

CURRENT WIDGET:
- Title: {{WIDGET_TITLE}}
- Type: {{WIDGET_TYPE}}
- Query: {{WIDGET_QUERY}}

WORKFLOW:
1. Understand the user's edit request.
2. If needed, use "execute_dashboard_query" to list tables and "get_table_info" to understand available data.
3. Use "execute_dashboard_query" to test your modified query.
4. Call "generate_final_response" with the updated widget configuration.

RULES:
- Only modify what the user asks — keep everything else the same
- The response should contain exactly ONE widget (the updated one)
- ALWAYS test the new query with "execute_dashboard_query"
- All queries must be READ-ONLY
- For table widgets, ALWAYS use cursor-based pagination:
  * The query MUST use a {{cursor_value}} placeholder and LIMIT 25 — never use a fixed .limit(N) larger than 25
  * Set table_config.cursor_field to the cursor column (e.g. "_id", "id", "created_at")
  * Set table_config.page_size to 25
  * PostgreSQL example: SELECT * FROM users WHERE id > '{{cursor_value}}' ORDER BY id ASC LIMIT 25
  * MongoDB example: db.users.find({_id: {$gt: ObjectId("{{cursor_value}}") }}).sort({_id: 1}).limit(25)
  * MySQL example: SELECT * FROM orders WHERE created_at > '{{cursor_value}}' ORDER BY created_at ASC LIMIT 25
===== END WIDGET EDIT MODE =====
`

// DashboardAddWidgetSystemPrompt is used when adding a new widget via AI prompt
const DashboardAddWidgetSystemPrompt = `
===== ADD WIDGET MODE =====
You are adding a new widget to an existing dashboard. The user describes what data they want to see.

EXISTING DASHBOARD: {{DASHBOARD_NAME}}
EXISTING WIDGETS: {{EXISTING_WIDGET_TITLES}}

WORKFLOW:
1. Use "execute_dashboard_query" to list all tables/collections (adapt query for the DB type).
2. Call "get_table_info" for relevant tables to understand their structure.
3. Optionally call "get_knowledge_base" to understand field meanings.
4. Write and test your query with "execute_dashboard_query".
5. Call "generate_final_response" with exactly ONE new widget.

RULES:
- ALWAYS discover tables using "execute_dashboard_query" — do NOT guess table/column names
- Create exactly ONE widget matching the user's request
- Choose the most appropriate widget type for the data
- ALWAYS test the query with "execute_dashboard_query"
- All queries must be READ-ONLY
- The widget should complement the existing dashboard, not duplicate
- For table widgets, ALWAYS use cursor-based pagination:
  * The query MUST use a {{cursor_value}} placeholder and LIMIT 25 — never use a fixed .limit(N) larger than 25
  * Set table_config.cursor_field to the cursor column (e.g. "_id", "id", "created_at")
  * Set table_config.page_size to 25
  * PostgreSQL example: SELECT * FROM users WHERE id > '{{cursor_value}}' ORDER BY id ASC LIMIT 25
  * MongoDB example: db.users.find({_id: {$gt: ObjectId("{{cursor_value}}") }}).sort({_id: 1}).limit(25)
  * MySQL example: SELECT * FROM orders WHERE created_at > '{{cursor_value}}' ORDER BY created_at ASC LIMIT 25
===== END ADD WIDGET MODE =====
`

// === Tool Response Schemas ===

// DashboardBlueprintResponseSchema is the JSON Schema for blueprint suggestions
var DashboardBlueprintResponseSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"blueprints": map[string]interface{}{
			"type":        "array",
			"description": "Array of dashboard blueprint suggestions (max 4)",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Dashboard name (e.g., 'E-Commerce Overview', 'User Analytics')",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Brief description of what this dashboard shows",
					},
					"template_type": map[string]interface{}{
						"type":        "string",
						"description": "Template category: ecommerce, user_analytics, db_health, financial, iot, saas, data_pipeline, custom",
					},
					"proposed_widgets": map[string]interface{}{
						"type":        "array",
						"description": "Proposed widgets for this dashboard (title + type only, no queries)",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"title": map[string]interface{}{
									"type":        "string",
									"description": "Widget title (e.g., 'Total Revenue', 'Daily Active Users')",
								},
								"widget_type": map[string]interface{}{
									"type":        "string",
									"enum":        []interface{}{"stat", "line", "bar", "area", "pie", "table", "combo"},
									"description": "Widget visualization type",
								},
							},
							"required": []interface{}{"title", "widget_type"},
						},
					},
				},
				"required": []interface{}{"name", "description", "template_type", "proposed_widgets"},
			},
		},
	},
	"required": []interface{}{"blueprints"},
}

// DashboardFinalResponseSchema is the JSON Schema for full dashboard generation
var DashboardFinalResponseSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"dashboard_name": map[string]interface{}{
			"type":        "string",
			"description": "A descriptive name for the dashboard",
		},
		"dashboard_description": map[string]interface{}{
			"type":        "string",
			"description": "Brief description of what this dashboard monitors",
		},
		"widgets": map[string]interface{}{
			"type":        "array",
			"description": "Array of widget configurations",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Widget display title",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Brief description of what this widget shows",
					},
					"widget_type": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"stat", "line", "bar", "area", "pie", "table", "combo"},
						"description": "Widget visualization type",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The SQL/MongoDB query for this widget's data",
					},
					"tables": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated table names referenced by the query",
					},
					"stat_config": map[string]interface{}{
						"type":        "object",
						"description": "Configuration for stat (KPI) widgets only",
						"properties": map[string]interface{}{
							"value_query": map[string]interface{}{
								"type":        "string",
								"description": "Query returning a single value for the KPI",
							},
							"comparison_query": map[string]interface{}{
								"type":        "string",
								"description": "Query for comparison value (e.g., yesterday's count)",
							},
							"format": map[string]interface{}{
								"type":        "string",
								"enum":        []interface{}{"number", "currency", "percentage", "duration"},
								"description": "How to format the value",
							},
							"prefix": map[string]interface{}{
								"type":        "string",
								"description": "Value prefix (e.g., '$', '€')",
							},
							"suffix": map[string]interface{}{
								"type":        "string",
								"description": "Value suffix (e.g., '%', 'ms')",
							},
							"decimal_places": map[string]interface{}{
								"type":        "integer",
								"description": "Number of decimal places to show",
							},
							"trend_direction": map[string]interface{}{
								"type":        "string",
								"enum":        []interface{}{"up_is_good", "down_is_good"},
								"description": "Whether an upward trend is positive or negative",
							},
						},
					},
					"chart_config": map[string]interface{}{
						"type":        "object",
						"description": "Chart rendering configuration (for line, bar, area, pie, combo widgets)",
						"properties": map[string]interface{}{
							"chart_type": map[string]interface{}{
								"type": "string",
								"enum": []interface{}{"line", "bar", "area", "pie", "scatter", "combo"},
							},
							"x_axis": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"data_key": map[string]interface{}{"type": "string"},
									"label":    map[string]interface{}{"type": "string"},
									"type":     map[string]interface{}{"type": "string", "enum": []interface{}{"date", "category", "number"}},
									"format":   map[string]interface{}{"type": "string"},
								},
							},
							"y_axis": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"data_key": map[string]interface{}{"type": "string"},
									"label":    map[string]interface{}{"type": "string"},
									"type":     map[string]interface{}{"type": "string"},
								},
							},
							"series": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"data_key": map[string]interface{}{"type": "string"},
										"name":     map[string]interface{}{"type": "string"},
										"type":     map[string]interface{}{"type": "string"},
										"stroke":   map[string]interface{}{"type": "string"},
										"fill":     map[string]interface{}{"type": "string"},
									},
								},
							},
							"pie": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"data_key":     map[string]interface{}{"type": "string"},
									"name_key":     map[string]interface{}{"type": "string"},
									"inner_radius": map[string]interface{}{"type": "integer"},
								},
							},
							"colors": map[string]interface{}{
								"type":  "array",
								"items": map[string]interface{}{"type": "string"},
							},
						},
					},
					"table_config": map[string]interface{}{
						"type":        "object",
						"description": "Configuration for table widgets only",
						"properties": map[string]interface{}{
							"columns": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"key":    map[string]interface{}{"type": "string"},
										"label":  map[string]interface{}{"type": "string"},
										"format": map[string]interface{}{"type": "string", "enum": []interface{}{"text", "number", "date", "currency"}},
										"width":  map[string]interface{}{"type": "string"},
									},
									"required": []interface{}{"key", "label"},
								},
							},
							"sort_by":        map[string]interface{}{"type": "string"},
							"sort_direction": map[string]interface{}{"type": "string", "enum": []interface{}{"asc", "desc"}},
							"page_size":      map[string]interface{}{"type": "integer", "description": "Rows per page. MUST be 25 for table widgets."},
							"cursor_field":   map[string]interface{}{"type": "string", "description": "REQUIRED for table widgets: the column used as the pagination cursor (e.g. '_id', 'id', 'created_at'). The query MUST contain a '{{cursor_value}}' placeholder filtered on this field."},
						},
					},
					"layout": map[string]interface{}{
						"type":        "object",
						"description": "Grid layout configuration",
						"properties": map[string]interface{}{
							"w": map[string]interface{}{"type": "integer", "description": "Width in grid units (1-12)"},
							"h": map[string]interface{}{"type": "integer", "description": "Height in grid units (2-6)"},
						},
					},
				},
				"required": []interface{}{"title", "widget_type", "query"},
			},
		},
		"suggested_refresh_interval": map[string]interface{}{
			"type":        "integer",
			"description": "Suggested refresh interval in seconds (15, 30, 60, 300, 600, 3600)",
		},
		"suggested_time_range": map[string]interface{}{
			"type":        "string",
			"description": "Suggested time range for queries (1h, 6h, 24h, 7d, 30d)",
		},
	},
	"required": []interface{}{"dashboard_name", "widgets"},
}

// DashboardExecuteQueryToolSchema is the parameter schema for execute_dashboard_query
var DashboardExecuteQueryToolSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "The read-only query to execute for testing. Must be SELECT, SHOW, DESCRIBE, EXPLAIN, or equivalent.",
		},
		"explanation": map[string]interface{}{
			"type":        "string",
			"description": "Brief explanation of what this query tests and why.",
		},
	},
	"required": []interface{}{"query"},
}

// DashboardGetSchemaToolSchema is the parameter schema for get_schema_summary
var DashboardGetSchemaToolSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"include_sample_data": map[string]interface{}{
			"type":        "boolean",
			"description": "Whether to include a few sample rows from each table (slower but more informative).",
		},
	},
}

// DashboardGetKnowledgeBaseToolSchema is the parameter schema for get_knowledge_base
var DashboardGetKnowledgeBaseToolSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"table_names": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Optional: specific table names to get descriptions for. If omitted, returns all table descriptions.",
		},
	},
}

// SSE Event type constants for dashboard events
const (
	SSEEventDashboardBlueprints         = "dashboard-blueprints"
	SSEEventDashboardGenerationProgress = "dashboard-generation-progress"
	SSEEventDashboardGenerationComplete = "dashboard-generation-complete"
	SSEEventDashboardWidgetData         = "dashboard-widget-data"
	SSEEventDashboardWidgetError        = "dashboard-widget-error"
)

// Blueprint Redis cache constants
const (
	DashboardBlueprintCacheTTL    = 10 * time.Minute        // Blueprints expire after 10 minutes
	DashboardBlueprintCachePrefix = "dashboard:blueprints:" // Redis key prefix for blueprint cache
)

// GetDashboardDBInstructions returns database-type-specific instructions for dashboard query generation.
// These are appended to the system prompt so the LLM writes correct syntax for the target database.
func GetDashboardDBInstructions(dbType string) string {
	switch dbType {
	case DatabaseTypePostgreSQL, DatabaseTypeYugabyteDB:
		return `
DATABASE-SPECIFIC INSTRUCTIONS (PostgreSQL/YugabyteDB):
- Write standard SQL queries using PostgreSQL syntax.
- Use double-quoted identifiers for case-sensitive names: "TableName"."ColumnName"
- Use single quotes for string literals: 'value'
- Use LIMIT/OFFSET for pagination. Default LIMIT 50 for table widgets.
- Use NOW() and INTERVAL for time-based filtering: WHERE created_at >= NOW() - INTERVAL '7 days'
- Use EXTRACT(epoch FROM ...) for duration calculations.
- Use COUNT(*), SUM(), AVG(), MIN(), MAX() for aggregations.
- Use DATE_TRUNC('day', col) for grouping by date periods.
- Use COALESCE(col, default) for null handling.
- Use TO_CHAR(col, 'YYYY-MM-DD') for date formatting.
- JOINs are preferred over subqueries for readability and performance.
- All queries MUST be SELECT-only (read-only).
`
	case DatabaseTypeMySQL:
		return `
DATABASE-SPECIFIC INSTRUCTIONS (MySQL):
- Write standard SQL queries using MySQL syntax.
- Use backtick-quoted identifiers for reserved words: ` + "`table`.`column`" + `
- Use single quotes for string literals: 'value'
- Use LIMIT for pagination. Default LIMIT 50 for table widgets.
- Use NOW() and INTERVAL for time-based filtering: WHERE created_at >= NOW() - INTERVAL 7 DAY
- Use UNIX_TIMESTAMP() for epoch conversions.
- Use COUNT(*), SUM(), AVG(), MIN(), MAX() for aggregations.
- Use DATE_FORMAT(col, '%Y-%m-%d') for date formatting.
- Use IFNULL(col, default) or COALESCE(col, default) for null handling.
- Use DATE(col) or DATE_FORMAT(col, '%Y-%m-%d') for grouping by date.
- JOINs are preferred over subqueries.
- All queries MUST be SELECT-only (read-only).
`
	case DatabaseTypeMongoDB:
		return `
DATABASE-SPECIFIC INSTRUCTIONS (MongoDB):
- Write MongoDB aggregation pipelines or find queries.
- Format: db.collection.aggregate([...stages...]) or db.collection.find({...})
- Each pipeline stage MUST be a separate object: [{"$match": {...}}, {"$group": {...}}]
- Use $$NOW (double dollar) for system date variables, NOT $NOW.
- Always quote field names in all objects: {"$match": {"status": "active"}}
- For date comparisons in $match, use $expr: {"$match": {"$expr": {"$gte": ["$date", {"$dateSubtract": {"startDate": "$$NOW", "unit": "day", "amount": 7}}]}}}
- Use $group with _id for aggregations: {"$group": {"_id": "$category", "count": {"$sum": 1}}}
- Use $sort for ordering: {"$sort": {"count": -1}}
- Use $limit for pagination: {"$limit": 50}
- Use $project to reshape output fields.
- For stat widgets, pipeline should return a single document with the value.
- All operations MUST be read-only (find/aggregate only, no update/delete/insert).
`
	case DatabaseTypeClickhouse:
		return `
DATABASE-SPECIFIC INSTRUCTIONS (ClickHouse):
- Write standard SQL queries using ClickHouse SQL dialect.
- Use double-quoted identifiers for names: "table"."column"
- Use single quotes for string literals: 'value'
- Use LIMIT for pagination. Default LIMIT 50 for table widgets.
- Use now() and INTERVAL for time filtering: WHERE created_at >= now() - INTERVAL 7 DAY
- Use toDate(), toDateTime(), toStartOfDay(), toStartOfWeek(), toStartOfMonth() for date grouping.
- Use formatDateTime(col, '%Y-%m-%d') for date formatting.
- Use count(), sum(), avg(), min(), max(), uniq() for aggregations.
- Use ifNull(col, default) for null handling.
- Use FORMAT JSON or FORMAT JSONEachRow when you need specific output format.
- Use FINAL after table name for ReplacingMergeTree tables.
- Avoid JOINs on large tables — prefer subqueries or pre-aggregated tables.
- All queries MUST be SELECT-only (read-only).
`
	default:
		// Generic SQL instructions for unsupported types
		return `
DATABASE-SPECIFIC INSTRUCTIONS (SQL):
- Write standard SQL queries.
- Use LIMIT for pagination. Default LIMIT 50 for table widgets.
- Use COUNT(*), SUM(), AVG(), MIN(), MAX() for aggregations.
- All queries MUST be SELECT-only (read-only).
`
	}
}
