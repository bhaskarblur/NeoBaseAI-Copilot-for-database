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

// MySQLSchemaFetcher implements schema fetching for MySQL
type MySQLSchemaFetcher struct {
	db DBExecutor
}

// NewMySQLSchemaFetcher creates a new MySQL schema fetcher
func NewMySQLSchemaFetcher(db DBExecutor) SchemaFetcher {
	return &MySQLSchemaFetcher{db: db}
}

// GetSchema retrieves the schema for the selected tables
func (f *MySQLSchemaFetcher) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	log.Printf("MySQLSchemaFetcher -> GetSchema -> Starting schema fetch with selected tables: %v", selectedTables)

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MySQLSchemaFetcher -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Fetch the full schema
	schema, err := f.FetchSchema(ctx)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> GetSchema -> Error fetching schema: %v", err)
		return nil, err
	}

	log.Printf("MySQLSchemaFetcher -> GetSchema -> Successfully fetched schema with %d tables", len(schema.Tables))

	// Log the tables and their column counts
	for tableName, table := range schema.Tables {
		log.Printf("MySQLSchemaFetcher -> GetSchema -> Table: %s, Columns: %d, Row Count: %d",
			tableName, len(table.Columns), table.RowCount)
	}

	// Filter the schema based on selected tables
	filteredSchema := f.filterSchemaForSelectedTables(schema, selectedTables)
	log.Printf("MySQLSchemaFetcher -> GetSchema -> Filtered schema to %d tables", len(filteredSchema.Tables))

	return filteredSchema, nil
}

// FetchSchema retrieves the full database schema
func (f *MySQLSchemaFetcher) FetchSchema(ctx context.Context) (*SchemaInfo, error) {
	log.Printf("MySQLSchemaFetcher -> FetchSchema -> Starting full schema fetch")

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MySQLSchemaFetcher -> FetchSchema -> Context cancelled: %v", err)
		return nil, err
	}

	schema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: time.Now(),
	}

	// Fetch tables
	tables, err := f.fetchTables(ctx)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> FetchSchema -> Error fetching tables: %v", err)
		return nil, err
	}

	log.Printf("MySQLSchemaFetcher -> FetchSchema -> Processing %d tables", len(tables))

	for _, table := range tables {
		log.Printf("MySQLSchemaFetcher -> FetchSchema -> Processing table: %s", table)

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
			log.Printf("MySQLSchemaFetcher -> FetchSchema -> Error fetching columns for table %s: %v", table, err)
			return nil, fmt.Errorf("failed to fetch columns for table %s: %v", table, err)
		}
		tableSchema.Columns = columns
		log.Printf("MySQLSchemaFetcher -> FetchSchema -> Fetched %d columns for table %s", len(columns), table)

		// Fetch indexes
		indexes, err := f.fetchIndexes(ctx, table)
		if err != nil {
			log.Printf("MySQLSchemaFetcher -> FetchSchema -> Error fetching indexes for table %s: %v", table, err)
			return nil, fmt.Errorf("failed to fetch indexes for table %s: %v", table, err)
		}
		tableSchema.Indexes = indexes
		log.Printf("MySQLSchemaFetcher -> FetchSchema -> Fetched %d indexes for table %s", len(indexes), table)

		// Fetch foreign keys
		fkeys, err := f.fetchForeignKeys(ctx, table)
		if err != nil {
			log.Printf("MySQLSchemaFetcher -> FetchSchema -> Error fetching foreign keys for table %s: %v", table, err)
			return nil, fmt.Errorf("failed to fetch foreign keys for table %s: %v", table, err)
		}
		tableSchema.ForeignKeys = fkeys
		log.Printf("MySQLSchemaFetcher -> FetchSchema -> Fetched %d foreign keys for table %s", len(fkeys), table)

		// Fetch constraints
		constraints, err := f.fetchConstraints(ctx, table)
		if err != nil {
			log.Printf("MySQLSchemaFetcher -> FetchSchema -> Error fetching constraints for table %s: %v", table, err)
			return nil, fmt.Errorf("failed to fetch constraints for table %s: %v", table, err)
		}
		tableSchema.Constraints = constraints
		log.Printf("MySQLSchemaFetcher -> FetchSchema -> Fetched %d constraints for table %s", len(constraints), table)

		// Get row count
		rowCount, err := f.getTableRowCount(ctx, table)
		if err != nil {
			log.Printf("MySQLSchemaFetcher -> FetchSchema -> Error getting row count for table %s: %v", table, err)
			return nil, fmt.Errorf("failed to get row count for table %s: %v", table, err)
		}
		tableSchema.RowCount = rowCount
		log.Printf("MySQLSchemaFetcher -> FetchSchema -> Table %s has %d rows", table, rowCount)

		// Calculate table schema checksum
		tableData, _ := json.Marshal(tableSchema)
		tableSchema.Checksum = fmt.Sprintf("%x", md5.Sum(tableData))

		schema.Tables[table] = tableSchema
	}

	// Fetch views
	views, err := f.fetchViews(ctx)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> FetchSchema -> Error fetching views: %v", err)
		return nil, fmt.Errorf("failed to fetch views: %v", err)
	}
	schema.Views = views
	log.Printf("MySQLSchemaFetcher -> FetchSchema -> Fetched %d views", len(views))

	// Calculate overall schema checksum
	schemaData, _ := json.Marshal(schema.Tables)
	schema.Checksum = fmt.Sprintf("%x", md5.Sum(schemaData))

	log.Printf("MySQLSchemaFetcher -> FetchSchema -> Successfully completed schema fetch with %d tables and %d views",
		len(schema.Tables), len(schema.Views))

	return schema, nil
}

