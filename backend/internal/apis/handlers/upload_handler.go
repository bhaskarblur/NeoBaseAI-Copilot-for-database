package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"

	"github.com/gin-gonic/gin"

	"github.com/xuri/excelize/v2"
)

type UploadHandler struct {
	chatService services.ChatService
}

func NewUploadHandler(chatService services.ChatService) *UploadHandler {
	return &UploadHandler{
		chatService: chatService,
	}
}

// UploadFile handles CSV/Excel file uploads
func (h *UploadHandler) UploadFile(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("chatID")

	if userID == "" || chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing userID or chatID"})
		return
	}

	// Parse multipart form
	err := c.Request.ParseMultipartForm(100 << 20) // 100 MB limit
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file"})
		return
	}
	defer file.Close()

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".csv" && ext != ".xlsx" && ext != ".xls" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only CSV and Excel files are allowed"})
		return
	}

	// Get table name from form data
	tableName := c.PostForm("tableName")
	if tableName == "" {
		// Generate table name from filename
		tableName = sanitizeTableName(header.Filename)
	}

	// Get merge strategy (default to "replace")
	mergeStrategy := c.DefaultPostForm("mergeStrategy", "replace")
	if mergeStrategy != "replace" && mergeStrategy != "append" && mergeStrategy != "merge" && mergeStrategy != "smart_merge" {
		mergeStrategy = "replace"
	}

	// Get merge options for advanced merge
	mergeOptions := services.MergeOptions{
		Strategy:         mergeStrategy,
		IgnoreCase:       c.DefaultPostForm("ignoreCase", "true") == "true",
		TrimWhitespace:   c.DefaultPostForm("trimWhitespace", "true") == "true",
		HandleNulls:      c.DefaultPostForm("handleNulls", "empty"),
		AddNewCols:       c.DefaultPostForm("addNewColumns", "true") == "true",
		DropMissingCols:  c.DefaultPostForm("dropMissingColumns", "false") == "true",
		UpdateExisting:   c.DefaultPostForm("updateExisting", "true") == "true",
		InsertNew:        c.DefaultPostForm("insertNew", "true") == "true",
		DeleteMissing:    c.DefaultPostForm("deleteMissing", "false") == "true",
	}

	log.Printf("UploadHandler -> Processing file: %s as table: %s", header.Filename, tableName)

	// Process the file based on type
	var data [][]string
	var columns []string

	if ext == ".csv" {
		data, columns, err = h.processCSV(file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to process CSV: %v", err)})
			return
		}
	} else {
		data, columns, err = h.processExcel(file, header.Filename)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to process Excel: %v", err)})
			return
		}
	}

	// Store the data in the spreadsheet database
	result, statusCode, err := h.chatService.StoreSpreadsheetData(userID, chatID, tableName, columns, data, mergeStrategy, mergeOptions)
	if err != nil {
		c.JSON(int(statusCode), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// processCSV reads and processes CSV file
func (h *UploadHandler) processCSV(file io.Reader) ([][]string, []string, error) {
	reader := csv.NewReader(file)

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, nil, fmt.Errorf("CSV file is empty")
	}

	// First row is headers
	columns := records[0]
	data := records[1:]

	return data, columns, nil
}

// processExcel reads and processes Excel file
func (h *UploadHandler) processExcel(file io.Reader, filename string) ([][]string, []string, error) {
	// Read file into memory
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Open Excel file from bytes
	f, err := excelize.OpenReader(strings.NewReader(string(fileBytes)))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get first sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, fmt.Errorf("no sheets found in Excel file")
	}

	sheetName := sheets[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("Excel sheet is empty")
	}

	// First row is headers
	columns := rows[0]
	data := rows[1:]

	return data, columns, nil
}

// GetTableData retrieves data from a spreadsheet table
func (h *UploadHandler) GetTableData(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("chatID")
	tableName := c.Param("tableName")

	if userID == "" || chatID == "" || tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters"})
		return
	}

	// Get pagination parameters
	page := 1
	pageSize := 50

	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if ps := c.Query("pageSize"); ps != "" {
		fmt.Sscanf(ps, "%d", &pageSize)
	}

	result, statusCode, err := h.chatService.GetSpreadsheetTableData(userID, chatID, tableName, page, pageSize)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    result,
	})
}

