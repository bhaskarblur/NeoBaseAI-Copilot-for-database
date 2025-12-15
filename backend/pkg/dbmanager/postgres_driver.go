package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/utils"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresDriver struct{}

func NewPostgresDriver() DatabaseDriver {
	return &PostgresDriver{}
}

func (d *PostgresDriver) Connect(config ConnectionConfig) (*Connection, error) {
	var dsn string
	var tempFiles []string
	var sshTunnel *SSHTunnel

	// Establish SSH tunnel if configured
	if config.SSHEnabled && config.SSHHost != nil && config.SSHPort != nil && config.SSHUsername != nil {
		log.Printf("PostgresDriver -> Connect -> Establishing SSH tunnel for PostgreSQL connection")

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
		defer func() {
			if err != nil {
				sshTunnel.Close()
			}
		}()
	}

	// Determine the host to connect to
	connectHost := config.Host
	if sshTunnel != nil {
		// When using SSH tunnel, connect through localhost
		connectHost = "localhost"
	}

	// Base connection parameters
	baseParams := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s",
		connectHost,
		*config.Port, // Dereference the port pointer
		*config.Username,
		config.Database,
	)

	// Add password if provided
	if config.Password != nil {
		baseParams += fmt.Sprintf(" password=%s", *config.Password)
	}

	// Configure SSL/TLS
	if config.UseSSL {
		sslMode := "require"
		if config.SSLMode != nil {
			sslMode = *config.SSLMode
		}

		if sslMode == "disable" {
			// Do nothing
		} else {
			baseParams += fmt.Sprintf(" sslmode=%s", sslMode)
		}

		// Fetch certificates from URLs only if they are provided
		var certPath, keyPath, rootCertPath string
		var certTempFiles []string
		var err error

		if config.SSLCertURL != nil && config.SSLKeyURL != nil && config.SSLRootCertURL != nil {
			certPath, keyPath, rootCertPath, certTempFiles, err = utils.PrepareCertificatesFromURLs(*config.SSLCertURL, *config.SSLKeyURL, *config.SSLRootCertURL)
			if err != nil {
				return nil, err
			}
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
		return nil, fmt.Errorf("failed to create connection: %v", err)
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
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
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
		SSHTunnel:   sshTunnel,
	}

	return conn, nil
}

func (d *PostgresDriver) Disconnect(conn *Connection) error {
	// Get the underlying SQL DB
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %v", err)
	}

	// Close the connection
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %v", err)
	}

	// Close SSH tunnel if present
	if conn.SSHTunnel != nil {
		if sshTunnel, ok := conn.SSHTunnel.(*SSHTunnel); ok {
			if err := sshTunnel.Close(); err != nil {
				log.Printf("PostgresDriver -> Disconnect -> Warning: Failed to close SSH tunnel: %v", err)
			}
		}
	}

	// Clean up temporary certificate files
	for _, file := range conn.TempFiles {
		os.Remove(file)
	}

	return nil
}

func (d *PostgresDriver) Ping(conn *Connection) error {
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (d *PostgresDriver) IsAlive(conn *Connection) bool {
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return false
	}
	return sqlDB.Ping() == nil
}

