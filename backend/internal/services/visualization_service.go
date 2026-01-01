package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GenerateVisualizationForMessage fetches message/query data and generates visualization
// This handles the new per-query visualization endpoint where messageID and queryID are provided
func (s *chatService) GenerateVisualizationForMessage(
	ctx context.Context,
	userID, chatID, messageID, queryID string,
	selectedLLMModel string,
) (*dtos.VisualizationResponse, error) {
	log.Printf("GenerateVisualizationForMessage -> userID: %s, chatID: %s, messageID: %s, queryID: %s, selectedLLMModel: %s", userID, chatID, messageID, queryID, selectedLLMModel)

	// Fetch messages for this chat
	msgResp, _, err := s.ListMessages(userID, chatID, 1, 100)
	if err != nil || msgResp == nil {
		return nil, fmt.Errorf("failed to fetch messages: %v", err)
	}

	// Find the target message
	var targetMessage *dtos.MessageResponse
	for i := range msgResp.Messages {
		if msgResp.Messages[i].ID == messageID {
			targetMessage = &msgResp.Messages[i]
			break
		}
	}

	if targetMessage == nil {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}

	// If no LLM model provided, use the message's LLM model
	if selectedLLMModel == "" && targetMessage.LLMModel != nil && *targetMessage.LLMModel != "" {
		selectedLLMModel = *targetMessage.LLMModel
		log.Printf("GenerateVisualizationForMessage -> Using message's LLM model: %s", selectedLLMModel)
	}

	// Find the query in the message
	var queryData *dtos.Query
	if targetMessage.Queries != nil {
		for i := range *targetMessage.Queries {
			if (*targetMessage.Queries)[i].ID == queryID {
				queryData = &(*targetMessage.Queries)[i]
				break
			}
		}
	}

	if queryData == nil {
		return nil, fmt.Errorf("query not found in message: %s", queryID)
	}

	// Extract query results - try execution result first, then example result
	var queryResults []map[string]interface{}
	var resultData interface{}

	log.Printf("GenerateVisualizationForMessage -> ExecutionResult: %v", queryData.ExecutionResult)
	log.Printf("GenerateVisualizationForMessage -> ExampleResult length: %d", len(queryData.ExampleResult))
	if len(queryData.ExampleResult) > 0 {
		log.Printf("GenerateVisualizationForMessage -> ExampleResult[0]: %v", queryData.ExampleResult[0])
	}

	if len(queryData.ExecutionResult) > 0 {
		resultData = queryData.ExecutionResult
		log.Printf("GenerateVisualizationForMessage -> Using ExecutionResult")
	} else if len(queryData.ExampleResult) > 0 {
		resultData = queryData.ExampleResult
		log.Printf("GenerateVisualizationForMessage -> Using ExampleResult")
	}

	if resultData == nil {
		return nil, fmt.Errorf("no query results found for query: %s", queryID)
	}

	// Convert result data to []map[string]interface{}
	switch v := resultData.(type) {
	case map[string]interface{}:
		// Check if it's wrapped in a 'results' key (common format from DB queries)
		if resultsArray, ok := v["results"].([]interface{}); ok {
			// Extract the nested results array
			for _, item := range resultsArray {
				if m, ok := item.(map[string]interface{}); ok {
					queryResults = append(queryResults, m)
				}
			}
			log.Printf("GenerateVisualizationForMessage -> Extracted from 'results' key, length: %d", len(queryResults))
		} else {
			// Single result object (e.g., DML result with rowsAffected, or aggregation)
			queryResults = []map[string]interface{}{v}
			log.Printf("GenerateVisualizationForMessage -> Single result object, length: %d", len(queryResults))
		}
	case []interface{}:
		// Array of results (most common for SELECT queries)
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				queryResults = append(queryResults, m)
			}
		}
		log.Printf("GenerateVisualizationForMessage -> Array of results, length: %d", len(queryResults))
	default:
		// Primitive value or other format - wrap it
		queryResults = []map[string]interface{}{{
			"value": resultData,
		}}
		log.Printf("GenerateVisualizationForMessage -> Wrapped primitive/unknown format, type: %T", resultData)
	}

	log.Printf("GenerateVisualizationForMessage -> Final queryResults length: %d", len(queryResults))
	if len(queryResults) > 0 {
		log.Printf("GenerateVisualizationForMessage -> First result: %v", queryResults[0])
	}

	// Check if data is suitable for visualization
	if len(queryResults) == 0 {
		return &dtos.VisualizationResponse{
			CanVisualize: false,
			Reason:       "No data rows returned by the query. Visualization requires at least one data row.",
		}, nil
	}

	// **PHASE 2: INTELLIGENT SAMPLING FOR LARGE DATASETS**
	// Use pagination total_records_count to decide strategy
	totalRecords := 0
	if queryData.Pagination != nil {
		totalRecords = queryData.Pagination.TotalRecordsCount
		log.Printf("GenerateVisualizationForMessage -> Total records available: %d", totalRecords)
	}

	// Strategy based on dataset size:
	// - < 1000 rows: Use all data from message (fast, accurate)
	// - 1000-10000 rows: Re-execute with smart LIMIT (reasonable size)
	// - > 10000 rows: Re-execute with aggregation/sampling (prevent OOM)

	const SMALL_DATASET_THRESHOLD = 1000
	const MEDIUM_DATASET_THRESHOLD = 10000
	const VISUALIZATION_SAMPLE_SIZE = 5000

	var finalResults []map[string]interface{}
	useAggregation := false

	if totalRecords > 0 {
		if totalRecords <= SMALL_DATASET_THRESHOLD {
			// Small dataset: Use existing sample (already have representative data)
			finalResults = queryResults
			log.Printf("GenerateVisualizationForMessage -> Using sample data (%d rows) for small dataset", len(queryResults))
		} else if totalRecords <= MEDIUM_DATASET_THRESHOLD {
			// Medium dataset: Try to fetch more data with LIMIT
			log.Printf("GenerateVisualizationForMessage -> Medium dataset detected (%d rows), attempting to fetch more data", totalRecords)

			// For now, use the sample we have (Phase 2.1 implementation)
			// Future: Re-execute with LIMIT VISUALIZATION_SAMPLE_SIZE
			finalResults = queryResults
			// TODO: Implement re-execution with:
			// - Original query wrapped with LIMIT
			// - Smart sampling (every Nth row)
		} else {
			// Large dataset: Use aggregation strategy
			log.Printf("GenerateVisualizationForMessage -> Large dataset detected (%d rows), using aggregation strategy", totalRecords)
			useAggregation = true

			// For now, use sample and mark for aggregation hint
			// AI will recommend aggregation-friendly charts
			finalResults = queryResults

			// TODO: Implement smart aggregation:
			// - Detect date columns → GROUP BY time period
			// - Detect categorical columns → GROUP BY category with SUM/AVG
			// - For numeric-only → Sample every Nth row
		}
	} else {
		// No pagination info: Use sample data
		finalResults = queryResults
		log.Printf("GenerateVisualizationForMessage -> No pagination info, using sample data")
	}

	// Check data suitability for visualization
	dataQuality := analyzeDataQuality(finalResults)
	log.Printf("GenerateVisualizationForMessage -> Data quality: %+v", dataQuality)

	// Pass context to AI for intelligent decision making
	visualizationContext := map[string]interface{}{
		"total_records":   totalRecords,
		"sample_size":     len(finalResults),
		"use_aggregation": useAggregation,
		"query_text":      queryData.Query,
		"has_pagination":  queryData.Pagination != nil,
		"data_quality":    dataQuality,
		"column_count":    dataQuality["column_count"],
		"has_numeric":     dataQuality["has_numeric"],
		"has_categorical": dataQuality["has_categorical"],
		"is_aggregation":  dataQuality["is_aggregation"],
		"is_single_value": dataQuality["is_single_value"],
	}

	// Generate visualization using the results
	// Create a map representation of the query for passing to GenerateVisualizationForQueryResults
	queryMap := map[string]interface{}{
		"query":          queryData.Query,
		"connectionType": "", // Will use chat connection type
	}

	visualization, err := s.GenerateVisualizationForQueryResults(
		ctx,
		userID,
		chatID,
		nil,              // Will be fetched inside GenerateVisualizationForQueryResults
		selectedLLMModel, // Use the selected LLM model from the chat
		"",               // No user question for this endpoint
		[]interface{}{queryMap, visualizationContext}, // Pass query data as map + context
		finalResults, // The results (sample or full)
		true,         // isExplicitRequest: true because this is a user-requested visualization
	)

	if err != nil {
		log.Printf("GenerateVisualizationForMessage -> Error generating visualization: %v", err)
		// Return error but don't fail - visualization is optional
		return &dtos.VisualizationResponse{
			CanVisualize: false,
			Reason:       fmt.Sprintf("Failed to generate visualization: %v", err),
		}, nil
	}

	// Save the visualization to the database linked to this query
	// Save BOTH successful and failed attempts so state persists after page refresh
	if visualization != nil {
		vizID, saveErr := s.SaveVisualizationToMessage(ctx, messageID, chatID, userID, visualization, queryID)
		if saveErr != nil {
			log.Printf("GenerateVisualizationForMessage -> Warning: Failed to save visualization: %v", saveErr)
			// Don't fail the response, visualization is still valid
		} else if vizID != "" {
			log.Printf("GenerateVisualizationForMessage -> Visualization saved with ID: %s", vizID)
			visualization.VisualizationID = vizID

			// Update the Query object to link it to this visualization
			msgObjID, _ := primitive.ObjectIDFromHex(messageID)
			queryObjID, _ := primitive.ObjectIDFromHex(queryID)
			vizObjID, _ := primitive.ObjectIDFromHex(vizID)

			if updateErr := s.chatRepo.UpdateQueryVisualizationID(msgObjID, queryObjID, vizObjID); updateErr != nil {
				log.Printf("GenerateVisualizationForMessage -> Warning: Failed to update query visualization ID: %v", updateErr)
				// Don't fail the response, visualization is still valid
			}

			// Include chart data directly from finalResults (we already have it!)
			// This avoids re-executing the query and preserves context
			if len(finalResults) > 0 {
				visualization.ChartData = finalResults
				visualization.TotalRecords = totalRecords
				visualization.ReturnedCount = len(finalResults)
				visualization.HasMore = totalRecords > len(finalResults)
				log.Printf("GenerateVisualizationForMessage -> Included %d chart data rows in response from execution", len(finalResults))
			}
		}
	}

	return visualization, nil
}

