package numa

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/eion/eion/internal/graph"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// TemporalOperations handles edge temporal logic - internal implementation
type TemporalOperations struct {
	logger *zap.Logger
	driver neo4j.DriverWithContext
}

// NewTemporalOperations creates a new temporal operations instance
func NewTemporalOperations(logger *zap.Logger, driver neo4j.DriverWithContext) *TemporalOperations {
	return &TemporalOperations{
		logger: logger,
		driver: driver,
	}
}

// ConflictDetection represents a detected conflict - copied from Week 3 requirements
type ConflictDetection struct {
	ConflictID       string    `json:"conflict_id"`
	UserID           string    `json:"user_id"`
	ResourceID       string    `json:"resource_id"`
	ExpectedVersion  int       `json:"expected_version"`
	ActualVersion    int       `json:"actual_version"`
	ConflictingAgent string    `json:"conflicting_agent"`
	LastModifiedBy   string    `json:"last_modified_by"`
	DetectedAt       time.Time `json:"detected_at"`
	Status           string    `json:"status"`
}

// ConflictResolution represents conflict resolution strategy
type ConflictResolution struct {
	ConflictID           string                 `json:"conflict_id"`
	ResolutionType       string                 `json:"resolution_type"`
	Strategy             string                 `json:"strategy"`
	ResolvedBy           string                 `json:"resolved_by"`
	ResolvedAt           time.Time              `json:"resolved_at"`
	ResolutionData       map[string]interface{} `json:"resolution_data"`
	RequiresManualAction bool                   `json:"requires_manual_action"`
}

// DuplicateDetectionStrategy represents different duplicate detection strategies
type DuplicateDetectionStrategy string

const (
	VectorSimilarity   DuplicateDetectionStrategy = "vector_similarity"
	EntityPatternMatch DuplicateDetectionStrategy = "entity_pattern"
	StringSimilarity   DuplicateDetectionStrategy = "string_similarity"
)

// GetEdgeInvalidationCandidates - internal implementation for temporal logic
func (t *TemporalOperations) GetEdgeInvalidationCandidates(
	ctx context.Context,
	edges []graph.EdgeNode,
	groupIDs []string,
	minScore float64,
	limit int,
	database string,
) ([][]graph.EdgeNode, error) {
	if len(edges) == 0 {
		return [][]graph.EdgeNode{}, nil
	}

	// Build query for temporal edge invalidation
	query := `
		UNWIND $edges AS edge
		MATCH (n:Entity)-[e:RELATES_TO {group_id: edge.group_id}]->(m:Entity)
		WHERE n.uuid IN [edge.source_node_uuid, edge.target_node_uuid] 
		   OR m.uuid IN [edge.target_node_uuid, edge.source_node_uuid]
		   AND e.group_id IN $group_ids
		WITH edge, e, 
		     gds.similarity.cosine(e.fact_embedding, edge.fact_embedding) AS score
		WHERE score > $min_score
		WITH edge, e, score
		ORDER BY score DESC
		RETURN edge.uuid AS search_edge_uuid,
		       collect({
		           uuid: e.uuid,
		           source_node_uuid: startNode(e).uuid,
		           target_node_uuid: endNode(e).uuid,
		           created_at: e.created_at,
		           relation_type: e.relation_type,
		           fact: e.fact,
		           group_id: e.group_id,
		           episodes: e.episodes,
		           expired_at: e.expired_at,
		           valid_at: e.valid_at,
		           invalid_at: e.invalid_at,
		           version: e.version,
		           last_modified_by: e.last_modified_by,
		           checksum_hash: e.checksum_hash,
		           fact_embedding: e.fact_embedding
		       })[..$limit] AS matches
	`

	params := map[string]any{
		"edges":     edgesToParams(edges),
		"group_ids": groupIDs,
		"min_score": minScore,
		"limit":     limit,
	}

	session := t.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: database,
	})
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get edge invalidation candidates: %w", err)
	}

	invalidationCandidates := make([][]graph.EdgeNode, len(edges))
	candidatesMap := make(map[string][]graph.EdgeNode)

	for result.Next(ctx) {
		record := result.Record()
		searchEdgeUUID := record.Values[0].(string)
		matches := record.Values[1].([]any)

		var candidates []graph.EdgeNode
		for _, match := range matches {
			matchMap := match.(map[string]any)
			candidate := graph.EdgeNode{
				UUID:           getString(matchMap, "uuid"),
				SourceNodeUUID: getString(matchMap, "source_node_uuid"),
				TargetNodeUUID: getString(matchMap, "target_node_uuid"),
				RelationType:   getString(matchMap, "relation_type"),
				Fact:           getString(matchMap, "fact"),
				GroupID:        getString(matchMap, "group_id"),
				CreatedAt:      getTime(matchMap, "created_at"),
				Episodes:       getStringSlice(matchMap, "episodes"),
				ExpiredAt:      getTimePtr(matchMap, "expired_at"),
				ValidAt:        getTimePtr(matchMap, "valid_at"),
				InvalidAt:      getTimePtr(matchMap, "invalid_at"),
				Version:        getInt(matchMap, "version"),
				LastModifiedBy: getString(matchMap, "last_modified_by"),
				ChecksumHash:   getString(matchMap, "checksum_hash"),
				FactEmbedding:  getFloat32Slice(matchMap, "fact_embedding"),
			}
			candidates = append(candidates, candidate)
		}
		candidatesMap[searchEdgeUUID] = candidates
	}

	// Map back to original edge order
	for i, edge := range edges {
		if candidates, exists := candidatesMap[edge.UUID]; exists {
			invalidationCandidates[i] = candidates
		} else {
			invalidationCandidates[i] = []graph.EdgeNode{}
		}
	}

	return invalidationCandidates, nil
}

