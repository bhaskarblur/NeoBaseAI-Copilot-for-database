package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/pkg/mongodb"
	"neobase-ai/pkg/redis"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DashboardRepository handles CRUD operations for dashboards and widgets
type DashboardRepository interface {
	// Dashboard CRUD
	CreateDashboard(ctx context.Context, dashboard *models.Dashboard) error
	UpdateDashboard(ctx context.Context, id primitive.ObjectID, dashboard *models.Dashboard) error
	DeleteDashboard(ctx context.Context, id primitive.ObjectID) error
	FindDashboardByID(ctx context.Context, id primitive.ObjectID) (*models.Dashboard, error)
	FindDashboardsByChatID(ctx context.Context, chatID primitive.ObjectID) ([]*models.Dashboard, error)
	CountDashboardsByChatID(ctx context.Context, chatID primitive.ObjectID) (int64, error)
	SetDefaultDashboard(ctx context.Context, chatID, dashboardID primitive.ObjectID) error

	// Widget CRUD
	CreateWidget(ctx context.Context, widget *models.Widget) error
	CreateWidgets(ctx context.Context, widgets []*models.Widget) error
	UpdateWidget(ctx context.Context, id primitive.ObjectID, widget *models.Widget) error
	DeleteWidget(ctx context.Context, id primitive.ObjectID) error
	DeleteWidgetsByDashboardID(ctx context.Context, dashboardID primitive.ObjectID) error
	FindWidgetByID(ctx context.Context, id primitive.ObjectID) (*models.Widget, error)
	FindWidgetsByDashboardID(ctx context.Context, dashboardID primitive.ObjectID) ([]*models.Widget, error)
	CountWidgetsByDashboardID(ctx context.Context, dashboardID primitive.ObjectID) (int64, error)
}

type dashboardRepository struct {
	dashboardCollection *mongo.Collection
	widgetCollection    *mongo.Collection
	redisRepo           redis.IRedisRepositories
}

func NewDashboardRepository(mongoClient *mongodb.MongoDBClient, redisRepo redis.IRedisRepositories) DashboardRepository {
	log.Println("🚀 Initialized Repository : Dashboard")

	dashboardCol := mongoClient.GetCollectionByName("dashboards")
	widgetCol := mongoClient.GetCollectionByName("widgets")

	// Create indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Dashboard indexes
	dashboardCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "chat_id", Value: 1}}},
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
		{Keys: bson.D{{Key: "chat_id", Value: 1}, {Key: "is_default", Value: 1}}},
	})

	// Widget indexes
	widgetCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "dashboard_id", Value: 1}}},
		{Keys: bson.D{{Key: "chat_id", Value: 1}}},
	})

	return &dashboardRepository{
		dashboardCollection: dashboardCol,
		widgetCollection:    widgetCol,
		redisRepo:           redisRepo,
	}
}

// === Dashboard Cache Helpers ===

func (r *dashboardRepository) cacheDashboard(dashboard *models.Dashboard) {
	if dashboard == nil {
		return
	}
	ctx := context.Background()
	cacheKey := fmt.Sprintf("dashboard:id:%s", dashboard.ID.Hex())

	data, err := json.Marshal(dashboard)
	if err != nil {
		log.Printf("[CACHE ERROR] Failed to marshal dashboard - ID: %s, Error: %v", dashboard.ID.Hex(), err)
		return
	}
	if err := r.redisRepo.SetCompressed(cacheKey, data, constants.DashboardCacheTTL, ctx); err != nil {
		log.Printf("[CACHE ERROR] Failed to cache dashboard - Key: %s, Error: %v", cacheKey, err)
	} else {
		log.Printf("[CACHE WRITE SUCCESS] Cached dashboard - Key: %s", cacheKey)
	}
}

func (r *dashboardRepository) invalidateDashboardCache(id primitive.ObjectID) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("dashboard:id:%s", id.Hex())
	r.redisRepo.Del(cacheKey, ctx)
	log.Printf("[CACHE INVALIDATE] Invalidated dashboard cache - ID: %s", id.Hex())
}

func (r *dashboardRepository) invalidateDashboardListCache(chatID primitive.ObjectID) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("dashboard:list:chat:%s", chatID.Hex())
	r.redisRepo.Del(cacheKey, ctx)
	log.Printf("[CACHE INVALIDATE] Invalidated dashboard list cache - ChatID: %s", chatID.Hex())
}

func (r *dashboardRepository) cacheWidgets(dashboardID primitive.ObjectID, widgets []*models.Widget) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("dashboard:widgets:%s", dashboardID.Hex())

	data, err := json.Marshal(widgets)
	if err != nil {
		log.Printf("[CACHE ERROR] Failed to marshal widgets - DashboardID: %s, Error: %v", dashboardID.Hex(), err)
		return
	}
	if err := r.redisRepo.SetCompressed(cacheKey, data, constants.DashboardCacheTTL, ctx); err != nil {
		log.Printf("[CACHE ERROR] Failed to cache widgets - Key: %s, Error: %v", cacheKey, err)
	}
}

