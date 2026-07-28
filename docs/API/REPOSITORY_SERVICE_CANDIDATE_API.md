# Repository Service Candidate API

## Status

- Phase: 4.0 design
- Contract version: `0.1.0`
- Status: Phase 4.0.2 accepted on 2026-07-27
- Transport: none
- Phase 4.0.1 spike: accepted on 2026-07-27
- Phase 4.0.2 neutral contract implementation: accepted
- Phase 4.0.3 repository lifecycle: accepted on 2026-07-27
- Phase 4.0.4 scan execution core: accepted on 2026-07-28
- Phase 4.0.5 intelligence and materialization adapters: locally complete, awaiting acceptance

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

type Digest [32]byte

type SourceHandle struct { /* private sensitive value */ }
```

IDs are bounded validated machine values. They are not paths, database keys,
credentials, authorization tokens, or metric labels.

## Already-authorized scope

`Scope` is a constructor-validated immutable value exposing `ScopeID()` and
`PrincipalID()`. Request values store this concrete detached value instead of
an implementation supplied by a caller.

The service validates shape but makes no authorization decision. A future host
must authorize the operation before constructing the scope.

## Repository lifecycle

```go
type RegisterRepositoryParams struct {
    Scope        Scope
    RequestID    RequestID
    RepositoryID RepositoryID
    DisplayName  string
    SourceHandle string
}

type RegisterRepositoryRequest struct { /* private fields */ }

// Accessors:
/*
    Scope() Scope
    RequestID() RequestID
    RepositoryID() RepositoryID
    DisplayName() string
    // SourceHandle is sensitive process-local routing data. It is never
    // returned, persisted, logged, or used as a metric label.
    SourceHandle() SourceHandle
*/
```

`SourceHandle.String`, `SourceHandle.GoString`, and the containing request
formatters are redacted. `Reveal` is reserved for the later authorized source
resolver. It must never be persisted, returned, logged, or used as a label.

Registration resolves the source, computes the normalized source proof, and
persists only that proof. A retry with the same request ID and normalized
values is idempotent.

```go
type Repository struct { /* private fields */ }

// Accessors:
/*
    RepositoryID() RepositoryID
    DisplayName() string
    SourceKind() string
    FingerprintScheme() string
    Fingerprint() Digest
    State() RepositoryState
    CurrentScanID() ScanID
    CreatedAt() time.Time
    UpdatedAt() time.Time
*/

type RepositoryState string // active | archived | purge_pending
```

Repository queries carry scope plus repository ID. List requests carry a
bounded page size and opaque cursor. Archive requests also carry request ID.

No repository response contains an absolute path or source handle.

## Scan execution

```go
type ExecuteScanParams struct {
    Scope        Scope
    RequestID    RequestID
    RepositoryID RepositoryID
    ScanID       ScanID
    SourceHandle string
    Profile      AnalysisProfile
}

type ExecuteScanRequest struct { /* private fields */ }

// Accessors:
/*
    Scope() Scope
    RequestID() RequestID
    RepositoryID() RepositoryID
    ScanID() ScanID
    SourceHandle() SourceHandle
    Profile() AnalysisProfile
*/

type AnalysisProfile struct { /* private fields */ }

// Accessors:
/*
    Name() string       // repository-go
    Version() string    // 1
    Digest() Digest     // exact SHA-256 of canonical profile bytes
*/
```

Phase 4.0 accepts only `repository-go/v1` and its frozen digest. Unknown
profiles fail with `invalid_input`; the service never substitutes a profile.

```go
type ScanResult struct { /* private fields */ }

// Accessors:
/*
    Scan() Scan
    Artifacts() []Artifact
    Disposition() Disposition
*/

type Scan struct { /* private fields */ }

// Accessors:
/*
    RepositoryID() RepositoryID
    ScanID() ScanID
    Profile() AnalysisProfile
    SourceRevision() string
    State() ScanState
    ReasonCode() string
    RequestedAt() time.Time
    StartedAt() time.Time
    FinishedAt() time.Time
*/

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
type Artifact struct { /* private fields */ }

// Accessors:
/*
    ArtifactID() ArtifactID
    ScanID() ScanID
    Name() string
    Version() string
    StableIDScheme() string
    CodecName() string
    CodecVersion() string
    MediaType() string
    PayloadDigest() Digest
    PayloadSize() uint64
    ProducerName() string
    ProducerVersion() string
    CreatedAt() time.Time
*/

