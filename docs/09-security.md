# 09 - Security

Defense-in-depth with five independent layers from transport to isolation. Each layer operates independently -- even if one layer is bypassed, the remaining layers continue to protect the system.

> **Managed mode**: Adds AES-256-GCM encryption for secrets stored in PostgreSQL (LLM provider API keys, MCP server API keys, custom tool environment variables), plus agent-level access control via the 4-step `CanAccess` pipeline (see [06-store-data-model.md](./06-store-data-model.md)).

---

## 1. Five Defense Layers

```mermaid
flowchart TD
    REQ["Request"] --> L1["Layer 1: Transport<br/>CORS, message size limits, timing-safe auth"]
    L1 --> L2["Layer 2: Input<br/>Injection detection (6 patterns), message truncation"]
    L2 --> L3["Layer 3: Tool<br/>Shell deny patterns, path traversal, SSRF, exec approval"]
    L3 --> L4["Layer 4: Output<br/>Credential scrubbing, content wrapping"]
    L4 --> L5["Layer 5: Isolation<br/>Docker sandbox, read-only root FS, network restrictions"]
```

### Layer 1: Transport Security

| Mechanism | Detail |
|-----------|--------|
| CORS (WebSocket) | `checkOrigin()` validates against `allowed_origins` (empty = allow all for backward compatibility) |
| WS message limit | `SetReadLimit(512KB)` -- gorilla auto-closes connection on exceed |
| HTTP body limit | `MaxBytesReader(1MB)` -- error returned before JSON decode |
| Token auth | `crypto/subtle.ConstantTimeCompare` (timing-safe) |
| Rate limiting | Token bucket per user/IP, configurable via `rate_limit_rpm` |

### Layer 2: Input -- Injection Detection

The input guard scans for 6 injection patterns.

| Pattern | Detection Target |
|---------|-----------------|
| `ignore_instructions` | "ignore all previous instructions" |
| `role_override` | "you are now...", "pretend you are..." |
| `system_tags` | `<system>`, `[SYSTEM]`, `[INST]`, `<<SYS>>` |
| `instruction_injection` | "new instructions:", "override:", "system prompt:" |
| `null_bytes` | Null characters `\x00` (obfuscation attempts) |
| `delimiter_escape` | "end of system", `</instructions>`, `</prompt>` |

**Configurable action** (`gateway.injection_action`):

| Value | Behavior |
|-------|----------|
| `"log"` | Log info level, continue processing |
| `"warn"` (default) | Log warning level, continue processing |
| `"block"` | Log warning, return error, stop processing |
| `"off"` | Disable detection entirely |

**Message truncation**: Messages exceeding `max_message_chars` (default 32K) are truncated (not rejected), and the LLM is notified of the truncation.

### Layer 3: Tool Security

**Shell deny patterns** -- 7 categories of blocked commands:

| Category | Examples |
|----------|----------|
| Destructive file ops | `rm -rf`, `del /f`, `rmdir /s` |
| Destructive disk ops | `mkfs`, `dd if=`, `> /dev/sd*` |
| System commands | `shutdown`, `reboot`, `poweroff` |
| Fork bombs | `:(){ ... };:` |
| Remote code execution | `curl \| sh`, `wget -O - \| sh` |
| Reverse shells | `/dev/tcp/`, `nc -e` |
| Eval injection | `eval $()`, `base64 -d \| sh` |

**SSRF protection** -- 3-step validation:

```mermaid
flowchart TD
    URL["URL to fetch"] --> S1["Step 1: Check blocked hostnames<br/>localhost, *.local, *.internal,<br/>metadata.google.internal"]
    S1 --> S2["Step 2: Check private IP ranges<br/>10.0.0.0/8, 172.16.0.0/12,<br/>192.168.0.0/16, 127.0.0.0/8,<br/>169.254.0.0/16, IPv6 loopback/link-local"]
    S2 --> S3["Step 3: DNS Pinning<br/>Resolve domain, check every resolved IP.<br/>Also applied to redirect targets."]
    S3 --> ALLOW["Allow request"]
```

**Path traversal**: `resolvePath()` applies `filepath.Clean()` then `HasPrefix()` to ensure all paths stay within the workspace. With `restrict = true`, any path outside the workspace is blocked.

### Layer 4: Output Security

