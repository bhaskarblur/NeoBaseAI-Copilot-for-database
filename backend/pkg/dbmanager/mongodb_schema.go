package dbmanager

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// ColumnDiff represents a difference in a column between two schemas
type ColumnDiff struct {
	Name    string
	OldType string
	NewType string
}

// MongoDBSchemaFetcher implements SchemaFetcher for MongoDB
type MongoDBSchemaFetcher struct {
	db DBExecutor
}

// NewMongoDBSchemaFetcher creates a new MongoDB schema fetcher
func NewMongoDBSchemaFetcher(db DBExecutor) SchemaFetcher {
	return &MongoDBSchemaFetcher{
		db: db,
	}
}

// GetSchema fetches the MongoDB schema
func (f *MongoDBSchemaFetcher) GetSchema(ctx context.Context, db DBExecutor, selectedCollections []string) (*SchemaInfo, error) {
	log.Printf("MongoDBSchemaFetcher -> GetSchema -> Fetching MongoDB schema")

	executor, ok := db.(*MongoDBExecutor)
	if !ok {
		return nil, fmt.Errorf("invalid MongoDB executor")
	}

	// Get all collections
	collections, err := executor.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %v", err)
	}
	log.Printf("MongoDBSchemaFetcher -> GetSchema -> Found %d collections: %v", len(collections), collections)
	// Filter collections if specific ones are selected
	var targetCollections []string
	if len(selectedCollections) == 0 || (len(selectedCollections) == 1 && selectedCollections[0] == "ALL") {
		log.Printf("MongoDBSchemaFetcher -> GetSchema -> Selecting all collections")
		targetCollections = collections
	} else {
		// Filter to only include selected collections that exist
		log.Printf("MongoDBSchemaFetcher -> GetSchema %s -> Filtering collections", selectedCollections)
		for _, selected := range selectedCollections {
			for _, coll := range collections {
				if selected == coll {
					targetCollections = append(targetCollections, selected)
					break
				}
			}
		}
	}

	log.Printf("MongoDBSchemaFetcher -> GetSchema -> Creating MongoDB schema")
	// Create MongoDB schema
	mongoSchema := MongoDBSchema{
		Collections: make(map[string]MongoDBCollection),
		Indexes:     make(map[string][]MongoDBIndex),
		Version:     time.Now().Unix(),
		UpdatedAt:   time.Now(),
	}

	// Process each collection
	for _, collName := range targetCollections {
		// Sample documents from collection, 50 is the default sample size
		samples, err := executor.SampleCollection(ctx, collName, 50)
		if err != nil {
			log.Printf("MongoDBSchemaFetcher -> GetSchema -> Error sampling collection %s: %v", collName, err)
			continue
		}

		log.Printf("MongoDBSchemaFetcher -> GetSchema -> Sampling collection %s, found %d samples", collName, len(samples))
		if len(samples) > 0 {
			// Log the first sample to help with debugging
			log.Printf("MongoDBSchemaFetcher -> GetSchema -> First sample from collection %s: %+v", collName, samples[0])
		} else {
			log.Printf("MongoDBSchemaFetcher -> GetSchema -> No samples found in collection %s despite having documents", collName)
		}
		// Get document count
		stats, err := executor.GetCollectionStats(ctx, collName)
		if err != nil {
			log.Printf("MongoDBSchemaFetcher -> GetSchema -> Error getting stats for collection %s: %v", collName, err)
			continue
		}

		log.Printf("MongoDBSchemaFetcher -> GetSchema -> Stats for collection %s: %v", collName, stats)
		var documentCount int64
		if count, ok := stats["count"].(int32); ok {
			documentCount = int64(count)
		} else if count, ok := stats["count"].(int64); ok {
			documentCount = count
		}

		log.Printf("MongoDBSchemaFetcher -> GetSchema -> Creating collection schema for %s", collName)
		// Create collection schema
		collection := MongoDBCollection{
			Name:           collName,
			Fields:         make(map[string]MongoDBField),
			DocumentCount:  documentCount,
			SampleDocument: bson.M{},
		}

		log.Printf("MongoDBSchemaFetcher -> GetSchema -> Using first sample as sample document for %s", collName)
		// Use the first sample as the sample document if available
		if len(samples) > 0 {
			collection.SampleDocument = samples[0]
		}

		log.Printf("MongoDBSchemaFetcher -> GetSchema -> Analyzing fields from all samples for %s", collName)
		// Analyze fields from all samples (max nesting depth of 3 to keep schema manageable)
		fieldFrequency := make(map[string]int)
		for _, sample := range samples {
			f.analyzeDocumentWithDepth(sample, "", &collection.Fields, fieldFrequency, 0, 3)
		}

		// If collection is empty (no samples), add a default _id field
		// This ensures empty collections are still included in the schema
		if len(samples) == 0 {
			log.Printf("MongoDBSchemaFetcher -> GetSchema -> Collection %s is empty, adding default _id field", collName)
			collection.Fields["_id"] = MongoDBField{
				Name:       "_id",
				Type:       "ObjectId",
				IsRequired: true,
				Frequency:  1.0,
			}
		}

		log.Printf("MongoDBSchemaFetcher -> GetSchema -> Calculating field frequency and setting IsRequired for %s", collName)
		// Calculate field frequency and set IsRequired
		sampleCount := len(samples)
		if sampleCount > 0 {
			for fieldName, field := range collection.Fields {
				frequency := float64(fieldFrequency[fieldName]) / float64(sampleCount)
				field.Frequency = frequency
				field.IsRequired = frequency > 0.9 // Consider required if present in >90% of samples
				collection.Fields[fieldName] = field
			}
		}

		log.Printf("MongoDBSchemaFetcher -> GetSchema -> Getting indexes for %s", collName)
		// Get indexes
		indexes, err := f.getCollectionIndexes(ctx, executor, collName)
		if err != nil {
			log.Printf("MongoDBSchemaFetcher -> GetSchema -> Error getting indexes for collection %s: %v", collName, err)
		} else {
			collection.Indexes = indexes
			mongoSchema.Indexes[collName] = indexes
		}

		// Add collection to schema
		mongoSchema.Collections[collName] = collection
	}

	// Convert MongoDB schema to generic SchemaInfo
	schemaInfo := f.convertToSchemaInfo(mongoSchema)
	return schemaInfo, nil
}

