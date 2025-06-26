from typing import Dict, List, Any

# Eion Agent Configuration
# Defines system prompts and integration configuration for agents

AGENT_CONFIGS = {
    "contract-parser": {
        "name": "Contract Parser Agent",
        "permission": "crud",
        "guest": False,
        "system_prompt": """You are a specialized legal contract analysis agent.

Your capabilities:
1. EXTRACT key contract terms (payment, termination, liability)
2. IDENTIFY legal clauses and obligations  
3. HANDOFF to risk assessment when needed

DECISION RULES:
- Always log contract analysis results to Eion memory
- Handoff to risk-assessor when you identify: liability, compliance, penalty terms
- Use structured JSON format for extracted terms

EION INTEGRATION:
- You have full read/write access to Eion memory
- Log important findings automatically
- Retrieve context before handoffs

EION API TEMPLATES:
GET memories: GET /sessions/v1/{session_id}/memories?agent_id=contract-parser&user_id={user_id}&last_n=10
ADD memory: POST /sessions/v1/{session_id}/memories?agent_id=contract-parser&user_id={user_id}&skip_processing=true
  Body: {"messages": [{"role": "assistant", "role_type": "assistant", "content": "your_content"}], "metadata": {"agent_decision": "auto_logged"}}
SEARCH memories: GET /sessions/v1/{session_id}/memories/search?agent_id=contract-parser&user_id={user_id}&q={query}
SEARCH knowledge: GET /sessions/v1/{session_id}/knowledge?agent_id=contract-parser&user_id={user_id}&query={query}

AUTHENTICATION:
- Use cluster API key for Eion access: dev_key_eion_2025
- This key provides full read/write permissions to Eion cluster""",
        
        "handoff_agents": {
            "risk-assessor": {
                "trigger_keywords": ["liability", "indemnification", "compliance", "penalty"],
                "message_template": "Contract analysis complete. Findings: {findings}. Risk areas: {risk_areas}"
            }
        }
    },
    
    "risk-assessor": {
        "name": "Legal Risk Assessment Agent",
        "permission": "crud",
        "guest": False,
        "system_prompt": """You are a specialized legal risk assessment agent.

Your capabilities:
1. ASSESS legal and business risks from contract analysis
2. EVALUATE liability exposure and compliance requirements
3. PROVIDE risk mitigation recommendations

DECISION RULES:
- Always retrieve contract analysis from Eion memory first
- Log risk assessment results to Eion memory
- Focus on: financial, compliance, and operational risks

EION INTEGRATION:
- You have full read/write access to Eion memory
- Retrieve context from contract analysis automatically
- Log all risk assessments for future reference

EION API TEMPLATES:
GET memories: GET /sessions/v1/{session_id}/memories?agent_id=risk-assessor&user_id={user_id}&last_n=10
ADD memory: POST /sessions/v1/{session_id}/memories?agent_id=risk-assessor&user_id={user_id}&skip_processing=true
  Body: {"messages": [{"role": "assistant", "role_type": "assistant", "content": "your_content"}], "metadata": {"agent_decision": "auto_logged"}}
SEARCH memories: GET /sessions/v1/{session_id}/memories/search?agent_id=risk-assessor&user_id={user_id}&q={query}
SEARCH knowledge: GET /sessions/v1/{session_id}/knowledge?agent_id=risk-assessor&user_id={user_id}&query={query}

AUTHENTICATION:
- Use cluster API key for Eion access: dev_key_eion_2025
- This key provides full read/write permissions to Eion cluster""",
        
        "analysis_categories": ["financial_risk", "compliance_risk", "operational_risk", "legal_risk"]
    },
    
    "portfolio-analyzer": {
        "name": "Portfolio Analyzer Agent",
        "permission": "crud", 
        "guest": False,
        "system_prompt": """You are a quantitative portfolio analyst.

Your capabilities:
1. ANALYZE portfolio positions and risk metrics
2. REQUEST external market data when needed
3. COMBINE internal analysis with external data

EXTERNAL AGENT ACCESS:
When you need real-time market data, you will call external market data agents.
You control the credential delivery and MCP command structure.

CREDENTIALS TO PROVIDE TO EXTERNAL AGENTS:
{
    "api_provider": "yahoo_finance",
    "service_url": "https://query1.finance.yahoo.com/v8/finance/chart", 
    "rate_limit": "100/minute",
    "access_level": "read_only",
    "supported_commands": ["/get-real-time-quotes", "/get-market-news"]
}

MCP PROTOCOL FOR EXTERNAL CALLS:
- Use /get-real-time-quotes for current prices
- Use /get-market-news for recent market updates  
- Always log external agent responses to Eion
- Include credentials in MCP call payload

EION INTEGRATION:
- You have full read/write access to Eion memory
- Log all analysis results and external agent interactions
- Retrieve context for comprehensive analysis

EION API TEMPLATES:
GET memories: GET /sessions/v1/{session_id}/memories?agent_id=portfolio-analyzer&user_id={user_id}&last_n=10
ADD memory: POST /sessions/v1/{session_id}/memories?agent_id=portfolio-analyzer&user_id={user_id}&skip_processing=true
  Body: {"messages": [{"role": "assistant", "role_type": "assistant", "content": "your_content"}], "metadata": {"agent_decision": "auto_logged"}}
SEARCH memories: GET /sessions/v1/{session_id}/memories/search?agent_id=portfolio-analyzer&user_id={user_id}&q={query}
SEARCH knowledge: GET /sessions/v1/{session_id}/knowledge?agent_id=portfolio-analyzer&user_id={user_id}&query={query}

EXTERNAL AGENT API TEMPLATES (for market-data-external):
When calling external agents, provide these templates:
GET memories: GET /sessions/v1/{session_id}/memories?agent_id=market-data-external&user_id={user_id}&last_n=10
ADD memory: POST /sessions/v1/{session_id}/memories?agent_id=market-data-external&user_id={user_id}&skip_processing=true
  Body: {"messages": [{"role": "assistant", "role_type": "assistant", "content": "market_data_content"}], "metadata": {"source": "external_agent"}}
SEARCH memories: GET /sessions/v1/{session_id}/memories/search?agent_id=market-data-external&user_id={user_id}&q={query}

AUTHENTICATION:
- Use cluster API key for Eion access: dev_key_eion_2025
- External API credentials are separate from Eion authentication""",

        "external_agents": {
            "market-data-external": {
                "credential_delivery": {
                    "method": "mcp_call_payload",
                    "credentials": {
                        "api_provider": "yahoo_finance",
                        "service_url": "https://query1.finance.yahoo.com/v8/finance/chart",
                        "rate_limit": "100/minute"
                    }
                },
                "mcp_commands": ["/get-real-time-quotes", "/get-market-news"],
                "guest_agent": True,
                "permission": "r"
            }
        }
    }
}

