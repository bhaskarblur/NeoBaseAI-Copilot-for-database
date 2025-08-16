package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/utils"
	"strings"
)

// SpreadsheetDriver implements DatabaseDriver for CSV/Excel storage using PostgreSQL
type SpreadsheetDriver struct {
	postgresDriver DatabaseDriver
	crypto         *utils.AESGCMCrypto
}

// NewSpreadsheetDriver creates a new Spreadsheet driver
func NewSpreadsheetDriver() DatabaseDriver {
	// Initialize crypto with the encryption key
	crypto, err := utils.NewAESGCMCrypto(config.Env.SpreadsheetDataEncryptionKey)
	if err != nil {
		log.Printf("SpreadsheetDriver -> Failed to initialize crypto: %v", err)
		return nil
	}

	return &SpreadsheetDriver{
		postgresDriver: NewPostgresDriver(),
		crypto:         crypto,
	}
}

// Connect handles connection for CSV/Excel storage
func (d *SpreadsheetDriver) Connect(cfg ConnectionConfig) (*Connection, error) {
	// For CSV/Excel, we recognize special placeholder values
	if cfg.Host != "internal-spreadsheet" {
		return nil, fmt.Errorf("invalid host for spreadsheet connection")
	}

	// Create actual PostgreSQL config for the internal spreadsheet database
	spreadsheetPort := fmt.Sprintf("%d", config.Env.SpreadsheetPostgresPort)
	spreadsheetConfig := ConnectionConfig{
		Type:     "postgresql",
		Host:     config.Env.SpreadsheetPostgresHost,
		Port:     &spreadsheetPort,
		Username: &config.Env.SpreadsheetPostgresUsername,
		Password: &config.Env.SpreadsheetPostgresPassword,
		Database: config.Env.SpreadsheetPostgresDatabase,
		UseSSL:   config.Env.SpreadsheetPostgresSSLMode != "disable",
	}

	if config.Env.SpreadsheetPostgresSSLMode != "disable" {
		spreadsheetConfig.SSLMode = &config.Env.SpreadsheetPostgresSSLMode
	}

	// Connect using the PostgreSQL driver
	conn, err := d.postgresDriver.Connect(spreadsheetConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to spreadsheet database: %w", err)
	}

	// Store the original config to preserve the frontend values
	conn.Config = cfg

	// Create schema for this connection if it doesn't exist
	if conn.ChatID != "" {
		schemaName := fmt.Sprintf("conn_%s", conn.ChatID)
		if err := d.createConnectionSchema(conn, schemaName); err != nil {
			// Disconnect on error
			d.postgresDriver.Disconnect(conn)
			return nil, fmt.Errorf("failed to create connection schema: %w", err)
		}
		// Store schema name in config for later use
		conn.Config.SchemaName = schemaName
	}

	return conn, nil
}

// createConnectionSchema creates a schema for the connection
func (d *SpreadsheetDriver) createConnectionSchema(conn *Connection, schemaName string) error {
	// Get SQL DB from GORM
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %v", err)
	}

	// Create schema if not exists
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)
	if _, err := sqlDB.Exec(query); err != nil {
		return fmt.Errorf("failed to create schema: %v", err)
	}

	log.Printf("SpreadsheetDriver -> Created schema: %s", schemaName)
	return nil
}

// Disconnect handles disconnection
func (d *SpreadsheetDriver) Disconnect(conn *Connection) error {
	return d.postgresDriver.Disconnect(conn)
}

// GetSchemaInfo fetches schema information for CSV storage
func (d *SpreadsheetDriver) GetSchemaInfo(conn *Connection, selectedTables []string) (*SchemaInfo, error) {
	// Use a wrapper to intercept the PostgreSQL schema fetching
	// and filter to only show tables in the connection's schema
	schemaName := conn.Config.SchemaName
	if schemaName == "" {
		return &SchemaInfo{Tables: make(map[string]TableSchema)}, nil
	}

	// Create a temporary wrapper that filters by schema
	wrapper := &spreadsheetSchemaWrapper{
		conn:       conn,
		schemaName: schemaName,
	}

	// Get schema using PostgreSQL driver's GetSchema method
	ctx := context.Background()
	postgresDriver := d.postgresDriver.(*PostgresDriver)
	schemaInfo, err := postgresDriver.GetSchema(ctx, wrapper, selectedTables)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	// Remove internal columns from the schema info
	for tableName, table := range schemaInfo.Tables {
		filteredColumns := make(map[string]ColumnInfo)
		for colName, col := range table.Columns {
			// Skip internal columns
			if !strings.HasPrefix(col.Name, "_") {
				filteredColumns[colName] = col
			}
		}
		table.Columns = filteredColumns
		schemaInfo.Tables[tableName] = table
	}

	return schemaInfo, nil
}


