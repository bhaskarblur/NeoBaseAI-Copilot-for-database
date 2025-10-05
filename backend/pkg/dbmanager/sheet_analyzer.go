package dbmanager

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// DataRegion represents a detected region of data in a sheet
type DataRegion struct {
	StartRow    int
	EndRow      int
	StartCol    int
	EndCol      int
	Headers     []string
	DataRows    [][]interface{}
	Quality     float64
	Issues      []string
	Suggestions []string
}

// ColumnAnalysis contains analysis results for a column
type ColumnAnalysis struct {
	Name           string
	OriginalName   string
	DataType       string
	NullCount      int
	UniqueCount    int
	SampleValues   []interface{}
	HasDuplicates  bool
	IsEmpty        bool
	IsPrimaryKey   bool
}

// SheetAnalyzer provides intelligent analysis of spreadsheet data
type SheetAnalyzer struct {
	data [][]interface{}
}

// NewSheetAnalyzer creates a new analyzer instance
func NewSheetAnalyzer(data [][]interface{}) *SheetAnalyzer {
	return &SheetAnalyzer{data: data}
}

// AnalyzeSheet performs comprehensive analysis of the sheet data
func (sa *SheetAnalyzer) AnalyzeSheet() (*DataRegion, error) {
	if len(sa.data) == 0 {
		return nil, fmt.Errorf("no data to analyze")
	}

	// Find the actual data region (skip empty rows/columns)
	region := sa.findDataRegion()
	
	// Detect headers intelligently
	headers, headerRow := sa.detectHeaders(region)
	region.Headers = headers
	
	// Extract data rows (skip header)
	if headerRow >= 0 && headerRow < len(region.DataRows) {
		region.DataRows = region.DataRows[headerRow+1:]
		region.StartRow = region.StartRow + headerRow + 1
	}
	
	// Analyze quality and detect issues
	region.Quality = sa.calculateQuality(region)
	region.Issues = sa.detectIssues(region)
	region.Suggestions = sa.generateSuggestions(region)
	
	return region, nil
}

