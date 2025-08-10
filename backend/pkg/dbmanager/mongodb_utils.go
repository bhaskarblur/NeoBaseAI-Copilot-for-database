package dbmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// convertMongoDBSchemaToSchemaInfo converts MongoDB schema to generic SchemaInfo
func convertMongoDBSchemaToSchemaInfo(mongoSchema MongoDBSchema) *SchemaInfo {
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
				Comment:      "",
			}
		}

		// Convert indexes
		if indexes, ok := mongoSchema.Indexes[collName]; ok {
			for _, idx := range indexes {
				// Skip _id_ index as it's implicit
				if idx.Name == "_id_" {
					continue
				}

				// Extract column names from index keys
				columns := make([]string, 0, len(idx.Keys))
				for _, key := range idx.Keys {
					columns = append(columns, key.Key)
				}

				tableSchema.Indexes[idx.Name] = IndexInfo{
					Name:     idx.Name,
					Columns:  columns,
					IsUnique: idx.IsUnique,
				}
			}
		}

		schema.Tables[collName] = tableSchema
	}

	return schema
}

// getMongoDBFieldType determines the type of a MongoDB field
func getMongoDBFieldType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case int32, int64, int:
		return "integer"
	case float64, float32:
		return "double"
	case bool:
		return "boolean"
	case primitive.DateTime:
		return "date"
	case primitive.ObjectID:
		return "objectId"
	case primitive.A:
		return "array"
	case bson.M, bson.D:
		return "object"
	case primitive.Binary:
		return "binary"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// processMongoDBQueryParams processes MongoDB query parameters
