package agents

import (
	"fmt"
	"time"
)

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	AgentStatusActive    AgentStatus = "active"
	AgentStatusInactive  AgentStatus = "inactive"
	AgentStatusSuspended AgentStatus = "suspended"
)

// IsValid checks if the agent status is valid
func (s AgentStatus) IsValid() bool {
	return s == AgentStatusActive || s == AgentStatusInactive || s == AgentStatusSuspended
}

// Agent represents an AI agent in the system
type Agent struct {
	ID          string      `json:"id" bun:",pk"`
	Name        string      `json:"name"`
	Permission  string      `json:"permission" bun:"permission"`
	Description *string     `json:"description,omitempty"`
	Status      AgentStatus `json:"status"`
	Guest       bool        `json:"guest" bun:"guest,default:false"`
	CreatedAt   time.Time   `json:"created_at" bun:",default:current_timestamp"`
	UpdatedAt   time.Time   `json:"updated_at" bun:",default:current_timestamp"`
}

// RegisterAgentRequest represents a request to register a new agent
type RegisterAgentRequest struct {
	AgentID     string  `json:"agent_id"`
	Name        string  `json:"name"`
	Permission  *string `json:"permission,omitempty"`
	Description *string `json:"description,omitempty"`
	Guest       *bool   `json:"guest,omitempty"`
}

// Validate validates the register agent request
func (r *RegisterAgentRequest) Validate() error {
	if r.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Permission != nil && !isValidPermission(*r.Permission) {
		return fmt.Errorf("permission must be a valid CRUD string (e.g., 'r', 'rw', 'crud')")
	}
	return nil
}

// isValidPermission checks if the permission string is valid
func isValidPermission(permission string) bool {
	if permission == "" {
		return false
	}
	// Check if all characters are valid CRUD characters
	validChars := map[rune]bool{'c': true, 'r': true, 'u': true, 'd': true}
	for _, char := range permission {
		if !validChars[char] {
			return false
		}
	}
	return true
}

// ToAgent converts the request to an Agent
func (r *RegisterAgentRequest) ToAgent() *Agent {
	permission := "r" // Default to read-only
	if r.Permission != nil {
		permission = *r.Permission
	}

	guest := false // Default to internal agent
	if r.Guest != nil {
		guest = *r.Guest
	}

	now := time.Now()
	return &Agent{
		ID:          r.AgentID,
		Name:        r.Name,
		Permission:  permission,
		Description: r.Description,
		Status:      AgentStatusActive,
		Guest:       guest,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ListAgentsRequest represents a request to list agents
type ListAgentsRequest struct {
	Permission *string `json:"permission,omitempty"` // For filtering by permission
	Guest      *bool   `json:"guest,omitempty"`      // For filtering by guest status
}

// UpdateAgentRequest represents a request to update an agent
type UpdateAgentRequest struct {
	AgentID     string  `json:"agent_id"`
	Name        *string `json:"name,omitempty"`
	Permission  *string `json:"permission,omitempty"`
	Description *string `json:"description,omitempty"`
	Guest       *bool   `json:"guest,omitempty"` // Update guest status
}

// Validate validates the update agent request
func (r *UpdateAgentRequest) Validate() error {
	if r.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if r.Permission != nil && !isValidPermission(*r.Permission) {
		return fmt.Errorf("permission must be a valid CRUD string (e.g., 'r', 'rw', 'crud')")
	}
	return nil
}

// DeleteAgentRequest represents a request to delete an agent
type DeleteAgentRequest struct {
	AgentID string `json:"agent_id"`
}

// Validate validates the delete agent request
func (r *DeleteAgentRequest) Validate() error {
	if r.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	return nil
}
