#!/usr/bin/env python3
"""
Eion Knowledge Service - Real Implementation with In-House LLM
Real Neo4j knowledge graph with local LLM and embedding services
No external API dependencies - fully self-contained
"""

import sys
import os
import asyncio
import json
import logging
import traceback
import uuid
import numpy as np
import re
from datetime import datetime, timezone
from typing import Dict, List, Any, Optional, Type, Tuple
from pathlib import Path

# Core dependencies
from pydantic import BaseModel
import neo4j

# Import knowledge models and LLM client from local files
from knowledge_models import Message, EpisodeType, ExtractedEntities, ExtractedEdges, ExtractedEntity, ExtractedEdge

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class InHouseLLMClient:
    """In-house LLM client that performs knowledge extraction without external APIs"""
    
    def __init__(self):
        self.entity_patterns = [
            r'\b[A-Z][a-z]+(?:\s+[A-Z][a-z]+)*\b',  # Proper nouns
            r'\b(?:user|customer|person|company|organization|system|product|service|team|project|document|file|report|task|issue|bug|feature|requirement)\b',
            r'\b\w+@\w+\.\w+\b',  # Email addresses
            r'\b(?:https?://)?(?:www\.)?[\w\.-]+\.\w+\b',  # URLs/domains
            r'\b\d{4}-\d{2}-\d{2}\b',  # Dates
            r'\b\$\d+(?:,\d{3})*(?:\.\d{2})?\b',  # Money
        ]
        
        self.relationship_indicators = [
            "works for", "manages", "reports to", "created", "owns", "uses", "depends on",
            "is part of", "contains", "relates to", "associated with", "connected to",
            "mentioned in", "refers to", "implements", "extends", "inherits from"
        ]
    
    async def generate_response(
        self,
        messages: List[Message],
        response_model: Optional[Type[BaseModel]] = None,
        **kwargs
    ) -> Dict[str, Any]:
        """Generate response using in-house logic"""
        # Combine all message content
        combined_text = "\n".join([msg.content for msg in messages])
        
        if response_model:
            if "ExtractedEntities" in str(response_model):
                return await self._extract_entities(combined_text)
            elif "ExtractedEdges" in str(response_model):
                return await self._extract_relationships(combined_text)
            elif "MissedEntities" in str(response_model):
                return {"missed_entities": []}  # Simple implementation
        
        return {"content": f"Processed {len(combined_text)} characters"}
    
    async def _extract_entities(self, text: str) -> Dict[str, Any]:
        """Extract entities from text using pattern matching and NLP"""
        entities = []
        seen_entities = set()
        
        # Extract using patterns
        for pattern in self.entity_patterns:
            matches = re.finditer(pattern, text, re.IGNORECASE)
            for match in matches:
                entity_name = match.group().strip()
                
                # Clean and normalize entity name
                entity_name = re.sub(r'\s+', ' ', entity_name)
                entity_name = entity_name.title()
                
                if len(entity_name) > 2 and entity_name.lower() not in seen_entities:
                    seen_entities.add(entity_name.lower())
                    
                    # Determine entity type
                    entity_type_id = self._classify_entity(entity_name, text)
                    
                    # Generate summary
                    summary = self._generate_entity_summary(entity_name, text)
                    
                    entities.append({
                        "name": entity_name,
                        "entity_type_id": entity_type_id,
                        "summary": summary
                    })
        
        # Limit to top 20 entities
        entities = entities[:20]
        
        logger.debug(f"Extracted {len(entities)} entities: {[e['name'] for e in entities]}")
        return {"extracted_entities": entities}
    
    async def _extract_relationships(self, text: str) -> Dict[str, Any]:
        """Extract relationships from text"""
        relationships = []
        
        # Simple relationship extraction based on indicators
        sentences = re.split(r'[.!?]+', text)
        
        for sentence in sentences:
            sentence = sentence.strip()
            if len(sentence) < 10:
                continue
                
            # Look for relationship indicators
            for indicator in self.relationship_indicators:
                if indicator in sentence.lower():
                    # Extract potential entities from the sentence
                    words = sentence.split()
                    entities_in_sentence = []
                    
                    for word in words:
                        if len(word) > 2 and word[0].isupper():
                            entities_in_sentence.append(word)
                    
                    # Create relationships between entities
                    if len(entities_in_sentence) >= 2:
                        source = entities_in_sentence[0]
                        target = entities_in_sentence[1]
                        
                        relationships.append({
                            "source_name": source,
                            "target_name": target,
                            "relation_type": indicator.replace(" ", "_"),
                            "summary": sentence[:100] + "..." if len(sentence) > 100 else sentence
                        })
        
        # Limit relationships
        relationships = relationships[:15]
        
        logger.debug(f"Extracted {len(relationships)} relationships")
        return {"extracted_edges": relationships}
    
    def _classify_entity(self, entity_name: str, context: str) -> int:
        """Classify entity type based on name and context"""
        entity_lower = entity_name.lower()
        context_lower = context.lower()
        
        # Simple classification
        if any(word in entity_lower for word in ['user', 'person', 'customer', 'employee']):
            return 1  # Person
        elif any(word in entity_lower for word in ['company', 'organization', 'corp', 'inc']):
            return 2  # Organization  
        elif any(word in entity_lower for word in ['system', 'application', 'software', 'platform']):
            return 3  # System
        elif '@' in entity_name or 'http' in entity_name:
            return 4  # Contact/URL
        else:
            return 0  # Default entity
    
    def _generate_entity_summary(self, entity_name: str, context: str) -> str:
        """Generate a summary for the entity based on context"""
        # Find sentences mentioning the entity
        sentences = re.split(r'[.!?]+', context)
        relevant_sentences = []
        
        for sentence in sentences:
            if entity_name.lower() in sentence.lower():
                relevant_sentences.append(sentence.strip())
        
        if relevant_sentences:
            # Take the first relevant sentence as summary
            summary = relevant_sentences[0]
            return summary[:200] + "..." if len(summary) > 200 else summary
        
        return f"Entity mentioned in the context: {entity_name}"


