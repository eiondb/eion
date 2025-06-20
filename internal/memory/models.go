package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Constants for memory operations
const (
	DefaultLastNMessages = 50
	MaxMessagesPerMemory = 30
	MaxMessageLength     = 50000
)

type RoleType string

const (
	NoRole        RoleType = "norole"
	SystemRole    RoleType = "system"
	AssistantRole RoleType = "assistant"
	UserRole      RoleType = "user"
	FunctionRole  RoleType = "function"
	ToolRole      RoleType = "tool"
)

var validRoleTypes = map[string]RoleType{
	string(NoRole):        NoRole,
	string(SystemRole):    SystemRole,
	string(AssistantRole): AssistantRole,
	string(UserRole):      UserRole,
	string(FunctionRole):  FunctionRole,
	string(ToolRole):      ToolRole,
}

func (rt *RoleType) UnmarshalJSON(b []byte) error {
	str := strings.Trim(string(b), "\"")

	if str == "" {
		*rt = NoRole
		return nil
	}

	value, ok := validRoleTypes[str]
	if !ok {
		return fmt.Errorf("invalid RoleType: %v", str)
	}

	*rt = value
	return nil
}

func (rt RoleType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", rt)), nil
}

// Message represents a message in an agent conversation within Eion
type Message struct {
	// The unique identifier of the message
	UUID uuid.UUID `json:"uuid"`
	// The timestamp of when the message was created
	CreatedAt time.Time `json:"created_at"`
	// The timestamp of when the message was last updated
	UpdatedAt time.Time `json:"updated_at"`
	// The agent ID that created this message
	AgentID string `json:"agent_id"`
	// The session ID for grouping related messages
	SessionID string `json:"session_id"`
	// The role of the sender of the message (e.g., "user", "assistant")
	Role string `json:"role"`
	// The type of the role (e.g., "user", "system")
	RoleType RoleType `json:"role_type,omitempty"`
	// The content of the message
	Content string `json:"content"`
	// The metadata associated with the message
	Metadata map[string]any `json:"metadata,omitempty"`
	// The number of tokens in the message
	TokenCount int `json:"token_count"`
	// Vector embedding for semantic search
	Embedding []float32 `json:"embedding,omitempty"`
}

// Memory represents a collection of messages and facts for an agent session
type Memory struct {
	// A list of message objects, where each message contains a role and content
	Messages []Message `json:"messages"`
	// Relevant facts extracted from the conversation
	RelevantFacts []Fact `json:"relevant_facts"`
	// A dictionary containing metadata associated with the memory
	Metadata map[string]any `json:"metadata,omitempty"`
	// The agent ID this memory belongs to
	AgentID string `json:"agent_id"`
	// The session ID for this memory
	SessionID string `json:"session_id"`
}

// Fact represents a structured piece of knowledge extracted from conversations
type Fact struct {
	// Unique identifier for the fact
	UUID uuid.UUID `json:"uuid"`
	// The fact content
	Content string `json:"content"`
	// The agent that contributed this fact
	AgentID string `json:"agent_id"`
	// When this fact was created
	CreatedAt time.Time `json:"created_at"`
	// When this fact was last updated
	UpdatedAt time.Time `json:"updated_at"`
	// Confidence rating of this fact (0.0 to 1.0)
	Rating float64 `json:"rating"`
	// Metadata associated with the fact
	Metadata map[string]any `json:"metadata,omitempty"`
	// Vector embedding for semantic search
	Embedding []float32 `json:"embedding,omitempty"`
}

// MessageMetadataUpdate represents a request to update message metadata
type MessageMetadataUpdate struct {
	// The metadata to update
	Metadata map[string]any `json:"metadata" validate:"required"`
}

// MessageListResponse represents a paginated list of messages
type MessageListResponse struct {
	// A list of message objects
	Messages []Message `json:"messages"`
	// The total number of messages
	TotalCount int `json:"total_count"`
	// The number of messages returned
	RowCount int `json:"row_count"`
}

// MemorySearchQuery represents a search query for memories
type MemorySearchQuery struct {
	// The search text
	Text string `json:"text"`
	// Optional agent ID to filter by
	AgentID string `json:"agent_id,omitempty"`
	// Maximum number of results to return
	Limit int `json:"limit"`
	// Minimum relevance score threshold
	MinScore float64 `json:"min_score,omitempty"`
	// Additional metadata filters
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MemorySearchResult represents search results
type MemorySearchResult struct {
	// Matching messages
	Messages []Message `json:"messages"`
	// Matching facts
	Facts []Fact `json:"facts"`
	// Total number of results found
	TotalCount int `json:"total_count"`
	// Search metadata
	SearchMetadata map[string]any `json:"search_metadata,omitempty"`
}

// AddMemoryRequest represents a request to add memory to a session
type AddMemoryRequest struct {
	// Messages to add
	Messages []Message `json:"messages" validate:"required,min=1,max=30"`
	// Optional metadata
	Metadata map[string]any `json:"metadata,omitempty"`
}

// FilterOption represents options for filtering memory queries
type FilterOption struct {
	// Minimum fact rating
	MinRating *float64 `json:"min_rating,omitempty"`
	// Maximum results
	Limit *int `json:"limit,omitempty"`
	// Metadata filters
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Session represents a memory session (updated to match orchestrator schema)
type Session struct {
	UUID          uuid.UUID `json:"uuid"`
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	SessionName   *string   `json:"session_name,omitempty"`
	SessionTypeID string    `json:"session_type_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateSessionRequest represents a request to create a new memory session
type CreateSessionRequest struct {
	SessionID     string  `json:"session_id"`
	UserID        string  `json:"user_id"`
	SessionTypeID string  `json:"session_type_id"`
	SessionName   *string `json:"session_name,omitempty"`
}

// UpdateSessionRequest represents a request to update a memory session
type UpdateSessionRequest struct {
	SessionID     string  `json:"session_id"`
	UserID        string  `json:"user_id"`
	SessionTypeID string  `json:"session_type_id"`
	SessionName   *string `json:"session_name,omitempty"`
}
