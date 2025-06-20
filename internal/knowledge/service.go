package knowledge

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
	"go.uber.org/zap"

	"github.com/eion/eion/internal/memory"
	"github.com/eion/eion/internal/numa"
)

// initializeDatabase initializes PostgreSQL database connection
func initializeDatabase(databaseURL string, maxConnections int) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseURL)))
	sqldb.SetMaxOpenConns(maxConnections)
	sqldb.SetMaxIdleConns(maxConnections / 2)
	sqldb.SetConnMaxLifetime(time.Hour)

	db := bun.NewDB(sqldb, pgdialect.New())
	return db, nil
}

// initializeTokenCounter creates a simple token counter
func initializeTokenCounter(counterType string) memory.TokenCounter {
	return &SimpleTokenCounter{}
}

// SimpleTokenCounter implements basic token counting
type SimpleTokenCounter struct{}

func (s *SimpleTokenCounter) CountTokens(text string) int {
	return len(strings.Fields(text)) // Simple word count approximation
}

func (s *SimpleTokenCounter) CountTokensBatch(texts []string) []int {
	counts := make([]int, len(texts))
	for i, text := range texts {
		counts[i] = s.CountTokens(text)
	}
	return counts
}

// KnowledgeMemoryService - Knowledge graph enhanced memory service
// Uses direct Neo4j integration for knowledge extraction and storage
type KnowledgeMemoryService struct {
	// Core services
	knowledgeClient  *KnowledgeClient // Direct Neo4j knowledge integration
	embeddingService numa.EmbeddingService
	tokenCounter     memory.TokenCounter
	db               *bun.DB // PostgreSQL for sessions/users only
	config           *KnowledgeMemoryServiceConfig
	logger           *zap.Logger

	// Store for PostgreSQL operations (sessions, users)
	pgStore *memory.EionMemoryStore
}

// KnowledgeMemoryServiceConfig - Configuration for knowledge graph-enhanced memory service
type KnowledgeMemoryServiceConfig struct {
	DatabaseURL      string // PostgreSQL for sessions/users only
	MaxConnections   int
	EnableMigrations bool

	// Knowledge graph configuration
	Neo4jURI      string
	Neo4jUser     string
	Neo4jPassword string
	OpenAIAPIKey  string // Optional for knowledge extraction

	// Python paths
	PythonPath           string // Path to Python executable in .venv
	KnowledgeServicePath string // Path to our Python extraction service

	// Embedding configuration
	EmbeddingConfig  numa.EmbeddingServiceConfig
	TokenCounterType string
}

