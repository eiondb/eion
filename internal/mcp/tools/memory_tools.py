"""
Memory Tools for MCP Server
Provides MCP tools for Eion Session API memory endpoints
"""

from typing import Dict, Any, List
from mcp import Tool
from mcp.types import TextContent, EmbeddedResource

from ..eion_client import EionSessionClient, EionAPIError


class MemoryTools:
    """Memory-related MCP tools"""
    
    def __init__(self, eion_client: EionSessionClient):
        self.eion_client = eion_client
    
    def get_tools(self) -> List[Tool]:
        """Get all memory tools"""
        return [
            Tool(
                name="get_memory",
                description="Retrieve recent memories from an Eion session",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "session_id": {
                            "type": "string",
                            "description": "Session ID to retrieve memories from"
                        },
                        "agent_id": {
                            "type": "string", 
                            "description": "Agent ID for authentication"
                        },
                        "user_id": {
                            "type": "string",
                            "description": "User ID that the agent is acting for"
                        },
                        "last_n": {
                            "type": "integer",
                            "description": "Number of recent memories to retrieve",
                            "default": 10,
                            "minimum": 1,
                            "maximum": 100
                        }
                    },
                    "required": ["session_id", "agent_id", "user_id"]
                }
            ),
            Tool(
                name="add_memory",
                description="Store new memory in an Eion session",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "session_id": {
                            "type": "string",
                            "description": "Session ID to store memory in"
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
                            "description": "Messages to store as memory",
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
                                        "description": "Message content"
                                    }
                                },
                                "required": ["role", "role_type", "content"]
                            }
                        },
                        "metadata": {
                            "type": "object",
                            "description": "Optional metadata for the memory"
                        },
                        "skip_processing": {
                            "type": "boolean",
                            "description": "Whether to skip knowledge processing",
                            "default": False
                        }
                    },
                    "required": ["session_id", "agent_id", "user_id", "messages"]
                }
            ),
            Tool(
                name="search_memory",
                description="Search for memories in an Eion session using text query",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "session_id": {
                            "type": "string",
                            "description": "Session ID to search memories in"
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
                            "description": "Search query text"
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Maximum number of results to return",
                            "default": 20,
                            "minimum": 1,
                            "maximum": 100
                        },
                        "min_score": {
                            "type": "number",
                            "description": "Minimum similarity score for results",
                            "default": 0.0,
                            "minimum": 0.0,
                            "maximum": 1.0
                        }
                    },
                    "required": ["session_id", "agent_id", "user_id", "query"]
                }
            ),
            Tool(
                name="delete_memory",
                description="Delete specific memories from an Eion session",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "session_id": {
                            "type": "string",
                            "description": "Session ID to delete memories from"
                        },
                        "agent_id": {
                            "type": "string",
                            "description": "Agent ID for authentication"
                        },
                        "user_id": {
                            "type": "string",
                            "description": "User ID that the agent is acting for"
                        },
                        "message_uuids": {
                            "type": "array",
                            "description": "UUIDs of messages to delete",
                            "items": {
                                "type": "string"
                            }
                        }
                    },
                    "required": ["session_id", "agent_id", "user_id", "message_uuids"]
                }
            )
        ]
    
    async def handle_get_memory(self, arguments: Dict[str, Any]) -> List[TextContent]:
        """Handle get_memory tool call"""
        try:
            result = await self.eion_client.get_memory(
                session_id=arguments["session_id"],
                agent_id=arguments["agent_id"],
                user_id=arguments["user_id"],
                last_n=arguments.get("last_n", 10)
            )
            
            return [TextContent(
                type="text",
                text=f"Retrieved {len(result.get('messages', []))} memories from session {arguments['session_id']}\n\nMemories:\n{self._format_memory_result(result)}"
            )]
            
        except EionAPIError as e:
            return [TextContent(
                type="text", 
                text=f"Error retrieving memories: {e.error_message}"
            )]
    
    async def handle_add_memory(self, arguments: Dict[str, Any]) -> List[TextContent]:
        """Handle add_memory tool call"""
        try:
            result = await self.eion_client.add_memory(
                session_id=arguments["session_id"],
                agent_id=arguments["agent_id"],
                user_id=arguments["user_id"],
                messages=arguments["messages"],
                metadata=arguments.get("metadata"),
                skip_processing=arguments.get("skip_processing", False)
            )
            
            return [TextContent(
                type="text",
                text=f"Successfully stored {len(arguments['messages'])} messages in session {arguments['session_id']}"
            )]
            
        except EionAPIError as e:
            return [TextContent(
                type="text",
                text=f"Error storing memory: {e.error_message}"
            )]
    
    async def handle_search_memory(self, arguments: Dict[str, Any]) -> List[TextContent]:
        """Handle search_memory tool call"""
        try:
            result = await self.eion_client.search_memory(
                session_id=arguments["session_id"],
                agent_id=arguments["agent_id"],
                user_id=arguments["user_id"],
                query=arguments["query"],
                limit=arguments.get("limit", 20),
                min_score=arguments.get("min_score", 0.0)
            )
            
            return [TextContent(
                type="text",
                text=f"Found {result.get('total_count', 0)} memories matching '{arguments['query']}'\n\nResults:\n{self._format_search_result(result)}"
            )]
            
        except EionAPIError as e:
            return [TextContent(
                type="text",
                text=f"Error searching memories: {e.error_message}"
            )]
    
    async def handle_delete_memory(self, arguments: Dict[str, Any]) -> List[TextContent]:
        """Handle delete_memory tool call"""
        try:
            result = await self.eion_client.delete_memory(
                session_id=arguments["session_id"],
                agent_id=arguments["agent_id"],
                user_id=arguments["user_id"],
                message_uuids=arguments["message_uuids"]
            )
            
            return [TextContent(
                type="text",
                text=f"Successfully deleted {len(arguments['message_uuids'])} memories from session {arguments['session_id']}"
            )]
            
        except EionAPIError as e:
            return [TextContent(
                type="text",
                text=f"Error deleting memories: {e.error_message}"
            )]
    
    def _format_memory_result(self, result: Dict[str, Any]) -> str:
        """Format memory result for display"""
        messages = result.get("messages", [])
        if not messages:
            return "No memories found"
        
        formatted = []
        for msg in messages:
            formatted.append(f"[{msg.get('role', 'unknown')}] {msg.get('content', '')[:200]}...")
        
        return "\n".join(formatted)
    
    def _format_search_result(self, result: Dict[str, Any]) -> str:
        """Format search result for display"""
        messages = result.get("messages", [])
        if not messages:
            return "No matching memories found"
        
        formatted = []
        for msg in messages:
            score = msg.get("score", 0.0)
            formatted.append(f"[Score: {score:.3f}] [{msg.get('role', 'unknown')}] {msg.get('content', '')[:200]}...")
        
        return "\n".join(formatted) 