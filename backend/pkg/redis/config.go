package redis

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func RedisClient(redisHost, redisPort, redisUsername, redisPassword string) (*redis.Client, error) {
	redisURL := fmt.Sprintf("%s:%s", redisHost, redisPort)

	// Debug logging
	log.Printf("Redis connection - Host: %s, Port: %s, Username: '%s', Password: %s", 
		redisHost, redisPort, redisUsername, 
		strings.Repeat("*", len(redisPassword)))

	// Create Redis options
	opts := &redis.Options{
		Addr:         redisURL,
		Password:     redisPassword,
		DB:           0,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     10,
		MaxRetries:   3,
	}

	// Only set Username if it's not empty
	if redisUsername != "" {
		opts.Username = redisUsername
	}

	// Create Redis client with retry logic
	client := redis.NewClient(opts)

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
