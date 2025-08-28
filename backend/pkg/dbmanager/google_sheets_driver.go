package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/redis"
	"strconv"
	"strings"
	"time"

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
	// Check if we need to sync data from Google Sheets
	schemaName := fmt.Sprintf("conn_%s", conn.ChatID)
	shouldSync, err := d.shouldSyncData(conn, schemaName)
	if err != nil {
		log.Printf("Warning: Failed to check if sync is needed: %v", err)
		// Still attempt sync if check fails
		shouldSync = true
	}

	if shouldSync {
		log.Printf("GoogleSheetsDriver -> Schema '%s' needs data sync, syncing from Google Sheets", schemaName)
		if err := d.syncDataFromSheets(conn); err != nil {
			log.Printf("Warning: Failed to sync data from Google Sheets: %v", err)
			// Don't fail the connection, allow user to retry sync later
		}
	} else {
		log.Printf("GoogleSheetsDriver -> Schema '%s' already has data, skipping sync from Google Sheets", schemaName)
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

		// Use robust analyzer to process the sheet
		robustAnalyzer := NewRobustSheetAnalyzer(resp.Values)
		regions, err := robustAnalyzer.AnalyzeRobust()
		if err != nil {
			log.Printf("Warning: Failed to analyze sheet %s: %v", sheetName, err)
			// Fall back to basic unstructured handling
			region := &DataRegion{
				Headers:  []string{"row_num", "col_num", "value"},
				DataRows: make([][]interface{}, 0),
			}
			for rowIdx, row := range resp.Values {
				for colIdx, cell := range row {
					if cell != nil && fmt.Sprintf("%v", cell) != "" {
						region.DataRows = append(region.DataRows, []interface{}{
							rowIdx + 1,
							columnIndexToName(colIdx),
							cell,
						})
					}
				}
			}
			if len(region.DataRows) == 0 {
				region.DataRows = append(region.DataRows, []interface{}{1, "A", "No data found"})
			}
			insertResult, err := d.storeSheetData(sqlDB, schemaName, tableName, region.Headers, region.DataRows)
			if err != nil {
				log.Printf("Warning: Failed to store sheet %s: %v", sheetName, err)
				if insertResult != nil && len(insertResult.Errors) > 0 {
					log.Printf("Sheet %s had %d errors during insertion", sheetName, len(insertResult.Errors))
				}
				continue
			} else if insertResult != nil && insertResult.HasErrors() {
				log.Printf("Sheet %s processed with %d successful and %d failed rows", sheetName, insertResult.SuccessfulRows, insertResult.FailedRows)
			}
		} else if len(regions) > 0 {
			// Process all detected regions
			for regionIdx, region := range regions {
				// Log analysis results
				// Use different table names for multiple regions
				currentTableName := tableName
				if len(regions) > 1 {
					currentTableName = fmt.Sprintf("%s_%d", tableName, regionIdx+1)
				}

				log.Printf("GoogleSheetsDriver -> Sheet analysis for '%s' (region %d/%d):", sheetName, regionIdx+1, len(regions))
				log.Printf("  - Table name: %s", currentTableName)
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
				insertResult, err := d.storeSheetData(sqlDB, schemaName, currentTableName, region.Headers, region.DataRows)
				if err != nil {
					log.Printf("Warning: Failed to store sheet %s region %d: %v", sheetName, regionIdx+1, err)
					if insertResult != nil && len(insertResult.Errors) > 0 {
						log.Printf("Sheet %s region %d had %d errors during insertion", sheetName, regionIdx+1, len(insertResult.Errors))
					}
					continue
				} else if insertResult != nil && insertResult.HasErrors() {
					log.Printf("Sheet %s region %d processed with %d successful and %d failed rows", sheetName, regionIdx+1, insertResult.SuccessfulRows, insertResult.FailedRows)
				}

				// Analyze columns for metadata
				analyzer := NewSheetAnalyzer(resp.Values)
				columnAnalyses := analyzer.AnalyzeColumns(region)

				// Create and store import metadata
				metadata := &dtos.ImportMetadata{
					TableName:   currentTableName,
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

				log.Printf("GoogleSheetsDriver -> Successfully synced sheet '%s' region %d with %d rows", sheetName, regionIdx+1, len(region.DataRows))
			}
		} else {
			log.Printf("Warning: No data regions found in sheet %s", sheetName)
		}
	}

	// Update the connection's schema name
	conn.Config.SchemaName = schemaName
	log.Printf("GoogleSheetsDriver -> syncDataFromSheets -> Set schema name: %s", schemaName)

	return nil
}

