package services

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"neobase-ai/pkg/dbmanager"
)

// ColumnMapping represents how columns map between old and new data
type ColumnMapping struct {
	OldName         string
	NewName         string
	IsNew           bool
	IsDeleted       bool
	IsMapped        bool
	DataType        string
	SimilarityScore float64
}

// MergeOptions contains options for merge operations
type MergeOptions struct {
	Strategy        string   // replace, append, merge, smart_merge
	KeyColumns      []string // columns to use as keys for matching rows
	IgnoreCase      bool     // ignore case when comparing values
	TrimWhitespace  bool     // trim whitespace from values
	HandleNulls     string   // how to handle null values: "keep", "empty", "default"
	DateFormat      string   // expected date format
	DropMissingCols bool     // drop columns not in new data
	AddNewCols      bool     // add columns from new data
	UpdateExisting  bool     // update existing rows (for merge)
	InsertNew       bool     // insert new rows (for merge)
	DeleteMissing   bool     // delete rows not in new data
}

// SpreadsheetMergeHandler handles complex merge operations
type SpreadsheetMergeHandler struct {
	conn       dbmanager.DBExecutor
	schemaName string
	tableName  string
}

// NewSpreadsheetMergeHandler creates a new merge handler
func NewSpreadsheetMergeHandler(conn dbmanager.DBExecutor, schemaName, tableName string) *SpreadsheetMergeHandler {
	return &SpreadsheetMergeHandler{
		conn:       conn,
		schemaName: schemaName,
		tableName:  tableName,
	}
}

// AnalyzeSchemaChanges analyzes differences between existing and new schema
func (h *SpreadsheetMergeHandler) AnalyzeSchemaChanges(existingCols, newCols []string) ([]ColumnMapping, error) {
	mappings := make([]ColumnMapping, 0)

	// Normalize column names for comparison
	normalizedExisting := make(map[string]string)
	normalizedNew := make(map[string]string)

	for _, col := range existingCols {
		normalized := h.normalizeColumnName(col)
		normalizedExisting[normalized] = col
	}

	for _, col := range newCols {
		normalized := h.normalizeColumnName(col)
		normalizedNew[normalized] = col
	}

	// Find exact matches first
	matchedNew := make(map[string]bool)
	matchedExisting := make(map[string]bool)

	for normNew, origNew := range normalizedNew {
		if origExisting, exists := normalizedExisting[normNew]; exists {
			mappings = append(mappings, ColumnMapping{
				OldName:  origExisting,
				NewName:  origNew,
				IsMapped: true,
			})
			matchedNew[origNew] = true
			matchedExisting[origExisting] = true
		}
	}

	// Find fuzzy matches for unmatched columns
	for _, newCol := range newCols {
		if matchedNew[newCol] {
			continue
		}

		bestMatch := ""
		bestScore := 0.0

		for _, existingCol := range existingCols {
			if matchedExisting[existingCol] {
				continue
			}

			score := h.calculateSimilarity(newCol, existingCol)
			if score > bestScore && score > 0.7 { // 70% similarity threshold
				bestMatch = existingCol
				bestScore = score
			}
		}

		if bestMatch != "" {
			mappings = append(mappings, ColumnMapping{
				OldName:         bestMatch,
				NewName:         newCol,
				IsMapped:        true,
				SimilarityScore: bestScore,
			})
			matchedNew[newCol] = true
			matchedExisting[bestMatch] = true
		}
	}

	// Mark new columns
	for _, newCol := range newCols {
		if !matchedNew[newCol] {
			mappings = append(mappings, ColumnMapping{
				NewName: newCol,
				IsNew:   true,
			})
		}
	}

	// Mark deleted columns
	for _, existingCol := range existingCols {
		if !matchedExisting[existingCol] && !strings.HasPrefix(existingCol, "_") {
			mappings = append(mappings, ColumnMapping{
				OldName:   existingCol,
				IsDeleted: true,
			})
		}
	}

	return mappings, nil
}

