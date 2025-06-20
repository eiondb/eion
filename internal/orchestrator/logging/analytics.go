package logging

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// analyticsService implements the MonitoringAnalytics interface
type analyticsService struct {
	store InteractionStore
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(store InteractionStore) MonitoringAnalytics {
	return &analyticsService{
		store: store,
	}
}

// GetAgentAnalytics returns comprehensive analytics for an agent across all sessions
func (a *analyticsService) GetAgentAnalytics(ctx context.Context, agentID string, timeRange TimeRange) (*AgentAnalytics, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required and cannot be empty")
	}

	if err := timeRange.Validate(); err != nil {
		return nil, fmt.Errorf("invalid time_range: %w", err)
	}

	// Get all interactions for the agent (no limit for comprehensive analysis)
	interactions, err := a.store.GetInteractionsByAgent(ctx, agentID, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent interactions: %w", err)
	}

	// Filter interactions by time range
	filteredInteractions := a.filterInteractionsByTimeRange(interactions, timeRange)

	if len(filteredInteractions) == 0 {
		return &AgentAnalytics{
			AgentID:            agentID,
			TimeRange:          timeRange,
			TotalInteractions:  0,
			SuccessfulOps:      0,
			FailedOps:          0,
			SuccessRate:        0.0,
			SessionsActive:     0,
			OperationBreakdown: make(map[string]int),
			SessionActivity:    []*SessionActivitySummary{},
		}, nil
	}

	return a.computeAgentAnalytics(agentID, timeRange, filteredInteractions), nil
}

// GetSessionAnalytics returns comprehensive analytics for a session across all agents
func (a *analyticsService) GetSessionAnalytics(ctx context.Context, sessionID string) (*SessionAnalytics, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required and cannot be empty")
	}

	// Get all interactions for the session (no limit for comprehensive analysis)
	interactions, err := a.store.GetInteractionsBySession(ctx, sessionID, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to get session interactions: %w", err)
	}

	if len(interactions) == 0 {
		return &SessionAnalytics{
			SessionID:         sessionID,
			TotalInteractions: 0,
			UniqueAgents:      0,
			AgentActivity:     []*AgentActivitySummary{},
			OperationTimeline: []*OperationTimelineEntry{},
			CollaborationFlow: []*CollaborationStep{},
		}, nil
	}

	return a.computeSessionAnalytics(sessionID, interactions), nil
}

// GetCollaborationInsights returns insights about agent collaboration patterns
func (a *analyticsService) GetCollaborationInsights(ctx context.Context, req *CollaborationInsightsRequest) (*CollaborationInsights, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	var allInteractions []*AgentInteractionLog
	var err error

	// Gather interactions based on the request filters
	if req.SessionID != nil {
		allInteractions, err = a.store.GetInteractionsBySession(ctx, *req.SessionID, 10000)
		if err != nil {
			return nil, fmt.Errorf("failed to get session interactions: %w", err)
		}
	} else if len(req.AgentIDs) > 0 {
		// Get interactions for all specified agents
		for _, agentID := range req.AgentIDs {
			agentInteractions, err := a.store.GetInteractionsByAgent(ctx, agentID, 10000)
			if err != nil {
				return nil, fmt.Errorf("failed to get interactions for agent %s: %w", agentID, err)
			}
			allInteractions = append(allInteractions, agentInteractions...)
		}
	} else if req.UserID != nil {
		allInteractions, err = a.store.GetInteractionsByUser(ctx, *req.UserID, 10000)
		if err != nil {
			return nil, fmt.Errorf("failed to get user interactions: %w", err)
		}
	} else if req.TimeRange != nil {
		allInteractions, err = a.store.GetInteractionsByTimeRange(ctx,
			req.TimeRange.StartTime.Format(time.RFC3339),
			req.TimeRange.EndTime.Format(time.RFC3339),
			10000)
		if err != nil {
			return nil, fmt.Errorf("failed to get time range interactions: %w", err)
		}
	}

	// Apply time range filter if provided
	if req.TimeRange != nil {
		allInteractions = a.filterInteractionsByTimeRange(allInteractions, *req.TimeRange)
	}

	return a.computeCollaborationInsights(req, allInteractions), nil
}

