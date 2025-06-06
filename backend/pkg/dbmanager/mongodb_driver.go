package dbmanager

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/utils"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"crypto/x509"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoDBDriver implements the DatabaseDriver interface for MongoDB
type MongoDBDriver struct{}

// NewMongoDBDriver creates a new MongoDB driver
func NewMongoDBDriver() DatabaseDriver {
	return &MongoDBDriver{}
}

// GetSchema retrieves the schema information for MongoDB
func (d *MongoDBDriver) GetSchema(ctx context.Context, db DBExecutor, selectedCollections []string) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MongoDBDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Get the MongoDB wrapper
	executor, ok := db.(*MongoDBExecutor)
	if !ok {
		return nil, fmt.Errorf("invalid MongoDB executor")
	}

	wrapper := executor.wrapper
	if wrapper == nil {
		return nil, fmt.Errorf("invalid MongoDB connection")
	}

	// Get all collections in the database
	var filter bson.M
	if len(selectedCollections) > 0 && selectedCollections[0] != "ALL" {
		filter = bson.M{"name": bson.M{"$in": selectedCollections}}
	}

	collections, err := wrapper.Client.Database(wrapper.Database).ListCollections(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %v", err)
	}
	defer collections.Close(ctx)

	// Create a map to store all collections
	mongoSchema := MongoDBSchema{
		Collections: make(map[string]MongoDBCollection),
		Indexes:     make(map[string][]MongoDBIndex),
	}

	// Process each collection
	for collections.Next(ctx) {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("MongoDBDriver -> GetSchema -> Context cancelled: %v", err)
			return nil, err
		}

		var collInfo bson.M
		if err := collections.Decode(&collInfo); err != nil {
			log.Printf("MongoDBDriver -> GetSchema -> Error decoding collection info: %v", err)
			continue
		}

		collName, ok := collInfo["name"].(string)
		if !ok {
			log.Printf("MongoDBDriver -> GetSchema -> Invalid collection name")
			continue
		}

		log.Printf("MongoDBDriver -> GetSchema -> Processing collection: %s", collName)

		// Get collection details
		collection, err := d.getCollectionDetails(ctx, wrapper, collName)
		if err != nil {
			log.Printf("MongoDBDriver -> GetSchema -> Error getting collection details: %v", err)
			continue
		}

		// Get indexes for the collection
		indexes, err := d.getCollectionIndexes(ctx, wrapper, collName)
		if err != nil {
			log.Printf("MongoDBDriver -> GetSchema -> Error getting collection indexes: %v", err)
			continue
		}

		// Add to schema
		mongoSchema.Collections[collName] = collection
		mongoSchema.Indexes[collName] = indexes
	}

	if err := collections.Err(); err != nil {
		return nil, fmt.Errorf("error iterating collections: %v", err)
	}

	// Convert to generic SchemaInfo
	return convertMongoDBSchemaToSchemaInfo(mongoSchema), nil
}

// getCollectionDetails retrieves details about a MongoDB collection
func (d *MongoDBDriver) getCollectionDetails(ctx context.Context, wrapper *MongoDBWrapper, collName string) (MongoDBCollection, error) {
	// Create a new collection
	collection := MongoDBCollection{
		Name:   collName,
		Fields: make(map[string]MongoDBField),
	}

	// Get document count
	count, err := wrapper.Client.Database(wrapper.Database).Collection(collName).CountDocuments(ctx, bson.M{})
	if err != nil {
		return collection, fmt.Errorf("failed to count documents: %v", err)
	}
	collection.DocumentCount = count

	// If collection is empty, return empty schema
	if count == 0 {
		return collection, nil
	}

	// Sample documents to infer schema
	sampleLimit := int64(50) // Sample up to 50 documents
	log.Printf("MongoDBDriver -> getCollectionDetails -> Will sample up to %d documents from collection %s for schema inference", sampleLimit, collName)

	opts := options.Find().SetLimit(sampleLimit)
	cursor, err := wrapper.Client.Database(wrapper.Database).Collection(collName).Find(ctx, bson.M{}, opts)
	if err != nil {
		return collection, fmt.Errorf("failed to sample documents: %v", err)
	}
	defer cursor.Close(ctx)

	// Process each document to infer schema
	var documents []bson.M
	if err := cursor.All(ctx, &documents); err != nil {
		return collection, fmt.Errorf("failed to decode documents: %v", err)
	}

	log.Printf("MongoDBDriver -> getCollectionDetails -> Retrieved exactly %d documents from collection %s for schema inference", len(documents), collName)

	// Store a sample document
	if len(documents) > 0 {
		collection.SampleDocument = documents[0]
	}

	// Infer schema from documents
	fields := make(map[string]MongoDBField)
	for _, doc := range documents {
		for key, value := range doc {
			field, exists := fields[key]
			if !exists {
				field = MongoDBField{
					Name:         key,
					IsRequired:   true,
					NestedFields: make(map[string]MongoDBField),
				}
			}

			// Determine field type
			fieldType := getMongoDBFieldType(value)
			if field.Type == "" {
				field.Type = fieldType
			} else if field.Type != fieldType && fieldType != "null" {
				// If types don't match, use a more generic type
				field.Type = "mixed"
			}

			// Check if it's an array
			if _, isArray := value.(primitive.A); isArray {
				field.IsArray = true
			}

			// Handle nested fields for objects
			if doc, isDoc := value.(bson.M); isDoc {
				for nestedKey, nestedValue := range doc {
					nestedField := MongoDBField{
						Name:       nestedKey,
						Type:       getMongoDBFieldType(nestedValue),
						IsRequired: true,
					}
					field.NestedFields[nestedKey] = nestedField
				}
			}

			fields[key] = field
		}
	}

	// Update required flag based on presence in all documents
	for _, doc := range documents {
		for key := range fields {
			if _, exists := doc[key]; !exists {
				field := fields[key]
				field.IsRequired = false
				fields[key] = field
			}
		}
	}

	collection.Fields = fields
	return collection, nil
}

