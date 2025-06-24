package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/eion/eion/internal/config"
	"github.com/eion/eion/internal/graph"
	"github.com/eion/eion/internal/knowledge"
	"github.com/eion/eion/internal/memory"
	"github.com/eion/eion/internal/numa"
	"github.com/eion/eion/internal/orchestrator"
	"github.com/eion/eion/internal/orchestrator/agentgroups"
	"github.com/eion/eion/internal/orchestrator/agents"
	"github.com/eion/eion/internal/orchestrator/logging"
	"github.com/eion/eion/internal/orchestrator/sessions"
	"github.com/eion/eion/internal/orchestrator/sessiontypes"
	"github.com/eion/eion/internal/orchestrator/users"
)

// AppState holds all application services
type AppState struct {
	MemoryService      memory.MemoryService
	Logger             *zap.Logger
	Config             *config.Config
	Orchestrator       *orchestrator.Orchestrator
	AgentService       agents.AgentManager
	AgentGroupService  agentgroups.AgentGroupManager
	SessionService     sessions.SessionManager
	SessionTypeService sessiontypes.SessionTypeManager
	UserService        users.UserService
}

func main() {
	// Load configuration
	config.Load()

	// Initialize logger with config
	logger := initLogger()
	logger.Info("Configuration loaded", zap.String("source", "config.Load()"))

	// Initialize application state
	as, err := newAppState(logger)
	if err != nil {
		logger.Fatal("Failed to initialize application state", zap.Error(err))
	}

	// Numa service integration handled through knowledge extraction service

	// Initialize services
	ctx := context.Background()
	if err := as.MemoryService.Initialize(ctx); err != nil {
		logger.Fatal("Failed to initialize memory service", zap.Error(err))
	}

	// Create HTTP server
	router := setupRouter(as)

	// Server configuration from config
	addr := fmt.Sprintf("%s:%d", config.Http().Host, config.Http().Port)

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Setup graceful shutdown
	done := setupSignalHandler(as, server, logger)

	// Setup default data
	ctx = context.Background()
	err = orchestrator.SetupDefaults(ctx, as.AgentGroupService, as.SessionTypeService)
	if err != nil {
		logger.Error("Failed to setup defaults", zap.Error(err))
		// Continue anyway - defaults are not critical for basic operation
	} else {
		logger.Info("Default data setup completed successfully")
	}

	// Start server
	logger.Info("Starting Eion server", zap.String("address", addr))

	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal("Failed to start server", zap.Error(err))
	}

	<-done
	logger.Info("Server shutdown complete")
}