// analyzeDataQuality analyzes the structure and content of query results
// to help AI decide if visualization is suitable and what type to use
func analyzeDataQuality(results []map[string]interface{}) map[string]interface{} {
	if len(results) == 0 {
		return map[string]interface{}{
			"column_count":    0,
			"row_count":       0,
			"has_numeric":     false,
			"has_categorical": false,
			"is_aggregation":  false,
			"is_single_value": false,
		}
	}

	firstRow := results[0]
	columnCount := len(firstRow)
	rowCount := len(results)

	// Analyze column types
	hasNumeric := false
	hasCategorical := false
	numericColumns := []string{}
	categoricalColumns := []string{}

	for key, value := range firstRow {
		switch v := value.(type) {
		case int, int32, int64, float32, float64:
			hasNumeric = true
			numericColumns = append(numericColumns, key)
		case string:
			// Try to parse as number
			if _, err := strconv.ParseFloat(v, 64); err == nil {
				hasNumeric = true
				numericColumns = append(numericColumns, key)
			} else {
				hasCategorical = true
				categoricalColumns = append(categoricalColumns, key)
			}
		default:
			// Other types (bool, date, etc.) are categorical for visualization purposes
			hasCategorical = true
			categoricalColumns = append(categoricalColumns, key)
		}
	}

	// Detect if this is an aggregation result (single row with numeric values)
	isAggregation := rowCount == 1 && hasNumeric

	// Detect if this is a single value result
	isSingleValue := rowCount == 1 && columnCount == 1

	return map[string]interface{}{
		"column_count":        columnCount,
		"row_count":           rowCount,
		"has_numeric":         hasNumeric,
		"has_categorical":     hasCategorical,
		"numeric_columns":     numericColumns,
		"categorical_columns": categoricalColumns,
		"is_aggregation":      isAggregation,
		"is_single_value":     isSingleValue,
	}
}

