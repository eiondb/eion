package orchestrator

import (
	"encoding/json"
	"fmt"
	"time"
)

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	AgentStatusActive    AgentStatus = "active"
	AgentStatusInactive  AgentStatus = "inactive"
	AgentStatusSuspended AgentStatus = "suspended"
)

// ValidAgentStatuses contains all valid agent status values
var ValidAgentStatuses = map[AgentStatus]bool{
	AgentStatusActive:    true,
	AgentStatusInactive:  true,
	AgentStatusSuspended: true,
}

// IsValid checks if the agent status is valid
func (s AgentStatus) IsValid() bool {
	return ValidAgentStatuses[s]
}

// Note: Agent models moved to internal/orchestrator/agents package

// KnowledgeGraphMetadata represents metadata to be added to KG entries
type KnowledgeGraphMetadata struct {
	LastModifiedBy string    `json:"last_modified_by"`
	LastModifiedAt time.Time `json:"last_modified_at"`
	CreatedBy      string    `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
}

// Custom JSON marshaling for AgentStatus to handle database storage
func (s AgentStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

func (s *AgentStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	status := AgentStatus(str)
	if !status.IsValid() {
		return fmt.Errorf("invalid agent status: %s", str)
	}

	*s = status
	return nil
}
