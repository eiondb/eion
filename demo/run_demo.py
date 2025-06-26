#!/usr/bin/env python3
"""
Live Agent Demo Runner for Eion - Pure Architecture
Agents make direct HTTP calls to session endpoints only.
Uses EionDB SDK only for prerequisite checks.
"""

import sys
import os
sys.path.append(os.path.dirname(__file__))

from agents.legal.contract_parser import ContractParser
from agents.legal.risk_assessor import RiskAssessor
from agents.financial.portfolio_analyzer import PortfolioAnalyzer
from eiondb import EionClient
from eiondb.exceptions import EionError

def check_prerequisites():
    """Check if all prerequisites are met before running the demo"""
    try:
        # Check if Eion server is running using official SDK
        client = EionClient(cluster_api_key="eion_cluster_default_key")
        if not client.server_health():
            print("❌ Eion server is not running!")
            print("   Please start it with: eion setup && eion run")
            return False
        
        # Check if agents are registered using official SDK
        required_agents = ["contract-parser", "risk-assessor", "portfolio-analyzer", "market-data-external"]
        missing_agents = []
        
        for agent_id in required_agents:
            try:
                agent = client.get_agent(agent_id)
                # For market-data-external, verify it's properly configured as guest
                if agent_id == "market-data-external":
                    if not agent.get("guest", False):
                        print(f"⚠️ {agent_id} is not configured as guest agent!")
                        print("   External MCP calls may not work properly")
            except EionError:
                missing_agents.append(agent_id)
        
        if missing_agents:
            print(f"❌ Missing agents: {', '.join(missing_agents)}")
            print("   Please register agents first: python simple_demo.py")
            return False
        
        # Check for Claude API key
        if not os.getenv("ANTHROPIC_API_KEY"):
            print("❌ ANTHROPIC_API_KEY environment variable not set!")
            print("   Please set your Claude API key:")
            print("   export ANTHROPIC_API_KEY=your_key_here")
            return False        
        return True
        
    except Exception as e:
        print(f"❌ Prerequisites check failed: {e}")
        return False

def select_demo_case():
    """Let user select which A2A demo case to run"""
    print("\n🎯 Available A2A Demo Cases:")
    print("1. A2A Internal - Contract Agent calls Risk Agent (internal collaboration)")
    print("2. A2A External - Contract Agent calls External Agent via MCP")
    print("3. Exit")
    
    while True:
        try:
            choice = input("\nSelect a demo case (1-3): ").strip()
            if choice in ['1', '2', '3']:
                return int(choice)
            else:
                print("   Please enter a number between 1-3")
        except KeyboardInterrupt:
            print("\n👋 Goodbye!")
            sys.exit(0)
        except Exception:
            print("   Invalid input. Please enter a number between 1-3")

