package logging

import (
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// AgentInteractionLog represents an audit log entry for agent actions
type AgentInteractionLog struct {
	bun.BaseModel `bun:"table:agent_interaction_logs,alias:ail"`

	LogID       string                 `bun:"id,pk" json:"log_id" db:"log_id"`
	AgentID     string                 `bun:"agent_id,notnull" json:"agent_id" db:"agent_id"`
	UserID      string                 `bun:"user_id,notnull" json:"user_id" db:"user_id"`
	Operation   string                 `bun:"operation,notnull" json:"operation" db:"operation"` // e.g., "create_session", "put_memory"
	Endpoint    string                 `bun:"endpoint" json:"endpoint" db:"endpoint"`            // e.g., "/api/v1/sessions/{id}/memory"
	Method      string                 `bun:"method" json:"method" db:"method"`                  // GET, POST, PUT, DELETE
	SessionID   string                 `bun:"session_id" json:"session_id,omitempty" db:"session_id"`
	Success     bool                   `bun:"success,notnull,default:true" json:"success" db:"success"`
	ErrorMsg    string                 `bun:"error_msg" json:"error_msg,omitempty" db:"error_msg"`
	Timestamp   time.Time              `bun:"timestamp,notnull,default:current_timestamp" json:"timestamp" db:"timestamp"`
	RequestData map[string]interface{} `bun:"request_data,type:jsonb" json:"request_data,omitempty" db:"request_data"`
}

// Validate validates the interaction log entry
func (l *AgentInteractionLog) Validate() error {
	if l.LogID == "" {
		return fmt.Errorf("log ID cannot be empty")
	}
	if l.AgentID == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}
	if l.UserID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if l.Operation == "" {
		return fmt.Errorf("operation cannot be empty")
	}
	if l.Endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}
	if l.Method == "" {
		return fmt.Errorf("method cannot be empty")
	}

	return nil
}