| Mechanism | Detail |
|-----------|--------|
| Credential scrubbing | Regex detection of: OpenAI (`sk-...`), Anthropic (`sk-ant-...`), GitHub (`ghp_/gho_/ghu_/ghs_/ghr_`), AWS (`AKIA...`), generic key-value patterns. All replaced with `[REDACTED]`. |
| Web content wrapping | Fetched content wrapped in `<<<EXTERNAL_UNTRUSTED_CONTENT>>>` tags with security warning |

### Layer 5: Isolation (Docker Sandbox)

| Hardening | Configuration |
|-----------|---------------|
| Read-only root FS | `--read-only` |
| Drop all capabilities | `--cap-drop ALL` |
| No new privileges | `--security-opt no-new-privileges` |
| Memory limit | 512 MB |
| CPU limit | 1.0 |
| PID limit | Enabled |
| Network disabled | `--network none` |
| Tmpfs mounts | `/tmp`, `/var/tmp`, `/run` |
| Output limit | 1 MB |
| Timeout | 300 seconds |

---

## 2. Encryption (Managed Mode)

AES-256-GCM encryption for secrets stored in PostgreSQL. Key provided via `GOCLAW_ENCRYPTION_KEY` environment variable.

| What's Encrypted | Table | Column |
|-----------------|-------|--------|
| LLM provider API keys | `llm_providers` | `api_key` |
| MCP server API keys | `mcp_servers` | `api_key` |
| Custom tool env vars | `custom_tools` | `env` |

**Format**: `"aes-gcm:" + base64(12-byte nonce + ciphertext + GCM tag)`

Backward compatible: values without the `aes-gcm:` prefix are returned as plaintext (for migration from unencrypted data).

---

## 3. Rate Limiting -- Gateway + Tool

Protection at two levels: gateway-wide (per user/IP) and tool-level (per session).

```mermaid
flowchart TD
    subgraph "Gateway Level"
        GW_REQ["Request"] --> GW_CHECK{"rate_limit_rpm > 0?"}
        GW_CHECK -->|No| GW_PASS["Allow all"]
        GW_CHECK -->|Yes| GW_BUCKET{"Token bucket<br/>has capacity?"}
        GW_BUCKET -->|Available| GW_ALLOW["Allow + consume token"]
        GW_BUCKET -->|Exhausted| GW_REJECT["WS: INVALID_REQUEST error<br/>HTTP: 429 + Retry-After header"]
    end

    subgraph "Tool Level"
        TL_REQ["Tool call"] --> TL_CHECK{"Entries in<br/>last 1 hour?"}
        TL_CHECK -->|">= maxPerHour"| TL_REJECT["Error: rate limit exceeded"]
        TL_CHECK -->|"< maxPerHour"| TL_ALLOW["Record + allow"]
    end
```

| Level | Algorithm | Key | Burst | Cleanup |
|-------|-----------|-----|:-----:|---------|
| Gateway | Token bucket | user/IP | 5 | Every 5 min (inactive > 10 min) |
| Tool | Sliding window | `agent:userID` | N/A | Manual `Cleanup()` |

Gateway rate limiting applies to both WebSocket (`chat.send`) and HTTP (`/v1/chat/completions`) chat endpoints. Config: `gateway.rate_limit_rpm` (0 = disabled, any positive value = enabled).

---

## 4. RBAC -- 3 Roles

Role-based access control for WebSocket RPC methods and HTTP API endpoints. Roles are hierarchical: higher levels include all permissions of lower levels.

```mermaid
flowchart LR
    V["Viewer (level 1)<br/>Read-only access"] --> O["Operator (level 2)<br/>Read + Write"]
    O --> A["Admin (level 3)<br/>Full control"]
```

| Role | Key Permissions |
|------|----------------|
| Viewer | agents.list, config.get, sessions.list, health, status, skills.list |
| Operator | + chat.send, chat.abort, sessions.delete/reset, cron.*, skills.update |
| Admin | + config.apply/patch, agents.create/update/delete, channels.toggle, device.pair.approve/revoke |

### Access Check Flow

```mermaid
flowchart TD
    REQ["Method call"] --> S1["Step 1: MethodRole(method)<br/>Determine minimum required role"]
    S1 --> S2{"Step 2: roleLevel(user) >= roleLevel(required)?"}
    S2 -->|Yes| ALLOW["Allow"]
    S2 -->|No| DENY["Deny"]
    S2 --> S3["Step 3 (optional):<br/>CanAccessWithScopes() for tokens<br/>with narrow scope restrictions"]
```

