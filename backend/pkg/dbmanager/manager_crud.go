package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	// Database drivers

	_ "github.com/lib/pq" // PostgreSQL/YugabyteDB Driver
	"gorm.io/gorm"

	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/redis"
)

const (
	cleanupInterval     = 10 * time.Minute   // Check every 10 minutes
	idleTimeout         = 15 * time.Minute   // Close after 15 minutes of inactivity
	schemaCheckInterval = 3 * 24 * time.Hour // Check after every 3 days
)

type cleanupMetrics struct {
	lastRun            time.Time
	connectionsRemoved int
	executionsRemoved  int
}

// DatabasePool represents a shared database connection with reference counting
type DatabasePool struct {
	DB         *sql.DB
	GORMDB     *gorm.DB
	RefCount   int
	Config     ConnectionConfig
	LastUsed   time.Time
	Mutex      sync.Mutex // For thread-safe reference counting
	MongoDBObj interface{}
}

// Manager handles database connections
type Manager struct {
	connections      map[string]*Connection    // chatID -> connection
	drivers          map[string]DatabaseDriver // type -> driver
	mu               sync.RWMutex
	redisRepo        redis.IRedisRepositories
	stopCleanup      chan struct{} // Channel to stop cleanup routine
	eventChan        chan SSEEvent // Channel for SSE events
	schemaManager    *SchemaManager
	streamHandler    StreamHandler              // Changed from *StreamHandler to StreamHandler
	activeExecutions map[string]*QueryExecution // key: streamID
	executionMu      sync.RWMutex
	cleanupMetrics   cleanupMetrics
	fetchers         map[string]FetcherFactory
	fetchersMu       sync.RWMutex
	dbPools          map[string]*DatabasePool // key: hash of connection config
	dbPoolsMu        sync.RWMutex
	poolMetrics      struct {
		totalPools       int
		totalConnections int
		reuseCount       int
	}
}

// NewManager creates a new connection manager
func NewManager(redisRepo redis.IRedisRepositories, encryptionKey string) (*Manager, error) {
	schemaManager, err := NewSchemaManager(redisRepo, encryptionKey, nil)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		connections:      make(map[string]*Connection),
		drivers:          make(map[string]DatabaseDriver),
		redisRepo:        redisRepo,
		stopCleanup:      make(chan struct{}),
		eventChan:        make(chan SSEEvent, 100),
		schemaManager:    schemaManager,
		activeExecutions: make(map[string]*QueryExecution),
		executionMu:      sync.RWMutex{},
		fetchers:         make(map[string]FetcherFactory),
		dbPools:          make(map[string]*DatabasePool),
	}

	// Set the DBManager in the SchemaManager
	schemaManager.SetDBManager(m)

	// Start cleanup routine in a separate goroutine with error handling
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("DBManager -> Cleanup routine panic recovered: %v", r)
				// Restart the cleanup routine
				go m.startCleanupRoutine()
			}
		}()
		m.startCleanupRoutine()
	}()

	// Register default fetchers
	m.RegisterFetcher("postgresql", func(db DBExecutor) SchemaFetcher {
		return &PostgresDriver{}
	})

	m.RegisterFetcher("yugabytedb", func(db DBExecutor) SchemaFetcher {
		return &PostgresDriver{}
	})

	// Add MySQL schema fetcher registration
	m.RegisterFetcher("mysql", func(db DBExecutor) SchemaFetcher {
		return NewMySQLSchemaFetcher(db)
	})

	// Add ClickHouse schema fetcher registration
	m.RegisterFetcher("clickhouse", func(db DBExecutor) SchemaFetcher {
		return NewClickHouseSchemaFetcher(db)
	})

	m.RegisterFetcher("mongodb", func(db DBExecutor) SchemaFetcher {
		return NewMongoDBSchemaFetcher(db)
	})

	m.registerDefaultDrivers()

	return m, nil
}

