# Repository Service Persistence & Runtime Integration

## Status

- Phase: 4.0.6 design
- Status: Proposed for engineering review
- Production implementation: Unauthorized
- Candidate integration contract: `0.1.0`
- Date: 2026-07-28

## Exact responsibility

Phase 4.0.6 connects the accepted Repository Service coordinator to Runtime
Infrastructure `1.0.0` and Persistence Port `1.0.0` without changing engine,
persistence, PostgreSQL Adapter, or runtime ownership boundaries.

## In scope

- adapt repository lifecycle storage to `persistence.RepositoryStore`;
- adapt scan lifecycle and publication to the runtime ingest capability;
- adapt artifact metadata and exact-byte reads to the runtime read capability;
- adapt runtime work admission to the scan coordinator's admission contract;
- stage sealed Phase 4.0.5 payloads and publish one atomic manifest;
- translate neutral persistence records into Repository Service values;
- translate persistence/runtime failures into stable service errors;
- reconcile publication responses that may have been lost after commit;
- compose the lifecycle, scan, intelligence, persistence, and runtime pieces;
- validate against disposable PostgreSQL using the accepted migrations.

## Explicitly out of scope

- migrations at application startup;
- changes to released RIE, Go LIE, Persistence Port, PostgreSQL Adapter, or
  Runtime Infrastructure contracts;
- new artifact codecs, projections, diagnostics, or statistics;
- retention scheduling or background garbage collection;
- distributed execution, queues, workers, retries, or leases;
- REST, gRPC, HTTP health endpoints, authentication, authorization policy, UI,
  AI orchestration, repository cloning, network access, or command execution;
- connection-pool creation, TLS, credentials, configuration loading, runtime
  startup, or runtime shutdown ownership.

## Frozen dependency direction

```text
Repository Service facade
  |-- Repository Lifecycle coordinator
  |     |-- Source-proof adapter
  |     `-- Persistence lifecycle store
  |
  `-- Scan coordinator
        |-- Runtime admission adapter
        |-- Phase 4.0.5 intelligence/materialization adapter
        `-- Persistence scan store
              |-- Runtime ingest capability
              `-- Runtime read capability

Runtime ingest/read capabilities
  -> PostgreSQL Adapter 1.0.0
  -> Persistence Port 1.0.0
  -> accepted PostgreSQL schema
```

The engines still do not import persistence. The persistence packages still do
not import engine artifacts. The integration package is the only application
package allowed to know both the service-owned store contracts and the frozen
persistence/runtime capability contracts.

## Ownership

| Concern | Owner |
|---|---|
| Source authorization and local root | Deployment source resolver |
| Analysis and sealed exact bytes | Phase 4.0.5 adapter |
| Scan policy and orchestration | Scan coordinator |
| Work admission and drain cancellation | Runtime Infrastructure |
| Persistence request construction and model translation | Phase 4.0.6 integration adapter |
| Transactions, SQL, chunks, atomic publication | PostgreSQL Adapter |
| Pools, TLS, compatibility proof, shutdown | Runtime Infrastructure |
| Migrations | Deployment preflight |

The integration bundle borrows runtime capabilities. It never closes pools or
owns runtime shutdown.

## Required candidate-contract refinements

Design review must approve the following refinements before implementation.
They affect only the unreleased Repository Service `0.1.0` contract and its
internal coordinator contracts.

### 1. Persistence-compatible public identifiers

The accepted PostgreSQL schema stores scope, repository, scan, and physical
artifact identifiers as UUID values. Repository Service currently accepts a
broader machine-name grammar.

For the production contract:

- `ScopeID`, `RepositoryID`, and `ScanID` must be canonical lowercase UUID
  strings;
- `RequestID` remains an opaque bounded machine value because the schema stores
  it as text;
- `PrincipalID` remains opaque bounded text;
- public service artifact IDs remain
  `repository-service-artifact-id/v1` values such as `rsaid1_...`.

The service contract constructors and conformance fixtures must enforce the
same UUID policy so fake and PostgreSQL-backed implementations cannot diverge.
Changing this after Repository Service `1.0.0` would be breaking, so it must be
settled now.

### 2. Physical artifact UUID mapping

Public artifact IDs cannot be written directly to PostgreSQL's UUID column.
The integration adapter derives a physical UUID using
`repository-service-storage-artifact-id/v1`:

```text
ASCII "repository-service-storage-artifact-id/v1" + NUL
uint32-be length + exact UTF-8 public artifact ID
SHA-256 of the complete preimage
first 16 digest bytes with RFC 9562 version-8 and variant bits applied
lowercase canonical UUID text
```

