# Phase 3 Persistence Roadmap

## Status

- Current milestone: Phase 3.3 — Migration Framework
- Current authorization: Phase 3.3 migration implementation only
- PostgreSQL connection: disposable local migration/test databases only
- Credentials: not required and must not be uploaded
- Transient benchmark DDL/harness: accepted evidence
- Migration implementation: authorized
- Go storage adapter, APIs, and UI: unauthorized

## Goal

Durably preserve immutable, versioned intelligence artifacts behind a
storage-neutral boundary without making any engine depend on PostgreSQL.

## Delivery Rules

1. Every milestone has an explicit engineering acceptance gate.
2. Later milestones remain unauthorized until the current gate is accepted.
3. Engines and artifact packages never import storage packages.
4. Exact artifact bytes remain authoritative; relational indexes are
   rebuildable projections.
5. No credential value is committed, documented, logged, or pasted into chat.
6. No API or UI begins before persistence correctness and recovery are proven.

## Phase 3.1 — Persistence Architecture

### Deliverables

- persistence responsibility and non-responsibility;
- artifact ownership and dependency direction;
- conceptual repository, scan, artifact, payload, dependency, projection, and
  audit models;
- artifact/codec/projector/database version rules;
- transaction, atomic publication, idempotency, and concurrency rules;
- repository lifecycle and retention boundaries;
- indexing principles;
- security and privacy model;
- backup, restore, and integrity-verification strategy;
- storage-neutral port responsibilities;
- ADR 0010 and the staged Phase 3 roadmap.

### Exit Gate

Engineering accepts `PERSISTENCE_BOUNDARY.md` and ADR 0010. Acceptance
authorizes Phase 3.2 only.

Accepted on 2026-07-23. Phase 3.2 schema specification is authorized.

## Phase 3.2 — PostgreSQL Schema Specification

Design the physical PostgreSQL model without connecting to a database.

### Current State

- Physical schema specification: accepted on 2026-07-24.
- ADR 0011 and four-MiB amendment: accepted.
- Payload benchmark plan: accepted; execution completed on 2026-07-24.
- Benchmark report: accepted on 2026-07-24.
- Frozen contract: four-MiB chunks and a four-GiB operational limit.
- Disposable local installation, connection, and transient benchmark DDL:
  authorized only for the documented spike.
- Credentials and Go persistence adapter code: unauthorized.

### Required Decisions

- supported PostgreSQL baseline and extensions;
- exact table, column, type, nullability, key, and constraint definitions;
- exact-payload byte representation and maximum size;
- repository/scan lifecycle constraints;
- artifact envelope and content-addressed payload model;
- dependency-edge integrity;
- projection schema and projector versioning;
- audit schema;
- indexes justified by approved queries;
- partitioning decision based on measured scale;
- role/privilege matrix;
- retention and purge dependency rules;
- schema naming and migration ownership.

### Required Spike

Measure storage, WAL, write/read latency, and peak client memory for released
RIE, Go Language, Package Identity, OpenTelemetry, and Kubernetes semantic
payloads. A disposable isolated PostgreSQL instance may be proposed only after
the schema design review explicitly authorizes the spike. No user or production
credentials are used.

### Exit Gate

Schema specification, benchmark plan, benchmark report, four-GiB operational
limit, four-MiB chunk refinement, and privilege model were accepted on
2026-07-24. Phase 3.3 migration implementation is authorized. Later milestones
remain gated.

## Phase 3.3 — Migration Framework

### Deliverables

- selected migration tool and ADR;
- ordered immutable checksum-verified migrations;
- schema-version compatibility checks;
- empty-database install and supported upgrade tests;
- rollback/forward-repair policy;
- lock-duration and backfill policy;
- migration documentation.

Migration filenames remain illustrative until Phase 3.3 selects its tool:

```text
001_create_repositories.sql
002_create_repository_scans.sql
003_create_artifact_payloads.sql
004_create_artifact_envelopes.sql
005_create_artifact_dependencies.sql
006_create_query_projections.sql
007_create_audit_events.sql
```

### Exit Gate

Migrations pass disposable-database installation, upgrade, concurrency,
least-privilege, and checksum tests. Acceptance authorizes Phase 3.4 only.

## Phase 3.4 — Persistence Layer

Implement the storage-neutral Go port and PostgreSQL adapter.

Suggested package direction:

```text
backend/persistence/                 domain port and values
backend/internal/storage/postgres/   PostgreSQL adapter
```

The final layout must follow the project package standard. The adapter owns SQL
and driver translation; engine packages remain unchanged.

### Exit Gate

- conformance tests pass against the port;
- exact payload round trips and digest checks pass;
- atomic publication and rollback pass;
- concurrency/idempotency pass;
- large-payload gates pass;
- full backend regression and race tests pass;
- no engine-to-storage dependency exists.

Acceptance authorizes Phase 3.5 only.

## Phase 3.5 — Development Environment

Only here are local connection settings introduced.

Required secret-handling rules:

```text
.env
.env.local
*.local.env
```

must be ignored. Commit only a placeholder such as `.env.example` containing
names, never values. Prefer a dedicated disposable development role and
database. Production credentials are never used locally.

### Exit Gate

- one-command disposable environment;
- health check and migration status;
- least-privilege roles;
- documented reset and recovery path;
- no secrets in Git history, logs, reports, or tests.

Acceptance authorizes Phase 3.6 only.

## Phase 3.6 — Query APIs

Design and implement REST/gRPC only after the persistence contract is stable.
Initial candidates include repository listing, scan history, exact artifact
retrieval, diagnostic projections, and statistics. Endpoint spelling is not
approved by this roadmap.

### Exit Gate

- API specification and authorization model approved;
- pagination, version negotiation, errors, and payload limits defined;
- contract, security, load, and compatibility tests pass.

React UI work remains separately gated after API stabilization.

## Explicitly Deferred

- database-dependent engines;
- mutable artifact updates;
- language-specific source-of-truth tables;
- Qdrant, embeddings, and semantic search;
- event bus/outbox without a real consumer;
- multi-region or multi-primary deployment;
- multi-tenancy and row-level security until tenancy is designed;
- object storage until PostgreSQL payload measurements justify it;
- Dependency, Architecture, Reasoning, Patch, and Validation engines unless
  separately authorized.

## Phase 3 Completion

Persistence is complete only when schema, migrations, adapter, local
environment, integrity, security, backup/restore, and API handoff have each
passed their own gates. Merely connecting to PostgreSQL is not completion.