func (m *Manager) registerDefaultDrivers() {
	// Register PostgreSQL driver
	m.RegisterDriver("postgresql", NewPostgresDriver())

	// Register YugabyteDB driver (uses PostgreSQL driver)
	m.RegisterDriver("yugabytedb", NewPostgresDriver())

	// Register MySQL driver
	m.RegisterDriver("mysql", NewMySQLDriver())

	// Register ClickHouse driver
	m.RegisterDriver("clickhouse", NewClickHouseDriver())

	// Register MongoDB driver
	m.RegisterDriver("mongodb", NewMongoDBDriver())

	// Register MongoDB schema fetcher
	m.RegisterFetcher("mongodb", func(db DBExecutor) SchemaFetcher {
		return NewMongoDBSchemaFetcher(db)
	})

	// Register Spreadsheet (CSV/Excel) driver
	m.RegisterDriver("spreadsheet", NewSpreadsheetDriver())
}

// GetPoolMetrics returns metrics about the connection pools
func (m *Manager) GetPoolMetrics() map[string]interface{} {
	m.dbPoolsMu.RLock()
	defer m.dbPoolsMu.RUnlock()

	totalRefs := 0
	for _, pool := range m.dbPools {
		pool.Mutex.Lock()
		totalRefs += pool.RefCount
		pool.Mutex.Unlock()
	}

	return map[string]interface{}{
		"total_pools":       len(m.dbPools),
		"total_connections": totalRefs,
		"reuse_count":       m.poolMetrics.reuseCount,
	}
}

// RegisterDriver registers a new database driver
func (m *Manager) RegisterDriver(dbType string, driver DatabaseDriver) {
	m.drivers[dbType] = driver
	log.Printf("DBManager -> Registered driver for type: %s", dbType)
}

// RegisterFetcher registers a schema fetcher for a database type
func (m *Manager) RegisterFetcher(dbType string, factory FetcherFactory) {
	m.fetchersMu.Lock()
	defer m.fetchersMu.Unlock()
	m.fetchers[dbType] = factory
}

