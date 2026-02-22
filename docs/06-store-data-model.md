# 06 - Store Layer and Data Model

The store layer abstracts all persistence behind Go interfaces, allowing the same core engine to run with file-based storage (standalone mode) or PostgreSQL (managed mode). Each store interface has independent implementations, and the system determines which backend to use based on configuration at startup.

---

## 1. Store Layer Routing

```mermaid
flowchart TD
    START["Gateway Startup"] --> CHECK{"StoreConfig.IsManaged()?<br/>(DSN + mode = managed)"}
    CHECK -->|Yes| PG["PostgreSQL Backend"]
    CHECK -->|No| FILE["File Backend"]

    PG --> PG_STORES["PGSessionStore<br/>PGAgentStore<br/>PGProviderStore<br/>PGCronStore<br/>PGPairingStore<br/>PGSkillStore<br/>PGMemoryStore<br/>PGTracingStore<br/>PGMCPServerStore<br/>PGCustomToolStore"]

    FILE --> FILE_STORES["FileSessionStore<br/>FileMemoryStore (SQLite + FTS5)<br/>FileCronStore<br/>FilePairingStore<br/>FileSkillStore<br/>AgentStore = nil<br/>ProviderStore = nil<br/>TracingStore = nil<br/>MCPServerStore = nil<br/>CustomToolStore = nil"]
```

---

## 2. Store Interface Map

The `Stores` struct is the top-level container holding all storage backends. In standalone mode, managed-only stores are `nil`.

| Interface | Standalone Implementation | Managed Implementation | Mode |
|-----------|--------------------------|------------------------|------|
| SessionStore | `FileSessionStore` via `sessions.Manager` | `PGSessionStore` | Both |
| MemoryStore | `FileMemoryStore` (SQLite + FTS5 + embeddings) | `PGMemoryStore` (tsvector + pgvector) | Both |
| CronStore | `FileCronStore` | `PGCronStore` | Both |
| PairingStore | `FilePairingStore` via `pairing.Service` | `PGPairingStore` | Both |
| SkillStore | `FileSkillStore` via `skills.Loader` | `PGSkillStore` | Both |
| AgentStore | `nil` | `PGAgentStore` | Managed only |
| ProviderStore | `nil` | `PGProviderStore` | Managed only |
| TracingStore | `nil` | `PGTracingStore` | Managed only |
| MCPServerStore | `nil` | `PGMCPServerStore` | Managed only |
| CustomToolStore | `nil` | `PGCustomToolStore` | Managed only |

---

## 3. Session Caching

The session store uses an in-memory write-behind cache to minimize database I/O during the agent tool loop. All reads and writes happen in memory; data is flushed to the persistent backend only when `Save()` is called at the end of a run.

```mermaid
flowchart TD
    subgraph "In-Memory Cache (map + mutex)"
        ADD["AddMessage()"] --> CACHE["Session Cache"]
        SET["SetSummary()"] --> CACHE
        ACC["AccumulateTokens()"] --> CACHE
        CACHE --> GET["GetHistory()"]
        CACHE --> GETSM["GetSummary()"]
    end

    CACHE -->|"Save(key)"| DB[("PostgreSQL / JSON file")]
    DB -->|"Cache miss via GetOrCreate"| CACHE
```

### Lifecycle

1. **GetOrCreate(key)**: Check cache; on miss, load from DB into cache; return session data.
2. **AddMessage/SetSummary/AccumulateTokens**: Update in-memory cache only (no DB write).
3. **Save(key)**: Snapshot data under read lock, flush to DB via UPDATE.
4. **Delete(key)**: Remove from both cache and DB. `List()` always reads directly from DB.

### Session Key Format

| Type | Format | Example |
|------|--------|---------|
| DM | `agent:{agentId}:{channel}:direct:{peerId}` | `agent:default:telegram:direct:386246614` |
| Group | `agent:{agentId}:{channel}:group:{groupId}` | `agent:default:telegram:group:-100123456` |
| Subagent | `agent:{agentId}:subagent:{label}` | `agent:default:subagent:my-task` |
| Cron | `agent:{agentId}:cron:{jobId}:run:{runId}` | `agent:default:cron:reminder:run:abc123` |
| Main | `agent:{agentId}:{mainKey}` | `agent:default:main` |

