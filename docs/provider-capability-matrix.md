# Provider capability matrix

**Matrix version:** 1.0

This matrix defines the support boundary for KV's provider adapters. It is
deliberately conservative: an adapter may only use a lifecycle signal marked
as supported here. Provider API changes require an update to this document and
the adapter contract fixtures before the signal is enabled.

| Provider | Supported configuration | Native session identity | Lifecycle signal used by KV | Delivery failure behavior | Adapter level |
| --- | --- | --- | --- | --- | --- |
| OpenCode | Global `opencode.json` MCP server entry using stdio | Use the provider session ID when supplied by a lifecycle/plugin callback | Adapter callback when available; otherwise first `kv_hook` call | Do not block provider work; persist the envelope to the KV spool and retry | **L1 — lazy** until a stable lifecycle callback is verified for the installed OpenCode version |
| Claude Code | Manual `~/.claude/settings.json` `mcpServers.kv` stdio entry; no KV install/uninstall command | `CLAUDE_SESSION_ID` when supplied to a hook | `SessionStart`, `SessionEnd`, and OpenSpec wrapper hooks | Hook exits successfully after queuing a recordable event; diagnostics report degraded assistance | **L2 — native lifecycle** when the hook supplies `CLAUDE_SESSION_ID`; otherwise stable correlation fallback |
| Codex | Manual `~/.codex/config.toml` `[mcp_servers.kv]` stdio entry; no KV CLI install/uninstall command | No native session ID is assumed | First `kv_hook` call and OpenSpec wrapper hooks | Do not block Codex work; persist to the KV spool and retry | **L1 — lazy** |

## Fallback levels

- **L2 — native lifecycle:** the adapter receives a documented lifecycle event
  and a provider-native session identifier. It forwards both unchanged to the
  provider-neutral hook contract.
- **L1 — lazy:** the provider may configure and call MCP, but KV does not rely
  on an unverified lifecycle callback. The adapter generates one stable,
  session-scoped correlation ID and `EnsureSession` runs on the first hook.
- **L0 — unavailable:** MCP/hook delivery is unavailable. Engineering work
  continues without context; recordable events remain in the local spool for a
  later retry.

## Contract rules

1. Adapters send only metadata needed by `HookEnvelope`; transcripts and
   chain-of-thought are never included.
2. Every delivery has an event ID and idempotency key. Retried deliveries use
   the same values.
3. A missing native session ID is valid only when the adapter supplies a stable
   correlation ID for the provider session.
4. Provider-specific fields remain in the adapter. The MCP/domain boundary is
   limited to the versioned envelope defined by KV.
5. Configuration merging, installation, and doctor checks are adapter work;
   they must not overwrite unrelated user configuration.

## Revalidation policy

Revalidate this matrix for each supported provider release. A provider's
lifecycle integration may be promoted from L1 to L2 only after an automated
fixture proves session correlation, lifecycle delivery, retry behavior, and
graceful degradation.

## Storage dependency decision

KV uses `modernc.org/sqlite` v1.56.0. It is BSD-3-Clause licensed, provides a
CGo-free SQLite driver, documents macOS, Linux, and Windows builds, and exposes
SQLite backup primitives. KV enables WAL and creates a private pre-migration
backup before applying a pending schema version. The module is pinned through
`go.mod` and tested through the normal Go module build, preserving a
single-binary deployment.
