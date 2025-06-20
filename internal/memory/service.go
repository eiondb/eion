package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/eion/eion/internal/graph"
	"github.com/eion/eion/internal/numa"
	"go.uber.org/zap"
)

// EionMemoryService is the main service that orchestrates all memory operations
// Now includes full Numa extraction capabilities (Numa-only) with Neo4j
type EionMemoryService struct {
	store            *EionMemoryStore
	embeddingService numa.EmbeddingService
	vectorStore      VectorStore
	tokenCounter     TokenCounter
	db               *bun.DB
	config           *MemoryServiceConfig
	healthManager    HealthManager
	// Neo4j knowledge graph client - MANDATORY for Eion
	neo4jClient *graph.Neo4jClient
	// Numa extraction service (Numa-only) - migrated from Numa
	pythonService *numa.PythonExtractionService
	// Temporal operations for conflict detection and resolution
	temporalOps *numa.TemporalOperations
	logger      *zap.Logger
}

// MemoryServiceConfig represents configuration for the memory service
// Now includes mandatory Neo4j configuration
type MemoryServiceConfig struct {
	DatabaseURL      string                      `json:"database_url" yaml:"database_url"`
	EmbeddingConfig  numa.EmbeddingServiceConfig `json:"embedding" yaml:"embedding"`
	VectorStoreType  string                      `json:"vector_store_type" yaml:"vector_store_type"` // MUST be "neo4j"
	VectorStoreURL   string                      `json:"vector_store_url" yaml:"vector_store_url"`
	TokenCounterType string                      `json:"token_counter_type" yaml:"token_counter_type"` // "tiktoken" or "simple"
	MaxConnections   int                         `json:"max_connections" yaml:"max_connections"`
	EnableMigrations bool                        `json:"enable_migrations" yaml:"enable_migrations"`
	// Neo4j configuration - MANDATORY for Eion
	Neo4jConfig graph.Neo4jConfig `json:"neo4j" yaml:"neo4j"`
	// Numa extraction configuration - no OpenAI API key (Numa-only)
	EnableExtraction bool `json:"enable_extraction" yaml:"enable_extraction"`
}

