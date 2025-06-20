#!/usr/bin/env python3
"""
Numa Extraction Service - Embedded Python service for knowledge extraction
Migrates Eion Knowledge's extraction logic EXACTLY with LLM/Numa dual client support
"""

import sys
import json
import asyncio
import logging
from typing import List, Dict, Any, Optional
from dataclasses import dataclass, asdict
from datetime import datetime
import uuid
import argparse
import os

# Add current directory to Python path for imports
current_dir = os.path.dirname(os.path.abspath(__file__))
if current_dir not in sys.path:
    sys.path.insert(0, current_dir)

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

@dataclass
class Message:
    uuid: str
    role: str
    content: str
    role_type: Optional[str] = None

@dataclass
class EpisodeData:
    uuid: str
    content: str
    timestamp: str
    source: str
    group_id: str

@dataclass
class EntityType:
    id: int
    name: str
    description: str

@dataclass
class EntityNode:
    uuid: str
    name: str
    group_id: str
    labels: List[str]
    summary: str
    created_at: str

@dataclass
class EdgeNode:
    uuid: str
    source_uuid: str
    target_uuid: str
    relation_type: str
    summary: str
    created_at: str

@dataclass
class ExtractionRequest:
    group_id: str
    messages: List[Message]
    previous_episodes: Optional[List[EpisodeData]] = None
    entity_types: Optional[List[EntityType]] = None
    use_numa: Optional[bool] = None

@dataclass
class ExtractionResponse:
    success: bool
    extracted_nodes: List[EntityNode]
    extracted_edges: List[EdgeNode] 
    error: Optional[str] = None

