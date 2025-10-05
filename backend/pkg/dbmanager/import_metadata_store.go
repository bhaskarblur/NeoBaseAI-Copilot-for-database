package dbmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/pkg/redis"
	"time"
	
	goredis "github.com/redis/go-redis/v9"
)

// ImportMetadataStore handles storage and retrieval of import metadata
type ImportMetadataStore struct {
	redisRepo redis.IRedisRepositories
}

// NewImportMetadataStore creates a new metadata store
func NewImportMetadataStore(redisRepo redis.IRedisRepositories) *ImportMetadataStore {
	return &ImportMetadataStore{
		redisRepo: redisRepo,
	}
}

// StoreMetadata stores import metadata for a connection
func (s *ImportMetadataStore) StoreMetadata(chatID string, metadata *dtos.ImportMetadata) error {
	key := fmt.Sprintf("import_metadata:%s", chatID)
	
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	// Store with 7 day expiration
	ctx := context.Background()
	if err := s.redisRepo.Set(key, data, 7*24*time.Hour, ctx); err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}
	
	log.Printf("ImportMetadataStore -> Stored metadata for chat %s", chatID)
	return nil
}

// GetMetadata retrieves import metadata for a connection
func (s *ImportMetadataStore) GetMetadata(chatID string) (*dtos.ImportMetadata, error) {
	key := fmt.Sprintf("import_metadata:%s", chatID)
	
	ctx := context.Background()
	data, err := s.redisRepo.Get(key, ctx)
	if err != nil {
		if err == goredis.Nil {
			return nil, nil // No metadata found
		}
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	
	if data == "" {
		return nil, nil // No metadata found
	}
	
	var metadata dtos.ImportMetadata
	if err := json.Unmarshal([]byte(data), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	
	return &metadata, nil
}

// DeleteMetadata removes import metadata for a connection
func (s *ImportMetadataStore) DeleteMetadata(chatID string) error {
	key := fmt.Sprintf("import_metadata:%s", chatID)
	
	ctx := context.Background()
	if err := s.redisRepo.Del(key, ctx); err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}
	
	log.Printf("ImportMetadataStore -> Deleted metadata for chat %s", chatID)
	return nil
}