// NewEionMemoryService creates a new memory service with all components
// Neo4j is MANDATORY - no fallbacks allowed
func NewEionMemoryService(config *MemoryServiceConfig, logger *zap.Logger) (*EionMemoryService, error) {
	if config == nil {
		config = DefaultMemoryServiceConfig()
	}

	// Initialize database connection
	db, err := initializeDatabase(config.DatabaseURL, config.MaxConnections)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize embedding service - MANDATORY
	embeddingService, err := numa.NewEmbeddingService(config.EmbeddingConfig)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize embedding service: %w", err)
	}

	// Initialize Neo4j client - MANDATORY (no fallbacks)
	neo4jClient, err := graph.NewNeo4jClient(config.Neo4jConfig, logger)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize Neo4j client (MANDATORY): %w", err)
	}

	// Initialize vector store - MUST be Neo4j
	if config.VectorStoreType != "neo4j" {
		db.Close()
		neo4jClient.Close(context.Background())
		return nil, fmt.Errorf("vector store type must be 'neo4j', got: %s", config.VectorStoreType)
	}

	// Initialize token counter
	tokenCounter := initializeTokenCounter(config.TokenCounterType)

	// Initialize memory store
	store := NewEionMemoryStore(db, embeddingService, nil, nil, tokenCounter)

	// Initialize Python extraction service for Numa
	var pythonService *numa.PythonExtractionService
	if config.EnableExtraction {
		pythonService, err = numa.NewPythonExtractionService(logger)
		if err != nil {
			logger.Warn("Failed to initialize Python extraction service", zap.Error(err))
			// Continue without extraction service - will fail on first extraction attempt
		}
	}

	// Initialize temporal operations
	temporalOps := numa.NewTemporalOperations(logger, neo4jClient.GetDriver())

	// Initialize health manager
	healthManager := NewEionHealthManager(logger)
	healthManager.AddChecker(NewDatabaseHealthChecker(db))
	healthManager.AddChecker(NewEmbeddingHealthChecker(embeddingService))
	// TODO: Add Neo4j health checker when implemented

	service := &EionMemoryService{
		store:            store,
		embeddingService: embeddingService,
		vectorStore:      nil, // Not used - Neo4j handles vectors
		tokenCounter:     tokenCounter,
		db:               db,
		config:           config,
		healthManager:    healthManager,
		neo4jClient:      neo4jClient,
		pythonService:    pythonService,
		temporalOps:      temporalOps,
		logger:           logger,
	}

	// Run migrations if enabled
	if config.EnableMigrations {
		err = service.runMigrations(context.Background())
		if err != nil {
			ctx := context.Background()
			service.Close(ctx)
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	logger.Info("Eion Memory Service initialized successfully with Neo4j knowledge graph")
	return service, nil
}

// Initialize initializes the memory service (already done in constructor)
func (s *EionMemoryService) Initialize(ctx context.Context) error {
	// Service is already initialized in NewEionMemoryService
	return nil
}

// StartupHealthCheck performs critical health checks for startup
func (s *EionMemoryService) StartupHealthCheck(ctx context.Context) error {
	return s.HealthCheck(ctx)
}

// Close closes the memory service and cleans up resources
func (s *EionMemoryService) Close(ctx context.Context) error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// HealthCheck performs a health check on all components
func (s *EionMemoryService) HealthCheck(ctx context.Context) error {
	// Use health manager for comprehensive health checking
	results := s.healthManager.RuntimeHealthCheck(ctx)

	var errors []string
	for service, err := range results {
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", service, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("health check failures: %v", errors)
	}

	return nil
}

// Delegate all MemoryStore methods to the internal store

func (s *EionMemoryService) GetMemory(ctx context.Context, sessionID, agentID string, lastNMessages int, opts ...FilterOption) (*Memory, error) {
	return s.store.GetMemory(ctx, sessionID, agentID, "", lastNMessages, opts...)
}

func (s *EionMemoryService) PutMemory(ctx context.Context, sessionID, agentID string, memory *Memory, skipProcessing bool) error {
	// CRITICAL: Knowledge extraction is MANDATORY in Eion - no fallbacks allowed
	if skipProcessing {
		return fmt.Errorf("skipProcessing=true not allowed in Eion - knowledge extraction is mandatory")
	}

	if s.pythonService == nil {
		return fmt.Errorf("Python extraction service not available - knowledge extraction is mandatory in Eion")
	}

	if s.neo4jClient == nil {
		return fmt.Errorf("Neo4j client not available - knowledge graph storage is mandatory in Eion")
	}

	// AUTOMATIC CONFLICT DETECTION: Get current version for conflict detection before any processing
	currentVersion, _, err := s.getCurrentResourceVersion(ctx, sessionID)
	if err != nil {
		s.logger.Warn("Failed to get current version for conflict detection",
			zap.String("session_id", sessionID),
			zap.Error(err))
		// Continue processing but without version conflict detection
		currentVersion = 0
	}

	// AUTOMATIC CONFLICT RESOLUTION: Attempt storage with automatic conflict resolution
	conflict, err := s.putMemoryWithAutoConflictResolution(ctx, sessionID, agentID, memory, currentVersion)
	if err != nil {
		return fmt.Errorf("failed to store memory with automatic conflict resolution: %w", err)
	}

	if conflict != nil {
		// Conflict detected and couldn't be auto-resolved, apply fallback strategies
		s.logger.Info("Conflict detected, applying automatic resolution strategies",
			zap.String("session_id", sessionID),
			zap.String("agent_id", agentID),
			zap.String("conflict_id", conflict.ConflictID))

		resolved, err := s.autoResolveConflictWithStrategies(ctx, sessionID, agentID, memory, *conflict)
		if err != nil {
			return fmt.Errorf("automatic conflict resolution failed: %w", err)
		}

		if !resolved {
			s.logger.Warn("Automatic conflict resolution failed, proceeding with last-writer-wins",
				zap.String("session_id", sessionID),
				zap.String("agent_id", agentID))
			// Proceed with storage using last-writer-wins as final fallback
		}
	}

	// Extract knowledge from messages using Numa (MANDATORY)
	if len(memory.Messages) > 0 {
		// Convert messages to Numa format for extraction
		numaMessages := make([]numa.Message, len(memory.Messages))
		for i, msg := range memory.Messages {
			numaMessages[i] = numa.Message{
				UUID:     msg.UUID.String(),
				Role:     msg.Role,
				RoleType: string(msg.RoleType),
				Content:  msg.Content,
			}
		}

		// Use session ID as group ID for knowledge grouping
		groupID := sessionID

		// Extract knowledge using Knowledge logic with Numa (MANDATORY)
		request := numa.ExtractionRequest{
			GroupID:          groupID,
			Messages:         numaMessages,
			PreviousEpisodes: []numa.EpisodeData{}, // TODO: Retrieve from Neo4j
			EntityTypes:      []numa.EntityType{},  // TODO: Configure entity types
			UseNuma:          true,                 // Force Numa for built-in Knowledge capability
		}

		response, err := s.pythonService.ExtractKnowledge(ctx, request)
		if err != nil {
			return fmt.Errorf("MANDATORY knowledge extraction failed: %w", err)
		}

		if !response.Success {
			return fmt.Errorf("MANDATORY knowledge extraction failed: %s", response.Error)
		}

		// Store extracted nodes and edges in Neo4j knowledge graph with AUTOMATIC temporal conflict resolution
		err = s.storeKnowledgeInNeo4jWithAutoConflictResolution(ctx, response, groupID, agentID)
		if err != nil {
			return fmt.Errorf("failed to store knowledge in Neo4j with automatic conflict resolution: %w", err)
		}

		// Store episodic memory in Neo4j
		err = s.storeEpisodicMemory(ctx, memory.Messages, groupID)
		if err != nil {
			return fmt.Errorf("failed to store episodic memory in Neo4j: %w", err)
		}

		s.logger.Info("Successfully processed memory with knowledge extraction and automatic conflict resolution",
			zap.String("session_id", sessionID),
			zap.String("group_id", groupID),
			zap.Int("extracted_nodes", len(response.ExtractedNodes)),
			zap.Int("extracted_edges", len(response.ExtractedEdges)),
			zap.Int("messages", len(memory.Messages)))
	}

	// Store basic message metadata in PostgreSQL for session management
	err = s.store.PutMemory(ctx, sessionID, agentID, "", memory, false)
	if err != nil {
		return fmt.Errorf("failed to store message metadata: %w", err)
	}

	// AUTOMATIC VERSION UPDATE: Increment version after successful processing
	err = s.incrementResourceVersion(ctx, sessionID, agentID)
	if err != nil {
		s.logger.Warn("Failed to increment resource version after processing",
			zap.String("session_id", sessionID),
			zap.String("agent_id", agentID),
			zap.Error(err))
		// Don't fail the operation for version tracking issues
	}

	return nil
}

// storeKnowledgeInNeo4j stores extracted knowledge with temporal processing
func (s *EionMemoryService) storeKnowledgeInNeo4j(ctx context.Context, response *numa.ExtractionResponse, groupID string) error {
	database := s.config.Neo4jConfig.Database
	if database == "" {
		return fmt.Errorf("database name is required for temporal operations")
	}

	// Store entity nodes first
	for _, node := range response.ExtractedNodes {
		// Generate embedding for the entity
		embedding, err := s.embeddingService.GenerateEmbedding(ctx, node.Name+" "+node.Summary)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for entity %s: %w", node.Name, err)
		}

		entityNode := graph.EntityNode{
			UUID:      node.UUID,
			Name:      node.Name,
			Labels:    []string{"Entity"},
			Summary:   node.Summary,
			GroupID:   groupID,
			CreatedAt: time.Now(),
			Metadata: map[string]any{
				"source": "extraction",
			},
			Embedding: embedding,
		}

		err = s.neo4jClient.StoreEntityNode(ctx, entityNode, database)
		if err != nil {
			return fmt.Errorf("failed to store entity node %s: %w", node.Name, err)
		}
	}

	// Process edges with temporal operations - internal implementation
	for _, extractedEdge := range response.ExtractedEdges {
		if err := s.processEdgeWithTemporalLogic(ctx, extractedEdge, groupID, database); err != nil {
			return fmt.Errorf("failed to process edge %s->%s: %w",
				extractedEdge.SourceUUID, extractedEdge.TargetUUID, err)
		}
	}

	return nil
}