### File-Based Persistence (Standalone)

- Startup: `loadAll()` reads all `.json` files into memory
- Save: temp file + rename (atomic write, prevents corruption on crash)
- Filename: session key with `:` replaced by `_`, plus `.json` extension

---

## 4. Agent Access Control

In managed mode, agent access is checked via a 4-step pipeline.

```mermaid
flowchart TD
    REQ["CanAccess(agentID, userID)"] --> S1{"Agent exists?"}
    S1 -->|No| DENY["Deny"]
    S1 -->|Yes| S2{"is_default = true?"}
    S2 -->|Yes| ALLOW["Allow<br/>(role = owner if owner,<br/>user otherwise)"]
    S2 -->|No| S3{"owner_id = userID?"}
    S3 -->|Yes| ALLOW_OWNER["Allow (role = owner)"]
    S3 -->|No| S4{"Record in agent_shares?"}
    S4 -->|Yes| ALLOW_SHARE["Allow (role from share)"]
    S4 -->|No| DENY
```

The `agent_shares` table stores `UNIQUE(agent_id, user_id)` with roles: `user`, `admin`, `operator`.

`ListAccessible(userID)` queries: `owner_id = ? OR is_default = true OR id IN (SELECT agent_id FROM agent_shares WHERE user_id = ?)`.

---

## 5. API Key Encryption

API keys in the `llm_providers` and `mcp_servers` tables are encrypted with AES-256-GCM before storage.

```mermaid
flowchart LR
    subgraph "Storing a key"
        PLAIN["Plaintext API key"] --> ENC["AES-256-GCM encrypt"]
        ENC --> DB["DB: 'aes-gcm:' + base64(nonce + ciphertext + tag)"]
    end

    subgraph "Loading a key"
        DB2["DB value"] --> CHECK{"Has 'aes-gcm:' prefix?"}
        CHECK -->|Yes| DEC["AES-256-GCM decrypt"]
        CHECK -->|No| RAW["Return as-is<br/>(backward compatibility)"]
        DEC --> USE["Plaintext key"]
        RAW --> USE
    end
```

`GOCLAW_ENCRYPTION_KEY` accepts three formats:
- **Hex**: 64 characters (decoded to 32 bytes)
- **Base64**: 44 characters (decoded to 32 bytes)
- **Raw**: 32 characters (32 bytes direct)

---

## 6. Hybrid Memory Search

Memory search combines full-text search (FTS) and vector similarity in a weighted merge.

```mermaid
flowchart TD
    QUERY["Search(query, agentID, userID)"] --> PAR

    subgraph PAR["Parallel Search"]
        FTS["FTS Search<br/>tsvector + plainto_tsquery<br/>Weight: 0.3"]
        VEC["Vector Search<br/>pgvector cosine distance<br/>Weight: 0.7"]
    end

    FTS --> MERGE["hybridMerge()"]
    VEC --> MERGE
    MERGE --> BOOST["Per-user scope: 1.2x boost<br/>Dedup: user copy wins over global"]
    BOOST --> FILTER["Min score filter<br/>+ max results limit"]
    FILTER --> RESULT["Sorted results"]
```

### Merge Rules

1. Normalize FTS scores to [0, 1] (divide by highest score)
2. Vector scores already in [0, 1] (cosine similarity)
3. Combined score: `vec_score * 0.7 + fts_score * 0.3` for chunks found by both
4. When only one channel returns results, its weight auto-adjusts to 1.0
5. Per-user results receive a 1.2x boost
6. Deduplication: if a chunk exists in both global and per-user scope, the per-user version wins

### Fallback

When FTS returns no results (e.g., cross-language queries), a `likeSearch()` fallback runs ILIKE queries using up to 5 keywords (minimum 3 characters each), scoped to the agent's index.

### Standalone vs Managed

