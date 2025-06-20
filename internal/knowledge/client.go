package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// KnowledgeClient calls the Python knowledge extraction service
type KnowledgeClient struct {
	pythonPath    string
	servicePath   string
	neo4jURI      string
	neo4jUser     string
	neo4jPassword string
	openaiAPIKey  string
	logger        *zap.Logger
	initialized   bool
}

// KnowledgeEpisodeResult represents the result from adding an episode
type KnowledgeEpisodeResult struct {
	EpisodeUUID  string   `json:"episode_uuid"`
	EpisodeName  string   `json:"episode_name"`
	NodesCreated int      `json:"nodes_created"`
	EdgesCreated int      `json:"edges_created"`
	NodeUUIDs    []string `json:"node_uuids"`
	EdgeUUIDs    []string `json:"edge_uuids"`
}

// KnowledgeSearchResult represents search results
type KnowledgeSearchResult struct {
	Results []KnowledgeEdge `json:"results"`
	Count   int             `json:"count"`
}

// KnowledgeEdge represents an edge in the knowledge graph
type KnowledgeEdge struct {
	UUID           string   `json:"uuid"`
	Name           string   `json:"name"`
	Fact           string   `json:"fact"`
	SourceNodeUUID string   `json:"source_node_uuid"`
	TargetNodeUUID string   `json:"target_node_uuid"`
	CreatedAt      string   `json:"created_at"`
	ValidAt        *string  `json:"valid_at"`
	Episodes       []string `json:"episodes"`
}

// KnowledgeEpisodesResult represents episodes
type KnowledgeEpisodesResult struct {
	Episodes []KnowledgeEpisode `json:"episodes"`
	Count    int                `json:"count"`
}

// KnowledgeEpisode represents an episode
type KnowledgeEpisode struct {
	UUID              string   `json:"uuid"`
	Name              string   `json:"name"`
	Content           string   `json:"content"`
	Source            string   `json:"source"`
	SourceDescription string   `json:"source_description"`
	CreatedAt         string   `json:"created_at"`
	ValidAt           string   `json:"valid_at"`
	GroupID           string   `json:"group_id"`
	EntityEdges       []string `json:"entity_edges"`
}

// NewKnowledgeClient creates a new knowledge graph client
func NewKnowledgeClient(pythonPath, servicePath, neo4jURI, neo4jUser, neo4jPassword, openaiAPIKey string, logger *zap.Logger) *KnowledgeClient {
	return &KnowledgeClient{
		pythonPath:    pythonPath,
		servicePath:   servicePath,
		neo4jURI:      neo4jURI,
		neo4jUser:     neo4jUser,
		neo4jPassword: neo4jPassword,
		openaiAPIKey:  openaiAPIKey,
		logger:        logger,
		initialized:   false,
	}
}

// Initialize initializes the knowledge extraction service
func (g *KnowledgeClient) Initialize(ctx context.Context) error {
	if g.initialized {
		return nil
	}

	g.logger.Info("Initializing knowledge extraction service")

	// Call our knowledge service with --test flag
	cmd := exec.CommandContext(ctx, g.pythonPath, g.servicePath, "--test")
	cmd.Dir = "." // Set working directory to eion root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		g.logger.Error("Failed to initialize knowledge extraction service",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()))
		return fmt.Errorf("failed to initialize knowledge extraction service: %w", err)
	}

	// Check if the output contains SUCCESS
	output := stdout.String()
	if !strings.Contains(output, "SUCCESS") {
		return fmt.Errorf("knowledge extraction service initialization failed: %s", output)
	}

	g.initialized = true
	g.logger.Info("Knowledge extraction service initialized successfully")
	return nil
}