// processEdgeWithTemporalLogic processes a single edge with full temporal operations
// This method now redirects to the enhanced automatic conflict resolution version
func (s *EionMemoryService) processEdgeWithTemporalLogic(
	ctx context.Context,
	extractedEdge numa.EdgeNode,
	groupID, database string,
) error {
	// Redirect to the enhanced automatic conflict resolution method
	// Use "extraction_service" as agentID for backward compatibility
	return s.processEdgeWithAutomaticConflictResolution(ctx, extractedEdge, groupID, "extraction_service", database)
}

// convertFloat64ToFloat32 converts float64 slice to float32 slice
func convertFloat64ToFloat32(input []float64) []float32 {
	output := make([]float32, len(input))
	for i, v := range input {
		output[i] = float32(v)
	}
	return output
}

// storeEpisodicMemory stores episodic memory nodes in Neo4j
func (s *EionMemoryService) storeEpisodicMemory(ctx context.Context, messages []Message, groupID string) error {
	database := s.config.Neo4jConfig.Database
	if database == "" {
		database = "neo4j" // Default database
	}

	for _, msg := range messages {
		// Generate embedding for the message content
		embedding, err := s.embeddingService.GenerateEmbedding(ctx, msg.Content)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for message %s: %w", msg.UUID.String(), err)
		}

		episodicNode := graph.EpisodicNode{
			UUID:      msg.UUID.String(),
			GroupID:   groupID,
			Content:   msg.Content,
			Source:    "message",
			Summary:   "", // TODO: Generate summary if needed
			CreatedAt: msg.CreatedAt,
			ValidAt:   msg.CreatedAt,
			Metadata: map[string]any{
				"session_id": groupID,
			},
			Embedding: embedding,
		}

		err = s.neo4jClient.StoreEpisodicNode(ctx, episodicNode, database)
		if err != nil {
			return fmt.Errorf("failed to store episodic node %s: %w", msg.UUID.String(), err)
		}
	}

	return nil
}

func (s *EionMemoryService) SearchMemory(ctx context.Context, query *MemorySearchQuery) (*MemorySearchResult, error) {
	return s.store.SearchMemory(ctx, query)
}

func (s *EionMemoryService) GetMessages(ctx context.Context, sessionID, agentID string, lastNMessages int, beforeUUID uuid.UUID) ([]Message, error) {
	return s.store.GetMessages(ctx, sessionID, agentID, "", lastNMessages, beforeUUID)
}

func (s *EionMemoryService) GetMessageList(ctx context.Context, sessionID, agentID string, pageNumber, pageSize int) (*MessageListResponse, error) {
	return s.store.GetMessageList(ctx, sessionID, agentID, "", pageNumber, pageSize)
}

func (s *EionMemoryService) GetMessagesByUUID(ctx context.Context, sessionID, agentID string, uuids []uuid.UUID) ([]Message, error) {
	return s.store.GetMessagesByUUID(ctx, sessionID, agentID, "", uuids)
}