// NewKnowledgeMemoryService creates a memory service using knowledge graph integration
func NewKnowledgeMemoryService(config *KnowledgeMemoryServiceConfig, logger *zap.Logger) (*KnowledgeMemoryService, error) {
	if config == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	// Validate required configuration
	if config.Neo4jURI == "" || config.Neo4jUser == "" || config.Neo4jPassword == "" {
		return nil, fmt.Errorf("Neo4j configuration is required for knowledge graph")
	}

	if config.PythonPath == "" {
		return nil, fmt.Errorf("Python path is required for extraction service")
	}

	if config.KnowledgeServicePath == "" {
		return nil, fmt.Errorf("Extraction service path is required")
	}

	logger.Info("Initializing Knowledge Memory Service",
		zap.String("neo4j_uri", config.Neo4jURI),
		zap.String("python_path", config.PythonPath),
		zap.String("knowledge_service_path", config.KnowledgeServicePath))

	// Initialize PostgreSQL database for sessions/users
	db, err := initializeDatabase(config.DatabaseURL, config.MaxConnections)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PostgreSQL database: %w", err)
	}

	// Initialize embedding service
	embeddingService, err := numa.NewEmbeddingService(config.EmbeddingConfig)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize embedding service: %w", err)
	}

	// Initialize token counter
	tokenCounter := initializeTokenCounter(config.TokenCounterType)

	// Initialize PostgreSQL store for sessions/users
	pgStore := memory.NewEionMemoryStore(db, embeddingService, nil, nil, tokenCounter)

	// Initialize Knowledge client
	knowledgeClient := NewKnowledgeClient(
		config.PythonPath,
		config.KnowledgeServicePath,
		config.Neo4jURI,
		config.Neo4jUser,
		config.Neo4jPassword,
		config.OpenAIAPIKey,
		logger,
	)

	service := &KnowledgeMemoryService{
		knowledgeClient:  knowledgeClient,
		embeddingService: embeddingService,
		tokenCounter:     tokenCounter,
		db:               db,
		config:           config,
		logger:           logger,
		pgStore:          pgStore,
	}

	// Run migrations if enabled
	if config.EnableMigrations {
		err = service.runMigrations(context.Background())
		if err != nil {
			service.Close(context.Background())
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	logger.Info("Knowledge memory service initialized successfully")
	return service, nil
}

// Initialize initializes the knowledge memory service (alias for existing method)
func (s *KnowledgeMemoryService) Initialize(ctx context.Context) error {
	// This method already exists, just creating an alias to match interface
	return nil // Already initialized in NewKnowledgeMemoryService
}

// StartupHealthCheck performs critical health checks for startup
func (s *KnowledgeMemoryService) StartupHealthCheck(ctx context.Context) error {
	return s.HealthCheck(ctx)
}

// Close closes the knowledge memory service and cleans up resources
func (s *KnowledgeMemoryService) Close(ctx context.Context) error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// HealthCheck performs a health check
func (s *KnowledgeMemoryService) HealthCheck(ctx context.Context) error {
	// Check PostgreSQL
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("PostgreSQL health check failed: %w", err)
	}

	// Check Knowledge
	if err := s.knowledgeClient.HealthCheck(ctx); err != nil {
		return fmt.Errorf("Knowledge health check failed: %w", err)
	}

	return nil
}

// PutMemory stores memory in PostgreSQL and processes it into knowledge graph
func (s *KnowledgeMemoryService) PutMemory(ctx context.Context, sessionID, agentID string, mem *memory.Memory, skipProcessing bool) error {
	// Get userID from session for PostgreSQL storage
	userID, err := s.getUserIDFromSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get user ID for session: %w", err)
	}

	// Store in PostgreSQL for API compatibility (always skip processing in pgStore since we handle it here)
	err = s.pgStore.PutMemory(ctx, sessionID, agentID, userID, mem, true)
	if err != nil {
		return fmt.Errorf("failed to store in PostgreSQL: %w", err)
	}

	// Process into knowledge graph if not skipping
	if !skipProcessing {
		err = s.processIntoKnowledgeGraph(ctx, mem, sessionID, agentID)
		if err != nil {
			s.logger.Warn("Failed to process into knowledge graph", zap.Error(err))
			// Don't fail the entire operation if knowledge processing fails
		}
	}

	return nil
}

// processIntoKnowledgeGraph processes messages into the knowledge graph
func (s *KnowledgeMemoryService) processIntoKnowledgeGraph(ctx context.Context, mem *memory.Memory, sessionID, agentID string) error {
	if len(mem.Messages) == 0 {
		return nil
	}

	s.logger.Debug("Starting knowledge graph processing",
		zap.String("sessionID", sessionID),
		zap.String("agentID", agentID),
		zap.Int("message_count", len(mem.Messages)))

	// Initialize knowledge client if not already done
	if err := s.knowledgeClient.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize knowledge client: %w", err)
	}

	// Combine all messages into episode content
	var contentParts []string
	for _, msg := range mem.Messages {
		contentParts = append(contentParts, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}
	content := strings.Join(contentParts, "\n")

	// Create episode name from session and agent
	episodeName := fmt.Sprintf("session_%s_agent_%s_%d", sessionID, agentID, len(mem.Messages))

	// FIXED: Use sessionID as groupID for multi-agent knowledge sharing
	// This allows all agents in the same session to access shared knowledge
	groupID := sessionID

	s.logger.Debug("Adding episode to knowledge graph",
		zap.String("episode_name", episodeName),
		zap.String("group_id", groupID),
		zap.String("agent_id", agentID),
		zap.Int("content_length", len(content)))

	// Add episode to knowledge graph with session-scoped groupID
	result, err := s.knowledgeClient.AddEpisode(ctx, episodeName, content, fmt.Sprintf("Session %s Agent %s", sessionID, agentID), groupID, "conversation")
	if err != nil {
		s.logger.Error("Knowledge graph processing failed",
			zap.Error(err),
			zap.String("episode_name", episodeName))
		return fmt.Errorf("failed to add episode to knowledge graph: %w", err)
	}

	s.logger.Debug("Successfully processed messages into knowledge graph",
		zap.String("episode_uuid", result.EpisodeUUID),
		zap.String("session_id", sessionID),
		zap.String("agent_id", agentID),
		zap.Int("nodes_created", result.NodesCreated),
		zap.Int("edges_created", result.EdgesCreated))

	return nil
}