The mapping is internal and deterministic. Reads reconstruct the public
artifact ID from repository ID, scan ID, artifact name, version, and stable-ID
scheme, then verify that its mapped UUID equals the stored UUID. Any mismatch
or collision is `integrity_failure`; it is never guessed through.

### 3. Persistence timestamps are authoritative

Repository Service clocks provide deterministic intent timestamps before a
store operation. PostgreSQL Adapter transactions publish database-owned
timestamps. Nanosecond equality between the two clocks is not a valid
integration requirement.

Successful store results use persistence timestamps. Coordinator validation
continues to require exact identifiers, profile, revision, state, artifact
metadata, digests, sizes, ordering, and manifest contents, while timestamp
validation becomes invariant-based:

- requested time is non-zero;
- started time is not before requested time;
- finished time is not before started time;
- artifact creation time is non-zero and not before scan start;
- all returned values are UTC.

Fake stores must exercise the same rules.

### 4. Complete internal persistence inputs

The following additive internal coordinator data is required:

- `BeginCommand.SourceFingerprint()` separate from its mutation fingerprint;
- `PublishCommand.RequestID()`;
- `FinalizeCommand.RequestID()`;
- `PublicationArtifact.Dependencies()` returning a defensive copy.

The source fingerprint lets scan begin prove that the resolved source still
matches the registered repository. Request IDs let persistence own durable
idempotency/audit records. Dependency access lets publication preserve the
frozen Phase 4.0.5 graph.

## Source identity consistency

Registration resolves an opaque handle into path-free source kind, fingerprint
scheme, digest, and optional revision. Scan preparation resolves the handle
again. Before `BeginScan`, the persistence adapter reads the repository within
the same authorized scope and requires the stored source fingerprint to equal
the scan session's source fingerprint.

A mismatch returns `source_unavailable` with safe reason
`source-proof-mismatch`. No scan begins, and no local path or source handle is
persisted or exposed.

## Runtime integration

The runtime adapter exposes only:

- `Admit(context.Context) (Work, error)`;
- runtime ingest capabilities;
- runtime read capabilities.

One newly created scan flight acquires one runtime work item. Joined callers do
not acquire more work. Drain cancellation flows from `Work.Context()` into the
existing scan leader. `Work.Done()` is invoked exactly once.

Repository reads and lifecycle mutations do not acquire scan work leases. A
future transport may have separate request admission, but that is not part of
this milestone.

## Persistence mappings

| Service operation | Persistence capability |
|---|---|
| Register repository | `RepositoryStore.RegisterRepository` |
| Get/list/archive repository | `RepositoryStore` |
| Begin/get/list/fail/cancel scan | `ScanStore` |
| Stage exact payload | `PayloadStager.StagePayload` |
| Publish scan | `PublicationStore.PublishScan` |
| Get/list artifact | `ArtifactReader` |
| Export exact payload | `ArtifactReader.ExportPayload` |

`IntegrityVerifier` remains available to validation and operational tooling but
is not executed on every query. Staging already verifies exact bytes, digest,
and size; export verifies the returned receipt against stored metadata.

## Scan publication sequence

```text
Acquire runtime work
  -> prepare and prove source
  -> verify registered source fingerprint
  -> BeginScan (persistence transaction)
  -> execute Phase 4.0.5 analysis
  -> for each artifact in deterministic order:
       open sealed payload
       StagePayload (one persistence transaction)
       require exact receipt digest and size
       close reader
  -> translate artifact and dependency submissions
  -> compute repository-service-manifest/v1 digest
  -> PublishScan once (one serializable atomic transaction)
  -> read durable scan and complete ordered artifact set
  -> translate and return detached service result
  -> close materialization session
  -> release runtime work
```

No service transaction spans staging and publication. Staged payloads remain
invisible until publication and can be reclaimed later by accepted retention.

## Frozen manifest candidate

The integration adapter proposes `repository-service-manifest/v1`. Its SHA-256
preimage is:

```text
ASCII "repository-service-manifest/v1" + NUL
field(repository ID)
field(scan ID)
field(profile name)
field(profile version)
raw 32-byte profile digest
field(source revision; zero length allowed)
uint32-be artifact count
for each artifact in canonical service order:
  field(public artifact ID)
  field(name)
  field(version)
  field(stable-ID scheme)
  field(codec name)
  field(codec version)
  field(media type)
  raw 32-byte payload digest
  uint64-be payload size
  field(producer name)
  field(producer version)
  uint32-be dependency count
  for each dependency in ordinal order:
    uint32-be ordinal
    field(source public artifact ID)
    field(declared name)
    field(declared version)
```

