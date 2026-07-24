# Database Design — Persistence Boundary

## Status

- Phase 3.1 architecture: accepted on 2026-07-23
- Phase 3.2 schema design: accepted on 2026-07-24
- Phase 3.2 benchmark and measured schema refinement: accepted on 2026-07-24
- Current milestone: Phase 3.3 migration framework
- PostgreSQL: planned first adapter; disposable benchmark connection only
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

The accepted Phase 3.2 physical contract is
[POSTGRESQL_SCHEMA_SPECIFICATION.md](POSTGRESQL_SCHEMA_SPECIFICATION.md). Its
isolated, authorized measurement plan is
[POSTGRESQL_PAYLOAD_BENCHMARK_PLAN.md](POSTGRESQL_PAYLOAD_BENCHMARK_PLAN.md).
ADR 0011 records the proposed schema decision. No executable database artifact
has been created.

The isolated benchmark evidence is recorded in
[POSTGRESQL_PAYLOAD_BENCHMARK_REPORT.md](../Validation/POSTGRESQL_PAYLOAD_BENCHMARK_REPORT.md).
It freezes four-MiB ordered chunks, a four-GiB operational limit, and the
eight-GiB schema ceiling. Phase 3.3 migration implementation is authorized.

Qdrant remains a future rebuildable retrieval index, never a source of truth.
