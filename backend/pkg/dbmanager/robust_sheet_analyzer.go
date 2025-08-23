package dbmanager

import (
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// RobustSheetAnalyzer provides advanced analysis for any kind of spreadsheet data
type RobustSheetAnalyzer struct {
	data          [][]interface{}
	config        AnalyzerConfig
	detectedAreas []DataArea
}

// AnalyzerConfig contains configuration for the analyzer
type AnalyzerConfig struct {
	MinDataDensity      float64 // Minimum density to consider area as data (0-1)
	MaxEmptyRowsAllowed int     // Max consecutive empty rows before splitting
	MaxEmptyColsAllowed int     // Max consecutive empty cols before splitting
	AutoDetectHeaders   bool    // Automatically detect headers
	HandleMergedCells   bool    // Handle merged cells
	DetectMultipleTables bool   // Detect multiple tables in one sheet
}

// DataArea represents a detected data area in the sheet
type DataArea struct {
	StartRow     int
	EndRow       int
	StartCol     int
	EndCol       int
	Headers      []string
	DataRows     [][]interface{}
	AreaType     string // "structured", "unstructured", "pivot", "matrix"
	Confidence   float64
	TableName    string
}

// NewRobustSheetAnalyzer creates a new robust analyzer with default config
func NewRobustSheetAnalyzer(data [][]interface{}) *RobustSheetAnalyzer {
	return &RobustSheetAnalyzer{
		data: data,
		config: AnalyzerConfig{
			MinDataDensity:       0.1,  // At least 10% cells should have data
			MaxEmptyRowsAllowed:  3,
			MaxEmptyColsAllowed:  3,
			AutoDetectHeaders:    true,
			HandleMergedCells:    true,
			DetectMultipleTables: true,
		},
	}
}

// AnalyzeRobust performs comprehensive analysis handling any sheet format
func (rsa *RobustSheetAnalyzer) AnalyzeRobust() ([]*DataRegion, error) {
	if len(rsa.data) == 0 {
		return nil, fmt.Errorf("no data to analyze")
	}

	log.Printf("RobustSheetAnalyzer -> Starting analysis of %d rows", len(rsa.data))

	// Step 1: Detect all data areas (could be multiple tables)
	areas := rsa.detectDataAreas()
	
	// Step 2: Analyze each area independently
	regions := make([]*DataRegion, 0)
	
	for i, area := range areas {
		log.Printf("RobustSheetAnalyzer -> Analyzing area %d: rows %d-%d, cols %d-%d", 
			i+1, area.StartRow, area.EndRow, area.StartCol, area.EndCol)
		
		region := rsa.analyzeDataArea(area)
		if region != nil {
			regions = append(regions, region)
		}
	}
	
	// Step 3: If no structured data found, treat entire sheet as unstructured
	if len(regions) == 0 {
		log.Printf("RobustSheetAnalyzer -> No structured data found, treating as unstructured")
		region := rsa.handleUnstructuredData()
		if region != nil {
			regions = append(regions, region)
		}
	}
	
	return regions, nil
}

// detectDataAreas finds all data areas in the sheet
func (rsa *RobustSheetAnalyzer) detectDataAreas() []DataArea {
	areas := make([]DataArea, 0)
	visited := make(map[string]bool)
	
	for rowIdx := 0; rowIdx < len(rsa.data); rowIdx++ {
		for colIdx := 0; colIdx < rsa.getMaxCols(); colIdx++ {
			key := fmt.Sprintf("%d-%d", rowIdx, colIdx)
			if visited[key] {
				continue
			}
			
			if rsa.hasDataAt(rowIdx, colIdx) {
				// Found data, expand to find the full area
				area := rsa.expandDataArea(rowIdx, colIdx, visited)
				if rsa.isValidDataArea(area) {
					areas = append(areas, area)
				}
			}
		}
	}
	
	// Merge overlapping areas
	areas = rsa.mergeOverlappingAreas(areas)
	
	return areas
}

// expandDataArea expands from a starting point to find the full data area
func (rsa *RobustSheetAnalyzer) expandDataArea(startRow, startCol int, visited map[string]bool) DataArea {
	area := DataArea{
		StartRow: startRow,
		EndRow:   startRow,
		StartCol: startCol,
		EndCol:   startCol,
	}
	
	// Use flood-fill algorithm to find connected data
	queue := [][]int{{startRow, startCol}}
	
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		
		row, col := current[0], current[1]
		key := fmt.Sprintf("%d-%d", row, col)
		
		if visited[key] {
			continue
		}
		visited[key] = true
		
		// Update area bounds
		if row < area.StartRow {
			area.StartRow = row
		}
		if row > area.EndRow {
			area.EndRow = row
		}
		if col < area.StartCol {
			area.StartCol = col
		}
		if col > area.EndCol {
			area.EndCol = col
		}
		
		// Check adjacent cells (with tolerance for empty cells)
		directions := [][]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
		for _, dir := range directions {
			newRow, newCol := row+dir[0], col+dir[1]
			if rsa.shouldIncludeInArea(newRow, newCol, area) {
				queue = append(queue, []int{newRow, newCol})
			}
		}
	}
	
	return area
}

