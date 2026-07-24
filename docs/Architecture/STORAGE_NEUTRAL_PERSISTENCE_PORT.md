# Storage-Neutral Persistence Port

## Status

- Phase: 3.4.1
- Contract version: candidate `0.1.0`
- State: accepted on 2026-07-24
- Implementation: Phase 3.4.2 neutral port and conformance authorized
- PostgreSQL credentials or connection: not required

## Exact Responsibility

The persistence port durably stages, publishes, verifies, retrieves, and
retires detached immutable artifact payloads and their lifecycle metadata
without exposing a database technology to application orchestration or any
intelligence engine.

## Dependency Direction

```text
Released intelligence artifacts
        |
        v
Application orchestration and artifact codecs
        |
        v
Detached persistence requests (metadata + exact bytes)
        |
        v
Storage-neutral persistence port
        |
        +-------------------+
        v                   v
PostgreSQL adapter      Future adapter
```

Intelligence engines publish immutable artifacts only. They do not import or
call the persistence package. Artifact codecs and application orchestration
produce the exact byte stream and persistence metadata. The persistence port
does not import `rie`, `lie`, or a language-specific artifact package.

An adapter may depend inward on the neutral port. The neutral port may not
depend outward on an adapter, SQL driver, database row model, or migration
package.

## In Scope

- repository registration, lookup, and archival lifecycle;
- scan creation, lookup, cancellation, and failure transitions;
- content-addressed exact-payload staging;
- caller-declared size and SHA-256 verification;
- immutable artifact envelopes and dependency edges;
- one atomic scan publication transition;
- exact payload streaming and integrity verification;
- bounded metadata, diagnostics, statistics, and projection persistence;
- idempotent retries and explicit lifecycle conflicts;
- repository-scoped reads and writes;
- retention marking, bounded purge, and unreferenced-payload garbage collection;
- stable error classification independent of PostgreSQL;
- reusable adapter-conformance tests.

## Out of Scope

- running an intelligence engine;
- serializing or interpreting RIE, LIE, or semantic facts;
- changing an artifact after publication;
- PostgreSQL connection construction or pooling;
- environment variables or credential loading;
- running migrations at application startup;
- REST, gRPC, React, authentication, or authorization policy decisions;
- dependency-injection wiring or service startup;
- business rules unrelated to repository, scan, artifact, and publication
  durability;
- network downloads, external tool execution, embeddings, AI, or code changes.

## Port Capabilities

The public contract is divided into small capabilities. An application may
request the minimum capability it needs. A convenience `Port` may compose all
capabilities, but adapters must remain substitutable at the smaller interface
boundaries.

| Capability | Responsibility |
|---|---|
| `RepositoryStore` | Register, retrieve, list, and archive repositories. |
| `ScanStore` | Begin scans, read status, and apply terminal failure/cancellation transitions. |
| `PayloadStager` | Consume and verify exact bytes, then stage them by digest. |
| `PublicationStore` | Validate and atomically publish a complete scan manifest. |
| `ArtifactReader` | Read envelopes and stream exact authoritative bytes. |
| `IntegrityVerifier` | Recompute and compare stored size/digest evidence. |
| `RetentionStore` | Mark, purge in bounded batches, and garbage-collect safely. |

The port deliberately does **not** expose `TransactionManager`, raw
transactions, SQL executors, or caller-controlled transaction callbacks.
Atomicity belongs to lifecycle operations. Exposing a generic transaction API
would allow callers to create unsupported partial-publication states and would
leak adapter behavior into application logic.

The detailed candidate Go surface is specified in
[`PERSISTENCE_PORT_CANDIDATE.md`](../API/PERSISTENCE_PORT_CANDIDATE.md).

## Core Values

All public values are storage-neutral and detached:

- opaque application-generated repository, scan, artifact, publication, and
  request identifiers;
- repository authorization scope supplied for every operation;
- artifact name and semantic version;
- stable-ID scheme name/version;
- codec name/version and media type;
- exact unsigned byte count;
- SHA-256 digest as a fixed 32-byte value;
- safe producer and source-artifact references;
- location-free dependency edges between artifact identities;
- bounded diagnostics, statistics, and projection envelopes;
- opaque pagination cursors;
- safe audit actor and correlation identifiers.

The neutral contract does not expose table names, UUID driver types,
PostgreSQL arrays, JSONB, `bytea`, SQLSTATE, transaction handles, connection
objects, absolute repository paths, credentials, or raw SQL errors.

## Artifact Lifecycle

```text
Register repository
        |
        v
Begin scan
        |
        v
Stage each exact payload independently
        |
        v
Verify declared size and SHA-256
        |
        v
Submit complete publication manifest
        |
        v
Validate artifacts and dependencies
        |
        v
Publish atomically
        |
        v
Read / verify / retain
        |
        v
Archive -> mark for purge -> bounded purge -> payload GC
```

