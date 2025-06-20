package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eion/eion/internal/numa"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// EionMemoryStore implements the MemoryStore interface using PostgreSQL
type EionMemoryStore struct {
	db               *bun.DB
	embeddingService numa.EmbeddingService
	vectorStore      VectorStore
	tokenCounter     TokenCounter
}

// NewEionMemoryStore creates a new memory store instance
func NewEionMemoryStore(db *bun.DB, embeddingService numa.EmbeddingService, vectorStore VectorStore, searchService SearchService, tokenCounter TokenCounter) *EionMemoryStore {
	// Note: searchService parameter ignored - using basic search inline now
	return &EionMemoryStore{
		db:               db,
		embeddingService: embeddingService,
		vectorStore:      vectorStore,
		tokenCounter:     tokenCounter,
	}
}

// Session management methods

func (s *EionMemoryStore) GetSession(ctx context.Context, sessionID, agentID string) (*Session, error) {
	if sessionID == "" || agentID == "" {
		return nil, fmt.Errorf("sessionID and agentID cannot be empty")
	}

	var sessionSchema SessionSchema
	err := s.db.NewSelect().
		Model(&sessionSchema).
		Where("session_id = ?", sessionID).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return SessionSchemaToSession(sessionSchema), nil
}

func (s *EionMemoryStore) CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error) {
	if req.SessionID == "" || req.UserID == "" || req.SessionTypeID == "" {
		return nil, fmt.Errorf("sessionID, userID, and sessionTypeID cannot be empty")
	}

	// Check if session already exists
	existingSession, err := s.GetSession(ctx, req.SessionID, "")
	if err == nil && existingSession != nil {
		return nil, fmt.Errorf("session already exists: %s", req.SessionID)
	}

	session := &Session{
		UUID:          uuid.New(),
		SessionID:     req.SessionID,
		UserID:        req.UserID,
		SessionTypeID: req.SessionTypeID,
		SessionName:   req.SessionName,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	sessionSchema := SessionToSessionSchema(session)

	_, err = s.db.NewInsert().
		Model(&sessionSchema).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	*session = *SessionSchemaToSession(sessionSchema)
	return session, nil
}

func (s *EionMemoryStore) UpdateSession(ctx context.Context, req *UpdateSessionRequest) (*Session, error) {
	if req.SessionID == "" || req.UserID == "" || req.SessionTypeID == "" {
		return nil, fmt.Errorf("sessionID, userID, and sessionTypeID are required")
	}

	// First check if session exists
	_, err := s.GetSession(ctx, req.SessionID, "")
	if err != nil {
		return nil, err
	}

	// Session exists and is active (soft deletion handles inactive sessions)

	var updatedSchema SessionSchema
	err = s.db.NewUpdate().
		Model(&updatedSchema).
		Where("session_id = ?", req.SessionID).
		Where("deleted_at IS NULL").
		Set("updated_at = ?", time.Now()).
		Set("session_name = ?", req.SessionName).
		Returning("*").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return SessionSchemaToSession(updatedSchema), nil
}

func (s *EionMemoryStore) DeleteSession(ctx context.Context, sessionID, agentID string) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}

	result, err := s.db.NewUpdate().
		Model((*SessionSchema)(nil)).
		Where("session_id = ?", sessionID).
		Where("deleted_at IS NULL").
		Set("deleted_at = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

func (s *EionMemoryStore) ListSessions(ctx context.Context, agentID string, cursor int64, limit int) ([]*Session, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var sessionSchemas []SessionSchema
	query := s.db.NewSelect().
		Model(&sessionSchemas).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(limit)

	if cursor > 0 {
		query = query.Where("created_at < (SELECT created_at FROM sessions WHERE uuid = ?)", cursor)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := make([]*Session, len(sessionSchemas))
	for i, schema := range sessionSchemas {
		sessions[i] = SessionSchemaToSession(schema)
	}

	return sessions, nil
}

// Memory operations

func (s *EionMemoryStore) GetMemory(ctx context.Context, sessionID, agentID, spaceID string, lastNMessages int, opts ...FilterOption) (*Memory, error) {
	if sessionID == "" || agentID == "" {
		return nil, fmt.Errorf("sessionID and agentID cannot be empty")
	}

	if lastNMessages < 0 {
		return nil, fmt.Errorf("lastNMessages cannot be negative")
	}

	if lastNMessages == 0 {
		lastNMessages = DefaultLastNMessages
	}

	// Get messages
	messages, err := s.GetMessages(ctx, sessionID, agentID, spaceID, lastNMessages, uuid.Nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Get relevant facts
	facts, err := s.GetFacts(ctx, spaceID, agentID, 10, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get facts: %w", err)
	}

	return &Memory{
		Messages:      messages,
		RelevantFacts: facts,
		AgentID:       agentID,
		SessionID:     sessionID,
		Metadata:      make(map[string]any),
	}, nil
}

func (s *EionMemoryStore) PutMemory(ctx context.Context, sessionID, agentID, spaceID string, memory *Memory, skipProcessing bool) error {
	// Extract userID from memory metadata or fail
	userID, exists := memory.Metadata["user_id"].(string)
	if !exists || userID == "" {
		return fmt.Errorf("user_id is required in memory metadata")
	}
	if sessionID == "" || agentID == "" {
		return fmt.Errorf("sessionID and agentID cannot be empty")
	}

	if len(memory.Messages) == 0 {
		return fmt.Errorf("memory must contain at least one message")
	}

	if len(memory.Messages) > MaxMessagesPerMemory {
		return fmt.Errorf("memory contains too many messages (max %d)", MaxMessagesPerMemory)
	}

	// Ensure session exists
	session, err := s.GetSession(ctx, sessionID, agentID)
	if err != nil {
		// Check if the session exists and is active
		if session == nil {
			return fmt.Errorf("session %s not found", sessionID)
		}

		// Create new session
		session = &Session{
			UUID:          uuid.New(),
			SessionID:     sessionID,
			UserID:        userID,
			SessionTypeID: "default",
			SessionName:   session.SessionName,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		sessionSchema := SessionToSessionSchema(session)

		_, err = s.db.NewInsert().
			Model(&sessionSchema).
			Returning("*").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}

		*session = *SessionSchemaToSession(sessionSchema)
	}

	// Set agent and session IDs on messages
	for i := range memory.Messages {
		memory.Messages[i].AgentID = agentID
		memory.Messages[i].SessionID = sessionID

		// Generate UUID if not set
		if memory.Messages[i].UUID == uuid.Nil {
			memory.Messages[i].UUID = uuid.New()
		}

		// Set timestamps
		if memory.Messages[i].CreatedAt.IsZero() {
			memory.Messages[i].CreatedAt = time.Now()
		}
		memory.Messages[i].UpdatedAt = time.Now()

		// Count tokens if not set
		if memory.Messages[i].TokenCount == 0 && s.tokenCounter != nil {
			memory.Messages[i].TokenCount = s.tokenCounter.CountTokens(memory.Messages[i].Content)
		}
	}

	// Store messages
	_, err = s.PutMessages(ctx, sessionID, agentID, spaceID, memory.Messages)
	if err != nil {
		return fmt.Errorf("failed to store messages: %w", err)
	}

	// Process messages if not skipping
	if !skipProcessing {
		err = s.processMessages(ctx, memory.Messages, spaceID, agentID)
		if err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to process messages: %v\n", err)
		}
	}

	return nil
}

func (s *EionMemoryStore) SearchMemory(ctx context.Context, query *MemorySearchQuery) (*MemorySearchResult, error) {
	if query.Text == "" {
		return nil, fmt.Errorf("query text is required")
	}

	// ALWAYS use basic search for Phase 1 Week 1.2 Subphase 1 (Numa is disabled)
	// This provides basic keyword search functionality
	return s.searchMemoryBasic(ctx, query)
}

// searchMemoryWithNuma performs enhanced search using Numa knowledge graph
func (s *EionMemoryStore) searchMemoryWithNuma(ctx context.Context, query *MemorySearchQuery) (*MemorySearchResult, error) {
	// Use Numa for semantic search - disabled for now
	return s.searchMemoryBasic(ctx, query)
}

// searchMemoryBasic performs basic keyword search
func (s *EionMemoryStore) searchMemoryBasic(ctx context.Context, query *MemorySearchQuery) (*MemorySearchResult, error) {
	var messages []MessageSchema

	// Build search query - support multiple search terms
	searchTerms := strings.Fields(strings.ToLower(query.Text))
	if len(searchTerms) == 0 {
		return &MemorySearchResult{
			Messages:   []Message{},
			Facts:      []Fact{},
			TotalCount: 0,
		}, nil
	}

	searchQuery := s.db.NewSelect().
		Model(&messages).
		Where("deleted_at IS NULL")

	// Add search conditions - search in content (case-insensitive)
	for i, term := range searchTerms {
		if i == 0 {
			searchQuery = searchQuery.Where("LOWER(content) LIKE ?", "%"+term+"%")
		} else {
			// For multiple terms, use AND logic (all terms must match)
			searchQuery = searchQuery.Where("LOWER(content) LIKE ?", "%"+term+"%")
		}
	}

	// Optional agent filter
	if query.AgentID != "" {
		searchQuery = searchQuery.Where("agent_id = ?", query.AgentID)
	}

	// Order by relevance (most recent first)
	searchQuery = searchQuery.Order("created_at DESC")

	// Apply limit
	limit := query.Limit
	if limit <= 0 {
		limit = 10 // Default limit
	}
	if limit > 100 {
		limit = 100 // Max limit to prevent abuse
	}
	searchQuery = searchQuery.Limit(limit)

	err := searchQuery.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}

	// Convert to Message objects
	resultMessages := make([]Message, len(messages))
	for i, msg := range messages {
		resultMessages[i] = *MessageSchemaToMessage(msg)
	}

	return &MemorySearchResult{
		Messages:   resultMessages,
		Facts:      []Fact{}, // No facts in basic search - would come from Numa
		TotalCount: len(resultMessages),
		SearchMetadata: map[string]any{
			"search_type":  "basic",
			"terms_used":   searchTerms,
			"numa_enabled": false,
		},
	}, nil
}

// Message operations

func (s *EionMemoryStore) GetMessages(ctx context.Context, sessionID, agentID, spaceID string, lastNMessages int, beforeUUID uuid.UUID) ([]Message, error) {
	if sessionID == "" || agentID == "" {
		return nil, fmt.Errorf("sessionID and agentID cannot be empty")
	}

	if lastNMessages < 0 {
		return nil, fmt.Errorf("lastNMessages cannot be negative")
	}

	query := s.db.NewSelect().
		Model((*MessageSchema)(nil)).
		Where("session_id = ? AND agent_id = ?", sessionID, agentID).
		Where("deleted_at IS NULL").
		Order("created_at DESC")

	if beforeUUID != uuid.Nil {
		query = query.Where("created_at < (SELECT created_at FROM messages WHERE uuid = ?)", beforeUUID)
	}

	if lastNMessages > 0 {
		query = query.Limit(lastNMessages)
	}

	var messageSchemas []MessageSchema
	err := query.Scan(ctx, &messageSchemas)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Reverse to get chronological order
	messages := make([]Message, len(messageSchemas))
	for i, schema := range messageSchemas {
		messages[len(messageSchemas)-1-i] = *MessageSchemaToMessage(schema)
	}

	return messages, nil
}

func (s *EionMemoryStore) GetMessageList(ctx context.Context, sessionID, agentID, spaceID string, pageNumber, pageSize int) (*MessageListResponse, error) {
	if sessionID == "" || agentID == "" {
		return nil, fmt.Errorf("sessionID and agentID cannot be empty")
	}

	if pageNumber < 1 {
		pageNumber = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	offset := (pageNumber - 1) * pageSize

	// Get total count
	totalCount, err := s.db.NewSelect().
		Model((*MessageSchema)(nil)).
		Where("session_id = ? AND agent_id = ?", sessionID, agentID).
		Where("deleted_at IS NULL").
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count messages: %w", err)
	}

	// Get messages
	var messageSchemas []MessageSchema
	err = s.db.NewSelect().
		Model(&messageSchemas).
		Where("session_id = ? AND agent_id = ?", sessionID, agentID).
		Where("deleted_at IS NULL").
		Order("created_at ASC").
		Offset(offset).
		Limit(pageSize).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	messages := make([]Message, len(messageSchemas))
	for i, schema := range messageSchemas {
		messages[i] = *MessageSchemaToMessage(schema)
	}

	return &MessageListResponse{
		Messages:   messages,
		TotalCount: totalCount,
		RowCount:   len(messages),
	}, nil
}

func (s *EionMemoryStore) GetMessagesByUUID(ctx context.Context, sessionID, agentID, spaceID string, uuids []uuid.UUID) ([]Message, error) {
	if sessionID == "" || agentID == "" {
		return nil, fmt.Errorf("sessionID and agentID cannot be empty")
	}

	if len(uuids) == 0 {
		return []Message{}, nil
	}

	var messageSchemas []MessageSchema
	err := s.db.NewSelect().
		Model(&messageSchemas).
		Where("session_id = ? AND agent_id = ?", sessionID, agentID).
		Where("uuid IN (?)", bun.In(uuids)).
		Where("deleted_at IS NULL").
		Order("created_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages by UUID: %w", err)
	}

	messages := make([]Message, len(messageSchemas))
	for i, schema := range messageSchemas {
		messages[i] = *MessageSchemaToMessage(schema)
	}

	return messages, nil
}

