# Dependency Intelligence neutral core

This candidate `0.1.0` package implements Phase 5.0.2 only: immutable neutral
graph models, canonical stable identities, deterministic normalization and
duplicate aggregation, bounded evidence/diagnostics/edges, and a hard
`MaxNodes` safety gate.

It does not import RIE, Go LIE, persistence, PostgreSQL, runtime, Repository
Service, transports, filesystem readers, network clients, or process execution.
SCC/cycle and impact computation, the Go adapter, integration, and release are
separately gated future milestones.

Containment IDs use the frozen ordered fields `kind`, `parent node ID`, and
`child node ID` with the common uint64-big-endian length-prefixed UTF-8 scheme.
