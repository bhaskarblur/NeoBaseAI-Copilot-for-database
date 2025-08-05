package dbmanager

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ClickHouseSchemaFetcher implements schema fetching for ClickHouse
type ClickHouseSchemaFetcher struct {
	db DBExecutor
}

// NewClickHouseSchemaFetcher creates a new ClickHouse schema fetcher
func NewClickHouseSchemaFetcher(db DBExecutor) SchemaFetcher {
	return &ClickHouseSchemaFetcher{db: db}
}

// GetSchema retrieves the schema for the selected tables
func (f *ClickHouseSchemaFetcher) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	log.Printf("ClickHouseSchemaFetcher -> GetSchema -> Starting schema fetch with selected tables: %v", selectedTables)

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseSchemaFetcher -> GetSchema -> Context cancelled: %v", err)
		return nil, fmt.Errorf("context cancelled: %v", err)
	}

	// Test connection with a simple query
	var result int
	if err := db.Query("SELECT 1", &result); err != nil {
		log.Printf("ClickHouseSchemaFetcher -> GetSchema -> Connection test failed: %v", err)
		return nil, fmt.Errorf("connection test failed: %v", err)
	}
	log.Printf("ClickHouseSchemaFetcher -> GetSchema -> Connection test successful")

	// Fetch the full schema
	schema, err := f.FetchSchema(ctx)
	if err != nil {
		log.Printf("ClickHouseSchemaFetcher -> GetSchema -> Error fetching schema: %v", err)
		return nil, fmt.Errorf("failed to fetch schema: %v", err)
	}

	// Log the tables and their column counts
	for tableName, table := range schema.Tables {
		log.Printf("ClickHouseSchemaFetcher -> GetSchema -> Table: %s, Columns: %d, Row Count: %d",
			tableName, len(table.Columns), table.RowCount)
	}

	// Filter the schema based on selected tables
	filteredSchema := f.filterSchemaForSelectedTables(schema, selectedTables)
	log.Printf("ClickHouseSchemaFetcher -> GetSchema -> Filtered schema to %d tables", len(filteredSchema.Tables))

	return filteredSchema, nil
}

// FetchSchema retrieves the full database schema
func (f *ClickHouseSchemaFetcher) FetchSchema(ctx context.Context) (*SchemaInfo, error) {
	log.Printf("ClickHouseSchemaFetcher -> FetchSchema -> Starting full schema fetch")

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseSchemaFetcher -> FetchSchema -> Context cancelled: %v", err)
		return nil, fmt.Errorf("context cancelled: %v", err)
	}

	schema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: time.Now(),
	}

	// Fetch tables
	tables, err := f.fetchTables(ctx)
	if err != nil {
		log.Printf("ClickHouseSchemaFetcher -> FetchSchema -> Error fetching tables: %v", err)
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}

	log.Printf("ClickHouseSchemaFetcher -> FetchSchema -> Processing %d tables", len(tables))

	for _, table := range tables {
		log.Printf("ClickHouseSchemaFetcher -> FetchSchema -> Processing table: %s", table)

		tableSchema := TableSchema{
			Name:        table,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
			Constraints: make(map[string]ConstraintInfo),
		}

		// Fetch columns
		columns, err := f.fetchColumns(ctx, table)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch columns for table %s: %v", table, err)
		}
		tableSchema.Columns = columns
		log.Printf("ClickHouseSchemaFetcher -> FetchSchema -> Fetched %d columns for table %s", len(columns), table)

		// Get row count
		rowCount, err := f.getTableRowCount(ctx, table)
		if err != nil {
			return nil, fmt.Errorf("failed to get row count for table %s: %v", table, err)
		}
		tableSchema.RowCount = rowCount
		log.Printf("ClickHouseSchemaFetcher -> FetchSchema -> Table %s has %d rows", table, rowCount)

		// Calculate table schema checksum
		tableData, _ := json.Marshal(tableSchema)
		tableSchema.Checksum = fmt.Sprintf("%x", md5.Sum(tableData))

		schema.Tables[table] = tableSchema
	}

	// Calculate overall schema checksum
	schemaData, _ := json.Marshal(schema.Tables)
	schema.Checksum = fmt.Sprintf("%x", md5.Sum(schemaData))

	log.Printf("ClickHouseSchemaFetcher -> FetchSchema -> Successfully completed schema fetch with %d tables",
		len(schema.Tables))

	return schema, nil
}

