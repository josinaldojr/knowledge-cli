# MCP Performance Budgets

These budgets are local development targets, not CI pass/fail thresholds. Run
the named Go benchmarks on a representative developer machine after a relevant
change. Results vary with filesystem, CPU, SQLite version, and concurrent MCP
processes.

| Benchmark | Fixed scope | Budget |
| --- | --- | --- |
| `BenchmarkMCPStartup` | In-process stdio server construction and one `initialize` request | under 100 ms |
| `BenchmarkBeforeHookRetrieval` | One before-apply hook against a migrated local SQLite store with empty memory | under 100 ms |
| `BenchmarkAfterHookDeltaProcessing` | Structural delta for 100 requirements, with 10 modified bodies | under 50 ms |
| `BenchmarkSQLiteContention` | Four-way parallel short SQLite write transactions in one MCP process | under 250 ms per operation |
| `BenchmarkSnapshotStorage` | Store one unique 256 KiB OpenSpec snapshot | under 100 ms and no more than 256 KiB of content per unique snapshot |

The production snapshot policy rejects artifacts larger than 1 MiB. Snapshot
contents are SHA-256 addressed, so recording an already-stored artifact must
not consume additional content storage. The size budget excludes SQLite
metadata and filesystem allocation overhead.