// shouldSyncData checks if we need to sync data from Google Sheets
// Returns true if schema doesn't exist or is empty, false if it already has data
func (d *GoogleSheetsDriver) shouldSyncData(conn *Connection, schemaName string) (bool, error) {
	// Get the underlying SQL database connection
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return true, fmt.Errorf("failed to get SQL DB: %w", err)
	}

	// Check if schema exists
	schemaExistsQuery := `
		SELECT EXISTS (
			SELECT FROM information_schema.schemata 
			WHERE schema_name = $1
		)
	`
	var schemaExists bool
	if err := sqlDB.QueryRow(schemaExistsQuery, schemaName).Scan(&schemaExists); err != nil {
		return true, fmt.Errorf("failed to check if schema exists: %w", err)
	}

	if !schemaExists {
		log.Printf("GoogleSheetsDriver -> shouldSyncData -> Schema '%s' does not exist, needs sync", schemaName)
		return true, nil
	}

	// Check if schema has any tables with data
	tablesQuery := `
		SELECT COUNT(*) 
		FROM information_schema.tables 
		WHERE table_schema = $1 
		AND table_type = 'BASE TABLE'
	`
	var tableCount int
	if err := sqlDB.QueryRow(tablesQuery, schemaName).Scan(&tableCount); err != nil {
		return true, fmt.Errorf("failed to count tables in schema: %w", err)
	}

	if tableCount == 0 {
		log.Printf("GoogleSheetsDriver -> shouldSyncData -> Schema '%s' has no tables, needs sync", schemaName)
		return true, nil
	}

	// Check if any table has data (sample one table to see if it has rows)
	// Get first table name
	tableNameQuery := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = $1 
		AND table_type = 'BASE TABLE'
		LIMIT 1
	`
	var tableName string
	if err := sqlDB.QueryRow(tableNameQuery, schemaName).Scan(&tableName); err != nil {
		return true, fmt.Errorf("failed to get table name: %w", err)
	}

	// Check if the table has any rows
	rowCountQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s`, schemaName, tableName)
	var rowCount int
	if err := sqlDB.QueryRow(rowCountQuery).Scan(&rowCount); err != nil {
		log.Printf("GoogleSheetsDriver -> shouldSyncData -> Warning: Failed to count rows in table '%s.%s', assuming needs sync: %v", schemaName, tableName, err)
		return true, nil
	}

	if rowCount == 0 {
		log.Printf("GoogleSheetsDriver -> shouldSyncData -> Schema '%s' tables are empty, needs sync", schemaName)
		return true, nil
	}

	log.Printf("GoogleSheetsDriver -> shouldSyncData -> Schema '%s' already has %d rows in table '%s', skipping sync", schemaName, rowCount, tableName)
	return false, nil
}

// DataInsertionResult contains results of data insertion including error details
type DataInsertionResult struct {
	TotalRowsProcessed int
	SuccessfulRows     int
	FailedRows         int
	Errors             []string
}

// HasErrors returns true if there were any errors during insertion
func (r *DataInsertionResult) HasErrors() bool {
	return r.FailedRows > 0 || len(r.Errors) > 0
}