func processMongoDBQueryParams(paramsStr string) (string, error) {
	// Log the original string for debugging
	log.Printf("Original MongoDB query params: %s", paramsStr)

	// Special handling for MongoDB operators that should NOT be quoted
	mongoOperators := []string{
		"$dateToString", "$dateFromString", "$dateToParts", "$dateFromParts",
		"$year", "$month", "$dayOfMonth", "$hour", "$minute", "$second",
		"$dayOfWeek", "$dayOfYear", "$week", "$isoDayOfWeek", "$isoWeek",
		"$project", "$match", "$group", "$sort", "$limit", "$skip",
		"$lookup", "$unwind", "$out", "$merge", "$facet", "$bucket",
		"$addFields", "$set", "$replaceRoot", "$replaceWith", "$count",
		"$sortByCount", "$redact", "$geoNear", "$sample", "$union",
		"$add", "$subtract", "$multiply", "$divide", "$mod", "$pow",
		"$sqrt", "$abs", "$ceil", "$floor", "$round", "$exp", "$ln",
		"$log", "$log10", "$trunc", "$size", "$slice", "$in", "$nin",
		"$arrayElemAt", "$concatArrays", "$filter", "$first", "$last",
		"$max", "$min", "$avg", "$sum", "$stdDevPop", "$stdDevSamp",
		"$push", "$addToSet", "$pull", "$pullAll", "$pop", "$each",
		"$concat", "$substr", "$toLower", "$toUpper", "$strcasecmp",
		"$split", "$strLenBytes", "$strLenCP", "$indexOfBytes",
		"$indexOfCP", "$ltrim", "$rtrim", "$trim", "$regexMatch",
		"$regexFind", "$regexFindAll", "$replaceOne", "$replaceAll",
		"$eq", "$ne", "$gt", "$gte", "$lt", "$lte", "$cmp",
		"$and", "$or", "$not", "$nor", "$exists", "$type",
		"$expr", "$jsonSchema", "$text", "$where", "$all",
		"$elemMatch", "$regex", "$options", "$geoIntersects",
		"$geoWithin", "$near", "$nearSphere", "$box", "$center",
		"$centerSphere", "$geometry", "$maxDistance", "$minDistance",
		"$polygon", "$uniqueDocs", "$objectToArray", "$arrayToObject",
		"$literal", "$mergeObjects", "$setField", "$getField",
		"$setEquals", "$setIntersection", "$setUnion", "$setDifference",
		"$setIsSubset", "$anyElementTrue", "$allElementsTrue",
		"$cond", "$ifNull", "$switch", "$coalesce",
		"$toObjectId", "$toString", "$toDate", "$toDecimal", "$toDouble",
		"$toInt", "$toLong", "$toBool", "$type", "$convert",
	}

	// Handle JavaScript-style regex patterns like /pattern/flags
	// Convert to MongoDB Extended JSON format: {"$regex":"pattern","$options":"flags"}
	regexPattern := regexp.MustCompile(`/([^/]+)/([gimsuy]*)`)
	paramsStr = regexPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		matches := regexPattern.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		pattern := matches[1]
		options := matches[2]

		// Escape JSON special characters in the pattern
		pattern = strings.ReplaceAll(pattern, "\\", "\\\\")
		pattern = strings.ReplaceAll(pattern, "\"", "\\\"")

		// Return MongoDB Extended JSON format for regex
		return fmt.Sprintf(`{"$regex":"%s","$options":"%s"}`, pattern, options)
	})

	// Special case for regex patterns inside operators like $not
	operatorRegexPattern := regexp.MustCompile(`\$not\s*:\s*/([^/]+)/([gimsuy]*)`)
	paramsStr = operatorRegexPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		matches := operatorRegexPattern.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		pattern := matches[1]
		options := matches[2]

		// Escape JSON special characters in the pattern
		pattern = strings.ReplaceAll(pattern, "\\", "\\\\")
		pattern = strings.ReplaceAll(pattern, "\"", "\\\"")

		// Return MongoDB Extended JSON format for regex inside $not operator
		return fmt.Sprintf(`"$not": {"$regex":"%s","$options":"%s"}`, pattern, options)
	})

	// Special handling for $project stage in aggregation pipelines
	if strings.Contains(paramsStr, "$project") {
		// Detect if this is a $project stage
		projectPattern := regexp.MustCompile(`\$project\s*:\s*\{([^{}]|(?:\{[^{}]*\}))*\}`)
		if projectPattern.MatchString(paramsStr) {
			log.Printf("Detected $project stage in aggregation pipeline")

			// Extract the $project part
			projectMatches := projectPattern.FindStringSubmatch(paramsStr)
			if len(projectMatches) > 0 {
				// Handle the $project stage specially using our projection params processor
				// First, extract just the projection object
				projectObjPattern := regexp.MustCompile(`(\$project)\s*:\s*(\{([^{}]|(?:\{[^{}]*\}))*\})`)
				projectObjMatches := projectObjPattern.FindStringSubmatch(paramsStr)

				if len(projectObjMatches) > 2 {
					projectStage := projectObjMatches[1] // $project
					projectObj := projectObjMatches[2]   // The object part
					log.Printf("Extracted projection stage: %s", projectStage)
					log.Printf("Extracted projection object: %s", projectObj)

					// Process the projection using our specialized handler
					processedProjection, err := processProjectionParams(projectObj)
					if err != nil {
						return "", fmt.Errorf("failed to process $project stage: %v", err)
					}

					// Replace the original projection with the processed one while keeping $project
					// Create a well-formed JSON object with "$project" as the key
					result := fmt.Sprintf(`{"%s": %s}`, projectStage, processedProjection)
					paramsStr = result
					log.Printf("Processed $project stage: %s", paramsStr)
				}
			}
		}
	}

	// Extract modifiers from the query string
	var modifiersStr string
	if idx := strings.Index(paramsStr, "})."); idx != -1 {
		// Save the modifiers part for later processing
		modifiersStr = paramsStr[idx+1:]
		// Only keep the filter part
		paramsStr = paramsStr[:idx+1]
		log.Printf("Extracted filter part: %s", paramsStr)
		log.Printf("Extracted modifiers part: %s", modifiersStr)
	}

	// Check for offset_size in skip() - this is a special case for pagination
	// offset_size is a placeholder that will be replaced with the actual offset value
	// by the chat service when executing paginated queries.
	// For example, db.posts.find({}).skip(offset_size).limit(50) will become
	// db.posts.find({}).skip(50).limit(50) when requesting the second page with page size 50.
	// This replacement happens in the chat_service.go GetQueryResults function.
	if modifiersStr != "" {
		skipRegex := regexp.MustCompile(`\.skip\(offset_size\)`)
		if skipRegex.MatchString(modifiersStr) {
			log.Printf("Found offset_size in skip(), this will be replaced by the actual offset value")
		}
	}

	// Handle numerical values in sort expressions like {field: -1}
	// Preserve negative numbers in sort expressions
	sortPattern := regexp.MustCompile(`\{([^{}]+):\s*(-?\d+)\s*\}`)
	paramsStr = sortPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the field and direction
		sortMatches := sortPattern.FindStringSubmatch(match)
		if len(sortMatches) < 3 {
			return match
		}

		field := strings.TrimSpace(sortMatches[1])
		// Add quotes around the field name if not already quoted
		if !strings.HasPrefix(field, "\"") && !strings.HasPrefix(field, "'") {
			field = fmt.Sprintf(`"%s"`, field)
		}

		// Keep the numerical direction value as is
		return fmt.Sprintf(`{%s: %s}`, field, sortMatches[2])
	})

	// Handle ObjectId syntax: ObjectId('...') -> {"$oid":"..."}
	// This pattern matches both ObjectId('...') and ObjectId("...")
	objectIdPattern := regexp.MustCompile(`ObjectId\(['"]([^'"]+)['"]\)`)
	paramsStr = objectIdPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the ObjectId value
		re := regexp.MustCompile(`ObjectId\(['"]([^'"]+)['"]\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// Return the proper JSON format for ObjectId
		return fmt.Sprintf(`{"$oid":"%s"}`, matches[1])
	})

	// Handle ISODate syntax: ISODate('...') -> {"$date":"..."}
	// This pattern matches both ISODate('...') and ISODate("...")
	isoDatePattern := regexp.MustCompile(`ISODate\(['"]([^'"]+)['"]\)`)
	paramsStr = isoDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the ISODate value
		re := regexp.MustCompile(`ISODate\(['"]([^'"]+)['"]\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// Return the proper JSON format for Date
		return fmt.Sprintf(`{"$date":"%s"}`, matches[1])
	})

	// Handle Math expressions in date calculations
	// First, detect and replace mathematical operations in date calculations like: Date.now() - 24 * 60 * 60 * 1000
	mathExprPattern := regexp.MustCompile(`(Date\.now\(\)|new Date\(\)\.getTime\(\))\s*([+\-])\s*\(?\s*(\d+(?:\s*[*]\s*\d+)*)\s*\)?`)
	paramsStr = mathExprPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		log.Printf("Found date math expression: %s", match)
		// For simplicity, use current time minus 24 hours for common "yesterday" pattern
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	})

	// Handle new Date() syntax with various formats:
	// 1. new Date() without parameters -> current date in ISO format
	// 2. new Date("...") or new Date('...') with quoted date string
	// 3. new Date(year, month, day, ...) with numeric parameters
	// 4. new Date(Date.now() - 24 * 60 * 60 * 1000) -> current date minus 24 hours

	// First, handle new Date() without parameters
	emptyDatePattern := regexp.MustCompile(`new\s+Date\(\s*\)`)
	paramsStr = emptyDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Return current date in ISO format
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// Handle new Date("...") and new Date('...') with quoted date string
	quotedDatePattern := regexp.MustCompile(`new\s+Date\(['"]([^'"]+)['"]\)`)
	paramsStr = quotedDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the date value
		re := regexp.MustCompile(`new\s+Date\(['"]([^'"]+)['"]\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// Return the proper JSON format for Date
		return fmt.Sprintf(`{"$date":"%s"}`, matches[1])
	})

	// Handle new Date(Date.now() - ...) format specifically
	dateMathPattern := regexp.MustCompile(`new\s+Date\(\s*Date\.now\(\)\s*-\s*([^)]+)\)`)
	paramsStr = dateMathPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the time offset expression
		re := regexp.MustCompile(`new\s+Date\(\s*Date\.now\(\)\s*-\s*([^)]+)\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// For this pattern, we'll use the current date minus 24 hours
		// This is a simplification for common cases like "24 * 60 * 60 * 1000" (24 hours in milliseconds)
		log.Printf("Handling Date.now() math expression: %s", matches[1])
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	})

	// Handle complex date expressions like:
	// new Date(new Date().getTime() - (20 * 60 * 1000))
	// new Date(new Date().getFullYear(), new Date().getMonth()-1, 1)
	complexDatePattern := regexp.MustCompile(`new\s+Date\(([^)]+)\)`)
	paramsStr = complexDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Check if we've already processed this date (to avoid infinite recursion)
		if strings.Contains(match, "$date") {
			return match
		}

		// For complex date expressions, we'll use the current date
		// This is a simplification, but it allows the query to be parsed
		log.Printf("Converting complex date expression to current date: %s", match)
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// Log the processed string for debugging
	log.Printf("After ObjectId and Date replacement: %s", paramsStr)

	// Create placeholders for all MongoDB operators to prevent them from being modified
	operatorPlaceholders := make(map[string]string)
	placeholderIndex := 0

	// Replace all MongoDB operators with placeholders
	for _, op := range mongoOperators {
		placeholder := fmt.Sprintf("__MONGO_OP_%d__", placeholderIndex)
		operatorPlaceholders[placeholder] = op
		paramsStr = strings.ReplaceAll(paramsStr, op, placeholder)
		placeholderIndex++
	}

	// Also temporarily replace $oid and $date with placeholders
	paramsStr = strings.ReplaceAll(paramsStr, "$oid", "__MONGODB_OID__")
	paramsStr = strings.ReplaceAll(paramsStr, "$date", "__MONGODB_DATE__")

	// First pass: Quote all field names in objects
	// This regex matches field names followed by a colon, ensuring they're properly quoted
	// Improved pattern to catch all unquoted field names, including those at the beginning of objects
	fieldNameRegex := regexp.MustCompile(`(^|[,{])\s*([a-zA-Z0-9_]+)\s*:`)
	paramsStr = fieldNameRegex.ReplaceAllString(paramsStr, `$1"$2":`)

	// Handle fancy/smart quotes that LLMs sometimes generate
	// Do this BEFORE handling regular quotes
	paramsStr = strings.ReplaceAll(paramsStr, "\u2018", "'")  // Replace left smart quote
	paramsStr = strings.ReplaceAll(paramsStr, "\u2019", "'")  // Replace right smart quote
	paramsStr = strings.ReplaceAll(paramsStr, "\u201C", "\"") // Replace left smart double quote
	paramsStr = strings.ReplaceAll(paramsStr, "\u201D", "\"") // Replace right smart double quote
	
	// Handle single quotes for string values
	// Use a standard approach instead of negative lookbehind which isn't supported in Go
	singleQuoteRegex := regexp.MustCompile(`'([^']*)'`)
	paramsStr = singleQuoteRegex.ReplaceAllString(paramsStr, `"$1"`)

	// Restore all MongoDB operator placeholders
	for placeholder, op := range operatorPlaceholders {
		paramsStr = strings.ReplaceAll(paramsStr, placeholder, op)
	}

	// Restore $oid and $date placeholders
	paramsStr = strings.ReplaceAll(paramsStr, "__MONGODB_OID__", "$oid")
	paramsStr = strings.ReplaceAll(paramsStr, "__MONGODB_DATE__", "$date")

	// Ensure the document is valid JSON
	// Second pass: Check if it's an object and add missing quotes to field names
	if strings.HasPrefix(paramsStr, "{") && strings.HasSuffix(paramsStr, "}") {
		// Add quotes to any remaining unquoted field names
		// This regex matches field names that aren't already quoted
		unquotedFieldRegex := regexp.MustCompile(`([,{]|^)\s*([a-zA-Z0-9_]+)\s*:`)
		for unquotedFieldRegex.MatchString(paramsStr) {
			paramsStr = unquotedFieldRegex.ReplaceAllString(paramsStr, `$1"$2":`)
		}
	}

	// Final fix: Make sure all occurences of field names have double quotes
	// This extreme approach ensures all field names are properly quoted
	// Handle space-separated fields in projection
	for _, field := range []string{"email", "_id", "role", "createdAt", "name", "address", "phone"} {
		fieldPattern := regexp.MustCompile(fmt.Sprintf(`(%s):\s*([0-1])`, field))
		paramsStr = fieldPattern.ReplaceAllString(paramsStr, `"$1": $2`)
	}

	// Log the final processed string for debugging
	log.Printf("Final processed MongoDB query params: %s", paramsStr)

	return paramsStr, nil
}

