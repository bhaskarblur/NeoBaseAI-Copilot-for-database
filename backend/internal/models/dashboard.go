package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Dashboard represents a saved dashboard configuration per chat/connection
type Dashboard struct {
	UserID          primitive.ObjectID `bson:"user_id" json:"user_id"`
	ChatID          primitive.ObjectID `bson:"chat_id" json:"chat_id"`
	Name            string             `bson:"name" json:"name"`
	Description     string             `bson:"description,omitempty" json:"description,omitempty"`
	TemplateType    string             `bson:"template_type,omitempty" json:"template_type,omitempty"`       // "ecommerce", "user_analytics", "db_health", "custom", etc.
	IsDefault       bool               `bson:"is_default" json:"is_default"`                                 // Default dashboard opened first
	RefreshInterval int                `bson:"refresh_interval" json:"refresh_interval"`                     // Seconds: 0 = manual, 15, 30, 60, 300, 600, 3600
	TimeRange       string             `bson:"time_range" json:"time_range"`                                 // "1h", "6h", "24h", "7d", "30d", "custom"
	Layout          []WidgetLayout     `bson:"layout" json:"layout"`                                         // Grid positions for react-grid-layout
	GeneratedPrompt string             `bson:"generated_prompt,omitempty" json:"generated_prompt,omitempty"` // The prompt used to generate this dashboard, if applicable
	LLMModel        string             `bson:"llm_model,omitempty" json:"llm_model,omitempty"`
	Base            `bson:",inline"`
}

// WidgetLayout stores the grid position/size for a widget (react-grid-layout compatible)
type WidgetLayout struct {
	WidgetID string `bson:"widget_id" json:"widget_id"` // References Widget.ID hex string
	X        int    `bson:"x" json:"x"`                 // Grid column position
	Y        int    `bson:"y" json:"y"`                 // Grid row position
	W        int    `bson:"w" json:"w"`                 // Width in grid units (1-12)
	H        int    `bson:"h" json:"h"`                 // Height in grid units
	MinW     int    `bson:"min_w,omitempty" json:"min_w,omitempty"`
	MinH     int    `bson:"min_h,omitempty" json:"min_h,omitempty"`
}

// Widget represents a single dashboard widget (stat card, chart, table)
type Widget struct {
	DashboardID primitive.ObjectID `bson:"dashboard_id" json:"dashboard_id"`
	ChatID      primitive.ObjectID `bson:"chat_id" json:"chat_id"`
	UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	WidgetType  string             `bson:"widget_type" json:"widget_type"` // "stat", "line", "bar", "area", "pie", "table", "combo"

	// Query Configuration
	Query     string `bson:"query" json:"query"`                               // The SQL/MongoDB query to execute
	QueryType string `bson:"query_type,omitempty" json:"query_type,omitempty"` // "SELECT", "AGGREGATE", etc.
	Tables    string `bson:"tables,omitempty" json:"tables,omitempty"`         // Comma-separated table names referenced

	// Visualization Configuration (reuses existing ChartConfiguration structure)
	ChartConfigJSON string `bson:"chart_config_json,omitempty" json:"chart_config_json,omitempty"` // Full chart config as JSON string

	// Stat Card specific
	StatConfig *StatWidgetConfig `bson:"stat_config,omitempty" json:"stat_config,omitempty"`

	// Table widget specific
	TableConfig *TableWidgetConfig `bson:"table_config,omitempty" json:"table_config,omitempty"`

	LastRefreshedAt *primitive.DateTime `bson:"last_refreshed_at,omitempty" json:"last_refreshed_at,omitempty"` // When data was last refreshed

	// AI Generation metadata
	GeneratedPrompt string `bson:"generated_prompt,omitempty" json:"generated_prompt,omitempty"` // The user prompt that generated this widget
	LLMModel        string `bson:"llm_model,omitempty" json:"llm_model,omitempty"`

	Base `bson:",inline"`
}

// StatWidgetConfig for single-value KPI widgets
type StatWidgetConfig struct {
	ValueQuery      string `bson:"value_query" json:"value_query"`                               // Query for the main value
	ComparisonQuery string `bson:"comparison_query,omitempty" json:"comparison_query,omitempty"` // Query for comparison (e.g., yesterday's value)
	Format          string `bson:"format,omitempty" json:"format,omitempty"`                     // "number", "currency", "percentage", "duration"
	Prefix          string `bson:"prefix,omitempty" json:"prefix,omitempty"`                     // "$", "€", etc.
	Suffix          string `bson:"suffix,omitempty" json:"suffix,omitempty"`                     // "%", "ms", etc.
	DecimalPlaces   int    `bson:"decimal_places,omitempty" json:"decimal_places,omitempty"`
	TrendDirection  string `bson:"trend_direction,omitempty" json:"trend_direction,omitempty"` // "up_is_good", "down_is_good"
}

// TableWidgetConfig for tabular data widgets
type TableWidgetConfig struct {
	Columns       []TableWidgetColumn `bson:"columns" json:"columns"`
	SortBy        string              `bson:"sort_by,omitempty" json:"sort_by,omitempty"`
	SortDirection string              `bson:"sort_direction,omitempty" json:"sort_direction,omitempty"` // "asc", "desc"
	PageSize      int                 `bson:"page_size,omitempty" json:"page_size,omitempty"`
}

// TableWidgetColumn defines a column in a table widget
type TableWidgetColumn struct {
	Key    string `bson:"key" json:"key"`
	Label  string `bson:"label" json:"label"`
	Format string `bson:"format,omitempty" json:"format,omitempty"` // "text", "number", "date", "currency"
	Width  string `bson:"width,omitempty" json:"width,omitempty"`
}

// DashboardBlueprint is a lightweight preview of a dashboard (no queries, just overview)
// Used during the recommendation flow before full generation
type DashboardBlueprint struct {
	Name            string                   `json:"name"`
	Description     string                   `json:"description"`
	TemplateType    string                   `json:"template_type"`
	ProposedWidgets []BlueprintWidgetPreview `json:"proposed_widgets"`
}

// BlueprintWidgetPreview is a lightweight widget preview in a blueprint
type BlueprintWidgetPreview struct {
	Title      string `json:"title"`
	WidgetType string `json:"widget_type"` // "stat", "line", "bar", "area", "pie", "table", "combo"
}

// NewDashboard creates a new Dashboard instance with defaults
func NewDashboard(userID, chatID primitive.ObjectID, name string) *Dashboard {
	return &Dashboard{
		UserID:          userID,
		ChatID:          chatID,
		Name:            name,
		IsDefault:       false,
		RefreshInterval: 60, // 1 minute default
		TimeRange:       "24h",
		Layout:          []WidgetLayout{},
		Base:            NewBase(),
	}
}

// NewWidget creates a new Widget instance
func NewWidget(dashboardID, chatID, userID primitive.ObjectID, title, widgetType, query string) *Widget {
	return &Widget{
		DashboardID: dashboardID,
		ChatID:      chatID,
		UserID:      userID,
		Title:       title,
		WidgetType:  widgetType,
		Query:       query,
		Base:        NewBase(),
	}
}
