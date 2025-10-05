package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/dbmanager"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ProcessAndStoreSpreadsheetUnified processes CSV/Excel data exactly like Google Sheets
// This ensures identical handling between all spreadsheet sources
func (s *chatService) ProcessAndStoreSpreadsheetUnified(
	userID string,
	chatID string,
	baseTableName string,
	data [][]interface{},
	mergeStrategy string,
	mergeOptions MergeOptions,
) (*dtos.SpreadsheetUploadResponse, uint32, error) {
	
	log.Printf("ProcessAndStoreSpreadsheetUnified -> Starting for chat %s, base table %s", 
		chatID, baseTableName)
	
	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return nil, http.StatusNotFound, fmt.Errorf("no connection found for chat: %s", chatID)
	}
	
	// Verify it's a spreadsheet connection
	if connInfo.Config.Type != constants.DatabaseTypeSpreadsheet {
		return nil, http.StatusBadRequest, fmt.Errorf("connection is not a spreadsheet type")
	}
	
	// Get database connection
	conn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get database connection: %v", err)
	}
	
	// Get SQL DB
	sqlDB := conn.GetDB()
	if sqlDB == nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get SQL DB connection")
	}
	
	// Set the schema name (same as Google Sheets)
	schemaName := fmt.Sprintf("conn_%s", chatID)
	
	// Create schema if it doesn't exist (same as Google Sheets)
	createSchemaQuery := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)
	if _, err := sqlDB.Exec(createSchemaQuery); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to create schema: %w", err)
	}
	
	// Use robust analyzer to process the data (exactly like Google Sheets)
	robustAnalyzer := dbmanager.NewRobustSheetAnalyzer(data)
	regions, err := robustAnalyzer.AnalyzeRobust()
	if err != nil {
		log.Printf("Warning: Failed to analyze data: %v, falling back to unstructured", err)
		// Create fallback unstructured region
		region := &dbmanager.DataRegion{
			Headers:  []string{"row_num", "col_num", "value"},
			DataRows: make([][]interface{}, 0),
		}
		for rowIdx, row := range data {
			for colIdx, cell := range row {
				if cell != nil && fmt.Sprintf("%v", cell) != "" {
					region.DataRows = append(region.DataRows, []interface{}{
						rowIdx + 1,
						columnIndexToLetter(colIdx),
						cell,
					})
				}
			}
		}
		if len(region.DataRows) == 0 {
			region.DataRows = append(region.DataRows, []interface{}{1, "A", "No data found"})
		}
		regions = []*dbmanager.DataRegion{region}
	}
	
	if len(regions) == 0 {
		log.Printf("No regions detected, creating unstructured table")
		// Create minimal unstructured region
		region := &dbmanager.DataRegion{
			Headers:  []string{"row_num", "col_num", "value"},
			DataRows: [][]interface{}{{1, "A", "No data found"}},
		}
		regions = []*dbmanager.DataRegion{region}
	}
	
	// Process all detected regions (same as Google Sheets)
	allTables := make([]string, 0)
	totalRows := 0
	totalColumns := 0
	var totalSizeBytes int64
	
	// Track overall error information
	allErrors := make([]string, 0)
	totalProcessed := 0
	totalSuccessful := 0
	totalFailed := 0
	
	for regionIdx, region := range regions {
		// Determine table name (same naming convention as Google Sheets)
		currentTableName := baseTableName
		if len(regions) > 1 {
			currentTableName = fmt.Sprintf("%s_%d", baseTableName, regionIdx+1)
		}
		
		log.Printf("Processing region %d/%d as table '%s'", regionIdx+1, len(regions), currentTableName)
		log.Printf("  - Headers: %v", region.Headers)
		log.Printf("  - Rows: %d", len(region.DataRows))
		log.Printf("  - Quality: %.1f%%", region.Quality)
		
		if len(region.Issues) > 0 {
			log.Printf("  - Issues detected:")
			for _, issue := range region.Issues {
				log.Printf("    • %s", issue)
			}
		}
		
		if len(region.Suggestions) > 0 {
			log.Printf("  - Suggestions:")
			for _, suggestion := range region.Suggestions {
				log.Printf("    • %s", suggestion)
			}
		}
		
		// Handle merge strategy for existing tables
		if mergeStrategy != "" && mergeStrategy != "replace" {
			// Check if table exists
			checkQuery := fmt.Sprintf(`
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = '%s' 
					AND table_name = '%s'
				)
			`, schemaName, currentTableName)
			
			var rows []map[string]interface{}
			if err := conn.QueryRows(checkQuery, &rows); err == nil && len(rows) > 0 {
				if exists, ok := rows[0]["exists"].(bool); ok && exists {
					// Table exists, handle merge
					log.Printf("Table %s exists, applying %s strategy", currentTableName, mergeStrategy)
					
					// Convert region data to string format for merge handler
					stringData := make([][]string, len(region.DataRows))
					for i, row := range region.DataRows {
						stringRow := make([]string, len(row))
						for j, cell := range row {
							if cell != nil {
								stringRow[j] = fmt.Sprintf("%v", cell)
							} else {
								stringRow[j] = ""
							}
						}
						stringData[i] = stringRow
					}
					
					mergeHandler := NewSpreadsheetMergeHandler(conn, schemaName, currentTableName)
					if mergeOptions.Strategy == "" {
						mergeOptions.Strategy = mergeStrategy
					}
					
					if err := mergeHandler.ExecuteMerge(region.Headers, stringData, mergeOptions); err != nil {
						log.Printf("Warning: Merge failed for table %s: %v", currentTableName, err)
						continue
					}
					
					allTables = append(allTables, currentTableName)
					totalRows += len(region.DataRows)
					if len(region.Headers) > totalColumns {
						totalColumns = len(region.Headers)
					}
					continue
				}
			}
		}
		
		// Store the region data (exactly like Google Sheets)
		insertResult, err := s.storeSheetDataUnified(sqlDB, schemaName, currentTableName, region.Headers, region.DataRows)
		if err != nil {
			log.Printf("Warning: Failed to store region %d: %v", regionIdx+1, err)
			if insertResult != nil {
				// Still collect error information even if storing failed
				totalProcessed += insertResult.TotalRowsProcessed
				totalSuccessful += insertResult.SuccessfulRows
				totalFailed += insertResult.FailedRows
				allErrors = append(allErrors, insertResult.Errors...)
			}
			continue
		}
		
		// Collect insertion statistics
		if insertResult != nil {
			totalProcessed += insertResult.TotalRowsProcessed
			totalSuccessful += insertResult.SuccessfulRows
			totalFailed += insertResult.FailedRows
			allErrors = append(allErrors, insertResult.Errors...)
		}
		
		allTables = append(allTables, currentTableName)
		totalRows += len(region.DataRows)
		if len(region.Headers) > totalColumns {
			totalColumns = len(region.Headers)
		}
		
		// Get table size
		sizeQuery := fmt.Sprintf(
			"SELECT pg_total_relation_size('%s.%s') as size",
			schemaName,
			currentTableName,
		)
		var sizeRows []map[string]interface{}
		if err := conn.QueryRows(sizeQuery, &sizeRows); err == nil && len(sizeRows) > 0 {
			if size, ok := sizeRows[0]["size"].(int64); ok {
				totalSizeBytes += size
			}
		}
		
		// Store metadata if available (similar to Google Sheets)
		redisRepo := s.dbManager.GetRedisRepo()
		if redisRepo != nil && chatID != "" {
			metadata := &dtos.ImportMetadata{
				TableName:   currentTableName,
				RowCount:    len(region.DataRows),
				ColumnCount: len(region.Headers),
				Quality:     region.Quality,
				Issues:      region.Issues,
				Suggestions: region.Suggestions,
				Columns:     make([]dtos.ImportColumnMetadata, 0),
			}
			
			// Add column metadata with inferred types
			for _, header := range region.Headers {
				dataType := "text" // default fallback
				if inferredTypes, err := utils.NewDataTypeInferrer().InferColumnTypes(region.Headers, region.DataRows); err == nil {
					if colType, exists := inferredTypes[header]; exists {
						dataType = strings.ToLower(colType.PostgreSQLType)
					}
				}
				
				metadata.Columns = append(metadata.Columns, dtos.ImportColumnMetadata{
					Name:         sanitizeColumnName(header),
					OriginalName: header,
					DataType:     dataType,
				})
			}
			
			metadataStore := dbmanager.NewImportMetadataStore(redisRepo)
			if err := metadataStore.StoreMetadata(chatID, metadata); err != nil {
				log.Printf("Warning: Failed to store import metadata: %v", err)
			}
		}
	}
	
	if len(allTables) == 0 {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to create any tables from spreadsheet data")
	}
	
	// Update the connection's schema name in the manager (like Google Sheets)
	// We need to get the actual connection and update it
	if actualConn, exists := s.dbManager.GetConnectionInfo(chatID); exists {
		actualConn.Config.SchemaName = schemaName
		log.Printf("ProcessAndStoreSpreadsheetUnified -> Set schema name: %s", schemaName)
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
	
	// Prepare response with error information
	response := &dtos.SpreadsheetUploadResponse{
		TableName:          strings.Join(allTables, ", "),
		RowCount:           totalSuccessful, // Only count successfully inserted rows
		ColumnCount:        totalColumns,
		SizeBytes:          totalSizeBytes,
		UploadedAt:         time.Now(),
		TotalRowsProcessed: totalProcessed,
		SuccessfulRows:     totalSuccessful,
		FailedRows:         totalFailed,
		Errors:             allErrors,
		HasErrors:          len(allErrors) > 0 || totalFailed > 0,
	}
	
	log.Printf("ProcessAndStoreSpreadsheetUnified -> Successfully created/updated %d table(s)", len(allTables))
	return response, http.StatusOK, nil
}