// getCollectionIndexes retrieves indexes for a MongoDB collection
func (d *MongoDBDriver) getCollectionIndexes(ctx context.Context, wrapper *MongoDBWrapper, collName string) ([]MongoDBIndex, error) {
	// Get indexes
	cursor, err := wrapper.Client.Database(wrapper.Database).Collection(collName).Indexes().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %v", err)
	}
	defer cursor.Close(ctx)

	// Process each index
	var indexes []MongoDBIndex
	for cursor.Next(ctx) {
		var idx bson.M
		if err := cursor.Decode(&idx); err != nil {
			log.Printf("MongoDBDriver -> getCollectionIndexes -> Error decoding index: %v", err)
			continue
		}

		// Extract index information
		name, _ := idx["name"].(string)
		keys, _ := idx["key"].(bson.D)
		unique, _ := idx["unique"].(bool)
		sparse, _ := idx["sparse"].(bool)

		// Create index
		index := MongoDBIndex{
			Name:     name,
			Keys:     keys,
			IsUnique: unique,
			IsSparse: sparse,
		}

		indexes = append(indexes, index)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating indexes: %v", err)
	}

	return indexes, nil
}

// GetTableChecksum calculates a checksum for a MongoDB collection
func (d *MongoDBDriver) GetTableChecksum(ctx context.Context, db DBExecutor, collection string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MongoDBDriver -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	// Get the MongoDB wrapper
	executor, ok := db.(*MongoDBExecutor)
	if !ok {
		return "", fmt.Errorf("invalid MongoDB executor")
	}

	wrapper := executor.wrapper
	if wrapper == nil {
		return "", fmt.Errorf("invalid MongoDB connection")
	}

	// Get collection schema
	coll, err := d.getCollectionDetails(ctx, wrapper, collection)
	if err != nil {
		return "", fmt.Errorf("failed to get collection details: %v", err)
	}

	// Get collection indexes
	indexes, err := d.getCollectionIndexes(ctx, wrapper, collection)
	if err != nil {
		return "", fmt.Errorf("failed to get collection indexes: %v", err)
	}

	// Create a checksum from collection fields
	fieldsChecksum := ""
	for fieldName, field := range coll.Fields {
		fieldType := field.Type
		if field.IsArray {
			fieldType = "array<" + fieldType + ">"
		}
		fieldsChecksum += fmt.Sprintf("%s:%s:%v,", fieldName, fieldType, field.IsRequired)
	}

	// Create a checksum from indexes
	indexesChecksum := ""
	for _, idx := range indexes {
		// Skip _id_ index as it's implicit
		if idx.Name == "_id_" {
			continue
		}

		// Extract key information
		keyInfo := ""
		for _, key := range idx.Keys {
			keyInfo += fmt.Sprintf("%s:%v,", key.Key, key.Value)
		}

		indexesChecksum += fmt.Sprintf("%s:%v:%v,", keyInfo, idx.IsUnique, idx.IsSparse)
	}

	// Combine checksums
	finalChecksum := fmt.Sprintf("%s:%s", fieldsChecksum, indexesChecksum)
	return utils.MD5Hash(finalChecksum), nil
}

// FetchExampleRecords fetches example records from a MongoDB collection
func (d *MongoDBDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, collection string, limit int) ([]map[string]interface{}, error) {
	// Ensure limit is reasonable
	if limit <= 0 {
		limit = 3 // Default to 3 records
		log.Printf("MongoDBDriver -> FetchExampleRecords -> Using default limit of 3 records for collection %s", collection)
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
		log.Printf("MongoDBDriver -> FetchExampleRecords -> Capping limit to maximum of 10 records for collection %s", collection)
	} else {
		log.Printf("MongoDBDriver -> FetchExampleRecords -> Using requested limit of %d records for collection %s", limit, collection)
	}

	// Get the MongoDB wrapper
	executor, ok := db.(*MongoDBExecutor)
	if !ok {
		return nil, fmt.Errorf("invalid MongoDB executor")
	}

	wrapper := executor.wrapper
	if wrapper == nil {
		return nil, fmt.Errorf("invalid MongoDB connection")
	}

	// Fetch sample documents
	opts := options.Find().SetLimit(int64(limit))
	cursor, err := wrapper.Client.Database(wrapper.Database).Collection(collection).Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch example records: %v", err)
	}
	defer cursor.Close(ctx)

	// Process results
	var results []map[string]interface{}
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode document: %v", err)
		}

		// Convert BSON to map
		result := make(map[string]interface{})
		for k, v := range doc {
			// Convert MongoDB-specific types to JSON-friendly formats
			result[k] = convertMongoDBValue(v)
		}

		results = append(results, result)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating documents: %v", err)
	}

	// Log the exact number of records fetched
	log.Printf("MongoDBDriver -> FetchExampleRecords -> Retrieved exactly %d example records from collection %s", len(results), collection)

	// If no records found, return empty slice
	if len(results) == 0 {
		return []map[string]interface{}{}, nil
	}

	return results, nil
}

