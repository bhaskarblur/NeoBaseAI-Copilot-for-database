package dbmanager

import (
	"log"
	"neobase-ai/internal/constants"
	"strconv"
	"strings"
)

// BuildCursorQuery is the unified entry point for cursor-based pagination across all DB types.
//
// Page 1 (cursorValue==""): returns baseQuery unchanged — a clean first-page query with no cursor condition.
//
// Pages 2+ (cursorValue!=""): replaces {{cursor_value}} in paginatedQuery with the formatted cursor value.
//
// IMPORTANT: baseQuery and paginatedQuery should be DIFFERENT queries:
//   - baseQuery = first-page query (no cursor placeholder, e.g. "SELECT * FROM users ORDER BY id LIMIT 50")
//   - paginatedQuery = template for pages 2+ (with {{cursor_value}}, e.g. "SELECT * FROM users WHERE id > '{{cursor_value}}' ORDER BY id LIMIT 50")
//
// The AI generates both queries. This function simply uses the right one based on whether a cursor value is present.
func BuildCursorQuery(dbType, baseQuery, paginatedQuery, cursorField, cursorDirection, cursorValue string) string {
	if cursorValue == "" {
		return baseQuery
	}

	const placeholder = "{{cursor_value}}"

	// Template-based replacement: AI included {{cursor_value}} in paginatedQuery
	if paginatedQuery != "" && strings.Contains(paginatedQuery, placeholder) {
		log.Printf("[CURSOR] Replacing {{cursor_value}} in paginatedQuery for %s", dbType)
		switch dbType {
		case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeMySQL,
			constants.DatabaseTypeYugabyteDB, constants.DatabaseTypeTimescaleDB,
			constants.DatabaseTypeStarRocks, constants.DatabaseTypeClickhouse:
			return strings.ReplaceAll(paginatedQuery, placeholder, sqlFormatCursorValue(cursorValue))
		default:
			return mongoInjectTemplatedCursor(paginatedQuery, cursorValue)
		}
	}

	// Fallback: no {{cursor_value}} in paginatedQuery.
	// For MongoDB, dynamically inject a cursor condition into the base query.
	if (dbType == constants.DatabaseTypeMongoDB) && cursorField != "" {
		log.Printf("[CURSOR] No {{cursor_value}} in template — attempting dynamic cursor injection (field=%s, dir=%s)", cursorField, cursorDirection)
		if injected := mongoDynamicCursorInject(baseQuery, cursorField, cursorDirection, cursorValue); injected != baseQuery {
			return injected
		}
	}

	log.Printf("[CURSOR] WARNING: no {{cursor_value}} in paginatedQuery — returning baseQuery unchanged")
	return baseQuery
}

// ---------------------------------------------------------------------------
// SQL helpers
// ---------------------------------------------------------------------------

// sqlFormatCursorValue returns the cursor value formatted for SQL.
// Numeric values are left unquoted; strings are single-quoted with escaping.
func sqlFormatCursorValue(value string) string {
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
