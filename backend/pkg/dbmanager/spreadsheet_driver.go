package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/utils"
	"strings"
)

// SpreadsheetDriver implements DatabaseDriver for CSV/Excel storage using PostgreSQL
type SpreadsheetDriver struct {
	postgresDriver DatabaseDriver
	crypto         *utils.AESGCMCrypto
}

// SpreadsheetTransaction wraps a PostgreSQL transaction with schema context
type SpreadsheetTransaction struct {
	pgTx       Transaction
	conn       *Connection
	schemaName string
	searchPathSet bool
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
	// Also accept "spreadsheet" for backward compatibility with existing connections
	if cfg.Host != "internal-spreadsheet" && cfg.Host != "spreadsheet" {
		return nil, fmt.Errorf("invalid host for spreadsheet connection: got '%s'", cfg.Host)
	}

	// Create actual PostgreSQL config for the internal spreadsheet database
	spreadsheetPort := config.Env.SpreadsheetPostgresPort
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

	// Note: Schema creation is now handled by the Manager after ChatID is set
	
	return conn, nil
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
		chatID:     conn.ChatID,
	}

	// Get schema using our own GetSchema method that filters by schema
	ctx := context.Background()
	schemaInfo, err := d.GetSchema(ctx, wrapper, selectedTables)
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
	tempPort := config.Env.SpreadsheetPostgresPort
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
	chatID     string
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
	// Ensure the schema context is set for the query
	schemaName := conn.Config.SchemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", conn.ChatID)
	}
	
	// Set the search_path to the specific schema
	setSearchPathQuery := fmt.Sprintf("SET search_path TO %s, public", schemaName)
	if err := conn.DB.Exec(setSearchPathQuery).Error; err != nil {
		log.Printf("SpreadsheetDriver -> ExecuteQuery -> Failed to set search path: %v", err)
	}
	
	// Execute the query with PostgreSQL driver
	result := d.postgresDriver.ExecuteQuery(ctx, conn, query, queryType, findCount)
	
	// Reset search path to default
	if err := conn.DB.Exec("SET search_path TO public").Error; err != nil {
		log.Printf("SpreadsheetDriver -> ExecuteQuery -> Failed to reset search path: %v", err)
	}
	
	return result
}

// BeginTx begins a transaction with schema context
func (d *SpreadsheetDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	// Get the underlying PostgreSQL transaction
	pgTx := d.postgresDriver.BeginTx(ctx, conn)
	if pgTx == nil {
		return nil
	}
	
	// Wrap it with our spreadsheet transaction that sets the search path
	return &SpreadsheetTransaction{
		pgTx:       pgTx,
		conn:       conn,
		schemaName: conn.Config.SchemaName,
	}
}

