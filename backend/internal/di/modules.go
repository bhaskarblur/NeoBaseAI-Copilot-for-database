package di

import (
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/handlers"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/repositories"
	"neobase-ai/internal/services"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/dbmanager"
	"neobase-ai/pkg/llm"
	"neobase-ai/pkg/mongodb"
	"neobase-ai/pkg/redis"
	"time"

	"go.uber.org/dig"
)

var DiContainer *dig.Container

func Initialize() {
	DiContainer = dig.New()

	// Initialize MongoDB
	dbConfig := mongodb.MongoDbConfigModel{
		ConnectionUrl: config.Env.MongoURI,
		DatabaseName:  config.Env.MongoDatabaseName,
	}
	mongodbClient := mongodb.InitializeDatabaseConnection(dbConfig)

	// Initialize Redis
	redisClient, err := redis.RedisClient(config.Env.RedisHost, config.Env.RedisPort, config.Env.RedisUsername, config.Env.RedisPassword)
	if err != nil {
		log.Fatalf("Failed to initialize Redis client: %v", err)
	}

	// Initialize services and repositories
	redisRepo := redis.NewRedisRepositories(redisClient)
	jwtService := utils.NewJWTService(
		config.Env.JWTSecret,
		time.Millisecond*time.Duration(config.Env.JWTExpirationMilliseconds),
		time.Millisecond*time.Duration(config.Env.JWTRefreshExpirationMilliseconds),
	)

	// Initialize token repository
	tokenRepo := repositories.NewTokenRepository(redisRepo)

	chatRepo := repositories.NewChatRepository(mongodbClient)
	llmRepo := repositories.NewLLMMessageRepository(mongodbClient)

	// Provide all dependencies to the container
	if err := DiContainer.Provide(func() *mongodb.MongoDBClient { return mongodbClient }); err != nil {
		log.Fatalf("Failed to provide MongoDB client: %v", err)
	}

	if err := DiContainer.Provide(func() redis.IRedisRepositories { return redisRepo }); err != nil {
		log.Fatalf("Failed to provide Redis repositories: %v", err)
	}

	if err := DiContainer.Provide(func() utils.JWTService { return jwtService }); err != nil {
		log.Fatalf("Failed to provide JWT service: %v", err)
	}

	if err := DiContainer.Provide(func() repositories.ChatRepository { return chatRepo }); err != nil {
		log.Fatalf("Failed to provide chat repository: %v", err)
	}

	if err := DiContainer.Provide(func() repositories.LLMMessageRepository { return llmRepo }); err != nil {
		log.Fatalf("Failed to provide LLM message repository: %v", err)
	}

	// Provide DB Manager
	if err := DiContainer.Provide(func(redisRepo redis.IRedisRepositories) (*dbmanager.Manager, error) {
		encryptionKey := config.Env.SchemaEncryptionKey
		manager, err := dbmanager.NewManager(redisRepo, encryptionKey)
		if err != nil {
			log.Fatalf("Failed to provide DB manager: %v", err)
		}
		// Register database drivers
		manager.RegisterDriver(constants.DatabaseTypePostgreSQL, dbmanager.NewPostgresDriver())
		manager.RegisterDriver(constants.DatabaseTypeYugabyteDB, dbmanager.NewPostgresDriver()) // Use same driver for both
		manager.RegisterDriver(constants.DatabaseTypeMySQL, dbmanager.NewMySQLDriver())
		manager.RegisterDriver(constants.DatabaseTypeClickhouse, dbmanager.NewClickHouseDriver())
		manager.RegisterDriver(constants.DatabaseTypeMongoDB, dbmanager.NewMongoDBDriver())
		manager.RegisterDriver(constants.DatabaseTypeSpreadsheet, dbmanager.NewSpreadsheetDriver())

		// Register schema fetchers
		manager.RegisterFetcher(constants.DatabaseTypePostgreSQL, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
			return &dbmanager.PostgresDriver{}
		})
		manager.RegisterFetcher(constants.DatabaseTypeYugabyteDB, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
			return &dbmanager.PostgresDriver{}
		})
		manager.RegisterFetcher(constants.DatabaseTypeMySQL, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
			return dbmanager.NewMySQLSchemaFetcher(db)
		})
		manager.RegisterFetcher(constants.DatabaseTypeClickhouse, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
			return &dbmanager.ClickHouseDriver{}
		})
		manager.RegisterFetcher(constants.DatabaseTypeMongoDB, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
			return &dbmanager.MongoDBDriver{}
		})
		manager.RegisterFetcher(constants.DatabaseTypeSpreadsheet, func(db dbmanager.DBExecutor) dbmanager.SchemaFetcher {
			return &dbmanager.PostgresDriver{}
		})

		return manager, nil
	}); err != nil {
		log.Fatalf("Failed to provide DB manager: %v", err)
	}

	if err := DiContainer.Provide(func(db *mongodb.MongoDBClient, redisRepo redis.IRedisRepositories) repositories.UserRepository {
		return repositories.NewUserRepository(db, redisRepo)
	}); err != nil {
		log.Fatalf("Failed to provide user repository: %v", err)
	}

	if err := DiContainer.Provide(func() repositories.TokenRepository { return tokenRepo }); err != nil {
		log.Fatalf("Failed to provide token repository: %v", err)
	}

	// Provide email service
	if err := DiContainer.Provide(func() services.EmailService {
		return services.NewEmailService()
	}); err != nil {
		log.Fatalf("Failed to provide email service: %v", err)
	}

	// Provide Google OAuth service
	if err := DiContainer.Provide(func() services.GoogleOAuthService {
		return services.NewGoogleOAuthService()
	}); err != nil {
		log.Fatalf("Failed to provide Google OAuth service: %v", err)
	}

	// Provide waitlist repository
	if err := DiContainer.Provide(func(db *mongodb.MongoDBClient) *repositories.WaitlistRepository {
		return repositories.NewWaitlistRepository(db.Client.Database(db.Config.DatabaseName))
	}); err != nil {
		log.Fatalf("Failed to provide waitlist repository: %v", err)
	}

	// Provide waitlist service
	if err := DiContainer.Provide(func(waitlistRepo *repositories.WaitlistRepository, emailService services.EmailService) *services.WaitlistService {
		return services.NewWaitlistService(waitlistRepo, emailService)
	}); err != nil {
		log.Fatalf("Failed to provide waitlist service: %v", err)
	}

	// Provide waitlist handler
	if err := DiContainer.Provide(func(waitlistService *services.WaitlistService) *handlers.WaitlistHandler {
		return handlers.NewWaitlistHandler(waitlistService)
	}); err != nil {
		log.Fatalf("Failed to provide waitlist handler: %v", err)
	}

	// Provide services
	if err := DiContainer.Provide(func(userRepo repositories.UserRepository, tokenRepo repositories.TokenRepository, jwt utils.JWTService, emailService services.EmailService, googleOAuthService services.GoogleOAuthService) services.AuthService {
		return services.NewAuthService(userRepo, jwt, tokenRepo, emailService, googleOAuthService)
	}); err != nil {
		log.Fatalf("Failed to provide auth service: %v", err)
	}

	// Add LLM Manager
	if err := DiContainer.Provide(func() *llm.Manager {
		manager := llm.NewManager()

		// Register OpenAI client if API key is available
		if config.Env.OpenAIAPIKey != "" {
			// Get default OpenAI model from supported models
			defaultOpenAIModel := constants.GetDefaultModelForProvider(constants.OpenAI)
			if defaultOpenAIModel == nil {
				log.Printf("Warning: No default OpenAI model found")
				defaultOpenAIModel = &constants.LLMModel{
					ID:                  "gpt-4o",
					Provider:            constants.OpenAI,
					MaxCompletionTokens: 30000,
					Temperature:         1,
				}
			}

			err := manager.RegisterClient(constants.OpenAI, llm.Config{
				Provider:            constants.OpenAI,
				Model:               defaultOpenAIModel.ID,
				APIKey:              config.Env.OpenAIAPIKey,
				MaxCompletionTokens: defaultOpenAIModel.MaxCompletionTokens,
				Temperature:         defaultOpenAIModel.Temperature,
				DBConfigs: []llm.LLMDBConfig{
					{
						DBType:       constants.DatabaseTypePostgreSQL,
						Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypePostgreSQL),
						SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypePostgreSQL, false),
					},
					{
						DBType:       constants.DatabaseTypeYugabyteDB,
						Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypeYugabyteDB),
						SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypeYugabyteDB, false),
					},
					{
						DBType:       constants.DatabaseTypeMySQL,
						Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypeMySQL),
						SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypeMySQL, false),
					},
					{
						DBType:       constants.DatabaseTypeClickhouse,
						Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypeClickhouse),
						SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypeClickhouse, false),
					},
					{
						DBType:       constants.DatabaseTypeMongoDB,
						Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypeMongoDB),
						SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypeMongoDB, false),
					},
					{
						DBType:       constants.DatabaseTypeSpreadsheet,
						Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypeSpreadsheet),
						SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypeSpreadsheet, false),
					},
				},
			})
			if err != nil {
				log.Printf("Warning: Failed to register OpenAI client: %v", err)
			}
		}

		// Register Gemini client if API key is available
		if config.Env.GeminiAPIKey != "" {
			// Get default Gemini model from supported models
			defaultGeminiModel := constants.GetDefaultModelForProvider(constants.Gemini)
			if defaultGeminiModel == nil {
				log.Printf("Warning: No default Gemini model found")
				defaultGeminiModel = &constants.LLMModel{
					ID:                  "gemini-2.5-flash",
					Provider:            constants.Gemini,
					MaxCompletionTokens: 100000,
					Temperature:         1,
				}
			}

			err := manager.RegisterClient(constants.Gemini, llm.Config{
				Provider:            constants.Gemini,
				Model:               defaultGeminiModel.ID,
				APIKey:              config.Env.GeminiAPIKey,
				MaxCompletionTokens: defaultGeminiModel.MaxCompletionTokens,
				Temperature:         defaultGeminiModel.Temperature,
				DBConfigs: []llm.LLMDBConfig{
					{
						DBType:       constants.DatabaseTypePostgreSQL,
						Schema:       constants.GetLLMResponseSchema(constants.Gemini, constants.DatabaseTypePostgreSQL),
						SystemPrompt: constants.GetSystemPrompt(constants.Gemini, constants.DatabaseTypePostgreSQL, false),
					},
					{
						DBType:       constants.DatabaseTypeYugabyteDB,
						Schema:       constants.GetLLMResponseSchema(constants.Gemini, constants.DatabaseTypeYugabyteDB),
						SystemPrompt: constants.GetSystemPrompt(constants.Gemini, constants.DatabaseTypeYugabyteDB, false),
					},
					{
						DBType:       constants.DatabaseTypeMySQL,
						Schema:       constants.GetLLMResponseSchema(constants.Gemini, constants.DatabaseTypeMySQL),
						SystemPrompt: constants.GetSystemPrompt(constants.Gemini, constants.DatabaseTypeMySQL, false),
					},
					{
						DBType:       constants.DatabaseTypeClickhouse,
						Schema:       constants.GetLLMResponseSchema(constants.Gemini, constants.DatabaseTypeClickhouse),
						SystemPrompt: constants.GetSystemPrompt(constants.Gemini, constants.DatabaseTypeClickhouse, false),
					},
					{
						DBType:       constants.DatabaseTypeMongoDB,
						Schema:       constants.GetLLMResponseSchema(constants.Gemini, constants.DatabaseTypeMongoDB),
						SystemPrompt: constants.GetSystemPrompt(constants.Gemini, constants.DatabaseTypeMongoDB, false),
					},
					{
						DBType:       constants.DatabaseTypeSpreadsheet,
						Schema:       constants.GetLLMResponseSchema(constants.Gemini, constants.DatabaseTypeSpreadsheet),
						SystemPrompt: constants.GetSystemPrompt(constants.Gemini, constants.DatabaseTypeSpreadsheet, false),
					},
				},
			})
			if err != nil {
				log.Printf("Warning: Failed to register Gemini client: %v", err)
			}
		}

		// Register Claude client if API key is available
		if config.Env.ClaudeAPIKey != "" {
			// Get default Claude model from supported models
			defaultClaudeModel := constants.GetDefaultModelForProvider(constants.Claude)
			if defaultClaudeModel == nil {
				log.Printf("Warning: No default Claude model found")
				defaultClaudeModel = &constants.LLMModel{
					ID:                  "claude-3-5-sonnet-20241022",
					Provider:            constants.Claude,
					MaxCompletionTokens: 8192,
					Temperature:         1,
				}
			}

			err := manager.RegisterClient(constants.Claude, llm.Config{
				Provider:            constants.Claude,
				Model:               defaultClaudeModel.ID,
				APIKey:              config.Env.ClaudeAPIKey,
				MaxCompletionTokens: defaultClaudeModel.MaxCompletionTokens,
				Temperature:         defaultClaudeModel.Temperature,
				DBConfigs: []llm.LLMDBConfig{
					{
						DBType:       constants.DatabaseTypePostgreSQL,
						Schema:       constants.GetLLMResponseSchema(constants.Claude, constants.DatabaseTypePostgreSQL),
						SystemPrompt: constants.GetSystemPrompt(constants.Claude, constants.DatabaseTypePostgreSQL, false),
					},
					{
						DBType:       constants.DatabaseTypeYugabyteDB,
						Schema:       constants.GetLLMResponseSchema(constants.Claude, constants.DatabaseTypeYugabyteDB),
						SystemPrompt: constants.GetSystemPrompt(constants.Claude, constants.DatabaseTypeYugabyteDB, false),
					},
					{
						DBType:       constants.DatabaseTypeMySQL,
						Schema:       constants.GetLLMResponseSchema(constants.Claude, constants.DatabaseTypeMySQL),
						SystemPrompt: constants.GetSystemPrompt(constants.Claude, constants.DatabaseTypeMySQL, false),
					},
					{
						DBType:       constants.DatabaseTypeClickhouse,
						Schema:       constants.GetLLMResponseSchema(constants.Claude, constants.DatabaseTypeClickhouse),
						SystemPrompt: constants.GetSystemPrompt(constants.Claude, constants.DatabaseTypeClickhouse, false),
					},
					{
						DBType:       constants.DatabaseTypeMongoDB,
						Schema:       constants.GetLLMResponseSchema(constants.Claude, constants.DatabaseTypeMongoDB),
						SystemPrompt: constants.GetSystemPrompt(constants.Claude, constants.DatabaseTypeMongoDB, false),
					},
					{
						DBType:       constants.DatabaseTypeSpreadsheet,
						Schema:       constants.GetLLMResponseSchema(constants.Claude, constants.DatabaseTypeSpreadsheet),
						SystemPrompt: constants.GetSystemPrompt(constants.Claude, constants.DatabaseTypeSpreadsheet, false),
					},
				},
			})
			if err != nil {
				log.Printf("Warning: Failed to register Claude client: %v", err)
			}
		}

		// Register Ollama client if base URL is available
		if config.Env.OllamaBaseURL != "" {
			// Get default Ollama model from supported models
			defaultOllamaModel := constants.GetDefaultModelForProvider(constants.Ollama)
			if defaultOllamaModel == nil {
				log.Printf("Warning: No default Ollama model found")
				defaultOllamaModel = &constants.LLMModel{
					ID:                  "llama3.1:latest",
					Provider:            constants.Ollama,
					MaxCompletionTokens: 4096,
					Temperature:         1,
				}
			}

			err := manager.RegisterClient(constants.Ollama, llm.Config{
				Provider:            constants.Ollama,
				Model:               defaultOllamaModel.ID,
				APIKey:              config.Env.OllamaBaseURL, // Use APIKey field for base URL
				MaxCompletionTokens: defaultOllamaModel.MaxCompletionTokens,
				Temperature:         defaultOllamaModel.Temperature,
				DBConfigs: []llm.LLMDBConfig{
					{
						DBType:       constants.DatabaseTypePostgreSQL,
						Schema:       constants.GetLLMResponseSchema(constants.Ollama, constants.DatabaseTypePostgreSQL),
						SystemPrompt: constants.GetSystemPrompt(constants.Ollama, constants.DatabaseTypePostgreSQL, false),
					},
					{
						DBType:       constants.DatabaseTypeYugabyteDB,
						Schema:       constants.GetLLMResponseSchema(constants.Ollama, constants.DatabaseTypeYugabyteDB),
						SystemPrompt: constants.GetSystemPrompt(constants.Ollama, constants.DatabaseTypeYugabyteDB, false),
					},
					{
						DBType:       constants.DatabaseTypeMySQL,
						Schema:       constants.GetLLMResponseSchema(constants.Ollama, constants.DatabaseTypeMySQL),
						SystemPrompt: constants.GetSystemPrompt(constants.Ollama, constants.DatabaseTypeMySQL, false),
					},
					{
						DBType:       constants.DatabaseTypeClickhouse,
						Schema:       constants.GetLLMResponseSchema(constants.Ollama, constants.DatabaseTypeClickhouse),
						SystemPrompt: constants.GetSystemPrompt(constants.Ollama, constants.DatabaseTypeClickhouse, false),
					},
					{
						DBType:       constants.DatabaseTypeMongoDB,
						Schema:       constants.GetLLMResponseSchema(constants.Ollama, constants.DatabaseTypeMongoDB),
						SystemPrompt: constants.GetSystemPrompt(constants.Ollama, constants.DatabaseTypeMongoDB, false),
					},
					{
						DBType:       constants.DatabaseTypeSpreadsheet,
						Schema:       constants.GetLLMResponseSchema(constants.Ollama, constants.DatabaseTypeSpreadsheet),
						SystemPrompt: constants.GetSystemPrompt(constants.Ollama, constants.DatabaseTypeSpreadsheet, false),
					},
				},
			})
			if err != nil {
				log.Printf("Warning: Failed to register Ollama client: %v", err)
			}
		}

		return manager
	}); err != nil {
		log.Fatalf("Failed to provide LLM manager: %v", err)
	}

	// Update Chat Service provider to include DB manager setup
	if err := DiContainer.Provide(func(
		chatRepo repositories.ChatRepository,
		llmRepo repositories.LLMMessageRepository,
		dbManager *dbmanager.Manager,
		llmManager *llm.Manager,
		redisRepo redis.IRedisRepositories,
	) services.ChatService {
		// Get a default LLM client - try in order of preference
		var llmClient llm.Client
		var err error

		// Try OpenAI first
		if config.Env.OpenAIAPIKey != "" {
			llmClient, err = llmManager.GetClient(constants.OpenAI)
			if err == nil {
				log.Printf("Using OpenAI as default LLM client")
			}
		}

		// If OpenAI not available, try Gemini
		if llmClient == nil && config.Env.GeminiAPIKey != "" {
			llmClient, err = llmManager.GetClient(constants.Gemini)
			if err == nil {
				log.Printf("Using Gemini as default LLM client")
			}
		}

		// If still no client, warn but continue (chat will fail gracefully)
		if llmClient == nil {
			log.Printf("Warning: No LLM client available. Please configure OPENAI_API_KEY or GEMINI_API_KEY")
		}

		chatService := services.NewChatService(chatRepo, llmRepo, dbManager, llmClient, llmManager, redisRepo)

		// Set chat service as stream handler for DB manager
		dbManager.SetStreamHandler(chatService)

		// Set chat service in auth service
		err = DiContainer.Invoke(func(authService services.AuthService) {
			authService.SetChatService(chatService)
		})
		if err != nil {
			log.Fatalf("Failed to set chat service in auth service: %v", err)
		}
		return chatService
	}); err != nil {
		log.Fatalf("Failed to provide chat service: %v", err)
	}

	if err := DiContainer.Provide(func(redisRepo redis.IRedisRepositories) services.GitHubService {
		return services.NewGitHubService(redisRepo)
	}); err != nil {
		log.Fatalf("Failed to provide github handler: %v", err)
	}

	// Provide handlers
	if err := DiContainer.Provide(func(authService services.AuthService) *handlers.AuthHandler {
		return handlers.NewAuthHandler(authService)
	}); err != nil {
		log.Fatalf("Failed to provide auth handler: %v", err)
	}

	if err := DiContainer.Provide(func(githubService services.GitHubService) *handlers.GitHubHandler {
		return handlers.NewGitHubHandler(githubService)
	}); err != nil {
		log.Fatalf("Failed to provide github handler: %v", err)
	}

	// Chat Handler
	if err := DiContainer.Provide(func(
		chatService services.ChatService,
	) *handlers.ChatHandler {
		handler := handlers.NewChatHandler(chatService)
		chatService.SetStreamHandler(handler)
		return handler
	}); err != nil {
		log.Fatalf("Failed to provide chat handler: %v", err)
	}
}

