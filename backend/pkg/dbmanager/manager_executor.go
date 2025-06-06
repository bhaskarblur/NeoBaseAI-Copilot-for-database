package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	// Database drivers
	mysqldriver "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq" // PostgreSQL/YugabyteDB Driver

	"crypto/tls"
	"crypto/x509"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/utils"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type QueryExecution struct {
	QueryID     string
	MessageID   string
	StartTime   time.Time
	IsExecuting bool
	IsRollback  bool
	Tx          Transaction // Changed from *sql.Tx to Transaction
	CancelFunc  context.CancelFunc
}

func (m *Manager) CancelQueryExecution(streamID string) {
	m.executionMu.Lock()
	defer m.executionMu.Unlock()

	if execution, exists := m.activeExecutions[streamID]; exists {
		log.Printf("Cancelling query execution for streamID: %s", streamID)

		// Cancel the context first
		execution.CancelFunc()

		// Rollback transaction if it exists
		if execution.Tx != nil {
			if err := execution.Tx.Rollback(); err != nil {
				log.Printf("Error rolling back transaction: %v", err)
			}
		}

		delete(m.activeExecutions, streamID)
		log.Printf("Query execution cancelled for streamID: %s", streamID)
	}
}

// ExecuteQuery executes a query and returns the result, synchronous, no SSE events are sent, findCount is used to strictly get the number/count of records that the query returns
func (m *Manager) ExecuteQuery(ctx context.Context, chatID, messageID, queryID, streamID string, query string, queryType string, isRollback bool, findCount bool) (*QueryExecutionResult, *dtos.QueryError) {
	m.executionMu.Lock()

	// Create cancellable context with timeout
	execCtx, cancel := context.WithTimeout(ctx, 1*time.Minute) // 1 minute timeout

	// Track execution
	execution := &QueryExecution{
		QueryID:     queryID,
		MessageID:   messageID,
		StartTime:   time.Now(),
		IsExecuting: true,
		IsRollback:  isRollback,
		CancelFunc:  cancel,
	}
	m.activeExecutions[streamID] = execution
	m.executionMu.Unlock()

	// Ensure cleanup
	defer func() {
		m.executionMu.Lock()
		delete(m.activeExecutions, streamID)
		m.executionMu.Unlock()
		cancel()
	}()

	// Get connection and driver
	conn, exists := m.connections[chatID]
	if !exists {
		return nil, &dtos.QueryError{
			Code:    "NO_CONNECTION_FOUND",
			Message: "no connection found",
			Details: "No connection found for chat ID: " + chatID,
		}
	}

	driver, exists := m.drivers[conn.Config.Type]
	if !exists {
		return nil, &dtos.QueryError{
			Code:    "NO_DRIVER_FOUND",
			Message: "no driver found",
			Details: "No driver found for type: " + conn.Config.Type,
		}
	}

	log.Printf("Manager -> ExecuteQuery -> Driver: %v", driver)
	// Begin transaction
	tx := driver.BeginTx(execCtx, conn)
	if tx == nil {
		return nil, &dtos.QueryError{
			Code:    "FAILED_TO_START_TRANSACTION",
			Message: "failed to start transaction",
			Details: "Failed to start transaction",
		}
	}

	// Check if transaction has an error (MongoDB transaction might return a non-nil transaction with an error)
	if mongoTx, ok := tx.(*MongoDBTransaction); ok && mongoTx.Error != nil {
		log.Printf("Manager -> ExecuteQuery -> MongoDB transaction error: %v", mongoTx.Error)
		return nil, &dtos.QueryError{
			Code:    "FAILED_TO_START_TRANSACTION",
			Message: "failed to start transaction",
			Details: mongoTx.Error.Error(),
		}
	}

	execution.Tx = tx

	// Execute query with proper cancellation handling
	var result *QueryExecutionResult
	done := make(chan struct{})
	var queryErr *dtos.QueryError

	go func() {
		defer close(done)
		log.Printf("Manager -> ExecuteQuery -> Executing query: %v", query)
		result = tx.ExecuteQuery(execCtx, conn, query, queryType, findCount)
		// log.Printf("Manager -> ExecuteQuery -> Result: %v", result)
		if result.Error != nil {
			queryErr = result.Error
		}
	}()

	select {
	case <-execCtx.Done():
		if err := tx.Rollback(); err != nil {
			log.Printf("Error rolling back transaction: %v", err)
		}
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, &dtos.QueryError{
				Code:    "QUERY_EXECUTION_TIMED_OUT",
				Message: "query execution timed out",
				Details: "Query execution timed out",
			}
		}
		return nil, &dtos.QueryError{
			Code:    "QUERY_EXECUTION_CANCELLED",
			Message: "query execution cancelled",
			Details: "Query execution cancelled",
		}

	case <-done:
		if queryErr != nil {
			if err := tx.Rollback(); err != nil {
				log.Printf("Error rolling back transaction: %v", err)
			}
			return result, queryErr
		}
		if err := tx.Commit(); err != nil {
			return nil, &dtos.QueryError{
				Code:    "QUERY_EXECUTION_FAILED",
				Message: "query execution failed",
				Details: err.Error(),
			}
		}
		log.Println("Manager -> ExecuteQuery -> Commit completed:")
		log.Printf("Manager -> ExecuteQuery -> Query type: %v", queryType)

		go func() {
			log.Println("Manager -> ExecuteQuery -> Checking if schema trigger is needed")
			time.Sleep(2 * time.Second)
			switch conn.Config.Type {
			case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB:
				if queryType == "DDL" || queryType == "ALTER" || queryType == "DROP" {
					if conn.OnSchemaChange != nil {
						conn.OnSchemaChange(conn.ChatID)
					}
				}
			case constants.DatabaseTypeMySQL:
				if queryType == "DDL" || queryType == "ALTER" || queryType == "DROP" {
					if conn.OnSchemaChange != nil {
						conn.OnSchemaChange(conn.ChatID)
					}
				}
			case constants.DatabaseTypeClickhouse:
				if queryType == "DDL" || queryType == "ALTER" || queryType == "DROP" {
					if conn.OnSchemaChange != nil {
						conn.OnSchemaChange(conn.ChatID)
					}
				}
			case constants.DatabaseTypeMongoDB:
				if queryType == "CREATE_COLLECTION" || queryType == "DROP_COLLECTION" {
					if conn.OnSchemaChange != nil {
						conn.OnSchemaChange(conn.ChatID)
					}
				}
			}
		}()

		return result, nil
	}
}

