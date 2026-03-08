package dtos

// === Dashboard Request DTOs ===

// CreateDashboardRequest is used when creating a dashboard via AI generation
type CreateDashboardRequest struct {
	Prompt string `json:"prompt" binding:"required"` // User's natural language description or "template:<type>"
}

// UpdateDashboardRequest is used when updating dashboard metadata
type UpdateDashboardRequest struct {
	Name            *string            `json:"name,omitempty"`
	Description     *string            `json:"description,omitempty"`
	RefreshInterval *int               `json:"refresh_interval,omitempty"` // Seconds: 0 = manual, 15, 30, 60, 300, 600, 3600
	TimeRange       *string            `json:"time_range,omitempty"`       // "1h", "6h", "24h", "7d", "30d"
	Layout          *[]WidgetLayoutDTO `json:"layout,omitempty"`
	IsDefault       *bool              `json:"is_default,omitempty"`
}

// RegenerateDashboardRequest is used when regenerating a dashboard
type RegenerateDashboardRequest struct {
	Reason             string `json:"reason" binding:"required"`     // "try_another_variant" or "schema_changed"
	CustomInstructions string `json:"custom_instructions,omitempty"` // Optional user instructions for regeneration
}

// AddWidgetRequest is used when adding a widget to a dashboard via AI
type AddWidgetRequest struct {
	Prompt string `json:"prompt" binding:"required"` // Natural language description for the widget
}

// EditWidgetRequest is used when editing a widget via AI
type EditWidgetRequest struct {
	Prompt string `json:"prompt" binding:"required"` // Natural language edit instruction
}

// GenerateBlueprintsRequest triggers AI blueprint generation
type GenerateBlueprintsRequest struct {
	// No body needed — uses chat's schema and KB context
}

// CreateFromBlueprintsRequest creates dashboards from selected blueprints
type CreateFromBlueprintsRequest struct {
	BlueprintIndices []int `json:"blueprint_indices" binding:"required"` // Indices of selected blueprints (0-based)
}

// RefreshDashboardRequest triggers a manual refresh of all widgets
type RefreshDashboardRequest struct {
	// No body needed — refreshes all widgets
}

// === Dashboard Response DTOs ===

// DashboardResponse is the API response for a dashboard
type DashboardResponse struct {
	ID              string            `json:"id"`
	ChatID          string            `json:"chat_id"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	TemplateType    string            `json:"template_type,omitempty"`
	IsDefault       bool              `json:"is_default"`
	RefreshInterval int               `json:"refresh_interval"`
	TimeRange       string            `json:"time_range"`
	Layout          []WidgetLayoutDTO `json:"layout"`
	Widgets         []WidgetResponse  `json:"widgets"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
}

// DashboardListItem is a lightweight dashboard response for list endpoint
type DashboardListItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	TemplateType string `json:"template_type,omitempty"`
	IsDefault    bool   `json:"is_default"`
	WidgetCount  int    `json:"widget_count"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// WidgetLayoutDTO is the API representation of a widget's grid layout
type WidgetLayoutDTO struct {
	WidgetID string `json:"widget_id"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	W        int    `json:"w"`
	H        int    `json:"h"`
	MinW     int    `json:"min_w,omitempty"`
	MinH     int    `json:"min_h,omitempty"`
}

// WidgetResponse is the API response for a widget
type WidgetResponse struct {
	ID              string                `json:"id"`
	DashboardID     string                `json:"dashboard_id"`
	Title           string                `json:"title"`
	Description     string                `json:"description,omitempty"`
	WidgetType      string                `json:"widget_type"`
	Query           string                `json:"query"`
	QueryType       string                `json:"query_type,omitempty"`
	Tables          string                `json:"tables,omitempty"`
	ChartConfigJSON string                `json:"chart_config_json,omitempty"`
	StatConfig      *StatWidgetConfigDTO  `json:"stat_config,omitempty"`
	TableConfig     *TableWidgetConfigDTO `json:"table_config,omitempty"`
	LastRefreshedAt string                `json:"last_refreshed_at,omitempty"`
	GeneratedPrompt string                `json:"generated_prompt,omitempty"`
	CreatedAt       string                `json:"created_at"`
	UpdatedAt       string                `json:"updated_at"`
}

// StatWidgetConfigDTO for single-value KPI widgets
type StatWidgetConfigDTO struct {
	ValueQuery      string `json:"value_query"`
	ComparisonQuery string `json:"comparison_query,omitempty"`
	Format          string `json:"format,omitempty"`
	Prefix          string `json:"prefix,omitempty"`
	Suffix          string `json:"suffix,omitempty"`
	DecimalPlaces   int    `json:"decimal_places,omitempty"`
	TrendDirection  string `json:"trend_direction,omitempty"`
}