// TimeRange represents a time range for analytics queries (HFD compliant - no defaults)
type TimeRange struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// Validate validates the time range
func (tr *TimeRange) Validate() error {
	if tr.StartTime.IsZero() {
		return fmt.Errorf("start_time is required and cannot be zero")
	}
	if tr.EndTime.IsZero() {
		return fmt.Errorf("end_time is required and cannot be zero")
	}
	if tr.EndTime.Before(tr.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	return nil
}

// AgentAnalytics represents comprehensive analytics for an agent across sessions
type AgentAnalytics struct {
	AgentID            string                    `json:"agent_id"`
	TimeRange          TimeRange                 `json:"time_range"`
	TotalInteractions  int                       `json:"total_interactions"`
	SuccessfulOps      int                       `json:"successful_operations"`
	FailedOps          int                       `json:"failed_operations"`
	SuccessRate        float64                   `json:"success_rate"`
	SessionsActive     int                       `json:"sessions_active"`
	OperationBreakdown map[string]int            `json:"operation_breakdown"`
	SessionActivity    []*SessionActivitySummary `json:"session_activity"`
	FirstActivity      *time.Time                `json:"first_activity,omitempty"`
	LastActivity       *time.Time                `json:"last_activity,omitempty"`
	ErrorPatterns      []*ErrorPattern           `json:"error_patterns,omitempty"`
}

// SessionAnalytics represents comprehensive analytics for a session across agents
type SessionAnalytics struct {
	SessionID          string                    `json:"session_id"`
	UserID             string                    `json:"user_id"`
	TotalInteractions  int                       `json:"total_interactions"`
	UniqueAgents       int                       `json:"unique_agents"`
	SessionDuration    *time.Duration            `json:"session_duration,omitempty"`
	AgentActivity      []*AgentActivitySummary   `json:"agent_activity"`
	OperationTimeline  []*OperationTimelineEntry `json:"operation_timeline"`
	CollaborationFlow  []*CollaborationStep      `json:"collaboration_flow"`
	SessionStart       *time.Time                `json:"session_start,omitempty"`
	SessionEnd         *time.Time                `json:"session_end,omitempty"`
	KnowledgeExchanges int                       `json:"knowledge_exchanges"`
}

// SessionActivitySummary represents agent activity within a specific session
type SessionActivitySummary struct {
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	Interactions  int       `json:"interactions"`
	SuccessfulOps int       `json:"successful_operations"`
	FailedOps     int       `json:"failed_operations"`
	FirstActivity time.Time `json:"first_activity"`
	LastActivity  time.Time `json:"last_activity"`
	Operations    []string  `json:"operations"`
}

// AgentActivitySummary represents agent activity within a session
type AgentActivitySummary struct {
	AgentID       string    `json:"agent_id"`
	Interactions  int       `json:"interactions"`
	SuccessfulOps int       `json:"successful_operations"`
	FailedOps     int       `json:"failed_operations"`
	FirstActivity time.Time `json:"first_activity"`
	LastActivity  time.Time `json:"last_activity"`
	Operations    []string  `json:"operations"`
}

// OperationTimelineEntry represents a timestamped operation in session timeline
type OperationTimelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	AgentID   string    `json:"agent_id"`
	Operation string    `json:"operation"`
	Success   bool      `json:"success"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}

// CollaborationStep represents a step in agent collaboration workflow
type CollaborationStep struct {
	StepNumber    int       `json:"step_number"`
	AgentID       string    `json:"agent_id"`
	Operation     string    `json:"operation"`
	Timestamp     time.Time `json:"timestamp"`
	Success       bool      `json:"success"`
	HandoffToNext bool      `json:"handoff_to_next"`
	NextAgentID   string    `json:"next_agent_id,omitempty"`
}

// ErrorPattern represents a pattern of errors for an agent
type ErrorPattern struct {
	Operation    string    `json:"operation"`
	ErrorType    string    `json:"error_type"`
	Count        int       `json:"count"`
	LastOccurred time.Time `json:"last_occurred"`
	SampleError  string    `json:"sample_error"`
}

// CollaborationInsightsRequest represents a request for collaboration insights
type CollaborationInsightsRequest struct {
	SessionID *string    `json:"session_id,omitempty"`
	AgentIDs  []string   `json:"agent_ids,omitempty"`
	TimeRange *TimeRange `json:"time_range,omitempty"`
	UserID    *string    `json:"user_id,omitempty"`
}

// Validate validates the collaboration insights request
func (r *CollaborationInsightsRequest) Validate() error {
	// At least one filter must be provided (HFD compliant)
	if r.SessionID == nil && len(r.AgentIDs) == 0 && r.TimeRange == nil && r.UserID == nil {
		return fmt.Errorf("at least one filter (session_id, agent_ids, time_range, or user_id) must be provided")
	}

	// Validate time range if provided
	if r.TimeRange != nil {
		if err := r.TimeRange.Validate(); err != nil {
			return fmt.Errorf("invalid time_range: %w", err)
		}
	}

	// Validate agent IDs if provided
	if len(r.AgentIDs) > 0 {
		for _, agentID := range r.AgentIDs {
			if agentID == "" {
				return fmt.Errorf("agent_ids cannot contain empty strings")
			}
		}
	}

	return nil
}

// CollaborationInsights represents insights about agent collaboration patterns
type CollaborationInsights struct {
	Request               *CollaborationInsightsRequest  `json:"request"`
	TotalCollaborations   int                            `json:"total_collaborations"`
	UniqueAgentPairs      int                            `json:"unique_agent_pairs"`
	AvgCollaborationTime  *time.Duration                 `json:"avg_collaboration_time,omitempty"`
	AgentCollaborations   []*AgentCollaborationSummary   `json:"agent_collaborations"`
	SessionCollaborations []*SessionCollaborationSummary `json:"session_collaborations"`
	HandoffPatterns       []*HandoffPattern              `json:"handoff_patterns"`
	KnowledgeFlowAnalysis *KnowledgeFlowAnalysis         `json:"knowledge_flow_analysis,omitempty"`
}

// AgentCollaborationSummary represents collaboration summary for an agent
type AgentCollaborationSummary struct {
	AgentID              string         `json:"agent_id"`
	SessionsParticipated int            `json:"sessions_participated"`
	CollaborationsCount  int            `json:"collaborations_count"`
	HandoffsGiven        int            `json:"handoffs_given"`
	HandoffsReceived     int            `json:"handoffs_received"`
	FrequentPartners     []string       `json:"frequent_partners"`
	AvgResponseTime      *time.Duration `json:"avg_response_time,omitempty"`
}

// SessionCollaborationSummary represents collaboration summary for a session
type SessionCollaborationSummary struct {
	SessionID           string               `json:"session_id"`
	UserID              string               `json:"user_id"`
	ParticipatingAgents []string             `json:"participating_agents"`
	HandoffCount        int                  `json:"handoff_count"`
	CollaborationFlow   []*CollaborationStep `json:"collaboration_flow"`
	SessionEfficiency   float64              `json:"session_efficiency"`
}

// HandoffPattern represents patterns in agent handoffs
type HandoffPattern struct {
	FromAgentID    string         `json:"from_agent_id"`
	ToAgentID      string         `json:"to_agent_id"`
	Frequency      int            `json:"frequency"`
	AvgHandoffTime *time.Duration `json:"avg_handoff_time,omitempty"`
	Operation      string         `json:"operation"`
	SuccessRate    float64        `json:"success_rate"`
}

// KnowledgeFlowAnalysis represents analysis of knowledge flow between agents
type KnowledgeFlowAnalysis struct {
	TotalKnowledgeExchanges int                     `json:"total_knowledge_exchanges"`
	KnowledgeContributors   []*KnowledgeContributor `json:"knowledge_contributors"`
	KnowledgeConsumers      []*KnowledgeConsumer    `json:"knowledge_consumers"`
	KnowledgeFlowPaths      []*KnowledgeFlowPath    `json:"knowledge_flow_paths"`
}

// KnowledgeContributor represents an agent that contributes knowledge
type KnowledgeContributor struct {
	AgentID             string   `json:"agent_id"`
	ContributionCount   int      `json:"contribution_count"`
	SessionsContributed int      `json:"sessions_contributed"`
	KnowledgeTypes      []string `json:"knowledge_types"`
}

// KnowledgeConsumer represents an agent that consumes knowledge
type KnowledgeConsumer struct {
	AgentID          string   `json:"agent_id"`
	ConsumptionCount int      `json:"consumption_count"`
	SessionsConsumed int      `json:"sessions_consumed"`
	KnowledgeTypes   []string `json:"knowledge_types"`
}

// KnowledgeFlowPath represents a path of knowledge flow between agents
type KnowledgeFlowPath struct {
	FromAgentID   string   `json:"from_agent_id"`
	ToAgentID     string   `json:"to_agent_id"`
	FlowCount     int      `json:"flow_count"`
	KnowledgeType string   `json:"knowledge_type"`
	SessionIDs    []string `json:"session_ids"`
}