// TestConnection tests if the provided credentials are valid without creating a persistent connection
func (m *Manager) TestConnection(config *ConnectionConfig) error {
	var tempFiles []string

	switch config.Type {
	case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB:
		var dsn string
		port := "5432" // Default port
		if config.Type == constants.DatabaseTypeYugabyteDB {
			port = "5433" // Default port
		}

		if config.Port != nil && *config.Port != "" {
			port = *config.Port
		}
		// Base connection parameters
		baseParams := fmt.Sprintf(
			"host=%s port=%s user=%s dbname=%s",
			config.Host, port, *config.Username, config.Database,
		)

		// Add password if provided
		if config.Password != nil {
			baseParams += fmt.Sprintf(" password=%s", *config.Password)
		}

		// Configure SSL/TLS
		if config.UseSSL {
			// Always use verify-full mode for maximum security
			if config.SSLMode != nil {
				baseParams += fmt.Sprintf(" sslmode=%s", *config.SSLMode)
			} else {
				baseParams += " sslmode=require"
			}

			// Fetch certificates from URLs
			certPath, keyPath, rootCertPath, certTempFiles, err := utils.PrepareCertificatesFromURLs(*config.SSLCertURL, *config.SSLKeyURL, *config.SSLRootCertURL)
			if err != nil {
				return err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Add certificate paths to connection string
			if certPath != "" {
				baseParams += fmt.Sprintf(" sslcert=%s", certPath)
			}

			if keyPath != "" {
				baseParams += fmt.Sprintf(" sslkey=%s", keyPath)
			}

			if rootCertPath != "" {
				baseParams += fmt.Sprintf(" sslrootcert=%s", rootCertPath)
			}
		} else {
			baseParams += " sslmode=disable"
		}

		dsn = baseParams

		// Open connection
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			// Clean up temporary files
			for _, file := range tempFiles {
				os.Remove(file)
			}
			return fmt.Errorf("failed to create connection: %v", err)
		}

		// Test connection
		err = db.Ping()

		// Close connection
		db.Close()

		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}

		if err != nil {
			return err
		}

		return nil

	case constants.DatabaseTypeMySQL:
		var dsn string
		port := "3306" // Default port for MySQL

		if config.Port != nil && *config.Port != "" {
			port = *config.Port
		}

		// Base connection parameters
		if config.Password != nil {
			dsn = fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s",
				*config.Username, *config.Password, config.Host, port, config.Database,
			)
		} else {
			dsn = fmt.Sprintf(
				"%s@tcp(%s:%s)/%s",
				*config.Username, config.Host, port, config.Database,
			)
		}

		// Add parameters
		dsn += "?parseTime=true"

		// Configure SSL/TLS
		if config.UseSSL {
			// Create a unique TLS config name
			tlsConfigName := fmt.Sprintf("custom-test-%d", time.Now().UnixNano())

			// Fetch certificates from URLs
			certPath, keyPath, rootCertPath, certTempFiles, err := utils.PrepareCertificatesFromURLs(*config.SSLCertURL, *config.SSLKeyURL, *config.SSLRootCertURL)
			if err != nil {
				return err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Create TLS config
			tlsConfig := &tls.Config{
				ServerName: config.Host,
				MinVersion: tls.VersionTLS12,
			}

			// Add client certificates if provided
			if certPath != "" && keyPath != "" {
				cert, err := tls.LoadX509KeyPair(certPath, keyPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to load client certificates: %v", err)
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
					return fmt.Errorf("failed to read CA certificate: %v", err)
				}
				if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to append CA certificate")
				}
				tlsConfig.RootCAs = rootCertPool
			}

			// Register TLS config
			mysqldriver.RegisterTLSConfig(tlsConfigName, tlsConfig)

			// Add TLS config to DSN
			dsn += "&tls=" + tlsConfigName
		}

		// Open connection
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			// Clean up temporary files
			for _, file := range tempFiles {
				os.Remove(file)
			}
			return fmt.Errorf("failed to create connection: %v", err)
		}

		// Test connection
		err = db.Ping()

		// Close connection
		db.Close()

		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}

		if err != nil {
			return fmt.Errorf("failed to connect to database: %v", err)
		}

		return nil

	case constants.DatabaseTypeClickhouse:
		var dsn string
		port := "9000" // Default port for ClickHouse

		if config.Port != nil && *config.Port != "" {
			port = *config.Port
		}

		// Base connection parameters
		protocol := "tcp"

		// Configure SSL/TLS
		if config.UseSSL {
			// Fetch certificates from URLs
			_, _, _, certTempFiles, err := utils.PrepareCertificatesFromURLs(*config.SSLCertURL, *config.SSLKeyURL, *config.SSLRootCertURL)
			if err != nil {
				return err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Use secure protocol
			protocol = "https"
		}

		// Build DSN
		if config.Password != nil {
			dsn = fmt.Sprintf("%s://%s:%s@%s:%s/%s",
				protocol, *config.Username, *config.Password, config.Host, port, config.Database)
		} else {
			dsn = fmt.Sprintf("%s://%s@%s:%s/%s",
				protocol, *config.Username, config.Host, port, config.Database)
		}

		// Add parameters
		dsn += "?dial_timeout=10s&read_timeout=20s"

		// Add secure parameter if using SSL
		if config.UseSSL {
			dsn += "&secure=true"
		}

		// Open connection
		db, err := sql.Open("clickhouse", dsn)
		if err != nil {
			// Clean up temporary files
			for _, file := range tempFiles {
				os.Remove(file)
			}
			return fmt.Errorf("failed to create connection: %v", err)
		}

		// Test connection
		err = db.Ping()

		// Close connection
		db.Close()

		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}

		if err != nil {
			return fmt.Errorf("failed to connect to database: %v", err)
		}

		return nil

	case constants.DatabaseTypeMongoDB:
		var port string
		if config.Port != nil && *config.Port != "" {
			port = *config.Port
		} else {
			port = "27017" // Default port for MongoDB
		}
		log.Printf("DBManager -> TestConnection -> Testing MongoDB connection at %s:%s", config.Host, port)

		var uri string

		// Check if we're using SRV records (mongodb+srv://)
		isSRV := strings.Contains(config.Host, ".mongodb.net")
		protocol := "mongodb"
		if isSRV {
			protocol = "mongodb+srv"
		}

		// Validate port value if not using SRV
		if !isSRV && config.Port != nil {
			// Log the port value for debugging
			log.Printf("DBManager -> TestConnection -> Port value before validation: %v", *config.Port)

			// Check if port is empty
			if *config.Port == "" {
				log.Printf("DBManager -> TestConnection -> Port is empty, using default port 27017")
			} else {
				port = *config.Port

				// Only validate port as numeric if it doesn't contain base64 characters
				// (which would indicate it's encrypted)
				if !strings.Contains(port, "+") && !strings.Contains(port, "/") && !strings.Contains(port, "=") {
					// Verify port is numeric
					if _, err := strconv.Atoi(port); err != nil {
						log.Printf("DBManager -> TestConnection -> Non-numeric port value: %v, might be encrypted", port)
						// Don't return error for potentially encrypted ports
					}
				}
			}
		}

		// Base connection parameters with authentication
		if config.Username != nil && *config.Username != "" {
			// URL encode username and password to handle special characters
			encodedUsername := url.QueryEscape(*config.Username)
			encodedPassword := url.QueryEscape(*config.Password)

			if isSRV {
				// For SRV records, don't include port
				uri = fmt.Sprintf("%s://%s:%s@%s/%s",
					protocol, encodedUsername, encodedPassword, config.Host, config.Database)
			} else {
				// Include port for standard connections
				uri = fmt.Sprintf("%s://%s:%s@%s:%s/%s",
					protocol, encodedUsername, encodedPassword, config.Host, port, config.Database)
			}
		} else {
			// Without authentication
			if isSRV {
				// For SRV records, don't include port
				uri = fmt.Sprintf("%s://%s/%s", protocol, config.Host, config.Database)
			} else {
				// Include port for standard connections
				uri = fmt.Sprintf("%s://%s:%s/%s", protocol, config.Host, port, config.Database)
			}
		}

		// Add connection options
		if isSRV {
			uri += "?retryWrites=true&w=majority"
		}

		// Add authSource parameter if specified
		if config.AuthDatabase != nil && *config.AuthDatabase != "" {
			if strings.Contains(uri, "?") {
				uri += "&authSource=" + url.QueryEscape(*config.AuthDatabase)
			} else {
				uri += "?authSource=" + url.QueryEscape(*config.AuthDatabase)
			}
		}
		
		// Log the final URI (with sensitive parts masked)
		maskedUri := uri
		if config.Password != nil && *config.Password != "" {
			maskedUri = strings.Replace(maskedUri, *config.Password, "********", -1)
		}
		log.Printf("DBManager -> TestConnection -> Connection URI: %s", maskedUri)

		// Configure client options
		clientOptions := options.Client().ApplyURI(uri)

		// Configure SSL/TLS
		if config.UseSSL {
			// Fetch certificates from URLs
			certPath, keyPath, rootCertPath, certTempFiles, err := utils.PrepareCertificatesFromURLs(*config.SSLCertURL, *config.SSLKeyURL, *config.SSLRootCertURL)
			if err != nil {
				return err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Configure TLS
			tlsConfig := &tls.Config{
				InsecureSkipVerify: false, // Default: verify certificates
			}

			// Apply SSL mode if specified
			if config.SSLMode != nil {
				switch *config.SSLMode {
				case "disable":
					// Don't use TLS at all
					// MongoDB driver doesn't have a direct SetTLS(nil) method,
					// so we'll skip setting TLS config at all
					goto skipTLSConfig
				case "require":
					// Require encryption but don't verify server certificates
					tlsConfig.InsecureSkipVerify = true
				case "verify-ca":
					// Verify server certificate but not hostname
					tlsConfig.InsecureSkipVerify = false
					// Clear ServerName to skip hostname verification
					tlsConfig.ServerName = ""
				case "verify-full":
					// Full verification including hostname
					tlsConfig.InsecureSkipVerify = false
					tlsConfig.ServerName = config.Host
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
					return fmt.Errorf("failed to load client certificates: %v", err)
				}
				tlsConfig.Certificates = []tls.Certificate{cert}
			}

			// Add root CA if provided
			if rootCertPath != "" {
				rootCA, err := os.ReadFile(rootCertPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to read root CA: %v", err)
				}

				rootCertPool := x509.NewCertPool()
				if ok := rootCertPool.AppendCertsFromPEM(rootCA); !ok {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to parse root CA certificate")
				}

				tlsConfig.RootCAs = rootCertPool
			}

			clientOptions.SetTLSConfig(tlsConfig)
		}

	skipTLSConfig:
		// Connect to MongoDB with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			// Clean up temporary files
			for _, file := range tempFiles {
				os.Remove(file)
			}
			log.Printf("DBManager -> TestConnection -> Error connecting to MongoDB: %v", err)
			return fmt.Errorf("failed to connect to MongoDB: %v", err)
		}

		// Ping the database to verify connection
		err = client.Ping(ctx, readpref.Primary())

		// Disconnect regardless of ping result
		client.Disconnect(ctx)

		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}

		if err != nil {
			log.Printf("DBManager -> TestConnection -> Error pinging MongoDB: %v", err)
			return fmt.Errorf("failed to ping MongoDB: %v", err)
		}

		log.Printf("DBManager -> TestConnection -> Successfully connected to MongoDB")
		return nil

	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