// Connect creates a new database connection
func (m *Manager) Connect(chatID, userID, streamID string, config ConnectionConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("DBManager -> Connect -> Starting connection for chatID: %s", chatID)

	// Get existing subscribers if connection exists
	var existingSubscribers map[string]bool
	if existingConn, exists := m.connections[chatID]; exists {
		existingConn.SubLock.RLock()
		existingSubscribers = make(map[string]bool)
		for id := range existingConn.Subscribers {
			existingSubscribers[id] = true
		}
		existingConn.SubLock.RUnlock()
		log.Printf("DBManager -> Connect -> Preserving existing subscribers: %+v", existingSubscribers)
	}

	// Generate a unique key for this database configuration
	configKey := utils.GenerateConfigKey(map[string]interface{}{
		"type":     config.Type,
		"host":     config.Host,
		"port":     config.Port,
		"username": config.Username,
		"password": config.Password,
		"database": config.Database, // Add database to the key to differentiate connections to different databases
	})
	log.Printf("DBManager -> Connect -> Generated config key: %s", configKey)

	// Check if we already have a connection to this database
	var conn *Connection
	var err error

	m.dbPoolsMu.RLock()
	pool, poolExists := m.dbPools[configKey]
	m.dbPoolsMu.RUnlock()

	// Get appropriate driver
	driver, exists := m.drivers[config.Type]
	if !exists {
		log.Printf("DBManager -> Connect -> No driver found for type: %s", config.Type)
		return fmt.Errorf("unsupported data source type: %s", config.Type)
	}

	log.Printf("DBManager -> Connect -> Found driver for type: %s", config.Type)

	// Check if connection already exists
	if existingConn, exists := m.connections[chatID]; exists && existingConn.Status == StatusConnected {
		log.Printf("DBManager -> Connect -> Connection already exists for chatID: %s", chatID)
		return fmt.Errorf("connection already exists for chat ID: %s", chatID)
	}

	if poolExists {
		// Use existing connection from pool
		pool.Mutex.Lock()
		pool.RefCount++
		pool.LastUsed = time.Now()
		pool.Mutex.Unlock()

		log.Printf("DBManager -> Connect -> Reusing existing connection from pool, refCount: %d", pool.RefCount)
		log.Printf("DBManager -> Connect -> Pool config: Type=%s, Host=%s, Database=%s",
			pool.Config.Type, pool.Config.Host, pool.Config.Database)
		log.Printf("DBManager -> Connect -> New connection config: Type=%s, Host=%s, Database=%s",
			config.Type, config.Host, config.Database)

		// Validate that we're connecting to the same database
		if pool.Config.Database != config.Database {
			log.Printf("DBManager -> Connect -> WARNING: Pool database (%s) doesn't match requested database (%s)",
				pool.Config.Database, config.Database)
		}

		log.Printf("DBManager -> Connect -> Connection config: %+v", config)
		// Create a new connection using the shared pool
		conn = &Connection{
			DB:          pool.GORMDB,
			LastUsed:    time.Now(),
			Status:      StatusConnected,
			Config:      config,
			UserID:      userID,
			ChatID:      chatID,
			StreamID:    streamID,
			Subscribers: make(map[string]bool),
			SubLock:     sync.RWMutex{},
			ConfigKey:   configKey, // Store the config key for reference
		}

		// Set MongoDBObj for MongoDB connections when reusing from pool
		if config.Type == "mongodb" && pool.MongoDBObj != nil {
			conn.MongoDBObj = pool.MongoDBObj
			log.Printf("DBManager -> Connect -> Set MongoDBObj from pool for MongoDB connection")
		}

		// Update metrics
		m.poolMetrics.reuseCount++
		
		// For spreadsheet connections from pool, ensure schema exists
		if config.Type == "spreadsheet" && chatID != "" {
			schemaName := fmt.Sprintf("conn_%s", chatID)
			// Store schema name in config
			conn.Config.SchemaName = schemaName
		}
	} else {
		// Create a new connection
		conn, err = driver.Connect(config)
		if err != nil {
			log.Printf("DBManager -> Connect -> Driver connection failed: %v", err)
			return err
		}

		log.Printf("DBManager -> Connect -> Connection Host, Name, Type: %+v, %+v, %+v", config.Host, config.Database, config.Type)
		log.Printf("DBManager -> Connect -> Driver connection successful, creating new pool")
		// Create and store the new pool
		newPool := &DatabasePool{
			DB:       nil, // The driver doesn't expose sql.DB directly
			GORMDB:   conn.DB,
			RefCount: 1,
			Config:   config,
			LastUsed: time.Now(),
		}

		// For MongoDB, store the MongoDB client in the pool
		if config.Type == "mongodb" {
			newPool.MongoDBObj = conn.MongoDBObj
		}

		m.dbPoolsMu.Lock()
		m.dbPools[configKey] = newPool
		m.dbPoolsMu.Unlock()

		// Update metrics
		m.poolMetrics.totalPools++

		// Initialize connection fields
		conn.LastUsed = time.Now()
		conn.Status = StatusConnected
		conn.Config = config
		conn.UserID = userID
		conn.ChatID = chatID
		conn.StreamID = streamID
		conn.ConfigKey = configKey
	}

	// For spreadsheet connections, create the schema after ChatID is set
	if config.Type == "spreadsheet" && chatID != "" {
		schemaName := fmt.Sprintf("conn_%s", chatID)
		if err := m.createSpreadsheetSchema(conn, schemaName); err != nil {
			// Clean up on error
			if !poolExists {
				// Only disconnect if we created a new connection
				driver.Disconnect(conn)
				m.dbPoolsMu.Lock()
				delete(m.dbPools, configKey)
				m.dbPoolsMu.Unlock()
			}
			return fmt.Errorf("failed to create spreadsheet schema: %w", err)
		}
		// Store schema name in config for later use
		conn.Config.SchemaName = schemaName
		log.Printf("DBManager -> Connect -> Created spreadsheet schema: %s", schemaName)
	}

	// Initialize subscribers map with existing subscribers
	conn.Subscribers = make(map[string]bool)

	for id := range existingSubscribers {
		conn.Subscribers[id] = true
	}
	// Add current streamID if not already present
	conn.Subscribers[streamID] = true

	log.Printf("DBManager -> Connect -> Initialized subscribers: %+v", conn.Subscribers)

	// Store connection
	m.connections[chatID] = conn
	log.Printf("DBManager -> Connect -> Stored connection in manager")

	// Notify subscribers in a separate goroutine
	go func() {
		m.notifySubscribers(chatID, userID, StatusConnected, "")
		log.Printf("DBManager -> Connect -> Notified subscribers")
	}()

	// Start background tasks in a separate goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("DBManager -> Connect -> Background task panic recovered: %v", r)
			}
		}()

		// Cache connection state in Redis
		ctx := context.Background()
		connKey := fmt.Sprintf("conn:%s", chatID)
		pipe := m.redisRepo.StartPipeline(ctx)
		pipe.Set(ctx, connKey, "connected", idleTimeout)
		if err := pipe.Execute(ctx); err != nil {
			log.Printf("DBManager -> Connect -> Failed to cache connection state: %v", err)
		} else {
			log.Printf("DBManager -> Connect -> Connection state cached in Redis")
		}

		// Start schema tracking
		m.StartSchemaTracking(chatID)
	}()

	conn.OnSchemaChange = func(chatID string) {
		m.doSchemaCheck(chatID)
	}

	return nil
}

