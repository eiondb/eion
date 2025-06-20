package sessiontypes

import "context"

// SessionTypeManager defines the interface for session type operations
type SessionTypeManager interface {
	RegisterSessionType(ctx context.Context, req *RegisterSessionTypeRequest) (*SessionType, error)
	ListSessionTypes(ctx context.Context, agentGroupID *string) ([]*SessionType, error)
	GetSessionType(ctx context.Context, sessionTypeID string) (*SessionType, error)
	UpdateSessionType(ctx context.Context, sessionTypeID string, variable string, newValue interface{}) (*SessionType, error)
	DeleteSessionType(ctx context.Context, sessionTypeID string) error
}

// SessionTypeStore defines the interface for session type storage operations
type SessionTypeStore interface {
	CreateSessionType(ctx context.Context, sessionType *SessionType) error
	GetByTypeID(ctx context.Context, sessionTypeID string) (*SessionType, error)
	List(ctx context.Context, agentGroupID *string) ([]*SessionType, error)
	Update(ctx context.Context, sessionTypeID string, updates map[string]interface{}) (*SessionType, error)
	Delete(ctx context.Context, sessionTypeID string) error
}
