"""
Knowledge Tools for MCP Server  
Provides MCP tools for Eion Session API knowledge endpoints
"""

from typing import Dict, Any, List
from mcp import Tool
from mcp.types import TextContent, EmbeddedResource

from ..eion_client import EionSessionClient, EionAPIError


class KnowledgeTools:
    """Knowledge-related MCP tools"""
    
    def __init__(self, eion_client: EionSessionClient):
        self.eion_client = eion_client
    
    def get_tools(self) -> List[Tool]:
        """Get all knowledge tools"""
        return [
            Tool(
                name="search_knowledge",
                description="Search for knowledge in an Eion session using semantic query",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "session_id": {
                            "type": "string",
                            "description": "Session ID to search knowledge in"
                        },
                        "agent_id": {
                            "type": "string",
                            "description": "Agent ID for authentication"
                        },
                        "user_id": {
                            "type": "string",
                            "description": "User ID that the agent is acting for"
                        },
                        "query": {
                            "type": "string",
                            "description": "Semantic search query"
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Maximum number of results to return",
                            "default": 20,
                            "minimum": 1,
                            "maximum": 100
                        }
                    },
                    "required": ["session_id", "agent_id", "user_id", "query"]
                }
            ),
            Tool(
                name="create_knowledge",
                description="Store new knowledge in an Eion session",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "session_id": {
                            "type": "string",
                            "description": "Session ID to store knowledge in"
                        },
                        "agent_id": {
                            "type": "string",
                            "description": "Agent ID for authentication"
                        },
                        "user_id": {
                            "type": "string",
                            "description": "User ID that the agent is acting for"
                        },
                        "messages": {
                            "type": "array",
                            "description": "Messages to process into knowledge",
                            "items": {
                                "type": "object",
                                "properties": {
                                    "role": {
                                        "type": "string",
                                        "enum": ["user", "assistant", "system"]
                                    },
                                    "role_type": {
                                        "type": "string",
                                        "enum": ["user", "assistant", "system"]
                                    },
                                    "content": {
                                        "type": "string",
                                        "description": "Message content to extract knowledge from"
                                    }
                                },
                                "required": ["role", "role_type", "content"]
                            }
                        },
                        "metadata": {
                            "type": "object",
                            "description": "Optional metadata for the knowledge"
                        }
                    },
                    "required": ["session_id", "agent_id", "user_id", "messages"]
                }
            ),
            Tool(
                name="update_knowledge",
                description="Update existing knowledge in an Eion session",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "session_id": {
                            "type": "string",
                            "description": "Session ID to update knowledge in"
                        },
                        "agent_id": {
                            "type": "string",
                            "description": "Agent ID for authentication"
                        },
                        "user_id": {
                            "type": "string",
                            "description": "User ID that the agent is acting for"
                        },
                        "messages": {
                            "type": "array",
                            "description": "Updated messages to replace existing knowledge",
                            "items": {
                                "type": "object",
                                "properties": {
                                    "role": {
                                        "type": "string",
                                        "enum": ["user", "assistant", "system"]
                                    },
                                    "role_type": {
                                        "type": "string",
                                        "enum": ["user", "assistant", "system"]
                                    },
                                    "content": {
                                        "type": "string",
                                        "description": "Updated message content"
                                    }
                                },
                                "required": ["role", "role_type", "content"]
                            }
                        }
                    },
                    "required": ["session_id", "agent_id", "user_id", "messages"]
                }
            ),
            Tool(
                name="delete_knowledge",
                description="Delete all knowledge from an Eion session",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "session_id": {
                            "type": "string",
                            "description": "Session ID to delete knowledge from"
                        },
                        "agent_id": {
                            "type": "string",
                            "description": "Agent ID for authentication"
                        },
                        "user_id": {
                            "type": "string",
                            "description": "User ID that the agent is acting for"
                        }
                    },
                    "required": ["session_id", "agent_id", "user_id"]
                }
            )
        ]
    
    async def handle_search_knowledge(self, arguments: Dict[str, Any]) -> List[TextContent]:
        """Handle search_knowledge tool call"""
        try:
            result = await self.eion_client.search_knowledge(
                session_id=arguments["session_id"],
                agent_id=arguments["agent_id"],
                user_id=arguments["user_id"],
                query=arguments["query"],
                limit=arguments.get("limit", 20)
            )
            
            return [TextContent(
                type="text",
                text=f"Found {result.get('total_count', 0)} knowledge items matching '{arguments['query']}'\n\nResults:\n{self._format_knowledge_result(result)}"
            )]
            
        except EionAPIError as e:
            return [TextContent(
                type="text",
                text=f"Error searching knowledge: {e.error_message}"
            )]
    
    async def handle_create_knowledge(self, arguments: Dict[str, Any]) -> List[TextContent]:
        """Handle create_knowledge tool call"""
        try:
            result = await self.eion_client.create_knowledge(
                session_id=arguments["session_id"],
                agent_id=arguments["agent_id"],
                user_id=arguments["user_id"],
                messages=arguments["messages"],
                metadata=arguments.get("metadata")
            )
            
            return [TextContent(
                type="text",
                text=f"Successfully created knowledge from {len(arguments['messages'])} messages in session {arguments['session_id']}"
            )]
            
        except EionAPIError as e:
            return [TextContent(
                type="text",
                text=f"Error creating knowledge: {e.error_message}"
            )]
    
    async def handle_update_knowledge(self, arguments: Dict[str, Any]) -> List[TextContent]:
        """Handle update_knowledge tool call"""
        try:
            result = await self.eion_client.update_knowledge(
                session_id=arguments["session_id"],
                agent_id=arguments["agent_id"],
                user_id=arguments["user_id"],
                messages=arguments["messages"]
            )
            
            return [TextContent(
                type="text",
                text=f"Successfully updated knowledge with {len(arguments['messages'])} messages in session {arguments['session_id']}"
            )]
            
        except EionAPIError as e:
            return [TextContent(
                type="text",
                text=f"Error updating knowledge: {e.error_message}"
            )]
    
    async def handle_delete_knowledge(self, arguments: Dict[str, Any]) -> List[TextContent]:
        """Handle delete_knowledge tool call"""
        try:
            result = await self.eion_client.delete_knowledge(
                session_id=arguments["session_id"],
                agent_id=arguments["agent_id"],
                user_id=arguments["user_id"]
            )
            
            return [TextContent(
                type="text",
                text=f"Successfully deleted all knowledge from session {arguments['session_id']}"
            )]
            
        except EionAPIError as e:
            return [TextContent(
                type="text",
                text=f"Error deleting knowledge: {e.error_message}"
            )]
    
    def _format_knowledge_result(self, result: Dict[str, Any]) -> str:
        """Format knowledge search result for display"""
        messages = result.get("messages", [])
        facts = result.get("facts", [])
        
        if not messages and not facts:
            return "No knowledge found"
        
        formatted = []
        
        # Format messages
        if messages:
            formatted.append("Messages:")
            for msg in messages:
                formatted.append(f"  [{msg.get('role', 'unknown')}] {msg.get('content', '')[:200]}...")
        
        # Format facts
        if facts:
            formatted.append("Facts:")
            for fact in facts:
                formatted.append(f"  {fact.get('content', '')[:200]}...")
        
        return "\n".join(formatted) 