package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/pkg/embedding"
	"neobase-ai/pkg/vectordb"
	"strings"
	"time"
)

// VectorizationService handles embedding and vector storage for schema, knowledge base, and messages.
// Schema/KB vectors live in SchemaCollectionName; message vectors live in MessageCollectionName.
type VectorizationService interface {
	// IsAvailable returns true if both embedding provider and vector DB are ready.
	IsAvailable(ctx context.Context) bool

	// --- Schema & KB ---

	// VectorizeSchema embeds and stores enriched schema chunks for a chat.
	VectorizeSchema(ctx context.Context, chatID string, parsedSchema []SchemaChunk) error

	// SearchSchema performs a similarity search over schema vectors for a chat.
	SearchSchema(ctx context.Context, chatID string, query string, topK int) ([]vectordb.SearchResult, error)

	// HasSchemaVectors returns true if vectorized schema data exists for this chat.
	HasSchemaVectors(ctx context.Context, chatID string) bool

	// --- Messages ---

	// VectorizeMessage embeds and stores a single chat message for conversational RAG.
	VectorizeMessage(ctx context.Context, chatID string, messageID string, role string, content string, messageIndex int) error

	// SearchMessages performs similarity search over vectorized messages for a chat.
	SearchMessages(ctx context.Context, chatID string, query string, topK int, excludeMessageIDs []string) ([]vectordb.SearchResult, error)

	// DeleteMessageVector removes the vector for a single message (used on message edit/delete).
	DeleteMessageVector(ctx context.Context, chatID string, messageID string) error

	// DeleteChatMessageVectors removes all message vectors for a chat (used on "clear messages").
	DeleteChatMessageVectors(ctx context.Context, chatID string) error

	// --- Lifecycle ---

	// DeleteChatVectors removes ALL vectors (schema + KB + messages) for a chat across both collections.
	DeleteChatVectors(ctx context.Context, chatID string) error

	// CopyVectorsForChat copies all vectors (schema + messages) from one chat to another.
	// Used during chat duplication. messageIDMap maps old message IDs to new ones (for message vectors).
	// If copyMessages is false, only schema vectors are copied.
	CopyVectorsForChat(ctx context.Context, sourceChatID, targetChatID string, copyMessages bool, messageIDMap map[string]string) error

	// EnsureReady ensures both Qdrant collections exist with the correct dimension.
	EnsureReady(ctx context.Context) error

	// GetEmbeddingDimension returns the embedding dimension for the current provider.
	GetEmbeddingDimension() int
}

// SchemaChunk represents a single table's schema text prepared for embedding.
type SchemaChunk struct {
	TableName   string            // e.g., "orders"
	Content     string            // Full text chunk for embedding (table description + columns + indexes + FKs)
	ColumnNames []string          // Column names for payload metadata
	ForeignKeys []string          // Referenced table names
	RowCount    int64             // Approximate row count
	Metadata    map[string]string // Additional metadata (db_type, etc.)
}

// TableEnrichment carries KB descriptions and example records for a single table.
// Used to enrich schema chunks before embedding with semantic + data context.
type TableEnrichment struct {
	TableDescription  string                   // AI-generated purpose description
	FieldDescriptions map[string]string        // field_name → description
	ExampleRecords    []map[string]interface{} // Up to 3 example rows from the DB
}

// vectorizationService is the concrete implementation.
type vectorizationService struct {
	embeddingProvider embedding.Provider
	vectorClient      vectordb.Client
}

// NewVectorizationService creates a new vectorization service.
// Returns nil if embedding provider or vector client is nil (graceful degradation).
func NewVectorizationService(
	embeddingProvider embedding.Provider,
	vectorClient vectordb.Client,
) VectorizationService {
	if embeddingProvider == nil || vectorClient == nil {
		log.Println("VectorizationService -> Disabled (embedding provider or vector client is nil)")
		return nil
	}

	svc := &vectorizationService{
		embeddingProvider: embeddingProvider,
		vectorClient:      vectorClient,
	}

	log.Printf("VectorizationService -> Initialized with %s/%s (dimension: %d)",
		embeddingProvider.GetProviderName(),
		embeddingProvider.GetModelName(),
		embeddingProvider.GetDimension(),
	)

	return svc
}