func (r *dashboardRepository) invalidateWidgetCache(dashboardID primitive.ObjectID) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("dashboard:widgets:%s", dashboardID.Hex())
	r.redisRepo.Del(cacheKey, ctx)
	log.Printf("[CACHE INVALIDATE] Invalidated widget cache - DashboardID: %s", dashboardID.Hex())
}

// === Dashboard CRUD ===

func (r *dashboardRepository) CreateDashboard(ctx context.Context, dashboard *models.Dashboard) error {
	_, err := r.dashboardCollection.InsertOne(ctx, dashboard)
	if err != nil {
		log.Printf("[DB ERROR] Failed to create dashboard - Error: %v", err)
		return err
	}

	r.cacheDashboard(dashboard)
	r.invalidateDashboardListCache(dashboard.ChatID)
	log.Printf("[DB SUCCESS] Created dashboard - ID: %s, ChatID: %s", dashboard.ID.Hex(), dashboard.ChatID.Hex())
	return nil
}

func (r *dashboardRepository) UpdateDashboard(ctx context.Context, id primitive.ObjectID, dashboard *models.Dashboard) error {
	dashboard.UpdatedAt = time.Now()
	_, err := r.dashboardCollection.ReplaceOne(ctx, bson.M{"_id": id}, dashboard)
	if err != nil {
		log.Printf("[DB ERROR] Failed to update dashboard - ID: %s, Error: %v", id.Hex(), err)
		return err
	}

	r.cacheDashboard(dashboard)
	r.invalidateDashboardListCache(dashboard.ChatID)
	log.Printf("[DB SUCCESS] Updated dashboard - ID: %s", id.Hex())
	return nil
}

func (r *dashboardRepository) DeleteDashboard(ctx context.Context, id primitive.ObjectID) error {
	// Get the dashboard first for cache invalidation
	dashboard, err := r.FindDashboardByID(ctx, id)
	if err != nil {
		return err
	}

	_, err = r.dashboardCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		log.Printf("[DB ERROR] Failed to delete dashboard - ID: %s, Error: %v", id.Hex(), err)
		return err
	}

	r.invalidateDashboardCache(id)
	r.invalidateWidgetCache(id)
	if dashboard != nil {
		r.invalidateDashboardListCache(dashboard.ChatID)
	}
	log.Printf("[DB SUCCESS] Deleted dashboard - ID: %s", id.Hex())
	return nil
}

func (r *dashboardRepository) FindDashboardByID(ctx context.Context, id primitive.ObjectID) (*models.Dashboard, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("dashboard:id:%s", id.Hex())
	cachedData, err := r.redisRepo.GetCompressed(cacheKey, ctx)
	if err == nil && cachedData != nil {
		var dashboard models.Dashboard
		if err := json.Unmarshal(cachedData, &dashboard); err == nil {
			log.Printf("[CACHE HIT] Dashboard found in cache - ID: %s", id.Hex())
			return &dashboard, nil
		}
	}

	// Fallback to DB
	var dashboard models.Dashboard
	err = r.dashboardCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&dashboard)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("dashboard not found: %s", id.Hex())
		}
		return nil, err
	}

	r.cacheDashboard(&dashboard)
	return &dashboard, nil
}

