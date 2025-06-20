package memory

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MemoryHandlers provides HTTP handlers for memory operations
type MemoryHandlers struct {
	memoryService MemoryService
}

// NewMemoryHandlers creates new memory handlers
func NewMemoryHandlers(memoryService MemoryService) *MemoryHandlers {
	return &MemoryHandlers{
		memoryService: memoryService,
	}
}

// RegisterRoutes registers all memory-related routes
// Note: This is now integrated into the main server router in cmd/eion-server/main.go
// keeping this for backwards compatibility or alternative usage
func (h *MemoryHandlers) RegisterRoutes(router *gin.RouterGroup) {
	// Memory routes (Legacy - now handled in main server)
	memory := router.Group("/memory")
	{
		memory.GET("/:sessionId", h.GetMemory)
		memory.POST("/:sessionId", h.PutMemory)
		memory.GET("/:sessionId/search", h.SearchMemory)
	}

	// Message routes (Legacy - now handled in main server)
	messages := router.Group("/messages")
	{
		messages.GET("/:sessionId", h.GetMessages)
		messages.GET("/:sessionId/list", h.GetMessageList)
		messages.POST("/:sessionId", h.PutMessages)
		messages.PUT("/:sessionId", h.UpdateMessages)
		messages.DELETE("/:sessionId", h.DeleteMessages)
	}

	// Fact routes (Legacy - now handled in main server)
	facts := router.Group("/facts")
	{
		facts.GET("", h.GetFacts)
		facts.POST("", h.PutFact)
		facts.PUT("/:factId", h.UpdateFact)
		facts.DELETE("/:factId", h.DeleteFact)
	}

	// Health check
	router.GET("/health", h.HealthCheck)
}

// Memory handlers

func (h *MemoryHandlers) GetMemory(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")

	if sessionID == "" || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId and agent_id are required"})
		return
	}

	lastN, _ := strconv.Atoi(c.Query("last_n"))
	if lastN <= 0 {
		lastN = DefaultLastNMessages
	}

	minRating, _ := strconv.ParseFloat(c.Query("min_rating"), 64)

	var opts []FilterOption
	if minRating > 0 {
		opts = append(opts, FilterOption{MinRating: &minRating})
	}

	memory, err := h.memoryService.GetMemory(c.Request.Context(), sessionID, agentID, lastN, opts...)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get memory", "details": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, memory)
}

func (h *MemoryHandlers) PutMemory(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")

	if sessionID == "" || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId and agent_id are required"})
		return
	}

	var req AddMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one message is required"})
		return
	}

	if len(req.Messages) > MaxMessagesPerMemory {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Too many messages (max %d)", MaxMessagesPerMemory)})
		return
	}

	// Validate messages
	for i, msg := range req.Messages {
		if msg.Content == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Message %d content cannot be empty", i)})
			return
		}
		if len(msg.Content) > MaxMessageLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Message %d content too long (max %d characters)", i, MaxMessageLength)})
			return
		}
		if msg.RoleType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Message %d role_type is required", i)})
			return
		}
	}

	memory := &Memory{
		Messages:  req.Messages,
		Metadata:  req.Metadata,
		AgentID:   agentID,
		SessionID: sessionID,
	}

	skipProcessing := c.Query("skip_processing") == "true"

	err := h.memoryService.PutMemory(c.Request.Context(), sessionID, agentID, memory, skipProcessing)
	if err != nil {
		if strings.Contains(err.Error(), "session has ended") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store memory", "details": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Memory stored successfully"})
}

func (h *MemoryHandlers) SearchMemory(c *gin.Context) {
	query := c.Query("q")
	agentID := c.Query("agent_id")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query (q) is required"})
		return
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	minScore, _ := strconv.ParseFloat(c.Query("min_score"), 64)

	searchQuery := &MemorySearchQuery{
		Text:     query,
		AgentID:  agentID,
		Limit:    limit,
		MinScore: minScore,
	}

	result, err := h.memoryService.SearchMemory(c.Request.Context(), searchQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search memory", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Message handlers

func (h *MemoryHandlers) GetMessages(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")

	if sessionID == "" || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId and agent_id are required"})
		return
	}

	lastN, _ := strconv.Atoi(c.Query("last_n"))
	beforeUUIDStr := c.Query("before_uuid")

	var beforeUUID uuid.UUID
	if beforeUUIDStr != "" {
		var err error
		beforeUUID, err = uuid.Parse(beforeUUIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid before_uuid format"})
			return
		}
	}

	messages, err := h.memoryService.GetMessages(c.Request.Context(), sessionID, agentID, lastN, beforeUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get messages", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"count":    len(messages),
	})
}