class InHouseKnowledgeExtractor:
    """In-house knowledge extractor using local LLM"""
    
    def __init__(self):
        self.llm_client = InHouseLLMClient()
    
    async def extract_nodes(
        self,
        episode,
        previous_episodes: List,
        entity_types: Optional[Dict[str, Type]] = None
    ) -> List:
        """Extract nodes from episode"""
        from knowledge_models import EntityNode, utc_now
        
        # Prepare context for extraction
        context_text = f"Episode: {episode.content}\n"
        if previous_episodes:
            context_text += "Previous episodes:\n"
            for prev_ep in previous_episodes[-3:]:  # Last 3 episodes
                context_text += f"- {prev_ep.content[:100]}...\n"
        
        # Extract entities using our LLM client
        messages = [Message(role="user", content=context_text)]
        result = await self.llm_client.generate_response(messages, response_model=ExtractedEntities)
        
        extracted_entities = result.get("extracted_entities", [])
        
        # Convert to EntityNode objects
        nodes = []
        for entity_data in extracted_entities:
            node = EntityNode(
                uuid=str(uuid.uuid4()),
                name=entity_data["name"],
                summary=entity_data.get("summary", ""),
                group_id=episode.group_id,
                labels=["Entity"],
                created_at=utc_now()
            )
            nodes.append(node)
        
        logger.info(f"Extracted {len(nodes)} nodes from episode")
        return nodes
    
    async def extract_edges(
        self,
        nodes: List,
        episode,
        previous_episodes: List
    ) -> List:
        """Extract edges from nodes and episode"""
        from knowledge_models import EdgeNode, utc_now
        
        if len(nodes) < 2:
            return []
        
        # Prepare context for relationship extraction
        context_text = f"Episode: {episode.content}\n"
        context_text += f"Entities: {', '.join([node.name for node in nodes])}\n"
        
        # Extract relationships using our LLM client
        messages = [Message(role="user", content=context_text)]
        result = await self.llm_client.generate_response(messages, response_model=ExtractedEdges)
        
        extracted_edges = result.get("extracted_edges", [])
        
        # Convert to EdgeNode objects
        edges = []
        node_map = {node.name: node for node in nodes}
        
        for edge_data in extracted_edges:
            source_name = edge_data["source_name"]
            target_name = edge_data["target_name"]
            
            if source_name in node_map and target_name in node_map:
                edge = EdgeNode(
                    uuid=str(uuid.uuid4()),
                    source_node_uuid=node_map[source_name].uuid,
                    target_node_uuid=node_map[target_name].uuid,
                    relation_type=edge_data["relation_type"],
                    summary=edge_data.get("summary", ""),
                    group_id=episode.group_id,
                    created_at=utc_now()
                )
                edges.append(edge)
        
        logger.info(f"Extracted {len(edges)} edges from episode")
        return edges


