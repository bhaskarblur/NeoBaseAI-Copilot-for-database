package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/pkg/dbmanager"
	"strings"
	"time"
)

// ProcessSpreadsheetWithRobustAnalyzer processes any spreadsheet data using the robust analyzer
// This ensures consistent handling between Google Sheets, CSV, and Excel uploads
func (s *chatService) ProcessSpreadsheetWithRobustAnalyzer(
	userID string,
	chatID string,
	baseTableName string,
	data [][]interface{},
) (*dtos.SpreadsheetUploadResponse, error) {
	
	log.Printf("ProcessSpreadsheetWithRobustAnalyzer -> Starting for chat %s, base table %s", 
		chatID, baseTableName)
	
	// Get the connection info for this chat
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return nil, fmt.Errorf("no connection found for chat %s", chatID)
	}
	
	if connInfo.Config.Type != "spreadsheet" && connInfo.Config.Type != "google_sheets" {
		return nil, fmt.Errorf("connection type %s does not support spreadsheet operations", 
			connInfo.Config.Type)
	}
	
	// Get the database connection
	conn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %v", err)
	}
	
	// Get the SQL DB connection
	sqlDB := conn.GetDB()
	if sqlDB == nil {
		return nil, fmt.Errorf("failed to get SQL DB")
	}
	
	// Get schema name
	schemaName := fmt.Sprintf("conn_%s", chatID)
	
	// Use robust analyzer
	robustAnalyzer := dbmanager.NewRobustSheetAnalyzer(data)
	regions, err := robustAnalyzer.AnalyzeRobust()
	if err != nil {
		log.Printf("Warning: Robust analysis failed: %v, creating unstructured table", err)
		
		// Create fallback unstructured region
		region := s.createUnstructuredRegion(data)
		regions = []*dbmanager.DataRegion{region}
	}
	
	if len(regions) == 0 {
		log.Printf("No regions detected, creating unstructured table")
		region := s.createUnstructuredRegion(data)
		regions = []*dbmanager.DataRegion{region}
	}
	
	// Process all detected regions
	allTables := make([]string, 0)
	totalRows := 0
	totalColumns := 0
	overallQuality := 0.0
	allIssues := make([]string, 0)
	allSuggestions := make([]string, 0)
	
	for idx, region := range regions {
		// Determine table name
		tableName := baseTableName
		if len(regions) > 1 {
			tableName = fmt.Sprintf("%s_%d", baseTableName, idx+1)
		}
		
		log.Printf("Processing region %d/%d as table '%s'", idx+1, len(regions), tableName)
		log.Printf("  - Headers: %v", region.Headers)
		log.Printf("  - Rows: %d", len(region.DataRows))
		log.Printf("  - Quality: %.1f%%", region.Quality)
		
		// Store the region data
		if err := s.storeRegionData(sqlDB, schemaName, tableName, region); err != nil {
			log.Printf("Warning: Failed to store region %d: %v", idx+1, err)
			continue
		}
		
		allTables = append(allTables, tableName)
		totalRows += len(region.DataRows)
		if len(region.Headers) > totalColumns {
			totalColumns = len(region.Headers)
		}
		overallQuality += region.Quality
		
		// Collect issues and suggestions
		for _, issue := range region.Issues {
			allIssues = append(allIssues, fmt.Sprintf("Table %s: %s", tableName, issue))
		}
		for _, suggestion := range region.Suggestions {
			allSuggestions = append(allSuggestions, fmt.Sprintf("Table %s: %s", tableName, suggestion))
		}
	}
	
	if len(allTables) == 0 {
		return nil, fmt.Errorf("failed to create any tables from spreadsheet data")
	}
	
	// Calculate average quality
	if len(regions) > 0 {
		overallQuality = overallQuality / float64(len(regions))
	}
	
	// Refresh schema
	ctx := context.Background()
	if _, err := s.RefreshSchema(ctx, userID, chatID, false); err != nil {
		log.Printf("Warning: Failed to refresh schema: %v", err)
	}
	
	// Update database name
	if err := s.updateSpreadsheetDatabaseName(chatID); err != nil {
		log.Printf("Warning: Failed to update database name: %v", err)
	}
	
	// Prepare response
	response := &dtos.SpreadsheetUploadResponse{
		TableName:   strings.Join(allTables, ", "),
		RowCount:    totalRows,
		ColumnCount: totalColumns,
		UploadedAt:  time.Now(),
	}
	
	// Log metadata if available
	if len(allIssues) > 0 || len(allSuggestions) > 0 || overallQuality < 100 {
		metadata := map[string]interface{}{
			"quality":     overallQuality,
			"tables":      allTables,
			"issues":      allIssues,
			"suggestions": allSuggestions,
			"message":     fmt.Sprintf("Successfully created %d table(s) from spreadsheet (Quality: %.1f%%)", len(allTables), overallQuality),
		}
		log.Printf("Spreadsheet processing metadata: %+v", metadata)
	} else {
		log.Printf("Successfully created %d table(s) from spreadsheet", len(allTables))
	}
	
	log.Printf("ProcessSpreadsheetWithRobustAnalyzer -> Completed successfully")
	return response, nil
}

