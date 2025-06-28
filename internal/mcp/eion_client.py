"""
Eion HTTP Client for MCP Server
Handles HTTP communication with Eion Session API endpoints
"""

import httpx
import json
from typing import Dict, Any, Optional, List
from urllib.parse import urljoin


class EionSessionClient:
    """HTTP client for Eion Session API endpoints"""
    
    def __init__(self, base_url: str, timeout: int = 30):
        self.base_url = base_url.rstrip('/')
        self.timeout = timeout
        self.client = httpx.AsyncClient(timeout=timeout)
    
    async def close(self):
        """Close the HTTP client"""
        await self.client.aclose()
    
    def _build_url(self, endpoint: str) -> str:
        """Build full URL for Session API endpoint"""
        return urljoin(self.base_url, endpoint)
    
    def _build_params(self, agent_id: str, user_id: str, **kwargs) -> Dict[str, str]:
        """Build query parameters for Session API"""
        params = {
            "agent_id": agent_id,
            "user_id": user_id
        }
        # Add additional parameters
        for key, value in kwargs.items():
            if value is not None:
                params[key] = str(value)
        return params
    
    async def _make_request(self, method: str, endpoint: str, params: Dict[str, str], json_data: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """Make HTTP request to Session API"""
        url = self._build_url(endpoint)
        
        try:
            response = await self.client.request(
                method=method,
                url=url,
                params=params,
                json=json_data
            )
            
            # Check for HTTP errors
            if response.status_code >= 400:
                error_data = {}
                try:
                    error_data = response.json()
                except:
                    error_data = {"error": response.text}
                
                raise EionAPIError(
                    status_code=response.status_code,
                    error_message=error_data.get("error", "Unknown error"),
                    error_data=error_data
                )
            
            return response.json()
            
        except httpx.RequestError as e:
            raise EionAPIError(
                status_code=0,
                error_message=f"Connection error: {str(e)}",
                error_data={"connection_error": str(e)}
            )
    
    # Memory API Methods
    async def get_memory(self, session_id: str, agent_id: str, user_id: str, last_n: int = 10) -> Dict[str, Any]:
        """Get memories from session"""
        endpoint = f"/sessions/v1/{session_id}/memories/"
        params = self._build_params(agent_id, user_id, last_n=last_n)
        return await self._make_request("GET", endpoint, params)
    
    async def add_memory(self, session_id: str, agent_id: str, user_id: str, messages: List[Dict[str, Any]], metadata: Optional[Dict[str, Any]] = None, skip_processing: bool = False) -> Dict[str, Any]:
        """Add memory to session"""
        endpoint = f"/sessions/v1/{session_id}/memories/"
        params = self._build_params(agent_id, user_id, skip_processing=str(skip_processing).lower())
        
        json_data = {
            "messages": messages,
            "metadata": metadata or {}
        }
        
        return await self._make_request("POST", endpoint, params, json_data)
    
    async def search_memory(self, session_id: str, agent_id: str, user_id: str, query: str, limit: int = 20, min_score: float = 0.0) -> Dict[str, Any]:
        """Search memories in session"""
        endpoint = f"/sessions/v1/{session_id}/memories/search/"
        params = self._build_params(agent_id, user_id, q=query, limit=limit, min_score=min_score)
        return await self._make_request("GET", endpoint, params)
    
    async def delete_memory(self, session_id: str, agent_id: str, user_id: str, message_uuids: List[str]) -> Dict[str, Any]:
        """Delete memories from session"""
        endpoint = f"/sessions/v1/{session_id}/memories/"
        params = self._build_params(agent_id, user_id)
        
        json_data = {"message_uuids": message_uuids}
        
        return await self._make_request("DELETE", endpoint, params, json_data)
    
    # Knowledge API Methods
    async def search_knowledge(self, session_id: str, agent_id: str, user_id: str, query: str, limit: int = 20) -> Dict[str, Any]:
        """Search knowledge in session"""
        endpoint = f"/sessions/v1/{session_id}/knowledge/"
        params = self._build_params(agent_id, user_id, query=query, limit=limit)
        return await self._make_request("GET", endpoint, params)
    
    async def create_knowledge(self, session_id: str, agent_id: str, user_id: str, messages: List[Dict[str, Any]], metadata: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """Create knowledge in session"""
        endpoint = f"/sessions/v1/{session_id}/knowledge/"
        params = self._build_params(agent_id, user_id)
        
        json_data = {
            "messages": messages,
            "metadata": metadata or {}
        }
        
        return await self._make_request("POST", endpoint, params, json_data)
    
    async def update_knowledge(self, session_id: str, agent_id: str, user_id: str, messages: List[Dict[str, Any]]) -> Dict[str, Any]:
        """Update knowledge in session"""
        endpoint = f"/sessions/v1/{session_id}/knowledge/"
        params = self._build_params(agent_id, user_id)
        
        json_data = {"messages": messages}
        
        return await self._make_request("PUT", endpoint, params, json_data)
    
    async def delete_knowledge(self, session_id: str, agent_id: str, user_id: str) -> Dict[str, Any]:
        """Delete knowledge from session"""
        endpoint = f"/sessions/v1/{session_id}/knowledge/"
        params = self._build_params(agent_id, user_id)
        return await self._make_request("DELETE", endpoint, params)


class EionAPIError(Exception):
    """Eion API error exception"""
    
    def __init__(self, status_code: int, error_message: str, error_data: Dict[str, Any]):
        self.status_code = status_code
        self.error_message = error_message
        self.error_data = error_data
        super().__init__(f"Eion API Error {status_code}: {error_message}") 