// Modify ExecuteQuery to check for schema changes
func (d *PostgresDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult {
	startTime := time.Now()
	log.Printf("PostgreSQL/YugabyteDB Driver -> ExecuteQuery -> Query: %v", query)
	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> ExecuteQuery -> Failed to get SQL connection: %v", err)
		return &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Error: &dtos.QueryError{
				Code:    "FAILED_TO_GET_SQL_CONNECTION",
				Message: "Failed to get SQL connection",
				Details: err.Error(),
			},
		}
	}

	// Split multiple statements
	statements := splitStatements(query)
	var lastResult *sql.Rows
	var lastError error

	log.Printf("PostgreSQL/YugabyteDB Driver -> ExecuteQuery -> Statements: %v", statements)
	// Execute each statement
	for _, stmt := range statements {
		if stmt = strings.TrimSpace(stmt); stmt == "" {
			continue
		}

		lastResult, lastError = sqlDB.QueryContext(ctx, stmt)
		if lastError != nil {
			log.Printf("PostgreSQL/YugabyteDB Driver -> ExecuteQuery -> Query execution failed: %v", lastError)
			return &QueryExecutionResult{
				ExecutionTime: int(time.Since(startTime).Milliseconds()),
				Error: &dtos.QueryError{
					Code:    "QUERY_EXECUTION_FAILED",
					Message: "Query execution failed",
					Details: lastError.Error(),
				},
			}
		}
		if lastResult != nil {
			defer lastResult.Close()
		}
	}

	// Process results from the last statement if it returned rows
	var result *QueryExecutionResult
	if lastResult != nil {
		results, err := processRows(lastResult, startTime)
		if err != nil {
			return &QueryExecutionResult{
				ExecutionTime: int(time.Since(startTime).Milliseconds()),
				Error: &dtos.QueryError{
					Code:    "RESULT_PROCESSING_FAILED",
					Message: err.Error(),
					Details: "Failed to process query results",
				},
			}
		}
		result = &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Result: map[string]interface{}{
				"results": results,
			},
		}
	} else {
		result = &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Result: map[string]interface{}{
				"message": "Query executed successfully",
			},
		}
	}

	return result
}

func (d *PostgresDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	log.Printf("PostgreSQL/YugabyteDB Driver -> BeginTx -> Starting transaction")

	if conn == nil || conn.DB == nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> BeginTx: Connection or DB is nil")
		return nil
	}

	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> BeginTx -> Failed to get SQL connection: %v", err)
		return nil
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> BeginTx -> Failed to begin transaction: %v", err)
		return nil
	}

	// Pass connection to transaction
	return &PostgresTransaction{
		tx:   tx,
		conn: conn,
	}
}

// Improve the GetSchema method to properly detect all tables
func (d *PostgresDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	sqlDB := db.GetDB()
	if sqlDB == nil {
		return nil, fmt.Errorf("failed to get SQL DB connection")
	}

	// Test connection is still valid before executing queries
	if err := sqlDB.PingContext(ctx); err != nil {
		log.Printf("PostgresDriver -> GetSchema -> Connection ping failed: %v", err)
		return nil, fmt.Errorf("database connection is not valid: %v", err)
	}

	// Get all tables in the database, filtered by selectedTables if provided
	var tableQuery string
	var args []interface{}

	if len(selectedTables) > 0 && selectedTables[0] != "ALL" {
		// Build a query with a WHERE IN clause for selected tables
		placeholders := make([]string, len(selectedTables))
		args = make([]interface{}, len(selectedTables))

		for i, table := range selectedTables {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = table
		}

		tableQuery = fmt.Sprintf(`
			SELECT tablename 
			FROM pg_catalog.pg_tables 
			WHERE schemaname = 'public'
			AND tablename IN (%s);
		`, strings.Join(placeholders, ","))
	} else {
		// Get all tables
		tableQuery = `
			SELECT tablename 
			FROM pg_catalog.pg_tables 
			WHERE schemaname = 'public';
		`
	}

	var tableRows *sql.Rows
	var err error

	if len(args) > 0 {
		tableRows, err = sqlDB.QueryContext(ctx, tableQuery, args...)
	} else {
		tableRows, err = sqlDB.QueryContext(ctx, tableQuery)
	}

	if err != nil {
		// Check if this is a prepared statement error
		if strings.Contains(err.Error(), "unnamed prepared statement does not exist") {
			log.Printf("PostgresDriver -> GetSchema -> Prepared statement error detected, retrying with fresh connection")
			// Try to recover by pinging the connection first
			if pingErr := sqlDB.PingContext(ctx); pingErr != nil {
				return nil, fmt.Errorf("connection lost: %v (original error: %v)", pingErr, err)
			}
			// Retry the query once more
			if len(args) > 0 {
				tableRows, err = sqlDB.QueryContext(ctx, tableQuery, args...)
			} else {
				tableRows, err = sqlDB.QueryContext(ctx, tableQuery)
			}
			if err != nil {
				return nil, fmt.Errorf("failed to query tables after retry: %v", err)
			}
		} else {
			return nil, fmt.Errorf("failed to query tables: %v", err)
		}
	}
	defer tableRows.Close()

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a list of all tables
	allTables := make([]string, 0)
	for tableRows.Next() {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
			return nil, err
		}

		var tableName string
		if err := tableRows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %v", err)
		}
		allTables = append(allTables, tableName)
	}

	log.Printf("PostgresDriver -> GetSchema -> Found %d tables in database: %v", len(allTables), allTables)

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Continue with the rest of the schema fetching...
	tables, err := d.getTables(ctx, sqlDB, allTables)
	if err != nil {
		return nil, err
	}

	// Verify that all tables were properly fetched
	for _, tableName := range allTables {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
			return nil, err
		}

		if _, exists := tables[tableName]; !exists {
			log.Printf("PostgresDriver -> GetSchema -> Warning: Table %s exists but wasn't properly fetched", tableName)
		}
	}

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Continue with the rest of the schema fetching...
	indexes, err := d.getIndexes(ctx, sqlDB, allTables)
	if err != nil {
		return nil, err
	}

	views, err := d.getViews(ctx, sqlDB)
	if err != nil {
		return nil, err
	}

	// Get foreign keys
	foreignKeys, err := d.getForeignKeys(ctx, sqlDB, allTables)
	if err != nil {
		return nil, err
	}

	// Add foreign keys to tables
	for tableName, tableFKs := range foreignKeys {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
			return nil, err
		}

		if table, exists := tables[tableName]; exists {
			table.ForeignKeys = tableFKs
			tables[tableName] = table
		}
	}

	// Convert to generic SchemaInfo
	return d.convertToSchemaInfo(tables, indexes, views), nil
}

