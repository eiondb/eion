package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// Neo4jClient implements knowledge graph operations using Neo4j
type Neo4jClient struct {
	driver neo4j.DriverWithContext
	logger *zap.Logger
}

// Neo4jConfig represents Neo4j connection configuration
type Neo4jConfig struct {
	URI      string `json:"uri" yaml:"uri"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Database string `json:"database" yaml:"database"`
}

// EntityNode represents a knowledge graph entity
type EntityNode struct {
	UUID      string         `json:"uuid"`
	Name      string         `json:"name"`
	Labels    []string       `json:"labels"`
	Summary   string         `json:"summary"`
	GroupID   string         `json:"group_id"`
	CreatedAt time.Time      `json:"created_at"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Embedding []float32      `json:"embedding,omitempty"`
}

// EdgeNode represents a knowledge graph relationship with temporal tracking
type EdgeNode struct {
	UUID           string    `json:"uuid"`
	SourceNodeUUID string    `json:"source_node_uuid"`
	TargetNodeUUID string    `json:"target_node_uuid"`
	RelationType   string    `json:"relation_type"`
	Summary        string    `json:"summary"`
	Fact           string    `json:"fact"`
	GroupID        string    `json:"group_id"`
	CreatedAt      time.Time `json:"created_at"`
	Episodes       []string  `json:"episodes"`
	// Temporal fields for edge tracking
	ExpiredAt *time.Time `json:"expired_at,omitempty"`
	ValidAt   *time.Time `json:"valid_at,omitempty"`
	InvalidAt *time.Time `json:"invalid_at,omitempty"`
	// Resource versioning for sequential agent conflict detection
	Version        int            `json:"version"`
	LastModifiedBy string         `json:"last_modified_by"`
	ChecksumHash   string         `json:"checksum_hash"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	FactEmbedding  []float32      `json:"fact_embedding,omitempty"`
}

// EpisodicNode represents an episodic memory node
type EpisodicNode struct {
	UUID      string         `json:"uuid"`
	GroupID   string         `json:"group_id"`
	Content   string         `json:"content"`
	Source    string         `json:"source"`
	Summary   string         `json:"summary"`
	CreatedAt time.Time      `json:"created_at"`
	ValidAt   time.Time      `json:"valid_at"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Embedding []float32      `json:"embedding,omitempty"`
}

// NewNeo4jClient creates a new Neo4j client
func NewNeo4jClient(config Neo4jConfig, logger *zap.Logger) (*Neo4jClient, error) {
	if config.URI == "" {
		return nil, fmt.Errorf("Neo4j URI is required")
	}

	auth := neo4j.BasicAuth(config.Username, config.Password, "")
	driver, err := neo4j.NewDriverWithContext(config.URI, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	client := &Neo4jClient{
		driver: driver,
		logger: logger,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.driver.VerifyConnectivity(ctx)
	if err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("failed to connect to Neo4j: %w", err)
	}

	// Initialize schema and constraints
	err = client.initializeSchema(ctx, config.Database)
	if err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("failed to initialize Neo4j schema: %w", err)
	}

	logger.Info("Neo4j client initialized successfully",
		zap.String("uri", config.URI),
		zap.String("database", config.Database))

	return client, nil
}

// Close closes the Neo4j driver
func (c *Neo4jClient) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}