// findDataRegion identifies the actual data boundaries, skipping empty rows/columns
func (sa *SheetAnalyzer) findDataRegion() *DataRegion {
	region := &DataRegion{
		StartRow: -1,
		EndRow:   -1,
		StartCol: -1,
		EndCol:   -1,
	}
	
	// Find first non-empty row
	for i, row := range sa.data {
		if sa.isRowNonEmpty(row) {
			region.StartRow = i
			break
		}
	}
	
	if region.StartRow == -1 {
		return region // No data found
	}
	
	// Find last non-empty row
	for i := len(sa.data) - 1; i >= region.StartRow; i-- {
		if sa.isRowNonEmpty(sa.data[i]) {
			region.EndRow = i
			break
		}
	}
	
	// Find column boundaries
	maxCols := 0
	for _, row := range sa.data {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	
	// Find first non-empty column
	for col := 0; col < maxCols; col++ {
		if sa.isColumnNonEmpty(col) {
			region.StartCol = col
			break
		}
	}
	
	// Find last non-empty column
	for col := maxCols - 1; col >= 0; col-- {
		if sa.isColumnNonEmpty(col) {
			region.EndCol = col
			break
		}
	}
	
	// Extract data for the region
	if region.StartRow >= 0 && region.EndRow >= region.StartRow {
		for i := region.StartRow; i <= region.EndRow && i < len(sa.data); i++ {
			row := sa.data[i]
			regionRow := make([]interface{}, 0)
			for j := region.StartCol; j <= region.EndCol && j < len(row); j++ {
				regionRow = append(regionRow, row[j])
			}
			region.DataRows = append(region.DataRows, regionRow)
		}
	}
	
	log.Printf("SheetAnalyzer -> Found data region: rows %d-%d, cols %d-%d", 
		region.StartRow, region.EndRow, region.StartCol, region.EndCol)
	
	return region
}

// detectHeaders intelligently identifies header row(s)
func (sa *SheetAnalyzer) detectHeaders(region *DataRegion) ([]string, int) {
	if len(region.DataRows) == 0 {
		return []string{}, -1
	}
	
	// Score each row for likelihood of being a header
	headerScores := make([]float64, 0)
	maxScore := 0.0
	headerRowIndex := 0
	
	for i := 0; i < len(region.DataRows) && i < 5; i++ { // Check first 5 rows
		score := sa.scoreHeaderRow(region.DataRows[i], region)
		headerScores = append(headerScores, score)
		if score > maxScore {
			maxScore = score
			headerRowIndex = i
		}
	}
	
	// If no good header found, use first row
	if maxScore < 0.5 {
		headerRowIndex = 0
	}
	
	// Extract and clean headers
	headers := make([]string, 0)
	headerCounts := make(map[string]int)
	
	for colIdx, cell := range region.DataRows[headerRowIndex] {
		headerName := sa.cleanHeaderName(cell, colIdx)
		
		// Handle duplicates
		if count, exists := headerCounts[strings.ToLower(headerName)]; exists {
			headerName = fmt.Sprintf("%s_%d", headerName, count+1)
		}
		headerCounts[strings.ToLower(headerName)]++
		
		headers = append(headers, headerName)
	}
	
	log.Printf("SheetAnalyzer -> Detected headers at row %d: %v", headerRowIndex, headers)
	
	return headers, headerRowIndex
}

// scoreHeaderRow calculates likelihood that a row is a header
func (sa *SheetAnalyzer) scoreHeaderRow(row []interface{}, region *DataRegion) float64 {
	score := 0.0
	nonEmptyCount := 0
	
	for _, cell := range row {
		cellStr := fmt.Sprintf("%v", cell)
		if cellStr != "" {
			nonEmptyCount++
			
			// Check for header-like characteristics
			if sa.looksLikeHeader(cellStr) {
				score += 0.3
			}
			
			// Penalize if it looks like data
			if sa.looksLikeData(cellStr) {
				score -= 0.2
			}
		}
	}
	
	// Bonus if all cells are non-empty (headers usually are)
	if nonEmptyCount == len(row) && nonEmptyCount > 0 {
		score += 0.2
	}
	
	// Normalize score
	if nonEmptyCount > 0 {
		score = score / float64(nonEmptyCount)
	}
	
	return score
}

// looksLikeHeader checks if a string looks like a column header
func (sa *SheetAnalyzer) looksLikeHeader(s string) bool {
	// Headers often contain these patterns
	headerPatterns := []string{
		"name", "id", "email", "phone", "date", "time", "number", "count",
		"total", "amount", "price", "cost", "value", "type", "status",
		"description", "address", "city", "state", "country", "code",
	}
	
	sLower := strings.ToLower(s)
	for _, pattern := range headerPatterns {
		if strings.Contains(sLower, pattern) {
			return true
		}
	}
	
	// Headers are often short and contain underscores or spaces
	if len(s) < 50 && (strings.Contains(s, " ") || strings.Contains(s, "_")) {
		return true
	}
	
	return false
}

// looksLikeData checks if a string looks like data rather than a header
func (sa *SheetAnalyzer) looksLikeData(s string) bool {
	// Check if it's a number
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return true // It's a date
	}
	
	// Check for email pattern
	if regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(s) {
		return true
	}
	
	// Check for URL pattern
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return true
	}
	
	// Long text is usually data
	if len(s) > 100 {
		return true
	}
	
	return false
}

// cleanHeaderName generates a clean column name from a header cell
func (sa *SheetAnalyzer) cleanHeaderName(cell interface{}, colIndex int) string {
	cellStr := fmt.Sprintf("%v", cell)
	
	// If empty, generate a name based on column index
	if cellStr == "" || cellStr == "<nil>" {
		return fmt.Sprintf("column_%d", colIndex+1)
	}
	
	// Clean the string
	cleaned := strings.TrimSpace(cellStr)
	
	// Replace special characters with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	cleaned = re.ReplaceAllString(cleaned, "_")
	
	// Remove consecutive underscores
	re = regexp.MustCompile(`_+`)
	cleaned = re.ReplaceAllString(cleaned, "_")
	
	// Trim underscores
	cleaned = strings.Trim(cleaned, "_")
	
	// Convert to lowercase for consistency
	cleaned = strings.ToLower(cleaned)
	
	// Ensure it starts with a letter
	if len(cleaned) > 0 && (cleaned[0] < 'a' || cleaned[0] > 'z') {
		cleaned = "col_" + cleaned
	}
	
	// If still empty, use column index
	if cleaned == "" {
		cleaned = fmt.Sprintf("column_%d", colIndex+1)
	}
	
	return cleaned
}

