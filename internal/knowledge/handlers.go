package knowledge

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/eion/eion/internal/memory"
	"github.com/eion/eion/internal/numa"
)

// KnowledgeHandlers provides HTTP handlers for knowledge operations at session level
type KnowledgeHandlers struct {
	memoryService memory.MemoryService
	logger        *zap.Logger
}

// NewKnowledgeHandlers creates new knowledge handlers
func NewKnowledgeHandlers(memoryService memory.MemoryService, logger *zap.Logger) *KnowledgeHandlers {
	return &KnowledgeHandlers{
		memoryService: memoryService,
		logger:        logger,
	}
}

// RegisterRoutes registers knowledge management routes with conflict resolution
// Note: This is now integrated into the main server router in cmd/eion-server/main.go
// keeping this for backwards compatibility or alternative usage
func (h *KnowledgeHandlers) RegisterRoutes(router *gin.RouterGroup) {
	knowledge := router.Group("/knowledge")
	{
		knowledge.GET("/:sessionId", h.SearchKnowledge)    // Session-level knowledge search
		knowledge.POST("/:sessionId", h.PostKnowledge)     // Session-level knowledge storage
		knowledge.PUT("/:sessionId", h.PutKnowledge)       // Session-level knowledge update
		knowledge.DELETE("/:sessionId", h.DeleteKnowledge) // Session-level knowledge deletion
	}

	// Conflict resolution endpoints for sequential agent operations
	conflicts := router.Group("/conflicts")
	{
		conflicts.GET("/:spaceId", h.GetConflicts)                // List unresolved conflicts
		conflicts.POST("/:conflictId/resolve", h.ResolveConflict) // Resolve specific conflict
	}
}

// SearchKnowledge retrieves knowledge with optional version information for conflict detection
func (h *KnowledgeHandlers) SearchKnowledge(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")
	userID := c.Query("user_id")
	query := c.Query("query")

	if sessionID == "" || agentID == "" || userID == "" {
		c.JSON(400, gin.H{
			"error": "sessionId, agent_id, and user_id are required",
			"hint":  "Add ?agent_id=YOUR_AGENT&user_id=YOUR_USER to the URL",
		})
		return
	}

	if query == "" {
		c.JSON(400, gin.H{
			"error": "query parameter is required",
			"hint":  "Add ?query=YOUR_SEARCH_QUERY to the URL",
		})
		return
	}

	ctx := c.Request.Context()

	// Parse limit parameter (replaces the confusing last_n)
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Perform semantic search
	searchQuery := &memory.MemorySearchQuery{
		Text:     query,
		AgentID:  agentID,
		Limit:    limit,
		MinScore: 0.7,
	}

	result, err := h.memoryService.SearchMemory(ctx, searchQuery)
	if err != nil {
		h.logger.Error("Failed to search knowledge", zap.String("session_id", sessionID), zap.Error(err))
		c.JSON(500, gin.H{"error": "Failed to search knowledge"})
		return
	}

	response := gin.H{
		"session_id":  sessionID,
		"query":       query,
		"messages":    result.Messages,
		"facts":       result.Facts,
		"total_count": result.TotalCount,
	}

	// Include version information for sequential agent conflict detection
	includeVersion := c.Query("include_version") == "true"
	if includeVersion {
		// For now, include basic version information
		// In production, would call the version tracking methods
		response["version"] = 1
		response["last_modified_by"] = agentID
		h.logger.Debug("Version information included in search response")
	}

	c.JSON(200, response)
}

// PostKnowledge stores new knowledge for a specific session with automatic conflict resolution integrated in pipeline
func (h *KnowledgeHandlers) PostKnowledge(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")
	userID := c.Query("user_id")

	if sessionID == "" || agentID == "" || userID == "" {
		c.JSON(400, gin.H{
			"error": "sessionId, agent_id, and user_id are required",
			"hint":  "Add ?agent_id=YOUR_AGENT&user_id=YOUR_USER to the URL",
		})
		return
	}

	var req memory.AddMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(400, gin.H{"error": "At least one message is required"})
		return
	}

	ctx := c.Request.Context()

	// Create memory object
	memory := &memory.Memory{
		Messages: req.Messages,
		AgentID:  agentID,
		Metadata: make(map[string]any),
	}

	// Store with automatic conflict resolution integrated in the main pipeline (Graphiti-style)
	err := h.memoryService.PutMemory(ctx, sessionID, agentID, memory, false)
	if err != nil {
		h.logger.Error("Failed to store knowledge with automatic conflict resolution",
			zap.String("session_id", sessionID),
			zap.String("agent_id", agentID),
			zap.Error(err))
		c.JSON(500, gin.H{"error": "Failed to store knowledge"})
		return
	}

	h.logger.Info("Knowledge stored successfully with automatic conflict resolution",
		zap.String("session_id", sessionID),
		zap.String("agent_id", agentID),
		zap.Int("messages_count", len(req.Messages)))

	response := gin.H{
		"message":             "Knowledge stored successfully with automatic conflict resolution",
		"session_id":          sessionID,
		"messages_count":      len(req.Messages),
		"conflict_resolution": "automatic", // Indicate that conflicts are automatically resolved
	}

	c.JSON(201, response)
}