// Update the convertToSchemaInfo function to pass indexes
func (d *PostgresDriver) convertToSchemaInfo(tables map[string]PostgresTable, indexes map[string][]PostgresIndex, views map[string]PostgresView) *SchemaInfo {
	schema := &SchemaInfo{
		Tables:    convertTablesToSchemaFormat(tables, indexes),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: time.Now(),
	}

	// Convert views
	for viewName, view := range views {
		schema.Views[viewName] = ViewSchema{
			Name:       viewName,
			Definition: view.Definition,
		}
	}

	return schema
}

// Improve the getTables method to properly fetch column details
func (d *PostgresDriver) getTables(ctx context.Context, db *sql.DB, tables []string) (map[string]PostgresTable, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a map to store all tables
	tablesMap := make(map[string]PostgresTable)
	allTableNames := make([]string, 0)

	// Initialize all tables first
	for _, tableName := range tables {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
			return nil, err
		}

		tablesMap[tableName] = PostgresTable{
			Name:        tableName,
			Columns:     make(map[string]PostgresColumn),
			Indexes:     make(map[string]PostgresIndex),
			ForeignKeys: make(map[string]PostgresForeignKey),
		}
		allTableNames = append(allTableNames, tableName)
	}

	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
		return nil, err
	}

	// For each table, get columns with SUPER DETAILED logging
	for tableName, table := range tablesMap {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
			return nil, err
		}

		log.Printf("PostgresDriver -> getTables -> Fetching columns for table: %s", tableName)

		// Get columns
		columnQuery := `
			SELECT 
				column_name, 
				data_type, 
				is_nullable,
				column_default,
				col_description((table_schema || '.' || table_name)::regclass::oid, ordinal_position) as column_comment
			FROM 
				information_schema.columns
			WHERE 
				table_schema = 'public' AND 
				table_name = $1
			ORDER BY 
				ordinal_position;
		`

		columnRows, err := db.QueryContext(ctx, columnQuery, tableName)
		if err != nil {
			log.Printf("PostgresDriver -> getTables -> Error fetching columns for table %s: %v", tableName, err)
			continue
		}

		columnCount := 0
		for columnRows.Next() {
			// Check for context cancellation
			if err := ctx.Err(); err != nil {
				log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
				return nil, err
			}

			var (
				columnName, dataType, isNullable string
				columnDefault, columnComment     sql.NullString
			)

			if err := columnRows.Scan(&columnName, &dataType, &isNullable, &columnDefault, &columnComment); err != nil {
				log.Printf("PostgresDriver -> getTables -> Error scanning column for table %s: %v", tableName, err)
				continue
			}

			// Convert is_nullable to bool
			isNullableBool := isNullable == "YES"

			// Get default value
			defaultValue := ""
			if columnDefault.Valid {
				defaultValue = columnDefault.String
			}

			// Get comment
			comment := ""
			if columnComment.Valid {
				comment = columnComment.String
			}

			log.Printf("PostgresDriver -> getTables -> Found column in table %s: name=%s, type=%s, nullable=%v, default=%s, comment=%s",
				tableName, columnName, dataType, isNullableBool, defaultValue, comment)

			table.Columns[columnName] = PostgresColumn{
				Name:         columnName,
				Type:         dataType,
				IsNullable:   isNullableBool,
				DefaultValue: defaultValue,
				Comment:      comment,
			}

			columnCount++
		}

		columnRows.Close()
		log.Printf("PostgresDriver -> getTables -> Fetched %d columns for table %s", columnCount, tableName)

		// Get indexes with SUPER DETAILED logging
		log.Printf("PostgresDriver -> getTables -> Fetching indexes for table: %s", tableName)
		indexQuery := `
			SELECT
				i.relname as index_name,
				array_to_string(array_agg(a.attname), ',') as column_names,
				ix.indisunique as is_unique,
				ix.indisprimary as is_primary
			FROM
				pg_class t,
				pg_class i,
				pg_index ix,
				pg_attribute a
			WHERE
				t.oid = ix.indrelid
				and i.oid = ix.indexrelid
				and a.attrelid = t.oid
				and a.attnum = ANY(ix.indkey)
				and t.relkind = 'r'
				and t.relname = $1
			GROUP BY
				i.relname,
				ix.indisunique,
				ix.indisprimary
			ORDER BY
				i.relname;
		`

		indexRows, err := db.QueryContext(ctx, indexQuery, tableName)
		if err != nil {
			log.Printf("PostgresDriver -> getTables -> Error fetching indexes for table %s: %v", tableName, err)
			continue
		}

		indexCount := 0
		for indexRows.Next() {
			// Check for context cancellation
			if err := ctx.Err(); err != nil {
				log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
				return nil, err
			}

			var (
				indexName, columnNames string
				isUnique, isPrimary    bool
			)

			if err := indexRows.Scan(&indexName, &columnNames, &isUnique, &isPrimary); err != nil {
				log.Printf("PostgresDriver -> getTables -> Error scanning index for table %s: %v", tableName, err)
				continue
			}

			log.Printf("PostgresDriver -> getTables -> Found index in table %s: name=%s, columns=%s, unique=%v, primary=%v",
				tableName, indexName, columnNames, isUnique, isPrimary)

			table.Indexes[indexName] = PostgresIndex{
				Name:     indexName,
				Columns:  strings.Split(columnNames, ","),
				IsUnique: isUnique,
			}

			indexCount++
		}

		indexRows.Close()
		log.Printf("PostgresDriver -> getTables -> Fetched %d indexes for table %s", indexCount, tableName)

		// Get foreign keys with SUPER DETAILED logging
		log.Printf("PostgresDriver -> getTables -> Fetching foreign keys for table: %s", tableName)
		fkQuery := `
			SELECT
				tc.constraint_name,
				kcu.column_name,
				ccu.table_name AS foreign_table_name,
				ccu.column_name AS foreign_column_name
			FROM
				information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
				  ON tc.constraint_name = kcu.constraint_name
				JOIN information_schema.constraint_column_usage AS ccu
				  ON ccu.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = $1;
		`

		fkRows, err := db.QueryContext(ctx, fkQuery, tableName)
		if err != nil {
			log.Printf("PostgresDriver -> getTables -> Error fetching foreign keys for table %s: %v", tableName, err)
			continue
		}

		fkCount := 0
		for fkRows.Next() {
			// Check for context cancellation
			if err := ctx.Err(); err != nil {
				log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
				return nil, err
			}

			var (
				constraintName, columnName, foreignTableName, foreignColumnName string
			)

			if err := fkRows.Scan(&constraintName, &columnName, &foreignTableName, &foreignColumnName); err != nil {
				log.Printf("PostgresDriver -> getTables -> Error scanning foreign key for table %s: %v", tableName, err)
				continue
			}

			log.Printf("PostgresDriver -> getTables -> Found foreign key in table %s: name=%s, column=%s, references=%s.%s",
				tableName, constraintName, columnName, foreignTableName, foreignColumnName)

			table.ForeignKeys[constraintName] = PostgresForeignKey{
				Name:      constraintName,
				Column:    columnName,
				RefTable:  foreignTableName,
				RefColumn: foreignColumnName,
			}

			fkCount++
		}

		fkRows.Close()
		log.Printf("PostgresDriver -> getTables -> Fetched %d foreign keys for table %s", fkCount, tableName)

		// Update the table in the map
		tablesMap[tableName] = table
	}

	// Verify all tables were processed
	for _, tableName := range allTableNames {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
			return nil, err
		}

		if _, exists := tablesMap[tableName]; !exists {
			log.Printf("PostgresDriver -> getTables -> Warning: Table %s exists but wasn't properly fetched", tableName)
		} else if len(tablesMap[tableName].Columns) == 0 {
			log.Printf("PostgresDriver -> getTables -> Warning: Table %s has no columns", tableName)
		}
	}

	log.Printf("PostgresDriver -> getTables -> Successfully fetched %d tables: %v", len(tablesMap), allTableNames)

	return tablesMap, nil
}