// DetectDuplicateEdge detects if an edge is a duplicate using API-free strategies
func (t *TemporalOperations) DetectDuplicateEdge(
	newEdge graph.EdgeNode,
	existingEdges []graph.EdgeNode,
	strategy DuplicateDetectionStrategy,
	threshold float64,
) *graph.EdgeNode {
	switch strategy {
	case VectorSimilarity:
		return t.detectByVectorSimilarity(newEdge, existingEdges, threshold)
	case EntityPatternMatch:
		return t.detectByEntityPattern(newEdge, existingEdges)
	case StringSimilarity:
		return t.detectByStringSimilarity(newEdge, existingEdges, threshold)
	default:
		return nil
	}
}

// detectByVectorSimilarity - API-free vector similarity duplicate detection
func (t *TemporalOperations) detectByVectorSimilarity(
	newEdge graph.EdgeNode,
	existingEdges []graph.EdgeNode,
	threshold float64,
) *graph.EdgeNode {
	if len(newEdge.FactEmbedding) == 0 {
		return nil
	}

	for _, edge := range existingEdges {
		if len(edge.FactEmbedding) == 0 {
			continue
		}

		similarity := cosineSimilarity(newEdge.FactEmbedding, edge.FactEmbedding)
		if similarity > threshold {
			t.logger.Debug("Found duplicate edge by vector similarity",
				zap.String("new_edge", newEdge.UUID),
				zap.String("existing_edge", edge.UUID),
				zap.Float64("similarity", similarity))
			return &edge
		}
	}
	return nil
}

// detectByEntityPattern - Rule-based pattern matching duplicate detection
func (t *TemporalOperations) detectByEntityPattern(
	newEdge graph.EdgeNode,
	existingEdges []graph.EdgeNode,
) *graph.EdgeNode {
	for _, edge := range existingEdges {
		// Same entities + similar relation type
		if edge.SourceNodeUUID == newEdge.SourceNodeUUID &&
			edge.TargetNodeUUID == newEdge.TargetNodeUUID &&
			edge.RelationType == newEdge.RelationType {
			t.logger.Debug("Found duplicate edge by entity pattern",
				zap.String("new_edge", newEdge.UUID),
				zap.String("existing_edge", edge.UUID))
			return &edge
		}
	}
	return nil
}

// detectByStringSimilarity - String similarity + entity matching duplicate detection
func (t *TemporalOperations) detectByStringSimilarity(
	newEdge graph.EdgeNode,
	existingEdges []graph.EdgeNode,
	threshold float64,
) *graph.EdgeNode {
	for _, edge := range existingEdges {
		// Same entity pair
		sameEntities := edge.SourceNodeUUID == newEdge.SourceNodeUUID &&
			edge.TargetNodeUUID == newEdge.TargetNodeUUID

		// High string similarity
		factSimilarity := stringSimilarity(edge.Fact, newEdge.Fact)

		if sameEntities && factSimilarity > threshold {
			t.logger.Debug("Found duplicate edge by string similarity",
				zap.String("new_edge", newEdge.UUID),
				zap.String("existing_edge", edge.UUID),
				zap.Float64("similarity", factSimilarity))
			return &edge
		}
	}
	return nil
}

// ResolveEdgeContradictions - internal implementation for temporal edge resolution
func (t *TemporalOperations) ResolveEdgeContradictions(
	resolvedEdge graph.EdgeNode,
	invalidationCandidates []graph.EdgeNode,
) []graph.EdgeNode {
	now := time.Now().UTC()
	var invalidatedEdges []graph.EdgeNode

	for _, candidate := range invalidationCandidates {
		// Skip if already expired
		if candidate.ExpiredAt != nil {
			continue
		}

		// Determine invalidation based on temporal logic
		shouldInvalidate := false

		// Case 1: New edge has explicit invalid_at that conflicts with candidate
		if resolvedEdge.InvalidAt != nil && candidate.ValidAt != nil {
			if resolvedEdge.InvalidAt.Before(*candidate.ValidAt) {
				shouldInvalidate = true
			}
		}

		// Case 2: Candidate should be invalidated if resolved edge is more recent
		if resolvedEdge.ValidAt != nil && candidate.ValidAt != nil {
			if resolvedEdge.ValidAt.After(*candidate.ValidAt) {
				shouldInvalidate = true
			}
		}

		// Case 3: Same entities with contradictory facts
		if candidate.SourceNodeUUID == resolvedEdge.SourceNodeUUID &&
			candidate.TargetNodeUUID == resolvedEdge.TargetNodeUUID &&
			candidate.RelationType == resolvedEdge.RelationType &&
			candidate.Fact != resolvedEdge.Fact {
			shouldInvalidate = true
		}

		if shouldInvalidate {
			// Create invalidated copy
			invalidated := candidate
			invalidated.ExpiredAt = &now
			if resolvedEdge.ValidAt != nil {
				invalidated.InvalidAt = resolvedEdge.ValidAt
			}
			invalidatedEdges = append(invalidatedEdges, invalidated)

			t.logger.Debug("Invalidated edge due to contradiction",
				zap.String("invalidated_edge", candidate.UUID),
				zap.String("resolved_edge", resolvedEdge.UUID))
		}
	}

	return invalidatedEdges
}

