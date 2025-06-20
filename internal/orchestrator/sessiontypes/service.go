package sessiontypes

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SessionTypeService implements SessionTypeManager interface
type SessionTypeService struct {
	store SessionTypeStore
}

// NewSessionTypeService creates a new session type service
func NewSessionTypeService(store SessionTypeStore) *SessionTypeService {
	return &SessionTypeService{
		store: store,
	}
}

// RegisterSessionType creates a new session type
func (s *SessionTypeService) RegisterSessionType(ctx context.Context, req *RegisterSessionTypeRequest) (*SessionType, error) {
	if req.SessionTypeID == "" || req.Name == "" {
		return nil, fmt.Errorf("session type id and name are required")
	}

	// Set default encryption if not provided
	encryption := req.Encryption
	if encryption == "" {
		encryption = "SHA256"
	}

	// Set default agent groups if not provided
	agentGroups := req.AgentGroups
	if agentGroups == nil {
		agentGroups = []string{}
	}

	sessionType := &SessionType{
		ID:          uuid.New(),
		TypeID:      req.SessionTypeID,
		Name:        req.Name,
		Description: req.Description,
		AgentGroups: agentGroups,
		Encryption:  encryption,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.store.CreateSessionType(ctx, sessionType); err != nil {
		return nil, fmt.Errorf("failed to create session type: %w", err)
	}

	return sessionType, nil
}

// ListSessionTypes lists session types with optional agent group filtering
func (s *SessionTypeService) ListSessionTypes(ctx context.Context, agentGroupID *string) ([]*SessionType, error) {
	return s.store.List(ctx, agentGroupID)
}

// GetSessionType retrieves a session type by ID
func (s *SessionTypeService) GetSessionType(ctx context.Context, sessionTypeID string) (*SessionType, error) {
	if sessionTypeID == "" {
		return nil, fmt.Errorf("session type id is required")
	}

	sessionType, err := s.store.GetByTypeID(ctx, sessionTypeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session type: %w", err)
	}

	return sessionType, nil
}

// UpdateSessionType updates a session type with format: UpdateSessionType(sessionTypeID, variable, newValue)
func (s *SessionTypeService) UpdateSessionType(ctx context.Context, sessionTypeID string, variable string, newValue interface{}) (*SessionType, error) {
	if sessionTypeID == "" {
		return nil, fmt.Errorf("session type id is required")
	}

	sessionType, err := s.store.GetByTypeID(ctx, sessionTypeID)
	if err != nil {
		return nil, fmt.Errorf("session type %s not found", sessionTypeID)
	}

	// Update the specified variable
	switch variable {
	case "name":
		if name, ok := newValue.(string); ok && name != "" {
			sessionType.Name = name
		} else {
			return nil, fmt.Errorf("invalid name value")
		}
	case "description":
		if desc, ok := newValue.(string); ok {
			sessionType.Description = &desc
		} else if newValue == nil {
			sessionType.Description = nil
		} else {
			return nil, fmt.Errorf("invalid description value")
		}
	case "agent_groups":
		if agentGroups, ok := newValue.([]string); ok {
			sessionType.AgentGroups = agentGroups
		} else {
			return nil, fmt.Errorf("invalid agent_groups value")
		}
	case "encryption":
		if encryption, ok := newValue.(string); ok && encryption != "" {
			sessionType.Encryption = encryption
		} else {
			return nil, fmt.Errorf("invalid encryption value")
		}
	default:
		return nil, fmt.Errorf("unknown variable: %s", variable)
	}

	sessionType.UpdatedAt = time.Now()

	updates := make(map[string]interface{})
	updates["updated_at"] = sessionType.UpdatedAt

	switch variable {
	case "name":
		updates["name"] = sessionType.Name
	case "description":
		updates["description"] = sessionType.Description
	case "agent_groups":
		updates["agent_groups"] = sessionType.AgentGroups
	case "encryption":
		updates["encryption"] = sessionType.Encryption
	}

	return s.store.Update(ctx, sessionTypeID, updates)
}

// DeleteSessionType deletes a session type
func (s *SessionTypeService) DeleteSessionType(ctx context.Context, sessionTypeID string) error {
	if sessionTypeID == "" {
		return fmt.Errorf("session type id is required")
	}

	return s.store.Delete(ctx, sessionTypeID)
}
