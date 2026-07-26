# Repository Service Candidate API

## Status

- Phase: 4.0 design
- Contract version: `0.1.0`
- Status: Design-approved candidate on 2026-07-27
- Transport: none
- Phase 4.0.1 spike implementation: authorized
- Production implementation: not authorized

This document defines Go service semantics. It is not an HTTP, gRPC, CLI, or
authorization contract.

## Contract identity

```go
const ContractVersion = "0.1.0"
```

The candidate may change before `1.0.0`. Once frozen, breaking changes require
a new major version. Persistence, runtime, and artifact versions evolve
independently.

## Capability interfaces

```go
type RepositoryLifecycleService interface {
    RegisterRepository(context.Context, RegisterRepositoryRequest) (Repository, error)
    GetRepository(context.Context, RepositoryQuery) (Repository, error)
    ListRepositories(context.Context, RepositoryListRequest) (RepositoryPage, error)
    ArchiveRepository(context.Context, ArchiveRepositoryRequest) (Repository, error)
}

type ScanExecutionService interface {
    ExecuteScan(context.Context, ExecuteScanRequest) (ScanResult, error)
    GetScan(context.Context, ScanQuery) (Scan, error)
    ListScans(context.Context, ScanListRequest) (ScanPage, error)
    CancelScan(context.Context, CancelScanRequest) (Scan, error)
}

type ArtifactQueryService interface {
    GetArtifact(context.Context, ArtifactQuery) (Artifact, error)
    ListArtifacts(context.Context, ArtifactListRequest) (ArtifactPage, error)
    ExportArtifact(context.Context, ExportArtifactRequest, io.Writer) (ExportReceipt, error)
}

type Service interface {
    RepositoryLifecycleService
    ScanExecutionService
    ArtifactQueryService
}
```

Callers should consume the smallest interface that satisfies their use case.

## Identity types

```go
type ScopeID string
type PrincipalID string
type RequestID string
type RepositoryID string
type ScanID string
type ArtifactID string
type Cursor string
type SourceHandle string
```

IDs are bounded validated machine values. They are not paths, database keys,
credentials, authorization tokens, or metric labels.

## Already-authorized scope

```go
type OperationScope interface {
    ScopeID() ScopeID
    PrincipalID() PrincipalID
}
```

The service validates shape but makes no authorization decision. A future host
must authorize the operation before constructing the scope.

## Repository lifecycle

```go
type RegisterRepositoryParams struct {
    Scope        OperationScope
    RequestID    RequestID
    RepositoryID RepositoryID
    DisplayName  string
    Source       SourceHandle
}

type RegisterRepositoryRequest interface {
    Scope() OperationScope
    RequestID() RequestID
    RepositoryID() RepositoryID
    DisplayName() string
    // SourceHandle is sensitive process-local routing data. It is never
    // returned, persisted, logged, or used as a metric label.
    SourceHandle() SourceHandle
}
```

Registration resolves the source, computes the normalized source proof, and
persists only that proof. A retry with the same request ID and normalized
values is idempotent.

```go
type Repository interface {
    RepositoryID() RepositoryID
    DisplayName() string
    SourceKind() string
    SourceFingerprintScheme() string
    SourceFingerprint() string
    State() RepositoryState
    CurrentScanID() ScanID
    CreatedAt() time.Time
    UpdatedAt() time.Time
}

type RepositoryState string // active | archived | purge_pending
```

Repository queries carry scope plus repository ID. List requests carry a
bounded page size and opaque cursor. Archive requests also carry request ID.

No repository response contains an absolute path or source handle.

## Scan execution

```go
type ExecuteScanParams struct {
    Scope        OperationScope
    RequestID    RequestID
    RepositoryID RepositoryID
    ScanID       ScanID
    Source       SourceHandle
    Profile      AnalysisProfile
}

type ExecuteScanRequest interface {
    Scope() OperationScope
    RequestID() RequestID
    RepositoryID() RepositoryID
    ScanID() ScanID
    SourceHandle() SourceHandle
    Profile() AnalysisProfile
}

type AnalysisProfile interface {
    Name() string       // repository-go
    Version() string    // 1
    Digest() string     // lowercase SHA-256 of canonical profile bytes
}
```

Phase 4.0 accepts only `repository-go/v1` and its frozen digest. Unknown
profiles fail with `invalid_input`; the service never substitutes a profile.

```go
type ScanResult interface {
    Scan() Scan
    Artifacts() []Artifact
    Disposition() Disposition
}

type Scan interface {
    RepositoryID() RepositoryID
    ScanID() ScanID
    ProfileName() string
    ProfileVersion() string
    ProfileDigest() string
    SourceRevision() string
    State() ScanState
    ReasonCode() string
    RequestedAt() time.Time
    StartedAt() time.Time
    FinishedAt() time.Time
}

type ScanState string // requested | running | succeeded | failed | canceled
type Disposition string // created | already_present | joined
```

`ExecuteScan` is synchronous. Success means publication is durably visible.
It never means merely queued or accepted. No worker, queue, or background task
is part of this candidate.

`CancelScan` is idempotent for a non-published running scan. A succeeded scan
cannot be canceled. Canceling a waiter does not automatically cancel a shared
leader execution.

## Artifact query and export

```go
type Artifact interface {
    ArtifactID() ArtifactID
    ScanID() ScanID
    Name() string
    Version() string
    StableIDScheme() string
    CodecName() string
    CodecVersion() string
    MediaType() string
    PayloadDigest() string
    PayloadSize() uint64
    ProducerName() string
    ProducerVersion() string
    CreatedAt() time.Time
}

type ExportReceipt interface {
    PayloadDigest() string
    PayloadSize() uint64
}
```

