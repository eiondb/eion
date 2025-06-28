package console

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/eion/eion/internal/config"
	"github.com/eion/eion/internal/orchestrator/agents"
	"github.com/eion/eion/internal/orchestrator/sessions"
	"github.com/eion/eion/internal/orchestrator/users"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

// ConsoleService handles the Register Console functionality
type ConsoleService struct {
	AgentService   agents.AgentManager
	SessionService sessions.SessionManager
	UserService    users.UserService
	Logger         *zap.Logger
	Config         *config.Config
}

// NewConsoleService creates a new console service
func NewConsoleService(
	agentService agents.AgentManager,
	sessionService sessions.SessionManager,
	userService users.UserService,
	logger *zap.Logger,
	cfg *config.Config,
) *ConsoleService {
	return &ConsoleService{
		AgentService:   agentService,
		SessionService: sessionService,
		UserService:    userService,
		Logger:         logger,
		Config:         cfg,
	}
}

// SetupRoutes sets up the console routes
func (cs *ConsoleService) SetupRoutes(router *gin.Engine) {
	// Serve static files - need to create a sub-filesystem to handle the embedded 'static/' prefix
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		cs.Logger.Error("Failed to create static sub-filesystem", zap.Error(err))
		return
	}
	router.StaticFS("/console/static", http.FS(staticFS))

	// Console routes
	consoleGroup := router.Group("/console")
	{
		consoleGroup.GET("/", cs.serveConsole)
		consoleGroup.GET("/api/config", cs.getConfig)
		consoleGroup.GET("/api/agents", cs.getAgents)
		consoleGroup.POST("/api/agents", cs.registerAgent)
		consoleGroup.GET("/api/sessions", cs.getSessions)
		consoleGroup.GET("/api/users", cs.getUsers)
		consoleGroup.GET("/api/health", cs.getHealth)
	}
}

// serveConsole serves the main console page
func (cs *ConsoleService) serveConsole(c *gin.Context) {
	tmpl, err := template.ParseFS(templateFiles, "templates/console.html")
	if err != nil {
		cs.Logger.Error("Failed to parse console template", zap.Error(err))
		c.String(http.StatusInternalServerError, "Template Error: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Title": "Eion Register Console",
	}

	c.Header("Content-Type", "text/html")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		cs.Logger.Error("Failed to execute console template", zap.Error(err))
		c.String(http.StatusInternalServerError, "Execution Error: "+err.Error())
		return
	}
}

// getConfig returns the current configuration (non-sensitive parts)
func (cs *ConsoleService) getConfig(c *gin.Context) {
	config := map[string]interface{}{
		"mcp_enabled":     cs.Config.Common.MCP.Enabled,
		"mcp_port":        cs.Config.Common.MCP.Port,
		"cluster_api_key": cs.Config.Common.Auth.ClusterAPIKey,
		"numa_enabled":    cs.Config.Common.Numa.Enabled,
		"neo4j_uri":       cs.Config.Common.Numa.Neo4j.URI,
		"host":            cs.Config.Common.Http.Host,
		"port":            cs.Config.Common.Http.Port,
	}

	c.JSON(http.StatusOK, gin.H{"config": config})
}

// getAgents returns the list of registered agents
func (cs *ConsoleService) getAgents(c *gin.Context) {
	req := &agents.ListAgentsRequest{}
	agents, err := cs.AgentService.ListAgents(c.Request.Context(), req)
	if err != nil {
		cs.Logger.Error("Failed to list agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// registerAgent handles agent registration
func (cs *ConsoleService) registerAgent(c *gin.Context) {
	var request struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Permissions string `json:"permissions"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate agent ID
	agentID := "agent-" + fmt.Sprintf("%d", time.Now().Unix())

	// Create agent registration request
	agentReq := &agents.RegisterAgentRequest{
		AgentID:    agentID,
		Name:       request.Name,
		Permission: &request.Permissions,
	}

	if request.Description != "" {
		agentReq.Description = &request.Description
	}

	// Register agent
	agent, err := cs.AgentService.RegisterAgent(c.Request.Context(), agentReq)
	if err != nil {
		cs.Logger.Error("Failed to register agent", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register agent"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

// getSessions returns the list of sessions
func (cs *ConsoleService) getSessions(c *gin.Context) {
	// Sessions don't have a list method in the current implementation
	// Return empty list for now
	c.JSON(http.StatusOK, gin.H{"sessions": []interface{}{}})
}

// getUsers returns the list of users
func (cs *ConsoleService) getUsers(c *gin.Context) {
	// Users don't have a list method in the current implementation
	// Return empty list for now
	c.JSON(http.StatusOK, gin.H{"users": []interface{}{}})
}

// getHealth returns system health information
func (cs *ConsoleService) getHealth(c *gin.Context) {
	health := map[string]interface{}{
		"status":       "healthy",
		"console":      true,
		"mcp_enabled":  cs.Config.Common.MCP.Enabled,
		"numa_enabled": cs.Config.Common.Numa.Enabled,
	}

	c.JSON(http.StatusOK, gin.H{"health": health})
}
