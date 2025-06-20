#!/bin/bash

# Eion Setup Script
# This script sets up the complete Eion environment following the README quickstart

set -e

echo "Setting up Eion - Shared Knowledge Graph Memory System"
echo "=================================================="

# Check prerequisites
echo "Checking prerequisites..."

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "Docker is required but not installed. Please install Docker first."
    exit 1
fi

# Check Docker Compose
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "Docker Compose is required but not installed. Please install Docker Compose first."
    exit 1
fi

# Check Go
if ! command -v go &> /dev/null; then
    echo "Go 1.21+ is required but not installed. Please install Go first."
    exit 1
fi

# Check Python
if ! command -v python3 &> /dev/null; then
    echo "Python 3.13+ is required but not installed. Please install Python first."
    exit 1
fi

echo "All prerequisites found!"

# 1. Start Database Services
echo "Starting database services..."
docker-compose up -d

# Verify databases are ready
echo "Verifying databases are ready..."
sleep 10

# Check PostgreSQL
echo "Checking PostgreSQL connection..."
until docker exec eion_postgres pg_isready -U eion -d eion > /dev/null 2>&1; do
    echo "   Waiting for PostgreSQL..."
    sleep 2
done
echo "PostgreSQL is ready"

# Check Neo4j
echo "Checking Neo4j connection..."
until docker exec eion_neo4j cypher-shell -u neo4j -p password "RETURN 1" > /dev/null 2>&1; do
    echo "   Waiting for Neo4j..."
    sleep 2
done
echo "Neo4j is ready"

# 2. Setup Database Extensions and Tables
echo "Setting up database extensions and tables..."

# Enable the pgvector extension (required for embeddings)
echo "Enabling pgvector extension..."
docker exec eion_postgres psql -U eion -d eion -c "CREATE EXTENSION IF NOT EXISTS vector;"

# Run main orchestrator migrations (includes sessions table)
echo "Running database migrations..."
if [ -f "database_setup.sql" ]; then
    docker exec -i eion_postgres psql -U eion -d eion < database_setup.sql
    echo "Database setup complete"
else
    echo "Warning: database_setup.sql not found - skipping migrations"
fi

# 3. Install Python Dependencies
echo "Setting up Python environment..."
if [ ! -d ".venv" ]; then
    python3 -m venv .venv
fi

source .venv/bin/activate

echo "Installing Python dependencies..."
pip install -r requirements.txt

# 4. Build and Run Eion Server
echo "Building Eion server..."
go build -o eion-server ./cmd/eion-server

echo ""
echo "Eion setup complete!"
echo ""
echo "To start the Eion server:"
echo "  ./eion-server"
echo ""
echo "To verify setup:"
echo "  curl http://localhost:8080/health"
echo ""
echo "Expected health response:"
echo '  {"status":"healthy","timestamp":"2024-12-19T10:30:00Z","services":{"database":"healthy","embedding":"healthy"}}'
echo ""
echo "Database URLs:"
echo "  PostgreSQL: postgresql://eion:eion_pass@localhost:5432/eion"
echo "  Neo4j:      bolt://neo4j:password@localhost:7687"
echo "  Neo4j Web:  http://localhost:7474 (neo4j/password)"
echo ""
echo "Documentation:"
echo "  README.md - Setup and architecture"
echo "  docs/SDK.md - Complete API documentation for AI agents"
echo "" 