// createSpreadsheetSchema creates a schema for spreadsheet connections
func (m *Manager) createSpreadsheetSchema(conn *Connection, schemaName string) error {
	// Get SQL DB from GORM
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %v", err)
	}

	// Create schema if not exists
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)
	if _, err := sqlDB.Exec(query); err != nil {
		return fmt.Errorf("failed to create schema: %v", err)
	}

	log.Printf("DBManager -> createSpreadsheetSchema -> Created schema: %s", schemaName)
	return nil
}

// Disconnect closes a database connection
func (m *Manager) Disconnect(chatID, userID string, deleteSchema bool) error {
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection not found for chat %s", chatID)
	}

	log.Printf("DBManager -> Disconnect -> Starting disconnect for chatID: %s", chatID)

	// Get the config key for the shared pool
	configKey := conn.ConfigKey

	// Decrement reference count in the pool
	m.dbPoolsMu.Lock()
	pool, poolExists := m.dbPools[configKey]

	if poolExists {
		pool.Mutex.Lock()
		pool.RefCount--
		refCount := pool.RefCount
		pool.Mutex.Unlock()

		log.Printf("DBManager -> Disconnect -> Decremented pool refCount to %d", refCount)

		// If reference count is zero, close the actual connection
		if refCount <= 0 {
			// Get the driver for this database type
			driver := m.drivers[conn.Config.Type]
			if driver != nil {
				if err := driver.Disconnect(conn); err != nil {
					m.dbPoolsMu.Unlock()
					return fmt.Errorf("failed to disconnect: %v", err)
				}
			}

			// Remove from pool
			delete(m.dbPools, configKey)
			log.Printf("DBManager -> Disconnect -> Removed pool from dbPools map")
		}
	}
	m.dbPoolsMu.Unlock()

	// Remove from connections map
	m.mu.Lock()
	delete(m.connections, chatID)
	m.mu.Unlock()

	log.Printf("DBManager -> Disconnect -> Removed connection from connections map")

	// Delete schema if requested
	if deleteSchema && m.schemaManager != nil {
		m.schemaManager.ClearSchemaCache(chatID)
		log.Printf("DBManager -> Disconnect -> Cleared schema cache for chatID: %s", chatID)
	}

	// For spreadsheet connections, delete the schema and all data when requested
	if deleteSchema && conn.Config.Type == "spreadsheet" {
		// Create a new spreadsheet driver instance to delete the schema
		spreadsheetDriver := &SpreadsheetDriver{
			postgresDriver: NewPostgresDriver(),
		}
		if err := spreadsheetDriver.DeleteConnectionData(chatID); err != nil {
			log.Printf("DBManager -> Disconnect -> Failed to delete spreadsheet schema: %v", err)
		} else {
			log.Printf("DBManager -> Disconnect -> Successfully deleted spreadsheet schema for chatID: %s", chatID)
		}
	}

	// Notify subscribers
	m.notifySubscribers(chatID, userID, StatusDisconnected, "")
	log.Printf("DBManager -> Disconnect -> Notified subscribers")

	return nil
}

