package dbmanager

import (
	"fmt"
	"strings"
)

// QueryValidator defines the interface for database-specific query validation.
// Each database driver can implement its own validation rules while sharing common patterns.
type QueryValidator interface {
	// ValidateSafety checks if a query is safe to execute
	// Returns an error if the query violates safety rules
	ValidateSafety(query string, queryType string, tableMetadata map[string]TableSchema) error
	
	// GetDatabaseType returns the database type this validator handles
	GetDatabaseType() string
}

// BaseQueryValidator provides common validation logic shared across all databases
type BaseQueryValidator struct {
	dbType string
}

// NewBaseQueryValidator creates a new base validator
func NewBaseQueryValidator(dbType string) *BaseQueryValidator {
	return &BaseQueryValidator{dbType: dbType}
}

// GetDatabaseType returns the database type
func (v *BaseQueryValidator) GetDatabaseType() string {
	return v.dbType
}

// ValidateDeleteOrUpdateWithoutWhere checks for DELETE/UPDATE without WHERE clause
// This is the most critical safety check - prevents accidental data loss
func (v *BaseQueryValidator) ValidateDeleteOrUpdateWithoutWhere(query string) error {
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	
	// Remove comments to avoid false positives
	queryUpper = v.removeComments(queryUpper)
	
	// Check for DELETE without WHERE
	if strings.Contains(queryUpper, "DELETE") && strings.Contains(queryUpper, "FROM") {
		if !strings.Contains(queryUpper, "WHERE") && !strings.Contains(queryUpper, "LIMIT") {
			return fmt.Errorf("SAFETY VIOLATION: DELETE without WHERE clause would delete ALL records. " +
				"Please add a WHERE condition to specify which records to delete")
		}
	}
	
	// Check for UPDATE without WHERE
	if strings.Contains(queryUpper, "UPDATE") && strings.Contains(queryUpper, "SET") {
		if !strings.Contains(queryUpper, "WHERE") && !strings.Contains(queryUpper, "LIMIT") {
			return fmt.Errorf("SAFETY VIOLATION: UPDATE without WHERE clause would update ALL records. " +
				"Please add a WHERE condition to specify which records to update")
		}
	}
	
	// Check for TRUNCATE (always destructive)
	if strings.HasPrefix(queryUpper, "TRUNCATE") {
		return fmt.Errorf("SAFETY WARNING: TRUNCATE will delete ALL records from the table. " +
			"Use DELETE with WHERE clause for safer selective deletion")
	}
	
	return nil
}

// ValidateTableScanRisk checks if a SELECT query on a large table has no LIMIT
func (v *BaseQueryValidator) ValidateTableScanRisk(query string, tableMetadata map[string]TableSchema) error {
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	
	// Only validate SELECT queries
	if !strings.HasPrefix(queryUpper, "SELECT") && !strings.HasPrefix(queryUpper, "WITH") {
		return nil
	}
	
	// Skip if query has LIMIT, COUNT, or is an aggregate
	if strings.Contains(queryUpper, "LIMIT") || 
	   strings.Contains(queryUpper, "COUNT(") ||
	   strings.Contains(queryUpper, "SUM(") ||
	   strings.Contains(queryUpper, "AVG(") ||
	   strings.Contains(queryUpper, "MAX(") ||
	   strings.Contains(queryUpper, "MIN(") {
		return nil
	}
	
	// Extract table names from query (basic extraction)
	tableNames := v.extractTableNames(query)
	
	// Check if any table is large and query has no filters
	for tableName := range tableNames {
		if table, exists := tableMetadata[tableName]; exists {
			// Warn for tables with >100K rows without WHERE clause
			if table.RowCount > 100000 && !strings.Contains(queryUpper, "WHERE") {
				return fmt.Errorf("PERFORMANCE WARNING: Query will scan large table '%s' (%d rows) without WHERE clause or LIMIT. " +
					"This may cause performance issues. Consider adding filters or LIMIT clause",
					tableName, table.RowCount)
			}
			
			// Auto-reject for tables with >1M rows without any filters
			if table.RowCount > 1000000 && !strings.Contains(queryUpper, "WHERE") && !strings.Contains(queryUpper, "LIMIT") {
				return fmt.Errorf("SAFETY VIOLATION: Query would scan extremely large table '%s' (%d rows) without filters. " +
					"Please add WHERE clause with indexed columns or LIMIT to prevent database overload",
					tableName, table.RowCount)
			}
		}
	}
	
	return nil
}