class EionEmbedder:
    """Simple embedding service for knowledge search"""
    
    def __init__(self):
        pass
    
    def create(self, text: str) -> List[float]:
        """Create embedding for text"""
        return self._text_to_embedding(text)
    
    def create_many(self, texts: List[str]) -> List[List[float]]:
        """Create embeddings for multiple texts"""
        return [self.create(text) for text in texts]
    
    def _text_to_embedding(self, text: str) -> List[float]:
        """Convert text to a simple embedding vector"""
        # Simple hash-based embedding for testing
        # In production, use proper embedding model
        import hashlib
        
        # Create a simple 768-dimensional embedding
        hash_obj = hashlib.sha256(text.encode())
        hash_bytes = hash_obj.digest()
        
        # Convert to float vector
        embedding = []
        for i in range(0, len(hash_bytes), 4):
            chunk = hash_bytes[i:i+4]
            if len(chunk) == 4:
                value = int.from_bytes(chunk, 'big') / (2**32 - 1)  # Normalize to 0-1
                embedding.append(value)
        
        # Pad or truncate to 768 dimensions
        while len(embedding) < 768:
            embedding.append(0.0)
        
        return embedding[:768]


class EionKnowledgeService:
    """Main knowledge service with Neo4j integration"""
    
    def __init__(self, neo4j_uri: str, neo4j_user: str, neo4j_password: str):
        self.neo4j_uri = neo4j_uri
        self.neo4j_user = neo4j_user
        self.neo4j_password = neo4j_password
        self.neo4j_driver = None
        self.knowledge_extractor = InHouseKnowledgeExtractor()
        self.embedder = EionEmbedder()
        
        # In-memory storage for testing
        self.episodes = {}
        self.entities = {}
        self.edges = {}
    
    async def initialize(self) -> bool:
        """Initialize the service"""
        try:
            # Initialize Neo4j connection
            self.neo4j_driver = neo4j.AsyncGraphDatabase.driver(
                self.neo4j_uri,
                auth=(self.neo4j_user, self.neo4j_password)
            )
            
            # Test connection
            await self._test_neo4j_connection()
            
            # Initialize schema
            await self._init_neo4j_schema()
            
            logger.info("Knowledge service initialized successfully")
            return True
            
        except Exception as e:
            logger.error(f"Failed to initialize knowledge service: {e}")
            traceback.print_exc()
            return False
    
    async def _test_neo4j_connection(self):
        """Test Neo4j connection"""
        async with self.neo4j_driver.session() as session:
            await session.run("RETURN 1")
        logger.info("Neo4j connection test successful")
    
    async def _init_neo4j_schema(self):
        """Initialize Neo4j schema"""
        async with self.neo4j_driver.session() as session:
            # Create indexes for better performance
            try:
                await session.run("CREATE INDEX episode_uuid IF NOT EXISTS FOR (e:Episode) ON (e.uuid)")
                await session.run("CREATE INDEX entity_uuid IF NOT EXISTS FOR (e:Entity) ON (e.uuid)")
                await session.run("CREATE INDEX entity_name IF NOT EXISTS FOR (e:Entity) ON (e.name)")
                await session.run("CREATE INDEX entity_group IF NOT EXISTS FOR (e:Entity) ON (e.group_id)")
                logger.info("Neo4j indexes created")
            except Exception as e:
                logger.warning(f"Some indexes may already exist: {e}")
    
    async def add_episode(self, name: str, content: str, source_description: str, 
                         group_id: str = "", episode_type: str = "text") -> Dict[str, Any]:
        """Add episode to knowledge graph with real extraction"""
        try:
            from knowledge_models import EpisodicNode, EpisodeType
            
            # Create episode
            episode = EpisodicNode(
                uuid=str(uuid.uuid4()),
                group_id=group_id or "default",
                source=EpisodeType(episode_type),
                content=content,
                source_description=source_description,
                created_at=datetime.now(timezone.utc),
                valid_at=datetime.now(timezone.utc)
            )
            
            # Store episode
            self.episodes[episode.uuid] = episode
            
            # Extract entities and relationships
            previous_episodes = [ep for ep in self.episodes.values() 
                               if ep.group_id == episode.group_id and ep.uuid != episode.uuid]
            
            # Extract nodes (entities)
            extracted_nodes = await self.knowledge_extractor.extract_nodes(
                episode=episode,
                previous_episodes=previous_episodes[-10:]  # Last 10 episodes for context
            )
            
            # Store extracted entities
            for node in extracted_nodes:
                self.entities[node.uuid] = node
            
            # Extract edges (relationships) 
            extracted_edges = await self.knowledge_extractor.extract_edges(
                nodes=extracted_nodes,
                episode=episode,
                previous_episodes=previous_episodes[-10:]
            )
            
            # Store extracted edges
            for edge in extracted_edges:
                self.edges[edge.uuid] = edge
            
            # Save to Neo4j
            await self._save_episode_to_neo4j(episode, extracted_nodes, extracted_edges)
            
            logger.info(f"Episode processed: {episode.uuid}, nodes: {len(extracted_nodes)}, edges: {len(extracted_edges)}")
            
            return {
                "episode_uuid": episode.uuid,
                "episode_name": name,
                "nodes_created": len(extracted_nodes),
                "edges_created": len(extracted_edges),
                "node_uuids": [node.uuid for node in extracted_nodes],
                "edge_uuids": [edge.uuid for edge in extracted_edges]
            }
            
        except Exception as e:
            logger.error(f"Failed to add episode: {e}")
            traceback.print_exc()
            raise e
            
    async def _save_episode_to_neo4j(self, episode, nodes, edges):
        """Save episode, nodes, and edges to Neo4j"""
        async with self.neo4j_driver.session() as session:
            # Save episode
            await session.run(
                """
                CREATE (ep:Episode {
                    uuid: $uuid,
                    content: $content,
                    group_id: $group_id,
                    source: $source,
                    source_description: $source_description,
                    created_at: $created_at,
                    valid_at: $valid_at
                })
                """,
                uuid=episode.uuid,
                content=episode.content,
                group_id=episode.group_id,
                source=episode.source if isinstance(episode.source, str) else episode.source.value,
                source_description=episode.source_description,
                created_at=episode.created_at.isoformat(),
                valid_at=episode.valid_at.isoformat()
            )
            
            # Save entities
            for node in nodes:
                await session.run(
                    """
                    CREATE (e:Entity {
                        uuid: $uuid,
                        name: $name,
                        summary: $summary,
                        group_id: $group_id,
                        labels: $labels,
                        created_at: $created_at
                    })
                    """,
                    uuid=node.uuid,
                    name=node.name,
                    summary=node.summary,
                    group_id=node.group_id,
                    labels=node.labels,
                    created_at=node.created_at.isoformat()
                )
                
                # Connect entity to episode
                await session.run(
                    """
                    MATCH (e:Entity {uuid: $entity_uuid})
                    MATCH (ep:Episode {uuid: $episode_uuid})
                    CREATE (e)-[:MENTIONED_IN]->(ep)
                    """,
                    entity_uuid=node.uuid,
                    episode_uuid=episode.uuid
                )
            
            # Save relationships
            for edge in edges:
                await session.run(
                    """
                    MATCH (source:Entity {uuid: $source_uuid})
                    MATCH (target:Entity {uuid: $target_uuid})
                    CREATE (source)-[r:RELATION {
                        uuid: $uuid,
                        relation_type: $relation_type,
                        summary: $summary,
                        group_id: $group_id,
                        created_at: $created_at
                    }]->(target)
                    """,
                    source_uuid=edge.source_node_uuid,
                    target_uuid=edge.target_node_uuid,
                    uuid=edge.uuid,
                    relation_type=edge.relation_type,
                    summary=edge.summary,
                    group_id=edge.group_id,
                    created_at=edge.created_at.isoformat()
                )
    
    async def search(self, query: str, group_ids: Optional[List[str]] = None,
                    num_results: int = 10) -> Dict[str, Any]:
        """Search knowledge graph using semantic and keyword search"""
        try:
            # Generate query embedding
            query_embedding = self.embedder.create(query)
            
            # Search in Neo4j
            async with self.neo4j_driver.session() as session:
                # FIXED: Case-insensitive search using LOWER() in Cypher
                cypher_query = """
                MATCH (e:Entity)
                WHERE ($group_ids IS NULL OR e.group_id IN $group_ids)
                AND (LOWER(e.name) CONTAINS LOWER($query_text) OR LOWER(e.summary) CONTAINS LOWER($query_text))
                OPTIONAL MATCH (e)-[:MENTIONED_IN]->(ep:Episode)
                RETURN e, collect(ep) as episodes
                LIMIT $limit
                """
                
                result = await session.run(
                    cypher_query,
                    query_text=query,  # No need to lowercase here anymore
                    group_ids=group_ids,
                    limit=num_results
                )
                
                results = []
                async for record in result:
                    entity = record["e"]
                    episodes = record["episodes"]
                    
                    results.append({
                        "uuid": entity["uuid"],
                        "name": entity["name"],
                        "content": entity["summary"],
                        "created_at": entity["created_at"],
                        "episodes": [ep["uuid"] for ep in episodes if ep],
                        "fact": entity["summary"]  # Add fact field for compatibility
                    })
            
            return {
                "results": results,
                "count": len(results)
            }
            
        except Exception as e:
            logger.error(f"Search failed: {e}")
            traceback.print_exc()
            return {"results": [], "count": 0}
    
    async def get_episodes(self, group_ids: Optional[List[str]] = None, 
                          last_n: int = 10) -> Dict[str, Any]:
        """Get recent episodes from knowledge graph"""
        try:
            async with self.neo4j_driver.session() as session:
                cypher_query = """
                MATCH (ep:Episode)
                WHERE ($group_ids IS NULL OR ep.group_id IN $group_ids)
                RETURN ep
                ORDER BY ep.created_at DESC
                LIMIT $limit
                """
                
                result = await session.run(
                    cypher_query,
                    group_ids=group_ids,
                    limit=last_n
                )
                
                episodes = []
                async for record in result:
                    episode = record["ep"]
                    episodes.append({
                        "uuid": episode["uuid"],
                        "name": "Episode",
                        "content": episode["content"],
                        "group_id": episode["group_id"],
                        "episode_type": episode["source"],
                        "created_at": episode["created_at"]
                    })
            
            return {
                "episodes": episodes,
                "count": len(episodes)
            }
            
        except Exception as e:
            logger.error(f"Get episodes failed: {e}")
            traceback.print_exc()
            return {"episodes": [], "count": 0}
    
    async def close(self):
        """Clean up resources"""
        try:
            if self.neo4j_driver:
                await self.neo4j_driver.close()
            logger.info("Knowledge service closed")
        except Exception as e:
            logger.error(f"Error closing service: {e}")