// GetConnection returns a database connection for a chat
func (m *Manager) GetConnection(chatID string) (DBExecutor, error) {
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("connection not found for chat %s", chatID)
	}

	// Update LastUsed in both connection and pool
	conn.LastUsed = time.Now()

	// Also update the pool's LastUsed
	if conn.ConfigKey != "" {
		m.dbPoolsMu.RLock()
		if pool, exists := m.dbPools[conn.ConfigKey]; exists {
			pool.Mutex.Lock()
			pool.LastUsed = time.Now()

			// Verify database consistency
			if pool.Config.Database != conn.Config.Database {
				log.Printf("DBManager -> GetConnection -> WARNING: Pool database (%s) doesn't match connection database (%s)",
					pool.Config.Database, conn.Config.Database)
			}

			pool.Mutex.Unlock()
		}
		m.dbPoolsMu.RUnlock()
	}

	// Log connection details for debugging
	log.Printf("DBManager -> GetConnection -> Returning connection for chatID: %s, database: %s",
		chatID, conn.Config.Database)

	// Create appropriate wrapper based on database type
	switch conn.Config.Type {
	case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB:
		return NewPostgresWrapper(conn.DB, m, chatID), nil
	case constants.DatabaseTypeMySQL:
		return NewMySQLWrapper(conn.DB, m, chatID), nil
	case constants.DatabaseTypeClickhouse:
		return NewClickHouseWrapper(conn.DB, m, chatID), nil
	case constants.DatabaseTypeMongoDB:
		// For MongoDB, we use the MongoDBObj field instead of DB
		_, ok := conn.MongoDBObj.(*MongoDBWrapper)
		if !ok {
			return nil, fmt.Errorf("invalid MongoDB connection")
		}
		executor, err := NewMongoDBExecutor(conn)
		if err != nil {
			return nil, fmt.Errorf("failed to create MongoDB executor: %v", err)
		}
		return executor, nil
	case "spreadsheet":
		// For Spreadsheet, we need to create a wrapper that includes the schema name
		wrapper := &spreadsheetSchemaWrapper{
			conn:       conn,
			schemaName: conn.Config.SchemaName,
			chatID:     chatID,
		}
		return wrapper, nil
	default:
		return nil, fmt.Errorf("unsupported data source type: %s", conn.Config.Type)
	}
}

