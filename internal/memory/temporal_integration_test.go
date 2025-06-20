package memory

import (
	"context"
	"testing"
	"time"

	"github.com/eion/eion/internal/graph"
	"github.com/eion/eion/internal/numa"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestTemporalOperationsIntegration demonstrates internal temporal logic working with Neo4j
func TestTemporalOperationsIntegration(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Skip if Neo4j not available (CI/local development flexibility)
	neo4jURI := "bolt://localhost:7687"
	driver, err := neo4j.NewDriverWithContext(neo4jURI, neo4j.BasicAuth("neo4j", "password", ""))
	if err != nil {
		t.Skipf("Neo4j not available, skipping integration test: %v", err)
		return
	}
	defer driver.Close(ctx)

	// Test Neo4j connection
	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		t.Skipf("Neo4j not reachable, skipping integration test: %v", err)
		return
	}

	// Clean up test data
	t.Cleanup(func() {
		cleanupTestData(ctx, driver, t)
	})

	t.Run("RealNeo4jEdgeInvalidationCandidates", func(t *testing.T) {
		temporalOps := numa.NewTemporalOperations(logger, driver)

		// Create test edges in Neo4j
		edge1 := graph.EdgeNode{
			UUID:           "test_edge_1",
			SourceNodeUUID: "node_john",
			TargetNodeUUID: "node_acme",
			RelationType:   "WORKS_FOR",
			Fact:           "John works for Acme Corp",
			FactEmbedding:  []float32{0.1, 0.2, 0.3, 0.4},
			GroupID:        "test_group",
			CreatedAt:      time.Now().UTC(),
			Version:        1,
			LastModifiedBy: "test_agent",
		}

		edge2 := graph.EdgeNode{
			UUID:           "test_edge_2",
			SourceNodeUUID: "node_john",
			TargetNodeUUID: "node_acme",
			RelationType:   "EMPLOYED_BY",
			Fact:           "John is employed by Acme Corporation",
			FactEmbedding:  []float32{0.11, 0.21, 0.31, 0.41}, // Very similar
			GroupID:        "test_group",
			CreatedAt:      time.Now().UTC(),
			Version:        1,
			LastModifiedBy: "test_agent",
		}

		// Store test edges in Neo4j
		storeTestEdges(ctx, driver, []graph.EdgeNode{edge1}, t)

		// Test real Neo4j vector similarity search
		candidates, err := temporalOps.GetEdgeInvalidationCandidates(
			ctx,
			[]graph.EdgeNode{edge2},
			[]string{"test_group"},
			0.8,     // similarity threshold
			10,      // limit
			"neo4j", // database
		)

		require.NoError(t, err)
		require.Len(t, candidates, 1)
		require.NotEmpty(t, candidates[0])

		// Should find edge1 as similar to edge2
		foundCandidate := candidates[0][0]
		assert.Equal(t, "test_edge_1", foundCandidate.UUID)
		assert.Equal(t, "John works for Acme Corp", foundCandidate.Fact)
	})

	t.Run("DuplicateDetectionWithNeo4jData", func(t *testing.T) {
		temporalOps := numa.NewTemporalOperations(logger, driver)

		// Test with actual data from Neo4j
		existingEdge := graph.EdgeNode{
			UUID:          "existing_edge",
			Fact:          "John manages the sales team",
			FactEmbedding: []float32{0.5, 0.6, 0.7, 0.8},
		}

		newEdge := graph.EdgeNode{
			UUID:          "new_edge",
			Fact:          "John leads the sales department",
			FactEmbedding: []float32{0.51, 0.61, 0.71, 0.81}, // Very similar
		}

		// Test vector similarity (API-free but with real embeddings)
		duplicate := temporalOps.DetectDuplicateEdge(
			newEdge,
			[]graph.EdgeNode{existingEdge},
			numa.VectorSimilarity,
			0.8,
		)

		assert.NotNil(t, duplicate)
		assert.Equal(t, "existing_edge", duplicate.UUID)
	})

	t.Run("SequentialAgentConflictDetectionProduction", func(t *testing.T) {
		temporalOps := numa.NewTemporalOperations(logger, driver)

		// Test realistic scenario: two agents working on same session
		conflict := temporalOps.DetectVersionConflict(
			1, // expected version
			3, // actual version (significant conflict!)
			"session123",
			"billing_agent",
			"customer_support",
		)

		assert.NotNil(t, conflict)
		assert.Equal(t, "session123", conflict.ResourceID)
		assert.Equal(t, 1, conflict.ExpectedVersion)
		assert.Equal(t, 3, conflict.ActualVersion)
		assert.Equal(t, "billing_agent", conflict.ConflictingAgent)
		assert.Equal(t, "detected", conflict.Status)

		// Test conflict resolution with production strategy
		resolution := temporalOps.ResolveConflict(
			*conflict,
			"last_writer_wins",
			"supervisor_agent",
		)

		assert.Equal(t, "last_writer_wins", resolution.Strategy)
		assert.Equal(t, "auto", resolution.ResolutionType)
		assert.Equal(t, "supervisor_agent", resolution.ResolvedBy)
		assert.False(t, resolution.RequiresManualAction)
	})

	t.Run("TemporalContradictionResolution", func(t *testing.T) {
		temporalOps := numa.NewTemporalOperations(logger, driver)

		now := time.Now().UTC()
		yesterday := now.AddDate(0, 0, -1)

		// Test internal temporal logic
		resolvedEdge := graph.EdgeNode{
			UUID:           "new_fact",
			SourceNodeUUID: "node_john",
			TargetNodeUUID: "node_microsoft",
			RelationType:   "WORKS_FOR",
			Fact:           "John works for Microsoft",
			ValidAt:        &now,
			Version:        2,
			LastModifiedBy: "hr_agent",
		}

		invalidationCandidate := graph.EdgeNode{
			UUID:           "old_fact",
			SourceNodeUUID: "node_john",
			TargetNodeUUID: "node_microsoft",
			RelationType:   "WORKS_FOR",
			Fact:           "John works for Google",
			ValidAt:        &yesterday,
			Version:        1,
			LastModifiedBy: "recruiting_agent",
		}

		// Test temporal contradiction resolution
		invalidated := temporalOps.ResolveEdgeContradictions(
			resolvedEdge,
			[]graph.EdgeNode{invalidationCandidate},
		)

		require.Len(t, invalidated, 1)
		assert.Equal(t, "old_fact", invalidated[0].UUID)
		assert.NotNil(t, invalidated[0].ExpiredAt)
		assert.NotNil(t, invalidated[0].InvalidAt)
	})
}

