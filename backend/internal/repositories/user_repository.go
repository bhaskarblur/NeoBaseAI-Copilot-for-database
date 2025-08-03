package repositories

import (
	"context"
	"fmt"
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
	Create(user *models.User) error
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

func (r *userRepository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.userCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.userCollection.FindOne(context.Background(), bson.M{"email": email}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
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
	userIDPrimitive, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}
	var user models.User
	fmt.Println("userID", userID)
	err = r.userCollection.FindOne(context.Background(), bson.M{"_id": userIDPrimitive}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
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
