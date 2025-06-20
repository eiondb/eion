"""
Numa Extractor - Local extraction using Numa module
Simplified extraction logic for local processing without external dependencies
"""

import asyncio
import logging
from time import time
from typing import List, Dict, Any, Optional, Type
from dataclasses import asdict

import sys
import os
sys.path.append(os.path.join(os.path.dirname(__file__), '..', '..', '..'))
from internal.knowledge.python.knowledge_models import (
    EpisodicNode, EntityNode, EdgeNode, ExtractedEntity, ExtractedEntities,
    ExtractedEdge, ExtractedEdges, MissedEntities, DuplicateEntities,
    Message, utc_now
)
from internal.numa.python.numa_client import NumaClient

logger = logging.getLogger(__name__)

# Constants for extraction iterations and concurrency
MAX_REFLEXION_ITERATIONS = 3
SEMAPHORE_LIMIT = 5


class NumaExtractor:
    """
    Local Extractor - uses comprehensive extraction logic with local client
    Drop-in replacement providing same interface and extraction flow
    """
    
    def __init__(self, numa_client: NumaClient):
        self.numa_client = numa_client
        self.semaphore = asyncio.Semaphore(SEMAPHORE_LIMIT)
    
    async def extract_nodes(
        self,
        episode: EpisodicNode,
        previous_episodes: List[EpisodicNode],
        entity_types: Optional[Dict[str, Type]] = None
    ) -> List[EntityNode]:
        """
        Extract nodes - comprehensive entity extraction using local processing
        """
        start = time()
        llm_response = {}
        custom_prompt = ''
        entities_missed = True
        reflexion_iterations = 0

        entity_types_context = [
            {
                'entity_type_id': 0,
                'entity_type_name': 'Entity',
                'entity_type_description': 'Default entity classification. Use this entity type if the entity is not one of the other listed types.',
            }
        ]

        # Fix: entity_types might be a list or dict, handle both cases
        if entity_types is not None:
            if isinstance(entity_types, dict):
                # Dict case - original code
                entity_types_context += [
                    {
                        'entity_type_id': i + 1,
                        'entity_type_name': type_name,
                        'entity_type_description': type_model.__doc__,
                    }
                    for i, (type_name, type_model) in enumerate(entity_types.items())
                ]
            elif isinstance(entity_types, list):
                # List case - handle list of entity types
                entity_types_context += [
                    {
                        'entity_type_id': i + 1,
                        'entity_type_name': str(et),
                        'entity_type_description': f"Entity type: {et}",
                    }
                    for i, et in enumerate(entity_types)
                ]

        context = {
            'episode_content': episode.content,
            'episode_timestamp': episode.valid_at.isoformat(),
            'previous_episodes': [ep.content for ep in previous_episodes],
            'custom_prompt': custom_prompt,
            'entity_types': entity_types_context,
            'source_description': episode.source_description,
        }

        while entities_missed and reflexion_iterations <= MAX_REFLEXION_ITERATIONS:
            if episode.source == "message":
                llm_response = await self.numa_client.generate_response(
                    self._create_extract_message_prompt(context),
                    response_model=ExtractedEntities,
                )
            elif episode.source == "text":
                llm_response = await self.numa_client.generate_response(
                    self._create_extract_text_prompt(context), 
                    response_model=ExtractedEntities
                )
            elif episode.source == "json":
                llm_response = await self.numa_client.generate_response(
                    self._create_extract_json_prompt(context), 
                    response_model=ExtractedEntities
                )

            extracted_entities: List[ExtractedEntity] = [
                ExtractedEntity(**entity_data)
                for entity_data in llm_response.get('extracted_entities', [])
            ]

            reflexion_iterations += 1
            if reflexion_iterations < MAX_REFLEXION_ITERATIONS:
                missing_entities = await self._extract_nodes_reflexion(
                    episode,
                    previous_episodes,
                    [entity.name for entity in extracted_entities],
                )

                entities_missed = len(missing_entities) != 0

                custom_prompt = 'Make sure that the following entities are extracted: '
                for entity in missing_entities:
                    custom_prompt += f'\n{entity},'

        filtered_extracted_entities = [entity for entity in extracted_entities if entity.name.strip()]
        end = time()
        logger.debug(f'Extracted new nodes with local extractor: {filtered_extracted_entities} in {(end - start) * 1000} ms')
        
        # Convert the extracted data into EntityNode objects
        extracted_nodes = []
        for extracted_entity in filtered_extracted_entities:
            # Fix: entity_types_context is a list, not a dict
            if 0 <= extracted_entity.entity_type_id < len(entity_types_context):
                entity_type_info = entity_types_context[extracted_entity.entity_type_id]
                entity_type_name = entity_type_info.get('entity_type_name', 'Entity')
            else:
                entity_type_name = 'Entity'

            labels: List[str] = list({'Entity', str(entity_type_name)})

            new_node = EntityNode(
                uuid=f"{episode.group_id}-{len(extracted_nodes)}",  # Simple UUID generation
                name=extracted_entity.name,
                group_id=episode.group_id,
                labels=labels,
                summary=extracted_entity.summary or '',
                created_at=utc_now(),
            )
            extracted_nodes.append(new_node)
            logger.debug(f'Created new node with local extractor: {new_node.name} (UUID: {new_node.uuid})')

        logger.debug(f'Extracted nodes with local extractor: {[(n.name, n.uuid) for n in extracted_nodes]}')
        return extracted_nodes
    
    async def _extract_nodes_reflexion(
        self,
        episode: EpisodicNode,
        previous_episodes: List[EpisodicNode],
        node_names: List[str],
    ) -> List[str]:
        """
        Extract nodes reflexion - EXACTLY same as Eion Knowledge but using Numa
        """
        # Prepare context for Numa
        context = {
            'episode_content': episode.content,
            'previous_episodes': [ep.content for ep in previous_episodes],
            'extracted_entities': node_names,
        }

        llm_response = await self.numa_client.generate_response(
            self._create_reflexion_prompt(context), 
            response_model=MissedEntities
        )
        missed_entities = llm_response.get('missed_entities', [])

        return missed_entities
    
    async def extract_edges(
        self,
        nodes: List[EntityNode],
        episode: EpisodicNode,
        previous_episodes: List[EpisodicNode]
    ) -> List[EdgeNode]:
        """
        Extract edges - EXACTLY same logic as Eion Knowledge but using Numa
        """
        start = time()
        
        if len(nodes) < 2:
            logger.debug("Not enough nodes to extract edges with Numa")
            return []
        
        # Prepare context for edge extraction
        node_names = [node.name for node in nodes]
        context = {
            'episode_content': episode.content,
            'previous_episodes': [ep.content for ep in previous_episodes],
            'node_names': node_names,
        }
        
        # Extract edges using Numa
        llm_response = await self.numa_client.generate_response(
            self._create_extract_edges_prompt(context),
            response_model=ExtractedEdges
        )
        
        extracted_edges_data = llm_response.get('extracted_edges', [])
        extracted_edges = [ExtractedEdge(**edge_data) for edge_data in extracted_edges_data]
        
        # Convert to EdgeNode objects - EXACT same logic as Eion Knowledge
        edge_nodes = []
        node_name_to_uuid = {node.name: node.uuid for node in nodes}
        
        for extracted_edge in extracted_edges:
            source_uuid = node_name_to_uuid.get(extracted_edge.source_name)
            target_uuid = node_name_to_uuid.get(extracted_edge.target_name)
            
            if source_uuid and target_uuid and source_uuid != target_uuid:
                edge_node = EdgeNode(
                    uuid=f"{episode.group_id}-edge-{len(edge_nodes)}",
                    source_node_uuid=source_uuid,
                    target_node_uuid=target_uuid,
                    relation_type=extracted_edge.relation_type,
                    summary=extracted_edge.summary or '',
                    group_id=episode.group_id,
                    created_at=utc_now(),
                )
                edge_nodes.append(edge_node)
        
        end = time()
        logger.debug(f'Extracted {len(edge_nodes)} edges with Numa in {(end - start) * 1000} ms')
        return edge_nodes
    
    # All prompt creation methods are IDENTICAL to Eion KnowledgeExtractor
    # This ensures 100% consistency in extraction logic
    
    def _create_extract_message_prompt(self, context: Dict[str, Any]) -> List[Message]:
        """Create prompt for message extraction - IDENTICAL to Eion Knowledge version"""
        system_message = """You are an AI assistant that extracts entity nodes from conversational messages.

Your task is to extract entity nodes mentioned **explicitly or implicitly** in the CURRENT MESSAGE.

**EXCLUDE** entities mentioned only in the PREVIOUS MESSAGES (they are for context only).

Guidelines:
1. Extract all entities, concepts, or actors mentioned in the current message
2. Include people, places, organizations, objects, concepts, events, and any other significant entities
3. Use the provided entity types to classify each entity
4. Provide a brief summary for each entity if additional context is available
5. Only extract entities that are actually mentioned or referenced in the current message"""

        user_content = f"""
<PREVIOUS MESSAGES>
{chr(10).join(context['previous_episodes'])}
</PREVIOUS MESSAGES>

<CURRENT MESSAGE>
{context['episode_content']}
</CURRENT MESSAGE>

<ENTITY TYPES>
{context['entity_types']}
</ENTITY TYPES>

{context['custom_prompt']}

Extract entities mentioned in the CURRENT MESSAGE only."""

        return [
            Message(role="system", content=system_message),
            Message(role="user", content=user_content)
        ]
    
    def _create_extract_text_prompt(self, context: Dict[str, Any]) -> List[Message]:
        """Create prompt for text extraction - IDENTICAL to Eion Knowledge version"""
        system_message = """You are an AI assistant that extracts entity nodes from text.

Your task is to extract all significant entities, concepts, or actors mentioned in the TEXT.

Guidelines:
1. Extract all significant entities, concepts, or actors mentioned in the text
2. Include people, places, organizations, objects, concepts, events, and any other significant entities
3. Use the provided entity types to classify each entity
4. Provide a brief summary for each entity if additional context is available
5. Be thorough but precise in your extraction"""

        user_content = f"""
<TEXT>
{context['episode_content']}
</TEXT>

<ENTITY TYPES>
{context['entity_types']}
</ENTITY TYPES>

{context['custom_prompt']}

Extract all significant entities from the text."""

        return [
            Message(role="system", content=system_message),
            Message(role="user", content=user_content)
        ]
    
    def _create_extract_json_prompt(self, context: Dict[str, Any]) -> List[Message]:
        """Create prompt for JSON extraction - IDENTICAL to Eion Knowledge version"""
        system_message = """You are an AI assistant that extracts entity nodes from JSON data.

Your task is to extract all significant entities, concepts, or actors mentioned in the JSON.

Guidelines:
1. Extract entities from both keys and values in the JSON
2. Include people, places, organizations, objects, concepts, events, and any other significant entities
3. Use the provided entity types to classify each entity
4. Provide a brief summary for each entity if additional context is available
5. Parse the JSON structure to understand the data context"""

        user_content = f"""
<JSON>
{context['episode_content']}
</JSON>

<ENTITY TYPES>
{context['entity_types']}
</ENTITY TYPES>

{context['custom_prompt']}

Extract all significant entities from the JSON data."""

        return [
            Message(role="system", content=system_message),
            Message(role="user", content=user_content)
        ]
    
    def _create_reflexion_prompt(self, context: Dict[str, Any]) -> List[Message]:
        """Create reflexion prompt - IDENTICAL to Eion Knowledge version"""
        system_message = """You are an AI assistant that reviews entity extractions to find missed entities.

Your task is to identify important entities that may have been overlooked in the initial extraction.

Guidelines:
1. Review the episode content and previously extracted entities
2. Look for important entities that were missed
3. Only suggest entities that are clearly mentioned or strongly implied
4. Focus on entities that are significant to understanding the content
5. Do not suggest entities that are too generic or not clearly present"""

        user_content = f"""
<EPISODE CONTENT>
{context['episode_content']}
</EPISODE CONTENT>

<PREVIOUSLY EXTRACTED ENTITIES>
{context['extracted_entities']}
</PREVIOUSLY EXTRACTED ENTITIES>

<PREVIOUS EPISODES (for context)>
{chr(10).join(context['previous_episodes'])}
</PREVIOUS EPISODES>

Review the episode content and identify any important entities that were missed in the initial extraction."""

        return [
            Message(role="system", content=system_message),
            Message(role="user", content=user_content)
        ]
    
    def _create_extract_edges_prompt(self, context: Dict[str, Any]) -> List[Message]:
        """Create edge extraction prompt - IDENTICAL to Eion Knowledge version"""
        system_message = """You are an AI assistant that extracts relationships between entities.

Your task is to identify explicit relationships between the entities mentioned in the text.

Guidelines:
1. Only extract relationships that are explicitly stated or strongly implied
2. Focus on meaningful relationships that add value to understanding
3. Use clear, descriptive relationship types
4. Ensure both source and target entities are from the provided entity list
5. Provide a brief summary of the relationship if additional context is available"""

        user_content = f"""
<EPISODE CONTENT>
{context['episode_content']}
</EPISODE CONTENT>

<ENTITIES>
{context['node_names']}
</ENTITIES>

<PREVIOUS EPISODES (for context)>
{chr(10).join(context['previous_episodes'])}
</PREVIOUS EPISODES>

Extract relationships between the entities that are mentioned in the episode content."""

        return [
            Message(role="system", content=system_message),
            Message(role="user", content=user_content)
        ] 