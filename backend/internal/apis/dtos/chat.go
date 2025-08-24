package dtos

type CreateChatSettings struct {
	AutoExecuteQuery *bool `json:"auto_execute_query"`
	ShareDataWithAI  *bool `json:"share_data_with_ai"`
	NonTechMode      *bool `json:"non_tech_mode"`
}

type ChatSettingsResponse struct {
	AutoExecuteQuery bool `json:"auto_execute_query"`
	ShareDataWithAI  bool `json:"share_data_with_ai"`
	NonTechMode      bool `json:"non_tech_mode"`
}
type CreateConnectionRequest struct {
	Type         string  `json:"type" binding:"required,oneof=postgresql yugabytedb mysql clickhouse mongodb redis neo4j cassandra spreadsheet google_sheets"`
	Host         string  `json:"host"`
	Port         *string `json:"port"`
	Username     string  `json:"username"`
	Password     *string `json:"password"`
	Database     string  `json:"database"`
	AuthDatabase *string `json:"auth_database,omitempty"` // Database to authenticate against (for MongoDB)

	// SSL/TLS Configuration
	UseSSL         bool    `json:"use_ssl"`
	SSLMode        *string `json:"ssl_mode,omitempty"` // type: disable, require, verify-ca, verify-full
	SSLCertURL     *string `json:"ssl_cert_url,omitempty"`
	SSLKeyURL      *string `json:"ssl_key_url,omitempty"`
	SSLRootCertURL *string `json:"ssl_root_cert_url,omitempty"`

	// Google Sheets specific fields
	GoogleSheetID      *string `json:"google_sheet_id,omitempty"`
	GoogleSheetURL     *string `json:"google_sheet_url,omitempty"`
	GoogleAuthToken    *string `json:"google_auth_token,omitempty"`
	GoogleRefreshToken *string `json:"google_refresh_token,omitempty"`
}

type ConnectionResponse struct {
	ID          string  `json:"id" binding:"required"`
	Type        string  `json:"type" binding:"required"`
	Host        string  `json:"host" binding:"required"`
	Port        *string `json:"port"`
	Username    string  `json:"username" binding:"required"`
	Database    string  `json:"database" binding:"required"`
	IsExampleDB bool    `json:"is_example_db"`
	// Password not exposed in response

	// SSL/TLS Configuration
	UseSSL         bool    `json:"use_ssl"`
	SSLMode        *string `json:"ssl_mode,omitempty"` // type: disable, require, verify-ca, verify-full
	SSLCertURL     *string `json:"ssl_cert_url,omitempty"`
	SSLKeyURL      *string `json:"ssl_key_url,omitempty"`
	SSLRootCertURL *string `json:"ssl_root_cert_url,omitempty"`

	// Google Sheets specific fields (no tokens exposed in response)
	GoogleSheetID  *string `json:"google_sheet_id,omitempty"`
	GoogleSheetURL *string `json:"google_sheet_url,omitempty"`
}

type CreateChatRequest struct {
	Connection CreateConnectionRequest `json:"connection" binding:"required"`
	Settings   CreateChatSettings      `json:"settings,omitempty"`
}

type UpdateChatRequest struct {
	Connection          *CreateConnectionRequest `json:"connection"`
	SelectedCollections *string                  `json:"selected_collections"` // "ALL" or comma-separated table names
	Settings            *CreateChatSettings      `json:"settings"`
}

type ChatResponse struct {
	ID                  string               `json:"id"`
	UserID              string               `json:"user_id"`
	Connection          ConnectionResponse   `json:"connection"`
	SelectedCollections string               `json:"selected_collections"`
	CreatedAt           string               `json:"created_at"`
	UpdatedAt           string               `json:"updated_at"`
	Settings            ChatSettingsResponse `json:"settings"`
}

type ChatListResponse struct {
	Chats []ChatResponse `json:"chats"`
	Total int64          `json:"total"`
}

// TableInfo represents a table with its columns
type TableInfo struct {
	Name       string       `json:"name"`
	Columns    []ColumnInfo `json:"columns"`
	IsSelected bool         `json:"is_selected"`
	RowCount   int64        `json:"row_count"`
	SizeBytes  int64        `json:"size_bytes"`
}

// ColumnInfo represents a column in a table
type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsNullable bool   `json:"is_nullable"`
}

// TablesResponse represents the response for the get tables API
type TablesResponse struct {
	Tables []TableInfo `json:"tables"`
}

// Query Recommendations DTOs
type QueryRecommendation struct {
	Text string `json:"text"`
}

type QueryRecommendationsResponse struct {
	Recommendations []QueryRecommendation `json:"recommendations"`
}