type ExportReceipt struct { /* private fields */ }

// Accessors:
/*
    PayloadDigest() Digest
    PayloadSize() uint64
*/
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

## Future implementation dependency interfaces

The following design remains reserved for later authorized implementation
milestones. These interfaces are not part of the Phase 4.0.2 package and are
shown only to preserve the accepted dependency direction.

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
scheme named `repository-service-artifact-id/v1`.

Phase 4.0.1 freezes the candidate preimage as:

```text
ASCII "repository-service-artifact-id/v1" followed by one NUL byte
uint32-be length + exact UTF-8 repository ID
uint32-be length + exact UTF-8 scan ID
uint32-be length + exact UTF-8 artifact name
uint32-be length + exact UTF-8 artifact version
uint32-be length + exact UTF-8 stable-ID scheme
```

Fields are non-empty, already trimmed, valid UTF-8, at most 1,024 bytes, and
contain no ASCII control characters. The output is `rsaid1_` followed by the
lowercase hexadecimal SHA-256 digest of the preimage.

The frozen spike golden vector is:

```text
repository ID: repo-001
scan ID: scan-01
artifact: go-semantic-inventory
artifact version: 1.0.0
stable-ID scheme: go-semantic-id/v1
artifact ID: rsaid1_3c55ac33a130d92a42bd4f782ad7868d9310b94e3fbb91cc3ba9abb85df8fce8
```

Changing the prefix, field order, field encoding, validation, or digest
requires a new identity-scheme version. Engineering accepted this candidate
representation with the Phase 4.0.1 spike on 2026-07-27. It remains at contract
version `0.1.0` until later conformance and integration gates are accepted.

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
type Config struct {
    FinalizationTimeout     time.Duration
    MaxArtifactsPerScan     int
    MaxArtifactBytes        uint64
    MaterializerMemoryBytes uint64
    MaxDiagnostics          int
    MaxConcurrentScans      int
    MaxPageSize             int
    MaxDisplayNameBytes     int
    MaxSourceHandleBytes    int
}
```

Candidate defaults:

- finalization timeout: 5 seconds;
- maximum artifacts per scan: 64;
- maximum artifact bytes: 4 GiB;
- materializer working-memory budget: 64 MiB;
- maximum diagnostics: 10,000;
- maximum concurrent scans: 64;
- maximum page size: 1,000;
- maximum display name: 256 bytes;
- maximum opaque source handle: 1,024 bytes.

Configuration is immutable after construction. Limits may become stricter in
staging/production but may not exceed released persistence limits.

## Analysis profile registry

`ProfileRegistry` is immutable and returns detached definitions. The initial
`repository-go` version `1` definition contains the seven released RIE
artifacts plus Go language, package-identity, and semantic inventories. Its
digest is calculated from versioned canonical profile bytes. Exact
name/version/digest matching is required; the contract never substitutes an
unknown profile.

## Phase 4.0.2 evidence

- neutral contract statement coverage: 95.3%;
- reusable conformance harness statement coverage: 85.4%;
- target and full-backend regression, vet, shuffle, and race: pass;
- Windows and Ubuntu race validation: pass, zero races;
- two final five-second fuzz campaigns: 1,176,977 executions, no panic;
- dependency audit: no RIE, LIE, persistence, PostgreSQL, pgx, runtime, SQL,
  filesystem, command, network, or transport dependency;
- conformance first validates scope isolation on reads, lists, export, and
  mutations using a thread-safe fake adapter.

## Phase 4.0.3 candidate implementation evidence

- production lifecycle package implements only register, get, list, and
  archive coordination;
- opaque source handles resolve into immutable path-free proof and are never
  passed to the atomic store;
- versioned SHA-256 mutation fingerprints support atomic store-owned
  idempotency and conflict detection;
- additive lifecycle conformance validates same-ID cross-scope isolation,
  idempotency, archival, pagination, cancellation, and cleanup;
- lifecycle/conformance coverage is 85.4%/85.3%;
- Windows and Ubuntu targeted and full-backend race validation pass with zero
  races;
- implementation evidence was accepted on 2026-07-27.

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
