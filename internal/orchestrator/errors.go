package orchestrator

import (
	"fmt"
)

// Error types for orchestrator operations following HFD protocol

// AgentError represents errors related to agent operations
type AgentError struct {
	Type    string
	AgentID string
	Message string
	Cause   error
}

func (e *AgentError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("agent error [%s] for agent %s: %s (caused by: %v)", e.Type, e.AgentID, e.Message, e.Cause)
	}
	return fmt.Sprintf("agent error [%s] for agent %s: %s", e.Type, e.AgentID, e.Message)
}

func (e *AgentError) Unwrap() error {
	return e.Cause
}

// Agent error types
const (
	AgentErrorTypeAlreadyExists    = "already_exists"
	AgentErrorTypeNotFound         = "not_found"
	AgentErrorTypeInvalidRequest   = "invalid_request"
	AgentErrorTypePermissionDenied = "permission_denied"
	AgentErrorTypeInvalidStatus    = "invalid_status"
	AgentErrorTypeValidationFailed = "validation_failed"
)

// NewAgentAlreadyExistsError creates an error for when an agent already exists
func NewAgentAlreadyExistsError(agentID string) *AgentError {
	return &AgentError{
		Type:    AgentErrorTypeAlreadyExists,
		AgentID: agentID,
		Message: "agent already exists and cannot be registered again",
	}
}

// NewAgentNotFoundError creates an error for when an agent is not found
func NewAgentNotFoundError(agentID string) *AgentError {
	return &AgentError{
		Type:    AgentErrorTypeNotFound,
		AgentID: agentID,
		Message: "agent not found",
	}
}

// NewAgentValidationError creates an error for agent validation failures
func NewAgentValidationError(agentID string, cause error) *AgentError {
	return &AgentError{
		Type:    AgentErrorTypeValidationFailed,
		AgentID: agentID,
		Message: "agent validation failed",
		Cause:   cause,
	}
}

// NewAgentPermissionError creates an error for permission-related failures
func NewAgentPermissionError(agentID string, message string) *AgentError {
	return &AgentError{
		Type:    AgentErrorTypePermissionDenied,
		AgentID: agentID,
		Message: message,
	}
}

// SpaceError represents errors related to space operations
type SpaceError struct {
	Type    string
	SpaceID string
	Message string
	Cause   error
}

func (e *SpaceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("space error [%s] for space %s: %s (caused by: %v)", e.Type, e.SpaceID, e.Message, e.Cause)
	}
	return fmt.Sprintf("space error [%s] for space %s: %s", e.Type, e.SpaceID, e.Message)
}

func (e *SpaceError) Unwrap() error {
	return e.Cause
}

// Space error types
const (
	SpaceErrorTypeAlreadyExists    = "already_exists"
	SpaceErrorTypeNotFound         = "not_found"
	SpaceErrorTypeInvalidRequest   = "invalid_request"
	SpaceErrorTypeValidationFailed = "validation_failed"
	SpaceErrorTypeHasActiveAgents  = "has_active_agents"
)

// NewSpaceAlreadyExistsError creates an error for when a space already exists
func NewSpaceAlreadyExistsError(spaceID string) *SpaceError {
	return &SpaceError{
		Type:    SpaceErrorTypeAlreadyExists,
		SpaceID: spaceID,
		Message: "knowledge space already exists and cannot be created again",
	}
}

// NewSpaceNotFoundError creates an error for when a space is not found
func NewSpaceNotFoundError(spaceID string) *SpaceError {
	return &SpaceError{
		Type:    SpaceErrorTypeNotFound,
		SpaceID: spaceID,
		Message: "knowledge space not found",
	}
}

// NewSpaceValidationError creates an error for space validation failures
func NewSpaceValidationError(spaceID string, cause error) *SpaceError {
	return &SpaceError{
		Type:    SpaceErrorTypeValidationFailed,
		SpaceID: spaceID,
		Message: "space validation failed",
		Cause:   cause,
	}
}