// removeComments removes SQL comments to avoid false positives in validation
func (v *BaseQueryValidator) removeComments(query string) string {
	// Remove single-line comments (-- comment)
	lines := strings.Split(query, "\n")
	var cleaned []string
	for _, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		if strings.TrimSpace(line) != "" {
			cleaned = append(cleaned, line)
		}
	}
	query = strings.Join(cleaned, " ")
	
	// Remove multi-line comments (/* comment */)
	for {
		start := strings.Index(query, "/*")
		if start == -1 {
			break
		}
		end := strings.Index(query[start:], "*/")
		if end == -1 {
			break
		}
		query = query[:start] + query[start+end+2:]
	}
	
	return query
}

// extractTableNames attempts to extract table names from SQL query
// This is a basic implementation - may need enhancement for complex queries
func (v *BaseQueryValidator) extractTableNames(query string) map[string]bool {
	tables := make(map[string]bool)
	queryUpper := strings.ToUpper(query)
	
	// Look for FROM and JOIN clauses
	keywords := []string{"FROM", "JOIN", "UPDATE", "INTO"}
	
	for _, keyword := range keywords {
		if idx := strings.Index(queryUpper, keyword); idx >= 0 {
			// Extract word after keyword
			after := queryUpper[idx+len(keyword):]
			words := strings.Fields(after)
			if len(words) > 0 {
				tableName := strings.TrimSpace(words[0])
				// Remove common SQL keywords that might follow table name
				tableName = strings.TrimSuffix(tableName, ",")
				tableName = strings.TrimSuffix(tableName, ";")
				tableName = strings.Trim(tableName, "`\"'[]")
				if tableName != "" && !isKeyword(tableName) {
					tables[strings.ToLower(tableName)] = true
				}
			}
		}
	}
	
	return tables
}

// isKeyword checks if a word is a common SQL keyword
func isKeyword(word string) bool {
	keywords := map[string]bool{
		"WHERE": true, "SET": true, "VALUES": true, "SELECT": true,
		"ON": true, "AND": true, "OR": true, "AS": true, "INNER": true,
		"LEFT": true, "RIGHT": true, "OUTER": true, "CROSS": true,
	}
	return keywords[strings.ToUpper(word)]
}

// ============================================================================
// SQL Database Validators (PostgreSQL, MySQL, ClickHouse, YugabyteDB)
// ============================================================================

// SQLQueryValidator implements validation for SQL-based databases
type SQLQueryValidator struct {
	*BaseQueryValidator
}

// NewSQLQueryValidator creates a validator for SQL databases
func NewSQLQueryValidator(dbType string) *SQLQueryValidator {
	return &SQLQueryValidator{
		BaseQueryValidator: NewBaseQueryValidator(dbType),
	}
}

// ValidateSafety performs comprehensive safety validation for SQL queries
func (v *SQLQueryValidator) ValidateSafety(query string, queryType string, tableMetadata map[string]TableSchema) error {
	// 1. Check for DELETE/UPDATE without WHERE (CRITICAL)
	if err := v.ValidateDeleteOrUpdateWithoutWhere(query); err != nil {
		return err
	}
	
	// 2. Check for table scan risks on large tables
	if err := v.ValidateTableScanRisk(query, tableMetadata); err != nil {
		return err
	}
	
	// 3. Check for DROP operations (always require explicit confirmation)
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(queryUpper, "DROP TABLE") || strings.HasPrefix(queryUpper, "DROP DATABASE") {
		return fmt.Errorf("SAFETY WARNING: DROP operations are destructive and permanent. " +
			"Please confirm this action explicitly")
	}
	
	return nil
}

