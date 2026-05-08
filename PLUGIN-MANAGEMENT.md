# Warren - Plugin Management

## The Challenge

Each Claude Code agent session may have:
- Different plugins installed
- Different plugin versions
- Different plugin enable/disable states
- Different plugin configurations

Warren needs to provide **central management** of plugins across all remote agent sessions.

## Plugin Management Requirements

### 1. Plugin Discovery

Warren needs to discover what plugins are available on each remote server.

**Claude Code plugin locations:**
```
~/.claude/plugins/          # User-installed plugins
/usr/local/share/claude/plugins/  # System-wide plugins (if any)
```

**Plugin structure:**
```
~/.claude/plugins/
├── plugin-name/
│   ├── plugin.json         # Plugin manifest
│   ├── commands/           # Slash commands
│   ├── agents/             # Subagents
│   ├── skills/             # Skills
│   └── hooks/              # Hooks
└── another-plugin/
    └── ...
```

**Discovery process:**
```python
def discover_plugins(server: str) -> List[Plugin]:
    """
    SSH to server and scan for plugins
    """
    # List plugin directories
    plugin_dirs = ssh_exec(server, "ls -1 ~/.claude/plugins/")
    
    plugins = []
    for plugin_dir in plugin_dirs:
        # Read plugin.json
        manifest = ssh_exec(server, f"cat ~/.claude/plugins/{plugin_dir}/plugin.json")
        plugin_info = json.loads(manifest)
        
        plugins.append(Plugin(
            name=plugin_info['name'],
            version=plugin_info['version'],
            description=plugin_info['description'],
            server=server,
            path=f"~/.claude/plugins/{plugin_dir}",
            enabled=check_if_enabled(server, plugin_info['name'])
        ))
    
    return plugins
```

### 2. Plugin State Tracking

Warren maintains a registry of plugins across all servers.

**Data model:**
```python
@dataclass
class Plugin:
    name: str
    version: str
    description: str
    server: str
    path: str
    enabled: bool
    
    # Components
    commands: List[str]      # Slash commands provided
    agents: List[str]        # Subagents provided
    skills: List[str]        # Skills provided
    hooks: List[str]         # Hooks provided
    
    # Configuration
    config: Dict[str, Any]   # Plugin-specific config
    
@dataclass
class AgentPluginState:
    """Plugin state for a specific agent session"""
    agent_name: str
    server: str
    plugins: Dict[str, PluginStatus]  # plugin_name -> status
    
@dataclass
class PluginStatus:
    name: str
    enabled: bool
    version: str
    config: Dict[str, Any]
```

**Plugin registry** (`~/.warren/plugins.json`):
```json
{
  "servers": {
    "prod-1": {
      "plugins": [
        {
          "name": "code-review",
          "version": "1.2.0",
          "enabled": true,
          "path": "~/.claude/plugins/code-review"
        },
        {
          "name": "database-tools",
          "version": "0.5.0",
          "enabled": false,
          "path": "~/.claude/plugins/database-tools"
        }
      ]
    },
    "prod-2": {
      "plugins": [
        {
          "name": "code-review",
          "version": "1.1.0",
          "enabled": true,
          "path": "~/.claude/plugins/code-review"
        }
      ]
    }
  },
  "agents": {
    "backend-api": {
      "server": "prod-1",
      "plugins": {
        "code-review": {
          "enabled": true,
          "config": {
            "auto_review": false
          }
        },
        "database-tools": {
          "enabled": false
        }
      }
    }
  }
}
```

### 3. Plugin Enable/Disable

Warren can enable or disable plugins for specific agent sessions.

**How Claude Code manages plugin state:**

Claude Code likely uses one of these approaches:
- Settings file: `~/.claude/settings.json` with `plugins.enabled` list
- Per-session config: Session-specific plugin state
- Plugin-specific config: Each plugin has enabled flag

**Warren's approach:**