// initializeSchema creates necessary constraints and indexes
func (c *Neo4jClient) initializeSchema(ctx context.Context, database string) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: database,
	})
	defer session.Close(ctx)

	// Create constraints and indexes for optimal performance
	constraints := []string{
		// Entity node constraints
		"CREATE CONSTRAINT entity_uuid IF NOT EXISTS FOR (e:Entity) REQUIRE e.uuid IS UNIQUE",
		"CREATE CONSTRAINT entity_name_group IF NOT EXISTS FOR (e:Entity) REQUIRE (e.name, e.group_id) IS UNIQUE",

		// Episodic node constraints
		"CREATE CONSTRAINT episodic_uuid IF NOT EXISTS FOR (ep:Episodic) REQUIRE ep.uuid IS UNIQUE",

		// Edge constraints
		"CREATE CONSTRAINT edge_uuid IF NOT EXISTS FOR ()-[r:RELATES_TO]-() REQUIRE r.uuid IS UNIQUE",
	}

	indexes := []string{
		// Vector indexes for semantic search (384 dimensions for all-MiniLM-L6-v2)
		"CREATE VECTOR INDEX entity_embedding IF NOT EXISTS FOR (e:Entity) ON (e.embedding) OPTIONS {indexConfig: {`vector.dimensions`: 384, `vector.similarity_function`: 'cosine'}}",
		"CREATE VECTOR INDEX episodic_embedding IF NOT EXISTS FOR (ep:Episodic) ON (ep.embedding) OPTIONS {indexConfig: {`vector.dimensions`: 384, `vector.similarity_function`: 'cosine'}}",

		// Text indexes for keyword search
		"CREATE FULLTEXT INDEX entity_text IF NOT EXISTS FOR (e:Entity) ON EACH [e.name, e.summary]",
		"CREATE FULLTEXT INDEX episodic_text IF NOT EXISTS FOR (ep:Episodic) ON EACH [ep.content, ep.summary]",

		// Property indexes for filtering
		"CREATE INDEX entity_group_id IF NOT EXISTS FOR (e:Entity) ON (e.group_id)",
		"CREATE INDEX episodic_group_id IF NOT EXISTS FOR (ep:Episodic) ON (ep.group_id)",
		"CREATE INDEX entity_created_at IF NOT EXISTS FOR (e:Entity) ON (e.created_at)",
		"CREATE INDEX episodic_created_at IF NOT EXISTS FOR (ep:Episodic) ON (ep.created_at)",
	}

	// Execute constraints
	for _, constraint := range constraints {
		_, err := session.Run(ctx, constraint, nil)
		if err != nil {
			c.logger.Warn("Failed to create constraint",
				zap.String("constraint", constraint),
				zap.Error(err))
		}
	}

	// Execute indexes
	for _, index := range indexes {
		_, err := session.Run(ctx, index, nil)
		if err != nil {
			c.logger.Warn("Failed to create index",
				zap.String("index", index),
				zap.Error(err))
		}
	}

	return nil
}

// StoreEntityNode stores an entity node in the knowledge graph
func (c *Neo4jClient) StoreEntityNode(ctx context.Context, node EntityNode, database string) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: database,
	})
	defer session.Close(ctx)

	query := `
		MERGE (e:Entity {uuid: $uuid})
		SET e.name = $name,
		    e.summary = $summary,
		    e.group_id = $group_id,
		    e.created_at = $created_at,
		    e.embedding = $embedding,
		    e.metadata = $metadata
		WITH e
		UNWIND $labels AS label
		CALL apoc.create.addLabels(e, [label]) YIELD node
		RETURN e.uuid
	`

	params := map[string]any{
		"uuid":       node.UUID,
		"name":       node.Name,
		"summary":    node.Summary,
		"group_id":   node.GroupID,
		"created_at": node.CreatedAt,
		"embedding":  node.Embedding,
		"metadata":   node.Metadata,
		"labels":     node.Labels,
	}

	_, err := session.Run(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to store entity node: %w", err)
	}

	c.logger.Debug("Stored entity node",
		zap.String("uuid", node.UUID),
		zap.String("name", node.Name),
		zap.String("group_id", node.GroupID))

	return nil
}