// FormatSchemaWithExamples formats the schema with example records for LLM
func (m *Manager) FormatSchemaWithExamples(ctx context.Context, chatID string, selectedCollections []string) (string, error) {
	log.Printf("DBManager -> FormatSchemaWithExamples -> Starting for chatID: %s with selected collections: %v", chatID, selectedCollections)

	// Get connection with read lock to ensure thread safety
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("DBManager -> FormatSchemaWithExamples -> Connection not found for chatID: %s", chatID)
		return "", fmt.Errorf("connection not found for chat ID: %s", chatID)
	}

	// Get database executor
	db, err := m.GetConnection(chatID)
	if err != nil {
		log.Printf("DBManager -> FormatSchemaWithExamples -> Error getting executor: %v", err)
		return "", fmt.Errorf("failed to get database executor: %v", err)
	}

	// Use schema manager to format schema with examples and selected collections
	formattedSchema, err := m.schemaManager.FormatSchemaWithExamplesAndCollections(ctx, chatID, db, conn.Config.Type, selectedCollections)
	if err != nil {
		log.Printf("DBManager -> FormatSchemaWithExamples -> Error formatting schema: %v", err)
		return "", fmt.Errorf("failed to format schema with examples: %v", err)
	}

	log.Printf("DBManager -> FormatSchemaWithExamples -> Successfully formatted schema for chatID: %s", chatID)
	return formattedSchema, nil
}

