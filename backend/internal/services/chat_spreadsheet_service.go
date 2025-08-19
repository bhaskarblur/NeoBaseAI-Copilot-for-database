package services

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// StoreSpreadsheetData stores CSV/Excel data in the spreadsheet database
func (s *chatService) StoreSpreadsheetData(userID, chatID, tableName string, columns []string, data [][]string, mergeStrategy string, mergeOptions MergeOptions) (*dtos.SpreadsheetUploadResponse, uint32, error) {
	log.Printf("ChatService -> StoreSpreadsheetData -> Starting for chatID: %s, table: %s, strategy: %s", chatID, tableName, mergeStrategy)

	// Validate inputs
	if tableName == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("table name is required")
	}
	if len(columns) == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("no columns provided")
	}
	if len(data) == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("no data provided")
	}

	// Default merge strategy
	if mergeStrategy == "" {
		mergeStrategy = "replace"
	}

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return nil, http.StatusNotFound, fmt.Errorf("connection not found")
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

	// Set the schema context
	schemaName := connInfo.Config.SchemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", chatID)
	}

	// Check if table exists
	tableExists := false
	var existingRowCount int64
	checkQuery := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = '%s' 
			AND table_name = '%s'
		)
	`, schemaName, tableName)
	
	var rows []map[string]interface{}
	err = conn.QueryRows(checkQuery, &rows)
	if err == nil && len(rows) > 0 && len(rows[0]) > 0 {
		if exists, ok := rows[0]["exists"].(bool); ok {
			tableExists = exists
		}
	}

	// If table exists, handle based on merge strategy
	if tableExists {
		// Get existing row count for reporting
		var countRows []map[string]interface{}
		err := conn.QueryRows(fmt.Sprintf("SELECT COUNT(*) as count FROM %s.%s", schemaName, tableName), &countRows)
		if err == nil && len(countRows) > 0 {
			if count, ok := countRows[0]["count"].(int64); ok {
				existingRowCount = count
			}
		}

		// Use merge handler for complex operations
		if mergeStrategy != "replace" {
			mergeHandler := NewSpreadsheetMergeHandler(conn, schemaName, tableName)
			
			// Use provided options or defaults
			if mergeOptions.Strategy == "" {
				mergeOptions.Strategy = mergeStrategy
			}
			
			// Execute merge
			if err := mergeHandler.ExecuteMerge(columns, data, mergeOptions); err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("merge operation failed: %v", err)
			}
			
			// Get final row count
			finalCount := existingRowCount + int64(len(data))
			if mergeStrategy == "merge" || mergeStrategy == "smart_merge" {
				// For merge, recount as rows might have been updated
				var countRows []map[string]interface{}
				err := conn.QueryRows(fmt.Sprintf("SELECT COUNT(*) as count FROM %s.%s", schemaName, tableName), &countRows)
				if err == nil && len(countRows) > 0 {
					if count, ok := countRows[0]["count"].(int64); ok {
						finalCount = count
					}
				}
			}
			
			// Get table size
			var sizeBytes int64
			sizeQuery := fmt.Sprintf(
				"SELECT pg_total_relation_size('%s.%s') as size",
				schemaName,
				tableName,
			)
			var sizeRows []map[string]interface{}
			err = conn.QueryRows(sizeQuery, &sizeRows)
			if err == nil && len(sizeRows) > 0 {
				if size, ok := sizeRows[0]["size"].(int64); ok {
					sizeBytes = size
				}
			}
			
			// Trigger schema refresh and update database name synchronously for better consistency
			log.Printf("ChatService -> StoreSpreadsheetData (merge) -> Starting schema refresh and database name update for chatID: %s", chatID)
			ctx := context.Background()
			if _, err := s.RefreshSchema(ctx, userID, chatID, false); err != nil {
				log.Printf("ChatService -> StoreSpreadsheetData -> Failed to refresh schema: %v", err)
			}
			// Update the database name based on tables
			log.Printf("ChatService -> StoreSpreadsheetData (merge) -> About to call updateSpreadsheetDatabaseName for chatID: %s", chatID)
			if err := s.updateSpreadsheetDatabaseName(chatID); err != nil {
				log.Printf("ChatService -> StoreSpreadsheetData -> Failed to update database name: %v", err)
			}
			log.Printf("ChatService -> StoreSpreadsheetData (merge) -> Completed schema refresh and database name update for chatID: %s", chatID)
			
			return &dtos.SpreadsheetUploadResponse{
				TableName:   tableName,
				RowCount:    int(finalCount),
				ColumnCount: len(columns),
				SizeBytes:   sizeBytes,
				UploadedAt:  time.Now(),
			}, http.StatusOK, nil
		}
		
		// Replace strategy - drop existing table
		if mergeStrategy == "replace" {
			dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s CASCADE", schemaName, tableName)
			if err := conn.Exec(dropQuery); err != nil {
				return nil, http.StatusInternalServerError, fmt.Errorf("failed to drop existing table: %v", err)
			}
			tableExists = false
		}
	}

	// Create table if it doesn't exist
	if !tableExists {
		// Create table with proper column types
		columnDefs := make([]string, 0)
		columnDefs = append(columnDefs, "_id SERIAL PRIMARY KEY")
		columnDefs = append(columnDefs, "_created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
		columnDefs = append(columnDefs, "_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
		
		for _, col := range columns {
			sanitizedCol := sanitizeColumnName(col)
			columnDefs = append(columnDefs, fmt.Sprintf("%s TEXT", sanitizedCol))
		}

		createTableQuery := fmt.Sprintf(
			"CREATE TABLE %s.%s (%s)",
			schemaName,
			tableName,
			strings.Join(columnDefs, ", "),
		)

		if err := conn.Exec(createTableQuery); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to create table: %v", err)
		}
	}

	// Insert data in batches
	batchSize := 1000
	totalRows := len(data)
	
	for i := 0; i < totalRows; i += batchSize {
		end := i + batchSize
		if end > totalRows {
			end = totalRows
		}

		batch := data[i:end]
		
		// Build insert query
		valueStrings := make([]string, 0, len(batch))
		for _, row := range batch {
			values := make([]string, len(columns))
			for j, val := range row {
				// Escape single quotes
				escapedVal := strings.ReplaceAll(val, "'", "''")
				values[j] = fmt.Sprintf("'%s'", escapedVal)
			}
			valueStrings = append(valueStrings, fmt.Sprintf("(%s)", strings.Join(values, ", ")))
		}

		sanitizedColumns := make([]string, len(columns))
		for i, col := range columns {
			sanitizedColumns[i] = sanitizeColumnName(col)
		}

		insertQuery := fmt.Sprintf(
			"INSERT INTO %s.%s (%s) VALUES %s",
			schemaName,
			tableName,
			strings.Join(sanitizedColumns, ", "),
			strings.Join(valueStrings, ", "),
		)

		if err := conn.Exec(insertQuery); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to insert data: %v", err)
		}
	}

	// Get table size
	var sizeBytes int64
	sizeQuery := fmt.Sprintf(
		"SELECT pg_total_relation_size('%s.%s')",
		schemaName,
		tableName,
	)
	var sizeBytesData []map[string]interface{}
	if err := conn.QueryRows(sizeQuery, &sizeBytesData); err == nil && len(sizeBytesData) > 0 {
		if size, ok := sizeBytesData[0]["total_size_bytes"].(float64); ok {
			sizeBytes = int64(size)
		}
	}

	// Trigger schema refresh and update database name synchronously for better consistency
	log.Printf("ChatService -> StoreSpreadsheetData -> Starting schema refresh and database name update for chatID: %s", chatID)
	ctx := context.Background()
	if _, err := s.RefreshSchema(ctx, userID, chatID, false); err != nil {
		log.Printf("ChatService -> StoreSpreadsheetData -> Failed to refresh schema: %v", err)
	}
	// Update the database name based on tables
	log.Printf("ChatService -> StoreSpreadsheetData -> About to call updateSpreadsheetDatabaseName for chatID: %s", chatID)
	if err := s.updateSpreadsheetDatabaseName(chatID); err != nil {
		log.Printf("ChatService -> StoreSpreadsheetData -> Failed to update database name: %v", err)
	}
	log.Printf("ChatService -> StoreSpreadsheetData -> Completed schema refresh and database name update for chatID: %s", chatID)

	return &dtos.SpreadsheetUploadResponse{
		TableName:   tableName,
		RowCount:    totalRows,
		ColumnCount: len(columns),
		SizeBytes:   sizeBytes,
		UploadedAt:  time.Now(),
	}, http.StatusOK, nil
}

// GetSpreadsheetTableData retrieves paginated data from a spreadsheet table
func (s *chatService) GetSpreadsheetTableData(userID, chatID, tableName string, page, pageSize int) (*dtos.SpreadsheetTableDataResponse, uint32, error) {
	log.Printf("ChatService -> GetSpreadsheetTableData -> Starting for chatID: %s, table: %s", chatID, tableName)

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 50
	}

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return nil, http.StatusNotFound, fmt.Errorf("connection not found")
	}

	// Get database connection
	conn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get database connection: %v", err)
	}

	schemaName := connInfo.Config.SchemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", chatID)
	}

	// Get total row count
	var totalRows int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) as count FROM %s.%s", schemaName, tableName)
	log.Printf("ChatService -> GetSpreadsheetTableData -> Count query: %s", countQuery)
	var countData []map[string]interface{}
	if err := conn.QueryRows(countQuery, &countData); err != nil {
		log.Printf("ChatService -> GetSpreadsheetTableData -> Error getting row count: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get row count: %v", err)
	}
	log.Printf("ChatService -> GetSpreadsheetTableData -> Count data: %+v", countData)
	if len(countData) > 0 {
		if count, ok := countData[0]["count"].(int64); ok {
			totalRows = count
		} else if count, ok := countData[0]["count"].(float64); ok {
			totalRows = int64(count)
		}
	}
	log.Printf("ChatService -> GetSpreadsheetTableData -> Total rows: %d", totalRows)

	// Get column information - get ALL columns first
	allColQuery := fmt.Sprintf(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = '%s' AND table_name = '%s' 
		ORDER BY ordinal_position
	`, schemaName, tableName)
	log.Printf("ChatService -> GetSpreadsheetTableData -> Column query: %s", allColQuery)
	var columnData []map[string]interface{}
	if err := conn.QueryRows(allColQuery, &columnData); err != nil {
		log.Printf("ChatService -> GetSpreadsheetTableData -> Error getting columns: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get columns: %v", err)
	}
	log.Printf("ChatService -> GetSpreadsheetTableData -> All column data: %+v", columnData)
	
	// Filter out internal columns in Go
	var columns []struct {
		ColumnName string `gorm:"column:column_name"`
	}
	for _, col := range columnData {
		var colName string
		
		// Handle both string and byte array formats
		if nameStr, ok := col["column_name"].(string); ok {
			colName = nameStr
		} else if nameBytes, ok := col["column_name"].([]uint8); ok {
			colName = string(nameBytes)
		} else {
			log.Printf("ChatService -> Unexpected column_name type: %T", col["column_name"])
			continue
		}
		
		// Skip internal columns
		if strings.HasPrefix(colName, "_") {
			continue
		}
		columns = append(columns, struct {
			ColumnName string `gorm:"column:column_name"`
		}{
			ColumnName: colName,
		})
	}

	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.ColumnName)
	}
	log.Printf("ChatService -> GetSpreadsheetTableData -> Column names: %v", columnNames)
	
	// If no columns found (shouldn't happen), use SELECT *
	selectClause := "*"
	if len(columnNames) > 0 {
		selectClause = strings.Join(columnNames, ", ")
	}

	// Get paginated data
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(
		"SELECT %s FROM %s.%s ORDER BY _id LIMIT %d OFFSET %d",
		selectClause,
		schemaName,
		tableName,
		pageSize,
		offset,
	)
	log.Printf("ChatService -> GetSpreadsheetTableData -> Data query: %s", dataQuery)

	var rows []map[string]interface{}
	if err := conn.QueryRows(dataQuery, &rows); err != nil {
		log.Printf("ChatService -> GetSpreadsheetTableData -> Error getting data: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get data: %v", err)
	}
	log.Printf("ChatService -> GetSpreadsheetTableData -> Retrieved %d rows", len(rows))
	
	// Process rows: decrypt and handle empty values
	for i, row := range rows {
		for key, value := range row {
			// Skip internal columns
			if strings.HasPrefix(key, "_") {
				continue
			}
			
			// Handle null/empty values
			if value == nil || (fmt.Sprintf("%v", value) == "") {
				rows[i][key] = "-"
				continue
			}
			
			// No decryption needed - data is stored in plain text
		}
	}

	totalPages := int((totalRows + int64(pageSize) - 1) / int64(pageSize))

	return &dtos.SpreadsheetTableDataResponse{
		TableName:  tableName,
		Columns:    columnNames,
		Rows:       rows,
		TotalRows:  int(totalRows),
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, http.StatusOK, nil
}

