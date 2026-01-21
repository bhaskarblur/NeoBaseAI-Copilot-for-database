package redis

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"neobase-ai/internal/utils"
	"reflect"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRepositories struct {
	Client *redis.Client
}

type IRedisRepositories interface {
	Set(key string, data []byte, expiredTime time.Duration, ctx context.Context) error
	Hset(key string, data string, expireAt time.Time, ctx context.Context) error
	Get(key string, ctx context.Context) (string, error)
	Del(key string, ctx context.Context) error
	GetAllByField(ctx context.Context, modelType interface{}, filterFunc func(interface{}) bool) ([]interface{}, error)
	TTL(key string, ctx context.Context) (time.Duration, error)
	Expire(key string, expiredTime time.Duration, ctx context.Context) error
	StartPipeline(ctx context.Context) *Pipeline
	// Compressed operations
	SetCompressed(key string, data []byte, expiredTime time.Duration, ctx context.Context) error
	GetCompressed(key string, ctx context.Context) ([]byte, error)
	// List operations for message caching
	LPush(key string, values [][]byte, expiredTime time.Duration, ctx context.Context) error
	RPush(key string, values [][]byte, expiredTime time.Duration, ctx context.Context) error
	LRange(key string, start, stop int64, ctx context.Context) ([][]byte, error)
	LLen(key string, ctx context.Context) (int64, error)
	LTrim(key string, start, stop int64, ctx context.Context) error
	LSet(key string, index int64, value []byte, ctx context.Context) error
	LIndex(key string, index int64, ctx context.Context) ([]byte, error)
}

func NewRedisRepositories(client *redis.Client) *RedisRepositories {
	log.Println("🚀 Initialized Repository : Redis")
	return &RedisRepositories{
		Client: client,
	}
}

func (r *RedisRepositories) Set(key string, data []byte, expiredTime time.Duration, ctx context.Context) error {
	log.Printf("Setting Redis key: %s with expiration: %v", key, expiredTime)
	err := r.Client.Set(ctx, key, string(data), expiredTime).Err()
	if err != nil {
		log.Printf("Error setting Redis key: %v", err)
		return err
	}
	log.Printf("Successfully set Redis key: %s", key)
	return nil
}

func (r *RedisRepositories) Hset(key string, data string, expireAt time.Time, ctx context.Context) error {
	err := r.Client.Set(ctx, key, data, time.Until(expireAt)).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisRepositories) Get(key string, ctx context.Context) (string, error) {
	log.Printf("Getting Redis key: %s", key)
	result, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		log.Printf("Redis key not found: %s (this is normal for first-time access)", key)
		return "", errors.New("key does not exist (normal for first-time access)")
	} else if err != nil {
		log.Printf("Error getting Redis key: %v", err)
		return "", err
	}
	log.Printf("Successfully got Redis key: %s", key)
	return result, nil
}

func (r *RedisRepositories) Del(key string, ctx context.Context) error {
	log.Printf("Deleting Redis key: %s", key)
	_, err := r.Client.Del(ctx, key).Result()
	if err != nil {
		log.Printf("Error deleting Redis key: %v", err)
		return err
	}
	log.Printf("Successfully deleted Redis key: %s", key)
	return nil
}

// GetAllByField fetches all records and filters them using a custom filter function
func (r *RedisRepositories) GetAllByField(ctx context.Context, modelType interface{}, filterFunc func(interface{}) bool) ([]interface{}, error) {
	var results []interface{}
	var cursor uint64

	for {
		// Use SCAN to fetch keys from Redis
		keys, nextCursor, err := r.Client.Scan(ctx, cursor, "*", 10).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			// Get the value for each key
			data, err := r.Client.Get(ctx, key).Result()
			if err == redis.Nil {
				continue // Skip non-existent keys
			} else if err != nil {
				return nil, err
			}

			// Create a new instance of the model type
			model := reflect.New(reflect.TypeOf(modelType)).Interface()

			// Unmarshal JSON into the model struct
			err = json.Unmarshal([]byte(data), &model)
			if err != nil {
				continue // Skip malformed data
			}

			// Apply the filter function
			if filterFunc(model) {
				results = append(results, model)
			}
		}

		// Break if SCAN iteration is complete
		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}

	return results, nil
}