// AddEpisode adds an episode to the knowledge graph
func (g *KnowledgeClient) AddEpisode(ctx context.Context, name, content, sourceDescription, groupID, episodeType string) (*KnowledgeEpisodeResult, error) {
	if !g.initialized {
		return nil, fmt.Errorf("knowledge extraction client not initialized")
	}

	g.logger.Debug("Adding episode to knowledge graph",
		zap.String("name", name),
		zap.String("content", content[:min(100, len(content))]),
		zap.String("groupID", groupID),
		zap.String("episodeType", episodeType))

	// Build command arguments
	args := []string{g.servicePath, "add_episode", name, content, sourceDescription}
	if groupID != "" {
		args = append(args, groupID)
	} else {
		args = append(args, "")
	}
	if episodeType != "" {
		args = append(args, episodeType)
	}

	// Execute command
	cmd := exec.CommandContext(ctx, g.pythonPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		g.logger.Error("Failed to add episode",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()))
		return nil, fmt.Errorf("failed to add episode: %w", err)
	}

	// Parse result
	var result KnowledgeEpisodeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		g.logger.Error("Failed to parse episode result",
			zap.Error(err),
			zap.String("output", stdout.String()))
		return nil, fmt.Errorf("failed to parse episode result: %w", err)
	}

	g.logger.Debug("Episode added successfully",
		zap.String("episodeUUID", result.EpisodeUUID),
		zap.Int("nodesCreated", result.NodesCreated),
		zap.Int("edgesCreated", result.EdgesCreated))

	return &result, nil
}

// Search searches the knowledge graph
func (g *KnowledgeClient) Search(ctx context.Context, query string, groupIDs []string, numResults int) (*KnowledgeSearchResult, error) {
	if !g.initialized {
		return nil, fmt.Errorf("knowledge extraction client not initialized")
	}

	g.logger.Debug("Searching knowledge graph",
		zap.String("query", query),
		zap.Strings("groupIDs", groupIDs),
		zap.Int("numResults", numResults))

	// Build command arguments
	args := []string{g.servicePath, "search", query}
	if len(groupIDs) > 0 {
		args = append(args, strings.Join(groupIDs, ","))
	} else {
		args = append(args, "")
	}
	if numResults > 0 {
		args = append(args, fmt.Sprintf("%d", numResults))
	}

	// Execute command
	cmd := exec.CommandContext(ctx, g.pythonPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		g.logger.Error("Failed to search knowledge graph",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()))
		return nil, fmt.Errorf("failed to search knowledge graph: %w", err)
	}

	// Parse result
	var result KnowledgeSearchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		g.logger.Error("Failed to parse search result",
			zap.Error(err),
			zap.String("output", stdout.String()))
		return nil, fmt.Errorf("failed to parse search result: %w", err)
	}

	g.logger.Debug("Search completed",
		zap.Int("resultsCount", result.Count))

	return &result, nil
}

// GetEpisodes retrieves episodes using Knowledge directly
func (g *KnowledgeClient) GetEpisodes(ctx context.Context, groupIDs []string, lastN int) (*KnowledgeEpisodesResult, error) {
	if !g.initialized {
		return nil, fmt.Errorf("Knowledge client not initialized")
	}

	g.logger.Debug("Getting episodes from knowledge graph",
		zap.Strings("groupIDs", groupIDs),
		zap.Int("lastN", lastN))

	// Build command arguments
	args := []string{g.servicePath, "get_episodes"}
	if len(groupIDs) > 0 {
		args = append(args, strings.Join(groupIDs, ","))
	} else {
		args = append(args, "")
	}
	if lastN > 0 {
		args = append(args, fmt.Sprintf("%d", lastN))
	}

	// Execute command
	cmd := exec.CommandContext(ctx, g.pythonPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		g.logger.Error("Failed to get episodes",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()))
		return nil, fmt.Errorf("failed to get episodes: %w", err)
	}

	// Parse result
	var result KnowledgeEpisodesResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		g.logger.Error("Failed to parse episodes result",
			zap.Error(err),
			zap.String("output", stdout.String()))
		return nil, fmt.Errorf("failed to parse episodes result: %w", err)
	}

	g.logger.Debug("Episodes retrieved",
		zap.Int("episodesCount", result.Count))

	return &result, nil
}

// HealthCheck checks if the service is healthy
func (g *KnowledgeClient) HealthCheck(ctx context.Context) error {
	if !g.initialized {
		// Try to initialize if not already done
		if err := g.Initialize(ctx); err != nil {
			return err
		}
	}

	// Call the Python service health check
	cmd := exec.CommandContext(ctx, g.pythonPath, g.servicePath, "--health")
	cmd.Dir = "."

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		g.logger.Error("Health check failed",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()))
		return fmt.Errorf("health check failed: %w", err)
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return fmt.Errorf("failed to parse health check response: %w", err)
	}

	status, ok := result["status"].(string)
	if !ok || status != "healthy" {
		errorMsg, _ := result["error"].(string)
		return fmt.Errorf("service unhealthy: %s", errorMsg)
	}

	return nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