// GetSchemaWithExamples gets the schema with example records
func (m *Manager) GetSchemaWithExamples(ctx context.Context, chatID string, selectedCollections []string) (*SchemaStorage, error) {
	log.Printf("DBManager -> GetSchemaWithExamples -> Starting for chatID: %s with selected collections: %v", chatID, selectedCollections)

	// Get connection with read lock to ensure thread safety
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("DBManager -> GetSchemaWithExamples -> Connection not found for chatID: %s", chatID)
		return nil, fmt.Errorf("connection not found for chat ID: %s", chatID)
	}

	// Get database executor
	db, err := m.GetConnection(chatID)
	if err != nil {
		log.Printf("DBManager -> GetSchemaWithExamples -> Error getting executor: %v", err)
		return nil, fmt.Errorf("failed to get database executor: %v", err)
	}

	// Use schema manager to get schema with examples
	storage, err := m.schemaManager.GetSchemaWithExamples(ctx, chatID, db, conn.Config.Type, selectedCollections)
	if err != nil {
		log.Printf("DBManager -> GetSchemaWithExamples -> Error getting schema: %v", err)
		return nil, fmt.Errorf("failed to get schema with examples: %v", err)
	}

	log.Printf("DBManager -> GetSchemaWithExamples -> Successfully retrieved schema for chatID: %s", chatID)
	return storage, nil
}