| Aspect | Standalone | Managed |
|--------|-----------|---------|
| FTS engine | SQLite FTS5 | PostgreSQL tsvector |
| Vector | Embedding cache | pgvector extension |
| Search function | `plainto_tsquery('simple', ...)` | Same |
| Distance operator | N/A | `<=>` (cosine) |

---

## 7. Context Files Routing

Context files are stored in two tables and routed based on agent type.

### Tables

| Table | Scope | Unique Key |
|-------|-------|------------|
| `agent_context_files` | Agent-level | `(agent_id, file_name)` |
| `user_context_files` | Per-user | `(agent_id, user_id, file_name)` |

### Routing by Agent Type

| Agent Type | Agent-Level Files | Per-User Files |
|------------|-------------------|----------------|
| `open` | Template fallback only | All 7 files (SOUL, IDENTITY, AGENTS, TOOLS, HEARTBEAT, BOOTSTRAP, USER) |
| `predefined` | 6 files (SOUL, IDENTITY, AGENTS, TOOLS, HEARTBEAT, BOOTSTRAP) | Only USER.md |

The `ContextFileInterceptor` checks agent type from context and routes read/write operations accordingly. For open agents, per-user files take priority with agent-level as fallback.

---

## 8. MCP Server Store

The MCP server store manages external tool server configurations and access grants.

### Tables

| Table | Purpose |
|-------|---------|
| `mcp_servers` | Server configurations (name, transport, command/URL, encrypted API key) |
| `mcp_agent_grants` | Per-agent access grants with tool allow/deny lists |
| `mcp_user_grants` | Per-user access grants with tool allow/deny lists |
| `mcp_access_requests` | Pending/approved/rejected access requests |

### Transport Types

| Transport | Fields Used |
|-----------|-------------|
| `stdio` | `command`, `args` (JSONB), `env` (JSONB) |
| `sse` | `url`, `headers` (JSONB) |
| `streamable-http` | `url`, `headers` (JSONB) |

`ListAccessible(agentID, userID)` returns all MCP servers the given agent+user combination can access, with effective tool allow/deny lists merged from both agent and user grants.

---

## 9. Custom Tool Store

Dynamic tool definitions stored in PostgreSQL. Each tool defines a shell command template that the LLM can invoke at runtime.

### Table: `custom_tools`

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID v7 | Primary key |
| `name` | VARCHAR | Unique tool name |
| `description` | TEXT | Tool description for the LLM |
| `parameters` | JSONB | JSON Schema for tool arguments |
| `command` | TEXT | Shell command template with `{{.key}}` placeholders |
| `working_dir` | VARCHAR | Optional working directory |
| `timeout_seconds` | INT | Execution timeout (default 60) |
| `env` | BYTEA | Encrypted environment variables (AES-256-GCM) |
| `agent_id` | UUID | `NULL` = global tool, UUID = per-agent tool |
| `enabled` | BOOLEAN | Soft enable/disable |
| `created_by` | VARCHAR | Audit trail |

**Scoping**: Global tools (`agent_id IS NULL`) are loaded at startup into the global registry. Per-agent tools are loaded on-demand when the agent is resolved, using a cloned registry to avoid polluting the global one.

---

## 10. Database Schema

All tables use UUID v7 (time-ordered) as primary keys via `GenNewID()`.

```mermaid
flowchart TD
    subgraph Providers
        LP["llm_providers"] --> LM["llm_models"]
    end

    subgraph Agents
        AG["agents"] --> AS["agent_shares"]
        AG --> ACF["agent_context_files"]
        AG --> UCF["user_context_files"]
        AG --> UAP["user_agent_profiles"]
    end

    subgraph Sessions
        SE["sessions"]
    end

    subgraph Memory
        MD["memory_documents"] --> MC["memory_chunks"]
    end

    subgraph Cron
        CJ["cron_jobs"] --> CRL["cron_run_logs"]
    end

    subgraph Pairing
        PR["pairing_requests"]
        PD["paired_devices"]
    end

    subgraph Skills
        SK["skills"] --> SAG["skill_agent_grants"]
        SK --> SUG["skill_user_grants"]
    end

    subgraph Tracing
        TR["traces"] --> SP["spans"]
    end

    subgraph MCP
        MS["mcp_servers"] --> MAG["mcp_agent_grants"]
        MS --> MUG["mcp_user_grants"]
        MS --> MAR["mcp_access_requests"]
    end

    subgraph "Custom Tools"
        CT["custom_tools"]
    end
```

