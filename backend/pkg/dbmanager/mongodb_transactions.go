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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoDBTransaction implements the Transaction interface for MongoDB
type MongoDBTransaction struct {
	Session mongo.Session
	Wrapper *MongoDBWrapper
	Error   error
}

// Commit commits a MongoDB transaction
func (tx *MongoDBTransaction) Commit() error {
	log.Printf("MongoDBTransaction -> Commit -> Committing MongoDB transaction")

	// Check if the session is nil (which can happen if there was an error creating the transaction)
	if tx.Session == nil {
		log.Printf("MongoDBTransaction -> Commit -> No session to commit (session is nil)")
		if tx.Error != nil {
			log.Printf("MongoDBTransaction -> Commit -> Original error: %v", tx.Error)
			return fmt.Errorf("cannot commit transaction: %v", tx.Error)
		}
		return fmt.Errorf("cannot commit transaction: session is nil")
	}

	// Check if the wrapper or client is nil
	if tx.Wrapper == nil || tx.Wrapper.Client == nil {
		log.Printf("MongoDBTransaction -> Commit -> Wrapper or client is nil")
		return fmt.Errorf("cannot commit: wrapper or client is nil")
	}

	// Check if there was an error starting the transaction
	if tx.Error != nil {
		log.Printf("MongoDBTransaction -> Commit -> Cannot commit with error: %v", tx.Error)
		// End the session if it exists
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tx.Session.EndSession(ctx)
		return fmt.Errorf("cannot commit transaction with error: %v", tx.Error)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Commit the transaction with retry logic
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		err = tx.Session.CommitTransaction(ctx)
		if err == nil {
			break
		}
		log.Printf("MongoDBTransaction -> Commit -> Error committing transaction (attempt %d/3): %v", attempts+1, err)
		time.Sleep(500 * time.Millisecond) // Wait before retrying
	}

	if err != nil {
		log.Printf("MongoDBTransaction -> Commit -> Failed to commit transaction after retries: %v", err)
		// Still try to end the session even if commit fails
		tx.Session.EndSession(ctx)
		return fmt.Errorf("failed to commit MongoDB transaction: %v", err)
	}

	// End the session
	tx.Session.EndSession(ctx)

	log.Printf("MongoDBTransaction -> Commit -> MongoDB transaction committed successfully")
	return nil
}

// Rollback rolls back a MongoDB transaction
func (tx *MongoDBTransaction) Rollback() error {
	log.Printf("MongoDBTransaction -> Rollback -> Rolling back MongoDB transaction")

	// Check if the session is nil (which can happen if there was an error creating the transaction)
	if tx.Session == nil {
		log.Printf("MongoDBTransaction -> Rollback -> No session to roll back (session is nil)")
		if tx.Error != nil {
			log.Printf("MongoDBTransaction -> Rollback -> Original error: %v", tx.Error)
			return tx.Error
		}
		return nil
	}

	// Check if the wrapper or client is nil
	if tx.Wrapper == nil || tx.Wrapper.Client == nil {
		log.Printf("MongoDBTransaction -> Rollback -> Wrapper or client is nil")
		return fmt.Errorf("cannot rollback: wrapper or client is nil")
	}

	// Check if there was an error starting the transaction
	if tx.Error != nil {
		// If there was an error starting the transaction, just end the session
		log.Printf("MongoDBTransaction -> Rollback -> Rolling back with error: %v", tx.Error)

		// Use a timeout context for ending the session
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tx.Session.EndSession(ctx)
		return tx.Error
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Abort the transaction with retry logic
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		err = tx.Session.AbortTransaction(ctx)
		if err == nil {
			break
		}
		log.Printf("MongoDBTransaction -> Rollback -> Error aborting transaction (attempt %d/3): %v", attempts+1, err)
		time.Sleep(500 * time.Millisecond) // Wait before retrying
	}

	if err != nil {
		log.Printf("MongoDBTransaction -> Rollback -> Failed to abort transaction after retries: %v", err)
		// Still try to end the session even if abort fails
		tx.Session.EndSession(ctx)
		return fmt.Errorf("failed to abort MongoDB transaction: %v", err)
	}

	// End the session
	tx.Session.EndSession(ctx)

	log.Printf("MongoDBTransaction -> Rollback -> MongoDB transaction rolled back successfully")
	return nil
}