`field(x)` is `uint32-be byte length` followed by exact UTF-8 bytes. Timestamps,
paths, source handles, request IDs, principals, and database UUID mappings are
excluded. Duplicate artifacts, missing dependencies, reordered ordinals,
self-edges, cycles, overflow, or trailing data fail closed.

The manifest algorithm must receive golden-vector validation before production
implementation is accepted.

## Request and actor policy

- Repository registration, archival, scan begin, publication, failure, and
  cancellation retain the caller's service `RequestID`.
- Each staged artifact receives a deterministic child request ID derived from
  the parent request ID and public artifact ID using
  `repository-service-stage-request/v1`.
- The persistence audit actor is `principal/<Scope.PrincipalID>`.
- Actor and request values never become metric labels.

## Profile reconstruction

Persistence stores the analysis-profile digest, while the service scan model
also exposes profile name and version. Record translation resolves the digest
through the immutable Repository Service `ProfileRegistry`.

An unknown digest is an `integrity_failure`, not an unknown custom profile.
Phase 4.0 supports only the frozen `repository-go/v1` profile.

## Publication ambiguity

If `PublishScan` or the immediate durable reads return an unavailable, timeout,
or ambiguous error, the scan coordinator invokes `Reconcile` using a detached
bounded context.

Reconciliation succeeds only when:

- the durable scan is succeeded;
- the profile digest and source revision match;
- the full ordered artifact set reconstructs exactly;
- every payload digest and size matches;
- the deterministic public and physical artifact identities match.

Running, missing, partial, or mismatched state remains
`persistence_unavailable` or `integrity_failure`. A possibly published scan is
never marked failed.

## Failure and cleanup

- failure before scan begin creates no scan;
- failure after begin and before publication finalizes failed/canceled through
  `ScanStore` using a detached bounded context;
- payload-stage failure rolls back that stage transaction;
- already staged unreferenced payloads remain invisible;
- publication failure rolls back atomically unless the response was lost after
  commit, in which case reconciliation decides;
- readers, materialized spools, source resolutions, runtime work, and detached
  contexts are closed exactly once;
- pool and runtime cleanup remain owned by Runtime Infrastructure.

## Error translation

Persistence kinds map to the closest stable Repository Service kind. Context
cancellation and deadlines are checked first. SQLSTATE, constraint names, pgx
errors, source paths, authenticated URLs, payload bytes, and raw driver text
never cross the boundary.

An adapter `invalid_input` after the service constructed a valid request is
treated as `internal` or `integrity_failure`, because it signals a mapping bug,
not caller input.

## Security and scope isolation

- every operation constructs a fresh persistence `Scope` from the authorized
  service scope;
- every read, list, export, stage, and mutation retains repository scope;
- scope, repository, scan, and physical artifact IDs are validated before the
  persistence call;
- query results are checked against their requested scope and identity;
- source handles and absolute paths never enter persistence requests;
- no credentials, pool handles, SQL, or runtime internals enter service models;
- the conformance suite must test cross-scope reads as well as writes.

## Performance and resource gates

- integration overhead excluding analysis, serialization, and database work:
  p95 below 25 ms for ten artifacts;
- staging remains streaming with no artifact-sized `[]byte` copy;
- at most one Phase 4.0.5 spool reader is open per sequential stage operation;
- exact export working memory remains bounded by persistence chunk buffers;
- 100 identical concurrent scan callers still produce one analysis,
  one staging sequence, and one publication;
- 1,000 repeated small repository lifecycles leak no work items, readers,
  spools, goroutines, or database connections.

## Proposed implementation package

```text
backend/internal/service/repository/integration/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    implementation_test.go
    implementation_benchmark_test.go
```

The package follows the project-wide eight-file standard. PostgreSQL-specific
tests remain in a separate integration test harness so production files import
only neutral persistence/runtime capabilities.

## Design acceptance gate

Production implementation remains unauthorized until engineering approves:

1. the UUID compatibility policy;
2. physical artifact UUID mapping;
3. persistence-authoritative timestamp policy;
4. additive scan-command refinements;
5. exact manifest preimage;
6. source-proof consistency check;
7. runtime admission and capability ownership;
8. transaction, ambiguity, error, and cleanup behavior;
9. validation plan and performance gates;
10. ADR 0017.

