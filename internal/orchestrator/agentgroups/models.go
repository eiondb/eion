package agentgroups

import (
	"fmt"
	"time"
)

// AgentGroup represents an agent group in the system
type AgentGroup struct {
	ID          string    `json:"id" bun:",pk"`
	Name        string    `json:"name"`
	AgentIDs    []string  `json:"agent_ids" bun:"agent_ids,type:jsonb,default:'[]'::jsonb"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at" bun:",default:current_timestamp"`
	UpdatedAt   time.Time `json:"updated_at" bun:",default:current_timestamp"`
}

// RegisterAgentGroupRequest - agent group id, agent group name, agent ids list, agent group desc=None
type RegisterAgentGroupRequest struct {
	AgentGroupID string   `json:"agent_group_id"`
	Name         string   `json:"name"`
	AgentIDs     []string `json:"agent_ids,omitempty"`
	Description  *string  `json:"description,omitempty"`
}

func (r *RegisterAgentGroupRequest) Validate() error {
	if r.AgentGroupID == "" {
		return fmt.Errorf("agent_group_id is required")
	}
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	// AgentIDs can be empty initially - no validation needed
	return nil
}

func (r *RegisterAgentGroupRequest) ToAgentGroup() *AgentGroup {
	agentIDs := []string{}
	if r.AgentIDs != nil {
		agentIDs = r.AgentIDs
	}

	now := time.Now()
	return &AgentGroup{
		ID:          r.AgentGroupID,
		Name:        r.Name,
		AgentIDs:    agentIDs,
		Description: r.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ListAgentGroupRequest - no permission level filter anymore
type ListAgentGroupRequest struct {
	// No filtering by permission level since agent groups don't have permissions
}

// GetAgentGroupRequest - agent group id=None
type GetAgentGroupRequest struct {
	AgentGroupID *string `json:"agent_group_id,omitempty"`
}

// UpdateAgentGroupRequest - agent group id (identifier), agent group name=None, agent ids=None, agent group desc=None
type UpdateAgentGroupRequest struct {
	AgentGroupID   string   `json:"agent_group_id"`
	Name           *string  `json:"name,omitempty"`
	AgentIDs       []string `json:"agent_ids,omitempty"`
	AddAgentIDs    []string `json:"add_agent_ids,omitempty"`    // Add specific agents
	RemoveAgentIDs []string `json:"remove_agent_ids,omitempty"` // Remove specific agents
	Description    *string  `json:"description,omitempty"`
}

func (r *UpdateAgentGroupRequest) Validate() error {
	if r.AgentGroupID == "" {
		return fmt.Errorf("agent_group_id is required")
	}

	// Validate that we don't have conflicting agent operations
	if len(r.AgentIDs) > 0 && (len(r.AddAgentIDs) > 0 || len(r.RemoveAgentIDs) > 0) {
		return fmt.Errorf("cannot use agent_ids with add_agent_ids or remove_agent_ids - use one approach")
	}

	return nil
}

// DeleteAgentGroupRequest - agent group id
type DeleteAgentGroupRequest struct {
	AgentGroupID string `json:"agent_group_id"`
}

func (r *DeleteAgentGroupRequest) Validate() error {
	if r.AgentGroupID == "" {
		return fmt.Errorf("agent_group_id is required")
	}
	return nil
}
