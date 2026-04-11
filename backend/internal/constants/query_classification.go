package constants

import "strings"

// ============================================================================
// Per-Database Read/Write Query Classification
// ============================================================================
// Each database type defines its own read-only prefixes, write prefixes, and
// (for MongoDB) operation-level patterns. These are used by the tool executor
// to validate that tool-called queries are strictly read-only.

// QueryClassification holds the read/write classification rules for a database type.
type QueryClassification struct {
	// ReadPrefixes are SQL statement prefixes that indicate a read-only query.
	// Matched against the lowercased, trimmed query start.
	ReadPrefixes []string

	// WritePrefixes are SQL statement prefixes that indicate a write operation.
	// Matched against the lowercased, trimmed query start.
	WritePrefixes []string

	// ReadContains are substrings that indicate a read operation (primarily for MongoDB).
	// Matched via strings.Contains on the lowercased query.
	ReadContains []string

	// WriteContains are substrings that indicate a write operation (primarily for MongoDB).
	// Matched via strings.Contains on the lowercased query.
	WriteContains []string
}

// --- SQL-family databases (common base) ---

var sqlReadPrefixes = []string{
	"select", "show", "describe", "desc", "explain", "with", "pragma",
}

var sqlWritePrefixes = []string{
	"insert", "update", "delete", "drop", "truncate", "alter",
	"create", "grant", "revoke", "merge", "upsert", "replace",
}

// PostgreSQLQueryClassification defines read/write rules for PostgreSQL.
var PostgreSQLQueryClassification = QueryClassification{
	ReadPrefixes:  sqlReadPrefixes,
	WritePrefixes: sqlWritePrefixes,
}

// YugabyteDBQueryClassification — YugabyteDB is PostgreSQL-compatible.
var YugabyteDBQueryClassification = PostgreSQLQueryClassification

// MySQLQueryClassification defines read/write rules for MySQL.
var MySQLQueryClassification = QueryClassification{
	ReadPrefixes:  sqlReadPrefixes,
	WritePrefixes: sqlWritePrefixes,
}

// ClickHouseQueryClassification defines read/write rules for ClickHouse.
var ClickHouseQueryClassification = QueryClassification{
	ReadPrefixes:  sqlReadPrefixes,
	WritePrefixes: sqlWritePrefixes,
}

// SpreadsheetQueryClassification — spreadsheets use PostgreSQL under the hood.
var SpreadsheetQueryClassification = PostgreSQLQueryClassification

// GoogleSheetsQueryClassification — same as Spreadsheet.
var GoogleSheetsQueryClassification = PostgreSQLQueryClassification

// --- MongoDB ---

// MongoDBQueryClassification defines read/write rules for MongoDB.
// MongoDB queries are not SQL — they use method-based syntax like db.collection.find().
var MongoDBQueryClassification = QueryClassification{
	ReadContains: []string{
		"db.", ".find(", ".findone(", ".aggregate(", ".count(",
		".countdocuments(", ".distinct(", ".estimateddocumentcount(",
		"listcollections", "getcollection",
	},
	WriteContains: []string{
		".insert(", ".insertone(", ".insertmany(",
		".update(", ".updateone(", ".updatemany(",
		".delete(", ".deleteone(", ".deletemany(",
		".remove(", ".drop(", ".dropcollection(",
		".createindex(", ".createcollection(", ".rename(",
		".findoneandupdate(", ".findoneandreplace(", ".findoneanddelete(",
		".bulkwrite(", ".replaceone(",
	},
}

// queryClassificationMap maps database type constants to their classification rules.
var queryClassificationMap = map[string]QueryClassification{
	DatabaseTypePostgreSQL:   PostgreSQLQueryClassification,
	DatabaseTypeYugabyteDB:   YugabyteDBQueryClassification,
	DatabaseTypeTimescaleDB:  PostgreSQLQueryClassification, // TimescaleDB extends PostgreSQL
	DatabaseTypeMySQL:        MySQLQueryClassification,
	DatabaseTypeStarRocks:    MySQLQueryClassification, // StarRocks is MySQL-wire-compatible
	DatabaseTypeClickhouse:   ClickHouseQueryClassification,
	DatabaseTypeMongoDB:      MongoDBQueryClassification,
	DatabaseTypeSpreadsheet:  SpreadsheetQueryClassification,
	DatabaseTypeGoogleSheets: GoogleSheetsQueryClassification,
}

// GetQueryClassification returns the QueryClassification for the given database type.
// Falls back to the generic SQL classification if the type is unknown.
func GetQueryClassification(dbType string) QueryClassification {
	if qc, ok := queryClassificationMap[dbType]; ok {
		return qc
	}
	// Fallback: generic SQL rules
	return QueryClassification{
		ReadPrefixes:  sqlReadPrefixes,
		WritePrefixes: sqlWritePrefixes,
	}
}

// IsReadOnlyQuery returns true if the given query is a read-only operation
// for the specified database type.
func IsReadOnlyQuery(query string, dbType string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(query))
	if trimmed == "" {
		return false
	}

	qc := GetQueryClassification(dbType)

	// MongoDB-style classification: uses Contains-based matching
	if len(qc.ReadContains) > 0 || len(qc.WriteContains) > 0 {
		// Check write operations first (reject)
		for _, op := range qc.WriteContains {
			if strings.Contains(trimmed, op) {
				return false
			}
		}
		// Check read operations (allow)
		for _, op := range qc.ReadContains {
			if strings.Contains(trimmed, op) {
				return true
			}
		}
		// Unknown MongoDB operation — reject
		return false
	}

	// SQL-style classification: uses prefix-based matching
	// Check write prefixes first (reject)
	for _, prefix := range qc.WritePrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return false
		}
	}
	// Check read prefixes (allow)
	for _, prefix := range qc.ReadPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	// Unknown SQL operation — reject
	return false
}

// IsWriteQuery returns true if the given query is a write/mutation operation
// for the specified database type.
func IsWriteQuery(query string, dbType string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(query))
	if trimmed == "" {
		return false
	}

	qc := GetQueryClassification(dbType)

	// MongoDB-style
	if len(qc.WriteContains) > 0 {
		for _, op := range qc.WriteContains {
			if strings.Contains(trimmed, op) {
				return true
			}
		}
		return false
	}

	// SQL-style
	for _, prefix := range qc.WritePrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
