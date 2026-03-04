package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/repositories"
	"neobase-ai/pkg/dbmanager"
	"neobase-ai/pkg/llm"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetDashboardTools returns the tool definitions for dashboard AI operations.
// The final response tool schema changes depending on the mode (blueprint vs generation).
func GetDashboardTools(mode string) []llm.ToolDefinition {
	// Core tools — available in ALL modes
	tools := []llm.ToolDefinition{
		{
			Name: constants.DashboardExecuteQueryToolName,
			Description: "Execute a read-only database query. " +
				"Use this to discover tables (e.g. SHOW TABLES, SELECT table_name FROM information_schema.tables, db.getCollectionNames()), " +
				"test widget queries, and explore data. " +
				"Only SELECT, SHOW, DESCRIBE, EXPLAIN, or MongoDB find/aggregate operations are allowed.",
			Parameters: constants.DashboardExecuteQueryToolSchema,
		},
		{
			Name: constants.DashboardGetKnowledgeBaseToolName,
			Description: "Get the knowledge base containing human-readable descriptions of tables and their fields. " +
				"Use this to understand what each table represents and what its columns/fields mean. " +
				"Returns descriptions only — use get_table_info for actual column types and constraints.",
			Parameters: constants.DashboardGetKnowledgeBaseToolSchema,
		},
		{
			Name: constants.DashboardGetTableInfoToolName,
			Description: "Get detailed schema information for specific tables or collections, " +
				"including column names, data types, constraints, indexes, and foreign keys. " +
				"Use this when you need more detail about specific tables.",
			Parameters: constants.GetTableInfoToolSchema,
		},
	}

	// Final response tool — schema depends on mode
	var responseSchema map[string]interface{}
	var responseDesc string

	switch mode {
	case constants.DashboardModeBlueprint:
		responseSchema = constants.DashboardBlueprintResponseSchema
		responseDesc = "Generate the final dashboard blueprint suggestions. " +
			"Call this when you have discovered the schema and are ready to suggest dashboard blueprints. " +
			"This MUST be the last tool you call."
	default:
		responseSchema = constants.DashboardFinalResponseSchema
		responseDesc = "Generate the final dashboard configuration with complete widget definitions. " +
			"Call this ONLY when you have tested all queries and are ready to submit the dashboard. " +
			"This MUST be the last tool you call."
	}

	tools = append(tools, llm.ToolDefinition{
		Name:        constants.DashboardFinalResponseToolName,
		Description: responseDesc,
		Parameters:  responseSchema,
	})

	return tools
}

// BuildDashboardToolExecutor creates a ToolExecutorFunc for dashboard AI operations.
// It handles execute_dashboard_query, get_table_info, list_all_tables, and get_knowledge_base.
func BuildDashboardToolExecutor(
	dbMgr *dbmanager.Manager,
	chatID string,
	dbType string,
	kbRepo repositories.KnowledgeBaseRepository,
) llm.ToolExecutorFunc {
	return func(ctx context.Context, call llm.ToolCall) (*llm.ToolResult, error) {
		switch call.Name {
		case constants.DashboardExecuteQueryToolName:
			return executeDashboardQuery(ctx, dbMgr, chatID, dbType, call)
		case constants.DashboardGetTableInfoToolName:
			return getDashboardTableInfo(ctx, dbMgr, chatID, dbType, call)
		case constants.DashboardGetKnowledgeBaseToolName:
			return getDashboardKnowledgeBase(ctx, kbRepo, chatID, call)
		default:
			return &llm.ToolResult{
				CallID:  call.ID,
				Name:    call.Name,
				Content: fmt.Sprintf("Unknown tool: %s", call.Name),
				IsError: true,
			}, nil
		}
	}
}