// storeSheetData stores sheet data in PostgreSQL
func (d *GoogleSheetsDriver) storeSheetData(db *sql.DB, schemaName, tableName string, headers []string, data [][]interface{}) (*DataInsertionResult, error) {
	// Drop existing table if it exists
	dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", schemaName, tableName)
	if _, err := db.Exec(dropQuery); err != nil {
		return nil, fmt.Errorf("failed to drop existing table: %w", err)
	}

	// Headers are already cleaned by the analyzer, just validate
	if len(headers) == 0 {
		return nil, fmt.Errorf("no columns found in sheet")
	}

	// Infer data types for columns using intelligent sampling
	inferrer := utils.NewDataTypeInferrer()
	inferredTypes, err := inferrer.InferColumnTypes(headers, data)
	if err != nil {
		log.Printf("Warning: Failed to infer column types: %v, falling back to TEXT", err)
		// Fallback to TEXT for all columns
		inferredTypes = make(map[string]utils.ColumnDataType)
		for _, header := range headers {
			inferredTypes[header] = utils.ColumnDataType{
				PostgreSQLType: "TEXT",
				SQLType:        "TEXT",
				IsNullable:     true,
			}
		}
	}

	// Create table with columns based on inferred types
	columns := make([]string, 0)
	for _, header := range headers {
		colName := sanitizeColumnName(header)
		dataType := inferredTypes[header]
		columns = append(columns, fmt.Sprintf("%s %s", colName, dataType.PostgreSQLType))

		log.Printf("Google Sheets Column %s -> %s (sample: %d, errors: %d)",
			header, dataType.PostgreSQLType, dataType.SampleSize, dataType.ErrorCount)
	}

	// Add internal columns
	columns = append(columns, "_row_id SERIAL PRIMARY KEY")
	columns = append(columns, "_imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")

	createQuery := fmt.Sprintf("CREATE TABLE %s.%s (%s)", schemaName, tableName, strings.Join(columns, ", "))
	if _, err := db.Exec(createQuery); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	// Insert data
	if len(data) > 0 {
		// Prepare column names for insert
		colNames := make([]string, 0)
		for _, header := range headers {
			colNames = append(colNames, sanitizeColumnName(header))
		}

		// Build insert query with type-aware conversion (batch insert for performance)
		batchSize := 100
		totalRows := 0
		successfulRows := 0
		failedRows := 0

		for i := 0; i < len(data); i += batchSize {
			end := i + batchSize
			if end > len(data) {
				end = len(data)
			}

			batch := data[i:end]
			validRows := make([]string, 0, len(batch)) // All rows for insertion

			for rowIdx, row := range batch {
				values := make([]string, 0)

				for j, header := range headers {
					var value string
					if j < len(row) && row[j] != nil {
						rawValue := fmt.Sprintf("%v", row[j])
						dataType := inferredTypes[header]

						// Convert value according to inferred type
						convertedValue, conversionErr := d.convertValueToType(rawValue, dataType.PostgreSQLType)
						if conversionErr != nil {
							// Instead of skipping the row, store NULL for invalid values
							log.Printf("CONVERSION_WARNING: Sheet '%s', Column '%s', Row %d: Cannot convert '%s' to %s, storing as NULL", 
								tableName, header, i+rowIdx+1, rawValue, dataType.PostgreSQLType)
							value = "" // Will be formatted as NULL by formatSQLValue
						} else {
							value = convertedValue
						}
					}
					// Use appropriate SQL value formatting
					values = append(values, d.formatSQLValue(value, inferredTypes[header].PostgreSQLType))
				}

				// Add all rows to the batch (no longer skipping rows with conversion errors)
				validRows = append(validRows, fmt.Sprintf("(%s)", strings.Join(values, ", ")))
				successfulRows++
				totalRows++
			}

			// Insert all rows (we no longer skip rows with conversion errors)
			if len(validRows) > 0 {
				insertQuery := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES %s",
					schemaName, tableName,
					strings.Join(colNames, ", "),
					strings.Join(validRows, ", "))

				if _, err := db.Exec(insertQuery); err != nil {
					// Log the batch failure but continue processing
					log.Printf("Error: Failed to insert batch %d-%d with %d rows: %v", i, end, len(validRows), err)
					log.Printf("Failed query: %s", insertQuery)
					// Mark these rows as failed
					failedRows += len(validRows)
					successfulRows -= len(validRows)
				}
			}
		}

		// Log comprehensive summary
		log.Printf("DATA_INSERTION_SUMMARY for sheet '%s':", tableName)
		log.Printf("  - Total rows processed: %d", totalRows)
		log.Printf("  - Successfully inserted: %d", successfulRows)
		log.Printf("  - Failed: %d", failedRows)

		// Return detailed results
		result := &DataInsertionResult{
			TotalRowsProcessed: totalRows,
			SuccessfulRows:     successfulRows,
			FailedRows:         failedRows,
			Errors:             []string{}, // No longer tracking individual errors
		}

		// Only return error if NO rows were successful
		if successfulRows == 0 && totalRows > 0 {
			return result, fmt.Errorf("no rows could be inserted into sheet '%s' - all %d rows failed database insertion", tableName, totalRows)
		}

		return result, nil
	}

	return &DataInsertionResult{
		TotalRowsProcessed: 0,
		SuccessfulRows:     0,
		FailedRows:         0,
		Errors:             []string{},
	}, nil
}