func (s *EionMemoryService) PutMessages(ctx context.Context, sessionID, agentID string, messages []Message) ([]Message, error) {
	return s.store.PutMessages(ctx, sessionID, agentID, "", messages)
}

func (s *EionMemoryService) UpdateMessages(ctx context.Context, sessionID, agentID string, messages []Message, includeContent bool) error {
	return s.store.UpdateMessages(ctx, sessionID, agentID, "", messages, includeContent)
}

func (s *EionMemoryService) DeleteMessages(ctx context.Context, sessionID, agentID string, messageUUIDs []uuid.UUID) error {
	return s.store.DeleteMessages(ctx, sessionID, agentID, "", messageUUIDs)
}

func (s *EionMemoryService) GetFacts(ctx context.Context, agentID string, limit int, opts ...FilterOption) ([]Fact, error) {
	return s.store.GetFacts(ctx, "", agentID, limit, opts...)
}

func (s *EionMemoryService) PutFact(ctx context.Context, fact *Fact) error {
	return s.store.PutFact(ctx, fact)
}

func (s *EionMemoryService) UpdateFact(ctx context.Context, fact *Fact) error {
	return s.store.UpdateFact(ctx, fact)
}

func (s *EionMemoryService) DeleteFact(ctx context.Context, factUUID uuid.UUID) error {
	return s.store.DeleteFact(ctx, factUUID, "")
}

func (s *EionMemoryService) PurgeDeleted(ctx context.Context, agentID string) error {
	return s.store.PurgeDeleted(ctx, "")
}

// Numa-style interface methods - migrated from Numa to Eion (Numa-only)

func (s *EionMemoryService) GetNumaMemory(ctx context.Context, payload *numa.GetMemoryRequest) (*numa.GetMemoryResponse, error) {
	// TODO: Implement knowledge graph retrieval from Neo4j
	// For now, return empty facts as placeholder
	return &numa.GetMemoryResponse{
		Facts: []numa.Fact{}, // Placeholder - would query Neo4j knowledge graph
	}, nil
}

func (s *EionMemoryService) PutNumaMemory(ctx context.Context, groupID string, messages []numa.Message, addGroupIDPrefix bool) error {
	if s.pythonService == nil {
		return fmt.Errorf("extraction service not available")
	}

	// Extract knowledge using Python service with Knowledge's exact logic (Numa-only)
	request := numa.ExtractionRequest{
		GroupID:          groupID,
		Messages:         messages,
		PreviousEpisodes: []numa.EpisodeData{}, // TODO: Retrieve from storage
		EntityTypes:      []numa.EntityType{},  // TODO: Configure entity types
		UseNuma:          true,                 // Force Numa in "New" Eion
	}

	response, err := s.pythonService.ExtractKnowledge(ctx, request)
	if err != nil {
		return fmt.Errorf("knowledge extraction failed: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("knowledge extraction failed: %s", response.Error)
	}

	// TODO: Store extracted nodes and edges in Neo4j knowledge graph
	// For now, just log the results
	fmt.Printf("Successfully extracted knowledge with Numa: %d nodes, %d edges\n",
		len(response.ExtractedNodes), len(response.ExtractedEdges))

	return nil
}

func (s *EionMemoryService) SearchNumaMemory(ctx context.Context, payload *numa.SearchRequest) (*numa.SearchResponse, error) {
	// TODO: Implement semantic search:
	// 1. Generate embedding for search text using Numa
	// 2. Query Neo4j for similar facts using vector similarity
	// 3. Apply MMR/RRF algorithms for result ranking

	// For now, return empty results
	return &numa.SearchResponse{
		Facts: []numa.Fact{}, // Placeholder - would perform semantic search
	}, nil
}

func (s *EionMemoryService) AddNumaNode(ctx context.Context, payload *numa.AddNodeRequest) error {
	// TODO: Add entity node to Neo4j knowledge graph
	return nil // Placeholder
}

func (s *EionMemoryService) GetNumaFact(ctx context.Context, factUUID uuid.UUID) (*numa.Fact, error) {
	// TODO: Retrieve specific fact from Neo4j
	return nil, fmt.Errorf("fact not found: %s", factUUID.String()) // Placeholder
}

func (s *EionMemoryService) DeleteNumaFact(ctx context.Context, factUUID uuid.UUID) error {
	// TODO: Delete fact from Neo4j
	return nil // Placeholder
}

func (s *EionMemoryService) DeleteNumaGroup(ctx context.Context, groupID string) error {
	// TODO: Delete all facts and relationships for group from Neo4j
	return nil // Placeholder
}

func (s *EionMemoryService) DeleteNumaMessage(ctx context.Context, messageUUID uuid.UUID) error {
	// TODO: Delete message-related facts from Neo4j
	return nil // Placeholder
}

func (s *EionMemoryService) IsExtractionEnabled() bool {
	return s.pythonService != nil
}

// Private helper methods

func initializeDatabase(databaseURL string, maxConnections int) (*bun.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	if maxConnections <= 0 {
		maxConnections = 10
	}

	// Create the SQL driver
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseURL)))
	sqldb.SetMaxOpenConns(maxConnections)
	sqldb.SetMaxIdleConns(maxConnections / 2)
	sqldb.SetConnMaxLifetime(time.Hour)

	// Create bun database with PostgreSQL dialect - this was the bug!
	db := bun.NewDB(sqldb, pgdialect.New())

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := db.PingContext(ctx)
	if err != nil {
		// Close the database connection on error
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

func initializeVectorStore(storeType, storeURL string) (VectorStore, error) {
	switch storeType {
	case "qdrant":
		// TODO: Implement Qdrant vector store
		return nil, fmt.Errorf("Qdrant vector store not yet implemented")
	case "postgres":
		// Use PostgreSQL with pgvector (handled in database)
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported vector store type: %s", storeType)
	}
}

func initializeTokenCounter(counterType string) TokenCounter {
	switch counterType {
	case "tiktoken":
		// TODO: Implement tiktoken counter
		return NewSimpleTokenCounter()
	case "simple":
		return NewSimpleTokenCounter()
	default:
		return NewSimpleTokenCounter()
	}
}

func (s *EionMemoryService) runMigrations(ctx context.Context) error {
	// Create tables
	models := []interface{}{
		(*UserSchema)(nil),
		(*SessionSchema)(nil),
		(*MessageSchema)(nil),
		(*FactSchema)(nil),
		(*MessageEmbeddingSchema)(nil),
		(*FactEmbeddingSchema)(nil),
	}

	for _, model := range models {
		_, err := s.db.NewCreateTable().
			Model(model).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create table for %T: %w", model, err)
		}
	}

	// Add user_id column to sessions table if it doesn't exist (migration)
	_, err := s.db.ExecContext(ctx, "ALTER TABLE sessions ADD COLUMN IF NOT EXISTS user_id text NOT NULL DEFAULT ''")
	if err != nil {
		fmt.Printf("Warning: failed to add user_id column to sessions: %v\n", err)
	}

	// Add rating column to facts table if it doesn't exist (migration)
	_, err = s.db.ExecContext(ctx, "ALTER TABLE facts ADD COLUMN IF NOT EXISTS rating float8 NOT NULL DEFAULT 0.0")
	if err != nil {
		fmt.Printf("Warning: failed to add rating column to facts: %v\n", err)
	}

	// Create indexes
	allIndexes := append(SessionIndexes, UserIndexes...)
	allIndexes = append(allIndexes, MessageIndexes...)
	allIndexes = append(allIndexes, FactIndexes...)
	allIndexes = append(allIndexes, EmbeddingIndexes...)

	for _, indexSQL := range allIndexes {
		_, err := s.db.ExecContext(ctx, indexSQL)
		if err != nil {
			// Log warning but continue - indexes might already exist
			fmt.Printf("Warning: failed to create index: %v\n", err)
		}
	}

	// Enable pgvector extension if using PostgreSQL
	_, err = s.db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		// Log warning but continue - extension might already exist or not be available
		fmt.Printf("Warning: failed to create vector extension: %v\n", err)
	}

	return nil
}

