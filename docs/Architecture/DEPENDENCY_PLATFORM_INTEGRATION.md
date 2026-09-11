# Phase 5.0.5 — Dependency Platform Integration

## Status and review package

Design preparation authorized on 2026-09-11. **Proposed for design review only.**
Canonical previous acceptance: `fca668d`. Phase 5.0.4 is closed; ADR 0022 is
Accepted. Dependency Intelligence remains candidate `0.1.0`.
No implementation, integration testing, downstream validation, or release is
authorized. DIE-HARDEN-001 and DIE-HARDEN-002 remain open.

Review this document together with:

- [Codec and publication specification](DEPENDENCY_PLATFORM_CODEC.md)
- [Candidate API](../API/DEPENDENCY_PLATFORM_INTEGRATION_CANDIDATE_API.md)
- [ADR 0023](../Decisions/0023-dependency-platform-integration.md)
- [Validation architecture and plan](../Validation/DEPENDENCY_PLATFORM_INTEGRATION_VALIDATION_PLAN.md)
- [Phase 5 roadmap](../Roadmap/PHASE_5_DEPENDENCY_INTELLIGENCE_ROADMAP.md)

## Decision proposed for review

Add a separate, opt-in dependency publication capability. It consumes an already
constructed immutable `die.DependencyInventory`, seals its canonical bytes, and
publishes a dedicated scan through Persistence Port 1.0.0. A narrow composition
adapter borrows Runtime Infrastructure 1.0.0 admission, ingest, and read
capabilities. It owns neither runtime nor PostgreSQL resources.

This is **artifact publication integration**, not a new repository scanning
service. The caller explicitly runs the accepted Go adapter and, if desired,
Analyzer before submitting the inventory. Publication must not trigger analysis,
source discovery, source rereading, or `Core.Normalize`.

The optional Repository Service profile extension is **deferred**, not assumed
to exist. This is an explicit design trade-off requiring review acceptance.
End-to-end validation will compose accepted engines with this capability in a
test host; no new production engine orchestrator is proposed.

## Compatibility evidence and rejected shortcuts

Inspection of the accepted implementation establishes the following constraints:

| Existing seam | Observed constraint | Proposed treatment |
|---|---|---|
| `repository.New` / `NewExecuteScanRequest` | Private registry contains only `DefaultRepositoryGoProfile`; other profiles are rejected | Do not alter default construction or manufacture private requests |
| `adapters.Adapter.Prepare` | Requires the exact released profile | Do not disguise dependency work as `repository-go/v1` |
| Adapter analysis session | Typed engine store and materializer are private | Do not reach into private state, use reflection, or rerun analysis to pretend it is the same execution |
| `integration.New` | Constructs the released adapter internally | Do not claim arbitrary preparer injection is supported by this constructor |
| `integration.NewStore` | Can receive a registry but still requires the released profile | This alone does not make the public request constructor extensible |
| Persistence Port | Supports generic artifact/codec/producer identities and profile digests | Use its existing public constructors directly in a separate integration adapter |
| Runtime | Public ingest/read/admission capabilities already exist | Borrow those capabilities; no pool inspection or ownership transfer |

Source files inspected: `backend/service/repository/implementation.go`,
`backend/service/repository/scan/interface.go`,
`backend/internal/service/repository/adapters/implementation.go`,
`backend/internal/service/repository/integration/implementation.go`,
`backend/persistence/interface.go`, and
`backend/internal/runtime/postgres/interface.go` at governance baseline `fca668d`.

A future service extension needs its own reviewed construction/profile/history
contract. A breaking change belongs in a separately governed major version.
No released request, default profile, registry, materializer, manifest, codec,
stable-ID scheme, migration, or runtime configuration is modified here.

## Ownership and package direction

