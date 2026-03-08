package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/internal/repositories"
	"neobase-ai/pkg/dbmanager"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DashboardImportExportService handles dashboard import/export operations
type DashboardImportExportService interface {
	// ExportDashboard converts a dashboard to portable JSON format
	ExportDashboard(ctx context.Context, userID, chatID, dashboardID string) (*constants.ExportFormat, error)

	// ValidateImport validates import JSON and suggests connection mappings
	ValidateImport(ctx context.Context, userID, chatID string, exportData *constants.ExportFormat) (*constants.ValidationResult, error)

	// ImportDashboard creates a dashboard from export data with connection mappings
	ImportDashboard(ctx context.Context, userID, chatID string, exportData *constants.ExportFormat, mappings map[string]string, options constants.ImportOptions) (*models.Dashboard, *constants.ImportSummary, error)
}

type dashboardImportExportService struct {
	dashboardRepo repositories.DashboardRepository
	chatRepo      repositories.ChatRepository
	dbManager     *dbmanager.Manager
}

// NewDashboardImportExportService creates a new import/export service
func NewDashboardImportExportService(
	dashboardRepo repositories.DashboardRepository,
	chatRepo repositories.ChatRepository,
	dbManager *dbmanager.Manager,
) DashboardImportExportService {
	return &dashboardImportExportService{
		dashboardRepo: dashboardRepo,
		chatRepo:      chatRepo,
		dbManager:     dbManager,
	}
}

// ExportDashboard converts a dashboard to portable JSON format
func (s *dashboardImportExportService) ExportDashboard(ctx context.Context, userID, chatID, dashboardID string) (*constants.ExportFormat, error) {
	// 1. Get dashboard
	dashObjID, err := primitive.ObjectIDFromHex(dashboardID)
	if err != nil {
		return nil, fmt.Errorf("invalid dashboard ID: %s", dashboardID)
	}

	dashboard, err := s.dashboardRepo.FindDashboardByID(ctx, dashObjID)
	if err != nil {
		return nil, fmt.Errorf("dashboard not found: %v", err)
	}

	// Verify ownership
	if dashboard.UserID.Hex() != userID || dashboard.ChatID.Hex() != chatID {
		return nil, fmt.Errorf("unauthorized access to dashboard")
	}

	// 2. Get widgets
	widgets, err := s.dashboardRepo.FindWidgetsByDashboardID(ctx, dashObjID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch widgets: %v", err)
	}

	// 3. Get connection info from chat
	chatObjID, _ := primitive.ObjectIDFromHex(chatID)
	connectionInfo, err := s.getConnectionInfo(ctx, chatObjID)
	if err != nil {
		log.Printf("[EXPORT] Warning: failed to get connection info: %v", err)
		connectionInfo = make(map[string]ConnectionMeta)
	}

	// 4. Transform to export format
	exportedWidgets := make([]constants.ExportedWidget, 0, len(widgets))
	for _, widget := range widgets {
		exportedWidget := constants.ExportedWidget{
			Title:       widget.Title,
			Description: widget.Description,
			WidgetType:  widget.WidgetType,
			Tables:      widget.Tables,
			Query:       widget.Query,
			Config:      make(map[string]interface{}),
		}

		// Extract data source from widget (stored in chat's connection)
		if meta, ok := connectionInfo[chatID]; ok {
			exportedWidget.DataSource = constants.DataSourceRef{
				Type:           meta.Type,
				ConnectionName: meta.Name,
				Database:       meta.Database,
			}
		}

		// Include widget configs
		if widget.StatConfig != nil {
			exportedWidget.Config["statConfig"] = widget.StatConfig
		}
		if widget.TableConfig != nil {
			exportedWidget.Config["tableConfig"] = widget.TableConfig
		}
		if widget.ChartConfigJSON != "" {
			var chartConfig map[string]interface{}
			if err := json.Unmarshal([]byte(widget.ChartConfigJSON), &chartConfig); err == nil {
				exportedWidget.Config["chartConfig"] = chartConfig
			}
		}

		exportedWidgets = append(exportedWidgets, exportedWidget)
	}

	// 5. Transform layout
	exportedLayout := make([]constants.ExportedLayout, 0, len(dashboard.Layout))
	for i, layout := range dashboard.Layout {
		exportedLayout = append(exportedLayout, constants.ExportedLayout{
			WidgetIndex: i,
			X:           layout.X,
			Y:           layout.Y,
			W:           layout.W,
			H:           layout.H,
		})
	}

	// 6. Build export format
	exportFormat := &constants.ExportFormat{
		SchemaVersion: constants.DashboardSchemaVersion,
		ExportedAt:    time.Now(),
		ExportedBy:    userID, // Could be replaced with user email
		Dashboard: constants.ExportedDashboard{
			Name:            dashboard.Name,
			Description:     dashboard.Description,
			TemplateType:    dashboard.TemplateType,
			RefreshInterval: dashboard.RefreshInterval,
			TimeRange:       dashboard.TimeRange,
			Widgets:         exportedWidgets,
			Layout:          exportedLayout,
		},
		Metadata: map[string]interface{}{
			"exportVersion": "1.0.0",
			"widgetCount":   len(widgets),
		},
	}

	return exportFormat, nil
}

