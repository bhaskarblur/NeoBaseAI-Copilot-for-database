package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"neobase-ai/internal/models"
)

type WaitlistRepository struct {
	collection *mongo.Collection
}

func NewWaitlistRepository(db *mongo.Database) *WaitlistRepository {
	return &WaitlistRepository{
		collection: db.Collection("enterprise_waitlist"),
	}
}

func (r *WaitlistRepository) AddToWaitlist(ctx context.Context, email string) (*models.EnterpriseWaitlist, error) {
	// Check if email already exists
	var existing models.EnterpriseWaitlist
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&existing)
	if err == nil {
		// Email already exists, return existing entry
		return &existing, nil
	}

	waitlistEntry := &models.EnterpriseWaitlist{
		ID:        primitive.NewObjectID(),
		Email:     email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = r.collection.InsertOne(ctx, waitlistEntry)
	if err != nil {
		return nil, err
	}

	return waitlistEntry, nil
}

func (r *WaitlistRepository) GetByEmail(ctx context.Context, email string) (*models.EnterpriseWaitlist, error) {
	var waitlistEntry models.EnterpriseWaitlist
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&waitlistEntry)
	if err != nil {
		return nil, err
	}
	return &waitlistEntry, nil
}

func (r *WaitlistRepository) GetAll(ctx context.Context) ([]models.EnterpriseWaitlist, error) {
	var waitlist []models.EnterpriseWaitlist
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &waitlist); err != nil {
		return nil, err
	}

	return waitlist, nil
}