```python
def enable_plugin(agent_name: str, plugin_name: str):
    """Enable a plugin for an agent session"""
    agent = get_agent(agent_name)
    server = agent.server
    
    # Method 1: Modify Claude Code settings
    ssh_exec(server, f"""
        jq '.plugins.enabled += ["{plugin_name}"]' ~/.claude/settings.json > tmp.json
        mv tmp.json ~/.claude/settings.json
    """)
    
    # Method 2: Send command to agent session
    send_to_agent(agent_name, f"/plugin enable {plugin_name}")
    
    # Update Warren's registry
    update_plugin_state(agent_name, plugin_name, enabled=True)

def disable_plugin(agent_name: str, plugin_name: str):
    """Disable a plugin for an agent session"""
    agent = get_agent(agent_name)
    server = agent.server
    
    # Method 1: Modify Claude Code settings
    ssh_exec(server, f"""
        jq '.plugins.enabled -= ["{plugin_name}"]' ~/.claude/settings.json > tmp.json
        mv tmp.json ~/.claude/settings.json
    """)
    
    # Method 2: Send command to agent session
    send_to_agent(agent_name, f"/plugin disable {plugin_name}")
    
    # Update Warren's registry
    update_plugin_state(agent_name, plugin_name, enabled=False)
```

### 4. Plugin Installation & Updates

Warren can install plugins on remote servers and keep them updated.

**Install plugin:**
```python
def install_plugin(server: str, plugin_source: str):
    """
    Install a plugin on a remote server
    
    plugin_source can be:
    - Git URL: https://github.com/user/plugin.git
    - Local path: /path/to/plugin
    - Plugin registry: plugin-name@version
    """
    
    if plugin_source.startswith('http'):
        # Clone from git
        ssh_exec(server, f"""
            cd ~/.claude/plugins
            git clone {plugin_source}
        """)
    elif plugin_source.startswith('/'):
        # Copy from local
        scp_upload(plugin_source, server, "~/.claude/plugins/")
    else:
        # Install from registry (future)
        install_from_registry(server, plugin_source)
    
    # Discover the new plugin
    plugins = discover_plugins(server)
    update_plugin_registry(server, plugins)

def update_plugin(server: str, plugin_name: str):
    """Update a plugin to latest version"""
    plugin = get_plugin(server, plugin_name)
    
    if is_git_repo(server, plugin.path):
        # Pull latest from git
        ssh_exec(server, f"""
            cd {plugin.path}
            git pull
        """)
    
    # Reload plugin in active sessions
    for agent in get_agents_on_server(server):
        if is_plugin_enabled(agent, plugin_name):
            reload_plugin(agent, plugin_name)
```

**Sync plugins across servers:**
```python
def sync_plugins(source_server: str, target_servers: List[str], plugin_name: str):
    """
    Sync a plugin from source server to target servers
    Useful for keeping plugin versions consistent
    """
    source_plugin = get_plugin(source_server, plugin_name)
    
    for target in target_servers:
        # Check if plugin exists on target
        target_plugin = get_plugin(target, plugin_name)
        
        if not target_plugin:
            # Install plugin
            install_plugin(target, source_plugin.path)
        elif target_plugin.version != source_plugin.version:
            # Update plugin
            update_plugin(target, plugin_name)
```

### 5. Plugin Configuration

Warren can manage plugin-specific configuration.

**Plugin config structure:**
```yaml
# ~/.warren/plugin-configs/code-review.yaml
name: code-review

# Global defaults
defaults:
  auto_review: false
  review_depth: standard
  
# Server-specific overrides
servers:
  prod-1:
    auto_review: true
    review_depth: thorough
  staging:
    auto_review: false
    
# Agent-specific overrides
agents:
  backend-api:
    auto_review: true
    review_checklist:
      - security
      - performance
      - tests
```

**Apply configuration:**
```python
def apply_plugin_config(agent_name: str, plugin_name: str):
    """Apply plugin configuration to an agent"""
    config = load_plugin_config(plugin_name)
    agent = get_agent(agent_name)
    
    # Merge configs: defaults -> server -> agent
    final_config = merge_configs(
        config.defaults,
        config.servers.get(agent.server, {}),
        config.agents.get(agent_name, {})
    )
    
    # Write config to remote
    config_path = f"~/.claude/plugins/{plugin_name}/config.local.yaml"
    ssh_exec(agent.server, f"cat > {config_path}", input=yaml.dump(final_config))
    
    # Reload plugin if running
    if is_agent_running(agent_name):
        reload_plugin(agent_name, plugin_name)
```

