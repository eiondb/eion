package agentgroups

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// PostgresStore implements AgentGroupStore interface using PostgreSQL
type PostgresStore struct {
	db *bun.DB
}

// NewPostgresStore creates a new PostgreSQL agent group store
func NewPostgresStore(db *bun.DB) AgentGroupStore {
	return &PostgresStore{db: db}
}

// CreateAgentGroup persists a new agent group to storage
func (s *PostgresStore) CreateAgentGroup(ctx context.Context, group *AgentGroup) error {
	_, err := s.db.NewInsert().Model(group).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create agent group: %w", err)
	}
	return nil
}

// GetAgentGroup retrieves an agent group by ID from storage
func (s *PostgresStore) GetAgentGroup(ctx context.Context, groupID string) (*AgentGroup, error) {
	group := &AgentGroup{}
	err := s.db.NewSelect().Model(group).Where("id = ?", groupID).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent group %s not found", groupID)
		}
		return nil, fmt.Errorf("failed to get agent group: %w", err)
	}
	return group, nil
}

// UpdateAgentGroup updates an existing agent group in storage
func (s *PostgresStore) UpdateAgentGroup(ctx context.Context, group *AgentGroup) error {
	group.UpdatedAt = time.Now()
	result, err := s.db.NewUpdate().Model(group).Where("id = ?", group.ID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update agent group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("agent group %s not found", group.ID)
	}
	return nil
}

// DeleteAgentGroup removes an agent group from storage
func (s *PostgresStore) DeleteAgentGroup(ctx context.Context, groupID string) error {
	result, err := s.db.NewDelete().Model((*AgentGroup)(nil)).Where("id = ?", groupID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete agent group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("agent group %s not found", groupID)
	}
	return nil
}

// ListAgentGroups returns agent groups based on filter criteria
func (s *PostgresStore) ListAgentGroups(ctx context.Context, req *ListAgentGroupRequest) ([]*AgentGroup, error) {
	query := s.db.NewSelect().Model(&[]*AgentGroup{})

	var groups []*AgentGroup
	err := query.Order("created_at DESC").Scan(ctx, &groups)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent groups: %w", err)
	}
	return groups, nil
}

// AgentGroupExists checks if an agent group exists in storage
func (s *PostgresStore) AgentGroupExists(ctx context.Context, groupID string) (bool, error) {
	count, err := s.db.NewSelect().Model((*AgentGroup)(nil)).Where("id = ?", groupID).Count(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check agent group existence: %w", err)
	}
	return count > 0, nil
}
