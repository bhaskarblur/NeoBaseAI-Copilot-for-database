package vectordb

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"time"

	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// QdrantConfig holds configuration for connecting to a Qdrant instance.
type QdrantConfig struct {
	Host   string
	Port   string
	APIKey string
	UseTLS bool
}

// QdrantClient implements the Client interface using Qdrant's gRPC API.
type QdrantClient struct {
	conn              *grpc.ClientConn
	pointsClient      pb.PointsClient
	collectionsClient pb.CollectionsClient
	qdrantClient      pb.QdrantClient
}

// NewQdrantClient creates a new Qdrant gRPC client.
func NewQdrantClient(cfg QdrantConfig) (*QdrantClient, error) {
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	if cfg.UseTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if cfg.APIKey != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(&apiKeyCredentials{apiKey: cfg.APIKey}))
	}

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant at %s: %w", addr, err)
	}

	client := &QdrantClient{
		conn:              conn,
		pointsClient:      pb.NewPointsClient(conn),
		collectionsClient: pb.NewCollectionsClient(conn),
		qdrantClient:      pb.NewQdrantClient(conn),
	}

	log.Printf("VectorDB -> Qdrant client created for %s (TLS: %v)", addr, cfg.UseTLS)

	return client, nil
}

// IsHealthy checks if Qdrant is reachable.
func (q *QdrantClient) IsHealthy(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := q.qdrantClient.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		log.Printf("VectorDB -> Qdrant health check failed: %v", err)
		return false
	}
	return true
}

// EnsureCollection creates a collection if it doesn't already exist, with the given payload indexes.
func (q *QdrantClient) EnsureCollection(ctx context.Context, collection string, dimension int) error {
	return q.ensureCollectionWithIndexes(ctx, collection, dimension, nil)
}

// EnsureSchemaCollection creates the schema collection with schema-specific payload indexes,
// including a full-text index on the "content" field for hybrid search.
func (q *QdrantClient) EnsureSchemaCollection(ctx context.Context, dimension int) error {
	indexes := []string{"chat_id", "type", "table_name"}
	if err := q.ensureCollectionWithIndexes(ctx, constants.SchemaCollectionName, dimension, indexes); err != nil {
		return err
	}
	// Create full-text index on "content" for hybrid keyword search (idempotent)
	if err := q.ensureTextIndex(ctx, constants.SchemaCollectionName, "content"); err != nil {
		log.Printf("VectorDB -> Warning: failed to create text index on 'content' in schema collection: %v", err)
	}
	// Create full-text index on "table_name" for direct table name matching
	if err := q.ensureTextIndex(ctx, constants.SchemaCollectionName, "table_name"); err != nil {
		log.Printf("VectorDB -> Warning: failed to create text index on 'table_name' in schema collection: %v", err)
	}
	return nil
}

// EnsureMessageCollection creates the message collection with message-specific payload indexes.
func (q *QdrantClient) EnsureMessageCollection(ctx context.Context, dimension int) error {
	indexes := []string{"chat_id", "message_id", "role"}
	return q.ensureCollectionWithIndexes(ctx, constants.MessageCollectionName, dimension, indexes)
}

