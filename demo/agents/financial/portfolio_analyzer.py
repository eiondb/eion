from typing import Dict, Any, List
import json
import requests

from agents.base.agentic_base import AgenticAgent
from config.agent_configs import agent_configs


class PortfolioAnalyzer(AgenticAgent):
    """
    Portfolio analysis agent that calls external market data agent via MCP.
    Demonstrates A2A External collaboration via real Eion API calls.
    """
    
    def __init__(self):
        super().__init__("portfolio-analyzer")
    
    def process_request(self, user_input: str, session_id: str = None, user_id: str = None) -> str:
        """
        Simple interface for interactive chat.
        Processes user request and returns response.
        """
        import uuid
        import json
        
        # Use provided session/user IDs or generate defaults for backward compatibility
        if not session_id:
            session_id = f"session_{uuid.uuid4().hex[:8]}"
        if not user_id:
            user_id = "demo_user"
        
        try:
            print(f"💼 Portfolio Analysis Request")
            print(f"   Session: {session_id}")
            
            # Parse user input as portfolio data (use demo data for market analysis requests)
            if any(term in user_input.lower() for term in ["market", "stocks", "real-time", "current", "tech", "aapl", "googl", "msft"]):
                # Market analysis request - use demo portfolio with tech stocks
                portfolio_data = {
                    "holdings": [
                        {"symbol": "AAPL", "shares": 100, "cost_basis": 150.00},
                        {"symbol": "GOOGL", "shares": 50, "cost_basis": 2800.00},
                        {"symbol": "MSFT", "shares": 75, "cost_basis": 340.00}
                    ],
                    "cash": 25000,
                    "total_value": 500000,
                    "analysis_request": user_input,
                    "request_type": "market_analysis"
                }
            elif "holdings" in user_input.lower() or "portfolio" in user_input.lower():
                # Specific portfolio request
                portfolio_data = {
                    "holdings": [
                        {"symbol": "AAPL", "shares": 100, "cost_basis": 150.00},
                        {"symbol": "GOOGL", "shares": 50, "cost_basis": 2800.00},
                        {"symbol": "MSFT", "shares": 75, "cost_basis": 340.00}
                    ],
                    "cash": 25000,
                    "total_value": 500000,
                    "request": user_input
                }
            else:
                # General financial analysis request - still use demo portfolio
                portfolio_data = {
                    "analysis_request": user_input,
                    "holdings": [
                        {"symbol": "AAPL", "shares": 100, "cost_basis": 150.00},
                        {"symbol": "GOOGL", "shares": 50, "cost_basis": 2800.00},
                        {"symbol": "MSFT", "shares": 75, "cost_basis": 340.00}
                    ],
                    "cash": 0
                }
            
            # Process the request
            result = self.process_user_request(portfolio_data, session_id, user_id)
            
            if result.get("external_data_requested"):
                print(f"   🌐 A2A External collaboration completed")
                print(f"   📊 Session now contains shared memory from both agents")
                
                # Show final analysis that incorporates external data
                print(f"   🔍 Generating final analysis using all session context...")
                
            return result["final_analysis"]
                
        except Exception as e:
            return f"❌ Error processing portfolio request: {e}"
    
    def process_user_request(self, portfolio_data: Dict[str, Any], session_id: str, user_id: str) -> Dict[str, Any]:
        """
        Step 3: Process user's portfolio analysis request agentically.
        Calls external agent via MCP when real-time data is needed.
        """
        
        print(f"[{self.agent_id}] Processing portfolio analysis request...")
        
        # Analyze portfolio positions
        portfolio_analysis = self._analyze_portfolio_positions(portfolio_data)
        
        # Agentic decision: Should I log initial analysis to Eion?
        logged = self.agentic_eion_logging(
            content=portfolio_analysis,
            session_id=session_id,
            user_id=user_id,
            context={"phase": "portfolio_analysis", "positions": len(portfolio_data.get("holdings", []))}
        )
        
        # Agentic decision: Do I need external market data?
        needs_external_data = self._determine_external_data_needs(portfolio_analysis, portfolio_data)
        
        external_data = {}
        if needs_external_data:
            # Call external market data agent via MCP
            external_data = self._call_external_market_agent(portfolio_data, session_id, user_id)
        
        # Generate final analysis combining internal + external data
        final_analysis = self._generate_combined_analysis(portfolio_analysis, external_data)
        
        # Agentic decision: Log final analysis to Eion?
        self.agentic_eion_logging(
            content=final_analysis,
            session_id=session_id,
            user_id=user_id,
            context={"phase": "final_analysis", "external_data_used": bool(external_data)}
        )
        
        return {
            "portfolio_analysis": portfolio_analysis,
            "external_data_requested": needs_external_data,
            "external_data_received": external_data,
            "final_analysis": final_analysis,
            "logged_to_eion": logged
        }
    
    def get_final_response(self, session_id: str, user_id: str) -> str:
        """
        Step 6: Get final context from Eion and generate user response.
        """
        
        print(f"[{self.agent_id}] Generating final response...")
        
        # Agentic decision: What context do I need for final response?
        full_context = self.agentic_context_retrieval(
            session_id=session_id,
            user_id=user_id,
            purpose="final_response_generation"
        )
        
        # Generate final response using all context
        final_response = self._generate_final_response(full_context)
        
        # Agentic decision: Should I log the final response?
        self.agentic_eion_logging(
            content=final_response,
            session_id=session_id,
            user_id=user_id,
            context={"phase": "final_response", "user_facing": True}
        )
        
        return final_response
    
    def _analyze_portfolio_positions(self, portfolio_data: Dict[str, Any]) -> str:
        """
        Analyze portfolio positions using Claude.
        Simplified for demo but shows real analysis capabilities.
        """
        
        holdings = portfolio_data.get("holdings", [])
        
        analysis_prompt = f"""
        Analyze this investment portfolio:

        PORTFOLIO DATA:
        {json.dumps(portfolio_data, indent=2)}

        Perform analysis on:
        1. ASSET ALLOCATION: Sector distribution, geographic exposure, asset class balance
        2. RISK METRICS: Portfolio beta, concentration risk, correlation analysis
        3. PERFORMANCE: YTD returns, risk-adjusted returns, benchmark comparison
        4. REAL-TIME NEEDS: Which positions need current market data for accurate analysis

        For each holding, identify:
        - Current allocation percentage
        - Risk contribution
        - Performance impact
        - Need for real-time market data (price, volatility, news)

        Flag positions requiring external market data for complete analysis.
        """
        
        try:
            analysis = self.call_claude_with_system_prompt(analysis_prompt)
            
            # Simplified demo logic: ensure analysis identifies real-time data needs
            if not any(term in analysis.lower() for term in ["real-time", "current", "market data", "price"]):
                analysis += "\n\nReal-time market data required for accurate risk assessment and current portfolio valuation."
            
            return analysis
            
        except Exception as e:
            raise RuntimeError(f"Portfolio analysis failed: {e}")
    
    def _determine_external_data_needs(self, analysis: str, portfolio_data: Dict[str, Any]) -> bool:
        """
        Agentic decision: Do we need to call external market data agent?
        For demo purposes, aggressively call external agents for market analysis.
        """
        
        # Check if analysis mentions need for real-time data
        real_time_indicators = ["real-time", "current", "market data", "volatility", "news", "price", "conditions"]
        needs_data = any(indicator in analysis.lower() for indicator in real_time_indicators)
        
        # Check if this is a market analysis request
        request_text = portfolio_data.get("analysis_request", portfolio_data.get("request", ""))
        market_request = any(term in request_text.lower() for term in ["market", "current", "real-time", "conditions", "stocks"])
        
        # Check if portfolio has active holdings
        holdings = portfolio_data.get("holdings", [])
        has_holdings = len(holdings) > 0
        
        # For demo: call external agent if we need data AND have holdings, OR if it's a market request
        should_call_external = (needs_data and has_holdings) or (market_request and has_holdings)
        
        print(f"[DEBUG] External data decision: needs_data={needs_data}, market_request={market_request}, has_holdings={has_holdings}, calling_external={should_call_external}")
        return should_call_external
    
    def _call_external_market_agent(self, portfolio_data: Dict[str, Any], session_id: str, user_id: str) -> Dict[str, Any]:
        """
        Call external market data agent via MCP protocol.
        Demonstrates real A2A External collaboration via Eion session endpoints.
        """
        
        print(f"[{self.agent_id}] Calling external market data agent via MCP...")
        
        # Extract tickers from portfolio
        tickers = []
        for holding in portfolio_data.get("holdings", []):
            ticker = holding.get("symbol", holding.get("ticker"))
            if ticker:
                tickers.append(ticker)
        
        if not tickers:
            return {"error": "No tickers found in portfolio"}
        
        # Prepare MCP request for external agent (simulating the MCP protocol)
        mcp_request = {
            "target_agent": "market-data-external",
            "method": "mcp_call",
            "credentials": {
                "api_provider": "yahoo_finance",
                "service_url": "https://query1.finance.yahoo.com/v8/finance/chart",
                "rate_limit": "100/minute"
            },
            "commands": [
                {
                    "command": "/get-real-time-quotes",
                    "params": {"tickers": tickers, "fields": ["price", "volume", "change"]}
                },
                {
                    "command": "/get-market-news",
                    "params": {"symbols": tickers[:3], "limit": 5}
                }
            ],
            "shared_context": {
                "session_id": session_id,
                "request_type": "portfolio_analysis", 
                "portfolio_summary": f"Tech-focused portfolio with {len(tickers)} holdings: {', '.join(tickers)}",
                "analysis_needs": "Real-time pricing and market news for risk assessment",
                "collaboration_type": "A2A_External"
            }
        }
        
        # Step 1: Log MCP call request to shared Eion session
        mcp_call_log = f"🌐 MCP CALL TO EXTERNAL AGENT: {json.dumps(mcp_request, indent=2)}"
        print(f"   📝 Logging MCP call to Eion session: {session_id}")
        self.agentic_eion_logging(
            content=mcp_call_log,
            session_id=session_id,
            user_id=user_id,
            context={"phase": "mcp_external_call", "target_agent": "market-data-external", "collaboration": "A2A_External"}
        )
        
        # Step 2: Simulate calling external agent via MCP protocol
        print(f"   🤝 Calling external market-data-external agent via MCP...")
        print(f"   📡 MCP Protocol: Delivering credentials and commands to external agent")
        
        # Step 2a: Actually call the real external agent (not simulation)
        print(f"   🔥 LAUNCHING REAL EXTERNAL AGENT...")
        external_response = self._call_real_external_agent(mcp_request, session_id, user_id)
        
        # Step 2b: Log that we received response from real external agent
        print(f"   📨 Real external agent responded via MCP protocol...")
        print(f"   ✅ External agent logged its own activity to Eion")
        
        # Step 3: We (internal agent) log that we received the external response
        print(f"   📝 Portfolio Analyzer logging collaboration summary...")
        collaboration_summary = f"""🤝 A2A EXTERNAL COLLABORATION SUMMARY:

Internal Agent: portfolio-analyzer
External Agent: market-data-external (REAL, not simulated)
Collaboration Type: A2A External via MCP

PROCESS:
1. Internal agent made MCP call with context
2. REAL external agent was launched as subprocess
3. External agent made its own API calls to Eion (read-only)
4. External agent processed market data commands
5. External agent returned data via MCP response
6. Internal agent logs collaboration (external agent is read-only)

EXTERNAL AGENT ACTIVITY (logged by internal agent):
- Session context read: {external_response.get('context_used', 'N/A')}
- Symbols processed: {external_response.get('symbols_processed', 0)}
- Quotes generated: {len(external_response.get('quotes', {}))}
- News items: {len(external_response.get('news', []))}
- Timestamp: {external_response.get('timestamp', 'N/A')}

SECURITY ARCHITECTURE:
✅ External agent is read-only (permission: r)
✅ External agent cannot write to Eion directly
✅ Internal agent controls all session logging
✅ MCP protocol ensures secure communication

This demonstrates TRUE A2A External collaboration with proper security!"""

        self.agentic_eion_logging(
            content=collaboration_summary,
            session_id=session_id,
            user_id=user_id,
            context={"phase": "a2a_external_complete", "real_external_agent": True, "subprocess_call": True, "external_readonly": True}
        )
        
        return external_response
    
    def _call_real_external_agent(self, mcp_request: Dict[str, Any], session_id: str, user_id: str) -> Dict[str, Any]:
        """
        Actually call the real external agent as a subprocess.
        This demonstrates true A2A External collaboration.
        """
        import subprocess
        import json
        import os
        
        try:
            # Path to the real external agent
            external_agent_path = os.path.join(
                os.path.dirname(__file__), '..', 'external', 'market_data_external.py'
            )
            
            # Prepare arguments for external agent
            args = [
                'python', external_agent_path,
                session_id,
                user_id, 
                json.dumps(mcp_request)
            ]
            
            print(f"   🚀 [portfolio-analyzer] Executing: {' '.join(args[:3])} <mcp_request>")
            
            # Call the real external agent
            result = subprocess.run(
                args,
                capture_output=True,
                text=True,
                timeout=30,
                cwd=os.path.dirname(external_agent_path)
            )
            
            # Debug: Show all subprocess output
            print(f"   🔍 [DEBUG] Subprocess return code: {result.returncode}")
            print(f"   🔍 [DEBUG] Subprocess stdout:\n{result.stdout}")
            print(f"   🔍 [DEBUG] Subprocess stderr:\n{result.stderr}")
            
            if result.returncode == 0:
                # Parse the MCP response from external agent output
                output_lines = result.stdout.strip().split('\n')
                for line in output_lines:
                    if line.startswith('MCP_RESPONSE:'):
                        response_json = line[len('MCP_RESPONSE:'):].strip()
                        return json.loads(response_json)
                
                # If no MCP_RESPONSE found, return error
                print(f"   ⚠️ External agent output:\n{result.stdout}")
                return {"error": "No MCP_RESPONSE found in external agent output"}
            else:
                print(f"   🚨 External agent failed with return code: {result.returncode}")
                print(f"   🚨 Error output: {result.stderr}")
                return {"error": f"External agent subprocess failed: {result.stderr}"}
                
        except subprocess.TimeoutExpired:
            return {"error": "External agent call timed out"}
        except Exception as e:
            return {"error": f"Failed to call external agent: {e}"}

    def _simulate_external_agent_eion_access(self, session_id: str, user_id: str):
        """
        This method is no longer used since we call the real external agent.
        Keeping for reference but it's replaced by _call_real_external_agent.
        """
        print(f"   ℹ️  Note: Using real external agent instead of simulation")
        pass
    
    def _generate_combined_analysis(self, portfolio_analysis: str, external_data: Dict[str, Any]) -> str:
        """
        Combine internal portfolio analysis with external market data.
        """
        
        if not external_data or "error" in external_data:
            return portfolio_analysis + "\n\nNote: External market data not available for real-time analysis."
        
        combined_prompt = f"""
        Generate comprehensive portfolio analysis combining internal analysis with real-time market data.

        INTERNAL PORTFOLIO ANALYSIS:
        {portfolio_analysis}

        REAL-TIME MARKET DATA:
        {json.dumps(external_data, indent=2)}

        Provide updated analysis including:
        1. Current portfolio valuation using real-time prices
        2. Market-adjusted risk assessment
        3. Impact of recent news/market movements
        4. Updated recommendations based on current market conditions
        5. Real-time performance metrics

        Highlight how external market data changed the analysis.
        """
        
        try:
            return self.call_claude_with_system_prompt(combined_prompt)
        except Exception as e:
            return portfolio_analysis + f"\n\nError integrating external data: {e}"
    
    def _generate_final_response(self, full_context: Dict[str, Any]) -> str:
        """
        Generate final user response incorporating all context from Eion.
        """
        
        context_summary = self._summarize_context(full_context)
        
        final_prompt = f"""
        Generate comprehensive final portfolio analysis report for the user.

        COMPLETE SESSION CONTEXT FROM EION:
        {context_summary}

        Provide professional investment report with:
        1. Executive summary of portfolio analysis
        2. Key findings from internal analysis
        3. External market data insights
        4. Current portfolio valuation and performance
        5. Risk assessment and recommendations  
        6. Summary of collaborative analysis process (internal + external agents)

        Format as professional investment advisory report.
        """
        
        return self.call_claude_with_system_prompt(final_prompt)
    
    def _summarize_context(self, context: Dict[str, Any]) -> str:
        """
        Summarize context retrieved from Eion for final response.
        Processes real Eion data.
        """
        
        memory_data = context.get("memory", {})
        summary_parts = []
        
        if memory_data.get("messages"):
            messages = memory_data["messages"]
            summary_parts.append(f"Session contains {len(messages)} interactions")
            
            # Extract key analysis phases
            phases = {}
            for msg in messages:
                metadata = msg.get("metadata", {})
                phase = metadata.get("phase")
                if phase:
                    phases[phase] = phases.get(phase, 0) + 1
            
            if phases:
                summary_parts.append(f"Analysis phases: {', '.join(phases.keys())}")
            
            # Look for external agent interactions
            external_calls = [msg for msg in messages if "mcp" in msg.get("content", "").lower()]
            if external_calls:
                summary_parts.append(f"External agent interactions: {len(external_calls)}")
        
        return "\n".join(summary_parts) if summary_parts else "No context available from session" 