| Component | Owns | Must not own |
|---|---|---|
| Accepted DIE core/Go adapter/Analyzer | Dependency facts and optional graph analysis | Storage, runtime, codec I/O policy |
| Proposed `backend/die/codec` | Versioned canonical encoding from public immutable values | SQL, runtime, source paths, decoder/rehydration |
| Proposed `backend/internal/integration/dependency` | Request validation, sealing, publication/reconciliation/export, neutral persistence translation | Repository discovery, engine execution, pools, migrations |
| Proposed runtime bridge in that integration package | Admission lease translation and capability borrowing | TLS loading, environment loading, shutdown of runtime |
| Process host | Input production, repository registration/routing, configuration, private spool location, runtime lifecycle | Redefining frozen engine facts |
| Persistence/adapter | Durability, transactions, exact payload integrity, retention | Artifact serialization or dependency reasoning |

Only the integration layer imports both DIE and persistence/runtime capabilities.
Neither `die` nor the Go adapter imports integration, persistence, runtime, or
Repository Service. No new public service interface is added to Repository
Service 1.0.0. New capability/API and codec remain candidate 0.1.0.

## Repository and provenance boundary

The host supplies an existing, active, scope-isolated persistence repository ID.
This capability does not register, archive, clone, or discover repositories.
Use a **dedicated integration repository record**, not a repository record whose
scan history is managed by Repository Service 1.0.0. Its reader resolves only its
frozen profile registry and can reject an unknown profile in a mixed history.
The host must route these records separately. `MakeCurrent` is always false.
Construction requires an immutable allowlist of exact (scope, repository) pairs
assigned by the host to this integration. Every operation checks it before I/O;
it is routing policy, not proof that another process cannot misuse the same IDs.
The same physical checkout may underlie both records, but that association is
host policy, not an inferred durable identity or an authorization mechanism.

The trusted in-process caller supplies the inventory and a safe opaque source
revision. This is not an untrusted JSON ingestion API. Registration and input
origin are host responsibilities. Scope isolation is enforced on every durable
operation but does not authenticate the caller.

The accepted Go adapter currently emits four `SourceArtifacts` name/version
references without artifact payload digests. Therefore:

- Preserve these references exactly; do not fill absent digests with a storage hash.
- Preserve file/manifest digests and upstream proof limitations unchanged.
- Do not assert cryptographic common-scan provenance or filesystem freshness.
- Publish one inventory envelope with **no persistence dependency edges** in this
  milestone. Its source references are logical provenance, not same-scan FKs.
- Do not point persistence edges into an earlier scan: the frozen port requires
  both endpoints in the same publication.
- Persisting a verified upstream artifact closure or supporting typed reload for
  new analysis is deferred to a separately reviewed extension.

This trade-off limits what a downstream consumer can prove from storage. The
release/validation evidence must state it plainly; a digest proves exact bytes,
not that the supplied graph is true or belongs to a particular source revision.

## Execution and transactions

1. Reject nil/canceled context, invalid request/config, zero inventory, unsupported
   artifact/ID/analysis versions, unsafe revision, or incompatible repository routing.
2. Acquire one runtime admission lease. Use its work context combined with caller
   cancellation. Enforce one active publication per integration instance initially.
   Concurrent calls are rejected with retryable busy; they are not joined.
3. Encode into a private bounded spool while computing SHA-256 and size; close
   and seal it before any scan begins. Derive deterministic identities and manifest.
4. Get the repository through the scoped port and require active state. Begin the
   dedicated scan with the frozen integration profile digest and source revision.
   Persistence is authoritative for lifecycle timestamps and concurrent archival.
5. Reopen the sealed spool and StagePayload with exact digest/size. Require complete
   stream consumption and matching receipt even on idempotent retries.
6. Publish exactly one envelope in one PublishScan transaction, with no projections,
   diagnostics/statistics projections, dependency edges, or current-scan promotion.
7. Verify the durable scan and exact artifact metadata before returning success.
   Reconcile any ambiguous publication result using the rules below.
8. Close readers, remove only owned spool files, and release the lease exactly once.

Sealing, BeginScan, staging, and publication are separate operations. No public
transaction manager and no transaction spanning encoding, analysis, or export.
Failed staging is never visible as a published artifact. Unreferenced payloads
are left to existing retention/GC; this integration never invokes GC automatically.
Only one final inventory is published: base and analyzed variants have the same
artifact name and must not be submitted as duplicate envelopes.

