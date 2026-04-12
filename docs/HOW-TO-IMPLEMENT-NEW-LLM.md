# How to Add a New LLM Provider to NeoBase

This document is the canonical reference for contributors adding support for a new LLM provider
(e.g. Mistral, Cohere, a new Anthropic API surface, a self-hosted vLLM endpoint, etc.).

It also includes a **short-form section (Part B)** for the far more common task of adding a new
*model* to an *existing* provider — which requires touching only one file.

---

## Architecture overview

Before touching any file, internalise the data-flow:

```
HTTP request (user message + chat history)
    │
    ▼
ChatService / VisualizationService
    │  Calls LLMManager.GetClient(providerName)
    ▼
LLM Manager  (pkg/llm/manager.go)
    │  Looks up a registered Client by provider name
    ▼
Provider Client  (pkg/llm/openai.go | gemini.go | claude.go | ollama.go)
    │  GenerateResponse / GenerateVisualization / GenerateWithTools / GenerateRawJSON
    ▼
Provider API  (OpenAI, Google Gemini, Anthropic Claude, Ollama, ...)
    │
    ▼
Raw JSON response → parsed into AssistantResponse struct → returned to user
```

**Key design decisions you must understand before starting:**

1. **The `Client` interface** (`pkg/llm/types.go`) is the only contract between the service
   layer and any LLM provider. Implementing this interface is ~80% of the work for a new provider.

2. **Response schemas are provider-specific.** Each provider has a different mechanism for
   structured output (OpenAI uses JSON Schema `response_format`, Gemini uses `genai.Schema`,
   Claude uses tool calling, Ollama uses a JSON schema string). The schema must be defined in a
   way the provider's SDK understands.

3. **System prompts are database-type-specific, NOT provider-specific.** The same text goes to
   every provider for a given DB type. When adding a new LLM provider you do NOT write or modify
   any DB prompts.

4. **Tool calling** (`GenerateWithTools`) is the iterative exploration loop used by the main chat
   flow. The LLM repeatedly calls tools (`execute_read_query`, `get_table_info`) to explore the
   DB, then calls `generate_final_response`. You must implement this loop using your provider's
   native tool-call mechanism. Study `claude.go` for the canonical full implementation.

5. **Model catalogue** is separate from the client code. `constants/supported_models.go`
   aggregates per-provider model slices. Adding a model is independent of adding a provider.

---

## Part A — Adding a Full New LLM Provider

---

### Step A.1 — Define the provider constant

**File:** `backend/internal/constants/llms.go`

```go
const (
    OpenAI   = "openai"
    Gemini   = "gemini"
    Claude   = "claude"
    Ollama   = "ollama"
    MyNewLLM = "mynewllm"   // ← ADD — must be unique, lowercase, no spaces
)
```

**Why:** This string is the registry key for `LLMManager.RegisterClient` and
`LLMManager.GetClient`, model catalogue lookups, and every `switch provider` case. A single
constant prevents typos across the codebase. The string value is also what goes in the
`ACTIVE_LLM_PROVIDER` env var if one is added.

---

### Step A.2 — Define the model catalogue

**File:** `backend/internal/constants/mynewllm.go` *(create new file)*

```go
package constants

// MyNewLLMLLMModels is the list of models available from the MyNewLLM provider.
// Add one entry per model variant. Only models with IsEnabled=true are surfaced to users.
var MyNewLLMLLMModels = []LLMModel{
    {
        ID:                  "mynewllm-pro-v1",     // exact model ID sent to the provider API
        Provider:            MyNewLLM,
        DisplayName:         "MyNewLLM Pro",        // shown in the model selector UI
        IsEnabled:           true,
        Default:             boolPtr(true),          // exactly ONE model per provider must be default
        APIVersion:          "",                     // set if provider needs a version field (e.g. Gemini uses "v1beta")
        MaxCompletionTokens: 8192,
        Temperature:         1.0,
        InputTokenLimit:     128000,
        Description:         "MyNewLLM Pro — best balance of speed and accuracy",
    },
    {
        ID:                  "mynewllm-fast-v1",
        Provider:            MyNewLLM,
        DisplayName:         "MyNewLLM Fast",
        IsEnabled:           true,
        Default:             nil,                    // not the default
        MaxCompletionTokens: 4096,
        Temperature:         1.0,
        InputTokenLimit:     64000,
        Description:         "Faster, lower cost — best for simple queries",
    },
}

// boolPtr is a helper for the Default field (already defined somewhere, search before adding)
func boolPtr(b bool) *bool { return &b }
```

