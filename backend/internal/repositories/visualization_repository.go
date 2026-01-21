package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/pkg/mongodb"
	"neobase-ai/pkg/redis"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type IVisualizationRepository interface {
	CreateVisualization(ctx context.Context, visualization *models.MessageVisualization) error
	UpdateVisualization(ctx context.Context, id interface{}, visualization *models.MessageVisualization) error
	GetVisualizationByQueryID(ctx context.Context, queryID interface{}) (*models.MessageVisualization, error) // Per-query visualization retrieval
	GetVisualizationByID(ctx context.Context, id interface{}) (*models.MessageVisualization, error)
	DeleteVisualization(ctx context.Context, id interface{}) error
	DeleteVisualizationsByMessageID(ctx context.Context, messageID interface{}) error
	DeleteVisualizationsByQueryID(ctx context.Context, queryID interface{}) error // Delete query-level visualizations
}

type VisualizationRepository struct {
	collection *mongo.Collection
	redisRepo  redis.IRedisRepositories
}

func NewVisualizationRepository(mongoClient *mongodb.MongoDBClient, redisRepo redis.IRedisRepositories) IVisualizationRepository {
	log.Println("🚀 Initialized Repository : Visualization")
	return &VisualizationRepository{
		collection: mongoClient.GetCollectionByName("message_visualizations"),
		redisRepo:  redisRepo,
	}
}

// cacheVisualization stores a visualization in Redis with multiple keys for different access patterns
func (r *VisualizationRepository) cacheVisualization(viz *models.MessageVisualization) {
	if viz == nil {
		return
	}

	ctx := context.Background()
	vizData, err := json.Marshal(viz)
	if err != nil {
		log.Printf("[CACHE ERROR] Failed to marshal visualization - ID: %s, Error: %v", viz.ID.Hex(), err)
		return
	}

	// Cache with primary key (by visualization ID)
	primaryKey := fmt.Sprintf("visualization:id:%s", viz.ID.Hex())
	if err := r.redisRepo.SetCompressed(primaryKey, vizData, constants.MessageCacheTTL, ctx); err != nil {
		log.Printf("[CACHE ERROR] Failed to cache visualization by ID - Key: %s, Error: %v", primaryKey, err)
	} else {
		log.Printf("[CACHE WRITE SUCCESS] Cached visualization by ID - Key: %s", primaryKey)
	}

	// Cache by query ID (if exists) - for query-level lookup
	if viz.QueryID != nil && !viz.QueryID.IsZero() {
		queryKey := fmt.Sprintf("visualization:query:%s", viz.QueryID.Hex())
		if err := r.redisRepo.SetCompressed(queryKey, vizData, constants.MessageCacheTTL, ctx); err != nil {
			log.Printf("[CACHE ERROR] Failed to cache visualization by query - Key: %s", queryKey)
		} else {
			log.Printf("[CACHE WRITE SUCCESS] Cached visualization by query - Key: %s", queryKey)
		}
	}
}

// invalidateVisualizationCache removes all cache keys for a visualization
func (r *VisualizationRepository) invalidateVisualizationCache(viz *models.MessageVisualization) {
	if viz == nil {
		return
	}

	ctx := context.Background()

	// Invalidate primary key
	primaryKey := fmt.Sprintf("visualization:id:%s", viz.ID.Hex())
	r.redisRepo.Del(primaryKey, ctx)

	// Invalidate query key
	if viz.QueryID != nil && !viz.QueryID.IsZero() {
		queryKey := fmt.Sprintf("visualization:query:%s", viz.QueryID.Hex())
		r.redisRepo.Del(queryKey, ctx)
	}

	log.Printf("[CACHE INVALIDATE SUCCESS] Invalidated visualization cache - ID: %s", viz.ID.Hex())
}

// invalidateVisualizationByID removes cache without needing full object
func (r *VisualizationRepository) invalidateVisualizationByID(id primitive.ObjectID) {
	ctx := context.Background()
	primaryKey := fmt.Sprintf("visualization:id:%s", id.Hex())
	r.redisRepo.Del(primaryKey, ctx)
	log.Printf("[CACHE INVALIDATE] Invalidated visualization by ID - %s", id.Hex())
}

