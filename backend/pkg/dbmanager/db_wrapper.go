package dbmanager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"
)

// DBExecutor interface defines common database operations
type DBExecutor interface {
	Raw(sql string, values ...interface{}) error
	Exec(sql string, values ...interface{}) error
	Query(sql string, dest interface{}, values ...interface{}) error
	QueryRows(sql string, dest *[]map[string]interface{}, values ...interface{}) error
	Close() error
	GetDB() *sql.DB
	GetSchema(ctx context.Context) (*SchemaInfo, error)
	GetTableChecksum(ctx context.Context, table string) (string, error)
}

// BaseWrapper provides common functionality for all DB wrappers
type BaseWrapper struct {
	db      *gorm.DB
	manager *Manager
	chatID  string
}

func (w *BaseWrapper) updateUsage() error {
	if err := w.manager.UpdateLastUsed(w.chatID); err != nil {
		log.Printf("Failed to update last used time: %v", err)
		return err
	}
	return nil
}

// PostgresWrapper implements DBExecutor for PostgreSQL
type PostgresWrapper struct {
	BaseWrapper
}

func NewPostgresWrapper(db *gorm.DB, manager *Manager, chatID string) *PostgresWrapper {
	return &PostgresWrapper{
		BaseWrapper: BaseWrapper{
			db:      db,
			manager: manager,
			chatID:  chatID,
		},
	}
}

// GetDB returns the underlying *sql.DB
func (w *PostgresWrapper) GetDB() *sql.DB {
	sqlDB, err := w.db.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
		return nil
	}
	return sqlDB
}

// GetSchema fetches the current database schema
func (w *PostgresWrapper) GetSchema(ctx context.Context) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresWrapper -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	driver, exists := w.manager.drivers["postgresql"]
	if !exists {
		// Check if yugabytedb driver exists
		driver, exists = w.manager.drivers["yugabytedb"]
		if !exists {
			// Check if timescaledb driver exists
			driver, exists = w.manager.drivers["timescaledb"]
			if !exists {
				return nil, fmt.Errorf("driver not found")
			}
		}
	}

	if fetcher, ok := driver.(SchemaFetcher); ok {
		// Get selected collections from the chat service if available
		var selectedTables []string
		if w.manager.streamHandler != nil {
			// Try to get selected collections from the chat service
			selectedCollections, err := w.manager.streamHandler.GetSelectedCollections(w.chatID)
			if err == nil && selectedCollections != "ALL" && selectedCollections != "" {
				selectedTables = strings.Split(selectedCollections, ",")
				log.Printf("PostgresWrapper -> GetSchema -> Using selected collections for chat %s: %v", w.chatID, selectedTables)
			} else {
				// Default to ALL if there's an error or no specific collections
				selectedTables = []string{"ALL"}
				log.Printf("PostgresWrapper -> GetSchema -> Using ALL tables for chat %s", w.chatID)
			}
		} else {
			// Default to ALL if stream handler is not available
			selectedTables = []string{"ALL"}
		}

		// Pass the selected tables to get the schema
		schema, err := fetcher.GetSchema(ctx, w, selectedTables)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("Schema fetch cancelled by context")
				return nil, err
			}
			return nil, err
		}
		return schema, nil
	}
	return nil, fmt.Errorf("driver does not support schema fetching")
}

// GetTableChecksum calculates checksum for a single table
func (w *PostgresWrapper) GetTableChecksum(ctx context.Context, table string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresWrapper -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	if err := w.updateUsage(); err != nil {
		return "", fmt.Errorf("failed to update usage: %v", err)
	}

	driver, exists := w.manager.drivers["postgresql"]
	if !exists {
		// Check if yugabytedb driver exists
		driver, exists = w.manager.drivers["yugabytedb"]
		if !exists {
			// Check if timescaledb driver exists
			driver, exists = w.manager.drivers["timescaledb"]
			if !exists {
				return "", fmt.Errorf("driver not found")
			}
		}
	}

	if fetcher, ok := driver.(SchemaFetcher); ok {
		return fetcher.GetTableChecksum(ctx, w, table)
	}
	return "", fmt.Errorf("driver does not support checksum calculation")
}

