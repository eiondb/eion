package numa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PythonExtractionService manages the embedded Python extraction service
type PythonExtractionService struct {
	logger      *zap.Logger
	pythonPath  string
	scriptPath  string
	mu          sync.RWMutex
	initialized bool
}

// ExtractionRequest represents the request sent to Python extraction service
type ExtractionRequest struct {
	GroupID          string        `json:"group_id"`
	Messages         []Message     `json:"messages"`
	PreviousEpisodes []EpisodeData `json:"previous_episodes,omitempty"`
	EntityTypes      []EntityType  `json:"entity_types,omitempty"`
	UseNuma          bool          `json:"use_numa,omitempty"`
}

// ExtractionResponse represents the response from Python extraction service
type ExtractionResponse struct {
	Success        bool         `json:"success"`
	Error          string       `json:"error,omitempty"`
	ExtractedNodes []EntityNode `json:"extracted_nodes"`
	ExtractedEdges []EdgeNode   `json:"extracted_edges"`
}

// EpisodeData represents episode data for context
type EpisodeData struct {
	UUID      string    `json:"uuid"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	GroupID   string    `json:"group_id"`
}

// EntityType represents entity type configuration
type EntityType struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// EntityNode represents extracted entity node
type EntityNode struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	GroupID   string    `json:"group_id"`
	Labels    []string  `json:"labels"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}

// EdgeNode represents extracted edge/relationship
type EdgeNode struct {
	UUID         string    `json:"uuid"`
	SourceUUID   string    `json:"source_uuid"`
	TargetUUID   string    `json:"target_uuid"`
	RelationType string    `json:"relation_type"`
	Summary      string    `json:"summary"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewPythonExtractionService creates a new Python extraction service
func NewPythonExtractionService(logger *zap.Logger) (*PythonExtractionService, error) {
	// Find Python executable
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		pythonPath, err = exec.LookPath("python")
		if err != nil {
			return nil, fmt.Errorf("python executable not found: %w", err)
		}
	}

	// Set script path relative to the current directory
	scriptPath := filepath.Join("python", "extraction_service.py")

	service := &PythonExtractionService{
		logger:     logger,
		pythonPath: pythonPath,
		scriptPath: scriptPath,
	}

	// Test if the service can be initialized
	if err := service.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize Python service: %w", err)
	}

	return service, nil
}

// initialize tests if the Python service can be started
func (p *PythonExtractionService) initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	// Skip initialization for mock mode
	if p.pythonPath == "mock" {
		p.initialized = true
		return nil
	}

	// Test command to verify Python service is available
	cmd := exec.Command(p.pythonPath, p.scriptPath, "--test")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("python service test failed: %w, output: %s", err, string(output))
	}

	p.initialized = true
	p.logger.Info("Python extraction service initialized successfully")
	return nil
}

// ExtractKnowledge extracts knowledge from messages using the Python service
func (p *PythonExtractionService) ExtractKnowledge(ctx context.Context, request ExtractionRequest) (*ExtractionResponse, error) {
	if err := p.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize python service: %w", err)
	}

	// Serialize request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Execute Python script with JSON input
	cmd := exec.CommandContext(ctx, p.pythonPath, p.scriptPath)
	cmd.Stdin = bytes.NewReader(requestJSON)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("python extraction failed: %w, output: %s", err, string(output))
	}

	// Parse response
	var response ExtractionResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse python response: %w, output: %s", err, string(output))
	}

	if !response.Success {
		return nil, fmt.Errorf("python extraction failed: %s", response.Error)
	}

	return &response, nil
}

// IsAvailable checks if the Python service is available
func (p *PythonExtractionService) IsAvailable() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.initialized
}
