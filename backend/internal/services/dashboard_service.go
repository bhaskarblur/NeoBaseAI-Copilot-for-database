package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/internal/repositories"
	"neobase-ai/pkg/dbmanager"
	"neobase-ai/pkg/llm"
	"neobase-ai/pkg/redis"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DashboardService handles dashboard CRUD and AI generation operations
type DashboardService interface {
	// CRUD operations
	CreateDashboard(ctx context.Context, userID, chatID string, req *dtos.CreateDashboardRequest) (*dtos.DashboardResponse, uint32, error)
	GetDashboard(ctx context.Context, userID, chatID, dashboardID string) (*dtos.DashboardResponse, uint32, error)
	ListDashboards(ctx context.Context, userID, chatID string) ([]dtos.DashboardListItem, uint32, error)
	UpdateDashboard(ctx context.Context, userID, chatID, dashboardID string, req *dtos.UpdateDashboardRequest) (*dtos.DashboardResponse, uint32, error)
	DeleteDashboard(ctx context.Context, userID, chatID, dashboardID string) (uint32, error)

	// Widget operations
	AddWidget(ctx context.Context, userID, chatID, dashboardID string, req *dtos.AddWidgetRequest) (*dtos.WidgetResponse, uint32, error)
	EditWidget(ctx context.Context, userID, chatID, dashboardID, widgetID string, req *dtos.EditWidgetRequest) (*dtos.WidgetResponse, uint32, error)
	DeleteWidget(ctx context.Context, userID, chatID, dashboardID, widgetID string) (uint32, error)

	// AI operations
	GenerateBlueprints(ctx context.Context, userID, chatID, streamID string, userPrompt string) (uint32, error)
	CreateFromBlueprints(ctx context.Context, userID, chatID, streamID string, req *dtos.CreateFromBlueprintsRequest) (uint32, error)
	RegenerateDashboard(ctx context.Context, userID, chatID, dashboardID, streamID string, req *dtos.RegenerateDashboardRequest) (uint32, error)

	// Data refresh
	RefreshDashboard(ctx context.Context, userID, chatID, dashboardID, streamID string) (uint32, error)
	RefreshWidget(ctx context.Context, userID, chatID, dashboardID, widgetID, streamID string) (uint32, error)

	// Stream handler for SSE
	SetStreamHandler(handler StreamHandler)
}

type dashboardService struct {
	dashboardRepo repositories.DashboardRepository
	chatRepo      repositories.ChatRepository
	kbRepo        repositories.KnowledgeBaseRepository
	dbManager     *dbmanager.Manager
	llmManager    *llm.Manager
	redisRepo     redis.IRedisRepositories
	streamHandler StreamHandler
}

// NewDashboardService creates a new dashboard service instance
func NewDashboardService(
	dashboardRepo repositories.DashboardRepository,
	chatRepo repositories.ChatRepository,
	dbManager *dbmanager.Manager,
	llmManager *llm.Manager,
	redisRepo redis.IRedisRepositories,
	kbRepo repositories.KnowledgeBaseRepository,
) DashboardService {
	return &dashboardService{
		dashboardRepo: dashboardRepo,
		chatRepo:      chatRepo,
		kbRepo:        kbRepo,
		dbManager:     dbManager,
		llmManager:    llmManager,
		redisRepo:     redisRepo,
	}
}

func (s *dashboardService) SetStreamHandler(handler StreamHandler) {
	s.streamHandler = handler
}

// === CRUD Operations ===

func (s *dashboardService) CreateDashboard(ctx context.Context, userID, chatID string, req *dtos.CreateDashboardRequest) (*dtos.DashboardResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid user ID: %s", userID)
	}
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid chat ID: %s", chatID)
	}

	// Check dashboard limit
	count, err := s.dashboardRepo.CountDashboardsByChatID(ctx, chatObjID)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to count dashboards: %v", err)
	}
	if count >= int64(constants.MaxDashboardsPerChatFree) {
		return nil, 403, fmt.Errorf("maximum number of dashboards (%d) reached for this chat", constants.MaxDashboardsPerChatFree)
	}

	// Create dashboard with placeholder name (AI will generate the actual content)
	dashboard := models.NewDashboard(userObjID, chatObjID, "New Dashboard")
	dashboard.GeneratedPrompt = req.Prompt
	dashboard.IsDefault = count == 0 // First dashboard is default

	if err := s.dashboardRepo.CreateDashboard(ctx, dashboard); err != nil {
		return nil, 500, fmt.Errorf("failed to create dashboard: %v", err)
	}

	resp := s.dashboardToResponse(dashboard, nil)
	return resp, 201, nil
}

func (s *dashboardService) GetDashboard(ctx context.Context, userID, chatID, dashboardID string) (*dtos.DashboardResponse, uint32, error) {
	dashObjID, err := primitive.ObjectIDFromHex(dashboardID)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid dashboard ID: %s", dashboardID)
	}

	dashboard, err := s.dashboardRepo.FindDashboardByID(ctx, dashObjID)
	if err != nil {
		return nil, 404, fmt.Errorf("dashboard not found: %v", err)
	}

	// Verify ownership
	if dashboard.UserID.Hex() != userID || dashboard.ChatID.Hex() != chatID {
		return nil, 403, fmt.Errorf("unauthorized access to dashboard")
	}

	widgets, err := s.dashboardRepo.FindWidgetsByDashboardID(ctx, dashObjID)
	if err != nil {
		log.Printf("[DASHBOARD] Failed to fetch widgets for dashboard %s: %v", dashboardID, err)
		widgets = []*models.Widget{}
	}

	resp := s.dashboardToResponse(dashboard, widgets)
	return resp, 200, nil
}

func (s *dashboardService) ListDashboards(ctx context.Context, userID, chatID string) ([]dtos.DashboardListItem, uint32, error) {
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid chat ID: %s", chatID)
	}

	dashboards, err := s.dashboardRepo.FindDashboardsByChatID(ctx, chatObjID)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to list dashboards: %v", err)
	}

	items := make([]dtos.DashboardListItem, 0, len(dashboards))
	for _, d := range dashboards {
		if d.UserID.Hex() != userID {
			continue
		}
		widgetCount, _ := s.dashboardRepo.CountWidgetsByDashboardID(ctx, d.ID)
		items = append(items, dtos.DashboardListItem{
			ID:           d.ID.Hex(),
			Name:         d.Name,
			Description:  d.Description,
			TemplateType: d.TemplateType,
			IsDefault:    d.IsDefault,
			WidgetCount:  int(widgetCount),
			CreatedAt:    d.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    d.UpdatedAt.Format(time.RFC3339),
		})
	}

	return items, 200, nil
}

func (s *dashboardService) UpdateDashboard(ctx context.Context, userID, chatID, dashboardID string, req *dtos.UpdateDashboardRequest) (*dtos.DashboardResponse, uint32, error) {
	dashObjID, err := primitive.ObjectIDFromHex(dashboardID)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid dashboard ID: %s", dashboardID)
	}

	dashboard, err := s.dashboardRepo.FindDashboardByID(ctx, dashObjID)
	if err != nil {
		return nil, 404, fmt.Errorf("dashboard not found: %v", err)
	}

	if dashboard.UserID.Hex() != userID || dashboard.ChatID.Hex() != chatID {
		return nil, 403, fmt.Errorf("unauthorized access to dashboard")
	}

	// Apply partial updates
	if req.Name != nil {
		dashboard.Name = *req.Name
	}
	if req.Description != nil {
		dashboard.Description = *req.Description
	}
	if req.RefreshInterval != nil {
		dashboard.RefreshInterval = *req.RefreshInterval
	}
	if req.TimeRange != nil {
		dashboard.TimeRange = *req.TimeRange
	}
	if req.Layout != nil {
		dashboard.Layout = make([]models.WidgetLayout, len(*req.Layout))
		for i, l := range *req.Layout {
			dashboard.Layout[i] = models.WidgetLayout{
				WidgetID: l.WidgetID,
				X:        l.X,
				Y:        l.Y,
				W:        l.W,
				H:        l.H,
				MinW:     l.MinW,
				MinH:     l.MinH,
			}
		}
	}
	if req.IsDefault != nil && *req.IsDefault {
		chatObjID, _ := primitive.ObjectIDFromHex(chatID)
		s.dashboardRepo.SetDefaultDashboard(ctx, chatObjID, dashObjID)
	}

	if err := s.dashboardRepo.UpdateDashboard(ctx, dashObjID, dashboard); err != nil {
		return nil, 500, fmt.Errorf("failed to update dashboard: %v", err)
	}

	widgets, _ := s.dashboardRepo.FindWidgetsByDashboardID(ctx, dashObjID)
	resp := s.dashboardToResponse(dashboard, widgets)
	return resp, 200, nil
}