// GenerateVisualizationForQueryResults generates chart configuration for executed query results
// This is called after AI response to auto-generate charts if AutoGenerateVisualization is enabled
func (s *chatService) GenerateVisualizationForQueryResults(
	ctx context.Context,
	userID, chatID string,
	chat *models.Chat,
	selectedLLMModel string,
	userQuestion string,
	executedQueries []interface{}, // Changed from []models.Query to avoid circular dependency
	queryResults []map[string]interface{},
	isExplicitRequest bool, // If true, generate visualization even if AutoGenerateVisualization is disabled
) (*dtos.VisualizationResponse, error) {
	log.Printf("GenerateVisualizationForQueryResults -> userID: %s, chatID: %s, isExplicitRequest: %v", userID, chatID, isExplicitRequest)

	// Fetch chat if not provided
	if chat == nil {
		chatObjID, err := primitive.ObjectIDFromHex(chatID)
		if err != nil {
			return nil, fmt.Errorf("invalid chat ID: %v", err)
		}
		var errFetch error
		chat, errFetch = s.chatRepo.FindByID(chatObjID)
		if errFetch != nil {
			return nil, fmt.Errorf("failed to fetch chat: %v", errFetch)
		}
	}

	// Only generate visualization if enabled (unless this is an explicit user request)
	if !isExplicitRequest && !chat.Settings.AutoGenerateVisualization {
		log.Printf("GenerateVisualizationForQueryResults -> AutoGenerateVisualization is disabled and this is not an explicit request, skipping visualization")
		return nil, nil
	}

	// Need at least some results
	if len(queryResults) == 0 {
		log.Printf("GenerateVisualizationForQueryResults -> No query results provided, cannot generate visualization")
		return nil, nil
	}

	// Determine LLM model if not provided
	if selectedLLMModel == "" {
		if chat.PreferredLLMModel != nil && *chat.PreferredLLMModel != "" {
			selectedLLMModel = *chat.PreferredLLMModel
		} else {
			// Get first enabled model
			models := constants.GetEnabledLLMModels()
			if len(models) > 0 {
				selectedLLMModel = models[0].ID
			} else {
				log.Printf("GenerateVisualizationForQueryResults -> No LLM models available")
				return nil, nil
			}
		}
	}

	// Use the first query's string representation if available
	var executedQueryStr string
	if len(executedQueries) > 0 {
		log.Printf("GenerateVisualizationForQueryResults -> executedQueries[0] type: %T", executedQueries[0])
		if queryMap, ok := executedQueries[0].(map[string]interface{}); ok {
			if q, ok := queryMap["query"].(string); ok {
				executedQueryStr = q
				log.Printf("GenerateVisualizationForQueryResults -> Extracted query from map")
			} else {
				log.Printf("GenerateVisualizationForQueryResults -> query field not found or not string in queryMap")
			}
		} else {
			log.Printf("GenerateVisualizationForQueryResults -> executedQueries[0] is not a map[string]interface{}")
		}
	} else {
		log.Printf("GenerateVisualizationForQueryResults -> No executedQueries provided")
	}

	if executedQueryStr == "" {
		executedQueryStr = "SELECT * FROM results" // Fallback
		log.Printf("GenerateVisualizationForQueryResults -> Using fallback query string")
	}

	// Build request for AI
	log.Printf("GenerateVisualizationForQueryResults -> Building visualization request with %d results", len(queryResults))
	req := s.buildVisualizationRequest(
		userQuestion,
		executedQueryStr,
		queryResults,
		len(queryResults), // Use actual result count
		chat.Connection.Type,
	)
	log.Printf("GenerateVisualizationForQueryResults -> Request built, connection type: %s", chat.Connection.Type)

	// Get visualization prompt
	visualizationPrompt := constants.GetVisualizationPrompt(chat.Connection.Type)
	log.Printf("GenerateVisualizationForQueryResults -> Visualization prompt retrieved for type: %s", chat.Connection.Type)

	// Call LLM to generate visualization configuration
	log.Printf("GenerateVisualizationForQueryResults -> Calling LLM with model: %s", selectedLLMModel)
	visualizationResponse, err := s.callVisualizationLLM(
		ctx,
		selectedLLMModel,
		visualizationPrompt,
		req,
	)
	log.Printf("GenerateVisualizationForQueryResults -> LLM call completed, err: %v", err)

	if err != nil {
		log.Printf("GenerateVisualizationForQueryResults -> Error calling LLM: %v", err)
		// Return error response instead of nil so frontend knows what happened
		return &dtos.VisualizationResponse{
			CanVisualize: false,
			Reason:       fmt.Sprintf("Failed to analyze data for visualization: %v", err),
		}, nil // Return nil error because visualization failure shouldn't crash the whole flow
	}

	// Add the actual query results data to the visualization response
	// This is needed by the frontend to render the chart
	if visualizationResponse != nil && visualizationResponse.CanVisualize {
		// Limit data to first 100 rows for frontend rendering to avoid huge payloads
		maxRows := 100
		if len(queryResults) > maxRows {
			visualizationResponse.ChartData = queryResults[:maxRows]
		} else {
			visualizationResponse.ChartData = queryResults
		}
	}

	return visualizationResponse, nil
}

