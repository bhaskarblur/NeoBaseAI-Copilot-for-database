package dbmanager

import (
	"fmt"
	"strings"
)

// InjectCursorIntoSQLQuery handles cursor-based pagination for SQL databases
// (PostgreSQL, MySQL, YugabyteDB, ClickHouse).
//
// Two modes:
//
//   - cursor == "" (initial page): strips the {{cursor_value}} placeholder and its surrounding
//     WHERE/AND clause so the first page is returned without any cursor condition. This avoids
//     any assumption about the cursor field name.
//
//   - cursor != "" (subsequent pages): replaces {{cursor_value}} with the literal cursor string.
//     The cursor value is treated as a string; callers that know the type can wrap it themselves.
//
// If the query does not contain the placeholder it is returned unchanged.
func InjectCursorIntoSQLQuery(query, cursor string) string {
	const placeholder = "{{cursor_value}}"
	if !strings.Contains(query, placeholder) {
		return query
	}

	if cursor == "" {
		return sqlStripCursorCondition(query)
	}

	// Simple literal replacement — SQL drivers handle type binding separately.
	return strings.ReplaceAll(query, placeholder, cursor)
}

// sqlStripCursorCondition removes the WHERE / AND clause that contains {{cursor_value}} from a
// SQL query so that the first page is fetched without any cursor condition.
func sqlStripCursorCondition(query string) string {
	const placeholder = "{{cursor_value}}"
	upper := strings.ToUpper(query)
	_ = upper // used for case-insensitive keyword scanning below

	// Try to find "AND <field> <op> '{{cursor_value}}'" or "WHERE <field> <op> '{{cursor_value}}'"
	// We scan the uppercased query for the keyword boundary, then surgically remove that clause.

	// Find position of the placeholder in the original string.
	pos := strings.Index(query, placeholder)
	if pos == -1 {
		return query
	}

	// Walk back from pos to find the start of the WHERE/AND keyword that owns this condition.
	clauseStart := -1
	for i := pos - 1; i >= 0; i-- {
		sub := strings.ToUpper(query[i:])
		if strings.HasPrefix(sub, "AND ") || strings.HasPrefix(sub, "AND\t") || strings.HasPrefix(sub, "AND\n") {
			clauseStart = i
			break
		}
		if strings.HasPrefix(sub, "WHERE ") || strings.HasPrefix(sub, "WHERE\t") || strings.HasPrefix(sub, "WHERE\n") {
			clauseStart = i
			break
		}
	}
	if clauseStart == -1 {
		// Fallback: we couldn't find the keyword — return query without the placeholder line.
		return strings.ReplaceAll(query, placeholder, "1=1")
	}

	// The clause ends at the next AND/OR/ORDER/GROUP/LIMIT keyword (upper), or end-of-string.
	afterClause := pos + len(placeholder)
	// Consume trailing quote, whitespace, and the next logical connector.
	clauseEnd := len(query)
	terminators := []string{" AND ", " OR ", " ORDER ", " GROUP ", " LIMIT ", " HAVING "}
	for _, term := range terminators {
		idx := strings.Index(strings.ToUpper(query[afterClause:]), term)
		if idx >= 0 {
			end := afterClause + idx
			if end < clauseEnd {
				clauseEnd = end
			}
		}
	}

	// Determine what to replace: if keyword was WHERE and there is nothing else in the clause,
	// drop the WHERE entirely.  If it was AND, just drop the AND clause.
	keywordUpper := strings.ToUpper(query[clauseStart:])
	var prefix string
	if strings.HasPrefix(keywordUpper, "WHERE") {
		// Check if there are other conditions left after removing this one.
		remaining := strings.TrimSpace(query[clauseEnd:])
		remainingUpper := strings.ToUpper(remaining)
		if strings.HasPrefix(remainingUpper, "AND ") || strings.HasPrefix(remainingUpper, "AND\t") {
			// Convert the first remaining AND into WHERE.
			remaining = "WHERE " + remaining[4:]
		}
		prefix = query[:clauseStart]
		return fmt.Sprintf("%s%s", prefix, remaining)
	}
	// AND clause — just remove it.
	prefix = query[:clauseStart]
	suffix := query[clauseEnd:]
	return fmt.Sprintf("%s%s", prefix, suffix)
}
