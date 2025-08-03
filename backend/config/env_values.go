package config

import (
	"fmt"
	"neobase-ai/internal/constants"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Environment struct {
	// Server configs
	IsDocker                bool
	Port                    string
	Environment             string
	MaxChatsPerUser         int
	CorsAllowedOrigin       string
	ExampleDatabaseType     string
	ExampleDatabaseHost     string
	ExampleDatabasePort     string
	ExampleDatabaseName     string
	ExampleDatabaseUsername string
	ExampleDatabasePassword string
	// Auth configs
	SchemaEncryptionKey              string
	JWTSecret                        string
	JWTExpirationMilliseconds        int
	JWTRefreshExpirationMilliseconds int
	AdminUser                        string
	AdminPassword                    string
	DefaultLLMClient                 string

	// Database configs
	MongoURI          string
	MongoDatabaseName string

	// Redis configs
	RedisHost     string
	RedisPort     string
	RedisUsername string
	RedisPassword string

	// OpenAI configs
	OpenAIAPIKey              string
	OpenAIModel               string
	OpenAIMaxCompletionTokens int
	OpenAITemperature         float64

	// Gemini configs
	GeminiAPIKey              string
	GeminiModel               string
	GeminiMaxCompletionTokens int
	GeminiTemperature         float64

	// SMTP Email configs
	SMTPHost     string
	SMTPPort     int
	SMTPSecure   bool
	SMTPUser     string
	SMTPPassword string
	SMTPFromName string
	SMTPFromEmail string
}

var Env Environment

// LoadEnv loads environment variables from .env file if present
// and validates required variables
func LoadEnv() error {
	// Check if running in Docker
	Env.IsDocker = os.Getenv("IS_DOCKER") == "true"

	// Load .env file only if not running in Docker
	if !Env.IsDocker {
		if err := godotenv.Load(); err != nil {
			fmt.Printf("Warning: .env file not found: %v\n", err)
		}
	}

	// Server configs
	Env.Port = getEnvWithDefault("PORT", "3000")
	Env.Environment = getEnvWithDefault("ENVIRONMENT", "DEVELOPMENT")
	Env.MaxChatsPerUser = getIntEnvWithDefault("MAX_CHATS_PER_USER", 1)
	Env.CorsAllowedOrigin = getEnvWithDefault("CORS_ALLOWED_ORIGIN", "http://localhost:5173")
	// Auth configs
	Env.SchemaEncryptionKey = getRequiredEnv("SCHEMA_ENCRYPTION_KEY", "neobase_schema_encryption_key")
	Env.JWTSecret = getRequiredEnv("JWT_SECRET", "neobase_jwt_secret")
	Env.JWTExpirationMilliseconds = getIntEnvWithDefault("JWT_EXPIRATION_MILLISECONDS", 1000*60*60*24*10)                 // 10 days default
	Env.JWTRefreshExpirationMilliseconds = getIntEnvWithDefault("_JWT_REFRESH_EXPIRATION_MILLISECONDS", 1000*60*60*24*30) // 30 days default
	Env.AdminUser = getEnvWithDefault("NEOBASE_ADMIN_USERNAME", "bhaskar")
	Env.AdminPassword = getEnvWithDefault("NEOBASE_ADMIN_PASSWORD", "bhaskar")

	// Database configs
	Env.MongoURI = getRequiredEnv("NEOBASE_MONGODB_URI", "mongodb://localhost:27017/neobase")
	Env.MongoDatabaseName = getRequiredEnv("NEOBASE_MONGODB_NAME", "neobase")
	Env.RedisHost = getRequiredEnv("NEOBASE_REDIS_HOST", "localhost")
	Env.RedisPort = getRequiredEnv("NEOBASE_REDIS_PORT", "6379")
	Env.RedisUsername = getRequiredEnv("NEOBASE_REDIS_USERNAME", "neobase")
	Env.RedisPassword = getRequiredEnv("NEOBASE_REDIS_PASSWORD", "default")

	// Example DB For Development Environment
	Env.ExampleDatabaseType = getRequiredEnv("EXAMPLE_DB_TYPE", "postgres")
	Env.ExampleDatabaseHost = getRequiredEnv("EXAMPLE_DB_HOST", "localhost")
	Env.ExampleDatabasePort = getRequiredEnv("EXAMPLE_DB_PORT", "5432")
	Env.ExampleDatabaseName = getRequiredEnv("EXAMPLE_DB_NAME", "postgres")
	Env.ExampleDatabaseUsername = getRequiredEnv("EXAMPLE_DB_USERNAME", "")
	Env.ExampleDatabasePassword = getRequiredEnv("EXAMPLE_DB_PASSWORD", "")

	// LLM configs
	Env.DefaultLLMClient = getEnvWithDefault("DEFAULT_LLM_CLIENT", constants.OpenAI)

	// OpenAI configs
	Env.OpenAIAPIKey = getRequiredEnv("OPENAI_API_KEY", "")
	Env.OpenAIModel = getEnvWithDefault("OPENAI_MODEL", constants.OpenAIModel)
	Env.OpenAIMaxCompletionTokens = getIntEnvWithDefault("OPENAI_MAX_COMPLETION_TOKENS", constants.OpenAIMaxCompletionTokens)
	Env.OpenAITemperature = getFloatEnvWithDefault("OPENAI_TEMPERATURE", constants.OpenAITemperature)

	// Gemini configs
	Env.GeminiAPIKey = getRequiredEnv("GEMINI_API_KEY", "")
	Env.GeminiModel = getEnvWithDefault("GEMINI_MODEL", constants.GeminiModel)
	Env.GeminiMaxCompletionTokens = getIntEnvWithDefault("GEMINI_MAX_COMPLETION_TOKENS", constants.GeminiMaxCompletionTokens)
	Env.GeminiTemperature = getFloatEnvWithDefault("GEMINI_TEMPERATURE", constants.GeminiTemperature)

	// SMTP Email configs
	Env.SMTPHost = getEnvWithDefault("SMTP_HOST", "")
	Env.SMTPPort = getIntEnvWithDefault("SMTP_PORT", 587)
	Env.SMTPSecure = getEnvWithDefault("SMTP_SECURE", "false") == "true"
	Env.SMTPUser = getEnvWithDefault("SMTP_USER", "")
	Env.SMTPPassword = getEnvWithDefault("SMTP_PASSWORD", "")
	Env.SMTPFromName = getEnvWithDefault("SMTP_FROM_NAME", "NeoBase")
	Env.SMTPFromEmail = getEnvWithDefault("SMTP_FROM_EMAIL", Env.SMTPUser)

	return validateConfig()
}

// Helper functions to get environment variables with defaults and validation
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getRequiredEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnvWithDefault(key string, defaultValue int) int {
	strValue := os.Getenv(key)
	if strValue == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(strValue)
	if err != nil {
		fmt.Printf("Warning: Invalid value for %s, using default: %d\n", key, defaultValue)
		return defaultValue
	}
	return value
}

func getFloatEnvWithDefault(key string, defaultValue float64) float64 {
	strValue := os.Getenv(key)
	if strValue == "" {
		return defaultValue
	}
	return defaultValue
}

func validateConfig() error {
	// Validate MongoDB URI format
	if !isValidURI(Env.MongoURI) {
		return fmt.Errorf("invalid MONGODB_URI format: %s", Env.MongoURI)
	}

	// Validate JWT expiration
	if Env.JWTExpirationMilliseconds <= 0 {
		return fmt.Errorf("JWT_EXPIRATION_MILLISECONDS must be positive, got: %d", Env.JWTExpirationMilliseconds)
	}

	if Env.AdminUser == "neobase-admin" || Env.AdminPassword == "neobase-password" {
		return fmt.Errorf("default credentials: neobase-admin and neobase-password should not be used")
	}

	return nil
}

func isValidURI(uri string) bool {
	return len(uri) > 0 && (len(uri) > 10)
}