// DeleteTable deletes a spreadsheet table
func (h *UploadHandler) DeleteTable(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("chatID")
	tableName := c.Param("tableName")

	if userID == "" || chatID == "" || tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters"})
		return
	}

	statusCode, err := h.chatService.DeleteSpreadsheetTable(userID, chatID, tableName)
	if err != nil {
		c.JSON(int(statusCode), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Table deleted successfully"})
}

// DownloadTableData downloads table data as CSV or XLSX
func (h *UploadHandler) DownloadTableData(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("chatID")
	tableName := c.Param("tableName")
	format := c.DefaultQuery("format", "csv")
	rowIDsParam := c.Query("rowIds")

	if userID == "" || chatID == "" || tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters"})
		return
	}

	if format != "csv" && format != "xlsx" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Must be 'csv' or 'xlsx'"})
		return
	}

	// Parse row IDs if provided
	var rowIDs []string
	if rowIDsParam != "" {
		rowIDs = strings.Split(rowIDsParam, ",")
	}

	// Get data for download (all data or filtered by row IDs)
	var data *dtos.SpreadsheetDownloadResponse
	var statusCode uint32
	var err error
	
	if len(rowIDs) > 0 {
		// Get filtered data
		data, statusCode, err = h.chatService.DownloadSpreadsheetTableDataWithFilter(userID, chatID, tableName, rowIDs)
	} else {
		// Get all data
		data, statusCode, err = h.chatService.DownloadSpreadsheetTableData(userID, chatID, tableName)
	}
	
	if err != nil {
		c.JSON(int(statusCode), gin.H{"error": err.Error()})
		return
	}
	
	log.Printf("DownloadTableData -> Got %d columns and %d rows for table %s", 
		len(data.Columns), len(data.Rows), tableName)

	if format == "csv" {
		// Set headers for CSV download
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", tableName))

		// Write CSV
		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// Write headers
		if len(data.Columns) > 0 {
			writer.Write(data.Columns)
		}

		// Write data rows
		for _, row := range data.Rows {
			rowData := make([]string, len(data.Columns))
			for i, col := range data.Columns {
				if val, ok := row[col]; ok {
					rowData[i] = fmt.Sprintf("%v", val)
				}
			}
			writer.Write(rowData)
		}
	} else {
		// Create XLSX file
		f := excelize.NewFile()
		sheetName := "Sheet1"
		f.SetSheetName(f.GetSheetName(0), sheetName)

		// Write headers
		for i, col := range data.Columns {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, col)
		}

		// Write data rows
		for rowIdx, row := range data.Rows {
			for colIdx, col := range data.Columns {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
				if val, ok := row[col]; ok {
					f.SetCellValue(sheetName, cell, val)
				}
			}
		}

		// Set headers for XLSX download
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.xlsx", tableName))

		// Write XLSX to response
		if err := f.Write(c.Writer); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate Excel file"})
			return
		}
	}
}

// DeleteRow deletes a specific row from a spreadsheet table
func (h *UploadHandler) DeleteRow(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("chatID")
	tableName := c.Param("tableName")
	rowID := c.Param("rowID")

	if userID == "" || chatID == "" || tableName == "" || rowID == "" {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   strPtr("Missing required parameters"),
		})
		return
	}

	statusCode, err := h.chatService.DeleteSpreadsheetRow(userID, chatID, tableName, rowID)
	if err != nil {
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   strPtr(err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    gin.H{"message": "Row deleted successfully"},
	})
}

// Helper function to get string pointer
func strPtr(s string) *string {
	return &s
}

// sanitizeTableName removes special characters from filename to create valid table name
func sanitizeTableName(filename string) string {
	// Remove file extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Replace special characters with underscore
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)

	// Remove consecutive underscores
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}

	// Trim underscores from start and end
	name = strings.Trim(name, "_")

	// Convert to lowercase
	name = strings.ToLower(name)

	// Ensure it starts with a letter
	if len(name) > 0 && (name[0] >= '0' && name[0] <= '9') {
		name = "table_" + name
	}

	return name
}
