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

// EnsureSchemaCollection creates the schema collection with schema-specific payload indexes.
func (q *QdrantClient) EnsureSchemaCollection(ctx context.Context, dimension int) error {
	indexes := []string{"chat_id", "type", "table_name"}
	return q.ensureCollectionWithIndexes(ctx, constants.SchemaCollectionName, dimension, indexes)
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

// Close cleans up the gRPC connection.
func (q *QdrantClient) Close() error {
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}