// ============================================================================
// MongoDB Validator
// ============================================================================

// MongoDBQueryValidator implements validation for MongoDB queries
type MongoDBQueryValidator struct {
	*BaseQueryValidator
}

// NewMongoDBQueryValidator creates a validator for MongoDB
func NewMongoDBQueryValidator() *MongoDBQueryValidator {
	return &MongoDBQueryValidator{
		BaseQueryValidator: NewBaseQueryValidator("mongodb"),
	}
}

// ValidateSafety performs safety validation for MongoDB queries
func (v *MongoDBQueryValidator) ValidateSafety(query string, queryType string, tableMetadata map[string]TableSchema) error {
	queryLower := strings.ToLower(strings.TrimSpace(query))
	
	// 1. Check for deleteMany without filter
	if strings.Contains(queryLower, ".deletemany({})") || strings.Contains(queryLower, ".deletemany( {} )") {
		return fmt.Errorf("SAFETY VIOLATION: deleteMany({}) without filter would delete ALL documents. " +
			"Please add a filter to specify which documents to delete")
	}
	
	// 2. Check for updateMany without filter
	if strings.Contains(queryLower, ".updatemany({})") || strings.Contains(queryLower, ".updatemany( {} )") {
		return fmt.Errorf("SAFETY VIOLATION: updateMany({}) without filter would update ALL documents. " +
			"Please add a filter to specify which documents to update")
	}
	
	// 3. Check for drop operations
	if strings.Contains(queryLower, ".drop()") {
		return fmt.Errorf("SAFETY WARNING: drop() will permanently delete the entire collection. " +
			"Please confirm this action explicitly")
	}
	
	// 4. Check for large collection scans without limit
	if strings.Contains(queryLower, ".find()") && !strings.Contains(queryLower, ".limit(") {
		// Extract collection name (basic)
		collectionName := v.extractMongoCollectionName(query)
		if collectionName != "" {
			if table, exists := tableMetadata[collectionName]; exists {
				if table.RowCount > 100000 {
					return fmt.Errorf("PERFORMANCE WARNING: find() on large collection '%s' (%d documents) without limit(). " +
						"Consider adding .limit() to prevent performance issues",
						collectionName, table.RowCount)
				}
			}
		}
	}
	
	return nil
}

// extractMongoCollectionName attempts to extract collection name from MongoDB query
func (v *MongoDBQueryValidator) extractMongoCollectionName(query string) string {
	// Look for db.collectionName pattern
	if idx := strings.Index(query, "db."); idx >= 0 {
		after := query[idx+3:]
		parts := strings.FieldsFunc(after, func(r rune) bool {
			return r == '.' || r == '(' || r == ')'
		})
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}

// ============================================================================
// Validator Factory
// ============================================================================

// GetValidatorForDatabase returns the appropriate validator for the database type
func GetValidatorForDatabase(dbType string) QueryValidator {
	switch strings.ToLower(dbType) {
	case "postgresql", "postgres":
		return NewSQLQueryValidator("postgresql")
	case "mysql":
		return NewSQLQueryValidator("mysql")
	case "clickhouse":
		return NewSQLQueryValidator("clickhouse")
	case "yugabyte", "yugabytedb":
		return NewSQLQueryValidator("yugabyte")
	case "mongodb", "mongo":
		return NewMongoDBQueryValidator()
	case "spreadsheet", "google_sheets":
		// Spreadsheet connections use PostgreSQL internally, so use SQL validator
		return NewSQLQueryValidator("spreadsheet")
	default:
		// Default to SQL validator for unknown databases
		return NewSQLQueryValidator(dbType)
	}
}
