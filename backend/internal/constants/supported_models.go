package constants

import (
	"log"
	"strings"
)

// LLMModel represents a supported LLM model configuration
type LLMModel struct {
	ID                  string  `json:"id"`                  // Unique identifier (e.g., "gpt-4o", "gemini-2.0-flash")
	Provider            string  `json:"provider"`            // Provider name (e.g., "openai", "gemini")
	DisplayName         string  `json:"displayName"`         // Human-readable name (e.g., "GPT-4 Omni")
	IsEnabled           bool    `json:"isEnabled"`           // Whether this model is available for use/allowed by the admin to use this.
	Default             *bool   `json:"default"`             // Whether this is the default model for its provider (only one per provider should be true)
	APIVersion          string  `json:"apiVersion"`          // API version for the model (e.g., "v1", "v1beta", "v1alpha" for Gemini models)
	MaxCompletionTokens int     `json:"maxCompletionTokens"` // Maximum tokens for completion
	Temperature         float64 `json:"temperature"`         // Default temperature for this model
	InputTokenLimit     int     `json:"inputTokenLimit"`     // Maximum input tokens
	Description         string  `json:"description"`         // Brief description of the model
}

var SupportedLLMModels = append(
	append(
		append(
			OpenAILLMModels,
			GeminiLLMModels...,
		),
		ClaudeLLMModels...,
	),
	OllamaLLMModels...,
)

// GetEnabledLLMModels returns only enabled LLM models
func GetEnabledLLMModels() []LLMModel {
	var enabled []LLMModel
	for _, model := range SupportedLLMModels {
		if model.IsEnabled {
			enabled = append(enabled, model)
		}
	}
	return enabled
}

// GetLLMModelsByProvider returns all models for a specific provider
func GetLLMModelsByProvider(provider string) []LLMModel {
	var models []LLMModel
	for _, model := range SupportedLLMModels {
		if model.Provider == provider && model.IsEnabled {
			models = append(models, model)
		}
	}
	return models
}

// GetLLMModel returns a specific model by ID
func GetLLMModel(modelID string) *LLMModel {
	for i := range SupportedLLMModels {
		if SupportedLLMModels[i].ID == modelID {
			return &SupportedLLMModels[i]
		}
	}
	return nil
}

// GetDefaultModelForProvider returns the default model for a provider
// Prioritizes the model with Default=true, then falls back to first enabled model
func GetDefaultModelForProvider(provider string) *LLMModel {
	// First pass: look for the model marked as default
	for i := range SupportedLLMModels {
		model := &SupportedLLMModels[i]
		if model.Provider == provider && model.IsEnabled && model.Default != nil && *model.Default {
			return model
		}
	}
	// Fallback: return first enabled model for this provider
	for i := range SupportedLLMModels {
		if SupportedLLMModels[i].Provider == provider && SupportedLLMModels[i].IsEnabled {
			return &SupportedLLMModels[i]
		}
	}
	return nil
}

// DisableUnavailableProviders disables all models for providers that don't have API keys configured
// Call this during application startup to prevent fallback to unavailable providers
func DisableUnavailableProviders(openAIKey, geminiKey, claudeKey, ollamaURL string) {
	for i := range SupportedLLMModels {
		model := &SupportedLLMModels[i]

		// Disable models for providers without API keys
		switch model.Provider {
		case OpenAI:
			if openAIKey == "" {
				model.IsEnabled = false
			}
		case Gemini:
			if geminiKey == "" {
				model.IsEnabled = false
			}
		case Claude:
			if claudeKey == "" {
				model.IsEnabled = false
			}
		case Ollama:
			if ollamaURL == "" {
				model.IsEnabled = false
			}
		}
	}
}

// GetFirstAvailableModel returns the first available (enabled) model from any provider
// Priority order: OpenAI -> Gemini -> Claude -> Ollama (matches initialization order)
func GetFirstAvailableModel() *LLMModel {
	// Try providers in order of preference
	providers := []string{OpenAI, Gemini, Claude, Ollama}
	for _, provider := range providers {
		if model := GetDefaultModelForProvider(provider); model != nil {
			return model
		}
	}
	return nil
}

// IsValidModel checks if a model ID is valid and enabled
func IsValidModel(modelID string) bool {
	model := GetLLMModel(modelID)
	return model != nil && model.IsEnabled
}

// GetAvailableModelsByAPIKeys returns only models for providers that have API keys configured
// Pass empty strings for API keys that are not available
// This filters the model list based on which API providers are actually configured
func GetAvailableModelsByAPIKeys(openAIKey, geminiKey, claudeKey, ollamaURL string) []LLMModel {
	var available []LLMModel

	for _, model := range SupportedLLMModels {
		if !model.IsEnabled {
			continue
		}

		// Include model only if its provider has an API key
		switch model.Provider {
		case OpenAI:
			if openAIKey != "" {
				available = append(available, model)
			}
		case Gemini:
			if geminiKey != "" {
				available = append(available, model)
			}
		case Claude:
			if claudeKey != "" {
				available = append(available, model)
			}
		case Ollama:
			if ollamaURL != "" {
				available = append(available, model)
			}
		}
	}

	return available
}

