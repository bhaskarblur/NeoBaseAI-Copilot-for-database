package repositories

import (
	"context"
	"errors"
	"log"
	"neobase-ai/internal/models"
	"neobase-ai/pkg/mongodb"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type IVisualizationRepository interface {
	CreateVisualization(ctx context.Context, visualization *models.MessageVisualization) error
	UpdateVisualization(ctx context.Context, id interface{}, visualization *models.MessageVisualization) error
	GetVisualizationByMessageID(ctx context.Context, messageID interface{}) (*models.MessageVisualization, error)
	GetVisualizationByQueryID(ctx context.Context, queryID interface{}) (*models.MessageVisualization, error) // Per-query visualization retrieval
	GetVisualizationByID(ctx context.Context, id interface{}) (*models.MessageVisualization, error)
	DeleteVisualization(ctx context.Context, id interface{}) error
	DeleteVisualizationsByMessageID(ctx context.Context, messageID interface{}) error
	DeleteVisualizationsByQueryID(ctx context.Context, queryID interface{}) error // Delete query-level visualizations
}

type VisualizationRepository struct {
	collection *mongo.Collection
}

func NewVisualizationRepository(mongoClient *mongodb.MongoDBClient) IVisualizationRepository {
	return &VisualizationRepository{
		collection: mongoClient.GetCollectionByName("message_visualizations"),
	}
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
	return err
}

func (r *VisualizationRepository) GetVisualizationByMessageID(ctx context.Context, messageID interface{}) (*models.MessageVisualization, error) {
	var visualization models.MessageVisualization
	filter := bson.M{"message_id": messageID}
	err := r.collection.FindOne(ctx, filter).Decode(&visualization)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return &visualization, nil
}

// GetVisualizationByQueryID retrieves the visualization for a specific query
// This enables per-query visualization retrieval for the 1:1 query-visualization relationship
func (r *VisualizationRepository) GetVisualizationByQueryID(ctx context.Context, queryIDOrVizID interface{}) (*models.MessageVisualization, error) {
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
	return &visualization, nil
}

func (r *VisualizationRepository) GetVisualizationByID(ctx context.Context, id interface{}) (*models.MessageVisualization, error) {
	var visualization models.MessageVisualization
	filter := bson.M{"_id": id}
	err := r.collection.FindOne(ctx, filter).Decode(&visualization)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return &visualization, nil
}

func (r *VisualizationRepository) DeleteVisualization(ctx context.Context, id interface{}) error {
	filter := bson.M{"_id": id}
	_, err := r.collection.DeleteOne(ctx, filter)
	return err
}

func (r *VisualizationRepository) DeleteVisualizationsByMessageID(ctx context.Context, messageID interface{}) error {
	filter := bson.M{"message_id": messageID}
	_, err := r.collection.DeleteMany(ctx, filter)
	return err
}

// DeleteVisualizationsByQueryID deletes all visualizations for a specific query
// Useful when cleaning up query-level visualizations
func (r *VisualizationRepository) DeleteVisualizationsByQueryID(ctx context.Context, queryID interface{}) error {
	filter := bson.M{"query_id": queryID}
	_, err := r.collection.DeleteMany(ctx, filter)
	return err
}
