# Live Agency Implementation Plan

## Executive Summary

This plan implements Live Agency as a **real-time collaborative layer** extending Eion's existing services. The implementation follows industry standards for real-time systems while leveraging Eion's current architecture of Orchestrator, Memory, and Knowledge services.

**Target Performance**: 500 messages/second per session, <50ms latency, 99.99% uptime SLA
**Timeline**: 4 phases over 12-16 weeks
**Architecture**: Event-driven microservices with Redis Streams backbone

---

## Technical Architecture Decisions

### 1. **Core Technology Stack**
- **Real-time Engine**: Redis Streams + WebSocket servers (Go)
- **State Management**: PostgreSQL + Redis for hybrid persistence/cache
- **Event Broadcasting**: Redis Pub/Sub with selective subscriptions
- **Conflict Resolution**: Optimistic locking + Vector clocks + CRDTs (future)
- **Authentication**: JWT with 15-minute expiration + refresh tokens

### 2. **Service Extensions**
- **New: Live Service** - WebSocket management, presence tracking, real-time coordination
- **Extended: Orchestrator** - Live session management, permission validation, conflict arbitration
- **Extended: Memory Service** - Real-time memory updates, collaborative editing support
- **Extended: Knowledge Service** - Live knowledge graph updates, fact collaboration

### 3. **Industry Standards Compliance**
- **Message Throughput**: 500 msg/sec per session (industry standard: 100-1000)
- **Latency Target**: <50ms (industry leader: Google Docs <50ms, Figma <50ms)
- **Uptime SLA**: 99.99% (enterprise standard)
- **Authentication**: JWT tokens in query parameters (industry standard despite logging concerns)
- **Connection Management**: 30-second ping/pong, exponential backoff reconnection

---

## Phase 1: Real-time Infrastructure Foundation (Weeks 1-4)

### Step 1.1: Live Service Implementation
**Purpose**: Core WebSocket server with authentication and presence tracking

**Components**:
```go
// New service: internal/live/
├── server.go           // WebSocket server with JWT auth
├── connection_manager.go // Connection pooling and lifecycle
├── presence_tracker.go  // Agent presence and heartbeat
├── message_router.go    // Message routing and broadcasting
├── auth_middleware.go   // JWT validation and session mapping
└── interfaces.go       // Service contracts
```

**Key Features**:
- **JWT Authentication**: Query parameter validation with 15-minute expiration
- **Connection Management**: Auto-reconnection with exponential backoff (1s→2s→4s→8s→30s max)
- **Presence Tracking**: Real-time agent online/offline status with heartbeat every 30 seconds
- **Message Routing**: Session-based message broadcasting with selective subscriptions
- **Rate Limiting**: 500 messages/second per session, 10 messages/second per agent

### Step 1.2: Redis Streams Integration
**Purpose**: Event backbone for real-time updates

**Implementation**:
```go
// Stream structure
streams := map[string]string{
    "session:{sessionId}:memory":    // Memory updates
    "session:{sessionId}:knowledge": // Knowledge graph changes  
    "session:{sessionId}:presence":  // Agent presence changes
    "session:{sessionId}:conflicts": // Conflict notifications
}
```

**Event Types**:
- `AGENT_JOIN`, `AGENT_LEAVE`, `AGENT_HEARTBEAT`
- `MEMORY_CREATE`, `MEMORY_UPDATE`, `MEMORY_DELETE`
- `FACT_CREATE`, `FACT_UPDATE`, `FACT_DELETE`
- `CONFLICT_DETECTED`, `CONFLICT_RESOLVED`

### Step 1.3: Orchestrator Extensions
**Purpose**: Live session management and permission validation

**New Components**:
```go
// internal/orchestrator/live/
├── session_manager.go   // Live session lifecycle
├── permission_validator.go // Real-time permission checks
├── presence_coordinator.go // Agent presence coordination
└── conflict_detector.go    // Basic conflict detection
```

