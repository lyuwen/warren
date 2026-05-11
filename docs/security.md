# Warren Security Documentation

## Overview

Warren is designed as a **localhost-only development tool** for monitoring AI agent sessions. This document outlines the current security posture, deployment requirements, and future roadmap.

## Current Security Model

### Deployment Requirements

**⚠️ CRITICAL: Warren web interface MUST only be deployed on localhost**

- **Bind address:** `127.0.0.1:8080` or `localhost:8080` only
- **Never expose to network:** Do not bind to `0.0.0.0` or public interfaces
- **No authentication:** Current version has no authentication mechanism
- **No encryption:** HTTP only (not HTTPS)

### Why Localhost-Only?

Warren currently has **no authentication or authorization**. Anyone who can access the web interface can:
- View all agent sessions
- See file paths and activity
- Read notification messages
- Access artifact profiles

**This is acceptable for localhost-only deployment** because:
1. Only the local user can access localhost
2. The user already has full access to their own tmux sessions
3. Warren provides read-only monitoring (no control capabilities)

### Current Security Features

✅ **WebSocket Origin Validation**
- Only accepts connections from localhost origins
- Rejects cross-origin WebSocket connections
- Prevents CSRF attacks via WebSocket

✅ **Read-Only Operations**
- Warren only monitors; it cannot control agents
- No write operations to agent sessions
- No command execution capabilities

✅ **Local File Access Only**
- Reads from local SQLite database
- No remote data sources
- No external API calls

### Current Security Limitations

🔴 **No Authentication**
- Anyone with localhost access can use the interface
- No user login or session management
- No API keys or tokens

🔴 **No HTTPS**
- Traffic is unencrypted (acceptable for localhost)
- Would be critical if exposed to network

🟡 **No CSRF Protection**
- Not critical for read-only operations
- Would be needed if write operations are added

🟡 **No Rate Limiting**
- Could allow local DoS
- Not critical for single-user localhost deployment

🟡 **No Input Validation**
- Limited user input in current version
- Should be added before network deployment

## Deployment Guidelines

### ✅ Safe Deployment (Localhost Only)

```bash
# Start Warren web interface (safe)
warren-web --bind 127.0.0.1:8080

# Or using localhost
warren-web --bind localhost:8080
```

### ❌ UNSAFE Deployment (Network Exposed)

```bash
# DO NOT DO THIS - Exposes to network without authentication
warren-web --bind 0.0.0.0:8080

# DO NOT DO THIS - Exposes to network
warren-web --bind 192.168.1.100:8080
```

### Docker Deployment

If running Warren in Docker:

```yaml
# Safe: Only expose to localhost
ports:
  - "127.0.0.1:8080:8080"

# UNSAFE: Exposes to network
ports:
  - "8080:8080"  # DO NOT USE
```

## Security Roadmap

### Phase 3: Network Deployment Support

Before Warren can be safely deployed on a network, the following must be implemented:

#### High Priority (Blockers for Network Deployment)

1. **Authentication & Authorization**
   - User login system
   - Session management
   - Role-based access control (RBAC)
   - API key authentication for programmatic access

2. **HTTPS/TLS Support**
   - TLS certificate management
   - Automatic HTTPS redirect
   - Secure WebSocket (WSS)

3. **CSRF Protection**
   - CSRF tokens for state-changing operations
   - SameSite cookie attributes

4. **Input Validation & Sanitization**
   - Validate all user inputs
   - Sanitize data before display
   - Prevent XSS attacks

#### Medium Priority (Hardening)

5. **Rate Limiting**
   - Per-IP rate limits
   - Per-user rate limits
   - Prevent brute force attacks

6. **Audit Logging**
   - Log all access attempts
   - Log authentication events
   - Log configuration changes

7. **Security Headers**
   - Content-Security-Policy
   - X-Frame-Options
   - X-Content-Type-Options
   - Strict-Transport-Security (HSTS)

#### Low Priority (Enhancements)

8. **Multi-factor Authentication (MFA)**
9. **IP Whitelisting**
10. **Intrusion Detection**

## Threat Model

### Current Threats (Localhost Deployment)

| Threat | Likelihood | Impact | Mitigation |
|--------|-----------|--------|------------|
| Malicious local process | Low | Medium | OS-level security |
| Local privilege escalation | Low | High | OS-level security |
| Data exfiltration via localhost | Low | Low | Read-only data |

### Future Threats (Network Deployment)

| Threat | Likelihood | Impact | Mitigation Required |
|--------|-----------|--------|---------------------|
| Unauthorized access | High | High | Authentication |
| Man-in-the-middle | High | High | HTTPS/TLS |
| CSRF attacks | Medium | Medium | CSRF tokens |
| XSS attacks | Medium | High | Input validation |
| Brute force | Medium | Medium | Rate limiting |
| Session hijacking | Medium | High | Secure sessions |

## Incident Response

### If Warren is Accidentally Exposed to Network

1. **Immediately stop the Warren web server**
   ```bash
   pkill warren-web
   ```

2. **Check for unauthorized access**
   ```bash
   # Check Warren logs
   tail -f ~/.warren/logs/web.log
   
   # Check system logs
   journalctl -u warren-web
   ```

3. **Rotate any sensitive data**
   - Change passwords for any systems monitored by agents
   - Review agent activity logs for suspicious behavior

4. **Restart with localhost binding**
   ```bash
   warren-web --bind 127.0.0.1:8080
   ```

## Security Best Practices

### For Users

1. **Always bind to localhost**
   - Use `127.0.0.1` or `localhost` only
   - Never use `0.0.0.0` or public IPs

2. **Keep Warren updated**
   - Security fixes will be released as updates
   - Subscribe to security announcements

3. **Limit access to Warren database**
   ```bash
   chmod 600 ~/.warren/warren.db
   ```

4. **Review agent activity regularly**
   - Check for unexpected file access
   - Monitor for unusual patterns

### For Developers

1. **Never commit secrets**
   - No API keys in code
   - No passwords in configuration

2. **Validate all inputs**
   - Even for localhost-only features
   - Defense in depth

3. **Follow secure coding practices**
   - Use parameterized queries
   - Sanitize outputs
   - Handle errors securely

4. **Security review before network features**
   - Any feature that enables network deployment requires security review
   - Authentication must be implemented first

## Reporting Security Issues

If you discover a security vulnerability in Warren:

1. **Do NOT open a public GitHub issue**
2. **Email security concerns to:** [security contact to be added]
3. **Include:**
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

## Compliance

### Current Status

Warren is currently **not suitable for**:
- Production environments
- Multi-user deployments
- Network-accessible deployments
- Environments with compliance requirements (HIPAA, SOC2, etc.)

Warren is **suitable for**:
- Local development environments
- Single-user localhost deployments
- Personal productivity tools
- Development and testing

### Future Compliance Goals

Phase 3+ will address:
- SOC2 compliance requirements
- GDPR data protection requirements
- Industry-specific compliance (as needed)

## Conclusion

Warren's current security model is **appropriate for its intended use case**: a localhost-only development tool for monitoring AI agent sessions.

**Before deploying Warren on a network, authentication and HTTPS must be implemented.**

The security roadmap in Phase 3 will enable safe network deployment with proper authentication, encryption, and hardening.

---

**Last Updated:** 2026-05-11  
**Version:** Phase 2 (Localhost-Only)  
**Next Review:** Phase 3 Planning