// analyzeDocument recursively analyzes a document to extract field information (no depth limit — legacy)
func (f *MongoDBSchemaFetcher) analyzeDocument(doc bson.M, prefix string, fields *map[string]MongoDBField, fieldFrequency map[string]int) {
	f.analyzeDocumentWithDepth(doc, prefix, fields, fieldFrequency, 0, 100)
}

// analyzeDocumentWithDepth recursively analyzes a document with a max nesting depth.
// When maxDepth is reached, nested objects/arrays are recorded as "object" or "array" without
// expanding their sub-fields. This keeps MongoDB schemas manageable for deeply nested documents.
func (f *MongoDBSchemaFetcher) analyzeDocumentWithDepth(doc bson.M, prefix string, fields *map[string]MongoDBField, fieldFrequency map[string]int, currentDepth, maxDepth int) {
	for key, value := range doc {
		fieldName := key
		if prefix != "" {
			fieldName = prefix + "." + key
		}

		// Increment field frequency counter
		fieldFrequency[fieldName]++

		// Get or create field
		field, exists := (*fields)[key]
		if !exists {
			field = MongoDBField{
				Name:         key,
				Type:         f.getMongoDBFieldType(value),
				IsRequired:   false,
				IsArray:      false,
				NestedFields: make(map[string]MongoDBField),
				Frequency:    0,
			}
		}

		// Check if it's an array
		if arr, ok := value.(bson.A); ok {
			field.IsArray = true
			// If array has elements, analyze the first one to determine element type
			if len(arr) > 0 {
				field.Type = f.getMongoDBFieldType(arr[0])

				// If array element is a document, recurse only if within depth limit
				if doc, ok := arr[0].(bson.M); ok && currentDepth < maxDepth {
					f.analyzeDocumentWithDepth(doc, fieldName+"[]", &field.NestedFields, fieldFrequency, currentDepth+1, maxDepth)
				}
			}
		} else if nestedDoc, ok := value.(bson.M); ok {
			// Handle nested document — only recurse if within depth limit
			field.Type = "object"
			if currentDepth < maxDepth {
				f.analyzeDocumentWithDepth(nestedDoc, fieldName, &field.NestedFields, fieldFrequency, currentDepth+1, maxDepth)
			}
		}

		// Update field in map
		(*fields)[key] = field
	}
}

