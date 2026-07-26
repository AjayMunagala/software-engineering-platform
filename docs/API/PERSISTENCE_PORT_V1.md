# Persistence Port 1.0 Public API

## Status

- Package: `backend/persistence`
- Contract version: `1.0.0`
- State: frozen after Phase 3.4.4 acceptance on 2026-07-26

This document records the frozen neutral Go contract. Phase 3.4.3 evidence
resolved the physical compatibility details and Phase 3.4.4 completed final
API, conformance, compatibility, security, performance, and documentation
review.

## Design Rules

1. No interface imports RIE, LIE, PostgreSQL, a SQL driver, or migrations.
2. Application orchestration supplies detached exact bytes and neutral values.
3. Every operation receives `context.Context` and an explicit repository
   authorization scope.
4. Payloads are streamed; no public API requires a whole artifact in memory.
5. Atomic lifecycle operations own their transactions internally.
6. Read results and request collections are defensively detached.
7. Idempotent success is represented in receipts, not as an error.
8. Stable error kinds do not expose database implementation details.

## Candidate Interfaces

```go
type RepositoryStore interface {
    RegisterRepository(context.Context, RegisterRepositoryRequest) (RepositoryRecord, error)
    GetRepository(context.Context, RepositoryQuery) (RepositoryRecord, error)
    ListRepositories(context.Context, RepositoryListRequest) (RepositoryPage, error)
    ArchiveRepository(context.Context, ArchiveRepositoryRequest) (RepositoryRecord, error)
}

type ScanStore interface {
    BeginScan(context.Context, BeginScanRequest) (ScanRecord, error)
    GetScan(context.Context, ScanQuery) (ScanRecord, error)
    ListScans(context.Context, ScanListRequest) (ScanPage, error)
    FailScan(context.Context, FinishScanRequest) (ScanRecord, error)
    CancelScan(context.Context, FinishScanRequest) (ScanRecord, error)
}

type PayloadStager interface {
    StagePayload(context.Context, StagePayloadRequest, io.Reader) (PayloadReceipt, error)
}

type PublicationStore interface {
    PublishScan(context.Context, PublishScanRequest) (PublicationReceipt, error)
}

type ArtifactReader interface {
    GetArtifact(context.Context, ArtifactQuery) (ArtifactRecord, error)
    ListArtifacts(context.Context, ArtifactListRequest) (ArtifactPage, error)
    ExportPayload(context.Context, PayloadQuery, io.Writer) (PayloadReceipt, error)
}

type IntegrityVerifier interface {
    VerifyPayload(context.Context, PayloadQuery) (VerificationReceipt, error)
}

type RetentionStore interface {
    MarkRepositoryForPurge(context.Context, MarkForPurgeRequest) (RepositoryRecord, error)
    PurgeRepositoryBatch(context.Context, PurgeBatchRequest) (PurgeReceipt, error)
    GarbageCollectPayloads(context.Context, GarbageCollectionRequest) (GarbageCollectionReceipt, error)
}

type Port interface {
    RepositoryStore
    ScanStore
    PayloadStager
    PublicationStore
    ArtifactReader
    IntegrityVerifier
    RetentionStore
}
```

The method names and signatures are the proposed `1.0.0` public surface. The
capability split and absence of a generic transaction API are architectural
decisions.

## Candidate Identity and Integrity Values

```go
type RepositoryID string
type ScanID string
type ArtifactID string
type RequestID string
type Cursor string

type Digest [32]byte
type ByteCount uint64

type Scope struct {
    ScopeID     string
    PrincipalID string
}

type VersionedName struct {
    Name    string
    Version string
}

type Codec struct {
    Name      string
    Version   string
    MediaType string
}

type SourceIdentity struct {
    Kind              string
    FingerprintScheme string
    Fingerprint       Digest
}
```

Opaque IDs are application-generated and validated by constructors. The
PostgreSQL adapter requires UUID-form repository, scan, artifact, projection,
and scope IDs because those are frozen physical keys, but the neutral port does
not expose a database-driver UUID type. Request IDs remain neutral machine
identities and are deterministically mapped to audit correlation UUIDs by the
adapter. `ScopeID` is an application-owned
authorization namespace; it is not a PostgreSQL role or a premature tenancy
model. `PrincipalID` is an already authenticated opaque actor reference. The
persistence port enforces supplied scope/target coherence but does not perform
authentication or decide access policy.

## Candidate Requests

