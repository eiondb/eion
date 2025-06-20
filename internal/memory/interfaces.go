package memory

import (
	"context"

	"github.com/google/uuid"
)

// MemoryStore defines the memory store interface
type MemoryStore interface {
	// Session operations
	GetSession(ctx context.Context, sessionID, agentID string) (*Session, error)
	CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error)
	UpdateSession(ctx context.Context, req *UpdateSessionRequest) (*Session, error)
	DeleteSession(ctx context.Context, sessionID, agentID string) error
	ListSessions(ctx context.Context, agentID string, cursor int64, limit int) ([]*Session, error)

	// Memory operations
	GetMemory(ctx context.Context, sessionID, agentID string, lastNMessages int, opts ...FilterOption) (*Memory, error)
	PutMemory(ctx context.Context, sessionID, agentID string, memory *Memory, skipProcessing bool) error
	SearchMemory(ctx context.Context, query *MemorySearchQuery) (*MemorySearchResult, error)

	// Message operations
	GetMessages(ctx context.Context, sessionID, agentID string, lastNMessages int, beforeUUID uuid.UUID) ([]Message, error)
	GetMessageList(ctx context.Context, sessionID, agentID string, pageNumber, pageSize int) (*MessageListResponse, error)
	GetMessagesByUUID(ctx context.Context, sessionID, agentID string, uuids []uuid.UUID) ([]Message, error)
	PutMessages(ctx context.Context, sessionID, agentID string, messages []Message) ([]Message, error)
	UpdateMessages(ctx context.Context, sessionID, agentID string, messages []Message, includeContent bool) error
	DeleteMessages(ctx context.Context, sessionID, agentID string, messageUUIDs []uuid.UUID) error

	// Fact operations
	GetFacts(ctx context.Context, agentID string, limit int, opts ...FilterOption) ([]Fact, error)
	PutFact(ctx context.Context, fact *Fact) error
	UpdateFact(ctx context.Context, fact *Fact) error
	DeleteFact(ctx context.Context, factUUID uuid.UUID) error

	// System operations
	PurgeDeleted(ctx context.Context, agentID string) error
}

// EmbeddingService defines the embedding interface
type EmbeddingService interface {
	// Process messages for embeddings
	ProcessMessages(ctx context.Context, messages []Message, agentID string) error
}

// VectorStore defines the interface for vector similarity operations
type VectorStore interface {
	// Store a vector with associated metadata
	StoreVector(ctx context.Context, id string, vector []float32, metadata map[string]any) error
	// Store multiple vectors
	StoreVectors(ctx context.Context, vectors []VectorData) error
	// Search for similar vectors
	SearchSimilar(ctx context.Context, query []float32, limit int, threshold float64, filters map[string]any) ([]VectorSearchResult, error)
	// Delete vectors by ID
	DeleteVectors(ctx context.Context, ids []string) error
	// Update vector metadata
	UpdateVectorMetadata(ctx context.Context, id string, metadata map[string]any) error
}

// VectorData represents a vector with its metadata
type VectorData struct {
	ID       string         `json:"id"`
	Vector   []float32      `json:"vector"`
	Metadata map[string]any `json:"metadata"`
}

// VectorSearchResult represents a vector search result
type VectorSearchResult struct {
	ID       string         `json:"id"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata"`
}

// SearchService defines the search service interface
type SearchService interface {
	// Search for messages
	SearchMessages(ctx context.Context, query string, agentID string, limit int, threshold float64) ([]Message, error)
	// Search for facts
	SearchFacts(ctx context.Context, query string, agentID string, limit int, threshold float64) ([]Fact, error)
}

// ExtractionService defines the fact extraction interface
type ExtractionService interface {
	// Extract facts from messages
	ExtractFacts(ctx context.Context, messages []Message, agentID string) ([]Fact, error)
	// Update facts based on new messages
	UpdateFacts(ctx context.Context, messages []Message, existingFacts []Fact, agentID string) ([]Fact, error)
}

// TokenCounter defines the interface for counting tokens in text
type TokenCounter interface {
	// Count tokens in text
	CountTokens(text string) int
	// Count tokens in multiple texts
	CountTokensBatch(texts []string) []int
}

// MemoryProcessor defines the interface for processing memory operations
type MemoryProcessor interface {
	// Process new messages (extract facts, generate embeddings, etc.)
	ProcessMessages(ctx context.Context, messages []Message, spaceID string, agentID string) error
	// Process memory updates
	ProcessMemoryUpdate(ctx context.Context, memory *Memory) error
}

// MemoryService defines the memory service interface
type MemoryService interface {
	// Service lifecycle
	Initialize(ctx context.Context) error
	StartupHealthCheck(ctx context.Context) error
	Close(ctx context.Context) error
	HealthCheck(ctx context.Context) error

	// Memory operations
	GetMemory(ctx context.Context, sessionID, agentID string, lastNMessages int, opts ...FilterOption) (*Memory, error)
	PutMemory(ctx context.Context, sessionID, agentID string, memory *Memory, skipProcessing bool) error
	SearchMemory(ctx context.Context, query *MemorySearchQuery) (*MemorySearchResult, error)

	// Message operations
	GetMessages(ctx context.Context, sessionID, agentID string, lastNMessages int, beforeUUID uuid.UUID) ([]Message, error)
	GetMessageList(ctx context.Context, sessionID, agentID string, pageNumber, pageSize int) (*MessageListResponse, error)
	GetMessagesByUUID(ctx context.Context, sessionID, agentID string, uuids []uuid.UUID) ([]Message, error)
	PutMessages(ctx context.Context, sessionID, agentID string, messages []Message) ([]Message, error)
	UpdateMessages(ctx context.Context, sessionID, agentID string, messages []Message, includeContent bool) error
	DeleteMessages(ctx context.Context, sessionID, agentID string, messageUUIDs []uuid.UUID) error

	// Fact operations
	GetFacts(ctx context.Context, agentID string, limit int, opts ...FilterOption) ([]Fact, error)
	PutFact(ctx context.Context, fact *Fact) error
	UpdateFact(ctx context.Context, fact *Fact) error
	DeleteFact(ctx context.Context, factUUID uuid.UUID) error

	// System operations
	PurgeDeleted(ctx context.Context, agentID string) error
}

// HealthChecker defines the interface for health checking components
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
	IsCritical() bool // Critical services block startup if unhealthy
	Name() string
}

// HealthManager manages health checks for all system components
type HealthManager interface {
	AddChecker(checker HealthChecker)
	StartupHealthCheck(ctx context.Context) error
	RuntimeHealthCheck(ctx context.Context) map[string]error
}