// visualizationRequestData is an internal struct for visualization request data
type visualizationRequestData struct {
	UserQuery         string
	ExecutedQuery     string
	QueryResultSample []map[string]interface{}
	TotalRowCount     int
	ConnectionType    string
	ColumnNames       []string
	ColumnTypes       map[string]string
	ColumnCount       int
}

// buildVisualizationRequest constructs the request for the visualization LLM prompt
func (s *chatService) buildVisualizationRequest(
	userQuery string,
	executedQuery string,
	queryResults []map[string]interface{},
	totalRowCount int,
	connectionType string,
) *visualizationRequestData {
	// Extract column names and types from first result row
	columnNames := []string{}
	columnTypes := make(map[string]string)

	if len(queryResults) > 0 {
		firstRow := queryResults[0]
		for key, value := range firstRow {
			columnNames = append(columnNames, key)
			columnTypes[key] = inferDataType(value)
		}
	}

	// Limit sample to first 50 rows
	sampleSize := 50
	if len(queryResults) < sampleSize {
		sampleSize = len(queryResults)
	}
	querySample := queryResults[:sampleSize]

	return &visualizationRequestData{
		UserQuery:         userQuery,
		ExecutedQuery:     executedQuery,
		QueryResultSample: querySample,
		TotalRowCount:     totalRowCount,
		ConnectionType:    connectionType,
		ColumnNames:       columnNames,
		ColumnTypes:       columnTypes,
		ColumnCount:       len(columnNames),
	}
}