func (r *RedisRepositories) TTL(key string, ctx context.Context) (time.Duration, error) {
	duration, err := r.Client.TTL(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	return duration, nil
}

func (r *RedisRepositories) Expire(key string, expiredTime time.Duration, ctx context.Context) error {
	log.Printf("[REDIS EXPIRE] Refreshing TTL for key: %s to %v", key, expiredTime)
	return r.Client.Expire(ctx, key, expiredTime).Err()
}

// Pipeline represents a Redis pipeline
type Pipeline struct {
	pipe redis.Pipeliner
}

// StartPipeline starts a new Redis pipeline
func (r *RedisRepositories) StartPipeline(ctx context.Context) *Pipeline {
	return &Pipeline{
		pipe: r.Client.Pipeline(),
	}
}

// ExecutePipeline executes all commands in the pipeline
func (p *Pipeline) Execute(ctx context.Context) error {
	_, err := p.pipe.Exec(ctx)
	return err
}

// PipelineSet adds a SET command to the pipeline
func (p *Pipeline) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) {
	p.pipe.Set(ctx, key, value, expiration)
}

// PipelineDel adds a DEL command to the pipeline
func (p *Pipeline) Del(ctx context.Context, keys ...string) {
	p.pipe.Del(ctx, keys...)
}

// PipelineExpire adds an EXPIRE command to the pipeline
func (p *Pipeline) Expire(ctx context.Context, key string, expiration time.Duration) {
	p.pipe.Expire(ctx, key, expiration)
}

// SetCompressed compresses data using gzip before storing in Redis
func (r *RedisRepositories) SetCompressed(key string, data []byte, expiredTime time.Duration, ctx context.Context) error {
	compressed, err := utils.CompressData(data)
	if err != nil {
		log.Printf("[REDIS COMPRESSION ERROR] Failed to compress data - Key: %s, Error: %v", key, err)
		return err
	}

	savings := (1.0 - float64(len(compressed))/float64(len(data))) * 100
	log.Printf("[REDIS WRITE] Storing compressed data - Key: %s, Original: %d bytes, Compressed: %d bytes, Saved: %.2f%%, TTL: %v",
		key, len(data), len(compressed), savings, expiredTime)

	err = r.Client.Set(ctx, key, compressed, expiredTime).Err()
	if err != nil {
		log.Printf("[REDIS ERROR] Failed to store compressed data - Key: %s, Error: %v", key, err)
		return err
	}

	return nil
}

// GetCompressed retrieves and decompresses data from Redis
func (r *RedisRepositories) GetCompressed(key string, ctx context.Context) ([]byte, error) {
	log.Printf("[REDIS READ] Retrieving compressed data - Key: %s", key)
	compressed, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		log.Printf("[REDIS MISS] Key does not exist - Key: %s", key)
		return nil, errors.New("key does not exist")
	} else if err != nil {
		log.Printf("[REDIS ERROR] Failed to retrieve data - Key: %s, Error: %v", key, err)
		return nil, err
	}

	decompressed, err := utils.DecompressData(compressed)
	if err != nil {
		log.Printf("[REDIS DECOMPRESSION ERROR] Failed to decompress data - Key: %s, Error: %v", key, err)
		return nil, err
	}

	expansion := (float64(len(decompressed)) / float64(len(compressed))) * 100
	log.Printf("[REDIS READ SUCCESS] Decompressed data - Key: %s, Compressed: %d bytes, Decompressed: %d bytes, Expansion: %.2fx",
		key, len(compressed), len(decompressed), expansion/100)

	return decompressed, nil
}

// Redis List Operations for Smart Message Caching

// LPush prepends one or multiple compressed values to a list (left push - adds to beginning)
func (r *RedisRepositories) LPush(key string, values [][]byte, expiredTime time.Duration, ctx context.Context) error {
	log.Printf("[REDIS LIST] LPush to key: %s, count: %d items", key, len(values))

	// Compress each value
	compressedValues := make([]interface{}, len(values))
	for i, val := range values {
		compressed, err := utils.CompressData(val)
		if err != nil {
			log.Printf("[REDIS ERROR] Failed to compress list item: %v", err)
			return err
		}
		compressedValues[i] = compressed
	}

	err := r.Client.LPush(ctx, key, compressedValues...).Err()
	if err != nil {
		log.Printf("[REDIS ERROR] LPush failed: %v", err)
		return err
	}

	// Set expiration
	if expiredTime > 0 {
		r.Client.Expire(ctx, key, expiredTime)
	}

	log.Printf("[REDIS LIST] LPush successful: %s", key)
	return nil
}

