# Dependency Platform Integration — Candidate API 0.1.0

2026-09-11. Design-only proposal; none of the declarations below are implemented.
Design approval and implementation authorization are separate gates.
See [architecture](../Architecture/DEPENDENCY_PLATFORM_INTEGRATION.md),
[codec](../Architecture/DEPENDENCY_PLATFORM_CODEC.md), and
[validation plan](../Validation/DEPENDENCY_PLATFORM_INTEGRATION_VALIDATION_PLAN.md).

## Capability boundary

Proposed `backend/internal/integration/dependency` is an application integration
component, not a new capability embedded in `repository.Service`. Constructor
dependencies expose only the frozen persistence capabilities actually used and
an admission abstraction and immutable host-approved (scope, repository) routing
allowlist. The runtime bridge implements that abstraction using
the released `app.Work` context/Done contract. Callers own runtime startup,
readiness, shutdown, credentials and repository registration.

Conceptual signatures (models remain private-field, constructor-validated values):

```go
type Publisher interface {
    Publish(context.Context, PublishRequest, die.DependencyInventory) (Receipt, error)
}
type Reader interface {
    Get(context.Context, Query) (Publication, error)
    Export(context.Context, Query, io.Writer) (ExportReceipt, error)
    Reconcile(context.Context, ExpectedPublication) (Reconciliation, error)
}
type Admission interface {
    Acquire(context.Context) (Lease, error)
}
type Lease interface {
    Context() context.Context
    Done()
}
```

The dependency constructor accepts immutable config, a Persistence Contract
1.0.0, scoped RepositoryStore/ScanStore/PayloadStager/PublicationStore/ArtifactReader
capabilities, Admission, and a host-owned private spool factory. No pgx/pool,
SQL, authenticated URL, source resolver, ArtifactStore, engine executor, public
TransactionManager, or runtime constructor appears in the capability contract.
Typed nil dependencies are rejected safely; constructors perform no I/O.

Proposed `backend/die/codec` exposes an encoder accepting context, immutable
inventory and writer and returning exact bytes-written/SHA receipt on success.
It performs no source/storage/runtime operations. It has no Unmarshal or Decode
method. Its config is independent of integration configuration.

## Models and invariants

| Model | Validated contents / behavior |
|---|---|
| PublishRequest | Scope/principal, opaque bounded request ID, lowercase UUID repository/scan IDs, safe opaque source revision; no source handle or path |
| Query | Scope/principal and exact repository/scan IDs; no global lookup by digest |
| ExpectedPublication | Query plus frozen profile, manifest and exact envelope metadata from a previously sealed request; immutable, bounded |
| Publication | Detached durable scope/repository/scan/artifact identity, profile, reconstructed manifest, revision, codec, producer, digest/size, durable lifecycle timestamps |
| Receipt | Publication plus `published` or `reconciled`; no claim that fresh analysis ran |
| Reconciliation | `published_exact`, `running_unknown`, `missing_unknown`, `failed`, or `canceled`; mismatched success is an integrity error |
| ExportReceipt | Exact delivered digest/size only after verified complete copy |
| Error | Stable kind, operation/reason code, retry classification and optional outcome-unknown flag; no raw cause formatting |

Requests contain no caller-selected codec, producer, manifest scheme, profile,
arbitrary dependency lists, projections, MakeCurrent flag, or mutable inventory
view. Those are fixed by the publication profile. Query and export are scoped
independently of a previous successful Publish call. Formatting of all values
is redacted; full source revisions and inventory contents are never error text.

The input may be an accepted normalized or analyzed inventory. A zero inventory,
unsupported metadata/version/scheme, invalid UTF-8, or inconsistent analysis
envelope is rejected; absent analysis is not equivalent to computed-empty.
No automatic analysis or inference of globally complete topology occurs.
Structural validity does not prove the trusted caller's claimed repository origin.

## Spool ownership

The host provides a private factory, not a caller arbitrary payload stream.
An integration-owned spool supports write-until-seal, seal receipt, read-only
reopen, and idempotent close/remove. Reopen after disposal fails. File handles
are closed before removal on Windows. Writer callbacks cannot supply a fabricated
digest: integration recomputes it and staging independently verifies it.

Use memory up to the configured threshold, then spill within the total spool
quota. The factory enforces exclusive creation and private access permissions;
no file name contains repository names, revisions, IDs or source paths. Allocation
and cleanup failures produce safe codes. No symlink traversal into arbitrary
targets; validate the owned directory/handle boundary. Local spool mutation is
not repository mutation. It does not grant general filesystem access to DIE.

The initial config proposes MaxArtifactBytes <= 4 GiB, MemorySpoolBytes <= 64 MiB,
CopyBufferBytes <= 1 MiB, one active publication/export per instance and bounded
finalization/cleanup of 1–30 seconds (default 5). Memory threshold must not exceed
artifact quota. Reject inconsistent config; zero fields select defaults, never
unbounded work. Spool budget is shared by operations in an instance so concurrent
export cannot bypass it. Get/Reconcile metadata work remains separately bounded.

No hard total-heap claim: accessor copies and large records remain uninterruptible
allocation units. Defensive-copy/config tests do not close DIE-HARDEN-001/002.

## Error and cancellation semantics

Stable proposed kinds: `invalid_input`, `unsupported_contract`, `scope_not_found`,
`conflict`, `busy`, `limit_exceeded`, `integrity_failure`, `unavailable`, `timeout`,
`canceled`, `internal`. Same-scope lookup absence is safe; unauthorized cross-scope
queries must not reveal whether another scope contains that ID/digest.

Only transient busy/unavailable/timeout conditions may be retryable. Retry does
not mean a publication is known not to exist. An outcome-unknown result carries
safe exact query IDs and requires Reconcile. Integrity failures are not silently
retried or repaired. Raw persistence/runtime/OS errors are translated privately.

Nil context is invalid input on every public method. Admission denial starts no
durable write. Lease cancellation and caller cancellation stop new work; the
detached bounded finalization path may still prove a committed success. Never
finalize a possibly committed publication as failed simply because its caller
left. Done is called exactly once after owned resources are no longer in use.
No abandoned copy goroutines to simulate cancellation of an uncooperative writer.

## Compatibility and deferred interfaces

No default Repository Service profile changes. The standalone integration's
repository records must not be routed to the frozen service's scan-history
reader. The host chooses a dedicated repository record before publication; the
integration cannot infer that choice from SQL or a filesystem path.

No persistent graph QueryEngine facade, typed payload loader, upstream artifact
closure import, or profile extension is proposed. Adding these later requires
explicit design and authorization, including how to validate decoded immutable
values and source-byte provenance. A custom registry alone is not sufficient to
extend the current Repository Service request constructor.

Candidate APIs may be refined only with recorded design review and validation.
No change here promotes DIE, the codec, or integration to 1.0.0.
