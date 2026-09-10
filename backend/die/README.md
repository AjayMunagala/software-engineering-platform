# Dependency Intelligence neutral core

This candidate `0.1.0` package includes the accepted Phase 5.0.2 immutable neutral
graph models, canonical stable identities, deterministic normalization and
duplicate aggregation, bounded evidence/diagnostics/edges, and a hard
`MaxNodes` safety gate.

It does not import RIE, Go LIE, persistence, PostgreSQL, runtime, Repository
Service, transports, filesystem readers, network clients, or process execution.
The accepted Go adapter remains in the separate `die/golang` package.
Phase 5.0.4 adds opt-in SCC/cycle and bounded query capabilities; implementation
acceptance is pending. Integration and release remain separately gated.

Containment IDs use the frozen ordered fields `kind`, `parent node ID`, and
`child node ID` with the common uint64-big-endian length-prefixed UTF-8 scheme.

## Opt-in analysis (Phase 5.0.4 candidate)

Construct `AnalysisConfig` with `NewAnalysisConfig`, then `NewAnalyzer` or
`NewQueryEngine`. Supply an immutable `DependencyInventory`; `Core.Normalize`
never invokes algorithms automatically. Each call builds a fresh private index.

`Analyze` returns a new inventory containing iterative SCCs, one structural cycle
summary per cyclic SCC, and detached analysis metadata. Direct dependency/dependent
pages use request- and fingerprint-bound cursors. `Impact` performs bounded forward
or reverse BFS, returning explicit depth/node/edge truncation reasons.

Only resolved-local same-kind edges join SCCs or local traversal. Containment,
cross-kind, external, stale, and ambiguous edges do not prove local reachability.
Partial topology never proves global acyclicity or exhaustive change impact.
Neighbor `Resolution` describes the neighbor node; edge uncertainty is retained in
the input edge records and the graph's topology-limited metadata.

Zero numeric configuration/query limits select documented defaults. Reducing input
caps requires explicitly reducing dependent caps; no silent clamping. Invalid
input/SCC caps fail with zero output, whereas traversal caps produce partial
results. Nil/canceled/deadline contexts fail without partial output.

Independent digest/cursor vectors were frozen at `ab4628c` before production
encoding. `c0fdd00` corrects only the domain-length prose. Existing SCC/cycle vectors
and core/adapter golden bytes are unchanged. Count limits are not byte-memory
ceilings; `DIE-HARDEN-001` and `DIE-HARDEN-002` remain open for the accepted core.

Validation: `go test ./die/...`, `go test -race ./die/...`, and the report at
`docs/Validation/DEPENDENCY_GRAPH_ANALYSIS_VALIDATION_REPORT.md`.