// isRowNonEmpty checks if a row contains any non-empty cells
func (sa *SheetAnalyzer) isRowNonEmpty(row []interface{}) bool {
	for _, cell := range row {
		if cell != nil && fmt.Sprintf("%v", cell) != "" {
			return true
		}
	}
	return false
}

// isColumnNonEmpty checks if a column contains any non-empty cells
func (sa *SheetAnalyzer) isColumnNonEmpty(colIndex int) bool {
	for _, row := range sa.data {
		if colIndex < len(row) {
			if row[colIndex] != nil && fmt.Sprintf("%v", row[colIndex]) != "" {
				return true
			}
		}
	}
	return false
}

// calculateQuality scores the overall data quality
func (sa *SheetAnalyzer) calculateQuality(region *DataRegion) float64 {
	if len(region.DataRows) == 0 {
		return 0.0
	}
	
	quality := 100.0
	
	// Penalize for empty cells
	emptyCells := 0
	totalCells := len(region.DataRows) * len(region.Headers)
	for _, row := range region.DataRows {
		for _, cell := range row {
			if cell == nil || fmt.Sprintf("%v", cell) == "" {
				emptyCells++
			}
		}
	}
	if totalCells > 0 {
		emptyRatio := float64(emptyCells) / float64(totalCells)
		quality -= emptyRatio * 30 // Up to 30 points for empty cells
	}
	
	// Penalize for duplicate headers
	uniqueHeaders := make(map[string]bool)
	for _, header := range region.Headers {
		uniqueHeaders[header] = true
	}
	if len(uniqueHeaders) < len(region.Headers) {
		quality -= 20 // Duplicate headers
	}
	
	// Penalize for too many columns
	if len(region.Headers) > 50 {
		quality -= 10
	}
	
	// Penalize for no data rows
	if len(region.DataRows) == 0 {
		quality -= 50
	}
	
	if quality < 0 {
		quality = 0
	}
	
	return quality
}

// detectIssues identifies potential problems with the data
func (sa *SheetAnalyzer) detectIssues(region *DataRegion) []string {
	issues := []string{}
	
	// Check for duplicate headers
	headerMap := make(map[string]int)
	for _, header := range region.Headers {
		headerMap[header]++
	}
	for header, count := range headerMap {
		if count > 1 {
			issues = append(issues, fmt.Sprintf("Duplicate column name: %s (appears %d times)", header, count))
		}
	}
	
	// Check for empty columns
	for i, header := range region.Headers {
		isEmpty := true
		for _, row := range region.DataRows {
			if i < len(row) && row[i] != nil && fmt.Sprintf("%v", row[i]) != "" {
				isEmpty = false
				break
			}
		}
		if isEmpty {
			issues = append(issues, fmt.Sprintf("Column '%s' is completely empty", header))
		}
	}
	
	// Check for inconsistent row lengths
	if len(region.DataRows) > 0 {
		expectedLen := len(region.Headers)
		for i, row := range region.DataRows {
			if len(row) != expectedLen {
				issues = append(issues, fmt.Sprintf("Row %d has %d columns, expected %d", i+1, len(row), expectedLen))
				break // Only report first occurrence
			}
		}
	}
	
	// Check for potential formula errors
	for _, row := range region.DataRows {
		for _, cell := range row {
			cellStr := fmt.Sprintf("%v", cell)
			if strings.HasPrefix(cellStr, "#") && (strings.Contains(cellStr, "ERROR") || 
				strings.Contains(cellStr, "REF") || strings.Contains(cellStr, "DIV")) {
				issues = append(issues, "Sheet contains formula errors")
				break
			}
		}
	}
	
	return issues
}