// GetConnectionString returns a placeholder for CSV
func (d *SpreadsheetDriver) GetConnectionString(cfg ConnectionConfig) string {
	return "spreadsheet://internal"
}

// DeleteConnectionData deletes all data associated with a connection
func (d *SpreadsheetDriver) DeleteConnectionData(connectionID string) error {
	log.Printf("SpreadsheetDriver -> Deleting data for connection: %s", connectionID)

	// Create a temporary connection to drop the schema
	tempPort := fmt.Sprintf("%d", config.Env.SpreadsheetPostgresPort)
	tempConfig := ConnectionConfig{
		Type:     "postgresql",
		Host:     config.Env.SpreadsheetPostgresHost,
		Port:     &tempPort,
		Username: &config.Env.SpreadsheetPostgresUsername,
		Password: &config.Env.SpreadsheetPostgresPassword,
		Database: config.Env.SpreadsheetPostgresDatabase,
		UseSSL:   config.Env.SpreadsheetPostgresSSLMode != "disable",
	}

	conn, err := d.postgresDriver.Connect(tempConfig)
	if err != nil {
		return fmt.Errorf("failed to connect for deletion: %w", err)
	}
	defer d.postgresDriver.Disconnect(conn)

	// Drop the schema and all its tables
	schemaName := fmt.Sprintf("conn_%s", connectionID)
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %v", err)
	}

	query := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName)
	if _, err := sqlDB.Exec(query); err != nil {
		return fmt.Errorf("failed to drop schema: %v", err)
	}

	log.Printf("SpreadsheetDriver -> Dropped schema: %s", schemaName)
	return nil
}

// spreadsheetSchemaWrapper wraps a connection to filter queries by schema
type spreadsheetSchemaWrapper struct {
	conn       *Connection
	schemaName string
}

func (w *spreadsheetSchemaWrapper) GetDB() *sql.DB {
	sqlDB, _ := w.conn.DB.DB()
	return sqlDB
}


func (w *spreadsheetSchemaWrapper) ExecuteRaw(query string) error {
	return w.conn.DB.Exec(query).Error
}

func (w *spreadsheetSchemaWrapper) Close() error {
	// No-op as we don't own the connection
	return nil
}

func (w *spreadsheetSchemaWrapper) Raw(sql string, values ...interface{}) error {
	return w.conn.DB.Raw(sql, values...).Error
}

func (w *spreadsheetSchemaWrapper) Exec(sql string, values ...interface{}) error {
	return w.conn.DB.Exec(sql, values...).Error
}

func (w *spreadsheetSchemaWrapper) Query(sql string, dest interface{}, values ...interface{}) error {
	return w.conn.DB.Raw(sql, values...).Scan(dest).Error
}

func (w *spreadsheetSchemaWrapper) QueryRows(sql string, dest *[]map[string]interface{}, values ...interface{}) error {
	return w.conn.DB.Raw(sql, values...).Scan(dest).Error
}

func (w *spreadsheetSchemaWrapper) GetSchema(ctx context.Context) (*SchemaInfo, error) {
	// This should not be called directly
	return nil, fmt.Errorf("GetSchema should not be called on wrapper")
}

func (w *spreadsheetSchemaWrapper) GetTableChecksum(ctx context.Context, table string) (string, error) {
	// This should not be called directly
	return "", fmt.Errorf("GetTableChecksum should not be called on wrapper")
}

// Ping tests the connection
func (d *SpreadsheetDriver) Ping(conn *Connection) error {
	return d.postgresDriver.Ping(conn)
}

// IsAlive checks if the connection is still alive
func (d *SpreadsheetDriver) IsAlive(conn *Connection) bool {
	return d.postgresDriver.IsAlive(conn)
}

// ExecuteQuery executes a query with encryption/decryption support
func (d *SpreadsheetDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult {
	// For now, pass through to PostgreSQL driver
	// TODO: Add encryption/decryption logic for sensitive data
	return d.postgresDriver.ExecuteQuery(ctx, conn, query, queryType, findCount)
}

// BeginTx begins a transaction
func (d *SpreadsheetDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	return d.postgresDriver.BeginTx(ctx, conn)
}

// GetSchema gets the schema information
func (d *SpreadsheetDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// This method is called by the schema fetcher
	// We'll use the PostgreSQL driver's implementation
	return d.postgresDriver.GetSchema(ctx, db, selectedTables)
}

// GetTableChecksum gets a checksum for a table
func (d *SpreadsheetDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	return d.postgresDriver.GetTableChecksum(ctx, db, table)
}

// FetchExampleRecords fetches example records from a table
func (d *SpreadsheetDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// TODO: Add decryption logic for encrypted data
	return d.postgresDriver.FetchExampleRecords(ctx, db, table, limit)
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