// Connect establishes a connection to a MongoDB database
func (d *MongoDBDriver) Connect(config ConnectionConfig) (*Connection, error) {
	var tempFiles []string
	log.Printf("MongoDBDriver -> Connect -> Connecting to MongoDB at %s:%v", config.Host, config.Port)

	var uri string
	port := "27017" // Default port for MongoDB

	// Check if we're using SRV records (mongodb+srv://)
	// Only check for .mongodb.net in non-encrypted hosts
	isSRV := false
	if !strings.Contains(config.Host, "+") && !strings.Contains(config.Host, "/") && !strings.Contains(config.Host, "=") {
		isSRV = strings.Contains(config.Host, ".mongodb.net")
	}

	protocol := "mongodb"
	if isSRV {
		protocol = "mongodb+srv"
	}

	// Validate port value if not using SRV
	if !isSRV && config.Port != nil {
		// Log the port value for debugging
		log.Printf("MongoDBDriver -> Connect -> Port value before validation: %v", *config.Port)

		// Check if port is empty
		if *config.Port == "" {
			log.Printf("MongoDBDriver -> Connect -> Port is empty, using default port 27017")
		} else {
			port = *config.Port

			// Skip port validation for encrypted ports (containing base64 characters)
			if strings.Contains(port, "+") || strings.Contains(port, "/") || strings.Contains(port, "=") {
				log.Printf("MongoDBDriver -> Connect -> Port appears to be encrypted, skipping validation")
			} else {
				// Verify port is numeric for non-encrypted ports
				if _, err := strconv.Atoi(port); err != nil {
					log.Printf("MongoDBDriver -> Connect -> Invalid port value: %v, error: %v", port, err)
					return nil, fmt.Errorf("invalid port value: %v, must be a number", port)
				}
			}
		}
	}

	// Base connection parameters with authentication
	if config.Username != nil && *config.Username != "" {
		// URL encode username and password to handle special characters
		encodedUsername := url.QueryEscape(*config.Username)
		var encodedPassword string
		if config.Password != nil {
			encodedPassword = url.QueryEscape(*config.Password)
		}

		if isSRV {
			// For SRV records, don't include port
			uri = fmt.Sprintf("%s://%s:%s@%s/%s",
				protocol, encodedUsername, encodedPassword, config.Host, config.Database)
		} else {
			// Include port for standard connections
			uri = fmt.Sprintf("%s://%s:%s@%s:%s/%s",
				protocol, encodedUsername, encodedPassword, config.Host, port, config.Database)
		}
	} else {
		// Without authentication
		if isSRV {
			// For SRV records, don't include port
			uri = fmt.Sprintf("%s://%s/%s", protocol, config.Host, config.Database)
		} else {
			// Include port for standard connections
			uri = fmt.Sprintf("%s://%s:%s/%s", protocol, config.Host, port, config.Database)
		}
	}

	// Add connection options
	if isSRV {
		uri += "?retryWrites=true&w=majority"
	} else {
		// For non-SRV connections, add a shorter server selection timeout
		uri += "?serverSelectionTimeoutMS=5000"
	}

	// Add authSource parameter if specified
	if config.AuthDatabase != nil && *config.AuthDatabase != "" {
		if strings.Contains(uri, "?") {
			uri += "&authSource=" + url.QueryEscape(*config.AuthDatabase)
		} else {
			uri += "?authSource=" + url.QueryEscape(*config.AuthDatabase)
		}
		log.Printf("MongoDBDriver -> Connect -> Using authentication database: %s", *config.AuthDatabase)
	}

	// Log the final URI (with sensitive parts masked)
	maskedUri := uri
	if config.Password != nil && *config.Password != "" {
		maskedUri = strings.Replace(maskedUri, *config.Password, "********", -1)
	}
	log.Printf("MongoDBDriver -> Connect -> Connection URI: %s", maskedUri)

	// Configure client options
	clientOptions := options.Client().ApplyURI(uri)

	// Set a shorter connection timeout for encrypted connections
	if strings.Contains(config.Host, "+") || strings.Contains(config.Host, "/") || strings.Contains(config.Host, "=") {
		clientOptions.SetConnectTimeout(5 * time.Second)
		clientOptions.SetServerSelectionTimeout(5 * time.Second)
		log.Printf("MongoDBDriver -> Connect -> Using shorter timeouts for encrypted connection")
	}

	// Explicitly disable SSL/TLS since the server doesn't support it
	// clientOptions.SetTLSConfig(nil)

	// Configure SSL/TLS only if explicitly enabled and server supports it
	if config.UseSSL {
		sslMode := "require"
		if config.SSLMode != nil {
			sslMode = *config.SSLMode
		}

		if sslMode == "disable" {
			// Do nothing
		} else {
			// Fetch certificates from URLs
			certPath, keyPath, rootCertPath, certTempFiles, err := utils.PrepareCertificatesFromURLs(*config.SSLCertURL, *config.SSLKeyURL, *config.SSLRootCertURL)
			if err != nil {
				return nil, err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Configure TLS
			tlsConfig := &tls.Config{
				InsecureSkipVerify: false, // Always verify certificates
			}

			// Add client certificates if provided
			if certPath != "" && keyPath != "" {
				cert, err := tls.LoadX509KeyPair(certPath, keyPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return nil, fmt.Errorf("failed to load client certificates: %v", err)
				}
				tlsConfig.Certificates = []tls.Certificate{cert}
			}

			// Add root CA if provided
			if rootCertPath != "" {
				rootCA, err := os.ReadFile(rootCertPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return nil, fmt.Errorf("failed to read root CA: %v", err)
				}

				rootCertPool := x509.NewCertPool()
				if ok := rootCertPool.AppendCertsFromPEM(rootCA); !ok {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return nil, fmt.Errorf("failed to parse root CA certificate")
				}

				tlsConfig.RootCAs = rootCertPool
			}

			clientOptions.SetTLSConfig(tlsConfig)
		}
	} else {
		// Disable SSL verification for encrypted connections
		// clientOptions.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
	}
	// Configure connection pool
	clientOptions.SetMaxPoolSize(25)
	clientOptions.SetMinPoolSize(5)
	clientOptions.SetMaxConnIdleTime(time.Hour)

	// Connect to MongoDB with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		log.Printf("MongoDBDriver -> Connect -> Error connecting to MongoDB: %v", err)
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Ping the database to verify connection
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		client.Disconnect(ctx)
		log.Printf("MongoDBDriver -> Connect -> Error pinging MongoDB: %v", err)
		return nil, fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	// Create a wrapper for the MongoDB client
	mongoWrapper := &MongoDBWrapper{
		Client:   client,
		Database: config.Database,
	}

	// Create a connection object
	conn := &Connection{
		DB:         nil, // MongoDB doesn't use GORM
		LastUsed:   time.Now(),
		Status:     StatusConnected,
		Config:     config,
		MongoDBObj: mongoWrapper, // Store MongoDB client in a custom field
		TempFiles:  tempFiles,    // Store temporary files for cleanup
		// Other fields will be set by the manager
	}

	log.Printf("MongoDBDriver -> Connect -> Successfully connected to MongoDB at %s:%v", config.Host, config.Port)
	return conn, nil
}

// Disconnect closes the MongoDB connection
func (d *MongoDBDriver) Disconnect(conn *Connection) error {
	log.Printf("MongoDBDriver -> Disconnect -> Disconnecting from MongoDB")

	// Get the MongoDB wrapper from the connection
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		return fmt.Errorf("invalid MongoDB connection")
	}

	// Disconnect from MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := wrapper.Client.Disconnect(ctx)
	if err != nil {
		log.Printf("MongoDBDriver -> Disconnect -> Error disconnecting from MongoDB: %v", err)
		return fmt.Errorf("failed to disconnect from MongoDB: %v", err)
	}

	// Clean up temporary certificate files
	for _, file := range conn.TempFiles {
		os.Remove(file)
	}

	log.Printf("MongoDBDriver -> Disconnect -> Successfully disconnected from MongoDB")
	return nil
}