// getMongoDBFieldType determines the type of a MongoDB field
func (f *MongoDBSchemaFetcher) getMongoDBFieldType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case int, int32, int64:
		return "integer"
	case float32, float64:
		return "number"
	case bool:
		return "boolean"
	case bson.A:
		return "array"
	case bson.M, bson.D:
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// getCollectionIndexes retrieves indexes for a collection
func (f *MongoDBSchemaFetcher) getCollectionIndexes(ctx context.Context, executor *MongoDBExecutor, collName string) ([]MongoDBIndex, error) {
	// Execute listIndexes command
	command := bson.D{{Key: "listIndexes", Value: collName}}
	result, err := executor.ExecuteRawCommand(ctx, command)
	if err != nil {
		return nil, err
	}

	// Extract indexes from result
	var indexes []MongoDBIndex
	if cursor, ok := result["cursor"].(bson.M); ok {
		if firstBatch, ok := cursor["firstBatch"].(bson.A); ok {
			for _, indexDoc := range firstBatch {
				if idx, ok := indexDoc.(bson.M); ok {
					index := MongoDBIndex{
						Name:     idx["name"].(string),
						Keys:     bson.D{},
						IsUnique: false,
						IsSparse: false,
					}

					// Extract key information
					if key, ok := idx["key"].(bson.M); ok {
						for k, v := range key {
							index.Keys = append(index.Keys, bson.E{Key: k, Value: v})
						}
					}

					// Check if unique
					if unique, ok := idx["unique"].(bool); ok {
						index.IsUnique = unique
					}

					// Check if sparse
					if sparse, ok := idx["sparse"].(bool); ok {
						index.IsSparse = sparse
					}

					indexes = append(indexes, index)
				}
			}
		}
	}

	return indexes, nil
}

// convertToSchemaInfo converts MongoDB schema to generic SchemaInfo
func (f *MongoDBSchemaFetcher) convertToSchemaInfo(mongoSchema MongoDBSchema) *SchemaInfo {
	schema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: time.Now(),
	}

	// Convert collections to tables
	for collName, coll := range mongoSchema.Collections {
		tableSchema := TableSchema{
			Name:        collName,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
			Constraints: make(map[string]ConstraintInfo),
			RowCount:    coll.DocumentCount,
		}

		// Convert fields to columns
		for fieldName, field := range coll.Fields {
			columnType := field.Type
			if field.IsArray {
				columnType = "array<" + columnType + ">"
			}

			tableSchema.Columns[fieldName] = ColumnInfo{
				Name:         fieldName,
				Type:         columnType,
				IsNullable:   !field.IsRequired,
				DefaultValue: "",
				Comment:      fmt.Sprintf("Present in %.1f%% of documents", field.Frequency*100),
			}

			// Add nested fields as separate columns with dot notation
			f.addNestedFieldsAsColumns(field.NestedFields, fieldName, &tableSchema.Columns)
		}

		// Convert indexes
		for _, idx := range coll.Indexes {
			indexInfo := IndexInfo{
				Name:     idx.Name,
				IsUnique: idx.IsUnique,
				Columns:  []string{},
			}

			// Add columns to index
			for _, key := range idx.Keys {
				indexInfo.Columns = append(indexInfo.Columns, key.Key)
			}

			tableSchema.Indexes[idx.Name] = indexInfo
		}

		// Add table to schema
		schema.Tables[collName] = tableSchema
	}

	return schema
}

