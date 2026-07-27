# Repository Service Layer Architecture

## Status

- Phase: 4.0
- Design version: `0.1.0`
- Status: Accepted on 2026-07-27
- Date: 2026-07-27
- Phase 4.0.1 design spike: accepted
- Phase 4.0.2 neutral contract: accepted
- Phase 4.0.3 repository lifecycle: authorized
- Production implementation: not started

## Purpose

The Repository Service Layer synchronously coordinates already-authorized
repository lifecycle, deterministic repository/Go analysis, exact artifact
materialization, durable staging, and atomic scan publication without exposing
transport, database, runtime, or engine implementation details to callers.

## One-sentence responsibility

Resolve one authorized repository source, execute the approved intelligence
profile, persist its immutable artifacts, and publish one complete scan or one
explicit terminal failure.

## In scope

- repository registration, retrieval, listing, and archival;
- synchronous scan execution and explicit cancellation;
- dependency injection for source, admission, analysis, materialization, and
  storage capabilities;
- RIE `1.0.0` orchestration;
- Go Language, Package Identity, and Semantic Inventory `1.0.0` orchestration
  when Go is present and required by the selected profile;
- deterministic artifact dependency manifests;
- exact-byte, SHA-256-verified streaming into Persistence Port `1.0.0`;
- atomic publication and terminal scan-state reconciliation;
- artifact metadata/list/export operations;
- stable service errors, idempotency, cancellation, and bounded cleanup.

## Out of scope

- HTTP, REST, gRPC, GraphQL, WebSocket, or CLI transports;
- authentication, authorization policy, tenants, or database roles;
- UI, IDE integration, webhooks, queues, workers, or schedulers;
- asynchronous/background scans;
- AI/LLM orchestration, prompts, reasoning, patching, or code execution;
- new RIE/LIE capabilities or changes to released artifacts;
- dependency or architecture intelligence;
- migrations, credentials, pool construction, or runtime process signals;
- Git clone, network fetch, package download, or repository mutation;
- persistence of local absolute repository paths.

## Architectural position

```text
Future transport / command host
              |
              v
Repository Service contract 0.1.0
              |
    +---------+----------+-------------+----------------+
    |                    |             |                |
    v                    v             v                v
Admission            Source         Analysis       Materializer
capability            resolver        runner          / codecs
    |                    |             |                |
    v                    v             v                v
Runtime 1.0       Authorized local   RIE/LIE 1.0   Exact byte streams
                                      |
                                      v
                          Storage-neutral service port
                                      |
                                      v
                          Persistence Port 1.0.0
                                      |
                                      v
                          PostgreSQL Adapter 1.0.0
```

The core service package owns orchestration policy. Adapters own translations
to released engines, runtime admission, and Persistence Port values. No engine
imports the service or persistence packages.

## Package boundaries

The planned implementation follows the project package standard:

```text
backend/service/repository/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    implementation_test.go
    implementation_benchmark_test.go

backend/internal/service/repository/adapters/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    implementation_test.go
    implementation_benchmark_test.go
```

The public service package contains no SQL, pgx, filesystem traversal, parser,
HTTP, or runtime pool type. The internal adapter package is the composition
boundary for released RIE/LIE, runtime, and persistence contracts.

## Capability model

The service contract is split into narrow capabilities:

- `RepositoryLifecycleService` — register, get, list, archive;
- `ScanExecutionService` — execute, get, list, cancel;
- `ArtifactQueryService` — get/list metadata and stream exact payloads;
- composed `Service` — convenience only.

Dependencies are also narrow:

- `AdmissionController` — obtains one cancellable work lease;
- `SourceResolver` — resolves an opaque handle to an authorized local source;
- `AnalysisRunner` — executes an immutable versioned analysis profile;
- `ArtifactMaterializer` — deterministically serializes and seals exact bytes;
- `ServiceStore` — maps service operations to Persistence Port capabilities;
- `Clock` and `IDGenerator` — injectable deterministic infrastructure.

Consumers depend on the smallest capability needed. There is no public
transaction manager and no service God Object.

## Authorized repository source

