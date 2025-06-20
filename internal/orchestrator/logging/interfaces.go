package logging

import (
	"context"
)

// InteractionLogger defines the interface for logging agent interactions
type InteractionLogger interface {
	// LogInteraction logs an agent interaction
	LogInteraction(ctx context.Context, log *AgentInteractionLog) error

	// GetAgentInteractions returns interaction logs for a specific agent
	GetAgentInteractions(ctx context.Context, agentID string, limit int) ([]*AgentInteractionLog, error)

	// GetUserInteractions returns interaction logs for a specific user
	GetUserInteractions(ctx context.Context, userID string, limit int) ([]*AgentInteractionLog, error)

	// GetSessionInteractions returns interaction logs for a specific session
	GetSessionInteractions(ctx context.Context, sessionID string, limit int) ([]*AgentInteractionLog, error)

	// GetInteractionsByTimeRange returns interaction logs within a time range
	GetInteractionsByTimeRange(ctx context.Context, startTime, endTime string, limit int) ([]*AgentInteractionLog, error)
}

// InteractionStore defines the interface for interaction log persistence
type InteractionStore interface {
	// CreateInteractionLog persists a new interaction log entry
	CreateInteractionLog(ctx context.Context, log *AgentInteractionLog) error

	// GetInteractionsByAgent returns interaction logs for a specific agent
	GetInteractionsByAgent(ctx context.Context, agentID string, limit int) ([]*AgentInteractionLog, error)

	// GetInteractionsByUser returns interaction logs for a specific user
	GetInteractionsByUser(ctx context.Context, userID string, limit int) ([]*AgentInteractionLog, error)

	// GetInteractionsBySpace returns interaction logs for a specific space
	GetInteractionsBySpace(ctx context.Context, spaceID string, limit int) ([]*AgentInteractionLog, error)

	// GetInteractionsBySession returns interaction logs for a specific session
	GetInteractionsBySession(ctx context.Context, sessionID string, limit int) ([]*AgentInteractionLog, error)

	// GetInteractionsByTimeRange returns interaction logs within a time range
	GetInteractionsByTimeRange(ctx context.Context, startTime, endTime string, limit int) ([]*AgentInteractionLog, error)

	// DeleteOldInteractionLogs removes interaction logs older than specified duration
	DeleteOldInteractionLogs(ctx context.Context, olderThan string) error
}

// MonitoringAnalytics defines the interface for monitoring insights
type MonitoringAnalytics interface {
	// GetAgentAnalytics returns comprehensive analytics for an agent across all sessions
	GetAgentAnalytics(ctx context.Context, agentID string, timeRange TimeRange) (*AgentAnalytics, error)

	// GetSessionAnalytics returns comprehensive analytics for a session across all agents
	GetSessionAnalytics(ctx context.Context, sessionID string) (*SessionAnalytics, error)

	// GetCollaborationInsights returns insights about agent collaboration patterns
	GetCollaborationInsights(ctx context.Context, req *CollaborationInsightsRequest) (*CollaborationInsights, error)
}