func (d *PostgresDriver) getIndexes(ctx context.Context, db *sql.DB, tables []string) (map[string][]PostgresIndex, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getIndexes -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a map to store indexes by table
	indexes := make(map[string][]PostgresIndex)

	// If no tables provided, return empty map
	if len(tables) == 0 {
		return indexes, nil
	}

	// Build query with table filter
	var query string
	var args []interface{}

	if len(tables) == 1 {
		// Simple case with one table
		query = `
			SELECT
				t.relname as table_name,
				i.relname as index_name,
				array_to_string(array_agg(a.attname), ',') as column_names,
				ix.indisunique as is_unique
			FROM
				pg_class t,
				pg_class i,
				pg_index ix,
				pg_attribute a
			WHERE
				t.oid = ix.indrelid
				and i.oid = ix.indexrelid
				and a.attrelid = t.oid
				and a.attnum = ANY(ix.indkey)
				and t.relkind = 'r'
				and t.relname = $1
			GROUP BY
				t.relname,
				i.relname,
				ix.indisunique
			ORDER BY
				t.relname,
				i.relname;
		`
		args = []interface{}{tables[0]}
	} else {
		// Multiple tables case
		placeholders := make([]string, len(tables))
		args = make([]interface{}, len(tables))

		for i, table := range tables {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = table
		}

		query = fmt.Sprintf(`
			SELECT
				t.relname as table_name,
				i.relname as index_name,
				array_to_string(array_agg(a.attname), ',') as column_names,
				ix.indisunique as is_unique
			FROM
				pg_class t,
				pg_class i,
				pg_index ix,
				pg_attribute a
			WHERE
				t.oid = ix.indrelid
				and i.oid = ix.indexrelid
				and a.attrelid = t.oid
				and a.attnum = ANY(ix.indkey)
				and t.relkind = 'r'
				and t.relname IN (%s)
			GROUP BY
				t.relname,
				i.relname,
				ix.indisunique
			ORDER BY
				t.relname,
				i.relname;
		`, strings.Join(placeholders, ","))
	}

	// Execute query
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	// Process results
	for rows.Next() {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getIndexes -> Context cancelled: %v", err)
			return nil, err
		}

		var (
			tableName, indexName, columnNames string
			isUnique                          bool
		)

		if err := rows.Scan(&tableName, &indexName, &columnNames, &isUnique); err != nil {
			return nil, fmt.Errorf("failed to scan index: %v", err)
		}

		// Create index
		index := PostgresIndex{
			Name:      indexName,
			Columns:   strings.Split(columnNames, ","),
			IsUnique:  isUnique,
			TableName: tableName,
		}

		// Add to map
		if _, exists := indexes[tableName]; !exists {
			indexes[tableName] = make([]PostgresIndex, 0)
		}
		indexes[tableName] = append(indexes[tableName], index)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating index rows: %v", err)
	}

	return indexes, nil
}