// newAppState creates and initializes the application state
func newAppState(logger *zap.Logger) (*AppState, error) {
	// Get database configuration - use defaults if not specified (for Week 1.2 Subphase 1)
	pgConfig := config.Postgres()

	logger.Info("Database configuration",
		zap.String("host", pgConfig.Host),
		zap.Int("port", pgConfig.Port),
		zap.String("database", pgConfig.Database),
		zap.String("user", pgConfig.User))

	// Get Neo4j configuration - MANDATORY for Eion
	numaConfig := config.Numa()
	neo4jConfig := graph.Neo4jConfig{
		URI:      numaConfig.Neo4j.URI,
		Username: numaConfig.Neo4j.Username,
		Password: numaConfig.Neo4j.Password,
		Database: numaConfig.Neo4j.Database,
	}

	// Validate Neo4j configuration
	if neo4jConfig.URI == "" {
		return nil, fmt.Errorf("Neo4j URI is required for Eion - please configure numa.neo4j.uri")
	}

	// Create memory service configuration with knowledge graph integration
	memoryConfig := &knowledge.KnowledgeMemoryServiceConfig{
		DatabaseURL:      pgConfig.DSN(),
		MaxConnections:   pgConfig.MaxOpenConnections,
		EnableMigrations: true,

		// Knowledge graph configuration
		Neo4jURI:      neo4jConfig.URI,
		Neo4jUser:     neo4jConfig.Username,
		Neo4jPassword: neo4jConfig.Password,
		OpenAIAPIKey:  "", // Optional - using default embedder

		// Python paths for extraction service
		PythonPath:           ".venv/bin/python",
		KnowledgeServicePath: "internal/knowledge/python/knowledge_service.py",

		// Embedding configuration
		EmbeddingConfig: numa.EmbeddingServiceConfig{
			Provider:  "local",            // Use all-MiniLM-L6-v2 by default
			Dimension: 384,                // all-MiniLM-L6-v2 dimensions
			Model:     "all-MiniLM-L6-v2", // Explicit model name
		},
		TokenCounterType: "simple",
	}

	// Initialize knowledge-enhanced memory service
	memoryService, err := knowledge.NewKnowledgeMemoryService(memoryConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge-enhanced memory service: %w", err)
	}

	// Initialize orchestrator with interaction logging
	// Get database from memory service
	db := memoryService.GetDB()

	interactionStore := logging.NewPostgresInteractionStore(db)

	var orchestratorService *orchestrator.Orchestrator
	orchestratorService, err = orchestrator.NewOrchestrator(
		config.Get(),
		interactionStore,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Initialize agent group service with database
	agentGroupStore := agentgroups.NewPostgresStore(db)
	agentGroupService := agentgroups.NewService(agentGroupStore)

	// Initialize agent service with database and agent group validation
	agentStore := agents.NewPostgresStore(db)
	agentService := agents.NewServiceWithAgentGroups(agentStore, agentGroupStore)

	// Initialize user service with database
	userStore := users.NewUserStore(db)
	userService := users.NewUserService(userStore)

	// Initialize session type service with PostgreSQL store
	sessionTypeStore := sessiontypes.NewPostgresStore(db)
	sessionTypeService := sessiontypes.NewSessionTypeService(sessionTypeStore)

	// Initialize session service with database
	sessionStore := sessions.NewPostgresStore(db)
	sessionService := sessions.NewService(sessionStore)

	return &AppState{
		MemoryService:      memoryService,
		Logger:             logger,
		Config:             config.Get(),
		Orchestrator:       orchestratorService,
		AgentService:       agentService,
		AgentGroupService:  agentGroupService,
		SessionService:     sessionService,
		SessionTypeService: sessionTypeService,
		UserService:        userService,
	}, nil
}

func initLogger() *zap.Logger {
	logConfig := config.Logger()

	var config zap.Config
	if logConfig.Format == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	// Set log level
	switch logConfig.Level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	return logger
}

// AccessControlMiddleware enforces proper scoping between developers and agents
func AccessControlMiddleware(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		// Skip auth for health endpoints (handled outside API group)
		if strings.HasPrefix(path, "/health") {
			c.Next()
			return
		}

		// Check if this is a developer-only endpoint (global scope)
		if isDeveloperOnlyEndpoint(path, method) {
			// Require developer authentication
			authHeader := c.GetHeader("Authorization")
			if !isValidDeveloperAuth(authHeader) {
				as.Logger.Warn("Unauthorized access to developer endpoint",
					zap.String("path", path),
					zap.String("method", method),
					zap.String("remote_addr", c.Request.RemoteAddr))

				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Developer authentication required for global user management",
					"scope": "developer_only",
					"hint":  "Use 'Authorization: Bearer dev_key_eion_2025' header",
				})
				c.Abort()
				return
			}

			as.Logger.Info("Developer access granted",
				zap.String("path", path),
				zap.String("method", method))

			// Developer authenticated for cluster endpoint - proceed
			c.Next()
			return
		}

		// Check if this is an agent-only endpoint (session scope)
		if isAgentOnlyEndpoint(path, method) {
			// Block developer access to session endpoints
			authHeader := c.GetHeader("Authorization")
			if isValidDeveloperAuth(authHeader) {
				as.Logger.Warn("Developer trying to access session endpoint",
					zap.String("path", path),
					zap.String("method", method))

				c.JSON(http.StatusForbidden, gin.H{
					"error": "Developers cannot access session-level endpoints",
					"scope": "session_only",
					"hint":  "Use agent authentication for knowledge operations",
				})
				c.Abort()
				return
			}
			// Require agent authentication and session context
			agentID := c.Query("agent_id")
			userID := c.Query("user_id")

			if agentID == "" || userID == "" {
				as.Logger.Warn("Missing agent context for session endpoint",
					zap.String("path", path),
					zap.String("method", method),
					zap.String("agent_id", agentID),
					zap.String("user_id", userID))

				c.JSON(http.StatusBadRequest, gin.H{
					"error": "agent_id and user_id are required for session-scoped operations",
					"scope": "session_only",
					"hint":  "Add ?agent_id=YOUR_AGENT&user_id=YOUR_USER to the URL",
				})
				c.Abort()
				return
			}

			// Extract session ID from session endpoints for proper validation
			var sessionID string
			if strings.Contains(path, "/sessions/v1/") {
				pathParts := strings.Split(path, "/")
				if len(pathParts) >= 4 && pathParts[1] == "sessions" && pathParts[2] == "v1" {
					sessionID = pathParts[3]
				}
			}

			// Validate agent is registered and has session access
			err := as.validateAgentAccess(c.Request.Context(), agentID, userID, sessionID, method)
			if err != nil {
				as.Logger.Warn("Agent access denied",
					zap.String("path", path),
					zap.String("method", method),
					zap.String("agent_id", agentID),
					zap.String("user_id", userID),
					zap.Error(err))

				c.JSON(http.StatusForbidden, gin.H{
					"error": fmt.Sprintf("Agent access denied: %v", err),
					"scope": "session_only",
					"hint":  "Ensure agent is registered via developer API first",
				})
				c.Abort()
				return
			}

			as.Logger.Info("Agent access granted",
				zap.String("path", path),
				zap.String("method", method),
				zap.String("agent_id", agentID),
				zap.String("user_id", userID))
		}

		c.Next()
	}
}

// isDeveloperOnlyEndpoint checks if the endpoint requires developer privileges
func isDeveloperOnlyEndpoint(path, method string) bool {
	developerEndpoints := []string{
		"/cluster/v1/users",         // User management
		"/cluster/v1/sessions",      // Session management
		"/cluster/v1/agents",        // Agent registration
		"/cluster/v1/agent-groups",  // Agent group registration
		"/cluster/v1/session-types", // Session type registration
		"/cluster/v1/monitoring",    // Monitoring endpoints
	}

	for _, endpoint := range developerEndpoints {
		if strings.HasPrefix(path, endpoint) {
			return true
		}
	}
	return false
}

// isAgentOnlyEndpoint checks if the endpoint requires agent session context
func isAgentOnlyEndpoint(path, method string) bool {
	agentEndpoints := []string{
		"/sessions/v1", // Session-level operations (memories, knowledge)
	}

	for _, endpoint := range agentEndpoints {
		if strings.HasPrefix(path, endpoint) {
			return true
		}
	}
	return false
}