// shouldIncludeInArea checks if a cell should be included in the current area
func (rsa *RobustSheetAnalyzer) shouldIncludeInArea(row, col int, currentArea DataArea) bool {
	if row < 0 || row >= len(rsa.data) || col < 0 || col >= rsa.getMaxCols() {
		return false
	}
	
	// Allow some empty cells within the area
	emptyRowGap := 0
	emptyColGap := 0
	
	// Check row gap
	if row < currentArea.StartRow {
		for r := row + 1; r < currentArea.StartRow; r++ {
			if !rsa.isRowEmpty(r, currentArea.StartCol, currentArea.EndCol) {
				break
			}
			emptyRowGap++
		}
	} else if row > currentArea.EndRow {
		for r := currentArea.EndRow + 1; r < row; r++ {
			if !rsa.isRowEmpty(r, currentArea.StartCol, currentArea.EndCol) {
				break
			}
			emptyRowGap++
		}
	}
	
	// Check column gap
	if col < currentArea.StartCol {
		for c := col + 1; c < currentArea.StartCol; c++ {
			if !rsa.isColumnEmpty(c, currentArea.StartRow, currentArea.EndRow) {
				break
			}
			emptyColGap++
		}
	} else if col > currentArea.EndCol {
		for c := currentArea.EndCol + 1; c < col; c++ {
			if !rsa.isColumnEmpty(c, currentArea.StartRow, currentArea.EndRow) {
				break
			}
			emptyColGap++
		}
	}
	
	// Include if gaps are within tolerance
	return emptyRowGap <= rsa.config.MaxEmptyRowsAllowed && 
	       emptyColGap <= rsa.config.MaxEmptyColsAllowed &&
	       rsa.hasDataAt(row, col)
}

// analyzeDataArea analyzes a specific data area
func (rsa *RobustSheetAnalyzer) analyzeDataArea(area DataArea) *DataRegion {
	// Extract data for this area
	areaData := rsa.extractAreaData(area)
	if len(areaData) == 0 {
		return nil
	}
	
	// Determine area type
	areaType := rsa.determineAreaType(areaData)
	
	var region *DataRegion
	
	switch areaType {
	case "structured":
		region = rsa.handleStructuredData(areaData, area)
	case "pivot":
		region = rsa.handlePivotTable(areaData, area)
	case "matrix":
		region = rsa.handleMatrixData(areaData, area)
	default:
		region = rsa.handleSemiStructuredData(areaData, area)
	}
	
	if region != nil {
		region.StartRow = area.StartRow
		region.EndRow = area.EndRow
		region.StartCol = area.StartCol
		region.EndCol = area.EndCol
	}
	
	return region
}

// handleStructuredData processes well-structured tabular data
func (rsa *RobustSheetAnalyzer) handleStructuredData(data [][]interface{}, area DataArea) *DataRegion {
	// Find headers using multiple strategies
	headers, headerRow := rsa.findBestHeaders(data)
	
	// Extract data rows
	dataRows := make([][]interface{}, 0)
	if headerRow >= 0 && headerRow < len(data)-1 {
		dataRows = data[headerRow+1:]
	} else if headerRow == -1 && len(data) > 0 {
		// No clear headers, generate them
		headers = rsa.generateHeaders(len(data[0]))
		dataRows = data
	}
	
	// Ensure all rows have same length as headers
	normalizedRows := rsa.normalizeRows(dataRows, len(headers))
	
	region := &DataRegion{
		Headers:  headers,
		DataRows: normalizedRows,
		Quality:  rsa.calculateDataQuality(normalizedRows, headers),
	}
	
	// Add analysis
	region.Issues = rsa.detectDataIssues(region)
	region.Suggestions = rsa.generateSuggestions(region)
	
	return region
}