class ExtractionService:
    """
    Main extraction service that replicates Eion Knowledge's functionality exactly
    with dual LLM/Numa client support
    """
    
    def __init__(self):
        self.llm_client = None
        self.numa_client = None
        self._initialize_clients()
    
    def _initialize_clients(self):
        """Initialize LLM and Numa clients"""
        try:
            # Try to initialize LLM client (OpenAI)
            import sys
            import os
            sys.path.append(os.path.join(os.path.dirname(__file__), '..', '..', '..'))
            from internal.llm.python.llm_client import LLMClient
            self.llm_client = LLMClient()
            logger.info("LLM client initialized successfully")
        except Exception as e:
            logger.warning(f"Failed to initialize LLM client: {e}")
        
        try:
            # Always initialize Numa client as fallback
            from internal.numa.python.numa_client import NumaClient
            self.numa_client = NumaClient()
            logger.info("Numa client initialized successfully")
        except Exception as e:
            logger.error(f"Failed to initialize Numa client: {e}")
            if self.llm_client is None:
                raise RuntimeError("No extraction clients available")
    
    async def extract_knowledge(self, request: ExtractionRequest) -> ExtractionResponse:
        """
        Extract knowledge from messages using LLM with local fallback
        Comprehensive extraction flow for entities and relationships
        """
        try:
            # Choose extraction method based on request and client availability
            use_numa = request.use_numa or self.llm_client is None
            
            if not use_numa and self.llm_client:
                try:
                    # Primary: Use LLM client (matches Eion Knowledge's flow)
                    result = await self._extract_with_llm(request)
                    logger.info(f"Successfully extracted with LLM: {len(result.extracted_nodes)} nodes, {len(result.extracted_edges)} edges")
                    return result
                except Exception as e:
                    logger.warning(f"LLM extraction failed, falling back to Numa: {e}")
            
            # Fallback: Use Numa client
            if self.numa_client:
                result = await self._extract_with_numa(request)
                logger.info(f"Successfully extracted with Numa: {len(result.extracted_nodes)} nodes, {len(result.extracted_edges)} edges")
                return result
            
            raise RuntimeError("No extraction clients available")
            
        except Exception as e:
            logger.error(f"Extraction failed: {e}")
            return ExtractionResponse(
                success=False,
                extracted_nodes=[],
                extracted_edges=[],
                error=str(e)
            )
    
    async def _extract_with_llm(self, request: ExtractionRequest) -> ExtractionResponse:
        """Extract using LLM client - comprehensive knowledge extraction"""
        from internal.knowledge.python.knowledge_extractor import KnowledgeExtractor
        
        extractor = KnowledgeExtractor(self.llm_client)
        
        # Convert request to episode format
        episode = self._convert_to_episode(request)
        previous_episodes = self._convert_previous_episodes(request.previous_episodes or [])
        entity_types = self._convert_entity_types(request.entity_types or [])
        
        # Extract nodes using comprehensive extraction logic
        extracted_nodes = await extractor.extract_nodes(episode, previous_episodes, entity_types)
        
        # Extract edges using comprehensive extraction logic
        extracted_edges = await extractor.extract_edges(extracted_nodes, episode, previous_episodes)
        
        return ExtractionResponse(
            success=True,
            extracted_nodes=extracted_nodes,
            extracted_edges=extracted_edges
        )
    
    async def _extract_with_numa(self, request: ExtractionRequest) -> ExtractionResponse:
        """Extract using Numa client - same logic as LLM but with local processing"""
        from internal.numa.python.numa_extractor import NumaExtractor
        
        extractor = NumaExtractor(self.numa_client)
        
        # Convert request to episode format (same as LLM)
        episode = self._convert_to_episode(request)
        previous_episodes = self._convert_previous_episodes(request.previous_episodes or [])
        entity_types = self._convert_entity_types(request.entity_types or [])
        
        # Extract nodes using local extraction logic
        extracted_nodes = await extractor.extract_nodes(episode, previous_episodes, entity_types)
        
        # Extract edges using local extraction logic  
        extracted_edges = await extractor.extract_edges(extracted_nodes, episode, previous_episodes)
        
        return ExtractionResponse(
            success=True,
            extracted_nodes=extracted_nodes,
            extracted_edges=extracted_edges
        )
    
    def _convert_to_episode(self, request: ExtractionRequest):
        """Convert request messages to episode format"""
        # Combine all messages into single episode content
        content_parts = []
        for msg in request.messages:
            role_prefix = f"[{msg.role}]" if msg.role else ""
            content_parts.append(f"{role_prefix} {msg.content}".strip())
        
        from internal.knowledge.python.knowledge_models import EpisodicNode, EpisodeType
        
        return EpisodicNode(
            uuid=str(uuid.uuid4()),
            group_id=request.group_id,
            content="\n".join(content_parts),
            source=EpisodeType.message,
            source_description="DMV Test Conversation",
            created_at=datetime.now(),
            valid_at=datetime.now()
        )
    
    def _convert_previous_episodes(self, previous_episodes: List[EpisodeData]):
        """Convert previous episodes to episode format"""
        from internal.knowledge.python.knowledge_models import EpisodicNode, EpisodeType
        
        converted = []
        for ep in previous_episodes:
            converted.append(EpisodicNode(
                uuid=ep.uuid,
                group_id=ep.group_id,
                content=ep.content,
                source=EpisodeType.message,
                source_description=ep.source,
                created_at=datetime.fromisoformat(ep.timestamp) if isinstance(ep.timestamp, str) else ep.timestamp,
                valid_at=datetime.fromisoformat(ep.timestamp) if isinstance(ep.timestamp, str) else ep.timestamp
            ))
        return converted
    
    def _convert_entity_types(self, entity_types: List[EntityType]):
        """Convert entity types to knowledge graph format"""
        converted = []
        for et in entity_types:
            # Create a local entity type class for compatibility
            class LocalEntityType:
                __doc__ = et.description
            converted.append(LocalEntityType)
        return converted

