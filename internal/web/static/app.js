// Warren Web Interface - Main JavaScript

class WarrenApp {
    constructor() {
        this.ws = null;
        this.reconnectInterval = 3000;
        this.currentView = 'agents';
        this.currentAgentId = null;

        this.init();
    }

    init() {
        this.setupEventListeners();
        this.connectWebSocket();
        this.loadInitialData();
    }

    setupEventListeners() {
        // Tab navigation
        document.querySelectorAll('.tab').forEach(tab => {
            tab.addEventListener('click', (e) => {
                const view = e.target.dataset.view;
                this.switchView(view);
            });
        });

        // Refresh buttons
        document.getElementById('refresh-agents').addEventListener('click', () => this.loadAgents());
        document.getElementById('refresh-notifications').addEventListener('click', () => this.loadNotifications());
        document.getElementById('refresh-servers').addEventListener('click', () => this.loadServers());

        // Back button
        document.getElementById('back-to-agents').addEventListener('click', () => {
            this.switchView('agents');
        });
    }

    switchView(view) {
        // Update tabs
        document.querySelectorAll('.tab').forEach(tab => {
            tab.classList.toggle('active', tab.dataset.view === view);
        });

        // Update views
        document.querySelectorAll('.view').forEach(v => {
            v.classList.remove('active');
        });

        if (view === 'agent-detail') {
            document.getElementById('agent-detail-view').classList.add('active');
        } else {
            document.getElementById(`${view}-view`).classList.add('active');
        }

        this.currentView = view;

        // Load data for the view
        if (view === 'agents') {
            this.loadAgents();
        } else if (view === 'notifications') {
            this.loadNotifications();
        } else if (view === 'servers') {
            this.loadServers();
        }
    }

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.updateConnectionStatus(true);
        };

        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            this.updateConnectionStatus(false);

            // Reconnect after delay
            setTimeout(() => this.connectWebSocket(), this.reconnectInterval);
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleWebSocketMessage(message);
        };
    }

    updateConnectionStatus(connected) {
        const statusDot = document.getElementById('ws-status');
        const statusText = document.getElementById('ws-status-text');

        if (connected) {
            statusDot.classList.remove('offline');
            statusDot.classList.add('online');
            statusText.textContent = 'Connected';
        } else {
            statusDot.classList.remove('online');
            statusDot.classList.add('offline');
            statusText.textContent = 'Disconnected';
        }
    }

    handleWebSocketMessage(message) {
        console.log('WebSocket message:', message);

        if (message.type === 'state_change') {
            this.handleStateChange(message);
        } else if (message.type === 'notification') {
            this.handleNotification(message);
        }

        this.updateLastUpdate();
    }

    handleStateChange(message) {
        // Update agent card if visible
        const agentCard = document.querySelector(`[data-agent-id="${message.agent_id}"]`);
        if (agentCard) {
            const stateBadge = agentCard.querySelector('.state-badge');
            if (stateBadge) {
                stateBadge.className = `state-badge state-${message.to_state}`;
                stateBadge.textContent = message.to_state.replace('_', ' ');
            }
        }

        // Reload agent detail if viewing this agent
        if (this.currentView === 'agent-detail' && this.currentAgentId === message.agent_id) {
            this.loadAgentDetail(message.agent_id);
        }
    }

    handleNotification(message) {
        // Update notification badge
        const badge = document.getElementById('notif-badge');
        if (message.count > 0) {
            badge.textContent = message.count;
            badge.classList.remove('hidden');
        } else {
            badge.classList.add('hidden');
        }

        // Reload notifications if viewing
        if (this.currentView === 'notifications') {
            this.loadNotifications();
        }
    }

    updateLastUpdate() {
        const now = new Date();
        document.getElementById('last-update').textContent = now.toLocaleTimeString();
    }

    loadInitialData() {
        this.loadAgents();
        this.loadNotifications();
        this.loadServers();
    }

    async loadAgents() {
        try {
            const response = await fetch('/api/agents');
            const agents = await response.json();

            const container = document.getElementById('agents-list');

            if (agents.length === 0) {
                container.innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">🤖</div>
                        <p>No agent sessions found</p>
                    </div>
                `;
                return;
            }

            container.innerHTML = agents.map(agent => `
                <div class="agent-card" data-agent-id="${agent.id}" onclick="app.showAgentDetail('${agent.id}')">
                    <div class="agent-card-header">
                        <div class="agent-id">${agent.id}</div>
                        <div class="state-badge state-${agent.state}">${agent.state.replace('_', ' ')}</div>
                    </div>
                    <div class="agent-meta">
                        <div class="agent-meta-item">
                            <span class="agent-meta-label">Pane ID:</span>
                            <span>${agent.pane_id}</span>
                        </div>
                        <div class="agent-meta-item">
                            <span class="agent-meta-label">Last Poll:</span>
                            <span>${this.formatTime(agent.last_poll)}</span>
                        </div>
                        <div class="agent-meta-item">
                            <span class="agent-meta-label">Errors:</span>
                            <span>${agent.error_count}</span>
                        </div>
                    </div>
                </div>
            `).join('');

            this.updateLastUpdate();
        } catch (error) {
            console.error('Failed to load agents:', error);
            document.getElementById('agents-list').innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">⚠️</div>
                    <p>Failed to load agents</p>
                </div>
            `;
        }
    }

    async showAgentDetail(agentId) {
        this.currentAgentId = agentId;
        this.switchView('agent-detail');

        document.getElementById('agent-detail-title').textContent = `Agent: ${agentId}`;
        document.getElementById('agent-detail-content').innerHTML = '<div class="loading">Loading...</div>';

        await this.loadAgentDetail(agentId);
    }

    async loadAgentDetail(agentId) {
        try {
            const response = await fetch(`/api/agents/${agentId}`);
            const agent = await response.json();

            const container = document.getElementById('agent-detail-content');

            container.innerHTML = `
                <div class="agent-detail-tabs">
                    <button class="detail-tab active" data-tab="info">Info</button>
                    <button class="detail-tab" data-tab="conversation">Conversation</button>
                </div>

                <div class="agent-detail-tab-content">
                    <div id="info-tab" class="tab-pane active">
                        <div class="agent-detail-grid">
                            <div class="detail-section">
                                <h3>Session Info</h3>
                                <div class="detail-item">
                                    <span class="detail-label">Agent ID:</span>
                                    <span class="detail-value">${agent.id}</span>
                                </div>
                                <div class="detail-item">
                                    <span class="detail-label">Pane ID:</span>
                                    <span class="detail-value">${agent.pane_id}</span>
                                </div>
                                <div class="detail-item">
                                    <span class="detail-label">State:</span>
                                    <span class="state-badge state-${agent.state}">${agent.state.replace('_', ' ')}</span>
                                </div>
                                <div class="detail-item">
                                    <span class="detail-label">Last Poll:</span>
                                    <span class="detail-value">${this.formatTime(agent.last_poll)}</span>
                                </div>
                                <div class="detail-item">
                                    <span class="detail-label">Error Count:</span>
                                    <span class="detail-value">${agent.error_count}</span>
                                </div>
                            </div>

                            <div class="detail-section">
                                <h3>Artifact Profile</h3>
                                ${agent.profile ? `
                                    <div class="detail-item">
                                        <span class="detail-label">Files Visited:</span>
                                        <span class="detail-value">${agent.profile.files_visited ? agent.profile.files_visited.length : 0}</span>
                                    </div>
                                    <div class="detail-item">
                                        <span class="detail-label">Total Reads:</span>
                                        <span class="detail-value">${agent.profile.total_reads || 0}</span>
                                    </div>
                                    <div class="detail-item">
                                        <span class="detail-label">Total Writes:</span>
                                        <span class="detail-value">${agent.profile.total_writes || 0}</span>
                                    </div>
                                    <div class="detail-item">
                                        <span class="detail-label">Repos:</span>
                                        <span class="detail-value">${agent.profile.repos_touched ? agent.profile.repos_touched.length : 0}</span>
                                    </div>
                                ` : '<p>No artifact profile available</p>'}
                            </div>
                        </div>

                        <div class="detail-section">
                            <h3>Recent Activities</h3>
                            <div class="activity-list">
                                ${agent.activities && agent.activities.length > 0 ?
                                    agent.activities.map(activity => `
                                        <div class="activity-item ${activity.activity_type}">
                                            <div class="activity-type">${activity.activity_type}</div>
                                            <div class="activity-content">${this.escapeHtml(activity.content)}</div>
                                            <div class="activity-time">${this.formatTime(activity.timestamp)}</div>
                                        </div>
                                    `).join('') :
                                    '<p>No recent activities</p>'
                                }
                            </div>
                        </div>
                    </div>

                    <div id="conversation-tab" class="tab-pane">
                        <div class="loading">Loading conversation...</div>
                    </div>
                </div>
            `;

            // Setup tab switching
            container.querySelectorAll('.detail-tab').forEach(tab => {
                tab.addEventListener('click', (e) => {
                    const tabName = e.target.dataset.tab;
                    this.switchAgentTab(tabName, agentId);
                });
            });

            this.updateLastUpdate();
        } catch (error) {
            console.error('Failed to load agent detail:', error);
            document.getElementById('agent-detail-content').innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">⚠️</div>
                    <p>Failed to load agent details</p>
                </div>
            `;
        }
    }

    switchAgentTab(tabName, agentId) {
        // Update tab buttons
        document.querySelectorAll('.detail-tab').forEach(tab => {
            tab.classList.toggle('active', tab.dataset.tab === tabName);
        });

        // Update tab panes
        document.querySelectorAll('.tab-pane').forEach(pane => {
            pane.classList.remove('active');
        });
        document.getElementById(`${tabName}-tab`).classList.add('active');

        // Load conversation if switching to conversation tab
        if (tabName === 'conversation') {
            this.loadConversation(agentId);
        }
    }

    async loadConversation(agentId) {
        const container = document.getElementById('conversation-tab');
        container.innerHTML = '<div class="loading">Loading conversation...</div>';

        try {
            const response = await fetch(`/api/conversation/${agentId}?limit=50`);
            const data = await response.json();

            if (data.status === 'pending_integration') {
                container.innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">💬</div>
                        <h3>Conversation History</h3>
                        <p>${data.message}</p>
                        <p class="text-muted">This feature will be available once Warren has full topology tracking.</p>
                    </div>
                `;
                return;
            }

            if (!data.messages || data.messages.length === 0) {
                container.innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">💬</div>
                        <p>No conversation history yet</p>
                    </div>
                `;
                return;
            }

            // Render messages
            container.innerHTML = `
                <div class="conversation-container">
                    <div class="conversation-header">
                        <h3>Conversation History</h3>
                        <span class="message-count">${data.total} messages</span>
                    </div>
                    <div class="conversation-messages">
                        ${data.messages.map(msg => this.renderMessage(msg)).join('')}
                    </div>
                </div>
            `;

            // Auto-scroll to bottom
            const messagesContainer = container.querySelector('.conversation-messages');
            messagesContainer.scrollTop = messagesContainer.scrollHeight;

        } catch (error) {
            console.error('Failed to load conversation:', error);
            container.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">⚠️</div>
                    <p>Failed to load conversation history</p>
                </div>
            `;
        }
    }

    renderMessage(msg) {
        const timestamp = msg.timestamp ? new Date(msg.timestamp).toLocaleTimeString() : '';
        const role = msg.type || 'unknown';

        // Extract content from message.content array
        let content = '';
        if (msg.message && msg.message.content && Array.isArray(msg.message.content)) {
            // Extract text from content blocks
            content = msg.message.content
                .filter(block => block.type === 'text' || block.text)
                .map(block => block.text || block.thinking || '')
                .join('\n');
        } else if (msg.content) {
            content = msg.content;
        }

        content = this.escapeHtml(content || '(no content)');

        let toolCallsHtml = '';
        if (msg.tool_calls && msg.tool_calls.length > 0) {
            toolCallsHtml = msg.tool_calls.map(tool => `
                <div class="tool-call">
                    <span class="tool-name">🔧 ${tool.name}</span>
                </div>
            `).join('');
        }

        return `
            <div class="message message-${role}">
                <div class="message-header">
                    <span class="message-role">${role === 'user' ? 'User' : 'Assistant'}</span>
                    <span class="message-time">${timestamp}</span>
                </div>
                <div class="message-content">${content}</div>
                ${toolCallsHtml}
            </div>
        `;
    }

    async loadNotifications() {
        try {
            const response = await fetch('/api/notifications');
            const notifications = await response.json();

            const container = document.getElementById('notifications-list');

            // Update badge
            const badge = document.getElementById('notif-badge');
            if (notifications.length > 0) {
                badge.textContent = notifications.length;
                badge.classList.remove('hidden');
            } else {
                badge.classList.add('hidden');
            }

            if (notifications.length === 0) {
                container.innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">✅</div>
                        <p>No notifications</p>
                    </div>
                `;
                return;
            }

            container.innerHTML = notifications.map(notif => `
                <div class="notification-card ${notif.notif_type}">
                    <div class="notification-header">
                        <span class="notification-type">${notif.notif_type.replace('_', ' ')}</span>
                        <span class="notification-time">${this.formatTime(notif.timestamp)}</span>
                    </div>
                    <div class="notification-message">${this.escapeHtml(notif.message)}</div>
                    <div class="notification-agent">Agent: ${notif.agent_id}</div>
                    <button class="btn-consume" onclick="app.consumeNotification('${notif.agent_id}', '${notif.notif_type}', '${notif.timestamp}')">
                        Mark as Read
                    </button>
                </div>
            `).join('');

            this.updateLastUpdate();
        } catch (error) {
            console.error('Failed to load notifications:', error);
            document.getElementById('notifications-list').innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">⚠️</div>
                    <p>Failed to load notifications</p>
                </div>
            `;
        }
    }

    async consumeNotification(agentId, notifType, timestamp) {
        try {
            const response = await fetch('/api/notifications/consume', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    agent_id: agentId,
                    notif_type: notifType,
                    timestamp: timestamp,
                }),
            });

            if (response.ok) {
                // Reload notifications
                this.loadNotifications();
            } else {
                console.error('Failed to consume notification');
            }
        } catch (error) {
            console.error('Failed to consume notification:', error);
        }
    }

    async loadServers() {
        try {
            const response = await fetch('/api/servers');
            const servers = await response.json();

            const container = document.getElementById('servers-list');

            if (servers.length === 0) {
                container.innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">🖥️</div>
                        <p>No servers found</p>
                    </div>
                `;
                return;
            }

            container.innerHTML = servers.map(server => `
                <div class="server-card">
                    <div class="server-header">
                        <div class="server-name">${server.name}</div>
                        <div class="server-status">${server.status}</div>
                    </div>
                    <div class="server-meta">
                        <div>Host: ${server.host}</div>
                        <div>Agents: ${server.agent_count}</div>
                    </div>
                </div>
            `).join('');

            this.updateLastUpdate();
        } catch (error) {
            console.error('Failed to load servers:', error);
            document.getElementById('servers-list').innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">⚠️</div>
                    <p>Failed to load servers</p>
                </div>
            `;
        }
    }

    formatTime(timestamp) {
        if (!timestamp) return 'Never';
        const date = new Date(timestamp);
        return date.toLocaleString();
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize app
const app = new WarrenApp();
