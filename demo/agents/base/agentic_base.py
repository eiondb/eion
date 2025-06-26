"""
Agentic Agent Base Classes - Updated for Pure Architecture

Agents make direct HTTP calls to session endpoints only.
No SDK access - agents get endpoints via system prompts.
"""

import json
import requests
from typing import Dict, List, Any, Optional, Tuple
from datetime import datetime, timezone
import anthropic

from config.api_config import api_config
from config.agent_configs import agent_configs


def log_agent_thought(agent_id: str, thought: str):
    """Log agent thought process if verbose mode is enabled"""
    if api_config.verbose_logging:
        print(f"🧠 [{agent_id}] THOUGHT: {thought}")


def log_eion_interaction(agent_id: str, action: str, details: str):
    """Log Eion API interactions if verbose mode is enabled"""
    if api_config.verbose_logging:
        # Color code based on action type
        if action == "ADD_MEMORY":
            color = "\033[36m"  # Cyan
        elif action == "GET_MEMORY":
            color = "\033[35m"  # Magenta
        elif action == "SEARCH_MEMORY":
            color = "\033[33m"  # Yellow
        elif action == "SEARCH_KNOWLEDGE":
            color = "\033[34m"  # Blue
        elif action == "SUCCESS":
            color = "\033[32m"  # Green
        elif action == "ERROR":
            color = "\033[31m"  # Red
        else:
            color = "\033[37m"  # White
        
        reset = "\033[0m"  # Reset color
        print(f"🔗 [{agent_id}] {color}EION {action}{reset}: {details}")


class AgenticDecisionEngine:
    """
    Agentic decision making engine using Claude.
    Makes decisions about when to use Eion HTTP endpoints.
    """
    
    def __init__(self, agent_id: str):
        self.agent_id = agent_id
        self.claude = anthropic.Anthropic(api_key=api_config.claude_api_key)
        self.config = agent_configs.get_agent_config(agent_id)
    
    def should_log_to_eion(self, content: str, context: Dict[str, Any]) -> bool:
        """
        Agentic decision: Should this content be logged to Eion memory?
        """
        
        log_agent_thought(self.agent_id, f"Deciding whether to log content to Eion: {content[:100]}...")
        
        # Simplified but still agentic decision logic for demo
        memory_triggers = self.config.get("memory_logging_triggers", [])
        
        # Check if content matches known triggers
        for trigger in memory_triggers:
            if trigger.replace("_", " ") in content.lower():
                log_agent_thought(self.agent_id, f"Content matches trigger '{trigger}' - will log to Eion")
                return True
        
        # Use Claude for more complex decision (A2A demo-optimized prompt)
        decision_prompt = f"""
        Should this content be logged to Eion memory for A2A collaboration demo?
        Content: {content[:500]}...
        Context: {context}
        
        ALWAYS LOG if content contains:
        - Analysis results or findings
        - MCP calls to external agents
        - External agent responses or data
        - Collaboration status messages
        - User-facing responses
        - Agent handoff information
        - Market data or external API responses
        
        ONLY SKIP if content is:
        - Pure debugging/technical logs
        - Duplicate messages
        - Error handling without valuable data
        
        For A2A demos, err on the side of logging more to show collaboration.
        Answer: YES or NO
        """
        
        try:
            log_agent_thought(self.agent_id, "Asking Claude for logging decision...")
            response = self.claude.messages.create(
                model="claude-3-5-sonnet-20241022",
                max_tokens=10,
                messages=[{"role": "user", "content": decision_prompt}]
            )
            
            decision = response.content[0].text.strip().upper()
            decision_bool = decision.startswith("YES")
            log_agent_thought(self.agent_id, f"Claude decision: {decision} -> {decision_bool}")
            return decision_bool
            
        except Exception as e:
            # Fallback decision for A2A demo reliability - be more aggressive
            log_agent_thought(self.agent_id, f"Claude decision failed, using fallback: {e}")
            a2a_keywords = ["analysis", "result", "finding", "mcp", "external", "collaboration", "market", "data", "agent"]
            fallback_decision = len(content) > 50 and any(keyword in content.lower() for keyword in a2a_keywords)
            log_agent_thought(self.agent_id, f"A2A fallback decision: {fallback_decision}")
            return fallback_decision
    
    def should_handoff_to_agent(self, analysis_result: str) -> Optional[Tuple[str, Dict[str, Any]]]:
        """
        Agentic decision: Should we hand off to another agent?
        """
        
        log_agent_thought(self.agent_id, "Evaluating whether to hand off to another agent...")
        
        handoff_agents = self.config.get("handoff_agents", {})
        
        for target_agent, handoff_config in handoff_agents.items():
            trigger_keywords = handoff_config.get("trigger_keywords", [])
            
            # Check if analysis contains trigger keywords
            analysis_lower = analysis_result.lower()
            triggers_found = [kw for kw in trigger_keywords if kw in analysis_lower]
            
            if triggers_found:
                log_agent_thought(self.agent_id, f"Found handoff triggers {triggers_found} -> handing off to {target_agent}")
                return target_agent, {
                    **handoff_config,
                    "triggered_by": triggers_found
                }
        
        log_agent_thought(self.agent_id, "No handoff triggers found - continuing without handoff")
        return None
    
    def determine_context_needs(self, session_id: str, user_id: str, purpose: str) -> Dict[str, Any]:
        """
        Agentic decision: What context do I need from Eion?
        """
        
        log_agent_thought(self.agent_id, f"Determining context needs for purpose: {purpose}")
        
        context_strategy = self.config.get("context_retrieval_strategy", {})
        
        # Default strategy for demo
        default_needs = {
            "message_count": 10,
            "search_terms": [],
            "knowledge_query": None
        }
        
        # Use configured strategy if available
        if context_strategy:
            needs = {
                "message_count": context_strategy.get("message_count", 10),
                "search_terms": context_strategy.get("search_terms", []),
                "knowledge_query": context_strategy.get("knowledge_query")
            }
            log_agent_thought(self.agent_id, f"Using configured context strategy: {needs}")
            return needs
        
        log_agent_thought(self.agent_id, f"Using default context strategy: {default_needs}")
        return default_needs