// startCleanupRoutine periodically checks for and closes inactive connections
func (m *Manager) startCleanupRoutine() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	log.Printf("DBManager -> startCleanupRoutine -> Starting cleanup routine with interval: %v", cleanupInterval)

	for {
		select {
		case <-m.stopCleanup:
			log.Printf("DBManager -> startCleanupRoutine -> Cleanup routine stopped")
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup removes idle connections
func (m *Manager) cleanup() {
	now := time.Now()
	m.cleanupMetrics.lastRun = now

	// Cleanup connections
	m.mu.Lock()
	for chatID, conn := range m.connections {
		if time.Since(conn.LastUsed) > idleTimeout {
			log.Printf("DBManager -> cleanup -> Removing idle connection for chatID: %s (idle for %v)", chatID, time.Since(conn.LastUsed))

			// Don't actually disconnect here, just remove from the map
			delete(m.connections, chatID)
			m.cleanupMetrics.connectionsRemoved++
		}
	}
	m.mu.Unlock()

	// Cleanup database pools
	m.dbPoolsMu.Lock()
	for key, pool := range m.dbPools {
		pool.Mutex.Lock()
		if pool.RefCount <= 0 && time.Since(pool.LastUsed) > idleTimeout {
			log.Printf("DBManager -> cleanup -> Removing idle connection pool: %s (idle for %v)", key, time.Since(pool.LastUsed))

			// Close the connection
			if pool.GORMDB != nil {
				sqlDB, err := pool.GORMDB.DB()
				if err == nil && sqlDB != nil {
					sqlDB.Close()
				}
			}
			delete(m.dbPools, key)
		}
		pool.Mutex.Unlock()
	}
	m.dbPoolsMu.Unlock()

	// Cleanup active executions
	m.executionMu.Lock()
	for streamID, execution := range m.activeExecutions {
		if !execution.IsExecuting && time.Since(execution.StartTime) > idleTimeout {
			log.Printf("DBManager -> cleanup -> Removing idle execution for streamID: %s (idle for %v)", streamID, time.Since(execution.StartTime))
			delete(m.activeExecutions, streamID)
			m.cleanupMetrics.executionsRemoved++
		}
	}
	m.executionMu.Unlock()
}

// Stop closes all connections and stops the cleanup routine
func (m *Manager) Stop() error {
	log.Println("DBManager -> Stop -> Stopping manager")

	// Signal cleanup routine to stop
	close(m.stopCleanup)
	log.Println("DBManager -> Stop -> Signaled cleanup routine to stop")

	// Close all connections
	m.mu.Lock()
	for chatID, conn := range m.connections {
		if driver, exists := m.drivers[conn.Config.Type]; exists {
			if err := driver.Disconnect(conn); err != nil {
				log.Printf("DBManager -> Stop -> Error disconnecting chat %s: %v", chatID, err)
			} else {
				log.Printf("DBManager -> Stop -> Disconnected chat %s", chatID)
			}
		}
	}
	m.connections = make(map[string]*Connection)
	m.mu.Unlock()
	log.Println("DBManager -> Stop -> Closed all connections")

	// Close all database pools
	m.dbPoolsMu.Lock()
	for key, pool := range m.dbPools {
		if pool.GORMDB != nil {
			sqlDB, err := pool.GORMDB.DB()
			if err == nil && sqlDB != nil {
				sqlDB.Close()
				log.Printf("DBManager -> Stop -> Closed pool: %s", key)
			}
		}
		delete(m.dbPools, key)
	}
	m.dbPoolsMu.Unlock()
	log.Println("DBManager -> Stop -> Closed all connection pools")

	// Cancel any active executions
	m.executionMu.Lock()
	for streamID, execution := range m.activeExecutions {
		execution.CancelFunc()
		if execution.Tx != nil {
			if err := execution.Tx.Rollback(); err != nil {
				log.Printf("DBManager -> Stop -> Error rolling back transaction for stream %s: %v", streamID, err)
			}
		}
		log.Printf("DBManager -> Stop -> Cancelled execution for stream %s", streamID)
	}
	m.activeExecutions = make(map[string]*QueryExecution)
	m.executionMu.Unlock()
	log.Println("DBManager -> Stop -> Cancelled all active executions")

	log.Println("DBManager -> Stop -> Manager stopped successfully")
	return nil
}

// UpdateLastUsed updates the last used timestamp for a connection
func (m *Manager) UpdateLastUsed(chatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[chatID]
	if !exists {
		return fmt.Errorf("no connection found for chat ID: %s", chatID)
	}

	conn.LastUsed = time.Now()

	// Refresh Redis TTL
	ctx := context.Background()
	connKey := fmt.Sprintf("conn:%s", chatID)
	pipe := m.redisRepo.StartPipeline(ctx)
	pipe.Set(ctx, connKey, "connected", idleTimeout)
	if err := pipe.Execute(ctx); err != nil {
		log.Printf("Failed to refresh connection TTL: %v", err)
	}

	return nil
}

// Subscribe adds a subscriber for connection status updates
func (m *Manager) Subscribe(chatID, streamID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("DBManager -> Subscribe -> Adding subscriber %s for chatID: %s", streamID, chatID)

	conn, exists := m.connections[chatID]
	if !exists {
		// Create a placeholder connection for subscribers
		conn = &Connection{
			Subscribers: make(map[string]bool),
			Status:      StatusDisconnected,
			ChatID:      chatID,
			StreamID:    streamID,
			// UserID will be set when actual connection is established
		}
		m.connections[chatID] = conn
		log.Printf("DBManager -> Subscribe -> Created new connection entry for chatID: %s", chatID)
	} else {
		// Update StreamID if connection exists
		conn.StreamID = streamID
	}

	conn.SubLock.Lock()
	if conn.Subscribers == nil {
		conn.Subscribers = make(map[string]bool)
	}
	conn.Subscribers[streamID] = true
	conn.SubLock.Unlock()

	log.Printf("DBManager -> Subscribe -> Added subscriber %s for chatID: %s, total subscribers: %d",
		streamID, chatID, len(conn.Subscribers))
}

// Remove subscriber
func (m *Manager) Unsubscribe(chatID, deviceID string) {
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	conn.SubLock.Lock()
	delete(conn.Subscribers, deviceID)
	conn.SubLock.Unlock()
}

// Get event channel for SSE
func (m *Manager) GetEventChannel() <-chan SSEEvent {
	return m.eventChan
}

// Notify subscribers of connection status change
func (m *Manager) notifySubscribers(chatID, userID string, status ConnectionStatus, err string) {
	log.Printf("DBManager -> notifySubscribers -> Notifying subscribers for chatID: %s", chatID)

	// Get connection and subscribers under read lock
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("DBManager -> notifySubscribers -> No connection found for chatID: %s", chatID)
		return
	}

	// Get a snapshot of subscribers under read lock
	conn.SubLock.RLock()
	subscribers := make(map[string]bool, len(conn.Subscribers))
	for id := range conn.Subscribers {
		subscribers[id] = true
	}
	conn.SubLock.RUnlock()

	log.Printf("DBManager -> notifySubscribers -> Notifying %d subscribers for chatID: %s", len(subscribers), chatID)

	// Notify subscribers without holding any locks
	for streamID := range subscribers {
		response := dtos.StreamResponse{
			Event: string(status),
			Data:  err,
		}

		if m.streamHandler != nil {
			m.streamHandler.HandleDBEvent(userID, chatID, streamID, response)
			log.Printf("DBManager -> notifySubscribers -> Sent event to stream handler: %+v", response)
		}
	}
}

func (m *Manager) StartSchemaTracking(chatID string) {
	log.Printf("DBManager -> StartSchemaTracking -> Starting for chatID: %s", chatID)

	go func() {
		// Initial delay to let connection stabilize
		time.Sleep(2 * time.Second)

		ticker := time.NewTicker(schemaCheckInterval)

		defer ticker.Stop()

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		// Call Schema change only when schema is empty
		if _, err := m.schemaManager.getStoredSchema(ctx, chatID); err != nil {
			log.Printf("DBManager -> StartSchemaTracking -> err: %v", err)
			// Do initial schema check
			if err := m.doSchemaCheck(chatID); err != nil {
				log.Printf("DBManager -> StartSchemaTracking -> Initial schema check failed: %v", err)
			}
		}

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
				defer cancel()

				// Call Schema change only when schema is empty
				if _, err := m.schemaManager.getStoredSchema(ctx, chatID); err != nil {
					log.Printf("DBManager -> StartSchemaTracking -> err: %v", err)
					// Do initial schema check
					if err := m.doSchemaCheck(chatID); err != nil {
						log.Printf("DBManager -> StartSchemaTracking -> Initial schema check failed: %v", err)
					}
				}
			case <-m.stopCleanup:
				log.Printf("DBManager -> StartSchemaTracking -> Stopping for chatID: %s", chatID)
				return
			}
		}
	}()
}