// TableInfo holds additional ClickHouse table metadata
type TableInfo struct {
	Engine       string
	PartitionKey string
	OrderBy      string
	PrimaryKey   []string
}

// fetchTables retrieves all tables in the database
func (f *ClickHouseSchemaFetcher) fetchTables(_ context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT name 
        FROM system.tables 
        WHERE database = currentDatabase() 
        AND engine NOT LIKE 'View%'
        ORDER BY name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

// fetchColumns retrieves all columns for a specific table
func (f *ClickHouseSchemaFetcher) fetchColumns(ctx context.Context, table string) (map[string]ColumnInfo, error) {
	log.Printf("ClickHouseSchemaFetcher -> fetchColumns -> Starting for table: %s", table)

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseSchemaFetcher -> fetchColumns -> Context cancelled: %v", err)
		return nil, fmt.Errorf("context cancelled: %v", err)
	}

	columns := make(map[string]ColumnInfo)
	var columnList []struct {
		Name         string `db:"name"`
		Type         string `db:"type"`
		DefaultType  string `db:"default_kind"`
		DefaultValue string `db:"default_expression"`
		Comment      string `db:"comment"`
	}

	query := `
        SELECT 
            name,
            type,
            default_kind,
            default_expression,
            comment
        FROM system.columns
        WHERE database = currentDatabase()
        AND table = ?
        ORDER BY position;
    `
	log.Printf("ClickHouseSchemaFetcher -> fetchColumns -> Executing query for table: %s", table)
	err := f.db.Query(query, &columnList, table)
	if err != nil {
		log.Printf("ClickHouseSchemaFetcher -> fetchColumns -> Error fetching columns for table %s: %v", table, err)
		return nil, fmt.Errorf("failed to fetch columns for table %s: %v", table, err)
	}

	log.Printf("ClickHouseSchemaFetcher -> fetchColumns -> Found %d columns for table: %s", len(columnList), table)

	for _, col := range columnList {
		// In ClickHouse, columns are nullable if the type contains "Nullable"
		isNullable := strings.Contains(col.Type, "Nullable")

		// Format default value
		defaultValue := ""
		if col.DefaultType != "" && col.DefaultValue != "" {
			defaultValue = fmt.Sprintf("%s %s", col.DefaultType, col.DefaultValue)
		}

		columns[col.Name] = ColumnInfo{
			Name:         col.Name,
			Type:         col.Type,
			IsNullable:   isNullable,
			DefaultValue: defaultValue,
			Comment:      col.Comment,
		}

		log.Printf("ClickHouseSchemaFetcher -> fetchColumns -> Processed column: %s, type: %s, nullable: %v",
			col.Name, col.Type, isNullable)
	}

	log.Printf("ClickHouseSchemaFetcher -> fetchColumns -> Successfully fetched %d columns for table: %s",
		len(columns), table)
	return columns, nil
}

// fetchTableInfo retrieves additional metadata for a table
func (f *ClickHouseSchemaFetcher) fetchTableInfo(_ context.Context, table string) (*TableInfo, error) {
	info := &TableInfo{}

	// Get engine
	var engine string
	engineQuery := `
        SELECT engine
        FROM system.tables
        WHERE database = currentDatabase()
        AND name = ?;
    `
	err := f.db.Query(engineQuery, &engine, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch engine for table %s: %v", table, err)
	}
	info.Engine = engine

	// Get partition key, order by, and primary key
	var tableSettings []struct {
		Name  string `db:"name"`
		Value string `db:"value"`
	}

	settingsQuery := `
        SELECT name, value
        FROM system.table_settings
        WHERE database = currentDatabase()
        AND table = ?
        AND name IN ('partition_key', 'sorting_key', 'primary_key');
    `
	err = f.db.Query(settingsQuery, &tableSettings, table)
	if err != nil {
		// Some engines don't have these settings, so we'll just continue
		return info, nil
	}

	for _, setting := range tableSettings {
		switch setting.Name {
		case "partition_key":
			info.PartitionKey = setting.Value
		case "sorting_key":
			info.OrderBy = setting.Value
		case "primary_key":
			// Primary key is a comma-separated list of columns
			if setting.Value != "" {
				info.PrimaryKey = strings.Split(setting.Value, ",")
				// Trim whitespace from each column name
				for i, col := range info.PrimaryKey {
					info.PrimaryKey[i] = strings.TrimSpace(col)
				}
			}
		}
	}

	return info, nil
}

