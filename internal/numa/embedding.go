package numa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// EmbeddingService defines the interface for embedding generation services
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
	GetDimension() int
}

// OpenAIEmbeddingService implements EmbeddingService using OpenAI's API
type OpenAIEmbeddingService struct {
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
	baseURL    string
}

// OpenAIEmbeddingRequest represents the request structure for OpenAI embeddings API
type OpenAIEmbeddingRequest struct {
	Input          interface{} `json:"input"`
	Model          string      `json:"model"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
	Dimensions     int         `json:"dimensions,omitempty"`
}

// OpenAIEmbeddingResponse represents the response structure from OpenAI embeddings API
type OpenAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// NewOpenAIEmbeddingService creates a new OpenAI embedding service
func NewOpenAIEmbeddingService(apiKey, model string) *OpenAIEmbeddingService {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if model == "" {
		model = "text-embedding-3-small" // Default to the latest small model
	}

	dimension := 1536 // Default dimension for text-embedding-3-small
	if model == "text-embedding-3-large" {
		dimension = 3072
	}

	return &OpenAIEmbeddingService{
		apiKey:    apiKey,
		model:     model,
		dimension: dimension,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.openai.com/v1",
	}
}

// GenerateEmbedding generates an embedding for a single text
func (s *OpenAIEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	embeddings, err := s.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embeddings[0], nil
}

// GenerateEmbeddings generates embeddings for multiple texts
func (s *OpenAIEmbeddingService) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	if s.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Clean and prepare texts
	cleanTexts := make([]string, len(texts))
	for i, text := range texts {
		cleanTexts[i] = strings.TrimSpace(text)
		if cleanTexts[i] == "" {
			return nil, fmt.Errorf("text at index %d is empty", i)
		}
	}

	request := OpenAIEmbeddingRequest{
		Input:          cleanTexts,
		Model:          s.model,
		EncodingFormat: "float",
	}

	// Set dimensions for models that support it
	if strings.HasPrefix(s.model, "text-embedding-3") {
		request.Dimensions = s.dimension
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/embeddings", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response OpenAIEmbeddingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(response.Data))
	}

	embeddings := make([][]float32, len(response.Data))
	for i, data := range response.Data {
		if len(data.Embedding) != s.dimension {
			return nil, fmt.Errorf("expected embedding dimension %d, got %d", s.dimension, len(data.Embedding))
		}
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// GetDimension returns the dimension of embeddings produced by this service
func (s *OpenAIEmbeddingService) GetDimension() int {
	return s.dimension
}

// LocalEmbeddingService is a mock implementation for testing/development
type LocalEmbeddingService struct {
	dimension int
}

// NewLocalEmbeddingService creates a new local embedding service for testing
func NewLocalEmbeddingService(dimension int) *LocalEmbeddingService {
	if dimension <= 0 {
		dimension = 384 // Default to all-MiniLM-L6-v2 dimension
	}
	return &LocalEmbeddingService{
		dimension: dimension,
	}
}

// GenerateEmbedding generates a mock embedding for a single text
func (s *LocalEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Generate a deterministic but pseudo-random embedding based on text content
	embedding := make([]float32, s.dimension)
	hash := simpleHash(text)

	for i := 0; i < s.dimension; i++ {
		// Create pseudo-random values between -1 and 1
		hash = (hash*1103515245 + 12345) & 0x7fffffff
		embedding[i] = float32(hash%2000-1000) / 1000.0
	}

	// Normalize the vector
	var norm float32
	for _, val := range embedding {
		norm += val * val
	}
	norm = float32(1.0 / (float64(norm) + 1e-8)) // Add small epsilon to avoid division by zero

	for i := range embedding {
		embedding[i] *= norm
	}

	return embedding, nil
}

// GenerateEmbeddings generates mock embeddings for multiple texts
func (s *LocalEmbeddingService) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := s.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// GetDimension returns the dimension of embeddings produced by this service
func (s *LocalEmbeddingService) GetDimension() int {
	return s.dimension
}

// Helper function to generate a simple hash from text
func simpleHash(text string) uint32 {
	var hash uint32 = 5381
	for _, char := range text {
		hash = ((hash << 5) + hash) + uint32(char)
	}
	return hash
}

// EmbeddingServiceConfig represents configuration for embedding services
type EmbeddingServiceConfig struct {
	Provider  string `json:"provider" yaml:"provider"`   // "openai" or "local"
	APIKey    string `json:"api_key" yaml:"api_key"`     // For OpenAI
	Model     string `json:"model" yaml:"model"`         // Model name
	Dimension int    `json:"dimension" yaml:"dimension"` // Embedding dimension
	BaseURL   string `json:"base_url" yaml:"base_url"`   // Custom API base URL
}

// SentenceTransformersEmbeddingService uses Python sentence-transformers for real embeddings
type SentenceTransformersEmbeddingService struct {
	dimension  int
	modelName  string
	pythonPath string
	scriptPath string
}

// NewSentenceTransformersEmbeddingService creates a real embedding service using sentence-transformers
func NewSentenceTransformersEmbeddingService(pythonPath, scriptPath string) *SentenceTransformersEmbeddingService {
	return &SentenceTransformersEmbeddingService{
		dimension:  384, // all-MiniLM-L6-v2 dimension
		modelName:  "all-MiniLM-L6-v2",
		pythonPath: pythonPath,
		scriptPath: scriptPath,
	}
}

// GenerateEmbedding generates a real embedding using sentence-transformers
func (s *SentenceTransformersEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	embeddings, err := s.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embeddings[0], nil
}

// GenerateEmbeddings generates real embeddings using sentence-transformers Python service
func (s *SentenceTransformersEmbeddingService) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Prepare request data
	requestData := map[string]interface{}{
		"texts": texts,
		"model": s.modelName,
	}

	requestJSON, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call Python embedding service
	cmd := exec.CommandContext(ctx, s.pythonPath, s.scriptPath, "embed")
	cmd.Stdin = strings.NewReader(string(requestJSON))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run embedding service: %w", err)
	}

	// Parse response
	var response struct {
		Embeddings [][]float32 `json:"embeddings"`
		Error      string      `json:"error,omitempty"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("embedding service error: %s", response.Error)
	}

	if len(response.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(response.Embeddings))
	}

	// Validate embedding dimensions
	for i, embedding := range response.Embeddings {
		if len(embedding) != s.dimension {
			return nil, fmt.Errorf("expected embedding dimension %d, got %d for text %d", s.dimension, len(embedding), i)
		}
	}

	return response.Embeddings, nil
}

// GetDimension returns the dimension of embeddings produced by this service
func (s *SentenceTransformersEmbeddingService) GetDimension() int {
	return s.dimension
}

// NewEmbeddingService creates an embedding service based on configuration
func NewEmbeddingService(config EmbeddingServiceConfig) (EmbeddingService, error) {
	switch strings.ToLower(config.Provider) {
	case "openai":
		service := NewOpenAIEmbeddingService(config.APIKey, config.Model)
		if config.BaseURL != "" {
			service.baseURL = config.BaseURL
		}
		if config.Dimension > 0 {
			service.dimension = config.Dimension
		}
		return service, nil
	case "sentence-transformers", "local":
		// Use real sentence-transformers model for production
		pythonPath := ".venv/bin/python"
		if envPath := os.Getenv("EION_PYTHON_PATH"); envPath != "" {
			pythonPath = envPath
		}
		scriptPath := "internal/numa/python/embedding_service.py"
		return NewSentenceTransformersEmbeddingService(pythonPath, scriptPath), nil
	case "mock":
		// Only use mock for testing
		return NewLocalEmbeddingService(config.Dimension), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", config.Provider)
	}
}
