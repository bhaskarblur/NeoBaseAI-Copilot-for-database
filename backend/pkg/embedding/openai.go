package embedding

import (
	"context"
	"fmt"
	"log"
	"neobase-ai/internal/constants"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements the Provider interface using OpenAI's embedding API.
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI embedding provider.
func NewOpenAIProvider(apiKey string, model string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required for embedding provider")
	}

	if model == "" {
		model = DefaultOpenAIModel
	}

	client := openai.NewClient(apiKey)
	log.Printf("Embedding -> OpenAI provider initialized with model: %s", model)

	return &OpenAIProvider{
		client: client,
		model:  model,
	}, nil
}

// Embed generates a vector embedding for a single text input.
func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input:      []string{text},
		Model:      openai.EmbeddingModel(p.model),
		Dimensions: p.GetDimension(),
	})
	if err != nil {
		return nil, fmt.Errorf("OpenAI embedding error: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned from OpenAI")
	}

	return resp.Data[0].Embedding, nil
}

// EmbedBatch generates vector embeddings for multiple texts in a single API call.
func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	// OpenAI supports up to 2048 inputs per batch request
	const maxBatchSize = 2048
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
			Input:      batch,
			Model:      openai.EmbeddingModel(p.model),
			Dimensions: p.GetDimension(),
		})
		if err != nil {
			return nil, fmt.Errorf("OpenAI batch embedding error: %w", err)
		}

		for _, d := range resp.Data {
			allEmbeddings = append(allEmbeddings, d.Embedding)
		}
	}

	return allEmbeddings, nil
}

// EmbedQuery generates a vector embedding optimized for search queries.
// OpenAI doesn't distinguish between document and query embeddings, so this is identical to Embed.
func (p *OpenAIProvider) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return p.Embed(ctx, text)
}

// GetDimension returns the dimensionality of embeddings produced by this provider.
// Looks up the dimension from the centralized embedding model definitions in constants.
func (p *OpenAIProvider) GetDimension() int {
	if model := constants.GetEmbeddingModel(p.model); model != nil {
		return model.Dimension
	}
	return OpenAIDimension // fallback
}

// GetProviderName returns "openai".
func (p *OpenAIProvider) GetProviderName() string {
	return "openai"
}

// GetModelName returns the embedding model name.
func (p *OpenAIProvider) GetModelName() string {
	return p.model
}
