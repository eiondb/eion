# Eion SDK Demo

A demonstration of Agent-to-Agent (A2A) interactions using the Eion SDK.

## Prerequisites

- An Eion server running on `http://localhost:8080` (or set `EION_BASE_URL` environment variable)
- Python 3.7+

## Quick Start

1. **Install dependencies:**
   ```bash
   pip install -r requirements.txt
   ```

2. **Register agents:**
   ```bash
   python simple_demo.py
   ```

3. **Run interactive demo:**
   ```bash
   python run_demo.py
   ```

## Demo Features

The demo showcases two types of Agent-to-Agent interactions:

### A2A Internal
- **Contract Parser** calls **Risk Assessor** (internal collaboration)
- Shows how agents can collaborate within the same Eion cluster

### A2A External  
- **Contract Parser** calls **External Agent** via MCP (Model Context Protocol)
- Demonstrates external agent integration

## Example Interactions

Try these prompts in the interactive demo:

**A2A Internal:**
```
"Analyze this contract for payment risks: 'Payment due within 90 days of delivery, with 2% penalty for late payment.'"
```

**A2A External:**
```
"Parse this contract and get external validation: 'This agreement expires on December 31, 2024.'"
```

## Configuration

The demo uses these default settings:
- **Base URL:** `http://localhost:8080`
- **API Key:** `dev_key_eion_2025`
- **Agents:** `contract-parser`, `risk-assessor`

Override the base URL with environment variable:
```bash
export EION_BASE_URL=http://your-server:8080
python simple_demo.py
```

## Files

- **`simple_demo.py`** - Agent registration (15 lines)
- **`run_demo.py`** - Interactive A2A chat interface (251 lines) 
- **`config/agent_configs.py`** - Agent system prompts

## SDK Usage Examples

```python
from eiondb import EionClient

# Connect to existing server and register agents
client = EionClient(cluster_api_key="dev_key_eion_2025")
client.register_agent("my-agent", "My Agent", "crud")

# Create users and sessions
client.create_user("user123", "John Doe")
client.create_session("session456", "user123")
```

## Agent System Prompts

Add to each agent's system prompt in `config/agent_configs.py`:

```
EION MEMORY ACCESS:
Your agent_id: your-agent-id
Base URL: http://localhost:8080

1. ADD MEMORY: POST /sessions/v1/{session_id}/memories?agent_id=your-agent-id&user_id={user_id}
2. GET MEMORIES: GET /sessions/v1/{session_id}/memories?agent_id=your-agent-id&user_id={user_id}&last_n=10
3. SEARCH MEMORIES: GET /sessions/v1/{session_id}/memories/search?agent_id=your-agent-id&user_id={user_id}&q={query}
4. SEARCH KNOWLEDGE: GET /sessions/v1/{session_id}/knowledge?agent_id=your-agent-id&user_id={user_id}&query={query}
``` 