## Warren Hub - Plugin Management UI

### Plugin Overview

```
┌─ Warren Hub - Plugins ──────────────────────────────────────┐
│                                                              │
│  Servers (3)                                                 │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ prod-1 (5 plugins)                                     │ │
│  │   ✓ code-review      v1.2.0                           │ │
│  │   ✓ database-tools   v0.5.0                           │ │
│  │   ✓ deployment       v2.1.0                           │ │
│  │   ✗ testing-utils    v1.0.0  (disabled)               │ │
│  │   ⚠ security-scan    v0.3.0  (outdated)               │ │
│  │                                                        │ │
│  │ prod-2 (3 plugins)                                     │ │
│  │   ✓ code-review      v1.1.0  ⚠ (older version)        │ │
│  │   ✓ database-tools   v0.5.0                           │ │
│  │   ✗ deployment       v2.0.0  (disabled)               │ │
│  │                                                        │ │
│  │ gpu-box (2 plugins)                                    │ │
│  │   ✓ ml-tools         v1.5.0                           │ │
│  │   ✓ data-processing  v0.8.0                           │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  [Install Plugin] [Sync Across Servers] [Update All]        │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### Agent Plugin View

```
┌─ Agent: backend-api - Plugins ──────────────────────────────┐
│                                                              │
│  Server: prod-1                                              │
│                                                              │
│  Active Plugins (3)                                          │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ ✓ code-review        v1.2.0                           │ │
│  │   Commands: /review, /code-review                     │ │
│  │   Config: auto_review=true, depth=thorough            │ │
│  │   [Configure] [Disable]                               │ │
│  │                                                        │ │
│  │ ✓ database-tools     v0.5.0                           │ │
│  │   Commands: /db, /query, /migrate                     │ │
│  │   Config: default_db=postgres                         │ │
│  │   [Configure] [Disable]                               │ │
│  │                                                        │ │
│  │ ✓ deployment         v2.1.0                           │ │
│  │   Commands: /deploy, /rollback                        │ │
│  │   Config: env=production, auto_deploy=false           │ │
│  │   [Configure] [Disable]                               │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Available Plugins (2)                                       │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ ✗ testing-utils      v1.0.0                           │ │
│  │   Commands: /test, /coverage                          │ │
│  │   [Enable]                                            │ │
│  │                                                        │ │
│  │ ✗ security-scan      v0.3.0  ⚠ Update available      │ │
│  │   Commands: /scan, /audit                             │ │
│  │   [Enable] [Update]                                   │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  [Install New Plugin]                                        │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### Plugin Configuration UI

```
┌─ Configure Plugin: code-review ─────────────────────────────┐
│                                                              │
│  Agent: backend-api                                          │
│  Server: prod-1                                              │
│                                                              │
│  Configuration:                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ auto_review:     [✓] Enable automatic reviews         │ │
│  │                                                        │ │
│  │ review_depth:    ○ Quick                              │ │
│  │                  ● Standard                            │ │
│  │                  ○ Thorough                            │ │
│  │                                                        │ │
│  │ review_checklist:                                      │ │
│  │   [✓] Security                                        │ │
│  │   [✓] Performance                                     │ │
│  │   [✓] Tests                                           │ │
│  │   [ ] Documentation                                   │ │
│  │   [ ] Style                                           │ │
│  │                                                        │ │
│  │ max_file_size:   [1000] KB                            │ │
│  │                                                        │ │
│  │ exclude_paths:                                         │ │
│  │   node_modules/                                       │ │
│  │   dist/                                               │ │
│  │   [+ Add Path]                                        │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Apply to:                                                   │
│  ○ This agent only                                           │
│  ○ All agents on prod-1                                      │
│  ○ All agents everywhere                                     │
│                                                              │
│  [Save] [Cancel] [Reset to Defaults]                        │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## Warren CLI - Plugin Commands

```bash
# List plugins
warren plugins list                          # All plugins across all servers
warren plugins list --server prod-1          # Plugins on specific server
warren plugins list --agent backend-api      # Plugins for specific agent

