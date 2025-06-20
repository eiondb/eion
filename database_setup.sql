-- Complete Database Setup Script for Eion System
-- This script matches the actual schema expected by the BUN ORM models
-- Based on internal/memory/schema.go and internal/orchestrator models

-- ===============================================
-- ORCHESTRATOR SCHEMA (User/Session Management)
-- ===============================================

-- Users table (simplified - removed redundant fields)
CREATE TABLE IF NOT EXISTS users (
    uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL
);

-- Agent groups table (groups don't have permission levels directly)
CREATE TABLE IF NOT EXISTS agent_groups (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    agent_ids JSONB DEFAULT '[]'::jsonb,
    description TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT agent_group_id_not_empty CHECK (id != ''),
    CONSTRAINT agent_group_name_not_empty CHECK (name != ''),
    CONSTRAINT agent_group_ids_is_array CHECK (jsonb_typeof(agent_ids) = 'array')
);

-- Agents table (REMOVED agent_group_ids - agents should not reference groups directly)
CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    permission VARCHAR(10) NOT NULL DEFAULT 'r' CHECK (permission ~ '^[crud]+$'),
    description TEXT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'suspended')),
    guest BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT agent_id_not_empty CHECK (id != ''),
    CONSTRAINT agent_name_not_empty CHECK (name != ''),
    CONSTRAINT agent_permission_valid CHECK (permission ~ '^[crud]+$'),
    CONSTRAINT agent_status_valid CHECK (status IN ('active', 'inactive', 'suspended'))
);

-- Session types table (required by SDK)
CREATE TABLE IF NOT EXISTS session_types (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    agent_group_ids JSONB DEFAULT '[]'::jsonb,
    description TEXT NULL,
    encryption VARCHAR(50) NOT NULL DEFAULT 'SHA256',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT session_type_id_not_empty CHECK (id != ''),
    CONSTRAINT session_type_name_not_empty CHECK (name != ''),
    CONSTRAINT session_type_agent_group_ids_is_array CHECK (jsonb_typeof(agent_group_ids) = 'array')
);

-- Sessions table (REMOVED ended_at - use only deleted_at for consistency)
CREATE TABLE IF NOT EXISTS sessions (
    uuid VARCHAR(255) PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL UNIQUE,
    user_id VARCHAR(255) NOT NULL,
    session_type_id VARCHAR(255) NOT NULL,
    session_name VARCHAR(255) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    
    CONSTRAINT session_id_not_empty CHECK (session_id != ''),
    CONSTRAINT session_user_id_not_empty CHECK (user_id != ''),
    CONSTRAINT session_type_id_not_empty CHECK (session_type_id != ''),
    
    -- Foreign key constraints
    FOREIGN KEY (session_type_id) REFERENCES session_types(id) ON DELETE CASCADE
);

-- Interaction logs table for monitoring (updated to use user_id instead of space_id)
CREATE TABLE IF NOT EXISTS agent_interaction_logs (
    id VARCHAR(255) PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NULL,
    operation VARCHAR(50) NOT NULL,
    endpoint VARCHAR(255) NULL,
    method VARCHAR(10) NULL,
    success BOOLEAN NOT NULL DEFAULT true,
    error_msg TEXT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    request_data JSONB NULL,
    
    CONSTRAINT interaction_log_id_not_empty CHECK (id != ''),
    CONSTRAINT interaction_agent_id_not_empty CHECK (agent_id != ''),
    CONSTRAINT interaction_user_id_not_empty CHECK (user_id != ''),
    CONSTRAINT interaction_operation_not_empty CHECK (operation != '')
);

-- ===============================================
-- MEMORY/KNOWLEDGE SCHEMA (BUN ORM Models)
-- ===============================================

-- Messages table (space_id removed)
CREATE TABLE IF NOT EXISTS messages (
    uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    role_type VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,
    metadata JSONB DEFAULT '{}'::jsonb,
    embedding VECTOR(384) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    
    CONSTRAINT message_session_id_not_empty CHECK (session_id != ''),
    CONSTRAINT message_agent_id_not_empty CHECK (agent_id != ''),
    CONSTRAINT message_role_not_empty CHECK (role != ''),
    CONSTRAINT message_content_not_empty CHECK (content != '')
);

-- Facts table (space_id removed)
CREATE TABLE IF NOT EXISTS facts (
    uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content TEXT NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    rating DECIMAL(10,2) NOT NULL DEFAULT 0.0 CHECK (rating >= 0.0 AND rating <= 1.0),
    metadata JSONB DEFAULT '{}'::jsonb,
    embedding VECTOR(384) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,
    
    CONSTRAINT fact_content_not_empty CHECK (content != ''),
    CONSTRAINT fact_agent_id_not_empty CHECK (agent_id != '')
);