// ensureCollectionWithIndexes is a reusable helper that creates a collection (if absent)
// and ensures the given keyword payload indexes exist.
func (q *QdrantClient) ensureCollectionWithIndexes(ctx context.Context, collection string, dimension int, indexFields []string) error {
	// Check if collection exists
	listResp, err := q.collectionsClient.List(ctx, &pb.ListCollectionsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	exists := false
	for _, col := range listResp.GetCollections() {
		if col.GetName() == collection {
			exists = true
			break
		}
	}

	if !exists {
		// Create collection with cosine distance
		dim := uint64(dimension)
		_, err = q.collectionsClient.Create(ctx, &pb.CreateCollection{
			CollectionName: collection,
			VectorsConfig: &pb.VectorsConfig{
				Config: &pb.VectorsConfig_Params{
					Params: &pb.VectorParams{
						Size:     dim,
						Distance: pb.Distance_Cosine,
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create collection '%s': %w", collection, err)
		}
		log.Printf("VectorDB -> Created collection '%s' with dimension %d", collection, dimension)
	} else {
		log.Printf("VectorDB -> Collection '%s' already exists", collection)
	}

	// Create payload indexes (idempotent — Qdrant ignores duplicates)
	boolTrue := true
	for _, field := range indexFields {
		_, indexErr := q.pointsClient.CreateFieldIndex(ctx, &pb.CreateFieldIndexCollection{
			CollectionName: collection,
			FieldName:      field,
			FieldType:      pb.FieldType_FieldTypeKeyword.Enum(),
			Wait:           &boolTrue,
		})
		if indexErr != nil {
			log.Printf("VectorDB -> Warning: failed to create '%s' index on '%s': %v", field, collection, indexErr)
		} else {
			log.Printf("VectorDB -> Ensured payload index '%s' on collection '%s'", field, collection)
		}
	}

	return nil
}

// ensureTextIndex creates a full-text payload index on a field (idempotent).
// Uses word tokenizer with lowercase for best keyword matching.
func (q *QdrantClient) ensureTextIndex(ctx context.Context, collection string, fieldName string) error {
	boolTrue := true
	lowercase := true
	fieldType := pb.FieldType_FieldTypeText

	_, err := q.pointsClient.CreateFieldIndex(ctx, &pb.CreateFieldIndexCollection{
		CollectionName: collection,
		FieldName:      fieldName,
		FieldType:      &fieldType,
		Wait:           &boolTrue,
		FieldIndexParams: &pb.PayloadIndexParams{
			IndexParams: &pb.PayloadIndexParams_TextIndexParams{
				TextIndexParams: &pb.TextIndexParams{
					Tokenizer: pb.TokenizerType_Word,
					Lowercase: &lowercase,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create text index '%s' on '%s': %w", fieldName, collection, err)
	}
	log.Printf("VectorDB -> Ensured text index '%s' on collection '%s'", fieldName, collection)
	return nil
}

// Upsert inserts or updates vector points in a collection.
func (q *QdrantClient) Upsert(ctx context.Context, collection string, points []VectorPoint) error {
	if len(points) == 0 {
		return nil
	}

	pbPoints := make([]*pb.PointStruct, 0, len(points))
	for _, pt := range points {
		// Convert payload to Qdrant Value format
		payload := make(map[string]*pb.Value)
		for k, v := range pt.Payload {
			payload[k] = toQdrantValue(v)
		}

		pbPoints = append(pbPoints, &pb.PointStruct{
			Id: &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{
					Uuid: pointIDToUUID(pt.ID),
				},
			},
			Vectors: &pb.Vectors{
				VectorsOptions: &pb.Vectors_Vector{
					Vector: &pb.Vector{
						Data: pt.Vector,
					},
				},
			},
			Payload: payload,
		})
	}

	boolTrue := true
	_, err := q.pointsClient.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: collection,
		Points:         pbPoints,
		Wait:           &boolTrue,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert %d points: %w", len(points), err)
	}

	return nil
}

// Search performs similarity search within a collection.
func (q *QdrantClient) Search(ctx context.Context, collection string, req SearchRequest) ([]SearchResult, error) {
	topK := uint64(req.TopK)
	if topK == 0 {
		topK = uint64(constants.DefaultTopK)
	}

	searchReq := &pb.SearchPoints{
		CollectionName: collection,
		Vector:         req.Vector,
		Limit:          topK,
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
		ScoreThreshold: &req.ScoreThreshold,
	}

	// Build filter from key-value pairs
	if len(req.Filter) > 0 {
		conditions := make([]*pb.Condition, 0, len(req.Filter))
		for key, value := range req.Filter {
			conditions = append(conditions, &pb.Condition{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key: key,
						Match: &pb.Match{
							MatchValue: &pb.Match_Keyword{
								Keyword: value,
							},
						},
					},
				},
			})
		}
		searchReq.Filter = &pb.Filter{
			Must: conditions,
		}
	}

	resp, err := q.pointsClient.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("qdrant search error: %w", err)
	}

	results := make([]SearchResult, 0, len(resp.GetResult()))
	for _, r := range resp.GetResult() {
		payload := make(map[string]interface{})
		for k, v := range r.GetPayload() {
			payload[k] = fromQdrantValue(v)
		}

		pointID := ""
		if r.GetId().GetUuid() != "" {
			pointID = r.GetId().GetUuid()
		}

		results = append(results, SearchResult{
			ID:      pointID,
			Score:   r.GetScore(),
			Payload: payload,
		})
	}

	return results, nil
}

// HybridSearch performs a server-side hybrid search using Qdrant's Query API.
// It runs two independent retrieval legs and fuses them via Reciprocal Rank Fusion (RRF):
//
//	Leg 1 (Semantic): Dense vector similarity search with score threshold.
//	Leg 2 (Keyword):  Dense vector search constrained by a full-text match filter on the text field.
//
// Both legs run inside Qdrant — zero application-side post-processing.
// Points that appear in BOTH legs get a significant RRF score boost.
func (q *QdrantClient) HybridSearch(ctx context.Context, collection string, req HybridSearchRequest) ([]SearchResult, error) {
	topK := uint64(req.TopK)
	if topK == 0 {
		topK = uint64(constants.DefaultTopK)
	}

	// --- Build shared filter (e.g., chat_id) ---
	var sharedConditions []*pb.Condition
	for key, value := range req.Filter {
		sharedConditions = append(sharedConditions, &pb.Condition{
			ConditionOneOf: &pb.Condition_Field{
				Field: &pb.FieldCondition{
					Key: key,
					Match: &pb.Match{
						MatchValue: &pb.Match_Keyword{
							Keyword: value,
						},
					},
				},
			},
		})
	}

	var sharedFilter *pb.Filter
	if len(sharedConditions) > 0 {
		sharedFilter = &pb.Filter{Must: sharedConditions}
	}

	// --- Prefetch Leg 1: Pure semantic vector search (broad) ---
	prefetchLimit := topK * 3 // fetch more candidates for better fusion
	semanticPrefetch := &pb.PrefetchQuery{
		Query:          pb.NewQueryDense(req.Vector),
		Filter:         sharedFilter,
		Limit:          &prefetchLimit,
		ScoreThreshold: &req.ScoreThreshold,
	}

	prefetches := []*pb.PrefetchQuery{semanticPrefetch}

	// --- Prefetch Leg 2: Keyword-constrained vector search ---
	if req.TextQuery != "" && req.TextField != "" {
		// Build a filter that requires full-text match AND the shared filter conditions
		textConditions := make([]*pb.Condition, 0, len(sharedConditions)+1)
		textConditions = append(textConditions, sharedConditions...)
		textConditions = append(textConditions, pb.NewMatchText(req.TextField, req.TextQuery))

		textFilter := &pb.Filter{Must: textConditions}

		keywordPrefetch := &pb.PrefetchQuery{
			Query:  pb.NewQueryDense(req.Vector),
			Filter: textFilter,
			Limit:  &prefetchLimit,
			// No score threshold for keyword leg — if the text matches, it's relevant
		}
		prefetches = append(prefetches, keywordPrefetch)
	}

	// --- Additional text-matching legs (e.g., table_name matching) ---
	// Each extra leg creates a prefetch that requires a full-text match on a different field.
	// When a table name appears directly in the query, this gives it a strong RRF boost
	// (appearing in 3+ fusion inputs instead of just 1-2).
	for _, leg := range req.ExtraTextLegs {
		if leg.Query == "" || leg.Field == "" {
			continue
		}
		legConditions := make([]*pb.Condition, 0, len(sharedConditions)+1)
		legConditions = append(legConditions, sharedConditions...)
		legConditions = append(legConditions, pb.NewMatchText(leg.Field, leg.Query))

		legFilter := &pb.Filter{Must: legConditions}

		legPrefetch := &pb.PrefetchQuery{
			Query:  pb.NewQueryDense(req.Vector),
			Filter: legFilter,
			Limit:  &prefetchLimit,
			// No score threshold — if the text matches, the item is relevant
		}
		prefetches = append(prefetches, legPrefetch)
	}

	// --- Top-level: RRF Fusion ---
	queryReq := &pb.QueryPoints{
		CollectionName: collection,
		Prefetch:       prefetches,
		Query:          pb.NewQueryFusion(pb.Fusion_RRF),
		Limit:          &topK,
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
	}

	resp, err := q.pointsClient.Query(ctx, queryReq)
	if err != nil {
		return nil, fmt.Errorf("qdrant hybrid search error: %w", err)
	}

	results := make([]SearchResult, 0, len(resp.GetResult()))
	for _, r := range resp.GetResult() {
		payload := make(map[string]interface{})
		for k, v := range r.GetPayload() {
			payload[k] = fromQdrantValue(v)
		}

		pointID := ""
		if r.GetId().GetUuid() != "" {
			pointID = r.GetId().GetUuid()
		}

		results = append(results, SearchResult{
			ID:      pointID,
			Score:   r.GetScore(),
			Payload: payload,
		})
	}

	return results, nil
}

// Delete removes points by their IDs from a collection.
func (q *QdrantClient) Delete(ctx context.Context, collection string, ids []PointID) error {
	if len(ids) == 0 {
		return nil
	}

	pbIDs := make([]*pb.PointId, 0, len(ids))
	for _, id := range ids {
		pbIDs = append(pbIDs, &pb.PointId{
			PointIdOptions: &pb.PointId_Uuid{
				Uuid: pointIDToUUID(id),
			},
		})
	}

	boolTrue := true
	_, err := q.pointsClient.Delete(ctx, &pb.DeletePoints{
		CollectionName: collection,
		Wait:           &boolTrue,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{
					Ids: pbIDs,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete points: %w", err)
	}

	return nil
}

// DeleteByFilter removes all points matching the filter conditions.
func (q *QdrantClient) DeleteByFilter(ctx context.Context, collection string, filters []FilterCondition) error {
	if len(filters) == 0 {
		return fmt.Errorf("at least one filter condition is required for DeleteByFilter")
	}

	conditions := make([]*pb.Condition, 0, len(filters))
	for _, f := range filters {
		conditions = append(conditions, &pb.Condition{
			ConditionOneOf: &pb.Condition_Field{
				Field: &pb.FieldCondition{
					Key: f.Key,
					Match: &pb.Match{
						MatchValue: &pb.Match_Keyword{
							Keyword: f.Value,
						},
					},
				},
			},
		})
	}

	boolTrue := true
	_, err := q.pointsClient.Delete(ctx, &pb.DeletePoints{
		CollectionName: collection,
		Wait:           &boolTrue,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Filter{
				Filter: &pb.Filter{
					Must: conditions,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete by filter: %w", err)
	}

	return nil
}

// Count returns the number of points matching the filter in a collection.
func (q *QdrantClient) Count(ctx context.Context, collection string, filters []FilterCondition) (int64, error) {
	exact := true
	countReq := &pb.CountPoints{
		CollectionName: collection,
		Exact:          &exact,
	}

	if len(filters) > 0 {
		conditions := make([]*pb.Condition, 0, len(filters))
		for _, f := range filters {
			conditions = append(conditions, &pb.Condition{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key: f.Key,
						Match: &pb.Match{
							MatchValue: &pb.Match_Keyword{
								Keyword: f.Value,
							},
						},
					},
				},
			})
		}
		countReq.Filter = &pb.Filter{
			Must: conditions,
		}
	}

	resp, err := q.pointsClient.Count(ctx, countReq)
	if err != nil {
		return 0, fmt.Errorf("failed to count points: %w", err)
	}

	return int64(resp.GetResult().GetCount()), nil
}

// ScrollByFilter retrieves all points matching the filter from a collection.
// When withVectors is true, the response includes the embedding vectors (needed for copying).
func (q *QdrantClient) ScrollByFilter(ctx context.Context, collection string, filters []FilterCondition, withVectors bool) ([]VectorPoint, error) {
	if len(filters) == 0 {
		return nil, fmt.Errorf("at least one filter condition is required for ScrollByFilter")
	}

	conditions := make([]*pb.Condition, 0, len(filters))
	for _, f := range filters {
		conditions = append(conditions, &pb.Condition{
			ConditionOneOf: &pb.Condition_Field{
				Field: &pb.FieldCondition{
					Key: f.Key,
					Match: &pb.Match{
						MatchValue: &pb.Match_Keyword{
							Keyword: f.Value,
						},
					},
				},
			},
		})
	}

	var allPoints []VectorPoint
	var offset *pb.PointId
	batchSize := uint32(100)

	for {
		scrollReq := &pb.ScrollPoints{
			CollectionName: collection,
			Filter:         &pb.Filter{Must: conditions},
			Limit:          &batchSize,
			WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
			WithVectors: &pb.WithVectorsSelector{
				SelectorOptions: &pb.WithVectorsSelector_Enable{Enable: withVectors},
			},
		}
		if offset != nil {
			scrollReq.Offset = offset
		}

		resp, err := q.pointsClient.Scroll(ctx, scrollReq)
		if err != nil {
			return nil, fmt.Errorf("qdrant scroll error: %w", err)
		}

		for _, r := range resp.GetResult() {
			payload := make(map[string]interface{})
			for k, v := range r.GetPayload() {
				payload[k] = fromQdrantValue(v)
			}

			pt := VectorPoint{
				Payload: payload,
			}

			// Extract point ID
			if r.GetId().GetUuid() != "" {
				pt.ID = PointID(r.GetId().GetUuid())
			}

			// Extract vector if requested
			if withVectors {
				if vec := r.GetVectors().GetVector(); vec != nil {
					pt.Vector = vec.GetData()
				}
			}

			allPoints = append(allPoints, pt)
		}

		// Check if there's a next page
		nextPage := resp.GetNextPageOffset()
		if nextPage == nil {
			break
		}
		offset = nextPage
	}

	return allPoints, nil
}

// Close cleans up the gRPC connection.
func (q *QdrantClient) Close() error {
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}