func (r *VisualizationRepository) CreateVisualization(ctx context.Context, visualization *models.MessageVisualization) error {
	log.Printf("CreateVisualization -> Saving visualization with ID: %s, CanVisualize: %v, ChartType: %s",
		visualization.ID.Hex(), visualization.CanVisualize, visualization.ChartType)
	result, err := r.collection.InsertOne(ctx, visualization)
	if err != nil {
		log.Printf("CreateVisualization -> Error inserting: %v", err)
		return err
	}
	log.Printf("CreateVisualization -> Successfully inserted with ID: %v", result.InsertedID)

	// Warm cache immediately after creation (write-through)
	go r.cacheVisualization(visualization)

	return nil
}

func (r *VisualizationRepository) UpdateVisualization(ctx context.Context, id interface{}, visualization *models.MessageVisualization) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"can_visualize":       visualization.CanVisualize,
			"reason":              visualization.Reason,
			"chart_type":          visualization.ChartType,
			"title":               visualization.Title,
			"description":         visualization.Description,
			"chart_config_json":   visualization.ChartConfigJSON,
			"optimized_query":     visualization.OptimizedQuery,
			"query_strategy":      visualization.QueryStrategy,
			"data_transformation": visualization.DataTransformation,
			"projected_row_count": visualization.ProjectedRowCount,
			"chart_height":        visualization.ChartHeight,
			"color_scheme":        visualization.ColorScheme,
			"data_density":        visualization.DataDensity,
			"x_axis_label":        visualization.XAxisLabel,
			"y_axis_label":        visualization.YAxisLabel,
			"generated_by":        visualization.GeneratedBy,
			"error":               visualization.Error,
			"updated_at":          visualization.UpdatedAt,
		},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)

	if err == nil {
		// Update cache with fresh data (write-through)
		go r.cacheVisualization(visualization)
	}

	return err
}

// GetVisualizationByQueryID retrieves the visualization for a specific query
// This enables per-query visualization retrieval for the 1:1 query-visualization relationship
func (r *VisualizationRepository) GetVisualizationByQueryID(ctx context.Context, queryIDOrVizID interface{}) (*models.MessageVisualization, error) {
	// Try cache first (by query ID or viz ID)
	if objID, ok := queryIDOrVizID.(primitive.ObjectID); ok {
		// Try as query ID first
		queryKey := fmt.Sprintf("visualization:query:%s", objID.Hex())
		cachedData, err := r.redisRepo.GetCompressed(queryKey, ctx)
		if err == nil && len(cachedData) > 0 {
			var visualization models.MessageVisualization
			if err := json.Unmarshal(cachedData, &visualization); err == nil {
				log.Printf("[CACHE HIT] Fetched visualization by query from cache - QueryID: %s", objID.Hex())
				return &visualization, nil
			}
		}

		// Try as visualization ID
		idKey := fmt.Sprintf("visualization:id:%s", objID.Hex())
		cachedData, err = r.redisRepo.GetCompressed(idKey, ctx)
		if err == nil && len(cachedData) > 0 {
			var visualization models.MessageVisualization
			if err := json.Unmarshal(cachedData, &visualization); err == nil {
				log.Printf("[CACHE HIT] Fetched visualization by ID from cache - VizID: %s", objID.Hex())
				return &visualization, nil
			}
		}
		log.Printf("[CACHE MISS] Visualization not in cache - ID: %s", objID.Hex())
	}

	// Fetch from DB
	var visualization models.MessageVisualization

	// Try first with visualization ID (_id)
	filter := bson.M{"_id": queryIDOrVizID}
	log.Printf("GetVisualizationByQueryID -> Attempting to fetch with ID filter: %v", filter)
	err := r.collection.FindOne(ctx, filter).Decode(&visualization)

	if err != nil {
		log.Printf("GetVisualizationByQueryID -> First attempt failed: %v", err)
		// If not found by ID, try by query_id for backward compatibility
		if errors.Is(err, mongo.ErrNoDocuments) {
			filter = bson.M{"query_id": queryIDOrVizID}
			log.Printf("GetVisualizationByQueryID -> Attempting fallback with query_id filter: %v", filter)
			err = r.collection.FindOne(ctx, filter).Decode(&visualization)

			if err != nil {
				log.Printf("GetVisualizationByQueryID -> Fallback also failed: %v", err)
				if errors.Is(err, mongo.ErrNoDocuments) {
					log.Printf("GetVisualizationByQueryID -> No document found for queryIDOrVizID: %v", queryIDOrVizID)
					return nil, nil
				}
				return nil, err
			}
		} else {
			log.Printf("GetVisualizationByQueryID -> Unexpected error: %v", err)
			return nil, err
		}
	}

	log.Printf("GetVisualizationByQueryID -> Successfully found visualization: ID=%s", visualization.ID.Hex())

	// Warm cache on DB hit
	go r.cacheVisualization(&visualization)

	return &visualization, nil
}