func (s *dashboardService) DeleteDashboard(ctx context.Context, userID, chatID, dashboardID string) (uint32, error) {
	dashObjID, err := primitive.ObjectIDFromHex(dashboardID)
	if err != nil {
		return 400, fmt.Errorf("invalid dashboard ID: %s", dashboardID)
	}

	dashboard, err := s.dashboardRepo.FindDashboardByID(ctx, dashObjID)
	if err != nil {
		return 404, fmt.Errorf("dashboard not found: %v", err)
	}

	if dashboard.UserID.Hex() != userID || dashboard.ChatID.Hex() != chatID {
		return 403, fmt.Errorf("unauthorized access to dashboard")
	}

	// Delete all widgets first
	if err := s.dashboardRepo.DeleteWidgetsByDashboardID(ctx, dashObjID); err != nil {
		log.Printf("[DASHBOARD] Warning: failed to delete widgets for dashboard %s: %v", dashboardID, err)
	}

	if err := s.dashboardRepo.DeleteDashboard(ctx, dashObjID); err != nil {
		return 500, fmt.Errorf("failed to delete dashboard: %v", err)
	}

	return 200, nil
}

// === Widget Operations ===

func (s *dashboardService) AddWidget(ctx context.Context, userID, chatID, dashboardID string, req *dtos.AddWidgetRequest) (*dtos.WidgetResponse, uint32, error) {
	dashObjID, err := primitive.ObjectIDFromHex(dashboardID)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid dashboard ID: %s", dashboardID)
	}

	dashboard, err := s.dashboardRepo.FindDashboardByID(ctx, dashObjID)
	if err != nil {
		return nil, 404, fmt.Errorf("dashboard not found: %v", err)
	}

	if dashboard.UserID.Hex() != userID || dashboard.ChatID.Hex() != chatID {
		return nil, 403, fmt.Errorf("unauthorized access to dashboard")
	}

	// Check widget limit
	widgetCount, _ := s.dashboardRepo.CountWidgetsByDashboardID(ctx, dashObjID)
	if widgetCount >= int64(constants.MaxWidgetsPerDashboard) {
		return nil, 403, fmt.Errorf("maximum number of widgets (%d) reached", constants.MaxWidgetsPerDashboard)
	}

	// Get chat context for AI generation
	chat, _, dbType, err := s.getChatContext(chatID)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to get chat context: %v", err)
	}

	// Build context with existing widgets for the add-widget prompt
	existingWidgets, _ := s.dashboardRepo.FindWidgetsByDashboardID(ctx, dashObjID)
	existingTitles := make([]string, 0, len(existingWidgets))
	for _, w := range existingWidgets {
		existingTitles = append(existingTitles, w.Title)
	}

	// Replace placeholders in add-widget system prompt
	addWidgetPrompt := constants.DashboardAddWidgetSystemPrompt
	addWidgetPrompt = replacePromptPlaceholder(addWidgetPrompt, "{{DASHBOARD_NAME}}", dashboard.Name)
	addWidgetPrompt = replacePromptPlaceholder(addWidgetPrompt, "{{EXISTING_WIDGET_TITLES}}", joinStrings(existingTitles, ", "))

	// Build LLM messages
	messages := s.buildDashboardLLMMessages(chat, addWidgetPrompt, req.Prompt, dbType, dashboard.TimeRange)

	llmClient, selectedModel, err := s.getLLMClient(chat)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to get LLM client: %v", err)
	}

	toolExecutor := BuildDashboardToolExecutor(s.dbManager, chatID, dbType, s.kbRepo)
	tools := GetDashboardTools(constants.DashboardModeAddWidget)

	toolCallConfig := llm.ToolCallConfig{
		MaxIterations: constants.DashboardMaxToolIterations,
		DBType:        dbType,
		ModelID:       selectedModel,
		SystemPrompt:  addWidgetPrompt,
		OnToolCall:    func(call llm.ToolCall) {},
		OnToolResult:  func(call llm.ToolCall, result llm.ToolResult) {},
		OnIteration: func(iteration int, toolCallCount int) {
			log.Printf("[DASHBOARD] AddWidget iteration %d, total calls: %d", iteration, toolCallCount)
		},
	}

	toolResult, err := llmClient.GenerateWithTools(ctx, messages, tools, toolExecutor, toolCallConfig)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to generate widget: %v", err)
	}

	// Parse the response to get the single widget config
	dashboardConfig, err := s.parseDashboardResponse(toolResult.Response)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to parse widget response: %v", err)
	}

	widgetsRaw, ok := dashboardConfig["widgets"].([]interface{})
	if !ok || len(widgetsRaw) == 0 {
		return nil, 500, fmt.Errorf("AI response contained no widgets")
	}

	// Take only the first widget
	widgetConfig, ok := widgetsRaw[0].(map[string]interface{})
	if !ok {
		return nil, 500, fmt.Errorf("invalid widget config in AI response")
	}

	userObjID, _ := primitive.ObjectIDFromHex(userID)
	chatObjID, _ := primitive.ObjectIDFromHex(chatID)

	widget := s.createWidgetFromConfig(ctx, dashObjID, chatObjID, userObjID, widgetConfig, selectedModel)
	if widget == nil {
		return nil, 500, fmt.Errorf("failed to create widget from AI config")
	}

	widget.GeneratedPrompt = req.Prompt

	// Add widget to dashboard layout
	layoutConfig := getLayoutFromWidgetConfig(widgetConfig)
	dashboard.Layout = append(dashboard.Layout, models.WidgetLayout{
		WidgetID: widget.ID.Hex(),
		X:        0,
		Y:        int(widgetCount) * 4,
		W:        layoutConfig.W,
		H:        layoutConfig.H,
	})
	s.dashboardRepo.UpdateDashboard(ctx, dashObjID, dashboard)

	resp := s.widgetToResponse(widget)
	return resp, 201, nil
}