Model fields explained:
- `ID` — must match the exact string the provider API expects in the model field.
- `Default: boolPtr(true)` — exactly one model per provider. This is the model used when a user
  has not selected a specific model. The fallback if `Default` is not found is the first enabled
  model in the slice.
- `MaxCompletionTokens` — hard cap on output tokens; exceeding this causes the API to truncate.
- `InputTokenLimit` — soft limit used for context management and prompt truncation decisions.

**Why:** The `LLMModel` struct is the single source of truth for model metadata. It is used by
`GetDefaultModelForProvider`, `IsValidModel`, the model-selector API, and the DI startup block.

---

### Step A.3 — Register the model catalogue in the aggregated slice

**File:** `backend/internal/constants/supported_models.go`

```go
var SupportedLLMModels = append(
    append(
        append(
            append(
                OpenAILLMModels,
                GeminiLLMModels...,
            ),
            ClaudeLLMModels...,
        ),
        OllamaLLMModels...,
    ),
    MyNewLLMLLMModels...,   // ← ADD — append last
)
```

**Why:** `SupportedLLMModels` is the single aggregated slice iterated by all helper functions
(`GetEnabledLLMModels`, `GetDefaultModelForProvider`, `IsValidModel`, etc.). The model will not
appear anywhere in the system until it is included here.

---

### Step A.4 — Define the response schemas

Structured output schemas enforce that the provider returns valid JSON in the exact shape NeoBase
expects (`query`, `assistantMessage`, `queries` array, etc.).

**File:** `backend/internal/constants/mynewllm.go`

If the provider accepts an OpenAI-compatible JSON Schema string (most modern providers do):

```go
// MyNewLLMLLMResponseSchema is the structured output schema for the main chat endpoint.
// Reuse the OpenAI schema if the provider's JSON Schema format is identical.
const MyNewLLMLLMResponseSchema = OpenAILLMResponseSchema

// If the format differs, define a custom one. Study openai.go for the expected structure.
// const MyNewLLMLLMResponseSchema = `{ "type": "object", ... }`

// MyNewLLMRecommendationsResponseSchema is used for the recommendations endpoint.
const MyNewLLMRecommendationsResponseSchema = OpenAIRecommendationsResponseSchema

// MyNewLLMVisualizationSchema is used for dashboard widget generation (if needed as a separate schema).
// Many providers reuse the main schema here.
```

If the provider uses a struct-based schema (like Gemini uses `*genai.Schema`):

```go
var MyNewLLMResponseSchema = &mynewllmsdk.Schema{
    Type: "OBJECT",
    Properties: map[string]*mynewllmsdk.Schema{
        // ... mirror the OpenAI JSON Schema structure using the SDK's types
    },
    Required: []string{"query", "assistantMessage"},
}
```

**Why:** Structured output is non-negotiable. Without it, the provider returns free-form prose
that cannot be parsed into the `AssistantResponse` struct. The parsing would fail and the user
would see an error on every message. Study `openai.go` lines ~195+ for the expected structure.

---

### Step A.5 — Wire the schemas into the dispatcher

**File:** `backend/internal/constants/llms.go`

**`GetLLMResponseSchema`** (main chat schema dispatch):
```go
func GetLLMResponseSchema(provider string, dbType string) interface{} {
    switch provider {
    case OpenAI:
        return OpenAILLMResponseSchema
    case Gemini:
        return GeminiLLMResponseSchema
    case Claude:
        return ClaudeLLMResponseSchemaJSON
    case Ollama:
        return OllamaLLMResponseSchemaJSON
    case MyNewLLM:                           // ← ADD
        return MyNewLLMLLMResponseSchema
    default:
        return OpenAILLMResponseSchema
    }
}
```

**`GetRecommendationsSchema`** (recommendations endpoint schema dispatch):
```go
func GetRecommendationsSchema(provider string) interface{} {
    switch provider {
    // ... existing cases ...
    case MyNewLLM:                           // ← ADD
        return MyNewLLMRecommendationsResponseSchema
    }
}
```

**Why:** These dispatcher functions are called at DI startup to pre-populate each provider's
`LLMDBConfig.Schema`. They must include every requested provider or the startup block will
store `nil` as the schema, causing the provider to send unstructured output requests.