// fetchViews retrieves all views in the database
func (f *ClickHouseSchemaFetcher) fetchViews(_ context.Context) (map[string]ViewSchema, error) {
	views := make(map[string]ViewSchema)
	var viewList []struct {
		Name       string `db:"name"`
		Definition string `db:"create_table_query"`
	}

	query := `
        SELECT 
            name,
            create_table_query
        FROM system.tables
        WHERE database = currentDatabase()
        AND engine LIKE 'View%'
        ORDER BY name;
    `
	err := f.db.Query(query, &viewList)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch views: %v", err)
	}

	for _, view := range viewList {
		views[view.Name] = ViewSchema{
			Name:       view.Name,
			Definition: view.Definition,
		}
	}
	return views, nil
}

// getTableRowCount gets the number of rows in a table
func (f *ClickHouseSchemaFetcher) getTableRowCount(ctx context.Context, table string) (int64, error) {
	log.Printf("Getting row count for table: %s", table)
	var count int64

	// Check for context cancellation
	if ctx.Err() != nil {
		return 0, fmt.Errorf("context cancelled while getting row count for table %s: %v", table, ctx.Err())
	}

	// First try to get from system.tables which is faster but approximate
	approxQuery := `
        SELECT 
            total_rows
        FROM system.tables
        WHERE database = currentDatabase()
        AND name = ?;
    `
	log.Printf("Executing approximate row count query for table: %s", table)
	err := f.db.Query(approxQuery, &count, table)
	if err != nil {
		log.Printf("Warning: Failed to get approximate row count for table %s: %v", table, err)
	} else if count > 0 {
		log.Printf("Successfully retrieved approximate row count for table %s: %d rows", table, count)
		return count, nil
	}

	// Check for context cancellation again before the exact count
	if ctx.Err() != nil {
		return 0, fmt.Errorf("context cancelled while getting row count for table %s: %v", table, ctx.Err())
	}

	// If error or zero (which might mean the count is not available), try counting
	log.Printf("Falling back to exact row count for table: %s", table)
	countQuery := fmt.Sprintf("SELECT count(*) FROM `%s`", table)
	err = f.db.Query(countQuery, &count)
	if err != nil {
		return 0, fmt.Errorf("failed to get exact row count for table %s: %v", table, err)
	}

	log.Printf("Successfully retrieved exact row count for table %s: %d rows", table, count)
	return count, nil
}

// GetTableChecksum calculates a checksum for a table's structure
func (f *ClickHouseSchemaFetcher) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	log.Printf("ClickHouseSchemaFetcher -> GetTableChecksum -> Starting for table: %s", table)

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseSchemaFetcher -> GetTableChecksum -> Context cancelled: %v", err)
		return "", fmt.Errorf("context cancelled: %v", err)
	}

	// Get table definition
	var tableDefinition string
	query := `
        SELECT create_table_query
        FROM system.tables
        WHERE database = currentDatabase()
        AND name = ?;
    `

	log.Printf("ClickHouseSchemaFetcher -> GetTableChecksum -> Executing query for table: %s", table)
	err := db.Query(query, &tableDefinition, table)
	if err != nil {
		log.Printf("ClickHouseSchemaFetcher -> GetTableChecksum -> Error getting table definition for %s: %v", table, err)
		return "", fmt.Errorf("failed to get table definition: %v", err)
	}

	if tableDefinition == "" {
		log.Printf("ClickHouseSchemaFetcher -> GetTableChecksum -> No table definition found for table: %s", table)
		return "", fmt.Errorf("no table definition found for table: %s", table)
	}

	// Calculate checksum
	checksum := fmt.Sprintf("%x", md5.Sum([]byte(tableDefinition)))
	log.Printf("ClickHouseSchemaFetcher -> GetTableChecksum -> Successfully calculated checksum for table: %s", table)
	return checksum, nil
}

