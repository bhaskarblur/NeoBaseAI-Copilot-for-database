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

// SupportedLLMModels contains all available LLM models
// Models are organized by provider and sorted by capability/recency
// Data sourced from official OpenAI and Google Gemini API documentation
var SupportedLLMModels = []LLMModel{
	// =====================================================
	// OPENAI MODELS (Latest as of December 2025)
	// Chat Completion Models Only
	// =====================================================
	// GPT-5 Series (Frontier Models - Chat Completions)
	{
		ID:                  "gpt-5.2",
		Provider:            OpenAI,
		DisplayName:         "GPT-5.2 (Best for Coding & Agentic)",
		IsEnabled:           true,
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Most advanced frontier model, best for coding tasks and agentic applications across all industries",
	},
	{
		ID:                  "gpt-5",
		Provider:            OpenAI,
		DisplayName:         "GPT-5 (Full Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Full reasoning model with configurable reasoning effort for complex problem-solving tasks",
	},
	{
		ID:                  "gpt-5-mini",
		Provider:            OpenAI,
		DisplayName:         "GPT-5 Mini (Fast & Cost-Efficient)",
		IsEnabled:           true,
		MaxCompletionTokens: 50000,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Faster, cost-efficient version of GPT-5 for well-defined tasks with good performance",
	},
	{
		ID:                  "gpt-5-nano",
		Provider:            OpenAI,
		DisplayName:         "GPT-5 Nano (Fastest)",
		IsEnabled:           true,
		MaxCompletionTokens: 30000,
		Temperature:         1,
		InputTokenLimit:     100000,
		Description:         "Fastest and most cost-efficient version of GPT-5 for rapid inference",
	},
	// Reasoning Models (O-Series - Chat Completions)
	{
		ID:                  "o3",
		Provider:            OpenAI,
		DisplayName:         "O3 (Complex Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Reasoning model for complex tasks, succeeded by GPT-5 but still available for specific use cases",
	},
	{
		ID:                  "o3-pro",
		Provider:            OpenAI,
		DisplayName:         "O3 Pro (Enhanced Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Version of O3 with more compute for better reasoning responses and complex problem analysis",
	},
	{
		ID:                  "o3-mini",
		Provider:            OpenAI,
		DisplayName:         "O3 Mini (Fast Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 50000,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Small model alternative to O3, faster and more cost-effective for reasoning tasks",
	},
	{
		ID:                  "o3-deep-research",
		Provider:            OpenAI,
		DisplayName:         "O3 Deep Research (Research)",
		IsEnabled:           true,
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Most advanced research model for deep, complex analysis of large datasets and documents",
	},
	// GPT-4.1 Series (Chat Completions)
	{
		ID:                  "gpt-4.1",
		Provider:            OpenAI,
		DisplayName:         "GPT-4.1 (Smartest Non-Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 30000,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Smartest non-reasoning model, excellent for general purpose tasks without reasoning overhead",
	},
	{
		ID:                  "gpt-4.1-mini",
		Provider:            OpenAI,
		DisplayName:         "GPT-4.1 Mini (Fast General)",
		IsEnabled:           true,
		MaxCompletionTokens: 20000,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Smaller, faster version of GPT-4.1 for focused general-purpose tasks",
	},
	// GPT-4o Series (Chat Completions - Multimodal)
	{
		ID:                  "gpt-4o",
		Provider:            OpenAI,
		DisplayName:         "GPT-4o (Omni - Fast & Intelligent)",
		IsEnabled:           true,
		Default:             ptrBool(true),
		MaxCompletionTokens: 30000,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Fast, intelligent, and flexible multimodal model with vision and audio capabilities",
	},
	{
		ID:                  "gpt-4o-mini",
		Provider:            OpenAI,
		DisplayName:         "GPT-4o Mini (Lightweight)",
		IsEnabled:           true,
		MaxCompletionTokens: 20000,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Fast and affordable small model for focused tasks, supports text and vision",
	},
	// Previous Generation (Chat Completions)
	{
		ID:                  "gpt-4-turbo",
		Provider:            OpenAI,
		DisplayName:         "GPT-4 Turbo (Previous Generation)",
		IsEnabled:           true,
		MaxCompletionTokens: 20000,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Older high-intelligence GPT-4 variant, still available for compatibility",
	},
	{
		ID:                  "gpt-3.5-turbo",
		Provider:            OpenAI,
		DisplayName:         "GPT-3.5 Turbo (Legacy)",
		IsEnabled:           true,
		MaxCompletionTokens: 15000,
		Temperature:         1,
		InputTokenLimit:     16385,
		Description:         "Legacy GPT model for cheaper chat tasks, maintained for backward compatibility",
	},

	// =====================================================
	// GOOGLE GEMINI MODELS (Latest as of December 2025)
	// =====================================================
	// Gemini 3 Series (Frontier Models)
	{
		ID:                  "gemini-3-pro-preview",
		Provider:            Gemini,
		DisplayName:         "Gemini 3 Pro (Most Intelligent)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Best model in the world for multimodal understanding with state-of-the-art reasoning and agentic capabilities",
	},
	{
		ID:                  "gemini-3-flash-preview",
		Provider:            Gemini,
		DisplayName:         "Gemini 3 Flash (Frontier Speed)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Most intelligent model built for speed, combining frontier intelligence with superior search and grounding",
	},
	// Gemini 2.5 Series (Advanced)
	{
		ID:                  "gemini-2.5-pro",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.5 Pro (Advanced Reasoning)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "State-of-the-art thinking model capable of reasoning over complex problems in code, math, and STEM",
	},
	{
		ID:                  "gemini-2.5-flash",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.5 Flash (Best Price-Performance)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 100000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Best model for price-performance with well-rounded capabilities, ideal for large-scale processing and agentic tasks",
	},
	{
		ID:                  "gemini-2.5-flash-lite",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.5 Flash-Lite (Ultra-Fast)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 50000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Fastest flash model optimized for cost-efficiency and high throughput on repetitive tasks",
	},
	// Gemini 2.0 Series (Previous Workhorse)
	{
		ID:                  "gemini-2.0-flash",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.0 Flash (Workhorse)",
		IsEnabled:           true,
		Default:             ptrBool(true),
		APIVersion:          "v1beta",
		MaxCompletionTokens: 30000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Second generation workhorse model with 1M token context window for large document processing",
	},
	{
		ID:                  "gemini-2.0-flash-lite",
		Provider:            Gemini,
		DisplayName:         "Gemini 2.0 Flash-Lite (Previous Fast)",
		IsEnabled:           true,
		APIVersion:          "v1beta",
		MaxCompletionTokens: 20000,
		Temperature:         1,
		InputTokenLimit:     1000000,
		Description:         "Second generation small workhorse model with 1M token context, lightweight version",
	},
}

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