**Database Schema Extensions**:
```sql
-- Live session tracking
CREATE TABLE live_sessions (
    id UUID PRIMARY KEY,
    session_id UUID REFERENCES sessions(id),
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    agent_count INTEGER DEFAULT 0,
    status live_session_status DEFAULT 'active'
);

-- Agent presence tracking  
CREATE TABLE agent_presence (
    id UUID PRIMARY KEY,
    live_session_id UUID REFERENCES live_sessions(id),
    agent_id UUID REFERENCES agents(id),
    status presence_status DEFAULT 'online',
    last_heartbeat TIMESTAMP DEFAULT NOW(),
    connection_id TEXT,
    metadata JSONB
);
```

**Phase 1 Deliverables**:
- [ ] Live Service WebSocket server
- [ ] Redis Streams event system
- [ ] Basic presence tracking
- [ ] JWT authentication system
- [ ] Database schema migrations
- [ ] Unit tests (90%+ coverage)
- [ ] Load testing framework
- [ ] Performance baseline measurements

---

## Phase 2: Dynamic Permissions & Access Control (Weeks 5-8)

### Step 2.1: Advanced Permission System
**Purpose**: Dynamic, time-based permissions with inheritance

**Database Schema**:
```sql
-- Dynamic permissions with expiration
CREATE TABLE dynamic_permissions (
    id UUID PRIMARY KEY,
    agent_id UUID REFERENCES agents(id),
    resource_type permission_resource_type, -- 'session', 'memory', 'fact'
    resource_id UUID,
    permission_type permission_type[], -- ['read', 'write', 'delete']
    granted_by UUID REFERENCES agents(id),
    granted_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    conditions JSONB, -- Context-based conditions
    status permission_status DEFAULT 'active'
);

-- Permission inheritance hierarchy
CREATE TABLE permission_templates (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    role_type TEXT, -- 'supervisor', 'peer', 'readonly'
    permissions JSONB, -- Template permissions
    inheritance_rules JSONB, -- How permissions cascade
    created_at TIMESTAMP DEFAULT NOW()
);

-- Agent role assignments with hierarchy
CREATE TABLE agent_roles (
    id UUID PRIMARY KEY,
    agent_id UUID REFERENCES agents(id),
    session_id UUID REFERENCES sessions(id),
    role_id UUID REFERENCES permission_templates(id),
    granted_by UUID REFERENCES agents(id),
    granted_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    scope JSONB -- What resources this role applies to
);
```

### Step 2.2: External Agent Access
**Purpose**: Temporary, scoped access for external agents

**Components**:
```go
// internal/orchestrator/external/
├── access_manager.go    // External access grants
├── whitelist_validator.go // Developer whitelist validation
├── scope_limiter.go     // Resource scope limitations
└── cleanup_scheduler.go // Automatic permission cleanup
```

**Access Control Flow**:
1. **Developer Request**: Developer requests external agent access to specific session/resources
2. **Approval Workflow**: Internal agent/supervisor approves access request
3. **Scoped Grant**: Time-limited access with specific resource permissions
4. **Auto-cleanup**: Automatic revocation on expiration
5. **Audit Trail**: Complete access logging for compliance

### Step 2.3: Real-time Permission Validation
**Purpose**: Live permission checking with caching

**Implementation Strategy**:
- **Redis Cache**: Permission cache with TTL matching JWT expiration
- **Event-driven Updates**: Permission changes broadcast to live agents
- **Fallback to DB**: Cache miss fallback to PostgreSQL
- **Permission Inheritance**: Real-time role-based permission calculation

**Phase 2 Deliverables**:
- [ ] Dynamic permission system
- [ ] External agent access management
- [ ] Permission inheritance engine
- [ ] Real-time permission validation
- [ ] Redis permission caching
- [ ] Approval workflow engine
- [ ] Access audit logging
- [ ] Integration tests for permission scenarios

---

## Phase 3: Real-time Conflict Detection & Resolution (Weeks 9-12)

### Step 3.1: Optimistic Locking System
**Purpose**: Prevent concurrent write conflicts

**Database Schema**:
```sql
-- Resource versioning for optimistic locking
CREATE TABLE resource_versions (
    id UUID PRIMARY KEY,
    resource_type TEXT, -- 'memory', 'fact', 'session'
    resource_id UUID,
    version_number INTEGER,
    last_modified_by UUID REFERENCES agents(id),
    last_modified_at TIMESTAMP DEFAULT NOW(),
    checksum TEXT, -- Content hash for verification
    metadata JSONB
);

-- Concurrent operation tracking
CREATE TABLE concurrent_operations (
    id UUID PRIMARY KEY,
    resource_type TEXT,
    resource_id UUID,
    operation_type TEXT, -- 'create', 'update', 'delete'
    agent_id UUID REFERENCES agents(id),
    started_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP,
    status operation_status DEFAULT 'in_progress'
);
```

