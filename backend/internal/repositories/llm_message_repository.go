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
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LLMMessageRepository interface {
	// Message operations
	CreateMessage(msg *models.LLMMessage) error
	UpdateMessage(id primitive.ObjectID, message *models.LLMMessage) error
	FindMessageByID(id primitive.ObjectID) (*models.LLMMessage, error)
	FindMessageByChatMessageID(messageID primitive.ObjectID) (*models.LLMMessage, error)
	FindMessagesByChatID(chatID primitive.ObjectID) ([]*models.LLMMessage, int64, error)
	FindMessagesByChatIDWithPagination(chatID primitive.ObjectID, page int, pageSize int) ([]*models.LLMMessage, int64, error)
	DeleteMessagesByChatID(chatID primitive.ObjectID, dontDeleteSystemMessages bool) error
	DeleteMessagesByRole(chatID primitive.ObjectID, role string) error
	GetByChatID(chatID primitive.ObjectID) ([]*models.LLMMessage, error)
}

type llmMessageRepository struct {
	messageCollection *mongo.Collection
	streamCollection  *mongo.Collection
	redisRepo         redis.IRedisRepositories
	cacheLocks        map[string]*sync.RWMutex // Per-chat cache locks
	locksMutex        sync.Mutex               // Protects cacheLocks map
}

func NewLLMMessageRepository(mongoClient *mongodb.MongoDBClient, redisRepo redis.IRedisRepositories) LLMMessageRepository {
	return &llmMessageRepository{
		messageCollection: mongoClient.GetCollectionByName("llm_messages"),
		streamCollection:  mongoClient.GetCollectionByName("llm_message_streams"),
		redisRepo:         redisRepo,
		cacheLocks:        make(map[string]*sync.RWMutex),
	}
}

// getCacheLock returns or creates a mutex for a specific cache key
func (r *llmMessageRepository) getCacheLock(cacheKey string) *sync.RWMutex {
	r.locksMutex.Lock()
	defer r.locksMutex.Unlock()

	if lock, exists := r.cacheLocks[cacheKey]; exists {
		return lock
	}

	// Create new lock for this cache key
	lock := &sync.RWMutex{}
	r.cacheLocks[cacheKey] = lock
	log.Printf("[CACHE LOCK] Created new mutex for key: %s", cacheKey)
	return lock
}

// addLLMMessageToCache adds a new LLM message to the sliding window cache
func (r *llmMessageRepository) addLLMMessageToCache(message *models.LLMMessage) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("chat:%s:llm_messages:recent", message.ChatID.Hex())

	msgData, err := json.Marshal(message)
	if err != nil {
		log.Printf("[CACHE ERROR] Failed to marshal LLM message for caching: %v", err)
		return
	}

	// Add message to the BEGINNING of the list (LPush) - newest first for display
	if err := r.redisRepo.LPush(cacheKey, [][]byte{msgData}, constants.LLMMessageCacheTTL, ctx); err != nil {
		log.Printf("[CACHE ERROR] Failed to add LLM message to cache: %v", err)
		return
	}

	// Trim to keep only first 50 messages (newest 50)
	if err := r.redisRepo.LTrim(cacheKey, 0, constants.MaxCachedMessages-1, ctx); err != nil {
		log.Printf("[CACHE ERROR] Failed to trim LLM message cache: %v", err)
	} else {
		log.Printf("[CACHE WRITE SUCCESS] Added LLM message to sliding window - ChatID: %s", message.ChatID.Hex())
	}
}

// getLLMMessagesFromCache retrieves LLM conversation context from cache
func (r *llmMessageRepository) getLLMMessagesFromCache(chatID primitive.ObjectID) ([]*models.LLMMessage, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("chat:%s:llm_messages:recent", chatID.Hex())

	// Check cache size
	cacheSize, err := r.redisRepo.LLen(cacheKey, ctx)
	if err != nil || cacheSize == 0 {
		log.Printf("[CACHE MISS] No LLM messages in cache - ChatID: %s", chatID.Hex())
		return nil, nil
	}

	log.Printf("[CACHE] Cache has %d LLM messages for ChatID: %s", cacheSize, chatID.Hex())

	// Get all cached messages (for conversation context, we want the full window)
	msgBytes, err := r.redisRepo.LRange(cacheKey, 0, -1, ctx)
	if err != nil {
		log.Printf("[CACHE ERROR] Failed to fetch LLM messages from cache: %v", err)
		return nil, err
	}

	messages := make([]*models.LLMMessage, 0, len(msgBytes))
	for _, data := range msgBytes {
		var msg models.LLMMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[CACHE ERROR] Failed to unmarshal cached LLM message: %v", err)
			continue
		}
		messages = append(messages, &msg)
	}

	log.Printf("[CACHE HIT] Fetched %d LLM messages from cache - ChatID: %s", len(messages), chatID.Hex())

	// Refresh TTL on access (sliding expiration for active conversations)
	go func() {
		if err := r.redisRepo.Expire(cacheKey, constants.LLMMessageCacheTTL, ctx); err != nil {
			log.Printf("[CACHE] Failed to refresh TTL: %v", err)
		} else {
			log.Printf("[CACHE] Refreshed TTL for active conversation - ChatID: %s", chatID.Hex())
		}
	}()

	return messages, nil
}