// isValidDeveloperAuth validates developer authentication using config
func isValidDeveloperAuth(authHeader string) bool {
	// Get cluster API key from config
	expectedKey := config.Auth().ClusterAPIKey
	if expectedKey == "" {
		return false // No key configured
	}

	if authHeader == "" {
		return false
	}

	// Accept either Bearer or Api-Key format
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return token == expectedKey
	}

	if strings.HasPrefix(authHeader, "Api-Key ") {
		token := strings.TrimPrefix(authHeader, "Api-Key ")
		return token == expectedKey
	}

	return false
}

// hasPermissionForOperation checks if the agent's permission string allows the HTTP method
func hasPermissionForOperation(permission, method string) bool {
	// Map HTTP methods to CRUD operations
	var requiredPerm string
	switch method {
	case "POST":
		requiredPerm = "c" // create
	case "GET":
		requiredPerm = "r" // read
	case "PUT", "PATCH":
		requiredPerm = "u" // update
	case "DELETE":
		requiredPerm = "d" // delete
	default:
		return false // Unknown method not allowed
	}

	// Check if permission string contains required permission
	return strings.Contains(permission, requiredPerm)
}

// validateAgentAccess validates if an agent has access to the specified user's session
func (as *AppState) validateAgentAccess(ctx context.Context, agentID, userID, sessionID, method string) error {
	// Basic validation - agent and user IDs must be non-empty
	if agentID == "" || userID == "" {
		return fmt.Errorf("agentID and userID cannot be empty")
	}

	// Check if agent exists and is active
	agent, err := as.AgentService.GetAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("agent %s not found or inactive", agentID)
	}

	if agent.Status != agents.AgentStatusActive {
		return fmt.Errorf("agent %s is not active (status: %s)", agentID, string(agent.Status))
	}

	// CRITICAL: Validate CRUD permissions against HTTP method
	if !hasPermissionForOperation(agent.Permission, method) {
		return fmt.Errorf("agent %s does not have permission for %s operations (has: %s)", agentID, method, agent.Permission)
	}

	// If sessionID is provided, validate session-based access
	if sessionID != "" {
		// For now, implement basic session validation
		// TODO: Add GetSession method to SessionManager interface

		// Basic session-based access control using session type
		sessionType, err := as.SessionTypeService.GetSessionType(ctx, "restaurant_project") // Default for testing
		if err != nil {
			as.Logger.Warn("Session type not found, defaulting to basic validation",
				zap.String("session_id", sessionID))
		} else {
			// Since agents no longer have direct group references, we need to check
			// if the agent is in any of the allowed groups via the group service
			agentAuthorized := false

			// Check if agent is in any of the session type's allowed groups
			for _, allowedGroup := range sessionType.AgentGroups {
				isInGroup, err := as.AgentGroupService.IsAgentInGroup(ctx, agentID, allowedGroup)
				if err == nil && isInGroup {
					agentAuthorized = true
					break
				}
			}

			// If no specific groups are defined, allow default group access
			if len(sessionType.AgentGroups) == 0 {
				agentAuthorized = true
			}

			if !agentAuthorized {
				return fmt.Errorf("agent %s is not authorized for session type %s (allowed groups: %v)",
					agentID, sessionType.TypeID, sessionType.AgentGroups)
			}

			as.Logger.Info("Session access validated",
				zap.String("agent_id", agentID),
				zap.String("user_id", userID),
				zap.String("session_id", sessionID),
				zap.String("session_type", sessionType.TypeID))
		}
	}

	as.Logger.Info("Agent access validated",
		zap.String("agent_id", agentID),
		zap.String("user_id", userID),
		zap.String("agent_status", string(agent.Status)))

	return nil
}

