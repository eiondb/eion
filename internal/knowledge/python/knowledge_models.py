"""
Eion Knowledge Models - Migrated EXACTLY from Eion Knowledge codebase
Maintains 100% compatibility with original Eion Knowledge logic
"""

from datetime import datetime, timezone
from enum import Enum
from typing import Any, Dict, List, Optional
from uuid import uuid4
import uuid as uuid_mod

from pydantic import BaseModel, Field


def utc_now() -> datetime:
    """Helper function to get current UTC time - matches Eion Knowledge"""
    return datetime.now(timezone.utc)


class EpisodeType(str, Enum):
    """Episode types - exactly from Eion Knowledge"""
    message = "message"
    text = "text"
    json = "json"
    conversation = "conversation"


class EpisodicNode(BaseModel):
    """Episodic Node - migrated exactly from Eion Knowledge"""
    uuid: str = Field(default_factory=lambda: str(uuid4()))
    group_id: str
    source: EpisodeType
    content: str
    source_description: Optional[str] = None
    created_at: datetime = Field(default_factory=utc_now)
    valid_at: datetime = Field(default_factory=utc_now)
    
    class Config:
        use_enum_values = True


class EntityNode(BaseModel):
    """Entity Node - migrated exactly from Eion Knowledge"""
    uuid: str = Field(default_factory=lambda: str(uuid4()))
    name: str
    labels: List[str] = Field(default_factory=list)
    summary: str = ""
    group_id: str
    created_at: datetime = Field(default_factory=utc_now)
    
    class Config:
        use_enum_values = True


class EdgeNode(BaseModel):
    """Edge Node - migrated exactly from Eion Knowledge"""
    uuid: str = Field(default_factory=lambda: str(uuid4()))
    source_node_uuid: str
    target_node_uuid: str
    relation_type: str
    summary: str = ""
    group_id: str
    created_at: datetime = Field(default_factory=utc_now)
    
    class Config:
        use_enum_values = True


# Extraction Models - exactly from Eion Knowledge prompts/models.py

class ExtractedEntity(BaseModel):
    """Extracted Entity - exactly from Eion Knowledge"""
    name: str
    entity_type_id: int = 0
    summary: Optional[str] = None


class ExtractedEntities(BaseModel):
    """Extracted Entities container - exactly from Eion Knowledge"""
    extracted_entities: List[ExtractedEntity]


class MissedEntities(BaseModel):
    """Missed Entities for reflexion - exactly from Eion Knowledge"""
    missed_entities: List[str]


class ExtractedEdge(BaseModel):
    """Extracted Edge - exactly from Eion Knowledge"""
    source_name: str
    target_name: str
    relation_type: str
    summary: Optional[str] = None


class ExtractedEdges(BaseModel):
    """Extracted Edges container - exactly from Eion Knowledge"""
    extracted_edges: List[ExtractedEdge]


class DuplicateEntity(BaseModel):
    """Duplicate entity for deduplication - exactly from Eion Knowledge"""
    uuid: str
    duplicate_of: str


class DuplicateEntities(BaseModel):
    """Duplicate entities container - exactly from Eion Knowledge"""
    duplicates: List[DuplicateEntity]


class Message(BaseModel):
    """Message model - exactly from Eion Knowledge"""
    role: str
    content: str


# Search Models - for compatibility with Eion Knowledge search functionality

class SearchResults(BaseModel):
    """Search results container"""
    nodes: List[EntityNode] = Field(default_factory=list)
    edges: List[EdgeNode] = Field(default_factory=list)


class SearchFilters(BaseModel):
    """Search filters for compatibility"""
    query_type: Optional[str] = None
    entity_types: Optional[List[str]] = None
    date_range: Optional[str] = None
    relevance_threshold: Optional[float] = None


# Configuration Models

class NodeHybridSearchRRF(BaseModel):
    """Node hybrid search configuration"""
    rank_constant: float = 60.0
    weights: Dict[str, float] = {"semantic": 0.7, "keyword": 0.3} 