Token-based role assignment happens during the WebSocket `connect` handshake. Scopes include: `operator.admin`, `operator.read`, `operator.write`, `operator.approvals`, `operator.pairing`.

---

## 5. Sandbox -- Container Lifecycle

Docker-based code isolation for shell command execution.

```mermaid
flowchart TD
    REQ["Exec request"] --> CHECK{"ShouldSandbox?"}
    CHECK -->|off| HOST["Execute on host<br/>timeout: 60s"]
    CHECK -->|non-main / all| SCOPE["ResolveScopeKey()"]
    SCOPE --> GET["DockerManager.Get(scopeKey)"]
    GET --> EXISTS{"Container exists?"}
    EXISTS -->|Yes| REUSE["Reuse existing container"]
    EXISTS -->|No| CREATE["docker run -d<br/>+ security flags<br/>+ resource limits<br/>+ workspace mount"]
    REUSE --> EXEC["docker exec sh -c [cmd]<br/>timeout: 300s"]
    CREATE --> EXEC
    EXEC --> RESULT["ExecResult{ExitCode, Stdout, Stderr}"]
```

### Sandbox Modes

| Mode | Behavior |
|------|----------|
| `off` (default) | Execute directly on host |
| `non-main` | Sandbox all agents except main/default |
| `all` | Sandbox every agent |

### Container Scope

| Scope | Reuse Level | Scope Key |
|-------|-------------|-----------|
| `session` (default) | One container per session | sessionKey |
| `agent` | Shared across sessions for the same agent | `"agent:" + agentID` |
| `shared` | One container for all agents | `"shared"` |

### Workspace Access

| Mode | Mount |
|------|-------|
| `none` | No workspace access |
| `ro` | Read-only mount |
| `rw` | Read-write mount |

### Auto-Pruning

| Parameter | Default | Action |
|-----------|---------|--------|
| `idle_hours` | 24 | Remove containers idle for more than 24 hours |
| `max_age_days` | 7 | Remove containers older than 7 days |
| `prune_interval_min` | 5 | Check every 5 minutes |

### FsBridge -- File Operations in Sandbox

| Operation | Docker Command |
|-----------|---------------|
| ReadFile | `docker exec [id] cat -- [path]` |
| WriteFile | `docker exec -i [id] sh -c 'cat > [path]'` |
| ListDir | `docker exec [id] ls -la -- [path]` |
| Stat | `docker exec [id] stat -- [path]` |

---

## 6. Security Logging Convention

All security events use `slog.Warn` with a `security.*` prefix for consistent filtering and alerting.

| Event | Meaning |
|-------|---------|
| `security.injection_detected` | Prompt injection pattern detected |
| `security.injection_blocked` | Message blocked due to injection (when action = block) |
| `security.rate_limited` | Request rejected due to rate limit |
| `security.cors_rejected` | WebSocket connection rejected due to CORS policy |
| `security.message_truncated` | Message truncated because it exceeded the size limit |

Filter all security events by grepping for the `security.` prefix in log output.

---

## File Reference

| File | Description |
|------|-------------|
| `internal/agent/input_guard.go` | Injection pattern detection (6 patterns) |
| `internal/tools/scrub.go` | Credential scrubbing (regex-based redaction) |
| `internal/tools/shell.go` | Shell deny patterns, command validation |
| `internal/tools/web_fetch.go` | Web content wrapping, SSRF protection |
| `internal/permissions/policy.go` | RBAC (3 roles, scope-based access) |
| `internal/gateway/ratelimit.go` | Gateway-level token bucket rate limiter |
| `internal/sandbox/` | Docker sandbox manager, FsBridge |
| `internal/crypto/aes.go` | AES-256-GCM encrypt/decrypt |

---

## Cross-References

| Document | Relevant Content |
|----------|-----------------|
| [03-tools-system.md](./03-tools-system.md) | Shell deny patterns, exec approval, policy engine |
| [04-gateway-protocol.md](./04-gateway-protocol.md) | WebSocket auth, RBAC, rate limiting |
| [06-store-data-model.md](./06-store-data-model.md) | API key encryption, agent access control pipeline |
| [08-scheduling-cron-heartbeat.md](./08-scheduling-cron-heartbeat.md) | Scheduler lanes, cron lifecycle |
| [10-tracing-observability.md](./10-tracing-observability.md) | Tracing and OTel export |
