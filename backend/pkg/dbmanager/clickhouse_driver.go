package dbmanager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

	clickhousedriver "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
)

// ClickHouseDriver implements the DatabaseDriver interface for ClickHouse
type ClickHouseDriver struct{}

// NewClickHouseDriver creates a new ClickHouse driver
func NewClickHouseDriver() DatabaseDriver {
	return &ClickHouseDriver{}
}

// Connect establishes a connection to a ClickHouse database
func (d *ClickHouseDriver) Connect(config ConnectionConfig) (*Connection, error) {
	var dsn string
	var tempFiles []string
	var sshTunnel *SSHTunnel
	var connectHost string

	// Establish SSH tunnel if configured
	if config.SSHEnabled && config.SSHHost != nil && config.SSHPort != nil && config.SSHUsername != nil {
		log.Printf("ClickHouseDriver -> Connect -> Establishing SSH tunnel for ClickHouse connection")

		// Determine SSH auth method
		authMethod := SSHAuthMethodPublicKey // Default
		if config.SSHAuthMethod != nil {
			authMethod = ToSSHAuthMethod(*config.SSHAuthMethod)
		}

		var tunnel *SSHTunnel
		var err error

		if authMethod == SSHAuthMethodPassword && config.SSHPassword != nil {
			// Use password-based authentication
			tunnel, err = CreateSSHTunnelWithPassword(*config.SSHHost, *config.SSHPort, *config.SSHUsername, *config.SSHPassword)
		} else {
			// Use public key authentication
			privateKey := ""
			if config.SSHPrivateKey != nil {
				privateKey = *config.SSHPrivateKey
			} else if config.SSHPrivateKeyURL != nil {
				// Load private key from URL
				var err error
				privateKey, err = LoadPrivateKeyFromURL(*config.SSHPrivateKeyURL)
				if err != nil {
					return nil, fmt.Errorf("failed to load SSH private key from URL: %v", err)
				}
			}

			if privateKey == "" {
				return nil, fmt.Errorf("SSH private key is required for public key authentication")
			}

			tunnel, err = CreateSSHTunnel(*config.SSHHost, *config.SSHPort, *config.SSHUsername, privateKey, getValue(config.SSHPassphrase))
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create SSH tunnel: %v", err)
		}

		sshTunnel = tunnel
		connectHost = "localhost" // When using SSH tunnel, connect through localhost
		defer func() {
			if err != nil {
				sshTunnel.Close()
			}
		}()
	} else {
		connectHost = config.Host
	}

	// Base connection parameters
	protocol := "tcp"

	// Configure SSL/TLS
	var tlsConfig *tls.Config
	if config.UseSSL {
		sslMode := "require"
		if config.SSLMode != nil {
			sslMode = *config.SSLMode
		}

		if sslMode == "disable" {
			tlsConfig = nil
		} else {
			// Fetch certificates from URLs
			certPath, keyPath, rootCertPath, certTempFiles, err := utils.PrepareCertificatesFromURLs(*config.SSLCertURL, *config.SSLKeyURL, *config.SSLRootCertURL)
			if err != nil {
				if sshTunnel != nil {
					sshTunnel.Close()
				}
				return nil, err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Create TLS config
			tlsConfig = &tls.Config{
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
					if sshTunnel != nil {
						sshTunnel.Close()
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
					if sshTunnel != nil {
						sshTunnel.Close()
					}
					return nil, fmt.Errorf("failed to read CA certificate: %v", err)
				}
				if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					if sshTunnel != nil {
						sshTunnel.Close()
					}
					return nil, fmt.Errorf("failed to append CA certificate")
				}
				tlsConfig.RootCAs = rootCertPool
			}
		}

		// Use secure protocol
		protocol = "https"
	}

	// Build DSN
	if config.Password != nil {
		dsn = fmt.Sprintf("%s://%s:%s@%s:%s/%s",
			protocol, *config.Username, *config.Password, connectHost, *config.Port, config.Database)
	} else {
		dsn = fmt.Sprintf("%s://%s@%s:%s/%s",
			protocol, *config.Username, connectHost, *config.Port, config.Database)
	}

	// Add parameters
	dsn += "?dial_timeout=10s&read_timeout=20s"

	// Create ClickHouse connection
	options := &clickhousedriver.Config{
		DSN: dsn,
	}

	// Add TLS config if SSL is enabled
	if config.UseSSL && tlsConfig != nil {
		// For ClickHouse, we need to add secure=true to the DSN
		// since the driver might not support direct TLS config
		dsn += "&secure=true"
		options.DSN = dsn
	}

	// Create GORM DB
	gormDB, err := gorm.Open(clickhousedriver.New(*options), &gorm.Config{})
	if err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		if sshTunnel != nil {
			sshTunnel.Close()
		}
		return nil, fmt.Errorf("failed to connect to ClickHouse: %v", err)
	}

	// Test connection
	sqlDB, err := gormDB.DB()
	if err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		return nil, fmt.Errorf("failed to get SQL DB: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		sqlDB.Close()
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

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

// Disconnect closes a ClickHouse database connection
func (d *ClickHouseDriver) Disconnect(conn *Connection) error {
	// Close SSH tunnel if present
	if conn.SSHTunnel != nil {
		if sshTunnel, ok := conn.SSHTunnel.(*SSHTunnel); ok {
			if err := sshTunnel.Close(); err != nil {
				log.Printf("ClickHouseDriver -> Disconnect -> Warning: Failed to close SSH tunnel: %v", err)
			}
		}
	}

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

// Ping checks if the ClickHouse connection is alive
func (d *ClickHouseDriver) Ping(conn *Connection) error {
	if conn == nil || conn.DB == nil {
		return fmt.Errorf("no active connection to ping")
	}

	// Get the underlying SQL DB
	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("ClickHouseDriver -> Ping -> Failed to get database connection: %v", err)
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	// First try standard ping
	if err := sqlDB.Ping(); err != nil {
		log.Printf("ClickHouseDriver -> Ping -> Standard ping failed: %v", err)
		return fmt.Errorf("ping failed: %v", err)
	}

	// Also execute a simple query to ensure the connection is fully functional
	var result int
	if err := conn.DB.Raw("SELECT 1").Scan(&result).Error; err != nil {
		log.Printf("ClickHouseDriver -> Ping -> Query test failed: %v", err)
		return fmt.Errorf("connection test query failed: %v", err)
	}

	log.Printf("ClickHouseDriver -> Ping -> Connection is healthy")
	return nil
}

// IsAlive checks if the ClickHouse connection is still valid
func (d *ClickHouseDriver) IsAlive(conn *Connection) bool {
	if conn == nil || conn.DB == nil {
		log.Printf("ClickHouseDriver -> IsAlive -> No connection or DB object")
		return false
	}

	// Get the underlying SQL DB
	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("ClickHouseDriver -> IsAlive -> Failed to get database connection: %v", err)
		return false
	}

	// First try standard ping
	if err := sqlDB.Ping(); err != nil {
		log.Printf("ClickHouseDriver -> IsAlive -> Standard ping failed: %v", err)
		return false
	}

	// Also execute a simple query to ensure the connection is fully functional
	var result int
	if err := conn.DB.Raw("SELECT 1").Scan(&result).Error; err != nil {
		log.Printf("ClickHouseDriver -> IsAlive -> Query test failed: %v", err)
		return false
	}

	log.Printf("ClickHouseDriver -> IsAlive -> Connection is healthy")
	return true
}

// ExecuteQuery executes a SQL query on the ClickHouse database
func (d *ClickHouseDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult {
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
	statements := splitClickHouseStatements(query)

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
			// For other queries (INSERT, CREATE, ALTER, etc.), execute and return affected rows
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
	// Store the result JSON in StreamData for streaming purposes
	result.StreamData = resultJSON

	return result
}

// BeginTx starts a new transaction
func (d *ClickHouseDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	if conn == nil || conn.DB == nil {
		log.Printf("ClickHouseDriver.BeginTx: Connection or DB is nil")
		return nil
	}

	// Start a new transaction
	tx := conn.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		log.Printf("Failed to begin transaction: %v", tx.Error)
		return nil
	}

	return &ClickHouseTransaction{
		tx:   tx,
		conn: conn,
	}
}

// GetSchema retrieves the database schema
func (d *ClickHouseDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a new ClickHouse schema fetcher
	fetcher := NewClickHouseSchemaFetcher(db)

	// Get the schema
	return fetcher.GetSchema(ctx, db, selectedTables)
}

// GetTableChecksum calculates a checksum for a table
func (d *ClickHouseDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseDriver -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	// Create a new ClickHouse schema fetcher
	fetcher := NewClickHouseSchemaFetcher(db)

	// Get the table checksum
	return fetcher.GetTableChecksum(ctx, db, table)
}

// FetchExampleRecords fetches example records from a table
func (d *ClickHouseDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseDriver -> FetchExampleRecords -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a new ClickHouse schema fetcher
	fetcher := NewClickHouseSchemaFetcher(db)

	// Get example records
	return fetcher.FetchExampleRecords(ctx, db, table, limit)
}
