package logging

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// PostgresInteractionStore implements InteractionStore interface using PostgreSQL
type PostgresInteractionStore struct {
	db *bun.DB
}

// NewPostgresInteractionStore creates a new PostgreSQL interaction store
func NewPostgresInteractionStore(db *bun.DB) InteractionStore {
	return &PostgresInteractionStore{db: db}
}

// CreateInteractionLog persists a new interaction log entry
func (s *PostgresInteractionStore) CreateInteractionLog(ctx context.Context, log *AgentInteractionLog) error {
	_, err := s.db.NewInsert().Model(log).Exec(ctx)
	return err
}

// GetInteractionsByAgent returns interaction logs for a specific agent
func (s *PostgresInteractionStore) GetInteractionsByAgent(ctx context.Context, agentID string, limit int) ([]*AgentInteractionLog, error) {
	var logs []*AgentInteractionLog
	err := s.db.NewSelect().
		Model(&logs).
		Where("agent_id = ?", agentID).
		Order("timestamp DESC").
		Limit(limit).
		Scan(ctx)
	return logs, err
}

// GetInteractionsBySpace returns interaction logs for a specific space
func (s *PostgresInteractionStore) GetInteractionsBySpace(ctx context.Context, spaceID string, limit int) ([]*AgentInteractionLog, error) {
	var logs []*AgentInteractionLog
	err := s.db.NewSelect().
		Model(&logs).
		Where("user_id = ?", spaceID).
		Order("timestamp DESC").
		Limit(limit).
		Scan(ctx)
	return logs, err
}

// GetInteractionsByUser returns interaction logs for a specific user
func (s *PostgresInteractionStore) GetInteractionsByUser(ctx context.Context, userID string, limit int) ([]*AgentInteractionLog, error) {
	var logs []*AgentInteractionLog
	err := s.db.NewSelect().
		Model(&logs).
		Where("user_id = ?", userID).
		Order("timestamp DESC").
		Limit(limit).
		Scan(ctx)
	return logs, err
}

// GetInteractionsByTimeRange returns interaction logs within a time range
func (s *PostgresInteractionStore) GetInteractionsByTimeRange(ctx context.Context, startTime, endTime string, limit int) ([]*AgentInteractionLog, error) {
	start, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, err
	}

	var logs []*AgentInteractionLog
	err = s.db.NewSelect().
		Model(&logs).
		Where("timestamp BETWEEN ? AND ?", start, end).
		Order("timestamp DESC").
		Limit(limit).
		Scan(ctx)
	return logs, err
}

// GetInteractionsBySession returns interaction logs for a specific session
func (s *PostgresInteractionStore) GetInteractionsBySession(ctx context.Context, sessionID string, limit int) ([]*AgentInteractionLog, error) {
	var logs []*AgentInteractionLog
	err := s.db.NewSelect().
		Model(&logs).
		Where("session_id = ?", sessionID).
		Order("timestamp DESC").
		Limit(limit).
		Scan(ctx)
	return logs, err
}

// DeleteOldInteractionLogs removes interaction logs older than specified duration
func (s *PostgresInteractionStore) DeleteOldInteractionLogs(ctx context.Context, olderThan string) error {
	cutoff, err := time.Parse(time.RFC3339, olderThan)
	if err != nil {
		return err
	}

	_, err = s.db.NewDelete().
		Model((*AgentInteractionLog)(nil)).
		Where("timestamp < ?", cutoff).
		Exec(ctx)
	return err
}