async def main():
    """Main entry point for the extraction service"""
    parser = argparse.ArgumentParser(description='Numa Extraction Service')
    parser.add_argument('--test', action='store_true', help='Test service availability')
    
    # Add support for Go command-line interface
    parser.add_argument('command', nargs='?', help='Command: add_episode, search, get_episodes')
    parser.add_argument('args', nargs='*', help='Command arguments')
    
    args = parser.parse_args()
    
    if args.test:
        # Test mode - just verify service can initialize
        try:
            service = ExtractionService()
            print(json.dumps({"status": "ok", "message": "Service initialized successfully"}))
            return 0
        except Exception as e:
            print(json.dumps({"status": "error", "message": str(e)}))
            return 1
    
    # Handle Go command-line interface
    if args.command:
        try:
            service = ExtractionService()
            
            if args.command == "add_episode":
                return await handle_add_episode(service, args.args)
            elif args.command == "search":
                return await handle_search(service, args.args)
            elif args.command == "get_episodes":
                return await handle_get_episodes(service, args.args)
            else:
                print(json.dumps({"error": f"Unknown command: {args.command}"}))
                return 1
                
        except Exception as e:
            logger.error(f"Command error: {e}")
            print(json.dumps({"error": str(e)}))
            return 1
    
    # Production mode - read from stdin, process, write to stdout
    try:
        # Read request from stdin
        request_data = json.loads(sys.stdin.read())
        
        # Parse request
        messages = [Message(**msg) for msg in request_data.get('messages', [])]
        previous_episodes = [EpisodeData(**ep) for ep in request_data.get('previous_episodes', [])]
        entity_types = [EntityType(**et) for et in request_data.get('entity_types', [])]
        
        request = ExtractionRequest(
            group_id=request_data['group_id'],
            messages=messages,
            previous_episodes=previous_episodes,
            entity_types=entity_types,
            use_numa=request_data.get('use_numa', False)
        )
        
        # Process extraction
        service = ExtractionService()
        response = await service.extract_knowledge(request)
        
        # Convert response to dict and output as JSON
        response_dict = asdict(response)
        print(json.dumps(response_dict))
        return 0
        
    except Exception as e:
        logger.error(f"Service error: {e}")
        error_response = ExtractionResponse(
            success=False,
            extracted_nodes=[],
            extracted_edges=[],
            error=str(e)
        )
        print(json.dumps(asdict(error_response)))
        return 1

async def handle_add_episode(service: ExtractionService, args: List[str]) -> int:
    """Handle add_episode command: add_episode <name> <content> <sourceDescription> [groupID] [episodeType]"""
    try:
        if len(args) < 3:
            print(json.dumps({"error": "add_episode requires at least 3 arguments: name, content, sourceDescription"}))
            return 1
        
        name = args[0]
        content = args[1]
        source_description = args[2]
        group_id = args[3] if len(args) > 3 and args[3] else str(uuid.uuid4())
        episode_type = args[4] if len(args) > 4 and args[4] else "conversation"
        
        # Create a message from the episode content
        message = Message(
            uuid=str(uuid.uuid4()),
            role="system",
            content=content,
            role_type="system"
        )
        
        # Create extraction request
        request = ExtractionRequest(
            group_id=group_id,
            messages=[message],
            previous_episodes=[],
            entity_types=[],
            use_numa=True  # Use local processing for now
        )
        
        # Extract knowledge
        response = await service.extract_knowledge(request)
        
        # Return result in format expected by Go
        result = {
            "episode_uuid": str(uuid.uuid4()),
            "episode_name": name,
            "nodes_created": len(response.extracted_nodes),
            "edges_created": len(response.extracted_edges),
            "node_uuids": [node.uuid for node in response.extracted_nodes],
            "edge_uuids": [edge.uuid for edge in response.extracted_edges]
        }
        
        print(json.dumps(result))
        return 0
        
    except Exception as e:
        logger.error(f"add_episode error: {e}")
        print(json.dumps({"error": str(e)}))
        return 1

async def handle_search(service: ExtractionService, args: List[str]) -> int:
    """Handle search command: search <query> [groupIDs] [numResults]"""
    try:
        if len(args) < 1:
            print(json.dumps({"error": "search requires at least 1 argument: query"}))
            return 1
        
        query = args[0]
        group_ids = args[1].split(",") if len(args) > 1 and args[1] else []
        num_results = int(args[2]) if len(args) > 2 and args[2] else 10
        
        # REAL IMPLEMENTATION: Search the knowledge graph using existing infrastructure
        try:
            import psycopg2
            import json
            
            # Get database connection details from environment or config
            db_config = {
                'host': os.getenv('DB_HOST', 'localhost'),
                'port': os.getenv('DB_PORT', '5432'),
                'database': os.getenv('DB_NAME', 'eion'),
                'user': os.getenv('DB_USER', 'postgres'),
                'password': os.getenv('DB_PASSWORD', 'password')
            }
            
            conn = psycopg2.connect(**db_config)
            cursor = conn.cursor()
            
            # Search across sessions for relevant memory/knowledge
            search_query = """
                SELECT s.id, s.session_name, s.metadata, s.created_at, s.updated_at
                FROM sessions s
                WHERE s.metadata::text ILIKE %s
                ORDER BY s.updated_at DESC
                LIMIT %s
            """
            
            search_term = f'%{query}%'
            cursor.execute(search_query, (search_term, num_results))
            rows = cursor.fetchall()
            
            results = []
            for row in rows:
                session_id, session_name, metadata, created_at, updated_at = row
                
                # Parse metadata for knowledge content
                session_metadata = metadata if metadata else {}
                
                results.append({
                    "uuid": str(session_id),
                    "name": session_name or f"Session {session_id}",
                    "fact": f"Knowledge from session: {session_name or session_id}",
                    "source_node_uuid": str(session_id),
                    "target_node_uuid": str(session_id),
                    "created_at": created_at.isoformat() if created_at else datetime.now().isoformat(),
                    "valid_at": updated_at.isoformat() if updated_at else datetime.now().isoformat(),
                    "episodes": [],
                    "metadata": session_metadata
                })
            
            cursor.close()
            conn.close()
            
            result = {
                "results": results,
                "count": len(results)
            }
            
        except Exception as e:
            logger.error(f"Database search failed: {e}")
            # Fallback to empty results rather than mock data
            result = {
                "results": [],
                "count": 0,
                "error": f"Search failed: {str(e)}"
            }
        
        print(json.dumps(result))
        return 0
        
    except Exception as e:
        logger.error(f"search error: {e}")
        print(json.dumps({"error": str(e)}))
        return 1