// Raw executes a raw SQL query
func (w *PostgresWrapper) Raw(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Error
}

// Exec executes a SQL statement
func (w *PostgresWrapper) Exec(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Exec(sql, values...).Error
}

// Query executes a SQL query and scans the result into dest
func (w *PostgresWrapper) Query(sql string, dest interface{}, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Scan(dest).Error
}

// QueryRows executes a SQL query and scans the result into dest
func (w *PostgresWrapper) QueryRows(sql string, dest *[]map[string]interface{}, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Scan(dest).Error
}

// Close closes the database connection
func (w *PostgresWrapper) Close() error {
	sqlDB, err := w.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// MySQLWrapper implements DBExecutor for MySQL
type MySQLWrapper struct {
	BaseWrapper
}

func NewMySQLWrapper(db *gorm.DB, manager *Manager, chatID string) *MySQLWrapper {
	return &MySQLWrapper{
		BaseWrapper: BaseWrapper{
			db:      db,
			manager: manager,
			chatID:  chatID,
		},
	}
}

// GetDB returns the underlying *sql.DB
func (w *MySQLWrapper) GetDB() *sql.DB {
	sqlDB, err := w.db.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
		return nil
	}
	return sqlDB
}

// GetSchema fetches the current database schema
func (w *MySQLWrapper) GetSchema(ctx context.Context) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MySQLWrapper -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Check if MySQL driver exists
	_, exists := w.manager.drivers["mysql"]
	if !exists {
		return nil, fmt.Errorf("MySQL driver not found")
	}

	// Get the schema fetcher factory for MySQL
	fetcherFactory, exists := w.manager.fetchers["mysql"]
	if !exists {
		return nil, fmt.Errorf("MySQL schema fetcher not found")
	}

	// Create a schema fetcher for this connection
	fetcher := fetcherFactory(w)

	// Get selected collections from the chat service if available
	var selectedTables []string
	if w.manager.streamHandler != nil {
		// Try to get selected collections from the chat service
		selectedCollections, err := w.manager.streamHandler.GetSelectedCollections(w.chatID)
		if err == nil && selectedCollections != "ALL" && selectedCollections != "" {
			selectedTables = strings.Split(selectedCollections, ",")
			log.Printf("MySQLWrapper -> GetSchema -> Using selected collections for chat %s: %v", w.chatID, selectedTables)
		} else {
			// Default to ALL if there's an error or no specific collections
			selectedTables = []string{"ALL"}
			log.Printf("MySQLWrapper -> GetSchema -> Using ALL tables for chat %s", w.chatID)
		}
	} else {
		// Default to ALL if stream handler is not available
		selectedTables = []string{"ALL"}
	}

	// Pass the selected tables to get the schema
	schema, err := fetcher.GetSchema(ctx, w, selectedTables)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Printf("Schema fetch cancelled by context")
			return nil, err
		}
		return nil, err
	}
	return schema, nil
}

// GetTableChecksum calculates checksum for a single table
func (w *MySQLWrapper) GetTableChecksum(ctx context.Context, table string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MySQLWrapper -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	if err := w.updateUsage(); err != nil {
		return "", fmt.Errorf("failed to update usage: %v", err)
	}

	// Get the schema fetcher factory for MySQL
	fetcherFactory, exists := w.manager.fetchers["mysql"]
	if !exists {
		return "", fmt.Errorf("MySQL schema fetcher not found")
	}

	// Create a schema fetcher for this connection
	fetcher := fetcherFactory(w)

	return fetcher.GetTableChecksum(ctx, w, table)
}

// Raw executes a raw SQL query
func (w *MySQLWrapper) Raw(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Error
}

// Exec executes a SQL statement
func (w *MySQLWrapper) Exec(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Exec(sql, values...).Error
}

// Query executes a SQL query and scans the result into dest
func (w *MySQLWrapper) Query(sql string, dest interface{}, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	log.Printf("MySQLWrapper -> Query -> Executing: %s with values: %v", sql, values)
	result := w.db.Raw(sql, values...).Scan(dest)
	if result.Error != nil {
		log.Printf("MySQLWrapper -> Query -> Error: %v", result.Error)
	} else {
		log.Printf("MySQLWrapper -> Query -> Success: %d rows affected", result.RowsAffected)
	}
	return result.Error
}