---

### Step A.6 — Implement the `Client` interface

This is the largest step. Create a new file that fully implements `pkg/llm/types.go:Client`.

**File:** `backend/pkg/llm/mynewllm.go` *(create new file)*

```go
package llm

import (
    "context"
    "neobase-ai/internal/constants"
    "neobase-ai/internal/models"
    mynewllmsdk "github.com/vendor/mynewllm-go"
)

type MyNewLLMClient struct {
    client              *mynewllmsdk.Client
    config              Config
    currentModel        string
}

func NewMyNewLLMClient(config Config) (*MyNewLLMClient, error) {
    client := mynewllmsdk.NewClient(config.APIKey)
    return &MyNewLLMClient{
        client:       client,
        config:       config,
        currentModel: config.Model,
    }, nil
}

// --- Required interface methods ---

// GenerateResponse is the main chat method.
// It must:
//   1. Build the message history from the LLMMessage slice
//   2. Apply the system prompt from config.DBConfigs (matched by dbType)
//   3. Apply the response schema to enforce structured JSON output
//   4. Call the provider API
//   5. Return the raw JSON string (NOT parsed — the service layer parses it)
func (c *MyNewLLMClient) GenerateResponse(
    ctx context.Context,
    messages []*models.LLMMessage,
    dbType string,
    nonTechMode bool,
    modelID ...string,
) (string, error) {
    // 1. Find the DBConfig for this dbType
    var systemPrompt string
    var schema interface{}
    for _, cfg := range c.config.DBConfigs {
        if cfg.DBType == dbType {
            systemPrompt = cfg.SystemPrompt
            schema = cfg.Schema
            break
        }
    }
    // 2. Determine model ID
    model := c.currentModel
    if len(modelID) > 0 && modelID[0] != "" {
        model = modelID[0]
    }
    // 3. Build provider-specific request, apply structured output schema
    // 4. Call API, handle errors and retries
    // 5. Return JSON string
    _ = systemPrompt; _ = schema; _ = model // placeholder
    return "", nil
}

// GenerateRecommendations generates 60 question suggestions for the user.
func (c *MyNewLLMClient) GenerateRecommendations(
    ctx context.Context,
    messages []*models.LLMMessage,
    dbType string,
) (string, error) {
    // Use constants.RecommendationsPrompt as the system prompt
    // Use MyNewLLMRecommendationsResponseSchema for structured output
    return "", nil
}

// GenerateVisualization generates a chart suggestion for a dashboard widget.
func (c *MyNewLLMClient) GenerateVisualization(
    ctx context.Context,
    systemPrompt string,
    visualizationPrompt string,
    dataRequest string,
    modelID ...string,
) (string, error) {
    return "", nil
}

// GenerateRawJSON calls the provider with a custom system prompt and returns
// the raw JSON string. Used for KB generation where NeoBase's standard response
// schema is not appropriate.
func (c *MyNewLLMClient) GenerateRawJSON(
    ctx context.Context,
    systemPrompt string,
    userMessage string,
    modelID ...string,
) (string, error) {
    return "", nil
}

// GenerateWithTools implements the iterative tool-calling exploration loop.
// This is the most complex method. Study claude.go for the canonical implementation.
//
// The loop:
//   1. Send messages + tools to the provider
//   2. If the provider calls a tool (execute_read_query / get_table_info / generate_final_response):
//      a. execute_read_query / get_table_info: call executor(toolName, args), append result, continue
//      b. generate_final_response: return the structured result — done
//   3. If the provider returns plain text, wrap it as the final response
//   4. Repeat up to config.MaxIterations times to prevent infinite loops
func (c *MyNewLLMClient) GenerateWithTools(
    ctx context.Context,
    messages []*models.LLMMessage,
    tools []ToolDefinition,
    executor ToolExecutorFunc,
    config ToolCallConfig,
) (*ToolCallResult, error) {
    // Implementation depends heavily on the provider's tool-call API.
    // Most modern providers expose native function/tool calling.
    // If the provider does not support tool calling, use TryParseTextToolCall()
    // helper from tools.go to attempt extracting a structured response from plain text.
    return nil, nil
}

func (c *MyNewLLMClient) GetModelInfo() ModelInfo {
    return ModelInfo{
        Name:                c.currentModel,
        Provider:            constants.MyNewLLM,
        MaxCompletionTokens: c.config.MaxCompletionTokens,
        ContextLimit:        128000, // or lookup from constants
    }
}

func (c *MyNewLLMClient) SetModel(modelID string) error {
    model := constants.GetLLMModel(modelID)
    if model == nil || !model.IsEnabled {
        return fmt.Errorf("model %s not found or not enabled", modelID)
    }
    c.currentModel = modelID
    return nil
}
```

