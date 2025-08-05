package dbmanager

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PostgresSchemaFetcher implements schema fetching for PostgreSQL
type PostgresSchemaFetcher struct {
	db DBExecutor
}

func (f *PostgresSchemaFetcher) FetchSchema(ctx context.Context) (*SchemaInfo, error) {
	schema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		Sequences: make(map[string]SequenceSchema),
		Enums:     make(map[string]EnumSchema),
		UpdatedAt: time.Now(),
	}

	// Fetch tables
	tables, err := f.fetchTables(ctx)
	if err != nil {
		return nil, err
	}

	for _, table := range tables {
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
			return nil, err
		}
		tableSchema.Columns = columns

		// Fetch indexes
		indexes, err := f.fetchIndexes(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.Indexes = indexes

		// Fetch foreign keys
		fkeys, err := f.fetchForeignKeys(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.ForeignKeys = fkeys

		// Fetch constraints
		constraints, err := f.fetchConstraints(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.Constraints = constraints

		// Calculate table schema checksum
		tableData, _ := json.Marshal(tableSchema)
		tableSchema.Checksum = fmt.Sprintf("%x", md5.Sum(tableData))

		schema.Tables[table] = tableSchema
	}

	// Fetch views
	views, err := f.fetchViews(ctx)
	if err != nil {
		return nil, err
	}
	schema.Views = views

	// Fetch sequences
	sequences, err := f.fetchSequences(ctx)
	if err != nil {
		return nil, err
	}
	schema.Sequences = sequences

	// Fetch enums
	enums, err := f.fetchEnums(ctx)
	if err != nil {
		return nil, err
	}
	schema.Enums = enums

	// Calculate overall schema checksum
	schemaData, _ := json.Marshal(schema.Tables)
	schema.Checksum = fmt.Sprintf("%x", md5.Sum(schemaData))

	return schema, nil
}

func (f *PostgresSchemaFetcher) fetchTables(_ context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

func (f *PostgresSchemaFetcher) fetchColumns(_ context.Context, table string) (map[string]ColumnInfo, error) {
	columns := make(map[string]ColumnInfo)
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
            col_description((table_schema || '.' || table_name)::regclass::oid, ordinal_position) as column_comment
        FROM information_schema.columns c
        WHERE table_schema = 'public'
        AND table_name = $1
        ORDER BY ordinal_position;
    `
	err := f.db.Query(query, &columnList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch columns for table %s: %v", table, err)
	}

	for _, col := range columnList {
		columns[col.Name] = ColumnInfo{
			Name:         col.Name,
			Type:         col.Type,
			IsNullable:   col.IsNullable == "YES",
			DefaultValue: col.DefaultValue,
			Comment:      col.Comment,
		}
	}
	return columns, nil
}

func (f *PostgresSchemaFetcher) fetchIndexes(_ context.Context, table string) (map[string]IndexInfo, error) {
	indexes := make(map[string]IndexInfo)
	var indexList []struct {
		Name     string `db:"indexname"`
		Columns  string `db:"columns"`
		IsUnique bool   `db:"is_unique"`
	}

	query := `
        SELECT 
            i.relname as indexname,
            array_to_string(array_agg(a.attname), ',') as columns,
            idx.indisunique as is_unique
        FROM pg_index idx
        JOIN pg_class i ON i.oid = idx.indexrelid
        JOIN pg_class t ON t.oid = idx.indrelid
        JOIN pg_attribute a ON a.attrelid = t.oid
        WHERE t.relname = $1
        AND a.attnum = ANY(idx.indkey)
        GROUP BY i.relname, idx.indisunique;
    `
	err := f.db.Query(query, &indexList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch indexes for table %s: %v", table, err)
	}

	for _, idx := range indexList {
		indexes[idx.Name] = IndexInfo{
			Name:     idx.Name,
			Columns:  strings.Split(idx.Columns, ","),
			IsUnique: idx.IsUnique,
		}
	}
	return indexes, nil
}

func (f *PostgresSchemaFetcher) fetchForeignKeys(_ context.Context, table string) (map[string]ForeignKey, error) {
	fkeys := make(map[string]ForeignKey)
	var fkList []struct {
		Name       string `db:"constraint_name"`
		ColumnName string `db:"column_name"`
		RefTable   string `db:"ref_table"`
		RefColumn  string `db:"ref_column"`
		OnDelete   string `db:"on_delete"`
		OnUpdate   string `db:"on_update"`
	}

	query := `
        SELECT
            tc.constraint_name,
            kcu.column_name,
            ccu.table_name as ref_table,
            ccu.column_name as ref_column,
            rc.delete_rule as on_delete,
            rc.update_rule as on_update
        FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu
            ON tc.constraint_name = kcu.constraint_name
        JOIN information_schema.constraint_column_usage ccu
            ON ccu.constraint_name = tc.constraint_name
        JOIN information_schema.referential_constraints rc
            ON tc.constraint_name = rc.constraint_name
        WHERE tc.table_name = $1
        AND tc.constraint_type = 'FOREIGN KEY';
    `
	err := f.db.Query(query, &fkList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch foreign keys for table %s: %v", table, err)
	}

	for _, fk := range fkList {
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

func (f *PostgresSchemaFetcher) FetchTableList(ctx context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

// Add GetTableChecksum method to PostgresSchemaFetcher
func (f *PostgresSchemaFetcher) GetTableChecksum(ctx context.Context, table string) (string, error) {
	// Get table definition
	var tableDefinition string
	query := `
        SELECT 
            'CREATE TABLE ' || relname || E'\n(\n' ||
            array_to_string(
                array_agg(
                    '    ' || column_name || ' ' ||  type || ' ' ||
                    case when is_nullable = 'NO' then 'NOT NULL' else '' end ||
                    case when column_default is not null then ' DEFAULT ' || column_default else '' end
                ), E',\n'
            ) || E'\n);\n' as definition
        FROM (
            SELECT 
                c.relname, a.attname AS column_name,
                pg_catalog.format_type(a.atttypid, a.atttypmod) as type,
                (SELECT substring(pg_catalog.pg_get_expr(d.adbin, d.adrelid) for 128)
                FROM pg_catalog.pg_attrdef d
                WHERE d.adrelid = a.attrelid AND d.adnum = a.attnum AND a.atthasdef) as column_default,
                n.nspname as schema,
                c.relname as table_name,
                a.attnum as column_position,
                case when a.attnotnull then 'NO' else 'YES' end as is_nullable
            FROM pg_catalog.pg_class c
            JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
            JOIN pg_catalog.pg_attribute a ON c.oid = a.attrelid
            WHERE n.nspname = 'public'
            AND c.relname = $1
            AND a.attnum > 0
            AND NOT a.attisdropped
            ORDER BY a.attnum
        ) t
        GROUP BY relname;
    `

	err := f.db.Query(query, &tableDefinition, table)
	if err != nil {
		return "", fmt.Errorf("failed to get table definition: %v", err)
	}

	// Get indexes
	var indexes []string
	indexQuery := `
        SELECT indexdef
        FROM pg_indexes
        WHERE tablename = $1
        AND schemaname = 'public'
        ORDER BY indexname;
    `

	err = f.db.Query(indexQuery, &indexes, table)
	if err != nil {
		return "", fmt.Errorf("failed to get indexes: %v", err)
	}

	// Get foreign keys
	var foreignKeys []string
	fkQuery := `
        SELECT
            'ALTER TABLE ' || tc.table_name || ' ADD CONSTRAINT ' || tc.constraint_name ||
            ' FOREIGN KEY (' || kcu.column_name || ') REFERENCES ' ||
            ccu.table_name || ' (' || ccu.column_name || ');'
        FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu
            ON tc.constraint_name = kcu.constraint_name
        JOIN information_schema.constraint_column_usage ccu
            ON ccu.constraint_name = tc.constraint_name
        WHERE tc.table_name = $1
        AND tc.constraint_type = 'FOREIGN KEY';
    `

	err = f.db.Query(fkQuery, &foreignKeys, table)
	if err != nil {
		return "", fmt.Errorf("failed to get foreign keys: %v", err)
	}

	// Combine all definitions
	fullDefinition := tableDefinition
	for _, idx := range indexes {
		fullDefinition += idx + ";\n"
	}
	for _, fk := range foreignKeys {
		fullDefinition += fk + "\n"
	}

	// Calculate checksum
	return fmt.Sprintf("%x", md5.Sum([]byte(fullDefinition))), nil
}

// Add FetchExampleRecords method to PostgresSchemaFetcher
func (f *PostgresSchemaFetcher) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Ensure limit is reasonable
	if limit <= 0 {
		limit = 3 // Default to 3 records
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
	}

	// Try to identify time-based or ID columns to order by for getting latest records
	var columns []struct {
		ColumnName string `db:"column_name"`
		DataType   string `db:"data_type"`
	}

	columnQuery := `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_schema = 'public' 
		AND table_name = $1
	`

	err := db.Query(columnQuery, &columns, table)
	if err != nil {
		// Fall back to simple LIMIT query if column metadata isn't available
		query := fmt.Sprintf("SELECT * FROM %s LIMIT %d", table, limit)
		var records []map[string]interface{}
		err := db.QueryRows(query, &records)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
		}
		return records, nil
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
						strings.Contains(strings.ToLower(col.DataType), "char") ||
						strings.Contains(strings.ToLower(col.DataType), "uuid")) {
					orderByColumn = col.ColumnName
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
		// Use double quotes around identifiers to handle reserved words
		query = fmt.Sprintf(`SELECT * FROM "%s" ORDER BY "%s" DESC LIMIT %d`, table, orderByColumn, limit)
	} else {
		// Fallback to simple query without ORDER BY
		query = fmt.Sprintf(`SELECT * FROM "%s" LIMIT %d`, table, limit)
	}

	var records []map[string]interface{}
	err = db.QueryRows(query, &records)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
	}

	// If no records found, return empty slice
	if len(records) == 0 {
		return []map[string]interface{}{}, nil
	}

	return records, nil
}

// fetchViews retrieves all views in the database
func (f *PostgresSchemaFetcher) fetchViews(_ context.Context) (map[string]ViewSchema, error) {
	views := make(map[string]ViewSchema)
	var viewList []struct {
		Name       string `db:"view_name"`
		Definition string `db:"view_definition"`
	}

	query := `
        SELECT 
            table_name as view_name,
            view_definition
        FROM information_schema.views
        WHERE table_schema = 'public'
        ORDER BY table_name;
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

// fetchConstraints retrieves all constraints for a specific table
func (f *PostgresSchemaFetcher) fetchConstraints(_ context.Context, table string) (map[string]ConstraintInfo, error) {
	constraints := make(map[string]ConstraintInfo)
	var constraintList []struct {
		Name       string `db:"constraint_name"`
		Type       string `db:"constraint_type"`
		Definition string `db:"definition"`
		Columns    string `db:"columns"`
	}

	query := `
        SELECT 
            tc.constraint_name,
            tc.constraint_type,
            pg_get_constraintdef(c.oid) as definition,
            array_to_string(array_agg(ccu.column_name), ',') as columns
        FROM information_schema.table_constraints tc
        JOIN pg_constraint c ON tc.constraint_name = c.conname
        JOIN information_schema.constraint_column_usage ccu 
            ON ccu.constraint_name = tc.constraint_name
        WHERE tc.table_name = $1
        AND tc.constraint_type != 'FOREIGN KEY'
        GROUP BY tc.constraint_name, tc.constraint_type, c.oid;
    `
	err := f.db.Query(query, &constraintList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch constraints for table %s: %v", table, err)
	}

	for _, constraint := range constraintList {
		constraints[constraint.Name] = ConstraintInfo{
			Name:       constraint.Name,
			Type:       constraint.Type,
			Definition: constraint.Definition,
			Columns:    strings.Split(constraint.Columns, ","),
		}
	}
	return constraints, nil
}

// fetchSequences retrieves all sequences in the database
func (f *PostgresSchemaFetcher) fetchSequences(_ context.Context) (map[string]SequenceSchema, error) {
	sequences := make(map[string]SequenceSchema)
	var sequenceList []struct {
		Name       string `db:"sequence_name"`
		StartValue int64  `db:"start_value"`
		Increment  int64  `db:"increment_by"`
		MinValue   int64  `db:"min_value"`
		MaxValue   int64  `db:"max_value"`
		CacheSize  int64  `db:"cache_size"`
		IsCycled   bool   `db:"is_cycled"`
	}

	query := `
        SELECT 
            sequence_name,
            start_value,
            increment_by,
            min_value,
            max_value,
            cache_size,
            cycle_option = 'YES' as is_cycled
        FROM information_schema.sequences
        WHERE sequence_schema = 'public'
        ORDER BY sequence_name;
    `
	err := f.db.Query(query, &sequenceList)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sequences: %v", err)
	}

	for _, seq := range sequenceList {
		sequences[seq.Name] = SequenceSchema{
			Name:       seq.Name,
			StartValue: seq.StartValue,
			Increment:  seq.Increment,
			MinValue:   seq.MinValue,
			MaxValue:   seq.MaxValue,
			CacheSize:  seq.CacheSize,
			IsCycled:   seq.IsCycled,
		}
	}
	return sequences, nil
}

// fetchEnums retrieves all enum types in the database
func (f *PostgresSchemaFetcher) fetchEnums(_ context.Context) (map[string]EnumSchema, error) {
	enums := make(map[string]EnumSchema)
	var enumList []struct {
		Name   string `db:"type_name"`
		Values string `db:"enum_values"`
		Schema string `db:"type_schema"`
	}

	query := `
        SELECT 
            t.typname as type_name,
            array_to_string(array_agg(e.enumlabel ORDER BY e.enumsortorder), ',') as enum_values,
            n.nspname as type_schema
        FROM pg_type t
        JOIN pg_enum e ON t.oid = e.enumtypid
        JOIN pg_namespace n ON n.oid = t.typnamespace
        WHERE n.nspname = 'public'
        GROUP BY t.typname, n.nspname
        ORDER BY t.typname;
    `
	err := f.db.Query(query, &enumList)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch enums: %v", err)
	}

	for _, enum := range enumList {
		enums[enum.Name] = EnumSchema{
			Name:   enum.Name,
			Values: strings.Split(enum.Values, ","),
			Schema: enum.Schema,
		}
	}
	return enums, nil
}

// filterSchemaForSelectedTables filters the schema to only include elements related to the selected tables
func (f *PostgresSchemaFetcher) filterSchemaForSelectedTables(schema *SchemaInfo, selectedTables []string) *SchemaInfo {
	// If no tables are selected or "ALL" is selected, return the full schema
	if len(selectedTables) == 0 || (len(selectedTables) == 1 && selectedTables[0] == "ALL") {
		return schema
	}

	// Create a map for quick lookup of selected tables
	selectedTablesMap := make(map[string]bool)
	for _, table := range selectedTables {
		selectedTablesMap[table] = true
	}

	// Create a new filtered schema
	filteredSchema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		Sequences: make(map[string]SequenceSchema),
		Enums:     make(map[string]EnumSchema),
		UpdatedAt: schema.UpdatedAt,
	}

	// Filter tables
	for tableName, tableSchema := range schema.Tables {
		if selectedTablesMap[tableName] {
			filteredSchema.Tables[tableName] = tableSchema
		}
	}

	// Collect all foreign key references to include related tables
	referencedTables := make(map[string]bool)
	for _, tableSchema := range filteredSchema.Tables {
		for _, fk := range tableSchema.ForeignKeys {
			referencedTables[fk.RefTable] = true
		}
	}

	// Add referenced tables that weren't in the original selection
	for refTable := range referencedTables {
		if !selectedTablesMap[refTable] {
			if tableSchema, ok := schema.Tables[refTable]; ok {
				filteredSchema.Tables[refTable] = tableSchema
			}
		}
	}

	// Filter views based on their definition referencing selected tables
	for viewName, viewSchema := range schema.Views {
		shouldInclude := false

		// Check if the view definition references any of the selected tables
		for tableName := range selectedTablesMap {
			if strings.Contains(strings.ToLower(viewSchema.Definition), strings.ToLower(tableName)) {
				shouldInclude = true
				break
			}
		}

		if shouldInclude {
			filteredSchema.Views[viewName] = viewSchema
		}
	}

	// Filter sequences based on their usage in selected tables
	// This is a simplistic approach - ideally we would check column defaults
	for seqName, seqSchema := range schema.Sequences {
		shouldInclude := false

		// Check if the sequence name matches a pattern like "tablename_columnname_seq"
		for tableName := range selectedTablesMap {
			if strings.HasPrefix(seqName, tableName+"_") {
				shouldInclude = true
				break
			}
		}

		if shouldInclude {
			filteredSchema.Sequences[seqName] = seqSchema
		}
	}

	// Filter enums based on their usage in selected tables
	for enumName, enumSchema := range schema.Enums {
		shouldInclude := false

		// Check if any column in selected tables uses this enum type
		for _, tableSchema := range filteredSchema.Tables {
			for _, column := range tableSchema.Columns {
				if column.Type == enumName {
					shouldInclude = true
					break
				}
			}
			if shouldInclude {
				break
			}
		}

		if shouldInclude {
			filteredSchema.Enums[enumName] = enumSchema
		}
	}

	// Recalculate checksum for the filtered schema
	schemaData, _ := json.Marshal(filteredSchema.Tables)
	filteredSchema.Checksum = fmt.Sprintf("%x", md5.Sum(schemaData))

	return filteredSchema
}

// GetSchema retrieves the schema for the selected tables
func (f *PostgresSchemaFetcher) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// Fetch the full schema
	schema, err := f.FetchSchema(ctx)
	if err != nil {
		return nil, err
	}

	// Filter the schema based on selected tables
	return f.filterSchemaForSelectedTables(schema, selectedTables), nil
}
