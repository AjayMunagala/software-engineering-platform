# Phase 3 Persistence Roadmap

## Status

- Current milestone: Phase 3.5.2 — PostgreSQL Runtime
- Current authorization: Phase 3.5.2 implementation only
- PostgreSQL connection: disposable local migration/test databases only
- Credentials: not required and must not be uploaded
- Transient benchmark DDL/harness: accepted evidence
- Migration implementation: accepted and frozen
- Phase 3.3 exit gate: accepted on 2026-07-24
- Go storage adapter: authorized on 2026-07-25
- PostgreSQL adapter local exit gate: reached on 2026-07-25
- PostgreSQL adapter accepted: 2026-07-26
- Phase 3.5.0 design and ADR 0015: accepted on 2026-07-26
- Phase 3.5.1 runtime configuration local exit gate: reached on 2026-07-26
- Phase 3.5.1 engineering acceptance: accepted on 2026-07-26
- Phase 3.5.2 PostgreSQL runtime implementation: authorized
- APIs and UI: unauthorized

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
- Phase 3.4.2 implementation and validation were accepted on 2026-07-25.
- Phase 3.4.3 PostgreSQL adapter implementation and validation were accepted
  on 2026-07-26.
- Phase 3.4.4 final contract validation was accepted on 2026-07-26.
- Persistence Port and PostgreSQL Adapter `1.0.0` are frozen.
- Phase 3.5.0 runtime infrastructure design and ADR 0015 were accepted on
  2026-07-26; Phase 3.5.1 only is authorized.
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
- `docs/API/PERSISTENCE_PORT_V1.md`;
- `docs/Decisions/0013-storage-neutral-persistence-port.md`.

No code, SQL, connections, credentials, APIs, UI, connection pooling, runtime
configuration, or dependency-injection wiring is authorized.

#### Exit Gate

Accepted on 2026-07-24. Engineering accepted the architecture, candidate API,
ADR 0013, dependency direction, lifecycle, transaction, error, and conformance
model together. Scope isolation must be tested across every public operation.
Phase 3.4.2 only is authorized.

### Phase 3.4.2 — Neutral Port and Conformance Harness (Accepted)

After explicit authorization, implement only:

- `backend/persistence` neutral values and interfaces;
- constructors, validation, defensive copies, and safe error helpers;
- adapter-independent conformance harness and contract documentation;
- public API tests, benchmarks, vet, race, and dependency checks.

PostgreSQL adapter code remains gated until this neutral contract is accepted.

Candidate evidence:

- neutral interfaces and immutable models implemented;
- reusable scope-isolation harness covers all 18 public operations;
- `persistence` statement coverage: 86.5%;
- `persistence/conformance` statement coverage: 88.1%;
- package/full regression, vet, shuffled, repeated, targeted/full race tests:
  pass with zero races;
- forbidden engine/database dependencies: zero;
- validation report:
  `docs/Validation/PERSISTENCE_PORT_CONFORMANCE_VALIDATION_REPORT.md`.

Engineering accepted this evidence on 2026-07-25. The neutral contract is
frozen for adapter work. Neutral conformance must be the first adapter gate.

#### Exit Gate

Accepted on 2026-07-25. Engineering froze the neutral contract candidate and
authorized Phase 3.4.3.

### Phase 3.4.3 — PostgreSQL Adapter (Accepted)

Implement the accepted port against the frozen migrated schema. The adapter
owns parameterized SQL, transaction execution, locking, ordered 4 MiB chunks,
streaming integrity, atomic publication, exact retrieval, error translation,
retention, and garbage collection.

The adapter receives an approved database execution capability in tests. It
does not load environment variables, construct production connection pools, or
run migrations.

The reusable neutral conformance suite runs before adapter-specific integration
tests. PostgreSQL behavior may not weaken or replace a neutral requirement.

Local evidence on 2026-07-25 records neutral conformance, exact 4 MiB chunking,
rollback, idempotency/conflict handling, atomic publication, scope isolation,
corruption detection, concurrent staging, projections, pagination, retention,
garbage collection, 85.1% statement coverage, race validation, and a
1,556,379,091-byte streaming round trip. Engineering accepted this evidence
and ADR 0014 on 2026-07-26.

Evidence: [`POSTGRESQL_ADAPTER_VALIDATION_REPORT.md`](../Validation/POSTGRESQL_ADAPTER_VALIDATION_REPORT.md).
Driver decision: [`0014-pgx-postgresql-adapter.md`](../Decisions/0014-pgx-postgresql-adapter.md).

#### Exit Gate

Accepted on 2026-07-26 after the PostgreSQL adapter passed the neutral
conformance suite, disposable database integration tests, failure and recovery
tests, and large-payload gates. Phase 3.4.4 only is authorized.