```go
type RegisterRepositoryRequest struct {
    Scope         Scope
    RequestID     RequestID
    RepositoryID RepositoryID
    DisplayName  string
    Source       SourceIdentity
    Actor         AuditActor
}

type BeginScanRequest struct {
    Scope                 Scope
    RequestID             RequestID
    RepositoryID          RepositoryID
    ScanID                ScanID
    AnalysisProfileDigest Digest
    SourceRevision        string
    Actor                 AuditActor
}

type StagePayloadRequest struct {
    Scope        Scope
    RequestID    RequestID
    RepositoryID RepositoryID
    ScanID       ScanID
    Digest       Digest
    ExpectedSize ByteCount
}

type PublishScanRequest struct {
    Scope          Scope
    RequestID      RequestID
    RepositoryID   RepositoryID
    ScanID         ScanID
    ManifestScheme string
    ManifestDigest Digest
    Artifacts      []ArtifactSubmission
    Dependencies   []DependencySubmission
    Projections    []ProjectionSubmission
    Diagnostics    []DiagnosticSubmission
    Statistics     []StatisticSubmission
    MakeCurrent    bool
    Actor          AuditActor
}

type ArtifactSubmission struct {
    ArtifactID     ArtifactID
    Artifact       VersionedName
    StableIDScheme string
    Codec           Codec
    PayloadDigest   Digest
    PayloadSize     ByteCount
    Producer        VersionedName
}

type DependencySubmission struct {
    ConsumerArtifactID ArtifactID
    Ordinal            uint32
    SourceArtifactID   ArtifactID
    DeclaredArtifact   VersionedName
}

type ProjectionSubmission struct {
    ProjectionID       string
    ArtifactID         ArtifactID
    SourceDigest       Digest
    Projector          VersionedName
    SchemaVersion      string
    DigestScheme       string
    ProjectionDigest   Digest
    CanonicalJSON      []byte
    RecordCount        uint64
}

type DiagnosticSubmission struct {
    ProjectionID string
    Ordinal      uint32
    Severity     string
    Code         string
    Engine       string
    RelativePath string
    Line         uint32
    Column       uint32
    Message      string
}

type StatisticSubmission struct {
    ProjectionID string
    Key          string
    Value        StatisticValue
    Unit         string
}
```

Artifact envelopes contain only fields represented by the accepted physical
schema. Arbitrary key/value metadata is intentionally excluded; query-oriented
data belongs in bounded, digest-verified projections.

Each request has a constructor that validates required fields, limits,
semantic versions, safe strings, scope coherence, and duplicate keys, then
defensively copies collections. Zero-value request structs are invalid.

`PublishScanRequest` contains one to 256 artifacts. Each artifact may have zero
to 4,096 dependencies. Projection/diagnostic/statistic limits follow the
accepted schema and configuration contract; exceeding them fails explicitly
and never truncates silently.

Projection JSON is a bounded rebuildable query document, never authoritative
artifact data. The constructor verifies valid JSON, a maximum canonical input
size of 8 MiB, and its declared SHA-256. The adapter may store a database-native
projection after validation, but exact authoritative artifact bytes remain only
in payload storage.

`StatisticValue` is a tagged union with exactly one integer, exact decimal,
boolean, or bounded text value. Binary floating point is not accepted for
deterministic projected counters or percentages. Diagnostics reject absolute
paths, payload excerpts, credentials, and unsanitized/unbounded messages.

## Candidate Read Models

Records are detached immutable views with these conceptual fields:

| Record | Required fields |
|---|---|
| `RepositoryRecord` | scope, repository ID, source identity, display name, lifecycle state, optional current scan ID, durable timestamps |
| `ScanRecord` | scope, repository ID, scan ID, analysis-profile digest, optional source revision, lifecycle state, requested/started/finished timestamps, safe terminal reason |
| `ArtifactRecord` | repository/scan/artifact IDs, artifact version, optional stable-ID scheme, codec, producer, payload digest/size, immutable timestamp |
| `PublicationReceipt` | scan ID, manifest scheme/digest, artifact count, disposition |

Repository lifecycle is `active`, `archived`, or `purge_pending`. Scan
lifecycle is `requested`, `running`, `succeeded`, `failed`, or `cancelled`.
Only legal compare-and-set transitions are accepted. There is no public setter
for lifecycle state or durable timestamps.

Every `Get`, `List`, archive, scan, payload, artifact, publication, and
retention request carries `Scope` plus the relevant repository ID. List methods
are bounded and return a record slice plus an opaque continuation cursor.
Returned ordering is stable: repositories and scans use descending durable
creation/request time with ID as the tie-breaker; artifacts use artifact name
and artifact ID.

## Candidate Receipts

```go
type Disposition string

const (
    DispositionCreated        Disposition = "created"
    DispositionAlreadyPresent Disposition = "already_present"
)

type PayloadReceipt struct {
    Digest      Digest
    Size        ByteCount
    Disposition Disposition
}

type PublicationReceipt struct {
    ScanID         ScanID
    ManifestScheme string
    ManifestDigest Digest
    ArtifactCount  uint32
    Disposition    Disposition
}

type VerificationReceipt struct {
    Digest Digest
    Size   ByteCount
}
```

