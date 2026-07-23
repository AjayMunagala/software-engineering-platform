# Go Semantic Resolution Public API

## Status

- Phase: Phase 2.2.2 accepted; Phase 2.2.3 authorized
- API status: Architecture-approved candidate; not frozen
- Authorization: Phase 2.2.3 only; Phase 2.2.4 and later remain unauthorized
- Package: `backend/lie/golang/semantic`
- Candidate engine version: `0.1.0`
- Candidate artifact: `go-semantic-inventory` `0.1.0`

This document describes the implemented `0.1.0` candidate API. It is not yet a stable or frozen API. The name `LSE_PUBLIC_API.md` is retained as the existing document path; public Go names use `semantic`, not the ambiguous `LSE` acronym.

## Package Boundary

The first semantic resolver is Go-specific. A generic semantic interface is not added to `backend/lie` until a second language implementation demonstrates a shared, stable contract.

```go
package semantic
```

## Input

```go
type Input struct {
    Snapshot          rie.RepositorySnapshot
    Syntax            golang.GoLanguageInventory
    PackageIdentities packageidentity.GoPackageIdentityInventory
}
```

Input rules:

- `Snapshot` must be `repository-snapshot` `1.0.0`.
- `Syntax` must be `go-language-inventory` `1.0.0`.
- `PackageIdentities` must be `go-package-identity-inventory` `0.1.0` during the candidate cycle.
- `Syntax.SourceArtifacts()` must identify the supplied snapshot version.
- The package-identity artifact must identify the supplied snapshot and syntax versions.
- Candidate files must belong to both artifacts.
- Both values are treated as immutable.
- No framework, build, metadata, summary, mutable run context, or report input is accepted.

Source access is not exposed as arbitrary caller callbacks in the candidate API. Phase 2.2.2 reads only authorized paths beneath `Snapshot.RootPath()`, applies boundary checks, and verifies each Go-file SHA-256 digest against the syntax artifact. Package-identity proofs are accepted as a prerequisite but are not consumed to create import bindings in this milestone. Their manifest evidence must be re-hashed immediately before proof consumption when Phase 2.2.5 is authorized; Phase 2.2.2 does not claim that validation prematurely.

## Engine Contract

```go
type Engine interface {
    Name() string
    Version() string
    Language() string
    ArtifactName() string
    Description() string
    Resolve(context.Context, Input) (GoSemanticInventory, error)
}
```

`Resolve` returns an immutable artifact only when run-level invariants remain trustworthy. File/package limitations are represented by outcomes and diagnostics; fatal prerequisite, configuration, cancellation, or invariant failures return an error and no publishable artifact.

## Construction

```go
func DefaultConfig() Config
func New(configs ...Config) (Engine, error)
```

Exactly zero or one `Config` value is accepted. Multiple values return `ErrTooManyConfigs`, matching the established construction style without silently merging configuration.

```go
type Config struct {
    MaxWorkers            int
    MaxSourceFileSize     int64
    MaxPackageFiles       int
    MaxPackageBytes       int64
    MaxDiagnostics        int
    MaxDiagnosticsPerFile int
    MaxRelationships      int
}
```

Configuration rules:

- Zero means “use the documented default” for every field, making `Config{}` valid and forward-compatible.
- Negative values are invalid.
- A positive `MaxWorkers` is between 1 and 8.
- `MaxSourceFileSize` is enforced before reading.
- `MaxPackageFiles` and `MaxPackageBytes` bound one synchronous package type-check.
- `MaxDiagnosticsPerFile` is applied before the global `MaxDiagnostics`; omitted diagnostics are counted.
- `MaxRelationships` bounds emitted semantic relationships and interface candidate evaluations.
- Reaching a bound creates an explicit partial outcome and diagnostic; the engine never silently truncates.
- The syntax artifact already decides whether tests are present, so semantic configuration does not reintroduce an `IncludeTests` switch.

Implemented defaults are: at most 8 workers (also bounded by `GOMAXPROCS`), 10 MiB per source file, 2,000 files and 256 MiB per future package operation, 1,000 diagnostics, 50 diagnostics per file, and 1,000,000 future relationships. Package and relationship limits are reserved for the later milestones that perform those operations; Phase 2.2.2 validates them but does not pretend to use them.

### Configuration Evolution

- Callers should use `DefaultConfig()` and named fields; positional struct literals are unsupported.
- The zero value of every present and future limit field means its documented default.
- A new optional field with zero-default behavior may be added in a minor release without changing `New(configs ...Config)`.
- Removing a field, changing a field's meaning, or changing zero from “default” requires a major version.
- Unknown fields are rejected by any future strict JSON/YAML decoder; they are never silently interpreted.
- A functional-options constructor or generic configuration map will not be added without a separate API review.

## Artifact Identity and Retrieval

```go
const (
    ArtifactName    = "go-semantic-inventory"
    ArtifactVersion = "0.1.0"
)

func InventoryFrom(*rie.ArtifactStore) (GoSemanticInventory, bool)
```

Only the engine constructs a valid artifact. The orchestration layer publishes the returned value through `rie.ArtifactStore.Put`; the engine does not mutate or reset the caller's store.

