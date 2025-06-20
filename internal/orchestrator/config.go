package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/eion/eion/internal/orchestrator/agentgroups"
	"github.com/eion/eion/internal/orchestrator/sessiontypes"
)

// OrchestratorConfig holds configuration for the multi-agent orchestrator
// Following HFD protocol: no defaults, all values must be explicitly set
type OrchestratorConfig struct {
	// Enabled controls whether the orchestrator is active
	// Must be explicitly set - no default value
	Enabled bool `yaml:"enabled" json:"enabled"`

	// AgentRegistry configuration
	AgentRegistry AgentRegistryConfig `yaml:"agent_registry" json:"agent_registry"`

	// SpaceManager configuration
	SpaceManager SpaceManagerConfig `yaml:"space_manager" json:"space_manager"`

	// AuditLogger configuration
	AuditLogger AuditLoggerConfig `yaml:"audit_logger" json:"audit_logger"`

	// PresenceTracker configuration
	PresenceTracker PresenceTrackerConfig `yaml:"presence_tracker" json:"presence_tracker"`

	// Database configuration
	Database DatabaseConfig `yaml:"database" json:"database"`
}

// Validate validates the orchestrator configuration
func (c *OrchestratorConfig) Validate() error {
	if !c.Enabled {
		return fmt.Errorf("orchestrator is disabled - set enabled: true to activate multi-agent features")
	}

	if err := c.AgentRegistry.Validate(); err != nil {
		return fmt.Errorf("agent registry config validation failed: %w", err)
	}

	if err := c.SpaceManager.Validate(); err != nil {
		return fmt.Errorf("space manager config validation failed: %w", err)
	}

	if err := c.AuditLogger.Validate(); err != nil {
		return fmt.Errorf("audit logger config validation failed: %w", err)
	}

	if err := c.PresenceTracker.Validate(); err != nil {
		return fmt.Errorf("presence tracker config validation failed: %w", err)
	}

	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("database config validation failed: %w", err)
	}

	return nil
}

// AgentRegistryConfig configures agent registration behavior
type AgentRegistryConfig struct {
	// RequireExplicitRegistration controls whether agents must be explicitly registered
	// Must be set explicitly - no default
	RequireExplicitRegistration bool `yaml:"require_explicit_registration" json:"require_explicit_registration"`

	// AllowAutoSpaceAssignment controls whether agents can be auto-assigned to spaces
	// Following HFD: should be false to require explicit space assignment
	AllowAutoSpaceAssignment bool `yaml:"allow_auto_space_assignment" json:"allow_auto_space_assignment"`

	// MaxAgentsPerSpace limits the number of agents per space
	// Must be set explicitly - no default
	MaxAgentsPerSpace int `yaml:"max_agents_per_space" json:"max_agents_per_space"`

	// AgentTimeoutDuration after which agents are considered inactive
	// Must be set explicitly - no default
	AgentTimeoutDuration string `yaml:"agent_timeout_duration" json:"agent_timeout_duration"`
}

// Validate validates agent registry configuration
func (c *AgentRegistryConfig) Validate() error {
	if !c.RequireExplicitRegistration {
		return fmt.Errorf("require_explicit_registration must be true - auto-registration not allowed under HFD protocol")
	}

	if c.AllowAutoSpaceAssignment {
		return fmt.Errorf("allow_auto_space_assignment must be false - explicit space assignment required under HFD protocol")
	}

	if c.MaxAgentsPerSpace <= 0 {
		return fmt.Errorf("max_agents_per_space must be a positive integer")
	}

	if c.AgentTimeoutDuration == "" {
		return fmt.Errorf("agent_timeout_duration is required and cannot be empty")
	}

	// Validate duration format
	if _, err := time.ParseDuration(c.AgentTimeoutDuration); err != nil {
		return fmt.Errorf("invalid agent_timeout_duration format: %w", err)
	}

	return nil
}

// SpaceManagerConfig configures knowledge space management
type SpaceManagerConfig struct {
	// AllowAutoCreation controls whether spaces can be auto-created
	// Following HFD: should be false to require explicit creation
	AllowAutoCreation bool `yaml:"allow_auto_creation" json:"allow_auto_creation"`

	// RequireExplicitAccessRules controls whether access rules must be explicitly defined
	// Must be set explicitly - no default
	RequireExplicitAccessRules bool `yaml:"require_explicit_access_rules" json:"require_explicit_access_rules"`

	// MaxSpaces limits the total number of spaces
	// Must be set explicitly - no default
	MaxSpaces int `yaml:"max_spaces" json:"max_spaces"`

	// DeleteProtection controls whether spaces with active agents can be deleted
	DeleteProtection bool `yaml:"delete_protection" json:"delete_protection"`
}