Callers supply an opaque source handle, not an absolute path. A deployment
owned `SourceResolver` maps that handle to an `AuthorizedSource` containing:

- a process-local root path available only to the analysis adapter;
- normalized source kind;
- SHA-256 source fingerprint;
- locally proven revision when available;
- a cleanup capability when the resolver owns temporary resources.

The root path is never persisted, logged, used as a metric label, returned in a
service response, or embedded in a durable artifact envelope. RIE's existing
report may contain a root path; the materializer must use the approved durable
artifact view that redacts or normalizes deployment-local path fields.

Phase 4.0 does not clone or fetch sources. The resolver can only return sources
already authorized and locally available to the process.

## Analysis profile

Phase 4.0 begins with one immutable profile:

```text
repository-go/v1
```

It contains:

1. the frozen RIE pipeline;
2. Go syntax analysis when Go files are present;
3. Go package identity after syntax;
4. Go semantic resolution after package identity;
5. the exact required artifact set and dependency order.

The canonical profile document is SHA-256 hashed. Its digest is stored in the
scan record. Adding, removing, reordering, or reconfiguring an engine creates a
new profile version and digest; it never silently changes `repository-go/v1`.

For a repository with no Go files, RIE artifacts remain a valid successful
result and Go artifacts are intentionally absent. If Go is present, all three
released Go artifacts are required; a failure is not silently downgraded.

## Engine orchestration

The analysis adapter executes released components in this order:

```text
RIE Pipeline
  -> RepositorySnapshot + RIE inventories + RepositoryIntelligenceSummary
  -> GoLanguageInventory (when Go is present)
  -> GoPackageIdentityInventory
  -> GoSemanticInventory
```

Every phase consumes the lowest immutable artifact containing its required
facts. The service does not modify artifacts or reuse mutable engine state
between scans. Each execution owns a new per-run artifact store.

Phase 4.0.1 confirmed that released RIE, Go syntax, Go package identity, and Go
semantic stages compose in that order without changing their public APIs. It
also confirmed that RIE presentation output requires an explicit durable view
that removes run IDs, wall-clock timing, throughput, and local root paths. The
Go syntax artifact similarly requires an external detached codec view built
from its frozen accessors; persistence does not interpret either artifact.

## Exact-byte materialization

Persistence requires digest and size before staging. To avoid a second
artifact-sized memory allocation, the materializer:

1. selects a registered codec by artifact name and version;
2. serializes the approved durable view once into a restricted spool;
3. computes SHA-256 and byte count while writing;
4. seals the spool against mutation;
5. exposes a reopenable read stream plus descriptor, digest, and size;
6. verifies the same digest while Persistence Port stages the stream;
7. removes the spool during bounded cleanup.

Large artifacts use a permission-restricted temporary file outside the scanned
repository. Small-artifact in-memory optimization is allowed only behind the
same sealed abstraction. The service never persists ASTs, source text, raw
repository files, or an unversioned serialization.

The design spike uses the file-backed path for every size so bounded behavior
can be measured directly. Its 64-MiB exact-byte proof allocates approximately
70 KiB on the Go heap, excluding the already-existing artifact object and
operating-system page cache.

The operational payload maximum remains 4 GiB. Materialization fails before
staging when an artifact exceeds this limit.

## Scan lifecycle

```text
Validate immutable request
  -> acquire runtime work lease
  -> join/create in-process scan flight
  -> resolve and prove source
  -> register/get repository identity
  -> begin durable running scan
  -> execute fresh analysis profile
  -> materialize exact artifacts
  -> stage every payload
  -> build deterministic envelope/dependency manifest
  -> publish scan atomically
  -> return detached successful result
```

At every return path, the work lease, source, and materialized spools are
released exactly once.

## Transaction boundaries

The service never opens a database transaction.

- repository registration is one persistence-owned transaction;
- scan begin is one persistence-owned transaction;
- each payload stage is one persistence-owned transaction and remains hidden;
- publication is one persistence-owned atomic transaction;
- failure/cancellation finalization is one persistence-owned transaction;
- reads and exports use their existing persistence-owned boundaries.

