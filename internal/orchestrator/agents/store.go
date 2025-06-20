package agents

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// PostgresStore implements AgentStore interface using PostgreSQL
type PostgresStore struct {
	db *bun.DB
}

// NewPostgresStore creates a new PostgreSQL agent store
func NewPostgresStore(db *bun.DB) AgentStore {
	return &PostgresStore{db: db}
}

// CreateAgent persists a new agent to storage
func (s *PostgresStore) CreateAgent(ctx context.Context, agent *Agent) error {
	_, err := s.db.NewInsert().Model(agent).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}
	return nil
}

// GetAgent retrieves an agent by ID from storage
func (s *PostgresStore) GetAgent(ctx context.Context, agentID string) (*Agent, error) {
	agent := &Agent{}
	err := s.db.NewSelect().Model(agent).Where("id = ?", agentID).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent %s not found", agentID)
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	return agent, nil
}

// UpdateAgent updates an existing agent in storage
func (s *PostgresStore) UpdateAgent(ctx context.Context, agent *Agent) error {
	agent.UpdatedAt = time.Now()
	result, err := s.db.NewUpdate().Model(agent).Where("id = ?", agent.ID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("agent %s not found", agent.ID)
	}
	return nil
}

// DeleteAgent removes an agent from storage
func (s *PostgresStore) DeleteAgent(ctx context.Context, agentID string) error {
	result, err := s.db.NewDelete().Model((*Agent)(nil)).Where("id = ?", agentID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("agent %s not found", agentID)
	}
	return nil
}

// ListAgents retrieves agents with optional filters
func (s *PostgresStore) ListAgents(ctx context.Context, req *ListAgentsRequest) ([]*Agent, error) {
	query := s.db.NewSelect().Model((*Agent)(nil))

	// Apply filters
	if req.Permission != nil {
		query = query.Where("permission = ?", *req.Permission)
	}

	if req.Guest != nil {
		query = query.Where("guest = ?", *req.Guest)
	}

	query = query.Order("created_at DESC")

	var agents []*Agent
	err := query.Scan(ctx, &agents)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return agents, nil
}

// AgentExists checks if an agent exists in storage
func (s *PostgresStore) AgentExists(ctx context.Context, agentID string) (bool, error) {
	count, err := s.db.NewSelect().Model((*Agent)(nil)).Where("id = ?", agentID).Count(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check agent existence: %w", err)
	}
	return count > 0, nil
}
