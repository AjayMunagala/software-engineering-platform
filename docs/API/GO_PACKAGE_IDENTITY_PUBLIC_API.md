# Go Package Identity Public API

## Status

- Milestone: Phase 2.2.1
- Package: `backend/lie/golang/packageidentity`
- Candidate artifact: `GoPackageIdentityInventory 0.1.0`
- Stability: candidate; not frozen as `1.0.0`
- Authorization: Phase 2.2.1 only

## Responsibility

Deterministically prove how imports in `GoLanguageInventory 1.0.0` map to Go packages present in `RepositorySnapshot 1.0.0`, using only snapshot-authorized local manifest evidence.

## Input

```go
type Input struct {
    Snapshot rie.RepositorySnapshot
    Syntax   golang.GoLanguageInventory
}
```

Both artifacts are immutable prerequisites. The engine rejects missing, incompatible, or structurally inconsistent inputs. It does not modify either input.

## Engine

```go
type Engine interface {
    Name() string
    Version() string
    ArtifactName() string
    Description() string
    Analyze(context.Context, Input) (GoPackageIdentityInventory, error)
}

func New(configs ...Config) (Engine, error)
```

`New()` accepts zero or one configuration. `Analyze` performs a complete rebuild; it does not reuse an earlier identity inventory.

## Configuration

```go
type Config struct {
    MaxWorkers            int
    MaxManifestSize       int64
    MaxDiagnostics        int
    MaxDiagnosticsPerFile int
}
```

Zero fields select documented defaults. Worker count is capped at eight. Invalid negative or out-of-range values are rejected. Additive fields may be introduced during the candidate period; incompatible meaning changes require a new artifact major after `1.0.0`.

## Artifact

```go
type GoPackageIdentityInventory struct { /* private state */ }

func (GoPackageIdentityInventory) ArtifactName() string
func (GoPackageIdentityInventory) ArtifactVersion() string
func (GoPackageIdentityInventory) Metadata() Metadata
func (GoPackageIdentityInventory) SourceArtifacts() []rie.ArtifactReference
func (GoPackageIdentityInventory) Contexts() []ResolutionContext
func (GoPackageIdentityInventory) Modules() []ModuleIdentity
func (GoPackageIdentityInventory) Proofs() []PackageIdentityProof
func (GoPackageIdentityInventory) Diagnostics() []lie.Diagnostic
func (GoPackageIdentityInventory) Statistics() PackageIdentityStatistics
```

Construction is private. Every collection accessor returns a deep copy, including evidence ranges, context evidence, candidate IDs, and statistics maps.

`InventoryFrom` retrieves a previously published artifact from `rie.ArtifactStore`. Phase 2.2.1 does not alter the frozen Phase 2.1 runner; orchestration may publish the returned artifact with `ArtifactStore.Put`.

## Resolution Contexts

- `single-module`: the importing package's nearest module.
- `workspace`: an applicable repository `go.work` context.
- `module-vendor`: automatic module vendoring is applicable.
- `workspace-vendor`: automatic workspace vendoring is applicable.
- `unmanaged`: the importing package has no owning repository module.

Every managed context contains manifest files, main module IDs, and location-aware evidence explaining why the context exists. Unmanaged contexts intentionally have no fabricated evidence.

## Proof Status

- `resolved`: exactly one repository package is proven.
- `unresolved`: local evidence is applicable but incomplete or has no target.
- `ambiguous`: multiple equal-precedence repository candidates remain.
- `external`: no approved local mapping exists.
- `stale`: reserved for consumers that recheck evidence digests and detect change.

The engine does not classify standard-library imports by path shape. Until an approved exact-version standard-library index exists, such imports remain external.

## Determinism

Contexts, modules, proofs, evidence, diagnostics, and candidate IDs have stable ordering independent of manifest discovery order and worker count. Proof IDs use `go-package-proof-id/v1` with UTF-8 byte-length-prefixed values.

## Error Boundary

Fatal errors are limited to invalid configuration, cancellation, missing/incompatible prerequisites, provenance mismatches, and an invalid repository root. Per-manifest failures are bounded diagnostics so valid independent manifests can still produce facts.

## Security and Side Effects

The engine:

- reads only manifest paths present in `RepositorySnapshot`;
- rejects lexical and symlink escapes;
- applies a manifest-size limit;
- parses `go.mod` and `go.work` in process with `golang.org/x/mod/modfile`;
- performs no command execution, package loading, module-cache access, network access, downloads, or repository writes.

## Deferred API

Semantic relationships, source re-parsing, digest revalidation by consumers, exact standard-library indexing, module-version selection, incremental execution, and external dependency loading are outside Phase 2.2.1.