// NewSpaceHasActiveAgentsError creates an error for when trying to delete a space with active agents
func NewSpaceHasActiveAgentsError(spaceID string, agentCount int) *SpaceError {
	return &SpaceError{
		Type:    SpaceErrorTypeHasActiveAgents,
		SpaceID: spaceID,
		Message: fmt.Sprintf("cannot delete space with %d active agents - deactivate agents first", agentCount),
	}
}

// ValidationError represents errors in request validation
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation error for field '%s' (value: %v): %s (caused by: %v)", e.Field, e.Value, e.Message, e.Cause)
	}
	return fmt.Sprintf("validation error for field '%s' (value: %v): %s", e.Field, e.Value, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NewValidationErrorWithCause creates a new validation error with a cause
func NewValidationErrorWithCause(field string, value interface{}, message string, cause error) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
		Cause:   cause,
	}
}

// StorageError represents errors related to storage operations
type StorageError struct {
	Type      string
	Operation string
	Resource  string
	Message   string
	Cause     error
}

func (e *StorageError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("storage error [%s] during %s on %s: %s (caused by: %v)",
			e.Type, e.Operation, e.Resource, e.Message, e.Cause)
	}
	return fmt.Sprintf("storage error [%s] during %s on %s: %s",
		e.Type, e.Operation, e.Resource, e.Message)
}

func (e *StorageError) Unwrap() error {
	return e.Cause
}

// Storage error types
const (
	StorageErrorTypeConnectionFailed    = "connection_failed"
	StorageErrorTypeQueryFailed         = "query_failed"
	StorageErrorTypeTransactionFailed   = "transaction_failed"
	StorageErrorTypeConstraintViolation = "constraint_violation"
	StorageErrorTypeDataCorruption      = "data_corruption"
)

// NewStorageConnectionError creates an error for storage connection failures
func NewStorageConnectionError(operation, resource string, cause error) *StorageError {
	return &StorageError{
		Type:      StorageErrorTypeConnectionFailed,
		Operation: operation,
		Resource:  resource,
		Message:   "failed to connect to storage",
		Cause:     cause,
	}
}

// NewStorageQueryError creates an error for storage query failures
func NewStorageQueryError(operation, resource string, cause error) *StorageError {
	return &StorageError{
		Type:      StorageErrorTypeQueryFailed,
		Operation: operation,
		Resource:  resource,
		Message:   "storage query failed",
		Cause:     cause,
	}
}

// NewStorageConstraintError creates an error for constraint violations
func NewStorageConstraintError(operation, resource string, cause error) *StorageError {
	return &StorageError{
		Type:      StorageErrorTypeConstraintViolation,
		Operation: operation,
		Resource:  resource,
		Message:   "storage constraint violation",
		Cause:     cause,
	}
}

// AuditError represents errors related to audit operations
type AuditError struct {
	Type    string
	Message string
	Cause   error
}

func (e *AuditError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("audit error [%s]: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("audit error [%s]: %s", e.Type, e.Message)
}

func (e *AuditError) Unwrap() error {
	return e.Cause
}

// Audit error types
const (
	AuditErrorTypeLogFailed       = "log_failed"
	AuditErrorTypeRetrievalFailed = "retrieval_failed"
	AuditErrorTypeInvalidQuery    = "invalid_query"
)

// NewAuditLogError creates an error for audit logging failures
func NewAuditLogError(message string, cause error) *AuditError {
	return &AuditError{
		Type:    AuditErrorTypeLogFailed,
		Message: message,
		Cause:   cause,
	}
}

// NewAuditRetrievalError creates an error for audit retrieval failures
func NewAuditRetrievalError(message string, cause error) *AuditError {
	return &AuditError{
		Type:    AuditErrorTypeRetrievalFailed,
		Message: message,
		Cause:   cause,
	}
}