func (s *dashboardService) EditWidget(ctx context.Context, userID, chatID, dashboardID, widgetID string, req *dtos.EditWidgetRequest) (*dtos.WidgetResponse, uint32, error) {
	widgetObjID, err := primitive.ObjectIDFromHex(widgetID)
	if err != nil {
		return nil, 400, fmt.Errorf("invalid widget ID: %s", widgetID)
	}

	widget, err := s.dashboardRepo.FindWidgetByID(ctx, widgetObjID)
	if err != nil {
		return nil, 404, fmt.Errorf("widget not found: %v", err)
	}

	if widget.UserID.Hex() != userID || widget.ChatID.Hex() != chatID || widget.DashboardID.Hex() != dashboardID {
		return nil, 403, fmt.Errorf("unauthorized access to widget")
	}

	// Get chat context for AI editing
	chat, _, dbType, err := s.getChatContext(chatID)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to get chat context: %v", err)
	}

	// Build edit system prompt with current widget context
	editPrompt := constants.DashboardWidgetEditSystemPrompt
	editPrompt = replacePromptPlaceholder(editPrompt, "{{WIDGET_TITLE}}", widget.Title)
	editPrompt = replacePromptPlaceholder(editPrompt, "{{WIDGET_TYPE}}", widget.WidgetType)
	editPrompt = replacePromptPlaceholder(editPrompt, "{{WIDGET_QUERY}}", widget.Query)

	// Build LLM messages (use dashboard's time range for edit context)
	dashObjIDForEdit, _ := primitive.ObjectIDFromHex(dashboardID)
	dashboardForEdit, _ := s.dashboardRepo.FindDashboardByID(ctx, dashObjIDForEdit)
	editTimeRange := "24h"
	if dashboardForEdit != nil && dashboardForEdit.TimeRange != "" {
		editTimeRange = dashboardForEdit.TimeRange
	}
	messages := s.buildDashboardLLMMessages(chat, editPrompt, req.Prompt, dbType, editTimeRange)

	llmClient, selectedModel, err := s.getLLMClient(chat)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to get LLM client: %v", err)
	}

	toolExecutor := BuildDashboardToolExecutor(s.dbManager, chatID, dbType, s.kbRepo)
	tools := GetDashboardTools(constants.DashboardModeEditWidget)

	toolCallConfig := llm.ToolCallConfig{
		MaxIterations: constants.DashboardMaxToolIterations,
		DBType:        dbType,
		ModelID:       selectedModel,
		SystemPrompt:  editPrompt,
		OnToolCall:    func(call llm.ToolCall) {},
		OnToolResult:  func(call llm.ToolCall, result llm.ToolResult) {},
		OnIteration: func(iteration int, toolCallCount int) {
			log.Printf("[DASHBOARD] EditWidget iteration %d, total calls: %d", iteration, toolCallCount)
		},
	}

	toolResult, err := llmClient.GenerateWithTools(ctx, messages, tools, toolExecutor, toolCallConfig)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to edit widget: %v", err)
	}

	// Parse the response
	dashboardConfig, err := s.parseDashboardResponse(toolResult.Response)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to parse widget edit response: %v", err)
	}

	widgetsRaw, ok := dashboardConfig["widgets"].([]interface{})
	if !ok || len(widgetsRaw) == 0 {
		return nil, 500, fmt.Errorf("AI response contained no widgets")
	}

	widgetConfig, ok := widgetsRaw[0].(map[string]interface{})
	if !ok {
		return nil, 500, fmt.Errorf("invalid widget config in AI response")
	}

	// Apply updates from AI response to existing widget
	s.applyWidgetConfigUpdates(widget, widgetConfig)
	widget.GeneratedPrompt = req.Prompt
	widget.LLMModel = selectedModel

	if err := s.dashboardRepo.UpdateWidget(ctx, widgetObjID, widget); err != nil {
		return nil, 500, fmt.Errorf("failed to update widget: %v", err)
	}

	resp := s.widgetToResponse(widget)
	return resp, 200, nil
}

func (s *dashboardService) DeleteWidget(ctx context.Context, userID, chatID, dashboardID, widgetID string) (uint32, error) {
	widgetObjID, err := primitive.ObjectIDFromHex(widgetID)
	if err != nil {
		return 400, fmt.Errorf("invalid widget ID: %s", widgetID)
	}

	widget, err := s.dashboardRepo.FindWidgetByID(ctx, widgetObjID)
	if err != nil {
		return 404, fmt.Errorf("widget not found: %v", err)
	}

	if widget.UserID.Hex() != userID || widget.ChatID.Hex() != chatID || widget.DashboardID.Hex() != dashboardID {
		return 403, fmt.Errorf("unauthorized access to widget")
	}

	if err := s.dashboardRepo.DeleteWidget(ctx, widgetObjID); err != nil {
		return 500, fmt.Errorf("failed to delete widget: %v", err)
	}

	// Remove from dashboard layout
	dashObjID, _ := primitive.ObjectIDFromHex(dashboardID)
	dashboard, err := s.dashboardRepo.FindDashboardByID(ctx, dashObjID)
	if err == nil {
		newLayout := make([]models.WidgetLayout, 0, len(dashboard.Layout))
		for _, l := range dashboard.Layout {
			if l.WidgetID != widgetID {
				newLayout = append(newLayout, l)
			}
		}
		dashboard.Layout = newLayout
		s.dashboardRepo.UpdateDashboard(ctx, dashObjID, dashboard)
	}

	return 200, nil
}

// === AI Operations ===

func (s *dashboardService) GenerateBlueprints(ctx context.Context, userID, chatID, streamID string, userPrompt string) (uint32, error) {
	log.Printf("[DASHBOARD] GenerateBlueprints -> userID: %s, chatID: %s", userID, chatID)

	// 1. Send initial progress
	s.sendDashboardProgress(userID, chatID, streamID, "", "generating", "Analyzing your database schema...", 10)

	// 2. Get chat and schema context
	chat, connInfo, dbType, err := s.getChatContext(chatID)
	if err != nil {
		return 500, fmt.Errorf("failed to get chat context: %v", err)
	}

	s.sendDashboardProgress(userID, chatID, streamID, "", "generating", "Planning dashboard blueprints...", 30)

	// 3. Build LLM messages (no schema — LLM discovers via tool calls)
	defaultPrompt := "Suggest dashboard blueprints for this database based on the available tables and data."
	if userPrompt != "" {
		defaultPrompt = fmt.Sprintf("The user wants to create a dashboard with the following description: %s\n\nSuggest dashboard blueprints that match this request, using the available tables and data.", userPrompt)
	}
	messages := s.buildDashboardLLMMessages(chat, constants.DashboardBlueprintSystemPrompt, defaultPrompt, dbType, "24h")

	// 5. Get LLM client
	llmClient, selectedModel, err := s.getLLMClient(chat)
	if err != nil {
		return 500, fmt.Errorf("failed to get LLM client: %v", err)
	}
	_ = selectedModel
	_ = connInfo

	// 6. Build tool executor and tools
	toolExecutor := BuildDashboardToolExecutor(s.dbManager, chatID, dbType, s.kbRepo)
	tools := GetDashboardTools(constants.DashboardModeBlueprint)

	// 7. Configure tool calling
	toolCallConfig := llm.ToolCallConfig{
		MaxIterations: constants.DashboardMaxToolIterations,
		DBType:        dbType,
		SystemPrompt:  constants.DashboardBlueprintSystemPrompt,
		OnToolCall: func(call llm.ToolCall) {
			if call.Name == constants.DashboardFinalResponseToolName {
				s.sendDashboardProgress(userID, chatID, streamID, "", "generating", "Preparing blueprint suggestions...", 80)
			} else if explanation, ok := call.Arguments["explanation"].(string); ok && explanation != "" {
				s.sendDashboardProgress(userID, chatID, streamID, "", "generating", fmt.Sprintf("Exploring: %s", explanation), 50)
			}
		},
		OnToolResult: func(call llm.ToolCall, result llm.ToolResult) {
			if result.IsError {
				log.Printf("[DASHBOARD] Blueprint tool %s error: %s", call.Name, result.Content)
			}
		},
		OnIteration: func(iteration int, toolCallCount int) {
			log.Printf("[DASHBOARD] Blueprint generation iteration %d, total calls: %d", iteration, toolCallCount)
		},
	}

	// 8. Call LLM with tools
	toolResult, err := llmClient.GenerateWithTools(ctx, messages, tools, toolExecutor, toolCallConfig)
	if err != nil {
		log.Printf("[DASHBOARD] Blueprint generation LLM error: %v", err)
		return 500, fmt.Errorf("failed to generate blueprints: %v", err)
	}

	log.Printf("[DASHBOARD] Blueprint generation completed: %d iterations, %d tool calls", toolResult.Iterations, toolResult.TotalCalls)

	// 9. Parse blueprint response
	blueprints, err := s.parseBlueprintResponse(toolResult.Response)
	if err != nil {
		log.Printf("[DASHBOARD] Failed to parse blueprint response: %v, raw: %s", err, toolResult.Response)
		return 500, fmt.Errorf("failed to parse blueprint response: %v", err)
	}

	// 10. Cache blueprints in Redis for later selection (horizontally scalable)
	s.cacheBlueprintsToRedis(chatID, blueprints)

	// 11. Send blueprints via SSE
	blueprintDTOs := make([]dtos.BlueprintDTO, 0, len(blueprints))
	for i, bp := range blueprints {
		widgetDTOs := make([]dtos.BlueprintWidgetDTO, 0, len(bp.ProposedWidgets))
		for _, w := range bp.ProposedWidgets {
			widgetDTOs = append(widgetDTOs, dtos.BlueprintWidgetDTO{
				Title:      w.Title,
				WidgetType: w.WidgetType,
			})
		}
		blueprintDTOs = append(blueprintDTOs, dtos.BlueprintDTO{
			Index:           i,
			Name:            bp.Name,
			Description:     bp.Description,
			TemplateType:    bp.TemplateType,
			ProposedWidgets: widgetDTOs,
		})
	}

	if s.streamHandler != nil {
		s.streamHandler.HandleStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: constants.SSEEventDashboardBlueprints,
			Data: dtos.DashboardBlueprintEvent{
				Blueprints: blueprintDTOs,
			},
		})
	}

	return 200, nil
}

