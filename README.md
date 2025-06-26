<div align="center">
  <img src="assets/eion-navy.png#gh-light-mode-only" alt="Eion Logo" width="200" height="200">
  <img src="assets/eion-cream.png#gh-dark-mode-only" alt="Eion Logo" width="200" height="200">
  
  <h1 style="border-bottom: none; margin-bottom: 0;">Eion</h1>
  
  *Connecting AI agents through shared memory and collaborative intelligence.*

  ![Version](https://img.shields.io/badge/Version-v0.1.3-green)
  [![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

</div>

<div align="center">

<img src="assets/eion-demo.gif" alt="Eion Demo" width="90%" />

</div>

&nbsp;

**Eion** is a shared memory storage that provides unified knowledge graph capabilities for multi-agent systems, adapting to different AI deployment scenarios from single LLM applications to complex multi-agency systems.

### 1. LLM Application
```
User ↔ LLM Application → Eion (context storage)
```

### 2. AI Agent Application  
```
Business Logic ↔ AI Agent → Eion (memory + knowledge graph)
```

### 3. Agency (Multi-Agent) Systems
#### 3a. Sequential Agency
```
Agent A → context → Agent B → context → Agent C
                ↓              ↓              ↓
              Eion ← shared memory & knowledge → Eion
```
#### 3b. Concurrent Live Agency (WIP)
```
Agent A ──┐
          ├── shared live context ← Eion (live sync + notifications)
Agent B ──┤
          │
Agent C ──┘
```

### 4. External Guest Agent Access
```
Internal Agency: Agent A ↔ Agent B → Eion ← External Agent C (guest)
                                            ↑
                                    (controlled access)
```

## Quick Start

### Prerequisites

- **Docker & Docker Compose**: For PostgreSQL and Neo4j
- **Go 1.21+**: For the Eion server
- **Python 3.13+**: For knowledge extraction services

### 1. Clone and Setup

```bash
git clone <repo>
cd eion
```

### 2. Start Database Services

```bash
# Start all required databases (PostgreSQL + Neo4j)
docker-compose up -d

# Verify databases are ready
docker-compose ps
```

### 3. Setup Database Extensions and Tables

```bash
# Enable the pgvector extension (required for embeddings)
docker exec eion_postgres psql -U eion -d eion -c "CREATE EXTENSION IF NOT EXISTS vector;"

# Run main orchestrator migrations (includes sessions table)
docker exec -i eion_postgres psql -U eion -d eion < database_setup.sql
```

### 4. Install Python Dependencies

```bash
# Create virtual environment
python3 -m venv .venv
source .venv/bin/activate  # On Windows: .venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt
```

### 5. Build and Run Eion Server

```bash
# Build the server
go build -o eion-server ./cmd/eion-server

# Run the server
./eion-server
```

### 6. Verify Setup

```bash
# Check server health
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","timestamp":"2024-12-19T10:30:00Z","services":{"database":"healthy","embedding":"healthy"}}
```

## Architecture

Eion provides a unified API that combines:

- **Memory Storage**: PostgreSQL with pgvector for conversation history and semantic search
- **Knowledge Graph**: Neo4j with in-house extraction for temporal knowledge storage
- **Real Embeddings**: `all-MiniLM-L6-v2` model (384 dimensions) using sentence-transformers - production-ready embeddings
- **Knowledge Extraction**: In-house extraction service for entity/relationship extraction

## Configuration

Create `eion.yaml` (optional - defaults work out of the box):

```yaml
common:
  http:
    host: "0.0.0.0"
    port: 8080
  
  postgres:
    user: "eion"
    password: "eion_pass" 
    host: "localhost"
    port: 5432
    database: "eion"
  
  # Neo4j Configuration (Required)
  numa:
    neo4j:
      uri: "bolt://localhost:7687"
      username: "neo4j"
      password: "password"
      database: "neo4j"
```

## Additional Configuration

For production deployments, you may want to customize the database settings in `docker-compose.yml` or create your own configuration.

Or use the automated setup script:

```bash
# One-command setup (includes database startup, Python env, and server build)
./setup.sh

# Then start the server
./eion-server
```
---
<div align="center">
  <img src="assets/eion-navy.png#gh-light-mode-only" alt="Eion Logo" width="50" height="50">
  <img src="assets/eion-cream.png#gh-dark-mode-only" alt="Eion Logo" width="50" height="50">
</div>