### Step 3.2: Vector Clock Implementation
**Purpose**: Complex causality tracking for distributed operations

**Components**:
```go
// internal/orchestrator/causality/
├── vector_clock.go      // Vector clock implementation
├── causality_tracker.go // Operation ordering
├── conflict_analyzer.go // Conflict detection logic
└── resolution_engine.go // Automatic conflict resolution
```

**Vector Clock Structure**:
```go
type VectorClock struct {
    AgentID   string            `json:"agent_id"`
    Clock     map[string]int64  `json:"clock"`
    Timestamp time.Time         `json:"timestamp"`
}

type Operation struct {
    ID           string      `json:"id"`
    Type         string      `json:"type"`
    ResourceID   string      `json:"resource_id"`
    AgentID      string      `json:"agent_id"`
    VectorClock  VectorClock `json:"vector_clock"`
    Data         interface{} `json:"data"`
    Dependencies []string    `json:"dependencies"`
}
```

### Step 3.3: Live Conflict Notifications
**Purpose**: Real-time conflict detection and agent notification

**Event Flow**:
1. **Operation Start**: Agent begins operation, vector clock incremented
2. **Conflict Detection**: System detects concurrent operations on same resource
3. **Live Notification**: Conflicting agents receive real-time conflict alerts
4. **Resolution Options**: Agents can abort, retry, or request manual resolution
5. **State Synchronization**: Final state broadcast to all affected agents

**Phase 3 Deliverables**:
- [ ] Optimistic locking system
- [ ] Vector clock implementation
- [ ] Conflict detection engine
- [ ] Live conflict notifications
- [ ] Automatic conflict resolution
- [ ] Manual resolution interface
- [ ] Conflict analytics and reporting
- [ ] Stress testing with concurrent operations

---

## Phase 4: Multi-Agency Collaboration (Weeks 13-16)

### Step 4.1: Shared Knowledge Spaces
**Purpose**: Temporary joint spaces for cross-agency collaboration

**Database Schema**:
```sql
-- Shared collaboration spaces
CREATE TABLE shared_spaces (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    participating_agencies UUID[], -- Array of agency IDs
    created_by UUID REFERENCES agents(id),
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    access_policy JSONB, -- Who can access what
    status shared_space_status DEFAULT 'active'
);

-- Cross-agency agent access
CREATE TABLE cross_agency_access (
    id UUID PRIMARY KEY,
    shared_space_id UUID REFERENCES shared_spaces(id),
    external_agent_id UUID, -- External agent reference
    local_sponsor_id UUID REFERENCES agents(id), -- Internal sponsor
    permissions JSONB, -- What they can do
    granted_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    last_activity TIMESTAMP,
    status access_status DEFAULT 'active'
);
```

### Step 4.2: Cross-Agency Knowledge Correlation
**Purpose**: Live fact correlation across agency boundaries

**Components**:
```python
# internal/knowledge/python/cross_agency/
├── correlation_engine.py    # Cross-agency fact correlation
├── shared_graph_manager.py  # Shared knowledge graph views
├── privacy_filter.py        # Data redaction and privacy
└── compliance_monitor.py    # Regulatory compliance tracking
```

**Correlation Features**:
- **Fact Linking**: Automatic correlation of related facts across agencies
- **Privacy Preservation**: Automatic redaction of sensitive information
- **Audit Trail**: Complete tracking of cross-agency data access
- **Compliance Monitoring**: Real-time regulatory compliance validation

### Step 4.3: Approval Workflows
**Purpose**: Enterprise-grade approval chains for sensitive operations

**Workflow Engine**:
```go
// internal/orchestrator/workflows/
├── approval_engine.go       // Workflow orchestration
├── rule_evaluator.go       // Business rule evaluation
├── notification_manager.go  // Approval notifications
└── escalation_handler.go   // Escalation and timeouts
```

