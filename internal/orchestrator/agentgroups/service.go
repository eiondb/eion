package agentgroups

import (
	"context"
	"fmt"
	"time"
)

// Service implements the AgentGroupManager interface
type Service struct {
	store AgentGroupStore
}

// NewService creates a new agent group service
func NewService(store AgentGroupStore) AgentGroupManager {
	return &Service{store: store}
}

// RegisterAgentGroup registers a new agent group: agent group id, agent group name, agent ids list, agent group desc=None
func (s *Service) RegisterAgentGroup(ctx context.Context, req *RegisterAgentGroupRequest) (*AgentGroup, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	exists, err := s.store.AgentGroupExists(ctx, req.AgentGroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to check agent group existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("agent group %s already exists", req.AgentGroupID)
	}

	group := req.ToAgentGroup()
	if err := s.store.CreateAgentGroup(ctx, group); err != nil {
		return nil, fmt.Errorf("failed to create agent group: %w", err)
	}

	return group, nil
}

// ListAgentGroups lists agent groups: no permission level filter
func (s *Service) ListAgentGroups(ctx context.Context, req *ListAgentGroupRequest) ([]*AgentGroup, error) {
	if req == nil {
		req = &ListAgentGroupRequest{}
	}
	return s.store.ListAgentGroups(ctx, req)
}

// GetAgentGroup gets an agent group: agent group id=None (but in practice we need the ID)
func (s *Service) GetAgentGroup(ctx context.Context, groupID string) (*AgentGroup, error) {
	if groupID == "" {
		return nil, fmt.Errorf("agent group ID is required")
	}
	return s.store.GetAgentGroup(ctx, groupID)
}

// UpdateAgentGroup updates an agent group with format: UpdateAgentGroup(groupID, variable, newValue)
func (s *Service) UpdateAgentGroup(ctx context.Context, groupID string, variable string, newValue interface{}) (*AgentGroup, error) {
	if groupID == "" {
		return nil, fmt.Errorf("agent group ID is required")
	}

	group, err := s.store.GetAgentGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("agent group %s not found", groupID)
	}

	// Update the specified variable
	switch variable {
	case "name":
		if name, ok := newValue.(string); ok && name != "" {
			group.Name = name
		} else {
			return nil, fmt.Errorf("invalid name value")
		}
	case "agent_ids":
		if agentIDs, ok := newValue.([]string); ok {
			group.AgentIDs = agentIDs
		} else if agentIDsInterface, ok := newValue.([]interface{}); ok {
			// Handle JSON unmarshaling where []string becomes []interface{}
			agentIDs := make([]string, len(agentIDsInterface))
			for i, v := range agentIDsInterface {
				if str, ok := v.(string); ok {
					agentIDs[i] = str
				} else {
					return nil, fmt.Errorf("invalid agent_ids value - all elements must be strings")
				}
			}
			group.AgentIDs = agentIDs
		} else {
			return nil, fmt.Errorf("invalid agent_ids value - must be array of strings")
		}
	case "description":
		if desc, ok := newValue.(string); ok {
			group.Description = &desc
		} else if newValue == nil {
			group.Description = nil
		} else {
			return nil, fmt.Errorf("invalid description value")
		}
	default:
		return nil, fmt.Errorf("unknown variable: %s", variable)
	}

	group.UpdatedAt = time.Now()

	if err := s.store.UpdateAgentGroup(ctx, group); err != nil {
		return nil, fmt.Errorf("failed to update agent group: %w", err)
	}

	return group, nil
}

// UpdateAgentGroupAdvanced handles the new structured update requests with add/remove operations
func (s *Service) UpdateAgentGroupAdvanced(ctx context.Context, req *UpdateAgentGroupRequest) (*AgentGroup, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	group, err := s.store.GetAgentGroup(ctx, req.AgentGroupID)
	if err != nil {
		return nil, fmt.Errorf("agent group %s not found", req.AgentGroupID)
	}

	// Update name if provided
	if req.Name != nil {
		group.Name = *req.Name
	}

	// Update description if provided
	if req.Description != nil {
		group.Description = req.Description
	}

	// Handle agent ID operations
	if len(req.AgentIDs) > 0 {
		// Replace entire list
		group.AgentIDs = req.AgentIDs
	} else {
		// Add specific agents
		if len(req.AddAgentIDs) > 0 {
			agentIDSet := make(map[string]bool)
			// Add existing agents to set
			for _, id := range group.AgentIDs {
				agentIDSet[id] = true
			}
			// Add new agents
			for _, id := range req.AddAgentIDs {
				agentIDSet[id] = true
			}
			// Convert back to slice
			newAgentIDs := make([]string, 0, len(agentIDSet))
			for id := range agentIDSet {
				newAgentIDs = append(newAgentIDs, id)
			}
			group.AgentIDs = newAgentIDs
		}

		// Remove specific agents
		if len(req.RemoveAgentIDs) > 0 {
			removeSet := make(map[string]bool)
			for _, id := range req.RemoveAgentIDs {
				removeSet[id] = true
			}

			filteredAgentIDs := make([]string, 0, len(group.AgentIDs))
			for _, id := range group.AgentIDs {
				if !removeSet[id] {
					filteredAgentIDs = append(filteredAgentIDs, id)
				}
			}
			group.AgentIDs = filteredAgentIDs
		}
	}

	group.UpdatedAt = time.Now()

	if err := s.store.UpdateAgentGroup(ctx, group); err != nil {
		return nil, fmt.Errorf("failed to update agent group: %w", err)
	}

	return group, nil
}

// DeleteAgentGroup deletes an agent group: agent group id
func (s *Service) DeleteAgentGroup(ctx context.Context, req *DeleteAgentGroupRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	exists, err := s.store.AgentGroupExists(ctx, req.AgentGroupID)
	if err != nil {
		return fmt.Errorf("failed to check agent group existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("agent group %s not found", req.AgentGroupID)
	}

	if err := s.store.DeleteAgentGroup(ctx, req.AgentGroupID); err != nil {
		return fmt.Errorf("failed to delete agent group: %w", err)
	}

	return nil
}

// IsAgentInGroup checks if an agent belongs to a specific group
func (s *Service) IsAgentInGroup(ctx context.Context, agentID, groupID string) (bool, error) {
	if agentID == "" || groupID == "" {
		return false, fmt.Errorf("agent ID and group ID are required")
	}

	group, err := s.store.GetAgentGroup(ctx, groupID)
	if err != nil {
		return false, fmt.Errorf("failed to get agent group: %w", err)
	}

	for _, id := range group.AgentIDs {
		if id == agentID {
			return true, nil
		}
	}

	return false, nil
}