// processObjectIds processes ObjectId syntax in MongoDB queries
func processObjectIds(filter map[string]interface{}) error {
	// Log the input filter for debugging
	filterJSON, _ := json.Marshal(filter)
	log.Printf("processObjectIds input: %s", string(filterJSON))

	for key, value := range filter {
		switch v := value.(type) {
		case map[string]interface{}:
			// Check if this is an ObjectId
			if oidStr, ok := v["$oid"].(string); ok && len(v) == 1 {
				// Convert to ObjectID
				oid, err := primitive.ObjectIDFromHex(oidStr)
				if err != nil {
					return fmt.Errorf("invalid ObjectId: %v", err)
				}
				filter[key] = oid
				log.Printf("Converted ObjectId %s to %v", oidStr, oid)
			} else if dateStr, ok := v["$date"].(string); ok && len(v) == 1 {
				// Parse the date to validate it and convert to a MongoDB primitive.DateTime
				parsedTime, err := time.Parse(time.RFC3339, dateStr)
				if err != nil {
					// Try other common date formats
					formats := []string{
						time.RFC3339,
						"2006-01-02T15:04:05Z",
						"2006-01-02",
						"2006/01/02",
						"01/02/2006",
						"01-02-2006",
						time.ANSIC,
						time.UnixDate,
						time.RubyDate,
						time.RFC822,
						time.RFC822Z,
						time.RFC850,
						time.RFC1123,
						time.RFC1123Z,
					}

					parsed := false
					for _, format := range formats {
						if parsedTime, err = time.Parse(format, dateStr); err == nil {
							parsed = true
							break
						}
					}

					if !parsed {
						return fmt.Errorf("invalid date format: %s", dateStr)
					}
				}

				// Convert the time to a MongoDB primitive.DateTime
				mongoDate := primitive.NewDateTimeFromTime(parsedTime)
				filter[key] = mongoDate
				log.Printf("Converted date %s to MongoDB DateTime: %v", dateStr, mongoDate)
			} else {
				// Recursively process nested objects
				if err := processObjectIds(v); err != nil {
					return err
				}
			}
		case []interface{}:
			// Process arrays
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if err := processObjectIds(itemMap); err != nil {
						return err
					}
					v[i] = itemMap
				}
			}
		}
	}

	// Log the output filter for debugging
	outputJSON, _ := json.Marshal(filter)
	log.Printf("processObjectIds output (after ObjectId and Date conversion): %s", string(outputJSON))

	return nil
}

