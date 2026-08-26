# Automatic MCP engineering memory

KV can run as a stdio MCP server (`kv mcp`) and records provider-session and
OpenSpec lifecycle metadata without requiring `kv init` or creating `.kv`
inside an observed repository.

## Installation and compatibility

Install the `kv` binary on `PATH` with `go install ./cmd/kv` (this module requires Go 1.26.3 or later). OpenCode has the supported automatic installer:

```bash
kv opencode install
kv mcp doctor --provider opencode
```

It merges a local `kv mcp` server into `~/.config/opencode/opencode.json` without overwriting existing MCP entries. `kv opencode uninstall` removes only that entry.

Claude Code and Codex adapters are available, but this release has no CLI installer or uninstaller for either provider. Configure their MCP entry manually to invoke `kv mcp`, preserving user configuration, then run the provider doctor:

```bash
kv mcp doctor --provider claude-code
kv mcp doctor --provider codex
```

Claude Code is checked at `~/.claude/settings.json` for `mcpServers.kv` and can forward `CLAUDE_SESSION_ID` from a hook. Codex is checked at `~/.codex/config.toml` for `[mcp_servers.kv]` and uses a generated, process-lifetime correlation ID. See the [provider capability matrix](provider-capability-matrix.md) for lifecycle fallback levels and configuration boundaries.

## Global storage

KV keeps its operational state outside workspaces with private directory/file
permissions (`0700` directories and `0600` database, snapshot, and spool
files):

| Platform | Data directory | Cache directory |
| --- | --- | --- |
| macOS | `~/Library/Application Support/kv` | `~/Library/Caches/kv` |
| Linux and other Unix | `${XDG_DATA_HOME:-~/.local/share}/kv` | `${XDG_CACHE_HOME:-~/.cache}/kv` |
| Windows | `${LOCALAPPDATA:-~/AppData/Local}/kv` | `<data>/cache` |

`KV_DATA_HOME` and `KV_CACHE_HOME` override these locations. The data directory
contains `kv.db`, content-addressed `snapshots/`, and the retry `spool/`.

## Data that KV records

- logical workspace and local checkout identities;
- provider kind, native or generated correlation session ID, lifecycle times,
  and status;
- OpenSpec change/operation correlation, manifests, revision hashes, verified
  transformations, evidence-linked memory, and advisory diagnostics;
- queued lifecycle envelopes only until acknowledged by KV.

KV does **not** persist provider transcripts, chain of thought, arbitrary
conversation, or non-OpenSpec workspace files.

## Snapshots, redaction, and retention

Only artifact paths resolved by OpenSpec are snapshot eligible. The current MCP
server applies a 1 MiB per-artifact limit and rejects paths or content rejected
by a configured redaction policy. Snapshot contents are deduplicated by
SHA-256. The snapshot store exposes a configurable age-based garbage-collection
policy and can protect hashes needed by pending operations; it deletes only
expired content, preserving manifests and all relational genealogy. User-facing
retention configuration and redaction policy controls are still planned.

SQLite migrations create a pre-migration backup. Existing backups and global
state are not uploaded or shared by KV.

## Administration and troubleshooting

Before hooks may return evidence-linked engineering context, but provider work
continues if KV is unavailable. Recordable events are queued locally for retry.
An archive hook attempts to consolidate memory; an incomplete consolidation is
reported as degraded assistance and does not rewrite or block OpenSpec.

```bash
kv mcp status                         # inspect storage and queued event count
kv mcp doctor --provider all          # check binaries and MCP configuration
kv events retry                       # retry due spooled events
kv mcp reconcile --timeout 24h        # reconcile stale sessions and operations
kv workspace status                   # inspect the current global workspace
kv session list                       # inspect its provider sessions
kv change history                     # inspect OpenSpec history
kv memory search "query"               # search current workspace memory
```

`binary=false` in a doctor report means `kv` is not discoverable on `PATH`. `mcp_configured=false` means the provider configuration has no KV MCP entry. Correct the configuration or run `kv opencode install`, then retry pending events. The doctor checks local prerequisites; it does not start an interactive provider session.

KV can export/import readable, versioned session metadata and evidence-linked OpenSpec memory, without native provider IDs, absolute paths, transcripts, or snapshot contents:

```bash
kv data export --file ./kv-knowledge.json
kv data import --file ./kv-knowledge.json
```

Exports are workspace-scoped and refuse to overwrite an existing file. A normal filesystem backup of the global data directory remains appropriate for complete operational recovery, including the database and snapshot contents.

## Limitations

- MCP requests and each eligible OpenSpec snapshot are limited to 1 MiB.
- Only OpenSpec-resolved artifacts are eligible for snapshots; arbitrary code changes and provider conversation do not become engineering memory.
- Retrieval is local lexical search and is workspace-isolated by default. KV does not synchronize memory between machines or users.
- Lifecycle coverage varies by provider. OpenCode and Codex use lazy fallback when no verified native callback is available; Claude Code installation is manual in this release.
- Diagnostics and archive consolidation are advisory. KV never edits or blocks OpenSpec artifacts on its own.

## Legacy migration and deprecation

The global MCP store is a new, separate source of derived OpenSpec memory. It
does not read, write, import, or promote legacy project-local `.kv` state
implicitly. Existing session commands, workflow commands, task memory drafts,
and direct Vault promotion remain available in this release.

Choose the migration path that matches the work:

- **Keep the legacy workflow:** continue using project-local sessions,
  workflows, task drafts, and their existing Vault promotion. This is the
  appropriate choice when the work is not represented in OpenSpec or when a
  team needs its current process unchanged.
- **Adopt MCP memory for new OpenSpec work:** install the provider adapter and
  let MCP hooks record verified OpenSpec transformations in the global store.
  Legacy `.kv` workflows can continue in the same repository, but their state
  remains separate from MCP memory.
- **Run both deliberately:** use legacy task drafts as planning material, then
  materialize the approved outcome in OpenSpec. MCP records only the resulting
  OpenSpec artifact transformations; it does not treat a draft or provider
  discussion as verified memory.
- **Promote to the Vault explicitly:** retain the existing direct Vault
  promotion path for workflows that require curated Markdown knowledge. MCP
  consolidation is not a replacement for that promotion and does not write to
  a Vault.

There is no scheduled removal of these legacy capabilities. A future
deprecation requires a separate approved change and release documentation, and
may begin only when all of the following are true:

- the replacement covers the documented legacy use case without requiring
  project-local `.kv` initialization;
- an explicit, validated migration or export path preserves the user-selected
  legacy data and leaves the original data intact;
- coexistence and rollback instructions have been tested, including continued
  access to task drafts and direct Vault promotion;
- provider support, privacy boundaries, retention behavior, and operational
  diagnostics are documented for the replacement; and
- a published compatibility window gives users a supported opt-in path before
  any command, file format, or promotion behavior is removed.

Until those criteria are met, legacy state remains user-owned and must not be
deleted, rewritten, or silently merged into the global MCP store.
