package sessiontypes

import (
	"time"

	"github.com/google/uuid"
)

// SessionType represents a session type in the system
type SessionType struct {
	ID          uuid.UUID `json:"id"`
	TypeID      string    `json:"session_type_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	AgentGroups []string  `json:"agent_groups,omitempty"`
	Encryption  string    `json:"encryption"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RegisterSessionTypeRequest represents the request to register a new session type
type RegisterSessionTypeRequest struct {
	SessionTypeID string   `json:"session_type_id" binding:"required"`
	Name          string   `json:"name" binding:"required"`
	Description   *string  `json:"description,omitempty"`
	AgentGroups   []string `json:"agent_groups,omitempty"`
	Encryption    string   `json:"encryption,omitempty"`
}

// UpdateSessionTypeRequest represents the request to update a session type
type UpdateSessionTypeRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	AgentGroups []string `json:"agent_groups,omitempty"`
	Encryption  *string  `json:"encryption,omitempty"`
}

// ListSessionTypesRequest represents the request to list session types
type ListSessionTypesRequest struct {
	AgentGroupID *string `json:"agent_group_id,omitempty"`
}