// IsAvailable checks if Qdrant is healthy and embedding is configured.
func (v *vectorizationService) IsAvailable(ctx context.Context) bool {
	return v.vectorClient.IsHealthy(ctx)
}

// EnsureReady creates both Qdrant collections (schema + messages) if they don't exist.
func (v *vectorizationService) EnsureReady(ctx context.Context) error {
	if err := v.vectorClient.EnsureSchemaCollection(ctx, v.embeddingProvider.GetDimension()); err != nil {
		return fmt.Errorf("failed to ensure schema collection: %w", err)
	}
	if err := v.vectorClient.EnsureMessageCollection(ctx, v.embeddingProvider.GetDimension()); err != nil {
		return fmt.Errorf("failed to ensure message collection: %w", err)
	}
	return nil
}

// GetEmbeddingDimension returns the current embedding dimension.
func (v *vectorizationService) GetEmbeddingDimension() int {
	return v.embeddingProvider.GetDimension()
}

// VectorizeSchema embeds schema chunks and upserts them to Qdrant.
func (v *vectorizationService) VectorizeSchema(ctx context.Context, chatID string, chunks []SchemaChunk) error {
	if len(chunks) == 0 {
		log.Printf("VectorizationService -> VectorizeSchema -> No chunks to vectorize for chat %s", chatID)
		return nil
	}

	start := time.Now()
	log.Printf("VectorizationService -> VectorizeSchema -> Starting for chat %s (%d tables)", chatID, len(chunks))

	// Delete existing schema vectors for this chat (to handle removed tables)
	err := v.vectorClient.DeleteByFilter(ctx, constants.SchemaCollectionName, []vectordb.FilterCondition{
		{Key: "chat_id", Value: chatID},
	})
	if err != nil {
		log.Printf("VectorizationService -> VectorizeSchema -> Warning: failed to delete old schema vectors: %v", err)
		// Continue anyway — upsert will overwrite existing IDs
	}

	// Prepare texts for batch embedding
	texts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		texts = append(texts, chunk.Content)
	}

	// Batch embed all chunks
	embeddings, err := v.embeddingProvider.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to embed schema chunks: %w", err)
	}

	if len(embeddings) != len(chunks) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(chunks))
	}

	// Build vector points
	points := make([]vectordb.VectorPoint, 0, len(chunks))
	relationshipTexts := make([]string, 0)
	relationshipMeta := make([]map[string]interface{}, 0)

	for i, chunk := range chunks {
		// Schema vector
		pointID := vectordb.PointID(fmt.Sprintf("schema_%s_%s", chatID, chunk.TableName))

		payload := map[string]interface{}{
			"chat_id":    chatID,
			"type":       "schema",
			"table_name": chunk.TableName,
			"content":    chunk.Content,
		}

		if len(chunk.ColumnNames) > 0 {
			payload["column_names"] = strings.Join(chunk.ColumnNames, ",")
		}
		if len(chunk.ForeignKeys) > 0 {
			payload["foreign_keys"] = strings.Join(chunk.ForeignKeys, ",")
		}
		if chunk.RowCount > 0 {
			payload["row_count"] = chunk.RowCount
		}
		for k, val := range chunk.Metadata {
			payload[k] = val
		}

		points = append(points, vectordb.VectorPoint{
			ID:      pointID,
			Vector:  embeddings[i],
			Payload: payload,
		})

		// Collect relationship texts for separate embedding
		for _, fkTable := range chunk.ForeignKeys {
			relText := fmt.Sprintf("%s has a foreign key referencing %s (relationship)", chunk.TableName, fkTable)
			relationshipTexts = append(relationshipTexts, relText)
			relationshipMeta = append(relationshipMeta, map[string]interface{}{
				"chat_id":      chatID,
				"type":         "relationship",
				"source_table": chunk.TableName,
				"target_table": fkTable,
				"content":      relText,
			})
		}
	}

	// Embed and store relationship vectors
	if len(relationshipTexts) > 0 {
		relEmbeddings, err := v.embeddingProvider.EmbedBatch(ctx, relationshipTexts)
		if err != nil {
			log.Printf("VectorizationService -> VectorizeSchema -> Warning: failed to embed relationships: %v", err)
			// Non-fatal — schema vectors are more important
		} else {
			for i, relEmb := range relEmbeddings {
				meta := relationshipMeta[i]
				relPointID := vectordb.PointID(fmt.Sprintf("rel_%s_%s_%s",
					chatID,
					meta["source_table"],
					meta["target_table"],
				))

				points = append(points, vectordb.VectorPoint{
					ID:      relPointID,
					Vector:  relEmb,
					Payload: meta,
				})
			}
		}
	}

	// Upsert all points into the schema collection
	err = v.vectorClient.Upsert(ctx, constants.SchemaCollectionName, points)
	if err != nil {
		return fmt.Errorf("failed to upsert schema vectors: %w", err)
	}

	log.Printf("VectorizationService -> VectorizeSchema -> Completed for chat %s: %d schema + %d relationship vectors in %v",
		chatID, len(chunks), len(relationshipTexts), time.Since(start))

	return nil
}