func (r *VisualizationRepository) GetVisualizationByID(ctx context.Context, id interface{}) (*models.MessageVisualization, error) {
	// Try cache first
	if objID, ok := id.(primitive.ObjectID); ok {
		cacheKey := fmt.Sprintf("visualization:id:%s", objID.Hex())
		cachedData, err := r.redisRepo.GetCompressed(cacheKey, ctx)
		if err == nil && len(cachedData) > 0 {
			var visualization models.MessageVisualization
			if err := json.Unmarshal(cachedData, &visualization); err == nil {
				log.Printf("[CACHE HIT] Fetched visualization by ID from cache - ID: %s", objID.Hex())
				return &visualization, nil
			}
		}
		log.Printf("[CACHE MISS] Visualization not in cache by ID - ID: %s", objID.Hex())
	}

	// Fetch from DB
	var visualization models.MessageVisualization
	filter := bson.M{"_id": id}
	err := r.collection.FindOne(ctx, filter).Decode(&visualization)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	// Warm cache on DB hit
	go r.cacheVisualization(&visualization)

	return &visualization, nil
}

func (r *VisualizationRepository) DeleteVisualization(ctx context.Context, id interface{}) error {
	// Fetch visualization to get all IDs before deleting (for cache invalidation)
	var viz models.MessageVisualization
	filter := bson.M{"_id": id}
	err := r.collection.FindOne(ctx, filter).Decode(&viz)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}

	// Delete from DB
	_, err = r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	// Invalidate cache
	if !viz.ID.IsZero() {
		go r.invalidateVisualizationCache(&viz)
	} else if objID, ok := id.(primitive.ObjectID); ok {
		go r.invalidateVisualizationByID(objID)
	}

	return nil
}

func (r *VisualizationRepository) DeleteVisualizationsByMessageID(ctx context.Context, messageID interface{}) error {
	// Fetch visualizations before deleting (for cache invalidation)
	filter := bson.M{"message_id": messageID}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var visualizations []models.MessageVisualization
	if err := cursor.All(ctx, &visualizations); err != nil {
		return err
	}

	// Delete from DB
	_, err = r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	// Invalidate cache for each visualization
	for _, viz := range visualizations {
		vizCopy := viz
		go r.invalidateVisualizationCache(&vizCopy)
	}

	return nil
}

// DeleteVisualizationsByQueryID deletes all visualizations for a specific query
// Useful when cleaning up query-level visualizations
func (r *VisualizationRepository) DeleteVisualizationsByQueryID(ctx context.Context, queryID interface{}) error {
	// Fetch visualizations before deleting (for cache invalidation)
	filter := bson.M{"query_id": queryID}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var visualizations []models.MessageVisualization
	if err := cursor.All(ctx, &visualizations); err != nil {
		return err
	}

	// Delete from DB
	_, err = r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	// Invalidate cache for each visualization
	for _, viz := range visualizations {
		vizCopy := viz
		go r.invalidateVisualizationCache(&vizCopy)
	}

	return nil
}
