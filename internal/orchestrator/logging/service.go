package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/eion/eion/internal/zerrors"
)

// interactionLogger implements the InteractionLogger interface
type interactionLogger struct {
	store InteractionStore
}

// NewInteractionLogger creates a new interaction logger
func NewInteractionLogger(store InteractionStore) InteractionLogger {
	return &interactionLogger{
		store: store,
	}
}

// LogInteraction logs an agent interaction
func (l *interactionLogger) LogInteraction(ctx context.Context, log *AgentInteractionLog) error {
	if err := log.Validate(); err != nil {
		return zerrors.NewValidationError("invalid interaction log", err)
	}

	// Set timestamp if not provided
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	// Create the log entry in storage
	if err := l.store.CreateInteractionLog(ctx, log); err != nil {
		return zerrors.NewInternalError("failed to create interaction log", err)
	}

	return nil
}

// GetAgentInteractions returns interaction logs for a specific agent
func (l *interactionLogger) GetAgentInteractions(ctx context.Context, agentID string, limit int) ([]*AgentInteractionLog, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agent ID cannot be empty")
	}

	if limit <= 0 {
		limit = 100
	}

	logs, err := l.store.GetInteractionsByAgent(ctx, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent interactions: %w", err)
	}

	return logs, nil
}

// GetUserInteractions returns interaction logs for a specific user
func (l *interactionLogger) GetUserInteractions(ctx context.Context, userID string, limit int) ([]*AgentInteractionLog, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	if limit <= 0 {
		limit = 100
	}

	logs, err := l.store.GetInteractionsBySpace(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get user interactions: %w", err)
	}

	return logs, nil
}

// GetSessionInteractions returns interaction logs for a specific session
func (l *interactionLogger) GetSessionInteractions(ctx context.Context, sessionID string, limit int) ([]*AgentInteractionLog, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	if limit <= 0 {
		limit = 100
	}

	logs, err := l.store.GetInteractionsBySession(ctx, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get session interactions: %w", err)
	}

	return logs, nil
}

// GetInteractionsByTimeRange returns interaction logs within a time range
func (l *interactionLogger) GetInteractionsByTimeRange(ctx context.Context, startTime, endTime string, limit int) ([]*AgentInteractionLog, error) {
	if startTime == "" {
		return nil, zerrors.NewValidationError("start time cannot be empty", nil)
	}
	if endTime == "" {
		return nil, zerrors.NewValidationError("end time cannot be empty", nil)
	}
	if limit <= 0 {
		limit = 100 // Default limit
	}

	logs, err := l.store.GetInteractionsByTimeRange(ctx, startTime, endTime, limit)
	if err != nil {
		return nil, zerrors.NewInternalError("failed to get interactions by time range", err)
	}

	return logs, nil
}