// StoreEdgeNode stores an edge relationship with temporal fields in the knowledge graph
func (c *Neo4jClient) StoreEdgeNode(ctx context.Context, edge EdgeNode, database string) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: database,
	})
	defer session.Close(ctx)

	query := `
		MATCH (source:Entity {uuid: $source_uuid})
		MATCH (target:Entity {uuid: $target_uuid})
		MERGE (source)-[r:RELATES_TO {uuid: $uuid}]->(target)
		SET r.relation_type = $relation_type,
		    r.summary = $summary,
		    r.fact = $fact,
		    r.group_id = $group_id,
		    r.created_at = $created_at,
		    r.episodes = $episodes,
		    r.expired_at = $expired_at,
		    r.valid_at = $valid_at,
		    r.invalid_at = $invalid_at,
		    r.version = $version,
		    r.last_modified_by = $last_modified_by,
		    r.checksum_hash = $checksum_hash,
		    r.fact_embedding = $fact_embedding,
		    r.metadata = $metadata
		RETURN r.uuid
	`

	params := map[string]any{
		"uuid":             edge.UUID,
		"source_uuid":      edge.SourceNodeUUID,
		"target_uuid":      edge.TargetNodeUUID,
		"relation_type":    edge.RelationType,
		"summary":          edge.Summary,
		"fact":             edge.Fact,
		"group_id":         edge.GroupID,
		"created_at":       edge.CreatedAt,
		"episodes":         edge.Episodes,
		"expired_at":       edge.ExpiredAt,
		"valid_at":         edge.ValidAt,
		"invalid_at":       edge.InvalidAt,
		"version":          edge.Version,
		"last_modified_by": edge.LastModifiedBy,
		"checksum_hash":    edge.ChecksumHash,
		"fact_embedding":   edge.FactEmbedding,
		"metadata":         edge.Metadata,
	}

	_, err := session.Run(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to store edge node: %w", err)
	}

	c.logger.Debug("Stored edge node",
		zap.String("uuid", edge.UUID),
		zap.String("relation_type", edge.RelationType),
		zap.String("group_id", edge.GroupID))

	return nil
}

// StoreEpisodicNode stores an episodic memory node
func (c *Neo4jClient) StoreEpisodicNode(ctx context.Context, node EpisodicNode, database string) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: database,
	})
	defer session.Close(ctx)

	query := `
		MERGE (ep:Episodic {uuid: $uuid})
		SET ep.group_id = $group_id,
		    ep.content = $content,
		    ep.source = $source,
		    ep.summary = $summary,
		    ep.created_at = $created_at,
		    ep.valid_at = $valid_at,
		    ep.embedding = $embedding,
		    ep.metadata = $metadata
		RETURN ep.uuid
	`

	params := map[string]any{
		"uuid":       node.UUID,
		"group_id":   node.GroupID,
		"content":    node.Content,
		"source":     node.Source,
		"summary":    node.Summary,
		"created_at": node.CreatedAt,
		"valid_at":   node.ValidAt,
		"embedding":  node.Embedding,
		"metadata":   node.Metadata,
	}

	_, err := session.Run(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to store episodic node: %w", err)
	}

	c.logger.Debug("Stored episodic node",
		zap.String("uuid", node.UUID),
		zap.String("group_id", node.GroupID))

	return nil
}