async def handle_get_episodes(service: ExtractionService, args: List[str]) -> int:
    """Handle get_episodes command: get_episodes [groupIDs] [lastN]"""
    try:
        group_ids = args[0].split(",") if len(args) > 0 and args[0] else []
        last_n = int(args[1]) if len(args) > 1 and args[1] else 10
        
        # REAL IMPLEMENTATION: Retrieve episodes from the database
        try:
            import psycopg2
            import json
            
            # Get database connection details from environment or config
            db_config = {
                'host': os.getenv('DB_HOST', 'localhost'),
                'port': os.getenv('DB_PORT', '5432'),
                'database': os.getenv('DB_NAME', 'eion'),
                'user': os.getenv('DB_USER', 'postgres'),
                'password': os.getenv('DB_PASSWORD', 'password')
            }
            
            conn = psycopg2.connect(**db_config)
            cursor = conn.cursor()
            
            # Query to get recent episodes (sessions) 
            if group_ids:
                # Filter by group IDs if provided
                episodes_query = """
                    SELECT s.id, s.session_name, s.metadata, s.created_at, s.updated_at, s.user_id
                    FROM sessions s
                    WHERE s.user_id = ANY(%s)
                    ORDER BY s.updated_at DESC
                    LIMIT %s
                """
                cursor.execute(episodes_query, (group_ids, last_n))
            else:
                # Get all recent episodes
                episodes_query = """
                    SELECT s.id, s.session_name, s.metadata, s.created_at, s.updated_at, s.user_id
                    FROM sessions s
                    ORDER BY s.updated_at DESC
                    LIMIT %s
                """
                cursor.execute(episodes_query, (last_n,))
            
            rows = cursor.fetchall()
            
            episodes = []
            for row in rows:
                session_id, session_name, metadata, created_at, updated_at, user_id = row
                
                # Parse metadata for episode content
                session_metadata = metadata if metadata else {}
                content = session_metadata.get('content', f'Session {session_id} content')
                
                episodes.append({
                    "uuid": str(session_id),
                    "name": session_name or f"Session {session_id}",
                    "content": content,
                    "source": "database",
                    "source_description": f"Session {session_name or session_id}",
                    "created_at": created_at.isoformat() if created_at else datetime.now().isoformat(),
                    "valid_at": updated_at.isoformat() if updated_at else datetime.now().isoformat(),
                    "group_id": str(user_id) if user_id else "default",
                    "entity_edges": [],
                    "metadata": session_metadata
                })
            
            cursor.close()
            conn.close()
            
            result = {
                "episodes": episodes,
                "count": len(episodes)
            }
            
        except Exception as e:
            logger.error(f"Database episode retrieval failed: {e}")
            # Fallback to empty results rather than mock data
            result = {
                "episodes": [],
                "count": 0,
                "error": f"Episode retrieval failed: {str(e)}"
            }
        
        print(json.dumps(result))
        return 0
        
    except Exception as e:
        logger.error(f"get_episodes error: {e}")
        print(json.dumps({"error": str(e)}))
        return 1

if __name__ == "__main__":
    sys.exit(asyncio.run(main())) 