// DeleteChatVectors removes ALL vectors for a chat across both collections.
func (v *vectorizationService) DeleteChatVectors(ctx context.Context, chatID string) error {
	filter := []vectordb.FilterCondition{{Key: "chat_id", Value: chatID}}

	// Delete from schema collection
	if err := v.vectorClient.DeleteByFilter(ctx, constants.SchemaCollectionName, filter); err != nil {
		log.Printf("VectorizationService -> DeleteChatVectors -> Warning: schema collection delete failed: %v", err)
	}

	// Delete from message collection
	if err := v.vectorClient.DeleteByFilter(ctx, constants.MessageCollectionName, filter); err != nil {
		log.Printf("VectorizationService -> DeleteChatVectors -> Warning: message collection delete failed: %v", err)
	}

	log.Printf("VectorizationService -> DeleteChatVectors -> Deleted all vectors for chat %s from both collections", chatID)
	return nil
}

// DeleteMessageVector removes the vector for a single message from the message collection.
// Called when a user message is edited (old vector deleted, new one created after re-embedding).
func (v *vectorizationService) DeleteMessageVector(ctx context.Context, chatID string, messageID string) error {
	pointID := vectordb.PointID(fmt.Sprintf("msg_%s_%s", chatID, messageID))
	if err := v.vectorClient.Delete(ctx, constants.MessageCollectionName, []vectordb.PointID{pointID}); err != nil {
		return fmt.Errorf("failed to delete message vector: %w", err)
	}
	log.Printf("VectorizationService -> DeleteMessageVector -> Deleted vector for msg %s in chat %s", messageID, chatID)
	return nil
}

// DeleteChatMessageVectors removes ALL message vectors for a chat ("clear messages" action).
// Schema/KB vectors in the schema collection are left untouched.
func (v *vectorizationService) DeleteChatMessageVectors(ctx context.Context, chatID string) error {
	err := v.vectorClient.DeleteByFilter(ctx, constants.MessageCollectionName, []vectordb.FilterCondition{
		{Key: "chat_id", Value: chatID},
	})
	if err != nil {
		return fmt.Errorf("failed to delete chat message vectors: %w", err)
	}
	log.Printf("VectorizationService -> DeleteChatMessageVectors -> Deleted all message vectors for chat %s", chatID)
	return nil
}