// SimpleTokenCounter is a basic token counter implementation
type SimpleTokenCounter struct{}

// NewSimpleTokenCounter creates a new simple token counter
func NewSimpleTokenCounter() *SimpleTokenCounter {
	return &SimpleTokenCounter{}
}

// CountTokens counts tokens using a simple word-based approximation
func (c *SimpleTokenCounter) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	// Simple approximation: ~0.75 tokens per word
	words := len(strings.Fields(text))
	return int(float64(words) * 0.75)
}

// CountTokensBatch counts tokens for multiple texts
func (c *SimpleTokenCounter) CountTokensBatch(texts []string) []int {
	counts := make([]int, len(texts))
	for i, text := range texts {
		counts[i] = c.CountTokens(text)
	}
	return counts
}

// DefaultMemoryServiceConfig returns a default configuration
func DefaultMemoryServiceConfig() *MemoryServiceConfig {
	return &MemoryServiceConfig{
		EmbeddingConfig: numa.EmbeddingServiceConfig{
			Provider: "local", // Use local embeddings by default
		},
		VectorStoreType:  "neo4j",  // Use Neo4j vector store
		TokenCounterType: "simple", // Use simple token counter
		MaxConnections:   10,       // Default connection pool size
		EnableMigrations: true,     // Enable automatic migrations
		EnableExtraction: true,     // Enable Knowledge functionality (knowledge extraction with Numa)
		Neo4jConfig: graph.Neo4jConfig{
			URI: "bolt://localhost:7687", // Default Neo4j URI
		},
	}
}

// DetectVersionConflict detects version conflicts for sequential agent operations
func (s *EionMemoryService) DetectVersionConflict(
	ctx context.Context,
	resourceID string,
	expectedVersion int,
	agentID string,
) (*numa.ConflictDetection, error) {
	// Get current version from database
	currentVersion, _, err := s.getCurrentResourceVersion(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current resource version: %w", err)
	}

	// Get userID from session to properly pass to temporal operations
	userID, err := s.getUserIDFromSession(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID for session: %w", err)
	}

	return s.temporalOps.DetectVersionConflict(
		expectedVersion,
		currentVersion,
		resourceID,
		agentID,
		userID, // Properly use userID instead of empty string
	), nil
}