// FetchExampleRecords retrieves sample records from a table
func (f *ClickHouseSchemaFetcher) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Starting for table: %s with limit: %d", table, limit)

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Context cancelled: %v", err)
		return nil, fmt.Errorf("context cancelled: %v", err)
	}

	// Ensure limit is reasonable
	if limit <= 0 {
		limit = 3 // Default to 3 records
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Adjusted limit to default: %d", limit)
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Capped limit to maximum: %d", limit)
	}

	// Try to identify time-based or ID columns to order by for getting latest records
	var columns []struct {
		Name string `db:"name"`
		Type string `db:"type"`
	}

	columnQuery := `
		SELECT name, type
		FROM system.columns
		WHERE database = currentDatabase()
		AND table = ?
	`

	err := db.Query(columnQuery, &columns, table)
	if err != nil {
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Error fetching columns for table %s: %v", table, err)
		// Fall back to simple LIMIT query if column metadata isn't available
		query := fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", table, limit)
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Using simple query: %s", query)

		var records []map[string]interface{}
		err := db.QueryRows(query, &records)
		if err != nil {
			log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Error fetching records from table %s: %v", table, err)
			return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
		}
		return processClickHouseRecords(records)
	}

	// Also check if the table has a predefined sorting key (ORDER BY) in ClickHouse
	var tableInfo *TableInfo
	tableInfo, err = f.fetchTableInfo(ctx, table)
	if err != nil {
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Error fetching table info: %v", err)
		// Continue anyway, we'll try to find appropriate timestamp columns
	}

	// If the table has a defined ORDER BY clause, use that
	var orderByColumn string
	if tableInfo != nil && tableInfo.OrderBy != "" {
		// Extract the first column from the ORDER BY clause (could be multiple columns)
		orderParts := strings.Split(tableInfo.OrderBy, ",")
		firstOrderColumn := strings.TrimSpace(orderParts[0])

		// Extract just the column name (in case it includes ASC/DESC)
		if spaceIdx := strings.Index(firstOrderColumn, " "); spaceIdx > 0 {
			firstOrderColumn = firstOrderColumn[:spaceIdx]
		}

		// Use this column for ordering
		orderByColumn = firstOrderColumn
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Using table's ORDER BY column: %s", orderByColumn)
	}

	// If no ORDER BY defined in table, look for common timestamp columns
	if orderByColumn == "" {
		// Common column names that might indicate recency (in priority order)
		timeColumns := []string{"updated_at", "modified_at", "update_time", "updated", "modified", "modified_time", "last_update", "last_modified"}
		creationColumns := []string{"created_at", "creation_time", "create_time", "created", "creation_date", "insert_time", "timestamp", "time", "datetime", "date"}
		idColumns := []string{"id", "uuid", "record_id", "key", "primary_key"}

		// First try timestamp columns with datetime/timestamp types
		// Check for time-based columns first (update timestamps take priority over creation timestamps)
		for _, colPattern := range timeColumns {
			for _, col := range columns {
				if strings.EqualFold(col.Name, colPattern) &&
					(strings.Contains(strings.ToLower(col.Type), "date") ||
						strings.Contains(strings.ToLower(col.Type), "time")) {
					orderByColumn = col.Name
					log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Using modification timestamp column: %s", orderByColumn)
					break
				}
			}
			if orderByColumn != "" {
				break
			}
		}

		// If no update timestamp found, look for creation timestamps
		if orderByColumn == "" {
			for _, colPattern := range creationColumns {
				for _, col := range columns {
					if strings.EqualFold(col.Name, colPattern) &&
						(strings.Contains(strings.ToLower(col.Type), "date") ||
							strings.Contains(strings.ToLower(col.Type), "time")) {
						orderByColumn = col.Name
						log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Using creation timestamp column: %s", orderByColumn)
						break
					}
				}
				if orderByColumn != "" {
					break
				}
			}
		}

		// As a last resort, look for ID columns (assuming higher IDs are newer records)
		if orderByColumn == "" {
			for _, colPattern := range idColumns {
				for _, col := range columns {
					if strings.EqualFold(col.Name, colPattern) &&
						(strings.Contains(strings.ToLower(col.Type), "int") ||
							strings.Contains(strings.ToLower(col.Type), "uuid") ||
							strings.Contains(strings.ToLower(col.Type), "string")) {
						orderByColumn = col.Name
						log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Using ID column: %s", orderByColumn)
						break
					}
				}
				if orderByColumn != "" {
					break
				}
			}
		}
	}

	// Build the query based on whether we found a column to order by
	var query string
	if orderByColumn != "" {
		// Use backticks around identifiers to handle reserved words
		query = fmt.Sprintf("SELECT * FROM `%s` ORDER BY `%s` DESC LIMIT %d", table, orderByColumn, limit)
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Using ordered query: %s", query)
	} else {
		// Fallback to simple query without ORDER BY
		query = fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", table, limit)
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> No suitable ordering column found, using simple query: %s", query)
	}

	var records []map[string]interface{}
	err = db.QueryRows(query, &records)
	if err != nil {
		log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Error fetching example records for table %s: %v", table, err)

		// If ordering failed (possibly due to syntax or unsupported column type), try without ordering
		if orderByColumn != "" {
			log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Ordering failed, falling back to simple query")
			simpleQuery := fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", table, limit)
			err = db.QueryRows(simpleQuery, &records)
			if err != nil {
				log.Printf("ClickHouseSchemaFetcher -> FetchExampleRecords -> Simple query also failed: %v", err)
				return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
			}
		} else {
			return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
		}
	}

	return processClickHouseRecords(records)
}