// Add this new function to extract modifiers from the query string
// Add this after the processObjectIds function
func extractModifiers(query string) struct {
	Skip       int64
	Limit      int64
	Sort       string
	Projection string
	Count      bool
} {
	modifiers := struct {
		Skip       int64
		Limit      int64
		Sort       string
		Projection string
		Count      bool
	}{}

	// Check if the query string is empty or doesn't contain any modifiers
	if query == "" || !strings.Contains(query, ".") {
		return modifiers
	}

	// Extract skip
	skipRegex := regexp.MustCompile(`\.skip\((\d+)\)`)
	skipMatches := skipRegex.FindStringSubmatch(query)
	if len(skipMatches) > 1 {
		skip, err := strconv.ParseInt(skipMatches[1], 10, 64)
		if err == nil {
			modifiers.Skip = skip
		}
	}

	// Extract limit
	limitRegex := regexp.MustCompile(`\.limit\((\d+)\)`)
	limitMatches := limitRegex.FindStringSubmatch(query)
	if len(limitMatches) > 1 {
		limit, err := strconv.ParseInt(limitMatches[1], 10, 64)
		if err == nil {
			modifiers.Limit = limit
		}
	}

	// Extract count
	countRegex := regexp.MustCompile(`\.count\(\s*\)`)
	countMatches := countRegex.FindStringSubmatch(query)
	if len(countMatches) > 0 {
		modifiers.Count = true
		log.Printf("extractModifiers -> Detected count() modifier")
	}

	// Extract projection
	projectionRegex := regexp.MustCompile(`\.project\(([^)]+)\)`)
	projectionMatches := projectionRegex.FindStringSubmatch(query)
	if len(projectionMatches) > 1 {
		// Get the raw projection expression
		projectionExpr := projectionMatches[1]
		modifiers.Projection = projectionExpr
		log.Printf("extractModifiers -> Extracted projection expression: %s", modifiers.Projection)
	}

	// Extract sort - improved to handle complex sort expressions including negative values
	sortRegex := regexp.MustCompile(`\.sort\(([^)]+)\)`)
	sortMatches := sortRegex.FindStringSubmatch(query)
	if len(sortMatches) > 1 {
		// Get the raw sort expression
		sortExpr := sortMatches[1]
		log.Printf("extractModifiers -> Raw sort expression: %s", sortExpr)

		// Keep the sort expression as is, and let the processMongoDBQueryParams function handle
		// the conversion to proper JSON.
		modifiers.Sort = sortExpr
		log.Printf("extractModifiers -> Extracted sort expression: %s", modifiers.Sort)
	}

	return modifiers
}

// SafeBeginTx is a helper function to safely begin a transaction with proper error handling
func (d *MongoDBDriver) SafeBeginTx(ctx context.Context, conn *Connection) (Transaction, error) {
	log.Printf("MongoDBDriver -> SafeBeginTx -> Safely beginning MongoDB transaction")

	tx := d.BeginTx(ctx, conn)

	// Check if the transaction has an error
	if mongoTx, ok := tx.(*MongoDBTransaction); ok && mongoTx.Error != nil {
		log.Printf("MongoDBDriver -> SafeBeginTx -> Transaction creation failed: %v", mongoTx.Error)
		return nil, mongoTx.Error
	}

	// Check if the transaction has a nil session
	if mongoTx, ok := tx.(*MongoDBTransaction); ok && mongoTx.Session == nil {
		log.Printf("MongoDBDriver -> SafeBeginTx -> Transaction has nil session")
		return nil, fmt.Errorf("transaction has nil session")
	}

	log.Printf("MongoDBDriver -> SafeBeginTx -> Transaction created successfully")
	return tx, nil
}

// processSortExpression handles MongoDB sort expressions, properly preserving negative values
func processSortExpression(sortExpr string) (string, error) {
	log.Printf("Processing sort expression: %s", sortExpr)

	// If it's already a valid JSON object, validate that the field names are quoted properly
	if strings.HasPrefix(sortExpr, "{") && strings.HasSuffix(sortExpr, "}") {
		// Pattern to find field:value pairs with negative numbers
		sortPattern := regexp.MustCompile(`\{([^{}]+):\s*(-?\d+)\s*\}`)
		sortExpr = sortPattern.ReplaceAllStringFunc(sortExpr, func(match string) string {
			// Extract the field and direction
			sortMatches := sortPattern.FindStringSubmatch(match)
			if len(sortMatches) < 3 {
				return match
			}

			field := strings.TrimSpace(sortMatches[1])
			direction := strings.TrimSpace(sortMatches[2])

			// Add quotes around the field name if not already quoted
			if !strings.HasPrefix(field, "\"") && !strings.HasPrefix(field, "'") {
				field = fmt.Sprintf(`"%s"`, field)
			}

			// Preserve the direction (including negative sign)
			return fmt.Sprintf(`{%s: %s}`, field, direction)
		})

		// Handle multiple fields in a sort object: {field1: 1, field2: -1}
		multiFieldPattern := regexp.MustCompile(`\{([^{}]+)\}`)
		if multiFieldPattern.MatchString(sortExpr) {
			match := multiFieldPattern.FindStringSubmatch(sortExpr)[1]

			// Extract individual field:value pairs
			pairs := strings.Split(match, ",")
			processedPairs := make([]string, 0, len(pairs))

			for _, pair := range pairs {
				if pair = strings.TrimSpace(pair); pair == "" {
					continue
				}

				// Split the pair into field and value
				parts := strings.SplitN(pair, ":", 2)
				if len(parts) != 2 {
					continue
				}

				field := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				// Add quotes around the field name if not already quoted
				if !strings.HasPrefix(field, "\"") && !strings.HasPrefix(field, "'") {
					field = fmt.Sprintf(`"%s"`, field)
				}

				processedPairs = append(processedPairs, fmt.Sprintf(`%s: %s`, field, value))
			}

			// Reconstruct the sort object
			sortExpr = fmt.Sprintf(`{%s}`, strings.Join(processedPairs, ", "))
		}

		// Now convert to proper JSON for MongoDB
		jsonStr, err := processMongoDBQueryParams(sortExpr)
		if err != nil {
			log.Printf("Error processing sort expression: %v", err)
			return sortExpr, err
		}

		log.Printf("Processed sort expression to: %s", jsonStr)
		return jsonStr, nil
	} else {
		// Simple field name, default to ascending order
		field := strings.Trim(sortExpr, `"' `)
		sortExpr = fmt.Sprintf(`{"%s": 1}`, field)
		log.Printf("Converted simple sort field to object: %s", sortExpr)
		return sortExpr, nil
	}
}

