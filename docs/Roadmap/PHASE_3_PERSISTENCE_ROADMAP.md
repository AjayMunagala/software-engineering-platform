# Phase 3 Persistence Roadmap

## Status

- Current milestone: Phase 3.4.2 — Neutral Port and Conformance Harness
- Current authorization: neutral Go package and conformance tests only
- PostgreSQL connection: disposable local migration/test databases only
- Credentials: not required and must not be uploaded
- Transient benchmark DDL/harness: accepted evidence
- Migration implementation: accepted and frozen
- Phase 3.3 exit gate: accepted on 2026-07-24
- Go storage adapter: not yet authorized; Phase 3.4.2 is the current gate
- Environment configuration, APIs, and UI: unauthorized

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

### Current State

- Atlas Community CLI v1.2.3 selected in accepted ADR 0012.
- Seven ordered SQL migrations and `atlas.sum` implemented.
- Fresh-cluster install, partial upgrade, repeat apply, checksum tamper,
  transaction rollback, concurrent apply, ownership, and least-privilege tests
  passed on PostgreSQL 18.4.
- Validation report completed on 2026-07-24.
- Engineering accepted Phase 3.3 on 2026-07-24 and authorized Phase 3.4.

### Deliverables

- selected migration tool and ADR;
- ordered immutable checksum-verified migrations;
- schema-version compatibility checks;
- empty-database install and supported upgrade tests;
- rollback/forward-repair policy;
- lock-duration and backfill policy;
- migration documentation.

Implemented migration dependency layers:

```text
202607240001_bootstrap_roles_and_schema.sql
202607240002_create_repositories_and_scans.sql
202607240003_create_artifact_payloads.sql
202607240004_create_artifact_envelopes.sql
202607240005_create_query_projections.sql
202607240006_create_audit_and_indexes.sql
202607240007_apply_runtime_privileges.sql
```

### Exit Gate

Migrations pass disposable-database installation, upgrade, concurrency,
least-privilege, and checksum tests. Acceptance authorizes Phase 3.4 only.

Accepted on 2026-07-24. Phase 3.4 architecture work is authorized. The refined
Phase 3.4 roadmap now requires neutral-port acceptance before any adapter
implementation; later phases remain gated.

## Phase 3.4 — Persistence Layer

### Current State

- Architecture work authorized on 2026-07-24.
- Phase 3.4.1 design and ADR 0013 were accepted on 2026-07-24.
- Phase 3.4.2 neutral package and conformance implementation is authorized.
- No Go persistence or PostgreSQL adapter implementation has started.
- Runtime migration execution, environment credentials, APIs, and UI remain
  outside this milestone.

Phase 3.4 is deliberately staged. Acceptance of one subphase authorizes only
the next subphase.

### Phase 3.4.1 — Storage-Neutral Port Design (Accepted)

Design only:

- neutral capability interfaces and detached domain values;
- artifact, scan, publication, retrieval, and retention lifecycle;
- internal transaction ownership and atomic publication;
- idempotency, cancellation, ambiguous-commit, and rollback behavior;
- stable safe error classification;
- PostgreSQL adapter responsibility and exclusions;
- adapter-independent conformance and benchmark strategy;
- proposed ADR 0013 and candidate API `0.1.0`.

Documents:

- `docs/Architecture/STORAGE_NEUTRAL_PERSISTENCE_PORT.md`;
- `docs/API/PERSISTENCE_PORT_CANDIDATE.md`;
- `docs/Decisions/0013-storage-neutral-persistence-port.md`.

No code, SQL, connections, credentials, APIs, UI, connection pooling, runtime
configuration, or dependency-injection wiring is authorized.

#### Exit Gate

Accepted on 2026-07-24. Engineering accepted the architecture, candidate API,
ADR 0013, dependency direction, lifecycle, transaction, error, and conformance
model together. Scope isolation must be tested across every public operation.
Phase 3.4.2 only is authorized.

### Phase 3.4.2 — Neutral Port and Conformance Harness (Current)

After explicit authorization, implement only:

- `backend/persistence` neutral values and interfaces;
- constructors, validation, defensive copies, and safe error helpers;
- adapter-independent conformance harness and contract documentation;
- public API tests, benchmarks, vet, race, and dependency checks.

PostgreSQL adapter code remains gated until this neutral contract is accepted.

#### Exit Gate

Engineering freezes the neutral contract candidate and authorizes Phase 3.4.3.

### Phase 3.4.3 — PostgreSQL Adapter

Implement the accepted port against the frozen migrated schema. The adapter
owns parameterized SQL, transaction execution, locking, ordered 4 MiB chunks,
streaming integrity, atomic publication, exact retrieval, error translation,
retention, and garbage collection.

The adapter receives an approved database execution capability in tests. It
does not load environment variables, construct production connection pools, or
run migrations.

#### Exit Gate

The PostgreSQL adapter passes the neutral conformance suite, disposable
database integration tests, failure and recovery tests, and large-payload
gates. Acceptance authorizes Phase 3.4.4 only.

### Phase 3.4.4 — Adapter Validation and Freeze

Complete regression, shuffled, vet, targeted/full race, dependency, security,
memory, performance, documentation, and API reviews. Record the execution
environment and compare against the accepted Phase 3.2 benchmark baseline.

Acceptance freezes Persistence Port and PostgreSQL Adapter `1.0.0` and
authorizes Phase 3.5 only.

### Planned Package Direction

Suggested package direction:

```text
backend/persistence/                 domain port and values
backend/internal/storage/postgres/   PostgreSQL adapter
```

The final layout must follow the project package standard. The adapter owns SQL
and driver translation; engine packages remain unchanged.

The final Phase 3.4 gate still requires conformance, exact round trips, digest
checks, atomic publication/rollback, concurrency/idempotency, large-payload
validation, full regression/race tests, and proof that no engine imports
storage. Passing an earlier subphase does not authorize Phase 3.5.

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