class AgenticAgent:
    """
    Base class for agentic agents that make direct HTTP calls to Eion session endpoints.
    No SDK access - agents use only session-level HTTP endpoints.
    """
    
    def __init__(self, agent_id: str):
        self.agent_id = agent_id
        self.config = agent_configs.get_agent_config(agent_id)
        self.base_url = api_config.eion_base_url
        self.claude = anthropic.Anthropic(api_key=api_config.claude_api_key)
        self.decision_engine = AgenticDecisionEngine(agent_id)
        
        # HTTP session for direct API calls
        self.session = requests.Session()
        self.session.headers.update({"Content-Type": "application/json"})
        
        # Track recent successful operations to suppress duplicate 500 errors
        self._recent_success = False
    
    def get_system_prompt(self) -> str:
        """Get system prompt for this agent"""
        return agent_configs.get_system_prompt(self.agent_id)
    
    def _make_session_request(self, method: str, endpoint: str, params: Dict[str, str] = None, 
                             json_data: Dict[str, Any] = None) -> Dict[str, Any]:
        """Make HTTP request to Eion session endpoint"""
        
        url = f"{self.base_url}{endpoint}"
        
        try:
            response = self.session.request(
                method=method,
                url=url,
                params=params,
                json=json_data,
                timeout=30
            )
            
            if response.status_code >= 400:
                error_msg = f"Eion API error: {response.status_code}"
                try:
                    error_data = response.json()
                    error_msg += f" - {error_data.get('error', 'Unknown error')}"
                except:
                    error_msg += f" - {response.text}"
                raise RuntimeError(error_msg)
            
            return response.json()
            
        except requests.exceptions.RequestException as e:
            raise RuntimeError(f"Failed to connect to Eion server: {e}")
    
    def agentic_eion_logging(self, content: str, session_id: str, user_id: str, 
                           context: Dict[str, Any] = None) -> bool:
        """
        Agentically decide whether to log content to Eion, then make direct HTTP call.
        Returns True if content was logged.
        """
        
        if context is None:
            context = {}
        
        # Agentic decision
        should_log = self.decision_engine.should_log_to_eion(content, context)
        
        if should_log:
            try:
                # Reset success flag at start of new operation
                self._recent_success = False
                log_eion_interaction(self.agent_id, "ADD_MEMORY", f"Logging {len(content)} chars to session {session_id}")
                
                # Direct HTTP call to session endpoint
                endpoint = f"/sessions/v1/{session_id}/memories"
                params = {"agent_id": self.agent_id, "user_id": user_id, "skip_processing": "true"}
                payload = {
                    "messages": [{
                        "role": "assistant",
                        "role_type": "assistant", 
                        "content": content
                    }],
                    "metadata": {
                        "agent_decision": "auto_logged",
                        "context": context,
                        "timestamp": datetime.now(timezone.utc).isoformat(),
                        "agent_id": self.agent_id,
                        "user_id": user_id  # Required by server
                    }
                }
                
                result = self._make_session_request("POST", endpoint, params, payload)
                
                log_eion_interaction(self.agent_id, "SUCCESS", "Memory logged successfully")
                self._recent_success = True
                return True
                
            except Exception as e:
                # Suppress 500 errors if we just had a success (likely duplicate call issue)
                if self._recent_success and "500" in str(e):
                    # Don't log the 500 error, just reset the flag
                    self._recent_success = False
                    return False
                else:
                    log_eion_interaction(self.agent_id, "ERROR", f"Failed to log memory: {e}")
                    self._recent_success = False
                    return False
        
        log_agent_thought(self.agent_id, "Content not logged - agent decided it wasn't necessary")
        return False
    
    def agentic_context_retrieval(self, session_id: str, user_id: str, 
                                 purpose: str = "general") -> Dict[str, Any]:
        """
        Agentically decide what context to retrieve from Eion, then make direct HTTP calls.
        """
        
        log_agent_thought(self.agent_id, f"Retrieving context for purpose: {purpose}")
        
        # Agentic decision about what context to retrieve
        context_needs = self.decision_engine.determine_context_needs(session_id, user_id, purpose)
        
        full_context = {}
        
        try:
            # Get recent memory
            if context_needs.get("message_count", 0) > 0:
                log_eion_interaction(self.agent_id, "GET_MEMORY", f"Retrieving {context_needs['message_count']} recent messages")
                
                endpoint = f"/sessions/v1/{session_id}/memories"
                params = {
                    "agent_id": self.agent_id,
                    "user_id": user_id,
                    "last_n": str(context_needs["message_count"])
                }
                
                memory_result = self._make_session_request("GET", endpoint, params)
                full_context["memory"] = memory_result
            
            # Search for specific content if needed
            search_terms = context_needs.get("search_terms", [])
            if search_terms:
                search_results = {}
                for term in search_terms:
                    log_eion_interaction(self.agent_id, "SEARCH_MEMORY", f"Searching for: {term}")
                    
                    endpoint = f"/sessions/v1/{session_id}/memories/search"
                    params = {
                        "agent_id": self.agent_id,
                        "user_id": user_id,
                        "q": term,
                        "limit": "5"
                    }
                    
                    search_result = self._make_session_request("GET", endpoint, params)
                    search_results[term] = search_result
                full_context["search_results"] = search_results
            
            # Search knowledge if needed
            knowledge_query = context_needs.get("knowledge_query")
            if knowledge_query:
                log_eion_interaction(self.agent_id, "SEARCH_KNOWLEDGE", f"Searching knowledge: {knowledge_query}")
                
                endpoint = f"/sessions/v1/{session_id}/knowledge"
                params = {
                    "agent_id": self.agent_id,
                    "user_id": user_id,
                    "query": knowledge_query,
                    "limit": "10"
                }
                
                knowledge_result = self._make_session_request("GET", endpoint, params)
                full_context["knowledge"] = knowledge_result
            
            log_eion_interaction(self.agent_id, "SUCCESS", f"Retrieved context with {len(full_context)} components")
            return full_context
            
        except Exception as e:
            log_eion_interaction(self.agent_id, "ERROR", f"Context retrieval failed: {e}")
            return {"error": str(e)}
    
    def call_claude_with_system_prompt(self, user_message: str, context: str = "") -> str:
        """
        Call Claude with the agent's system prompt and optional context.
        """
        
        system_prompt = self.get_system_prompt()
        
        if context:
            full_message = f"CONTEXT:\n{context}\n\nUSER MESSAGE:\n{user_message}"
        else:
            full_message = user_message
        
        try:
            response = self.claude.messages.create(
                model="claude-3-5-sonnet-20241022",
                max_tokens=4000,
                system=system_prompt,
                messages=[{"role": "user", "content": full_message}]
            )
            
            return response.content[0].text
            
        except Exception as e:
            raise RuntimeError(f"Claude API call failed: {e}")
    
    def agentic_handoff_decision(self, analysis_result: str, session_id: str, user_id: str) -> Optional[str]:
        """
        Agentically decide whether to hand off to another agent.
        """
        
        handoff_info = self.decision_engine.should_handoff_to_agent(analysis_result)
        
        if handoff_info:
            target_agent, handoff_config = handoff_info
            
            # Log the handoff decision to Eion for other agents to see
            handoff_message = f"Handing off to {target_agent}: {handoff_config.get('reason', 'Analysis requires specialized processing')}"
            
            self.agentic_eion_logging(
                content=handoff_message,
                session_id=session_id,
                user_id=user_id,
                context={
                    "phase": "handoff",
                    "target_agent": target_agent,
                    "handoff_config": handoff_config
                }
            )
            
            return target_agent
        
        return None 
        return None 