// fetchTables retrieves all tables in the database
func (f *MySQLSchemaFetcher) fetchTables(_ context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = DATABASE() 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name;
    `
	log.Printf("MySQLSchemaFetcher -> fetchTables -> Executing query: %s", query)
	err := f.db.Query(query, &tables)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> fetchTables -> Error: %v", err)
		log.Printf("MySQLSchemaFetcher -> fetchTables -> Trying alternative SHOW TABLES approach")

		// Try using SHOW TABLES
		var showTablesResults []map[string]interface{}
		showTablesQuery := "SHOW TABLES"
		err := f.db.QueryRows(showTablesQuery, &showTablesResults)
		if err != nil {
			log.Printf("MySQLSchemaFetcher -> fetchTables -> SHOW TABLES error: %v", err)
			return nil, fmt.Errorf("failed to fetch tables: %v", err)
		}

		log.Printf("MySQLSchemaFetcher -> fetchTables -> SHOW TABLES found %d tables: %v",
			len(showTablesResults), showTablesResults)

		// Process SHOW TABLES results
		for _, tableRow := range showTablesResults {
			// SHOW TABLES returns a single column with the table name
			// The column name is "Tables_in_<database_name>"
			for _, value := range tableRow {
				if tableName, ok := value.(string); ok && tableName != "" {
					tables = append(tables, tableName)
					log.Printf("MySQLSchemaFetcher -> fetchTables -> Added table from SHOW TABLES: %s", tableName)
				}
			}
		}

		if len(tables) == 0 {
			log.Printf("MySQLSchemaFetcher -> fetchTables -> No tables found using SHOW TABLES")
			return nil, fmt.Errorf("no tables found in database")
		}

		return tables, nil
	}
	log.Printf("MySQLSchemaFetcher -> fetchTables -> Found %d tables: %v", len(tables), tables)
	return tables, nil
}

// fetchColumns retrieves all columns for a specific table
func (f *MySQLSchemaFetcher) fetchColumns(_ context.Context, table string) (map[string]ColumnInfo, error) {
	columns := make(map[string]ColumnInfo)

	// Try using DESCRIBE table first, which is more reliable
	log.Printf("MySQLSchemaFetcher -> fetchColumns -> Using DESCRIBE for table %s", table)
	var describeResults []map[string]interface{}
	describeQuery := fmt.Sprintf("DESCRIBE `%s`", table)
	err := f.db.QueryRows(describeQuery, &describeResults)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> fetchColumns -> DESCRIBE error for table %s: %v", table, err)
	} else if len(describeResults) > 0 {
		log.Printf("MySQLSchemaFetcher -> fetchColumns -> DESCRIBE found %d columns for table %s: %+v",
			len(describeResults), table, describeResults)

		// Process DESCRIBE results
		for _, col := range describeResults {
			// Debug the column data
			log.Printf("MySQLSchemaFetcher -> fetchColumns -> Processing column data: %+v", col)

			// Extract field values, handling potential type conversions
			var name, dataType, nullable, defaultVal string

			// Handle Field
			if fieldVal, ok := col["Field"]; ok {
				if strVal, ok := fieldVal.(string); ok {
					name = strVal
				} else if byteVal, ok := fieldVal.([]byte); ok {
					name = string(byteVal)
				}
			}

			// Handle Type
			if typeVal, ok := col["Type"]; ok {
				if strVal, ok := typeVal.(string); ok {
					dataType = strVal
				} else if byteVal, ok := typeVal.([]byte); ok {
					dataType = string(byteVal)
				}
			}

			// Handle Null
			if nullVal, ok := col["Null"]; ok {
				if strVal, ok := nullVal.(string); ok {
					nullable = strVal
				} else if byteVal, ok := nullVal.([]byte); ok {
					nullable = string(byteVal)
				}
			}

			// Handle Default
			if defaultValField, ok := col["Default"]; ok && defaultValField != nil {
				if strVal, ok := defaultValField.(string); ok {
					defaultVal = strVal
				} else if byteVal, ok := defaultValField.([]byte); ok {
					defaultVal = string(byteVal)
				}
			}

			if name != "" {
				isNullable := false
				if nullable == "YES" {
					isNullable = true
				}

				columns[name] = ColumnInfo{
					Name:         name,
					Type:         dataType,
					IsNullable:   isNullable,
					DefaultValue: defaultVal,
					Comment:      "",
				}
				log.Printf("MySQLSchemaFetcher -> fetchColumns -> Added column from DESCRIBE: %s, Type: %s, IsNullable: %v",
					name, dataType, isNullable)
			}
		}

		// If we found columns using DESCRIBE, return them
		if len(columns) > 0 {
			return columns, nil
		}
	}

	// Fall back to information_schema if DESCRIBE didn't work
	var columnList []struct {
		Name         string `db:"column_name"`
		Type         string `db:"data_type"`
		IsNullable   string `db:"is_nullable"`
		DefaultValue string `db:"column_default"`
		Comment      string `db:"column_comment"`
	}

	query := `
        SELECT 
            column_name,
            data_type,
            is_nullable,
            column_default,
            column_comment
        FROM information_schema.columns
        WHERE table_schema = DATABASE()
        AND table_name = ?
        ORDER BY ordinal_position;
    `
	log.Printf("MySQLSchemaFetcher -> fetchColumns -> Executing query for table %s: %s", table, query)
	err = f.db.Query(query, &columnList, table)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> fetchColumns -> Error for table %s: %v", table, err)
		return nil, fmt.Errorf("failed to fetch columns for table %s: %v", table, err)
	}
	log.Printf("MySQLSchemaFetcher -> fetchColumns -> Found %d columns for table %s", len(columnList), table)

	// Debug the column list
	for i, col := range columnList {
		log.Printf("MySQLSchemaFetcher -> fetchColumns -> Column %d: Name=%s, Type=%s, IsNullable=%s, DefaultValue=%s, Comment=%s",
			i, col.Name, col.Type, col.IsNullable, col.DefaultValue, col.Comment)
	}

	for _, col := range columnList {
		if col.Name != "" {
			log.Printf("MySQLSchemaFetcher -> fetchColumns -> Adding column: %s, Type: %s, IsNullable: %s", col.Name, col.Type, col.IsNullable)
			columns[col.Name] = ColumnInfo{
				Name:         col.Name,
				Type:         col.Type,
				IsNullable:   col.IsNullable == "YES",
				DefaultValue: col.DefaultValue,
				Comment:      col.Comment,
			}
		}
	}

	// If we still have no columns, try a direct query
	if len(columns) == 0 {
		log.Printf("MySQLSchemaFetcher -> fetchColumns -> No columns found, trying direct query")
		var directResults []map[string]interface{}
		directQuery := fmt.Sprintf("SELECT * FROM `%s` LIMIT 1", table)
		err := f.db.QueryRows(directQuery, &directResults)
		if err != nil {
			log.Printf("MySQLSchemaFetcher -> fetchColumns -> Direct query error for table %s: %v", table, err)
		} else if len(directResults) > 0 {
			log.Printf("MySQLSchemaFetcher -> fetchColumns -> Direct query found columns for table %s: %+v", table, directResults[0])

			// Extract column names from the result
			for colName := range directResults[0] {
				columns[colName] = ColumnInfo{
					Name:         colName,
					Type:         "varchar", // Default type
					IsNullable:   true,      // Default to nullable
					DefaultValue: "",
					Comment:      "",
				}
				log.Printf("MySQLSchemaFetcher -> fetchColumns -> Added column from direct query: %s", colName)
			}
		}
	}

	return columns, nil
}

// fetchIndexes retrieves all indexes for a specific table
func (f *MySQLSchemaFetcher) fetchIndexes(_ context.Context, table string) (map[string]IndexInfo, error) {
	indexes := make(map[string]IndexInfo)
	var indexList []struct {
		Name     string `db:"index_name"`
		Column   string `db:"column_name"`
		IsUnique bool   `db:"non_unique"`
	}

	query := `
        SELECT 
            index_name,
            column_name,
            non_unique = 0 as non_unique
        FROM information_schema.statistics
        WHERE table_schema = DATABASE()
        AND table_name = ?
        ORDER BY index_name, seq_in_index;
    `
	log.Printf("MySQLSchemaFetcher -> fetchIndexes -> Executing query for table %s: %s", table, query)
	err := f.db.Query(query, &indexList, table)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> fetchIndexes -> Error for table %s: %v", table, err)
		log.Printf("MySQLSchemaFetcher -> fetchIndexes -> Trying alternative SHOW INDEX approach")

		// Try using SHOW INDEX
		var showIndexResults []map[string]interface{}
		showIndexQuery := fmt.Sprintf("SHOW INDEX FROM `%s`", table)
		err := f.db.QueryRows(showIndexQuery, &showIndexResults)
		if err != nil {
			log.Printf("MySQLSchemaFetcher -> fetchIndexes -> SHOW INDEX error for table %s: %v", table, err)
			// Return empty indexes rather than failing
			log.Printf("MySQLSchemaFetcher -> fetchIndexes -> Returning empty indexes for table %s", table)
			return indexes, nil
		}

		log.Printf("MySQLSchemaFetcher -> fetchIndexes -> SHOW INDEX found %d indexes for table %s",
			len(showIndexResults), table)

		// Process SHOW INDEX results
		indexColumns := make(map[string][]string)
		indexUnique := make(map[string]bool)

		for _, idx := range showIndexResults {
			indexName, _ := idx["Key_name"].(string)
			columnName, _ := idx["Column_name"].(string)
			nonUnique, _ := idx["Non_unique"].(int64)

			if indexName != "" && columnName != "" {
				indexColumns[indexName] = append(indexColumns[indexName], columnName)
				indexUnique[indexName] = nonUnique == 0
				log.Printf("MySQLSchemaFetcher -> fetchIndexes -> Added index from SHOW INDEX: %s, Column: %s, IsUnique: %v",
					indexName, columnName, nonUnique == 0)
			}
		}

		// Create index info objects
		for name, columns := range indexColumns {
			indexes[name] = IndexInfo{
				Name:     name,
				Columns:  columns,
				IsUnique: indexUnique[name],
			}
		}

		return indexes, nil
	}

	// Group columns by index name
	indexColumns := make(map[string][]string)
	indexUnique := make(map[string]bool)
	for _, idx := range indexList {
		indexColumns[idx.Name] = append(indexColumns[idx.Name], idx.Column)
		indexUnique[idx.Name] = idx.IsUnique
	}

	// Create index info objects
	for name, columns := range indexColumns {
		indexes[name] = IndexInfo{
			Name:     name,
			Columns:  columns,
			IsUnique: indexUnique[name],
		}
	}
	return indexes, nil
}

// fetchForeignKeys retrieves all foreign keys for a specific table
func (f *MySQLSchemaFetcher) fetchForeignKeys(_ context.Context, table string) (map[string]ForeignKey, error) {
	fkeys := make(map[string]ForeignKey)
	var fkList []struct {
		Name       string `db:"constraint_name"`
		ColumnName string `db:"column_name"`
		RefTable   string `db:"referenced_table_name"`
		RefColumn  string `db:"referenced_column_name"`
		OnDelete   string `db:"delete_rule"`
		OnUpdate   string `db:"update_rule"`
	}

	query := `
        SELECT
            rc.constraint_name,
            kcu.column_name,
            kcu.referenced_table_name,
            kcu.referenced_column_name,
            rc.delete_rule,
            rc.update_rule
        FROM information_schema.referential_constraints rc
        JOIN information_schema.key_column_usage kcu
            ON kcu.constraint_name = rc.constraint_name
            AND kcu.constraint_schema = rc.constraint_schema
        WHERE rc.constraint_schema = DATABASE()
        AND kcu.table_name = ?;
    `
	log.Printf("MySQLSchemaFetcher -> fetchForeignKeys -> Executing query for table %s: %s", table, query)
	err := f.db.Query(query, &fkList, table)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> fetchForeignKeys -> Error for table %s: %v", table, err)
		// Return empty foreign keys rather than failing
		log.Printf("MySQLSchemaFetcher -> fetchForeignKeys -> Returning empty foreign keys for table %s", table)
		return fkeys, nil
	}

	log.Printf("MySQLSchemaFetcher -> fetchForeignKeys -> Found %d foreign keys for table %s", len(fkList), table)
	for _, fk := range fkList {
		log.Printf("MySQLSchemaFetcher -> fetchForeignKeys -> Foreign key: %s, Column: %s, RefTable: %s, RefColumn: %s",
			fk.Name, fk.ColumnName, fk.RefTable, fk.RefColumn)
		fkeys[fk.Name] = ForeignKey{
			Name:       fk.Name,
			ColumnName: fk.ColumnName,
			RefTable:   fk.RefTable,
			RefColumn:  fk.RefColumn,
			OnDelete:   fk.OnDelete,
			OnUpdate:   fk.OnUpdate,
		}
	}
	return fkeys, nil
}

// fetchViews retrieves all views in the database
func (f *MySQLSchemaFetcher) fetchViews(_ context.Context) (map[string]ViewSchema, error) {
	views := make(map[string]ViewSchema)
	var viewList []struct {
		Name       string `db:"table_name"`
		Definition string `db:"view_definition"`
	}

	query := `
        SELECT 
            table_name,
            view_definition
        FROM information_schema.views
        WHERE table_schema = DATABASE()
        ORDER BY table_name;
    `
	log.Printf("MySQLSchemaFetcher -> fetchViews -> Executing query: %s", query)
	err := f.db.Query(query, &viewList)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> fetchViews -> Error: %v", err)
		// Return empty views rather than failing
		log.Printf("MySQLSchemaFetcher -> fetchViews -> Returning empty views")
		return views, nil
	}

	log.Printf("MySQLSchemaFetcher -> fetchViews -> Found %d views", len(viewList))
	for _, view := range viewList {
		log.Printf("MySQLSchemaFetcher -> fetchViews -> Added view: %s", view.Name)
		views[view.Name] = ViewSchema{
			Name:       view.Name,
			Definition: view.Definition,
		}
	}
	return views, nil
}

// fetchConstraints retrieves all constraints for a specific table
func (f *MySQLSchemaFetcher) fetchConstraints(_ context.Context, table string) (map[string]ConstraintInfo, error) {
	constraints := make(map[string]ConstraintInfo)

	// Get primary key constraints
	var pkColumns []string
	pkQuery := `
        SELECT 
            column_name
        FROM information_schema.key_column_usage
        WHERE table_schema = DATABASE()
        AND table_name = ?
        AND constraint_name = 'PRIMARY'
        ORDER BY ordinal_position;
    `
	log.Printf("MySQLSchemaFetcher -> fetchConstraints -> Executing primary key query for table %s", table)
	err := f.db.Query(pkQuery, &pkColumns, table)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> fetchConstraints -> Primary key error for table %s: %v", table, err)
		// Continue without primary key rather than failing
	} else {
		log.Printf("MySQLSchemaFetcher -> fetchConstraints -> Found %d primary key columns for table %s",
			len(pkColumns), table)

		if len(pkColumns) > 0 {
			constraints["PRIMARY"] = ConstraintInfo{
				Name:    "PRIMARY",
				Type:    "PRIMARY KEY",
				Columns: pkColumns,
			}
		}
	}

	// Get unique constraints (excluding primary key)
	var uniqueList []struct {
		Name   string `db:"constraint_name"`
		Column string `db:"column_name"`
	}
	uniqueQuery := `
        SELECT 
            tc.constraint_name,
            kcu.column_name
        FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu
            ON kcu.constraint_name = tc.constraint_name
            AND kcu.constraint_schema = tc.constraint_schema
            AND kcu.table_name = tc.table_name
        WHERE tc.constraint_schema = DATABASE()
        AND tc.table_name = ?
        AND tc.constraint_type = 'UNIQUE'
        ORDER BY tc.constraint_name, kcu.ordinal_position;
    `
	log.Printf("MySQLSchemaFetcher -> fetchConstraints -> Executing unique constraints query for table %s", table)
	err = f.db.Query(uniqueQuery, &uniqueList, table)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> fetchConstraints -> Unique constraints error for table %s: %v", table, err)
		// Continue without unique constraints rather than failing
		return constraints, nil
	}

	log.Printf("MySQLSchemaFetcher -> fetchConstraints -> Found %d unique constraint entries for table %s",
		len(uniqueList), table)

	// Group columns by constraint name
	uniqueColumns := make(map[string][]string)
	for _, unique := range uniqueList {
		uniqueColumns[unique.Name] = append(uniqueColumns[unique.Name], unique.Column)
	}

	// Create constraint info objects for unique constraints
	for name, columns := range uniqueColumns {
		log.Printf("MySQLSchemaFetcher -> fetchConstraints -> Added unique constraint: %s with columns: %v",
			name, columns)
		constraints[name] = ConstraintInfo{
			Name:    name,
			Type:    "UNIQUE",
			Columns: columns,
		}
	}

	// MySQL doesn't have CHECK constraints in older versions, but we can add support for them here
	// for MySQL 8.0+ if needed

	return constraints, nil
}

// getTableRowCount gets the number of rows in a table
func (f *MySQLSchemaFetcher) getTableRowCount(_ context.Context, table string) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", table)
	log.Printf("MySQLSchemaFetcher -> getTableRowCount -> Executing query for table %s: %s", table, query)
	err := f.db.Query(query, &count)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> getTableRowCount -> Error for table %s: %v", table, err)
		// If error (e.g., table too large), use approximate count from information_schema
		approxQuery := `
            SELECT 
                table_rows
            FROM information_schema.tables
            WHERE table_schema = DATABASE()
            AND table_name = ?;
        `
		log.Printf("MySQLSchemaFetcher -> getTableRowCount -> Trying approximate count for table %s: %s", table, approxQuery)
		err = f.db.Query(approxQuery, &count, table)
		if err != nil {
			log.Printf("MySQLSchemaFetcher -> getTableRowCount -> Approximate count error for table %s: %v", table, err)
			log.Printf("MySQLSchemaFetcher -> getTableRowCount -> Returning 0 rows for table %s", table)
			// If both methods fail, just return 0 as the count to avoid breaking schema fetch
			return 0, nil
		}
	}
	log.Printf("MySQLSchemaFetcher -> getTableRowCount -> Found %d rows for table %s", count, table)
	return count, nil
}

// GetTableChecksum calculates a checksum for a table's structure
func (f *MySQLSchemaFetcher) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	// Get table definition
	var tableDefinition string
	query := `
        SELECT 
            CONCAT(
                'CREATE TABLE ', table_name, ' (\n',
                GROUP_CONCAT(
                    CONCAT(
                        '  ', column_name, ' ', column_type, 
                        IF(is_nullable = 'NO', ' NOT NULL', ''),
                        IF(column_default IS NOT NULL, CONCAT(' DEFAULT ', column_default), ''),
                        IF(extra != '', CONCAT(' ', extra), '')
                    )
                    ORDER BY ordinal_position
                    SEPARATOR ',\n'
                ),
                '\n);'
            ) as definition
        FROM information_schema.columns
        WHERE table_schema = DATABASE()
        AND table_name = ?
        GROUP BY table_name;
    `

	err := db.Query(query, &tableDefinition, table)
	if err != nil {
		return "", fmt.Errorf("failed to get table definition: %v", err)
	}

	// Get indexes
	var indexes []string
	indexQuery := `
        SELECT 
            CONCAT(
                IF(non_unique = 0, 'CREATE UNIQUE INDEX ', 'CREATE INDEX '),
                index_name, ' ON ', table_name, ' (',
                GROUP_CONCAT(
                    column_name
                    ORDER BY seq_in_index
                    SEPARATOR ', '
                ),
                ');'
            ) as index_definition
        FROM information_schema.statistics
        WHERE table_schema = DATABASE()
        AND table_name = ?
        AND index_name != 'PRIMARY'
        GROUP BY index_name;
    `

	err = db.Query(indexQuery, &indexes, table)
	if err != nil {
		return "", fmt.Errorf("failed to get indexes: %v", err)
	}

	// Get foreign keys
	var foreignKeys []string
	fkQuery := `
        SELECT 
            CONCAT(
                'ALTER TABLE ', table_name, ' ADD CONSTRAINT ', constraint_name,
                ' FOREIGN KEY (', column_name, ') REFERENCES ',
                referenced_table_name, ' (', referenced_column_name, ');'
            ) as fk_definition
        FROM information_schema.key_column_usage
        WHERE table_schema = DATABASE()
        AND table_name = ?
        AND referenced_table_name IS NOT NULL;
    `

	err = db.Query(fkQuery, &foreignKeys, table)
	if err != nil {
		return "", fmt.Errorf("failed to get foreign keys: %v", err)
	}

	// Combine all definitions
	fullDefinition := tableDefinition
	for _, idx := range indexes {
		fullDefinition += "\n" + idx
	}
	for _, fk := range foreignKeys {
		fullDefinition += "\n" + fk
	}

	// Calculate checksum
	return fmt.Sprintf("%x", md5.Sum([]byte(fullDefinition))), nil
}

// FetchExampleRecords retrieves sample records from a table
func (f *MySQLSchemaFetcher) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Context cancelled: %v", err)
		return nil, err
	}

	// Ensure limit is reasonable
	if limit <= 0 {
		limit = 3 // Default to 3 records
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
	}

	log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Fetching up to %d example records from table %s", limit, table)

	// Try to identify time-based or ID columns to order by for getting latest records
	var columns []struct {
		ColumnName string `db:"column_name"`
		DataType   string `db:"data_type"`
	}

	columnQuery := `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_schema = DATABASE() 
		AND table_name = ?
	`

	err := db.Query(columnQuery, &columns, table)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Error fetching columns for table %s: %v", table, err)
		// Fall back to simple LIMIT query if column metadata isn't available
		query := fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", table, limit)
		var records []map[string]interface{}
		err := db.QueryRows(query, &records)
		if err != nil {
			log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Error fetching records from table %s: %v", table, err)
			return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
		}
		return processRecords(records, table)
	}

	// Common column names that might indicate recency (in priority order)
	timeColumns := []string{"updated_at", "modified_at", "update_time", "updated", "modified", "modified_time", "last_update", "last_modified"}
	creationColumns := []string{"created_at", "creation_time", "create_time", "created", "creation_date", "insert_time", "timestamp"}
	idColumns := []string{"id", "uuid", "record_id", "key", "primary_key"}

	// First try timestamp columns with datetime/timestamp types
	var orderByColumn string

	// Check for time-based columns first (update timestamps take priority over creation timestamps)
	for _, colPattern := range timeColumns {
		for _, col := range columns {
			if strings.EqualFold(col.ColumnName, colPattern) &&
				(strings.Contains(strings.ToLower(col.DataType), "time") ||
					strings.Contains(strings.ToLower(col.DataType), "date")) {
				orderByColumn = col.ColumnName
				log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Using modification timestamp column: %s", orderByColumn)
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
				if strings.EqualFold(col.ColumnName, colPattern) &&
					(strings.Contains(strings.ToLower(col.DataType), "time") ||
						strings.Contains(strings.ToLower(col.DataType), "date")) {
					orderByColumn = col.ColumnName
					log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Using creation timestamp column: %s", orderByColumn)
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
				if strings.EqualFold(col.ColumnName, colPattern) &&
					(strings.Contains(strings.ToLower(col.DataType), "int") ||
						strings.Contains(strings.ToLower(col.DataType), "char")) {
					orderByColumn = col.ColumnName
					log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Using ID column: %s", orderByColumn)
					break
				}
			}
			if orderByColumn != "" {
				break
			}
		}
	}

	// Build the query based on whether we found a column to order by
	var query string
	if orderByColumn != "" {
		// Use backticks around identifiers to handle reserved words
		query = fmt.Sprintf("SELECT * FROM `%s` ORDER BY `%s` DESC LIMIT %d", table, orderByColumn, limit)
		log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Using ordered query: %s", query)
	} else {
		// Fallback to simple query without ORDER BY
		query = fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", table, limit)
		log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> No suitable ordering column found, using simple query: %s", query)
	}

	var records []map[string]interface{}
	err = db.QueryRows(query, &records)
	if err != nil {
		log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Error fetching records from table %s: %v", table, err)
		return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
	}

	return processRecords(records, table)
}

// Helper function to process the records
func processRecords(records []map[string]interface{}, table string) ([]map[string]interface{}, error) {
	// If no records found, return empty slice
	if len(records) == 0 {
		log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> No records found in table %s", table)
		return []map[string]interface{}{}, nil
	}

	log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Successfully fetched %d records from table %s", len(records), table)

	// Debug the raw records
	for i, record := range records {
		log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Raw record %d: %+v", i, record)
	}

	// Process records to ensure all values are properly formatted
	processedRecords := make([]map[string]interface{}, len(records))
	for i, record := range records {
		processedRecords[i] = make(map[string]interface{})
		for key, value := range record {
			// Handle nil values
			if value == nil {
				processedRecords[i][key] = nil
				continue
			}

			// Handle byte arrays (common in MySQL results)
			if byteVal, ok := value.([]byte); ok {
				// Try to convert to string
				processedRecords[i][key] = string(byteVal)
				log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Converted []byte to string for key %s: %s", key, string(byteVal))
			} else {
				// Keep the original value
				processedRecords[i][key] = value
				log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Kept original value for key %s: %v (type: %T)", key, value, value)
			}
		}
	}

	// Debug the processed records
	for i, record := range processedRecords {
		log.Printf("MySQLSchemaFetcher -> FetchExampleRecords -> Processed record %d: %+v", i, record)
	}

	return processedRecords, nil
}

// FetchTableList retrieves a list of all tables in the database
func (f *MySQLSchemaFetcher) FetchTableList(ctx context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = DATABASE() 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

// filterSchemaForSelectedTables filters the schema to only include elements related to the selected tables
func (f *MySQLSchemaFetcher) filterSchemaForSelectedTables(schema *SchemaInfo, selectedTables []string) *SchemaInfo {
	log.Printf("MySQLSchemaFetcher -> filterSchemaForSelectedTables -> Starting with %d tables in schema and %d selected tables",
		len(schema.Tables), len(selectedTables))

	// If no tables are selected or "ALL" is selected, return the full schema
	if len(selectedTables) == 0 || (len(selectedTables) == 1 && selectedTables[0] == "ALL") {
		log.Printf("MySQLSchemaFetcher -> filterSchemaForSelectedTables -> No filtering needed, returning full schema")
		return schema
	}

	// Create a map for quick lookup of selected tables
	selectedTablesMap := make(map[string]bool)
	for _, table := range selectedTables {
		selectedTablesMap[table] = true
		log.Printf("MySQLSchemaFetcher -> filterSchemaForSelectedTables -> Added table to selection: %s", table)
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
			log.Printf("MySQLSchemaFetcher -> filterSchemaForSelectedTables -> Including table: %s with %d columns",
				tableName, len(tableSchema.Columns))
			filteredSchema.Tables[tableName] = tableSchema
		}
	}

	// Calculate new checksum for filtered schema
	schemaData, _ := json.Marshal(filteredSchema.Tables)
	filteredSchema.Checksum = fmt.Sprintf("%x", md5.Sum(schemaData))

	log.Printf("MySQLSchemaFetcher -> filterSchemaForSelectedTables -> Filtered schema contains %d tables",
		len(filteredSchema.Tables))

	return filteredSchema
}