// ExecuteQuery executes a MongoDB query within a transaction
func (tx *MongoDBTransaction) ExecuteQuery(ctx context.Context, query string) (*QueryExecutionResult, error) {
	log.Printf("MongoDBTransaction -> ExecuteQuery -> Executing MongoDB query in transaction: %s", query)
	startTime := time.Now()

	// Check if the session is nil (which can happen if there was an error creating the transaction)
	if tx.Session == nil {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Cannot execute query: session is nil")
		errorMsg := "Cannot execute query: transaction session is nil"
		if tx.Error != nil {
			errorMsg = fmt.Sprintf("Cannot execute query: %v", tx.Error)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Original error: %v", tx.Error)
		}
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: errorMsg,
				Code:    "TRANSACTION_ERROR",
			},
		}, nil
	}

	// Check if there was an error starting the transaction
	if tx.Error != nil {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Transaction error: %v", tx.Error)
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Transaction error: %v", tx.Error),
				Code:    "TRANSACTION_ERROR",
			},
		}, nil
	}

	// Check if the wrapper is nil
	if tx.Wrapper == nil {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Wrapper is nil")
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Transaction wrapper is nil",
				Code:    "TRANSACTION_ERROR",
			},
		}, nil
	}

	// Verify the client is not nil
	if tx.Wrapper.Client == nil {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB client is nil")
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "MongoDB client is nil",
				Code:    "TRANSACTION_ERROR",
			},
		}, nil
	}

	// Verify the session is still valid by checking if the client is still connected
	// This is a lightweight check that doesn't require a full ping
	if tx.Wrapper.Client.NumberSessionsInProgress() == 0 {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> No active sessions, session may have expired")
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Transaction session may have expired",
				Code:    "TRANSACTION_ERROR",
			},
		}, nil
	}

	// Special case for createCollection which has a different format
	// Example: db.createCollection("collectionName", {...})
	if strings.Contains(query, "createCollection") {
		createCollectionRegex := regexp.MustCompile(`(?s)db\.createCollection\(["']([^"']+)["'](?:\s*,\s*)(.*)\)`)
		matches := createCollectionRegex.FindStringSubmatch(query)
		if len(matches) >= 3 {
			collectionName := matches[1]
			optionsStr := strings.TrimSpace(matches[2])

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Matched createCollection with collection: %s and options length: %d", collectionName, len(optionsStr))

			// Process the options
			var optionsMap bson.M
			if optionsStr != "" {
				// Process the options to handle MongoDB syntax
				jsonStr, err := processMongoDBQueryParams(optionsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process collection options: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
				}

				if err := json.Unmarshal([]byte(jsonStr), &optionsMap); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse collection options: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
				}
			}

			// Check if collection already exists
			collections, err := tx.Wrapper.Client.Database(tx.Wrapper.Database).ListCollectionNames(ctx, bson.M{"name": collectionName})
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}, nil
			}

			// If collection already exists, return an error
			if len(collections) > 0 {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Collection '%s' already exists", collectionName),
						Code:    "COLLECTION_EXISTS",
					},
				}, nil
			}

			// Create collection options
			var createOptions *options.CreateCollectionOptions
			if optionsMap != nil {
				// Convert validator to proper format if it exists
				if validator, ok := optionsMap["validator"]; ok {
					createOptions = &options.CreateCollectionOptions{
						Validator: validator,
					}
				}
			}

			// Execute the createCollection operation
			err = tx.Wrapper.Client.Database(tx.Wrapper.Database).CreateCollection(ctx, collectionName, createOptions)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to create collection: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}, nil
			}

			result := map[string]interface{}{
				"ok":      1,
				"message": fmt.Sprintf("Collection '%s' created successfully", collectionName),
			}

			// Convert the result to JSON
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to marshal result to JSON: %v", err),
						Code:    "JSON_ERROR",
					},
				}, nil
			}

			executionTime := int(time.Since(startTime).Milliseconds())
			log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

			return &QueryExecutionResult{
				Result:        result,
				StreamData:    resultJSON,
				ExecutionTime: executionTime,
			}, nil
		}
	}

	// Handle database-level operations
	dbOperationRegex := regexp.MustCompile(`db\.(\w+)\(\s*(.*)\s*\)`)
	if dbOperationMatches := dbOperationRegex.FindStringSubmatch(query); len(dbOperationMatches) >= 2 {
		operation := dbOperationMatches[1]
		paramsStr := ""
		if len(dbOperationMatches) >= 3 {
			paramsStr = dbOperationMatches[2]
		}

		log.Printf("MongoDBTransaction -> ExecuteQuery -> Matched database operation: %s with params: %s", operation, paramsStr)

		switch operation {
		case "getCollectionNames":
			// List all collections in the database
			collections, err := tx.Wrapper.Client.Database(tx.Wrapper.Database).ListCollectionNames(ctx, bson.M{})
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to list collections: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}, nil
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
				}, nil
			}

			executionTime := int(time.Since(startTime).Milliseconds())
			log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

			return &QueryExecutionResult{
				Result:        result,
				StreamData:    resultJSON,
				ExecutionTime: executionTime,
			}, nil

		// Add more database-level operations here as needed
		default:
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Unsupported database operation: %s", operation),
					Code:    "UNSUPPORTED_OPERATION",
				},
			}, nil
		}
	}

	// Parse the query
	// Example: db.collection.find({name: "John"})
	parts := strings.SplitN(query, ".", 3)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "db") {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB query format. Expected: db.collection.operation({...})",
				Code:    "INVALID_QUERY",
			},
		}, nil
	}

	collectionName := parts[1]
	operationWithParams := parts[2]

	// Special case handling for empty find() with modifiers
	// Like db.collection.find().sort()
	if strings.HasPrefix(operationWithParams, "find()") && len(operationWithParams) > 6 {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Detected empty find() with modifiers: %s", operationWithParams)
		// Replace find() with find({}) to ensure proper parsing
		operationWithParams = "find({})" + operationWithParams[6:]
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Reformatted query part: %s", operationWithParams)
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
		}, nil
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
		}, nil
	}

	log.Printf("MongoDBTransaction -> ExecuteQuery -> Extracted operation: %s, params: %s", operation, paramsStr)

	// Special case for find() with no parameters but with modifiers like .sort(), .limit()
	// For example: db.collection.find().sort({field: -1})
	if operation == "find" && strings.HasPrefix(paramsStr, ")") && strings.Contains(paramsStr, ".") {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Detected find() with no parameters but with modifiers")

		// Extract modifiers from the parameters string
		modifiersStr := paramsStr

		// Set parameters to empty object
		paramsStr = "{}"

		log.Printf("MongoDBTransaction -> ExecuteQuery -> Using empty object for parameters and parsing modifiers: %s", modifiersStr)
	}

	// Handle empty parameters case - if the parameters are empty, use an empty JSON object
	if strings.TrimSpace(paramsStr) == "" {
		paramsStr = "{}"
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Empty parameters detected, using empty object {}")
	}

	// Handle query modifiers like .limit(), .skip(), etc.
	modifiers := make(map[string]interface{})
	if closeParenIndex < len(operationWithParams)-1 {
		// There might be modifiers after the closing parenthesis
		modifiersStr := operationWithParams[closeParenIndex+1:]

		log.Printf("MongoDBTransaction -> ExecuteQuery -> Modifiers string: %s", modifiersStr)

		// Extract limit modifier
		limitRegex := regexp.MustCompile(`\.limit\((\d+)\)`)
		if limitMatches := limitRegex.FindStringSubmatch(modifiersStr); len(limitMatches) > 1 {
			if limit, err := strconv.Atoi(limitMatches[1]); err == nil {
				modifiers["limit"] = limit
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Found limit modifier: %d", limit)
			}
		}

		// Extract skip modifier
		skipRegex := regexp.MustCompile(`\.skip\((\d+)\)`)
		if skipMatches := skipRegex.FindStringSubmatch(modifiersStr); len(skipMatches) > 1 {
			if skip, err := strconv.Atoi(skipMatches[1]); err == nil {
				modifiers["skip"] = skip
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Found skip modifier: %d", skip)
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
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Processed sort modifier: %s", jsonStr)
			} else {
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Error processing sort modifier: %v", err)
				modifiers["sort"] = sortExpr
			}
		}
	}

	// Get the MongoDB collection
	collection := tx.Wrapper.Client.Database(tx.Wrapper.Database).Collection(collectionName)

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
			}, nil
		}

		if len(collections) == 0 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Collection '%s' does not exist", collectionName),
					Code:    "COLLECTION_NOT_FOUND",
				},
			}, nil
		}
	}

	var result interface{}
	var err error

	log.Printf("MongoDBTransaction -> ExecuteQuery -> operation: %s", operation)
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

				log.Printf("MongoDBTransaction -> ExecuteQuery -> Split parameters into filter: %s and projection: %s", filterStr, projectionStr)

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
						}, nil
					}

					if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to parse filter: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}, nil
					}

					// Handle ObjectId in the filter
					if err := processObjectIds(filter); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to process ObjectIds in filter: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}, nil
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
					}, nil
				}

				if err := json.Unmarshal([]byte(jsonProjStr), &projection); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse projection: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
				}
			} else {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: "Invalid parameters format for find. Expected: find({filter}, {projection})",
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}
		} else {
			// Just a filter
			if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
				// Try to handle MongoDB syntax with unquoted keys and ObjectId
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

				// Process the query parameters to handle MongoDB syntax
				jsonStr, err := processMongoDBQueryParams(paramsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process query parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
				}

				log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted query: %s", jsonStr)

				if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
				}

				// Handle ObjectId in the filter
				if err := processObjectIds(filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
				}

				// Log the final filter for debugging
				filterJSON, _ := json.Marshal(filter)
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
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
				}, nil
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
					}, nil
				}

				if err := json.Unmarshal([]byte(jsonStr), &sortMap); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse sort parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
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
				}, nil
			}

			var projectionMap bson.M
			if err := json.Unmarshal([]byte(jsonProjStr), &projectionMap); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse projection: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
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
			}, nil
		}
		defer cursor.Close(ctx)

		// Process the results
		result := processAggregationResultsFromCursor(cursor, ctx)

		// Set the execution time
		result.ExecutionTime = int(time.Since(startTime).Milliseconds())

		// Log the execution time
		log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB query executed in %d ms", result.ExecutionTime)

		return result, nil

	case "findOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process query parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted query: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
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
				}, nil
			}
		} else {
			result = doc
		}

	case "insertOne":
		// Parse the parameters as a BSON document
		var document bson.M
		if err := json.Unmarshal([]byte(paramsStr), &document); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and special types like Date
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB document: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process document: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted document: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &document); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse document: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Handle ObjectId and other special types in the document
			if err := processObjectIds(document); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
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
				}, nil
			}

			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}, nil
		}

		result = map[string]interface{}{
			"insertedId": insertResult.InsertedID,
		}

	case "insertMany":
		// Parse the parameters as an array of BSON documents
		var documents []interface{}
		if err := json.Unmarshal([]byte(paramsStr), &documents); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB documents: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process documents: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted documents: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &documents); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse documents after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
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
			}, nil
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
			}, nil
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

		log.Printf("MongoDBTransaction -> ExecuteQuery -> Split parameters into filter: %s and update: %s", filterStr, updateStr)

		// Parse the filter
		var filter bson.M
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", filterStr)

			// Process the query parameters to handle MongoDB syntax
			jsonFilterStr, err := processMongoDBQueryParams(filterStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted filter: %s", jsonFilterStr)

			if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after conversion: %s", string(filterJSON))
		}

		// Process update with MongoDB syntax
		var update bson.M
		if err := json.Unmarshal([]byte(updateStr), &update); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB update: %s", updateStr)

			// Process the query parameters to handle MongoDB syntax
			jsonUpdateStr, err := processMongoDBQueryParams(updateStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process update parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted update: %s", jsonUpdateStr)

			if err := json.Unmarshal([]byte(jsonUpdateStr), &update); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse update after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
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
			}, nil
		}

		// Check if any document was matched
		if updateResult.MatchedCount == 0 {
			log.Printf("MongoDBTransaction -> ExecuteQuery -> No document matched the filter criteria for updateOne")
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
			}, nil
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

		log.Printf("MongoDBTransaction -> ExecuteQuery -> Split parameters into filter: %s and update: %s", filterStr, updateStr)

		// Parse the filter
		var filter bson.M
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", filterStr)

			// Process the query parameters to handle MongoDB syntax
			jsonFilterStr, err := processMongoDBQueryParams(filterStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted filter: %s", jsonFilterStr)

			if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after conversion: %s", string(filterJSON))
		}

		// Process update with MongoDB syntax
		var update bson.M
		if err := json.Unmarshal([]byte(updateStr), &update); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB update: %s", updateStr)

			// Process the query parameters to handle MongoDB syntax
			jsonUpdateStr, err := processMongoDBQueryParams(updateStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process update parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted update: %s", jsonUpdateStr)

			if err := json.Unmarshal([]byte(jsonUpdateStr), &update); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse update after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
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
			}, nil
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
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process query parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted query: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Execute the deleteOne operation
		deleteResult, err := collection.DeleteOne(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}, nil
		}

		// Check if any document was deleted
		if deleteResult.DeletedCount == 0 {
			log.Printf("MongoDBTransaction -> ExecuteQuery -> No document matched the filter criteria for deleteOne")
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "deleteMany":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and operators like $or
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax

			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted filter: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after conversion: %s", string(filterJSON))
		}

		// Execute the deleteMany operation
		deleteResult, err := collection.DeleteMany(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}, nil
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "aggregate":
		// Extract the aggregation pipeline if necessary
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
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Extracted aggregation pipeline: %s", paramsStr)
			}
		}

		var pipeline []bson.M

		// Try to parse the pipeline directly as a JSON array
		if err := json.Unmarshal([]byte(paramsStr), &pipeline); err != nil {
			// If direct parsing fails, handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB aggregation pipeline: %s", paramsStr)

			// Use the new parser to properly extract stages
			stages, parseErr := ParseAggregationPipeline(paramsStr)
			if parseErr != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse aggregation pipeline: %v", parseErr),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}

			// Create an array of processed stages
			processedStages := make([]string, 0, len(stages))

			for i, stageContent := range stages {
				// Trim any whitespace and trailing commas from the stage
				stageContent = strings.TrimSpace(stageContent)
				stageContent = strings.TrimSuffix(stageContent, ",")
				
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Processing stage %d: %s", i, stageContent)
				
				// Check if this is a $project stage
				if strings.Contains(stageContent, "$project") {
					log.Printf("MongoDBTransaction -> ExecuteQuery -> Detected $project stage in pipeline")
				}

				// Process the stage content
				processedStage, err := processMongoDBQueryParams(stageContent)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process aggregation stage: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
				}

				// Clean up the processed stage - remove any trailing commas or whitespace
				processedStage = strings.TrimSpace(processedStage)
				processedStage = strings.TrimSuffix(processedStage, ",")
				
				// Don't replace dates with placeholders - let processObjectIds handle them
				// This preserves the date values for proper BSON conversion

				log.Printf("MongoDBTransaction -> ExecuteQuery -> Processed stage %d: %s", i, processedStage)
				
				// Only add non-empty stages
				if processedStage != "" && processedStage != "," {
					processedStages = append(processedStages, processedStage)
				}
			}

			// Combine the processed stages into a valid JSON array
			jsonStr := "[" + strings.Join(processedStages, ", ") + "]"
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted aggregation pipeline: %s", jsonStr)

			// Don't replace dates - processObjectIds will handle them properly

			// Fix any corrupted field names with extra double quotes
			// This matches patterns like ""user.email"" and replaces them with "user.email"
			fixFieldNamesPattern := regexp.MustCompile(`""([^"]+)""`)
			jsonStr = fixFieldNamesPattern.ReplaceAllString(jsonStr, `"$1"`)

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final aggregation pipeline after cleanup: %s", jsonStr)

			// Don't replace dates - let processObjectIds handle proper BSON conversion

			// Try to parse the cleaned-up JSON
			if err := json.Unmarshal([]byte(jsonStr), &pipeline); err != nil {
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Error parsing pipeline JSON: %v", err)
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse aggregation pipeline after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}, nil
			}
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Successfully parsed aggregation pipeline with %d stages", len(pipeline))
		}

		// Process dot notation fields in the pipeline for improved support of
		// accessing fields from joined documents after $lookup and $unwind
		ProcessDotNotationFields(map[string]interface{}{"pipeline": pipeline})

		// Also use specialized processor for dot notation in aggregations
		if err := processDotNotationInAggregation(pipeline); err != nil {
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Error processing dot notation in pipeline: %v", err)
		}
		
		// Process ObjectIds and Dates in the pipeline
		for _, stage := range pipeline {
			if err := processObjectIds(stage); err != nil {
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Error processing ObjectIds/Dates in pipeline: %v", err)
			}
		}

		// Execute the aggregation
		cursor, err := collection.Aggregate(ctx, pipeline)
		if err != nil {
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Error executing aggregation: %v", err)
			
			// Log the actual pipeline that failed for debugging
			pipelineJSON, _ := json.Marshal(pipeline)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Failed pipeline: %s", string(pipelineJSON))
			
			// Check for specific MongoDB errors and provide better error messages
			errMsg := err.Error()
			if strings.Contains(errMsg, "$dateToString is not allowed") {
				errMsg = "Date formatting operators like $dateToString may not be supported in this MongoDB deployment. Consider using application-level date formatting instead."
			} else if strings.Contains(errMsg, "unknown operator") {
				errMsg = fmt.Sprintf("MongoDB operator error: %v. Some operators may not be supported in your MongoDB version or deployment type.", err)
			}
			
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: errMsg,
					Code:    "EXECUTION_ERROR",
				},
			}, nil
		}
		defer cursor.Close(ctx)

		// Process the results
		result := processAggregationResultsFromCursor(cursor, ctx)

		// Set the execution time
		result.ExecutionTime = int(time.Since(startTime).Milliseconds())

		// Log the execution time
		log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB query executed in %d ms", result.ExecutionTime)

		return result, nil

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
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", paramsStr)

				// Process the query parameters to handle MongoDB syntax
				jsonStr, err := processMongoDBQueryParams(paramsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
				}

				log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted filter: %s", jsonStr)

				if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse filter: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
				}

				// Handle ObjectId in the filter
				if err := processObjectIds(filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}, nil
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
			}, nil
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
				}, nil
			}

			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to create collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}, nil
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
			}, nil
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' dropped successfully", collectionName),
		}

	case "drop":
		// Check if collection exists before dropping
		collections, err := tx.Wrapper.Client.Database(tx.Wrapper.Database).ListCollectionNames(ctx, bson.M{"name": collectionName})
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}, nil
		}

		// If collection doesn't exist, return an error
		if len(collections) == 0 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Collection '%s' does not exist", collectionName),
					Code:    "COLLECTION_NOT_FOUND",
				},
			}, nil
		}

		// Execute the drop operation
		err = collection.Drop(ctx)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to drop collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}, nil
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
		}, nil
	}

	// After creating the result map
	executionTime := int(time.Since(startTime).Milliseconds())
	log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

	// Create a proper map[string]interface{} for the Result field
	var resultMap map[string]interface{}
	if tempResultMap, ok := result.(map[string]interface{}); ok {
		// Result is already a map, use it directly
		resultMap = tempResultMap
	} else {
		// Wrap the result in a map with "results" key
		resultMap = map[string]interface{}{
			"results": result,
		}
	}

	// Marshal the result to JSON format for the ResultJSON field
	resultJSON, err := json.Marshal(resultMap)
	if err != nil {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Error marshalling results to JSON: %v", err)
		return &QueryExecutionResult{
			Result: resultMap,
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Failed to marshal results to JSON: %v", err),
				Code:    "MARSHAL_ERROR",
			},
			ExecutionTime: executionTime,
		}, nil
	}

	return &QueryExecutionResult{
		Result:        resultMap,
		StreamData:    resultJSON,
		ExecutionTime: executionTime,
	}, nil
}

