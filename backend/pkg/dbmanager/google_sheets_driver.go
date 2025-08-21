package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/pkg/redis"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// GoogleSheetsDriver implements DatabaseDriver for Google Sheets using PostgreSQL storage
type GoogleSheetsDriver struct {
	postgresDriver DatabaseDriver
	sheetsService  *sheets.Service
	redisRepo      redis.IRedisRepositories
}

// NewGoogleSheetsDriver creates a new Google Sheets driver
func NewGoogleSheetsDriver(redisRepo redis.IRedisRepositories) DatabaseDriver {
	return &GoogleSheetsDriver{
		postgresDriver: NewPostgresDriver(),
		redisRepo:      redisRepo,
	}
}

// Connect handles connection for Google Sheets
func (d *GoogleSheetsDriver) Connect(cfg ConnectionConfig) (*Connection, error) {
	// Validate Google Sheets specific fields
	if cfg.GoogleSheetID == nil || *cfg.GoogleSheetID == "" {
		return nil, fmt.Errorf("google sheet ID is required")
	}

	// Initialize Google Sheets API client
	if err := d.initializeSheetsService(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize Google Sheets service: %w", err)
	}

	// Create a spreadsheet config for internal storage
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

	// Connect directly using the PostgreSQL driver
	conn, err := d.postgresDriver.Connect(spreadsheetConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to internal storage: %w", err)
	}

	// Store the original config to preserve the Google Sheets settings
	conn.Config = cfg
	// Set the ChatID from config if available
	if cfg.ChatID != "" {
		conn.ChatID = cfg.ChatID
	}

	// Sync data from Google Sheets on initial connection
	if err := d.syncDataFromSheets(conn); err != nil {
		log.Printf("Warning: Failed to sync data from Google Sheets: %v", err)
		// Don't fail the connection, allow user to retry sync later
	}

	return conn, nil
}

// initializeSheetsService initializes the Google Sheets API service
func (d *GoogleSheetsDriver) initializeSheetsService(cfg ConnectionConfig) error {
	if cfg.GoogleAuthToken == nil || cfg.GoogleRefreshToken == nil {
		return fmt.Errorf("google authentication tokens are required")
	}

	// Create OAuth2 config
	oauthConfig := &oauth2.Config{
		ClientID:     config.Env.GoogleClientID,
		ClientSecret: config.Env.GoogleClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  config.Env.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/spreadsheets.readonly",
		},
	}

	// Create token
	token := &oauth2.Token{
		AccessToken:  *cfg.GoogleAuthToken,
		RefreshToken: *cfg.GoogleRefreshToken,
		TokenType:    "Bearer",
	}

	// Create HTTP client with OAuth2
	client := oauthConfig.Client(context.Background(), token)

	// Create Sheets service
	service, err := sheets.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create sheets service: %w", err)
	}

	d.sheetsService = service
	return nil
}