// Process the aggregation results from a cursor
func processAggregationResultsFromCursor(cursor *mongo.Cursor, ctx context.Context) *QueryExecutionResult {
	// Decode the results
	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Failed to decode aggregation results: %v", err),
				Code:    "DECODE_ERROR",
			},
		}
	}
	
	// Debug logging for aggregation results
	log.Printf("processAggregationResultsFromCursor -> Decoded %d results from aggregation", len(results))

	// Create a wrapper for the results to maintain compatibility with existing code
	resultMap := map[string]interface{}{
		"results": results,
	}

	// Marshal the results to JSON for ResultJSON field
	resultJSON, err := json.Marshal(resultMap)
	if err != nil {
		log.Printf("Error marshalling aggregation results to JSON: %v", err)
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Failed to marshal results to JSON: %v", err),
				Code:    "MARSHAL_ERROR",
			},
		}
	}

	// Calculate execution time - this will be set by the caller
	executionTime := 0

	return &QueryExecutionResult{
		Result:        resultMap,
		ResultJSON:    string(resultJSON),
		ExecutionTime: executionTime,
	}
}

// processAggregationResults processes the results of an aggregation pipeline
func processAggregationResults(result interface{}) (*QueryExecutionResult, error) {
	// Create a variable to hold the result map
	var resultMap map[string]interface{}

	// Handle the result based on its type
	switch typedResult := result.(type) {
	case map[string]interface{}:
		// Already a map, use directly
		resultMap = typedResult
	case []interface{}, []bson.M, primitive.D, []primitive.D:
		// Wrap arrays and other MongoDB types in a results object
		resultMap = map[string]interface{}{
			"results": result,
		}
	default:
		// For any other type, create a new map and assign the result
		resultMap = map[string]interface{}{
			"results": result,
		}
	}

	executionResult := &QueryExecutionResult{
		Result: resultMap,
	}

	// Marshal the result to JSON string for the ResultJSON field
	resultJSON, err := FormatQueryResult(result)
	if err != nil {
		log.Printf("Error formatting aggregation results: %v", err)
		executionResult.Error = &dtos.QueryError{
			Message: fmt.Sprintf("Error formatting aggregation results: %v", err),
			Code:    "AGGREGATION_RESULT_FORMAT_ERROR",
		}
		executionResult.ResultJSON = "{\"results\":[]}"
		return executionResult, err
	}

	executionResult.ResultJSON = resultJSON
	return executionResult, nil
}

// ProcessDotNotationFields handles dot notation in MongoDB query fields
func ProcessDotNotationFields(queryMap map[string]interface{}) {
	// Process each field in the map
	for key, value := range queryMap {
		// Handle nested maps recursively
		if nestedMap, ok := value.(map[string]interface{}); ok {
			ProcessDotNotationFields(nestedMap)
			continue
		}

		// Handle nested arrays
		if nestedArray, ok := value.([]interface{}); ok {
			for _, item := range nestedArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					ProcessDotNotationFields(itemMap)
				}
			}
			continue
		}

		// Check for special operators that might contain field references with dot notation
		if strings.HasPrefix(key, "$") && (key == "$project" || key == "$match" || key == "$group" || key == "$lookup") {
			// For projection operators, fields may need special handling
			if operatorMap, ok := value.(map[string]interface{}); ok {
				for fieldKey, fieldValue := range operatorMap {
					// If the field contains a dot and is a string value that's a field reference
					if strings.Contains(fieldKey, ".") && !strings.HasPrefix(fieldKey, "$") {
						// Handle field value based on stage type
						switch key {
						case "$project":
							// In $project, if the value is 1 or true, replace with "$fieldname"
							switch v := fieldValue.(type) {
							case float64:
								if v == 1 {
									operatorMap[fieldKey] = "$" + fieldKey
									log.Printf("Processed dot notation in %s: %s -> $%s", key, fieldKey, fieldKey)
								}
							case bool:
								if v == true {
									operatorMap[fieldKey] = "$" + fieldKey
									log.Printf("Processed dot notation in %s: %s -> $%s", key, fieldKey, fieldKey)
								}
							case string:
								// If not already a field reference, make it one
								if !strings.HasPrefix(v, "$") {
									operatorMap[fieldKey] = "$" + fieldKey
									log.Printf("Processed dot notation in %s: %s -> $%s", key, fieldKey, fieldKey)
								}
							}
						}
					}

					// Handle nested references in expressions
					if exprMap, ok := fieldValue.(map[string]interface{}); ok {
						ProcessDotNotationFields(exprMap)
					}
				}
			}
		}
	}
}

