package dtos

import "time"

// SpreadsheetUploadResponse represents the response after uploading spreadsheet data
type SpreadsheetUploadResponse struct {
	TableName   string    `json:"table_name"`
	RowCount    int       `json:"row_count"`
	ColumnCount int       `json:"column_count"`
	SizeBytes   int64     `json:"size_bytes"`
	UploadedAt  time.Time `json:"uploaded_at"`
	// Error reporting fields
	TotalRowsProcessed int      `json:"total_rows_processed"`
	SuccessfulRows     int      `json:"successful_rows"`
	FailedRows         int      `json:"failed_rows"`
	Errors             []string `json:"errors,omitempty"`
	HasErrors          bool     `json:"has_errors"`
}

// SpreadsheetTableDataResponse represents paginated table data
type SpreadsheetTableDataResponse struct {
	TableName   string                   `json:"table_name"`
	Columns     []string                 `json:"columns"`
	Rows        []map[string]interface{} `json:"rows"`
	TotalRows   int                      `json:"total_rows"`
	Page        int                      `json:"page"`
	PageSize    int                      `json:"page_size"`
	TotalPages  int                      `json:"total_pages"`
}

// SpreadsheetDownloadResponse represents data for downloading
type SpreadsheetDownloadResponse struct {
	TableName string                   `json:"table_name"`
	Columns   []string                 `json:"columns"`
	Rows      []map[string]interface{} `json:"rows"`
}