// inferDataType infers the data type from a value
func inferDataType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch v := value.(type) {
	case float64:
		// Check if it's actually an integer
		if v == float64(int64(v)) {
			return "integer"
		}
		return "numeric"
	case string:
		// Try to detect if it's a date/time
		if isDateLike(v) {
			return "date"
		}
		return "string"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}

// isDateLike checks if a string looks like a date/timestamp
func isDateLike(s string) bool {
	// Common date patterns - check if string contains typical date characters
	if len(s) >= 10 && (s[0] >= '1' && s[0] <= '2') {
		// Year typically starts with 1 or 2
		if len(s) >= 4 && s[4] == '-' {
			// Looks like YYYY-MM format
			return true
		}
		if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
			// Looks like YYYY-MM-DD format
			return true
		}
	}
	// Check for ISO format with T
	for i := 0; i < len(s); i++ {
		if s[i] == 'T' && i > 8 && i < len(s)-1 {
			return true // ISO 8601 format
		}
	}
	return false
}

// callVisualizationLLM calls the LLM with visualization prompt and request
func (s *chatService) callVisualizationLLM(
	ctx context.Context,
	selectedLLMModel string,
	visualizationPrompt string,
	request *visualizationRequestData,
) (*dtos.VisualizationResponse, error) {
	log.Printf("callVisualizationLLM -> selectedLLMModel: %s", selectedLLMModel)

	// Get the correct LLM client based on the selected model's provider
	llmClient := s.llmClient
	if s.llmManager != nil && selectedLLMModel != "" {
		selectedModel := constants.GetLLMModel(selectedLLMModel)
		if selectedModel != nil {
			providerClient, err := s.llmManager.GetClient(selectedModel.Provider)
			if err != nil {
				log.Printf("Warning: Failed to get LLM client for provider '%s': %v, will use default client", selectedModel.Provider, err)
			} else {
				llmClient = providerClient
				log.Printf("callVisualizationLLM -> Using LLM client for provider: %s", selectedModel.Provider)
			}
		}
	}

	// Prepare the message for the LLM
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal visualization request: %v", err)
	}

	// Build message array for LLM using LLMMessage format
	// For visualization, we need to bypass the normal system prompt and use a visualization-specific instruction
	// This is critical - the visualization response format must be strictly JSON with specific fields
	visualizationSystemPrompt := `You are a data visualization expert. Your ONLY task is to analyze query results and recommend the best chart type.

CRITICAL: You MUST respond with ONLY a JSON object (no markdown, no explanation outside JSON).

Response MUST have this exact structure:
{
  "can_visualize": boolean,
  "reason": "string explaining why or why not",
  "chart_configuration": {
    "chart_type": "string",
    "title": "string", 
    "description": "string",
    "data_fetch": {},
    "chart_render": {}
  }
}

REQUIRED RULES:
1. Always include "can_visualize" and "reason" fields
2. If can_visualize=true, chart_configuration must have valid values
3. If can_visualize=false, chart_configuration can be null
4. Use ONLY valid JSON - no markdown code blocks, no text before/after JSON
5. Valid chart types: line, bar, pie, area, scatter, heatmap, funnel, bubble, waterfall
`

	// Call the LLM client with the new GenerateVisualization method
	llmResponse, err := llmClient.GenerateVisualization(
		ctx,
		visualizationSystemPrompt,
		visualizationPrompt,
		string(requestJSON),
		selectedLLMModel,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM response: %v", err)
	}

	// Log the raw LLM response for debugging
	log.Printf("callVisualizationLLM -> Raw LLM Response: %s", llmResponse)

	// Parse the LLM response
	var vizResponse dtos.VisualizationResponse
	if err := json.Unmarshal([]byte(llmResponse), &vizResponse); err != nil {
		log.Printf("callVisualizationLLM -> Failed to parse LLM response: %v, response: %s", err, llmResponse)
		return nil, fmt.Errorf("failed to parse visualization response: %v", err)
	}

	log.Printf("callVisualizationLLM -> Successfully parsed response, can_visualize: %v", vizResponse.CanVisualize)

	return &vizResponse, nil
}