// DeleteSpreadsheetTable deletes a table from the spreadsheet database
func (s *chatService) DeleteSpreadsheetTable(userID, chatID, tableName string) (uint32, error) {
	log.Printf("ChatService -> DeleteSpreadsheetTable -> Starting for chatID: %s, table: %s", chatID, tableName)

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return http.StatusNotFound, fmt.Errorf("connection not found")
	}

	// Get database connection
	conn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to get database connection: %v", err)
	}

	schemaName := connInfo.Config.SchemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", chatID)
	}

	// Drop the table
	dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s CASCADE", schemaName, tableName)
	if err := conn.Exec(dropQuery); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to drop table: %v", err)
	}

	// Update selected collections if this table was selected
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid chat ID")
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil || chat == nil {
		return http.StatusNotFound, fmt.Errorf("chat not found")
	}

	if chat.SelectedCollections != "ALL" && chat.SelectedCollections != "" {
		collections := strings.Split(chat.SelectedCollections, ",")
		newCollections := make([]string, 0)
		for _, col := range collections {
			if col != tableName {
				newCollections = append(newCollections, col)
			}
		}
		chat.SelectedCollections = strings.Join(newCollections, ",")
		if err := s.chatRepo.Update(chat.ID, chat); err != nil {
			log.Printf("ChatService -> DeleteSpreadsheetTable -> Failed to update selected collections: %v", err)
		}
	}

	// Trigger schema refresh and update database name
	go func() {
		ctx := context.Background()
		if _, err := s.RefreshSchema(ctx, userID, chatID, false); err != nil {
			log.Printf("ChatService -> DeleteSpreadsheetTable -> Failed to refresh schema: %v", err)
		}
		// Update the database name based on remaining tables
		if err := s.updateSpreadsheetDatabaseName(chatID); err != nil {
			log.Printf("ChatService -> DeleteSpreadsheetTable -> Failed to update database name: %v", err)
		}
	}()

	return http.StatusOK, nil
}

