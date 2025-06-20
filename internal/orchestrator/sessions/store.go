package sessions

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryStore implements SessionStore interface with in-memory storage
type InMemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new session
func (s *InMemoryStore) CreateSession(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[session.SessionID]; exists {
		return fmt.Errorf("session with id %s already exists", session.SessionID)
	}

	s.sessions[session.SessionID] = session
	return nil
}

// DeleteSession removes a session
func (s *InMemoryStore) DeleteSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[sessionID]; !exists {
		return fmt.Errorf("session with id %s not found", sessionID)
	}

	delete(s.sessions, sessionID)
	return nil
}

// GetSession retrieves a session by ID
func (s *InMemoryStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session with id %s not found", sessionID)
	}

	return session, nil
}
