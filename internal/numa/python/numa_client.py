"""
NUMA Client - Simplified client for local NUMA/extraction operations
Provides unified interface for NUMA-based extraction without external dependencies
"""

import asyncio
import json
import logging
import sys
import traceback
from typing import Dict, List, Optional, Any, Type

import sys
import os
sys.path.append(os.path.join(os.path.dirname(__file__), '..', '..', '..'))
from internal.knowledge.python.knowledge_models import Message
from pydantic import BaseModel
from internal.numa.python.numa_module import Numa

logger = logging.getLogger(__name__)


class NumaClient:
    """Numa Client - implements LLM client interface for seamless replacement"""
    
    def __init__(self, model: str = "en_core_web_sm", **kwargs):
        """Initialize Numa client"""
        self.model = model
        self.numa = Numa(model=model)
        
        # Custom templates for Eion Knowledge compatibility
        self.templates = {
            'extract_nodes': self._get_node_extraction_template(),
            'extract_edges': self._get_edge_extraction_template(),
            'deduplicate': self._get_deduplication_template(),
            'reflexion': self._get_reflexion_template()
        }
    
    def _clean_input(self, input_str: str) -> str:
        """Clean input string - matches LLM client interface"""
        # Clean any invalid Unicode
        cleaned = input_str.encode('utf-8', errors='ignore').decode('utf-8')

        # Remove zero-width characters and other invisible unicode
        zero_width = '\u200b\u200c\u200d\ufeff\u2060'
        for char in zero_width:
            cleaned = cleaned.replace(char, '')

        # Remove control characters except newlines, returns, and tabs
        cleaned = ''.join(char for char in cleaned if ord(char) >= 32 or char in '\n\r\t')

        return cleaned
    
    async def generate_response(
        self,
        messages: List[Message],
        response_model: Optional[Type[BaseModel]] = None,
        max_tokens: Optional[int] = None,
    ) -> Dict[str, Any]:
        """Generate response using Numa - matches LLM client interface exactly"""
        
        # Clean input messages
        for message in messages:
            message.content = self._clean_input(message.content)
        
        # Combine messages into single text
        combined_text = "\n".join([f"[{msg.role}] {msg.content}" for msg in messages])
        
        # Determine extraction type based on response model
        template_type = self._determine_template_type(response_model)
        
        # Prepare template variables
        template_vars = {
            'text': combined_text,
            'response_schema': response_model.model_json_schema() if response_model else None
        }
        
        # Generate response using Numa
        try:
            result = await self._generate_with_template(template_type, template_vars, response_model)
            return result
            
        except Exception as e:
            logger.error(f"Numa generation failed: {e}")
            # Return empty result matching expected format
            if response_model:
                return self._get_empty_response(response_model)
            return {"content": ""}
    
    def _determine_template_type(self, response_model: Optional[Type[BaseModel]]) -> str:
        """Determine which template to use based on response model"""
        if not response_model:
            return 'extract_nodes'  # default
        
        model_name = response_model.__name__.lower()
        
        if 'entities' in model_name or 'entity' in model_name:
            return 'extract_nodes'
        elif 'edges' in model_name or 'edge' in model_name:
            return 'extract_edges'
        elif 'duplicate' in model_name:
            return 'deduplicate'
        elif 'missed' in model_name:
            return 'reflexion'
        else:
            return 'extract_nodes'  # default
    
    async def _generate_with_template(
        self, 
        template_type: str, 
        template_vars: Dict[str, Any], 
        response_model: Optional[Type[BaseModel]]
    ) -> Dict[str, Any]:
        """Generate response using specific template"""
        
        template = self.templates.get(template_type, self.templates['extract_nodes'])
        
        # Use Numa for extraction - use proper template rendering
        from jinja2 import Template
        jinja_template = Template(template)
        rendered_prompt = jinja_template.render(**template_vars)
        
        result = self.numa.fit(
            rendered_prompt,
            domain="knowledge_extraction"
        )
        
        # Format result to match expected response model
        if response_model:
            return self._format_result_for_model(result, response_model)
        
        return {"content": str(result)}
    
    def _format_result_for_model(self, result: Any, response_model: Type[BaseModel]) -> Dict[str, Any]:
        """Format Numa result to match Pydantic response model"""
        model_name = response_model.__name__.lower()
        
        try:
            if 'extractedentities' in model_name:
                return {"extracted_entities": self._extract_entities_from_result(result)}
            elif 'extractededges' in model_name:
                return {"extracted_edges": self._extract_edges_from_result(result)}
            elif 'missedentities' in model_name:
                return {"missed_entities": self._extract_missed_entities_from_result(result)}
            elif 'duplicateentities' in model_name:
                return {"duplicates": self._extract_duplicates_from_result(result)}
            else:
                # Generic extraction
                return {"extracted_entities": self._extract_entities_from_result(result)}
                
        except Exception as e:
            logger.warning(f"Failed to format result for model {model_name}: {e}")
            return self._get_empty_response(response_model)
    
    def _extract_entities_from_result(self, result: Any) -> List[Dict[str, Any]]:
        """Extract entities from Numa result"""
        entities = []
        
        # Handle different result types
        if isinstance(result, dict):
            # Look for entities in the dict structure
            entities_list = result.get("entities", [])
            if isinstance(entities_list, list):
                for item in entities_list:
                    entities.append({
                        "name": str(item),
                        "entity_type_id": 0,  # Default entity type
                        "summary": None
                    })
            # Also check other keys for entity-like data
            for key, value in result.items():
                if key != "entities" and isinstance(value, list):
                    for item in value:
                        if isinstance(item, str) and item.strip():
                            entities.append({
                                "name": str(item),
                                "entity_type_id": 0,
                                "summary": None
                            })
        elif isinstance(result, list):
            for item in result:
                if isinstance(item, dict):
                    # Handle list of result dicts
                    entities.extend(self._extract_entities_from_result(item))
                else:
                    entities.append({
                        "name": str(item),
                        "entity_type_id": 0,
                        "summary": None
                    })
        
        return entities
    
    def _extract_edges_from_result(self, result: Any) -> List[Dict[str, Any]]:
        """Extract edges from Numa result"""
        edges = []
        
        # Extract relationships from Numa result
        if isinstance(result, dict):
            # Look for relations in the dict structure
            relations_list = result.get("relations", [])
            if isinstance(relations_list, list):
                for rel in relations_list:
                    if isinstance(rel, dict):
                        edges.append({
                            "source_name": rel.get("source", ""),
                            "target_name": rel.get("target", ""),
                            "relation_type": rel.get("relation", "related_to"),
                            "summary": rel.get("summary", None)
                        })
            # Also check other keys for relation-like data
            for key, value in result.items():
                if "relation" in key.lower() and isinstance(value, list):
                    for rel in value:
                        if isinstance(rel, dict):
                            edges.append({
                                "source_name": rel.get("source", ""),
                                "target_name": rel.get("target", ""),
                                "relation_type": rel.get("relation", "related_to"),
                                "summary": rel.get("summary", None)
                            })
        elif isinstance(result, list):
            for item in result:
                if isinstance(item, dict):
                    # Handle list of result dicts
                    edges.extend(self._extract_edges_from_result(item))
        
        return edges
    
    def _extract_missed_entities_from_result(self, result: Any) -> List[str]:
        """Extract missed entities from Numa result"""
        missed = []
        
        if isinstance(result, list):
            missed = [str(item) for item in result]
        elif isinstance(result, dict):
            for value in result.values():
                if isinstance(value, list):
                    missed.extend([str(item) for item in value])
        
        return missed
    
    def _extract_duplicates_from_result(self, result: Any) -> List[Dict[str, Any]]:
        """Extract duplicates from Numa result"""
        duplicates = []
        
        if isinstance(result, dict):
            for key, value in result.items():
                if "duplicate" in key.lower() and isinstance(value, list):
                    for dup in value:
                        if isinstance(dup, dict):
                            duplicates.append({
                                "uuid": dup.get("uuid", ""),
                                "duplicate_of": dup.get("duplicate_of", "")
                            })
        
        return duplicates
    
    def _get_empty_response(self, response_model: Type[BaseModel]) -> Dict[str, Any]:
        """Get empty response matching response model structure"""
        model_name = response_model.__name__.lower()
        
        if 'extractedentities' in model_name:
            return {"extracted_entities": []}
        elif 'extractededges' in model_name:
            return {"extracted_edges": []}
        elif 'missedentities' in model_name:
            return {"missed_entities": []}
        elif 'duplicateentities' in model_name:
            return {"duplicates": []}
        else:
            return {"extracted_entities": []}
    
    def _get_node_extraction_template(self) -> str:
        """Jinja template for node extraction"""
        return """
        Extract entities and concepts from the following text:
        
        {{ text }}
        
        {% if response_schema %}
        Return entities as a JSON object matching this schema:
        {{ response_schema | tojson }}
        {% endif %}
        
        Focus on:
        1. People, organizations, locations
        2. Important concepts and topics
        3. Specific objects or items mentioned
        4. Events or actions
        
        Extract entities that are explicitly mentioned in the text.
        """
    
    def _get_edge_extraction_template(self) -> str:
        """Jinja template for edge extraction"""
        return """
        Extract relationships between entities from the following text:
        
        {{ text }}
        
        {% if response_schema %}
        Return relationships as a JSON object matching this schema:
        {{ response_schema | tojson }}
        {% endif %}
        
        Focus on relationships such as:
        1. Person works for Organization
        2. Person knows Person
        3. Event happened at Location
        4. Object belongs to Person
        
        Extract clear, explicit relationships mentioned in the text.
        """
    
    def _get_deduplication_template(self) -> str:
        """Jinja template for deduplication"""
        return """
        Identify duplicate entities in the following text:
        
        {{ text }}
        
        {% if response_schema %}
        Return duplicates as a JSON object matching this schema:
        {{ response_schema | tojson }}
        {% endif %}
        
        Look for entities that refer to the same real-world object but are named differently.
        """
    
    def _get_reflexion_template(self) -> str:
        """Jinja template for reflexion (finding missed entities)"""
        return """
        Review the following text and identify any important entities that might have been missed:
        
        {{ text }}
        
        {% if response_schema %}
        Return missed entities as a JSON object matching this schema:
        {{ response_schema | tojson }}
        {% endif %}
        
        Look for entities that are important but might have been overlooked in initial extraction.
        """ 