// storeTestEdges stores edges in Neo4j for testing
func storeTestEdges(ctx context.Context, driver neo4j.DriverWithContext, edges []graph.EdgeNode, t *testing.T) {
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	for _, edge := range edges {
		query := `
		CREATE (e:Edge {
			uuid: $uuid,
			source_node_uuid: $source_node_uuid,
			target_node_uuid: $target_node_uuid,
			relation_type: $relation_type,
			fact: $fact,
			fact_embedding: $fact_embedding,
			group_id: $group_id,
			created_at: datetime($created_at),
			version: $version,
			last_modified_by: $last_modified_by
		})
		`

		params := map[string]any{
			"uuid":             edge.UUID,
			"source_node_uuid": edge.SourceNodeUUID,
			"target_node_uuid": edge.TargetNodeUUID,
			"relation_type":    edge.RelationType,
			"fact":             edge.Fact,
			"fact_embedding":   edge.FactEmbedding,
			"group_id":         edge.GroupID,
			"created_at":       edge.CreatedAt.Format(time.RFC3339),
			"version":          edge.Version,
			"last_modified_by": edge.LastModifiedBy,
		}

		_, err := session.Run(ctx, query, params)
		require.NoError(t, err)
	}
}

// cleanupTestData removes test data from Neo4j
func cleanupTestData(ctx context.Context, driver neo4j.DriverWithContext, t *testing.T) {
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	query := `MATCH (e:Edge) WHERE e.group_id = 'test_group' OR e.uuid STARTS WITH 'test_' DELETE e`
	_, err := session.Run(ctx, query, nil)
	if err != nil {
		t.Logf("Cleanup warning: %v", err)
	}
}
