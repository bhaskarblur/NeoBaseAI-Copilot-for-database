package dbmanager

import (
	"database/sql"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"strings"
)

// SpreadsheetProcessor handles processing and storage of spreadsheet data
type SpreadsheetProcessor struct {
	redisRepo interface{}
}

// NewSpreadsheetProcessor creates a new spreadsheet processor
func NewSpreadsheetProcessor(redisRepo interface{}) *SpreadsheetProcessor {
	return &SpreadsheetProcessor{
		redisRepo: redisRepo,
	}
}

// ProcessAndStoreSpreadsheet processes spreadsheet data using robust analyzer
func (p *SpreadsheetProcessor) ProcessAndStoreSpreadsheet(
	db *sql.DB,
	schemaName string,
	baseTableName string,
	data [][]interface{},
	chatID string,
) error {
	if len(data) == 0 {
		return fmt.Errorf("no data to process")
	}

	log.Printf("SpreadsheetProcessor -> Processing spreadsheet with %d rows", len(data))

	// Use robust analyzer
	analyzer := NewRobustSheetAnalyzer(data)
	regions, err := analyzer.AnalyzeRobust()
	if err != nil {
		log.Printf("Warning: Robust analysis failed: %v, falling back to unstructured handling", err)
		
		// Create unstructured fallback
		region := p.createUnstructuredRegion(data)
		regions = []*DataRegion{region}
	}

	if len(regions) == 0 {
		log.Printf("No data regions found, creating unstructured table")
		region := p.createUnstructuredRegion(data)
		regions = []*DataRegion{region}
	}

	// Process each region
	successCount := 0
	allMetadata := make([]*dtos.ImportMetadata, 0)

	for idx, region := range regions {
		// Determine table name
		tableName := baseTableName
		if len(regions) > 1 {
			// Multiple regions detected, append index
			tableName = fmt.Sprintf("%s_%d", baseTableName, idx+1)
		}

		log.Printf("SpreadsheetProcessor -> Processing region %d/%d as table '%s'", 
			idx+1, len(regions), tableName)
		log.Printf("  - Headers: %v", region.Headers)
		log.Printf("  - Rows: %d", len(region.DataRows))
		log.Printf("  - Quality: %.1f%%", region.Quality)

		// Store the data
		if err := p.storeRegionData(db, schemaName, tableName, region); err != nil {
			log.Printf("Warning: Failed to store region %d: %v", idx+1, err)
			continue
		}

		successCount++

		// Create metadata for this region
		metadata := p.createMetadata(tableName, region)
		allMetadata = append(allMetadata, metadata)
	}

	if successCount == 0 {
		return fmt.Errorf("failed to store any data regions")
	}

	// Store combined metadata if we have redis
	if p.redisRepo != nil && chatID != "" {
		p.storeAllMetadata(chatID, allMetadata)
	}

	log.Printf("SpreadsheetProcessor -> Successfully processed %d/%d regions", 
		successCount, len(regions))

	return nil
}

// createUnstructuredRegion creates a fallback region for unstructured data
func (p *SpreadsheetProcessor) createUnstructuredRegion(data [][]interface{}) *DataRegion {
	headers := []string{"row_num", "col_letter", "value"}
	dataRows := make([][]interface{}, 0)

	for rowIdx, row := range data {
		for colIdx, cell := range row {
			if cell != nil && fmt.Sprintf("%v", cell) != "" {
				colLetter := p.indexToColumnLetter(colIdx)
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
		dataRows = append(dataRows, []interface{}{1, "A", ""})
	}

	return &DataRegion{
		Headers:     headers,
		DataRows:    dataRows,
		Quality:     50.0,
		Issues:      []string{"Data is unstructured - stored as cell references"},
		Suggestions: []string{"Consider restructuring data into tabular format"},
	}
}

// storeRegionData stores a data region in the database
func (p *SpreadsheetProcessor) storeRegionData(
	db *sql.DB,
	schemaName string,
	tableName string,
	region *DataRegion,
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
	columns = append(columns, "_quality_score DECIMAL(5,2) DEFAULT 0.00")

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
		colNames = append(colNames, "_quality_score")

		// Batch insert for better performance
		batchSize := 100
		for i := 0; i < len(region.DataRows); i += batchSize {
			end := i + batchSize
			if end > len(region.DataRows) {
				end = len(region.DataRows)
			}

			if err := p.insertBatch(db, schemaName, tableName, colNames, 
				region.DataRows[i:end], region.Quality); err != nil {
				log.Printf("Warning: Failed to insert batch %d-%d: %v", i, end, err)
			}
		}
	}

	// Create indexes for better query performance
	p.createIndexes(db, schemaName, tableName, region.Headers)

	return nil
}

// insertBatch inserts a batch of rows
func (p *SpreadsheetProcessor) insertBatch(
	db *sql.DB,
	schemaName string,
	tableName string,
	columns []string,
	rows [][]interface{},
	quality float64,
) error {
	if len(rows) == 0 {
		return nil
	}

	// Build values for batch insert
	valueStrings := make([]string, 0, len(rows))
	
	for _, row := range rows {
		values := make([]string, 0, len(columns))
		
		// Add data values
		for i := 0; i < len(columns)-1; i++ { // -1 for quality score
			var value string
			if i < len(row) && row[i] != nil {
				valueStr := fmt.Sprintf("%v", row[i])
				// Escape quotes
				value = strings.ReplaceAll(valueStr, "'", "''")
			}
			values = append(values, fmt.Sprintf("'%s'", value))
		}
		
		// Add quality score
		values = append(values, fmt.Sprintf("%.2f", quality))
		
		valueStrings = append(valueStrings, fmt.Sprintf("(%s)", strings.Join(values, ", ")))
	}

	insertQuery := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES %s",
		schemaName, tableName,
		strings.Join(columns, ", "),
		strings.Join(valueStrings, ", "))

	_, err := db.Exec(insertQuery)
	return err
}

