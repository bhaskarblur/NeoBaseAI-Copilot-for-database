package llm

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
)

type Manager struct {
	clients map[string]Client
	mu      sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]Client),
	}
}

func (m *Manager) RegisterClient(name string, config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var client Client
	var err error

	switch config.Provider {
	case "openai":
		client, err = NewOpenAIClient(config)
	case "gemini":
		client, err = NewGeminiClient(config)
	case "claude":
		client, err = NewClaudeClient(config)
	case "ollama":
		client, err = NewOllamaClient(config)
	// Add other providers here
	default:
		return fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}

	if err != nil {
		return fmt.Errorf("failed to create LLM client: %v", err)
	}

	m.clients[name] = client
	return nil
}

func (m *Manager) GetClient(name string) (Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[name]
	if !exists {
		return nil, fmt.Errorf("LLM client not found: %s", name)
	}

	return client, nil
}

func (m *Manager) RemoveClient(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, name)
}

// Add helper function to properly format assistant response
func formatAssistantResponse(response map[string]interface{}) string {
	// Convert the response to JSON string
	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Printf("Error formatting assistant response: %v", err)
		return fmt.Sprintf("%v", response)
	}
	return string(jsonBytes)
}

// getAssistantContent safely extracts assistant response content from a message's
// Content map, handling both map[string]interface{} (normal JSON responses) and
// string (legacy/fallback format) types for the "assistant_response" key.
func getAssistantContent(content map[string]interface{}) string {
	if assistantMsg, ok := content["assistant_response"].(map[string]interface{}); ok {
		return formatAssistantResponse(assistantMsg)
	}
	if assistantStr, ok := content["assistant_response"].(string); ok {
		return assistantStr
	}
	return ""
}

// Helper functions
func mapRole(role string) string {
	switch strings.ToLower(role) {
	case "user":
		return "user"
	case "assistant":
		return "assistant"
	case "system":
		return "system"
	default:
		return "user"
	}
}