// convertValueToType attempts to convert a string value to the specified PostgreSQL type
func (d *GoogleSheetsDriver) convertValueToType(value string, postgresType string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil // NULL value
	}

	switch strings.ToUpper(postgresType) {
	case "INTEGER":
		// Remove common number formatting (commas, spaces) before parsing
		cleanValue := strings.ReplaceAll(value, ",", "")
		cleanValue = strings.ReplaceAll(cleanValue, " ", "")

		_, err := strconv.ParseInt(cleanValue, 10, 64)
		if err != nil {
			return "", fmt.Errorf("cannot convert '%s' to INTEGER: %w", value, err)
		}
		return cleanValue, nil

	case "NUMERIC", "DECIMAL":
		// Remove common number formatting (commas, spaces) before parsing
		cleanValue := strings.ReplaceAll(value, ",", "")
		cleanValue = strings.ReplaceAll(cleanValue, " ", "")

		_, err := strconv.ParseFloat(cleanValue, 64)
		if err != nil {
			return "", fmt.Errorf("cannot convert '%s' to NUMERIC: %w", value, err)
		}
		return cleanValue, nil

	case "BOOLEAN":
		lower := strings.ToLower(value)
		switch lower {
		case "true", "yes", "1", "y":
			return "true", nil
		case "false", "no", "0", "n":
			return "false", nil
		default:
			return "", fmt.Errorf("cannot convert '%s' to BOOLEAN", value)
		}

	case "DATE":
		// Try to parse various date formats
		dateFormats := []string{
			"2006-01-02",
			"01/02/2006",
			"01-02-2006",
			"2006/01/02",
			"02/01/2006",
			"02-01-2006",
		}
		for _, format := range dateFormats {
			if parsedTime, err := time.Parse(format, value); err == nil {
				return parsedTime.Format("2006-01-02"), nil
			}
		}
		return "", fmt.Errorf("cannot convert '%s' to DATE", value)

	case "TIMESTAMP":
		// Try to parse various timestamp formats
		timestampFormats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05.000",
			"01/02/2006 15:04:05",
			"01-02-2006 15:04:05",
		}
		for _, format := range timestampFormats {
			if parsedTime, err := time.Parse(format, value); err == nil {
				return parsedTime.Format("2006-01-02 15:04:05"), nil
			}
		}
		return "", fmt.Errorf("cannot convert '%s' to TIMESTAMP", value)

	default:
		// For TEXT, VARCHAR, UUID, etc., return as-is
		return value, nil
	}
}

// formatSQLValue formats a value for SQL insertion based on the column type
func (d *GoogleSheetsDriver) formatSQLValue(value string, postgresType string) string {
	if value == "" {
		return "NULL"
	}

	switch strings.ToUpper(postgresType) {
	case "INTEGER", "NUMERIC", "DECIMAL", "BOOLEAN":
		// Numeric and boolean values don't need quotes
		return value
	default:
		// String types need quotes and escaping
		escaped := strings.ReplaceAll(value, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	}
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

// Helper function to convert column index to letter
func columnIndexToName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+index%26)) + name
		index = index/26 - 1
	}
	return name
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
