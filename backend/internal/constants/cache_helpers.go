package constants

import "time"

// Cache TTL constants for all repositories
const (
	// User cache TTL
	UserCacheTTL = 1 * time.Hour // 1 hour cache for user data

	// Chat cache TTLs
	ChatCacheTTL     = 30 * time.Minute // 30 minutes cache for individual chats
	ChatListCacheTTL = 15 * time.Minute // 10 minutes cache for chat lists

	MaxCachedMessages = 50 // Maximum number of messages to cache per chat
	// Message cache TTLs
	MessageCacheTTL  = 15 * time.Minute // 15 minutes cache for individual messages
	MessageListTTL   = 10 * time.Minute // 10 minutes cache for message lists
	PinnedMessageTTL = 30 * time.Minute // 30 minutes cache for pinned messages

	// LLM message cache TTL
	LLMMessageCacheTTL = 15 * time.Minute // 15 minutes cache for LLM message context

	// Knowledge Base cache TTLs
	KnowledgeBaseCacheTTL = 30 * time.Minute // 30 minutes cache for knowledge base per chat

	// Dashboard cache TTLs
	DashboardCacheTTL     = 30 * time.Minute // 30 minutes cache for individual dashboards
	DashboardListCacheTTL = 15 * time.Minute // 15 minutes cache for dashboard lists per chat
	WidgetDataCacheTTL    = 5 * time.Minute  // 5 minutes cache for widget query result data
)