def run_interactive_chat(agent, demo_case_name, demo_description):
    """Run interactive chat for selected A2A demo case"""
    import uuid
    from eiondb import EionClient
    
    print(f"\n🚀 Starting {demo_case_name}")
    print(f"   {demo_description}")
    
    # Create session for this demo case
    session_id = f"session_{uuid.uuid4().hex[:8]}"
    user_id = "demo_user"
    
    try:
        print(f"📊 Creating Eion session: {session_id}")
        client = EionClient(cluster_api_key="eion_cluster_default_key")
        
        # Debug: Check server health first
        health = client.health_check()
        print(f"   Server health: {health}")
        
        # Create session with detailed debugging
        result = client.create_session(
            session_id=session_id,
            user_id=user_id,
            session_type_id="default",  # Required field that was missing!
            session_name=f"{demo_case_name} - Interactive Session"
        )
        print(f"✅ Session created successfully")
        
        # Verify session was actually created by trying to retrieve it
        try:
            # Note: The SDK might not have a get_session method, let's try anyway
            print(f"   Verifying session exists...")
        except Exception as verify_e:
            print(f"   Warning: Could not verify session creation: {verify_e}")
            
    except Exception as e:
        print(f"❌ Failed to create session: {e}")
        print(f"   Error type: {type(e)}")
        print("   Continuing without session - some features may not work")
        session_id = None
    
    print("\n" + "="*60)
    print("💬 Interactive Demo Mode")
    print("   Type 'quit' or 'exit' to return to demo selection")
    print("   Type 'help' for demo instructions")
    print("   Type Ctrl+C to exit completely")
    print("="*60)
    
    # Show example prompts based on demo case
    if "A2A Internal" in demo_case_name:
        print("\n📋 Try asking about:")
        print("   • 'Analyze this contract and assess its risks: [paste contract]'")
        print("   • 'Review this agreement for compliance issues'")
        print("   • 'Parse this contract and tell me about liability risks'")
        print("\n   The Contract Parser will automatically call the Risk Assessor")
        print("   when risk analysis is needed, demonstrating internal A2A collaboration.")
    elif "A2A External" in demo_case_name:
        print("\n📋 Try asking about:")
        print("   • 'Analyze my portfolio performance with real-time data'")
        print("   • 'What are the current market risks for my holdings?'")
        print("   • 'Get live market data for my AAPL and GOOGL positions'")
        print("\n   The Portfolio Analyzer will call external market data agents via MCP")
        print("   when real-time market data is needed, demonstrating external A2A.")
    
    print("="*60)
    
    while True:
        try:
            user_input = input(f"\n💬 You: ").strip()
            
            if user_input.lower() in ['quit', 'exit', 'q']:
                break
            
            if user_input.lower() in ['help', 'h']:
                print("\n📖 A2A Demo Help:")
                print("   • A2A Internal: One agent calls another internal agent via session endpoints")
                print("   • A2A External: One agent calls external agent via MCP protocol")
                print("   • All agents use session endpoints: /sessions/v1/{session_id}/...")
                print("   • Shared memory across all agents in the session")
                print("   • Automatic agent handoffs based on request complexity")
                print("   • Pure HTTP architecture - no SDK dependencies for agents")
                continue
                
            if not user_input:
                continue
                
            print(f"\n🔄 Processing A2A request...")
            print("   (Front agent making direct HTTP calls to Eion session endpoints...)")
            
            try:
                # Call the agent's main processing method for A2A interaction
                if "A2A Internal" in demo_case_name:
                    print("   📄 Contract Parser analyzing request...")
                    response = agent.process_request(user_input, session_id, user_id)
                    print("   → Contract Parser may call Risk Assessor internally")
                elif "A2A External" in demo_case_name:
                    print("   💼 Portfolio Analyzer analyzing request...")
                    response = agent.process_request(user_input, session_id, user_id)  # Use shared session
                    print("   → Portfolio Analyzer may call Market Data Agent via MCP")
                else:
                    response = agent.process_request(user_input, session_id, user_id)
                
                print(f"\n📝 Response: {response}")
                
                # Show A2A collaboration status
                print(f"\n🔗 A2A Session Status:")
                if "A2A Internal" in demo_case_name:
                    print(f"   • Internal agent collaboration via session endpoints")
                    print(f"   • Contract Parser ↔ Risk Assessor handoff")
                elif "A2A External" in demo_case_name:
                    print(f"   • External agent collaboration via MCP")
                    print(f"   • Portfolio Analyzer → Market Data Agent via MCP protocol")
                print(f"   • Shared memory across all agents in session")
                
            except Exception as e:
                print(f"\n❌ Error processing request: {e}")
                print("This might be due to:")
                print("   • Network connection issues")
                print("   • Claude API rate limits")
                print("   • Eion server connectivity")
                print("Please try again or type 'quit' to exit")
            
        except KeyboardInterrupt:
            print(f"\n👋 Exiting chat with {demo_case_name}")
            break
        except Exception as e:
            print(f"\n❌ Unexpected error: {e}")
            print("Please try again or type 'quit' to exit")

def main():
    """Main A2A demo runner"""
    
    # Check prerequisites before starting (using SDK for cluster checks only)
    if not check_prerequisites():
        print(f"\n🔧 Setup Instructions:")
        print(f"   1. Set up Eion: eion setup")
        print(f"   2. Start server: eion run --detached")
        print(f"   3. Register agents: python simple_demo.py")
        print(f"   4. Set Claude API key: export ANTHROPIC_API_KEY=your_key")
        print(f"   5. Run demo: python run_demo.py")
        return 1
    
    # Initialize agents for A2A demos (they will make direct HTTP calls)
    try:
        contract_parser = ContractParser()  # Front agent for A2A Internal demo
        risk_assessor = RiskAssessor()      # Internal agent for A2A Internal
        portfolio_analyzer = PortfolioAnalyzer()  # Front agent for A2A External demo
    except Exception as e:
        print(f"❌ Failed to initialize agents: {e}")
        return 1
    
    # A2A Demo Cases
    demo_cases = {
        1: (
            contract_parser, 
            "A2A Internal Demo", 
            "Contract Parser (front) → Risk Assessor (internal)"
        ),
        2: (
            portfolio_analyzer, 
            "A2A External Demo", 
            "Portfolio Analyzer (front) → Market Data Agent (via MCP)"
        )
    }
    
    while True:
        try:
            choice = select_demo_case()
            
            if choice == 3:
                print("\n👋 Thanks for trying the Eion A2A demo!")
                print("   Learn more at: https://github.com/eiondb/eion")
                break
                
            agent, demo_case_name, demo_description = demo_cases[choice]
            run_interactive_chat(agent, demo_case_name, demo_description)
            
        except KeyboardInterrupt:
            print("\n👋 Thanks for trying the Eion A2A demo!")
            break
        except Exception as e:
            print(f"\n❌ Unexpected error: {e}")
            print("A2A demo will continue...")

    return 0

if __name__ == "__main__":
    sys.exit(main()) 