func (m *Manager) doSchemaCheck(chatID string) error {
	conn, err := m.GetConnection(chatID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %v", err)
	}

	m.mu.RLock()
	dbConn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection not found")
	}

	// Get selected collections from the chat service if available
	var selectedTables []string
	if m.streamHandler != nil {
		// Try to get selected collections from the chat service
		selectedCollections, err := m.streamHandler.GetSelectedCollections(chatID)
		if err == nil && selectedCollections != "ALL" && selectedCollections != "" {
			selectedTables = strings.Split(selectedCollections, ",")
			log.Printf("DBManager -> doSchemaCheck -> Using selected collections for chat %s: %v", chatID, selectedTables)
		} else {
			// Default to ALL if there's an error or no specific collections
			selectedTables = []string{"ALL"}
			log.Printf("DBManager -> doSchemaCheck -> Using ALL tables for chat %s", chatID)
		}
	} else {
		// Default to ALL if stream handler is not available
		selectedTables = []string{"ALL"}
	}

	// Force clear any cached schema to ensure we get fresh data
	m.schemaManager.ClearSchemaCache(chatID)

	// Get fresh schema from database
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	// Pass selectedTables instead of hardcoded "ALL"
	diff, hasChanged, err := m.schemaManager.CheckSchemaChanges(ctx, chatID, conn, dbConn.Config.Type, selectedTables)
	if err != nil {
		// Check if this is a first-time schema storage error, which we can ignore
		if strings.Contains(err.Error(), "first-time schema storage") || strings.Contains(err.Error(), "key does not exist") {
			log.Printf("DBManager -> doSchemaCheck -> First-time schema storage for chat %s (expected behavior)", chatID)
			return nil
		}

		// Check if this is a schema fetcher not found error, which means we need to register the fetcher
		if strings.Contains(err.Error(), "schema fetcher not found") || strings.Contains(err.Error(), "no schema fetcher registered") {
			log.Printf("DBManager -> doSchemaCheck -> Schema fetcher not found for type %s. This is likely a configuration issue.", dbConn.Config.Type)
			return nil
		}

		return fmt.Errorf("schema check failed: %v", err)
	}

	if diff != nil {
		log.Printf("DBManager -> doSchemaCheck -> Schema diff for chat %s: %+v", chatID, diff)
	}

	if hasChanged {
		log.Printf("DBManager -> doSchemaCheck -> Schema has changed for chat %s: %t", chatID, hasChanged)
		if m.streamHandler != nil {
			m.streamHandler.HandleSchemaChange(dbConn.UserID, chatID, dbConn.StreamID, diff)
		}
	}

	return nil
}