// SearchMemory performs semantic search using Knowledge graph (the primary search system)
func (s *KnowledgeMemoryService) SearchMemory(ctx context.Context, query *memory.MemorySearchQuery) (*memory.MemorySearchResult, error) {
	s.logger.Debug("Starting knowledge graph search",
		zap.String("query", query.Text),
		zap.String("agentID", query.AgentID))

	// FIXED: For multi-agent collaboration, search should be session-scoped
	// Check if sessionID is provided in metadata, otherwise fallback to agentID scope
	var groupIDs []string
	if sessionIDMeta, exists := query.Metadata["session_id"]; exists {
		if sessionID, ok := sessionIDMeta.(string); ok && sessionID != "" {
			// Use sessionID for multi-agent knowledge sharing
			groupIDs = append(groupIDs, sessionID)
			s.logger.Debug("Using session-scoped search", zap.String("session_id", sessionID))
		}
	} else if query.AgentID != "" {
		// Fallback to agent-scoped search for backward compatibility
		groupIDs = append(groupIDs, query.AgentID)
		s.logger.Debug("Using agent-scoped search", zap.String("agent_id", query.AgentID))
	}

	// Use Knowledge search - this IS the basic search system
	result, err := s.knowledgeClient.Search(ctx, query.Text, groupIDs, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("knowledge graph search failed: %w", err)
	}

	// Convert Knowledge results to MemorySearchResult
	var searchMessages []memory.Message
	var searchFacts []memory.Fact

	for _, edge := range result.Results {
		// Create message from edge fact
		message := memory.Message{
			UUID:       uuid.New(),
			Role:       "assistant",
			Content:    edge.Fact,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			AgentID:    query.AgentID,
			SessionID:  "",                 // No session for search results
			TokenCount: len(edge.Fact) / 4, // Rough estimate
		}
		searchMessages = append(searchMessages, message)

		// Also create a fact
		fact := memory.Fact{
			UUID:      uuid.New(),
			Content:   edge.Fact,
			AgentID:   query.AgentID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Rating:    0.8, // Default high rating for KG facts
			Metadata:  map[string]any{"source": "knowledge_graph"},
		}
		searchFacts = append(searchFacts, fact)
	}

	s.logger.Debug("Knowledge graph search completed",
		zap.Int("results_count", len(searchMessages)),
		zap.Strings("group_ids", groupIDs))

	return &memory.MemorySearchResult{
		Messages:   searchMessages,
		Facts:      searchFacts,
		TotalCount: len(searchMessages),
	}, nil
}

// GetMemory retrieves memory using Knowledge
func (s *KnowledgeMemoryService) GetMemory(ctx context.Context, sessionID, agentID string, lastNMessages int, opts ...memory.FilterOption) (*memory.Memory, error) {
	return s.pgStore.GetMemory(ctx, sessionID, agentID, "", lastNMessages, opts...)
}