// Helper functions for analytics computation

func (a *analyticsService) filterInteractionsByTimeRange(interactions []*AgentInteractionLog, timeRange TimeRange) []*AgentInteractionLog {
	var filtered []*AgentInteractionLog
	for _, interaction := range interactions {
		if interaction.Timestamp.After(timeRange.StartTime) && interaction.Timestamp.Before(timeRange.EndTime) {
			filtered = append(filtered, interaction)
		}
	}
	return filtered
}

func (a *analyticsService) computeAgentAnalytics(agentID string, timeRange TimeRange, interactions []*AgentInteractionLog) *AgentAnalytics {
	analytics := &AgentAnalytics{
		AgentID:            agentID,
		TimeRange:          timeRange,
		TotalInteractions:  len(interactions),
		OperationBreakdown: make(map[string]int),
		SessionActivity:    []*SessionActivitySummary{},
		ErrorPatterns:      []*ErrorPattern{},
	}

	// Track session activity
	sessionMap := make(map[string]*SessionActivitySummary)
	errorMap := make(map[string]*ErrorPattern)

	var firstActivity, lastActivity *time.Time

	for _, interaction := range interactions {
		// Count success/failure
		if interaction.Success {
			analytics.SuccessfulOps++
		} else {
			analytics.FailedOps++
		}

		// Track operations
		analytics.OperationBreakdown[interaction.Operation]++

		// Track session activity
		if interaction.SessionID != "" {
			if _, exists := sessionMap[interaction.SessionID]; !exists {
				sessionMap[interaction.SessionID] = &SessionActivitySummary{
					SessionID:     interaction.SessionID,
					UserID:        interaction.UserID,
					FirstActivity: interaction.Timestamp,
					LastActivity:  interaction.Timestamp,
					Operations:    []string{},
				}
			}

			session := sessionMap[interaction.SessionID]
			session.Interactions++
			if interaction.Success {
				session.SuccessfulOps++
			} else {
				session.FailedOps++
			}

			// Update time bounds
			if interaction.Timestamp.Before(session.FirstActivity) {
				session.FirstActivity = interaction.Timestamp
			}
			if interaction.Timestamp.After(session.LastActivity) {
				session.LastActivity = interaction.Timestamp
			}

			// Track unique operations
			if !contains(session.Operations, interaction.Operation) {
				session.Operations = append(session.Operations, interaction.Operation)
			}
		}

		// Track error patterns
		if !interaction.Success && interaction.ErrorMsg != "" {
			errorKey := fmt.Sprintf("%s:%s", interaction.Operation, extractErrorType(interaction.ErrorMsg))
			if _, exists := errorMap[errorKey]; !exists {
				errorMap[errorKey] = &ErrorPattern{
					Operation:   interaction.Operation,
					ErrorType:   extractErrorType(interaction.ErrorMsg),
					Count:       0,
					SampleError: interaction.ErrorMsg,
				}
			}

			pattern := errorMap[errorKey]
			pattern.Count++
			pattern.LastOccurred = interaction.Timestamp
		}

		// Track first/last activity
		if firstActivity == nil || interaction.Timestamp.Before(*firstActivity) {
			firstActivity = &interaction.Timestamp
		}
		if lastActivity == nil || interaction.Timestamp.After(*lastActivity) {
			lastActivity = &interaction.Timestamp
		}
	}

	// Calculate success rate
	if analytics.TotalInteractions > 0 {
		analytics.SuccessRate = float64(analytics.SuccessfulOps) / float64(analytics.TotalInteractions)
	}

	// Convert maps to slices
	analytics.SessionsActive = len(sessionMap)
	for _, session := range sessionMap {
		analytics.SessionActivity = append(analytics.SessionActivity, session)
	}

	for _, pattern := range errorMap {
		analytics.ErrorPatterns = append(analytics.ErrorPatterns, pattern)
	}

	analytics.FirstActivity = firstActivity
	analytics.LastActivity = lastActivity

	return analytics
}