// SaveVisualizationToMessage saves the visualization configuration to the database as a MessageVisualization document
// If queryID is provided (non-empty), the visualization will be linked to that specific query (1:1 query-visualization relationship)
// If queryID is empty, the visualization will be linked to the message only (for backward compatibility)
// Returns the saved visualization's ID or empty string if save failed
func (s *chatService) SaveVisualizationToMessage(
	ctx context.Context,
	messageID, chatID, userID string,
	visualization *dtos.VisualizationResponse,
	queryID string, // Optional: ID of the query to link visualization to
) (string, error) {
	if visualization == nil {
		return "", nil
	}

	// Convert message ID to ObjectID (can be empty for query-level visualizations)
	var msgObjID *primitive.ObjectID
	if messageID != "" {
		msgID, err := primitive.ObjectIDFromHex(messageID)
		if err != nil {
			return "", fmt.Errorf("invalid message ID: %v", err)
		}
		msgObjID = &msgID
	}

	// Convert query ID to ObjectID if provided
	var queryObjID *primitive.ObjectID
	if queryID != "" {
		qID, err := primitive.ObjectIDFromHex(queryID)
		if err != nil {
			return "", fmt.Errorf("invalid query ID: %v", err)
		}
		queryObjID = &qID
	}

	// Convert chat ID to ObjectID
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return "", fmt.Errorf("invalid chat ID: %v", err)
	}

	// Convert user ID to ObjectID
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return "", fmt.Errorf("invalid user ID: %v", err)
	}

	// Create MessageVisualization document with optional query linking
	msgViz := models.NewMessageVisualization(msgObjID, chatObjID, userObjID, queryObjID)
	msgViz.CanVisualize = visualization.CanVisualize
	msgViz.Reason = visualization.Reason

	if visualization.ChartConfiguration != nil {
		msgViz.ChartType = visualization.ChartConfiguration.ChartType
		msgViz.Title = visualization.ChartConfiguration.Title
		msgViz.Description = visualization.ChartConfiguration.Description
		msgViz.QueryStrategy = visualization.ChartConfiguration.DataFetch.QueryStrategy
		msgViz.OptimizedQuery = visualization.ChartConfiguration.DataFetch.OptimizedQuery
		msgViz.DataTransformation = visualization.ChartConfiguration.DataFetch.Transformation
		msgViz.ProjectedRowCount = visualization.ChartConfiguration.DataFetch.ProjectedRows
		msgViz.ChartHeight = visualization.ChartConfiguration.RenderingHints.ChartHeight
		msgViz.ColorScheme = visualization.ChartConfiguration.RenderingHints.ColorScheme
		msgViz.DataDensity = visualization.ChartConfiguration.RenderingHints.DataDensity

		if visualization.ChartConfiguration.ChartRender.XAxis.Label != "" {
			msgViz.XAxisLabel = visualization.ChartConfiguration.ChartRender.XAxis.Label
		}
		if visualization.ChartConfiguration.ChartRender.YAxis != nil && visualization.ChartConfiguration.ChartRender.YAxis.Label != "" {
			msgViz.YAxisLabel = visualization.ChartConfiguration.ChartRender.YAxis.Label
		}

		// Store full configuration as JSON for frontend
		vizJSON, err := json.Marshal(visualization.ChartConfiguration)
		if err != nil {
			log.Printf("SaveVisualizationToMessage -> Error marshaling chart config: %v", err)
		} else {
			msgViz.ChartConfigJSON = string(vizJSON)
		}
	}

	msgViz.Error = visualization.Error

	// Save to database
	err = s.visualizationRepo.CreateVisualization(ctx, msgViz)
	if err != nil {
		return "", err
	}

	// Return the ID of the saved visualization
	return msgViz.ID.Hex(), nil
}

// GetVisualizationForMessage retrieves saved visualization for a message
func (s *chatService) GetVisualizationForMessage(
	ctx context.Context,
	messageID string,
) (*dtos.VisualizationResponse, error) {
	// Convert message ID to ObjectID
	msgObjID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, fmt.Errorf("invalid message ID: %v", err)
	}

	// Fetch visualization from database
	msgViz, err := s.visualizationRepo.GetVisualizationByMessageID(ctx, msgObjID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch visualization: %v", err)
	}

	if msgViz == nil {
		return nil, nil
	}

	// Convert to VisualizationResponse DTO
	response := &dtos.VisualizationResponse{
		CanVisualize: msgViz.CanVisualize,
		Reason:       msgViz.Reason,
		Error:        msgViz.Error,
	}

	// Parse chart configuration if available
	if msgViz.ChartConfigJSON != "" {
		var chartConfig dtos.ChartConfiguration
		if err := json.Unmarshal([]byte(msgViz.ChartConfigJSON), &chartConfig); err != nil {
			log.Printf("GetVisualizationForMessage -> Error parsing chart config: %v", err)
		} else {
			response.ChartConfiguration = &chartConfig
		}
	}

	return response, nil
}