// DataInsertionResult contains results of data insertion including error details
type DataInsertionResult struct {
	TotalRowsProcessed int
	SuccessfulRows     int
	FailedRows         int
	Errors             []string
}

// HasErrors returns true if there were any errors during insertion
func (r *DataInsertionResult) HasErrors() bool {
	return r.FailedRows > 0 || len(r.Errors) > 0
}

// storeSheetDataUnified stores sheet data exactly like Google Sheets driver
func (s *chatService) storeSheetDataUnified(db *sql.DB, schemaName, tableName string, headers []string, data [][]interface{}) (*DataInsertionResult, error) {
	// Drop existing table if it exists
	dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", schemaName, tableName)
	if _, err := db.Exec(dropQuery); err != nil {
		return nil, fmt.Errorf("failed to drop existing table: %w", err)
	}
	
	// Headers are already cleaned by the analyzer, just validate
	if len(headers) == 0 {
		return nil, fmt.Errorf("no columns found in sheet")
	}
	
	// Infer data types for columns using intelligent sampling
	inferrer := utils.NewDataTypeInferrer()
	inferredTypes, err := inferrer.InferColumnTypes(headers, data)
	if err != nil {
		log.Printf("Warning: Failed to infer column types: %v, falling back to TEXT", err)
		// Fallback to TEXT for all columns
		inferredTypes = make(map[string]utils.ColumnDataType)
		for _, header := range headers {
			inferredTypes[header] = utils.ColumnDataType{
				PostgreSQLType: "TEXT",
				SQLType:        "TEXT",
				IsNullable:     true,
			}
		}
	}

	// Create table with columns based on inferred types
	columns := make([]string, 0)
	for _, header := range headers {
		colName := sanitizeColumnName(header)
		dataType := inferredTypes[header]
		columns = append(columns, fmt.Sprintf("%s %s", colName, dataType.PostgreSQLType))
		
		log.Printf("Column %s -> %s (sample: %d, errors: %d)", 
			header, dataType.PostgreSQLType, dataType.SampleSize, dataType.ErrorCount)
	}
	
	// Add internal columns (same as Google Sheets)
	columns = append(columns, "_row_id SERIAL PRIMARY KEY")
	columns = append(columns, "_imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
	
	createQuery := fmt.Sprintf("CREATE TABLE %s.%s (%s)", schemaName, tableName, strings.Join(columns, ", "))
	if _, err := db.Exec(createQuery); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}
	
	// Insert data
	if len(data) > 0 {
		// Prepare column names for insert
		colNames := make([]string, 0)
		for _, header := range headers {
			colNames = append(colNames, sanitizeColumnName(header))
		}
		
		// Build insert query with type-aware conversion (batch insert for performance)
		batchSize := 100
		totalRows := 0
		successfulRows := 0
		failedRows := 0
		
		for i := 0; i < len(data); i += batchSize {
			end := i + batchSize
			if end > len(data) {
				end = len(data)
			}
			
			batch := data[i:end]
			validRows := make([]string, 0, len(batch)) // All rows for insertion
			
			for rowIdx, row := range batch {
				values := make([]string, 0)
				
				for j, header := range headers {
					var value string
					if j < len(row) && row[j] != nil {
						rawValue := fmt.Sprintf("%v", row[j])
						dataType := inferredTypes[header]
						
						// Convert value according to inferred type
						convertedValue, conversionErr := s.convertValueToType(rawValue, dataType.PostgreSQLType)
						if conversionErr != nil {
							// Instead of skipping the row, store NULL for invalid values
							log.Printf("CONVERSION_WARNING: Table '%s', Column '%s', Row %d: Cannot convert '%s' to %s, storing as NULL", 
								tableName, header, i+rowIdx+1, rawValue, dataType.PostgreSQLType)
							value = "" // Will be formatted as NULL by formatSQLValue
						} else {
							value = convertedValue
						}
					}
					// Use appropriate SQL value formatting
					values = append(values, s.formatSQLValue(value, inferredTypes[header].PostgreSQLType))
				}
				
				// Add all rows to the batch (no longer skipping rows with conversion errors)
				validRows = append(validRows, fmt.Sprintf("(%s)", strings.Join(values, ", ")))
				successfulRows++
				totalRows++
			}
			
			// Insert all rows (we no longer skip rows with conversion errors)
			if len(validRows) > 0 {
				insertQuery := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES %s",
					schemaName, tableName,
					strings.Join(colNames, ", "),
					strings.Join(validRows, ", "))
				
				if _, err := db.Exec(insertQuery); err != nil {
					// Log the batch failure but continue processing
					log.Printf("Error: Failed to insert batch %d-%d with %d rows: %v", i, end, len(validRows), err)
					log.Printf("Failed query: %s", insertQuery)
					// Mark these rows as failed
					failedRows += len(validRows)
					successfulRows -= len(validRows)
				}
			}
		}
		
		// Log comprehensive summary
		log.Printf("DATA_INSERTION_SUMMARY for table '%s':", tableName)
		log.Printf("  - Total rows processed: %d", totalRows)
		log.Printf("  - Successfully inserted: %d", successfulRows)
		log.Printf("  - Failed: %d", failedRows)
		
		// Return detailed results
		result := &DataInsertionResult{
			TotalRowsProcessed: totalRows,
			SuccessfulRows:     successfulRows,
			FailedRows:         failedRows,
			Errors:             []string{}, // No longer tracking individual errors
		}
		
		// Only return error if NO rows were successful
		if successfulRows == 0 && totalRows > 0 {
			return result, fmt.Errorf("no rows could be inserted into table '%s' - all %d rows failed database insertion", tableName, totalRows)
		}
		
		return result, nil
	}
	
	return &DataInsertionResult{
		TotalRowsProcessed: 0,
		SuccessfulRows:     0,
		FailedRows:         0,
		Errors:             []string{},
	}, nil
}