func (d *PostgresDriver) getViews(ctx context.Context, db *sql.DB) (map[string]PostgresView, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getViews -> Context cancelled: %v", err)
		return nil, err
	}

	query := `
		SELECT 
			viewname,
			definition
		FROM pg_views
		WHERE schemaname = 'public';
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	views := make(map[string]PostgresView)
	for rows.Next() {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getViews -> Context cancelled: %v", err)
			return nil, err
		}

		var view PostgresView
		if err := rows.Scan(&view.Name, &view.Definition); err != nil {
			return nil, err
		}
		views[view.Name] = view
	}
	return views, nil
}

func (d *PostgresDriver) getForeignKeys(ctx context.Context, db *sql.DB, tables []string) (map[string]map[string]PostgresForeignKey, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getForeignKeys -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a map to store foreign keys by table
	foreignKeys := make(map[string]map[string]PostgresForeignKey)

	// If no tables provided, return empty map
	if len(tables) == 0 {
		return foreignKeys, nil
	}

	// Build query with table filter
	var query string
	var args []interface{}

	if len(tables) == 1 {
		// Simple case with one table
		query = `
			SELECT
				tc.table_name,
				tc.constraint_name,
				kcu.column_name,
				ccu.table_name AS foreign_table_name,
				ccu.column_name AS foreign_column_name,
				rc.delete_rule,
				rc.update_rule
			FROM
				information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
				  ON tc.constraint_name = kcu.constraint_name
				JOIN information_schema.constraint_column_usage AS ccu
				  ON ccu.constraint_name = tc.constraint_name
				JOIN information_schema.referential_constraints AS rc
				  ON rc.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = $1;
		`
		args = []interface{}{tables[0]}
	} else {
		// Multiple tables case
		placeholders := make([]string, len(tables))
		args = make([]interface{}, len(tables))

		for i, table := range tables {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = table
		}

		query = fmt.Sprintf(`
			SELECT
				tc.table_name,
				tc.constraint_name,
				kcu.column_name,
				ccu.table_name AS foreign_table_name,
				ccu.column_name AS foreign_column_name,
				rc.delete_rule,
				rc.update_rule
			FROM
				information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
				  ON tc.constraint_name = kcu.constraint_name
				JOIN information_schema.constraint_column_usage AS ccu
				  ON ccu.constraint_name = tc.constraint_name
				JOIN information_schema.referential_constraints AS rc
				  ON rc.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name IN (%s);
		`, strings.Join(placeholders, ","))
	}

	// Execute query
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %v", err)
	}
	defer rows.Close()

	// Process results
	for rows.Next() {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getForeignKeys -> Context cancelled: %v", err)
			return nil, err
		}

		var (
			tableName, constraintName, columnName, foreignTableName, foreignColumnName string
			deleteRule, updateRule                                                     string
		)

		if err := rows.Scan(&tableName, &constraintName, &columnName, &foreignTableName, &foreignColumnName, &deleteRule, &updateRule); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %v", err)
		}

		// Create foreign key
		fk := PostgresForeignKey{
			Name:      constraintName,
			Column:    columnName,
			RefTable:  foreignTableName,
			RefColumn: foreignColumnName,
			OnDelete:  deleteRule,
			OnUpdate:  updateRule,
		}

		// Add to map
		if _, exists := foreignKeys[tableName]; !exists {
			foreignKeys[tableName] = make(map[string]PostgresForeignKey)
		}
		foreignKeys[tableName][constraintName] = fk
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating foreign key rows: %v", err)
	}

	return foreignKeys, nil
}

// Fix the GetTableChecksum method to be more stable and consistent
func (d *PostgresDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	sqlDB := db.GetDB()
	if sqlDB == nil {
		return "", fmt.Errorf("failed to get SQL DB connection")
	}

	// Get table definition checksum - use a more stable approach that ignores non-structural changes
	query := `
		SELECT md5(string_agg(column_definition, ',' ORDER BY ordinal_position))
		FROM (
			SELECT 
				ordinal_position,
				concat(
					column_name, ':', 
					data_type, ':', 
					is_nullable, ':', 
					coalesce(column_default, '')
				) as column_definition
			FROM information_schema.columns 
			WHERE table_schema = 'public' AND table_name = $1
		) t;
	`

	var checksum string
	if err := sqlDB.QueryRowContext(ctx, query, table).Scan(&checksum); err != nil {
		return "", fmt.Errorf("failed to get table checksum: %v", err)
	}

	// Get indexes checksum - use a more stable approach that ignores index names
	indexQuery := `
		SELECT md5(string_agg(index_definition, ',' ORDER BY index_columns))
		FROM (
			SELECT 
				array_to_string(array_agg(a.attname ORDER BY a.attnum), ',') as index_columns,
				concat(
					array_to_string(array_agg(a.attname ORDER BY a.attnum), ','), ':',
					ix.indisunique
				) as index_definition
			FROM pg_class t
			JOIN pg_index ix ON t.oid = ix.indrelid
			JOIN pg_class i ON i.oid = ix.indexrelid
			JOIN pg_attribute a ON a.attrelid = t.oid
			WHERE a.attnum = ANY(ix.indkey)
			AND t.relname = $1
			GROUP BY ix.indexrelid, ix.indisunique
		) t;
	`

	var indexChecksum string
	if err := sqlDB.QueryRowContext(ctx, indexQuery, table).Scan(&indexChecksum); err != nil {
		return "", fmt.Errorf("failed to get index checksum: %v", err)
	}

	// Get foreign keys checksum - use a more stable approach that ignores constraint names
	fkQuery := `
		SELECT md5(string_agg(fk_definition, ',' ORDER BY source_column, target_table, target_column))
		FROM (
			SELECT 
				kcu.column_name as source_column,
				ccu.table_name as target_table,
				ccu.column_name as target_column,
				concat(
					kcu.column_name, ':',
					ccu.table_name, ':',
					ccu.column_name
				) as fk_definition
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
			WHERE tc.table_name = $1 AND tc.constraint_type = 'FOREIGN KEY'
		) t;
	`

	var fkChecksum string
	if err := sqlDB.QueryRowContext(ctx, fkQuery, table).Scan(&fkChecksum); err != nil {
		return "", fmt.Errorf("failed to get foreign key checksum: %v", err)
	}

	// Combine all checksums
	finalChecksum := fmt.Sprintf("%s:%s:%s", checksum, indexChecksum, fkChecksum)
	return utils.MD5Hash(finalChecksum), nil
}

// Add FetchExampleRecords method to PostgresDriver
func (d *PostgresDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Ensure limit is reasonable
	if limit <= 0 {
		limit = 3 // Default to 3 records
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
	}

	// Build a simple query to fetch example records
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d", table, limit)

	var records []map[string]interface{}
	err := db.QueryRows(query, &records)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
	}
	// If no records found, return empty slice
	if len(records) == 0 {
		return []map[string]interface{}{}, nil
	}

	return records, nil
}

// Helper function to get value from pointer or empty string
func getValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