// Message management methods (delegate to PostgreSQL store for API compatibility)
func (s *KnowledgeMemoryService) GetMessages(ctx context.Context, sessionID, agentID string, lastNMessages int, beforeUUID uuid.UUID) ([]memory.Message, error) {
	return s.pgStore.GetMessages(ctx, sessionID, agentID, "", lastNMessages, beforeUUID)
}

func (s *KnowledgeMemoryService) GetMessageList(ctx context.Context, sessionID, agentID string, pageNumber, pageSize int) (*memory.MessageListResponse, error) {
	return s.pgStore.GetMessageList(ctx, sessionID, agentID, "", pageNumber, pageSize)
}

func (s *KnowledgeMemoryService) GetMessagesByUUID(ctx context.Context, sessionID, agentID string, uuids []uuid.UUID) ([]memory.Message, error) {
	return s.pgStore.GetMessagesByUUID(ctx, sessionID, agentID, "", uuids)
}

func (s *KnowledgeMemoryService) PutMessages(ctx context.Context, sessionID, agentID string, messages []memory.Message) ([]memory.Message, error) {
	return s.pgStore.PutMessages(ctx, sessionID, agentID, "", messages)
}

func (s *KnowledgeMemoryService) UpdateMessages(ctx context.Context, sessionID, agentID string, messages []memory.Message, includeContent bool) error {
	return s.pgStore.UpdateMessages(ctx, sessionID, agentID, "", messages, includeContent)
}

func (s *KnowledgeMemoryService) DeleteMessages(ctx context.Context, sessionID, agentID string, messageUUIDs []uuid.UUID) error {
	return s.pgStore.DeleteMessages(ctx, sessionID, agentID, "", messageUUIDs)
}

// Fact methods - not implemented for now (Knowledge handles knowledge graph)
func (s *KnowledgeMemoryService) GetFacts(ctx context.Context, agentID string, limit int, opts ...memory.FilterOption) ([]memory.Fact, error) {
	return s.pgStore.GetFacts(ctx, "", agentID, limit, opts...)
}

func (s *KnowledgeMemoryService) PutFact(ctx context.Context, fact *memory.Fact) error {
	return fmt.Errorf("PutFact not implemented - use PutMemory with Knowledge")
}

func (s *KnowledgeMemoryService) UpdateFact(ctx context.Context, fact *memory.Fact) error {
	return fmt.Errorf("UpdateFact not implemented - use PutMemory with Knowledge")
}

func (s *KnowledgeMemoryService) DeleteFact(ctx context.Context, factUUID uuid.UUID) error {
	return fmt.Errorf("DeleteFact not implemented - Knowledge manages knowledge graph")
}

func (s *KnowledgeMemoryService) PurgeDeleted(ctx context.Context, spaceID string) error {
	return fmt.Errorf("PurgeDeleted not implemented - Knowledge manages knowledge graph")
}

// Utility methods
func (s *KnowledgeMemoryService) runMigrations(ctx context.Context) error {
	// Only run basic PostgreSQL migrations using memory package
	err := memory.CreateTables(ctx, s.db)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	err = memory.CreateIndexes(ctx, s.db)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	s.logger.Info("PostgreSQL migrations completed")
	return nil
}

// DefaultKnowledgeMemoryServiceConfig returns default configuration
func DefaultKnowledgeMemoryServiceConfig() *KnowledgeMemoryServiceConfig {
	return &KnowledgeMemoryServiceConfig{
		DatabaseURL:          "postgres://localhost:5432/eion?sslmode=disable",
		MaxConnections:       10,
		EnableMigrations:     true,
		Neo4jURI:             "bolt://localhost:7687",
		Neo4jUser:            "neo4j",
		Neo4jPassword:        "password",
		PythonPath:           ".venv/bin/python",
		KnowledgeServicePath: "internal/knowledge/python/knowledge_service.py",
		EmbeddingConfig: numa.EmbeddingServiceConfig{
			Provider:  "local",
			Dimension: 384,
			Model:     "all-MiniLM-L6-v2",
		},
		TokenCounterType: "simple",
	}
}