// handlePivotTable handles pivot table format
func (rsa *RobustSheetAnalyzer) handlePivotTable(data [][]interface{}, area DataArea) *DataRegion {
	// Pivot tables have row headers and column headers
	// Convert to regular table format
	
	if len(data) < 2 || len(data[0]) < 2 {
		return rsa.handleSemiStructuredData(data, area)
	}
	
	// First row contains column headers (except first cell)
	colHeaders := make([]string, 0)
	for i := 1; i < len(data[0]); i++ {
		colHeaders = append(colHeaders, rsa.cellToString(data[0][i], i))
	}
	
	// First column contains row headers
	rowHeaders := make([]string, 0)
	for i := 1; i < len(data); i++ {
		if len(data[i]) > 0 {
			rowHeaders = append(rowHeaders, rsa.cellToString(data[i][0], 0))
		}
	}
	
	// Create flattened structure
	headers := []string{"row_label"}
	headers = append(headers, colHeaders...)
	
	dataRows := make([][]interface{}, 0)
	for i := 1; i < len(data); i++ {
		if len(data[i]) > 0 {
			row := []interface{}{data[i][0]}
			for j := 1; j < len(data[i]); j++ {
				row = append(row, data[i][j])
			}
			dataRows = append(dataRows, row)
		}
	}
	
	return &DataRegion{
		Headers:     headers,
		DataRows:    dataRows,
		Quality:     80.0,
		Issues:      []string{"Data appears to be in pivot table format - converted to regular table"},
		Suggestions: []string{"Consider using the original pivot structure for analysis"},
	}
}