Only `PublishScan` makes artifact envelopes and a succeeded scan visible. A
failed or canceled scan may leave content-addressed staged payloads, which are
invisible and later eligible for accepted garbage collection.

## Idempotency and concurrency

Every mutating request carries a bounded `RequestID`. Repository, scan, and
artifact IDs are explicit immutable request fields.

- same request ID plus identical normalized request: same logical outcome;
- same request ID plus different normalized request: `idempotency_conflict`;
- concurrent identical `ExecuteScan` calls in one process join one keyed flight;
- the leader owns execution; waiters receive the same detached result;
- waiter cancellation does not cancel the leader unless all interested callers
  cancel and the leader context is also canceled by policy;
- different request IDs targeting the same scan ID are rejected;
- no artifact is published twice.

If a process restarts and finds a durable `running` scan without an in-process
leader, Phase 4.0 returns `orphaned_scan` and does not guess whether analysis can
resume. Lease-based recovery is future work requiring a separate design.

## Failure and cancellation

Failure classification is stable and redacted:

```text
invalid_input
not_found
conflict
idempotency_conflict
scan_already_running
orphaned_scan
source_unavailable
analysis_failed
materialization_failed
integrity_failure
persistence_unavailable
timeout
canceled
internal
```

Raw engine, filesystem, codec, persistence, SQL, pgx, path, or driver error
text never crosses the service boundary.

Before publication:

- caller/leader cancellation runs bounded detached cleanup;
- a running scan is finalized as canceled;
- other failures finalize it as failed with a stable reason code;
- staging remains invisible.

After a possibly committed publication error, the service queries durable scan
state. `succeeded` is returned as success; a nonterminal or unavailable result
becomes an explicit ambiguous persistence failure. The service never marks a
published scan failed.

## Cancellation checkpoints

The service checks cancellation:

- before admission and after admission;
- before and after source resolution;
- before durable scan begin;
- between every engine stage;
- during materialization writes;
- before and after every payload stage;
- while constructing the manifest;
- immediately before publication;
- while streaming an exported payload.

Finalization and resource cleanup use a separate bounded context because the
caller context may already be canceled.

## Security boundary

- authorization decisions occur before the service receives `Scope`;
- source handles are opaque and validated;
- source roots never enter durable metadata or telemetry;
- symbolic-link and repository-boundary policy remains owned by released RIE;
- no network access, command execution, package download, or repository write;
- exact payloads are streamed, not logged;
- IDs may appear in audit records but repository IDs, scan IDs, artifact IDs,
  source handles, paths, and digests are forbidden metric labels;
- service errors expose only stable codes and safe bounded messages.

## Observability

The service receives a narrow event/metric capability. Proposed bounded
operations are `register_repository`, `archive_repository`, `execute_scan`,
`cancel_scan`, `get_scan`, `list_scans`, `get_artifact`, and `export_artifact`.

Metrics may include operation duration/outcome, artifacts materialized/staged,
exact bytes staged/exported, and terminal scan outcome. Labels remain bounded
to operation, outcome, artifact contract name/version, and neutral error kind.

No path, source handle, repository/scan/artifact ID, digest, principal, or raw
message is a metric label.

## Performance and resource targets

- orchestration overhead excluding analysis, serialization, and persistence:
  p95 below 25 ms for a 20-artifact manifest;
- no full artifact payload is duplicated in memory;
- materializer working memory below 64 MiB independent of payload size;
- service list operations preserve Persistence Port pagination limits;
- cancellation is observed within one bounded chunk or engine checkpoint;
- 100 concurrent identical scan requests execute one analysis flight;
- zero resource leaks across 1,000 small scan lifecycles.

Targets are candidate gates and may be refined only with recorded benchmark
evidence.

## Exit gate for implementation authorization

- architecture, candidate API, ADR, roadmap, and validation plan accepted
  together;
- source identity and durable path-redaction rules approved;
- analysis profile and required artifact graph frozen at candidate `0.1.0`;
- idempotency, orphaned scan, publication ambiguity, and cleanup policies
  approved;
- no conflict with released RIE, Go LIE, Persistence, or Runtime contracts.

Until this design gate is accepted, Phase 4.0 production implementation does
not begin.