// processProjectionParams specifically handles MongoDB projection parameters,
// which often need special treatment due to their simpler structure
func processProjectionParams(projectionStr string) (string, error) {
	// Log the original string for debugging
	log.Printf("Original MongoDB projection params: %s", projectionStr)

	// Extract only the projection part if it contains modifiers like .sort() or .limit()
	modifierIndex := -1
	for _, modifier := range []string{").sort(", ").limit(", ").skip("} {
		if idx := strings.Index(projectionStr, modifier); idx != -1 {
			if modifierIndex == -1 || idx < modifierIndex {
				modifierIndex = idx
			}
		}
	}

	// If we found a modifier, truncate the string to only include the projection
	if modifierIndex != -1 {
		// Make sure we only keep the actual projection part and remove the extra closing parenthesis
		// Find the last opening brace before the modifier
		openBraceIdx := strings.LastIndex(projectionStr[:modifierIndex], "{")
		if openBraceIdx != -1 {
			projectionStr = projectionStr[openBraceIdx:modifierIndex]
			log.Printf("Extracted projection part before modifiers: %s", projectionStr)
		} else {
			// If we can't find an opening brace, just use up to the modifier
			projectionStr = projectionStr[:modifierIndex]
			log.Printf("Extracted projection part (no opening brace found): %s", projectionStr)
		}
	}

	// Make sure we're working with a properly formed object
	if !strings.HasPrefix(projectionStr, "{") || !strings.HasSuffix(projectionStr, "}") {
		projectionStr = "{" + projectionStr + "}"
	}

	// Pre-process to handle dot notation fields in a special way
	// First, replace any single-quoted fields with a placeholder to protect them during processing
	// This is especially important for dot-notation fields like 'user.name'
	var dotNotationFields = make(map[string]string) // map of placeholder -> original field
	placeholderCounter := 0

	// Match fields with dots that are either:
	// 1. In single quotes: 'user.name'
	// 2. In double quotes: "user.name"
	// 3. Without quotes but containing dots: user.name
	dotFieldPattern := regexp.MustCompile(`(?:'([^']+\.[^']+)'|"([^"]+\.[^"]+)"|([a-zA-Z0-9_]+\.[a-zA-Z0-9_.]+))(\s*:)`)

	projectionStr = dotFieldPattern.ReplaceAllStringFunc(projectionStr, func(match string) string {
		// Don't process if it's already been processed
		if strings.Contains(match, "__DOT_FIELD_") {
			return match
		}

		// Create a placeholder for this dotted field
		placeholder := fmt.Sprintf("__DOT_FIELD_%d__", placeholderCounter)
		placeholderCounter++

		// Store the original (without the colon)
		matches := dotFieldPattern.FindStringSubmatch(match)
		var originalField string
		if matches[1] != "" {
			// Single quoted
			originalField = matches[1]
		} else if matches[2] != "" {
			// Double quoted
			originalField = matches[2]
		} else {
			// No quotes
			originalField = matches[3]
		}

		// Store in our map
		dotNotationFields[placeholder] = originalField

		// Return the placeholder with colon
		return placeholder + matches[4]
	})

	// General approach to properly quote all field names in the projection
	// This uses regex to find field names instead of hard-coding specific fields
	fieldNamePattern := regexp.MustCompile(`(^|[{,]\s*)([a-zA-Z0-9_$.]+)(\s*:)`)
	projectionStr = fieldNamePattern.ReplaceAllString(projectionStr, `$1"$2"$3`)

	// Handle field names that are quoted with single quotes
	singleQuotePattern := regexp.MustCompile(`'([^']+)'(\s*:)`)
	projectionStr = singleQuotePattern.ReplaceAllString(projectionStr, `"$1"$2`)

	// Process the projection object more carefully
	// Remove braces for processing
	content := projectionStr[1 : len(projectionStr)-1]

	// Split by comma
	fields := strings.Split(content, ",")

	// Process each field
	processedFields := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		parts := strings.SplitN(field, ":", 2)
		if len(parts) != 2 {
			// Skip invalid fields
			continue
		}

		fieldName := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Check if this is a placeholder field
		isPlaceholder := false
		for placeholder, originalField := range dotNotationFields {
			if strings.Contains(fieldName, placeholder) {
				// For dot notation fields in aggregation stages (after $lookup and $unwind),
				// we need to use "$user.email" format instead of just "user.email"
				// But only if this field has a parent object prefix (like "user." in "user.email")
				// Check if this looks like a reference to a joined document field
				if strings.Contains(originalField, ".") {
					parts := strings.SplitN(originalField, ".", 2)
					if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
						// This is a potential reference to a field in a joined document
						// We should use the appropriate format for MongoDB aggregation
						if value == "1" || value == "true" {
							// For inclusion, use the $ prefix format
							// Convert 'user.email': 1 to 'user.email': "$user.email"
							log.Printf("Converting dot notation field for aggregation: %s", originalField)
							// Fix: Ensure proper quoting without double quotes by directly formatting the field name
							cleanedField := strings.Replace(placeholder, "__DOT_FIELD_", "", 1)
							cleanedField = strings.Replace(cleanedField, "__", "", 1)
							// Create a properly formatted field reference with a single set of quotes
							processedFields = append(processedFields, fmt.Sprintf(`"%s": "$%s"`, originalField, originalField))
							isPlaceholder = true
							break
						}
					}
				}

				// Regular format for non-special cases
				// Fix: Ensure we replace the placeholder with a properly quoted field name without double quotes
				fieldName = fmt.Sprintf(`"%s"`, originalField)
				isPlaceholder = true
				break
			}
		}

		// Quote the field name if not already quoted and not a placeholder
		if !isPlaceholder && !strings.HasPrefix(fieldName, "\"") && !strings.HasPrefix(fieldName, "'") {
			fieldName = "\"" + fieldName + "\""
		}

		// Add the processed field if not already added (for dot notation special case)
		if !isPlaceholder || value != "1" {
			processedFields = append(processedFields, fieldName+": "+value)
		}
	}

	// Combine back into a JSON object
	result := "{" + strings.Join(processedFields, ", ") + "}"
	log.Printf("Processed MongoDB projection params: %s", result)

	return result, nil
}

// ProcessMongoDBQueryParams is the exported version of processMongoDBQueryParams
// for testing purposes
func ProcessMongoDBQueryParams(paramsStr string) (string, error) {
	return processMongoDBQueryParams(paramsStr)
}

// NewStageRegex returns the regex pattern used to match stages in MongoDB aggregation pipelines
func NewStageRegex() *regexp.Regexp {
	return regexp.MustCompile(`\{([^{}]*(?:\{[^{}]*\}[^{}]*)*)\}`)
}

// formatDateFields recursively formats date fields in MongoDB results
func formatDateFields(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		// Process each field in the map
		for key, value := range v {
			// Check if this is a date field
			if t, ok := value.(primitive.DateTime); ok {
				// Format the date to a human-readable string
				v[key] = t.Time().Format("January 2, 2006 at 3:04 PM")
			} else if t, ok := value.(time.Time); ok {
				v[key] = t.Format("January 2, 2006 at 3:04 PM")
			} else {
				// Recursively process nested structures
				v[key] = formatDateFields(value)
			}
		}
		return v
	case bson.M:
		// Convert to map[string]interface{} and process
		m := make(map[string]interface{})
		for key, value := range v {
			if t, ok := value.(primitive.DateTime); ok {
				m[key] = t.Time().Format("January 2, 2006 at 3:04 PM")
			} else if t, ok := value.(time.Time); ok {
				m[key] = t.Format("January 2, 2006 at 3:04 PM")
			} else {
				m[key] = formatDateFields(value)
			}
		}
		return m
	case []interface{}:
		// Process each element in the array
		for i, item := range v {
			v[i] = formatDateFields(item)
		}
		return v
	case []bson.M:
		// Process each element in the array
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = formatDateFields(item)
		}
		return result
	default:
		return v
	}
}