// CopyVectorsForChat copies all vectors from one chat to another.
// Schema/relationship vectors get new point IDs with the target chatID.
// When copyMessages is true, message vectors are also copied with remapped message IDs.
func (v *vectorizationService) CopyVectorsForChat(ctx context.Context, sourceChatID, targetChatID string, copyMessages bool, messageIDMap map[string]string) error {
	start := time.Now()

	// --- 1. Copy schema + relationship vectors ---
	schemaFilter := []vectordb.FilterCondition{{Key: "chat_id", Value: sourceChatID}}
	sourceSchemaPoints, err := v.vectorClient.ScrollByFilter(ctx, constants.SchemaCollectionName, schemaFilter, true)
	if err != nil {
		return fmt.Errorf("failed to scroll schema vectors for chat %s: %w", sourceChatID, err)
	}

	if len(sourceSchemaPoints) > 0 {
		newSchemaPoints := make([]vectordb.VectorPoint, 0, len(sourceSchemaPoints))
		for _, pt := range sourceSchemaPoints {
			if len(pt.Vector) == 0 {
				continue // skip points without vectors
			}

			// Clone payload and update chat_id
			newPayload := make(map[string]interface{}, len(pt.Payload))
			for k, val := range pt.Payload {
				newPayload[k] = val
			}
			newPayload["chat_id"] = targetChatID

			// Determine new point ID based on type
			vecType, _ := pt.Payload["type"].(string)
			var newPointID vectordb.PointID
			switch vecType {
			case "schema":
				tableName, _ := pt.Payload["table_name"].(string)
				newPointID = vectordb.PointID(fmt.Sprintf("schema_%s_%s", targetChatID, tableName))
			case "relationship":
				srcTable, _ := pt.Payload["source_table"].(string)
				tgtTable, _ := pt.Payload["target_table"].(string)
				newPointID = vectordb.PointID(fmt.Sprintf("rel_%s_%s_%s", targetChatID, srcTable, tgtTable))
			default:
				// Unknown type — generate a safe ID
				newPointID = vectordb.PointID(fmt.Sprintf("copy_%s_%s", targetChatID, pt.ID))
			}

			newSchemaPoints = append(newSchemaPoints, vectordb.VectorPoint{
				ID:      newPointID,
				Vector:  pt.Vector,
				Payload: newPayload,
			})
		}

		if len(newSchemaPoints) > 0 {
			if err := v.vectorClient.Upsert(ctx, constants.SchemaCollectionName, newSchemaPoints); err != nil {
				return fmt.Errorf("failed to upsert copied schema vectors: %w", err)
			}
		}

		log.Printf("VectorizationService -> CopyVectorsForChat -> Copied %d schema vectors from chat %s to %s",
			len(newSchemaPoints), sourceChatID, targetChatID)
	} else {
		log.Printf("VectorizationService -> CopyVectorsForChat -> No schema vectors to copy for chat %s", sourceChatID)
	}

	// --- 2. Copy message vectors (if requested) ---
	if copyMessages && len(messageIDMap) > 0 {
		msgFilter := []vectordb.FilterCondition{{Key: "chat_id", Value: sourceChatID}}
		sourceMsgPoints, err := v.vectorClient.ScrollByFilter(ctx, constants.MessageCollectionName, msgFilter, true)
		if err != nil {
			log.Printf("VectorizationService -> CopyVectorsForChat -> Warning: failed to scroll message vectors: %v", err)
			// Non-fatal — schema vectors are the priority
		} else if len(sourceMsgPoints) > 0 {
			newMsgPoints := make([]vectordb.VectorPoint, 0, len(sourceMsgPoints))
			for _, pt := range sourceMsgPoints {
				if len(pt.Vector) == 0 {
					continue
				}

				oldMsgID, _ := pt.Payload["message_id"].(string)
				newMsgID, exists := messageIDMap[oldMsgID]
				if !exists {
					continue // message was not duplicated, skip its vector
				}

				// Clone payload
				newPayload := make(map[string]interface{}, len(pt.Payload))
				for k, val := range pt.Payload {
					newPayload[k] = val
				}
				newPayload["chat_id"] = targetChatID
				newPayload["message_id"] = newMsgID

				newPointID := vectordb.PointID(fmt.Sprintf("msg_%s_%s", targetChatID, newMsgID))

				newMsgPoints = append(newMsgPoints, vectordb.VectorPoint{
					ID:      newPointID,
					Vector:  pt.Vector,
					Payload: newPayload,
				})
			}

			if len(newMsgPoints) > 0 {
				if err := v.vectorClient.Upsert(ctx, constants.MessageCollectionName, newMsgPoints); err != nil {
					log.Printf("VectorizationService -> CopyVectorsForChat -> Warning: failed to upsert message vectors: %v", err)
				} else {
					log.Printf("VectorizationService -> CopyVectorsForChat -> Copied %d message vectors from chat %s to %s",
						len(newMsgPoints), sourceChatID, targetChatID)
				}
			}
		}
	}

	log.Printf("VectorizationService -> CopyVectorsForChat -> Completed in %v (source=%s, target=%s)",
		time.Since(start), sourceChatID, targetChatID)
	return nil
}

