package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/eion/eion/internal/orchestrator/agentgroups"
)

// Service implements the AgentManager interface
type Service struct {
	store           AgentStore
	agentGroupStore agentgroups.AgentGroupStore
}

// NewService creates a new agent service
func NewService(store AgentStore) AgentManager {
	return &Service{store: store}
}

// NewServiceWithAgentGroups creates a new agent service with agent group validation
func NewServiceWithAgentGroups(store AgentStore, agentGroupStore agentgroups.AgentGroupStore) AgentManager {
	return &Service{
		store:           store,
		agentGroupStore: agentGroupStore,
	}
}

// RegisterAgent registers a new agent with required parameters: agent id, agent name, permission level=1, agent desc=None
func (s *Service) RegisterAgent(ctx context.Context, req *RegisterAgentRequest) (*Agent, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if agent already exists
	exists, err := s.store.AgentExists(ctx, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if agent exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("agent %s already exists", req.AgentID)
	}

	agent := req.ToAgent()

	// Store the agent
	if err := s.store.CreateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return agent, nil
}

// ListAgents lists agents with optional filters: permission level=None
func (s *Service) ListAgents(ctx context.Context, req *ListAgentsRequest) ([]*Agent, error) {
	return s.store.ListAgents(ctx, req)
}

// UpdateAgent updates an agent by setting the specified variable to the new value
func (s *Service) UpdateAgent(ctx context.Context, agentID string, variable string, newValue interface{}) (*Agent, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}

	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	// Update based on variable type
	switch variable {
	case "name":
		if name, ok := newValue.(string); ok {
			agent.Name = name
		} else {
			return nil, fmt.Errorf("name must be a string")
		}
	case "permission":
		if permission, ok := newValue.(string); ok {
			agent.Permission = permission
		} else {
			return nil, fmt.Errorf("permission must be a string")
		}
	case "description":
		if desc, ok := newValue.(*string); ok {
			agent.Description = desc
		} else if desc, ok := newValue.(string); ok {
			agent.Description = &desc
		} else {
			return nil, fmt.Errorf("description must be a string or *string")
		}
	case "guest":
		if guest, ok := newValue.(bool); ok {
			agent.Guest = guest
		} else {
			return nil, fmt.Errorf("guest must be a boolean")
		}
	default:
		return nil, fmt.Errorf("unknown variable: %s", variable)
	}

	// Update timestamp
	agent.UpdatedAt = time.Now()

	// Save the updated agent
	if err := s.store.UpdateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	return agent, nil
}

// DeleteAgent deletes an agent with required parameter: agent id
func (s *Service) DeleteAgent(ctx context.Context, req *DeleteAgentRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	// Check if agent exists
	exists, err := s.store.AgentExists(ctx, req.AgentID)
	if err != nil {
		return fmt.Errorf("failed to check agent existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("agent %s not found", req.AgentID)
	}

	// Delete the agent
	if err := s.store.DeleteAgent(ctx, req.AgentID); err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	return nil
}

// GetAgent retrieves an agent by ID
func (s *Service) GetAgent(ctx context.Context, agentID string) (*Agent, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}

	return s.store.GetAgent(ctx, agentID)
}

// IsGuestAgent checks if an agent is a guest agent
func (s *Service) IsGuestAgent(ctx context.Context, agentID string) (bool, error) {
	if agentID == "" {
		return false, fmt.Errorf("agent ID cannot be empty")
	}

	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return false, fmt.Errorf("failed to get agent: %w", err)
	}

	return agent.Guest, nil
}

// ListGuestAgents returns all guest agents
func (s *Service) ListGuestAgents(ctx context.Context) ([]*Agent, error) {
	guest := true
	req := &ListAgentsRequest{
		Guest: &guest,
	}

	return s.store.ListAgents(ctx, req)
}

// ListInternalAgents returns all internal (non-guest) agents
func (s *Service) ListInternalAgents(ctx context.Context) ([]*Agent, error) {
	guest := false
	req := &ListAgentsRequest{
		Guest: &guest,
	}

	return s.store.ListAgents(ctx, req)
}
