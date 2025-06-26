from typing import Dict, Any
import json

from agents.base.agentic_base import AgenticAgent


class RiskAssessor(AgenticAgent):
    """
    Legal risk assessment agent that receives handoffs and agentically retrieves context.
    Demonstrates A2A Internal collaboration via real Eion API calls.
    """
    
    def __init__(self):
        super().__init__("risk-assessor")
    
    def process_request(self, user_input: str) -> str:
        """
        Simple interface for interactive chat.
        Processes user request and returns response.
        """
        import uuid
        
        # Generate session and user IDs for demo
        session_id = f"session_{uuid.uuid4().hex[:8]}"
        user_id = "demo_user"
        
        try:
            print(f"⚖️ Performing risk assessment...")
            print(f"📊 Retrieving context from Eion session: {session_id}")
            
            # For standalone use, treat user input as contract to assess
            # First log the input as contract analysis
            self.agentic_eion_logging(
                content=f"Contract for risk assessment: {user_input}",
                session_id=session_id,
                user_id=user_id,
                context={"phase": "contract_input", "source": "direct"}
            )
            
            # Process handoff (will retrieve context we just logged)
            result = self.process_handoff(session_id, user_id)
            
            return result["risk_analysis"]
                
        except Exception as e:
            return f"❌ Error processing risk assessment: {e}"
    
    def process_handoff(self, session_id: str, user_id: str) -> Dict[str, Any]:
        """
        Step 5: Process handoff from contract parser agent.
        Agentically retrieves full context and performs risk assessment.
        """
        
        print(f"[{self.agent_id}] Processing handoff for risk assessment...")
        
        # Agentic decision: What context do I need from Eion?
        contract_context = self.agentic_context_retrieval(
            session_id=session_id,
            user_id=user_id,
            purpose="risk_assessment"
        )
        
        # Perform risk assessment using retrieved context
        risk_analysis = self._assess_contract_risks(contract_context)
        
        # Agentic decision: Should I log this risk assessment?
        logged = self.agentic_eion_logging(
            content=risk_analysis,
            session_id=session_id,
            user_id=user_id,
            context={"phase": "risk_assessment", "final_result": True}
        )
        
        return {
            "risk_analysis": risk_analysis,
            "logged_to_eion": logged,
            "context_retrieved": len(str(contract_context)),
            "assessment_complete": True
        }
    
    def _assess_contract_risks(self, context: Dict[str, Any]) -> str:
        """
        Perform risk assessment using context retrieved from Eion.
        Uses real Eion data but simplified logic for demo.
        """
        
        contract_analysis = self._extract_contract_analysis(context)
        
        risk_prompt = f"""
        Perform comprehensive legal and business risk assessment based on contract analysis.

        CONTRACT ANALYSIS FROM PREVIOUS AGENT:
        {contract_analysis}

        Evaluate and provide risk scores (1-10) for:

        1. REGULATORY COMPLIANCE RISKS:
           - GDPR compliance gaps
           - SOX requirements
           - Industry-specific regulations
           - Data privacy violations

        2. FINANCIAL EXPOSURE RISKS:
           - Liability cap adequacy
           - Penalty calculations
           - Insurance coverage gaps
           - Payment term risks

        3. OPERATIONAL RISKS:
           - Service level dependencies
           - Termination impact
           - Business continuity risks
           - Vendor lock-in risks

        4. LEGAL RISKS:
           - Dispute resolution mechanisms
           - Jurisdiction disadvantages
           - Enforcement challenges
           - Contract ambiguities

        For each risk category:
        - Risk score (1-10)
        - Impact assessment
        - Likelihood assessment  
        - Specific mitigation recommendations

        Conclude with overall risk rating and priority actions.
        """
        
        try:
            risk_assessment = self.call_claude_with_system_prompt(risk_prompt)
            
            # Simplified demo logic: ensure assessment is comprehensive
            if len(risk_assessment) < 500:
                risk_assessment += "\n\nRecommendation: Schedule legal review meeting to discuss high-priority risks and mitigation strategies."
            
            return risk_assessment
            
        except Exception as e:
            raise RuntimeError(f"Risk assessment failed: {e}")
    
    def _extract_contract_analysis(self, context: Dict[str, Any]) -> str:
        """
        Extract contract analysis from Eion context data.
        Processes real Eion API responses.
        """
        
        memory_data = context.get("memory", {})
        search_results = context.get("search_results", {})
        
        # Extract contract analysis from memory
        contract_analysis_parts = []
        
        if memory_data.get("messages"):
            for message in memory_data["messages"]:
                content = message.get("content", "")
                metadata = message.get("metadata", {})
                
                # Look for contract analysis content
                if any(term in content.lower() for term in ["contract", "analysis", "terms", "clauses"]):
                    contract_analysis_parts.append(content)
                
                # Look for handoff messages with analysis
                if metadata.get("handoff") and "findings" in content.lower():
                    contract_analysis_parts.append(content)
        
        # Extract relevant search results
        for term, results in search_results.items():
            if results.get("messages"):
                for result_msg in results["messages"]:
                    content = result_msg.get("content", "")
                    if "analysis" in content.lower() or "contract" in content.lower():
                        contract_analysis_parts.append(f"Search result for '{term}': {content}")
        
        if contract_analysis_parts:
            return "\n\n".join(contract_analysis_parts)
        else:
            return "No contract analysis found in session context. Unable to perform risk assessment without contract analysis data." 