// RPush appends one or multiple compressed values to a list (right push - adds to end)
func (r *RedisRepositories) RPush(key string, values [][]byte, expiredTime time.Duration, ctx context.Context) error {
	log.Printf("[REDIS LIST] RPush to key: %s, count: %d items", key, len(values))

	// Compress each value
	compressedValues := make([]interface{}, len(values))
	for i, val := range values {
		compressed, err := utils.CompressData(val)
		if err != nil {
			log.Printf("[REDIS ERROR] Failed to compress list item: %v", err)
			return err
		}
		compressedValues[i] = compressed
	}

	err := r.Client.RPush(ctx, key, compressedValues...).Err()
	if err != nil {
		log.Printf("[REDIS ERROR] RPush failed: %v", err)
		return err
	}

	// Set expiration
	if expiredTime > 0 {
		r.Client.Expire(ctx, key, expiredTime)
	}

	log.Printf("[REDIS LIST] RPush successful: %s", key)
	return nil
}

// LRange returns a range of compressed elements from the list (0-based, inclusive)
// Use negative indices: -1 is last element, -2 is second to last, etc.
func (r *RedisRepositories) LRange(key string, start, stop int64, ctx context.Context) ([][]byte, error) {
	log.Printf("[REDIS LIST] LRange key: %s, range: %d to %d", key, start, stop)

	compressedStrings, err := r.Client.LRange(ctx, key, start, stop).Result()
	if err != nil {
		log.Printf("[REDIS ERROR] LRange failed: %v", err)
		return nil, err
	}

	if len(compressedStrings) == 0 {
		log.Printf("[REDIS LIST] LRange returned empty list: %s", key)
		return [][]byte{}, nil
	}

	// Decompress each value
	results := make([][]byte, len(compressedStrings))
	for i, compressed := range compressedStrings {
		decompressed, err := utils.DecompressData(compressed)
		if err != nil {
			log.Printf("[REDIS ERROR] Failed to decompress list item %d: %v", i, err)
			return nil, err
		}
		results[i] = decompressed
	}

	log.Printf("[REDIS LIST] LRange successful: %s, returned %d items", key, len(results))
	return results, nil
}

// LLen returns the length of the list
func (r *RedisRepositories) LLen(key string, ctx context.Context) (int64, error) {
	len, err := r.Client.LLen(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	log.Printf("[REDIS LIST] LLen key: %s, length: %d", key, len)
	return len, nil
}

// LTrim trims the list to only keep elements in the specified range
// Useful for keeping only last N messages
func (r *RedisRepositories) LTrim(key string, start, stop int64, ctx context.Context) error {
	log.Printf("[REDIS LIST] LTrim key: %s, keeping range: %d to %d", key, start, stop)
	err := r.Client.LTrim(ctx, key, start, stop).Err()
	if err != nil {
		log.Printf("[REDIS ERROR] LTrim failed: %v", err)
		return err
	}
	log.Printf("[REDIS LIST] LTrim successful: %s", key)
	return nil
}

// LSet sets the value of an element at a specific index
func (r *RedisRepositories) LSet(key string, index int64, value []byte, ctx context.Context) error {
	log.Printf("[REDIS LIST] LSet key: %s, index: %d", key, index)

	compressed, err := utils.CompressData(value)
	if err != nil {
		log.Printf("[REDIS ERROR] Failed to compress value for LSet: %v", err)
		return err
	}

	err = r.Client.LSet(ctx, key, index, compressed).Err()
	if err != nil {
		log.Printf("[REDIS ERROR] LSet failed: %v", err)
		return err
	}

	log.Printf("[REDIS LIST] LSet successful: %s", key)
	return nil
}

// LIndex returns the element at a specific index
func (r *RedisRepositories) LIndex(key string, index int64, ctx context.Context) ([]byte, error) {
	log.Printf("[REDIS LIST] LIndex key: %s, index: %d", key, index)

	compressed, err := r.Client.LIndex(ctx, key, index).Result()
	if err == redis.Nil {
		log.Printf("[REDIS LIST] LIndex index out of range: %s[%d]", key, index)
		return nil, errors.New("index out of range")
	} else if err != nil {
		log.Printf("[REDIS ERROR] LIndex failed: %v", err)
		return nil, err
	}

	decompressed, err := utils.DecompressData(compressed)
	if err != nil {
		log.Printf("[REDIS ERROR] Failed to decompress LIndex result: %v", err)
		return nil, err
	}

	log.Printf("[REDIS LIST] LIndex successful: %s[%d]", key, index)
	return decompressed, nil
}