// DownloadSpreadsheetTableData gets all data from a table for download
func (s *chatService) DownloadSpreadsheetTableData(userID, chatID, tableName string) (*dtos.SpreadsheetDownloadResponse, uint32, error) {
	log.Printf("ChatService -> DownloadSpreadsheetTableData -> Starting for chatID: %s, table: %s", chatID, tableName)

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return nil, http.StatusNotFound, fmt.Errorf("connection not found")
	}

	// Get database connection
	conn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get database connection: %v", err)
	}

	schemaName := connInfo.Config.SchemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", chatID)
	}

	// Get column information - get ALL columns first
	allColQuery := fmt.Sprintf(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = '%s' AND table_name = '%s' 
		ORDER BY ordinal_position
	`, schemaName, tableName)
	log.Printf("ChatService -> GetSpreadsheetTableData -> Column query: %s", allColQuery)
	var columnData []map[string]interface{}
	if err := conn.QueryRows(allColQuery, &columnData); err != nil {
		log.Printf("ChatService -> GetSpreadsheetTableData -> Error getting columns: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get columns: %v", err)
	}
	log.Printf("ChatService -> GetSpreadsheetTableData -> All column data: %+v", columnData)
	
	// Filter out internal columns in Go
	var columns []struct {
		ColumnName string `gorm:"column:column_name"`
	}
	for _, col := range columnData {
		var colName string
		
		// Handle both string and byte array formats
		if nameStr, ok := col["column_name"].(string); ok {
			colName = nameStr
		} else if nameBytes, ok := col["column_name"].([]uint8); ok {
			colName = string(nameBytes)
		} else {
			log.Printf("ChatService -> Unexpected column_name type: %T", col["column_name"])
			continue
		}
		
		// Skip internal columns
		if strings.HasPrefix(colName, "_") {
			continue
		}
		columns = append(columns, struct {
			ColumnName string `gorm:"column:column_name"`
		}{
			ColumnName: colName,
		})
	}

	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.ColumnName)
	}

	// Get all data
	dataQuery := fmt.Sprintf(
		"SELECT %s FROM %s.%s ORDER BY _id",
		strings.Join(columnNames, ", "),
		schemaName,
		tableName,
	)

	var rows []map[string]interface{}
	if err := conn.QueryRows(dataQuery, &rows); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get data: %v", err)
	}
	
	// Process rows: decrypt and handle empty values
	for i, row := range rows {
		for key, value := range row {
			// Skip internal columns
			if strings.HasPrefix(key, "_") {
				continue
			}
			
			// Handle null/empty values
			if value == nil || (fmt.Sprintf("%v", value) == "") {
				rows[i][key] = "-"
				continue
			}
			
			// No decryption needed - data is stored in plain text
		}
	}

	log.Printf("ChatService -> DownloadSpreadsheetTableData -> Returning %d columns and %d rows", 
		len(columnNames), len(rows))
	
	return &dtos.SpreadsheetDownloadResponse{
		TableName: tableName,
		Columns:   columnNames,
		Rows:      rows,
	}, http.StatusOK, nil
}

// DownloadSpreadsheetTableDataWithFilter gets filtered data from a table for download
func (s *chatService) DownloadSpreadsheetTableDataWithFilter(userID, chatID, tableName string, rowIDs []string) (*dtos.SpreadsheetDownloadResponse, uint32, error) {
	log.Printf("ChatService -> DownloadSpreadsheetTableDataWithFilter -> Starting for chatID: %s, table: %s, rowIDs: %v", chatID, tableName, rowIDs)

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return nil, http.StatusNotFound, fmt.Errorf("connection not found")
	}

	// Get database connection
	conn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get database connection: %v", err)
	}

	schemaName := connInfo.Config.SchemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", chatID)
	}

	// Get column information - get ALL columns first
	allColQuery := fmt.Sprintf(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = '%s' AND table_name = '%s' 
		ORDER BY ordinal_position
	`, schemaName, tableName)
	log.Printf("ChatService -> DownloadSpreadsheetTableDataWithFilter -> Column query: %s", allColQuery)
	var columnData []map[string]interface{}
	if err := conn.QueryRows(allColQuery, &columnData); err != nil {
		log.Printf("ChatService -> DownloadSpreadsheetTableDataWithFilter -> Error getting columns: %v", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get columns: %v", err)
	}
	
	// Filter out internal columns in Go
	var columns []struct {
		ColumnName string `gorm:"column:column_name"`
	}
	for _, col := range columnData {
		var colName string
		
		// Handle both string and byte array formats
		if nameStr, ok := col["column_name"].(string); ok {
			colName = nameStr
		} else if nameBytes, ok := col["column_name"].([]uint8); ok {
			colName = string(nameBytes)
		} else {
			log.Printf("ChatService -> Unexpected column_name type: %T", col["column_name"])
			continue
		}
		
		// Skip internal columns
		if strings.HasPrefix(colName, "_") {
			continue
		}
		columns = append(columns, struct {
			ColumnName string `gorm:"column:column_name"`
		}{
			ColumnName: colName,
		})
	}

	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.ColumnName)
	}

	// Build WHERE clause for row IDs with proper escaping
	escapedRowIDs := make([]string, len(rowIDs))
	for i, id := range rowIDs {
		// Escape single quotes in row IDs
		escapedID := strings.ReplaceAll(id, "'", "''")
		escapedRowIDs[i] = fmt.Sprintf("'%s'", escapedID)
	}
	whereClause := fmt.Sprintf("WHERE _id IN (%s)", strings.Join(escapedRowIDs, ", "))

	// Get filtered data
	dataQuery := fmt.Sprintf(
		"SELECT %s FROM %s.%s %s ORDER BY _id",
		strings.Join(columnNames, ", "),
		schemaName,
		tableName,
		whereClause,
	)

	var rows []map[string]interface{}
	if err := conn.QueryRows(dataQuery, &rows); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get data: %v", err)
	}
	
	// Process rows: decrypt and handle empty values
	for i, row := range rows {
		for key, value := range row {
			// Skip internal columns
			if strings.HasPrefix(key, "_") {
				continue
			}
			
			// Handle null/empty values
			if value == nil || (fmt.Sprintf("%v", value) == "") {
				rows[i][key] = "-"
				continue
			}
			
			// No decryption needed - data is stored in plain text
		}
	}

	log.Printf("ChatService -> DownloadSpreadsheetTableDataWithFilter -> Returning %d columns and %d rows", 
		len(columnNames), len(rows))
	
	return &dtos.SpreadsheetDownloadResponse{
		TableName: tableName,
		Columns:   columnNames,
		Rows:      rows,
	}, http.StatusOK, nil
}