# EXTERNAL AGENT INFORMATION (Not system prompts - we don't control those)
EXTERNAL_AGENT_INFO = {
    "market-data-external": {
        "name": "External Market Data Provider",
        "permission": "r",  # Read-only for security
        "guest": True,
        "description": "External market data guest agent",
        "controlled_by": "third_party",  # Not under our control
        "mcp_interface": {
            "expected_commands": ["/get-real-time-quotes", "/get-market-news"],
            "credential_format": {
                "api_provider": "string",
                "service_url": "string", 
                "rate_limit": "string"
            }
        },
        "note": "We can only register this agent and send it credentials via MCP. We cannot control its system prompt or behavior."
    }
}

class AgentConfigs:
    """Centralized agent configuration with system prompts and MCP templates"""
    
    def __init__(self):
        self.configs = AGENT_CONFIGS
    
    def get_agent_config(self, agent_id: str) -> Dict[str, Any]:
        """Get configuration for specific agent"""
        if agent_id not in self.configs:
            raise ValueError(f"No configuration found for agent: {agent_id}")
        return self.configs[agent_id]
    
    def get_system_prompt(self, agent_id: str) -> str:
        """Get system prompt for agent"""
        return self.get_agent_config(agent_id)["system_prompt"]
    
    def get_handoff_config(self, from_agent: str, to_agent: str) -> Dict[str, Any]:
        """Get handoff configuration between agents"""
        config = self.get_agent_config(from_agent)
        handoff_agents = config.get("handoff_agents", {})
        
        if to_agent not in handoff_agents:
            raise ValueError(f"No handoff configuration from {from_agent} to {to_agent}")
        
        return handoff_agents[to_agent]
    
    def get_mcp_template(self, agent_id: str, command: str) -> Dict[str, Any]:
        """Get MCP command template for external agent"""
        config = self.get_agent_config(agent_id)
        external_agents = config.get("external_agents", {})
        
        for ext_agent_id, ext_config in external_agents.items():
            for cmd in ext_config.get("mcp_commands", []):
                if cmd["name"] == command:
                    return cmd["template"]
        
        raise ValueError(f"No MCP template found for {agent_id} command {command}")
    
    def get_external_agent_credentials(self, internal_agent_id: str, external_agent_id: str) -> Dict[str, Any]:
        """Get credentials that internal agent should deliver to external agent"""
        config = self.get_agent_config(internal_agent_id)
        external_agents = config.get("external_agents", {})
        
        if external_agent_id not in external_agents:
            raise ValueError(f"No external agent config for {external_agent_id} in {internal_agent_id}")
        
        credential_delivery = external_agents[external_agent_id].get("credential_delivery", {})
        if not credential_delivery:
            raise ValueError(f"No credential delivery config for {external_agent_id}")
        
        return credential_delivery

# Global config instance
agent_configs = AgentConfigs() 