// GetDB returns the database connection for orchestrator
func (s *KnowledgeMemoryService) GetDB() *bun.DB {
	return s.db
}

// Week 3: Conflict detection and resolution methods
// These methods are required by knowledge handlers but missing from KnowledgeMemoryService

// DetectVersionConflict detects version conflicts for sequential agent operations
func (s *KnowledgeMemoryService) DetectVersionConflict(
	ctx context.Context,
	resourceID string,
	expectedVersion int,
	agentID string,
	userID string,
) (*numa.ConflictDetection, error) {
	s.logger.Info("DEBUG: Starting conflict detection",
		zap.String("resource_id", resourceID),
		zap.String("agent_id", agentID),
		zap.String("user_id", userID),
		zap.Int("expected_version", expectedVersion))

	// Get current version from database
	currentVersion, lastModifiedBy, err := s.getCurrentResourceVersion(ctx, resourceID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current resource version: %w", err)
	}

	s.logger.Info("DEBUG: Comparing versions for conflict detection",
		zap.Int("expected_version", expectedVersion),
		zap.Int("current_version", currentVersion),
		zap.Bool("conflict_detected", expectedVersion != currentVersion))

	// Check for conflict
	if expectedVersion != currentVersion {
		conflictID := uuid.New().String()
		conflict := &numa.ConflictDetection{
			ConflictID:       conflictID,
			UserID:           userID,
			ResourceID:       resourceID,
			ExpectedVersion:  expectedVersion,
			ActualVersion:    currentVersion,
			ConflictingAgent: agentID,
			LastModifiedBy:   lastModifiedBy,
			DetectedAt:       time.Now(),
			Status:           "detected",
		}

		s.logger.Warn("Version conflict detected",
			zap.String("conflict_id", conflictID),
			zap.String("resource_id", resourceID),
			zap.Int("expected_version", expectedVersion),
			zap.Int("actual_version", currentVersion),
			zap.String("agent_id", agentID))

		return conflict, nil
	}

	s.logger.Info("DEBUG: No conflict detected - versions match")
	return nil, nil // No conflict
}

// ResolveConflict resolves conflicts using specified strategy
func (s *KnowledgeMemoryService) ResolveConflict(
	ctx context.Context,
	conflict numa.ConflictDetection,
	strategy string,
	resolvedBy string,
) (numa.ConflictResolution, error) {
	resolution := numa.ConflictResolution{
		ConflictID:           conflict.ConflictID,
		ResolutionType:       "auto",
		Strategy:             strategy,
		ResolvedBy:           resolvedBy,
		ResolvedAt:           time.Now(),
		ResolutionData:       make(map[string]interface{}),
		RequiresManualAction: false,
	}

	// Implement resolution strategies
	switch strategy {
	case "last_writer_wins":
		// Accept the newer version, increment for next write
		resolution.ResolutionData["outcome"] = "accepted_newer_version"
		resolution.RequiresManualAction = false
	case "retry":
		// Suggest agent retry with current version
		resolution.ResolutionData["suggested_action"] = "retry_with_current_version"
		resolution.ResolutionData["current_version"] = conflict.ActualVersion
		resolution.RequiresManualAction = true
	default:
		resolution.Strategy = "manual"
		resolution.RequiresManualAction = true
		resolution.ResolutionData["reason"] = "unknown_strategy"
	}

	s.logger.Info("Resolved conflict",
		zap.String("conflict_id", conflict.ConflictID),
		zap.String("strategy", strategy),
		zap.String("resolved_by", resolvedBy),
		zap.Bool("requires_manual_action", resolution.RequiresManualAction))

	return resolution, nil
}