// syncDataFromSheets syncs data from Google Sheets to internal storage
func (d *GoogleSheetsDriver) syncDataFromSheets(conn *Connection) error {
	if d.sheetsService == nil {
		return fmt.Errorf("sheets service not initialized")
	}

	sheetID := conn.Config.GoogleSheetID
	if sheetID == nil || *sheetID == "" {
		return fmt.Errorf("google sheet ID not found")
	}

	// Get spreadsheet metadata
	spreadsheet, err := d.sheetsService.Spreadsheets.Get(*sheetID).Do()
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	// Schema name for this connection
	if conn.ChatID == "" {
		return fmt.Errorf("chat ID not set for connection")
	}
	schemaName := fmt.Sprintf("conn_%s", conn.ChatID)

	// Create schema if it doesn't exist
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %w", err)
	}

	// Create schema
	createSchemaQuery := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)
	if _, err := sqlDB.Exec(createSchemaQuery); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Process each sheet
	for _, sheet := range spreadsheet.Sheets {
		sheetName := sheet.Properties.Title
		tableName := sanitizeTableName(sheetName)

		log.Printf("GoogleSheetsDriver -> Syncing sheet '%s' as table '%s'", sheetName, tableName)

		// Get sheet data
		readRange := fmt.Sprintf("%s!A:ZZ", sheetName) // Read all columns
		resp, err := d.sheetsService.Spreadsheets.Values.Get(*sheetID, readRange).Do()
		if err != nil {
			log.Printf("Warning: Failed to read sheet %s: %v", sheetName, err)
			continue
		}

		if len(resp.Values) == 0 {
			log.Printf("Sheet %s is empty, skipping", sheetName)
			continue
		}

		// Use intelligent analyzer to process the sheet
		analyzer := NewSheetAnalyzer(resp.Values)
		region, err := analyzer.AnalyzeSheet()
		if err != nil {
			log.Printf("Warning: Failed to analyze sheet %s: %v", sheetName, err)
			// Fall back to old method
			headers := make([]string, 0)
			for _, cell := range resp.Values[0] {
				headers = append(headers, fmt.Sprintf("%v", cell))
			}
			data := resp.Values[1:]
			if err := d.storeSheetData(sqlDB, schemaName, tableName, headers, data); err != nil {
				log.Printf("Warning: Failed to store sheet %s: %v", sheetName, err)
				continue
			}
		} else {
			// Log analysis results
			log.Printf("GoogleSheetsDriver -> Sheet analysis for '%s':", sheetName)
			log.Printf("  - Data quality: %.1f%%", region.Quality)
			log.Printf("  - Headers detected: %v", region.Headers)
			log.Printf("  - Data rows: %d", len(region.DataRows))
			
			if len(region.Issues) > 0 {
				log.Printf("  - Issues detected:")
				for _, issue := range region.Issues {
					log.Printf("    • %s", issue)
				}
			}
			
			if len(region.Suggestions) > 0 {
				log.Printf("  - Suggestions:")
				for _, suggestion := range region.Suggestions {
					log.Printf("    • %s", suggestion)
				}
			}
			
			// Store the analyzed data
			if err := d.storeSheetData(sqlDB, schemaName, tableName, region.Headers, region.DataRows); err != nil {
				log.Printf("Warning: Failed to store sheet %s: %v", sheetName, err)
				continue
			}
			
			// Analyze columns for metadata
			columnAnalyses := analyzer.AnalyzeColumns(region)
			
			// Create and store import metadata
			metadata := &dtos.ImportMetadata{
				TableName:   tableName,
				RowCount:    len(region.DataRows),
				ColumnCount: len(region.Headers),
				Quality:     region.Quality,
				Issues:      region.Issues,
				Suggestions: region.Suggestions,
				Columns:     make([]dtos.ImportColumnMetadata, 0),
			}
			
			for _, colAnalysis := range columnAnalyses {
				metadata.Columns = append(metadata.Columns, dtos.ImportColumnMetadata{
					Name:         colAnalysis.Name,
					OriginalName: colAnalysis.OriginalName,
					DataType:     colAnalysis.DataType,
					NullCount:    colAnalysis.NullCount,
					UniqueCount:  colAnalysis.UniqueCount,
					IsEmpty:      colAnalysis.IsEmpty,
					IsPrimaryKey: colAnalysis.IsPrimaryKey,
				})
			}
			
			// Store metadata if we have the connection ID
			if conn.ChatID != "" && d.redisRepo != nil {
				metadataStore := NewImportMetadataStore(d.redisRepo)
				if err := metadataStore.StoreMetadata(conn.ChatID, metadata); err != nil {
					log.Printf("Warning: Failed to store import metadata: %v", err)
				} else {
					log.Printf("GoogleSheetsDriver -> Successfully stored import metadata for chat %s", conn.ChatID)
				}
			}
			
			log.Printf("Import Metadata - Table: %s, Quality: %.1f%%, Rows: %d, Columns: %d", 
				metadata.TableName, metadata.Quality, metadata.RowCount, metadata.ColumnCount)
			
			log.Printf("GoogleSheetsDriver -> Successfully synced sheet '%s' with %d rows", sheetName, len(region.DataRows))
		}
	}

	// Update the connection's schema name
	conn.Config.SchemaName = schemaName
	log.Printf("GoogleSheetsDriver -> syncDataFromSheets -> Set schema name: %s", schemaName)

	return nil
}