func (s *dashboardService) CreateFromBlueprints(ctx context.Context, userID, chatID, streamID string, req *dtos.CreateFromBlueprintsRequest) (uint32, error) {
	log.Printf("[DASHBOARD] CreateFromBlueprints -> userID: %s, chatID: %s, indices: %v", userID, chatID, req.BlueprintIndices)

	// 1. Retrieve cached blueprints from Redis
	blueprints, exists := s.getBlueprintsFromRedis(chatID)

	if !exists || len(blueprints) == 0 {
		return 400, fmt.Errorf("no cached blueprints found for this chat; please generate blueprints first")
	}

	// 2. Validate indices
	for _, idx := range req.BlueprintIndices {
		if idx < 0 || idx >= len(blueprints) {
			return 400, fmt.Errorf("invalid blueprint index: %d (available: 0-%d)", idx, len(blueprints)-1)
		}
	}

	// 3. Get chat context
	chat, _, dbType, err := s.getChatContext(chatID)
	if err != nil {
		return 500, fmt.Errorf("failed to get chat context: %v", err)
	}

	// 4. Get LLM client
	llmClient, selectedModel, err := s.getLLMClient(chat)
	if err != nil {
		return 500, fmt.Errorf("failed to get LLM client: %v", err)
	}

	userObjID, _ := primitive.ObjectIDFromHex(userID)
	chatObjID, _ := primitive.ObjectIDFromHex(chatID)

	// 5. Generate each selected blueprint as a full dashboard (sequentially to avoid LLM rate limits)
	for bpIdx, selectedIdx := range req.BlueprintIndices {
		blueprint := blueprints[selectedIdx]
		progress := 10 + (bpIdx * 90 / len(req.BlueprintIndices))

		s.sendDashboardProgress(userID, chatID, streamID, "", "generating",
			fmt.Sprintf("Building dashboard %d/%d: %s...", bpIdx+1, len(req.BlueprintIndices), blueprint.Name), progress)

		// Build a specific prompt from the blueprint
		blueprintPrompt := s.buildBlueprintGenerationPrompt(blueprint)

		// Build LLM messages
		messages := s.buildDashboardLLMMessages(chat, constants.DashboardGenerationSystemPrompt, blueprintPrompt, dbType, "24h")

		// Build tool executor and tools
		toolExecutor := BuildDashboardToolExecutor(s.dbManager, chatID, dbType, s.kbRepo)
		tools := GetDashboardTools(constants.DashboardModeGenerate)

		toolCallConfig := llm.ToolCallConfig{
			MaxIterations: constants.DashboardMaxToolIterations,
			DBType:        dbType,
			ModelID:       selectedModel,
			SystemPrompt:  constants.DashboardGenerationSystemPrompt,
			OnToolCall: func(call llm.ToolCall) {
				if call.Name == constants.DashboardFinalResponseToolName {
					s.sendDashboardProgress(userID, chatID, streamID, "", "finalizing",
						fmt.Sprintf("Finalizing dashboard: %s", blueprint.Name), progress+30)
				} else if call.Name == constants.DashboardExecuteQueryToolName {
					s.sendDashboardProgress(userID, chatID, streamID, "", "testing_queries",
						"Curating & testing the widget queries. This may take a few moments...", progress+15)
				}
			},
			OnToolResult: func(call llm.ToolCall, result llm.ToolResult) {
				if result.IsError {
					log.Printf("[DASHBOARD] Tool %s error during generation: %s", call.Name, result.Content)
				}
			},
			OnIteration: func(iteration int, toolCallCount int) {
				log.Printf("[DASHBOARD] Dashboard generation iteration %d, total calls: %d", iteration, toolCallCount)
			},
		}

		// Call LLM
		toolResult, err := llmClient.GenerateWithTools(ctx, messages, tools, toolExecutor, toolCallConfig)
		if err != nil {
			log.Printf("[DASHBOARD] Failed to generate dashboard for blueprint '%s': %v", blueprint.Name, err)
			s.sendDashboardProgress(userID, chatID, streamID, "", "generating",
				fmt.Sprintf("Failed to generate dashboard '%s': %v", blueprint.Name, err), progress)
			continue
		}

		// Parse the dashboard response
		dashboardConfig, err := s.parseDashboardResponse(toolResult.Response)
		if err != nil {
			log.Printf("[DASHBOARD] Failed to parse dashboard response for '%s': %v", blueprint.Name, err)
			continue
		}

		// Create dashboard and widgets in DB
		dashboard, widgets, err := s.createDashboardFromConfig(ctx, userObjID, chatObjID, dashboardConfig, blueprint.TemplateType, selectedModel)
		if err != nil {
			log.Printf("[DASHBOARD] Failed to create dashboard '%s' in DB: %v", blueprint.Name, err)
			continue
		}

		// Send completion event for this dashboard
		resp := s.dashboardToResponse(dashboard, widgets)
		if s.streamHandler != nil {
			s.streamHandler.HandleStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: constants.SSEEventDashboardGenerationComplete,
				Data: dtos.DashboardGenerationCompleteEvent{
					DashboardID: dashboard.ID.Hex(),
					Dashboard:   *resp,
				},
			})
		}

		// Refresh all widget data
		go func(dashID string) {
			refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			s.RefreshDashboard(refreshCtx, userID, chatID, dashID, streamID)
		}(dashboard.ID.Hex())
	}

	// Clean up cached blueprints from Redis
	s.deleteBlueprintsFromRedis(chatID)

	return 200, nil
}