// GetVisualizationForQuery retrieves saved visualization for a specific query
// Enables per-query visualization retrieval for the 1:1 query-visualization relationship
func (s *chatService) GetVisualizationForQuery(
	ctx context.Context,
	queryID string,
) (*dtos.VisualizationResponse, error) {
	// Convert query ID to ObjectID
	queryObjID, err := primitive.ObjectIDFromHex(queryID)
	if err != nil {
		return nil, fmt.Errorf("invalid query ID: %v", err)
	}

	// Fetch visualization from database
	msgViz, err := s.visualizationRepo.GetVisualizationByQueryID(ctx, queryObjID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch visualization: %v", err)
	}

	if msgViz == nil {
		return nil, nil
	}

	// Convert to VisualizationResponse DTO
	response := &dtos.VisualizationResponse{
		CanVisualize: msgViz.CanVisualize,
		Reason:       msgViz.Reason,
		Error:        msgViz.Error,
	}

	// Parse chart configuration if available
	if msgViz.ChartConfigJSON != "" {
		var chartConfig dtos.ChartConfiguration
		if err := json.Unmarshal([]byte(msgViz.ChartConfigJSON), &chartConfig); err != nil {
			log.Printf("GetVisualizationForQuery -> Error parsing chart config: %v", err)
		} else {
			response.ChartConfiguration = &chartConfig
		}
	}

	return response, nil
}

// ExecuteChartQuery executes a query for chart data rendering
// Uses the query configuration from chart_configuration to fetch optimized data
func (s *chatService) ExecuteChartQuery(
	ctx context.Context,
	userID, chatID string,
	chartConfig *dtos.ChartConfiguration,
	limit int,
) ([]map[string]interface{}, error) {
	log.Printf("ExecuteChartQuery -> userID: %s, chatID: %s, limit: %d", userID, chatID, limit)

	if chartConfig == nil {
		return nil, fmt.Errorf("chart configuration is required")
	}

	// Get chat for database connection
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %v", err)
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chat: %v", err)
	}

	// Verify chat belongs to user
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %v", err)
	}

	if chat.UserID != userObjID {
		return nil, fmt.Errorf("unauthorized access to chat")
	}

	// Determine which query to execute
	queryToExecute := chartConfig.DataFetch.OptimizedQuery
	if queryToExecute == "" {
		// If no optimized query provided, skip execution
		log.Printf("ExecuteChartQuery -> No optimized query in DataFetch, cannot execute")
		return nil, fmt.Errorf("no query specified for chart execution")
	}

	// Add limit to the query
	if limit > 0 {
		queryToExecute = fmt.Sprintf("%s LIMIT %d", queryToExecute, limit)
	}

	log.Printf("ExecuteChartQuery -> Executing query: %s", queryToExecute)

	// Execute the query using dbManager
	result, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, "", "", "", queryToExecute, "SELECT", false, false)
	if queryErr != nil {
		log.Printf("ExecuteChartQuery -> Error executing query: %v", queryErr.Message)
		return nil, fmt.Errorf("failed to execute chart query: %s", queryErr.Message)
	}

	// Convert result to map format
	var data []map[string]interface{}
	if result != nil && result.Result != nil {
		switch v := result.Result.(type) {
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					data = append(data, m)
				}
			}
		case []map[string]interface{}:
			data = v
		}
	}

	log.Printf("ExecuteChartQuery -> Retrieved %d rows for chart", len(data))
	return data, nil
}

