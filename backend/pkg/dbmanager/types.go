package dbmanager

import (
	"context"
	"neobase-ai/internal/apis/dtos"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"gorm.io/gorm"
)

// ConnectionStatus represents the current state of a database connection
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "db-connected"
	StatusDisconnected ConnectionStatus = "db-disconnected"
	StatusError        ConnectionStatus = "db-error"
)

// Connection represents an active database connection
type Connection struct {
	DB             *gorm.DB
	MongoDBObj     interface{} // MongoDB client object
	LastUsed       time.Time
	Status         ConnectionStatus
	Error          string
	Config         ConnectionConfig
	UserID         string
	ChatID         string
	StreamID       string
	Subscribers    map[string]bool     // Map of subscriber IDs (e.g., streamIDs) that need notifications
	SubLock        sync.RWMutex        // Lock for thread-safe subscriber operations
	OnSchemaChange func(chatID string) // Callback for schema changes
	ConfigKey      string              // Reference to the shared connection pool
	TempFiles      []string            // Temporary certificate files to clean up on disconnect
}

// ConnectionConfig holds the configuration for a database connection
type ConnectionConfig struct {
	Type     string  `json:"type"`
	Host     string  `json:"host"`
	Port     *string `json:"port"`
	Username *string `json:"username"`
	Password *string `json:"password"`
	Database string  `json:"database"`
	AuthDatabase *string `json:"auth_database"` // Database to authenticate against (for MongoDB)

	// SSL/TLS Configuration
	UseSSL         bool    `json:"use_ssl"`
	SSLMode        *string `json:"ssl_mode,omitempty"`          // type: disable, require, verify-ca, verify-full
	SSLCertURL     *string `json:"ssl_cert_url,omitempty"`      // URL to client certificate
	SSLKeyURL      *string `json:"ssl_key_url,omitempty"`       // URL to client key
	SSLRootCertURL *string `json:"ssl_root_cert_url,omitempty"` // URL to CA certificate
}

// SSEEvent represents an event to be sent via SSE
type SSEEvent struct {
	UserID    string           `json:"user_id"`
	ChatID    string           `json:"chat_id"`
	StreamID  string           `json:"stream_id"`
	Status    ConnectionStatus `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Error     string           `json:"error,omitempty"`
}

// StreamHandler interface for handling database events
type StreamHandler interface {
	HandleDBEvent(userID, chatID, streamID string, response dtos.StreamResponse)
	HandleSchemaChange(userID, chatID, streamID string, diff *SchemaDiff)
	GetSelectedCollections(chatID string) (string, error)
}

// QueryExecutionResult represents the result of a query execution
type QueryExecutionResult struct {
	Result        map[string]interface{} `json:"result"`
	ResultJSON    string                 `json:"result_json"`
	ExecutionTime int                    `json:"execution_time"`
	Error         *dtos.QueryError       `json:"error,omitempty"`

	// Additional fields for testing and query parsing
	Database   string    `json:"-"` // Database name
	Collection string    `json:"-"` // Collection name
	Query      string    `json:"-"` // Original query string
	Timestamp  time.Time `json:"-"` // Time the query was executed
	Pipeline   []bson.M  `json:"-"` // Aggregation pipeline
	Filter     bson.M    `json:"-"` // Query filter
	Projection bson.M    `json:"-"` // Query projection
	Update     bson.M    `json:"-"` // Update document
	Documents  []bson.M  `json:"-"` // Documents for insert operations
}

type DatabaseDriver interface {
	Connect(config ConnectionConfig) (*Connection, error)
	Disconnect(conn *Connection) error
	Ping(conn *Connection) error
	IsAlive(conn *Connection) bool
	ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult
	BeginTx(ctx context.Context, conn *Connection) Transaction
}

// Add new Transaction interface
type Transaction interface {
	ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult
	Commit() error
	Rollback() error
}

// FetcherFactory is a function that creates a SchemaFetcher for a database
type FetcherFactory func(db DBExecutor) SchemaFetcher