func (a *analyticsService) computeSessionAnalytics(sessionID string, interactions []*AgentInteractionLog) *SessionAnalytics {
	analytics := &SessionAnalytics{
		SessionID:         sessionID,
		TotalInteractions: len(interactions),
		AgentActivity:     []*AgentActivitySummary{},
		OperationTimeline: []*OperationTimelineEntry{},
		CollaborationFlow: []*CollaborationStep{},
	}

	if len(interactions) == 0 {
		return analytics
	}

	// Get user ID from first interaction
	analytics.UserID = interactions[0].UserID

	// Track agent activity
	agentMap := make(map[string]*AgentActivitySummary)
	var timeline []*OperationTimelineEntry

	var sessionStart, sessionEnd *time.Time

	for _, interaction := range interactions {
		// Track agent activity
		if _, exists := agentMap[interaction.AgentID]; !exists {
			agentMap[interaction.AgentID] = &AgentActivitySummary{
				AgentID:       interaction.AgentID,
				FirstActivity: interaction.Timestamp,
				LastActivity:  interaction.Timestamp,
				Operations:    []string{},
			}
		}

		agent := agentMap[interaction.AgentID]
		agent.Interactions++
		if interaction.Success {
			agent.SuccessfulOps++
		} else {
			agent.FailedOps++
		}

		// Update time bounds for agent
		if interaction.Timestamp.Before(agent.FirstActivity) {
			agent.FirstActivity = interaction.Timestamp
		}
		if interaction.Timestamp.After(agent.LastActivity) {
			agent.LastActivity = interaction.Timestamp
		}

		// Track unique operations for agent
		if !contains(agent.Operations, interaction.Operation) {
			agent.Operations = append(agent.Operations, interaction.Operation)
		}

		// Add to timeline
		timeline = append(timeline, &OperationTimelineEntry{
			Timestamp: interaction.Timestamp,
			AgentID:   interaction.AgentID,
			Operation: interaction.Operation,
			Success:   interaction.Success,
			ErrorMsg:  interaction.ErrorMsg,
		})

		// Track session bounds
		if sessionStart == nil || interaction.Timestamp.Before(*sessionStart) {
			sessionStart = &interaction.Timestamp
		}
		if sessionEnd == nil || interaction.Timestamp.After(*sessionEnd) {
			sessionEnd = &interaction.Timestamp
		}

		// Count knowledge exchanges (operations involving memory/knowledge)
		if isKnowledgeOperation(interaction.Operation) {
			analytics.KnowledgeExchanges++
		}
	}

	// Sort timeline by timestamp
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})

	analytics.UniqueAgents = len(agentMap)
	for _, agent := range agentMap {
		analytics.AgentActivity = append(analytics.AgentActivity, agent)
	}
	analytics.OperationTimeline = timeline

	// Calculate session duration
	if sessionStart != nil && sessionEnd != nil {
		duration := sessionEnd.Sub(*sessionStart)
		analytics.SessionDuration = &duration
	}

	analytics.SessionStart = sessionStart
	analytics.SessionEnd = sessionEnd

	// Compute collaboration flow (simplified for sequential patterns)
	analytics.CollaborationFlow = a.computeCollaborationFlow(timeline)

	return analytics
}

