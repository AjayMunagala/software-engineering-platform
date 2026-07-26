# ADR 0016 — Repository Service Layer Ownership and Scan Publication

- Status: Accepted on 2026-07-27
- Date: 2026-07-27
- Scope: Phase 4.0 repository lifecycle, synchronous scan orchestration, artifact materialization, and publication
- Frozen dependencies: RIE `1.0.0`, Go LIE `1.0.0`, Persistence Port `1.0.0`, PostgreSQL Adapter `1.0.0`, Runtime Infrastructure `1.0.0`

## Context

The platform can analyze local repositories, produce immutable repository and
Go-language artifacts, persist exact artifact bytes, and operate those
components through a stable runtime. It does not yet have an application layer
that coordinates those capabilities into one repository lifecycle and one
observable scan outcome.

Putting orchestration into an intelligence engine would couple analysis to
persistence. Putting it in a transport would duplicate behavior across future
REST, gRPC, CLI, and IDE hosts. Allowing persistence to interpret engine
artifacts would make storage own semantic contracts. A dedicated service layer
is therefore required between future transports and the released foundation.

## Decision

1. Introduce a transport-neutral Repository Service Layer downstream from
   future hosts and upstream from analysis, runtime admission, and persistence.
2. Keep every released `1.0.0` contract unchanged. Engines never import the
   service or persistence packages.
3. Split the public service contract into repository lifecycle, scan execution,
   and artifact query capabilities. A composed interface is convenience only.
4. Require a previously authorized operation scope on every public operation.
   Authentication and authorization policy remain outside this phase.
5. Accept an opaque source handle. A deployment-owned resolver supplies one
   proven local source to the analysis adapter. Absolute source paths are never
   persisted, returned, logged, or used as metric labels.
6. Support one frozen analysis profile, `repository-go/v1`. Every scan performs
   a fresh deterministic RIE run and, when Go exists, the released Go syntax,
   package-identity, and semantic stages.
7. Treat Go stages as required when the profile detects Go. Do not silently
   downgrade a failed Go analysis to a repository-only success.
8. Materialize approved durable artifact views once through versioned codecs.
   Compute size and SHA-256 while writing to a sealed bounded spool, then stream
   the exact bytes through Persistence Port for independent verification.
9. Keep database transactions private to Persistence Port operations. Do not
   expose a public service transaction manager.
10. Make `ExecuteScan` synchronous in Phase 4.0. Success means the complete scan
    is atomically published and durably visible, not queued or accepted.
11. Require request IDs and deterministic entity IDs for mutations. Concurrent
    identical in-process scan requests join one keyed execution; conflicting
    reuse fails explicitly.
12. Never guess how to resume a durable running scan after process loss. Report
    `orphaned_scan`; lease-based recovery requires a later ADR.
13. Reconcile an ambiguous publication response against durable scan state
    before finalizing failure. Never mark a successfully published scan failed.
14. Use stable redacted service errors. Raw engine, path, filesystem, codec,
    SQL, pgx, persistence, or driver errors do not cross the boundary.
15. Keep API transports, queues, workers, scheduling, authentication, UI, AI,
    new intelligence, repository cloning, and repository mutation out of scope.

## Rationale

The service layer is the first component that understands the application use
case while remaining independent of deployment transport and physical storage.
It is therefore the correct owner for orchestration, profile selection,
idempotency, cancellation policy, failure classification, and the mapping from
immutable engine artifacts to durable publication manifests.

An exact-byte spool is required because Persistence Port needs the declared
digest and size before staging while large released artifacts cannot safely be
duplicated in memory. Sealing the spool also prevents the bytes from changing
between measurement and staging.

Synchronous execution is deliberately narrow. It provides a complete and
testable service contract before queues, distributed leases, recovery workers,
or scheduling semantics are justified by measured operational requirements.

## Alternatives

### Put orchestration in REST or gRPC handlers

Rejected. Every transport would reproduce lifecycle, idempotency, cleanup,
publication, and error behavior, making correctness dependent on presentation.

### Let intelligence engines write directly to PostgreSQL

Rejected. It violates the released database-independent engine boundary and
makes analysis correctness depend on storage availability.

### Let persistence serialize engine artifacts

Rejected. Persistence must store exact bytes without owning or interpreting
RIE/LIE contracts. Versioned codecs remain outside the storage layer.

### Persist local absolute repository paths

Rejected. Paths are deployment-specific, privacy-sensitive, non-portable, and
not repository identity. Only normalized source proof is durable.

### Return artifact payloads as byte slices

Rejected. Released semantic artifacts can be gigabytes. Query and staging
remain streaming and bounded.

### Add a public transaction manager

Rejected. It lets callers create unsupported partial scan states and leaks
storage transaction semantics into the service contract.

### Start with asynchronous jobs and a queue

Rejected for Phase 4.0. Durable leasing, ownership transfer, retries, and
recovery require a separate design and operational evidence.

### Resume orphaned scans automatically

Rejected. The first implementation has no durable execution lease or safe
checkpoint contract. Restarting or assuming completion would violate the rule
that unknown state must not be guessed.

## Consequences

- A new application service contract and internal adapters are introduced.
- A codec/materializer registry must cover every artifact in the frozen profile.
- Temporary artifact spools require restrictive permissions, bounded cleanup,
  and validation on Windows and Linux.
- Callers wait for complete scan publication in the first release.
- Process loss can leave an explicit orphaned scan and invisible staged payloads;
  accepted retention later reclaims unreferenced content.
- Future transports can share one service contract without importing engines,
  pgx, SQL, runtime pools, or local paths.

## Security Consequences

- The service receives only an already-authorized scope and opaque source handle.
- No source path, authenticated URL, secret, SQL, payload, or raw error appears
  in service responses or telemetry.
- Spools are outside the repository and are not followed through repository
  traversal.
- Repository scope is enforced on every read and write, including exports.
- Analysis remains local, read-only, and without network or command execution.

## Validation Gate

The architecture, candidate API, staged roadmap, and validation plan were
accepted together on 2026-07-27. That acceptance authorizes only the Phase
4.0.1 design spike. Production service implementation remains gated until the
spike validates exact-byte materialization, deterministic artifact identity,
single-flight behavior, scope isolation, path redaction, and publication
reconciliation.

## Authoritative References

- `docs/Architecture/REPOSITORY_SERVICE_LAYER.md`
- `docs/API/REPOSITORY_SERVICE_CANDIDATE_API.md`
- `docs/Roadmap/PHASE_4_REPOSITORY_SERVICE_ROADMAP.md`
- `docs/Validation/REPOSITORY_SERVICE_LAYER_VALIDATION_PLAN.md`
- `docs/API/PERSISTENCE_PORT_V1.md`
- `docs/Architecture/ARTIFACT_DEPENDENCY_GRAPH.md`
- `docs/Decisions/0015-runtime-infrastructure.md`