func (h *MemoryHandlers) GetMessageList(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")

	if sessionID == "" || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId and agent_id are required"})
		return
	}

	pageNumber, _ := strconv.Atoi(c.Query("page"))
	if pageNumber < 1 {
		pageNumber = 1
	}

	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	result, err := h.memoryService.GetMessageList(c.Request.Context(), sessionID, agentID, pageNumber, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get message list", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *MemoryHandlers) PutMessages(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")

	if sessionID == "" || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId and agent_id are required"})
		return
	}

	var messages []Message
	if err := c.ShouldBindJSON(&messages); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if len(messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one message is required"})
		return
	}

	result, err := h.memoryService.PutMessages(c.Request.Context(), sessionID, agentID, messages)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store messages", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"messages": result,
		"count":    len(result),
	})
}

func (h *MemoryHandlers) UpdateMessages(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")

	if sessionID == "" || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId and agent_id are required"})
		return
	}

	var messages []Message
	if err := c.ShouldBindJSON(&messages); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	includeContent := c.Query("include_content") == "true"

	err := h.memoryService.UpdateMessages(c.Request.Context(), sessionID, agentID, messages, includeContent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update messages", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Messages updated successfully"})
}

func (h *MemoryHandlers) DeleteMessages(c *gin.Context) {
	sessionID := c.Param("sessionId")
	agentID := c.Query("agent_id")

	if sessionID == "" || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId and agent_id are required"})
		return
	}

	var request struct {
		MessageUUIDs []string `json:"message_uuids"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if len(request.MessageUUIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one message UUID is required"})
		return
	}

	messageUUIDs := make([]uuid.UUID, len(request.MessageUUIDs))
	for i, uuidStr := range request.MessageUUIDs {
		messageUUID, err := uuid.Parse(uuidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid UUID format: %s", uuidStr)})
			return
		}
		messageUUIDs[i] = messageUUID
	}

	err := h.memoryService.DeleteMessages(c.Request.Context(), sessionID, agentID, messageUUIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete messages", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Messages deleted successfully"})
}

// Fact handlers

func (h *MemoryHandlers) GetFacts(c *gin.Context) {
	agentID := c.Query("agent_id")
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	minRating, _ := strconv.ParseFloat(c.Query("min_rating"), 64)

	var opts []FilterOption
	if minRating > 0 {
		opts = append(opts, FilterOption{MinRating: &minRating})
	}

	facts, err := h.memoryService.GetFacts(c.Request.Context(), agentID, limit, opts...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get facts", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"facts": facts,
		"count": len(facts),
	})
}

func (h *MemoryHandlers) PutFact(c *gin.Context) {
	var fact Fact
	if err := c.ShouldBindJSON(&fact); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if fact.Content == "" || fact.AgentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content and agent_id are required"})
		return
	}

	err := h.memoryService.PutFact(c.Request.Context(), &fact)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store fact", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, fact)
}

func (h *MemoryHandlers) UpdateFact(c *gin.Context) {
	factIDStr := c.Param("factId")
	factUUID, err := uuid.Parse(factIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid fact ID format"})
		return
	}

	var fact Fact
	if err := c.ShouldBindJSON(&fact); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	fact.UUID = factUUID

	err = h.memoryService.UpdateFact(c.Request.Context(), &fact)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update fact", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Fact updated successfully"})
}

func (h *MemoryHandlers) DeleteFact(c *gin.Context) {
	factIDStr := c.Param("factId")
	factUUID, err := uuid.Parse(factIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid fact ID format"})
		return
	}

	err = h.memoryService.DeleteFact(c.Request.Context(), factUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete fact", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Fact deleted successfully"})
}

// Health check handler

func (h *MemoryHandlers) HealthCheck(c *gin.Context) {
	err := h.memoryService.HealthCheck(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "eion-memory",
	})
}