// ExecuteMerge performs the actual merge operation based on options
func (h *SpreadsheetMergeHandler) ExecuteMerge(newColumns []string, newData [][]string, options MergeOptions) error {
	switch options.Strategy {
	case "replace":
		return h.executeReplace(newColumns, newData)
	case "append":
		return h.executeAppend(newColumns, newData, options)
	case "merge", "smart_merge":
		return h.executeSmartMerge(newColumns, newData, options)
	default:
		return fmt.Errorf("unknown merge strategy: %s", options.Strategy)
	}
}

// executeReplace drops and recreates the table
func (h *SpreadsheetMergeHandler) executeReplace(columns []string, data [][]string) error {
	// This is already implemented in the main service
	// Just drop and recreate
	return nil
}

// executeAppend appends data with schema reconciliation
func (h *SpreadsheetMergeHandler) executeAppend(newColumns []string, newData [][]string, options MergeOptions) error {
	// Get existing columns
	existingCols, err := h.getTableColumns()
	if err != nil {
		return fmt.Errorf("failed to get existing columns: %v", err)
	}

	// Analyze schema changes
	mappings, err := h.AnalyzeSchemaChanges(existingCols, newColumns)
	if err != nil {
		return fmt.Errorf("failed to analyze schema: %v", err)
	}

	// Add new columns if needed
	if options.AddNewCols {
		for _, mapping := range mappings {
			if mapping.IsNew {
				alterQuery := fmt.Sprintf(
					"ALTER TABLE %s.%s ADD COLUMN %s TEXT",
					h.schemaName, h.tableName, sanitizeColumnName(mapping.NewName),
				)
				if err := h.conn.Exec(alterQuery); err != nil {
					log.Printf("Warning: Failed to add column %s: %v", mapping.NewName, err)
				}
			}
		}
	}

	// Build column list for insert
	insertCols := make([]string, 0)
	colIndexMap := make(map[int]string) // maps new data index to table column

	for i, newCol := range newColumns {
		for _, mapping := range mappings {
			if mapping.NewName == newCol && !mapping.IsNew {
				insertCols = append(insertCols, sanitizeColumnName(mapping.OldName))
				colIndexMap[i] = sanitizeColumnName(mapping.OldName)
				break
			} else if mapping.NewName == newCol && mapping.IsNew && options.AddNewCols {
				insertCols = append(insertCols, sanitizeColumnName(mapping.NewName))
				colIndexMap[i] = sanitizeColumnName(mapping.NewName)
				break
			}
		}
	}

	// Insert data with proper column mapping
	return h.insertDataWithMapping(insertCols, newData, colIndexMap, options)
}

// executeSmartMerge performs intelligent merge with updates and inserts
func (h *SpreadsheetMergeHandler) executeSmartMerge(newColumns []string, newData [][]string, options MergeOptions) error {
	if len(options.KeyColumns) == 0 {
		// If no key columns specified, try to find an ID column or use all columns
		options.KeyColumns = h.detectKeyColumns(newColumns)
	}

	// Get existing data for comparison
	existingData, existingCols, err := h.getExistingData()
	if err != nil {
		return fmt.Errorf("failed to get existing data: %v", err)
	}

	// Analyze schema changes
	mappings, err := h.AnalyzeSchemaChanges(existingCols, newColumns)
	if err != nil {
		return fmt.Errorf("failed to analyze schema: %v", err)
	}

	// Handle schema changes
	if err := h.handleSchemaChanges(mappings, options); err != nil {
		return fmt.Errorf("failed to handle schema changes: %v", err)
	}

	// Build key indices
	keyIndicesNew := h.getColumnIndices(newColumns, options.KeyColumns)
	keyIndicesExisting := h.getColumnIndices(existingCols, options.KeyColumns)

	// Create lookup map for existing data
	existingMap := make(map[string]map[string]interface{})
	for _, row := range existingData {
		key := h.buildRowKey(row, existingCols, keyIndicesExisting, options)
		existingMap[key] = row
	}

	// Process new data
	updates := make([]map[string]interface{}, 0)
	inserts := make([][]string, 0)
	processedKeys := make(map[string]bool)

	for _, newRow := range newData {
		key := h.buildRowKey(h.rowToMap(newRow, newColumns), newColumns, keyIndicesNew, options)
		processedKeys[key] = true

		if existingRow, exists := existingMap[key]; exists && options.UpdateExisting {
			// Check if update needed
			if h.rowNeedsUpdate(existingRow, newRow, mappings, options) {
				updateData := h.prepareUpdateData(existingRow, newRow, mappings, options)
				updateData["_key"] = key
				updates = append(updates, updateData)
			}
		} else if options.InsertNew {
			inserts = append(inserts, newRow)
		}
	}

	// Handle deletions
	deletes := make([]string, 0)
	if options.DeleteMissing {
		for key := range existingMap {
			if !processedKeys[key] {
				deletes = append(deletes, key)
			}
		}
	}

	// Execute updates
	if len(updates) > 0 {
		if err := h.executeUpdates(updates, options); err != nil {
			return fmt.Errorf("failed to execute updates: %v", err)
		}
	}

	// Execute inserts
	if len(inserts) > 0 {
		if err := h.executeAppend(newColumns, inserts, options); err != nil {
			return fmt.Errorf("failed to execute inserts: %v", err)
		}
	}

	// Execute deletes
	if len(deletes) > 0 {
		if err := h.executeDeletes(deletes, options); err != nil {
			return fmt.Errorf("failed to execute deletes: %v", err)
		}
	}

	return nil
}

