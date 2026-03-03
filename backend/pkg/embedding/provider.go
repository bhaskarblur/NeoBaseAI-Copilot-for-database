package embedding

import (
	"fmt"
	"log"
)

// NewProvider creates an embedding provider based on configuration.
// If provider is empty, it auto-detects from available API keys.
func NewProvider(cfg Config) (Provider, error) {
	provider := cfg.Provider

	// Auto-detect provider from API key if not explicitly set
	if provider == "" {
		if cfg.APIKey != "" {
			// Caller should set provider; this is a fallback
			return nil, fmt.Errorf("embedding provider must be specified when API key is provided")
		}
		return nil, fmt.Errorf("no embedding provider configured")
	}

	switch provider {
	case "openai":
		model := cfg.Model
		if model == "" {
			model = DefaultOpenAIModel
		}
		return NewOpenAIProvider(cfg.APIKey, model)

	case "gemini":
		model := cfg.Model
		if model == "" {
			model = DefaultGeminiModel
		}
		return NewGeminiProvider(cfg.APIKey, model)

	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", provider)
	}
}

// AutoDetectProvider creates an embedding provider by auto-detecting from available API keys.
// Priority: OpenAI (higher quality) > Gemini (free).
func AutoDetectProvider(openAIKey, geminiKey, preferredProvider, preferredModel string) (Provider, error) {
	// If user explicitly set a provider, use that
	if preferredProvider != "" {
		apiKey := ""
		switch preferredProvider {
		case "openai":
			apiKey = openAIKey
		case "gemini":
			apiKey = geminiKey
		}
		if apiKey == "" {
			return nil, fmt.Errorf("embedding provider %s configured but no API key available", preferredProvider)
		}
		return NewProvider(Config{
			Provider: preferredProvider,
			Model:    preferredModel,
			APIKey:   apiKey,
		})
	}

	// Auto-detect: prefer OpenAI > Gemini
	if openAIKey != "" {
		log.Printf("Embedding -> Auto-detected OpenAI as embedding provider")
		return NewProvider(Config{
			Provider: "openai",
			Model:    preferredModel,
			APIKey:   openAIKey,
		})
	}

	if geminiKey != "" {
		log.Printf("Embedding -> Auto-detected Gemini as embedding provider")
		return NewProvider(Config{
			Provider: "gemini",
			Model:    preferredModel,
			APIKey:   geminiKey,
		})
	}

	log.Printf("Embedding -> No embedding provider available (no OpenAI or Gemini API key)")
	return nil, nil // nil, nil = no provider available, not an error
}
