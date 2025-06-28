class EionConsole {
    constructor() {
        this.config = null;
        this.agents = [];
        this.sessions = [];
        this.users = [];
        this.currentTab = 'register';
        
        this.init();
    }
    
    async init() {
        this.setupEventListeners();
        await this.loadConfig();
        this.showTab('register');
        this.displayConfig();
    }
    
    setupEventListeners() {
        // Tab switching
        document.querySelectorAll('.tab').forEach(tab => {
            tab.addEventListener('click', (e) => {
                const tabName = e.target.dataset.tab;
                this.showTab(tabName);
            });
        });
        
        // Agent registration form
        const agentForm = document.getElementById('agentForm');
        if (agentForm) {
            agentForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.registerAgent();
            });
        }
        
        // Refresh buttons
        document.querySelectorAll('.refresh-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                this.refreshData();
            });
        });
        
        // Copy buttons
        document.querySelectorAll('.copy-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                this.copyToClipboard(e.target.dataset.copy);
            });
        });
    }
    
    showTab(tabName) {
        // Update tab buttons
        document.querySelectorAll('.tab').forEach(tab => {
            tab.classList.remove('active');
        });
        document.querySelector(`[data-tab="${tabName}"]`).classList.add('active');
        
        // Update tab content
        document.querySelectorAll('.tab-content').forEach(content => {
            content.classList.remove('active');
        });
        document.getElementById(`${tabName}Tab`).classList.add('active');
        
        this.currentTab = tabName;
        
        // Load data for the current tab
        this.loadTabData(tabName);
    }
    
    async loadTabData(tabName) {
        switch (tabName) {
            case 'monitoring':
                await this.loadAgents();
                await this.loadSessions();
                await this.loadUsers();
                this.renderMonitoringData();
                break;
            case 'resources':
                this.renderResourcesData();
                break;
        }
    }
    
    async loadConfig() {
        try {
            const response = await fetch('/console/api/config');
            const data = await response.json();
            this.config = data.config;
        } catch (error) {
            console.error('Failed to load config:', error);
            this.showError('Failed to load configuration');
        }
    }
    
    displayConfig() {
        if (!this.config) return;
        
        const configDisplay = document.getElementById('configDisplay');
        if (!configDisplay) return;
        
        configDisplay.innerHTML = `
            <div class="config-item">
                <span class="config-label">Cluster API Key</span>
                <span class="config-value">${this.config.cluster_api_key}</span>
            </div>
            <div class="config-item">
                <span class="config-label">Server</span>
                <span class="config-value">${this.config.host}:${this.config.port}</span>
            </div>
            <div class="config-item">
                <span class="config-label">MCP Enabled</span>
                <span class="config-value">${this.config.mcp_enabled ? 'Yes' : 'No'}</span>
            </div>
            ${this.config.mcp_enabled ? `
            <div class="config-item">
                <span class="config-label">MCP Port</span>
                <span class="config-value">${this.config.mcp_port}</span>
            </div>
            ` : ''}
            <div class="config-item">
                <span class="config-label">Numa Enabled</span>
                <span class="config-value">${this.config.numa_enabled ? 'Yes' : 'No'}</span>
            </div>
            ${this.config.numa_enabled ? `
            <div class="config-item">
                <span class="config-label">Neo4j URI</span>
                <span class="config-value">${this.config.neo4j_uri}</span>
            </div>
            ` : ''}
        `;
    }
    
    async registerAgent() {
        const form = document.getElementById('agentForm');
        const formData = new FormData(form);
        
        const agentData = {
            name: formData.get('name'),
            description: formData.get('description'),
            permissions: formData.get('permissions')
        };
        
        try {
            const response = await fetch('/console/api/agents', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(agentData)
            });
            
            if (response.ok) {
                const result = await response.json();
                this.showSuccess(`Agent "${agentData.name}" registered successfully!`);
                form.reset();
                await this.loadAgents();
                this.renderAgentsList();
            } else {
                const error = await response.json();
                this.showError(`Failed to register agent: ${error.error}`);
            }
        } catch (error) {
            console.error('Agent registration error:', error);
            this.showError('Failed to register agent');
        }
    }
    
    async loadAgents() {
        try {
            const response = await fetch('/console/api/agents');
            const data = await response.json();
            this.agents = data.agents || [];
        } catch (error) {
            console.error('Failed to load agents:', error);
            this.agents = [];
        }
    }
    
    async loadSessions() {
        try {
            const response = await fetch('/console/api/sessions');
            const data = await response.json();
            this.sessions = data.sessions || [];
        } catch (error) {
            console.error('Failed to load sessions:', error);
            this.sessions = [];
        }
    }
    
    async loadUsers() {
        try {
            const response = await fetch('/console/api/users');
            const data = await response.json();
            this.users = data.users || [];
        } catch (error) {
            console.error('Failed to load users:', error);
            this.users = [];
        }
    }
    
    renderAgentsList() {
        const agentsList = document.getElementById('agentsList');
        if (!agentsList) return;
        
        if (this.agents.length === 0) {
            agentsList.innerHTML = '<p class="loading">No agents registered yet.</p>';
            return;
        }
        
        const agentsTable = `
            <table class="table">
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>ID</th>
                        <th>Permissions</th>
                        <th>Status</th>
                        <th>Created</th>
                    </tr>
                </thead>
                <tbody>
                    ${this.agents.map(agent => `
                        <tr>
                            <td>${agent.name}</td>
                            <td><code>${agent.id}</code></td>
                            <td><code>${agent.permission || 'r'}</code></td>
                            <td><span class="status-badge status-${agent.status}">${agent.status}</span></td>
                            <td>${new Date(agent.created_at).toLocaleDateString()}</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
        
        agentsList.innerHTML = agentsTable;
    }
    
    renderMonitoringData() {
        this.renderAgentsList();
        
        // Render sessions
        const sessionsList = document.getElementById('sessionsList');
        if (sessionsList) {
            if (this.sessions.length === 0) {
                sessionsList.innerHTML = '<p class="loading">No active sessions.</p>';
            } else {
                // Render sessions table (similar structure)
                sessionsList.innerHTML = '<p class="loading">Sessions monitoring coming soon...</p>';
            }
        }
        
        // Render users
        const usersList = document.getElementById('usersList');
        if (usersList) {
            if (this.users.length === 0) {
                usersList.innerHTML = '<p class="loading">No users found.</p>';
            } else {
                // Render users table (similar structure)
                usersList.innerHTML = '<p class="loading">Users monitoring coming soon...</p>';
            }
        }
    }
    
    renderResourcesData() {
        const serverUrl = `${this.config?.host || 'localhost'}:${this.config?.port || 8080}`;
        const clusterKey = this.config?.cluster_api_key || 'your-cluster-key';
        
        // HTTP API Examples
        const httpExamples = document.getElementById('httpExamples');
        if (httpExamples) {
            httpExamples.innerHTML = `
                <h4>Agent Registration</h4>
                <div class="code-block">POST http://${serverUrl}/api/v1/agents/register
Content-Type: application/json
Authorization: Bearer ${clusterKey}

{
  "agent_id": "my-agent-001",
  "name": "My Agent",
  "permission": "crud",
  "description": "My AI agent"
}</div>
                
                <h4>Memory Operations</h4>
                <div class="code-block">POST http://${serverUrl}/api/v1/memory
Content-Type: application/json
X-Agent-ID: my-agent-001
X-User-ID: user-123
X-Session-ID: session-456

{
  "content": "Store this information",
  "metadata": {"type": "note"}
}</div>
                
                <button class="copy-btn" data-copy="POST http://${serverUrl}/api/v1/agents/register">Copy Agent Registration</button>
            `;
        }
        
        // MCP Examples
        const mcpExamples = document.getElementById('mcpExamples');
        if (mcpExamples) {
            if (this.config?.mcp_enabled) {
                mcpExamples.innerHTML = `
                    <h4>MCP Client Configuration</h4>
                    <div class="code-block">{
  "mcpServers": {
    "eion": {
      "command": "python",
      "args": ["-m", "mcp_server_eion"],
      "env": {
        "EION_SERVER_URL": "http://${serverUrl}",
        "EION_API_KEY": "${clusterKey}"
      }
    }
  }
}</div>
                    
                    <h4>Connect to MCP Server</h4>
                    <div class="code-block">mcp://localhost:${this.config.mcp_port}</div>
                    
                    <button class="copy-btn" data-copy='{"mcpServers":{"eion":{"command":"python","args":["-m","mcp_server_eion"],"env":{"EION_SERVER_URL":"http://${serverUrl}","EION_API_KEY":"${clusterKey}"}}}}'>Copy MCP Config</button>
                `;
            } else {
                mcpExamples.innerHTML = '<p class="loading">MCP is not enabled. Enable it in your configuration to see MCP examples.</p>';
            }
        }
    }
    
    async refreshData() {
        await this.loadConfig();
        this.displayConfig();
        await this.loadTabData(this.currentTab);
    }
    
    copyToClipboard(text) {
        navigator.clipboard.writeText(text).then(() => {
            this.showSuccess('Copied to clipboard!');
        }).catch(() => {
            this.showError('Failed to copy to clipboard');
        });
    }
    
    showSuccess(message) {
        this.showMessage(message, 'success');
    }
    
    showError(message) {
        this.showMessage(message, 'error');
    }
    
    showMessage(message, type) {
        const container = document.querySelector('.container');
        const messageDiv = document.createElement('div');
        messageDiv.className = type;
        messageDiv.textContent = message;
        
        container.insertBefore(messageDiv, container.firstChild);
        
        setTimeout(() => {
            messageDiv.remove();
        }, 5000);
    }
}

// Initialize console when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new EionConsole();
}); 