// generateSuggestions provides improvement recommendations
func (sa *SheetAnalyzer) generateSuggestions(region *DataRegion) []string {
	suggestions := []string{}
	
	// Suggest removing empty columns
	for i, header := range region.Headers {
		isEmpty := true
		for _, row := range region.DataRows {
			if i < len(row) && row[i] != nil && fmt.Sprintf("%v", row[i]) != "" {
				isEmpty = false
				break
			}
		}
		if isEmpty {
			suggestions = append(suggestions, fmt.Sprintf("Consider removing empty column '%s'", header))
		}
	}
	
	// Suggest better column names
	for _, header := range region.Headers {
		if strings.HasPrefix(header, "column_") {
			suggestions = append(suggestions, "Add meaningful header names to improve query clarity")
			break
		}
	}
	
	// Suggest data structure improvements
	if len(region.Headers) > 20 {
		suggestions = append(suggestions, "Consider splitting wide table into multiple related tables")
	}
	
	return suggestions
}

// AnalyzeColumns performs detailed analysis of each column
func (sa *SheetAnalyzer) AnalyzeColumns(region *DataRegion) []ColumnAnalysis {
	analyses := make([]ColumnAnalysis, 0)
	
	for colIdx, header := range region.Headers {
		analysis := ColumnAnalysis{
			Name:         header,
			OriginalName: header,
			SampleValues: make([]interface{}, 0),
		}
		
		// Collect column data
		values := make([]interface{}, 0)
		uniqueValues := make(map[string]bool)
		
		for _, row := range region.DataRows {
			if colIdx < len(row) {
				value := row[colIdx]
				values = append(values, value)
				
				if value == nil || fmt.Sprintf("%v", value) == "" {
					analysis.NullCount++
				} else {
					uniqueValues[fmt.Sprintf("%v", value)] = true
					if len(analysis.SampleValues) < 5 {
						analysis.SampleValues = append(analysis.SampleValues, value)
					}
				}
			}
		}
		
		analysis.UniqueCount = len(uniqueValues)
		analysis.IsEmpty = analysis.NullCount == len(values)
		analysis.HasDuplicates = analysis.UniqueCount < len(values)-analysis.NullCount
		
		// Infer data type
		analysis.DataType = sa.inferColumnType(values)
		
		// Check if could be primary key
		analysis.IsPrimaryKey = analysis.UniqueCount == len(values) && analysis.NullCount == 0
		
		analyses = append(analyses, analysis)
	}
	
	return analyses
}

// inferColumnType attempts to determine the data type of a column
func (sa *SheetAnalyzer) inferColumnType(values []interface{}) string {
	if len(values) == 0 {
		return "text"
	}
	
	// Count different types
	intCount := 0
	floatCount := 0
	dateCount := 0
	boolCount := 0
	emailCount := 0
	urlCount := 0
	textCount := 0
	
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	urlRegex := regexp.MustCompile(`^https?://`)
	
	for _, value := range values {
		if value == nil {
			continue
		}
		
		valueStr := fmt.Sprintf("%v", value)
		if valueStr == "" {
			continue
		}
		
		// Check for boolean
		if valueStr == "true" || valueStr == "false" || valueStr == "TRUE" || valueStr == "FALSE" {
			boolCount++
			continue
		}
		
		// Check for email
		if emailRegex.MatchString(valueStr) {
			emailCount++
			continue
		}
		
		// Check for URL
		if urlRegex.MatchString(valueStr) {
			urlCount++
			continue
		}
		
		// Check for date
		if _, err := time.Parse("2006-01-02", valueStr); err == nil {
			dateCount++
			continue
		}
		if _, err := time.Parse("01/02/2006", valueStr); err == nil {
			dateCount++
			continue
		}
		
		// Check for number
		if strings.Contains(valueStr, ".") {
			floatCount++
		} else if regexp.MustCompile(`^-?\d+$`).MatchString(valueStr) {
			intCount++
		} else {
			textCount++
		}
	}
	
	// Determine predominant type
	total := intCount + floatCount + dateCount + boolCount + emailCount + urlCount + textCount
	if total == 0 {
		return "text"
	}
	
	// Use thresholds for type determination
	threshold := float64(total) * 0.8
	
	if float64(emailCount) > threshold {
		return "email"
	}
	if float64(urlCount) > threshold {
		return "url"
	}
	if float64(dateCount) > threshold {
		return "date"
	}
	if float64(boolCount) > threshold {
		return "boolean"
	}
	if float64(floatCount) > threshold {
		return "decimal"
	}
	if float64(intCount) > threshold {
		return "integer"
	}
	
	return "text"
}