### Phase 3.4.4 — Adapter Validation and Freeze (Accepted)

Complete regression, shuffled, vet, targeted/full race, dependency, security,
memory, performance, documentation, and API reviews. Record the execution
environment and compare against the accepted Phase 3.2 benchmark baseline.

Local evidence on 2026-07-26 records final API and compatibility review,
mandatory conformance-first adapter validation, 86.0% neutral coverage, 88.1%
conformance coverage, 85.1% PostgreSQL integration coverage, zero Windows and
Linux race findings, a clean dependency/security audit, and an exact
1,556,379,091-byte adapter round trip. Engineering accepted the evidence and
promoted the Persistence Port and PostgreSQL Adapter to `1.0.0` on 2026-07-26.

Evidence:
[`PERSISTENCE_CONTRACT_STABILIZATION_REPORT.md`](../Validation/PERSISTENCE_CONTRACT_STABILIZATION_REPORT.md).

Accepted on 2026-07-26. Persistence Port and PostgreSQL Adapter `1.0.0` are
frozen. Phase 3.5.0 design was subsequently accepted and Phase 3.5.1 only is
authorized.

### Planned Package Direction

Suggested package direction:

```text
backend/persistence/                 domain port and values
backend/internal/storage/postgres/   PostgreSQL adapter
```

The final layout must follow the project package standard. The adapter owns SQL
and driver translation; engine packages remain unchanged.

The completed Phase 3.4 gate required conformance, exact round trips, digest
checks, atomic publication/rollback, concurrency/idempotency, large-payload
validation, full regression/race tests, and proof that no engine imports
storage. Its acceptance authorized Phase 3.5.0 design only.

## Phase 3.5 — Runtime Infrastructure

### Phase 3.5.0 — Architecture and Design (Accepted)

Design only:

- configuration ownership, strict sources, precedence, validation, redaction,
  and startup immutability;
- capability-specific PostgreSQL pool ownership and lifecycle;
- local/CI TLS-disabled boundary and staging/production `verify-full` policy;
- startup, schema compatibility, health, graceful shutdown, and failure model;
- structured safe logging and bounded metric semantics;
- local, CI, staging, and production profiles;
- ADR 0015 and the implementation validation plan.

Documents:

- `docs/Architecture/RUNTIME_INFRASTRUCTURE.md`;
- `docs/Architecture/RUNTIME_CONFIGURATION_SPECIFICATION.md`;
- `docs/Architecture/RUNTIME_LIFECYCLE_SPECIFICATION.md`;
- `docs/Architecture/HEALTH_OBSERVABILITY_SPECIFICATION.md`;
- `docs/Decisions/0015-runtime-infrastructure.md`;
- `docs/Validation/RUNTIME_INFRASTRUCTURE_VALIDATION_PLAN.md`.

No Go runtime code, compatibility migration, database connection, credential,
`.env` file, listener, API, UI, or engine change is authorized by this design.

#### Design Exit Gate

Engineering accepted all six documents and ADR 0015 together on 2026-07-26.
That acceptance authorized Phase 3.5.1 only.

### Phase 3.5.1 — Configuration and Secret Boundaries (Accepted)

Implement strict immutable configuration, source precedence, safe views,
profile validation, secret-provider interfaces, redaction tests, fuzzing, and
benchmarks. No live database or pool construction.

Local implementation and validation completed on 2026-07-26. Evidence:
[`RUNTIME_CONFIGURATION_VALIDATION_REPORT.md`](../Validation/RUNTIME_CONFIGURATION_VALIDATION_REPORT.md).
Engineering accepted the implementation and evidence on 2026-07-26 and
authorized Phase 3.5.2 only.

### Phase 3.5.2 — PostgreSQL Runtime (Current, Authorized)

Implement TLS loading, capability pool set, additive compatibility-record
migration, deployment/runtime compatibility verification, adapter construction,
and disposable TLS/PostgreSQL integration tests.

### Phase 3.5.3 — Lifecycle, Health, and Observability (Gated)

Implement startup resource ownership, narrow capability routing, admission and
drain, graceful/forced shutdown, callable health, structured logging, bounded
metrics, and failure injection.

### Phase 3.5.4 — Integrated Validation and Freeze (Gated)

Run local/CI/profile, TLS, compatibility, regression, shuffle, vet, race,
coverage, leak, security, benchmark, recovery, and documentation gates. Create
the safe example configuration and runbooks. Acceptance authorizes Phase 3.6
API design only.

Required secret-handling rules remain:

```text
.env
.env.local
*.local.env
```

These files must be ignored. Commit only names/placeholders in an example.
Production credentials are never used locally or accepted in review evidence.

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