**Key contract rules for `GenerateResponse`:**
- The returned JSON string **must** deserialise into `AssistantResponse` (see `models/message.go`).
- Never return partially parsed structs — always return the raw JSON string.
- Respect `ctx` cancellation; call `ctx.Err()` before and after API calls.
- Log token usage, model name, and response time at INFO level for observability.

**Key contract rules for `GenerateWithTools`:**
- Tools are passed as `[]ToolDefinition` — convert them to the provider's native format.
- Tool results must be appended in provider-native format before the next API call.
- `generate_final_response` is the sentinel tool call that ends the loop — parse its args as
  `AssistantResponse` and return.
- Respect `config.MaxIterations` (typically 10) to prevent runaway loops.
- Call `config.OnToolCall(ToolCall{...})` for every tool call to update the UI streaming state.

**Go SDK dependency:** Add the provider's Go SDK to `go.mod`:
```bash
cd backend
go get github.com/vendor/mynewllm-go
go mod tidy
```

---

### Step A.7 — Register the client in the LLM Manager factory

**File:** `backend/pkg/llm/manager.go` — `RegisterClient` method

```go
func (m *Manager) RegisterClient(name string, config Config) error {
    var client Client
    var err error
    switch config.Provider {
    case constants.OpenAI:
        client, err = NewOpenAIClient(config)
    case constants.Gemini:
        client, err = NewGeminiClient(config)
    case constants.Claude:
        client, err = NewClaudeClient(config)
    case constants.Ollama:
        client, err = NewOllamaClient(config)
    case constants.MyNewLLM:               // ← ADD
        client, err = NewMyNewLLMClient(config)
    default:
        return fmt.Errorf("unsupported LLM provider: %s", config.Provider)
    }
    if err != nil {
        return fmt.Errorf("failed to create %s client: %v", config.Provider, err)
    }
    m.clients[name] = client
    return nil
}
```

**Why:** `RegisterClient` is the factory. Without a case here, `RegisterClient` returns an error
and the provider is never registered, so `GetClient("mynewllm")` will always fail at request time.

---

### Step A.8 — Read the API key / base URL from environment

**File:** `backend/config/env_values.go`

```go
// In the EnvConfig struct:
MyNewLLMAPIKey string  // OR: MyNewLLMBaseURL string for self-hosted providers

// In LoadEnv():
// Using empty string as the default makes the field optional.
// The application starts normally when the key is absent; the provider is simply skipped.
Env.MyNewLLMAPIKey = getRequiredEnv("MYNEWLLM_API_KEY", "")
```

**Why:** Credentials must never be hardcoded. `getRequiredEnv` with `""` as the default means
the app starts normally even when the key is absent — the provider is simply not registered.
This is how all existing providers work.

> For self-hosted providers (like Ollama), use `getEnvWithDefault("MYNEWLLM_BASE_URL", "")` and
> store the URL string in the `APIKey` field of `llm.Config` (Ollama does this today).

---

### Step A.9 — Register the provider at application startup

**File:** `backend/internal/di/modules.go`

Add a new registration block in the LLM Manager `Provide` function, mirroring the existing
provider blocks exactly:

```go
// Register MyNewLLM client if API key is available
if config.Env.MyNewLLMAPIKey != "" {
    // Get the default model from the constants catalogue
    defaultModel := constants.GetDefaultModelForProvider(constants.MyNewLLM)
    if defaultModel == nil {
        log.Printf("Warning: No default MyNewLLM model found in supported_models.go")
        // Hard-coded fallback (should not be needed if Step A.2 is done correctly)
        t := true
        defaultModel = &constants.LLMModel{
            ID:                  "mynewllm-pro-v1",
            Provider:            constants.MyNewLLM,
            MaxCompletionTokens: 8192,
            Temperature:         1.0,
            Default:             &t,
        }
    }

    err := manager.RegisterClient(constants.MyNewLLM, llm.Config{
        Provider:            constants.MyNewLLM,
        Model:               defaultModel.ID,
        APIKey:              config.Env.MyNewLLMAPIKey,
        MaxCompletionTokens: defaultModel.MaxCompletionTokens,
        Temperature:         defaultModel.Temperature,
        // DBConfigs pre-loads system prompts + schemas for every supported DB type.
        // Every DB type supported by NeoBase must appear here.
        DBConfigs: []llm.LLMDBConfig{
            {
                DBType:       constants.DatabaseTypePostgreSQL,
                Schema:       constants.GetLLMResponseSchema(constants.MyNewLLM, constants.DatabaseTypePostgreSQL),
                SystemPrompt: constants.GetSystemPrompt(constants.MyNewLLM, constants.DatabaseTypePostgreSQL, false),
            },
            {
                DBType:       constants.DatabaseTypeTimescaleDB,
                Schema:       constants.GetLLMResponseSchema(constants.MyNewLLM, constants.DatabaseTypeTimescaleDB),
                SystemPrompt: constants.GetSystemPrompt(constants.MyNewLLM, constants.DatabaseTypeTimescaleDB, false),
            },
            {
                DBType:       constants.DatabaseTypeYugabyteDB,
                Schema:       constants.GetLLMResponseSchema(constants.MyNewLLM, constants.DatabaseTypeYugabyteDB),
                SystemPrompt: constants.GetSystemPrompt(constants.MyNewLLM, constants.DatabaseTypeYugabyteDB, false),
            },
            {
                DBType:       constants.DatabaseTypeMySQL,
                Schema:       constants.GetLLMResponseSchema(constants.MyNewLLM, constants.DatabaseTypeMySQL),
                SystemPrompt: constants.GetSystemPrompt(constants.MyNewLLM, constants.DatabaseTypeMySQL, false),
            },
            {
                DBType:       constants.DatabaseTypeStarRocks,
                Schema:       constants.GetLLMResponseSchema(constants.MyNewLLM, constants.DatabaseTypeStarRocks),
                SystemPrompt: constants.GetSystemPrompt(constants.MyNewLLM, constants.DatabaseTypeStarRocks, false),
            },
            {
                DBType:       constants.DatabaseTypeClickhouse,
                Schema:       constants.GetLLMResponseSchema(constants.MyNewLLM, constants.DatabaseTypeClickhouse),
                SystemPrompt: constants.GetSystemPrompt(constants.MyNewLLM, constants.DatabaseTypeClickhouse, false),
            },
            {
                DBType:       constants.DatabaseTypeMongoDB,
                Schema:       constants.GetLLMResponseSchema(constants.MyNewLLM, constants.DatabaseTypeMongoDB),
                SystemPrompt: constants.GetSystemPrompt(constants.MyNewLLM, constants.DatabaseTypeMongoDB, false),
            },
            {
                DBType:       constants.DatabaseTypeSpreadsheet,
                Schema:       constants.GetLLMResponseSchema(constants.MyNewLLM, constants.DatabaseTypeSpreadsheet),
                SystemPrompt: constants.GetSystemPrompt(constants.MyNewLLM, constants.DatabaseTypeSpreadsheet, false),
            },
            // Add any future DB types here as they are added to the system
        },
    })
    if err != nil {
        log.Printf("Warning: Failed to register MyNewLLM client: %v", err)
    }
}
```

**Why the `if config.Env.MyNewLLMAPIKey != ""` guard:** This is how graceful degradation works.
If a user hasn't configured the provider's key, the app starts normally and the provider is just
unavailable. Without this guard, an empty key would cause `NewMyNewLLMClient` to fail, panicking
at startup.

**Why DBConfig for every DB type:** `LLMDBConfig` pre-loads the system prompt and response schema
so the client can do an O(1) lookup at request time instead of re-computing. More importantly,
it prevents race conditions from concurrent chat requests all calling `GetSystemPrompt` at once.

---

### Step A.10 — Disable unavailable provider models at startup

**File:** `backend/internal/constants/supported_models.go` — `DisableUnavailableProviders`

Update the function signature and body:

```go
// Before:
func DisableUnavailableProviders(openAIKey, geminiKey, claudeKey, ollamaURL string) {

// After:
func DisableUnavailableProviders(openAIKey, geminiKey, claudeKey, ollamaURL, myNewLLMKey string) {
    for i := range SupportedLLMModels {
        model := &SupportedLLMModels[i]
        switch model.Provider {
        case OpenAI:
            if openAIKey == "" { model.IsEnabled = false }
        case Gemini:
            if geminiKey == "" { model.IsEnabled = false }
        case Claude:
            if claudeKey == "" { model.IsEnabled = false }
        case Ollama:
            if ollamaURL == "" { model.IsEnabled = false }
        case MyNewLLM:              // ← ADD
            if myNewLLMKey == "" { model.IsEnabled = false }
        }
    }
}
```

Update both call-sites in `backend/config/env_values.go`:

```go
constants.DisableUnavailableProviders(
    Env.OpenAIAPIKey,
    Env.GeminiAPIKey,
    Env.ClaudeAPIKey,
    Env.OllamaBaseURL,
    Env.MyNewLLMAPIKey,   // ← ADD
)

// Same for LogModelInitialization if its signature also needs updating
constants.LogModelInitialization(
    Env.OpenAIAPIKey,
    Env.GeminiAPIKey,
    Env.ClaudeAPIKey,
    Env.OllamaBaseURL,
    Env.MyNewLLMAPIKey,   // ← ADD if signature matches
)
```

**Why:** Disabled models are filtered out of the model selector in the UI and excluded from the
fallback chain. Without this, a user whose admin did not configure the key would see the new
provider's models listed as available, click one, and get an empty-key API error on every message.

---

### Step A.11 — Update `GetAvailableModelsByAPIKeys`

**File:** `backend/internal/constants/supported_models.go`

This function is called by the model-selector API to return models filtered by which providers
have keys configured. Update its signature and body:

```go
// Before:
func GetAvailableModelsByAPIKeys(openAIKey, geminiKey, claudeKey, ollamaURL string) []LLMModel {

// After:
func GetAvailableModelsByAPIKeys(openAIKey, geminiKey, claudeKey, ollamaURL, myNewLLMKey string) []LLMModel {
    // ...
    switch model.Provider {
    case OpenAI:
        if openAIKey != "" { available = append(available, model) }
    case Gemini:
        if geminiKey != "" { available = append(available, model) }
    case Claude:
        if claudeKey != "" { available = append(available, model) }
    case Ollama:
        if ollamaURL != "" { available = append(available, model) }
    case MyNewLLM:              // ← ADD
        if myNewLLMKey != "" { available = append(available, model) }
    }
}
```

Update its call-site (typically in an API handler that returns supported models to the frontend).

**Why:** If this function doesn't know about the new provider, the model selector API always
returns zero models for it, and users can't select it even when the key is configured.

---

### Step A.12 — Update `GetFirstAvailableModel` priority list

**File:** `backend/internal/constants/supported_models.go`

```go
func GetFirstAvailableModel() *LLMModel {
    // Priority order: first configured provider wins
    providers := []string{OpenAI, Gemini, Claude, Ollama, MyNewLLM}  // ← ADD (choose priority order)
    for _, provider := range providers {
        if model := GetDefaultModelForProvider(provider); model != nil {
            return model
        }
    }
    return nil
}
```

**Why:** This function is the absolute last-resort fallback when no model has been explicitly
selected and no provider has a default. The order matters — place the new provider where its
desired priority rank is among existing providers.

---

### Step A.13 — Add environment variable to Docker Compose

**Files:** `docker-compose/docker-compose-local.yml`, `docker-compose/docker-compose-server.yml`,
`docker-compose/docker-compose-dokploy.yml`

```yaml
environment:
  # LLM providers — configure at least one
  OPENAI_API_KEY: ${OPENAI_API_KEY:-}
  GEMINI_API_KEY: ${GEMINI_API_KEY:-}
  CLAUDE_API_KEY: ${CLAUDE_API_KEY:-}
  OLLAMA_BASE_URL: ${OLLAMA_BASE_URL:-}
  MYNEWLLM_API_KEY: ${MYNEWLLM_API_KEY:-}   # ← ADD — the :- makes it optional
```

**Why:** Compose files pass environment variables from the host (or `.env` file) into the
container. The `:-` (empty default) makes the variable optional — the container starts normally
even without the key, matching the graceful degradation behaviour in the Go code.

Also add the variable to any `.env.example` file or `SETUP.md` docs so deployers know about it.

---

## Part B — Adding a New Model to an Existing Provider

This is the most common operation and requires changing exactly **one file**.

### Step B.1 — Add the model entry to the provider's model slice