func (a *analyticsService) computeCollaborationInsights(req *CollaborationInsightsRequest, interactions []*AgentInteractionLog) *CollaborationInsights {
	insights := &CollaborationInsights{
		Request:               req,
		TotalCollaborations:   0,
		UniqueAgentPairs:      0,
		AgentCollaborations:   []*AgentCollaborationSummary{},
		SessionCollaborations: []*SessionCollaborationSummary{},
		HandoffPatterns:       []*HandoffPattern{},
	}

	if len(interactions) == 0 {
		return insights
	}

	// Group by sessions for collaboration analysis
	sessionGroups := make(map[string][]*AgentInteractionLog)
	for _, interaction := range interactions {
		if interaction.SessionID != "" {
			sessionGroups[interaction.SessionID] = append(sessionGroups[interaction.SessionID], interaction)
		}
	}

	insights.TotalCollaborations = len(sessionGroups)

	// Analyze each session for collaboration patterns
	agentCollabMap := make(map[string]*AgentCollaborationSummary)
	var sessionCollabs []*SessionCollaborationSummary
	handoffPatternMap := make(map[string]*HandoffPattern)

	for sessionID, sessionInteractions := range sessionGroups {
		sessionCollab := a.analyzeSessionCollaboration(sessionID, sessionInteractions)
		sessionCollabs = append(sessionCollabs, sessionCollab)

		// Track agent collaboration summaries
		for _, agentID := range sessionCollab.ParticipatingAgents {
			if _, exists := agentCollabMap[agentID]; !exists {
				agentCollabMap[agentID] = &AgentCollaborationSummary{
					AgentID:          agentID,
					FrequentPartners: []string{},
				}
			}

			agentSummary := agentCollabMap[agentID]
			agentSummary.SessionsParticipated++
			agentSummary.CollaborationsCount += len(sessionCollab.ParticipatingAgents) - 1
		}

		// Analyze handoff patterns in this session
		handoffs := a.detectHandoffPatterns(sessionInteractions)
		for _, handoff := range handoffs {
			key := fmt.Sprintf("%s->%s:%s", handoff.FromAgentID, handoff.ToAgentID, handoff.Operation)
			if existing, exists := handoffPatternMap[key]; exists {
				existing.Frequency++
				// Update success rate
				if handoff.SuccessRate > 0 {
					existing.SuccessRate = (existing.SuccessRate + handoff.SuccessRate) / 2
				}
			} else {
				handoffPatternMap[key] = handoff
			}
		}
	}

	// Convert maps to slices
	for _, agentSummary := range agentCollabMap {
		insights.AgentCollaborations = append(insights.AgentCollaborations, agentSummary)
	}
	insights.SessionCollaborations = sessionCollabs

	for _, pattern := range handoffPatternMap {
		insights.HandoffPatterns = append(insights.HandoffPatterns, pattern)
	}

	// Count unique agent pairs
	agentPairs := make(map[string]bool)
	for _, sessionCollab := range sessionCollabs {
		agents := sessionCollab.ParticipatingAgents
		for i := 0; i < len(agents); i++ {
			for j := i + 1; j < len(agents); j++ {
				pair := fmt.Sprintf("%s-%s", agents[i], agents[j])
				agentPairs[pair] = true
			}
		}
	}
	insights.UniqueAgentPairs = len(agentPairs)

	return insights
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func extractErrorType(errorMsg string) string {
	// Simple error type extraction
	if strings.Contains(errorMsg, "validation") {
		return "validation_error"
	}
	if strings.Contains(errorMsg, "not found") {
		return "not_found_error"
	}
	if strings.Contains(errorMsg, "permission") || strings.Contains(errorMsg, "unauthorized") {
		return "permission_error"
	}
	if strings.Contains(errorMsg, "connection") || strings.Contains(errorMsg, "network") {
		return "connection_error"
	}
	return "unknown_error"
}

func isKnowledgeOperation(operation string) bool {
	knowledgeOps := []string{"put_memory", "get_memory", "search_memory", "put_knowledge", "get_knowledge"}
	for _, op := range knowledgeOps {
		if operation == op {
			return true
		}
	}
	return false
}

func (a *analyticsService) computeCollaborationFlow(timeline []*OperationTimelineEntry) []*CollaborationStep {
	var flow []*CollaborationStep

	for i, entry := range timeline {
		step := &CollaborationStep{
			StepNumber: i + 1,
			AgentID:    entry.AgentID,
			Operation:  entry.Operation,
			Timestamp:  entry.Timestamp,
			Success:    entry.Success,
		}

		// Detect handoff to next agent
		if i < len(timeline)-1 {
			nextEntry := timeline[i+1]
			if nextEntry.AgentID != entry.AgentID {
				step.HandoffToNext = true
				step.NextAgentID = nextEntry.AgentID
			}
		}

		flow = append(flow, step)
	}

	return flow
}

func (a *analyticsService) analyzeSessionCollaboration(sessionID string, interactions []*AgentInteractionLog) *SessionCollaborationSummary {
	agentSet := make(map[string]bool)
	handoffCount := 0
	var userID string

	if len(interactions) > 0 {
		userID = interactions[0].UserID
	}

	// Sort interactions by timestamp
	sort.Slice(interactions, func(i, j int) bool {
		return interactions[i].Timestamp.Before(interactions[j].Timestamp)
	})

	var lastAgentID string
	var flow []*CollaborationStep

	for i, interaction := range interactions {
		agentSet[interaction.AgentID] = true

		// Detect handoffs (agent changes)
		if lastAgentID != "" && lastAgentID != interaction.AgentID {
			handoffCount++
		}
		lastAgentID = interaction.AgentID

		// Add to collaboration flow
		step := &CollaborationStep{
			StepNumber: i + 1,
			AgentID:    interaction.AgentID,
			Operation:  interaction.Operation,
			Timestamp:  interaction.Timestamp,
			Success:    interaction.Success,
		}

		if i < len(interactions)-1 && interactions[i+1].AgentID != interaction.AgentID {
			step.HandoffToNext = true
			step.NextAgentID = interactions[i+1].AgentID
		}

		flow = append(flow, step)
	}

	// Convert agent set to slice
	var participatingAgents []string
	for agentID := range agentSet {
		participatingAgents = append(participatingAgents, agentID)
	}

	// Calculate session efficiency (success rate)
	successfulOps := 0
	for _, interaction := range interactions {
		if interaction.Success {
			successfulOps++
		}
	}

	efficiency := 0.0
	if len(interactions) > 0 {
		efficiency = float64(successfulOps) / float64(len(interactions))
	}

	return &SessionCollaborationSummary{
		SessionID:           sessionID,
		UserID:              userID,
		ParticipatingAgents: participatingAgents,
		HandoffCount:        handoffCount,
		CollaborationFlow:   flow,
		SessionEfficiency:   efficiency,
	}
}

func (a *analyticsService) detectHandoffPatterns(interactions []*AgentInteractionLog) []*HandoffPattern {
	var patterns []*HandoffPattern

	// Sort by timestamp
	sort.Slice(interactions, func(i, j int) bool {
		return interactions[i].Timestamp.Before(interactions[j].Timestamp)
	})

	for i := 0; i < len(interactions)-1; i++ {
		current := interactions[i]
		next := interactions[i+1]

		// Detect handoff (different agents in sequence)
		if current.AgentID != next.AgentID {
			// Calculate handoff time
			handoffTime := next.Timestamp.Sub(current.Timestamp)

			pattern := &HandoffPattern{
				FromAgentID:    current.AgentID,
				ToAgentID:      next.AgentID,
				Frequency:      1,
				AvgHandoffTime: &handoffTime,
				Operation:      current.Operation,
				SuccessRate:    0.0,
			}

			// Calculate success rate for this handoff
			if current.Success && next.Success {
				pattern.SuccessRate = 1.0
			} else if current.Success || next.Success {
				pattern.SuccessRate = 0.5
			}

			patterns = append(patterns, pattern)
		}
	}

	return patterns
}