// DeleteSpreadsheetRow deletes a specific row from a spreadsheet table
func (s *chatService) DeleteSpreadsheetRow(userID, chatID, tableName, rowID string) (uint32, error) {
	log.Printf("ChatService -> DeleteSpreadsheetRow -> Starting for chatID: %s, table: %s, row: %s", chatID, tableName, rowID)

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return http.StatusNotFound, fmt.Errorf("connection not found")
	}

	// Get database connection
	conn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to get database connection: %v", err)
	}

	schemaName := connInfo.Config.SchemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", chatID)
	}

	// Delete the row
	deleteQuery := fmt.Sprintf("DELETE FROM %s.%s WHERE _id = $1", schemaName, tableName)
	if err := conn.Exec(deleteQuery, rowID); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete row: %v", err)
	}

	// Trigger schema refresh to update table size and row count
	go func() {
		ctx := context.Background()
		if _, err := s.RefreshSchema(ctx, userID, chatID, false); err != nil {
			log.Printf("ChatService -> DeleteSpreadsheetRow -> Failed to refresh schema: %v", err)
		}
	}()

	return http.StatusOK, nil
}

// sanitizeColumnName removes special characters from column names
func sanitizeColumnName(name string) string {
	// Replace spaces and special characters with underscores
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)
	
	// Remove consecutive underscores
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}
	
	// Trim underscores
	sanitized = strings.Trim(sanitized, "_")
	
	// Ensure it starts with a letter
	if len(sanitized) > 0 && (sanitized[0] >= '0' && sanitized[0] <= '9') {
		sanitized = "col_" + sanitized
	}
	
	// Convert to lowercase
	return strings.ToLower(sanitized)
}

