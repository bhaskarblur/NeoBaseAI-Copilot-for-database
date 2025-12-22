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

	// =====================================================
	// ANTHROPIC CLAUDE MODELS (Latest as of December 2025)
	// Claude 4.5 series are the world's best models for coding, agents, and computer use
	// All models support chat completion API
	// =====================================================

	// Claude 4.5 Series (Latest - State of the Art)
	{
		ID:                  "claude-opus-4-5-20251101",
		Provider:            Claude,
		DisplayName:         "Claude Opus 4.5 (Best in World)",
		IsEnabled:           true,
		MaxCompletionTokens: 16384,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "World's best model for coding, agents, and computer use. State-of-the-art across all domains with 2x token efficiency",
	},
	{
		ID:                  "claude-sonnet-4-5",
		Provider:            Claude,
		DisplayName:         "Claude Sonnet 4.5 (Frontier Intelligence)",
		IsEnabled:           true,
		MaxCompletionTokens: 16384,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Best coding model in the world. State-of-the-art on SWE-bench, strongest for complex agents, most aligned model",
	},
	{
		ID:                  "claude-haiku-4-5",
		Provider:            Claude,
		DisplayName:         "Claude Haiku 4.5 (Fast Intelligence)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "State-of-the-art coding with unprecedented speed and cost-efficiency. Matches older frontier models at fraction of cost",
	},

	// Claude 4 Series (Reliable Production)
	{
		ID:                  "claude-sonnet-4",
		Provider:            Claude,
		DisplayName:         "Claude Sonnet 4 (Production Workhorse)",
		IsEnabled:           true,
		Default:             ptrBool(true),
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Reliable production model with frontier performance, improved coding over 3.7. Practical for most AI use cases and high-volume tasks",
	},

	// Claude 3.5 Series (Production Ready)
	{
		ID:                  "claude-3-5-sonnet-20241022",
		Provider:            Claude,
		DisplayName:         "Claude 3.5 Sonnet v2 (Oct 2024)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Previous generation flagship with excellent coding and reasoning, reliable for production use",
	},
	{
		ID:                  "claude-3-5-sonnet-20240620",
		Provider:            Claude,
		DisplayName:         "Claude 3.5 Sonnet v1 (June 2024)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "First version of Claude 3.5 Sonnet, highly capable for most enterprise tasks",
	},
	{
		ID:                  "claude-3-5-haiku-20241022",
		Provider:            Claude,
		DisplayName:         "Claude 3.5 Haiku (Fast)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Fast and cost-effective 3.5 model for high-volume production tasks",
	},

	// Claude 3 Series (Stable Legacy)
	{
		ID:                  "claude-3-opus-20240229",
		Provider:            Claude,
		DisplayName:         "Claude 3 Opus (Legacy Powerful)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Legacy Claude 3 model for complex tasks, superseded by 4.5 series",
	},
	{
		ID:                  "claude-3-sonnet-20240229",
		Provider:            Claude,
		DisplayName:         "Claude 3 Sonnet (Legacy Balanced)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Legacy balanced model for standard workloads, superseded by 4.5 series",
	},
	{
		ID:                  "claude-3-haiku-20240307",
		Provider:            Claude,
		DisplayName:         "Claude 3 Haiku (Legacy Fast)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     200000,
		Description:         "Legacy fast model for simple tasks, superseded by Haiku 4.5",
	},

	// =====================================================
	// OLLAMA MODELS (Open Source & Self-Hosted)
	// Popular models with millions of downloads from ollama.com/library
	// =====================================================

	// === REASONING MODELS ===
	// DeepSeek R1 Series (74.6M+ pulls) - State-of-the-art reasoning
	{
		ID:                  "deepseek-r1:latest",
		Provider:            Ollama,
		DisplayName:         "DeepSeek R1 (Top Reasoning)",
		IsEnabled:           true,
		Default:             ptrBool(true),
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "State-of-the-art reasoning model rivaling GPT-4, excellent for complex problem-solving and analysis",
	},
	{
		ID:                  "deepseek-r1:70b",
		Provider:            Ollama,
		DisplayName:         "DeepSeek R1 70B (Powerful Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "70B parameter reasoning model for demanding analytical tasks",
	},
	{
		ID:                  "deepseek-r1:8b",
		Provider:            Ollama,
		DisplayName:         "DeepSeek R1 8B (Fast Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Lightweight reasoning model running efficiently on consumer hardware",
	},
	{
		ID:                  "qwq:32b",
		Provider:            Ollama,
		DisplayName:         "QwQ 32B (Qwen Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Alibaba's reasoning model from Qwen series, 1.9M+ pulls, strong math and logic",
	},

	// === META LLAMA FAMILY ===
	// Llama 3.1 Series (107.8M+ pulls) - Most popular
	{
		ID:                  "llama3.1:latest",
		Provider:            Ollama,
		DisplayName:         "Llama 3.1 8B (Most Popular)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Most downloaded model on Ollama (107M+ pulls), excellent all-around performance",
	},
	{
		ID:                  "llama3.1:70b",
		Provider:            Ollama,
		DisplayName:         "Llama 3.1 70B (Flagship)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Meta's flagship model with best quality, competitive with GPT-4",
	},
	{
		ID:                  "llama3.1:405b",
		Provider:            Ollama,
		DisplayName:         "Llama 3.1 405B (Largest Open)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Largest open-source model, requires high-end hardware but exceptional quality",
	},
	{
		ID:                  "llama3.3:70b",
		Provider:            Ollama,
		DisplayName:         "Llama 3.3 70B (Latest Quality)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Newest Llama 3.3 with quality matching 405B model (2.8M+ pulls)",
	},
	{
		ID:                  "llama3.2:latest",
		Provider:            Ollama,
		DisplayName:         "Llama 3.2 (Lightweight)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Small 1B-3B models for edge devices and rapid inference (50M+ pulls)",
	},

	// === QWEN FAMILY (ALIBABA) ===
	// Qwen 2.5 Series (18.3M+ pulls)
	{
		ID:                  "qwen2.5-coder:latest",
		Provider:            Ollama,
		DisplayName:         "Qwen 2.5 Coder (Top Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Best open-source coding model (9.3M+ pulls), excels at code generation and debugging",
	},
	{
		ID:                  "qwen2.5:32b",
		Provider:            Ollama,
		DisplayName:         "Qwen 2.5 32B (Powerful General)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "32B general purpose with 128K context, excellent multilingual support",
	},
	{
		ID:                  "qwen2.5:72b",
		Provider:            Ollama,
		DisplayName:         "Qwen 2.5 72B (Flagship)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Largest Qwen 2.5 model with best performance across all tasks",
	},
	// Qwen 3 Series (15.4M+ pulls) - Latest
	{
		ID:                  "qwen3:latest",
		Provider:            Ollama,
		DisplayName:         "Qwen 3 (Latest Generation)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Latest Qwen with MoE architecture and improved reasoning capabilities",
	},
	{
		ID:                  "qwen3-coder:latest",
		Provider:            Ollama,
		DisplayName:         "Qwen 3 Coder (Latest Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Latest coding model from Qwen 3 series (1.4M+ pulls)",
	},

	// === DEEPSEEK CODING ===
	{
		ID:                  "deepseek-coder-v2:latest",
		Provider:            Ollama,
		DisplayName:         "DeepSeek Coder V2 (Strong Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "GPT-4 level coding model with MoE architecture (1.3M+ pulls)",
	},
	{
		ID:                  "deepseek-v3:latest",
		Provider:            Ollama,
		DisplayName:         "DeepSeek V3 (MoE 671B)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Massive MoE model with 671B parameters, 37B active (3M+ pulls)",
	},

	// === GOOGLE GEMMA ===
	{
		ID:                  "gemma3:latest",
		Provider:            Ollama,
		DisplayName:         "Gemma 3 (Google Latest)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Latest Google model with vision support (28.5M+ pulls)",
	},
	{
		ID:                  "gemma2:27b",
		Provider:            Ollama,
		DisplayName:         "Gemma 2 27B (Powerful)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Largest Gemma 2 model, excellent performance (11.6M+ pulls)",
	},
	{
		ID:                  "gemma2:9b",
		Provider:            Ollama,
		DisplayName:         "Gemma 2 9B (Balanced)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Balanced 9B model with good quality and speed",
	},

	// === MISTRAL FAMILY ===
	{
		ID:                  "mistral:latest",
		Provider:            Ollama,
		DisplayName:         "Mistral 7B (Efficient)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     32000,
		Description:         "Fast and efficient 7B model (23.3M+ pulls), great balance of speed and quality",
	},
	{
		ID:                  "mistral-nemo:12b",
		Provider:            Ollama,
		DisplayName:         "Mistral Nemo 12B (128K Context)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "12B model with 128K context by Mistral + NVIDIA (3.1M+ pulls)",
	},
	{
		ID:                  "mistral-large:123b",
		Provider:            Ollama,
		DisplayName:         "Mistral Large 123B (Flagship)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Mistral's flagship with 128K context, top-tier quality (299K+ pulls)",
	},
	{
		ID:                  "mixtral:8x7b",
		Provider:            Ollama,
		DisplayName:         "Mixtral 8x7B (MoE)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     32000,
		Description:         "Mixture of Experts model with 47B params, 13B active (1.5M+ pulls)",
	},
	{
		ID:                  "mixtral:8x22b",
		Provider:            Ollama,
		DisplayName:         "Mixtral 8x22B (Large MoE)",
		IsEnabled:           true,
		MaxCompletionTokens: 8192,
		Temperature:         1,
		InputTokenLimit:     64000,
		Description:         "Larger MoE with 141B params, 39B active per token",
	},

	// === PHI FAMILY (MICROSOFT) ===
	{
		ID:                  "phi4:latest",
		Provider:            Ollama,
		DisplayName:         "Phi 4 14B (Microsoft Latest)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     16000,
		Description:         "Microsoft's latest small model with strong reasoning (6.6M+ pulls)",
	},
	{
		ID:                  "phi3:14b",
		Provider:            Ollama,
		DisplayName:         "Phi 3 14B (Efficient)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Lightweight 14B model with excellent quality (15.2M+ pulls)",
	},

	// === CODING SPECIALISTS ===
	{
		ID:                  "codellama:latest",
		Provider:            Ollama,
		DisplayName:         "Code Llama 7B (Meta Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     16000,
		Description:         "Meta's specialized coding model, reliable and fast (3.7M+ pulls)",
	},
	{
		ID:                  "codellama:34b",
		Provider:            Ollama,
		DisplayName:         "Code Llama 34B (Powerful Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     16000,
		Description:         "Larger Code Llama for complex coding tasks",
	},
	{
		ID:                  "starcoder2:15b",
		Provider:            Ollama,
		DisplayName:         "StarCoder2 15B (Code Generation)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     16000,
		Description:         "Open code generation model trained on 600+ languages (1.6M+ pulls)",
	},
	{
		ID:                  "granite-code:20b",
		Provider:            Ollama,
		DisplayName:         "Granite Code 20B (IBM Coding)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     8000,
		Description:         "IBM's code intelligence model (397K+ pulls)",
	},

	// === VISION MODELS ===
	{
		ID:                  "llava:latest",
		Provider:            Ollama,
		DisplayName:         "LLaVA (Vision + Language)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Multimodal model combining vision and language (11.9M+ pulls)",
	},
	{
		ID:                  "llama3.2-vision:11b",
		Provider:            Ollama,
		DisplayName:         "Llama 3.2 Vision 11B (Image Reasoning)",
		IsEnabled:           true,
		MaxCompletionTokens: 4096,
		Temperature:         1,
		InputTokenLimit:     128000,
		Description:         "Vision-enabled Llama for image understanding and reasoning (3.3M+ pulls)",
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