// addNestedFieldsAsColumns adds nested fields as columns with dot notation
func (f *MongoDBSchemaFetcher) addNestedFieldsAsColumns(nestedFields map[string]MongoDBField, prefix string, columns *map[string]ColumnInfo) {
	for fieldName, field := range nestedFields {
		fullName := prefix + "." + fieldName

		columnType := field.Type
		if field.IsArray {
			columnType = "array<" + columnType + ">"
		}

		(*columns)[fullName] = ColumnInfo{
			Name:         fullName,
			Type:         columnType,
			IsNullable:   !field.IsRequired,
			DefaultValue: "",
			Comment:      fmt.Sprintf("Present in %.1f%% of documents", field.Frequency*100),
		}

		// Recursively add nested fields
		if len(field.NestedFields) > 0 {
			f.addNestedFieldsAsColumns(field.NestedFields, fullName, columns)
		}
	}
}

// GetTableChecksum calculates a checksum for a MongoDB collection
func (f *MongoDBSchemaFetcher) GetTableChecksum(ctx context.Context, db DBExecutor, collection string) (string, error) {
	// Use the MongoDB driver to get the table checksum
	driver := &MongoDBDriver{}
	return driver.GetTableChecksum(ctx, db, collection)
}

// FetchExampleRecords fetches example records from a MongoDB collection
func (f *MongoDBSchemaFetcher) FetchExampleRecords(ctx context.Context, db DBExecutor, collection string, limit int) ([]map[string]interface{}, error) {
	// Use the MongoDB driver to fetch example records
	driver := &MongoDBDriver{}
	return driver.FetchExampleRecords(ctx, db, collection, limit)
}

// CompareSchemas compares two MongoDB schemas and returns the differences
func (f *MongoDBSchemaFetcher) CompareSchemas(oldSchema, newSchema *SchemaInfo) *SchemaDiff {
	diff := &SchemaDiff{
		AddedTables:    make([]string, 0),
		RemovedTables:  make([]string, 0),
		ModifiedTables: make(map[string]TableDiff),
	}

	// Check for added/removed collections
	for tableName := range newSchema.Tables {
		if _, exists := oldSchema.Tables[tableName]; !exists {
			diff.AddedTables = append(diff.AddedTables, tableName)
		}
	}

	for tableName := range oldSchema.Tables {
		if _, exists := newSchema.Tables[tableName]; !exists {
			diff.RemovedTables = append(diff.RemovedTables, tableName)
		}
	}

	// Check for modified collections
	for tableName, newTable := range newSchema.Tables {
		oldTable, exists := oldSchema.Tables[tableName]
		if !exists {
			continue // Already handled as added table
		}

		tableDiff := TableDiff{
			AddedColumns:    make([]string, 0),
			RemovedColumns:  make([]string, 0),
			ModifiedColumns: make([]string, 0),
			AddedIndexes:    make([]string, 0),
			RemovedIndexes:  make([]string, 0),
		}

		// Check for added/removed/modified columns
		for colName, newCol := range newTable.Columns {
			if oldCol, exists := oldTable.Columns[colName]; !exists {
				tableDiff.AddedColumns = append(tableDiff.AddedColumns, colName)
			} else if oldCol.Type != newCol.Type || oldCol.IsNullable != newCol.IsNullable {
				// Add column name to modified columns
				tableDiff.ModifiedColumns = append(tableDiff.ModifiedColumns, colName)
				log.Printf("Column %s modified: old type %s, new type %s", colName, oldCol.Type, newCol.Type)
			}
		}

		for colName := range oldTable.Columns {
			if _, exists := newTable.Columns[colName]; !exists {
				tableDiff.RemovedColumns = append(tableDiff.RemovedColumns, colName)
			}
		}

		// Check for added/removed indexes
		for idxName := range newTable.Indexes {
			if _, exists := oldTable.Indexes[idxName]; !exists {
				tableDiff.AddedIndexes = append(tableDiff.AddedIndexes, idxName)
			}
		}

		for idxName := range oldTable.Indexes {
			if _, exists := newTable.Indexes[idxName]; !exists {
				tableDiff.RemovedIndexes = append(tableDiff.RemovedIndexes, idxName)
			}
		}

		// Add table diff if there are changes
		if len(tableDiff.AddedColumns) > 0 || len(tableDiff.RemovedColumns) > 0 ||
			len(tableDiff.ModifiedColumns) > 0 || len(tableDiff.AddedIndexes) > 0 ||
			len(tableDiff.RemovedIndexes) > 0 {
			diff.ModifiedTables[tableName] = tableDiff
		}
	}

	return diff
}