func setupRouter(as *AppState) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add CORS middleware
	router.Use(cors.Default())

	// Add logging middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Add debug middleware to see ALL requests
	// Uncomment for debugging requests
	// router.Use(func(c *gin.Context) {
	// 	fmt.Printf("DEBUG: %s %s from %s\n",
	// 		c.Request.Method, c.Request.URL.String(), c.Request.RemoteAddr)
	// 	c.Next()
	// })

	// Health endpoint
	router.GET("/health", func(c *gin.Context) {
		ctx := c.Request.Context()

		// Perform comprehensive health check
		err := as.MemoryService.StartupHealthCheck(ctx)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "unhealthy",
				"timestamp": time.Now().Format(time.RFC3339),
				"error":     err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"services": gin.H{
				"database": "healthy",
				"memory":   "healthy",
			},
		})
	})

	// Add Access Control Middleware to API routes
	router.Use(AccessControlMiddleware(as))
	// Add Interaction Logging Middleware after access control
	router.Use(InteractionLoggingMiddleware(as))

	// CLUSTER API (Developer SDK Routes) - Requires cluster authentication
	cluster := router.Group("/cluster/v1")
	{
		// User Management
		users := cluster.Group("/users")
		{
			users.POST("/", createUser(as))          // eion.CreateUser(user_id, name=None)
			users.DELETE("/:userId", deleteUser(as)) // eion.DeleteUser(user_id)
		}

		// Agent Management
		agents := cluster.Group("/agents")
		{
			agents.POST("/", registerAgent(as))         // eion.RegisterAgent(agent_id, name, permission='r', description=None)
			agents.GET("/:agentId", getAgent(as))       // eion.GetAgent(agent_id)
			agents.PUT("/:agentId", updateAgent(as))    // eion.UpdateAgent(agent_id, variable, new_value)
			agents.DELETE("/:agentId", deleteAgent(as)) // eion.DeleteAgent(agent_id)
			agents.GET("/", listAgents(as))             // eion.ListAgents(permission=None)
		}

		// Agent Group Management
		agentGroups := cluster.Group("/agent-groups")
		{
			agentGroups.POST("/", registerAgentGroup(as))         // eion.RegisterAgentGroup(group_id, name, agent_ids=[], description=None)
			agentGroups.GET("/", listAgentGroups(as))             // eion.ListAgentGroups(agent_id=None)
			agentGroups.GET("/:groupId", getAgentGroup(as))       // eion.GetAgentGroup(group_id)
			agentGroups.PUT("/:groupId", updateAgentGroup(as))    // eion.UpdateAgentGroup(group_id, variable, new_value)
			agentGroups.DELETE("/:groupId", deleteAgentGroup(as)) // eion.DeleteAgentGroup(group_id)
		}

		// Session Type Management
		sessionTypes := cluster.Group("/session-types")
		{
			sessionTypes.POST("/", registerSessionType(as))               // eion.RegisterSessionType(session_type_id, name, agent_group_ids=[], description=None, encryption="SHA256")
			sessionTypes.GET("/", listSessionTypes(as))                   // eion.ListSessionTypes(agent_group_id=None)
			sessionTypes.GET("/:sessionTypeId", getSessionType(as))       // eion.GetSessionType(session_type_id)
			sessionTypes.PUT("/:sessionTypeId", updateSessionType(as))    // eion.UpdateSessionType(session_type_id, variable, new_value)
			sessionTypes.DELETE("/:sessionTypeId", deleteSessionType(as)) // eion.DeleteSessionType(session_type_id)
		}

		// Session Management
		sessions := cluster.Group("/sessions")
		{
			sessions.POST("/", createSession(as))             // eion.CreateSession(session_id, user_id, session_type_id='default', session_name=None)
			sessions.DELETE("/:sessionId", deleteSession(as)) // eion.DeleteSession(session_id)
		}

		// Monitoring & Analytics
		monitoring := cluster.Group("/monitoring")
		{
			monitoring.GET("/agents/:agentId", monitorAgent(as))       // eion.MonitorAgent(agent_id=None)
			monitoring.GET("/sessions/:sessionId", monitorSession(as)) // eion.MonitorSession(session_id=None)
		}
	}

	// SESSION API (Agent Routes) - Requires agent_id but no cluster auth
	sessions := router.Group("/sessions/v1")
	{
		// Memory operations
		memories := sessions.Group("/:sessionId/memories")
		{
			memories.GET("/", getMemory(as))          // Get memories for session
			memories.POST("/", createMemory(as))      // Add memory to session
			memories.GET("/search", searchMemory(as)) // Search memories
			memories.DELETE("/", deleteMemory(as))    // Delete memories
		}

		// Knowledge operations
		knowledge := sessions.Group("/:sessionId/knowledge")
		{
			knowledge.GET("/", searchKnowledge(as))    // Search knowledge in session
			knowledge.POST("/", createKnowledge(as))   // Add knowledge to session
			knowledge.PUT("/", updateKnowledge(as))    // Update knowledge in session
			knowledge.DELETE("/", deleteKnowledge(as)) // Delete knowledge from session
		}
	}

	return router
}

func setupSignalHandler(as *AppState, server *http.Server, logger *zap.Logger) chan struct{} {
	done := make(chan struct{}, 1)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalCh

		logger.Info("Shutting down server...")

		// Create context with timeout for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown server
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Error during server shutdown", zap.Error(err))
		}

		// Close memory service
		if err := as.MemoryService.Close(ctx); err != nil {
			logger.Error("Error closing memory service", zap.Error(err))
		}

		done <- struct{}{}
	}()

	return done
}

// Handler functions using new session service
func createSession(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req sessions.CreateSessionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Validate required fields
		if req.SessionID == "" || req.UserID == "" || req.SessionTypeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id, user_id, and session_type_id are required"})
			return
		}

		session, err := as.SessionService.CreateSession(c.Request.Context(), &req)
		if err != nil {
			as.Logger.Error("Failed to create session", zap.Error(err))
			if strings.Contains(err.Error(), "already exists") {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
			}
			return
		}

		c.JSON(http.StatusCreated, session)
	}
}

func deleteSession(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("sessionId")

		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
			return
		}

		req := &sessions.DeleteSessionRequest{
			SessionID: sessionID,
		}

		err := as.SessionService.DeleteSession(c.Request.Context(), req)
		if err != nil {
			as.Logger.Error("Failed to delete session", zap.String("session_id", sessionID), zap.Error(err))
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session"})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Session deleted successfully"})
	}
}

func getMemory(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("sessionId")
		agentID := c.Query("agent_id")
		userID := c.Query("user_id")

		if sessionID == "" || agentID == "" || userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId, agent_id, and user_id are required"})
			return
		}

		lastN := 10
		if lastNStr := c.Query("last_n"); lastNStr != "" {
			if parsed, err := strconv.Atoi(lastNStr); err == nil && parsed > 0 {
				lastN = parsed
			}
		}

		ctx := c.Request.Context()

		// Validate agent access
		if err := as.validateAgentAccess(ctx, agentID, userID, sessionID, c.Request.Method); err != nil {
			as.Logger.Warn("Agent access validation failed",
				zap.String("agent_id", agentID),
				zap.String("user_id", userID),
				zap.String("session_id", sessionID),
				zap.String("method", c.Request.Method),
				zap.Error(err))
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		// Fix: Use proper parameters (removed spaceID, fixed lastN parameter)
		opts := make([]memory.FilterOption, 0)
		memory, err := as.MemoryService.GetMemory(ctx, sessionID, agentID, lastN, opts...)
		if err != nil {
			as.Logger.Error("Failed to get memory", zap.String("session_id", sessionID), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "memory not found"})
			return
		}

		c.JSON(http.StatusOK, memory)
	}
}