// ValidateImport validates import data and suggests connection mappings
func (s *dashboardImportExportService) ValidateImport(ctx context.Context, userID, chatID string, exportData *constants.ExportFormat) (*constants.ValidationResult, error) {
	result := &constants.ValidationResult{
		Valid:               true,
		Errors:              []string{},
		Warnings:            []string{},
		RequiredConnections: []constants.ConnectionRequired{},
	}

	// 1. Validate schema version
	if !s.isSupportedVersion(exportData.SchemaVersion) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Unsupported schema version: %s. Supported versions: %v", exportData.SchemaVersion, constants.SupportedSchemaVersions))
		return result, nil
	}

	// 2. Validate dashboard structure
	if exportData.Dashboard.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "Dashboard name is required")
	}

	if len(exportData.Dashboard.Widgets) == 0 {
		result.Warnings = append(result.Warnings, "Dashboard has no widgets")
	}

	// 3. Extract required connections
	connectionMap := make(map[string]constants.ConnectionRequired)
	for _, widget := range exportData.Dashboard.Widgets {
		connName := widget.DataSource.ConnectionName
		if connName == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Widget '%s' has no data source", widget.Title))
			continue
		}

		if conn, exists := connectionMap[connName]; exists {
			conn.UsedBy = append(conn.UsedBy, widget.Title)
			connectionMap[connName] = conn
		} else {
			connectionMap[connName] = constants.ConnectionRequired{
				Name:   connName,
				Type:   widget.DataSource.Type,
				UsedBy: []string{widget.Title},
			}
		}
	}

	// 4. Check if connections exist in target environment
	chatObjID, _ := primitive.ObjectIDFromHex(chatID)
	existingConnections, _ := s.getConnectionInfo(ctx, chatObjID)

	for connName, connReq := range connectionMap {
		// Check if connection exists
		found := false
		suggestions := []string{}

		for _, existing := range existingConnections {
			if existing.Name == connName && existing.Type == connReq.Type {
				found = true
				break
			}
			// Add as suggestion if type matches
			if existing.Type == connReq.Type {
				suggestions = append(suggestions, existing.Name)
			}
		}

		if !found {
			connReq.Suggestions = suggestions
			result.RequiredConnections = append(result.RequiredConnections, connReq)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Connection '%s' (%s) not found in target environment", connName, connReq.Type))
		}
	}

	return result, nil
}