// GetSchema gets the schema information with proper schema filtering
func (d *SpreadsheetDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// Extract schema name from the wrapper
	var schemaName string
	var chatID string
	if wrapper, ok := db.(*spreadsheetSchemaWrapper); ok {
		schemaName = wrapper.schemaName
		chatID = wrapper.chatID
	} else {
		// Try to extract from connection if available
		log.Printf("SpreadsheetDriver -> GetSchema -> Warning: Schema name not available from wrapper")
		return &SchemaInfo{Tables: make(map[string]TableSchema)}, nil
	}
	
	if schemaName == "" {
		return &SchemaInfo{Tables: make(map[string]TableSchema)}, nil
	}

	// Get SQL DB
	sqlDB := db.GetDB()
	if sqlDB == nil {
		return nil, fmt.Errorf("failed to get SQL DB connection")
	}
	
	// Test connection is still valid
	if err := sqlDB.PingContext(ctx); err != nil {
		log.Printf("SpreadsheetDriver -> GetSchema -> Connection ping failed: %v", err)
		return nil, fmt.Errorf("database connection is not valid: %v", err)
	}

	// Query tables in the specific schema
	tableQuery := `
		SELECT tablename 
		FROM pg_catalog.pg_tables 
		WHERE schemaname = $1
		ORDER BY tablename;
	`
	
	rows, err := sqlDB.QueryContext(ctx, tableQuery, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables in schema %s: %w", schemaName, err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		// Skip system tables
		if tableName == "default" {
			continue
		}
		tableNames = append(tableNames, tableName)
	}

	log.Printf("SpreadsheetDriver -> GetSchema -> Found %d tables in schema %s", len(tableNames), schemaName)

	// Now get the schema details for these tables
	schema := &SchemaInfo{
		Tables: make(map[string]TableSchema),
	}

	for _, tableName := range tableNames {
		table := TableSchema{
			Name:        tableName,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
			Constraints: make(map[string]ConstraintInfo),
		}

		// Get columns for the table
		columnQuery := `
			SELECT 
				column_name,
				data_type,
				is_nullable,
				column_default,
				character_maximum_length,
				numeric_precision,
				numeric_scale
			FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2
			ORDER BY ordinal_position;
		`
		
		colRows, err := sqlDB.QueryContext(ctx, columnQuery, schemaName, tableName)
		if err != nil {
			log.Printf("SpreadsheetDriver -> GetSchema -> Error getting columns for table %s: %v", tableName, err)
			continue
		}
		
		for colRows.Next() {
			var colName, dataType string
			var isNullable string
			var columnDefault, charMaxLength, numPrecision, numScale sql.NullString
			
			if err := colRows.Scan(&colName, &dataType, &isNullable, &columnDefault, 
				&charMaxLength, &numPrecision, &numScale); err != nil {
				continue
			}
			
			// Skip internal columns
			if strings.HasPrefix(colName, "_") {
				continue
			}
			
			table.Columns[colName] = ColumnInfo{
				Name:         colName,
				Type:         dataType,
				IsNullable:   isNullable == "YES",
				DefaultValue: columnDefault.String,
			}
		}
		colRows.Close()
		
		// Get constraints (including primary keys)
		constraintQuery := `
			SELECT 
				tc.constraint_name,
				tc.constraint_type,
				kcu.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu 
				ON tc.constraint_name = kcu.constraint_name
				AND tc.table_schema = kcu.table_schema
				AND tc.table_name = kcu.table_name
			WHERE tc.table_schema = $1 AND tc.table_name = $2
			ORDER BY tc.constraint_name, kcu.ordinal_position;
		`
		
		constRows, err := sqlDB.QueryContext(ctx, constraintQuery, schemaName, tableName)
		if err == nil {
			constraints := make(map[string]*ConstraintInfo)
			for constRows.Next() {
				var constName, constType, colName string
				if err := constRows.Scan(&constName, &constType, &colName); err != nil {
					continue
				}
				
				if _, exists := constraints[constName]; !exists {
					constraints[constName] = &ConstraintInfo{
						Name:    constName,
						Type:    constType,
						Columns: []string{},
					}
				}
				constraints[constName].Columns = append(constraints[constName].Columns, colName)
			}
			constRows.Close()
			
			// Convert to the expected format
			for name, info := range constraints {
				table.Constraints[name] = *info
			}
		}
		
		// Get row count
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s`, schemaName, tableName)
		var count int64
		if err := sqlDB.QueryRowContext(ctx, countQuery).Scan(&count); err == nil {
			table.RowCount = count
		}
		
		// Get table size
		sizeQuery := fmt.Sprintf(`SELECT pg_total_relation_size('%s.%s')`, schemaName, tableName)
		var size int64
		if err := sqlDB.QueryRowContext(ctx, sizeQuery).Scan(&size); err == nil {
			table.SizeBytes = size
		}
		
		schema.Tables[tableName] = table
	}

	log.Printf("SpreadsheetDriver -> GetSchema -> Returning %d tables for schema %s (chatID: %s)", len(schema.Tables), schemaName, chatID)
	return schema, nil
}

// GetTableChecksum gets a checksum for a table
func (d *SpreadsheetDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	return d.postgresDriver.GetTableChecksum(ctx, db, table)
}

// FetchExampleRecords fetches example records from a table
func (d *SpreadsheetDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Get the schema name from the connection
	wrapper, ok := db.(*spreadsheetSchemaWrapper)
	if !ok {
		return nil, fmt.Errorf("invalid database wrapper for spreadsheet")
	}
	
	// Build schema-qualified table name
	schemaName := wrapper.schemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", wrapper.chatID)
	}
	qualifiedTable := fmt.Sprintf("%s.%s", schemaName, table)
	
	// Use the qualified table name to fetch records
	return d.postgresDriver.FetchExampleRecords(ctx, db, qualifiedTable, limit)
}

// setSearchPath sets the search path for this transaction if not already set
func (t *SpreadsheetTransaction) setSearchPath(ctx context.Context) error {
	if t.searchPathSet {
		return nil
	}
	
	schemaName := t.schemaName
	if schemaName == "" {
		schemaName = fmt.Sprintf("conn_%s", t.conn.ChatID)
	}
	
	// Set the search_path to the specific schema
	setSearchPathQuery := fmt.Sprintf("SET search_path TO %s, public", schemaName)
	result, err := t.pgTx.ExecuteQuery(ctx, setSearchPathQuery)
	if err != nil {
		return err
	}
	if result.Error != nil {
		return fmt.Errorf("failed to set search path: %s", result.Error.Message)
	}
	
	t.searchPathSet = true
	return nil
}

// ExecuteQuery executes queries with schema context
func (t *SpreadsheetTransaction) ExecuteQuery(ctx context.Context, query string) (*QueryExecutionResult, error) {
	// Set search path before executing queries
	if err := t.setSearchPath(ctx); err != nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Code:    "SCHEMA_SETUP_FAILED",
				Message: err.Error(),
				Details: "Failed to set schema context for spreadsheet query",
			},
		}, err
	}
	
	// Execute query using the underlying PostgreSQL transaction
	return t.pgTx.ExecuteQuery(ctx, query)
}

// Commit commits the transaction
func (t *SpreadsheetTransaction) Commit() error {
	return t.pgTx.Commit()
}

// Rollback rolls back the transaction
func (t *SpreadsheetTransaction) Rollback() error {
	return t.pgTx.Rollback()
}