func createMemory(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("sessionId")

		var req memory.AddMemoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Convert API request to internal memory format
		memoryData := &memory.Memory{
			Messages:  req.Messages,
			Metadata:  req.Metadata,
			SessionID: sessionID,
		}

		// Extract user_id and agent_id from query params OR from messages
		userID := c.Query("user_id")
		agentID := c.Query("agent_id")

		// If not in query params, try to extract from messages
		if len(req.Messages) > 0 {
			if agentID == "" && req.Messages[0].AgentID != "" {
				agentID = req.Messages[0].AgentID
			}
			// Try to extract user_id from message metadata
			if userID == "" {
				if userIDFromMsg, ok := req.Messages[0].Metadata["user_id"].(string); ok {
					userID = userIDFromMsg
				}
			}
		}

		// If still missing, require explicit parameters
		if agentID == "" || userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id and user_id are required (provide in query params or message data)"})
			return
		}

		memoryData.AgentID = agentID

		// Pass userID in metadata if available
		if memoryData.Metadata == nil {
			memoryData.Metadata = make(map[string]any)
		}
		if userID != "" {
			memoryData.Metadata["user_id"] = userID
		}

		ctx := c.Request.Context()
		// Fix: Remove spaceID parameter from PutMemory call
		err := as.MemoryService.PutMemory(ctx, sessionID, agentID, memoryData, false)
		if err != nil {
			as.Logger.Error("Failed to create memory", zap.String("session_id", sessionID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create memory"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Memory created"})
	}
}

func deleteMemory(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("sessionId")
		agentID := c.Query("agent_id")
		userID := c.Query("user_id")

		if agentID == "" || userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id and user_id are required"})
			return
		}

		ctx := c.Request.Context()
		// Fix: Remove spaceID parameter from GetMessages call
		messages, err := as.MemoryService.GetMessages(ctx, sessionID, agentID, -1, uuid.Nil)
		if err != nil {
			as.Logger.Error("Failed to get messages for deletion", zap.String("session_id", sessionID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete memory"})
			return
		}

		var messageUUIDs []uuid.UUID
		for _, msg := range messages {
			messageUUIDs = append(messageUUIDs, msg.UUID)
		}

		if len(messageUUIDs) > 0 {
			// Fix: Remove spaceID parameter from DeleteMessages call
			err = as.MemoryService.DeleteMessages(ctx, sessionID, agentID, messageUUIDs)
			if err != nil {
				as.Logger.Error("Failed to delete messages", zap.String("session_id", sessionID), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete memory"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Memory deleted"})
	}
}

func searchMemory(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		var query memory.MemorySearchQuery
		if err := c.ShouldBindJSON(&query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()
		results, err := as.MemoryService.SearchMemory(ctx, &query)
		if err != nil {
			as.Logger.Error("Failed to search memory", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}

		c.JSON(http.StatusOK, results)
	}
}

// User handlers

func createUser(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()
		as.Logger.Info("DEBUG: createUser called",
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("url", c.Request.URL.String()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.String("remote_addr", c.Request.RemoteAddr))

		var req users.CreateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		as.Logger.Info("DEBUG: About to create user", zap.String("request_id", requestID), zap.String("user_id", req.UserID))

		user, err := as.UserService.CreateUser(c.Request.Context(), &req)
		if err != nil {
			as.Logger.Error("Failed to create user", zap.Error(err))
			if strings.Contains(err.Error(), "user already exists") {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		as.Logger.Info("DEBUG: User created successfully", zap.String("request_id", requestID), zap.String("user_id", req.UserID))
		c.JSON(http.StatusCreated, user)
	}
}

func deleteUser(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("userId")
		if userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "userId parameter is required"})
			return
		}

		err := as.UserService.DeleteUser(c.Request.Context(), userID)
		if err != nil {
			as.Logger.Error("Failed to delete user", zap.String("user_id", userID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
	}
}

// DEVELOPER LEVEL HANDLERS

// Registration handlers
func registerAgent(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req agents.RegisterAgentRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if err := req.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		agent, err := as.AgentService.RegisterAgent(c.Request.Context(), &req)
		if err != nil {
			as.Logger.Error("Failed to register agent", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register agent"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"agent_id":    agent.ID,
			"name":        agent.Name,
			"permission":  agent.Permission,
			"description": agent.Description,
			"status":      string(agent.Status),
			"guest":       agent.Guest,
			"created_at":  agent.CreatedAt.Format(time.RFC3339),
		})
	}
}

func getAgent(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := c.Param("agentId")
		if agentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agentId parameter is required"})
			return
		}

		agent, err := as.AgentService.GetAgent(c.Request.Context(), agentID)
		if err != nil {
			as.Logger.Error("Failed to get agent", zap.String("agent_id", agentID), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"agent_id":    agent.ID,
			"name":        agent.Name,
			"permission":  agent.Permission,
			"description": agent.Description,
			"status":      string(agent.Status),
			"guest":       agent.Guest,
			"created_at":  agent.CreatedAt.Format(time.RFC3339),
			"updated_at":  agent.UpdatedAt.Format(time.RFC3339),
		})
	}
}