func (r *dashboardRepository) FindDashboardsByChatID(ctx context.Context, chatID primitive.ObjectID) ([]*models.Dashboard, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("dashboard:list:chat:%s", chatID.Hex())
	cachedData, err := r.redisRepo.GetCompressed(cacheKey, ctx)
	if err == nil && cachedData != nil {
		var dashboards []*models.Dashboard
		if err := json.Unmarshal(cachedData, &dashboards); err == nil {
			log.Printf("[CACHE HIT] Dashboard list found in cache - ChatID: %s", chatID.Hex())
			return dashboards, nil
		}
	}

	// Fallback to DB
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.dashboardCollection.Find(ctx, bson.M{"chat_id": chatID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var dashboards []*models.Dashboard
	if err := cursor.All(ctx, &dashboards); err != nil {
		return nil, err
	}

	// Cache the list
	if data, err := json.Marshal(dashboards); err == nil {
		r.redisRepo.SetCompressed(cacheKey, data, constants.DashboardListCacheTTL, ctx)
	}

	return dashboards, nil
}

func (r *dashboardRepository) CountDashboardsByChatID(ctx context.Context, chatID primitive.ObjectID) (int64, error) {
	count, err := r.dashboardCollection.CountDocuments(ctx, bson.M{"chat_id": chatID})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *dashboardRepository) SetDefaultDashboard(ctx context.Context, chatID, dashboardID primitive.ObjectID) error {
	// Unset all defaults for this chat
	_, err := r.dashboardCollection.UpdateMany(ctx,
		bson.M{"chat_id": chatID},
		bson.M{"$set": bson.M{"is_default": false, "updated_at": time.Now()}},
	)
	if err != nil {
		return err
	}

	// Set the new default
	_, err = r.dashboardCollection.UpdateOne(ctx,
		bson.M{"_id": dashboardID},
		bson.M{"$set": bson.M{"is_default": true, "updated_at": time.Now()}},
	)
	if err != nil {
		return err
	}

	r.invalidateDashboardListCache(chatID)
	r.invalidateDashboardCache(dashboardID)
	return nil
}

// === Widget CRUD ===

func (r *dashboardRepository) CreateWidget(ctx context.Context, widget *models.Widget) error {
	_, err := r.widgetCollection.InsertOne(ctx, widget)
	if err != nil {
		log.Printf("[DB ERROR] Failed to create widget - Error: %v", err)
		return err
	}

	r.invalidateWidgetCache(widget.DashboardID)
	log.Printf("[DB SUCCESS] Created widget - ID: %s, DashboardID: %s", widget.ID.Hex(), widget.DashboardID.Hex())
	return nil
}

func (r *dashboardRepository) CreateWidgets(ctx context.Context, widgets []*models.Widget) error {
	if len(widgets) == 0 {
		return nil
	}

	docs := make([]interface{}, len(widgets))
	for i, w := range widgets {
		docs[i] = w
	}

	_, err := r.widgetCollection.InsertMany(ctx, docs)
	if err != nil {
		log.Printf("[DB ERROR] Failed to create widgets batch - Error: %v", err)
		return err
	}

	r.invalidateWidgetCache(widgets[0].DashboardID)
	log.Printf("[DB SUCCESS] Created %d widgets - DashboardID: %s", len(widgets), widgets[0].DashboardID.Hex())
	return nil
}

func (r *dashboardRepository) UpdateWidget(ctx context.Context, id primitive.ObjectID, widget *models.Widget) error {
	widget.UpdatedAt = time.Now()
	_, err := r.widgetCollection.ReplaceOne(ctx, bson.M{"_id": id}, widget)
	if err != nil {
		log.Printf("[DB ERROR] Failed to update widget - ID: %s, Error: %v", id.Hex(), err)
		return err
	}

	r.invalidateWidgetCache(widget.DashboardID)
	log.Printf("[DB SUCCESS] Updated widget - ID: %s", id.Hex())
	return nil
}

func (r *dashboardRepository) DeleteWidget(ctx context.Context, id primitive.ObjectID) error {
	// Get widget first for cache invalidation
	widget, err := r.FindWidgetByID(ctx, id)
	if err != nil {
		return err
	}

	_, err = r.widgetCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		log.Printf("[DB ERROR] Failed to delete widget - ID: %s, Error: %v", id.Hex(), err)
		return err
	}

	if widget != nil {
		r.invalidateWidgetCache(widget.DashboardID)
	}
	log.Printf("[DB SUCCESS] Deleted widget - ID: %s", id.Hex())
	return nil
}

func (r *dashboardRepository) DeleteWidgetsByDashboardID(ctx context.Context, dashboardID primitive.ObjectID) error {
	result, err := r.widgetCollection.DeleteMany(ctx, bson.M{"dashboard_id": dashboardID})
	if err != nil {
		log.Printf("[DB ERROR] Failed to delete widgets for dashboard - DashboardID: %s, Error: %v", dashboardID.Hex(), err)
		return err
	}

	r.invalidateWidgetCache(dashboardID)
	log.Printf("[DB SUCCESS] Deleted %d widgets for dashboard - DashboardID: %s", result.DeletedCount, dashboardID.Hex())
	return nil
}

func (r *dashboardRepository) FindWidgetByID(ctx context.Context, id primitive.ObjectID) (*models.Widget, error) {
	var widget models.Widget
	err := r.widgetCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&widget)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("widget not found: %s", id.Hex())
		}
		return nil, err
	}
	return &widget, nil
}

func (r *dashboardRepository) FindWidgetsByDashboardID(ctx context.Context, dashboardID primitive.ObjectID) ([]*models.Widget, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("dashboard:widgets:%s", dashboardID.Hex())
	cachedData, err := r.redisRepo.GetCompressed(cacheKey, ctx)
	if err == nil && cachedData != nil {
		var widgets []*models.Widget
		if err := json.Unmarshal(cachedData, &widgets); err == nil {
			log.Printf("[CACHE HIT] Widgets found in cache - DashboardID: %s", dashboardID.Hex())
			return widgets, nil
		}
	}

	// Fallback to DB
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cursor, err := r.widgetCollection.Find(ctx, bson.M{"dashboard_id": dashboardID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var widgets []*models.Widget
	if err := cursor.All(ctx, &widgets); err != nil {
		return nil, err
	}

	r.cacheWidgets(dashboardID, widgets)
	return widgets, nil
}

func (r *dashboardRepository) CountWidgetsByDashboardID(ctx context.Context, dashboardID primitive.ObjectID) (int64, error) {
	count, err := r.widgetCollection.CountDocuments(ctx, bson.M{"dashboard_id": dashboardID})
	if err != nil {
		return 0, err
	}
	return count, nil
}