// getUserIDFromSession gets the userID associated with a session
func (s *EionMemoryService) getUserIDFromSession(ctx context.Context, sessionID string) (string, error) {
	var session SessionSchema
	err := s.db.NewSelect().
		Model(&session).
		Where("session_id = ?", sessionID).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	return session.UserID, nil
}

// ResolveConflict resolves conflicts using specified strategy
func (s *EionMemoryService) ResolveConflict(
	ctx context.Context,
	conflict numa.ConflictDetection,
	strategy string,
	resolvedBy string,
) (numa.ConflictResolution, error) {
	resolution := s.temporalOps.ResolveConflict(conflict, strategy, resolvedBy)

	// Log the resolution
	s.logger.Info("Resolved conflict",
		zap.String("conflict_id", conflict.ConflictID),
		zap.String("strategy", strategy),
		zap.String("resolved_by", resolvedBy),
		zap.Bool("requires_manual_action", resolution.RequiresManualAction))

	return resolution, nil
}

// PutMemoryWithVersionCheck stores memory with version conflict detection
func (s *EionMemoryService) PutMemoryWithVersionCheck(
	ctx context.Context,
	sessionID, agentID string,
	memory *Memory,
	expectedVersion int,
) (*numa.ConflictDetection, error) {
	// Check for version conflicts first
	conflict, err := s.DetectVersionConflict(ctx, sessionID, expectedVersion, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to detect version conflict: %w", err)
	}

	if conflict != nil {
		// Return conflict for agent to handle
		return conflict, nil
	}

	// No conflict, proceed with normal storage
	err = s.PutMemory(ctx, sessionID, agentID, memory, false)
	if err != nil {
		return nil, fmt.Errorf("failed to store memory: %w", err)
	}

	// Update version
	err = s.incrementResourceVersion(ctx, sessionID, agentID)
	if err != nil {
		s.logger.Error("Failed to increment resource version",
			zap.String("session_id", sessionID),
			zap.String("agent_id", agentID),
			zap.Error(err))
		// Don't fail the operation for version tracking issues
	}

	return nil, nil
}

// getCurrentResourceVersion gets the current version of a resource
func (s *EionMemoryService) getCurrentResourceVersion(ctx context.Context, resourceID string) (int, string, error) {
	// Query the sessions table for version information
	var session SessionSchema
	err := s.db.NewSelect().
		Model(&session).
		Where("session_id = ?", resourceID).
		Scan(ctx)

	if err != nil {
		// Resource doesn't exist yet, start with version 0
		return 0, "", nil
	}

	// Since SessionSchema no longer has metadata, use simple version tracking
	version := 1 // default version
	lastModifiedBy := ""

	return version, lastModifiedBy, nil
}

// incrementResourceVersion increments the version of a resource
func (s *EionMemoryService) incrementResourceVersion(ctx context.Context, resourceID, agentID string) error {
	// Since SessionSchema no longer has metadata, just update the updated_at timestamp
	_, err := s.db.NewUpdate().
		Model((*SessionSchema)(nil)).
		Set("updated_at = ?", time.Now()).
		Where("session_id = ?", resourceID).
		Exec(ctx)

	return err
}

// putMemoryWithAutoConflictResolution attempts to store memory with automatic conflict detection
func (s *EionMemoryService) putMemoryWithAutoConflictResolution(
	ctx context.Context,
	sessionID, agentID string,
	memory *Memory,
	expectedVersion int,
) (*numa.ConflictDetection, error) {
	// Use existing version check method
	return s.PutMemoryWithVersionCheck(ctx, sessionID, agentID, memory, expectedVersion)
}

// autoResolveConflictWithStrategies applies automatic conflict resolution strategies similar to Graphiti
func (s *EionMemoryService) autoResolveConflictWithStrategies(
	ctx context.Context,
	sessionID, agentID string,
	memory *Memory,
	conflict numa.ConflictDetection,
) (bool, error) {
	const maxRetries = 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		strategy := s.selectResolutionStrategy(attempt)

		s.logger.Info("Applying automatic conflict resolution strategy",
			zap.String("session_id", sessionID),
			zap.String("agent_id", agentID),
			zap.String("strategy", strategy),
			zap.Int("attempt", attempt),
			zap.String("conflict_id", conflict.ConflictID))

		resolution, err := s.ResolveConflict(ctx, conflict, strategy, agentID)
		if err != nil {
			s.logger.Error("Failed to resolve conflict",
				zap.String("session_id", sessionID),
				zap.String("strategy", strategy),
				zap.Int("attempt", attempt),
				zap.Error(err))
			continue
		}

		// Apply the resolution strategy
		switch strategy {
		case "last_writer_wins":
			// Accept the write and proceed - this is handled naturally by continuing processing
			s.logger.Info("Conflict resolved via last writer wins",
				zap.String("session_id", sessionID),
				zap.String("resolution_id", resolution.ConflictID))
			return true, nil

		case "content_merge":
			// Try to merge non-conflicting content
			merged := s.tryContentMerge(ctx, memory, conflict)
			if merged {
				s.logger.Info("Conflict resolved via content merge",
					zap.String("session_id", sessionID),
					zap.String("resolution_id", resolution.ConflictID))
				return true, nil
			}

		case "temporal_ordering":
			// Apply temporal ordering to resolve conflicts
			ordered := s.applyTemporalOrdering(ctx, memory, conflict)
			if ordered {
				s.logger.Info("Conflict resolved via temporal ordering",
					zap.String("session_id", sessionID),
					zap.String("resolution_id", resolution.ConflictID))
				return true, nil
			}
		}

		s.logger.Warn("Resolution strategy failed, trying next strategy",
			zap.String("strategy", strategy),
			zap.Int("attempt", attempt))
	}

	s.logger.Warn("All automatic resolution strategies exhausted",
		zap.String("session_id", sessionID),
		zap.String("agent_id", agentID),
		zap.String("conflict_id", conflict.ConflictID),
		zap.Int("max_attempts", maxRetries))

	return false, nil
}