**File:** `backend/internal/constants/<provider>.go`
(e.g. `openai.go`, `gemini.go`, `claude.go`, `ollama.go`)

```go
var OpenAILLMModels = []LLMModel{
    // ... existing models ...
    {
        ID:                  "gpt-5",              // exact API model ID
        Provider:            OpenAI,
        DisplayName:         "GPT-5",              // shown in UI model selector
        IsEnabled:           true,
        Default:             nil,                  // set to boolPtr(true) ONLY if replacing current default
        MaxCompletionTokens: 16384,
        Temperature:         1.0,
        InputTokenLimit:     256000,
        Description:         "Most capable OpenAI model as of 2026",
    },
}
```

**Rules:**
- `IsEnabled: true` — model appears in selector immediately.
- `Default: boolPtr(true)` — **only if replacing the current default.** When you do this, set the
  previous default's `Default` field to `nil`.
- `APIVersion` — required only for Gemini (e.g. `"v1beta"`). Leave empty for other providers.
- `ID` must match exactly what the provider API expects — check the provider's model list docs.

### Step B.2 — No other changes needed

The model is automatically:
- Included in `SupportedLLMModels` via the `append` chain in `supported_models.go`
- Returned by `GetLLMModelsByProvider(provider)`
- Validated by `IsValidModel(modelID)`
- Surfaced to the frontend through the models API
- Selectable per-chat in the UI
- Used as the default for its provider if `Default: boolPtr(true)`

**Why this works:** System prompts are database-type-specific, not model-specific. Moving from
`gpt-4o` to `gpt-5` doesn't require any prompt change — the same prompts work. The provider
client (`openai.go`) already reads `modelID` from the `SetModel` / `GenerateResponse` call, so
the new model ID flows through automatically.

---

## Part C — Testing a New Provider

---

### Step C.1 — Build the backend

```bash
cd backend
go build ./...
```

Must produce **zero errors**. Common failures:
- Missing `case MyNewLLM:` in a switch → add it
- `RegisterClient` factory not updated → Step A.7
- `DisableUnavailableProviders` signature mismatch → Step A.10

---

### Step C.2 — Set environment variables

```bash
export MYNEWLLM_API_KEY="your-real-api-key"
# Then start the backend with the env var set
```

---

### Step C.3 — Verify provider registration in logs

Start the backend. Look for log lines like:

```
Registered LLM client: mynewllm (model: mynewllm-pro-v1)
MyNewLLM models: 2 enabled
```

If you see `Warning: Failed to register MyNewLLM client`, the error is in `NewMyNewLLMClient`
or the API key is wrong.

If the registration block is silently skipped (no log line at all), the env var is empty.

---

### Step C.4 — Verify model appears in the model selector

1. Open the NeoBase UI.
2. Open any chat → click the model selector dropdown.
3. Confirm **MyNewLLM Pro** (or your display name) appears.

If it doesn't appear, check:
- `GetAvailableModelsByAPIKeys` — Step A.11
- `DisableUnavailableProviders` — Step A.10

---

### Step C.5 — Send a basic query

In a chat with any connected database, type:

```
Show me all tables
```

Then switch to your new provider in the model selector and send the same message.

**Expected:** The LLM returns a valid structured JSON response, parsed correctly, and displayed as
a message tile with SQL and an assistant message.

**If you see a raw JSON error or "failed to parse response":** The response schema in
`GetLLMResponseSchema` is wrong or the provider is not returning structured output. Debug by
adding a `log.Printf("raw response: %s", rawJSON)` before the parse step in `GenerateResponse`.

---

### Step C.6 — Test tool calling (database exploration)

Connect to any PostgreSQL or MySQL database and ask:

```
What are the top 10 most recently created users?
```

**Expected:** The LLM calls `get_table_info` (or `execute_read_query`) one or more times, then
calls `generate_final_response` with a valid SQL query.

**If the LLM just returns plain text without calling tools:** `GenerateWithTools` is not wired
correctly. Check that:
- Tools are converted to the provider's native function-call format
- The provider's response is checked for tool calls
- The `generate_final_response` sentinel is detected and returned

If the provider does not support native tool calling, use `TryParseTextToolCall()` from `tools.go`
as a fallback — this parses XML-like tool call blocks from plain text.

---

### Step C.7 — Test non-tech mode

1. Enable **Non-Technical Mode** in chat settings.
2. Ask: `How many orders do we have this month?`

