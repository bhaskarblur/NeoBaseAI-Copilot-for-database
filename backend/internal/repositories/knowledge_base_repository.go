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

// KnowledgeBaseRepository defines operations for knowledge base persistence.
type KnowledgeBaseRepository interface {
	Upsert(ctx context.Context, kb *models.KnowledgeBase) error
	FindByChatID(ctx context.Context, chatID primitive.ObjectID) (*models.KnowledgeBase, error)
	DeleteByChatID(ctx context.Context, chatID primitive.ObjectID) error
	GetTableDescriptions(ctx context.Context, chatID primitive.ObjectID, tableNames []string) ([]models.TableDescription, error)
}

type knowledgeBaseRepository struct {
	collection *mongo.Collection
	redisRepo  redis.IRedisRepositories
}

// NewKnowledgeBaseRepository creates a new repository backed by the `knowledge_bases` MongoDB collection.
func NewKnowledgeBaseRepository(mongoClient *mongodb.MongoDBClient, redisRepo redis.IRedisRepositories) KnowledgeBaseRepository {
	repo := &knowledgeBaseRepository{
		collection: mongoClient.GetCollectionByName("knowledge_bases"),
		redisRepo:  redisRepo,
	}

	// Ensure unique index on chat_id (one KB per chat)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := repo.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "chat_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		})
		if err != nil {
			log.Printf("KnowledgeBase -> Warning: failed to create chat_id index: %v", err)
		}
	}()

	return repo
}

// ─── Cache helpers ───────────────────────────────────────────────────────────

func kbCacheKey(chatID primitive.ObjectID) string {
	return fmt.Sprintf("kb:chat:%s", chatID.Hex())
}

// cacheKB stores a knowledge base in Redis with compression.
func (r *knowledgeBaseRepository) cacheKB(kb *models.KnowledgeBase) {
	if kb == nil {
		return
	}
	ctx := context.Background()
	key := kbCacheKey(kb.ChatID)

	data, err := json.Marshal(kb)
	if err != nil {
		log.Printf("[CACHE ERROR] Failed to marshal KB for caching - ChatID: %s, Error: %v", kb.ChatID.Hex(), err)
		return
	}

	if err := r.redisRepo.SetCompressed(key, data, constants.KnowledgeBaseCacheTTL, ctx); err != nil {
		log.Printf("[CACHE ERROR] Failed to cache KB - Key: %s, Error: %v", key, err)
	} else {
		log.Printf("[CACHE WRITE SUCCESS] Cached KB - Key: %s, Size: %d bytes", key, len(data))
	}
}

// getCachedKB attempts to retrieve a knowledge base from Redis.
// Returns nil, nil on cache miss.
func (r *knowledgeBaseRepository) getCachedKB(chatID primitive.ObjectID) (*models.KnowledgeBase, error) {
	ctx := context.Background()
	key := kbCacheKey(chatID)

	compressed, err := r.redisRepo.GetCompressed(key, ctx)
	if err != nil {
		// Cache miss — totally normal
		return nil, nil
	}

	var kb models.KnowledgeBase
	if err := json.Unmarshal(compressed, &kb); err != nil {
		log.Printf("[CACHE ERROR] Failed to unmarshal cached KB - Key: %s, Error: %v", key, err)
		// Delete corrupted cache entry
		r.redisRepo.Del(key, ctx)
		return nil, nil
	}

	log.Printf("[CACHE HIT] KB found in cache - Key: %s", key)
	return &kb, nil
}

// invalidateKBCache removes the cached KB for a chat.
func (r *knowledgeBaseRepository) invalidateKBCache(chatID primitive.ObjectID) {
	ctx := context.Background()
	key := kbCacheKey(chatID)
	if err := r.redisRepo.Del(key, ctx); err != nil {
		log.Printf("[CACHE ERROR] Failed to invalidate KB cache - Key: %s, Error: %v", key, err)
	} else {
		log.Printf("[CACHE DELETE] Invalidated KB cache - Key: %s", key)
	}
}

// ─── Repository methods ─────────────────────────────────────────────────────

// Upsert creates or updates the knowledge base for a chat.
func (r *knowledgeBaseRepository) Upsert(ctx context.Context, kb *models.KnowledgeBase) error {
	now := time.Now()
	kb.UpdatedAt = now

	filter := bson.M{"chat_id": kb.ChatID}
	update := bson.M{
		"$set": bson.M{
			"table_descriptions": kb.TableDescriptions,
			"updated_at":         now,
		},
		"$setOnInsert": bson.M{
			"_id":        kb.ID,
			"chat_id":    kb.ChatID,
			"created_at": kb.CreatedAt,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert knowledge base for chat %s: %w", kb.ChatID.Hex(), err)
	}

	// Write-aside: update cache with fresh data from DB
	go r.updateKBCache(kb.ChatID)

	return nil
}

// updateKBCache fetches fresh KB from MongoDB and caches it (write-aside pattern).
func (r *knowledgeBaseRepository) updateKBCache(chatID primitive.ObjectID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var kb models.KnowledgeBase
	err := r.collection.FindOne(ctx, bson.M{"chat_id": chatID}).Decode(&kb)
	if err != nil {
		log.Printf("[CACHE ERROR] Failed to fetch KB from DB for cache update - ChatID: %s, Error: %v", chatID.Hex(), err)
		return
	}

	r.cacheKB(&kb)
}

// FindByChatID retrieves the knowledge base for a specific chat.
func (r *knowledgeBaseRepository) FindByChatID(ctx context.Context, chatID primitive.ObjectID) (*models.KnowledgeBase, error) {
	// Try cache first
	if cached, _ := r.getCachedKB(chatID); cached != nil {
		return cached, nil
	}

	// Cache miss — fetch from MongoDB
	var kb models.KnowledgeBase
	err := r.collection.FindOne(ctx, bson.M{"chat_id": chatID}).Decode(&kb)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // No KB yet — this is normal
		}
		return nil, fmt.Errorf("failed to find knowledge base for chat %s: %w", chatID.Hex(), err)
	}

	// Populate cache for next time
	go r.cacheKB(&kb)

	return &kb, nil
}

// DeleteByChatID removes the knowledge base when a chat is deleted.
func (r *knowledgeBaseRepository) DeleteByChatID(ctx context.Context, chatID primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"chat_id": chatID})
	if err != nil {
		return fmt.Errorf("failed to delete knowledge base for chat %s: %w", chatID.Hex(), err)
	}

	// Invalidate cache
	go r.invalidateKBCache(chatID)

	return nil
}

// GetTableDescriptions returns descriptions only for the requested tables.
// Useful for RAG context enrichment — only fetch descriptions for tables in the query.
func (r *knowledgeBaseRepository) GetTableDescriptions(ctx context.Context, chatID primitive.ObjectID, tableNames []string) ([]models.TableDescription, error) {
	kb, err := r.FindByChatID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if kb == nil {
		return nil, nil
	}

	// Build a set of requested table names for quick lookup
	requested := make(map[string]bool, len(tableNames))
	for _, name := range tableNames {
		requested[name] = true
	}

	result := make([]models.TableDescription, 0)
	for _, td := range kb.TableDescriptions {
		if requested[td.TableName] {
			result = append(result, td)
		}
	}

	return result, nil
}