// createUnstructuredRegion creates a fallback region for completely unstructured data
func (s *chatService) createUnstructuredRegion(data [][]interface{}) *dbmanager.DataRegion {
	headers := []string{"row_num", "col_letter", "value"}
	dataRows := make([][]interface{}, 0)
	
	for rowIdx, row := range data {
		for colIdx, cell := range row {
			if cell != nil && fmt.Sprintf("%v", cell) != "" {
				colLetter := columnIndexToLetter(colIdx)
				dataRows = append(dataRows, []interface{}{
					rowIdx + 1,
					colLetter,
					cell,
				})
			}
		}
	}
	
	// Ensure at least one row
	if len(dataRows) == 0 {
		dataRows = append(dataRows, []interface{}{1, "A", "No data found"})
	}
	
	return &dbmanager.DataRegion{
		Headers:     headers,
		DataRows:    dataRows,
		Quality:     50.0,
		Issues:      []string{"Data is unstructured - stored as cell references"},
		Suggestions: []string{"Consider restructuring data into tabular format"},
	}
}

// storeRegionData stores a data region in the database
func (s *chatService) storeRegionData(
	db *sql.DB,
	schemaName string,
	tableName string,
	region *dbmanager.DataRegion,
) error {
	// Sanitize table and column names
	tableName = sanitizeTableName(tableName)
	
	// Drop existing table
	dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s CASCADE", schemaName, tableName)
	if _, err := db.Exec(dropQuery); err != nil {
		return fmt.Errorf("failed to drop existing table: %w", err)
	}
	
	// Create column definitions
	columns := make([]string, 0)
	for _, header := range region.Headers {
		colName := sanitizeColumnName(header)
		columns = append(columns, fmt.Sprintf("%s TEXT", colName))
	}
	
	// Add system columns
	columns = append(columns, "_row_id SERIAL PRIMARY KEY")
	columns = append(columns, "_imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
	
	// Create table
	createQuery := fmt.Sprintf("CREATE TABLE %s.%s (%s)", 
		schemaName, tableName, strings.Join(columns, ", "))
	
	if _, err := db.Exec(createQuery); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	
	// Insert data
	if len(region.DataRows) > 0 {
		// Prepare column names
		colNames := make([]string, len(region.Headers))
		for i, header := range region.Headers {
			colNames[i] = sanitizeColumnName(header)
		}
		
		// Batch insert for better performance
		batchSize := 100
		for i := 0; i < len(region.DataRows); i += batchSize {
			end := i + batchSize
			if end > len(region.DataRows) {
				end = len(region.DataRows)
			}
			
			if err := s.insertBatch(db, schemaName, tableName, colNames, 
				region.DataRows[i:end]); err != nil {
				log.Printf("Warning: Failed to insert batch %d-%d: %v", i, end, err)
			}
		}
	}
	
	return nil
}

// insertBatch inserts a batch of rows efficiently
func (s *chatService) insertBatch(
	db *sql.DB,
	schemaName string,
	tableName string,
	columns []string,
	rows [][]interface{},
) error {
	if len(rows) == 0 {
		return nil
	}
	
	// Build values for batch insert
	valueStrings := make([]string, 0, len(rows))
	
	for _, row := range rows {
		values := make([]string, 0, len(columns))
		
		for i := range columns {
			var value string
			if i < len(row) && row[i] != nil {
				valueStr := fmt.Sprintf("%v", row[i])
				// Escape single quotes
				value = strings.ReplaceAll(valueStr, "'", "''")
			}
			values = append(values, fmt.Sprintf("'%s'", value))
		}
		
		valueStrings = append(valueStrings, fmt.Sprintf("(%s)", strings.Join(values, ", ")))
	}
	
	insertQuery := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES %s",
		schemaName, tableName,
		strings.Join(columns, ", "),
		strings.Join(valueStrings, ", "))
	
	_, err := db.Exec(insertQuery)
	return err
}

// Helper function to sanitize table names
func sanitizeTableName(name string) string {
	// Convert to lowercase and replace special characters
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, " ", "_")
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")
	result = strings.ReplaceAll(result, "(", "")
	result = strings.ReplaceAll(result, ")", "")
	
	// Remove consecutive underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	
	// Trim underscores
	result = strings.Trim(result, "_")
	
	// Ensure it starts with a letter
	if len(result) > 0 && (result[0] < 'a' || result[0] > 'z') {
		result = "t_" + result
	}
	
	return result
}

// Helper function to convert column index to Excel-style letter
func columnIndexToLetter(index int) string {
	letter := ""
	for index >= 0 {
		letter = string(rune('A'+index%26)) + letter
		index = index/26 - 1
	}
	return letter
}