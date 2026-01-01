package repositories

import (
	"context"
	"log"
	"neobase-ai/internal/models"
	"neobase-ai/pkg/mongodb"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ChatRepository interface {
	Create(chat *models.Chat) error
	Update(id primitive.ObjectID, chat *models.Chat) error
	Delete(id primitive.ObjectID) error
	FindByID(id primitive.ObjectID) (*models.Chat, error)
	FindByUserID(userID primitive.ObjectID, page, pageSize int) ([]*models.Chat, int64, error)
	CreateMessage(message *models.Message) error
	UpdateMessage(id primitive.ObjectID, message *models.Message) error
	DeleteMessages(chatID primitive.ObjectID) error
	FindMessagesByChat(chatID primitive.ObjectID, page, pageSize int) ([]*models.Message, int64, error)
	FindLatestMessageByChat(chatID primitive.ObjectID, page, pageSize int) ([]*models.Message, int64, error)
	FindMessageByID(id primitive.ObjectID) (*models.Message, error)
	FindNextMessageByID(id primitive.ObjectID) (*models.Message, error)
	FindPinnedMessagesByChat(chatID primitive.ObjectID) ([]models.Message, error)
	FindMessagesByChatAfterTime(chatID primitive.ObjectID, after time.Time, page, pageSize int) ([]models.Message, int64, error)
	UpdateQueryVisualizationID(messageID, queryID, visualizationID primitive.ObjectID) error
}

// concrete implementation of ChatRepository, using interface composition
type chatRepository struct {
	chatCollection    *mongo.Collection
	messageCollection *mongo.Collection
}

func NewChatRepository(mongoClient *mongodb.MongoDBClient) ChatRepository {
	return &chatRepository{
		chatCollection:    mongoClient.GetCollectionByName("chats"),
		messageCollection: mongoClient.GetCollectionByName("messages"),
	}
}

func (r *chatRepository) Create(chat *models.Chat) error {
	_, err := r.chatCollection.InsertOne(context.Background(), chat)
	return err
}

func (r *chatRepository) Update(id primitive.ObjectID, chat *models.Chat) error {
	chat.UpdatedAt = time.Now()
	filter := bson.M{"_id": id}
	update := bson.M{"$set": chat}
	_, err := r.chatCollection.UpdateOne(context.Background(), filter, update)
	return err
}

func (r *chatRepository) Delete(id primitive.ObjectID) error {
	filter := bson.M{"_id": id}
	_, err := r.chatCollection.DeleteOne(context.Background(), filter)
	return err
}

func (r *chatRepository) FindByID(id primitive.ObjectID) (*models.Chat, error) {
	var chat models.Chat
	err := r.chatCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&chat)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &chat, err
}

func (r *chatRepository) FindByUserID(userID primitive.ObjectID, page, pageSize int) ([]*models.Chat, int64, error) {
	var chats []*models.Chat
	filter := bson.M{"user_id": userID}

	// Get total count
	total, err := r.chatCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	// Setup pagination
	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.chatCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &chats)
	return chats, total, err
}

func (r *chatRepository) CreateMessage(message *models.Message) error {
	log.Printf("CreateMessage -> message: %v", message)
	r.updateChatTimeStamp(message.ChatID)
	_, err := r.messageCollection.InsertOne(context.Background(), message)
	return err
}

func (r *chatRepository) UpdateMessage(id primitive.ObjectID, message *models.Message) error {
	r.updateChatTimeStamp(message.ChatID)
	message.UpdatedAt = time.Now()
	filter := bson.M{"_id": id}
	update := bson.M{"$set": message}
	_, err := r.messageCollection.UpdateOne(context.Background(), filter, update)
	return err
}

func (r *chatRepository) DeleteMessages(chatID primitive.ObjectID) error {
	filter := bson.M{"chat_id": chatID}
	_, err := r.messageCollection.DeleteMany(context.Background(), filter)
	return err
}

func (r *chatRepository) FindMessagesByChat(chatID primitive.ObjectID, page, pageSize int) ([]*models.Message, int64, error) {
	var messages []*models.Message
	filter := bson.M{"chat_id": chatID}

	// Get total count
	total, err := r.messageCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	// Setup pagination
	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}). // Descending order for messages, ex: latest message will be first
		SetSkip(skip).
		SetLimit(int64(pageSize))

	cursor, err := r.messageCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)
	return messages, total, err
}