// TableWidgetConfigDTO for tabular data widgets
type TableWidgetConfigDTO struct {
	Columns       []TableWidgetColumnDTO `json:"columns"`
	SortBy        string                 `json:"sort_by,omitempty"`
	SortDirection string                 `json:"sort_direction,omitempty"`
	PageSize      int                    `json:"page_size,omitempty"`
}

// TableWidgetColumnDTO defines a column in a table widget
type TableWidgetColumnDTO struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Format string `json:"format,omitempty"`
	Width  string `json:"width,omitempty"`
}

// === SSE Event Data DTOs ===

// DashboardBlueprintEvent is sent via SSE during recommendation generation
type DashboardBlueprintEvent struct {
	Blueprints []BlueprintDTO `json:"blueprints"`
}

// BlueprintDTO is a lightweight dashboard preview
type BlueprintDTO struct {
	Index           int                  `json:"index"` // Selection index
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	TemplateType    string               `json:"template_type"`
	ProposedWidgets []BlueprintWidgetDTO `json:"proposed_widgets"`
}

// BlueprintWidgetDTO is a widget preview within a blueprint
type BlueprintWidgetDTO struct {
	Title      string `json:"title"`
	WidgetType string `json:"widget_type"`
}

// DashboardGenerationProgressEvent is sent via SSE during dashboard creation
type DashboardGenerationProgressEvent struct {
	DashboardID string `json:"dashboard_id,omitempty"` // Set once dashboard is created
	Status      string `json:"status"`                 // "generating", "testing_queries", "finalizing"
	Message     string `json:"message"`
	Progress    int    `json:"progress"` // 0-100
}

// DashboardGenerationCompleteEvent is sent via SSE when dashboard creation finishes
type DashboardGenerationCompleteEvent struct {
	DashboardID string            `json:"dashboard_id"`
	Dashboard   DashboardResponse `json:"dashboard"`
}

// DashboardWidgetDataEvent is sent via SSE for individual widget data refresh
type DashboardWidgetDataEvent struct {
	WidgetID        string                   `json:"widget_id"`
	Data            []map[string]interface{} `json:"data"`
	RowCount        int                      `json:"row_count"`
	ExecutionTimeMs float64                  `json:"execution_time_ms"`
	Error           string                   `json:"error,omitempty"`
}

// === Import/Export DTOs ===

// ValidateImportRequest validates import JSON before actual import
type ValidateImportRequest struct {
	JSON string `json:"json" binding:"required"`
}

// ValidateImportResponse returns validation results and mapping suggestions
type ValidateImportResponse struct {
	Valid               bool                    `json:"valid"`
	Errors              []string                `json:"errors,omitempty"`
	Warnings            []string                `json:"warnings,omitempty"`
	RequiredConnections []ConnectionRequiredDTO `json:"requiredConnections,omitempty"`
	MappingSuggestions  []MappingSuggestionDTO  `json:"mappingSuggestions,omitempty"`
}

// ConnectionRequiredDTO describes a required connection for import
type ConnectionRequiredDTO struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	UsedBy      []string `json:"usedBy"` // Widget titles
	Suggestions []string `json:"suggestions,omitempty"`
}

// MappingSuggestionDTO suggests connection mappings
type MappingSuggestionDTO struct {
	SourceName  string           `json:"sourceName"`
	SourceType  string           `json:"sourceType"`
	Suggestions []ConnectionInfo `json:"suggestions"`
}

// ConnectionInfo provides connection details
type ConnectionInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ImportDashboardRequest imports a dashboard with connection mappings
type ImportDashboardRequest struct {
	JSON     string            `json:"json" binding:"required"`
	Mappings map[string]string `json:"mappings"` // source connection name -> target connection ID
	Options  ImportOptionsDTO  `json:"options"`
}

// ImportOptionsDTO configures import behavior
type ImportOptionsDTO struct {
	SkipInvalidWidgets    bool `json:"skipInvalidWidgets"`
	AutoCreateConnections bool `json:"autoCreateConnections"`
}

// ImportDashboardResponse returns import results
type ImportDashboardResponse struct {
	DashboardID string           `json:"dashboardId"`
	Summary     ImportSummaryDTO `json:"summary"`
}

// ImportSummaryDTO provides import feedback
type ImportSummaryDTO struct {
	WidgetsImported int               `json:"widgetsImported"`
	WidgetsSkipped  int               `json:"widgetsSkipped"`
	Warnings        []string          `json:"warnings,omitempty"`
	ConnectionsUsed map[string]string `json:"connectionsUsed"` // source name -> target name
}