A payload may be staged before publication, but it is not a visible repository
artifact. Only an immutable envelope attached to a succeeded scan through a
publication certificate makes an artifact visible to readers.

A valid repository with no published scan is not an error. A scan publication
must contain at least one artifact and at most the accepted schema limit of 256
artifacts. One artifact may declare no more than 4,096 dependencies.

Published artifacts and succeeded scans are immutable. Corrections require a
new scan and new publication. Archival and retention change visibility and
lifecycle metadata; they never rewrite authoritative bytes.

The v1 persistence contract validates declared edge identity, ordering,
same-scan ownership, exact source name/version, duplicate edges, and self-loops.
It does not infer transitive semantic meaning or add a database-specific
whole-graph cycle rule that is absent from the frozen schema. If an artifact
contract requires a directed acyclic dependency graph, application
orchestration validates that contract before publication and records a normal
`invalid_dependency` failure.

## Payload Contract

- Exact serialized bytes are authoritative.
- The caller declares digest and byte count before staging.
- The adapter consumes the input stream, counts bytes, and computes SHA-256.
- Success requires exact agreement with both declarations.
- A digest mismatch, size mismatch, short read, read failure, cancellation, or
  operational-limit violation rolls back the attempted stage.
- The operational maximum is 4 GiB; the schema ceiling is 8 GiB.
- PostgreSQL uses fixed ordered 4 MiB chunks, but chunk size is an adapter
  detail and is absent from the neutral port.
- An already-present digest does not permit the adapter to trust unverified
  caller input. The submitted stream must still be fully consumed and verified
  before returning idempotent success.
- The port never truncates, normalizes, decodes, re-encodes, or silently changes
  a payload.

Exact retrieval streams bytes to a caller-provided writer and verifies the
stored byte count and digest. A writer may have received partial bytes when an
error occurs, so the caller must treat output as untrusted until the operation
returns success. APIs that later expose downloads must therefore write to a
temporary or otherwise uncommitted destination before making output visible.

## Transaction Boundaries

### Repository and Scan Lifecycle

Repository registration, scan creation, cancellation, and failure are each
short compare-and-set transactions. A terminal scan cannot return to a mutable
state.

### Payload Staging

One complete payload is staged per transaction. The adapter writes the payload
record and ordered chunks while calculating the digest. It commits only after
the stream is complete and verified. Multiple payloads are never tied together
in one long staging transaction.

### Atomic Publication

One final transaction:

1. locks the target repository and scan lifecycle rows;
2. verifies repository scope and that the scan is publishable;
3. verifies every staged payload, artifact envelope, codec, size, digest, and
   operational limit;
4. validates exact same-scan dependency identities and prevents dependency
   self-loops or duplicates;
5. inserts immutable artifact envelopes and dependency edges;
6. stores bounded rebuildable projections, diagnostics, and statistics with
   source-digest provenance;
7. records the publication manifest digest and certificate;
8. changes the scan to `succeeded`;
9. optionally changes the repository current-scan pointer;
10. appends the audit event;
11. commits once.

Readers observe either the previous published state or the complete new state.
They never observe a subset of a publication.

### Reads

Metadata reads use short read-only transactions. Exact payload export obtains
a consistent ordered chunk stream and verifies it before reporting success.
Reads never lock a repository for publication-sized durations.

### Retention

Archive, purge, and garbage-collection work in bounded transactions. A payload
is removed only when no artifact envelope references it and the approved safety
interval has elapsed. Every material deletion records an audit event.

### Forbidden Transaction Scope

A storage transaction must never include engine execution, artifact
serialization, multiple payload staging operations, an HTTP/gRPC request body,
user callbacks, migration execution, backup/restore, or external network/tool
work.

## Cancellation and Ambiguous Commits

Adapters check `context.Context` before database acquisition, before and during
bounded stream/chunk work, before publication validation, and before commit.
Cancellation before a successful commit returns `canceled` and leaves no
partial stage or publication.

If connectivity is lost while commit status is unknown, the port reports an
`unavailable` error marked as retryable. Callers resolve ambiguity by retrying
with the same request and idempotency identity or by reading the durable record.
They must not generate a replacement identity for an ambiguous attempt.

## Idempotency Rules

| Operation | Same identity and same content | Same identity and different content |
|---|---|---|
| Register repository | Return the existing record. | `idempotency_conflict` |
| Begin scan | Return the existing scan. | `idempotency_conflict` |
| Stage payload | Consume/verify input; return `already_present`. | `integrity_failure` |
| Publish scan | Return the existing receipt when manifest digest matches. | `idempotency_conflict` |
| Fail/cancel scan | Return the same terminal state. | `lifecycle_conflict` |

Idempotent success is a normal result with an explicit disposition; it is not
reported as an error.