func (s *dashboardService) RegenerateDashboard(ctx context.Context, userID, chatID, dashboardID, streamID string, req *dtos.RegenerateDashboardRequest) (uint32, error) {
	log.Printf("[DASHBOARD] RegenerateDashboard -> dashboardID: %s, reason: %s", dashboardID, req.Reason)

	dashObjID, err := primitive.ObjectIDFromHex(dashboardID)
	if err != nil {
		return 400, fmt.Errorf("invalid dashboard ID: %s", dashboardID)
	}

	// 1. Get existing dashboard
	dashboard, err := s.dashboardRepo.FindDashboardByID(ctx, dashObjID)
	if err != nil {
		return 404, fmt.Errorf("dashboard not found: %v", err)
	}

	if dashboard.UserID.Hex() != userID || dashboard.ChatID.Hex() != chatID {
		return 403, fmt.Errorf("unauthorized access to dashboard")
	}

	existingWidgets, _ := s.dashboardRepo.FindWidgetsByDashboardID(ctx, dashObjID)

	// 2. Get chat context
	chat, _, dbType, err := s.getChatContext(chatID)
	if err != nil {
		return 500, fmt.Errorf("failed to get chat context: %v", err)
	}

	s.sendDashboardProgress(userID, chatID, streamID, dashboardID, "generating", "Regenerating dashboard...", 10)

	// 3. Build context-aware prompt
	var regeneratePrompt string
	if req.Reason == constants.RegenerateReasonVariant {
		// Build context of what current dashboard has so LLM generates something different
		existingTitles := make([]string, 0, len(existingWidgets))
		for _, w := range existingWidgets {
			existingTitles = append(existingTitles, fmt.Sprintf("%s (%s)", w.Title, w.WidgetType))
		}
		regeneratePrompt = fmt.Sprintf(
			"Regenerate the dashboard '%s' with a DIFFERENT variant. The current dashboard has these widgets: [%s]. "+
				"Create a new set of widgets that explore DIFFERENT aspects of the same data. "+
				"Avoid duplicating the same metrics — show alternative insights, breakdowns, or perspectives.",
			dashboard.Name, joinStrings(existingTitles, ", "))
	} else {
		regeneratePrompt = fmt.Sprintf(
			"Regenerate the dashboard '%s' because the database schema has changed. "+
				"Re-analyze the schema and create updated widgets that reflect the current database structure. "+
				"Previous dashboard description: %s",
			dashboard.Name, dashboard.Description)
	}

	// 4. Build LLM messages and call
	messages := s.buildDashboardLLMMessages(chat, constants.DashboardGenerationSystemPrompt, regeneratePrompt, dbType, dashboard.TimeRange)

	llmClient, selectedModel, err := s.getLLMClient(chat)
	if err != nil {
		return 500, fmt.Errorf("failed to get LLM client: %v", err)
	}

	toolExecutor := BuildDashboardToolExecutor(s.dbManager, chatID, dbType, s.kbRepo)
	tools := GetDashboardTools(constants.DashboardModeRegenerate)

	toolCallConfig := llm.ToolCallConfig{
		MaxIterations: constants.DashboardMaxToolIterations,
		DBType:        dbType,
		ModelID:       selectedModel,
		SystemPrompt:  constants.DashboardGenerationSystemPrompt,
		OnToolCall: func(call llm.ToolCall) {
			if call.Name == constants.DashboardFinalResponseToolName {
				s.sendDashboardProgress(userID, chatID, streamID, dashboardID, "finalizing", "Finalizing regenerated dashboard...", 80)
			} else if call.Name == constants.DashboardExecuteQueryToolName {
				s.sendDashboardProgress(userID, chatID, streamID, dashboardID, "testing_queries", "Testing widget queries...", 50)
			}
		},
		OnToolResult: func(call llm.ToolCall, result llm.ToolResult) {},
		OnIteration: func(iteration int, toolCallCount int) {
			log.Printf("[DASHBOARD] Regenerate iteration %d, total calls: %d", iteration, toolCallCount)
		},
	}

	toolResult, err := llmClient.GenerateWithTools(ctx, messages, tools, toolExecutor, toolCallConfig)
	if err != nil {
		return 500, fmt.Errorf("failed to regenerate dashboard: %v", err)
	}

	// 5. Parse response
	dashboardConfig, err := s.parseDashboardResponse(toolResult.Response)
	if err != nil {
		return 500, fmt.Errorf("failed to parse regenerated dashboard response: %v", err)
	}

	// 6. Delete old widgets
	if err := s.dashboardRepo.DeleteWidgetsByDashboardID(ctx, dashObjID); err != nil {
		log.Printf("[DASHBOARD] Warning: failed to delete old widgets: %v", err)
	}

	// 7. Update dashboard and create new widgets
	dashboard.Name = s.getStringFromConfig(dashboardConfig, "dashboard_name", dashboard.Name)
	dashboard.Description = s.getStringFromConfig(dashboardConfig, "dashboard_description", dashboard.Description)
	dashboard.LLMModel = selectedModel
	dashboard.Layout = []models.WidgetLayout{} // Reset layout

	userObjID, _ := primitive.ObjectIDFromHex(userID)
	chatObjID, _ := primitive.ObjectIDFromHex(chatID)

	newWidgets, layout := s.createWidgetsFromConfig(ctx, dashObjID, chatObjID, userObjID, dashboardConfig, selectedModel)
	dashboard.Layout = layout

	if err := s.dashboardRepo.UpdateDashboard(ctx, dashObjID, dashboard); err != nil {
		return 500, fmt.Errorf("failed to update regenerated dashboard: %v", err)
	}

	// 8. Send completion event
	resp := s.dashboardToResponse(dashboard, newWidgets)
	if s.streamHandler != nil {
		s.streamHandler.HandleStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: constants.SSEEventDashboardGenerationComplete,
			Data: dtos.DashboardGenerationCompleteEvent{
				DashboardID: dashboard.ID.Hex(),
				Dashboard:   *resp,
			},
		})
	}

	// 9. Refresh widget data
	go func() {
		refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.RefreshDashboard(refreshCtx, userID, chatID, dashboardID, streamID)
	}()

	return 200, nil
}

// === Data Refresh ===

func (s *dashboardService) RefreshDashboard(ctx context.Context, userID, chatID, dashboardID, streamID string) (uint32, error) {
	log.Printf("RefreshDashboard called - userID: %s, chatID: %s, dashboardID: %s, streamID: %s", userID, chatID, dashboardID, streamID)

	// Check if database connection exists
	isDBConnected := s.dbManager.IsConnected(chatID)
	log.Printf("RefreshDashboard -> DB connection check: %v", isDBConnected)
	if !isDBConnected {
		return 400, fmt.Errorf("database connection not found for this chat - please connect to database first")
	}

	// Check if SSE stream exists
	hasStream := s.streamHandler != nil && s.streamHandler.HasStream(userID, chatID, streamID)
	log.Printf("RefreshDashboard -> SSE stream check: streamHandler=%v, hasStream=%v", s.streamHandler != nil, hasStream)
	if !hasStream {
		return 400, fmt.Errorf("SSE stream not found - please ensure you are connected before refreshing")
	}

	dashObjID, err := primitive.ObjectIDFromHex(dashboardID)
	if err != nil {
		return 400, fmt.Errorf("invalid dashboard ID: %s", dashboardID)
	}

	dashboard, err := s.dashboardRepo.FindDashboardByID(ctx, dashObjID)
	if err != nil {
		return 404, fmt.Errorf("dashboard not found: %v", err)
	}

	if dashboard.UserID.Hex() != userID || dashboard.ChatID.Hex() != chatID {
		return 403, fmt.Errorf("unauthorized access to dashboard")
	}

	widgets, err := s.dashboardRepo.FindWidgetsByDashboardID(ctx, dashObjID)
	if err != nil {
		return 500, fmt.Errorf("failed to fetch widgets: %v", err)
	}

	// Execute all widget queries in parallel
	var wg sync.WaitGroup
	for _, widget := range widgets {
		wg.Add(1)
		go func(w *models.Widget) {
			defer wg.Done()
			s.refreshSingleWidget(ctx, userID, chatID, streamID, w)
		}(widget)
	}
	wg.Wait()

	return 200, nil
}

func (s *dashboardService) RefreshWidget(ctx context.Context, userID, chatID, dashboardID, widgetID, streamID string) (uint32, error) {
	// Check if database connection exists
	if !s.dbManager.IsConnected(chatID) {
		return 400, fmt.Errorf("database connection not found for this chat - please connect to database first")
	}

	// Check if SSE stream exists
	if s.streamHandler == nil || !s.streamHandler.HasStream(userID, chatID, streamID) {
		return 400, fmt.Errorf("SSE stream not found - please ensure you are connected before refreshing")
	}

	widgetObjID, err := primitive.ObjectIDFromHex(widgetID)
	if err != nil {
		return 400, fmt.Errorf("invalid widget ID: %s", widgetID)
	}

	widget, err := s.dashboardRepo.FindWidgetByID(ctx, widgetObjID)
	if err != nil {
		return 404, fmt.Errorf("widget not found: %v", err)
	}

	if widget.UserID.Hex() != userID || widget.ChatID.Hex() != chatID || widget.DashboardID.Hex() != dashboardID {
		return 403, fmt.Errorf("unauthorized access to widget")
	}

	s.refreshSingleWidget(ctx, userID, chatID, streamID, widget)
	return 200, nil
}

