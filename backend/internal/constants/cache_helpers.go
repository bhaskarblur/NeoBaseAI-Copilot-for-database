package constants

import "time"

// Cache TTL constants for all repositories
const (
	// User cache TTL
	UserCacheTTL = 1 * time.Hour // 1 hour cache for user data

	// Chat cache TTLs
	ChatCacheTTL     = 30 * time.Minute // 30 minutes cache for individual chats
	ChatListCacheTTL = 5 * time.Minute  // 5 minutes cache for chat lists

	// Message cache TTLs
	MessageCacheTTL  = 15 * time.Minute // 15 minutes cache for individual messages
	MessageListTTL   = 10 * time.Minute // 10 minutes cache for message lists
	PinnedMessageTTL = 30 * time.Minute // 30 minutes cache for pinned messages

	// LLM message cache TTL
	LLMMessageCacheTTL = 15 * time.Minute // 15 minutes cache for LLM message context
)
