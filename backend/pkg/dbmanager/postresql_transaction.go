package dbmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"strings"
	"time"
)

type PostgresTransaction struct {
	tx   *sql.Tx
	conn *Connection // Add connection reference
}

func (tx *PostgresTransaction) ExecuteQuery(ctx context.Context, query string) (*QueryExecutionResult, error) {
	startTime := time.Now()

	// Split into individual statements
	statements := splitStatements(query)
	log.Printf("PostgreSQL Transaction -> ExecuteQuery -> Statements: %v", statements)

	var lastResult sql.Result
	var rows *sql.Rows
	var err error

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// For SELECT queries
		if strings.HasPrefix(strings.ToUpper(stmt), "SELECT") {
			rows, err = tx.tx.QueryContext(ctx, stmt)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Code:    "QUERY_EXECUTION_FAILED",
						Message: err.Error(),
						Details: fmt.Sprintf("Failed to execute SELECT: %s", stmt),
					},
				}, nil
			}
		} else {
			// For non-SELECT queries
			lastResult, err = tx.tx.ExecContext(ctx, stmt)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Code:    "QUERY_EXECUTION_FAILED",
						Message: err.Error(),
						Details: fmt.Sprintf("Failed to execute statement: %s", stmt),
					},
				}, nil
			}

			// Check for specific PostgreSQL errors
			if strings.Contains(strings.ToUpper(stmt), "DROP TABLE") {
				// Extract table name
				tableName := extractTableName(stmt)
				log.Printf("PostgresDriver -> ExecuteQuery -> Checking existence of table: %s", tableName)

				// Check if table exists before dropping
				var exists bool
				checkStmt := `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`

				err = tx.tx.QueryRow(checkStmt, tableName).Scan(&exists)
				if err != nil {
					log.Printf("PostgresDriver -> ExecuteQuery -> Error checking table existence: %v", err)
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Code:    "TABLE_NOT_FOUND",
							Message: err.Error(),
							Details: "Cannot drop a table that doesn't exist",
						},
					}, nil
				}

				log.Printf("PostgresDriver -> ExecuteQuery -> Table '%s' exists, proceeding with DROP", tableName)
			}
		}
	}

	// Process results
	result := &QueryExecutionResult{
		ExecutionTime: int(time.Since(startTime).Milliseconds()),
	}

	if rows != nil {
		defer rows.Close()
		results, err := processRows(rows, startTime)
		if err != nil {
			return &QueryExecutionResult{
				ExecutionTime: int(time.Since(startTime).Milliseconds()),
				Error: &dtos.QueryError{
					Code:    "RESULT_PROCESSING_FAILED",
					Message: err.Error(),
					Details: "Failed to process query results",
				},
			}, nil
		}
		result.Result = map[string]interface{}{
			"results": results,
		}
	} else if lastResult != nil {
		rowsAffected, _ := lastResult.RowsAffected()
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

func (t *PostgresTransaction) Commit() error {
	log.Printf("PostgreSQL Transaction -> Commit -> Committing transaction")
	return t.tx.Commit()
}

func (t *PostgresTransaction) Rollback() error {
	log.Printf("PostgreSQL Transaction -> Rollback -> Rolling back transaction")
	return t.tx.Rollback()
}