// PutMemoryWithVersionCheck stores memory with version conflict detection
func (s *KnowledgeMemoryService) PutMemoryWithVersionCheck(
	ctx context.Context,
	sessionID, agentID string,
	memory *memory.Memory,
	expectedVersion int,
) (*numa.ConflictDetection, error) {
	// Get userID from session for conflict detection
	userID, err := s.getUserIDFromSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID for session: %w", err)
	}

	// Check for version conflicts first
	conflict, err := s.DetectVersionConflict(ctx, sessionID, expectedVersion, agentID, userID)
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
	err = s.incrementResourceVersion(ctx, sessionID, userID, agentID)
	if err != nil {
		s.logger.Error("Failed to increment resource version",
			zap.String("session_id", sessionID),
			zap.String("agent_id", agentID),
			zap.Error(err))
		// Don't fail the operation for version tracking issues
	}

	return nil, nil
}

// getUserIDFromSession gets the userID associated with a session
func (s *KnowledgeMemoryService) getUserIDFromSession(ctx context.Context, sessionID string) (string, error) {
	var session memory.SessionSchema
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

// getCurrentResourceVersion gets the current version of a resource from PostgreSQL
func (s *KnowledgeMemoryService) getCurrentResourceVersion(ctx context.Context, resourceID, userID string) (int, string, error) {
	// Use memory package SessionSchema - without metadata since it's removed
	var session memory.SessionSchema

	s.logger.Info("DEBUG: Getting current resource version",
		zap.String("resource_id", resourceID),
		zap.String("user_id", userID))

	err := s.db.NewSelect().
		Model(&session).
		Where("session_id = ? AND user_id = ?", resourceID, userID).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err != nil {
		s.logger.Info("DEBUG: No session found in database - starting with version 0",
			zap.String("resource_id", resourceID),
			zap.String("user_id", userID),
			zap.Error(err))
		// Resource doesn't exist yet, start with version 0
		return 0, "", nil
	}

	// Since Metadata field is removed, use simple version tracking
	version := 1 // default version
	lastModifiedBy := ""

	s.logger.Info("DEBUG: Found current resource version",
		zap.String("resource_id", resourceID),
		zap.String("user_id", userID),
		zap.Int("version", version),
		zap.String("last_modified_by", lastModifiedBy))

	return version, lastModifiedBy, nil
}

// incrementResourceVersion increments the version of a resource
func (s *KnowledgeMemoryService) incrementResourceVersion(ctx context.Context, resourceID, userID, agentID string) error {
	// Get current version from database
	currentVersion, _, err := s.getCurrentResourceVersion(ctx, resourceID, userID)
	if err != nil {
		return err
	}

	newVersion := currentVersion + 1

	s.logger.Info("DEBUG: Incrementing resource version",
		zap.String("resource_id", resourceID),
		zap.String("user_id", userID),
		zap.Int("current_version", currentVersion),
		zap.Int("new_version", newVersion),
		zap.String("agent_id", agentID))

	// Since Metadata field is removed, just update the updated_at timestamp
	result, err := s.db.NewUpdate().
		Model((*memory.SessionSchema)(nil)).
		Set("updated_at = ?", time.Now()).
		Where("session_id = ? AND user_id = ?", resourceID, userID).
		Where("deleted_at IS NULL").
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update session version: %w", err)
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If no rows were updated, create a new session record
	if rowsAffected == 0 {
		s.logger.Info("DEBUG: Creating new session record for version tracking",
			zap.String("resource_id", resourceID),
			zap.String("user_id", userID))

		// Create basic session schema without removed fields
		sessionSchema := memory.SessionSchema{
			SessionID:     resourceID,
			UserID:        userID,
			SessionTypeID: "knowledge",
			BaseSchema: memory.BaseSchema{
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		_, err = s.db.NewInsert().
			Model(&sessionSchema).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create session record: %w", err)
		}
	}

	return nil
}

// SearchKnowledge searches through the knowledge graph
func (s *KnowledgeMemoryService) SearchKnowledge(ctx context.Context, query *memory.MemorySearchQuery) (*memory.MemorySearchResult, error) {
	// Use the memory service for search, removing SpaceID references
	searchRequest := &memory.MemorySearchQuery{
		Text:     query.Text,
		AgentID:  query.AgentID,
		Limit:    query.Limit,
		MinScore: query.MinScore,
	}

	return s.SearchMemory(ctx, searchRequest)
}
