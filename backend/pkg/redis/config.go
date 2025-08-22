package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func RedisClient(redisHost, redisPort, redisUsername, redisPassword string) (*redis.Client, error) {
	redisURL := fmt.Sprintf("%s:%s", redisHost, redisPort)

	// Create Redis client with retry logic
	client := redis.NewClient(&redis.Options{
		Addr:         redisURL,
		Username:     redisUsername,
		Password:     redisPassword,
		DB:           0,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     10,
		MaxRetries:   3,
	})

	// Add retry logic for initial connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to ping Redis server with retries
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		if err := client.Ping(ctx).Err(); err != nil {
			log.Printf("Failed to connect to Redis with username %s (attempt %d/%d): %v", redisUsername, i+1, maxRetries, err)
			if i == maxRetries-1 {
				return nil, fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, err)
			}
			time.Sleep(2 * time.Second)
			continue
		}
		log.Println("✨ Connected to Redis successfully")
		return client, nil
	}

	return client, nil
}