// Helper methods

func (h *SpreadsheetMergeHandler) normalizeColumnName(name string) string {
	// Remove special characters and normalize
	normalized := strings.ToLower(name)
	normalized = strings.TrimSpace(normalized)
	normalized = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(normalized, "_")
	normalized = strings.Trim(normalized, "_")
	return normalized
}

func (h *SpreadsheetMergeHandler) calculateSimilarity(s1, s2 string) float64 {
	// Simple Levenshtein distance-based similarity
	s1 = strings.ToLower(s1)
	s2 = strings.ToLower(s2)

	if s1 == s2 {
		return 1.0
	}

	maxLen := float64(max(len(s1), len(s2)))
	if maxLen == 0 {
		return 0.0
	}

	distance := float64(levenshteinDistance(s1, s2))
	return 1.0 - (distance / maxLen)
}

func (h *SpreadsheetMergeHandler) getTableColumns() ([]string, error) {
	query := fmt.Sprintf(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = '%s' AND table_name = '%s'
		AND column_name NOT LIKE '_%%'
		ORDER BY ordinal_position
	`, h.schemaName, h.tableName)

	var rows []map[string]interface{}
	err := h.conn.QueryRows(query, &rows)
	if err != nil {
		return nil, err
	}

	columns := make([]string, 0)
	for _, row := range rows {
		if colName, ok := row["column_name"].(string); ok {
			columns = append(columns, colName)
		}
	}

	return columns, nil
}

func (h *SpreadsheetMergeHandler) detectKeyColumns(columns []string) []string {
	// Try to detect ID columns
	for _, col := range columns {
		normalized := strings.ToLower(col)
		if normalized == "id" || normalized == "_id" ||
			strings.HasSuffix(normalized, "_id") ||
			normalized == "key" || normalized == "code" {
			return []string{col}
		}
	}

	// If no ID column found, use first few columns as key to avoid too complex keys
	if len(columns) > 3 {
		return columns[:3]
	}
	return columns
}

func (h *SpreadsheetMergeHandler) getColumnIndices(columns []string, keyColumns []string) []int {
	indices := make([]int, 0)
	for _, keyCol := range keyColumns {
		for i, col := range columns {
			if h.normalizeColumnName(col) == h.normalizeColumnName(keyCol) {
				indices = append(indices, i)
				break
			}
		}
	}
	return indices
}

func (h *SpreadsheetMergeHandler) buildRowKey(row map[string]interface{}, columns []string, keyIndices []int, options MergeOptions) string {
	keyParts := make([]string, 0)

	for _, idx := range keyIndices {
		if idx < len(columns) {
			col := columns[idx]
			val := ""
			if v, exists := row[col]; exists {
				val = fmt.Sprintf("%v", v)
			}

			if options.TrimWhitespace {
				val = strings.TrimSpace(val)
			}
			if options.IgnoreCase {
				val = strings.ToLower(val)
			}

			keyParts = append(keyParts, val)
		}
	}

	return strings.Join(keyParts, "|")
}

func (h *SpreadsheetMergeHandler) rowToMap(row []string, columns []string) map[string]interface{} {
	result := make(map[string]interface{})
	for i, col := range columns {
		if i < len(row) {
			result[col] = row[i]
		} else {
			result[col] = nil
		}
	}
	return result
}

func (h *SpreadsheetMergeHandler) handleSchemaChanges(mappings []ColumnMapping, options MergeOptions) error {
	// Add new columns
	if options.AddNewCols {
		for _, mapping := range mappings {
			if mapping.IsNew {
				query := fmt.Sprintf(
					"ALTER TABLE %s.%s ADD COLUMN IF NOT EXISTS %s TEXT",
					h.schemaName, h.tableName, sanitizeColumnName(mapping.NewName),
				)
				if err := h.conn.Exec(query); err != nil {
					log.Printf("Warning: Failed to add column %s: %v", mapping.NewName, err)
				}
			}
		}
	}

	// Drop missing columns
	if options.DropMissingCols {
		for _, mapping := range mappings {
			if mapping.IsDeleted {
				query := fmt.Sprintf(
					"ALTER TABLE %s.%s DROP COLUMN IF EXISTS %s",
					h.schemaName, h.tableName, sanitizeColumnName(mapping.OldName),
				)
				if err := h.conn.Exec(query); err != nil {
					log.Printf("Warning: Failed to drop column %s: %v", mapping.OldName, err)
				}
			}
		}
	}

	// Rename columns with high similarity
	for _, mapping := range mappings {
		if mapping.IsMapped && mapping.SimilarityScore > 0.8 && mapping.OldName != mapping.NewName {
			query := fmt.Sprintf(
				"ALTER TABLE %s.%s RENAME COLUMN %s TO %s",
				h.schemaName, h.tableName,
				sanitizeColumnName(mapping.OldName),
				sanitizeColumnName(mapping.NewName),
			)
			if err := h.conn.Exec(query); err != nil {
				log.Printf("Warning: Failed to rename column %s to %s: %v",
					mapping.OldName, mapping.NewName, err)
			}
		}
	}

	return nil
}

func (h *SpreadsheetMergeHandler) getExistingData() ([]map[string]interface{}, []string, error) {
	// Get columns
	columns, err := h.getTableColumns()
	if err != nil {
		return nil, nil, err
	}

	// Get data
	query := fmt.Sprintf(
		"SELECT %s FROM %s.%s",
		strings.Join(columns, ", "),
		h.schemaName,
		h.tableName,
	)

	var rows []map[string]interface{}
	err = h.conn.QueryRows(query, &rows)
	if err != nil {
		return nil, nil, err
	}

	return rows, columns, nil
}

func (h *SpreadsheetMergeHandler) rowNeedsUpdate(existingRow map[string]interface{}, newRow []string, mappings []ColumnMapping, options MergeOptions) bool {
	for i, mapping := range mappings {
		if !mapping.IsMapped || i >= len(newRow) {
			continue
		}

		existingVal := fmt.Sprintf("%v", existingRow[mapping.OldName])
		newVal := newRow[i]

		if options.TrimWhitespace {
			existingVal = strings.TrimSpace(existingVal)
			newVal = strings.TrimSpace(newVal)
		}

		if options.IgnoreCase {
			existingVal = strings.ToLower(existingVal)
			newVal = strings.ToLower(newVal)
		}

		if existingVal != newVal {
			return true
		}
	}

	return false
}

func (h *SpreadsheetMergeHandler) prepareUpdateData(existingRow map[string]interface{}, newRow []string, mappings []ColumnMapping, options MergeOptions) map[string]interface{} {
	updateData := make(map[string]interface{})

	for i, mapping := range mappings {
		if !mapping.IsMapped || i >= len(newRow) {
			continue
		}

		newVal := newRow[i]
		if options.TrimWhitespace {
			newVal = strings.TrimSpace(newVal)
		}

		// Handle null values
		if newVal == "" && options.HandleNulls == "keep" {
			continue // Don't update
		} else if newVal == "" && options.HandleNulls == "null" {
			updateData[mapping.OldName] = nil
		} else {
			updateData[mapping.OldName] = newVal
		}
	}

	updateData["_updated_at"] = time.Now()
	return updateData
}

func (h *SpreadsheetMergeHandler) insertDataWithMapping(columns []string, data [][]string, colIndexMap map[int]string, options MergeOptions) error {
	if len(data) == 0 || len(columns) == 0 {
		return nil
	}

	// Process in batches
	batchSize := 1000
	for i := 0; i < len(data); i += batchSize {
		end := i + batchSize
		if end > len(data) {
			end = len(data)
		}

		batch := data[i:end]
		valueStrings := make([]string, 0, len(batch))

		for _, row := range batch {
			values := make([]string, len(columns))

			for j, col := range columns {
				// Find the index in the new data
				val := ""
				for idx, mappedCol := range colIndexMap {
					if mappedCol == col && idx < len(row) {
						val = row[idx]
						break
					}
				}

				if options.TrimWhitespace {
					val = strings.TrimSpace(val)
				}

				// Escape single quotes
				val = strings.ReplaceAll(val, "'", "''")
				values[j] = fmt.Sprintf("'%s'", val)
			}

			valueStrings = append(valueStrings, fmt.Sprintf("(%s)", strings.Join(values, ", ")))
		}

		insertQuery := fmt.Sprintf(
			"INSERT INTO %s.%s (%s) VALUES %s",
			h.schemaName,
			h.tableName,
			strings.Join(columns, ", "),
			strings.Join(valueStrings, ", "),
		)

		if err := h.conn.Exec(insertQuery); err != nil {
			return fmt.Errorf("failed to insert batch: %v", err)
		}
	}

	return nil
}

func (h *SpreadsheetMergeHandler) executeUpdates(updates []map[string]interface{}, options MergeOptions) error {
	for _, update := range updates {
		key := update["_key"].(string)
		delete(update, "_key")

		setClauses := make([]string, 0)
		for col, val := range update {
			if col == "_updated_at" {
				setClauses = append(setClauses, fmt.Sprintf("%s = CURRENT_TIMESTAMP", col))
			} else if val == nil {
				setClauses = append(setClauses, fmt.Sprintf("%s = NULL", col))
			} else {
				valStr := strings.ReplaceAll(fmt.Sprintf("%v", val), "'", "''")
				setClauses = append(setClauses, fmt.Sprintf("%s = '%s'", col, valStr))
			}
		}

		whereClause := h.buildWhereClause(key, options.KeyColumns, options)

		updateQuery := fmt.Sprintf(
			"UPDATE %s.%s SET %s WHERE %s",
			h.schemaName,
			h.tableName,
			strings.Join(setClauses, ", "),
			whereClause,
		)

		if err := h.conn.Exec(updateQuery); err != nil {
			log.Printf("Failed to update row: %v", err)
		}
	}

	return nil
}

func (h *SpreadsheetMergeHandler) executeDeletes(deletes []string, options MergeOptions) error {
	for _, key := range deletes {
		whereClause := h.buildWhereClause(key, options.KeyColumns, options)

		deleteQuery := fmt.Sprintf(
			"DELETE FROM %s.%s WHERE %s",
			h.schemaName,
			h.tableName,
			whereClause,
		)

		if err := h.conn.Exec(deleteQuery); err != nil {
			log.Printf("Failed to delete row: %v", err)
		}
	}

	return nil
}

func (h *SpreadsheetMergeHandler) buildWhereClause(key string, keyColumns []string, options MergeOptions) string {
	keyParts := strings.Split(key, "|")
	conditions := make([]string, 0)

	for i, keyCol := range keyColumns {
		if i < len(keyParts) {
			val := keyParts[i]
			val = strings.ReplaceAll(val, "'", "''")

			if options.IgnoreCase {
				conditions = append(conditions, fmt.Sprintf("LOWER(%s) = LOWER('%s')", keyCol, val))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s = '%s'", keyCol, val))
			}
		}
	}

	return strings.Join(conditions, " AND ")
}

// Utility functions

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize first column and row
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func min(nums ...int) int {
	if len(nums) == 0 {
		return 0
	}
	m := nums[0]
	for _, n := range nums[1:] {
		if n < m {
			m = n
		}
	}
	return m
}