// SearchSimilarEntities performs vector similarity search on entity nodes
func (c *Neo4jClient) SearchSimilarEntities(ctx context.Context, queryEmbedding []float32, groupIDs []string, limit int, database string) ([]EntityNode, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: database,
	})
	defer session.Close(ctx)

	var query string
	params := map[string]any{
		"embedding": queryEmbedding,
		"limit":     limit,
	}

	if len(groupIDs) > 0 {
		query = `
			CALL db.index.vector.queryNodes('entity_embedding', $limit, $embedding)
			YIELD node, score
			WHERE node.group_id IN $group_ids
			RETURN node.uuid as uuid, node.name as name, node.summary as summary, 
			       node.group_id as group_id, node.created_at as created_at,
			       node.metadata as metadata, node.embedding as embedding,
			       labels(node) as labels, score
			ORDER BY score DESC
		`
		params["group_ids"] = groupIDs
	} else {
		query = `
			CALL db.index.vector.queryNodes('entity_embedding', $limit, $embedding)
			YIELD node, score
			RETURN node.uuid as uuid, node.name as name, node.summary as summary, 
			       node.group_id as group_id, node.created_at as created_at,
			       node.metadata as metadata, node.embedding as embedding,
			       labels(node) as labels, score
			ORDER BY score DESC
		`
	}

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar entities: %w", err)
	}

	var entities []EntityNode
	for result.Next(ctx) {
		record := result.Record()

		entity := EntityNode{
			UUID:      record.Values[0].(string),
			Name:      record.Values[1].(string),
			Summary:   record.Values[2].(string),
			GroupID:   record.Values[3].(string),
			CreatedAt: record.Values[4].(time.Time),
		}

		if metadata, ok := record.Values[5].(map[string]any); ok {
			entity.Metadata = metadata
		}

		if embedding, ok := record.Values[6].([]float32); ok {
			entity.Embedding = embedding
		}

		if labels, ok := record.Values[7].([]any); ok {
			entity.Labels = make([]string, len(labels))
			for i, label := range labels {
				entity.Labels[i] = label.(string)
			}
		}

		entities = append(entities, entity)
	}

	if err = result.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return entities, nil
}

// DeleteGroup deletes all nodes and relationships for a specific group
func (c *Neo4jClient) DeleteGroup(ctx context.Context, groupID string, database string) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: database,
	})
	defer session.Close(ctx)

	// Delete all relationships first, then nodes
	queries := []string{
		"MATCH ()-[r:RELATES_TO {group_id: $group_id}]-() DELETE r",
		"MATCH (e:Entity {group_id: $group_id}) DELETE e",
		"MATCH (ep:Episodic {group_id: $group_id}) DELETE ep",
	}

	params := map[string]any{"group_id": groupID}

	for _, query := range queries {
		_, err := session.Run(ctx, query, params)
		if err != nil {
			return fmt.Errorf("failed to delete group %s: %w", groupID, err)
		}
	}

	c.logger.Info("Deleted group from knowledge graph", zap.String("group_id", groupID))
	return nil
}

// GetEntityByUUID retrieves an entity by its UUID
func (c *Neo4jClient) GetEntityByUUID(ctx context.Context, entityUUID string, database string) (*EntityNode, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: database,
	})
	defer session.Close(ctx)

	query := `
		MATCH (e:Entity {uuid: $uuid})
		RETURN e.uuid as uuid, e.name as name, e.summary as summary, 
		       e.group_id as group_id, e.created_at as created_at,
		       e.metadata as metadata, e.embedding as embedding,
		       labels(e) as labels
	`

	result, err := session.Run(ctx, query, map[string]any{"uuid": entityUUID})
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	if !result.Next(ctx) {
		return nil, fmt.Errorf("entity not found: %s", entityUUID)
	}

	record := result.Record()
	entity := &EntityNode{
		UUID:      record.Values[0].(string),
		Name:      record.Values[1].(string),
		Summary:   record.Values[2].(string),
		GroupID:   record.Values[3].(string),
		CreatedAt: record.Values[4].(time.Time),
	}

	if metadata, ok := record.Values[5].(map[string]any); ok {
		entity.Metadata = metadata
	}

	if embedding, ok := record.Values[6].([]float32); ok {
		entity.Embedding = embedding
	}

	if labels, ok := record.Values[7].([]any); ok {
		entity.Labels = make([]string, len(labels))
		for i, label := range labels {
			entity.Labels[i] = label.(string)
		}
	}

	return entity, nil
}

// HealthCheck performs a basic health check on the Neo4j connection
func (c *Neo4jClient) HealthCheck(ctx context.Context) error {
	return c.driver.VerifyConnectivity(ctx)
}

// GetDriver returns the underlying Neo4j driver for advanced operations
func (c *Neo4jClient) GetDriver() neo4j.DriverWithContext {
	return c.driver
}