// updateMessageInCache updates a single message in the cached list
func (r *llmMessageRepository) updateMessageInCache(message *models.LLMMessage) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("chat:%s:llm_messages:recent", message.ChatID.Hex())

	// Acquire write lock for this chat's cache
	lock := r.getCacheLock(cacheKey)
	lock.Lock()
	defer lock.Unlock()

	log.Printf("[CACHE LOCK] Acquired write lock for update - ChatID: %s", message.ChatID.Hex())

	// Fetch entire list
	msgBytes, err := r.redisRepo.LRange(cacheKey, 0, -1, ctx)
	if err != nil || len(msgBytes) == 0 {
		log.Printf("[CACHE SKIP] No cache to update - ChatID: %s", message.ChatID.Hex())
		return
	}

	// Find and update the message in list
	updated := false
	updatedList := make([][]byte, 0, len(msgBytes))
	for _, data := range msgBytes {
		var msg models.LLMMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[CACHE ERROR] Failed to unmarshal cached message: %v", err)
			continue
		}

		// If this is the message to update, replace it
		if msg.ID == message.ID {
			newData, _ := json.Marshal(message)
			updatedList = append(updatedList, newData)
			updated = true
			log.Printf("[CACHE UPDATE] Found and updated message in cache - MessageID: %s", message.ID.Hex())
		} else {
			updatedList = append(updatedList, data)
		}
	}

	if !updated {
		log.Printf("[CACHE SKIP] Message not found in cache - MessageID: %s", message.ID.Hex())
		return
	}

	// Delete old list and write updated list
	r.redisRepo.Del(cacheKey, ctx)
	for _, data := range updatedList {
		r.redisRepo.RPush(cacheKey, [][]byte{data}, constants.LLMMessageCacheTTL, ctx)
	}

	log.Printf("[CACHE UPDATE SUCCESS] Updated message in cache list - ChatID: %s", message.ChatID.Hex())
}

// deleteMessageFromCache removes a single message from the cached list
func (r *llmMessageRepository) deleteMessageFromCache(chatID, messageID primitive.ObjectID) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("chat:%s:llm_messages:recent", chatID.Hex())

	// Acquire write lock for this chat's cache
	lock := r.getCacheLock(cacheKey)
	lock.Lock()
	defer lock.Unlock()

	log.Printf("[CACHE LOCK] Acquired write lock for delete - ChatID: %s", chatID.Hex())

	// Fetch entire list
	msgBytes, err := r.redisRepo.LRange(cacheKey, 0, -1, ctx)
	if err != nil || len(msgBytes) == 0 {
		log.Printf("[CACHE SKIP] No cache to update - ChatID: %s", chatID.Hex())
		return
	}

	// Filter out the deleted message
	found := false
	updatedList := make([][]byte, 0, len(msgBytes))
	for _, data := range msgBytes {
		var msg models.LLMMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		// Skip the message to delete
		if msg.ID == messageID {
			found = true
			log.Printf("[CACHE DELETE] Found and removing message from cache - MessageID: %s", messageID.Hex())
			continue
		}
		updatedList = append(updatedList, data)
	}

	if !found {
		log.Printf("[CACHE SKIP] Message not found in cache - MessageID: %s", messageID.Hex())
		return
	}

	// Delete old list and write updated list
	r.redisRepo.Del(cacheKey, ctx)
	if len(updatedList) > 0 {
		for _, data := range updatedList {
			r.redisRepo.RPush(cacheKey, [][]byte{data}, constants.LLMMessageCacheTTL, ctx)
		}
		log.Printf("[CACHE DELETE SUCCESS] Removed message from cache list - ChatID: %s", chatID.Hex())
	} else {
		log.Printf("[CACHE DELETE] Cache is now empty - ChatID: %s", chatID.Hex())
	}
}

// invalidateLLMMessageCache removes LLM message cache for a chat
func (r *llmMessageRepository) invalidateLLMMessageCache(chatID primitive.ObjectID) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("chat:%s:llm_messages:recent", chatID.Hex())

	if err := r.redisRepo.Del(cacheKey, ctx); err != nil {
		log.Printf("[CACHE ERROR] Failed to invalidate LLM message cache: %v", err)
	} else {
		log.Printf("[CACHE INVALIDATE SUCCESS] Invalidated LLM message cache - ChatID: %s", chatID.Hex())
	}
}