// SearchSchema performs similarity search across schema + KB vectors for a chat.
// Logs detailed results (table names, types, scores) for manual quality assessment.
func (v *vectorizationService) SearchSchema(ctx context.Context, chatID string, query string, topK int) ([]vectordb.SearchResult, error) {
	if topK <= 0 {
		topK = constants.DefaultTopK
	}

	log.Printf("VectorizationService -> SearchSchema -> chat=%s | topK=%d | query=\"%s\"", chatID, topK, truncateForLog(query, 200))

	// Embed the query
	queryVector, err := v.embeddingProvider.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Search the schema collection with chat_id filter
	results, err := v.vectorClient.Search(ctx, constants.SchemaCollectionName, vectordb.SearchRequest{
		Vector: queryVector,
		Filter: map[string]string{
			"chat_id": chatID,
		},
		TopK:           topK,
		ScoreThreshold: float32(constants.DefaultScoreThreshold),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search schema: %w", err)
	}

	// --- Detailed logging for quality assessment ---
	if len(results) == 0 {
		log.Printf("VectorizationService -> SearchSchema -> chat=%s | NO RESULTS (threshold=%.2f) for query: \"%s\"",
			chatID, constants.DefaultScoreThreshold, truncateForLog(query, 200))
	} else {
		var summary strings.Builder
		summary.WriteString(fmt.Sprintf("VectorizationService -> SearchSchema -> chat=%s | %d results (threshold=%.2f):\n",
			chatID, len(results), float64(constants.DefaultScoreThreshold)))
		for i, r := range results {
			tableName, _ := r.Payload["table_name"].(string)
			vecType, _ := r.Payload["type"].(string)
			dbType, _ := r.Payload["db_type"].(string)
			cols, _ := r.Payload["column_names"].(string)
			rowCount := 0
			if rc, ok := r.Payload["row_count"]; ok {
				switch v := rc.(type) {
				case float64:
					rowCount = int(v)
				case int:
					rowCount = v
				case int64:
					rowCount = int(v)
				}
			}
			contentPreview := ""
			if c, ok := r.Payload["content"].(string); ok {
				contentPreview = truncateForLog(c, 150)
			}
			summary.WriteString(fmt.Sprintf("  [%d] score=%.4f | type=%-14s | table=%-30s | db=%-12s | rows=%-5d | id=%s\n",
				i+1, r.Score, vecType, tableName, dbType, rowCount, r.ID))
			if cols != "" {
				summary.WriteString(fmt.Sprintf("       cols=%s\n", truncateForLog(cols, 120)))
			}
			if contentPreview != "" {
				summary.WriteString(fmt.Sprintf("       content=%s\n", contentPreview))
			}
		}
		log.Print(summary.String())
	}

	return results, nil
}

// HasSchemaVectors returns true if at least one schema vector exists for this chat.
func (v *vectorizationService) HasSchemaVectors(ctx context.Context, chatID string) bool {
	count, err := v.vectorClient.Count(ctx, constants.SchemaCollectionName, []vectordb.FilterCondition{
		{Key: "chat_id", Value: chatID},
		{Key: "type", Value: "schema"},
	})
	if err != nil {
		log.Printf("VectorizationService -> HasSchemaVectors -> Error checking count for chat %s: %v", chatID, err)
		return false
	}
	return count > 0
}

// truncateForLog truncates a string to maxLen and appends "..." if needed.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// VectorizeMessage embeds a single chat message and upserts it to Qdrant.
// Each user/assistant message gets its own vector for conversational RAG retrieval.
func (v *vectorizationService) VectorizeMessage(ctx context.Context, chatID string, messageID string, role string, content string, messageIndex int) error {
	if content == "" {
		return nil
	}

	// Truncate very long messages to keep embedding quality high
	// Embedding models work best with focused text (< ~2000 tokens ≈ 8000 chars)
	embedText := content
	if len(embedText) > 8000 {
		embedText = embedText[:8000]
	}

	// Prefix with role for better semantic differentiation
	embedText = fmt.Sprintf("%s: %s", role, embedText)

	vec, err := v.embeddingProvider.Embed(ctx, embedText)
	if err != nil {
		return fmt.Errorf("failed to embed message: %w", err)
	}

	pointID := vectordb.PointID(fmt.Sprintf("msg_%s_%s", chatID, messageID))

	point := vectordb.VectorPoint{
		ID:     pointID,
		Vector: vec,
		Payload: map[string]interface{}{
			"chat_id":       chatID,
			"type":          "message",
			"message_id":    messageID,
			"role":          role,
			"content":       content,
			"message_index": messageIndex,
		},
	}

	if err := v.vectorClient.Upsert(ctx, constants.MessageCollectionName, []vectordb.VectorPoint{point}); err != nil {
		return fmt.Errorf("failed to upsert message vector: %w", err)
	}

	log.Printf("VectorizationService -> VectorizeMessage -> Stored message vector for chat %s, msg %s (role=%s, idx=%d)",
		chatID, messageID, role, messageIndex)

	return nil
}

// SearchMessages performs similarity search over vectorized messages for a chat.
// Results are filtered to type="message" and exclude messages whose message_id is in excludeMessageIDs.
func (v *vectorizationService) SearchMessages(ctx context.Context, chatID string, query string, topK int, excludeMessageIDs []string) ([]vectordb.SearchResult, error) {
	if topK <= 0 {
		topK = 5 // default for message retrieval
	}

	queryVector, err := v.embeddingProvider.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed message search query: %w", err)
	}

	// Search the dedicated message collection — no type filter needed since this collection only has messages
	results, err := v.vectorClient.Search(ctx, constants.MessageCollectionName, vectordb.SearchRequest{
		Vector: queryVector,
		Filter: map[string]string{
			"chat_id": chatID,
		},
		TopK:           topK + len(excludeMessageIDs), // fetch extra to compensate for exclusions
		ScoreThreshold: float32(constants.MessageScoreThreshold),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}

	// Build exclusion set
	excludeSet := make(map[string]struct{}, len(excludeMessageIDs))
	for _, id := range excludeMessageIDs {
		excludeSet[id] = struct{}{}
	}

	// Filter out excluded messages and cap at topK
	filtered := make([]vectordb.SearchResult, 0, topK)
	for _, r := range results {
		if msgID, ok := r.Payload["message_id"].(string); ok {
			if _, excluded := excludeSet[msgID]; excluded {
				continue
			}
		}
		filtered = append(filtered, r)
		if len(filtered) >= topK {
			break
		}
	}

	log.Printf("VectorizationService -> SearchMessages -> chat %s: %d results (from %d raw, %d excluded)",
		chatID, len(filtered), len(results), len(excludeMessageIDs))

	return filtered, nil
}

// SchemaInfoToChunks converts a dbmanager.SchemaInfo into vectorization-ready SchemaChunks.
// It produces DB-type-aware text so embeddings capture the right semantics for each engine.
func SchemaInfoToChunks(tables map[string]SchemaTable, dbType string) []SchemaChunk {
	return BuildEnrichedSchemaChunks(tables, dbType, nil)
}

// BuildEnrichedSchemaChunks converts tables into SchemaChunks, optionally enriching each chunk
// with KB descriptions and example records from the enrichment map.
// If enrichments is nil, produces the same output as SchemaInfoToChunks.
func BuildEnrichedSchemaChunks(tables map[string]SchemaTable, dbType string, enrichments map[string]*TableEnrichment) []SchemaChunk {
	chunks := make([]SchemaChunk, 0, len(tables))

	for tableName, table := range tables {
		var enrichment *TableEnrichment
		if enrichments != nil {
			enrichment = enrichments[tableName]
		}
		content := buildSchemaChunkText(tableName, table, dbType, enrichment)

		columnNames := make([]string, 0, len(table.Columns))
		for _, col := range table.Columns {
			columnNames = append(columnNames, col.Name)
		}

		fkTables := make([]string, 0)
		for _, fk := range table.ForeignKeys {
			fkTables = append(fkTables, fk.RefTable)
		}

		chunks = append(chunks, SchemaChunk{
			TableName:   tableName,
			Content:     content,
			ColumnNames: columnNames,
			ForeignKeys: fkTables,
			RowCount:    table.RowCount,
			Metadata: map[string]string{
				"db_type": dbType,
			},
		})
	}

	return chunks
}

// dbTerminology holds DB-engine-specific labels used in schema chunk text.
type dbTerminology struct {
	EntityLabel string // "Table", "Collection", "Sheet", etc.
	CountLabel  string // "rows", "documents", etc.
	FieldLabel  string // "Columns", "Document Fields", etc.
	EngineNote  string // optional one-liner appended after the header, e.g. "schema inferred from document sampling"
}

// getDBTerminology returns the right vocabulary for a given database type.
func getDBTerminology(dbType string) dbTerminology {
	switch dbType {
	case constants.DatabaseTypeMongoDB:
		return dbTerminology{
			EntityLabel: "Collection",
			CountLabel:  "documents",
			FieldLabel:  "Document Fields (inferred)",
			EngineNote:  "NoSQL document store — schema is flexible and inferred from sampled documents",
		}
	case constants.DatabaseTypeSpreadsheet:
		return dbTerminology{
			EntityLabel: "Sheet",
			CountLabel:  "rows",
			FieldLabel:  "Columns",
			EngineNote:  "Spreadsheet backed by PostgreSQL — use standard SQL syntax for queries",
		}
	case constants.DatabaseTypeGoogleSheets:
		return dbTerminology{
			EntityLabel: "Sheet",
			CountLabel:  "rows",
			FieldLabel:  "Columns",
			EngineNote:  "Google Sheets backed by PostgreSQL — use standard SQL syntax for queries",
		}
	case constants.DatabaseTypeRedis:
		return dbTerminology{
			EntityLabel: "Key Pattern",
			CountLabel:  "keys",
			FieldLabel:  "Fields",
			EngineNote:  "Redis key-value / data-structure store",
		}
	case constants.DatabaseTypeClickhouse:
		return dbTerminology{
			EntityLabel: "Table",
			CountLabel:  "rows",
			FieldLabel:  "Columns",
			EngineNote:  "ClickHouse columnar OLAP database — optimised for analytical queries",
		}
	case constants.DatabaseTypeCassandra:
		return dbTerminology{
			EntityLabel: "Table",
			CountLabel:  "rows",
			FieldLabel:  "Columns",
			EngineNote:  "Cassandra wide-column store — queries must include partition key",
		}
	case constants.DatabaseTypeNeo4j:
		return dbTerminology{
			EntityLabel: "Node Label",
			CountLabel:  "nodes",
			FieldLabel:  "Properties",
			EngineNote:  "Neo4j graph database — use Cypher query language",
		}
	default:
		// PostgreSQL, MySQL, YugabyteDB, and any unknown → standard SQL
		return dbTerminology{
			EntityLabel: "Table",
			CountLabel:  "rows",
			FieldLabel:  "Columns",
		}
	}
}

// buildSchemaChunkText produces a human-readable, embedding-friendly text block
// for a single table/collection/sheet using DB-aware terminology.
// If enrichment is non-nil, KB descriptions and example records are woven into the text.
func buildSchemaChunkText(tableName string, table SchemaTable, dbType string, enrichment *TableEnrichment) string {
	t := getDBTerminology(dbType)
	var sb strings.Builder

	// Header: entity label + name + row/doc count
	sb.WriteString(fmt.Sprintf("%s: %s", t.EntityLabel, tableName))
	if table.RowCount > 0 {
		sb.WriteString(fmt.Sprintf(" (%d %s)", table.RowCount, t.CountLabel))
	}
	sb.WriteString("\n")

	// Engine note (if any)
	if t.EngineNote != "" {
		sb.WriteString(fmt.Sprintf("[%s]\n", t.EngineNote))
	}

	// Description / comment — prefer KB description, fall back to schema comment
	if enrichment != nil && enrichment.TableDescription != "" {
		sb.WriteString(fmt.Sprintf("Purpose: %s\n", enrichment.TableDescription))
	}
	if table.Comment != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", table.Comment))
	}

	// Build a quick lookup for field descriptions from enrichment
	var fieldDescs map[string]string
	if enrichment != nil {
		fieldDescs = enrichment.FieldDescriptions
	}

	// Fields / Columns
	sb.WriteString(fmt.Sprintf("%s:\n", t.FieldLabel))
	for _, col := range table.Columns {
		sb.WriteString(fmt.Sprintf("  - %s (%s", col.Name, col.Type))

		// MongoDB: nullable means "field not always present", so skip NOT NULL noise
		if dbType != constants.DatabaseTypeMongoDB {
			if !col.IsNullable {
				sb.WriteString(", NOT NULL")
			}
		}
		if col.IsPrimaryKey {
			switch dbType {
			case constants.DatabaseTypeMongoDB:
				sb.WriteString(", _id")
			case constants.DatabaseTypeCassandra:
				sb.WriteString(", PARTITION KEY")
			default:
				sb.WriteString(", PK")
			}
		}
		if col.DefaultValue != "" {
			sb.WriteString(fmt.Sprintf(", default: %s", col.DefaultValue))
		}
		if col.Comment != "" {
			sb.WriteString(fmt.Sprintf(" — %s", col.Comment))
		}
		sb.WriteString(")")

		// Append KB field description inline if available
		if fieldDescs != nil {
			if desc, ok := fieldDescs[col.Name]; ok && desc != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", desc))
			}
		}
		sb.WriteString("\n")
	}

	// Indexes
	if len(table.Indexes) > 0 {
		sb.WriteString("Indexes:\n")
		for _, idx := range table.Indexes {
			uniqueStr := ""
			if idx.IsUnique {
				uniqueStr = "UNIQUE "
			}
			sb.WriteString(fmt.Sprintf("  - %s%s (%s)\n", uniqueStr, idx.Name, strings.Join(idx.Columns, ", ")))
		}
	}

	// Foreign keys — only relevant for relational / SQL-backed engines
	if len(table.ForeignKeys) > 0 {
		switch dbType {
		case constants.DatabaseTypeMongoDB, constants.DatabaseTypeRedis, constants.DatabaseTypeNeo4j:
			// These engines don't have FK constraints — skip section
		default:
			sb.WriteString("Foreign Keys:\n")
			for _, fk := range table.ForeignKeys {
				sb.WriteString(fmt.Sprintf("  - %s → %s.%s\n", fk.ColumnName, fk.RefTable, fk.RefColumn))
			}
		}
	}

	// Example records from the live database (enrichment)
	// Cap at 3 records and truncate each to 500 chars to prevent enormous chunks
	// (e.g. collections that embed the full schema in nested fields).
	if enrichment != nil && len(enrichment.ExampleRecords) > 0 {
		const maxExampleRecords = 3
		const maxRecordChars = 500
		sb.WriteString("Example Records:\n")
		count := len(enrichment.ExampleRecords)
		if count > maxExampleRecords {
			count = maxExampleRecords
		}
		for i := 0; i < count; i++ {
			recJSON, err := json.Marshal(enrichment.ExampleRecords[i])
			if err != nil {
				continue
			}
			recStr := string(recJSON)
			if len(recStr) > maxRecordChars {
				recStr = recStr[:maxRecordChars] + "...(truncated)"
			}
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, recStr))
		}
		if len(enrichment.ExampleRecords) > maxExampleRecords {
			sb.WriteString(fmt.Sprintf("  ... and %d more records (omitted for brevity)\n", len(enrichment.ExampleRecords)-maxExampleRecords))
		}
	}

	return sb.String()
}

// SchemaTable is a simplified representation of a database table used for building schema chunks.
// This avoids a direct dependency on the dbmanager package from the services package.
type SchemaTable struct {
	Name        string
	Comment     string
	RowCount    int64
	Columns     []SchemaColumn
	Indexes     []SchemaIndex
	ForeignKeys []SchemaFK
}

// SchemaColumn is a simplified column representation.
type SchemaColumn struct {
	Name         string
	Type         string
	IsNullable   bool
	IsPrimaryKey bool
	DefaultValue string
	Comment      string
}

// SchemaIndex is a simplified index representation.
type SchemaIndex struct {
	Name     string
	Columns  []string
	IsUnique bool
}

// SchemaFK is a simplified foreign key representation.
type SchemaFK struct {
	ColumnName string
	RefTable   string
	RefColumn  string
}
