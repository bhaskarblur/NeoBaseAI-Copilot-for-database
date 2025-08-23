package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/pkg/dbmanager"
	"net/http"
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
		if err := s.storeSheetDataUnified(sqlDB, schemaName, currentTableName, region.Headers, region.DataRows); err != nil {
			log.Printf("Warning: Failed to store region %d: %v", regionIdx+1, err)
			continue
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
			
			// Add column metadata
			for _, header := range region.Headers {
				metadata.Columns = append(metadata.Columns, dtos.ImportColumnMetadata{
					Name:         sanitizeColumnName(header),
					OriginalName: header,
					DataType:     "text",
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
	
	// Prepare response
	response := &dtos.SpreadsheetUploadResponse{
		TableName:   strings.Join(allTables, ", "),
		RowCount:    totalRows,
		ColumnCount: totalColumns,
		SizeBytes:   totalSizeBytes,
		UploadedAt:  time.Now(),
	}
	
	log.Printf("ProcessAndStoreSpreadsheetUnified -> Successfully created/updated %d table(s)", len(allTables))
	return response, http.StatusOK, nil
}

// storeSheetDataUnified stores sheet data exactly like Google Sheets driver
func (s *chatService) storeSheetDataUnified(db *sql.DB, schemaName, tableName string, headers []string, data [][]interface{}) error {
	// Drop existing table if it exists
	dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", schemaName, tableName)
	if _, err := db.Exec(dropQuery); err != nil {
		return fmt.Errorf("failed to drop existing table: %w", err)
	}
	
	// Headers are already cleaned by the analyzer, just validate
	if len(headers) == 0 {
		return fmt.Errorf("no columns found in sheet")
	}
	
	// Create table with columns based on headers (same as Google Sheets)
	columns := make([]string, 0)
	for _, header := range headers {
		colName := sanitizeColumnName(header)
		columns = append(columns, fmt.Sprintf("%s TEXT", colName))
	}
	
	// Add internal columns (same as Google Sheets)
	columns = append(columns, "_row_id SERIAL PRIMARY KEY")
	columns = append(columns, "_imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
	
	createQuery := fmt.Sprintf("CREATE TABLE %s.%s (%s)", schemaName, tableName, strings.Join(columns, ", "))
	if _, err := db.Exec(createQuery); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	
	// Insert data
	if len(data) > 0 {
		// Prepare column names for insert
		colNames := make([]string, 0)
		for _, header := range headers {
			colNames = append(colNames, sanitizeColumnName(header))
		}
		
		// Build insert query (batch insert for performance)
		batchSize := 100
		for i := 0; i < len(data); i += batchSize {
			end := i + batchSize
			if end > len(data) {
				end = len(data)
			}
			
			batch := data[i:end]
			valueStrings := make([]string, 0, len(batch))
			
			for _, row := range batch {
				values := make([]string, 0)
				for j := range headers {
					var value string
					if j < len(row) && row[j] != nil {
						// Convert value to string
						switch v := row[j].(type) {
						case string:
							value = v
						case float64:
							value = fmt.Sprintf("%v", v)
						case bool:
							value = fmt.Sprintf("%v", v)
						default:
							value = fmt.Sprintf("%v", v)
						}
						// Escape single quotes for SQL
						value = strings.ReplaceAll(value, "'", "''")
					}
					values = append(values, fmt.Sprintf("'%s'", value))
				}
				valueStrings = append(valueStrings, fmt.Sprintf("(%s)", strings.Join(values, ", ")))
			}
			
			insertQuery := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES %s",
				schemaName, tableName,
				strings.Join(colNames, ", "),
				strings.Join(valueStrings, ", "))
			
			if _, err := db.Exec(insertQuery); err != nil {
				log.Printf("Warning: Failed to insert batch %d-%d: %v", i, end, err)
				// Continue with other batches
			}
		}
	}
	
	return nil
}

