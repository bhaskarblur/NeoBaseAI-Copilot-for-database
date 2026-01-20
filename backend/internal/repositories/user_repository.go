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
)

type UserRepository interface {
	FindByUsername(username string) (*models.User, error)
	FindByEmail(email string) (*models.User, error)
	FindByUsernameOrEmail(usernameOrEmail string) (*models.User, error)
	FindByGoogleID(googleID string) (*models.User, error)
	Create(user *models.User) error
	Update(userID string, user *models.User) error
	CreateUserSignUpSecret(secret string) (*models.UserSignupSecret, error)
	ValidateUserSignupSecret(secret string) bool
	DeleteUserSignupSecret(secret string) error
	FindByID(userID string) (*models.User, error)
	UpdatePassword(userID, newPassword string) error
	StorePasswordResetOTP(email, otp string) error
	ValidatePasswordResetOTP(email, otp string) bool
	DeletePasswordResetOTP(email string) error
}

type userRepository struct {
	userCollection             *mongo.Collection
	userSignupSecretCollection *mongo.Collection
	redisRepo                  redis.IRedisRepositories
}

func NewUserRepository(mongoClient *mongodb.MongoDBClient, redisRepo redis.IRedisRepositories) UserRepository {
	return &userRepository{
		userCollection:             mongoClient.GetCollectionByName("users"),
		userSignupSecretCollection: mongoClient.GetCollectionByName("userSignupSecrets"),
		redisRepo:                  redisRepo,
	}
}

// Helper methods for caching

// cacheUser stores a user in Redis under multiple keys for different lookup patterns
func (r *userRepository) cacheUser(user *models.User) {
	if user == nil {
		return
	}

	ctx := context.Background()
	userData, err := json.Marshal(user)
	if err != nil {
		log.Printf("[CACHE ERROR] Failed to marshal user for caching - UserID: %s, Error: %v", user.ID.Hex(), err)
		return
	}

	// Cache under all three keys: ID, email, and username
	keys := []string{
		fmt.Sprintf("user:id:%s", user.ID.Hex()),
		fmt.Sprintf("user:email:%s", user.Email),
		fmt.Sprintf("user:username:%s", user.Username),
	}

	log.Printf("[CACHE WRITE] Caching user under %d keys - UserID: %s, Size: %d bytes, TTL: %v", len(keys), user.ID.Hex(), len(userData), constants.UserCacheTTL)
	for _, key := range keys {
		if err := r.redisRepo.SetCompressed(key, userData, constants.UserCacheTTL, ctx); err != nil {
			log.Printf("[CACHE ERROR] Failed to cache user - Key: %s, Error: %v", key, err)
		} else {
			log.Printf("[CACHE WRITE SUCCESS] Cached user - Key: %s", key)
		}
	}
}

// updateUserCache updates the cache with fresh user data (write-aside pattern)
// This prevents thundering herd by keeping cache warm instead of invalidating
func (r *userRepository) updateUserCache(userID string) {
	ctx := context.Background()
	log.Printf("[CACHE UPDATE] Starting cache update - UserID: %s", userID)

	// First, try to get old cached data to detect email/username changes
	oldCacheKey := fmt.Sprintf("user:id:%s", userID)
	var oldUser *models.User
	if compressed, err := r.redisRepo.GetCompressed(oldCacheKey, ctx); err == nil {
		var u models.User
		if err := json.Unmarshal(compressed, &u); err == nil {
			oldUser = &u
		}
	}

	// Fetch fresh user data from MongoDB
	userIDPrimitive, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Printf("[CACHE ERROR] Invalid user ID for cache update - UserID: %s, Error: %v", userID, err)
		return
	}

	var user models.User
	err = r.userCollection.FindOne(ctx, bson.M{"_id": userIDPrimitive}).Decode(&user)
	if err != nil {
		log.Printf("[CACHE ERROR] Failed to fetch user from DB for cache update - UserID: %s, Error: %v", userID, err)
		return
	}

	// If email or username changed, delete old keys first
	if oldUser != nil {
		if oldUser.Email != user.Email {
			oldKey := fmt.Sprintf("user:email:%s", oldUser.Email)
			log.Printf("[CACHE UPDATE] Email changed, removing old key - Key: %s", oldKey)
			r.redisRepo.Del(oldKey, ctx)
		}
		if oldUser.Username != user.Username {
			oldKey := fmt.Sprintf("user:username:%s", oldUser.Username)
			log.Printf("[CACHE UPDATE] Username changed, removing old key - Key: %s", oldKey)
			r.redisRepo.Del(oldKey, ctx)
		}
	}

	// Update cache with fresh data (this is write-aside)
	log.Printf("[CACHE UPDATE] Updating cache with fresh data - UserID: %s", userID)
	r.cacheUser(&user)
}

func (r *userRepository) FindByUsername(username string) (*models.User, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:username:%s", username)

	// Try cache first
	log.Printf("[CACHE] Attempting to fetch from cache - Key: %s", cacheKey)
	if compressed, err := r.redisRepo.GetCompressed(cacheKey, ctx); err == nil {
		var user models.User
		if err := json.Unmarshal(compressed, &user); err == nil {
			log.Printf("[CACHE HIT] Successfully retrieved user by username from cache - Key: %s, UserID: %s", cacheKey, user.ID.Hex())
			return &user, nil
		}
		log.Printf("[CACHE ERROR] Failed to unmarshal cached user - Key: %s, Error: %v", cacheKey, err)
	}

	// Cache miss - query database
	log.Printf("[CACHE MISS] User not in cache, querying database - Key: %s", cacheKey)
	var user models.User
	err := r.userCollection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Cache the result asynchronously
	go r.cacheUser(&user)

	return &user, nil
}