The frozen read port does not expose the stored publication certificate or its
manifest digest: ScanRecord and ArtifactRecord expose only scan/envelope metadata.
Successful Publish receipts must match the requested manifest. Later reconciliation
reconstructs the expected manifest from scoped durable metadata and compares it
with ExpectedPublication; it does not claim to reread the stored certificate.
Direct certificate-corruption inspection is test-only through a disposable DB
observer, not production SQL or a new port method.

## Failure, idempotency, and recovery

| Situation | Required behavior |
|---|---|
| Validation, encoding, quota, or cancellation before Begin | No durable scan created; discard owned spool |
| Begin timeout or lost response | Scoped lookup; do not assume absence or create a replacement scan ID |
| Definite pre-publication staging failure | Bounded detached fail/cancel finalization if the scan is known running; never overwrite a terminal state |
| Publish timeout/unavailable/lost response | Reconcile; never immediately mark a possibly committed scan failed |
| Reconciliation proves exact succeeded publication | Return durable success, including after caller cancellation; caller may receive cancellation before proof completes and must retry/reconcile |
| Succeeded scan differs in profile, revision, reconstructed manifest, artifact ID, codec, producer, size, or hash | Integrity failure; never accept scan ID alone |
| Running or absent state after ambiguous publication | Return outcome-unknown/unavailable with exact IDs; no automatic takeover or fail transition |
| Definite failed/canceled prior scan | Return terminal result; a new logical attempt requires a new scan/request ID |
| Export corrupt, short, trailing, or wrong bytes | Fail verification; do not return a successful receipt |
| Process/DB crash | Reopen host runtime, use durable IDs to reconcile; no in-memory state is authoritative |

Idempotency binds scope/repository/scan, profile digest, source revision, artifact
metadata, and exact payload digest/size. Same request ID with different content
is a conflict. Concurrent publishers are ultimately serialized by existing port
transactions, not an asserted distributed single-flight guarantee. A previously
running scan after restart is not silently resumed: reconciliation reports it for
host-directed recovery. Fresh retry encoding must still satisfy size/hash checks.

## Resource, cancellation, and secret boundaries

Candidate limits: one concurrent publication, 4 GiB maximum artifact/spool bytes,
64 MiB in-memory spool threshold, 1 MiB copy buffer, and five-second bounded
finalization/cleanup attempt. Settings may only lower byte/concurrency ceilings;
finalization may be configured from one to thirty seconds. Zero means documented
default, never unlimited. Partial output on quota exhaustion is not published.
The host provisions private spool storage outside any analyzed repository and
ensures free-space/OS permissions; paths never enter durable metadata or errors.

The 4 GiB limit is inherited from persistence, not evidence that every graph of
that encoded size can be analyzed on the reference workstation. Defensive
accessor copies, maps, sorting, and individual JSON records remain memory costs.
Encode one top-level collection at a time, release its copy, and avoid a whole
inventory View plus a second full JSON byte slice. Count limits are not hard heap
ceilings. Both open hardening items stay open.

Check cancellation before each accessor, each record, each copy read/write, and
each port call. A blocked caller writer or filesystem/driver operation is not
made interruptible by spawning an abandoned goroutine. The API requires
cooperating sinks; report uninterruptible units and measured latency honestly.
Cleanup cannot delete an in-use spool just to meet a deadline: defer to explicit
host recovery for an owned orphan. Never delete arbitrary temp-directory files.

Logs/errors contain stable codes and safe correlation IDs only, not inventory
values, source handles, paths, SQL, URLs, or raw dependency errors. Relative code
paths inside the artifact remain permitted existing facts, not log labels.

## Exit and downstream boundaries

Design acceptance must explicitly approve the standalone publication scope,
dedicated repository routing, and logical-only upstream provenance limitation.
If any is unacceptable, revise the design before implementation; do not silently
extend Repository Service or add upstream codecs.

After design approval, record it and wait for separate implementation
authorization. Independent codec/profile/identity/manifest vectors precede their
production encoders. Only then implement and execute the linked validation plan.
Engineering acceptance, Phase 5.0.6, and production release remain separate gates.
