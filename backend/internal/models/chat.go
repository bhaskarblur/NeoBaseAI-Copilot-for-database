package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatSettings struct {
	AutoExecuteQuery          bool   `bson:"auto_execute_query" json:"auto_execute_query,omitempty"`                   // default is true, Execute query automatically when LLM response is received
	ShareDataWithAI           bool   `bson:"share_data_with_ai" json:"share_data_with_ai,omitempty"`                   // default is false, Don't share data with AI
	NonTechMode               bool   `bson:"non_tech_mode" json:"non_tech_mode,omitempty"`                             // default is false, Enable non-technical mode for simplified responses
	SelectedLLMModel          string `bson:"selected_llm_model" json:"selected_llm_model,omitempty"`                   // LLM model selected for this chat (e.g., "gpt-4o", "gemini-2.0-flash")
	AutoGenerateVisualization bool   `bson:"auto_generate_visualization" json:"auto_generate_visualization,omitempty"` // default is false, Auto-generate chart visualizations for compatible queries
}

type Connection struct {
	Type         string  `bson:"type" json:"type"`
	Host         string  `bson:"host" json:"host"`
	Port         *string `bson:"port" json:"port"`
	Username     *string `bson:"username" json:"username"`
	Password     *string `bson:"password" json:"-"` // Hide in JSON
	Database     string  `bson:"database" json:"database"`
	AuthDatabase *string `bson:"auth_database" json:"auth_database"` // Database to authenticate against
	IsExampleDB  bool    `bson:"is_example_db" json:"is_example_db"` // default is false, if true, then the database is an example database configs setup from environment variables

	// SSL/TLS Configuration
	UseSSL         bool    `bson:"use_ssl" json:"use_ssl"`
	SSLMode        *string `bson:"ssl_mode,omitempty" json:"ssl_mode,omitempty"` // type: disable, require, verify-ca, verify-full
	SSLCertURL     *string `bson:"ssl_cert_url,omitempty" json:"ssl_cert_url,omitempty"`
	SSLKeyURL      *string `bson:"ssl_key_url,omitempty" json:"ssl_key_url,omitempty"`
	SSLRootCertURL *string `bson:"ssl_root_cert_url,omitempty" json:"ssl_root_cert_url,omitempty"`

	// SSH Tunnel Configuration
	SSHEnabled       bool    `bson:"ssh_enabled,omitempty" json:"ssh_enabled,omitempty"`
	SSHHost          *string `bson:"ssh_host,omitempty" json:"ssh_host,omitempty"`
	SSHPort          *string `bson:"ssh_port,omitempty" json:"ssh_port,omitempty"`
	SSHUsername      *string `bson:"ssh_username,omitempty" json:"ssh_username,omitempty"`
	SSHAuthMethod    *string `bson:"ssh_auth_method,omitempty" json:"ssh_auth_method,omitempty"`         // "publickey" or "password"
	SSHPrivateKey    *string `bson:"ssh_private_key,omitempty" json:"-"`                                 // Hide in JSON
	SSHPrivateKeyURL *string `bson:"ssh_private_key_url,omitempty" json:"ssh_private_key_url,omitempty"` // URL to fetch from
	SSHPassphrase    *string `bson:"ssh_passphrase,omitempty" json:"-"`                                  // Hide in JSON
	SSHPassword      *string `bson:"ssh_password,omitempty" json:"-"`                                    // Hide in JSON

	// Google Sheets specific fields
	GoogleSheetID      *string `bson:"google_sheet_id,omitempty" json:"google_sheet_id,omitempty"`
	GoogleSheetURL     *string `bson:"google_sheet_url,omitempty" json:"google_sheet_url,omitempty"` // Encrypted, show in JSON for user reference
	GoogleAuthToken    *string `bson:"google_auth_token,omitempty" json:"-"`                         // Hide in JSON
	GoogleRefreshToken *string `bson:"google_refresh_token,omitempty" json:"-"`                      // Hide in JSON

	Base `bson:",inline"`
}

type Chat struct {
	UserID              primitive.ObjectID `bson:"user_id" json:"user_id"`
	Connection          Connection         `bson:"connection" json:"connection"`
	SelectedCollections string             `bson:"selected_collections" json:"selected_collections"` // "ALL" or comma-separated table names
	Settings            ChatSettings       `bson:"settings" json:"settings"`
	PreferredLLMModel   *string            `bson:"preferred_llm_model" json:"preferred_llm_model"` // User's preferred LLM model for this chat
	Base                `bson:",inline"`
}

func NewChat(userID primitive.ObjectID, connection Connection, settings ChatSettings) *Chat {
	return &Chat{
		UserID:              userID,
		Connection:          connection,
		Settings:            settings,
		SelectedCollections: "ALL", // Default to ALL tables
		Base:                NewBase(),
	}
}

func DefaultChatSettings() ChatSettings {
	return ChatSettings{
		AutoExecuteQuery:          true,  // default is true, Execute query automatically when LLM response is received
		ShareDataWithAI:           false, // default is false, Don't share data with AI
		NonTechMode:               false, // default is false, Technical mode enabled by default
		AutoGenerateVisualization: false, // default is false, Don't auto-generate visualizations
	}
}