# Plugin info
warren plugins info code-review              # Show plugin details
warren plugins info code-review --server prod-1

# Enable/disable
warren plugins enable code-review --agent backend-api
warren plugins disable code-review --agent backend-api
warren plugins enable code-review --server prod-1  # Enable for all agents on server

# Install/update
warren plugins install https://github.com/user/plugin.git --server prod-1
warren plugins install ./local-plugin --server prod-1
warren plugins update code-review --server prod-1
warren plugins update --all --server prod-1  # Update all plugins

# Sync across servers
warren plugins sync code-review --from prod-1 --to prod-2,staging
warren plugins sync --all --from prod-1 --to prod-2  # Sync all plugins

# Configuration
warren plugins config code-review --agent backend-api
warren plugins config code-review --set auto_review=true
warren plugins config code-review --edit  # Open in editor

# Troubleshooting
warren plugins doctor                        # Check for issues
warren plugins doctor --agent backend-api    # Check specific agent
warren plugins reload code-review --agent backend-api  # Reload plugin
```

## Plugin Version Management

Warren tracks plugin versions and can help keep them consistent.

**Version comparison:**
```
┌─ Plugin Version Report ─────────────────────────────────────┐
│                                                              │
│  code-review:                                                │
│    prod-1:   v1.2.0  ✓ Latest                               │
│    prod-2:   v1.1.0  ⚠ Outdated (update available)          │
│    staging:  v1.2.0  ✓ Latest                               │
│                                                              │
│  database-tools:                                             │
│    prod-1:   v0.5.0  ✓ Consistent                           │
│    prod-2:   v0.5.0  ✓ Consistent                           │
│                                                              │
│  deployment:                                                 │
│    prod-1:   v2.1.0  ✓ Latest                               │
│    prod-2:   v2.0.0  ⚠ Outdated                             │
│    staging:  Not installed                                   │
│                                                              │
│  [Update All Outdated] [Sync Versions]                       │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## Plugin Dependency Management

Some plugins may depend on others or have conflicts.

**Plugin manifest with dependencies:**
```json
{
  "name": "advanced-code-review",
  "version": "2.0.0",
  "dependencies": {
    "code-review": ">=1.2.0",
    "testing-utils": ">=1.0.0"
  },
  "conflicts": {
    "legacy-review": "*"
  }
}
```

**Warren checks dependencies:**
```python
def check_plugin_dependencies(server: str, plugin_name: str) -> List[str]:
    """Check if plugin dependencies are satisfied"""
    plugin = get_plugin(server, plugin_name)
    issues = []
    
    for dep_name, version_req in plugin.dependencies.items():
        dep_plugin = get_plugin(server, dep_name)
        
        if not dep_plugin:
            issues.append(f"Missing dependency: {dep_name}")
        elif not version_matches(dep_plugin.version, version_req):
            issues.append(f"Incompatible version: {dep_name} {dep_plugin.version} (need {version_req})")
    
    for conflict_name, version_pattern in plugin.conflicts.items():
        conflict_plugin = get_plugin(server, conflict_name)
        
        if conflict_plugin and version_matches(conflict_plugin.version, version_pattern):
            issues.append(f"Conflict: {conflict_name} {conflict_plugin.version}")
    
    return issues
```

## Open Questions

1. **Plugin reload**: How to reload plugins in running agent sessions?
   - Restart agent?
   - Hot reload if Claude Code supports it?
   - Send command to session?

2. **Plugin registry**: Should Warren have a central plugin registry?
   - Discover and install plugins easily
   - Version management
   - Security/trust model

3. **Plugin conflicts**: How to handle conflicting plugins?
   - Prevent installation?
   - Warn user?
   - Allow but disable conflicting features?

4. **Plugin state persistence**: Where to store plugin enable/disable state?
   - Claude Code settings?
   - Warren's registry?
   - Both (sync)?

5. **Plugin updates**: How to handle breaking changes?
   - Automatic updates safe?
   - Require manual approval?
   - Rollback mechanism?

6. **Custom plugins**: How to develop and test plugins via Warren?
   - Local development workflow
   - Deploy to test server
   - Promote to production