// updateSpreadsheetDatabaseName updates the database name based on uploaded tables
func (s *chatService) updateSpreadsheetDatabaseName(chatID string) error {
	log.Printf("ChatService -> updateSpreadsheetDatabaseName -> CALLED! Starting for chatID: %s", chatID)
	
	// Get chat object
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %v", err)
	}
	
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil || chat == nil {
		return fmt.Errorf("chat not found")
	}
	
	// Only update for spreadsheet connections
	if chat.Connection.Type != constants.DatabaseTypeSpreadsheet {
		return nil
	}
	
	// Get connection info to get schema
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return fmt.Errorf("connection not found")
	}
	
	// Get database connection
	conn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}
	
	schemaName := connInfo.Config.SchemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", chatID)
	}
	
	// Query all tables in the schema
	tableQuery := fmt.Sprintf(`
		SELECT tablename 
		FROM pg_catalog.pg_tables 
		WHERE schemaname = '%s'
		ORDER BY tablename
	`, schemaName)
	
	var tableData []map[string]interface{}
	if err := conn.QueryRows(tableQuery, &tableData); err != nil {
		log.Printf("ChatService -> updateSpreadsheetDatabaseName -> Error getting tables: %v", err)
		return fmt.Errorf("failed to get tables: %v", err)
	}
	
	// Collect table names
	var tableNames []string
	for _, row := range tableData {
		if tableName, ok := row["tablename"].(string); ok {
			tableNames = append(tableNames, tableName)
		}
	}
	
	// Generate database name from table names
	var dbName string
	if len(tableNames) == 0 {
		// Keep the current name if no tables, or use default for new connections
		if chat.Connection.Database != "" && chat.Connection.Database != "spreadsheet_data" {
			dbName = chat.Connection.Database
		} else {
			dbName = "spreadsheet_db"
		}
	} else if len(tableNames) == 1 {
		dbName = tableNames[0]
	} else {
		// Remove common suffixes like _base, _data, _table from table names
		cleanedNames := make([]string, len(tableNames))
		for i, name := range tableNames {
			cleaned := name
			// Remove common suffixes
			suffixes := []string{"_base", "_data", "_table", "_tbl"}
			for _, suffix := range suffixes {
				if strings.HasSuffix(cleaned, suffix) {
					cleaned = strings.TrimSuffix(cleaned, suffix)
					break
				}
			}
			cleanedNames[i] = cleaned
		}
		
		// Join cleaned names
		joined := strings.Join(cleanedNames, "_")
		
		// Limit to 50 characters
		if len(joined) > 50 {
			// Use a smarter approach: take first letters of each word if too long
			if len(cleanedNames) > 2 {
				// For many tables, abbreviate
				abbreviated := ""
				for i, name := range cleanedNames {
					if i > 0 {
						abbreviated += "_"
					}
					// Take first 3-4 characters of each table name
					if len(name) > 4 {
						abbreviated += name[:4]
					} else {
						abbreviated += name
					}
				}
				if len(abbreviated) <= 50 {
					dbName = abbreviated + "_db"
				} else {
					// Fall back to first two tables
					dbName = cleanedNames[0]
					if len(cleanedNames) > 1 {
						dbName += "_" + cleanedNames[1]
					}
					dbName += "_and_more"
				}
			} else {
				// For 2 tables, just use them
				dbName = joined
				if len(dbName) > 50 {
					dbName = dbName[:47] + "..."
				}
			}
		} else {
			dbName = joined
		}
	}
	
	// Update the connection database name
	oldDbName := chat.Connection.Database
	chat.Connection.Database = dbName
	
	log.Printf("ChatService -> updateSpreadsheetDatabaseName -> Updating database name from '%s' to '%s' for tables: %v", oldDbName, dbName, tableNames)
	
	// Save the updated chat
	if err := s.chatRepo.Update(chat.ID, chat); err != nil {
		log.Printf("ChatService -> updateSpreadsheetDatabaseName -> Failed to update chat: %v", err)
		return fmt.Errorf("failed to update chat: %v", err)
	}
	
	log.Printf("ChatService -> updateSpreadsheetDatabaseName -> SUCCESS! Updated database name from '%s' to '%s'", oldDbName, dbName)
	return nil
}