// Ping checks if the MongoDB connection is alive
func (d *MongoDBDriver) Ping(conn *Connection) error {
	log.Printf("MongoDBDriver -> Ping -> Pinging MongoDB")

	// Get the MongoDB wrapper from the connection
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		return fmt.Errorf("invalid MongoDB connection")
	}

	// Ping MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := wrapper.Client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Printf("MongoDBDriver -> Ping -> Error pinging MongoDB: %v", err)
		return fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	log.Printf("MongoDBDriver -> Ping -> Successfully pinged MongoDB")
	return nil
}

// IsAlive checks if the MongoDB connection is alive
func (d *MongoDBDriver) IsAlive(conn *Connection) bool {
	err := d.Ping(conn)
	return err == nil
}

// ExecuteQuery executes a MongoDB query
func (d *MongoDBDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult {
	log.Printf("MongoDBDriver -> ExecuteQuery -> Executing MongoDB query: %s", query)

	startTime := time.Now()

	// Get the MongoDB wrapper from the connection
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Failed to get MongoDB wrapper from connection",
				Code:    "INTERNAL_ERROR",
			},
		}
	}

	// Handle special query format for MongoDB operations like db.getCollectionNames()
	if strings.HasPrefix(query, "db.") && !strings.Contains(query[3:], ".") {
		// Operations that are not tied to a specific collection
		operationWithParams := strings.TrimPrefix(query, "db.")
		openParenIndex := strings.Index(operationWithParams, "(")
		closeParenIndex := strings.LastIndex(operationWithParams, ")")

		if openParenIndex == -1 || closeParenIndex == -1 || closeParenIndex <= openParenIndex {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: "Invalid MongoDB query format. Expected: db.operation(...)",
					Code:    "INVALID_QUERY",
				},
			}
		}

		operation := operationWithParams[:openParenIndex]
		paramsStr := operationWithParams[openParenIndex+1 : closeParenIndex]

		log.Printf("MongoDBDriver -> ExecuteQuery -> Matched database operation: %s with params: %s", operation, paramsStr)

		switch operation {
		case "getCollectionNames":
			// List all collections in the database
			collections, err := wrapper.Client.Database(wrapper.Database).ListCollectionNames(ctx, bson.M{})
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to list collections: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			// Convert the result to a map for consistent output
			result := map[string]interface{}{
				"collections": collections,
			}

			// Convert the result to JSON
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to marshal result to JSON: %v", err),
						Code:    "JSON_ERROR",
					},
				}
			}

			executionTime := int(time.Since(startTime).Milliseconds())
			log.Printf("MongoDBDriver -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

			return &QueryExecutionResult{
				Result:        result,
				ResultJSON:    string(resultJSON),
				ExecutionTime: executionTime,
			}
		default:
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Unsupported database operation: %s", operation),
					Code:    "UNSUPPORTED_OPERATION",
				},
			}
		}
	}

	// Parse the query
	// Example: db.collection.find({name: "John"})
	parts := strings.SplitN(query, ".", 3)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "db") {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB query format. Expected: db.collection.operation({...}) or db.operation(...)",
				Code:    "INVALID_QUERY",
			},
		}
	}

	collectionName := parts[1]
	operationWithParams := parts[2]

	// Special case handling for empty find() with modifiers
	// Like db.collection.find().sort()
	if strings.HasPrefix(operationWithParams, "find()") && len(operationWithParams) > 6 {
		log.Printf("MongoDBDriver -> ExecuteQuery -> Detected empty find() with modifiers: %s", operationWithParams)
		// Replace find() with find({}) to ensure proper parsing
		operationWithParams = "find({})" + operationWithParams[6:]
		log.Printf("MongoDBDriver -> ExecuteQuery -> Reformatted query part: %s", operationWithParams)
	}

	// Split the operation and parameters
	// Example: find({name: "John"}) -> operation = find, params = {name: "John"}
	openParenIndex := strings.Index(operationWithParams, "(")
	if openParenIndex == -1 {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB query format. Expected: operation({...})",
				Code:    "INVALID_QUERY",
			},
		}
	}

	// Extract the operation and parameters using the helper function that handles nested parentheses
	operation := operationWithParams[:openParenIndex]
	paramsStr, closeParenIndex, extractErr := extractParenthesisContent(operationWithParams, openParenIndex)
	if extractErr != nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Invalid MongoDB query format: %v", extractErr),
				Code:    "INVALID_QUERY",
			},
		}
	}

	log.Printf("MongoDBDriver -> ExecuteQuery -> Extracted operation: %s, params: %s", operation, paramsStr)

	// Special case for find() with no parameters but with modifiers like .sort(), .limit()
	// For example: db.collection.find().sort({field: -1})
	if operation == "find" && strings.HasPrefix(paramsStr, ")") && strings.Contains(paramsStr, ".") {
		log.Printf("MongoDBDriver -> ExecuteQuery -> Detected find() with no parameters but with modifiers")

		// Extract modifiers from the parameters string
		modifiersStr := paramsStr

		// Set parameters to empty object
		paramsStr = "{}"

		log.Printf("MongoDBDriver -> ExecuteQuery -> Using empty object for parameters and parsing modifiers: %s", modifiersStr)
	}

	// Handle empty parameters case - if the parameters are empty, use an empty JSON object
	if strings.TrimSpace(paramsStr) == "" {
		paramsStr = "{}"
		log.Printf("MongoDBDriver -> ExecuteQuery -> Empty parameters detected, using empty object {}")
	}

	// Handle query modifiers like .limit(), .skip(), etc.
	modifiers := make(map[string]interface{})
	if closeParenIndex < len(operationWithParams)-1 {
		// There might be modifiers after the closing parenthesis
		modifiersStr := operationWithParams[closeParenIndex+1:]

		log.Printf("MongoDBDriver -> ExecuteQuery -> Modifiers string: %s", modifiersStr)

		// Extract limit modifier
		limitRegex := regexp.MustCompile(`\.limit\((\d+)\)`)
		if limitMatches := limitRegex.FindStringSubmatch(modifiersStr); len(limitMatches) > 1 {
			if limit, err := strconv.Atoi(limitMatches[1]); err == nil {
				modifiers["limit"] = limit
				log.Printf("MongoDBDriver -> ExecuteQuery -> Found limit modifier: %d", limit)
			}
		}

		// Extract skip modifier
		skipRegex := regexp.MustCompile(`\.skip\((\d+)\)`)
		if skipMatches := skipRegex.FindStringSubmatch(modifiersStr); len(skipMatches) > 1 {
			if skip, err := strconv.Atoi(skipMatches[1]); err == nil {
				modifiers["skip"] = skip
				log.Printf("MongoDBDriver -> ExecuteQuery -> Found skip modifier: %d", skip)
			}
		}

		// Extract sort modifier - improved to handle the entire sort expression
		sortRegex := regexp.MustCompile(`\.sort\(([^)]+)\)`)
		if sortMatches := sortRegex.FindStringSubmatch(modifiersStr); len(sortMatches) > 1 {
			sortExpr := sortMatches[1]

			// Process the sort expression using our dedicated function
			jsonStr, err := processSortExpression(sortExpr)
			if err == nil {
				modifiers["sort"] = jsonStr
				log.Printf("MongoDBDriver -> ExecuteQuery -> Processed sort modifier: %s", jsonStr)
			} else {
				log.Printf("MongoDBDriver -> ExecuteQuery -> Error processing sort modifier: %v", err)
				modifiers["sort"] = sortExpr
			}
		}
	}

	// Get the MongoDB collection
	collection := wrapper.Client.Database(wrapper.Database).Collection(collectionName)

	// Check if the collection exists (except for dropCollection operation)
	if operation != "dropCollection" {
		// Check if collection exists by listing collections with a filter
		collections, err := collection.Database().ListCollectionNames(ctx, bson.M{"name": collectionName})
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		if len(collections) == 0 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Collection '%s' does not exist", collectionName),
					Code:    "COLLECTION_NOT_FOUND",
				},
			}
		}
	}

	var result interface{}
	var err error

	log.Printf("MongoDBDriver -> ExecuteQuery -> operation: %s", operation)
	// Execute the operation based on the type
	switch operation {
	case "find":
		// Parse the parameters as a BSON filter and projection
		// The parameters can be in two formats:
		// 1. find({filter}) - just a filter
		// 2. find({filter}, {projection}) - filter and projection

		var filter bson.M
		var projection bson.M

		// Check if we have both filter and projection
		if strings.Contains(paramsStr, "}, {") {
			// Split the parameters into filter and projection
			parts := strings.SplitN(paramsStr, "}, {", 2)
			if len(parts) == 2 {
				filterStr := parts[0] + "}"
				projectionStr := "{" + parts[1]

				log.Printf("MongoDBDriver -> ExecuteQuery -> Split parameters into filter: %s and projection: %s", filterStr, projectionStr)

				// Parse the filter
				if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
					// Try to handle MongoDB syntax with unquoted keys
					jsonFilterStr, err := processMongoDBQueryParams(filterStr)
					if err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}

					if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to parse filter: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}

					// Handle ObjectId in the filter
					if err := processObjectIds(filter); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to process ObjectIds in filter: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}
				}

				// Parse the projection
				// Use our specialized projection processor for better handling
				jsonProjStr, err := processProjectionParams(projectionStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process projection parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				if err := json.Unmarshal([]byte(jsonProjStr), &projection); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse projection: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			} else {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: "Invalid parameters format for find. Expected: find({filter}, {projection})",
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		} else {
			// Just a filter
			if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
				// Try to handle MongoDB syntax with unquoted keys and ObjectId
				log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

				// Process the query parameters to handle MongoDB syntax
				jsonStr, err := processMongoDBQueryParams(paramsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process query parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				log.Printf("MongoDBDriver -> ExecuteQuery -> Converted query: %s", jsonStr)

				if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Handle ObjectId in the filter
				if err := processObjectIds(filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Log the final filter for debugging
				filterJSON, _ := json.Marshal(filter)
				log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
			}
		}

		// Extract modifiers from the query string
		modifiers := extractModifiers(query)

		// If count() modifier is present, perform a count operation instead of find
		if modifiers.Count {
			// Execute the countDocuments operation
			count, err := collection.CountDocuments(ctx, filter)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to execute count operation: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			result = map[string]interface{}{
				"count": count,
			}
			break
		}

		// Create find options
		findOptions := options.Find()

		// Apply limit if specified
		if modifiers.Limit > 0 {
			findOptions.SetLimit(modifiers.Limit)
		}

		// Apply skip if specified
		if modifiers.Skip > 0 {
			findOptions.SetSkip(modifiers.Skip)
		}

		// Apply sort if specified
		if modifiers.Sort != "" {
			var sortDoc bson.D
			sortJSON := modifiers.Sort

			// Process the sort expression to handle MongoDB syntax
			if !strings.HasPrefix(sortJSON, "{") {
				sortJSON = fmt.Sprintf(`{"%s": 1}`, sortJSON)
			}

			// Parse the sort document
			var sortMap bson.M
			if err := json.Unmarshal([]byte(sortJSON), &sortMap); err != nil {
				jsonStr, err := processMongoDBQueryParams(sortJSON)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process sort parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				if err := json.Unmarshal([]byte(jsonStr), &sortMap); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse sort parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}

			// Convert the sort map to a bson.D
			for k, v := range sortMap {
				sortDoc = append(sortDoc, bson.E{Key: k, Value: v})
			}

			findOptions.SetSort(sortDoc)
		}

		// Apply projection if specified from the parameters or modifiers
		if projection != nil {
			// Convert the projection map to a bson.D
			var projectionDoc bson.D
			for k, v := range projection {
				projectionDoc = append(projectionDoc, bson.E{Key: k, Value: v})
			}
			findOptions.SetProjection(projectionDoc)
		} else if modifiers.Projection != "" {
			var projectionDoc bson.D
			projectionJSON := modifiers.Projection

			// Process the projection expression to handle MongoDB syntax
			if !strings.HasPrefix(projectionJSON, "{") {
				projectionJSON = fmt.Sprintf(`{"%s": 1}`, projectionJSON)
			}

			// Parse the projection document using our specialized processor
			jsonProjStr, err := processProjectionParams(projectionJSON)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process projection parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			var projectionMap bson.M
			if err := json.Unmarshal([]byte(jsonProjStr), &projectionMap); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse projection: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Convert the projection map to a bson.D
			for k, v := range projectionMap {
				projectionDoc = append(projectionDoc, bson.E{Key: k, Value: v})
			}

			findOptions.SetProjection(projectionDoc)
		}

		// Execute the find operation
		cursor, err := collection.Find(ctx, filter, findOptions)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute find operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}
		defer cursor.Close(ctx)

		// Decode the results
		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to decode find results: %v", err),
					Code:    "DECODE_ERROR",
				},
			}
		}

		result = results

	case "findOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process query parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted query: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Execute the findOne operation
		var doc bson.M
		err = collection.FindOne(ctx, filter).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// No documents found, return empty result
				result = nil
			} else {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to execute findOne operation: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}
		} else {
			result = doc
		}

	case "insertOne":
		// Parse the parameters as a BSON document
		var document bson.M
		if err := json.Unmarshal([]byte(paramsStr), &document); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and special types like Date
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB document: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process document: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted document: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &document); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse document: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId and other special types in the document
			if err := processObjectIds(document); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the insertOne operation
		insertResult, err := collection.InsertOne(ctx, document)
		if err != nil {
			// Check for duplicate key error
			if mongo.IsDuplicateKeyError(err) {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: "Document with the same unique key already exists",
						Code:    "DUPLICATE_KEY",
					},
				}
			}

			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"insertedId": insertResult.InsertedID,
		}

	case "insertMany":
		// Parse the parameters as an array of BSON documents
		var documents []interface{}
		if err := json.Unmarshal([]byte(paramsStr), &documents); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB documents: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process documents: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted documents: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &documents); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse documents after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the insertMany operation
		insertResult, err := collection.InsertMany(ctx, documents)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"insertedIds":   insertResult.InsertedIDs,
			"insertedCount": len(insertResult.InsertedIDs),
		}

	case "updateOne":
		// Parse the parameters as a BSON filter and update
		// The parameters should be in the format {filter}, {update}

		// Split the parameters into filter and update
		splitParams := strings.Split(paramsStr, "}, {")
		if len(splitParams) < 2 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: "Invalid parameters for updateOne. Expected format: {filter}, {update}",
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Reconstruct the filter and update objects
		filterStr := splitParams[0]
		if !strings.HasPrefix(filterStr, "{") {
			filterStr = "{" + filterStr
		}
		if !strings.HasSuffix(filterStr, "}") {
			filterStr = filterStr + "}"
		}

		updateStr := "{" + splitParams[1]
		if !strings.HasSuffix(updateStr, "}") {
			updateStr = updateStr + "}"
		}

		log.Printf("MongoDBDriver -> ExecuteQuery -> Split parameters into filter: %s and update: %s", filterStr, updateStr)

		// Parse the filter
		var filter bson.M
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", filterStr)

			// Process the query parameters to handle MongoDB syntax
			jsonFilterStr, err := processMongoDBQueryParams(filterStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted filter: %s", jsonFilterStr)

			if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Process update with MongoDB syntax
		var update bson.M
		if err := json.Unmarshal([]byte(updateStr), &update); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB update: %s", updateStr)

			// Process the query parameters to handle MongoDB syntax
			jsonUpdateStr, err := processMongoDBQueryParams(updateStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process update parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted update: %s", jsonUpdateStr)

			if err := json.Unmarshal([]byte(jsonUpdateStr), &update); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse update after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the updateOne operation
		updateResult, err := collection.UpdateOne(ctx, filter, update)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute updateOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		// Check if any document was matched
		if updateResult.MatchedCount == 0 {
			log.Printf("MongoDBDriver -> ExecuteQuery -> No document matched the filter criteria for updateOne")
		}

		result = map[string]interface{}{
			"matchedCount":  updateResult.MatchedCount,
			"modifiedCount": updateResult.ModifiedCount,
			"upsertedId":    updateResult.UpsertedID,
		}

	case "updateMany":
		// Parse the parameters as a BSON filter and update
		// The parameters should be in the format {filter}, {update}

		// Split the parameters into filter and update
		splitParams := strings.Split(paramsStr, "}, {")
		if len(splitParams) < 2 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: "Invalid parameters for updateMany. Expected format: {filter}, {update}",
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Reconstruct the filter and update objects
		filterStr := splitParams[0]
		if !strings.HasPrefix(filterStr, "{") {
			filterStr = "{" + filterStr
		}
		if !strings.HasSuffix(filterStr, "}") {
			filterStr = filterStr + "}"
		}

		updateStr := "{" + splitParams[1]
		if !strings.HasSuffix(updateStr, "}") {
			updateStr = updateStr + "}"
		}

		log.Printf("MongoDBDriver -> ExecuteQuery -> Split parameters into filter: %s and update: %s", filterStr, updateStr)

		// Parse the filter
		var filter bson.M
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", filterStr)

			// Process the query parameters to handle MongoDB syntax
			jsonFilterStr, err := processMongoDBQueryParams(filterStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted filter: %s", jsonFilterStr)

			if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after conversion: %s", string(filterJSON))
		}

		// Process update with MongoDB syntax
		var update bson.M
		if err := json.Unmarshal([]byte(updateStr), &update); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB update: %s", updateStr)

			// Process the query parameters to handle MongoDB syntax
			jsonUpdateStr, err := processMongoDBQueryParams(updateStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process update parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted update: %s", jsonUpdateStr)

			if err := json.Unmarshal([]byte(jsonUpdateStr), &update); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse update after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the updateMany operation
		updateResult, err := collection.UpdateMany(ctx, filter, update)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute updateMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"matchedCount":  updateResult.MatchedCount,
			"modifiedCount": updateResult.ModifiedCount,
			"upsertedId":    updateResult.UpsertedID,
		}

	case "deleteOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process query parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted query: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Execute the deleteOne operation
		deleteResult, err := collection.DeleteOne(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		// Check if any document was deleted
		if deleteResult.DeletedCount == 0 {
			log.Printf("MongoDBDriver -> ExecuteQuery -> No document matched the filter criteria for deleteOne")
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "deleteMany":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and operators like $or
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax

			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted filter: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after conversion: %s", string(filterJSON))
		}

		// Execute the deleteMany operation
		deleteResult, err := collection.DeleteMany(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "aggregate":
		// Extract the aggregation pipeline
		// Handle both db.collection.aggregate([...]) and aggregate([...]) formats
		// Remove .toArray() if present
		// Find the opening parenthesis
		pipelineStart := strings.Index(query, ".aggregate(")
		if pipelineStart == -1 {
			pipelineStart = strings.Index(query, "aggregate(")
		}

		if pipelineStart != -1 {
			// Extract the parenthesis content using the helper function
			openParenIndex := pipelineStart + strings.Index(query[pipelineStart:], "(")
			pipelineStr, _, pipelineErr := extractParenthesisContent(query, openParenIndex)
			if pipelineErr == nil {
				// If the extraction was successful, use the extracted pipeline
				paramsStr = pipelineStr
				log.Printf("MongoDBDriver -> ExecuteQuery -> Extracted aggregation pipeline: %s", paramsStr)
			}
		}

		// Parse the parameters as a pipeline
		var pipeline []bson.M
		if err := json.Unmarshal([]byte(paramsStr), &pipeline); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB aggregation pipeline: %s", paramsStr)

			// Process each stage of the pipeline individually
			// This helps with complex expressions that might not parse correctly as a whole
			stagesRegex := regexp.MustCompile(`\{([^{}]*(?:\{[^{}]*\}[^{}]*)*)\}`)
			stageMatches := stagesRegex.FindAllStringSubmatch(paramsStr, -1)

			// Create an array of processed stages
			processedStages := make([]string, 0, len(stageMatches))

			for _, match := range stageMatches {
				if len(match) < 2 {
					continue
				}

				// Get the stage content and wrap it in curly braces
				stageContent := "{" + match[1] + "}"

				// Check if this is a $project stage to use special handling
				if strings.Contains(stageContent, "$project") {
					log.Printf("MongoDBDriver -> ExecuteQuery -> Detected $project stage in pipeline: %s", stageContent)
				}

				// Process the stage content
				processedStage, err := processMongoDBQueryParams(stageContent)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process aggregation stage: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Replace "new Date(...)" with string placeholder before JSON parsing
				// First clean up the format to ensure it's valid JSON
				dateObjPattern := regexp.MustCompile(`new\s+Date\(([^)]*)\)`)
				processedStage = dateObjPattern.ReplaceAllString(processedStage, `"__DATE_PLACEHOLDER__"`)

				// Also replace any remaining date objects
				dateJsonPattern := regexp.MustCompile(`\{\s*"\$date"\s*:\s*"[^"]+"\s*\}`)
				processedStage = dateJsonPattern.ReplaceAllString(processedStage, `"__DATE_PLACEHOLDER__"`)

				log.Printf("MongoDBDriver -> ExecuteQuery -> Processed stage: %s", processedStage)
				processedStages = append(processedStages, processedStage)
			}

			// Combine the processed stages into a valid JSON array
			jsonStr := "[" + strings.Join(processedStages, ",") + "]"
			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted aggregation pipeline: %s", jsonStr)

			// Fix any remaining date expressions that might have slipped through
			// This ensures we don't have "new Date(...)" in the JSON string
			dateRegex := regexp.MustCompile(`new\s+Date\((?:[^)]*)\)`)
			jsonStr = dateRegex.ReplaceAllString(jsonStr, `"__DATE_PLACEHOLDER__"`)

			// Extra fix for the specific pattern seen in logs
			specificDatePattern := regexp.MustCompile(`new\s+Date\(["']__DATE_PLACEHOLDER__["']\)`)
			jsonStr = specificDatePattern.ReplaceAllString(jsonStr, `"__DATE_PLACEHOLDER__"`)

			// Fix any corrupted field names with extra double quotes
			// This matches patterns like ""user.email"" and replaces them with "user.email"
			fixFieldNamesPattern := regexp.MustCompile(`""([^"]+)""`)
			jsonStr = fixFieldNamesPattern.ReplaceAllString(jsonStr, `"$1"`)

			log.Printf("MongoDBDriver -> ExecuteQuery -> Final aggregation pipeline after cleanup: %s", jsonStr)

			// Make sure to catch any other variations
			for strings.Contains(jsonStr, "new Date") {
				jsonStr = strings.Replace(jsonStr, "new Date", `"__DATE_PLACEHOLDER__"`, -1)
			}

			// Try to parse the cleaned-up JSON
			if err := json.Unmarshal([]byte(jsonStr), &pipeline); err != nil {
				log.Printf("MongoDBDriver -> ExecuteQuery -> Error parsing pipeline JSON: %v", err)
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse aggregation pipeline after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
			log.Printf("MongoDBDriver -> ExecuteQuery -> Successfully parsed aggregation pipeline with %d stages", len(pipeline))
		}

		// Process dot notation fields in the pipeline for improved support of
		// accessing fields from joined documents after $lookup and $unwind
		ProcessDotNotationFields(map[string]interface{}{"pipeline": pipeline})

		// Also use specialized processor for dot notation in aggregations
		if err := processDotNotationInAggregation(pipeline); err != nil {
			log.Printf("MongoDBDriver -> ExecuteQuery -> Error processing dot notation in pipeline: %v", err)
		}

		// Execute the aggregation
		cursor, err := collection.Aggregate(ctx, pipeline)
		if err != nil {
			log.Printf("MongoDBDriver -> ExecuteQuery -> Error executing aggregation: %v", err)
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute aggregation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}
		defer cursor.Close(ctx)

		// Process the results
		result := processAggregationResultsFromCursor(cursor, ctx)

		// Set the execution time
		result.ExecutionTime = int(time.Since(startTime).Milliseconds())

		// Log the execution time
		log.Printf("MongoDBDriver -> ExecuteQuery -> MongoDB query executed in %d ms", result.ExecutionTime)

		return result

	case "countDocuments":
		// Parse the parameters as a BSON filter
		var filter bson.M

		// Handle empty parameters for countDocuments()
		if strings.TrimSpace(paramsStr) == "" {
			// Use an empty filter to count all documents
			filter = bson.M{}
		} else {
			// Parse the provided filter
			if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
				// Try to handle MongoDB syntax with unquoted keys
				log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", paramsStr)

				// Process the query parameters to handle MongoDB syntax

				jsonStr, err := processMongoDBQueryParams(paramsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				log.Printf("MongoDBDriver -> ExecuteQuery -> Converted filter: %s", jsonStr)

				if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse filter: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Handle ObjectId in the filter
				if err := processObjectIds(filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}
		}

		// Execute the countDocuments operation
		count, err := collection.CountDocuments(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute countDocuments operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"count": count,
		}

	case "createCollection":
		// Execute the createCollection operation with default options
		// We're simplifying this implementation to avoid complex option handling
		err := collection.Database().CreateCollection(ctx, collectionName)
		if err != nil {
			// Check if collection already exists
			if strings.Contains(err.Error(), "already exists") {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Collection '%s' already exists", collectionName),
						Code:    "COLLECTION_EXISTS",
					},
				}
			}

			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to create collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' created successfully", collectionName),
		}

	case "dropCollection":
		// Execute the dropCollection operation
		err := collection.Drop(ctx)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to drop collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' dropped successfully", collectionName),
		}

	case "drop":
		// Check if collection exists before dropping
		collections, err := wrapper.Client.Database(wrapper.Database).ListCollectionNames(ctx, bson.M{"name": collectionName})
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		// If collection doesn't exist, return an error
		if len(collections) == 0 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Collection '%s' does not exist", collectionName),
					Code:    "COLLECTION_NOT_FOUND",
				},
			}
		}

		// Execute the drop operation
		err = collection.Drop(ctx)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to drop collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' dropped successfully", collectionName),
		}

	default:
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Unsupported MongoDB operation: %s", operation),
				Code:    "UNSUPPORTED_OPERATION",
			},
		}
	}

	// Convert the result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Failed to marshal result to JSON: %v", err),
				Code:    "JSON_ERROR",
			},
		}
	}

	var resultMap map[string]interface{}
	if tempResultMap, ok := result.(map[string]interface{}); ok {
		// Create a result map
		resultMap = tempResultMap
	} else {
		resultMap = map[string]interface{}{
			"results": result,
		}
	}

	executionTime := int(time.Since(startTime).Milliseconds())
	log.Printf("MongoDBDriver -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

	return &QueryExecutionResult{
		Result:        resultMap,
		ResultJSON:    string(resultJSON),
		ExecutionTime: executionTime,
	}
}

