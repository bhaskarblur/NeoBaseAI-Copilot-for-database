package dbmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"neobase-ai/internal/apis/dtos"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MySQLTransaction implements the Transaction interface for MySQL
type MySQLTransaction struct {
	tx   *gorm.DB
	conn *Connection
}

// ExecuteQuery executes a query within a transaction
func (t *MySQLTransaction) ExecuteQuery(ctx context.Context, query string) (*QueryExecutionResult, error) {
	if t.tx == nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "No active transaction",
				Code:    "TRANSACTION_ERROR",
			},
		}, nil
	}

	startTime := time.Now()
	result := &QueryExecutionResult{}

	// Split the query into individual statements
	statements := splitMySQLStatements(query)

	// Execute each statement
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			result.Error = &dtos.QueryError{
				Message: "Query execution cancelled",
				Code:    "EXECUTION_CANCELLED",
			}
			return result, nil
		}

		// Execute the statement based on query type
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "SELECT") ||
			strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "SHOW") ||
			strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "DESCRIBE") {
			// For SELECT, SHOW, DESCRIBE queries, return the results
			var rows []map[string]interface{}
			if err := t.tx.WithContext(ctx).Raw(stmt).Scan(&rows).Error; err != nil {
				result.Error = &dtos.QueryError{
					Message: err.Error(),
					Code:    "EXECUTION_ERROR",
				}
				return result, nil
			}

			// Process the rows to ensure proper type handling
			processedRows := make([]map[string]interface{}, len(rows))
			for i, row := range rows {
				processedRow := make(map[string]interface{})
				for key, val := range row {
					// Handle different types properly
					switch v := val.(type) {
					case []byte:
						// Convert []byte to string
						processedRow[key] = string(v)
					case string:
						// Keep strings as is
						processedRow[key] = v
					case float64:
						// Keep numbers as is
						processedRow[key] = v
					case int64:
						// Keep integers as is
						processedRow[key] = v
					case bool:
						// Keep booleans as is
						processedRow[key] = v
					case nil:
						// Keep nulls as is
						processedRow[key] = nil
					default:
						// For other types, convert to string
						processedRow[key] = fmt.Sprintf("%v", v)
					}
				}
				processedRows[i] = processedRow
			}

			result.Result = map[string]interface{}{
				"results": processedRows,
			}
		} else {
			// For other queries (INSERT, UPDATE, DELETE, etc.), execute and return affected rows
			execResult := t.tx.WithContext(ctx).Exec(stmt)
			if execResult.Error != nil {
				result.Error = &dtos.QueryError{
					Message: execResult.Error.Error(),
					Code:    "EXECUTION_ERROR",
				}
				return result, nil
			}

			rowsAffected := execResult.RowsAffected
			if rowsAffected > 0 {
				result.Result = map[string]interface{}{
					"rowsAffected": rowsAffected,
					"message":      fmt.Sprintf("%d row(s) affected", rowsAffected),
				}
			} else {
				result.Result = map[string]interface{}{
					"message": "Query performed successfully",
				}
			}
		}
	}

	// Calculate execution time
	executionTime := int(time.Since(startTime).Milliseconds())
	result.ExecutionTime = executionTime

	// Marshal the result to JSON
	resultJSON, err := json.Marshal(result.Result)
	if err != nil {
		return &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Error: &dtos.QueryError{
				Code:    "JSON_MARSHAL_FAILED",
				Message: err.Error(),
				Details: "Failed to marshal query results",
			},
		}, nil
	}
	result.StreamData = resultJSON

	return result, nil
}

// Commit commits the transaction
func (t *MySQLTransaction) Commit() error {
	if t.tx == nil {
		return fmt.Errorf("no active transaction to commit")
	}
	return t.tx.Commit().Error
}

// Rollback rolls back the transaction
func (t *MySQLTransaction) Rollback() error {
	if t.tx == nil {
		return fmt.Errorf("no active transaction to rollback")
	}
	return t.tx.Rollback().Error
}
