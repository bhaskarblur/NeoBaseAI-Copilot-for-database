package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/pkg/dbmanager"
	"neobase-ai/pkg/llm"
)

// BuildToolExecutor creates a ToolExecutorFunc closure that has access to the
// dbManager for executing queries and fetching schema. This is the bridge
// between the LLM's tool calls and the actual database.
func BuildToolExecutor(
	dbMgr *dbmanager.Manager,
	chatID string,
	dbType string,
) llm.ToolExecutorFunc {
	return func(ctx context.Context, call llm.ToolCall) (*llm.ToolResult, error) {
		switch call.Name {
		case llm.ExecuteQueryToolName:
			return executeReadQuery(ctx, dbMgr, chatID, dbType, call)
		case llm.GetTableInfoToolName:
			return getTableInfo(ctx, dbMgr, chatID, dbType, call)
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

// executeReadQuery handles the execute_read_query tool call.
func executeReadQuery(
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

	// Validate read-only using centralized per-DB classification
	if !constants.IsReadOnlyQuery(query, dbType) {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: "Error: Only read-only queries (SELECT, SHOW, DESCRIBE, EXPLAIN, or MongoDB find/aggregate) are allowed. Write operations must be included in the final response for user approval.",
			IsError: true,
		}, nil
	}

	log.Printf("ToolExecutor -> execute_read_query -> chatID=%s explanation=%q query=%q", chatID, explanation, query)

	// Execute the query using dbManager.
	// Pass empty strings for messageID, queryID, and streamID since this is a tool-call
	// exploration query, not a user-initiated execution.
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

	// Marshal the result to JSON
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

	// Include execution time metadata
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

// getTableInfo handles the get_table_info tool call.
func getTableInfo(
	ctx context.Context,
	dbMgr *dbmanager.Manager,
	chatID string,
	dbType string,
	call llm.ToolCall,
) (*llm.ToolResult, error) {
	// Parse table_names from arguments
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

	log.Printf("ToolExecutor -> get_table_info -> chatID=%s tables=%v", chatID, tableNames)

	// Get the database connection
	dbConn, err := dbMgr.GetConnection(chatID)
	if err != nil {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: fmt.Sprintf("Error getting database connection: %v", err),
			IsError: true,
		}, nil
	}

	// Fetch schema for the specific tables using SchemaManager
	schema, err := dbMgr.GetSchemaManager().GetSchema(ctx, chatID, dbConn, dbType, tableNames)
	if err != nil {
		return &llm.ToolResult{
			CallID:  call.ID,
			Name:    call.Name,
			Content: fmt.Sprintf("Error fetching schema for tables %v: %v", tableNames, err),
			IsError: true,
		}, nil
	}

	// Format the schema for LLM readability
	content := dbMgr.GetSchemaManager().FormatSchemaForLLM(schema)

	// Detect table name mismatches (e.g. spreadsheet 'sheet_data' → actual 'ventures').
	// When the schema driver maps raw table names to user-facing names, the returned
	// schema may contain different table names than what the LLM requested.
	// Prepend a clear note so the LLM uses the correct names in its SQL.
	requested := make(map[string]bool, len(tableNames))
	for _, t := range tableNames {
		requested[t] = true
	}
	var mismatches []string
	for actualName := range schema.Tables {
		if !requested[actualName] {
			mismatches = append(mismatches, actualName)
		}
	}
	if len(mismatches) > 0 {
		var note string
		for _, actual := range mismatches {
			note += fmt.Sprintf("⚠️ IMPORTANT: The table you requested was mapped to '%s' in the database. "+
				"You MUST use '%s' as the table name in all your SQL queries, NOT the name from information_schema.\n", actual, actual)
		}
		content = note + content
		log.Printf("ToolExecutor -> get_table_info -> Table name mismatch detected: requested=%v, actual schema tables=%v", tableNames, mismatches)
	}

	return &llm.ToolResult{
		CallID:  call.ID,
		Name:    call.Name,
		Content: llm.TruncateToolResult(content),
		IsError: false,
	}, nil
}
