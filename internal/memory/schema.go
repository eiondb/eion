package memory

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// BaseSchema provides common fields for all database tables
type BaseSchema struct {
	bun.BaseModel `bun:"table:base_schema,alias:bs"`

	CreatedAt time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
}

// SessionSchema represents the sessions table (updated for orchestrator schema)
type SessionSchema struct {
	bun.BaseModel `bun:"table:sessions,alias:s"`
	BaseSchema

	UUID          uuid.UUID `bun:"uuid,pk,type:uuid" json:"uuid"`
	SessionID     string    `bun:"session_id,notnull,unique" json:"session_id"`
	UserID        string    `bun:"user_id,notnull" json:"user_id"`
	SessionTypeID string    `bun:"session_type_id,notnull" json:"session_type_id"`
	SessionName   *string   `bun:"session_name" json:"session_name,omitempty"`
}

// MessageSchema represents the messages table
type MessageSchema struct {
	bun.BaseModel `bun:"table:messages,alias:m"`
	BaseSchema

	UUID       uuid.UUID      `bun:"uuid,pk,type:uuid" json:"uuid"`
	SessionID  string         `bun:"session_id,notnull" json:"session_id"`
	AgentID    string         `bun:"agent_id,notnull" json:"agent_id"`
	Role       string         `bun:"role,notnull" json:"role"`
	RoleType   string         `bun:"role_type,notnull" json:"role_type"`
	Content    string         `bun:"content,notnull" json:"content"`
	TokenCount int            `bun:"token_count,notnull,default:0" json:"token_count"`
	Metadata   map[string]any `bun:"metadata,type:jsonb" json:"metadata,omitempty"`
	Embedding  []float32      `bun:"embedding,type:vector(384),nullzero" json:"embedding,omitempty"`
}

// FactSchema represents the facts table
type FactSchema struct {
	bun.BaseModel `bun:"table:facts,alias:f"`
	BaseSchema

	UUID      uuid.UUID      `bun:"uuid,pk,type:uuid" json:"uuid"`
	Content   string         `bun:"content,notnull" json:"content"`
	AgentID   string         `bun:"agent_id,notnull" json:"agent_id"`
	Rating    float64        `bun:"rating,notnull,default:0.0" json:"rating"`
	Metadata  map[string]any `bun:"metadata,type:jsonb" json:"metadata,omitempty"`
	Embedding []float32      `bun:"embedding,type:vector(384),nullzero" json:"embedding,omitempty"`
}

// MessageEmbeddingSchema represents the message_embeddings table for vector search
type MessageEmbeddingSchema struct {
	bun.BaseModel `bun:"table:message_embeddings,alias:me"`
	BaseSchema

	UUID        uuid.UUID `bun:"uuid,pk,type:uuid" json:"uuid"`
	MessageUUID uuid.UUID `bun:"message_uuid,notnull,unique" json:"message_uuid"`
	AgentID     string    `bun:"agent_id,notnull" json:"agent_id"`
	Embedding   []float32 `bun:"embedding,type:vector(384),notnull" json:"embedding"`
	IsDeleted   bool      `bun:"is_deleted,notnull,default:false" json:"is_deleted"`
}

// FactEmbeddingSchema represents the fact_embeddings table for vector search
type FactEmbeddingSchema struct {
	bun.BaseModel `bun:"table:fact_embeddings,alias:fe"`
	BaseSchema

	UUID      uuid.UUID `bun:"uuid,pk,type:uuid" json:"uuid"`
	FactUUID  uuid.UUID `bun:"fact_uuid,notnull,unique" json:"fact_uuid"`
	AgentID   string    `bun:"agent_id,notnull" json:"agent_id"`
	Embedding []float32 `bun:"embedding,type:vector(384),notnull" json:"embedding"`
	IsDeleted bool      `bun:"is_deleted,notnull,default:false" json:"is_deleted"`
}

// UserSchema represents the users table
type UserSchema struct {
	bun.BaseModel `bun:"table:users,alias:u"`
	BaseSchema

	UUID   uuid.UUID `bun:"uuid,pk,type:uuid" json:"uuid"`
	UserID string    `bun:"user_id,notnull,unique" json:"user_id"`
	Name   *string   `bun:"name" json:"name,omitempty"`
}

