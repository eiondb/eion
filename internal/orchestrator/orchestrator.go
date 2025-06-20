package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/eion/eion/internal/config"
	"github.com/eion/eion/internal/orchestrator/logging"
	"github.com/eion/eion/internal/zerrors"
)

// Orchestrator manages multi-agent coordination
type Orchestrator struct {
	logger   logging.InteractionLogger
	config   *config.Config
	logStore logging.InteractionStore
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(
	config *config.Config,
	logStore logging.InteractionStore,
) (*Orchestrator, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if logStore == nil {
		return nil, fmt.Errorf("log store cannot be nil")
	}

	// Create logger
	logger := logging.NewInteractionLogger(logStore)

	return &Orchestrator{
		logger:   logger,
		config:   config,
		logStore: logStore,
	}, nil
}

// Note: Agent management methods moved to internal/orchestrator/agents package

// LogAgentInteraction logs an agent's interaction with the system
func (o *Orchestrator) LogAgentInteraction(ctx context.Context, log *logging.AgentInteractionLog) error {
	if err := log.Validate(); err != nil {
		return zerrors.NewValidationError("invalid interaction log", err)
	}

	// Set log ID if not provided
	if log.LogID == "" {
		log.LogID = uuid.New().String()
	}

	// Set timestamp if not provided
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	return o.logger.LogInteraction(ctx, log)
}

// Note: Additional agent methods moved to internal/orchestrator/agents package

// GetAgentInteractions returns interaction logs for a specific agent
func (o *Orchestrator) GetAgentInteractions(ctx context.Context, agentID string, limit int) ([]*logging.AgentInteractionLog, error) {
	if agentID == "" {
		return nil, zerrors.NewValidationError("agent ID is required", nil)
	}
	if limit <= 0 {
		limit = 100 // Default limit
	}

	return o.logger.GetAgentInteractions(ctx, agentID, limit)
}

// GetUserInteractions returns interaction logs for a specific user (replaces GetSpaceInteractions)
func (o *Orchestrator) GetUserInteractions(ctx context.Context, userID string, limit int) ([]*logging.AgentInteractionLog, error) {
	if userID == "" {
		return nil, zerrors.NewValidationError("user ID is required", nil)
	}
	if limit <= 0 {
		limit = 100 // Default limit
	}

	return o.logger.GetUserInteractions(ctx, userID, limit)
}

// GetSessionInteractions returns interaction logs for a specific session
func (o *Orchestrator) GetSessionInteractions(ctx context.Context, sessionID string, limit int) ([]*logging.AgentInteractionLog, error) {
	if sessionID == "" {
		return nil, zerrors.NewValidationError("session ID is required", nil)
	}
	if limit <= 0 {
		limit = 100 // Default limit
	}

	return o.logger.GetSessionInteractions(ctx, sessionID, limit)
}

// MonitorAgent returns comprehensive analytics for an agent across all sessions (SDK function)
func (o *Orchestrator) MonitorAgent(ctx context.Context, agentID string, timeRange logging.TimeRange) (*logging.AgentAnalytics, error) {
	if agentID == "" {
		return nil, zerrors.NewValidationError("agent_id is required and cannot be empty", nil)
	}

	if err := timeRange.Validate(); err != nil {
		return nil, zerrors.NewValidationError("invalid time_range", err)
	}

	// Create analytics service
	analytics := logging.NewAnalyticsService(o.logStore)

	return analytics.GetAgentAnalytics(ctx, agentID, timeRange)
}

// MonitorSession returns comprehensive analytics for a session across all agents (SDK function)
func (o *Orchestrator) MonitorSession(ctx context.Context, sessionID string) (*logging.SessionAnalytics, error) {
	if sessionID == "" {
		return nil, zerrors.NewValidationError("session_id is required and cannot be empty", nil)
	}

	// Create analytics service
	analytics := logging.NewAnalyticsService(o.logStore)

	return analytics.GetSessionAnalytics(ctx, sessionID)
}

// GetCollaborationInsights returns insights about agent collaboration patterns
func (o *Orchestrator) GetCollaborationInsights(ctx context.Context, req *logging.CollaborationInsightsRequest) (*logging.CollaborationInsights, error) {
	if req == nil {
		return nil, zerrors.NewValidationError("request cannot be nil", nil)
	}

	if err := req.Validate(); err != nil {
		return nil, zerrors.NewValidationError("invalid request", err)
	}

	// Create analytics service
	analytics := logging.NewAnalyticsService(o.logStore)

	return analytics.GetCollaborationInsights(ctx, req)
}

// HealthCheck performs a health check of the orchestrator
func (o *Orchestrator) HealthCheck(ctx context.Context) error {
	// Check if log store is accessible
	_, err := o.logStore.GetInteractionsByTimeRange(ctx, time.Now().Add(-1*time.Hour).Format(time.RFC3339), time.Now().Format(time.RFC3339), 1)
	if err != nil {
		return fmt.Errorf("log store health check failed: %w", err)
	}

	return nil
}

// CreateKnowledgeGraphMetadata creates metadata to be attached to KG entries
func (o *Orchestrator) CreateKnowledgeGraphMetadata(agentID string) *KnowledgeGraphMetadata {
	now := time.Now()
	return &KnowledgeGraphMetadata{
		LastModifiedBy: agentID,
		LastModifiedAt: now,
		CreatedBy:      agentID,
		CreatedAt:      now,
	}
}

// UpdateKnowledgeGraphMetadata updates existing metadata for KG modifications
func (o *Orchestrator) UpdateKnowledgeGraphMetadata(existing *KnowledgeGraphMetadata, agentID string) *KnowledgeGraphMetadata {
	if existing == nil {
		return o.CreateKnowledgeGraphMetadata(agentID)
	}

	existing.LastModifiedBy = agentID
	existing.LastModifiedAt = time.Now()
	return existing
}