// storeSheetData stores sheet data in PostgreSQL
func (d *GoogleSheetsDriver) storeSheetData(db *sql.DB, schemaName, tableName string, headers []string, data [][]interface{}) error {
	// Drop existing table if it exists
	dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", schemaName, tableName)
	if _, err := db.Exec(dropQuery); err != nil {
		return fmt.Errorf("failed to drop existing table: %w", err)
	}

	// Headers are already cleaned by the analyzer, just validate
	if len(headers) == 0 {
		return fmt.Errorf("no columns found in sheet")
	}

	// Create table with columns based on headers
	columns := make([]string, 0)
	for _, header := range headers {
		colName := sanitizeColumnName(header)
		columns = append(columns, fmt.Sprintf("%s TEXT", colName))
	}

	// Add internal columns
	columns = append(columns, "_row_id SERIAL PRIMARY KEY")
	columns = append(columns, "_imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")

	createQuery := fmt.Sprintf("CREATE TABLE %s.%s (%s)", schemaName, tableName, strings.Join(columns, ", "))
	if _, err := db.Exec(createQuery); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Insert data
	if len(data) > 0 {
		// Prepare column names for insert
		colNames := make([]string, 0)
		for _, header := range headers {
			colNames = append(colNames, sanitizeColumnName(header))
		}

		// Build insert query
		for _, row := range data {
			values := make([]string, 0)
			for i := range headers {
				var value string
				if i < len(row) && row[i] != nil {
					// Convert value to string
					switch v := row[i].(type) {
					case string:
						value = v
					case float64:
						value = fmt.Sprintf("%v", v)
					case bool:
						value = fmt.Sprintf("%v", v)
					default:
						value = fmt.Sprintf("%v", v)
					}
					// Escape single quotes for SQL
					value = strings.ReplaceAll(value, "'", "''")
				}
				values = append(values, fmt.Sprintf("'%s'", value))
			}

			insertQuery := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)",
				schemaName, tableName,
				strings.Join(colNames, ", "),
				strings.Join(values, ", "))

			if _, err := db.Exec(insertQuery); err != nil {
				log.Printf("Warning: Failed to insert row: %v", err)
				// Continue with other rows
			}
		}
	}

	return nil
}

// RefreshData refreshes data from Google Sheets
func (d *GoogleSheetsDriver) RefreshData(conn *Connection) error {
	return d.syncDataFromSheets(conn)
}

// Disconnect handles disconnection
func (d *GoogleSheetsDriver) Disconnect(conn *Connection) error {
	return d.postgresDriver.Disconnect(conn)
}

// GetSchemaInfo fetches schema information
func (d *GoogleSheetsDriver) GetSchemaInfo(conn *Connection, selectedTables []string) (*SchemaInfo, error) {
	// Create a SpreadsheetDriver instance for schema info
	spreadsheetDriver := &SpreadsheetDriver{
		postgresDriver: d.postgresDriver,
	}
	return spreadsheetDriver.GetSchemaInfo(conn, selectedTables)
}

// GetConnectionString returns a placeholder for Google Sheets
func (d *GoogleSheetsDriver) GetConnectionString(cfg ConnectionConfig) string {
	if cfg.GoogleSheetID != nil {
		return fmt.Sprintf("google-sheets://%s", *cfg.GoogleSheetID)
	}
	return "google-sheets://unknown"
}

// DeleteConnectionData deletes all data associated with a connection
func (d *GoogleSheetsDriver) DeleteConnectionData(connectionID string) error {
	// Create a SpreadsheetDriver instance for deletion
	spreadsheetDriver := &SpreadsheetDriver{
		postgresDriver: d.postgresDriver,
	}
	return spreadsheetDriver.DeleteConnectionData(connectionID)
}

