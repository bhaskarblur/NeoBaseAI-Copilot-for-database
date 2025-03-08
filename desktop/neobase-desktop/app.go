package main

import (
	"context"
)

// App struct
type App struct {
	ctx        context.Context
	apiService *APIService
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		apiService: NewAPIService(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.apiService.SetContext(ctx)
}

// Login handles user login
func (a *App) Login(username, password string) map[string]interface{} {
	return a.apiService.Login(username, password)
}

// Signup handles user registration
func (a *App) Signup(username, password string) map[string]interface{} {
	return a.apiService.Signup(username, password)
}

// GetUser returns the current user
func (a *App) GetUser() map[string]interface{} {
	return a.apiService.GetUser()
}

// Logout logs out the current user
func (a *App) Logout() map[string]interface{} {
	return a.apiService.Logout()
}

// GetChats returns all chats for the current user
func (a *App) GetChats() map[string]interface{} {
	return a.apiService.GetChats()
}

// CreateChat creates a new chat
func (a *App) CreateChat(connection Connection, autoExecuteQuery bool) map[string]interface{} {
	return a.apiService.CreateChat(connection, autoExecuteQuery)
}

// GetMessages returns all messages for a chat
func (a *App) GetMessages(chatID string) map[string]interface{} {
	return a.apiService.GetMessages(chatID)
}

// SendMessage sends a message to a chat
func (a *App) SendMessage(chatID, content string) map[string]interface{} {
	return a.apiService.SendMessage(chatID, content)
}

// ClearChat clears all messages from a chat
func (a *App) ClearChat(chatID string) map[string]interface{} {
	return a.apiService.ClearChat(chatID)
}

// DeleteChat deletes a chat
func (a *App) DeleteChat(chatID string) map[string]interface{} {
	return a.apiService.DeleteChat(chatID)
}

// UpdateSelectedTables updates the selected tables for a chat
func (a *App) UpdateSelectedTables(chatID, selectedTables string) map[string]interface{} {
	return a.apiService.UpdateSelectedTables(chatID, selectedTables)
}

// UpdateAutoExecuteQuery updates the auto-execute query setting for a chat
func (a *App) UpdateAutoExecuteQuery(chatID string, autoExecuteQuery bool) map[string]interface{} {
	return a.apiService.UpdateAutoExecuteQuery(chatID, autoExecuteQuery)
}

// UpdateConnection updates a connection
func (a *App) UpdateConnection(chatID string, connection Connection, autoExecuteQuery bool) map[string]interface{} {
	return a.apiService.UpdateConnection(chatID, connection, autoExecuteQuery)
}