// Validate validates space manager configuration
func (c *SpaceManagerConfig) Validate() error {
	if c.AllowAutoCreation {
		return fmt.Errorf("allow_auto_creation must be false - explicit space creation required under HFD protocol")
	}

	if !c.RequireExplicitAccessRules {
		return fmt.Errorf("require_explicit_access_rules must be true - explicit rules required under HFD protocol")
	}

	if c.MaxSpaces <= 0 {
		return fmt.Errorf("max_spaces must be a positive integer")
	}

	return nil
}

// AuditLoggerConfig configures audit logging
type AuditLoggerConfig struct {
	// EnableAuditLogging controls whether audit logging is active
	// Must be set explicitly - no default
	EnableAuditLogging bool `yaml:"enable_audit_logging" json:"enable_audit_logging"`

	// LogAllOperations controls whether all operations are logged
	LogAllOperations bool `yaml:"log_all_operations" json:"log_all_operations"`

	// RetentionDuration for audit logs
	// Must be set explicitly - no default
	RetentionDuration string `yaml:"retention_duration" json:"retention_duration"`

	// BatchSize for bulk audit log operations
	BatchSize int `yaml:"batch_size" json:"batch_size"`
}

// Validate validates audit logger configuration
func (c *AuditLoggerConfig) Validate() error {
	if !c.EnableAuditLogging {
		return fmt.Errorf("enable_audit_logging must be true - audit logging required for multi-agent operations")
	}

	if !c.LogAllOperations {
		return fmt.Errorf("log_all_operations must be true - comprehensive audit trail required")
	}

	if c.RetentionDuration == "" {
		return fmt.Errorf("retention_duration is required and cannot be empty")
	}

	// Validate duration format
	if _, err := time.ParseDuration(c.RetentionDuration); err != nil {
		return fmt.Errorf("invalid retention_duration format: %w", err)
	}

	if c.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be a positive integer")
	}

	return nil
}

// PresenceTrackerConfig configures agent presence tracking
type PresenceTrackerConfig struct {
	// EnablePresenceTracking controls whether presence tracking is active
	// Must be set explicitly - no default
	EnablePresenceTracking bool `yaml:"enable_presence_tracking" json:"enable_presence_tracking"`

	// HeartbeatInterval for agent heartbeats
	// Must be set explicitly - no default
	HeartbeatInterval string `yaml:"heartbeat_interval" json:"heartbeat_interval"`

	// InactiveThreshold after which agents are considered inactive
	// Must be set explicitly - no default
	InactiveThreshold string `yaml:"inactive_threshold" json:"inactive_threshold"`

	// CleanupInterval for cleaning up inactive presence records
	CleanupInterval string `yaml:"cleanup_interval" json:"cleanup_interval"`
}

// Validate validates presence tracker configuration
func (c *PresenceTrackerConfig) Validate() error {
	if !c.EnablePresenceTracking {
		return fmt.Errorf("enable_presence_tracking must be true - presence tracking required for multi-agent coordination")
	}

	if c.HeartbeatInterval == "" {
		return fmt.Errorf("heartbeat_interval is required and cannot be empty")
	}

	if c.InactiveThreshold == "" {
		return fmt.Errorf("inactive_threshold is required and cannot be empty")
	}

	if c.CleanupInterval == "" {
		return fmt.Errorf("cleanup_interval is required and cannot be empty")
	}

	// Validate duration formats
	intervals := map[string]string{
		"heartbeat_interval": c.HeartbeatInterval,
		"inactive_threshold": c.InactiveThreshold,
		"cleanup_interval":   c.CleanupInterval,
	}

	for name, interval := range intervals {
		if _, err := time.ParseDuration(interval); err != nil {
			return fmt.Errorf("invalid %s format: %w", name, err)
		}
	}

	return nil
}

// DatabaseConfig configures database connections for orchestrator
type DatabaseConfig struct {
	// UseSharedConnection controls whether to use the existing memory service connection
	UseSharedConnection bool `yaml:"use_shared_connection" json:"use_shared_connection"`

	// ConnectionString for dedicated orchestrator database (if not using shared)
	ConnectionString string `yaml:"connection_string" json:"connection_string"`

	// MaxConnections for the database pool
	MaxConnections int `yaml:"max_connections" json:"max_connections"`

	// ConnectionTimeout for database operations
	ConnectionTimeout string `yaml:"connection_timeout" json:"connection_timeout"`

	// EnableMigrations controls whether database migrations run automatically
	EnableMigrations bool `yaml:"enable_migrations" json:"enable_migrations"`
}