// GetDefaultModelForProvider returns the default (first enabled) model for a provider
func GetDefaultModelForProvider(provider string) *LLMModel {
	for i := range SupportedLLMModels {
		if SupportedLLMModels[i].Provider == provider && SupportedLLMModels[i].IsEnabled {
			return &SupportedLLMModels[i]
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
func GetAvailableModelsByAPIKeys(openAIKey, geminiKey string) []LLMModel {
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
func LogModelInitialization(openAIKey, geminiKey string) {
	separator := strings.Repeat("=", 80)
	log.Println("\n" + separator)
	log.Println("🤖 LLM MODEL INITIALIZATION REPORT")
	log.Println(separator)

	availableModels := GetAvailableModelsByAPIKeys(openAIKey, geminiKey)

	// Count by provider
	openAIModels := GetLLMModelsByProvider(OpenAI)
	geminiModels := GetLLMModelsByProvider(Gemini)
	openAIAvailable := []LLMModel{}
	geminiAvailable := []LLMModel{}

	for _, model := range availableModels {
		if model.Provider == OpenAI {
			openAIAvailable = append(openAIAvailable, model)
		} else if model.Provider == Gemini {
			geminiAvailable = append(geminiAvailable, model)
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

	log.Printf("\n📌 TOTAL AVAILABLE MODELS: %d/%d\n", len(availableModels), len(SupportedLLMModels))

	if len(availableModels) == 0 {
		log.Println("\n⚠️  WARNING: No LLM models are available!")
		log.Println("   Please configure at least one API key (OPENAI_API_KEY or GEMINI_API_KEY)")
	}

	log.Println(separator + "\n")
}

// ptrBool is a helper function to create a pointer to a bool value
func ptrBool(b bool) *bool {
	return &b
}
