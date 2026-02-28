package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"neobase-ai/internal/constants"
	"net/http"
)

// GeminiProvider implements the Provider interface using Google Gemini's embedding REST API.
// We use the REST API directly instead of the genai SDK because the SDK is hardcoded to v1beta,
// and different embedding models require different API versions.
type GeminiProvider struct {
	apiKey  string
	model   string
	baseURL string
}

// geminiModelAPIVersion maps embedding models to the API version where they are available.
// Newer models (gemini-embedding-001) are on v1beta; older models (text-embedding-004) are on v1.
var geminiModelAPIVersion = map[string]string{
	"text-embedding-004": "v1",
	"embedding-001":      "v1",
	// gemini-embedding-001 and any future models default to v1beta
}

// getGeminiBaseURL returns the correct API base URL for the given model.
func getGeminiBaseURL(model string) string {
	if version, ok := geminiModelAPIVersion[model]; ok {
		return "https://generativelanguage.googleapis.com/" + version
	}
	// Default to v1beta for newer models
	return "https://generativelanguage.googleapis.com/v1beta"
}

// --- REST API request/response types ---

type geminiEmbedRequest struct {
	Model                string             `json:"model"`
	Content              geminiEmbedContent `json:"content"`
	OutputDimensionality int                `json:"outputDimensionality,omitempty"`
}

type geminiEmbedContent struct {
	Parts []geminiEmbedPart `json:"parts"`
}

type geminiEmbedPart struct {
	Text string `json:"text"`
}

type geminiBatchEmbedRequest struct {
	Requests []geminiEmbedRequest `json:"requests"`
}

type geminiEmbedResponse struct {
	Embedding *geminiEmbedding `json:"embedding"`
}

type geminiBatchEmbedResponse struct {
	Embeddings []geminiEmbedding `json:"embeddings"`
}

type geminiEmbedding struct {
	Values []float32 `json:"values"`
}

type geminiAPIError struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// NewGeminiProvider creates a new Gemini embedding provider using the REST API.
func NewGeminiProvider(apiKey string, model string) (*GeminiProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key is required for embedding provider")
	}

	if model == "" {
		model = DefaultGeminiModel
	}

	baseURL := getGeminiBaseURL(model)
	log.Printf("Embedding -> Gemini provider initialized with model: %s (using REST API at %s)", model, baseURL)

	return &GeminiProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
	}, nil
}

// Embed generates a vector embedding for a single text input.
func (p *GeminiProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	reqBody := geminiEmbedRequest{
		Model: fmt.Sprintf("models/%s", p.model),
		Content: geminiEmbedContent{
			Parts: []geminiEmbedPart{{Text: text}},
		},
		OutputDimensionality: p.GetDimension(),
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Gemini embed request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:embedContent?key=%s", p.baseURL, p.model, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini embed request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Gemini embed response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr geminiAPIError
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("Gemini embedding error (HTTP %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("Gemini embedding error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result geminiEmbedResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini embed response: %w", err)
	}

	if result.Embedding == nil || len(result.Embedding.Values) == 0 {
		return nil, fmt.Errorf("no embedding returned from Gemini")
	}

	return result.Embedding.Values, nil
}

// EmbedBatch generates vector embeddings for multiple texts in a single API call.
func (p *GeminiProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	// Gemini batchEmbedContents supports up to 100 texts per call
	const maxBatchSize = 100
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		var requests []geminiEmbedRequest
		for _, text := range batch {
			requests = append(requests, geminiEmbedRequest{
				Model: fmt.Sprintf("models/%s", p.model),
				Content: geminiEmbedContent{
					Parts: []geminiEmbedPart{{Text: text}},
				},
				OutputDimensionality: p.GetDimension(),
			})
		}

		reqBody := geminiBatchEmbedRequest{Requests: requests}
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Gemini batch embed request: %w", err)
		}

		url := fmt.Sprintf("%s/models/%s:batchEmbedContents?key=%s", p.baseURL, p.model, p.apiKey)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini batch embed request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Gemini batch embed request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Gemini batch embed response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			var apiErr geminiAPIError
			if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
				return nil, fmt.Errorf("Gemini batch embedding error (HTTP %d): %s", resp.StatusCode, apiErr.Error.Message)
			}
			return nil, fmt.Errorf("Gemini batch embedding error (HTTP %d): %s", resp.StatusCode, string(body))
		}

		var result geminiBatchEmbedResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("failed to parse Gemini batch embed response: %w", err)
		}

		for _, emb := range result.Embeddings {
			allEmbeddings = append(allEmbeddings, emb.Values)
		}
	}

	return allEmbeddings, nil
}

// GetDimension returns the dimensionality of embeddings produced by this provider.
// Looks up the dimension from the centralized embedding model definitions in constants.
func (p *GeminiProvider) GetDimension() int {
	if model := constants.GetEmbeddingModel(p.model); model != nil {
		return model.Dimension
	}
	return GeminiDimension // fallback
}

// GetProviderName returns "gemini".
func (p *GeminiProvider) GetProviderName() string {
	return "gemini"
}

// GetModelName returns the embedding model name.
func (p *GeminiProvider) GetModelName() string {
	return p.model
}