func updateAgent(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := c.Param("agentId")
		if agentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agentId parameter is required"})
			return
		}

		var reqBody struct {
			Variable string      `json:"variable"`
			Value    interface{} `json:"value"`
		}

		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if reqBody.Variable == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "variable is required"})
			return
		}

		agent, err := as.AgentService.UpdateAgent(c.Request.Context(), agentID, reqBody.Variable, reqBody.Value)
		if err != nil {
			as.Logger.Error("Failed to update agent", zap.String("agent_id", agentID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"agent_id":    agent.ID,
			"name":        agent.Name,
			"permission":  agent.Permission,
			"description": agent.Description,
			"status":      string(agent.Status),
			"guest":       agent.Guest,
			"updated_at":  agent.UpdatedAt.Format(time.RFC3339),
			"message":     "Agent updated successfully",
		})
	}
}

func deleteAgent(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := c.Param("agentId")
		if agentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agentId parameter is required"})
			return
		}

		req := &agents.DeleteAgentRequest{AgentID: agentID}
		if err := req.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err := as.AgentService.DeleteAgent(c.Request.Context(), req)
		if err != nil {
			as.Logger.Error("Failed to delete agent", zap.String("agent_id", agentID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete agent"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":    "Agent deleted successfully",
			"agent_id":   agentID,
			"deleted_at": time.Now().Format(time.RFC3339),
		})
	}
}

func listAgents(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse query parameters for filtering: permission level=None, guest=None
		req := &agents.ListAgentsRequest{}

		if permissionStr := c.Query("permission"); permissionStr != "" {
			req.Permission = &permissionStr
		}

		if guestStr := c.Query("guest"); guestStr != "" {
			if guest, err := strconv.ParseBool(guestStr); err == nil {
				req.Guest = &guest
			}
		}

		agentList, err := as.AgentService.ListAgents(c.Request.Context(), req)
		if err != nil {
			as.Logger.Error("Failed to list agents", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agents"})
			return
		}

		// Convert agents to response format
		agentResponses := make([]gin.H, len(agentList))
		for i, agent := range agentList {
			agentResponses[i] = gin.H{
				"agent_id":    agent.ID,
				"name":        agent.Name,
				"permission":  agent.Permission,
				"description": agent.Description,
				"status":      string(agent.Status),
				"guest":       agent.Guest,
				"created_at":  agent.CreatedAt.Format(time.RFC3339),
				"updated_at":  agent.UpdatedAt.Format(time.RFC3339),
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"agents": agentResponses,
			"count":  len(agentResponses),
			"filters": gin.H{
				"permission": req.Permission,
				"guest":      req.Guest,
			},
		})
	}
}

func registerAgentGroup(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req agentgroups.RegisterAgentGroupRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if err := req.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		group, err := as.AgentGroupService.RegisterAgentGroup(c.Request.Context(), &req)
		if err != nil {
			as.Logger.Error("Failed to register agent group", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register agent group"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"agent_group_id": group.ID,
			"name":           group.Name,
			"description":    group.Description,
			"created_at":     group.CreatedAt.Format(time.RFC3339),
		})
	}
}

func listAgentGroups(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &agentgroups.ListAgentGroupRequest{}

		// Note: Permission level filtering removed since agent groups no longer have permission levels

		groups, err := as.AgentGroupService.ListAgentGroups(c.Request.Context(), req)
		if err != nil {
			as.Logger.Error("Failed to list agent groups", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agent groups"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"agent_groups": groups})
	}
}

func getAgentGroup(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := c.Param("groupId")
		if groupID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "groupId parameter is required"})
			return
		}

		group, err := as.AgentGroupService.GetAgentGroup(c.Request.Context(), groupID)
		if err != nil {
			as.Logger.Error("Failed to get agent group", zap.String("group_id", groupID), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "Agent group not found"})
			return
		}

		c.JSON(http.StatusOK, group)
	}
}

func updateAgentGroup(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := c.Param("groupId")
		if groupID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "groupId parameter is required"})
			return
		}

		var reqBody struct {
			Variable string      `json:"variable"`
			Value    interface{} `json:"value"`
		}

		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if reqBody.Variable == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "variable is required"})
			return
		}

		group, err := as.AgentGroupService.UpdateAgentGroup(c.Request.Context(), groupID, reqBody.Variable, reqBody.Value)
		if err != nil {
			as.Logger.Error("Failed to update agent group", zap.String("group_id", groupID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent group"})
			return
		}

		c.JSON(http.StatusOK, group)
	}
}

func deleteAgentGroup(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := c.Param("groupId")
		if groupID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "groupId parameter is required"})
			return
		}

		req := &agentgroups.DeleteAgentGroupRequest{AgentGroupID: groupID}
		if err := req.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err := as.AgentGroupService.DeleteAgentGroup(c.Request.Context(), req)
		if err != nil {
			as.Logger.Error("Failed to delete agent group", zap.String("group_id", groupID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete agent group"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Agent group deleted successfully"})
	}
}

func registerSessionType(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req sessiontypes.RegisterSessionTypeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		sessionType, err := as.SessionTypeService.RegisterSessionType(c.Request.Context(), &req)
		if err != nil {
			as.Logger.Error("Failed to register session type", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register session type"})
			return
		}

		c.JSON(http.StatusCreated, sessionType)
	}
}

