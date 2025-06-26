from typing import Dict, Any, Optional
import json

from agents.base.agentic_base import AgenticAgent


class ContractParser(AgenticAgent):
    """
    Legal contract parsing agent that agentically interacts with Eion.
    Demonstrates A2A Internal collaboration via real Eion API calls.
    """
    
    def __init__(self):
        super().__init__("contract-parser")
    
    def process_request(self, user_input: str, session_id: str = None, user_id: str = None) -> str:
        """
        Simple interface for interactive chat.
        Processes user request and returns response.
        """
        # Use provided session/user IDs or generate defaults for backward compatibility
        if not session_id:
            import uuid
            session_id = f"session_{uuid.uuid4().hex[:8]}"
        if not user_id:
            user_id = "demo_user"
        
        try:
            print(f"🔍 Analyzing contract request...")
            print(f"📊 Using Eion session: {session_id}")
            
            # Process the request
            result = self.process_user_request(user_input, session_id, user_id)
            
            if result.get("requires_followup"):
                print(f"🤝 Collaborating with {result['handoff_target']}")
                
                # Actually call the target agent
                if result['handoff_target'] == 'risk-assessor':
                    from agents.legal.risk_assessor import RiskAssessor
                    risk_agent = RiskAssessor()
                    risk_result = risk_agent.process_handoff(session_id, user_id)
                    print(f"✅ Risk assessment completed")
                
                # Get final response after collaboration
                final_response = self.get_final_response(session_id, user_id)
                return final_response
            else:
                return result["analysis"]
                
        except Exception as e:
            return f"❌ Error processing request: {e}"
    
    def process_user_request(self, contract_text: str, session_id: str, user_id: str) -> Dict[str, Any]:
        """
        Step 3: Process user's contract analysis request agentically.
        Makes real decisions about Eion interactions and agent handoffs.
        """
        
        print(f"[{self.agent_id}] Processing contract analysis request...")
        
        # Analyze the contract using Claude
        analysis_result = self._analyze_contract(contract_text)
        
        # Agentic decision: Should I log this analysis to Eion?
        logged = self.agentic_eion_logging(
            content=analysis_result,
            session_id=session_id,
            user_id=user_id,
            context={"phase": "contract_analysis", "input_length": len(contract_text)}
        )
        
        # Agentic decision: Should I hand off to another agent?
        handoff_target = self.agentic_handoff_decision(
            analysis_result=analysis_result,
            session_id=session_id,
            user_id=user_id
        )
        
        result = {
            "analysis": analysis_result,
            "logged_to_eion": logged,
            "handoff_target": handoff_target,
            "requires_followup": handoff_target is not None
        }
        
        if handoff_target:
            # Trigger the next agent in the pipeline
            result["next_step"] = f"Handed off to {handoff_target} for risk assessment"
        else:
            result["next_step"] = "Analysis complete - no additional processing needed"
        
        return result
    
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
    
    def _analyze_contract(self, contract_text: str) -> str:
        """
        Perform contract analysis using Claude.
        This logic can be simplified for demo purposes.
        """
        
        analysis_prompt = f"""
        Analyze this legal contract and extract key information:

        CONTRACT:
        {contract_text}

        Extract and analyze:
        1. PAYMENT TERMS: Payment schedule, amounts, late fees
        2. TERMINATION: Notice periods, termination conditions  
        3. LIABILITY: Liability caps, indemnification clauses
        4. DATA & PRIVACY: Data handling requirements, privacy obligations
        5. INTELLECTUAL PROPERTY: IP ownership, licensing rights
        6. COMPLIANCE: Regulatory requirements (GDPR, SOX, etc.)

        For each section, provide:
        - Key terms found
        - Potential risk areas
        - Compliance concerns

        Format as structured analysis with clear sections.
        Include risk assessment recommendations at the end.
        """
        
        try:
            analysis = self.call_claude_with_system_prompt(analysis_prompt)
            
            # Simplified demo logic: ensure analysis contains risk indicators
            if not any(risk_word in analysis.lower() for risk_word in ["liability", "penalty", "compliance", "risk"]):
                analysis += "\n\nRisk Assessment Needed: Contract contains liability and compliance clauses requiring specialist review."
            
            return analysis
            
        except Exception as e:
            raise RuntimeError(f"Contract analysis failed: {e}")
    
    def _generate_final_response(self, full_context: Dict[str, Any]) -> str:
        """
        Generate final user response incorporating all context from Eion.
        """
        
        context_summary = self._summarize_context(full_context)
        
        final_prompt = f"""
        Generate a comprehensive final response for the user based on the complete contract analysis workflow.

        CONTEXT FROM EION SESSION:
        {context_summary}

        Provide:
        1. Executive summary of contract analysis
        2. Key findings from contract review
        3. Risk assessment results (if available)
        4. Recommended next steps
        5. Summary of collaborative analysis process

        Make it professional and actionable for business decision-making.
        """
        
        return self.call_claude_with_system_prompt(final_prompt)
    
    def _summarize_context(self, context: Dict[str, Any]) -> str:
        """
        Summarize context retrieved from Eion for use in final response.
        Simplified for demo but processes real Eion data.
        """
        
        memory_data = context.get("memory", {})
        search_results = context.get("search_results", {})
        
        summary_parts = []
        
        # Process memory data
        if memory_data.get("messages"):
            messages = memory_data["messages"]
            summary_parts.append(f"Session contains {len(messages)} interactions")
            
            # Extract key content
            agent_messages = [msg for msg in messages if msg.get("metadata", {}).get("handoff")]
            if agent_messages:
                summary_parts.append(f"Found {len(agent_messages)} agent handoffs")
        
        # Process search results
        if search_results:
            for term, results in search_results.items():
                if results.get("messages"):
                    summary_parts.append(f"Search for '{term}': {len(results['messages'])} relevant messages")
        
        return "\n".join(summary_parts) if summary_parts else "No context available from session" 