// createIndexes creates useful indexes
func (p *SpreadsheetProcessor) createIndexes(
	db *sql.DB,
	schemaName string,
	tableName string,
	headers []string,
) {
	// Create index on first column if it looks like an ID
	if len(headers) > 0 {
		firstCol := sanitizeColumnName(headers[0])
		firstColLower := strings.ToLower(headers[0])
		
		if strings.Contains(firstColLower, "id") || 
		   strings.Contains(firstColLower, "key") ||
		   strings.Contains(firstColLower, "code") {
			indexName := fmt.Sprintf("idx_%s_%s", tableName, firstCol)
			indexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s.%s (%s)",
				indexName, schemaName, tableName, firstCol)
			
			if _, err := db.Exec(indexQuery); err != nil {
				log.Printf("Warning: Failed to create index on %s: %v", firstCol, err)
			}
		}
	}

	// Create index on quality score for filtering
	qualityIndexName := fmt.Sprintf("idx_%s_quality", tableName)
	qualityIndexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s.%s (_quality_score)",
		qualityIndexName, schemaName, tableName)
	
	if _, err := db.Exec(qualityIndexQuery); err != nil {
		log.Printf("Warning: Failed to create quality index: %v", err)
	}
}

// createMetadata creates import metadata for a region
func (p *SpreadsheetProcessor) createMetadata(tableName string, region *DataRegion) *dtos.ImportMetadata {
	metadata := &dtos.ImportMetadata{
		TableName:   tableName,
		RowCount:    len(region.DataRows),
		ColumnCount: len(region.Headers),
		Quality:     region.Quality,
		Issues:      region.Issues,
		Suggestions: region.Suggestions,
		Columns:     make([]dtos.ImportColumnMetadata, 0),
	}

	// Add column metadata
	for _, header := range region.Headers {
		colMeta := dtos.ImportColumnMetadata{
			Name:         sanitizeColumnName(header),
			OriginalName: header,
			DataType:     "text", // Default to text for simplicity
		}
		metadata.Columns = append(metadata.Columns, colMeta)
	}

	return metadata
}

// storeAllMetadata stores combined metadata for all regions
func (p *SpreadsheetProcessor) storeAllMetadata(chatID string, allMetadata []*dtos.ImportMetadata) {
	// Implementation depends on redis interface
	// This is a placeholder
	log.Printf("SpreadsheetProcessor -> Would store metadata for %d tables in chat %s", 
		len(allMetadata), chatID)
}

// indexToColumnLetter converts a column index to Excel-style letter
func (p *SpreadsheetProcessor) indexToColumnLetter(index int) string {
	letter := ""
	for index >= 0 {
		letter = string(rune('A'+index%26)) + letter
		index = index/26 - 1
	}
	return letter
}

// ConvertStringDataToInterface converts string data to interface{} for processing
func ConvertStringDataToInterface(stringData [][]string) [][]interface{} {
	data := make([][]interface{}, len(stringData))
	for i, row := range stringData {
		data[i] = make([]interface{}, len(row))
		for j, cell := range row {
			data[i][j] = cell
		}
	}
	return data
}