// convertValueToType attempts to convert a string value to the specified PostgreSQL type
func (s *chatService) convertValueToType(value string, postgresType string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil // NULL value
	}

	switch strings.ToUpper(postgresType) {
	case "INTEGER":
		// Remove common number formatting (commas, spaces) before parsing
		cleanValue := strings.ReplaceAll(value, ",", "")
		cleanValue = strings.ReplaceAll(cleanValue, " ", "")
		
		_, err := strconv.ParseInt(cleanValue, 10, 64)
		if err != nil {
			return "", fmt.Errorf("cannot convert '%s' to INTEGER: %w", value, err)
		}
		return cleanValue, nil

	case "NUMERIC", "DECIMAL":
		// Remove common number formatting (commas, spaces) before parsing
		cleanValue := strings.ReplaceAll(value, ",", "")
		cleanValue = strings.ReplaceAll(cleanValue, " ", "")
		
		_, err := strconv.ParseFloat(cleanValue, 64)
		if err != nil {
			return "", fmt.Errorf("cannot convert '%s' to NUMERIC: %w", value, err)
		}
		return cleanValue, nil

	case "BOOLEAN":
		lower := strings.ToLower(value)
		switch lower {
		case "true", "yes", "1", "y":
			return "true", nil
		case "false", "no", "0", "n":
			return "false", nil
		default:
			return "", fmt.Errorf("cannot convert '%s' to BOOLEAN", value)
		}

	case "DATE":
		// Try to parse various date formats
		dateFormats := []string{
			"2006-01-02",
			"01/02/2006",
			"01-02-2006",
			"2006/01/02",
			"02/01/2006",
			"02-01-2006",
		}
		for _, format := range dateFormats {
			if parsedTime, err := time.Parse(format, value); err == nil {
				return parsedTime.Format("2006-01-02"), nil
			}
		}
		return "", fmt.Errorf("cannot convert '%s' to DATE", value)

	case "TIMESTAMP":
		// Try to parse various timestamp formats
		timestampFormats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05.000",
			"01/02/2006 15:04:05",
			"01-02-2006 15:04:05",
		}
		for _, format := range timestampFormats {
			if parsedTime, err := time.Parse(format, value); err == nil {
				return parsedTime.Format("2006-01-02 15:04:05"), nil
			}
		}
		return "", fmt.Errorf("cannot convert '%s' to TIMESTAMP", value)

	default:
		// For TEXT, VARCHAR, UUID, etc., return as-is
		return value, nil
	}
}

// formatSQLValue formats a value for SQL insertion based on the column type
func (s *chatService) formatSQLValue(value string, postgresType string) string {
	if value == "" {
		return "NULL"
	}

	switch strings.ToUpper(postgresType) {
	case "INTEGER", "NUMERIC", "DECIMAL", "BOOLEAN":
		// Numeric and boolean values don't need quotes
		return value
	default:
		// String types need quotes and escaping
		escaped := strings.ReplaceAll(value, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	}
}