func (r *chatRepository) FindLatestMessageByChat(chatID primitive.ObjectID, page, pageSize int) ([]*models.Message, int64, error) {
	var messages []*models.Message
	filter := bson.M{"chat_id": chatID}

	// Get total count
	total, err := r.messageCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	// Setup pagination
	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}}) // Descending order for messages, ex: latest message will be first

	cursor, err := r.messageCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)
	return messages, total, err
}

func (r *chatRepository) FindMessageByID(id primitive.ObjectID) (*models.Message, error) {
	var message models.Message
	err := r.messageCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&message)
	return &message, err
}

func (r *chatRepository) updateChatTimeStamp(chatID primitive.ObjectID) error {
	go func() {
		filter := bson.M{"_id": chatID}
		update := bson.M{"$set": bson.M{"updated_at": time.Now()}}
		_, err := r.chatCollection.UpdateOne(context.Background(), filter, update)
		if err != nil {
			log.Printf("Error updating chat timestamp: %v", err)
		}
	}()
	return nil
}

// FindNextMessageByID finds the next message by ID of the previous user message (ex: if the previous message is a user message, find the next message that has userMessageId as the previous message id and is an assistant message)
func (r *chatRepository) FindNextMessageByID(id primitive.ObjectID) (*models.Message, error) {
	// First, find the current message to get its chat ID
	currentMsg, err := r.FindMessageByID(id)
	if err != nil {
		return nil, err
	}
	if currentMsg == nil {
		return nil, mongo.ErrNoDocuments
	}

	// If this is a user message, find the AI message that has this message ID as its UserMessageId
	if currentMsg.Type == "user" {
		filter := bson.M{
			"chat_id":         currentMsg.ChatID,
			"user_message_id": id,
			"type":            "assistant",
		}

		var aiMsg models.Message
		err = r.messageCollection.FindOne(context.Background(), filter).Decode(&aiMsg)
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return &aiMsg, err
	} else {
		// If it's not a user message, fall back to timestamp-based search
		filter := bson.M{
			"chat_id": currentMsg.ChatID,
			"created_at": bson.M{
				"$gt": currentMsg.CreatedAt,
			},
		}

		opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: 1}})

		var nextMsg models.Message
		err = r.messageCollection.FindOne(context.Background(), filter, opts).Decode(&nextMsg)
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return &nextMsg, err
	}
}

// FindPinnedMessagesByChat finds all pinned messages for a chat, sorted by pinnedAt descending
func (r *chatRepository) FindPinnedMessagesByChat(chatID primitive.ObjectID) ([]models.Message, error) {
	var messages []models.Message
	filter := bson.M{
		"chat_id":   chatID,
		"is_pinned": true,
	}

	// Sort by pinnedAt descending (latest first)
	opts := options.Find().SetSort(bson.D{{Key: "pinned_at", Value: -1}})

	cursor, err := r.messageCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)
	return messages, err
}

// FindMessagesByChatAfterTime finds messages in a chat created after a specific time
func (r *chatRepository) FindMessagesByChatAfterTime(chatID primitive.ObjectID, after time.Time, page, pageSize int) ([]models.Message, int64, error) {
	var messages []models.Message
	filter := bson.M{
		"chat_id": chatID,
		"created_at": bson.M{
			"$gt": after,
		},
	}

	// Get total count
	total, err := r.messageCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	// Setup pagination
	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}}) // Descending order

	cursor, err := r.messageCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)
	return messages, total, err
}

// UpdateQueryVisualizationID updates a query's visualization ID within a message
func (r *chatRepository) UpdateQueryVisualizationID(messageID, queryID, visualizationID primitive.ObjectID) error {
	filter := bson.M{
		"_id":        messageID,
		"queries.id": queryID,
	}
	update := bson.M{
		"$set": bson.M{
			"queries.$.visualization_id": visualizationID,
		},
	}
	_, err := r.messageCollection.UpdateOne(context.Background(), filter, update)
	return err
}
