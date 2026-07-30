# Repository Service 1.0 Release Candidate Contract

## Status

- Phase: 4.0.8 design
- Status: Proposed freeze surface
- Current implementation version: `0.1.0`
- Proposed version after final acceptance: `1.0.0`
- Version promotion: unauthorized
- Date: 2026-07-30

This document identifies the existing candidate API proposed for stabilization.
It does not change source code or authorize a release.

## Capability interfaces

The proposed `1.0.0` contract freezes these exact capability shapes:

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

`Service` is convenience composition only. Consumers should depend on the
narrowest capability they use.

## Frozen behavioral contract

- All operations require explicit repository scope.
- Scope, repository, and scan IDs are canonical lowercase UUID strings.
- Requests and returned values are immutable and constructor-validated.
- Returned slices and byte-bearing values are detached defensive copies.
- Repository registration and scan execution have explicit idempotency rules.
- A repository scan is synchronous and uses keyed in-process single-flight.
- Cancellation is explicit; one waiter cannot cancel other interested callers.
- Artifacts are invisible until atomic publication succeeds.
- Ambiguous publication outcomes are reconciled against the complete manifest.
- Lists use bounded deterministic pagination and stable ordering.
- Artifact export streams exact authoritative bytes and verifies the receipt.
- Source handles are opaque and never durable or observable.
- Absolute source paths never appear in persistence, service results, logs,
  metrics, diagnostics, canonical manifests, or public IDs.

## Stable error kinds

The proposed `1.0.0` error vocabulary is:

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

Storage errors, SQLSTATE values, driver errors, source paths, handles,
credentials, and payload contents never cross the public boundary. Retryability
and `errors.Is` cancellation/deadline behavior are part of conformance.

## Frozen versioned algorithms

| Contract | Freeze candidate |
| --- | --- |
| Public artifact identity | `repository-service-artifact-id/v1` |
| Physical storage artifact identity | `repository-service-storage-artifact-id/v1` |
| Publication manifest | `repository-service-manifest/v1` |
| Analysis profile identity | `repository-service-profile/v1` |
| Supported analysis profile | `repository-go/v1` with exact released digest |
| Canonical codec | `canonical-json/1.0.0` |

Canonical preimages, byte encodings, prefixes, field order, sorting rules, and
golden outputs remain those recorded in
`REPOSITORY_SERVICE_INTEGRATION_GOLDEN_VECTORS.md`. Any incompatible revision
requires a new scheme or codec version.

## Versioning rules

For `1.x`:

1. Existing capability interfaces receive no new methods.
2. Existing exported signatures and observable meanings do not break.
3. Optional capabilities use new interfaces and separate acceptance gates.
4. New profile behavior uses a new profile version and digest.
5. Unknown future values fail safely unless a contract explicitly defines
   forward-compatible handling.
6. No compatibility promise is made for unexported implementation details.
7. The neutral service contract does not define a transport wire format.

## Release evidence required

Promotion from `0.1.0` to `1.0.0` requires all evidence in
`REPOSITORY_SERVICE_STABILIZATION_VALIDATION_PLAN.md`, an explicit disposition
of every open release qualification, an engineering acceptance decision, and
a reviewed commit before annotated tags are created.

## Not part of 1.0

- remote repository acquisition;
- asynchronous or distributed execution;
- repository command execution or mutation;
- transports, authentication, authorization, UI, IDE, or AI behavior;
- dependency or architecture intelligence beyond released artifacts.