// selectResolutionStrategy selects appropriate resolution strategy based on attempt number (Graphiti-style)
func (s *EionMemoryService) selectResolutionStrategy(attempt int) string {
	switch attempt {
	case 1:
		return "content_merge" // Try merging first
	case 2:
		return "temporal_ordering" // Then temporal ordering
	default:
		return "last_writer_wins" // Finally last writer wins
	}
}

// tryContentMerge attempts to merge non-conflicting content (similar to Graphiti's approach)
func (s *EionMemoryService) tryContentMerge(ctx context.Context, memory *Memory, conflict numa.ConflictDetection) bool {
	// For now, implement basic content merge logic
	// In production, this would use more sophisticated content analysis
	s.logger.Debug("Attempting content merge resolution",
		zap.String("resource_id", conflict.ResourceID),
		zap.String("conflict_id", conflict.ConflictID))

	// Simple heuristic: if messages are complementary (different roles), they can be merged
	// This is a simplified version - Graphiti uses more sophisticated semantic analysis
	return len(memory.Messages) > 0 // Allow merge if there are messages to process
}

// applyTemporalOrdering applies temporal ordering to resolve conflicts (similar to Graphiti)
func (s *EionMemoryService) applyTemporalOrdering(ctx context.Context, memory *Memory, conflict numa.ConflictDetection) bool {
	// Add temporal metadata to preserve chronological order
	now := time.Now()
	for i := range memory.Messages {
		if memory.Messages[i].Metadata == nil {
			memory.Messages[i].Metadata = make(map[string]any)
		}
		memory.Messages[i].Metadata["conflict_resolved_at"] = now
		memory.Messages[i].Metadata["resolution_strategy"] = "temporal_ordering"
		memory.Messages[i].Metadata["conflict_id"] = conflict.ConflictID
	}

	s.logger.Debug("Applied temporal ordering resolution",
		zap.String("resource_id", conflict.ResourceID),
		zap.String("conflict_id", conflict.ConflictID))

	return true
}

// storeKnowledgeInNeo4jWithAutoConflictResolution stores knowledge with automatic temporal conflict resolution (Graphiti-style)
func (s *EionMemoryService) storeKnowledgeInNeo4jWithAutoConflictResolution(
	ctx context.Context,
	response *numa.ExtractionResponse,
	groupID, agentID string,
) error {
	database := s.config.Neo4jConfig.Database
	if database == "" {
		return fmt.Errorf("database name is required for temporal operations")
	}

	// Store entity nodes first (same as before)
	for _, node := range response.ExtractedNodes {
		// Generate embedding for the entity
		embedding, err := s.embeddingService.GenerateEmbedding(ctx, node.Name+" "+node.Summary)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for entity %s: %w", node.Name, err)
		}

		entityNode := graph.EntityNode{
			UUID:      node.UUID,
			Name:      node.Name,
			Labels:    []string{"Entity"},
			Summary:   node.Summary,
			GroupID:   groupID,
			CreatedAt: time.Now(),
			Metadata: map[string]any{
				"source":   "extraction",
				"agent_id": agentID,
			},
			Embedding: embedding,
		}

		err = s.neo4jClient.StoreEntityNode(ctx, entityNode, database)
		if err != nil {
			return fmt.Errorf("failed to store entity node %s: %w", node.Name, err)
		}
	}

	// Process edges with AUTOMATIC temporal conflict resolution (enhanced version of processEdgeWithTemporalLogic)
	for _, extractedEdge := range response.ExtractedEdges {
		if err := s.processEdgeWithAutomaticConflictResolution(ctx, extractedEdge, groupID, agentID, database); err != nil {
			return fmt.Errorf("failed to process edge %s->%s with automatic conflict resolution: %w",
				extractedEdge.SourceUUID, extractedEdge.TargetUUID, err)
		}
	}

	return nil
}