// Process ClickHouse records to handle specific data types
func processClickHouseRecords(records []map[string]interface{}) ([]map[string]interface{}, error) {
	// If no records found, return empty slice
	if len(records) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Process records to ensure all values are properly formatted
	for i, record := range records {
		for key, value := range record {
			// Handle nil values
			if value == nil {
				continue
			}

			// Handle byte arrays
			if byteVal, ok := value.([]byte); ok {
				// Try to convert to string
				records[i][key] = string(byteVal)
			}
		}
	}

	return records, nil
}

// FetchTableList retrieves a list of all tables in the database
func (f *ClickHouseSchemaFetcher) FetchTableList(ctx context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT name 
        FROM system.tables 
        WHERE database = currentDatabase() 
        AND engine NOT LIKE 'View%'
        ORDER BY name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

// filterSchemaForSelectedTables filters the schema to only include elements related to the selected tables
func (f *ClickHouseSchemaFetcher) filterSchemaForSelectedTables(schema *SchemaInfo, selectedTables []string) *SchemaInfo {
	log.Printf("ClickHouseSchemaFetcher -> filterSchemaForSelectedTables -> Starting with %d tables in schema and %d selected tables",
		len(schema.Tables), len(selectedTables))

	// If no tables are selected or "ALL" is selected, return the full schema
	if len(selectedTables) == 0 || (len(selectedTables) == 1 && selectedTables[0] == "ALL") {
		log.Printf("ClickHouseSchemaFetcher -> filterSchemaForSelectedTables -> No filtering needed, returning full schema")
		return schema
	}

	// Create a map for quick lookup of selected tables
	selectedTablesMap := make(map[string]bool)
	for _, table := range selectedTables {
		selectedTablesMap[table] = true
		log.Printf("ClickHouseSchemaFetcher -> filterSchemaForSelectedTables -> Added table to selection: %s", table)
	}

	// Create a new filtered schema
	filteredSchema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: schema.UpdatedAt,
	}

	// Filter tables
	for tableName, tableSchema := range schema.Tables {
		if selectedTablesMap[tableName] {
			log.Printf("ClickHouseSchemaFetcher -> filterSchemaForSelectedTables -> Including table: %s with %d columns",
				tableName, len(tableSchema.Columns))
			filteredSchema.Tables[tableName] = tableSchema
		}
	}

	// Calculate new checksum for filtered schema
	schemaData, _ := json.Marshal(filteredSchema.Tables)
	filteredSchema.Checksum = fmt.Sprintf("%x", md5.Sum(schemaData))

	log.Printf("ClickHouseSchemaFetcher -> filterSchemaForSelectedTables -> Filtered schema contains %d tables",
		len(filteredSchema.Tables))

	return filteredSchema
}