### Key Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `agents` | Agent definitions | `agent_key` (UNIQUE), `owner_id`, `agent_type` (open/predefined), `is_default`, soft delete via `deleted_at` |
| `agent_shares` | Agent RBAC sharing | UNIQUE(agent_id, user_id), `role` (user/admin/operator) |
| `agent_context_files` | Agent-level context | UNIQUE(agent_id, file_name) |
| `user_context_files` | Per-user context | UNIQUE(agent_id, user_id, file_name) |
| `user_agent_profiles` | User tracking | `first_seen_at`, `last_seen_at`, `workspace` |
| `sessions` | Conversation history | `session_key` (UNIQUE), `messages` (JSONB), `summary`, token counts |
| `memory_documents` | Memory docs | UNIQUE(agent_id, COALESCE(user_id, ''), path) |
| `memory_chunks` | Chunked + embedded text | `embedding` (VECTOR), `tsv` (TSVECTOR) |
| `llm_providers` | Provider configuration | `api_key` (AES-256-GCM encrypted) |
| `traces` | LLM call traces | `agent_id`, `user_id`, `status`, aggregated token counts |
| `spans` | Individual operations | `span_type` (llm_call, tool_call, agent, embedding), `parent_span_id` |
| `skills` | Skill definitions | Content, metadata, grants |
| `cron_jobs` | Scheduled tasks | `schedule_kind` (at/every/cron), `payload` (JSONB) |
| `mcp_servers` | MCP server configs | `transport`, `api_key` (encrypted), `tool_prefix` |
| `custom_tools` | Dynamic tool definitions | `command` (template), `agent_id` (NULL = global), `env` (encrypted) |

### Required PostgreSQL Extensions

- **pgvector**: Vector similarity search for memory embeddings
- **pgcrypto**: UUID generation functions

---

## 11. Context Propagation

Metadata flows through `context.Context` instead of mutable state, ensuring thread safety across concurrent agent runs.

```mermaid
flowchart TD
    HANDLER["HTTP/WS Handler"] -->|"store.WithUserID(ctx)<br/>store.WithAgentID(ctx)<br/>store.WithAgentType(ctx)"| LOOP["Agent Loop"]
    LOOP -->|"tools.WithToolChannel(ctx)<br/>tools.WithToolChatID(ctx)<br/>tools.WithToolPeerKind(ctx)"| TOOL["Tool Execute(ctx)"]
    TOOL -->|"store.UserIDFromContext(ctx)<br/>store.AgentIDFromContext(ctx)<br/>tools.ToolChannelFromCtx(ctx)"| LOGIC["Domain Logic"]
```

### Store Context Keys

| Key | Type | Purpose |
|-----|------|---------|
| `goclaw_user_id` | string | External user ID (e.g., Telegram user ID) |
| `goclaw_agent_id` | uuid.UUID | Agent UUID (managed mode) |
| `goclaw_agent_type` | string | Agent type: `"open"` or `"predefined"` |

### Tool Context Keys

| Key | Purpose |
|-----|---------|
| `tool_channel` | Current channel (telegram, discord, etc.) |
| `tool_chat_id` | Chat/conversation identifier |
| `tool_peer_kind` | Peer type: `"direct"` or `"group"` |
| `tool_sandbox_key` | Docker sandbox scope key |
| `tool_async_cb` | Callback for async tool execution |

---

## 12. Key PostgreSQL Patterns

### Database Driver

All PG stores use `database/sql` with the `pgx/v5/stdlib` driver. No ORM is used -- all queries are raw SQL with positional parameters (`$1`, `$2`, ...).

### Nullable Columns

