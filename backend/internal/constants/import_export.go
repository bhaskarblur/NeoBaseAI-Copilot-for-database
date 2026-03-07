package constants

import "time"

// Import/Export Constants
const (
	DashboardSchemaVersion = "1.0.0"
	MaxImportSizeMB        = 10
	MaxImportSizeBytes     = MaxImportSizeMB * 1024 * 1024
)

var SupportedSchemaVersions = []string{"1.0.0"}

// ExportFormat represents the complete exported dashboard structure
type ExportFormat struct {
	SchemaVersion string                 `json:"schemaVersion"`
	ExportedAt    time.Time              `json:"exportedAt"`
	ExportedBy    string                 `json:"exportedBy"`
	Dashboard     ExportedDashboard      `json:"dashboard"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ExportedDashboard represents a portable dashboard
type ExportedDashboard struct {
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	TemplateType    string             `json:"templateType,omitempty"`
	RefreshInterval int                `json:"refreshInterval"`
	TimeRange       string             `json:"timeRange,omitempty"`
	Widgets         []ExportedWidget   `json:"widgets"`
	Layout          []ExportedLayout   `json:"layout"`
	Variables       []ExportedVariable `json:"variables,omitempty"`
}

// ExportedWidget represents a portable widget
type ExportedWidget struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	WidgetType  string                 `json:"widgetType"`
	Tables      string                 `json:"tables,omitempty"`
	DataSource  DataSourceRef          `json:"dataSource"`
	Query       string                 `json:"query"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// DataSourceRef represents a connection reference by name
type DataSourceRef struct {
	Type           string `json:"type"`           // mongodb, postgresql, mysql, etc.
	ConnectionName string `json:"connectionName"` // Reference by name for portability
	Database       string `json:"database,omitempty"`
	Schema         string `json:"schema,omitempty"`
	Collection     string `json:"collection,omitempty"`
	Table          string `json:"table,omitempty"`
}

// ExportedLayout represents widget positioning
type ExportedLayout struct {
	WidgetIndex int `json:"widgetIndex"` // Index in widgets array
	X           int `json:"x"`
	Y           int `json:"y"`
	W           int `json:"w"`
	H           int `json:"h"`
}

// ExportedVariable represents dashboard variables (future use)
type ExportedVariable struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Default interface{} `json:"default,omitempty"`
}

// ValidationResult holds import validation results
type ValidationResult struct {
	Valid               bool                 `json:"valid"`
	Errors              []string             `json:"errors,omitempty"`
	Warnings            []string             `json:"warnings,omitempty"`
	RequiredConnections []ConnectionRequired `json:"requiredConnections,omitempty"`
	IncompatibleWidgets []int                `json:"incompatibleWidgets,omitempty"`
}

// ConnectionRequired represents a connection that needs mapping
type ConnectionRequired struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	UsedBy      []string `json:"usedBy"` // Widget titles that use this connection
	Suggestions []string `json:"suggestions,omitempty"`
}

// MappingSuggestion suggests connection mappings
type MappingSuggestion struct {
	SourceName     string           `json:"sourceName"`
	SourceType     string           `json:"sourceType"`
	Suggestions    []string         `json:"suggestions"` // Connection IDs
	SuggestionInfo []ConnectionInfo `json:"suggestionInfo,omitempty"`
}

// ConnectionInfo provides details about a suggested connection
type ConnectionInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ImportSummary provides feedback after import
type ImportSummary struct {
	DashboardID     string            `json:"dashboardId"`
	WidgetsImported int               `json:"widgetsImported"`
	WidgetsSkipped  int               `json:"widgetsSkipped"`
	Warnings        []string          `json:"warnings,omitempty"`
	ConnectionsUsed map[string]string `json:"connectionsUsed"` // source name -> target name
}

// ImportOptions configures import behavior
type ImportOptions struct {
	SkipInvalidWidgets    bool `json:"skipInvalidWidgets"`
	AutoCreateConnections bool `json:"autoCreateConnections"`
}