## Error Contract

Every failure has one stable storage-neutral kind:

- `invalid_input`
- `not_found`
- `idempotency_conflict`
- `lifecycle_conflict`
- `duplicate_artifact`
- `invalid_dependency`
- `unsupported_version`
- `payload_too_large`
- `integrity_failure`
- `authorization_denied`
- `timeout`
- `canceled`
- `unavailable`
- `internal`

The error may contain a safe operation name, retryability, opaque repository or
scan identifier, artifact identity, and a shortened digest. It must not expose
SQL text, table/column/constraint names, connection strings, credentials,
payload bytes, absolute paths, or driver internals.

Context cancellation and deadlines remain discoverable through `errors.Is`.
The PostgreSQL adapter maps driver and SQLSTATE failures internally. Unknown
database failures map conservatively to `internal` or `unavailable`, never to a
guessed semantic category.

## PostgreSQL Adapter Responsibility

The PostgreSQL adapter owns only:

- parameterized SQL and row conversion;
- short transaction execution and isolation choices;
- repository/scan locking and compare-and-set lifecycle transitions;
- ordered 4 MiB `bytea` chunk writes and reads;
- streaming size and SHA-256 verification;
- immutable envelope, dependency, projection, audit, and publication writes;
- constraint and driver-error translation;
- idempotency checks;
- bounded retention and garbage-collection queries.

It does not own artifact serialization, semantic interpretation, engine
execution, user authorization decisions, API presentation, environment/secret
loading, connection-pool construction, runtime migrations, deployment,
business workflows, or projection generation.

## Immutability and Detachment

Request slices, maps, and byte values are caller-owned and may be reused after
a method returns. The implementation must not retain mutable caller memory.
Constructors validate and defensively copy request collections before use.
Returned records and receipts expose copies or read-only value types.

The port does not return a mutable in-memory artifact. Exact bytes are streamed;
metadata views are detached values. This mirrors the immutable artifact
contracts already released by the intelligence engines.

## Planned Package Boundaries

No package is created during this design gate. The accepted implementation is
expected to follow the project package standard:

```text
backend/persistence/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    *_test.go
    *_benchmark_test.go

backend/persistence/conformance/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    *_test.go
    *_benchmark_test.go

backend/internal/storage/postgres/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    *_test.go
    *_benchmark_test.go
```

`backend/persistence` contains only neutral values and interfaces.
`conformance` contains the reusable adapter contract suite. The internal
PostgreSQL package contains SQL and driver translation.

## Validation Strategy

After design acceptance, the implementation must provide an adapter-independent
conformance suite driven by a disposable store factory. The PostgreSQL adapter
must pass:

- empty and invalid input tests;
- defensive-copy and returned-view immutability tests;
- exact byte round trips and SHA-256 corruption detection;
- same-request retry and conflicting-request tests;
- failed/short/cancelled stream rollback tests;
- publication atomicity and visibility tests;
- same-scan dependency, duplicate, and self-loop policy tests;
- concurrent stage and publication tests;
- ambiguous-commit recovery tests where reproducible;
- scope-isolation and safe-error tests;
- cross-scope denial tests for every public read, list, verification, write,
  publication, and retention operation;
- bounded retention, purge, and garbage-collection tests;
- migration compatibility against a disposable PostgreSQL database;
- full backend regression, vet, shuffled tests, targeted/full race tests;
- repeatable benchmarks using released small, OpenTelemetry, and Kubernetes
  artifacts.

The accepted Phase 3.2 measurements remain the baseline: metadata p95 remained
1.643 ms with 1,000,050 dependency rows, publication p95 was 130.15 ms against
the 500 ms gate, and 4 MiB staging met the accepted warm throughput gate for
the largest released payload. The implementation may improve these values but
may not weaken an accepted gate without a new reviewed decision.

Benchmark reports must record CPU, RAM, storage, OS, PostgreSQL configuration,
driver/client configuration, workers, cache state, payload digest/size, sample
count, latency distribution, throughput, WAL, and peak client/database memory.

## API Evolution

The candidate contract begins at `0.1.0`. It is not frozen by this document.
Implementation evidence may refine naming and ergonomics without changing the
accepted ownership, lifecycle, atomicity, integrity, or error semantics.

The contract may become `1.0.0` only after:

- neutral package review;
- PostgreSQL conformance validation;
- public API and error review;
- immutability and dependency audit;
- large-payload and cancellation validation;
- regression, race, security, and benchmark gates;
- explicit engineering acceptance.

## Future Extension Points

- external object storage for exact payloads behind the same logical port;
- generated projection rebuilders;
- approved multi-tenant scoping after a tenancy ADR;
- deployment health/version checks;
- incremental and resumable staging if measured requirements justify it;
- event/outbox publication only after a real consumer is approved.

None of these extensions is part of Phase 3.4.1.