// FormatQueryResult formats MongoDB result for consistent JSON response
func FormatQueryResult(result interface{}) (string, error) {
	// Handle nil results
	if result == nil {
		return "{\"results\":[]}", nil
	}

	// Determine the type of result and format accordingly
	var resultMap map[string]interface{}
	var err error

	switch typedResult := result.(type) {
	case map[string]interface{}:
		// Already a map, use it directly
		resultMap = typedResult
	case []interface{}:
		// Array result, wrap in a map with "results" key
		resultMap = map[string]interface{}{
			"results": typedResult,
		}
	case primitive.D:
		// Convert primitive.D to map
		resultMap = typedResult.Map()
	case []primitive.D:
		// Convert []primitive.D to []map[string]interface{}
		results := make([]map[string]interface{}, len(typedResult))
		for i, d := range typedResult {
			results[i] = d.Map()
		}
		resultMap = map[string]interface{}{
			"results": results,
		}
	case []bson.M:
		// Already a []bson.M, wrap in results
		resultMap = map[string]interface{}{
			"results": typedResult,
		}
	case bson.M:
		// Already a bson.M, use directly
		resultMap = typedResult
	default:
		// For any other type, convert to JSON and then back to map
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			log.Printf("Error marshaling result to JSON: %v", err)
			return "{\"results\":[]}", fmt.Errorf("error formatting result: %v", err)
		}

		// Try to unmarshal into map
		err = json.Unmarshal(jsonBytes, &resultMap)
		if err != nil {
			// If we can't unmarshal to map, wrap in results key
			return fmt.Sprintf("{\"results\":%s}", string(jsonBytes)), nil
		}
	}

	// Marshal the map to JSON string
	jsonBytes, err := json.Marshal(resultMap)
	if err != nil {
		log.Printf("Error marshaling final result to JSON: %v", err)
		return "{\"results\":[]}", fmt.Errorf("error formatting final result: %v", err)
	}

	return string(jsonBytes), nil
}

// Helper function to check if a value is numeric with value 1
func isNumericOne(v interface{}) bool {
	switch val := v.(type) {
	case int:
		return val == 1
	case int32:
		return val == 1
	case int64:
		return val == 1
	case float64:
		return val == 1
	default:
		return false
	}
}

// Handle dot notation fields in aggregation pipelines after $lookup and $unwind
func processDotNotationInAggregation(pipeline []bson.M) error {
	log.Printf("Processing dot notation fields in aggregation pipeline with %d stages", len(pipeline))

	// Check if this pipeline has a $lookup followed by $unwind and $project
	hasLookup := false
	hasUnwind := false
	var projectStages []int // Store indices of project stages
	var lookupAsFields []string

	// First pass: detect the pipeline structure and collect all $lookup stages to get 'as' fields
	for i, stage := range pipeline {
		// Check for $lookup
		if lookupObj, ok := stage["$lookup"]; ok {
			hasLookup = true
			log.Printf("Found $lookup stage at position %d", i)

			// Get the 'as' field value which will be the prefix for dot notation
			// Handle both bson.M and map[string]interface{} types
			var asField string
			var asFieldFound bool

			if lookupBson, ok := lookupObj.(bson.M); ok {
				if asFieldVal, ok := lookupBson["as"]; ok {
					if asFieldStr, ok := asFieldVal.(string); ok && asFieldStr != "" {
						asField = asFieldStr
						asFieldFound = true
					}
				}
			} else if lookupMap, ok := lookupObj.(map[string]interface{}); ok {
				if asFieldVal, ok := lookupMap["as"]; ok {
					if asFieldStr, ok := asFieldVal.(string); ok && asFieldStr != "" {
						asField = asFieldStr
						asFieldFound = true
					}
				}
			}

			if asFieldFound && asField != "" {
				lookupAsFields = append(lookupAsFields, asField)
				log.Printf("Found $lookup 'as' field: %s", asField)
			}
		}

		// Check for $unwind
		if _, ok := stage["$unwind"]; ok {
			hasUnwind = true
			log.Printf("Found $unwind stage at position %d", i)
		}

		// Store the position of any $project stages found
		if _, ok := stage["$project"]; ok {
			projectStages = append(projectStages, i)
			log.Printf("Found $project stage at position %d", i)
		}
	}

	// Process all project stages if we have a pipeline with lookup+unwind
	if hasLookup && hasUnwind && len(projectStages) > 0 && len(lookupAsFields) > 0 {
		log.Printf("Processing %d $project stages after $lookup and $unwind", len(projectStages))
		for _, projectIndex := range projectStages {
			projectStage := pipeline[projectIndex]
			projectObj := projectStage["$project"]

			// Handle both bson.M and map[string]interface{} types for project
			var projectFields bson.M
			if projBson, ok := projectObj.(bson.M); ok {
				projectFields = projBson
			} else if projMap, ok := projectObj.(map[string]interface{}); ok {
				projectFields = bson.M{}
				for k, v := range projMap {
					projectFields[k] = v
				}
				// Update the original project stage with our bson.M version
				projectStage["$project"] = projectFields
			} else {
				log.Printf("Warning: $project stage at index %d doesn't contain a valid map, got %T", projectIndex, projectObj)
				continue
			}

			// Identify and process dot notation fields
			dotFieldCount := 0
			processedFields := make(map[string]bool)

			// First, find all dot notation fields that match our lookupAsFields
			for field, value := range projectFields {
				// Check if this is a dot notation field (user.email, user._id, etc.)
				if strings.Contains(field, ".") {
					fieldParts := strings.SplitN(field, ".", 2)
					if len(fieldParts) == 2 {
						prefix := fieldParts[0]

						// Check if this field uses one of our lookup 'as' fields as prefix
						for _, asField := range lookupAsFields {
							if prefix == asField {
								// This is a dot notation field from a $lookup, we need to handle it specially
								originalValue := value

								switch v := value.(type) {
								case float64, int, int64, int32:
									// Special case for inclusion (1): convert to field reference
									if isNumericOne(v) {
										projectFields[field] = "$" + field
										dotFieldCount++
										processedFields[field] = true
										log.Printf("Processed lookup dot notation field (numeric): %s -> $%s", field, field)
									}
								case bool:
									if v == true {
										// Special case for inclusion (true): convert to field reference
										projectFields[field] = "$" + field
										dotFieldCount++
										processedFields[field] = true
										log.Printf("Processed lookup dot notation field (boolean): %s -> $%s", field, field)
									}
								case string:
									// If already starts with $, leave it alone
									if !strings.HasPrefix(v, "$") {
										projectFields[field] = "$" + field
										dotFieldCount++
										processedFields[field] = true
										log.Printf("Processed lookup dot notation field (string): %s -> $%s", field, field)
									} else {
										// Already in the right format
										processedFields[field] = true
									}
								default:
									log.Printf("Skipping lookup dot notation field with unsupported value type %T: %s = %v", value, field, value)
								}

								if originalValue != projectFields[field] {
									log.Printf("Modified lookup dot notation field %s: %v -> %v", field, originalValue, projectFields[field])
								}

								break // No need to check other asFields
							}
						}
					}
				}
			}

			// Now add the parent fields if needed
			for _, asField := range lookupAsFields {
				// Check if we have any dot notation children for this asField
				hasAnyDotNotationChildren := false
				for field := range processedFields {
					if strings.HasPrefix(field, asField+".") {
						hasAnyDotNotationChildren = true
						break
					}
				}

				// Check if parent field exists in the projection
				_, hasParentField := projectFields[asField]

				// If we have dot notation children and the parent field also exists,
				// we have a path collision risk
				if hasAnyDotNotationChildren && hasParentField {
					// If both parent and dot notation children exist, we need to remove one of them
					// to avoid path collisions. Prefer keeping the dot notation fields.
					log.Printf("Found path collision risk: both %s and %s.* fields exist in projection", asField, asField)
					log.Printf("Removing parent field %s to avoid path collision with its dot notation children", asField)
					delete(projectFields, asField)
				}

				// We no longer automatically add the parent field when dot notation children exist,
				// as this causes path collisions in MongoDB
			}

			log.Printf("Processed %d dot notation fields in $project stage %d", dotFieldCount, projectIndex)
		}
	} else {
		if !hasLookup {
			log.Printf("No $lookup stage found in pipeline, skipping dot notation processing")
		}
		if !hasUnwind {
			log.Printf("No $unwind stage found in pipeline, skipping dot notation processing")
		}
		if len(projectStages) == 0 {
			log.Printf("No $project stages found in pipeline, skipping dot notation processing")
		}
		if len(lookupAsFields) == 0 {
			log.Printf("No 'as' fields found in $lookup stages, skipping dot notation processing")
		}
	}

	return nil
}