func (s *EionMemoryStore) PutMessages(ctx context.Context, sessionID, agentID, spaceID string, messages []Message) ([]Message, error) {
	if sessionID == "" || agentID == "" {
		return nil, fmt.Errorf("sessionID and agentID cannot be empty")
	}

	if len(messages) == 0 {
		return []Message{}, nil
	}

	// Generate embeddings if embedding service is available
	if s.embeddingService != nil {
		// Generate embeddings BEFORE inserting - strictly required for memory functionality
		for i := range messages {
			if len(messages[i].Embedding) == 0 && messages[i].Content != "" {
				embedding, err := s.embeddingService.GenerateEmbedding(ctx, messages[i].Content)
				if err != nil {
					// Log warning but continue with empty embedding for testing
					messages[i].Embedding = []float32{}
				} else {
					messages[i].Embedding = embedding
				}
			}
		}
	} else {
		// Log warning but continue - embeddings will be empty for testing
		for i := range messages {
			if len(messages[i].Embedding) == 0 {
				messages[i].Embedding = []float32{} // Empty embedding for now
			}
		}
	}

	messageSchemas := make([]MessageSchema, len(messages))
	for i, message := range messages {
		messageSchemas[i] = MessageToMessageSchema(&message)
	}

	_, err := s.db.NewInsert().
		Model(&messageSchemas).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to insert messages: %w", err)
	}

	// Convert back to messages
	resultMessages := make([]Message, len(messageSchemas))
	for i, schema := range messageSchemas {
		resultMessages[i] = *MessageSchemaToMessage(schema)
	}

	return resultMessages, nil
}