Artifact payload export is streaming. The service does not return `[]byte` and
does not decode the authoritative payload during export. Scope isolation and
SHA-256 verification remain mandatory.

## Immutable construction

Mutable `*Params` values are accepted only by constructors. Constructors:

- validate every identifier, enum, timestamp, page size, profile, and handle;
- deep-copy slices and byte fields;
- reject unknown fields in any configuration representation;
- return immutable request/value types with getters;
- never store caller contexts, writers, readers, raw errors, or source roots.

Zero values are invalid unless explicitly documented as an optional field.

## Required dependency interfaces

These are application-side capabilities, not public transport APIs.

```go
type AdmissionController interface {
    Acquire(context.Context) (WorkLease, error)
}

type WorkLease interface {
    Context() context.Context
    Done()
}

type SourceResolver interface {
    Resolve(context.Context, SourceResolveRequest) (AuthorizedSource, error)
}

type AuthorizedSource interface {
    RootPath() string // analysis adapter only; never durable or observable
    Kind() string
    FingerprintScheme() string
    Fingerprint() [32]byte
    Revision() string
    Close(context.Context) error
}

type AnalysisRunner interface {
    Analyze(context.Context, AnalysisRequest) (AnalysisResult, error)
}

type AnalysisResult interface {
    Profile() AnalysisProfile
    Artifacts() []ArtifactCandidate
}

type ArtifactCandidate interface {
    Name() string
    Version() string
    StableIDScheme() string
    ProducerName() string
    ProducerVersion() string
    Dependencies() []ArtifactDependency
}

type ArtifactMaterializer interface {
    Materialize(context.Context, ArtifactCandidate) (MaterializedArtifact, error)
}

type MaterializedArtifact interface {
    Descriptor() ArtifactDescriptor
    Open(context.Context) (io.ReadCloser, error)
    Close(context.Context) error
}
```

`ArtifactCandidate` is an opaque bridge; only the matching registered codec
adapter can inspect its underlying immutable artifact value.

The internal `ServiceStore` mirrors required repository, scan, stage,
publication, and artifact read operations in service-owned models. Its
Persistence Port adapter owns all model translation and error mapping.

## Deterministic artifact manifest

Artifact candidates are sorted by `(name, version, stable ID scheme)` before
materialization. Duplicate names are rejected. Dependencies use the order
declared by the frozen analysis profile and receive stable ordinals.

The initial required dependency graph is:

```text
RIE artifacts: declared by the frozen RIE artifact graph
GoLanguageInventory:
  RepositorySnapshot
  LanguageInventory
GoPackageIdentityInventory:
  RepositorySnapshot
  GoLanguageInventory
GoSemanticInventory:
  RepositorySnapshot
  GoLanguageInventory
  GoPackageIdentityInventory
```

Artifact IDs are explicit and deterministic within a scan using a versioned
scheme proposed as `repository-service-artifact-id/v1`. The exact algorithm
must be frozen by the implementation design spike before `1.0.0`.

## Stable error contract

```go
type ErrorKind string

const (
    ErrorInvalidInput           ErrorKind = "invalid_input"
    ErrorNotFound               ErrorKind = "not_found"
    ErrorConflict               ErrorKind = "conflict"
    ErrorIdempotencyConflict    ErrorKind = "idempotency_conflict"
    ErrorScanAlreadyRunning     ErrorKind = "scan_already_running"
    ErrorOrphanedScan           ErrorKind = "orphaned_scan"
    ErrorSourceUnavailable      ErrorKind = "source_unavailable"
    ErrorAnalysisFailed         ErrorKind = "analysis_failed"
    ErrorMaterializationFailed  ErrorKind = "materialization_failed"
    ErrorIntegrityFailure       ErrorKind = "integrity_failure"
    ErrorPersistenceUnavailable ErrorKind = "persistence_unavailable"
    ErrorTimeout                ErrorKind = "timeout"
    ErrorCanceled               ErrorKind = "canceled"
    ErrorInternal               ErrorKind = "internal"
)
```

Errors expose kind, operation, retryable flag, and a bounded safe reason code.
`Unwrap` preserves only `context.Canceled` and `context.DeadlineExceeded`.
Engine, codec, filesystem, persistence, SQL, pgx, and driver errors remain
private.

## Configuration candidate

```go
type ConfigParams struct {
    FinalizationTimeout       time.Duration
    MaximumArtifactsPerScan  int
    MaximumArtifactBytes     uint64
    MaterializerMemoryBytes  uint64
    MaximumDiagnostics       int
    MaximumConcurrentScans   int
}
```

Candidate defaults:

- finalization timeout: 5 seconds;
- maximum artifacts per scan: 64;
- maximum artifact bytes: 4 GiB;
- in-memory materializer threshold: 8 MiB;
- materializer working-memory budget: 64 MiB;
- maximum diagnostics: inherited from the frozen profile;
- maximum concurrent scans: bounded by runtime admission and configuration.

Configuration is immutable after construction. Limits may become stricter in
staging/production but may not exceed released persistence limits.

## API freeze gate

The candidate remains `0.1.0` until conformance and integration evidence proves:

- immutable request/response behavior;
- deterministic artifact/profile identities;
- exact idempotency and flight-joining semantics;
- source-root non-persistence;
- bounded materialization and cleanup;
- complete scan publication or explicit terminal failure;
- scope isolation on every read and write;
- stable errors and cancellation behavior;
- no engine, persistence, runtime, or transport boundary violation.