// executeDashboardQuery handles the execute_dashboard_query tool call for widget query testing
func executeDashboardQuery(
	ctx context.Context,
	dbMgr *dbmanager.Manager,
	chatID string,
	dbType string,
	call llm.ToolCall,
) (*llm.ToolResult, error) {
	query, _ := call.Arguments["query"].(string)
	explanation, _ := call.Arguments["explanation"].(string)

	if query == "" {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: "Error: 'query' argument is required",
			IsError: true,
		}, nil
	}

	// Validate read-only
	if !constants.IsReadOnlyQuery(query, dbType) {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: "Error: Only read-only queries (SELECT, SHOW, DESCRIBE, EXPLAIN, or MongoDB find/aggregate) are allowed for dashboard widgets.",
			IsError: true,
		}, nil
	}

	log.Printf("[DASHBOARD-TOOL] execute_dashboard_query -> chatID=%s explanation=%q query=%q", chatID, explanation, query)

	result, queryErr := dbMgr.ExecuteQuery(ctx, chatID, "", "", "", query, "SELECT", false, false)
	if queryErr != nil {
		errContent := fmt.Sprintf("Query execution error [%s]: %s\nDetails: %s", queryErr.Code, queryErr.Message, queryErr.Details)
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: llm.TruncateToolResult(errContent),
			IsError: true,
		}, nil
	}

	var content string
	if result.Result != nil {
		resultJSON, err := json.Marshal(result.Result)
		if err != nil {
			content = fmt.Sprintf("Query executed successfully but failed to serialize result: %v", err)
		} else {
			content = string(resultJSON)
		}
	} else {
		content = "Query executed successfully. No rows returned."
	}

	if result.ExecutionTime > 0 {
		content = fmt.Sprintf("Execution time: %dms\n%s", result.ExecutionTime, content)
	}

	return &llm.ToolResult{
		CallID:  call.ID,
		Name:    call.Name,
		Content: llm.TruncateToolResult(content),
		IsError: false,
	}, nil
}

// getDashboardTableInfo handles the get_table_info tool call for dashboard generation
func getDashboardTableInfo(
	ctx context.Context,
	dbMgr *dbmanager.Manager,
	chatID string,
	dbType string,
	call llm.ToolCall,
) (*llm.ToolResult, error) {
	tableNamesRaw, ok := call.Arguments["table_names"]
	if !ok {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: "Error: 'table_names' argument is required",
			IsError: true,
		}, nil
	}

	var tableNames []string
	switch v := tableNamesRaw.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				tableNames = append(tableNames, s)
			}
		}
	case []string:
		tableNames = v
	default:
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: fmt.Sprintf("Error: 'table_names' must be an array of strings, got %T", tableNamesRaw),
			IsError: true,
		}, nil
	}

	if len(tableNames) == 0 {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: "Error: 'table_names' must contain at least one table name",
			IsError: true,
		}, nil
	}

	log.Printf("[DASHBOARD-TOOL] get_table_info -> chatID=%s tables=%v", chatID, tableNames)

	dbConn, err := dbMgr.GetConnection(chatID)
	if err != nil {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: fmt.Sprintf("Error getting database connection: %v", err),
			IsError: true,
		}, nil
	}

	schema, err := dbMgr.GetSchemaManager().GetSchema(ctx, chatID, dbConn, dbType, tableNames)
	if err != nil {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: fmt.Sprintf("Error fetching schema for tables %v: %v", tableNames, err),
			IsError: true,
		}, nil
	}

	formatted := dbMgr.GetSchemaManager().FormatSchemaForLLM(schema)
	return &llm.ToolResult{
		CallID:  call.ID,
		Name:    call.Name,
		Content: llm.TruncateToolResult(formatted),
		IsError: false,
	}, nil
}

// getDashboardKnowledgeBase fetches the knowledge base table/field descriptions for the chat.
func getDashboardKnowledgeBase(
	ctx context.Context,
	kbRepo repositories.KnowledgeBaseRepository,
	chatID string,
	call llm.ToolCall,
) (*llm.ToolResult, error) {
	log.Printf("[DASHBOARD-TOOL] get_knowledge_base -> chatID=%s", chatID)

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: fmt.Sprintf("Error: invalid chat ID: %v", err),
			IsError: true,
		}, nil
	}

	kb, err := kbRepo.FindByChatID(ctx, chatObjID)
	if err != nil || kb == nil || len(kb.TableDescriptions) == 0 {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: "No knowledge base found for this chat. The user has not added descriptions for tables or fields yet. Use list_all_tables and get_table_info to discover the schema instead.",
			IsError: false,
		}, nil
	}

	// Format KB as a readable text
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Knowledge Base (%d tables):\n\n", len(kb.TableDescriptions)))
	for _, td := range kb.TableDescriptions {
		sb.WriteString(fmt.Sprintf("## %s\n", td.TableName))
		if td.Description != "" {
			sb.WriteString(fmt.Sprintf("   Description: %s\n", td.Description))
		}
		if len(td.FieldDescriptions) > 0 {
			sb.WriteString("   Fields:\n")
			for _, fd := range td.FieldDescriptions {
				sb.WriteString(fmt.Sprintf("   - %s: %s\n", fd.FieldName, fd.Description))
			}
		}
		sb.WriteString("\n")
	}

	return &llm.ToolResult{
		CallID:  call.ID,
		Name:    call.Name,
		Content: llm.TruncateToolResult(sb.String()),
		IsError: false,
	}, nil
}