// processEdgeWithAutomaticConflictResolution processes edges with fully automatic conflict resolution (Graphiti-style)
func (s *EionMemoryService) processEdgeWithAutomaticConflictResolution(
	ctx context.Context,
	extractedEdge numa.EdgeNode,
	groupID, agentID, database string,
) error {
	now := time.Now().UTC()

	// Generate fact embedding for the edge
	factEmbedding, err := s.embeddingService.GenerateEmbedding(ctx, extractedEdge.Summary)
	if err != nil {
		return fmt.Errorf("failed to generate fact embedding: %w", err)
	}

	// Convert to Eion EdgeNode with temporal fields
	newEdge := graph.EdgeNode{
		UUID:           extractedEdge.UUID,
		SourceNodeUUID: extractedEdge.SourceUUID,
		TargetNodeUUID: extractedEdge.TargetUUID,
		RelationType:   extractedEdge.RelationType,
		Summary:        extractedEdge.Summary,
		Fact:           extractedEdge.Summary, // Use summary as fact for now
		GroupID:        groupID,
		CreatedAt:      now,
		Episodes:       []string{groupID}, // Link to current episode
		ValidAt:        &now,              // Temporal field - when fact became true
		Version:        1,                 // Initial version for conflict detection
		LastModifiedBy: agentID,           // Track the agent who modified
		ChecksumHash:   s.temporalOps.GenerateChecksum(extractedEdge.Summary),
		Metadata: map[string]any{
			"source":                   "extraction",
			"agent_id":                 agentID,
			"auto_conflict_resolution": true, // Mark as processed with automatic conflict resolution
		},
		FactEmbedding: factEmbedding,
	}

	// AUTOMATIC CONFLICT DETECTION: Get invalidation candidates using temporal logic
	candidates, err := s.temporalOps.GetEdgeInvalidationCandidates(
		ctx,
		[]graph.EdgeNode{newEdge},
		[]string{groupID},
		0.7, // minimum similarity score
		10,  // limit candidates
		database,
	)
	if err != nil {
		return fmt.Errorf("failed to get invalidation candidates: %w", err)
	}

	var invalidationCandidates []graph.EdgeNode
	if len(candidates) > 0 {
		invalidationCandidates = candidates[0]
	}

	// AUTOMATIC DUPLICATE DETECTION: Check for duplicates using API-free strategies
	duplicate := s.temporalOps.DetectDuplicateEdge(
		newEdge,
		invalidationCandidates,
		numa.VectorSimilarity, // Try vector similarity first
		0.85,                  // High threshold for duplicates
	)

	var resolvedEdge graph.EdgeNode
	if duplicate != nil {
		// AUTOMATIC MERGE: Merge with existing edge
		resolvedEdge = *duplicate
		resolvedEdge.Episodes = append(resolvedEdge.Episodes, groupID)
		resolvedEdge.LastModifiedBy = agentID
		resolvedEdge.Version++
		resolvedEdge.ChecksumHash = s.temporalOps.GenerateChecksum(resolvedEdge.Fact)
		// Add metadata about automatic resolution
		if resolvedEdge.Metadata == nil {
			resolvedEdge.Metadata = make(map[string]any)
		}
		resolvedEdge.Metadata["auto_merged"] = true
		resolvedEdge.Metadata["merged_with"] = newEdge.UUID
		resolvedEdge.Metadata["merged_by"] = agentID
		resolvedEdge.Metadata["merged_at"] = now

		s.logger.Info("Automatically merged with existing edge",
			zap.String("new_edge", newEdge.UUID),
			zap.String("existing_edge", duplicate.UUID),
			zap.String("agent_id", agentID))
	} else {
		// Use new edge
		resolvedEdge = newEdge

		s.logger.Info("Created new edge with automatic conflict resolution",
			zap.String("edge", newEdge.UUID),
			zap.String("agent_id", agentID))
	}

	// AUTOMATIC CONTRADICTION RESOLUTION: Resolve contradictions using temporal logic (like Graphiti)
	invalidatedEdges := s.temporalOps.ResolveEdgeContradictions(resolvedEdge, invalidationCandidates)

	// AUTOMATIC STORAGE: Store the resolved edge
	err = s.neo4jClient.StoreEdgeNode(ctx, resolvedEdge, database)
	if err != nil {
		return fmt.Errorf("failed to store resolved edge: %w", err)
	}

	// AUTOMATIC INVALIDATION: Update invalidated edges
	for _, invalidatedEdge := range invalidatedEdges {
		// Add metadata about automatic invalidation
		if invalidatedEdge.Metadata == nil {
			invalidatedEdge.Metadata = make(map[string]any)
		}
		invalidatedEdge.Metadata["auto_invalidated"] = true
		invalidatedEdge.Metadata["invalidated_by"] = agentID
		invalidatedEdge.Metadata["invalidated_at"] = now
		invalidatedEdge.Metadata["superseded_by"] = resolvedEdge.UUID

		err = s.neo4jClient.StoreEdgeNode(ctx, invalidatedEdge, database)
		if err != nil {
			s.logger.Error("Failed to update automatically invalidated edge",
				zap.String("edge_uuid", invalidatedEdge.UUID),
				zap.String("agent_id", agentID),
				zap.Error(err))
			// Continue processing other edges
		} else {
			s.logger.Info("Automatically invalidated contradictory edge",
				zap.String("invalidated_edge", invalidatedEdge.UUID),
				zap.String("resolved_edge", resolvedEdge.UUID),
				zap.String("agent_id", agentID))
		}
	}

	return nil
}
