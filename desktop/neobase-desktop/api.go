package main

import (
	"context"
	"fmt"
	"time"
)

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// Connection represents a database connection
type Connection struct {
	Type           string `json:"type"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	Database       string `json:"database"`
	SSL            bool   `json:"ssl"`
	SSLMode        string `json:"ssl_mode,omitempty"`
	ConnectionName string `json:"connection_name"`
}

// Chat represents a chat session with a database
type Chat struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	ConnectionName    string     `json:"connection_name"`
	ConnectionDetails Connection `json:"connection_details"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	SelectedTables    string     `json:"selected_tables"`
	AutoExecuteQuery  bool       `json:"auto_execute_query"`
}

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	ChatID    string    `json:"chat_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// APIService handles API requests
type APIService struct {
	ctx        context.Context
	user       *User
	chats      []Chat
	messages   map[string][]Message
	isLoggedIn bool
}

// NewAPIService creates a new API service
func NewAPIService() *APIService {
	return &APIService{
		messages: make(map[string][]Message),
		chats:    []Chat{},
	}
}

// SetContext sets the context for the API service
func (a *APIService) SetContext(ctx context.Context) {
	a.ctx = ctx
}

// Login handles user login
func (a *APIService) Login(username, password string) map[string]interface{} {
	// In a real app, you would validate credentials against a database
	// For this demo, we'll accept any non-empty username/password
	if username == "" || password == "" {
		return map[string]interface{}{
			"success": false,
			"message": "Username and password are required",
		}
	}

	a.user = &User{
		ID:        "user-1",
		Username:  username,
		CreatedAt: time.Now(),
	}
	a.isLoggedIn = true

	return map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"user": a.user,
		},
	}
}

// Signup handles user registration
func (a *APIService) Signup(username, password string) map[string]interface{} {
	// In a real app, you would create a new user in the database
	// For this demo, we'll accept any non-empty username/password
	if username == "" || password == "" {
		return map[string]interface{}{
			"success": false,
			"message": "Username and password are required",
		}
	}

	a.user = &User{
		ID:        "user-1",
		Username:  username,
		CreatedAt: time.Now(),
	}
	a.isLoggedIn = true

	return map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"user": a.user,
		},
	}
}

// GetUser returns the current user
func (a *APIService) GetUser() map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	return map[string]interface{}{
		"success": true,
		"data":    a.user,
	}
}

// Logout logs out the current user
func (a *APIService) Logout() map[string]interface{} {
	a.user = nil
	a.isLoggedIn = false

	return map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	}
}

// GetChats returns all chats for the current user
func (a *APIService) GetChats() map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	return map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"chats": a.chats,
		},
	}
}

// CreateChat creates a new chat
func (a *APIService) CreateChat(connection Connection, autoExecuteQuery bool) map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	// In a real app, you would validate the connection and create a chat in the database
	chat := Chat{
		ID:                fmt.Sprintf("chat-%d", len(a.chats)+1),
		UserID:            a.user.ID,
		ConnectionName:    connection.ConnectionName,
		ConnectionDetails: connection,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		SelectedTables:    "",
		AutoExecuteQuery:  autoExecuteQuery,
	}

	a.chats = append(a.chats, chat)
	a.messages[chat.ID] = []Message{}

	return map[string]interface{}{
		"success": true,
		"data":    chat,
	}
}

// GetMessages returns all messages for a chat
func (a *APIService) GetMessages(chatID string) map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	messages, ok := a.messages[chatID]
	if !ok {
		return map[string]interface{}{
			"success": false,
			"message": "Chat not found",
		}
	}

	return map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"messages": messages,
		},
	}
}

// SendMessage sends a message to a chat
func (a *APIService) SendMessage(chatID, content string) map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	messages, ok := a.messages[chatID]
	if !ok {
		return map[string]interface{}{
			"success": false,
			"message": "Chat not found",
		}
	}

	// Create user message
	userMessage := Message{
		ID:        fmt.Sprintf("msg-%d", len(messages)+1),
		ChatID:    chatID,
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}

	// Add user message to chat
	a.messages[chatID] = append(a.messages[chatID], userMessage)

	// Create AI response (in a real app, this would be generated by an AI model)
	aiMessage := Message{
		ID:        fmt.Sprintf("msg-%d", len(messages)+2),
		ChatID:    chatID,
		Role:      "assistant",
		Content:   "This is a simulated AI response. In a real application, this would be generated by an AI model based on your database query.",
		CreatedAt: time.Now(),
	}

	// Add AI message to chat
	a.messages[chatID] = append(a.messages[chatID], aiMessage)

	return map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"message":   userMessage,
			"stream_id": "stream-1", // In a real app, this would be a unique ID for the stream
		},
	}
}

// ClearChat clears all messages from a chat
func (a *APIService) ClearChat(chatID string) map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	_, ok := a.messages[chatID]
	if !ok {
		return map[string]interface{}{
			"success": false,
			"message": "Chat not found",
		}
	}

	a.messages[chatID] = []Message{}

	return map[string]interface{}{
		"success": true,
		"message": "Chat cleared successfully",
	}
}

// DeleteChat deletes a chat
func (a *APIService) DeleteChat(chatID string) map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	for i, chat := range a.chats {
		if chat.ID == chatID {
			a.chats = append(a.chats[:i], a.chats[i+1:]...)
			delete(a.messages, chatID)
			return map[string]interface{}{
				"success": true,
				"message": "Chat deleted successfully",
			}
		}
	}

	return map[string]interface{}{
		"success": false,
		"message": "Chat not found",
	}
}

// UpdateSelectedTables updates the selected tables for a chat
func (a *APIService) UpdateSelectedTables(chatID, selectedTables string) map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	for i, chat := range a.chats {
		if chat.ID == chatID {
			a.chats[i].SelectedTables = selectedTables
			a.chats[i].UpdatedAt = time.Now()
			return map[string]interface{}{
				"success": true,
				"message": "Selected tables updated successfully",
			}
		}
	}

	return map[string]interface{}{
		"success": false,
		"message": "Chat not found",
	}
}

// UpdateAutoExecuteQuery updates the auto-execute query setting for a chat
func (a *APIService) UpdateAutoExecuteQuery(chatID string, autoExecuteQuery bool) map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	for i, chat := range a.chats {
		if chat.ID == chatID {
			a.chats[i].AutoExecuteQuery = autoExecuteQuery
			a.chats[i].UpdatedAt = time.Now()
			return map[string]interface{}{
				"success": true,
				"message": "Auto-execute query setting updated successfully",
			}
		}
	}

	return map[string]interface{}{
		"success": false,
		"message": "Chat not found",
	}
}

// UpdateConnection updates a connection
func (a *APIService) UpdateConnection(chatID string, connection Connection, autoExecuteQuery bool) map[string]interface{} {
	if !a.isLoggedIn || a.user == nil {
		return map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		}
	}

	for i, chat := range a.chats {
		if chat.ID == chatID {
			a.chats[i].ConnectionName = connection.ConnectionName
			a.chats[i].ConnectionDetails = connection
			a.chats[i].AutoExecuteQuery = autoExecuteQuery
			a.chats[i].UpdatedAt = time.Now()
			return map[string]interface{}{
				"success": true,
				"message": "Connection updated successfully",
			}
		}
	}

	return map[string]interface{}{
		"success": false,
		"message": "Chat not found",
	}
}