// GetAvailableModelsByProviderAndKeys returns models for a specific provider if API key is configured
func GetAvailableModelsByProviderAndKeys(provider, apiKey string) []LLMModel {
	if apiKey == "" {
		return []LLMModel{} // Return empty list if API key not configured
	}

	return GetLLMModelsByProvider(provider)
}

// LogModelInitialization logs which models are enabled/disabled and why
// Call this during application startup to inform admin about model availability
func LogModelInitialization(openAIKey, geminiKey, claudeKey, ollamaURL string) {
	separator := strings.Repeat("=", 80)
	log.Println("\n" + separator)
	log.Println("🤖 LLM MODEL INITIALIZATION REPORT")
	log.Println(separator)

	availableModels := GetAvailableModelsByAPIKeys(openAIKey, geminiKey, claudeKey, ollamaURL)

	// Count by provider
	openAIModels := GetLLMModelsByProvider(OpenAI)
	geminiModels := GetLLMModelsByProvider(Gemini)
	claudeModels := GetLLMModelsByProvider(Claude)
	ollamaModels := GetLLMModelsByProvider(Ollama)

	openAIAvailable := []LLMModel{}
	geminiAvailable := []LLMModel{}
	claudeAvailable := []LLMModel{}
	ollamaAvailable := []LLMModel{}

	for _, model := range availableModels {
		switch model.Provider {
		case OpenAI:
			openAIAvailable = append(openAIAvailable, model)
		case Gemini:
			geminiAvailable = append(geminiAvailable, model)
		case Claude:
			claudeAvailable = append(claudeAvailable, model)
		case Ollama:
			ollamaAvailable = append(ollamaAvailable, model)
		}
	}

	// OpenAI Status
	log.Println("\n📘 OPENAI MODELS:")
	if openAIKey != "" {
		log.Printf("  ✅ API Key: CONFIGURED\n")
		log.Printf("  📊 Available Models: %d/%d\n", len(openAIAvailable), len(openAIModels))
		for _, model := range openAIAvailable {
			log.Printf("     • %s (%s)\n", model.DisplayName, model.ID)
		}
	} else {
		log.Printf("  ❌ API Key: NOT CONFIGURED\n")
		log.Printf("  📊 Available Models: 0/%d\n", len(openAIModels))
		log.Printf("  ⚠️  To enable OpenAI models, set OPENAI_API_KEY environment variable\n")
	}

	// Gemini Status
	log.Println("\n🔵 GOOGLE GEMINI MODELS:")
	if geminiKey != "" {
		log.Printf("  ✅ API Key: CONFIGURED\n")
		log.Printf("  📊 Available Models: %d/%d\n", len(geminiAvailable), len(geminiModels))
		for _, model := range geminiAvailable {
			log.Printf("     • %s (%s)\n", model.DisplayName, model.ID)
		}
	} else {
		log.Printf("  ❌ API Key: NOT CONFIGURED\n")
		log.Printf("  📊 Available Models: 0/%d\n", len(geminiModels))
		log.Printf("  ⚠️  To enable Gemini models, set GEMINI_API_KEY environment variable\n")
	}

	// Claude Status
	log.Println("\n🟣 ANTHROPIC CLAUDE MODELS:")
	if claudeKey != "" {
		log.Printf("  ✅ API Key: CONFIGURED\n")
		log.Printf("  📊 Available Models: %d/%d\n", len(claudeAvailable), len(claudeModels))
		for _, model := range claudeAvailable {
			log.Printf("     • %s (%s)\n", model.DisplayName, model.ID)
		}
	} else {
		log.Printf("  ❌ API Key: NOT CONFIGURED\n")
		log.Printf("  📊 Available Models: 0/%d\n", len(claudeModels))
		log.Printf("  ⚠️  To enable Claude models, set CLAUDE_API_KEY environment variable\n")
	}

	// Ollama Status
	log.Println("\n🦙 OLLAMA MODELS (Self-Hosted):")
	if ollamaURL != "" {
		log.Printf("  ✅ Base URL: CONFIGURED (%s)\n", ollamaURL)
		log.Printf("  📊 Available Models: %d/%d\n", len(ollamaAvailable), len(ollamaModels))
		for _, model := range ollamaAvailable {
			log.Printf("     • %s (%s)\n", model.DisplayName, model.ID)
		}
	} else {
		log.Printf("  ❌ Base URL: NOT CONFIGURED\n")
		log.Printf("  📊 Available Models: 0/%d\n", len(ollamaModels))
		log.Printf("  ⚠️  To enable Ollama models, set OLLAMA_BASE_URL environment variable\n")
	}

	log.Printf("\n📌 TOTAL AVAILABLE MODELS: %d/%d\n", len(availableModels), len(SupportedLLMModels))

	if len(availableModels) == 0 {
		log.Println("\n⚠️  WARNING: No LLM models are available!")
		log.Println("   Please configure at least one provider:")
		log.Println("   - OPENAI_API_KEY for OpenAI models")
		log.Println("   - GEMINI_API_KEY for Google Gemini models")
		log.Println("   - CLAUDE_API_KEY for Anthropic Claude models")
		log.Println("   - OLLAMA_BASE_URL for self-hosted Ollama models")
	}

	log.Println(separator + "\n")
}

// ptrBool is a helper function to create a pointer to a bool value
func ptrBool(b bool) *bool {
	return &b
}
