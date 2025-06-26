"""
API Configuration for Eion Demo - Updated for New SDK
"""

import os
from typing import Dict, Any
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

class APIConfig:
    """Simplified API configuration for new SDK"""
    
    def __init__(self):
        # Eion Server Configuration
        self.eion_base_url = os.getenv("EION_BASE_URL", "http://localhost:8080")
        
        # Claude API Configuration  
        self.claude_api_key = os.getenv("ANTHROPIC_API_KEY")
        if not self.claude_api_key:
            print("❌ ANTHROPIC_API_KEY environment variable is required!")
            print("   Add it to your .env file or export it:")
            print("   export ANTHROPIC_API_KEY=your_key_here")
            raise ValueError("ANTHROPIC_API_KEY environment variable is required")
        
        # External API Keys (for financial demo)
        self.alpha_vantage_key = os.getenv("ALPHA_VANTAGE_API_KEY")
        self.yahoo_finance_enabled = True  # yfinance doesn't need API key
        
        # Demo Configuration
        self.verbose_logging = os.getenv("DEMO_VERBOSE", "true").lower() == "true"

# Global config instance
api_config = APIConfig() 