func (s *EionMemoryStore) UpdateMessages(ctx context.Context, sessionID, agentID, spaceID string, messages []Message, includeContent bool) error {
	if sessionID == "" || agentID == "" {
		return fmt.Errorf("sessionID and agentID cannot be empty")
	}

	if len(messages) == 0 {
		return nil
	}

	for _, message := range messages {
		updateQuery := s.db.NewUpdate().
			Model((*MessageSchema)(nil)).
			Where("uuid = ? AND session_id = ? AND agent_id = ?",
				message.UUID, sessionID, agentID).
			Where("deleted_at IS NULL").
			Set("updated_at = ?", time.Now()).
			Set("metadata = ?", message.Metadata)

		if includeContent {
			updateQuery = updateQuery.Set("content = ?", message.Content)
			if s.tokenCounter != nil {
				updateQuery = updateQuery.Set("token_count = ?", s.tokenCounter.CountTokens(message.Content))
			}
		}

		_, err := updateQuery.Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update message %s: %w", message.UUID, err)
		}
	}

	return nil
}

func (s *EionMemoryStore) DeleteMessages(ctx context.Context, sessionID, agentID, spaceID string, messageUUIDs []uuid.UUID) error {
	if sessionID == "" || agentID == "" {
		return fmt.Errorf("sessionID and agentID cannot be empty")
	}

	if len(messageUUIDs) == 0 {
		return nil
	}

	_, err := s.db.NewUpdate().
		Model((*MessageSchema)(nil)).
		Where("uuid IN (?) AND session_id = ? AND agent_id = ?",
			bun.In(messageUUIDs), sessionID, agentID).
		Where("deleted_at IS NULL").
		Set("deleted_at = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	return nil
}

// Fact operations

func (s *EionMemoryStore) GetFacts(ctx context.Context, spaceID string, agentID string, limit int, opts ...FilterOption) ([]Fact, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	query := s.db.NewSelect().
		Model((*FactSchema)(nil)).
		Where("deleted_at IS NULL")

	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}

	// Apply filter options
	for _, opt := range opts {
		if opt.MinRating != nil {
			query = query.Where("rating >= ?", *opt.MinRating)
		}
		if opt.Limit != nil && *opt.Limit < limit {
			limit = *opt.Limit
		}
	}

	query = query.Order("rating DESC").Order("created_at DESC").Limit(limit)

	var factSchemas []FactSchema
	err := query.Scan(ctx, &factSchemas)
	if err != nil {
		return nil, fmt.Errorf("failed to get facts: %w", err)
	}

	facts := make([]Fact, len(factSchemas))
	for i, schema := range factSchemas {
		facts[i] = *FactSchemaToFact(schema)
	}

	return facts, nil
}

