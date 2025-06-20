package agents

import "context"

// AgentStore defines the interface for agent data persistence
type AgentStore interface {
	CreateAgent(ctx context.Context, agent *Agent) error
	GetAgent(ctx context.Context, agentID string) (*Agent, error)
	UpdateAgent(ctx context.Context, agent *Agent) error
	DeleteAgent(ctx context.Context, agentID string) error
	ListAgents(ctx context.Context, req *ListAgentsRequest) ([]*Agent, error)
	AgentExists(ctx context.Context, agentID string) (bool, error)
}

// AgentManager defines the interface for agent management operations
type AgentManager interface {
	RegisterAgent(ctx context.Context, req *RegisterAgentRequest) (*Agent, error)
	ListAgents(ctx context.Context, req *ListAgentsRequest) ([]*Agent, error)
	GetAgent(ctx context.Context, agentID string) (*Agent, error)
	UpdateAgent(ctx context.Context, agentID string, variable string, newValue interface{}) (*Agent, error)
	DeleteAgent(ctx context.Context, req *DeleteAgentRequest) error
}
