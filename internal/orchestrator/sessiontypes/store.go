package sessiontypes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// InMemoryStore implements SessionTypeStore interface with in-memory storage
type InMemoryStore struct {
	mu           sync.RWMutex
	sessionTypes map[string]*SessionType
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		sessionTypes: make(map[string]*SessionType),
	}
}

// PostgresStore implements SessionTypeStore interface with PostgreSQL storage
type PostgresStore struct {
	db *bun.DB
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(db *bun.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// SessionTypeSchema represents the session_types table schema
type SessionTypeSchema struct {
	bun.BaseModel `bun:"table:session_types,alias:st"`

	ID          string    `bun:"id,pk" json:"id"`
	Name        string    `bun:"name,notnull" json:"name"`
	AgentGroups []string  `bun:"agent_group_ids,type:jsonb" json:"agent_groups"`
	Description *string   `bun:"description" json:"description,omitempty"`
	Encryption  string    `bun:"encryption,notnull,default:'SHA256'" json:"encryption"`
	CreatedAt   time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt   time.Time `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`
}

// schemaToSessionType converts database schema to session type model
func schemaToSessionType(schema SessionTypeSchema) *SessionType {
	return &SessionType{
		ID:          uuid.New(), // Generate a new UUID for the ID field
		TypeID:      schema.ID,
		Name:        schema.Name,
		Description: schema.Description,
		AgentGroups: schema.AgentGroups,
		Encryption:  schema.Encryption,
		CreatedAt:   schema.CreatedAt,
		UpdatedAt:   schema.UpdatedAt,
	}
}

// sessionTypeToSchema converts session type model to database schema
func sessionTypeToSchema(sessionType *SessionType) SessionTypeSchema {
	return SessionTypeSchema{
		ID:          sessionType.TypeID,
		Name:        sessionType.Name,
		AgentGroups: sessionType.AgentGroups,
		Description: sessionType.Description,
		Encryption:  sessionType.Encryption,
		CreatedAt:   sessionType.CreatedAt,
		UpdatedAt:   sessionType.UpdatedAt,
	}
}

// CreateSessionType creates a new session type in PostgreSQL
func (s *PostgresStore) CreateSessionType(ctx context.Context, sessionType *SessionType) error {
	schema := sessionTypeToSchema(sessionType)

	_, err := s.db.NewInsert().
		Model(&schema).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create session type: %w", err)
	}

	return nil
}

// GetByTypeID retrieves a session type by type ID from PostgreSQL
func (s *PostgresStore) GetByTypeID(ctx context.Context, sessionTypeID string) (*SessionType, error) {
	var schema SessionTypeSchema
	err := s.db.NewSelect().
		Model(&schema).
		Where("id = ?", sessionTypeID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("session type with id %s not found: %w", sessionTypeID, err)
	}

	return schemaToSessionType(schema), nil
}

// List returns all session types from PostgreSQL, optionally filtered by agent group
func (s *PostgresStore) List(ctx context.Context, agentGroupID *string) ([]*SessionType, error) {
	query := s.db.NewSelect().Model(&[]SessionTypeSchema{})

	if agentGroupID != nil {
		query = query.Where("agent_group_ids ? ?", *agentGroupID)
	}

	var schemas []SessionTypeSchema
	err := query.Order("created_at DESC").Scan(ctx, &schemas)
	if err != nil {
		return nil, fmt.Errorf("failed to list session types: %w", err)
	}

	var result []*SessionType
	for _, schema := range schemas {
		result = append(result, schemaToSessionType(schema))
	}

	return result, nil
}

// Update updates a session type in PostgreSQL
func (s *PostgresStore) Update(ctx context.Context, sessionTypeID string, updates map[string]interface{}) (*SessionType, error) {
	// Build the update query
	query := s.db.NewUpdate().
		Model((*SessionTypeSchema)(nil)).
		Where("id = ?", sessionTypeID).
		Set("updated_at = ?", time.Now())

	for key, value := range updates {
		switch key {
		case "name":
			query = query.Set("name = ?", value)
		case "description":
			query = query.Set("description = ?", value)
		case "agent_groups":
			query = query.Set("agent_group_ids = ?", value)
		case "encryption":
			query = query.Set("encryption = ?", value)
		}
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update session type: %w", err)
	}

	// Return the updated session type
	return s.GetByTypeID(ctx, sessionTypeID)
}

// Delete removes a session type from PostgreSQL
func (s *PostgresStore) Delete(ctx context.Context, sessionTypeID string) error {
	result, err := s.db.NewDelete().
		Model((*SessionTypeSchema)(nil)).
		Where("id = ?", sessionTypeID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete session type: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session type with id %s not found", sessionTypeID)
	}

	return nil
}

// === In-Memory Store Implementation (keeping for backward compatibility) ===

// CreateSessionType creates a new session type
func (s *InMemoryStore) CreateSessionType(ctx context.Context, sessionType *SessionType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessionTypes[sessionType.TypeID]; exists {
		return fmt.Errorf("session type with id %s already exists", sessionType.TypeID)
	}

	s.sessionTypes[sessionType.TypeID] = sessionType
	return nil
}

// GetByTypeID retrieves a session type by type ID
func (s *InMemoryStore) GetByTypeID(ctx context.Context, sessionTypeID string) (*SessionType, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionType, exists := s.sessionTypes[sessionTypeID]
	if !exists {
		return nil, fmt.Errorf("session type with id %s not found", sessionTypeID)
	}

	return sessionType, nil
}

// List returns all session types, optionally filtered by agent group
func (s *InMemoryStore) List(ctx context.Context, agentGroupID *string) ([]*SessionType, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SessionType
	for _, sessionType := range s.sessionTypes {
		if agentGroupID == nil {
			result = append(result, sessionType)
		} else {
			// Check if the session type is associated with the agent group
			for _, groupID := range sessionType.AgentGroups {
				if groupID == *agentGroupID {
					result = append(result, sessionType)
					break
				}
			}
		}
	}

	return result, nil
}

// Update updates a session type
func (s *InMemoryStore) Update(ctx context.Context, sessionTypeID string, updates map[string]interface{}) (*SessionType, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionType, exists := s.sessionTypes[sessionTypeID]
	if !exists {
		return nil, fmt.Errorf("session type with id %s not found", sessionTypeID)
	}

	// Create a copy to avoid modifying the original
	updated := *sessionType

	for key, value := range updates {
		switch key {
		case "name":
			if name, ok := value.(string); ok {
				updated.Name = name
			}
		case "description":
			if desc, ok := value.(*string); ok {
				updated.Description = desc
			}
		case "agent_groups":
			if groups, ok := value.([]string); ok {
				updated.AgentGroups = groups
			}
		case "encryption":
			if enc, ok := value.(string); ok {
				updated.Encryption = enc
			}
		case "updated_at":
			if updatedAt, ok := value.(interface{}); ok {
				if t, ok := updatedAt.(interface{ Time() interface{} }); ok {
					// Handle time properly
					_ = t
				}
			}
		}
	}

	s.sessionTypes[sessionTypeID] = &updated
	return &updated, nil
}

// Delete removes a session type
func (s *InMemoryStore) Delete(ctx context.Context, sessionTypeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessionTypes[sessionTypeID]; !exists {
		return fmt.Errorf("session type with id %s not found", sessionTypeID)
	}

	delete(s.sessionTypes, sessionTypeID)
	return nil
}