Nullable columns are handled via Go pointers: `*string`, `*int`, `*time.Time`, `*uuid.UUID`. Helper functions `nilStr()`, `nilInt()`, `nilUUID()`, `nilTime()` convert zero values to `nil` for clean SQL insertion.

### Dynamic Updates

`execMapUpdate()` builds UPDATE statements dynamically from a `map[string]any` of column-value pairs. This avoids writing a separate UPDATE query for every combination of updatable fields.

### Upsert Pattern

All "create or update" operations use `INSERT ... ON CONFLICT DO UPDATE`, ensuring idempotency:

| Operation | Conflict Key |
|-----------|-------------|
| `SetAgentContextFile` | `(agent_id, file_name)` |
| `SetUserContextFile` | `(agent_id, user_id, file_name)` |
| `ShareAgent` | `(agent_id, user_id)` |
| `PutDocument` (memory) | `(agent_id, COALESCE(user_id, ''), path)` |
| `GrantToAgent` (skill) | `(skill_id, agent_id)` |

### User Profile Detection

`GetOrCreateUserProfile` uses the PostgreSQL `xmax` trick:
- `xmax = 0` after RETURNING means a real INSERT occurred (new user) -- triggers context file seeding
- `xmax != 0` means an UPDATE on conflict (existing user) -- no seeding needed

### Batch Span Insert

`BatchCreateSpans` inserts spans in batches of 100. If a batch fails, it falls back to inserting each span individually to prevent data loss.

---

## File Reference

| File | Purpose |
|------|---------|
| `internal/store/stores.go` | `Stores` container struct (all 9 store interfaces) |
| `internal/store/types.go` | `BaseModel`, `StoreConfig`, `GenNewID()` |
| `internal/store/context.go` | Context propagation: `WithUserID`, `WithAgentID`, `WithAgentType` |
| `internal/store/session_store.go` | `SessionStore` interface, `SessionData`, `SessionInfo` |
| `internal/store/memory_store.go` | `MemoryStore` interface, `MemorySearchResult`, `EmbeddingProvider` |
| `internal/store/skill_store.go` | `SkillStore` interface |
| `internal/store/agent_store.go` | `AgentStore` interface |
| `internal/store/provider_store.go` | `ProviderStore` interface |
| `internal/store/tracing_store.go` | `TracingStore` interface, `TraceData`, `SpanData` |
| `internal/store/mcp_store.go` | `MCPServerStore` interface, grant types, access request types |
| `internal/store/pairing_store.go` | `PairingStore` interface |
| `internal/store/cron_store.go` | `CronStore` interface |
| `internal/store/custom_tool_store.go` | `CustomToolStore` interface |
| `internal/store/pg/factory.go` | PG store factory: creates all PG store instances from a connection pool |
| `internal/store/pg/sessions.go` | `PGSessionStore`: session cache, Save, GetOrCreate |
| `internal/store/pg/agents.go` | `PGAgentStore`: CRUD, soft delete, access control |
| `internal/store/pg/agents_context.go` | Agent and user context file operations |
| `internal/store/pg/memory_docs.go` | `PGMemoryStore`: document CRUD, indexing, chunking |
| `internal/store/pg/memory_search.go` | Hybrid search: FTS, vector, ILIKE fallback, merge |
| `internal/store/pg/skills.go` | `PGSkillStore`: skill CRUD and grants |
| `internal/store/pg/skills_grants.go` | Skill agent and user grants |
| `internal/store/pg/mcp_servers.go` | `PGMCPServerStore`: server CRUD, grants, access requests |
| `internal/store/pg/custom_tools.go` | `PGCustomToolStore`: custom tool CRUD with encrypted env |
| `internal/store/pg/providers.go` | `PGProviderStore`: provider CRUD with encrypted keys |
| `internal/store/pg/tracing.go` | `PGTracingStore`: traces and spans with batch insert |
| `internal/store/pg/pool.go` | Connection pool management |
| `internal/store/pg/helpers.go` | Nullable helpers, JSON helpers, `execMapUpdate()` |
| `internal/store/validate.go` | Input validation utilities |