Durable timestamps are storage-owned and returned in records. Callers cannot
choose creation/publication timestamps. Tests use adapter-internal clock
control rather than time fields in production requests.

## Candidate Errors

```go
type ErrorKind string

const (
    ErrorInvalidInput        ErrorKind = "invalid_input"
    ErrorNotFound            ErrorKind = "not_found"
    ErrorIdempotencyConflict ErrorKind = "idempotency_conflict"
    ErrorLifecycleConflict   ErrorKind = "lifecycle_conflict"
    ErrorDuplicateArtifact   ErrorKind = "duplicate_artifact"
    ErrorInvalidDependency   ErrorKind = "invalid_dependency"
    ErrorUnsupportedVersion  ErrorKind = "unsupported_version"
    ErrorPayloadTooLarge     ErrorKind = "payload_too_large"
    ErrorIntegrityFailure    ErrorKind = "integrity_failure"
    ErrorAuthorizationDenied ErrorKind = "authorization_denied"
    ErrorTimeout             ErrorKind = "timeout"
    ErrorCanceled            ErrorKind = "canceled"
    ErrorUnavailable         ErrorKind = "unavailable"
    ErrorInternal            ErrorKind = "internal"
)

type Error struct {
    Kind      ErrorKind
    Operation string
    Retryable bool
    // Safe opaque context only; the wrapped cause is never serialized/logged
    // without an explicit redaction boundary.
}
```

The implementation will provide `KindOf(error)`, `Retryable(error)`, and
`errors.Is` support for cancellation and deadlines. SQLSTATE, constraint names,
driver errors, SQL, connection strings, absolute paths, and payloads remain
inside the PostgreSQL adapter.

## Streaming Semantics

`StagePayload` consumes the complete reader on success, even when the digest is
already stored. It calculates the digest and size while streaming and commits
only verified bytes. The caller must not mutate/reuse a custom reader
concurrently.

`ExportPayload` writes in exact stored order. Because `io.Writer` cannot be
rolled back, bytes written before an error are untrusted. Callers expose output
only after success. The returned receipt is the proof of completed streaming
and verification.

Neither operation closes the caller-owned reader or writer.

## Pagination and Ordering

List operations use bounded page sizes and opaque cursors. Their ordering is
stable and documented per record type. A cursor is adapter-issued and must not
encode SQL or expose row identifiers beyond the neutral contract. Invalid,
expired, or scope-mismatched cursors return `invalid_input`.

## Configuration Boundary

Neutral validation limits are explicit immutable configuration. The release candidate
includes operational payload size (default 4 GiB), artifact/dependency limits,
bounded page sizes, safe-string limits, and retention batch limits.

PostgreSQL hosts, ports, credentials, TLS, connection pooling, statement
timeouts, and driver configuration are not neutral port configuration and are
deferred beyond this design gate.

## Compatibility Policy

- The frozen contract is identified by `persistence.ContractVersion` as
  `1.0.0`.
- The accepted lifecycle, exact-byte authority, integrity, atomicity,
  idempotency, scope, and safe-error semantics require a reviewed ADR to change.
- Existing exported names, signatures, interface method sets, validation
  behavior, and zero-value defaults do not change incompatibly in `1.x`.
- Adding a method to an existing capability interface or `Port` is breaking;
  optional capabilities use new standalone interfaces.
- Compatible symbols or fields may be added only when existing behavior and
  zero-value semantics remain unambiguous.
- Breaking request/receipt or error-kind changes require a major version.
- The Go values are not a JSON wire contract. Exact intelligence-artifact bytes
  remain separately versioned and authoritative.
- PostgreSQL schema, codec, projector, artifact, and port versions evolve
  independently.

## Approval Gate

Engineering review must answer:

1. Are engines completely independent of the port?
2. Are serialized bytes produced outside persistence?
3. Are public values storage-neutral and bounded?
4. Are transaction boundaries owned by lifecycle methods?
5. Are stage, publication, retry, and ambiguous-commit rules complete?
6. Can every failure be classified without leaking PostgreSQL details?
7. Can future adapters implement the interfaces without SQL semantics?
8. Can a conformance suite verify every mandatory behavior?
9. Does every public operation, including reads and lists, reject a target from
   a different authorization scope without revealing whether it exists?

Engineering accepted this design on 2026-07-24, the neutral implementation on
2026-07-25, the PostgreSQL adapter on 2026-07-26, and the final Phase 3.4.4
freeze on 2026-07-26.
