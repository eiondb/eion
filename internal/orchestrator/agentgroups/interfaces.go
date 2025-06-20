package agentgroups

import "context"

// AgentGroupStore defines the interface for agent group data persistence
type AgentGroupStore interface {
	CreateAgentGroup(ctx context.Context, group *AgentGroup) error
	GetAgentGroup(ctx context.Context, groupID string) (*AgentGroup, error)
	UpdateAgentGroup(ctx context.Context, group *AgentGroup) error
	DeleteAgentGroup(ctx context.Context, groupID string) error
	ListAgentGroups(ctx context.Context, req *ListAgentGroupRequest) ([]*AgentGroup, error)
	AgentGroupExists(ctx context.Context, groupID string) (bool, error)
}

// AgentGroupManager defines the interface for agent group management operations
type AgentGroupManager interface {
	RegisterAgentGroup(ctx context.Context, req *RegisterAgentGroupRequest) (*AgentGroup, error)
	ListAgentGroups(ctx context.Context, req *ListAgentGroupRequest) ([]*AgentGroup, error)
	GetAgentGroup(ctx context.Context, groupID string) (*AgentGroup, error)
	UpdateAgentGroup(ctx context.Context, groupID string, variable string, newValue interface{}) (*AgentGroup, error)
	UpdateAgentGroupAdvanced(ctx context.Context, req *UpdateAgentGroupRequest) (*AgentGroup, error)
	DeleteAgentGroup(ctx context.Context, req *DeleteAgentGroupRequest) error
	IsAgentInGroup(ctx context.Context, agentID, groupID string) (bool, error)
}