-- Message embeddings table (space_id removed)
CREATE TABLE IF NOT EXISTS message_embeddings (
    uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_uuid UUID NOT NULL UNIQUE,
    agent_id VARCHAR(255) NOT NULL,
    embedding VECTOR(384) NOT NULL,
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL
);

-- Fact embeddings table (space_id removed)
CREATE TABLE IF NOT EXISTS fact_embeddings (
    uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fact_uuid UUID NOT NULL UNIQUE,
    agent_id VARCHAR(255) NOT NULL,
    embedding VECTOR(384) NOT NULL,
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL
);

-- ===============================================
-- INDEXES FOR PERFORMANCE
-- ===============================================

-- User indexes (fixed to use user_id instead of id)
CREATE INDEX IF NOT EXISTS idx_users_user_id ON users(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at) WHERE deleted_at IS NULL;

-- Session indexes (removed ended_at references)
CREATE INDEX IF NOT EXISTS idx_sessions_session_id ON sessions(session_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_session_type ON sessions(session_type_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at) WHERE deleted_at IS NULL;

-- Message indexes (updated without space_id)
CREATE INDEX IF NOT EXISTS idx_messages_session_agent ON messages(session_id, agent_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_agent_id ON messages(agent_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_embedding ON messages USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100) WHERE deleted_at IS NULL AND embedding IS NOT NULL;

-- Fact indexes (updated without space_id)
CREATE INDEX IF NOT EXISTS idx_facts_agent_id ON facts(agent_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_facts_rating ON facts(rating DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_facts_created_at ON facts(created_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_facts_embedding ON facts USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100) WHERE deleted_at IS NULL AND embedding IS NOT NULL;

-- Embedding indexes (updated without space_id)
CREATE INDEX IF NOT EXISTS idx_message_embeddings_agent_id ON message_embeddings(agent_id) WHERE deleted_at IS NULL AND is_deleted = false;
CREATE INDEX IF NOT EXISTS idx_message_embeddings_embedding ON message_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100) WHERE deleted_at IS NULL AND is_deleted = false;
CREATE INDEX IF NOT EXISTS idx_fact_embeddings_agent_id ON fact_embeddings(agent_id) WHERE deleted_at IS NULL AND is_deleted = false;
CREATE INDEX IF NOT EXISTS idx_fact_embeddings_embedding ON fact_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100) WHERE deleted_at IS NULL AND is_deleted = false;

-- Orchestrator indexes (removed agent_group_ids from agents)
CREATE INDEX IF NOT EXISTS idx_agent_groups_agent_ids ON agent_groups USING GIN (agent_ids);
CREATE INDEX IF NOT EXISTS idx_agent_groups_created_at ON agent_groups(created_at);

-- Agent indexes (removed agent_group_ids reference)
CREATE INDEX IF NOT EXISTS idx_agents_permission ON agents(permission);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_created_at ON agents(created_at);

CREATE INDEX IF NOT EXISTS idx_session_types_agent_group_ids ON session_types USING GIN (agent_group_ids);
CREATE INDEX IF NOT EXISTS idx_session_types_created_at ON session_types(created_at);

CREATE INDEX IF NOT EXISTS idx_agent_interaction_logs_agent_id ON agent_interaction_logs(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_interaction_logs_user_id ON agent_interaction_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_interaction_logs_session_id ON agent_interaction_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_agent_interaction_logs_timestamp ON agent_interaction_logs(timestamp);

-- ===============================================
-- UPDATE TRIGGERS
-- ===============================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Orchestrator triggers
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sessions_updated_at 
    BEFORE UPDATE ON sessions 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agent_groups_updated_at 
    BEFORE UPDATE ON agent_groups 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agents_updated_at 
    BEFORE UPDATE ON agents 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_session_types_updated_at 
    BEFORE UPDATE ON session_types 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Memory service triggers
CREATE TRIGGER update_messages_updated_at 
    BEFORE UPDATE ON messages 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_facts_updated_at 
    BEFORE UPDATE ON facts 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_message_embeddings_updated_at 
    BEFORE UPDATE ON message_embeddings 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_fact_embeddings_updated_at 
    BEFORE UPDATE ON fact_embeddings 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ===============================================
-- DEFAULT DATA (as specified in SDK.md)
-- ===============================================

-- Insert default agent group
INSERT INTO agent_groups (id, name, agent_ids, description) 
VALUES ('default', 'Default Agent Group', '[]'::jsonb, 'Default agent group for system initialization')
ON CONFLICT (id) DO NOTHING;

-- Insert default session type with agent group 'default'
INSERT INTO session_types (id, name, agent_group_ids, description, encryption) 
VALUES ('default', 'Default Session Type', '["default"]'::jsonb, 'Default session type for system initialization', 'SHA256')
ON CONFLICT (id) DO NOTHING;

-- ===============================================
-- COMPLETION MESSAGE
-- ===============================================

DO $$
BEGIN
    RAISE NOTICE 'Eion database setup completed successfully!';
END $$; 