# Database Design — Persistence Boundary

## Status

- Phase 3.1 architecture: accepted on 2026-07-23
- Current milestone: Phase 3.2 schema specification (design only)
- PostgreSQL: planned first adapter; not connected
- Schema/SQL/migrations: not implemented
- Credentials: not required

PostgreSQL will sit downstream of immutable versioned artifacts. Intelligence
engines remain database-free.

The authoritative durable value is the exact serialized artifact payload plus
its envelope, digest, version, provenance, and dependency edges. Relational
metadata, `JSONB`, diagnostics, statistics, and future search structures are
rebuildable query projections.

The conceptual persistence model contains repositories, scans, artifact
envelopes, exact payloads, artifact dependencies, projections, and audit events.
It intentionally does not create source-of-truth tables for Go functions,
methods, variables, or other language internals.

The complete Phase 3.1 design is
[PERSISTENCE_BOUNDARY.md](../Architecture/PERSISTENCE_BOUNDARY.md). The staged
delivery plan is
[PHASE_3_PERSISTENCE_ROADMAP.md](../Roadmap/PHASE_3_PERSISTENCE_ROADMAP.md).

Qdrant remains a future rebuildable retrieval index, never a source of truth.