// Validate validates database configuration
func (c *DatabaseConfig) Validate() error {
	if !c.UseSharedConnection && c.ConnectionString == "" {
		return fmt.Errorf("connection_string is required when use_shared_connection is false")
	}

	if c.MaxConnections <= 0 {
		return fmt.Errorf("max_connections must be a positive integer")
	}

	if c.ConnectionTimeout == "" {
		return fmt.Errorf("connection_timeout is required and cannot be empty")
	}

	// Validate timeout format
	if _, err := time.ParseDuration(c.ConnectionTimeout); err != nil {
		return fmt.Errorf("invalid connection_timeout format: %w", err)
	}

	return nil
}

// NewDefaultOrchestratorConfig creates a new orchestrator config with HFD-compliant defaults
// Note: This function provides a starting template, but all values must be explicitly reviewed and set
func NewDefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		Enabled: false, // Must be explicitly enabled

		AgentRegistry: AgentRegistryConfig{
			RequireExplicitRegistration: true,  // HFD compliant
			AllowAutoSpaceAssignment:    false, // HFD compliant
			MaxAgentsPerSpace:           100,   // Must be explicitly set
			AgentTimeoutDuration:        "1h",  // Must be explicitly set
		},

		SpaceManager: SpaceManagerConfig{
			AllowAutoCreation:          false, // HFD compliant
			RequireExplicitAccessRules: true,  // HFD compliant
			MaxSpaces:                  50,    // Must be explicitly set
			DeleteProtection:           true,  // Safe default
		},

		AuditLogger: AuditLoggerConfig{
			EnableAuditLogging: true,  // Required for multi-agent
			LogAllOperations:   true,  // Comprehensive audit trail
			RetentionDuration:  "30d", // Must be explicitly set
			BatchSize:          100,   // Must be explicitly set
		},

		PresenceTracker: PresenceTrackerConfig{
			EnablePresenceTracking: true,  // Required for multi-agent
			HeartbeatInterval:      "30s", // Must be explicitly set
			InactiveThreshold:      "5m",  // Must be explicitly set
			CleanupInterval:        "1h",  // Must be explicitly set
		},

		Database: DatabaseConfig{
			UseSharedConnection: true,  // Use existing memory service connection
			ConnectionString:    "",    // Not needed when using shared
			MaxConnections:      10,    // Must be explicitly set
			ConnectionTimeout:   "30s", // Must be explicitly set
			EnableMigrations:    true,  // Safe default for development
		},
	}
}

// SetupDefaults creates default permissions, agent groups, and session types
// This function is designed to run only on first-time cluster initialization.
// It checks if default data already exists and only creates it if missing.
func SetupDefaults(ctx context.Context,
	agentGroupService agentgroups.AgentGroupManager,
	sessionTypeService sessiontypes.SessionTypeManager) error {

	// Check if defaults are already initialized by looking for any of the default entities
	_, agentGroupErr := agentGroupService.GetAgentGroup(ctx, "default")
	_, sessionTypeErr := sessionTypeService.GetSessionType(ctx, "default")

	// If all default entities exist, cluster is already initialized
	if agentGroupErr == nil && sessionTypeErr == nil {
		// Defaults already exist, skip initialization
		return nil
	}

	// Create default agent group (only if doesn't exist)
	if agentGroupErr != nil {
		defaultAgentGroupReq := &agentgroups.RegisterAgentGroupRequest{
			AgentGroupID: "default",
			Name:         "Default Agent Group",
			Description:  &[]string{"Default agent group with read permissions"}[0],
		}
		_, err := agentGroupService.RegisterAgentGroup(ctx, defaultAgentGroupReq)
		if err != nil {
			return fmt.Errorf("failed to create default agent group: %w", err)
		}
	}

	// Create default session type with permission 0 (only if doesn't exist)
	if sessionTypeErr != nil {
		defaultSessionTypeReq := &sessiontypes.RegisterSessionTypeRequest{
			SessionTypeID: "default",
			Name:          "Default Session Type",
			Description:   &[]string{"Default session type for standard operations"}[0],
			AgentGroups:   []string{"default"},
			Encryption:    "SHA256",
		}
		_, err := sessionTypeService.RegisterSessionType(ctx, defaultSessionTypeReq)
		if err != nil {
			return fmt.Errorf("failed to create default session type: %w", err)
		}
	}

	return nil
}
