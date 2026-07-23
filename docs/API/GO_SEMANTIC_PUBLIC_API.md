# Go Semantic Public API

## Status

- Package: `backend/lie/golang/semantic`
- Candidate artifact: `GoSemanticInventory 0.1.0`
- Proposed stable artifact: `GoSemanticInventory 1.0.0`
- Stability: stabilized release candidate; freeze approval pending

The version constants remain `0.1.0` until Phase 2.2.9 is explicitly accepted.

## Responsibility

Digest-verify snapshot-authorized Go source and deterministically synthesize a
bounded immutable semantic inventory from `RepositorySnapshot`,
`GoLanguageInventory`, and `GoPackageIdentityInventory`.

## Construction and Execution

```go
func New(configs ...Config) (Engine, error)
func NewIntegrator(configs ...Config) (Integrator, error)
func InventoryFrom(*rie.ArtifactStore) (GoSemanticInventory, bool)
```

`New` and `NewIntegrator` accept zero or one configuration. Every execution is
a fresh full rebuild. The integrator retrieves prerequisites by exact typed
artifact, publishes exactly one semantic artifact, and retains no semantic
state.

## Configuration

```go
type Config struct {
    MaxWorkers            int   `json:"max_workers"`
    MaxSourceFileSize     int64 `json:"max_source_file_size"`
    MaxPackageFiles       int   `json:"max_package_files"`
    MaxPackageBytes       int64 `json:"max_package_bytes"`
    MaxDiagnostics        int   `json:"max_diagnostics"`
    MaxDiagnosticsPerFile int   `json:"max_diagnostics_per_file"`
    MaxRelationships      int   `json:"max_relationships"`
}
```

Zero fields select documented bounded defaults. Workers are capped at eight.
Negative or unsafe values are rejected. New fields may be added with zero
meaning a documented default; incompatible meaning changes require a major
version.

## Artifact

`GoSemanticInventory` has private construction and implements `rie.Artifact`
and `lie.LanguageArtifact`. Its public accessors are:

```go
Metadata() Metadata
SourceArtifacts() []rie.ArtifactReference
Files() []SemanticFile
Declarations() []SemanticDeclaration
References() []SemanticReference
ReceiverBindings() []ReceiverBinding
ImportBindings() []ImportBinding
TypeRelations() []TypeRelation
InterfaceSatisfaction() []InterfaceSatisfaction
Diagnostics() []lie.Diagnostic
Statistics() SemanticStatistics
View() GoSemanticInventoryView
MarshalJSON() ([]byte, error)
```

All returned collections and nested slices, maps, evidence ranges, and
diagnostic locations are detached. JSON encoding serializes the detached view
and uses explicit empty arrays.

## Stable Wire Semantics

- Enums serialize to documented lower-case strings and reject invalid or
  unknown values.
- Ordering is deterministic and part of the contract.
- Locations use repository-relative paths and UTF-8 byte offsets plus
  one-based line/column positions.
- `go-semantic-id/v1` is the stable identifier scheme.
- Re-keying after `1.0.0` requires a new artifact major and ID scheme.
- Omitted relationships and diagnostics are counted explicitly.
- Missing evidence produces unresolved, ambiguous, external, partial, stale,
  failed, or unknown states; it never produces a guessed fact.

## Compatibility

The proposed `1.0.0` contract freezes exported type names, fields, JSON keys,
enum strings, stable-ID algorithms, ordering, position meaning, error
boundaries, default configuration behavior, and deep immutability. Additive
fields are allowed when old consumers can safely ignore them. Removing or
reinterpreting an existing contract element requires a new artifact major.

## Side-Effect Boundary

The engine executes no repository tools, reads no module cache, performs no
network access or downloads, and writes no repository files. It stores no
source, AST, token set, or `go/types` state in the artifact.