// refreshSingleWidget executes a widget's query and sends the result via SSE
func (s *dashboardService) refreshSingleWidget(ctx context.Context, userID, chatID, streamID string, widget *models.Widget) {
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(constants.WidgetQueryTimeoutSeconds)*time.Second)
	defer cancel()

	widgetID := widget.ID.Hex()
	startTime := time.Now()

	// Execute the widget query
	result, queryErr := s.dbManager.ExecuteQuery(queryCtx, chatID, "", "", "", widget.Query, "SELECT", false, false)

	executionTimeMs := float64(time.Since(startTime).Milliseconds())

	if queryErr != nil {
		log.Printf("[DASHBOARD] Widget query failed - WidgetID: %s, Error: %s", widgetID, queryErr.Message)
		if s.streamHandler != nil {
			s.streamHandler.HandleStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: constants.SSEEventDashboardWidgetError,
				Data: dtos.DashboardWidgetDataEvent{
					WidgetID:        widgetID,
					Error:           queryErr.Message,
					ExecutionTimeMs: executionTimeMs,
				},
			})
		}
		return
	}

	// Parse result data
	var data []map[string]interface{}
	if result != nil && result.Result != nil {
		switch v := result.Result.(type) {
		case []map[string]interface{}:
			data = v
		case map[string]interface{}:
			// MongoDB wraps query results as {"results": [...]} or {"count": N}
			if results, ok := v["results"]; ok {
				// Marshal the results sub-field and unmarshal as []map
				if b, err := json.Marshal(results); err == nil {
					json.Unmarshal(b, &data)
				}
			} else {
				// Single result like {"count": N} — wrap as a one-element array
				data = []map[string]interface{}{v}
			}
		default:
			// Fallback: JSON round-trip
			if b, err := json.Marshal(v); err == nil {
				json.Unmarshal(b, &data)
			}
		}
	}

	// Send via SSE
	if s.streamHandler != nil {
		rowCount := 0
		if data != nil {
			rowCount = len(data)
		}
		s.streamHandler.HandleStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: constants.SSEEventDashboardWidgetData,
			Data: dtos.DashboardWidgetDataEvent{
				WidgetID:        widgetID,
				Data:            data,
				RowCount:        rowCount,
				ExecutionTimeMs: executionTimeMs,
			},
		})
	}
}

// === Response Mapping Helpers ===

func (s *dashboardService) dashboardToResponse(dashboard *models.Dashboard, widgets []*models.Widget) *dtos.DashboardResponse {
	layoutDTOs := make([]dtos.WidgetLayoutDTO, len(dashboard.Layout))
	for i, l := range dashboard.Layout {
		layoutDTOs[i] = dtos.WidgetLayoutDTO{
			WidgetID: l.WidgetID,
			X:        l.X,
			Y:        l.Y,
			W:        l.W,
			H:        l.H,
			MinW:     l.MinW,
			MinH:     l.MinH,
		}
	}

	widgetDTOs := make([]dtos.WidgetResponse, 0, len(widgets))
	for _, w := range widgets {
		widgetDTOs = append(widgetDTOs, *s.widgetToResponse(w))
	}

	return &dtos.DashboardResponse{
		ID:              dashboard.ID.Hex(),
		ChatID:          dashboard.ChatID.Hex(),
		Name:            dashboard.Name,
		Description:     dashboard.Description,
		TemplateType:    dashboard.TemplateType,
		IsDefault:       dashboard.IsDefault,
		RefreshInterval: dashboard.RefreshInterval,
		TimeRange:       dashboard.TimeRange,
		Layout:          layoutDTOs,
		Widgets:         widgetDTOs,
		CreatedAt:       dashboard.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       dashboard.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *dashboardService) widgetToResponse(widget *models.Widget) *dtos.WidgetResponse {
	resp := &dtos.WidgetResponse{
		ID:              widget.ID.Hex(),
		DashboardID:     widget.DashboardID.Hex(),
		Title:           widget.Title,
		Description:     widget.Description,
		WidgetType:      widget.WidgetType,
		Query:           widget.Query,
		QueryType:       widget.QueryType,
		Tables:          widget.Tables,
		ChartConfigJSON: widget.ChartConfigJSON,
		GeneratedPrompt: widget.GeneratedPrompt,
		CreatedAt:       widget.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       widget.UpdatedAt.Format(time.RFC3339),
	}

	if widget.LastRefreshedAt != nil {
		t := widget.LastRefreshedAt.Time()
		resp.LastRefreshedAt = t.Format(time.RFC3339)
	}

	if widget.StatConfig != nil {
		resp.StatConfig = &dtos.StatWidgetConfigDTO{
			ValueQuery:      widget.StatConfig.ValueQuery,
			ComparisonQuery: widget.StatConfig.ComparisonQuery,
			Format:          widget.StatConfig.Format,
			Prefix:          widget.StatConfig.Prefix,
			Suffix:          widget.StatConfig.Suffix,
			DecimalPlaces:   widget.StatConfig.DecimalPlaces,
			TrendDirection:  widget.StatConfig.TrendDirection,
		}
	}

	if widget.TableConfig != nil {
		columns := make([]dtos.TableWidgetColumnDTO, len(widget.TableConfig.Columns))
		for i, c := range widget.TableConfig.Columns {
			columns[i] = dtos.TableWidgetColumnDTO{
				Key:    c.Key,
				Label:  c.Label,
				Format: c.Format,
				Width:  c.Width,
			}
		}
		resp.TableConfig = &dtos.TableWidgetConfigDTO{
			Columns:       columns,
			SortBy:        widget.TableConfig.SortBy,
			SortDirection: widget.TableConfig.SortDirection,
			PageSize:      widget.TableConfig.PageSize,
		}
	}

	return resp
}

// === AI Helper Methods ===

// getChatContext retrieves the chat, connection info, and db type for dashboard operations
func (s *dashboardService) getChatContext(chatID string) (*models.Chat, interface{}, string, error) {
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid chat ID: %s", chatID)
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("chat not found: %v", err)
	}

	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return nil, nil, "", fmt.Errorf("no active database connection for chat %s", chatID)
	}

	return chat, connInfo, connInfo.Config.Type, nil
}

// buildDashboardLLMMessages constructs the LLM message array for dashboard operations.
// Does NOT include the full schema — the LLM discovers schema via tool calls
// (list_all_tables, get_table_info, get_knowledge_base) to keep token usage minimal.
func (s *dashboardService) buildDashboardLLMMessages(chat *models.Chat, systemPrompt, userPrompt, dbType, timeRange string) []*models.LLMMessage {
	now := time.Now()

	// System message — DB instructions + dashboard-mode prompt only, NO schema
	systemContent := map[string]interface{}{}

	// Inject per-database-type query syntax instructions
	dbInstructions := constants.GetDashboardDBInstructions(dbType)
	systemContent["dashboard_instructions"] = systemPrompt + "\n" + dbInstructions

	// Inject time range context so queries can use appropriate time-based filters
	if timeRange != "" {
		systemContent["time_range_context"] = fmt.Sprintf(
			"The user's preferred dashboard time range is '%s'. "+
				"When writing time-based queries (e.g., trends, recent data), "+
				"use this as the default lookback period unless the data has no temporal dimension. "+
				"For example, if time_range is '7d', filter data from the last 7 days. "+
				"If the schema has no date/time columns, ignore this setting.",
			timeRange,
		)
	}

	systemMessage := &models.LLMMessage{
		ChatID:    chat.ID,
		UserID:    chat.UserID,
		Role:      string(constants.MessageTypeSystem),
		Content:   systemContent,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// User message with the dashboard request
	userMessage := &models.LLMMessage{
		ChatID: chat.ID,
		UserID: chat.UserID,
		Role:   string(constants.MessageTypeUser),
		Content: map[string]interface{}{
			"user_message": userPrompt,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	return []*models.LLMMessage{systemMessage, userMessage}
}

// getLLMClient selects the appropriate LLM client based on chat preferences
func (s *dashboardService) getLLMClient(chat *models.Chat) (llm.Client, string, error) {
	var selectedModel string

	// Priority: chat's preferred model > settings model > provider default
	if chat.PreferredLLMModel != nil && *chat.PreferredLLMModel != "" {
		selectedModel = *chat.PreferredLLMModel
	} else if chat.Settings.SelectedLLMModel != "" {
		selectedModel = chat.Settings.SelectedLLMModel
	}

	// Fallback to searching enabled providers
	if selectedModel == "" {
		providers := []string{constants.OpenAI, constants.Gemini, constants.Claude, constants.Ollama}
		for _, provider := range providers {
			if defaultModel := constants.GetDefaultModelForProvider(provider); defaultModel != nil && defaultModel.IsEnabled {
				selectedModel = defaultModel.ID
				break
			}
		}
	}

	if selectedModel == "" {
		return nil, "", fmt.Errorf("no LLM model available; please configure at least one provider")
	}

	// Get the provider client
	modelInfo := constants.GetLLMModel(selectedModel)
	if modelInfo == nil {
		return nil, "", fmt.Errorf("unknown LLM model: %s", selectedModel)
	}

	client, err := s.llmManager.GetClient(modelInfo.Provider)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get LLM client for provider '%s': %v", modelInfo.Provider, err)
	}

	return client, selectedModel, nil
}

// parseBlueprintResponse parses the LLM tool response into DashboardBlueprint models
func (s *dashboardService) parseBlueprintResponse(response string) ([]models.DashboardBlueprint, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %v", err)
	}

	blueprintsRaw, ok := parsed["blueprints"].([]interface{})
	if !ok || len(blueprintsRaw) == 0 {
		return nil, fmt.Errorf("no blueprints found in response")
	}

	blueprints := make([]models.DashboardBlueprint, 0, len(blueprintsRaw))
	for _, bpRaw := range blueprintsRaw {
		bpMap, ok := bpRaw.(map[string]interface{})
		if !ok {
			continue
		}

		bp := models.DashboardBlueprint{
			Name:         getStringVal(bpMap, "name"),
			Description:  getStringVal(bpMap, "description"),
			TemplateType: getStringVal(bpMap, "template_type"),
		}

		if widgetsRaw, ok := bpMap["proposed_widgets"].([]interface{}); ok {
			for _, wRaw := range widgetsRaw {
				wMap, ok := wRaw.(map[string]interface{})
				if !ok {
					continue
				}
				bp.ProposedWidgets = append(bp.ProposedWidgets, models.BlueprintWidgetPreview{
					Title:      getStringVal(wMap, "title"),
					WidgetType: getStringVal(wMap, "widget_type"),
				})
			}
		}

		if bp.Name != "" {
			blueprints = append(blueprints, bp)
		}
	}

	// Limit to max suggestions
	if len(blueprints) > constants.MaxBlueprintSuggestions {
		blueprints = blueprints[:constants.MaxBlueprintSuggestions]
	}

	return blueprints, nil
}

// parseDashboardResponse parses the LLM tool response into a dashboard config map
func (s *dashboardService) parseDashboardResponse(response string) (map[string]interface{}, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %v", err)
	}
	return parsed, nil
}