// GetAuthHandler retrieves the AuthHandler from the DI container
func GetAuthHandler() (*handlers.AuthHandler, error) {
	var handler *handlers.AuthHandler
	err := DiContainer.Invoke(func(h *handlers.AuthHandler) {
		handler = h
	})
	if err != nil {
		return nil, err
	}
	return handler, nil
}

// GetChatHandler retrieves the ChatHandler from the DI container
func GetChatHandler() (*handlers.ChatHandler, error) {
	var handler *handlers.ChatHandler
	err := DiContainer.Invoke(func(h *handlers.ChatHandler) {
		handler = h
	})
	if err != nil {
		return nil, err
	}
	return handler, nil
}

// GetGitHubHandler retrieves the GitHubHandler from the DI container
func GetGitHubHandler() (*handlers.GitHubHandler, error) {
	var handler *handlers.GitHubHandler
	err := DiContainer.Invoke(func(h *handlers.GitHubHandler) {
		handler = h
	})
	if err != nil {
		return nil, err
	}
	return handler, nil
}

// GetWaitlistHandler retrieves the WaitlistHandler from the DI container
func GetWaitlistHandler() (*handlers.WaitlistHandler, error) {
	var handler *handlers.WaitlistHandler
	err := DiContainer.Invoke(func(h *handlers.WaitlistHandler) {
		handler = h
	})
	if err != nil {
		return nil, err
	}
	return handler, nil
}
