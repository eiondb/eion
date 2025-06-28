#!/usr/bin/env python3
"""
Eion MCP Server
Provides Model Context Protocol integration for Eion Session API
"""

import asyncio
import json
import logging
import os
import sys
from typing import Any, Sequence

from mcp.server import Server, NotificationOptions
from mcp.server.models import InitializationOptions
import mcp.server.stdio
import mcp.types as types

from .eion_client import EionSessionClient
from .tools import MemoryTools, KnowledgeTools


# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("eion-mcp-server")


class EionMCPServer:
    """Eion MCP Server implementation"""
    
    def __init__(self):
        self.server = Server("eion-mcp-server")
        self.eion_client = None
        self.memory_tools = None
        self.knowledge_tools = None
        
        # Get configuration from environment
        self.eion_base_url = os.getenv("EION_BASE_URL", "http://localhost:8080")
        self.eion_timeout = int(os.getenv("EION_TIMEOUT", "30"))
        
        logger.info(f"Eion MCP Server initialized with base URL: {self.eion_base_url}")
    
    async def setup(self):
        """Setup the MCP server"""
        # Initialize Eion client
        self.eion_client = EionSessionClient(
            base_url=self.eion_base_url,
            timeout=self.eion_timeout
        )
        
        # Initialize tool handlers
        self.memory_tools = MemoryTools(self.eion_client)
        self.knowledge_tools = KnowledgeTools(self.eion_client)
        
        # Setup handlers
        await self._setup_handlers()
        
        logger.info(f"Available memory tools: {len(self.memory_tools.get_tools())}")
        logger.info(f"Available knowledge tools: {len(self.knowledge_tools.get_tools())}")
        logger.info("Eion MCP Server setup complete")
    
    async def cleanup(self):
        """Cleanup resources"""
        if self.eion_client:
            await self.eion_client.close()
        logger.info("Eion MCP Server cleanup complete")
    
    async def _setup_handlers(self):
        """Setup MCP handlers"""
        
        # List tools handler
        @self.server.list_tools()
        async def handle_list_tools() -> list[types.Tool]:
            """List available tools"""
            tools = []
            tools.extend(self.memory_tools.get_tools())
            tools.extend(self.knowledge_tools.get_tools())
            return tools
        
        # Call tool handler  
        @self.server.call_tool()
        async def handle_call_tool(
            name: str, arguments: dict[str, Any] | None
        ) -> list[types.TextContent | types.ImageContent | types.EmbeddedResource]:
            """Handle tool calls"""
            if arguments is None:
                arguments = {}
            
            logger.info(f"Tool call: {name} with arguments: {arguments}")
            
            try:
                # Route to appropriate tool handler
                if name == "get_memory":
                    return await self.memory_tools.handle_get_memory(arguments)
                elif name == "add_memory":
                    return await self.memory_tools.handle_add_memory(arguments)
                elif name == "search_memory":
                    return await self.memory_tools.handle_search_memory(arguments)
                elif name == "delete_memory":
                    return await self.memory_tools.handle_delete_memory(arguments)
                elif name == "search_knowledge":
                    return await self.knowledge_tools.handle_search_knowledge(arguments)
                elif name == "create_knowledge":
                    return await self.knowledge_tools.handle_create_knowledge(arguments)
                elif name == "update_knowledge":
                    return await self.knowledge_tools.handle_update_knowledge(arguments)
                elif name == "delete_knowledge":
                    return await self.knowledge_tools.handle_delete_knowledge(arguments)
                else:
                    raise ValueError(f"Unknown tool: {name}")
                    
            except Exception as e:
                logger.error(f"Error handling tool call {name}: {str(e)}")
                return [types.TextContent(
                    type="text",
                    text=f"Error: {str(e)}"
                )]
    
    async def run(self):
        """Run the MCP server"""
        logger.info("Starting Eion MCP Server...")
        
        try:
            await self.setup()
            
            # Run the server using stdio transport
            async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
                await self.server.run(
                    read_stream,
                    write_stream,
                    InitializationOptions(
                        server_name="eion-mcp-server",
                        server_version="1.0.0",
                        capabilities=self.server.get_capabilities(
                            notification_options=NotificationOptions(),
                            experimental_capabilities={}
                        )
                    )
                )
        
        except Exception as e:
            logger.error(f"Server error: {str(e)}")
            raise
        
        finally:
            await self.cleanup()


async def main():
    """Main entry point"""
    server = EionMCPServer()
    await server.run()


if __name__ == "__main__":
    asyncio.run(main()) 