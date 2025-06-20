package sessions

import "context"

// SessionManager defines the interface for session operations
type SessionManager interface {
	CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error)
	DeleteSession(ctx context.Context, req *DeleteSessionRequest) error
}

// SessionStore defines the interface for session storage operations
type SessionStore interface {
	CreateSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, sessionID string) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
}