// BeginTx begins a MongoDB transaction
func (d *MongoDBDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	log.Printf("MongoDBDriver -> BeginTx -> Beginning MongoDB transaction")

	// Debug logging: Is MongoDBObj set in the connection?
	if conn.MongoDBObj == nil {
		log.Printf("MongoDBDriver -> BeginTx -> ERROR: MongoDBObj is nil in connection struct")
		return &MongoDBTransaction{
			Error: fmt.Errorf("MongoDBObj is not connected properly, try disconnecting and reconnecting"),
		}
	}

	// Get the MongoDB wrapper
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		log.Printf("MongoDBDriver -> BeginTx -> Invalid MongoDB connection, type: %T", conn.MongoDBObj)
		return &MongoDBTransaction{
			Error: fmt.Errorf("invalid MongoDB connection"),
			// Session is nil here, but that's expected since we have an error
		}
	}

	// Ensure the client is not nil
	if wrapper.Client == nil {
		log.Printf("MongoDBDriver -> BeginTx -> MongoDB client is nil")
		return &MongoDBTransaction{
			Error:   fmt.Errorf("MongoDB client is nil"),
			Wrapper: wrapper,
			// Session is nil here, but that's expected since we have an error
		}
	}

	// Verify the connection is alive before starting a transaction
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := wrapper.Client.Ping(pingCtx, readpref.Primary()); err != nil {
		log.Printf("MongoDBDriver -> BeginTx -> MongoDB connection is not alive: %v", err)
		return &MongoDBTransaction{
			Error:   fmt.Errorf("MongoDB connection is not alive: %v", err),
			Wrapper: wrapper,
		}
	}

	// Start a new session with retry logic
	var session mongo.Session
	var err error

	// Try up to 3 times to start a session
	for attempts := 0; attempts < 3; attempts++ {
		session, err = wrapper.Client.StartSession()
		if err == nil {
			break
		}
		log.Printf("MongoDBDriver -> BeginTx -> Error starting MongoDB session (attempt %d/3): %v", attempts+1, err)
		time.Sleep(500 * time.Millisecond) // Wait before retrying
	}

	if err != nil {
		log.Printf("MongoDBDriver -> BeginTx -> Failed to start MongoDB session after retries: %v", err)
		return &MongoDBTransaction{
			Error:   fmt.Errorf("failed to start MongoDB session after retries: %v", err),
			Wrapper: wrapper,
		}
	}

	// Start a transaction with retry logic
	for attempts := 0; attempts < 3; attempts++ {
		err = session.StartTransaction()
		if err == nil {
			break
		}
		log.Printf("MongoDBDriver -> BeginTx -> Error starting MongoDB transaction (attempt %d/3): %v", attempts+1, err)
		time.Sleep(500 * time.Millisecond) // Wait before retrying
	}

	if err != nil {
		log.Printf("MongoDBDriver -> BeginTx -> Failed to start MongoDB transaction after retries: %v", err)
		session.EndSession(ctx)
		return &MongoDBTransaction{
			Error:   fmt.Errorf("failed to start MongoDB transaction after retries: %v", err),
			Wrapper: wrapper,
		}
	}

	// Create a new transaction object
	tx := &MongoDBTransaction{
		Session: session,
		Wrapper: wrapper,
		Error:   nil,
	}

	log.Printf("MongoDBDriver -> BeginTx -> MongoDB transaction started successfully")
	return tx
}