**Workflow Examples**:
- **Cross-Agency Access**: Multi-level approval for external access
- **Sensitive Data Sharing**: Supervisor approval for classified information
- **Emergency Access**: Fast-track approval with post-access review
- **Compliance Review**: Automatic compliance checking with manual override

**Phase 4 Deliverables**:
- [ ] Shared knowledge spaces
- [ ] Cross-agency access management
- [ ] Knowledge correlation engine
- [ ] Privacy filtering system
- [ ] Approval workflow engine
- [ ] Compliance monitoring
- [ ] Cross-agency audit trails
- [ ] End-to-end integration testing

---

## Risk Assessment & Mitigation

### 1. **Performance Risks**
**Risk**: System unable to meet 500 msg/sec, <50ms latency targets
**Mitigation**: 
- Load testing from Phase 1
- Redis Streams horizontal scaling capability
- Connection pooling and message batching
- Graceful degradation under high load

### 2. **Security Risks**
**Risk**: JWT tokens logged in query parameters, unauthorized access
**Mitigation**:
- Log scrubbing for sensitive data
- Short JWT expiration (15 minutes)
- IP-based access controls
- Comprehensive audit logging

### 3. **Data Consistency Risks**
**Risk**: Conflict resolution failures, data corruption
**Mitigation**:
- Optimistic locking with rollback capability
- Vector clock validation before operations
- Backup and restore procedures
- Manual conflict resolution interface

### 4. **Scalability Risks**
**Risk**: System degradation under high agent count
**Mitigation**:
- Horizontal scaling architecture
- Connection load balancing
- Database query optimization
- Redis cluster configuration

---

## Testing Strategy

### 1. **Unit Testing** (Ongoing)
- All new services: 90%+ code coverage
- Mock external dependencies (Redis, PostgreSQL, Neo4j)
- Vector clock and conflict resolution logic
- Permission inheritance calculations

### 2. **Integration Testing** (Each phase)
- WebSocket connection handling
- Redis Streams event flow
- Database transaction consistency
- Service-to-service communication

### 3. **Performance Testing** (Phase 1, 3, 4)
- Load testing: 1000 concurrent connections
- Throughput testing: 500 messages/second sustained
- Latency testing: <50ms p95 response time
- Memory usage under load

### 4. **Security Testing** (Phase 2, 4)
- JWT token validation edge cases
- Permission bypass attempts
- Cross-agency access boundary testing
- SQL injection and data validation

### 5. **End-to-End Testing** (Phase 4)
- Multi-agent collaboration scenarios
- Cross-agency workflow testing
- Conflict resolution user stories
- Disaster recovery procedures

---

## Deployment Strategy

### 1. **Backward Compatibility**
- All existing APIs remain unchanged
- Live Agency features opt-in via feature flags
- Database migrations with rollback capability
- Progressive rollout by session

### 2. **Infrastructure Requirements**
- **Redis Cluster**: 3-node cluster for high availability
- **WebSocket Load Balancer**: Sticky session support
- **Database Scaling**: Read replicas for performance
- **Monitoring**: Prometheus + Grafana dashboards

### 3. **Rollout Plan**
- **Phase 1**: Internal testing with synthetic agents
- **Phase 2**: Limited beta with select customers
- **Phase 3**: Gradual rollout to all customers
- **Phase 4**: Full production deployment

---

## Success Metrics

### Performance Metrics
- **Latency**: <50ms p95 response time
- **Throughput**: 500 messages/second sustained per session
- **Uptime**: 99.99% availability
- **Connection Recovery**: <5 seconds average reconnection time

### Business Metrics
- **Agent Collaboration**: Concurrent agents per session
- **Conflict Resolution**: Automatic vs manual resolution ratio
- **Cross-Agency Usage**: External access requests and approvals
- **Developer Adoption**: Live Agency feature usage rates

### Technical Metrics
- **Resource Usage**: Memory and CPU consumption
- **Database Performance**: Query response times
- **Cache Hit Rate**: Redis cache effectiveness
- **Error Rates**: Connection failures and timeouts

This implementation plan provides a robust, industry-standard approach to implementing Live Agency while maintaining Eion's architectural integrity and ensuring scalable, secure real-time collaboration.

---