**Expected:** The assistant message contains no SQL jargon (no "query", "GROUP BY", "fetch").

---

### Step C.8 — Test visualization / dashboard

1. Open **Dashboards** → Add a widget.
2. Select the new provider in the model selector.

**Expected:** `GenerateVisualization` is called and returns a valid chart suggestion.

---

### Step C.9 — Test recommendations

1. In any chat, click the recommendations button (the wand / sparkle icon).
2. Confirm recommendations appear.

**Expected:** `GenerateRecommendations` returns 40–60 question suggestions. If it returns
`[]` or an error, debug the recommendations schema and the recommendations prompt.

---

### Step C.10 — Test context cancellation

1. Send a complex query that takes time.
2. Click the **Stop** / cancel button before it completes.

**Expected:** The in-flight API call is cancelled within ~1 second and the UI shows a cancelled
state, not a stuck spinner. If it doesn't cancel, add `ctx` checks to the API call loop in
`GenerateResponse` and `GenerateWithTools`.

---

## Full Checklist — New Provider

Paste into your PR description.

### Backend
- [ ] `constants/llms.go` — new provider constant defined
- [ ] `constants/mynewllm.go` — `LLMModel` slice + response schema(s) created
- [ ] `constants/supported_models.go` — model slice appended to `SupportedLLMModels`
- [ ] `constants/llms.go` — `GetLLMResponseSchema` case added
- [ ] `constants/llms.go` — `GetRecommendationsSchema` case added
- [ ] `pkg/llm/mynewllm.go` — all 7 `Client` interface methods implemented
- [ ] `pkg/llm/manager.go` — factory switch case added
- [ ] `config/env_values.go` — env var struct field added; `LoadEnv()` assignment added
- [ ] `di/modules.go` — startup registration block added with all DB types in `DBConfigs`
- [ ] `constants/supported_models.go` — `DisableUnavailableProviders` signature + body updated
- [ ] `config/env_values.go` — `DisableUnavailableProviders` call-site(s) updated
- [ ] `constants/supported_models.go` — `GetAvailableModelsByAPIKeys` signature + body updated
- [ ] `constants/supported_models.go` — `GetFirstAvailableModel` priority list updated
- [ ] `go build ./...` passes with zero errors

### Infrastructure
- [ ] `docker-compose/docker-compose-local.yml` — env var added
- [ ] `docker-compose/docker-compose-server.yml` — env var added
- [ ] `docker-compose/docker-compose-dokploy.yml` — env var added
- [ ] `SETUP.md` or `.env.example` — env var documented

### Testing
- [ ] Backend builds clean
- [ ] Provider appears in model selector UI
- [ ] Basic query returns a valid structured response
- [ ] Tool-calling loop works (LLM calls `get_table_info` / `execute_read_query`)
- [ ] `generate_final_response` is correctly detected and returned
- [ ] Non-tech mode strips jargon
- [ ] Visualization generates a chart suggestion
- [ ] Recommendations return 40+ suggestions
- [ ] Context cancellation stops the in-flight request

---

## Full Checklist — New Model (existing provider)

- [ ] Entry added to `constants/<provider>.go` model slice
- [ ] If new default: previous default's `Default` field set to `nil`
- [ ] `go build ./...` passes with zero errors
- [ ] Model appears in UI model selector
- [ ] A query sent using the new model returns a valid response


```
HTTP request (user message)
    │
    ▼
LLM Manager  (pkg/llm/manager.go)
    │  GetClient(providerName) → Client interface
    ▼
Provider Client  (pkg/llm/openai.go | gemini.go | claude.go | ollama.go)
    │  GenerateResponse / GenerateVisualization / GenerateWithTools / ...
    ▼
Provider API  (OpenAI, Google, Anthropic, Ollama, ...)
```

Key points:

- **`pkg/llm/Client` interface** — every provider implements this single interface. Adding a
  provider means writing one new file that satisfies it.
- **Response schemas** — each provider has its own structured output schema (JSON Schema for
  OpenAI/Claude/Ollama, `genai.Schema` for Gemini). These live in `constants/<provider>.go`.
- **System prompts** — prompts are *database-type-specific, not provider-specific*. The same
  prompt string is passed to every provider. You do not need to duplicate or modify prompts when
  adding a new provider.
- **Model catalogue** — `constants/supported_models.go` aggregates per-provider model slices.
  Available models are surfaced to the UI through the API.

---