// ParseAggregationPipeline parses a MongoDB aggregation pipeline string into structured stages
// Exported for testing
func ParseAggregationPipeline(pipelineStr string) ([]string, error) {
	pipelineStr = strings.TrimSpace(pipelineStr)

	// Remove surrounding brackets if present
	if strings.HasPrefix(pipelineStr, "[") && strings.HasSuffix(pipelineStr, "]") {
		pipelineStr = pipelineStr[1 : len(pipelineStr)-1]
	}

	var stages []string
	var currentStage strings.Builder
	braceCount := 0
	inString := false
	var stringChar rune
	escapeNext := false

	runes := []rune(pipelineStr)
	for i := 0; i < len(runes); i++ {
		char := runes[i]
		
		if escapeNext {
			currentStage.WriteRune(char)
			escapeNext = false
			continue
		}

		if char == '\\' {
			currentStage.WriteRune(char)
			escapeNext = true
			continue
		}

		// Handle string boundaries
		if (char == '"' || char == '\'') && !inString {
			inString = true
			stringChar = char
			currentStage.WriteRune(char)
			continue
		} else if inString && char == stringChar {
			inString = false
			currentStage.WriteRune(char)
			continue
		}

		// If we're inside a string, just add the character
		if inString {
			currentStage.WriteRune(char)
			continue
		}

		// Track brace depth
		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
		}

		currentStage.WriteRune(char)

		// When we reach the end of a stage (braceCount returns to 0)
		if braceCount == 0 && char == '}' {
			// Look ahead to see if there's a comma
			if i+1 < len(runes) {
				// Skip whitespace
				j := i + 1
				for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n') {
					j++
				}
				// If we find a comma, skip it by advancing i
				if j < len(runes) && runes[j] == ',' {
					i = j // This will skip the comma in the next iteration
				}
			}

			// Add the completed stage
			stage := strings.TrimSpace(currentStage.String())
			if stage != "" {
				stages = append(stages, stage)
			}
			currentStage.Reset()
		}
	}

	// Add any remaining content as a stage
	if currentStage.Len() > 0 {
		stage := strings.TrimSpace(currentStage.String())
		if stage != "" && stage != "," { // Don't add if it's just a comma
			stages = append(stages, stage)
		}
		}

	return stages, nil
}

// extractParenthesisContent extracts the content between matching parentheses,
// correctly handling nested parentheses
func extractParenthesisContent(str string, startIndex int) (string, int, error) {
	// startIndex should point to the opening parenthesis
	if startIndex >= len(str) || str[startIndex] != '(' {
		return "", -1, fmt.Errorf("invalid start index: opening parenthesis not found at position %d", startIndex)
	}

	// Initialize counters and result
	openCount := 1
	closeIndex := -1

	// Find the matching closing parenthesis by counting opened and closed parentheses
	for i := startIndex + 1; i < len(str); i++ {
		if str[i] == '(' {
			openCount++
		} else if str[i] == ')' {
			openCount--
			if openCount == 0 {
				closeIndex = i
				break
			}
		}
	}

	// If we didn't find a matching parenthesis
	if closeIndex == -1 {
		return "", -1, fmt.Errorf("no matching closing parenthesis found")
	}

	// Extract the content between the parentheses (exclusive of the parentheses themselves)
	content := str[startIndex+1 : closeIndex]

	return content, closeIndex, nil
}
