package sessions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// PostgresStore implements SessionStore interface with PostgreSQL storage
type PostgresStore struct {
	db *bun.DB
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(db *bun.DB) *PostgresStore {
	return &PostgresStore{
		db: db,
	}
}

// SessionSchema represents the sessions table schema
type SessionSchema struct {
	bun.BaseModel `bun:"table:sessions,alias:s"`

	UUID          string     `bun:"uuid,pk,type:uuid,default:gen_random_uuid()" json:"uuid"`
	SessionID     string     `bun:"session_id,notnull,unique" json:"session_id"`
	UserID        string     `bun:"user_id,notnull" json:"user_id"`
	SessionName   *string    `bun:"session_name,nullzero" json:"session_name,omitempty"`
	SessionTypeID string     `bun:"session_type_id,notnull" json:"session_type_id"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	DeletedAt     *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
}

// CreateSession creates a new session
func (s *PostgresStore) CreateSession(ctx context.Context, session *Session) error {
	schema := &SessionSchema{
		UUID:          session.UUID.String(),
		SessionID:     session.SessionID,
		UserID:        session.UserID,
		SessionName:   session.SessionName,
		SessionTypeID: session.SessionTypeID,
		CreatedAt:     session.CreatedAt,
		UpdatedAt:     session.UpdatedAt,
	}

	_, err := s.db.NewInsert().
		Model(schema).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// DeleteSession soft-deletes a session by setting deleted_at timestamp
func (s *PostgresStore) DeleteSession(ctx context.Context, sessionID string) error {
	now := time.Now()

	result, err := s.db.NewUpdate().
		Model((*SessionSchema)(nil)).
		Where("session_id = ?", sessionID).
		Where("deleted_at IS NULL").
		Set("deleted_at = ?", now).
		Set("updated_at = ?", now).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session with id %s not found or already deleted", sessionID)
	}

	// TODO: Implement KG cleanup logic for deleted sessions
	// This should remove knowledge graph data tied to this session
	// while preserving interaction logs for audit purposes

	return nil
}

// GetSession retrieves a session by ID (active sessions only)
func (s *PostgresStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	var schema SessionSchema
	err := s.db.NewSelect().
		Model(&schema).
		Where("session_id = ?", sessionID).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session with id %s not found or deleted", sessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return schemaToSession(schema), nil
}

// schemaToSession converts database schema to session model
func schemaToSession(schema SessionSchema) *Session {
	uuid, _ := uuid.Parse(schema.UUID)
	return &Session{
		UUID:          uuid,
		SessionID:     schema.SessionID,
		UserID:        schema.UserID,
		SessionName:   schema.SessionName,
		SessionTypeID: schema.SessionTypeID,
		CreatedAt:     schema.CreatedAt,
		UpdatedAt:     schema.UpdatedAt,
	}
}