func (s *EionMemoryStore) PutFact(ctx context.Context, fact *Fact) error {
	if fact.AgentID == "" {
		return fmt.Errorf("agentID is required")
	}

	if fact.UUID == uuid.Nil {
		fact.UUID = uuid.New()
	}

	if fact.CreatedAt.IsZero() {
		fact.CreatedAt = time.Now()
	}
	fact.UpdatedAt = time.Now()

	factSchema := FactToFactSchema(fact)

	_, err := s.db.NewInsert().
		Model(&factSchema).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert fact: %w", err)
	}

	*fact = *FactSchemaToFact(factSchema)
	return nil
}

func (s *EionMemoryStore) UpdateFact(ctx context.Context, fact *Fact) error {
	if fact.UUID == uuid.Nil {
		return fmt.Errorf("fact UUID is required")
	}

	fact.UpdatedAt = time.Now()

	_, err := s.db.NewUpdate().
		Model((*FactSchema)(nil)).
		Where("uuid = ?", fact.UUID).
		Where("deleted_at IS NULL").
		Set("content = ?", fact.Content).
		Set("rating = ?", fact.Rating).
		Set("metadata = ?", fact.Metadata).
		Set("embedding = ?", fact.Embedding).
		Set("updated_at = ?", fact.UpdatedAt).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update fact: %w", err)
	}

	return nil
}