// RefreshSchemaWithExamples refreshes the schema and returns it with example records
func (m *Manager) RefreshSchemaWithExamples(ctx context.Context, chatID string, selectedCollections []string) (string, error) {
	log.Printf("DBManager -> RefreshSchemaWithExamples -> Starting for chatID: %s with selected collections: %v", chatID, selectedCollections)

	// Create a new context with a longer timeout specifically for this operation
	schemaCtx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	// Get connection with read lock to ensure thread safety
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Connection not found for chatID: %s", chatID)
		return "", fmt.Errorf("connection not found for chat ID: %s", chatID)
	}

	// Get database executor
	db, err := m.GetConnection(chatID)
	if err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Error getting executor: %v", err)
		return "", fmt.Errorf("failed to get database executor: %v", err)
	}

	// Clear schema cache to force refresh
	m.schemaManager.ClearSchemaCache(chatID)
	log.Printf("DBManager -> RefreshSchemaWithExamples -> Cleared schema cache for chatID: %s", chatID)

	// Check for context cancellation
	if err := schemaCtx.Err(); err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Context cancelled: %v", err)
		return "", fmt.Errorf("operation cancelled: %v", err)
	}

	// Force a fresh schema fetch by directly calling GetSchema first
	log.Printf("DBManager -> RefreshSchemaWithExamples -> Forcing fresh schema fetch for chatID: %s", chatID)

	// Convert selectedCollections to the format expected by GetSchema
	var selectedTables []string
	if len(selectedCollections) == 0 || (len(selectedCollections) == 1 && selectedCollections[0] == "ALL") {
		selectedTables = []string{"ALL"}
	} else {
		selectedTables = selectedCollections
	}

	// Fetch fresh schema directly with the longer timeout context
	freshSchema, err := m.schemaManager.GetSchema(schemaCtx, chatID, db, conn.Config.Type, selectedTables)
	if err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Error fetching fresh schema: %v", err)
		return "", fmt.Errorf("failed to fetch fresh schema: %v", err)
	}

	// Store the fresh schema
	err = m.schemaManager.storeSchema(schemaCtx, chatID, freshSchema, db, conn.Config.Type)
	if err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Error storing fresh schema: %v", err)
		// Continue anyway, as we have the fresh schema
	}

	// Check for context cancellation
	if err := schemaCtx.Err(); err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Context cancelled after schema fetch: %v", err)
		return "", fmt.Errorf("operation cancelled: %v", err)
	}

	// Format schema with examples and selected collections
	formattedSchema, err := m.schemaManager.FormatSchemaWithExamplesAndCollections(schemaCtx, chatID, db, conn.Config.Type, selectedCollections)
	if err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Error formatting schema: %v", err)
		return "", fmt.Errorf("failed to format schema with examples: %v", err)
	}

	log.Printf("DBManager -> RefreshSchemaWithExamples -> Successfully refreshed schema for chatID: %s (schema length: %d)", chatID, len(formattedSchema))
	return formattedSchema, nil
}