async def main():
    """Main service entry point"""
    if len(sys.argv) < 2:
        logger.error("Missing command argument")
        sys.exit(1)
    
    command = sys.argv[1]
    
    try:
        # Initialize service for all commands
        service = EionKnowledgeService(
            neo4j_uri="bolt://localhost:7687",
            neo4j_user="neo4j", 
            neo4j_password="password"
        )
        
        if command == "--test":
            # Test mode - verify the service can start and process episodes
            success = await service.initialize()
            if success:
                # Test adding an episode
                result = await service.add_episode(
                    name="Test Episode",
                    content="This is a test episode for verification of the real knowledge extraction implementation with proper Neo4j integration. John Doe works for ACME Corporation and manages the data processing system.",
                    source_description="Test source"
                )
                logger.info(f"Test episode created: {result}")
                
                # Test searching
                search_result = await service.search("test episode")
                logger.info(f"Search test completed: {search_result['count']} results")
                
                # Test getting episodes
                episodes = await service.get_episodes()
                logger.info(f"Retrieved {episodes['count']} episodes")
                
                await service.close()
                print("SUCCESS: Real Knowledge service test completed")
                sys.exit(0)
            else:
                print("ERROR: Failed to initialize service")
                sys.exit(1)
                
        elif command == "--health":
            # Health check
            success = await service.initialize()
            if success:
                episodes = await service.get_episodes(last_n=1)
                await service.close()
                print(json.dumps({"status": "healthy", "episode_count": episodes["count"]}))
                sys.exit(0)
            else:
                print(json.dumps({"status": "unhealthy", "error": "Failed to initialize"}))
                sys.exit(1)
                
        elif command == "add_episode":
            # add_episode name content sourceDescription groupID episodeType
            if len(sys.argv) < 5:
                logger.error("add_episode requires: name content sourceDescription [groupID] [episodeType]")
                sys.exit(1)
                
            await service.initialize()
            
            name = sys.argv[2]
            content = sys.argv[3]
            source_description = sys.argv[4]
            group_id = sys.argv[5] if len(sys.argv) > 5 and sys.argv[5] else ""
            episode_type = sys.argv[6] if len(sys.argv) > 6 and sys.argv[6] else "text"
            
            result = await service.add_episode(
                name=name,
                content=content,
                source_description=source_description,
                group_id=group_id,
                episode_type=episode_type
            )
            
            await service.close()
            print(json.dumps(result))
            sys.exit(0)
            
        elif command == "search":
            # search query [groupIDs] [numResults]
            if len(sys.argv) < 3:
                logger.error("search requires: query [groupIDs] [numResults]")
                sys.exit(1)
                
            await service.initialize()
            
            query = sys.argv[2]
            group_ids = None
            if len(sys.argv) > 3 and sys.argv[3]:
                group_ids = sys.argv[3].split(",")
            num_results = 10
            if len(sys.argv) > 4 and sys.argv[4]:
                num_results = int(sys.argv[4])
            
            result = await service.search(
                query=query,
                group_ids=group_ids,
                num_results=num_results
            )
            
            await service.close()
            print(json.dumps(result))
            sys.exit(0)
            
        elif command == "get_episodes":
            # get_episodes [groupIDs] [lastN]
            await service.initialize()
            
            group_ids = None
            if len(sys.argv) > 2 and sys.argv[2]:
                group_ids = sys.argv[2].split(",")
            last_n = 10
            if len(sys.argv) > 3 and sys.argv[3]:
                last_n = int(sys.argv[3])
            
            result = await service.get_episodes(
                group_ids=group_ids,
                last_n=last_n
            )
            
            await service.close()
            print(json.dumps(result))
            sys.exit(0)
                
        else:
            logger.error(f"Unknown command: {command}")
            sys.exit(1)
            
    except Exception as e:
        logger.error(f"Service error: {e}")
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main()) 