// QueryRows executes a SQL query and scans the result into dest
func (w *MySQLWrapper) QueryRows(sql string, dest *[]map[string]interface{}, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Scan(dest).Error
}

// Close closes the database connection
func (w *MySQLWrapper) Close() error {
	sqlDB, err := w.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// ClickHouseWrapper implements DBExecutor for ClickHouse
type ClickHouseWrapper struct {
	BaseWrapper
}

func NewClickHouseWrapper(db *gorm.DB, manager *Manager, chatID string) *ClickHouseWrapper {
	return &ClickHouseWrapper{
		BaseWrapper: BaseWrapper{
			db:      db,
			manager: manager,
			chatID:  chatID,
		},
	}
}

// GetDB returns the underlying *sql.DB
func (w *ClickHouseWrapper) GetDB() *sql.DB {
	sqlDB, err := w.db.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
		return nil
	}
	return sqlDB
}

// GetSchema fetches the current database schema
func (w *ClickHouseWrapper) GetSchema(ctx context.Context) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseWrapper -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Check if ClickHouse driver exists
	_, exists := w.manager.drivers["clickhouse"]
	if !exists {
		return nil, fmt.Errorf("ClickHouse driver not found")
	}

	// Get the schema fetcher factory for ClickHouse
	fetcherFactory, exists := w.manager.fetchers["clickhouse"]
	if !exists {
		return nil, fmt.Errorf("ClickHouse schema fetcher not found")
	}

	// Create a schema fetcher for this connection
	fetcher := fetcherFactory(w)

	// Get selected collections from the chat service if available
	var selectedTables []string
	if w.manager.streamHandler != nil {
		// Try to get selected collections from the chat service
		selectedCollections, err := w.manager.streamHandler.GetSelectedCollections(w.chatID)
		if err == nil && selectedCollections != "ALL" && selectedCollections != "" {
			selectedTables = strings.Split(selectedCollections, ",")
			log.Printf("ClickHouseWrapper -> GetSchema -> Using selected collections for chat %s: %v", w.chatID, selectedTables)
		} else {
			// Default to ALL if there's an error or no specific collections
			selectedTables = []string{"ALL"}
			log.Printf("ClickHouseWrapper -> GetSchema -> Using ALL tables for chat %s", w.chatID)
		}
	} else {
		// Default to ALL if stream handler is not available
		selectedTables = []string{"ALL"}
	}

	// Pass the selected tables to get the schema
	schema, err := fetcher.GetSchema(ctx, w, selectedTables)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Printf("Schema fetch cancelled by context")
			return nil, err
		}
		return nil, err
	}
	return schema, nil
}

// GetTableChecksum calculates checksum for a single table
func (w *ClickHouseWrapper) GetTableChecksum(ctx context.Context, table string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseWrapper -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	if err := w.updateUsage(); err != nil {
		return "", fmt.Errorf("failed to update usage: %v", err)
	}

	// Get the schema fetcher factory for ClickHouse
	fetcherFactory, exists := w.manager.fetchers["clickhouse"]
	if !exists {
		return "", fmt.Errorf("ClickHouse schema fetcher not found")
	}

	// Create a schema fetcher for this connection
	fetcher := fetcherFactory(w)

	return fetcher.GetTableChecksum(ctx, w, table)
}

// Raw executes a raw SQL query
func (w *ClickHouseWrapper) Raw(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Error
}

// Exec executes a SQL statement
func (w *ClickHouseWrapper) Exec(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Exec(sql, values...).Error
}

// Query executes a SQL query and scans the result into dest
func (w *ClickHouseWrapper) Query(sql string, dest interface{}, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Scan(dest).Error
}

// QueryRows executes a SQL query and scans the result into dest
func (w *ClickHouseWrapper) QueryRows(sql string, dest *[]map[string]interface{}, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Scan(dest).Error
}

// Close closes the database connection
func (w *ClickHouseWrapper) Close() error {
	sqlDB, err := w.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