// GetVisualizationData fetches chart data for a specific query on-demand
// This implements the lazy-loading pattern: fetch data only when user wants to view the visualization
// Returns paginated data based on the saved visualization configuration
func (s *chatService) GetVisualizationData(
	ctx context.Context,
	userID, chatID, messageID, queryID string,
	limit, offset int,
) (interface{}, error) {
	log.Printf("GetVisualizationData -> userID: %s, chatID: %s, messageID: %s, queryID: %s, limit: %d, offset: %d",
		userID, chatID, messageID, queryID, limit, offset)

	// Convert IDs to ObjectID
	msgObjID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, fmt.Errorf("invalid message ID: %v", err)
	}

	queryObjID, err := primitive.ObjectIDFromHex(queryID)
	if err != nil {
		return nil, fmt.Errorf("invalid query ID: %v", err)
	}

	// Fetch the specific message directly by ID
	message, err := s.chatRepo.FindMessageByID(msgObjID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message: %v", err)
	}

	if message == nil {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}

	// Find the query in the message
	var targetQuery *models.Query
	if message.Queries != nil {
		for i := range *message.Queries {
			if (*message.Queries)[i].ID == queryObjID {
				targetQuery = &(*message.Queries)[i]
				break
			}
		}
	}

	if targetQuery == nil {
		return nil, fmt.Errorf("query not found: messageID=%s, queryID=%s", messageID, queryID)
	}

	// Fetch the visualization metadata for this query
	visualization, err := s.GetVisualizationForQuery(ctx, queryID)
	if err != nil {
		log.Printf("GetVisualizationData -> Warning: Failed to fetch visualization: %v", err)
		// Continue anyway - we can still execute the query without visualization metadata
	}

	// Execute the query to get the data
	// If we have visualization configuration with an optimized query, use that
	var queryToExecute string

	if visualization != nil && visualization.ChartConfiguration != nil {
		// Use optimized query from visualization configuration if available
		if visualization.ChartConfiguration.DataFetch.OptimizedQuery != "" {
			queryToExecute = visualization.ChartConfiguration.DataFetch.OptimizedQuery
			log.Printf("GetVisualizationData -> Using optimized query from visualization config")
		} else {
			queryToExecute = targetQuery.Query
		}
	} else {
		queryToExecute = targetQuery.Query
	}

	// Execute the query on the user's database
	result, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, "", "", "", queryToExecute, "SELECT", false, false)
	if queryErr != nil {
		log.Printf("GetVisualizationData -> Error executing query: %v", queryErr.Message)
		return nil, fmt.Errorf("failed to execute chart query: %s", queryErr.Message)
	}

	log.Printf("GetVisualizationData -> result: %+v", result)
	log.Printf("GetVisualizationData -> result.Result type: %T", result.Result)

	// Convert result to map format
	var fullData []map[string]interface{}
	if result != nil && result.Result != nil {
		log.Printf("GetVisualizationData -> Result is not nil, processing data...")
		switch v := result.Result.(type) {
		case []interface{}:
			log.Printf("GetVisualizationData -> Result is []interface{}, length: %d", len(v))
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					fullData = append(fullData, m)
				}
			}
		case []map[string]interface{}:
			log.Printf("GetVisualizationData -> Result is []map[string]interface{}, length: %d", len(v))
			fullData = v
		case map[string]interface{}:
			log.Printf("GetVisualizationData -> Result is map[string]interface{}, checking for 'results' key")
			// Handle wrapped results with "results" key
			if resultsInterface, ok := v["results"]; ok {
				log.Printf("GetVisualizationData -> Found 'results' key, type: %T", resultsInterface)
				// Handle both []interface{} and []map[string]interface{} from results key
				switch resultsVal := resultsInterface.(type) {
				case []interface{}:
					log.Printf("GetVisualizationData -> 'results' is []interface{}, length: %d", len(resultsVal))
					for _, item := range resultsVal {
						if m, ok := item.(map[string]interface{}); ok {
							fullData = append(fullData, m)
						}
					}
				case []map[string]interface{}:
					log.Printf("GetVisualizationData -> 'results' is []map[string]interface{}, length: %d", len(resultsVal))
					fullData = resultsVal
				}
			}
		default:
			log.Printf("GetVisualizationData -> Unexpected result type: %T, value: %+v", v, v)
		}
	} else {
		log.Printf("GetVisualizationData -> Result is nil or Result.Result is nil")
		if result == nil {
			log.Printf("GetVisualizationData -> result object is nil")
		} else {
			log.Printf("GetVisualizationData -> result.Result is nil")
		}
	}

	log.Printf("GetVisualizationData -> Retrieved %d total rows", len(fullData))

	// Apply pagination
	totalRowCount := len(fullData)
	var paginatedData []map[string]interface{}

	if offset >= totalRowCount {
		// Offset is beyond the data
		paginatedData = []map[string]interface{}{}
	} else {
		endIdx := offset + limit
		if endIdx > totalRowCount {
			endIdx = totalRowCount
		}
		paginatedData = fullData[offset:endIdx]
	}

	log.Printf("GetVisualizationData -> Returning %d rows (offset: %d, limit: %d, total: %d)", len(paginatedData), offset, limit, totalRowCount)

	// Return response with metadata and data
	response := gin.H{
		"can_visualize":  true,
		"chart_data":     paginatedData,
		"total_records":  totalRowCount,
		"returned_count": len(paginatedData),
		"offset":         offset,
		"limit":          limit,
		"has_more":       (offset + limit) < totalRowCount,
	}

	// Include visualization configuration if available
	if visualization != nil {
		response["can_visualize"] = visualization.CanVisualize
		response["chart_configuration"] = visualization.ChartConfiguration
		if visualization.Reason != "" {
			response["reason"] = visualization.Reason
		}
		if visualization.Error != "" {
			response["error"] = visualization.Error
		}
	}

	return response, nil
}

// Helper functions for string manipulation
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func indexOf(s, substr string) int {
	return strings.Index(s, substr)
}