// handleMatrixData handles matrix/grid format data
func (rsa *RobustSheetAnalyzer) handleMatrixData(data [][]interface{}, area DataArea) *DataRegion {
	// Matrix data doesn't have clear headers
	// Generate column names based on position
	
	maxCols := 0
	for _, row := range data {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	
	headers := make([]string, maxCols)
	for i := 0; i < maxCols; i++ {
		headers[i] = fmt.Sprintf("col_%d", i+1)
	}
	
	// Normalize all rows
	normalizedRows := rsa.normalizeRows(data, maxCols)
	
	return &DataRegion{
		Headers:     headers,
		DataRows:    normalizedRows,
		Quality:     70.0,
		Issues:      []string{"Data appears to be in matrix format without headers"},
		Suggestions: []string{"Consider adding meaningful column headers"},
	}
}

// handleSemiStructuredData handles partially structured or irregular data
func (rsa *RobustSheetAnalyzer) handleSemiStructuredData(data [][]interface{}, area DataArea) *DataRegion {
	// Try to find any pattern in the data
	patterns := rsa.detectDataPatterns(data)
	
	var headers []string
	var dataRows [][]interface{}
	
	if patterns["key_value"] {
		// Handle key-value pairs
		headers = []string{"key", "value"}
		dataRows = rsa.extractKeyValuePairs(data)
	} else if patterns["list"] {
		// Handle list format
		headers = []string{"item"}
		dataRows = rsa.extractListItems(data)
	} else {
		// Fall back to treating each unique position as a column
		headers, dataRows = rsa.extractPositionalData(data)
	}
	
	return &DataRegion{
		Headers:     headers,
		DataRows:    dataRows,
		Quality:     60.0,
		Issues:      []string{"Data structure is irregular or semi-structured"},
		Suggestions: []string{"Consider restructuring data into a regular table format"},
	}
}

// handleUnstructuredData handles completely unstructured data
func (rsa *RobustSheetAnalyzer) handleUnstructuredData() *DataRegion {
	// Find all non-empty cells and create a simple structure
	headers := []string{"row_num", "col_num", "value"}
	dataRows := make([][]interface{}, 0)
	
	for rowIdx, row := range rsa.data {
		for colIdx, cell := range row {
			if cell != nil && fmt.Sprintf("%v", cell) != "" {
				dataRows = append(dataRows, []interface{}{
					rowIdx + 1,
					rsa.columnIndexToName(colIdx),
					cell,
				})
			}
		}
	}
	
	if len(dataRows) == 0 {
		// Create at least one row to avoid empty table
		dataRows = append(dataRows, []interface{}{1, "A", "No data found"})
	}
	
	return &DataRegion{
		Headers:     headers,
		DataRows:    dataRows,
		Quality:     50.0,
		Issues:      []string{"Sheet contains unstructured data - converted to cell reference format"},
		Suggestions: []string{"Original data preserved with row and column references"},
	}
}

// findBestHeaders uses multiple strategies to find the best headers
func (rsa *RobustSheetAnalyzer) findBestHeaders(data [][]interface{}) ([]string, int) {
	if len(data) == 0 {
		return []string{}, -1
	}
	
	strategies := []func([][]interface{}) ([]string, int, float64){
		rsa.strategyFirstNonEmpty,
		rsa.strategyMostText,
		rsa.strategyUniqueValues,
		rsa.strategyPatternMatch,
	}
	
	bestHeaders := []string{}
	bestRow := -1
	bestScore := 0.0
	
	for _, strategy := range strategies {
		headers, row, score := strategy(data)
		if score > bestScore {
			bestHeaders = headers
			bestRow = row
			bestScore = score
		}
	}
	
	// If no good headers found, use first row or generate
	if bestScore < 0.3 {
		if len(data) > 0 && rsa.rowHasData(data[0]) {
			bestHeaders = rsa.rowToHeaders(data[0])
			bestRow = 0
		} else {
			bestHeaders = rsa.generateHeaders(rsa.getMaxColsInData(data))
			bestRow = -1
		}
	}
	
	return bestHeaders, bestRow
}

// Strategy functions for header detection
func (rsa *RobustSheetAnalyzer) strategyFirstNonEmpty(data [][]interface{}) ([]string, int, float64) {
	for i, row := range data {
		if rsa.rowHasData(row) {
			headers := rsa.rowToHeaders(row)
			score := rsa.scoreAsHeaders(headers, data, i)
			return headers, i, score
		}
	}
	return []string{}, -1, 0.0
}

func (rsa *RobustSheetAnalyzer) strategyMostText(data [][]interface{}) ([]string, int, float64) {
	if len(data) == 0 {
		return []string{}, -1, 0.0
	}
	
	maxTextRow := 0
	maxTextCount := 0
	
	// Check first 5 rows
	limit := 5
	if len(data) < limit {
		limit = len(data)
	}
	
	for i := 0; i < limit; i++ {
		textCount := 0
		for _, cell := range data[i] {
			if rsa.isTextCell(cell) {
				textCount++
			}
		}
		if textCount > maxTextCount {
			maxTextCount = textCount
			maxTextRow = i
		}
	}
	
	if maxTextCount > 0 {
		headers := rsa.rowToHeaders(data[maxTextRow])
		score := rsa.scoreAsHeaders(headers, data, maxTextRow)
		return headers, maxTextRow, score
	}
	
	return []string{}, -1, 0.0
}

func (rsa *RobustSheetAnalyzer) strategyUniqueValues(data [][]interface{}) ([]string, int, float64) {
	if len(data) < 2 {
		return []string{}, -1, 0.0
	}
	
	// Check first few rows for uniqueness
	limit := 3
	if len(data) < limit {
		limit = len(data)
	}
	
	bestRow := 0
	bestUniqueness := 0.0
	
	for i := 0; i < limit; i++ {
		uniqueness := rsa.calculateUniqueness(data[i])
		if uniqueness > bestUniqueness {
			bestUniqueness = uniqueness
			bestRow = i
		}
	}
	
	if bestUniqueness > 0.5 {
		headers := rsa.rowToHeaders(data[bestRow])
		score := rsa.scoreAsHeaders(headers, data, bestRow)
		return headers, bestRow, score
	}
	
	return []string{}, -1, 0.0
}

func (rsa *RobustSheetAnalyzer) strategyPatternMatch(data [][]interface{}) ([]string, int, float64) {
	commonHeaders := []string{
		"id", "name", "email", "phone", "date", "time", "amount", "price",
		"quantity", "status", "type", "category", "description", "address",
		"city", "state", "country", "code", "number", "value", "total",
	}
	
	bestRow := -1
	bestScore := 0.0
	
	limit := 5
	if len(data) < limit {
		limit = len(data)
	}
	
	for i := 0; i < limit; i++ {
		score := 0.0
		count := 0
		
		for _, cell := range data[i] {
			cellStr := strings.ToLower(rsa.cellToString(cell, 0))
			for _, pattern := range commonHeaders {
				if strings.Contains(cellStr, pattern) {
					score += 1.0
					break
				}
			}
			count++
		}
		
		if count > 0 {
			normalizedScore := score / float64(count)
			if normalizedScore > bestScore {
				bestScore = normalizedScore
				bestRow = i
			}
		}
	}
	
	if bestRow >= 0 && bestScore > 0.2 {
		headers := rsa.rowToHeaders(data[bestRow])
		return headers, bestRow, bestScore
	}
	
	return []string{}, -1, 0.0
}

// Helper functions
func (rsa *RobustSheetAnalyzer) hasDataAt(row, col int) bool {
	if row >= len(rsa.data) || col >= len(rsa.data[row]) {
		return false
	}
	cell := rsa.data[row][col]
	return cell != nil && fmt.Sprintf("%v", cell) != ""
}

func (rsa *RobustSheetAnalyzer) getMaxCols() int {
	maxCols := 0
	for _, row := range rsa.data {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	return maxCols
}

func (rsa *RobustSheetAnalyzer) getMaxColsInData(data [][]interface{}) int {
	maxCols := 0
	for _, row := range data {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	return maxCols
}

func (rsa *RobustSheetAnalyzer) isRowEmpty(row, startCol, endCol int) bool {
	if row >= len(rsa.data) {
		return true
	}
	for col := startCol; col <= endCol && col < len(rsa.data[row]); col++ {
		if rsa.hasDataAt(row, col) {
			return false
		}
	}
	return true
}

func (rsa *RobustSheetAnalyzer) isColumnEmpty(col, startRow, endRow int) bool {
	for row := startRow; row <= endRow && row < len(rsa.data); row++ {
		if rsa.hasDataAt(row, col) {
			return false
		}
	}
	return true
}

func (rsa *RobustSheetAnalyzer) isValidDataArea(area DataArea) bool {
	// Calculate density
	totalCells := (area.EndRow - area.StartRow + 1) * (area.EndCol - area.StartCol + 1)
	filledCells := 0
	
	for row := area.StartRow; row <= area.EndRow && row < len(rsa.data); row++ {
		for col := area.StartCol; col <= area.EndCol && col < len(rsa.data[row]); col++ {
			if rsa.hasDataAt(row, col) {
				filledCells++
			}
		}
	}
	
	if totalCells == 0 {
		return false
	}
	
	density := float64(filledCells) / float64(totalCells)
	return density >= rsa.config.MinDataDensity && filledCells > 0
}

func (rsa *RobustSheetAnalyzer) mergeOverlappingAreas(areas []DataArea) []DataArea {
	if len(areas) <= 1 {
		return areas
	}
	
	merged := make([]DataArea, 0)
	used := make(map[int]bool)
	
	for i := 0; i < len(areas); i++ {
		if used[i] {
			continue
		}
		
		current := areas[i]
		
		for j := i + 1; j < len(areas); j++ {
			if used[j] {
				continue
			}
			
			if rsa.areasOverlap(current, areas[j]) {
				// Merge areas
				current = rsa.mergeAreas(current, areas[j])
				used[j] = true
			}
		}
		
		merged = append(merged, current)
	}
	
	return merged
}

func (rsa *RobustSheetAnalyzer) areasOverlap(a1, a2 DataArea) bool {
	return !(a1.EndRow < a2.StartRow || a2.EndRow < a1.StartRow ||
		a1.EndCol < a2.StartCol || a2.EndCol < a1.StartCol)
}

func (rsa *RobustSheetAnalyzer) mergeAreas(a1, a2 DataArea) DataArea {
	return DataArea{
		StartRow: min(a1.StartRow, a2.StartRow),
		EndRow:   max(a1.EndRow, a2.EndRow),
		StartCol: min(a1.StartCol, a2.StartCol),
		EndCol:   max(a1.EndCol, a2.EndCol),
	}
}

func (rsa *RobustSheetAnalyzer) extractAreaData(area DataArea) [][]interface{} {
	data := make([][]interface{}, 0)
	
	for row := area.StartRow; row <= area.EndRow && row < len(rsa.data); row++ {
		rowData := make([]interface{}, 0)
		for col := area.StartCol; col <= area.EndCol; col++ {
			if col < len(rsa.data[row]) {
				rowData = append(rowData, rsa.data[row][col])
			} else {
				rowData = append(rowData, nil)
			}
		}
		data = append(data, rowData)
	}
	
	return data
}

func (rsa *RobustSheetAnalyzer) determineAreaType(data [][]interface{}) string {
	if len(data) == 0 {
		return "empty"
	}
	
	// Check for pivot table characteristics
	if rsa.looksLikePivot(data) {
		return "pivot"
	}
	
	// Check for structured table
	if rsa.looksLikeStructured(data) {
		return "structured"
	}
	
	// Check for matrix
	if rsa.looksLikeMatrix(data) {
		return "matrix"
	}
	
	return "unstructured"
}

func (rsa *RobustSheetAnalyzer) looksLikePivot(data [][]interface{}) bool {
	if len(data) < 2 || len(data[0]) < 2 {
		return false
	}
	
	// Pivot tables often have empty top-left cell
	topLeftEmpty := data[0][0] == nil || fmt.Sprintf("%v", data[0][0]) == ""
	
	// Has headers in both first row and first column
	hasRowHeaders := true
	hasColHeaders := true
	
	for i := 1; i < len(data) && i < 5; i++ {
		if len(data[i]) > 0 && !rsa.isTextCell(data[i][0]) {
			hasRowHeaders = false
			break
		}
	}
	
	for j := 1; j < len(data[0]) && j < 5; j++ {
		if !rsa.isTextCell(data[0][j]) {
			hasColHeaders = false
			break
		}
	}
	
	return topLeftEmpty && hasRowHeaders && hasColHeaders
}

func (rsa *RobustSheetAnalyzer) looksLikeStructured(data [][]interface{}) bool {
	if len(data) < 2 {
		return false
	}
	
	// Check if rows have consistent column count
	firstRowLen := len(data[0])
	consistentCount := 0
	
	for _, row := range data {
		if len(row) == firstRowLen {
			consistentCount++
		}
	}
	
	consistency := float64(consistentCount) / float64(len(data))
	return consistency > 0.7
}

func (rsa *RobustSheetAnalyzer) looksLikeMatrix(data [][]interface{}) bool {
	if len(data) < 2 {
		return false
	}
	
	// Matrix data is mostly numeric
	numericCount := 0
	totalCount := 0
	
	for _, row := range data {
		for _, cell := range row {
			if cell != nil && fmt.Sprintf("%v", cell) != "" {
				totalCount++
				if rsa.isNumericCell(cell) {
					numericCount++
				}
			}
		}
	}
	
	if totalCount == 0 {
		return false
	}
	
	numericRatio := float64(numericCount) / float64(totalCount)
	return numericRatio > 0.8
}

func (rsa *RobustSheetAnalyzer) rowHasData(row []interface{}) bool {
	for _, cell := range row {
		if cell != nil && fmt.Sprintf("%v", cell) != "" {
			return true
		}
	}
	return false
}

func (rsa *RobustSheetAnalyzer) rowToHeaders(row []interface{}) []string {
	headers := make([]string, 0)
	headerCounts := make(map[string]int)
	
	for i, cell := range row {
		header := rsa.cellToString(cell, i)
		
		// Handle duplicates
		baseHeader := header
		if count, exists := headerCounts[strings.ToLower(header)]; exists {
			header = fmt.Sprintf("%s_%d", baseHeader, count+1)
		}
		headerCounts[strings.ToLower(baseHeader)]++
		
		headers = append(headers, header)
	}
	
	return headers
}

func (rsa *RobustSheetAnalyzer) cellToString(cell interface{}, colIndex int) string {
	if cell == nil {
		return fmt.Sprintf("column_%d", colIndex+1)
	}
	
	str := fmt.Sprintf("%v", cell)
	str = strings.TrimSpace(str)
	
	if str == "" || str == "<nil>" {
		return fmt.Sprintf("column_%d", colIndex+1)
	}
	
	// Clean the string for use as header
	cleaned := rsa.cleanHeaderString(str)
	if cleaned == "" {
		return fmt.Sprintf("column_%d", colIndex+1)
	}
	
	return cleaned
}

func (rsa *RobustSheetAnalyzer) cleanHeaderString(s string) string {
	// Remove special characters but keep underscores and spaces
	re := regexp.MustCompile(`[^a-zA-Z0-9_\s]+`)
	cleaned := re.ReplaceAllString(s, "_")
	
	// Replace multiple spaces/underscores with single underscore
	re = regexp.MustCompile(`[\s_]+`)
	cleaned = re.ReplaceAllString(cleaned, "_")
	
	// Trim underscores
	cleaned = strings.Trim(cleaned, "_")
	
	// Convert to lowercase
	cleaned = strings.ToLower(cleaned)
	
	// Ensure it starts with a letter
	if len(cleaned) > 0 && !unicode.IsLetter(rune(cleaned[0])) {
		cleaned = "col_" + cleaned
	}
	
	// Limit length
	if len(cleaned) > 50 {
		cleaned = cleaned[:50]
	}
	
	return cleaned
}

func (rsa *RobustSheetAnalyzer) generateHeaders(count int) []string {
	headers := make([]string, count)
	for i := 0; i < count; i++ {
		headers[i] = fmt.Sprintf("column_%d", i+1)
	}
	return headers
}

func (rsa *RobustSheetAnalyzer) normalizeRows(rows [][]interface{}, targetLength int) [][]interface{} {
	normalized := make([][]interface{}, 0)
	
	for _, row := range rows {
		newRow := make([]interface{}, targetLength)
		for i := 0; i < targetLength; i++ {
			if i < len(row) {
				newRow[i] = row[i]
			} else {
				newRow[i] = nil
			}
		}
		normalized = append(normalized, newRow)
	}
	
	return normalized
}

func (rsa *RobustSheetAnalyzer) scoreAsHeaders(headers []string, data [][]interface{}, headerRow int) float64 {
	score := 0.0
	
	// Check for meaningful header names
	for _, header := range headers {
		if !strings.HasPrefix(header, "column_") {
			score += 0.2
		}
	}
	
	// Check uniqueness
	unique := make(map[string]bool)
	for _, header := range headers {
		unique[header] = true
	}
	uniqueRatio := float64(len(unique)) / float64(len(headers))
	score += uniqueRatio * 0.3
	
	// Check if following rows have different types
	if headerRow < len(data)-1 {
		differentTypes := 0
		for i, header := range headers {
			if i < len(data[headerRow+1]) {
				if rsa.isTextCell(header) && !rsa.isTextCell(data[headerRow+1][i]) {
					differentTypes++
				}
			}
		}
		if len(headers) > 0 {
			score += (float64(differentTypes) / float64(len(headers))) * 0.5
		}
	}
	
	return score
}

func (rsa *RobustSheetAnalyzer) isTextCell(cell interface{}) bool {
	if cell == nil {
		return false
	}
	
	str := fmt.Sprintf("%v", cell)
	
	// Try to parse as number
	if _, err := strconv.ParseFloat(str, 64); err == nil {
		return false
	}
	
	// Try to parse as date
	dateFormats := []string{
		"2006-01-02",
		"01/02/2006",
		"02-01-2006",
		"2006-01-02 15:04:05",
	}
	
	for _, format := range dateFormats {
		if _, err := time.Parse(format, str); err == nil {
			return false
		}
	}
	
	// Check for boolean
	lower := strings.ToLower(str)
	if lower == "true" || lower == "false" || lower == "yes" || lower == "no" {
		return false
	}
	
	return true
}

func (rsa *RobustSheetAnalyzer) isNumericCell(cell interface{}) bool {
	if cell == nil {
		return false
	}
	
	str := fmt.Sprintf("%v", cell)
	_, err := strconv.ParseFloat(str, 64)
	return err == nil
}

func (rsa *RobustSheetAnalyzer) calculateUniqueness(row []interface{}) float64 {
	if len(row) == 0 {
		return 0.0
	}
	
	unique := make(map[string]bool)
	nonEmpty := 0
	
	for _, cell := range row {
		if cell != nil && fmt.Sprintf("%v", cell) != "" {
			unique[fmt.Sprintf("%v", cell)] = true
			nonEmpty++
		}
	}
	
	if nonEmpty == 0 {
		return 0.0
	}
	
	return float64(len(unique)) / float64(nonEmpty)
}

func (rsa *RobustSheetAnalyzer) detectDataPatterns(data [][]interface{}) map[string]bool {
	patterns := make(map[string]bool)
	
	// Check for key-value pattern
	if rsa.isKeyValuePattern(data) {
		patterns["key_value"] = true
	}
	
	// Check for list pattern
	if rsa.isListPattern(data) {
		patterns["list"] = true
	}
	
	return patterns
}

func (rsa *RobustSheetAnalyzer) isKeyValuePattern(data [][]interface{}) bool {
	if len(data) == 0 {
		return false
	}
	
	// Key-value pairs typically have 2 columns
	twoColCount := 0
	for _, row := range data {
		if len(row) == 2 {
			twoColCount++
		}
	}
	
	return float64(twoColCount)/float64(len(data)) > 0.7
}

func (rsa *RobustSheetAnalyzer) isListPattern(data [][]interface{}) bool {
	if len(data) == 0 {
		return false
	}
	
	// List pattern typically has 1 column
	oneColCount := 0
	for _, row := range data {
		nonEmpty := 0
		for _, cell := range row {
			if cell != nil && fmt.Sprintf("%v", cell) != "" {
				nonEmpty++
			}
		}
		if nonEmpty == 1 {
			oneColCount++
		}
	}
	
	return float64(oneColCount)/float64(len(data)) > 0.7
}

func (rsa *RobustSheetAnalyzer) extractKeyValuePairs(data [][]interface{}) [][]interface{} {
	pairs := make([][]interface{}, 0)
	
	for _, row := range data {
		if len(row) >= 2 {
			key := row[0]
			value := row[1]
			if key != nil && fmt.Sprintf("%v", key) != "" {
				pairs = append(pairs, []interface{}{key, value})
			}
		}
	}
	
	return pairs
}

func (rsa *RobustSheetAnalyzer) extractListItems(data [][]interface{}) [][]interface{} {
	items := make([][]interface{}, 0)
	
	for _, row := range data {
		for _, cell := range row {
			if cell != nil && fmt.Sprintf("%v", cell) != "" {
				items = append(items, []interface{}{cell})
				break // Only take first non-empty cell per row
			}
		}
	}
	
	return items
}

func (rsa *RobustSheetAnalyzer) extractPositionalData(data [][]interface{}) ([]string, [][]interface{}) {
	// Find max columns
	maxCols := 0
	for _, row := range data {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	
	// Generate headers
	headers := make([]string, maxCols)
	for i := 0; i < maxCols; i++ {
		headers[i] = fmt.Sprintf("field_%d", i+1)
	}
	
	// Normalize rows
	rows := rsa.normalizeRows(data, maxCols)
	
	return headers, rows
}

func (rsa *RobustSheetAnalyzer) calculateDataQuality(rows [][]interface{}, headers []string) float64 {
	if len(rows) == 0 || len(headers) == 0 {
		return 0.0
	}
	
	quality := 100.0
	
	// Check for empty cells
	totalCells := len(rows) * len(headers)
	emptyCells := 0
	
	for _, row := range rows {
		for _, cell := range row {
			if cell == nil || fmt.Sprintf("%v", cell) == "" {
				emptyCells++
			}
		}
	}
	
	if totalCells > 0 {
		emptyRatio := float64(emptyCells) / float64(totalCells)
		quality -= emptyRatio * 30
	}
	
	// Check for consistent data types per column
	for colIdx := range headers {
		types := make(map[string]int)
		for _, row := range rows {
			if colIdx < len(row) && row[colIdx] != nil {
				cellType := rsa.getCellType(row[colIdx])
				types[cellType]++
			}
		}
		
		// Penalize mixed types
		if len(types) > 1 {
			quality -= 5
		}
	}
	
	// Check for duplicate rows
	rowSet := make(map[string]bool)
	duplicates := 0
	for _, row := range rows {
		rowStr := fmt.Sprintf("%v", row)
		if rowSet[rowStr] {
			duplicates++
		}
		rowSet[rowStr] = true
	}
	
	if len(rows) > 0 {
		dupRatio := float64(duplicates) / float64(len(rows))
		quality -= dupRatio * 20
	}
	
	return math.Max(0, math.Min(100, quality))
}

func (rsa *RobustSheetAnalyzer) getCellType(cell interface{}) string {
	if cell == nil {
		return "null"
	}
	
	str := fmt.Sprintf("%v", cell)
	
	if _, err := strconv.ParseFloat(str, 64); err == nil {
		return "number"
	}
	
	if _, err := time.Parse("2006-01-02", str); err == nil {
		return "date"
	}
	
	lower := strings.ToLower(str)
	if lower == "true" || lower == "false" {
		return "boolean"
	}
	
	return "text"
}

func (rsa *RobustSheetAnalyzer) detectDataIssues(region *DataRegion) []string {
	issues := make([]string, 0)
	
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
			issues = append(issues, fmt.Sprintf("Column '%s' is empty", header))
		}
	}
	
	// Check for formula errors
	for _, row := range region.DataRows {
		for _, cell := range row {
			cellStr := fmt.Sprintf("%v", cell)
			if strings.HasPrefix(cellStr, "#") && 
			   (strings.Contains(cellStr, "ERROR") || 
			    strings.Contains(cellStr, "REF") || 
			    strings.Contains(cellStr, "DIV")) {
				issues = append(issues, "Sheet contains formula errors")
				break
			}
		}
	}
	
	return issues
}

func (rsa *RobustSheetAnalyzer) generateSuggestions(region *DataRegion) []string {
	suggestions := make([]string, 0)
	
	// Check for generic column names
	genericCount := 0
	for _, header := range region.Headers {
		if strings.HasPrefix(header, "column_") || strings.HasPrefix(header, "field_") {
			genericCount++
		}
	}
	
	if genericCount > len(region.Headers)/2 {
		suggestions = append(suggestions, "Add meaningful column names for better data understanding")
	}
	
	// Check for wide tables
	if len(region.Headers) > 20 {
		suggestions = append(suggestions, "Consider normalizing the data structure")
	}
	
	// Check for low data quality
	if region.Quality < 60 {
		suggestions = append(suggestions, "Data quality is low - consider cleaning and structuring the data")
	}
	
	return suggestions
}

func (rsa *RobustSheetAnalyzer) columnIndexToName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+index%26)) + name
		index = index/26 - 1
	}
	return name
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}