func listSessionTypes(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentGroupID := c.Query("agent_group_id")
		var agentGroupPtr *string
		if agentGroupID != "" {
			agentGroupPtr = &agentGroupID
		}

		sessionTypes, err := as.SessionTypeService.ListSessionTypes(c.Request.Context(), agentGroupPtr)
		if err != nil {
			as.Logger.Error("Failed to list session types", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list session types"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"session_types": sessionTypes})
	}
}

func getSessionType(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionTypeID := c.Param("sessionTypeId")

		sessionType, err := as.SessionTypeService.GetSessionType(c.Request.Context(), sessionTypeID)
		if err != nil {
			as.Logger.Error("Failed to get session type", zap.String("session_type_id", sessionTypeID), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "Session type not found"})
			return
		}

		c.JSON(http.StatusOK, sessionType)
	}
}

func updateSessionType(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionTypeID := c.Param("sessionTypeId")

		var reqBody struct {
			Variable string      `json:"variable"`
			Value    interface{} `json:"value"`
		}

		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if reqBody.Variable == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "variable is required"})
			return
		}

		sessionType, err := as.SessionTypeService.UpdateSessionType(c.Request.Context(), sessionTypeID, reqBody.Variable, reqBody.Value)
		if err != nil {
			as.Logger.Error("Failed to update session type", zap.String("session_type_id", sessionTypeID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session type"})
			return
		}

		c.JSON(http.StatusOK, sessionType)
	}
}

func deleteSessionType(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionTypeID := c.Param("sessionTypeId")

		err := as.SessionTypeService.DeleteSessionType(c.Request.Context(), sessionTypeID)
		if err != nil {
			as.Logger.Error("Failed to delete session type", zap.String("session_type_id", sessionTypeID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session type"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Session type deleted successfully"})
	}
}

// Monitoring handlers
func monitorAgent(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := c.Param("agentId")
		if agentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
			return
		}

		// Parse time range from query parameters (optional)
		timeRange := logging.TimeRange{
			StartTime: time.Now().Add(-24 * time.Hour), // Default: last 24 hours
			EndTime:   time.Now(),
		}

		if startStr := c.Query("start_time"); startStr != "" {
			if start, err := time.Parse(time.RFC3339, startStr); err == nil {
				timeRange.StartTime = start
			}
		}

		if endStr := c.Query("end_time"); endStr != "" {
			if end, err := time.Parse(time.RFC3339, endStr); err == nil {
				timeRange.EndTime = end
			}
		}

		// Get real analytics from orchestrator
		analytics, err := as.Orchestrator.MonitorAgent(c.Request.Context(), agentID, timeRange)
		if err != nil {
			as.Logger.Error("Failed to get agent analytics",
				zap.String("agent_id", agentID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve agent analytics"})
			return
		}

		c.JSON(http.StatusOK, analytics)
	}
}

func monitorSession(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("sessionId")
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
			return
		}

		// Get real session analytics from orchestrator
		analytics, err := as.Orchestrator.MonitorSession(c.Request.Context(), sessionID)
		if err != nil {
			as.Logger.Error("Failed to get session analytics",
				zap.String("session_id", sessionID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve session analytics"})
			return
		}

		c.JSON(http.StatusOK, analytics)
	}
}

// PERMISSION MANAGEMENT HANDLERS

// KNOWLEDGE MANAGEMENT HANDLERS - Following SDK.md specification exactly
// GET /api/knowledge/session_123?agent_id=agent_456&space_id=space_789&query=user+authentication&limit=10

// Helper function to extract UUIDs from messages
func extractMessageUUIDs(messages []memory.Message) []uuid.UUID {
	uuids := make([]uuid.UUID, len(messages))
	for i, msg := range messages {
		uuids[i] = msg.UUID
	}
	return uuids
}

func searchKnowledge(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract sessionID from URL path parameter
		sessionID := c.Param("sessionId")

		var queryParams struct {
			Query   string `form:"query" binding:"required"`
			AgentID string `form:"agent_id" binding:"required"`
			UserID  string `form:"user_id" binding:"required"`
			Limit   int    `form:"limit"`
		}

		if err := c.ShouldBindQuery(&queryParams); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if queryParams.Limit <= 0 {
			queryParams.Limit = 10
		}

		// FIXED: Create search query with sessionID in metadata for multi-agent knowledge sharing
		// This ensures agents can search knowledge from the same session
		searchQuery := &memory.MemorySearchQuery{
			Text:     queryParams.Query,
			AgentID:  queryParams.AgentID,
			Limit:    queryParams.Limit,
			MinScore: 0.7,
			Metadata: map[string]any{
				"session_id": sessionID, // Pass sessionID for session-scoped search
			},
		}

		ctx := c.Request.Context()
		results, err := as.MemoryService.SearchMemory(ctx, searchQuery)
		if err != nil {
			as.Logger.Error("Failed to search knowledge",
				zap.Error(err),
				zap.String("session_id", sessionID),
				zap.String("agent_id", queryParams.AgentID))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}

		as.Logger.Debug("Knowledge search completed",
			zap.String("session_id", sessionID),
			zap.String("agent_id", queryParams.AgentID),
			zap.String("query", queryParams.Query),
			zap.Int("results_count", results.TotalCount))

		c.JSON(http.StatusOK, results)
	}
}

