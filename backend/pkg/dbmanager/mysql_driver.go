package dbmanager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/utils"
	"os"
	"strings"
	"sync"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MySQLDriver implements the DatabaseDriver interface for MySQL
type MySQLDriver struct{}

// NewMySQLDriver creates a new MySQL driver
func NewMySQLDriver() DatabaseDriver {
	return &MySQLDriver{}
}

// Connect establishes a connection to a MySQL database
func (d *MySQLDriver) Connect(config ConnectionConfig) (*Connection, error) {
	var dsn string
	var tempFiles []string

	// Base connection parameters
	if config.Password != nil {
		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s",
			*config.Username, *config.Password, config.Host, *config.Port, config.Database,
		)
	} else {
		dsn = fmt.Sprintf(
			"%s@tcp(%s:%s)/%s",
			*config.Username, config.Host, *config.Port, config.Database,
		)
	}

	// Add parameters
	dsn += "?parseTime=true"

	// Configure SSL/TLS
	if config.UseSSL {
		sslMode := "require"
		if config.SSLMode != nil {
			sslMode = *config.SSLMode
		}

		if sslMode == "disable" {
			// Do nothing
		} else {
			// Create a unique TLS config name
			tlsConfigName := fmt.Sprintf("custom-%d", time.Now().UnixNano())

			// Fetch certificates from URLs
			certPath, keyPath, rootCertPath, certTempFiles, err := utils.PrepareCertificatesFromURLs(*config.SSLCertURL, *config.SSLKeyURL, *config.SSLRootCertURL)
			if err != nil {
				return nil, err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Create TLS config
			tlsConfig := &tls.Config{
				ServerName: config.Host,
				MinVersion: tls.VersionTLS12,
			}

			// Set verification mode based on SSL mode
			switch sslMode {
			case "require":
				// Require encryption but don't verify certificates
				tlsConfig.InsecureSkipVerify = true
			case "verify-ca", "verify-full":
				// Verify certificates
				tlsConfig.InsecureSkipVerify = false

				// For verify-full, ensure ServerName is set for hostname verification
				if sslMode == "verify-full" {
					// ServerName is already set above
				}
			}

			// Add client certificates if provided
			if certPath != "" && keyPath != "" {
				cert, err := tls.LoadX509KeyPair(certPath, keyPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return nil, fmt.Errorf("failed to load client certificates: %v", err)
				}
				tlsConfig.Certificates = []tls.Certificate{cert}
			}

			// Add CA certificate if provided
			if rootCertPath != "" {
				rootCertPool := x509.NewCertPool()
				pem, err := ioutil.ReadFile(rootCertPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return nil, fmt.Errorf("failed to read CA certificate: %v", err)
				}
				if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return nil, fmt.Errorf("failed to append CA certificate")
				}
				tlsConfig.RootCAs = rootCertPool
			}

			// Register TLS config
			mysqldriver.RegisterTLSConfig(tlsConfigName, tlsConfig)

			// Add TLS config to DSN
			dsn += "&tls=" + tlsConfigName
		}
	}

	// Open connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		db.Close()
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Create GORM DB
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		DSN: dsn,
	}), &gorm.Config{})

	if err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		db.Close()
		return nil, fmt.Errorf("failed to create GORM connection: %v", err)
	}

	// Create connection object
	conn := &Connection{
		DB:          gormDB,
		LastUsed:    time.Now(),
		Status:      StatusConnected,
		Config:      config,
		Subscribers: make(map[string]bool),
		SubLock:     sync.RWMutex{},
		TempFiles:   tempFiles,
	}

	return conn, nil
}

// Disconnect closes a MySQL database connection
func (d *MySQLDriver) Disconnect(conn *Connection) error {
	// Get the underlying SQL DB
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %v", err)
	}

	// Close the connection
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %v", err)
	}

	// Clean up temporary certificate files
	for _, file := range conn.TempFiles {
		os.Remove(file)
	}

	return nil
}

// Ping checks if the MySQL connection is alive
func (d *MySQLDriver) Ping(conn *Connection) error {
	if conn == nil || conn.DB == nil {
		return fmt.Errorf("no active connection to ping")
	}

	sqlDB, err := conn.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	return sqlDB.Ping()
}

// IsAlive checks if the MySQL connection is still valid
func (d *MySQLDriver) IsAlive(conn *Connection) bool {
	if conn == nil || conn.DB == nil {
		return false
	}

	sqlDB, err := conn.DB.DB()
	if err != nil {
		return false
	}

	return sqlDB.Ping() == nil
}

// ExecuteQuery executes a SQL query on the MySQL database
func (d *MySQLDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult {
	if conn == nil || conn.DB == nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "No active connection",
				Code:    "CONNECTION_ERROR",
			},
		}
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
			return result
		}

		// Execute the statement based on query type
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "SELECT") ||
			strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "SHOW") ||
			strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "DESCRIBE") {
			// For SELECT, SHOW, DESCRIBE queries, return the results
			var rows []map[string]interface{}
			if err := conn.DB.WithContext(ctx).Raw(stmt).Scan(&rows).Error; err != nil {
				result.Error = &dtos.QueryError{
					Message: err.Error(),
					Code:    "EXECUTION_ERROR",
				}
				return result
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
			execResult := conn.DB.WithContext(ctx).Exec(stmt)
			if execResult.Error != nil {
				result.Error = &dtos.QueryError{
					Message: execResult.Error.Error(),
					Code:    "EXECUTION_ERROR",
				}
				return result
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
		}
	}
	result.StreamData = resultJSON

	return result
}

// BeginTx starts a new transaction
func (d *MySQLDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	if conn == nil || conn.DB == nil {
		log.Printf("MySQLDriver.BeginTx: Connection or DB is nil")
		return nil
	}

	// Start a new transaction
	tx := conn.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		log.Printf("Failed to begin transaction: %v", tx.Error)
		return nil
	}

	return &MySQLTransaction{
		tx:   tx,
		conn: conn,
	}
}

// GetSchema retrieves the database schema
func (d *MySQLDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MySQLDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a new MySQL schema fetcher
	fetcher := NewMySQLSchemaFetcher(db)

	// Get the schema
	return fetcher.GetSchema(ctx, db, selectedTables)
}

// GetTableChecksum calculates a checksum for a table
func (d *MySQLDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MySQLDriver -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	// Create a new MySQL schema fetcher
	fetcher := NewMySQLSchemaFetcher(db)

	// Get the table checksum
	return fetcher.GetTableChecksum(ctx, db, table)
}

// FetchExampleRecords fetches example records from a table
func (d *MySQLDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MySQLDriver -> FetchExampleRecords -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a new MySQL schema fetcher
	fetcher := NewMySQLSchemaFetcher(db)

	// Get example records
	return fetcher.FetchExampleRecords(ctx, db, table, limit)
}