// ExecuteCommand executes a MongoDB command and returns the result
func (t *MongoDBTransaction) ExecuteCommand(ctx context.Context, dbName string, command interface{}, readPreference *readpref.ReadPref) (*QueryExecutionResult, error) {
	startTime := time.Now()
	log.Printf("Executing MongoDB command on database %s: %+v", dbName, command)

	// Get the database
	db := t.Wrapper.Client.Database(dbName)
	if db == nil {
		return nil, fmt.Errorf("database %s not found", dbName)
	}

	// Set read preference if provided
	opts := options.RunCmd()
	if readPreference != nil {
		opts.SetReadPreference(readPreference)
	}

	// Run the command
	var result bson.M
	err := db.RunCommand(ctx, command, opts).Decode(&result)
	executionTime := time.Since(startTime)

	if err != nil {
		log.Printf("Error executing MongoDB command: %v", err)
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Error executing command: %v", err),
				Code:    "COMMAND_EXECUTION_ERROR",
			},
			ExecutionTime: int(executionTime.Milliseconds()),
			StreamData:    []byte("{\"results\":[]}"),
		}, err
	}

	log.Printf("MongoDB command execution completed in %v", executionTime)

	// Create the result map
	executionResult := &QueryExecutionResult{
		Result:        result,
		ExecutionTime: int(executionTime.Milliseconds()),
	}

	// Format the result as JSON
	resultJSON, err := FormatQueryResult(result)
	if err != nil {
		log.Printf("Error formatting command results: %v", err)
		executionResult.Error = &dtos.QueryError{
			Message: fmt.Sprintf("Error formatting command results: %v", err),
			Code:    "COMMAND_RESULT_FORMAT_ERROR",
		}
		executionResult.StreamData = []byte("{\"results\":[]}")
		return executionResult, err
	}

	executionResult.StreamData = []byte(resultJSON)
	return executionResult, nil
}