// populateLLMMessageCache populates cache with messages from DB
func (r *llmMessageRepository) populateLLMMessageCache(chatID primitive.ObjectID, messages []*models.LLMMessage) {
	if len(messages) == 0 {
		return
	}

	ctx := context.Background()
	cacheKey := fmt.Sprintf("chat:%s:llm_messages:recent", chatID.Hex())

	// Clear existing cache first
	r.redisRepo.Del(cacheKey, ctx)

	// Reverse messages to store newest first (since DB returns oldest first)
	// LPush adds to beginning, so we push in reverse order
	for i := len(messages) - 1; i >= 0; i-- {
		msgData, err := json.Marshal(messages[i])
		if err != nil {
			continue
		}
		r.redisRepo.LPush(cacheKey, [][]byte{msgData}, constants.LLMMessageCacheTTL, ctx)
	}

	// Trim to max 50 (keep first 50 which are newest)
	r.redisRepo.LTrim(cacheKey, 0, constants.MaxCachedMessages-1, ctx)
	log.Printf("[CACHE POPULATE] Cached %d LLM messages for ChatID: %s", len(messages), chatID.Hex())
}

// Message operations
func (r *llmMessageRepository) CreateMessage(msg *models.LLMMessage) error {
	_, err := r.messageCollection.InsertOne(context.Background(), msg)

	if err == nil {
		// Add message to sliding window cache (important for LLM context)
		go r.addLLMMessageToCache(msg)
	}

	return err
}

func (r *llmMessageRepository) UpdateMessage(id primitive.ObjectID, message *models.LLMMessage) error {
	message.UpdatedAt = time.Now()
	filter := bson.M{"_id": id}
	update := bson.M{"$set": message}
	_, err := r.messageCollection.UpdateOne(context.Background(), filter, update)

	if err == nil {
		// Update message in cached list (in-place update)
		go r.updateMessageInCache(message)
	}

	return err
}

func (r *llmMessageRepository) FindMessageByID(id primitive.ObjectID) (*models.LLMMessage, error) {
	var message models.LLMMessage
	err := r.messageCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&message)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &message, err
}

func (r *llmMessageRepository) FindMessagesByChatID(chatID primitive.ObjectID) ([]*models.LLMMessage, int64, error) {
	var messages []*models.LLMMessage
	filter := bson.M{"chat_id": chatID}

	// Get total count
	total, err := r.messageCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	// Sort by created_at DESCENDING (newest first) for display
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.messageCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)
	return messages, total, err
}

func (r *llmMessageRepository) FindMessagesByChatIDWithPagination(chatID primitive.ObjectID, page int, pageSize int) ([]*models.LLMMessage, int64, error) {
	var messages []*models.LLMMessage
	filter := bson.M{"chat_id": chatID}

	total, err := r.messageCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	// Sort by created_at DESCENDING (newest first) for display
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cursor, err := r.messageCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)
	return messages, total, err
}

func (r *llmMessageRepository) DeleteMessagesByChatID(chatID primitive.ObjectID, dontDeleteSystemMessages bool) error {
	filter := bson.M{"chat_id": chatID}
	if dontDeleteSystemMessages {
		filter["role"] = bson.M{"$ne": "system"}
	}
	_, err := r.messageCollection.DeleteMany(context.Background(), filter)

	if err == nil {
		go r.invalidateLLMMessageCache(chatID)
	}

	return err
}

func (r *llmMessageRepository) GetByChatID(chatID primitive.ObjectID) ([]*models.LLMMessage, error) {
	// Try cache first - this is critical for LLM conversation context
	log.Printf("[CACHE] Attempting to fetch LLM conversation context from cache - ChatID: %s", chatID.Hex())
	cachedMessages, err := r.getLLMMessagesFromCache(chatID)
	if err == nil && len(cachedMessages) > 0 {
		log.Printf("[CACHE HIT] Using cached LLM conversation context (%d messages)", len(cachedMessages))
		return cachedMessages, nil
	}

	// Cache miss - query database
	log.Printf("[CACHE MISS] Fetching LLM conversation context from DB - ChatID: %s", chatID.Hex())
	var messages []*models.LLMMessage
	filter := bson.M{"chat_id": chatID}

	// Important: Sort by created_at ASCENDING for chronological order (oldest first)
	// LLMs need messages in the order they were sent
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := r.messageCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)

	// Populate cache for future requests
	if err == nil && len(messages) > 0 {
		go r.populateLLMMessageCache(chatID, messages)
	}

	return messages, err
}

// FindMessageByChatMessageID finds a message by the chat message ID(original message id)
func (r *llmMessageRepository) FindMessageByChatMessageID(messageID primitive.ObjectID) (*models.LLMMessage, error) {
	var message models.LLMMessage
	err := r.messageCollection.FindOne(context.Background(), bson.M{"message_id": messageID}).Decode(&message)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &message, err
}

// DeleteMessagesByRole deletes all messages by role for a given chat
func (r *llmMessageRepository) DeleteMessagesByRole(chatID primitive.ObjectID, role string) error {
	filter := bson.M{"chat_id": chatID, "role": role}
	_, err := r.messageCollection.DeleteMany(context.Background(), filter)

	if err == nil {
		go r.invalidateLLMMessageCache(chatID)
	}

	return err
}
