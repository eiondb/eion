#!/usr/bin/env python3
"""
Real External Market Data Agent
Demonstrates actual A2A External collaboration with Eion API calls
"""

import sys
import os
sys.path.append(os.path.join(os.path.dirname(__file__), '..', '..'))

import requests
import json
import time
from datetime import datetime, timezone
from typing import Dict, Any, List

class MarketDataExternalAgent:
    """
    Real external agent that actually interacts with Eion APIs.
    Demonstrates true A2A External collaboration.
    """
    
    def __init__(self):
        self.agent_id = "market-data-external"
        self.base_url = "http://localhost:8080"
        self.user_id = "demo_user_2025"  # Will be provided by MCP call
        
    def process_mcp_request(self, mcp_request: Dict[str, Any], session_id: str, user_id: str) -> Dict[str, Any]:
        """
        Process MCP request and interact with Eion session.
        This is called by the MCP protocol when internal agents request external data.
        """
        
        print(f"🌐 External Agent Activated")
        print(f"   Session: {session_id}")
        
        try:
            # Step 1: Real External Agent reads session context
            session_context = self._read_session_context(session_id, user_id)
            
            # Step 2: Process the MCP commands
            shared_context = mcp_request.get("shared_context", {})
            commands = mcp_request.get("commands", [])
            
            # Step 3: Generate market data response
            market_response = self._process_market_data_commands(commands, shared_context)
            
            # Step 4: Note: External agent is read-only, so it doesn't log to Eion
            # The internal agent will log the collaboration on its behalf
            print(f"   📝 Read-only access: Internal agent will log collaboration")
            
            return market_response
            
        except Exception as e:
            error_response = {"error": f"External agent processing failed: {e}"}
            print(f"   ❌ Error: {e}")
            return error_response
    
    def _read_session_context(self, session_id: str, user_id: str) -> Dict[str, Any]:
        """
        Real external agent reads session context from Eion.
        """
        print(f"   📡 Reading Eion session context...")
        
        # Build the exact URL and params for transparency
        url = f"{self.base_url}/sessions/v1/{session_id}/memories"
        params = {
            "agent_id": self.agent_id,
            "user_id": user_id,
            "last_n": "10"
        }
        
        print(f"   🌐 GET {url}")
        print(f"   🔧 Params: {params}")
        
        try:
            response = requests.get(url, params=params, timeout=10)
            
            print(f"   📊 Status: {response.status_code}")
            
            if response.status_code == 200:
                data = response.json()
                message_count = len(data.get("messages", []))
                print(f"   ✅ Retrieved {message_count} messages from session")
                return data
            else:
                print(f"   ⚠️ API Error: {response.status_code}")
                return {"messages": [], "error": f"API returned {response.status_code}"}
                
        except Exception as e:
            print(f"   ❌ Connection Error: {e}")
            return {"messages": [], "error": str(e)}
    
    def _process_market_data_commands(self, commands: List[Dict], context: Dict[str, Any]) -> Dict[str, Any]:
        """
        Process market data commands and generate realistic responses.
        """
        print(f"   📊 Processing {len(commands)} market data commands...")
        
        # Extract tickers from commands
        tickers = []
        for cmd in commands:
            if cmd.get("command") == "/get-real-time-quotes":
                tickers.extend(cmd.get("params", {}).get("tickers", []))
        
        # Remove duplicates
        tickers = list(set(tickers))
        print(f"   📈 Fetching data for: {', '.join(tickers)}")
        
        # Simulate Yahoo Finance API call
        quotes = {}
        for ticker in tickers:
            quotes[ticker] = {
                "price": f"${(hash(ticker) % 500 + 50):.2f}",
                "change": f"{((hash(ticker) % 20) - 10) / 10:.2f}%",
                "volume": f"{(hash(ticker) % 10000 + 1000):,}",
                "volatility": f"{(hash(ticker) % 30 + 10):.1f}%",
                "timestamp": datetime.now(timezone.utc).isoformat()
            }
        
        # Generate news for top symbols
        news = []
        for ticker in tickers[:3]:
            news.append({
                "title": f"Market Update: {ticker} shows strong momentum",
                "summary": f"Analysts upgrade {ticker} price target amid strong earnings",
                "symbol": ticker,
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "source": "external-market-data-agent"
            })
        
        print(f"   ✅ Generated market data for {len(tickers)} symbols")
        
        return {
            "quotes": quotes,
            "news": news,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "source": "external-market-data-agent",
            "symbols_processed": len(tickers),
            "context_used": context.get("portfolio_summary", "No context provided")
        }

def main():
    """
    Entry point for external agent when called via MCP.
    """
    if len(sys.argv) != 4:
        print("Usage: python market_data_external.py <session_id> <user_id> <mcp_request_json>")
        sys.exit(1)
    
    session_id = sys.argv[1]
    user_id = sys.argv[2] 
    mcp_request = json.loads(sys.argv[3])
    
    agent = MarketDataExternalAgent()
    response = agent.process_mcp_request(mcp_request, session_id, user_id)
    
    # Return response for MCP protocol
    print(f"MCP_RESPONSE: {json.dumps(response)}")

if __name__ == "__main__":
    main() 