// buildBlueprintGenerationPrompt creates a detailed prompt from a blueprint for full generation
func (s *dashboardService) buildBlueprintGenerationPrompt(blueprint models.DashboardBlueprint) string {
	widgetDescriptions := ""
	for i, w := range blueprint.ProposedWidgets {
		widgetDescriptions += fmt.Sprintf("  %d. %s (type: %s)\n", i+1, w.Title, w.WidgetType)
	}

	return fmt.Sprintf(
		"Generate a complete dashboard called '%s'.\n"+
			"Description: %s\n"+
			"Template type: %s\n\n"+
			"Create the following widgets with working, tested queries:\n%s\n"+
			"Make sure to test each query with execute_dashboard_query before including it.",
		blueprint.Name,
		blueprint.Description,
		blueprint.TemplateType,
		widgetDescriptions,
	)
}

// createDashboardFromConfig creates a dashboard and its widgets from the LLM-generated config
func (s *dashboardService) createDashboardFromConfig(
	ctx context.Context,
	userObjID, chatObjID primitive.ObjectID,
	config map[string]interface{},
	templateType, selectedModel string,
) (*models.Dashboard, []*models.Widget, error) {
	// Check dashboard limit
	count, err := s.dashboardRepo.CountDashboardsByChatID(ctx, chatObjID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to count dashboards: %v", err)
	}

	dashboardName := s.getStringFromConfig(config, "dashboard_name", "Generated Dashboard")
	dashboard := models.NewDashboard(userObjID, chatObjID, dashboardName)
	dashboard.Description = s.getStringFromConfig(config, "dashboard_description", "")
	dashboard.TemplateType = templateType
	dashboard.LLMModel = selectedModel
	dashboard.IsDefault = count == 0

	if refreshInterval, ok := config["suggested_refresh_interval"].(float64); ok {
		dashboard.RefreshInterval = int(refreshInterval)
	}
	if timeRange, ok := config["suggested_time_range"].(string); ok && timeRange != "" {
		dashboard.TimeRange = timeRange
	}

	if err := s.dashboardRepo.CreateDashboard(ctx, dashboard); err != nil {
		return nil, nil, fmt.Errorf("failed to create dashboard: %v", err)
	}

	// Create widgets
	widgets, layout := s.createWidgetsFromConfig(ctx, dashboard.ID, chatObjID, userObjID, config, selectedModel)
	dashboard.Layout = layout

	// Update dashboard with layout
	if err := s.dashboardRepo.UpdateDashboard(ctx, dashboard.ID, dashboard); err != nil {
		log.Printf("[DASHBOARD] Warning: failed to update dashboard layout: %v", err)
	}

	return dashboard, widgets, nil
}

// createWidgetsFromConfig creates widgets from the AI config and returns them with layout
func (s *dashboardService) createWidgetsFromConfig(
	ctx context.Context,
	dashboardID, chatObjID, userObjID primitive.ObjectID,
	config map[string]interface{},
	selectedModel string,
) ([]*models.Widget, []models.WidgetLayout) {
	widgetsRaw, ok := config["widgets"].([]interface{})
	if !ok {
		return nil, nil
	}

	widgets := make([]*models.Widget, 0, len(widgetsRaw))
	layout := make([]models.WidgetLayout, 0, len(widgetsRaw))

	curX, curY := 0, 0

	for _, wRaw := range widgetsRaw {
		wConfig, ok := wRaw.(map[string]interface{})
		if !ok {
			continue
		}

		widget := s.createWidgetFromConfig(ctx, dashboardID, chatObjID, userObjID, wConfig, selectedModel)
		if widget == nil {
			continue
		}

		widgets = append(widgets, widget)

		// Calculate layout
		lc := getLayoutFromWidgetConfig(wConfig)
		if curX+lc.W > 12 {
			curX = 0
			curY += lc.H
		}

		layout = append(layout, models.WidgetLayout{
			WidgetID: widget.ID.Hex(),
			X:        curX,
			Y:        curY,
			W:        lc.W,
			H:        lc.H,
		})

		curX += lc.W
		if curX >= 12 {
			curX = 0
			curY += lc.H
		}
	}

	return widgets, layout
}

// createWidgetFromConfig creates a single widget from AI config and saves it to DB
func (s *dashboardService) createWidgetFromConfig(
	ctx context.Context,
	dashboardID, chatObjID, userObjID primitive.ObjectID,
	wConfig map[string]interface{},
	selectedModel string,
) *models.Widget {
	title := getStringVal(wConfig, "title")
	widgetType := getStringVal(wConfig, "widget_type")
	query := getStringVal(wConfig, "query")

	if title == "" || widgetType == "" {
		return nil
	}

	widget := models.NewWidget(dashboardID, chatObjID, userObjID, title, widgetType, query)
	widget.Description = getStringVal(wConfig, "description")
	widget.Tables = getStringVal(wConfig, "tables")
	widget.LLMModel = selectedModel

	// Parse stat_config
	if statRaw, ok := wConfig["stat_config"].(map[string]interface{}); ok && widgetType == "stat" {
		widget.StatConfig = &models.StatWidgetConfig{
			ValueQuery:      getStringVal(statRaw, "value_query"),
			ComparisonQuery: getStringVal(statRaw, "comparison_query"),
			Format:          getStringVal(statRaw, "format"),
			Prefix:          getStringVal(statRaw, "prefix"),
			Suffix:          getStringVal(statRaw, "suffix"),
			TrendDirection:  getStringVal(statRaw, "trend_direction"),
		}
		if dp, ok := statRaw["decimal_places"].(float64); ok {
			widget.StatConfig.DecimalPlaces = int(dp)
		}
		// For stat widgets, use value_query as the primary query if main query is empty
		if widget.Query == "" && widget.StatConfig.ValueQuery != "" {
			widget.Query = widget.StatConfig.ValueQuery
		}
	}

	// Parse chart_config as JSON string
	if chartRaw, ok := wConfig["chart_config"].(map[string]interface{}); ok {
		if chartJSON, err := json.Marshal(chartRaw); err == nil {
			widget.ChartConfigJSON = string(chartJSON)
		}
	}

	// Parse table_config
	if tableRaw, ok := wConfig["table_config"].(map[string]interface{}); ok && widgetType == "table" {
		widget.TableConfig = &models.TableWidgetConfig{
			SortBy:        getStringVal(tableRaw, "sort_by"),
			SortDirection: getStringVal(tableRaw, "sort_direction"),
		}
		if ps, ok := tableRaw["page_size"].(float64); ok {
			widget.TableConfig.PageSize = int(ps)
		}
		if colsRaw, ok := tableRaw["columns"].([]interface{}); ok {
			for _, cRaw := range colsRaw {
				cMap, ok := cRaw.(map[string]interface{})
				if !ok {
					continue
				}
				widget.TableConfig.Columns = append(widget.TableConfig.Columns, models.TableWidgetColumn{
					Key:    getStringVal(cMap, "key"),
					Label:  getStringVal(cMap, "label"),
					Format: getStringVal(cMap, "format"),
					Width:  getStringVal(cMap, "width"),
				})
			}
		}
	}

	if err := s.dashboardRepo.CreateWidget(ctx, widget); err != nil {
		log.Printf("[DASHBOARD] Failed to create widget '%s': %v", title, err)
		return nil
	}

	return widget
}