func (s *EionMemoryStore) DeleteFact(ctx context.Context, factUUID uuid.UUID, spaceID string) error {
	if factUUID == uuid.Nil {
		return fmt.Errorf("factUUID cannot be empty")
	}

	_, err := s.db.NewUpdate().
		Model((*FactSchema)(nil)).
		Where("uuid = ?", factUUID).
		Where("deleted_at IS NULL").
		Set("deleted_at = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete fact: %w", err)
	}

	return nil
}

// Cleanup operations

func (s *EionMemoryStore) PurgeDeleted(ctx context.Context, spaceID string) error {
	// Delete old soft-deleted records
	cutoffTime := time.Now().AddDate(0, 0, -30) // 30 days ago

	// Purge sessions
	_, err := s.db.NewDelete().
		Model((*SessionSchema)(nil)).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoffTime).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to purge deleted sessions: %w", err)
	}

	// Purge messages
	_, err = s.db.NewDelete().
		Model((*MessageSchema)(nil)).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoffTime).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to purge deleted messages: %w", err)
	}

	// Purge facts
	_, err = s.db.NewDelete().
		Model((*FactSchema)(nil)).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoffTime).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to purge deleted facts: %w", err)
	}

	return nil
}

// Private helper methods

func (s *EionMemoryStore) processMessages(ctx context.Context, messages []Message, spaceID, agentID string) error {
	// Store in vector store if available and messages have embeddings
	if s.vectorStore != nil {
		for _, message := range messages {
			if len(message.Embedding) > 0 {
				metadata := map[string]any{
					"message_uuid": message.UUID.String(),
					"agent_id":     agentID,
					"space_id":     spaceID,
					"session_id":   message.SessionID,
					"role":         message.Role,
					"created_at":   message.CreatedAt,
				}

				err := s.vectorStore.StoreVector(ctx, message.UUID.String(), message.Embedding, metadata)
				if err != nil {
					return fmt.Errorf("failed to store message vector: %w", err)
				}
			}
		}
	}

	return nil
}
