package numa

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/eion/eion/internal/config"
)

// Fact represents a knowledge fact in Numa - matches Knowledge's Fact struct
type Fact struct {
	UUID      uuid.UUID  `json:"uuid"`
	Name      string     `json:"name"`
	Fact      string     `json:"fact"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiredAt *time.Time `json:"expired_at"`
	ValidAt   *time.Time `json:"valid_at"`
	InvalidAt *time.Time `json:"invalid_at"`
}

func (f Fact) ExtractCreatedAt() time.Time {
	if f.ValidAt != nil {
		return *f.ValidAt
	}
	return f.CreatedAt
}

// Message represents a message for Numa processing
type Message struct {
	UUID     string `json:"uuid"`
	Role     string `json:"role"`
	RoleType string `json:"role_type,omitempty"`
	Content  string `json:"content"`
}

// GetMemoryRequest matches Knowledge's GetMemoryRequest
type GetMemoryRequest struct {
	GroupID        string    `json:"group_id"`
	MaxFacts       int       `json:"max_facts"`
	CenterNodeUUID string    `json:"center_node_uuid"`
	Messages       []Message `json:"messages"`
}

// GetMemoryResponse matches Knowledge's GetMemoryResponse
type GetMemoryResponse struct {
	Facts []Fact `json:"facts"`
}

// PutMemoryRequest matches Knowledge's PutMemoryRequest
type PutMemoryRequest struct {
	GroupId  string    `json:"group_id"`
	Messages []Message `json:"messages"`
}

// SearchRequest matches Knowledge's SearchRequest
type SearchRequest struct {
	GroupIDs []string `json:"group_ids"`
	Text     string   `json:"query"`
	MaxFacts int      `json:"max_facts,omitempty"`
}

// SearchResponse matches Knowledge's SearchResponse
type SearchResponse struct {
	Facts []Fact `json:"facts"`
}

// AddNodeRequest matches Knowledge's AddNodeRequest
type AddNodeRequest struct {
	GroupID string `json:"group_id"`
	UUID    string `json:"uuid"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// EionServiceInterface - Interface to avoid import cycle
type EionServiceInterface interface {
	GetNumaMemory(ctx context.Context, payload *GetMemoryRequest) (*GetMemoryResponse, error)
	PutNumaMemory(ctx context.Context, groupID string, messages []Message, addGroupIDPrefix bool) error
	SearchNumaMemory(ctx context.Context, payload *SearchRequest) (*SearchResponse, error)
	AddNumaNode(ctx context.Context, payload *AddNodeRequest) error
	GetNumaFact(ctx context.Context, factUUID uuid.UUID) (*Fact, error)
	DeleteNumaFact(ctx context.Context, factUUID uuid.UUID) error
	DeleteNumaGroup(ctx context.Context, groupID string) error
	DeleteNumaMessage(ctx context.Context, messageUUID uuid.UUID) error
	IsExtractionEnabled() bool
}

// Service interface matches Knowledge's Service interface exactly
type Service interface {
	GetMemory(ctx context.Context, payload GetMemoryRequest) (*GetMemoryResponse, error)
	PutMemory(ctx context.Context, groupID string, messages []Message, addGroupIDPrefix bool) error
	Search(ctx context.Context, payload SearchRequest) (*SearchResponse, error)
	AddNode(ctx context.Context, payload AddNodeRequest) error
	GetFact(ctx context.Context, factUUID uuid.UUID) (*Fact, error)
	DeleteFact(ctx context.Context, factUUID uuid.UUID) error
	DeleteGroup(ctx context.Context, groupID string) error
	DeleteMessage(ctx context.Context, messageUUID uuid.UUID) error
	IsEnabled() bool // New method to check if Numa is enabled
}

var _instance Service

// I returns the singleton Numa service instance - matches Knowledge's I() function
func I() Service {
	return _instance
}

// NoOpService implements Service interface but does nothing (when no API key)
type NoOpService struct{}

func (n *NoOpService) GetMemory(ctx context.Context, payload GetMemoryRequest) (*GetMemoryResponse, error) {
	return &GetMemoryResponse{Facts: []Fact{}}, nil
}

func (n *NoOpService) PutMemory(ctx context.Context, groupID string, messages []Message, addGroupIDPrefix bool) error {
	return nil // No-op
}

func (n *NoOpService) Search(ctx context.Context, payload SearchRequest) (*SearchResponse, error) {
	return &SearchResponse{Facts: []Fact{}}, nil
}

func (n *NoOpService) AddNode(ctx context.Context, payload AddNodeRequest) error {
	return nil // No-op
}

func (n *NoOpService) GetFact(ctx context.Context, factUUID uuid.UUID) (*Fact, error) {
	return nil, fmt.Errorf("numa+ enhancement not available (OpenAI API key required)")
}

func (n *NoOpService) DeleteFact(ctx context.Context, factUUID uuid.UUID) error {
	return nil // No-op
}

func (n *NoOpService) DeleteGroup(ctx context.Context, groupID string) error {
	return nil // No-op
}

func (n *NoOpService) DeleteMessage(ctx context.Context, messageUUID uuid.UUID) error {
	return nil // No-op
}

func (n *NoOpService) IsEnabled() bool {
	return false
}

// NumaPlusService - Enhanced Numa with OpenAI integration
// Adds cloud LLM capability on top of local Numa processing
type NumaPlusService struct {
	logger        *zap.Logger
	openaiAPIKey  string
	pythonService *PythonExtractionService
	eionService   EionServiceInterface // Interface to avoid import cycle
}

func (s *NumaPlusService) IsEnabled() bool {
	return true
}

// Setup initializes the Numa service - New complementary architecture
func Setup(logger *zap.Logger, eionService EionServiceInterface) {
	if _instance != nil {
		return
	}

	numaConfig := config.Numa()

	// Check if OpenAI API key is configured for Numa+ enhancement
	if numaConfig.OpenAIAPIKey == "" {
		logger.Info("No OpenAI API key configured - Numa+ enhancement disabled, using local Numa")
		_instance = &NoOpService{}
		return
	}

	logger.Info("OpenAI API key configured - Numa+ enhancement enabled",
		zap.String("openai_model", "gpt-4"))

	// Initialize Python extraction service with LLM capability
	pythonService, err := NewPythonExtractionService(logger)
	if err != nil {
		logger.Warn("Failed to initialize Python extraction service - using NoOp service", zap.Error(err))
		_instance = &NoOpService{}
		return
	}

	_instance = &NumaPlusService{
		logger:        logger,
		openaiAPIKey:  numaConfig.OpenAIAPIKey,
		pythonService: pythonService,
		eionService:   eionService,
	}
}

// Service implementation - Enhanced layer with LLM + fallback to Eion

func (s *NumaPlusService) GetMemory(ctx context.Context, payload GetMemoryRequest) (*GetMemoryResponse, error) {
	// Delegate to Eion for memory retrieval
	return s.eionService.GetNumaMemory(ctx, &payload)
}

func (s *NumaPlusService) PutMemory(ctx context.Context, groupID string, messages []Message, addGroupIDPrefix bool) error {
	s.logger.Debug("PutMemory called with Numa+ LLM enhancement",
		zap.String("group_id", groupID),
		zap.Int("message_count", len(messages)))

	// Try LLM extraction first
	if s.pythonService != nil {
		request := ExtractionRequest{
			GroupID:          groupID,
			Messages:         messages,
			PreviousEpisodes: []EpisodeData{}, // TODO: Retrieve from storage
			EntityTypes:      []EntityType{},  // TODO: Configure entity types
			UseNuma:          false,           // Use LLM in Numa+
		}

		response, err := s.pythonService.ExtractKnowledge(ctx, request)
		if err == nil && response.Success {
			s.logger.Info("Successfully extracted knowledge with Numa+ LLM",
				zap.String("group_id", groupID),
				zap.Int("extracted_nodes", len(response.ExtractedNodes)),
				zap.Int("extracted_edges", len(response.ExtractedEdges)))

			// TODO: Store results in Neo4j via Eion
			return nil
		}

		s.logger.Warn("Numa+ LLM extraction failed, falling back to local Numa", zap.Error(err))
	}

	// Fallback to local Numa
	return s.eionService.PutNumaMemory(ctx, groupID, messages, addGroupIDPrefix)
}

func (s *NumaPlusService) Search(ctx context.Context, payload SearchRequest) (*SearchResponse, error) {
	// Delegate to Eion for search functionality
	return s.eionService.SearchNumaMemory(ctx, &payload)
}

func (s *NumaPlusService) AddNode(ctx context.Context, payload AddNodeRequest) error {
	// Delegate to Eion
	return s.eionService.AddNumaNode(ctx, &payload)
}

func (s *NumaPlusService) GetFact(ctx context.Context, factUUID uuid.UUID) (*Fact, error) {
	// Delegate to Eion
	return s.eionService.GetNumaFact(ctx, factUUID)
}

func (s *NumaPlusService) DeleteFact(ctx context.Context, factUUID uuid.UUID) error {
	// Delegate to Eion
	return s.eionService.DeleteNumaFact(ctx, factUUID)
}

func (s *NumaPlusService) DeleteGroup(ctx context.Context, groupID string) error {
	// Delegate to Eion
	return s.eionService.DeleteNumaGroup(ctx, groupID)
}

func (s *NumaPlusService) DeleteMessage(ctx context.Context, messageUUID uuid.UUID) error {
	// Delegate to Eion
	return s.eionService.DeleteNumaMessage(ctx, messageUUID)
}