// User represents a user in the system (for memory package)
type User struct {
	UUID      uuid.UUID `json:"uuid"`
	UserID    string    `json:"user_id"`
	Name      *string   `json:"name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetTableName returns the table name for the schema
func (s *SessionSchema) GetTableName() string {
	return "sessions"
}

func (m *MessageSchema) GetTableName() string {
	return "messages"
}

func (f *FactSchema) GetTableName() string {
	return "facts"
}

func (me *MessageEmbeddingSchema) GetTableName() string {
	return "message_embeddings"
}

func (fe *FactEmbeddingSchema) GetTableName() string {
	return "fact_embeddings"
}

func (u *UserSchema) GetTableName() string {
	return "users"
}

// Conversion functions from schema to models

func SessionSchemaToSession(schema SessionSchema) *Session {
	return &Session{
		UUID:          schema.UUID,
		SessionID:     schema.SessionID,
		UserID:        schema.UserID,
		SessionTypeID: schema.SessionTypeID,
		SessionName:   schema.SessionName,
		CreatedAt:     schema.CreatedAt,
		UpdatedAt:     schema.UpdatedAt,
	}
}

func MessageSchemaToMessage(schema MessageSchema) *Message {
	return &Message{
		UUID:       schema.UUID,
		CreatedAt:  schema.CreatedAt,
		UpdatedAt:  schema.UpdatedAt,
		AgentID:    schema.AgentID,
		SessionID:  schema.SessionID,
		Role:       schema.Role,
		RoleType:   RoleType(schema.RoleType),
		Content:    schema.Content,
		Metadata:   schema.Metadata,
		TokenCount: schema.TokenCount,
		Embedding:  schema.Embedding,
	}
}

func FactSchemaToFact(schema FactSchema) *Fact {
	return &Fact{
		UUID:      schema.UUID,
		Content:   schema.Content,
		AgentID:   schema.AgentID,
		CreatedAt: schema.CreatedAt,
		UpdatedAt: schema.UpdatedAt,
		Rating:    schema.Rating,
		Metadata:  schema.Metadata,
		Embedding: schema.Embedding,
	}
}

// Conversion functions from models to schema

func SessionToSessionSchema(session *Session) SessionSchema {
	return SessionSchema{
		UUID:          session.UUID,
		SessionID:     session.SessionID,
		UserID:        session.UserID,
		SessionTypeID: session.SessionTypeID,
		SessionName:   session.SessionName,
		BaseSchema: BaseSchema{
			CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
		},
	}
}

func MessageToMessageSchema(message *Message) MessageSchema {
	return MessageSchema{
		UUID:       message.UUID,
		SessionID:  message.SessionID,
		AgentID:    message.AgentID,
		Role:       message.Role,
		RoleType:   string(message.RoleType),
		Content:    message.Content,
		TokenCount: message.TokenCount,
		Metadata:   message.Metadata,
		Embedding:  message.Embedding,
		BaseSchema: BaseSchema{
			CreatedAt: message.CreatedAt,
			UpdatedAt: message.UpdatedAt,
		},
	}
}

func FactToFactSchema(fact *Fact) FactSchema {
	return FactSchema{
		UUID:      fact.UUID,
		Content:   fact.Content,
		AgentID:   fact.AgentID,
		Rating:    fact.Rating,
		Metadata:  fact.Metadata,
		Embedding: fact.Embedding,
		BaseSchema: BaseSchema{
			CreatedAt: fact.CreatedAt,
			UpdatedAt: fact.UpdatedAt,
		},
	}
}

// Conversion functions for UserSchema

func UserSchemaToUser(schema UserSchema) *User {
	return &User{
		UUID:      schema.UUID,
		UserID:    schema.UserID,
		Name:      schema.Name,
		CreatedAt: schema.CreatedAt,
		UpdatedAt: schema.UpdatedAt,
	}
}

func UserToUserSchema(user *User) UserSchema {
	return UserSchema{
		UUID:   user.UUID,
		UserID: user.UserID,
		Name:   user.Name,
		BaseSchema: BaseSchema{
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
	}
}

// Database indexes - updated to remove space_id references
var SessionIndexes = []string{
	"CREATE INDEX IF NOT EXISTS idx_sessions_session_id ON sessions(session_id) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_sessions_session_type ON sessions(session_type_id) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at) WHERE deleted_at IS NULL",
}

var UserIndexes = []string{
	"CREATE INDEX IF NOT EXISTS idx_users_user_id ON users(user_id) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at) WHERE deleted_at IS NULL",
}

var MessageIndexes = []string{
	"CREATE INDEX IF NOT EXISTS idx_messages_session_agent ON messages(session_id, agent_id) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_messages_agent_id ON messages(agent_id) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_messages_embedding ON messages USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100) WHERE deleted_at IS NULL AND embedding IS NOT NULL",
}

var FactIndexes = []string{
	"CREATE INDEX IF NOT EXISTS idx_facts_agent_id ON facts(agent_id) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_facts_rating ON facts(rating DESC) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_facts_created_at ON facts(created_at) WHERE deleted_at IS NULL",
	"CREATE INDEX IF NOT EXISTS idx_facts_embedding ON facts USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100) WHERE deleted_at IS NULL AND embedding IS NOT NULL",
}

var EmbeddingIndexes = []string{
	"CREATE INDEX IF NOT EXISTS idx_message_embeddings_agent_id ON message_embeddings(agent_id) WHERE deleted_at IS NULL AND is_deleted = false",
	"CREATE INDEX IF NOT EXISTS idx_message_embeddings_embedding ON message_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100) WHERE deleted_at IS NULL AND is_deleted = false",
	"CREATE INDEX IF NOT EXISTS idx_fact_embeddings_agent_id ON fact_embeddings(agent_id) WHERE deleted_at IS NULL AND is_deleted = false",
	"CREATE INDEX IF NOT EXISTS idx_fact_embeddings_embedding ON fact_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100) WHERE deleted_at IS NULL AND is_deleted = false",
}