// DetectVersionConflict detects version conflicts for sequential agent access
func (t *TemporalOperations) DetectVersionConflict(
	expectedVersion int,
	actualVersion int,
	resourceID string,
	agentID string,
	userID string,
) *ConflictDetection {
	if expectedVersion != actualVersion {
		return &ConflictDetection{
			ConflictID:       fmt.Sprintf("conflict_%s_%d", resourceID, time.Now().Unix()),
			UserID:           userID,
			ResourceID:       resourceID,
			ExpectedVersion:  expectedVersion,
			ActualVersion:    actualVersion,
			ConflictingAgent: agentID,
			DetectedAt:       time.Now().UTC(),
			Status:           "detected",
		}
	}
	return nil
}

// ResolveConflict resolves conflicts using specified strategy
func (t *TemporalOperations) ResolveConflict(
	conflict ConflictDetection,
	strategy string,
	resolvedBy string,
) ConflictResolution {
	now := time.Now().UTC()

	resolution := ConflictResolution{
		ConflictID:     conflict.ConflictID,
		ResolutionType: "auto",
		Strategy:       strategy,
		ResolvedBy:     resolvedBy,
		ResolvedAt:     now,
		ResolutionData: make(map[string]interface{}),
	}

	switch strategy {
	case "last_writer_wins":
		resolution.ResolutionData["action"] = "accept_newer_version"
		resolution.RequiresManualAction = false
	case "retry":
		resolution.ResolutionData["action"] = "request_agent_retry"
		resolution.RequiresManualAction = false
	case "manual":
		resolution.ResolutionType = "manual"
		resolution.RequiresManualAction = true
	default:
		resolution.Strategy = "last_writer_wins"
		resolution.ResolutionData["action"] = "accept_newer_version"
		resolution.RequiresManualAction = false
	}

	return resolution
}

// GenerateChecksum generates checksum for content integrity
func (t *TemporalOperations) GenerateChecksum(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// Helper functions
func edgesToParams(edges []graph.EdgeNode) []map[string]any {
	params := make([]map[string]any, len(edges))
	for i, edge := range edges {
		params[i] = map[string]any{
			"uuid":             edge.UUID,
			"source_node_uuid": edge.SourceNodeUUID,
			"target_node_uuid": edge.TargetNodeUUID,
			"group_id":         edge.GroupID,
			"fact_embedding":   edge.FactEmbedding,
		}
	}
	return params
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func stringSimilarity(a, b string) float64 {
	// Simple Jaccard similarity on words
	wordsA := make(map[string]bool)
	wordsB := make(map[string]bool)

	// Split by space and normalize
	for _, word := range strings.Fields(strings.ToLower(a)) {
		wordsA[word] = true
	}
	for _, word := range strings.Fields(strings.ToLower(b)) {
		wordsB[word] = true
	}

	// Calculate intersection and union
	intersection := 0
	union := len(wordsA)

	for word := range wordsB {
		if wordsA[word] {
			intersection++
		} else {
			union++
		}
	}

	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// Helper functions for type conversion
func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key]; ok && v != nil {
		if i, ok := v.(int); ok {
			return i
		}
		if i, ok := v.(int64); ok {
			return int(i)
		}
	}
	return 0
}

func getTime(m map[string]any, key string) time.Time {
	if v, ok := m[key]; ok && v != nil {
		if t, ok := v.(time.Time); ok {
			return t
		}
	}
	return time.Time{}
}

func getTimePtr(m map[string]any, key string) *time.Time {
	if v, ok := m[key]; ok && v != nil {
		if t, ok := v.(time.Time); ok {
			return &t
		}
	}
	return nil
}

func getStringSlice(m map[string]any, key string) []string {
	if v, ok := m[key]; ok && v != nil {
		if slice, ok := v.([]any); ok {
			result := make([]string, len(slice))
			for i, item := range slice {
				if s, ok := item.(string); ok {
					result[i] = s
				}
			}
			return result
		}
	}
	return []string{}
}

func getFloat32Slice(m map[string]any, key string) []float32 {
	if v, ok := m[key]; ok && v != nil {
		if slice, ok := v.([]any); ok {
			result := make([]float32, len(slice))
			for i, item := range slice {
				if f, ok := item.(float64); ok {
					result[i] = float32(f)
				}
			}
			return result
		}
	}
	return []float32{}
}
