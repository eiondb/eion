package sessions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SessionService implements the SessionManager interface
type SessionService struct {
	store SessionStore
}

// NewSessionService creates a new session service
func NewSessionService(store SessionStore) *SessionService {
	return &SessionService{
		store: store,
	}
}

// NewService creates a new session service (alias for NewSessionService)
func NewService(store SessionStore) *SessionService {
	return NewSessionService(store)
}

// CreateSession creates a new session
func (s *SessionService) CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error) {
	if req.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	if req.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	if req.SessionTypeID == "" {
		return nil, fmt.Errorf("session_type_id is required")
	}

	// Check if session already exists
	existing, err := s.store.GetSession(ctx, req.SessionID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("session with id %s already exists", req.SessionID)
	}

	now := time.Now()
	session := &Session{
		UUID:          uuid.New(),
		SessionID:     req.SessionID,
		UserID:        req.UserID,
		SessionName:   req.SessionName,
		SessionTypeID: req.SessionTypeID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.store.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// DeleteSession deletes a session
func (s *SessionService) DeleteSession(ctx context.Context, req *DeleteSessionRequest) error {
	if req.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}

	// Check if session exists
	existing, err := s.store.GetSession(ctx, req.SessionID)
	if err != nil || existing == nil {
		return fmt.Errorf("session with id %s not found", req.SessionID)
	}

	if err := s.store.DeleteSession(ctx, req.SessionID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}
