package dbmanager

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"
	"neobase-ai/internal/apis/dtos"
)

// ConnectionStatus represents the status of a database connection
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusError        ConnectionStatus = "error"
)

// ConnectionConfig represents database connection configuration
type ConnectionConfig struct {
	Type             string  `json:"type"`
	Host             string  `json:"host"`
	Port             *string `json:"port"`
	Username         *string `json:"username"`
	Password         *string `json:"password"`
	Database         string  `json:"database"`
	AuthDatabase     *string `json:"auth_database,omitempty"`
	UseSSL           bool    `json:"use_ssl"`
	SSLMode          *string `json:"ssl_mode,omitempty"`
	SSLCertURL       *string `json:"ssl_cert_url,omitempty"`
	SSLKeyURL        *string `json:"ssl_key_url,omitempty"`
	SSLRootCertURL   *string `json:"ssl_root_cert_url,omitempty"`
	SSHEnabled       bool    `json:"ssh_enabled"`
	SSHHost          *string `json:"ssh_host,omitempty"`
	SSHPort          *string `json:"ssh_port,omitempty"`
	SSHUsername      *string `json:"ssh_username,omitempty"`
	SSHPrivateKey    *string `json:"ssh_private_key,omitempty"`
	SSHPassphrase    *string `json:"ssh_passphrase,omitempty"`
	MongoDBURI       *string `json:"mongodb_uri,omitempty"`
	SchemaName       string  `json:"schema_name,omitempty"` // For spreadsheet connections
}

// Connection represents an active database connection
type Connection struct {
	DB              *gorm.DB
	LastUsed        time.Time
	Status          ConnectionStatus
	Config          ConnectionConfig
	UserID          string
	ChatID          string
	StreamID        string
	Subscribers     map[string]bool
	SubLock         sync.RWMutex
	TempFiles       []string
	OnSchemaChange  func(chatID string)
	MongoDBObj      interface{} // For MongoDB connections
	ConfigKey       string      // Key for connection pooling
}

// DatabaseDriver interface defines methods that all database drivers must implement
type DatabaseDriver interface {
	Connect(config ConnectionConfig) (*Connection, error)
	Disconnect(conn *Connection) error
	Ping(conn *Connection) error
	IsAlive(conn *Connection) bool
	ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult
	BeginTx(ctx context.Context, conn *Connection) Transaction
	GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error)
	GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error)
	FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error)
}

// Transaction interface represents a database transaction
type Transaction interface {
	Commit() error
	Rollback() error
	ExecuteQuery(ctx context.Context, query string) (*QueryExecutionResult, error)
}

// QueryExecutionResult represents the result of a query execution
type QueryExecutionResult struct {
	Result        interface{}        `json:"result,omitempty"`
	Error         *dtos.QueryError   `json:"error,omitempty"`
	ExecutionTime int                `json:"execution_time"`
	RowsAffected  int64              `json:"rows_affected,omitempty"`
	StreamData    []byte             `json:"stream_data,omitempty"`
}

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	UserID   string      `json:"user_id"`
	ChatID   string      `json:"chat_id"`
	StreamID string      `json:"stream_id"`
	Event    string      `json:"event"`
	Data     interface{} `json:"data"`
}

// StreamHandler interface for handling stream events
type StreamHandler interface {
	HandleDBEvent(userID, chatID, streamID string, response dtos.StreamResponse)
	HandleSchemaChange(userID, chatID, streamID string, diff interface{})
	GetSelectedCollections(chatID string) (string, error)
}

// FetcherFactory is a function that creates a SchemaFetcher for a given database executor
type FetcherFactory func(db DBExecutor) SchemaFetcher