func (r *userRepository) FindByEmail(email string) (*models.User, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:email:%s", email)

	// Try cache first
	log.Printf("[CACHE] Attempting to fetch from cache - Key: %s", cacheKey)
	if compressed, err := r.redisRepo.GetCompressed(cacheKey, ctx); err == nil {
		var user models.User
		if err := json.Unmarshal(compressed, &user); err == nil {
			log.Printf("[CACHE HIT] Successfully retrieved user by email from cache - Key: %s, UserID: %s", cacheKey, user.ID.Hex())
			return &user, nil
		}
		log.Printf("[CACHE ERROR] Failed to unmarshal cached user - Key: %s, Error: %v", cacheKey, err)
	}

	// Cache miss - query database
	log.Printf("[CACHE MISS] User not in cache, querying database - Key: %s", cacheKey)
	var user models.User
	err := r.userCollection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Cache the result asynchronously
	log.Printf("[CACHE] Populating cache after DB query - UserID: %s", user.ID.Hex())
	go r.cacheUser(&user)

	return &user, nil
}

func (r *userRepository) FindByUsernameOrEmail(usernameOrEmail string) (*models.User, error) {
	var user models.User
	filter := bson.M{
		"$or": []bson.M{
			{"username": usernameOrEmail},
			{"email": usernameOrEmail},
		},
	}
	err := r.userCollection.FindOne(context.Background(), filter).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Create(user *models.User) error {
	if user.ID.IsZero() {
		user.Base = models.NewBase()
	}
	_, err := r.userCollection.InsertOne(context.Background(), user)

	if err == nil {
		// Warm cache immediately after creation (write-aside)
		go r.cacheUser(user)
	}

	return err
}

func (r *userRepository) CreateUserSignUpSecret(secret string) (*models.UserSignupSecret, error) {
	signupSecret := models.NewUserSignupSecret(secret)
	_, err := r.userSignupSecretCollection.InsertOne(context.Background(), signupSecret)
	if err != nil {
		return nil, err
	}
	return signupSecret, nil
}

func (r *userRepository) ValidateUserSignupSecret(secret string) bool {
	var signupSecret models.UserSignupSecret
	err := r.userSignupSecretCollection.FindOne(context.Background(), bson.M{"secret": secret}).Decode(&signupSecret)
	return err == nil
}

func (r *userRepository) DeleteUserSignupSecret(secret string) error {
	_, err := r.userSignupSecretCollection.DeleteOne(context.Background(), bson.M{"secret": secret})
	return err
}

func (r *userRepository) FindByID(userID string) (*models.User, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:id:%s", userID)

	// Try cache first
	log.Printf("[CACHE] Attempting to fetch from cache - Key: %s", cacheKey)
	if compressed, err := r.redisRepo.GetCompressed(cacheKey, ctx); err == nil {
		var user models.User
		if err := json.Unmarshal(compressed, &user); err == nil {
			log.Printf("[CACHE HIT] Successfully retrieved user by ID from cache - Key: %s", cacheKey)
			return &user, nil
		}
		log.Printf("[CACHE ERROR] Failed to unmarshal cached user - Key: %s, Error: %v", cacheKey, err)
	}

	// Cache miss - query database
	log.Printf("[CACHE MISS] User not in cache, querying database - Key: %s", cacheKey)
	userIDPrimitive, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}
	var user models.User
	fmt.Println("userID", userID)
	err = r.userCollection.FindOne(ctx, bson.M{"_id": userIDPrimitive}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Cache the result asynchronously
	go r.cacheUser(&user)

	return &user, nil
}

func (r *userRepository) UpdatePassword(userID, newPassword string) error {
	userIDPrimitive, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	update := bson.M{
		"$set": bson.M{
			"password":   newPassword,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	_, err = r.userCollection.UpdateOne(
		context.Background(),
		bson.M{"_id": userIDPrimitive},
		update,
	)

	if err == nil {
		// Update cache asynchronously with fresh data (write-aside pattern)
		// This prevents thundering herd by keeping cache warm
		go r.updateUserCache(userID)
	}

	return err
}

func (r *userRepository) StorePasswordResetOTP(email, otp string) error {
	// Store OTP in Redis with 10 minutes expiration
	key := fmt.Sprintf("password_reset_otp:%s", email)
	ctx := context.Background()
	return r.redisRepo.Set(key, []byte(otp), 10*time.Minute, ctx)
}

func (r *userRepository) ValidatePasswordResetOTP(email, otp string) bool {
	key := fmt.Sprintf("password_reset_otp:%s", email)
	ctx := context.Background()
	storedOTP, err := r.redisRepo.Get(key, ctx)
	if err != nil {
		return false
	}
	return storedOTP == otp
}

func (r *userRepository) DeletePasswordResetOTP(email string) error {
	key := fmt.Sprintf("password_reset_otp:%s", email)
	ctx := context.Background()
	return r.redisRepo.Del(key, ctx)
}

// FindByGoogleID finds a user by their Google ID
func (r *userRepository) FindByGoogleID(googleID string) (*models.User, error) {
	var user models.User
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"google_id": googleID}
	err := r.userCollection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // User not found is not an error
		}
		return nil, err
	}
	return &user, nil
}

// Update updates a user document
func (r *userRepository) Update(userID string, user *models.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %v", err)
	}

	user.UpdatedAt = time.Now()

	filter := bson.M{"_id": objectID}
	update := bson.M{"$set": user}

	_, err = r.userCollection.UpdateOne(ctx, filter, update)

	if err == nil {
		// Update cache asynchronously with fresh data (write-aside pattern)
		// This prevents thundering herd by keeping cache warm
		go r.updateUserCache(userID)
	}

	return err
}