// Add exported methods to access internal fields
func (m *Manager) GetConnections() map[string]*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections
}

func (m *Manager) GetSchemaManager() *SchemaManager {
	return m.schemaManager
}

func (m *Manager) GetConnectionInfo(chatID string) (*ConnectionInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[chatID]
	if !exists {
		return nil, false
	}

	// Convert Connection to ConnectionInfo
	connInfo := &ConnectionInfo{
		Config: conn.Config,
	}

	// Get the underlying *sql.DB from gorm.DB
	if conn.DB != nil {
		sqlDB, err := conn.DB.DB()
		if err == nil {
			connInfo.DB = sqlDB
		}
	}

	return connInfo, true
}

// IsConnected checks if there is an active connection for the given chat
func (m *Manager) IsConnected(chatID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[chatID]
	if !exists {
		return false
	}

	// For MongoDB connections
	if conn.Config.Type == "mongodb" {
		if wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper); ok && wrapper != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := wrapper.Client.Ping(ctx, nil)
			return err == nil
		}
		return false
	}

	// For SQL connections
	if conn.DB != nil {
		sqlDB, err := conn.DB.DB()
		if err != nil {
			return false
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = sqlDB.PingContext(ctx)
		return err == nil
	}

	return false
}

type ConnectionInfo struct {
	DB     *sql.DB
	Config ConnectionConfig
}

// SetStreamHandler sets the stream handler for database events
func (m *Manager) SetStreamHandler(handler StreamHandler) {
	m.streamHandler = handler
}