func createKnowledge(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("sessionId")

		var req memory.AddMemoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		agentID := c.Query("agent_id")
		userID := c.Query("user_id")

		if agentID == "" || userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id and user_id are required"})
			return
		}

		// Validate and fix message data - remove SpaceID references
		for i := range req.Messages {
			if req.Messages[i].AgentID == "" {
				req.Messages[i].AgentID = agentID
			}
			if req.Messages[i].SessionID == "" {
				req.Messages[i].SessionID = sessionID
			}
		}

		ctx := c.Request.Context()
		// Create memory object with proper metadata including user_id
		memory := &memory.Memory{
			Messages:  req.Messages,
			AgentID:   agentID,
			SessionID: sessionID,
			Metadata: map[string]any{
				"user_id": userID, // Required by memory store
			},
		}

		// Use PutMemory with knowledge processing enabled (messages must be processed into Neo4j for search)
		err := as.MemoryService.PutMemory(ctx, sessionID, agentID, memory, false)
		if err != nil {
			as.Logger.Error("Failed to create knowledge", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":        "Knowledge created successfully",
			"session_id":     sessionID,
			"messages_count": len(req.Messages),
		})
	}
}

func updateKnowledge(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("sessionId")

		var req struct {
			Messages []memory.Message `json:"messages"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		agentID := c.Query("agent_id")
		userID := c.Query("user_id")

		if agentID == "" || userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id and user_id are required"})
			return
		}

		// Get current memory and check for updates - fix method call parameters
		ctx := c.Request.Context()
		opts := make([]memory.FilterOption, 0)
		currentMemory, err := as.MemoryService.GetMemory(ctx, sessionID, agentID, -1, opts...)
		if err != nil {
			as.Logger.Error("Failed to get current knowledge", zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "knowledge not found"})
			return
		}

		// Update messages logic here...
		messageUUIDs := extractMessageUUIDs(currentMemory.Messages)

		if len(messageUUIDs) > 0 {
			// Fix: Remove spaceID parameter from DeleteMessages call
			err = as.MemoryService.DeleteMessages(ctx, sessionID, agentID, messageUUIDs)
			if err != nil {
				as.Logger.Error("Failed to delete old knowledge", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update knowledge"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Knowledge updated"})
	}
}

func deleteKnowledge(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("sessionId")
		agentID := c.Query("agent_id")
		userID := c.Query("user_id")

		if sessionID == "" || agentID == "" || userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId, agent_id, and user_id are required"})
			return
		}

		ctx := c.Request.Context()

		// Get existing messages to delete them - fix method call
		opts := make([]memory.FilterOption, 0)
		existingMemory, err := as.MemoryService.GetMemory(ctx, sessionID, agentID, -1, opts...)
		if err != nil {
			as.Logger.Warn("No existing knowledge found to delete", zap.Error(err))
			// Return success anyway - idempotent delete
			c.JSON(http.StatusOK, gin.H{
				"message": "Knowledge deleted successfully (nothing to delete)",
			})
			return
		}

		if len(existingMemory.Messages) > 0 {
			// Delete existing messages - fix method call
			existingUUIDs := extractMessageUUIDs(existingMemory.Messages)
			err = as.MemoryService.DeleteMessages(ctx, sessionID, agentID, existingUUIDs)
			if err != nil {
				as.Logger.Error("Failed to delete knowledge", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete knowledge"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Knowledge deleted successfully",
			"deleted_count": len(existingMemory.Messages),
		})
	}
}

// InteractionLoggingMiddleware logs all agent interactions automatically
func InteractionLoggingMiddleware(as *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Skip logging for health endpoints and non-agent operations
		if strings.HasPrefix(path, "/health") || !isAgentOnlyEndpoint(path, method) {
			c.Next()
			return
		}

		// Extract agent context
		agentID := c.Query("agent_id")
		userID := c.Query("user_id")
		sessionID := ""

		// Extract session ID from knowledge endpoints
		if strings.Contains(path, "/knowledge/") {
			pathParts := strings.Split(path, "/")
			if len(pathParts) >= 5 && pathParts[3] == "knowledge" {
				sessionID = pathParts[4]
			}
		}

		// Execute the request
		c.Next()

		// Skip logging if context is incomplete
		if agentID == "" || userID == "" {
			return
		}

		// Determine operation type from path
		operation := "unknown"
		if strings.Contains(path, "/knowledge") {
			switch method {
			case "GET":
				operation = "search_knowledge"
			case "POST":
				operation = "create_knowledge"
			case "PUT":
				operation = "update_knowledge"
			case "DELETE":
				operation = "delete_knowledge"
			}
		}

		// Create interaction log
		interactionLog := &logging.AgentInteractionLog{
			LogID:     uuid.New().String(),
			AgentID:   agentID,
			UserID:    userID,
			Operation: operation,
			Endpoint:  path,
			Method:    method,
			SessionID: sessionID,
			Success:   c.Writer.Status() < 400,
			Timestamp: startTime,
			RequestData: map[string]interface{}{
				"status_code":      c.Writer.Status(),
				"response_time_ms": time.Since(startTime).Milliseconds(),
				"query_params":     c.Request.URL.RawQuery,
			},
		}

		// Add error message if request failed
		if c.Writer.Status() >= 400 {
			// Try to extract error from response (simplified)
			interactionLog.ErrorMsg = fmt.Sprintf("HTTP %d", c.Writer.Status())
		}

		// Log the interaction asynchronously to avoid blocking response
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := as.Orchestrator.LogAgentInteraction(ctx, interactionLog); err != nil {
				as.Logger.Error("Failed to log agent interaction",
					zap.String("agent_id", agentID),
					zap.String("operation", operation),
					zap.Error(err))
			}
		}()
	}
}