// applyWidgetConfigUpdates updates an existing widget model from AI-generated config
func (s *dashboardService) applyWidgetConfigUpdates(widget *models.Widget, wConfig map[string]interface{}) {
	if title := getStringVal(wConfig, "title"); title != "" {
		widget.Title = title
	}
	if desc := getStringVal(wConfig, "description"); desc != "" {
		widget.Description = desc
	}
	if wType := getStringVal(wConfig, "widget_type"); wType != "" {
		widget.WidgetType = wType
	}
	if query := getStringVal(wConfig, "query"); query != "" {
		widget.Query = query
	}
	if tables := getStringVal(wConfig, "tables"); tables != "" {
		widget.Tables = tables
	}

	// Update stat_config if present
	if statRaw, ok := wConfig["stat_config"].(map[string]interface{}); ok {
		widget.StatConfig = &models.StatWidgetConfig{
			ValueQuery:      getStringVal(statRaw, "value_query"),
			ComparisonQuery: getStringVal(statRaw, "comparison_query"),
			Format:          getStringVal(statRaw, "format"),
			Prefix:          getStringVal(statRaw, "prefix"),
			Suffix:          getStringVal(statRaw, "suffix"),
			TrendDirection:  getStringVal(statRaw, "trend_direction"),
		}
		if dp, ok := statRaw["decimal_places"].(float64); ok {
			widget.StatConfig.DecimalPlaces = int(dp)
		}
	}

	// Update chart_config if present
	if chartRaw, ok := wConfig["chart_config"].(map[string]interface{}); ok {
		if chartJSON, err := json.Marshal(chartRaw); err == nil {
			widget.ChartConfigJSON = string(chartJSON)
		}
	}

	// Update table_config if present
	if tableRaw, ok := wConfig["table_config"].(map[string]interface{}); ok {
		widget.TableConfig = &models.TableWidgetConfig{
			SortBy:        getStringVal(tableRaw, "sort_by"),
			SortDirection: getStringVal(tableRaw, "sort_direction"),
		}
		if ps, ok := tableRaw["page_size"].(float64); ok {
			widget.TableConfig.PageSize = int(ps)
		}
		if colsRaw, ok := tableRaw["columns"].([]interface{}); ok {
			for _, cRaw := range colsRaw {
				cMap, ok := cRaw.(map[string]interface{})
				if !ok {
					continue
				}
				widget.TableConfig.Columns = append(widget.TableConfig.Columns, models.TableWidgetColumn{
					Key:    getStringVal(cMap, "key"),
					Label:  getStringVal(cMap, "label"),
					Format: getStringVal(cMap, "format"),
					Width:  getStringVal(cMap, "width"),
				})
			}
		}
	}
}

// === Redis Blueprint Cache ===

// cacheBlueprintsToRedis stores generated blueprints in Redis for cross-instance access
func (s *dashboardService) cacheBlueprintsToRedis(chatID string, blueprints []models.DashboardBlueprint) {
	ctx := context.Background()
	cacheKey := constants.DashboardBlueprintCachePrefix + chatID

	data, err := json.Marshal(blueprints)
	if err != nil {
		log.Printf("[DASHBOARD] Failed to marshal blueprints for Redis cache: %v", err)
		return
	}

	if err := s.redisRepo.SetCompressed(cacheKey, data, constants.DashboardBlueprintCacheTTL, ctx); err != nil {
		log.Printf("[DASHBOARD] Failed to cache blueprints in Redis: %v", err)
		return
	}

	log.Printf("[DASHBOARD] Cached %d blueprints in Redis for chat %s (TTL: %v)", len(blueprints), chatID, constants.DashboardBlueprintCacheTTL)
}

// getBlueprintsFromRedis retrieves cached blueprints from Redis
func (s *dashboardService) getBlueprintsFromRedis(chatID string) ([]models.DashboardBlueprint, bool) {
	ctx := context.Background()
	cacheKey := constants.DashboardBlueprintCachePrefix + chatID

	data, err := s.redisRepo.GetCompressed(cacheKey, ctx)
	if err != nil || data == nil {
		return nil, false
	}

	var blueprints []models.DashboardBlueprint
	if err := json.Unmarshal(data, &blueprints); err != nil {
		log.Printf("[DASHBOARD] Failed to unmarshal blueprints from Redis: %v", err)
		return nil, false
	}

	log.Printf("[DASHBOARD] Retrieved %d blueprints from Redis for chat %s", len(blueprints), chatID)
	return blueprints, true
}

// deleteBlueprintsFromRedis removes cached blueprints from Redis after creation
func (s *dashboardService) deleteBlueprintsFromRedis(chatID string) {
	ctx := context.Background()
	cacheKey := constants.DashboardBlueprintCachePrefix + chatID
	s.redisRepo.Del(cacheKey, ctx)
	log.Printf("[DASHBOARD] Deleted blueprint cache from Redis for chat %s", chatID)
}

// sendDashboardProgress sends a progress SSE event
func (s *dashboardService) sendDashboardProgress(userID, chatID, streamID, dashboardID, status, message string, progress int) {
	if s.streamHandler != nil {
		s.streamHandler.HandleStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: constants.SSEEventDashboardGenerationProgress,
			Data: dtos.DashboardGenerationProgressEvent{
				DashboardID: dashboardID,
				Status:      status,
				Message:     message,
				Progress:    progress,
			},
		})
	}
}

// getStringFromConfig safely extracts a string from a map with a default
func (s *dashboardService) getStringFromConfig(config map[string]interface{}, key, defaultVal string) string {
	if val, ok := config[key].(string); ok && val != "" {
		return val
	}
	return defaultVal
}

// === Package-level utility functions ===

// getStringVal safely extracts a string from a map
func getStringVal(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// joinStrings joins string slices with a separator
func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// replacePromptPlaceholder replaces a placeholder in a prompt string
func replacePromptPlaceholder(prompt, placeholder, value string) string {
	result := ""
	for i := 0; i < len(prompt); {
		if i+len(placeholder) <= len(prompt) && prompt[i:i+len(placeholder)] == placeholder {
			result += value
			i += len(placeholder)
		} else {
			result += string(prompt[i])
			i++
		}
	}
	return result
}

// layoutConfig holds widget layout dimensions
type layoutConfig struct {
	W int
	H int
}

// getLayoutFromWidgetConfig extracts layout dimensions from widget config with defaults
func getLayoutFromWidgetConfig(wConfig map[string]interface{}) layoutConfig {
	lc := layoutConfig{W: 6, H: 4} // Default: half-width, medium height

	if layoutRaw, ok := wConfig["layout"].(map[string]interface{}); ok {
		if w, ok := layoutRaw["w"].(float64); ok && w >= 1 && w <= 12 {
			lc.W = int(w)
		}
		if h, ok := layoutRaw["h"].(float64); ok && h >= 2 && h <= 6 {
			lc.H = int(h)
		}
	} else {
		// Infer defaults from widget type
		widgetType := getStringVal(wConfig, "widget_type")
		switch widgetType {
		case "stat":
			lc.W = 3
			lc.H = 2
		case "line", "area", "combo":
			lc.W = 6
			lc.H = 4
		case "bar", "pie":
			lc.W = 6
			lc.H = 4
		case "table":
			lc.W = 12
			lc.H = 4
		}
	}

	return lc
}
