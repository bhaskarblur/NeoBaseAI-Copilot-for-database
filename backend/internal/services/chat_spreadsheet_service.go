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
func (s *chatService) StoreSpreadsheetData(userID, chatID, tableName string, columns []string, data [][]string) (*dtos.SpreadsheetUploadResponse, uint32, error) {
	log.Printf("ChatService -> StoreSpreadsheetData -> Starting for chatID: %s, table: %s", chatID, tableName)

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

	// Create table with proper column types
	columnDefs := make([]string, len(columns))
	for i, col := range columns {
		// Sanitize column name
		sanitizedCol := sanitizeColumnName(col)
		// Add internal columns for tracking
		if i == 0 {
			columnDefs[i] = fmt.Sprintf("_id SERIAL PRIMARY KEY, _created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, _updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, %s TEXT", sanitizedCol)
		} else {
			columnDefs[i] = fmt.Sprintf("%s TEXT", sanitizedCol)
		}
	}

	createTableQuery := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s.%s (%s)",
		schemaName,
		tableName,
		strings.Join(columnDefs, ", "),
	)

	// Execute create table
	if err := conn.Exec(createTableQuery); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to create table: %v", err)
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

	// Trigger schema refresh
	go func() {
		ctx := context.Background()
		if _, err := s.RefreshSchema(ctx, userID, chatID, false); err != nil {
			log.Printf("ChatService -> StoreSpreadsheetData -> Failed to refresh schema: %v", err)
		}
	}()

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
	var countData []map[string]interface{}
	if err := conn.QueryRows(countQuery, &countData); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get row count: %v", err)
	}
	if len(countData) > 0 {
		if count, ok := countData[0]["count"].(int64); ok {
			totalRows = count
		} else if count, ok := countData[0]["count"].(float64); ok {
			totalRows = int64(count)
		}
	}

	// Get column information
	var columns []struct {
		ColumnName string `gorm:"column:column_name"`
	}
	colQuery := fmt.Sprintf(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = '%s' AND table_name = '%s' 
		AND column_name NOT LIKE '\_%' 
		ORDER BY ordinal_position
	`, schemaName, tableName)
	var columnData []map[string]interface{}
	if err := conn.QueryRows(colQuery, &columnData); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get columns: %v", err)
	}
	for _, col := range columnData {
		if colName, ok := col["column_name"].(string); ok {
			columns = append(columns, struct {
				ColumnName string `gorm:"column:column_name"`
			}{
				ColumnName: colName,
			})
		}
	}

	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.ColumnName)
	}

	// Get paginated data
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(
		"SELECT %s FROM %s.%s ORDER BY _id LIMIT %d OFFSET %d",
		strings.Join(columnNames, ", "),
		schemaName,
		tableName,
		pageSize,
		offset,
	)

	var rows []map[string]interface{}
	if err := conn.QueryRows(dataQuery, &rows); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get data: %v", err)
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

	// Trigger schema refresh
	go func() {
		ctx := context.Background()
		if _, err := s.RefreshSchema(ctx, userID, chatID, false); err != nil {
			log.Printf("ChatService -> DeleteSpreadsheetTable -> Failed to refresh schema: %v", err)
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

	// Get column information
	var columns []struct {
		ColumnName string `gorm:"column:column_name"`
	}
	colQuery := fmt.Sprintf(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = '%s' AND table_name = '%s' 
		AND column_name NOT LIKE '\_%' 
		ORDER BY ordinal_position
	`, schemaName, tableName)
	var columnData []map[string]interface{}
	if err := conn.QueryRows(colQuery, &columnData); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get columns: %v", err)
	}
	for _, col := range columnData {
		if colName, ok := col["column_name"].(string); ok {
			columns = append(columns, struct {
				ColumnName string `gorm:"column:column_name"`
			}{
				ColumnName: colName,
			})
		}
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

	return &dtos.SpreadsheetDownloadResponse{
		TableName: tableName,
		Columns:   columnNames,
		Rows:      rows,
	}, http.StatusOK, nil
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