// PutKnowledge updates knowledge for a specific session with automatic conflict resolution integrated in pipeline
func (h *KnowledgeHandlers) PutKnowledge(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")
	userID := c.Query("user_id")

	if sessionID == "" || agentID == "" || userID == "" {
		c.JSON(400, gin.H{
			"error": "sessionId, agent_id, and user_id are required",
			"hint":  "Add ?agent_id=YOUR_AGENT&user_id=YOUR_USER to the URL",
		})
		return
	}

	var req memory.AddMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(400, gin.H{"error": "At least one message is required"})
		return
	}

	ctx := c.Request.Context()

	// Create memory object for update
	memory := &memory.Memory{
		Messages: req.Messages,
		AgentID:  agentID,
		Metadata: make(map[string]any),
	}

	// Update with automatic conflict resolution integrated in the main pipeline (Graphiti-style)
	err := h.memoryService.PutMemory(ctx, sessionID, agentID, memory, false)
	if err != nil {
		h.logger.Error("Failed to update knowledge with automatic conflict resolution",
			zap.String("session_id", sessionID),
			zap.String("agent_id", agentID),
			zap.Error(err))
		c.JSON(500, gin.H{"error": "Failed to update knowledge"})
		return
	}

	h.logger.Info("Knowledge updated successfully with automatic conflict resolution",
		zap.String("session_id", sessionID),
		zap.String("agent_id", agentID),
		zap.Int("messages_count", len(req.Messages)))

	response := gin.H{
		"message":             "Knowledge updated successfully with automatic conflict resolution",
		"session_id":          sessionID,
		"messages_count":      len(req.Messages),
		"conflict_resolution": "automatic", // Indicate that conflicts are automatically resolved
	}

	c.JSON(200, response)
}

// DeleteKnowledge deletes knowledge for a specific session
func (h *KnowledgeHandlers) DeleteKnowledge(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")
	userID := c.Query("user_id")

	if sessionID == "" || agentID == "" || userID == "" {
		c.JSON(400, gin.H{
			"error": "sessionId, agent_id, and user_id are required",
			"hint":  "Add ?agent_id=YOUR_AGENT&user_id=YOUR_USER to the URL",
		})
		return
	}

	ctx := c.Request.Context()

	// Delete all messages for the session
	messages, err := h.memoryService.GetMessages(ctx, sessionID, agentID, -1, uuid.Nil)
	if err != nil {
		h.logger.Error("Failed to get messages for deletion", zap.String("session_id", sessionID), zap.Error(err))
		c.JSON(500, gin.H{"error": "Failed to delete knowledge"})
		return
	}

	var messageUUIDs []uuid.UUID
	for _, msg := range messages {
		messageUUIDs = append(messageUUIDs, msg.UUID)
	}

	err = h.memoryService.DeleteMessages(ctx, sessionID, agentID, messageUUIDs)
	if err != nil {
		h.logger.Error("Failed to delete messages", zap.String("session_id", sessionID), zap.Error(err))
		c.JSON(500, gin.H{"error": "Failed to delete knowledge"})
		return
	}

	c.JSON(200, gin.H{
		"message":       "Knowledge deleted successfully",
		"session_id":    sessionID,
		"deleted_count": len(messageUUIDs),
	})
}

// GetConflicts lists unresolved conflicts for a space (Week 3 requirement)
func (h *KnowledgeHandlers) GetConflicts(c *gin.Context) {
	spaceID := c.Param("spaceId")
	if spaceID == "" {
		c.JSON(400, gin.H{"error": "spaceId is required"})
		return
	}

	// For now, return empty list as conflicts are handled immediately
	// In a production system, this would query a conflicts table
	c.JSON(200, gin.H{
		"space_id":  spaceID,
		"conflicts": []interface{}{},
		"count":     0,
		"message":   "Conflicts are resolved automatically or require manual intervention",
	})
}

// ResolveConflict resolves a specific conflict (Week 3 requirement)
func (h *KnowledgeHandlers) ResolveConflict(c *gin.Context) {
	conflictID := c.Param("conflictId")
	if conflictID == "" {
		c.JSON(400, gin.H{"error": "conflictId is required"})
		return
	}

	var req struct {
		Strategy   string `json:"strategy"`    // "last_writer_wins", "retry", "manual"
		ResolvedBy string `json:"resolved_by"` // Agent ID resolving the conflict
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	if req.Strategy == "" {
		req.Strategy = "last_writer_wins" // Default strategy
	}
	if req.ResolvedBy == "" {
		req.ResolvedBy = "system" // Default resolver
	}

	ctx := c.Request.Context()

	// Use temporal operations for conflict resolution
	if knowledgeService, ok := h.memoryService.(interface {
		ResolveConflict(ctx context.Context, conflict numa.ConflictDetection, strategy string, resolvedBy string) (numa.ConflictResolution, error)
	}); ok {
		// Create a mock conflict for resolution (in production, would lookup from database)
		conflict := numa.ConflictDetection{
			ConflictID:       conflictID,
			ResourceID:       "unknown",
			ExpectedVersion:  0,
			ActualVersion:    1,
			ConflictingAgent: "unknown",
			DetectedAt:       time.Now(),
			Status:           "detected",
		}

		resolution, err := knowledgeService.ResolveConflict(ctx, conflict, req.Strategy, req.ResolvedBy)
		if err != nil {
			h.logger.Error("Failed to resolve conflict", zap.String("conflict_id", conflictID), zap.Error(err))
			c.JSON(500, gin.H{"error": "Failed to resolve conflict"})
			return
		}

		c.JSON(200, gin.H{
			"message":     "Conflict resolved successfully",
			"conflict_id": conflictID,
			"resolution":  resolution,
		})
		return
	}

	c.JSON(500, gin.H{"error": "Temporal operations not available"})
}