## Artifact Contract

```go
func (GoSemanticInventory) ArtifactName() string
func (GoSemanticInventory) ArtifactVersion() string
func (GoSemanticInventory) Language() string
func (GoSemanticInventory) Metadata() Metadata
func (GoSemanticInventory) SourceArtifacts() []rie.ArtifactReference
func (GoSemanticInventory) Files() []SemanticFile
func (GoSemanticInventory) Declarations() []SemanticDeclaration
func (GoSemanticInventory) References() []SemanticReference
func (GoSemanticInventory) ReceiverBindings() []ReceiverBinding
func (GoSemanticInventory) ImportBindings() []ImportBinding
func (GoSemanticInventory) TypeRelations() []TypeRelation
func (GoSemanticInventory) InterfaceSatisfaction() []InterfaceSatisfaction
func (GoSemanticInventory) Diagnostics() []lie.Diagnostic
func (GoSemanticInventory) Statistics() SemanticStatistics
```

All collection accessors return deep defensive copies, including nested candidate IDs, evidence, missing-method lists, type arguments, diagnostic locations, and statistics maps.

The semantic artifact references syntax IDs. It does not return or copy `golang.GoSymbol`, `golang.GoFile`, or `golang.GoPackage`; consumers use the retained syntax artifact when those models are needed.

## Error Contract

Planned sentinel errors:

```go
var (
    ErrTooManyConfigs                 error
    ErrInvalidConfig                  error
    ErrMissingRepositorySnapshot      error
    ErrIncompatibleRepositorySnapshot error
    ErrMissingGoLanguageInventory     error
    ErrIncompatibleGoInventory        error
    ErrMissingPackageIdentityInventory error
    ErrIncompatiblePackageIdentity    error
    ErrArtifactProvenanceMismatch     error
    ErrInvalidRepositoryRoot          error
)
```

Context cancellation and deadline errors preserve `errors.Is` behavior. Path/file/type problems and configured-limit exhaustion that do not invalidate the whole run use stable diagnostic codes and semantic outcomes rather than untyped error strings.

## Deterministic Behavior

For equivalent input artifacts and identical verified bytes, the engine guarantees:

- identical IDs;
- identical relationship states;
- stable collection ordering;
- stable diagnostic ordering;
- stable duplicate suppression and diagnostic aggregation;
- no dependence on worker scheduling or Go map iteration;
- no absolute-machine-path data in IDs.

Timing and platform-specific error text are not artifact identity inputs.

Every `Resolve` call is a full rebuild. The API accepts no previous semantic inventory or cache, and the implementation revalidates/reparses all eligible files and reconstructs every package/type relationship. Incremental state requires a separately versioned input and ADR; it cannot be introduced as a hidden optimization unless its output is proven equivalent to a full rebuild.

## Cancellation

Phase 2.2.2 checks context before scheduling work, in every worker, after source I/O, after hashing, and before artifact construction. Later milestones must additionally check at least every 1,024 custom AST nodes and every 256 emitted references/interface candidates, and before/after synchronous parsing and package checking. A synchronous standard-library parse/type-check cannot be interrupted mid-call, so `MaxSourceFileSize`, `MaxPackageFiles`, and `MaxPackageBytes` bound the largest unchecked unit.

## Diagnostics

Diagnostics are deduplicated by stable code/location/message identity, sorted by normalized location/severity/code/message, limited per file, then limited globally. The global limit includes one reserved final slot for the aggregate omission diagnostic; that aggregate is the documented final-item exception to ordinary sorting. When the limit is one and anything is omitted, only that aggregate is emitted. Every limit-suppressed or displaced item contributes to `OmittedDiagnostics`; exact duplicates do not. These rules and code meanings freeze with `1.0.0`.

## Side-Effect Contract

The engine performs local read-only filesystem access required to verify authorized source. It does not:

- execute external commands;
- access the network;
- download or resolve dependencies online;
- modify repository files;
- write repository caches;
- persist source text, ASTs, or `go/types` objects;
- mutate either input artifact.

## Package Structure

```text
backend/lie/golang/semantic/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    implementation_test.go
    implementation_benchmark_test.go
```

Every public symbol must have documentation. Internal parsing, scope, importer, and ID helpers may be split only after measured complexity justifies a subpackage; the eight-file package standard remains the initial release structure.

## Versioning and Freeze Gate

- `0.x` APIs may change only through reviewed design or measured implementation findings.
- The public API may become `1.0.0` only after behavior tests, race tests, repeatable benchmarks, real-repository validation, documentation review, and an explicit freeze decision.
- Patch releases after `1.0.0` are compatible defect fixes only.
- Changed IDs, ordering, source-position meaning, enum meaning, or removals require a major version.
- IDs carry `go-semantic-id/v1`; any post-`1.0.0` re-key requires a new artifact major version and ID-scheme prefix. Migration is a full deterministic rebuild unless a major-version release supplies a proven one-to-one mapping.

No implementation commit may describe this API as frozen before the Phase 2.2 stabilization and release gate is complete.
