# Repository Service Persistence & Runtime Integration Candidate API

## Status

- Phase: 4.0.6 design
- Version: `0.1.0`
- Status: Accepted with recommendations on 2026-07-28
- Production implementation: Authorized under the frozen golden-vector rules
- Date: 2026-07-28

This document describes internal application integration boundaries. It does
not add methods to Persistence Port `1.0.0`, PostgreSQL Adapter `1.0.0`, or
Runtime Infrastructure `1.0.0`.

## Proposed integration package

```go
package integration

const ContractVersion = "0.1.0"

type Runtime interface {
    Admit(context.Context) (runtimeapp.Work, error)
    Ingest() runtimepostgres.IngestCapabilities
    Read() runtimepostgres.ReadCapabilities
}

type Dependencies struct {
    Runtime          Runtime
    Persistence      *persistence.Contract
    ServiceContract  *repository.Contract
    SourceResolver   adapters.SourceResolver
    Clock            scan.Clock
}

type Bundle struct {
    service repository.Service
}

func New(Dependencies, ...Config) (*Bundle, error)
func (bundle *Bundle) Service() repository.Service
```

`Bundle` owns no pool, secret, migration, runtime shutdown, engine artifact, or
transport. It only owns service coordinators and stateless adapters.

## Capability adapters

```go
type RepositoryPersistence interface {
    persistence.RepositoryStore
}

type ScanPersistence interface {
    persistence.ScanStore
    persistence.PayloadStager
    persistence.PublicationStore
    persistence.ArtifactReader
}
```

The production adapter receives separate runtime ingest and read views. It
does not recover the complete PostgreSQL adapter through type assertions.

## Required scan-contract additions

```go
type BeginCommand interface {
    Scope() repository.Scope
    RequestID() repository.RequestID
    MutationFingerprint() repository.Digest
    SourceFingerprint() repository.Digest
    Scan() repository.Scan
}

type PublishCommand interface {
    Scope() repository.Scope
    RequestID() repository.RequestID
    Scan() repository.Scan
    Artifacts() []PublicationArtifact
}

type FinalizeCommand interface {
    Scope() repository.Scope
    RequestID() repository.RequestID
    Scan() repository.Scan
}

type PublicationArtifact interface {
    Metadata() repository.Artifact
    Dependencies() []ArtifactDependency
    Open(context.Context) (io.ReadCloser, error)
}
```

These are conceptual method sets; the concrete values remain immutable structs
with defensive-copy accessors. The existing constructors remain internal to
the scan coordinator.

## Runtime admission adapter

```go
type Admission struct {
    runtime Runtime
}

func (admission *Admission) Acquire(context.Context) (scan.WorkLease, error)
```

The returned lease delegates `Context` and `Done` to `runtimeapp.Work` without
exposing the runtime itself.

## Source-proof adapter

```go
type SourceProofAdapter struct {
    resolver adapters.SourceResolver
}

func (adapter *SourceProofAdapter) Resolve(
    context.Context,
    repository.Scope,
    repository.SourceHandle,
) (lifecycle.SourceResolution, error)
```

It converts `AuthorizedSource` evidence to a path-free lifecycle proof and
closes the authorized source through the same bounded ownership path. It never
calls `RootPath`.

## Identifier compatibility

Candidate public construction rules become:

```text
ScopeID      canonical lowercase UUID
RepositoryID canonical lowercase UUID
ScanID       canonical lowercase UUID
RequestID    existing bounded machine value
PrincipalID  existing bounded opaque value
ArtifactID   repository-service-artifact-id/v1
```

The exact UUID grammar, accepted/rejected fixtures, physical UUID algorithm,
and manifest fixtures are normative in
`docs/API/REPOSITORY_SERVICE_INTEGRATION_GOLDEN_VECTORS.md`. All service
constructors and conformance fixtures must use those rules before any
PostgreSQL-backed behavior is considered valid.

Physical artifact IDs use the internal
`repository-service-storage-artifact-id/v1` UUID mapping defined by the
architecture document. No physical UUID appears in a service response.

## Translation functions

The implementation should keep conversion functions narrow and testable:

```go
func toPersistenceScope(repository.Scope) (persistence.Scope, error)
func toAuditActor(repository.Scope) (persistence.AuditActor, error)
func toRepositoryRecord(persistence.RepositoryRecord) (repository.Repository, error)
func toScanRecord(persistence.ScanRecord, *repository.ProfileRegistry) (repository.Scan, error)
func toArtifactRecord(repository.RepositoryID, repository.ScanID, persistence.ArtifactRecord) (repository.Artifact, error)
func toPhysicalArtifactID(repository.ArtifactID) (persistence.ArtifactID, error)
func toServiceArtifactID(repository.RepositoryID, repository.ScanID, persistence.ArtifactRecord) (repository.ArtifactID, error)
```

Every conversion verifies all available identity, scope, version, digest,
size, state, timestamp, and ordering invariants.

## Manifest API

```go
const ManifestScheme = "repository-service-manifest/v1"

func CanonicalManifest(
    repository.Scan,
    []scan.PublicationArtifact,
) ([]byte, error)

func ManifestDigest(
    repository.Scan,
    []scan.PublicationArtifact,
) (repository.Digest, error)
```

`CanonicalManifest` is intended for golden-vector tests and bounded manifests,
not artifact payloads. `ManifestDigest` may hash directly while encoding.

## Persistence-store behavior

The production store implements both existing internal contracts:

```go
var _ lifecycle.Store = (*Store)(nil)
var _ scan.Store = (*Store)(nil)
```

Its constructor requires only neutral capabilities:

```go
func NewStore(
    persistence.RepositoryStore,
    runtimepostgres.IngestCapabilities,
    runtimepostgres.ReadCapabilities,
    *persistence.Contract,
    *repository.ProfileRegistry,
    ...Config,
) (*Store, error)
```

The constructor rejects nil capabilities, incompatible limits, an unknown
profile registry, or more than one configuration.

## Configuration

```go
type Config struct {
    FinalizationTimeout time.Duration
    ReadPageSize        int
    MaxArtifacts        int
    MaxDependencies     int
    MaxPayloadBytes     uint64
}
```

Defaults must not exceed the frozen service/persistence limits. Configuration
contains no DSN, host, database, credential, TLS, pool, path, or secret.

## Error contract

The adapter returns only existing Repository Service errors. It may define
private sentinel errors for tests, but those never cross the service boundary.

Required safe reasons include:

```text
identifier-incompatible
source-proof-mismatch
profile-unrecognized
record-mismatch
payload-receipt-mismatch
manifest-build-failed
publication-ambiguous
physical-artifact-id-mismatch
persistence-contract-failed
runtime-admission-failed
```

## Compatibility

This API remains candidate `0.1.0`. Acceptance authorizes implementation, not
freeze. It may reach `1.0.0` only during Phase 4.0.8 after real-repository and
cross-platform integration evidence is accepted.