// ImportDashboard creates a dashboard from export data.
// Smart mapping: auto-maps widgets to the current chat's connection by matching DB type, tables, etc.
// All widgets are always imported — if the connection type doesn't match, the widget is imported anyway
// (queries may need regeneration by the user via AI).
func (s *dashboardImportExportService) ImportDashboard(ctx context.Context, userID, chatID string, exportData *constants.ExportFormat, mappings map[string]string, options constants.ImportOptions) (*models.Dashboard, *constants.ImportSummary, error) {
	summary := &constants.ImportSummary{
		WidgetsImported: 0,
		WidgetsSkipped:  0,
		Warnings:        []string{},
		ConnectionsUsed: make(map[string]string),
	}

	// 1. Validate basic structure (schema version, name)
	validation, err := s.ValidateImport(ctx, userID, chatID, exportData)
	if err != nil {
		return nil, nil, fmt.Errorf("validation failed: %v", err)
	}

	if !validation.Valid {
		return nil, nil, fmt.Errorf("import data is invalid: %v", validation.Errors)
	}

	// 2. Get current chat's connection info for smart mapping
	chatObjID, _ := primitive.ObjectIDFromHex(chatID)
	connectionInfo, err := s.getConnectionInfo(ctx, chatObjID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get connection info: %v", err)
	}

	// Get the chat's connection metadata (the chat has exactly one connection)
	var chatConn *ConnectionMeta
	for _, meta := range connectionInfo {
		m := meta // capture loop variable
		chatConn = &m
		break
	}

	// Determine if the source DB type matches the target chat's DB type
	dbTypeMatched := false
	if chatConn != nil {
		for _, widget := range exportData.Dashboard.Widgets {
			if widget.DataSource.Type != "" && widget.DataSource.Type == chatConn.Type {
				dbTypeMatched = true
				break
			}
		}
	}

	if dbTypeMatched {
		log.Printf("[IMPORT] DB type matched (%s) — widgets will be mapped to chat connection", chatConn.Type)
	} else if chatConn != nil {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("Source DB type differs from target (%s). Widgets imported but queries may need regeneration.", chatConn.Type))
		log.Printf("[IMPORT] DB type mismatch — importing all widgets anyway for user to regenerate")
	}

	// 3. Create dashboard
	userObjID, _ := primitive.ObjectIDFromHex(userID)
	dashboard := models.NewDashboard(userObjID, chatObjID, exportData.Dashboard.Name)
	dashboard.Description = exportData.Dashboard.Description
	dashboard.TemplateType = exportData.Dashboard.TemplateType
	dashboard.RefreshInterval = exportData.Dashboard.RefreshInterval
	dashboard.TimeRange = exportData.Dashboard.TimeRange

	if err := s.dashboardRepo.CreateDashboard(ctx, dashboard); err != nil {
		return nil, nil, fmt.Errorf("failed to create dashboard: %v", err)
	}

	// 4. Create widgets — always import all widgets regardless of connection mapping
	widgets := []*models.Widget{}
	layout := []models.WidgetLayout{}

	for i, exportedWidget := range exportData.Dashboard.Widgets {
		// Create widget with the target chat's connection
		widget := models.NewWidget(dashboard.ID, chatObjID, userObjID, exportedWidget.Title, exportedWidget.WidgetType, exportedWidget.Query)
		widget.Description = exportedWidget.Description
		widget.Tables = exportedWidget.Tables

		// Apply config
		if statConfig, ok := exportedWidget.Config["statConfig"]; ok {
			if statBytes, err := json.Marshal(statConfig); err == nil {
				var stat models.StatWidgetConfig
				if err := json.Unmarshal(statBytes, &stat); err == nil {
					widget.StatConfig = &stat
				}
			}
		}

		if tableConfig, ok := exportedWidget.Config["tableConfig"]; ok {
			if tableBytes, err := json.Marshal(tableConfig); err == nil {
				var table models.TableWidgetConfig
				if err := json.Unmarshal(tableBytes, &table); err == nil {
					widget.TableConfig = &table
				}
			}
		}

		if chartConfig, ok := exportedWidget.Config["chartConfig"]; ok {
			if chartBytes, err := json.Marshal(chartConfig); err == nil {
				widget.ChartConfigJSON = string(chartBytes)
			}
		}

		widgets = append(widgets, widget)
		summary.WidgetsImported++

		if chatConn != nil {
			summary.ConnectionsUsed[exportedWidget.DataSource.ConnectionName] = chatConn.Name
		}

		// Add to layout
		if i < len(exportData.Dashboard.Layout) {
			exportedLayout := exportData.Dashboard.Layout[i]
			layout = append(layout, models.WidgetLayout{
				WidgetID: widget.ID.Hex(),
				X:        exportedLayout.X,
				Y:        exportedLayout.Y,
				W:        exportedLayout.W,
				H:        exportedLayout.H,
			})
		}
	}

	// 5. Save widgets to database
	if len(widgets) > 0 {
		if err := s.dashboardRepo.CreateWidgets(ctx, widgets); err != nil {
			log.Printf("[IMPORT] Warning: failed to batch create widgets, falling back to individual: %v", err)
			for _, w := range widgets {
				if err := s.dashboardRepo.CreateWidget(ctx, w); err != nil {
					log.Printf("[IMPORT] Failed to create widget '%s': %v", w.Title, err)
					summary.WidgetsSkipped++
					summary.WidgetsImported--
				}
			}
		}
	}

	// 6. Update dashboard with layout
	dashboard.Layout = layout
	if err := s.dashboardRepo.UpdateDashboard(ctx, dashboard.ID, dashboard); err != nil {
		log.Printf("[IMPORT] Warning: failed to update dashboard layout: %v", err)
	}

	summary.DashboardID = dashboard.ID.Hex()

	log.Printf("[IMPORT] Dashboard imported — ID: %s, Widgets: %d imported, %d skipped",
		dashboard.ID.Hex(), summary.WidgetsImported, summary.WidgetsSkipped)

	return dashboard, summary, nil
}

// Helper: Check if schema version is supported
func (s *dashboardImportExportService) isSupportedVersion(version string) bool {
	for _, supported := range constants.SupportedSchemaVersions {
		if version == supported {
			return true
		}
	}
	return false
}

// Helper: Get connection info from chat
type ConnectionMeta struct {
	ID       string
	Name     string
	Type     string
	Database string
}

func (s *dashboardImportExportService) getConnectionInfo(ctx context.Context, chatID primitive.ObjectID) (map[string]ConnectionMeta, error) {
	// Fetch the chat to get its connection information
	chat, err := s.chatRepo.FindByID(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to find chat: %v", err)
	}

	if chat == nil {
		return nil, fmt.Errorf("chat not found")
	}

	// Each chat has one connection - map it by chat ID for simplicity
	// This allows widgets to reference the chat's connection
	connectionMap := make(map[string]ConnectionMeta)

	// Create a connection name from the database + type
	connectionName := fmt.Sprintf("%s-%s", chat.Connection.Database, chat.Connection.Type)
	if chat.Connection.Host != "" {
		connectionName = fmt.Sprintf("%s-%s", chat.Connection.Host, chat.Connection.Database)
	}

	connectionMap[chatID.Hex()] = ConnectionMeta{
		ID:       chatID.Hex(),
		Name:     connectionName,
		Type:     chat.Connection.Type,
		Database: chat.Connection.Database,
	}

	log.Printf("[getConnectionInfo] Found connection for chat %s: %s (%s)", chatID.Hex(), connectionName, chat.Connection.Type)

	return connectionMap, nil
}