// DeleteConnectionDataWithConn deletes all data using a provided connection
func (d *GoogleSheetsDriver) DeleteConnectionDataWithConn(connectionID string, conn *Connection) error {
	// Create a SpreadsheetDriver instance for deletion
	spreadsheetDriver := &SpreadsheetDriver{
		postgresDriver: d.postgresDriver,
	}
	return spreadsheetDriver.DeleteConnectionDataWithConn(connectionID, conn)
}

// Ping tests the connection
func (d *GoogleSheetsDriver) Ping(conn *Connection) error {
	return d.postgresDriver.Ping(conn)
}

// IsAlive checks if the connection is still alive
func (d *GoogleSheetsDriver) IsAlive(conn *Connection) bool {
	return d.postgresDriver.IsAlive(conn)
}

// ExecuteQuery executes a query using the postgres driver with schema context
func (d *GoogleSheetsDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult {
	// Set the search path to the schema for this connection
	schemaName := conn.Config.SchemaName
	if schemaName == "" && conn.ChatID != "" {
		schemaName = fmt.Sprintf("conn_%s", conn.ChatID)
	}
	
	if schemaName != "" {
		// Set search path to the spreadsheet schema
		setSearchPathQuery := fmt.Sprintf("SET search_path TO %s, public", schemaName)
		if err := conn.DB.Exec(setSearchPathQuery).Error; err != nil {
			log.Printf("GoogleSheetsDriver -> ExecuteQuery -> Failed to set search path: %v", err)
		}
	}
	
	// Execute the query with PostgreSQL driver
	result := d.postgresDriver.ExecuteQuery(ctx, conn, query, queryType, findCount)
	
	// Reset search path to default
	if schemaName != "" {
		if err := conn.DB.Exec("SET search_path TO public").Error; err != nil {
			log.Printf("GoogleSheetsDriver -> ExecuteQuery -> Failed to reset search path: %v", err)
		}
	}
	
	return result
}

// BeginTx begins a transaction with schema context
func (d *GoogleSheetsDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	// Get the underlying PostgreSQL transaction
	pgTx := d.postgresDriver.BeginTx(ctx, conn)
	if pgTx == nil {
		return nil
	}
	
	// Get schema name
	schemaName := conn.Config.SchemaName
	if schemaName == "" && conn.ChatID != "" {
		schemaName = fmt.Sprintf("conn_%s", conn.ChatID)
	}
	
	// Wrap it with our spreadsheet transaction that sets the search path
	return &SpreadsheetTransaction{
		pgTx:       pgTx,
		conn:       conn,
		schemaName: schemaName,
		driver:     &SpreadsheetDriver{postgresDriver: d.postgresDriver},
	}
}

// GetSchema gets the schema information
func (d *GoogleSheetsDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	return d.postgresDriver.GetSchema(ctx, db, selectedTables)
}

// GetTableChecksum gets a checksum for a table
func (d *GoogleSheetsDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	return d.postgresDriver.GetTableChecksum(ctx, db, table)
}

// FetchExampleRecords fetches example records from a table
func (d *GoogleSheetsDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	return d.postgresDriver.FetchExampleRecords(ctx, db, table, limit)
}

// Helper function to sanitize table names
func sanitizeTableName(name string) string {
	// Convert to lowercase and replace special characters with underscores
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, " ", "_")
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")
	result = strings.ReplaceAll(result, "(", "")
	result = strings.ReplaceAll(result, ")", "")
	result = strings.ReplaceAll(result, "/", "_")
	result = strings.ReplaceAll(result, "\\", "_")
	
	// Remove consecutive underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	
	// Remove leading/trailing underscores
	result = strings.Trim(result, "_")
	
	// Ensure it starts with a letter
	if len(result) > 0 && (result[0] < 'a' || result[0] > 'z') {
		result = "t_" + result
	}
	
	return result
}

// Helper function to sanitize column names
func sanitizeColumnName(name string) string {
	// Similar to table name sanitization
	result := sanitizeTableName(name)
	
	// Ensure it starts with a letter
	if len(result) > 0 && (result[0] < 'a' || result[0] > 'z') {
		result = "c_" + result
	}
	
	return result
}