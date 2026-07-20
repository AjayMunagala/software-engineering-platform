# LIE Candidate Public API

## Status

Phase 2.0 design candidate. This API is not frozen and has no implementation yet.

## Compatibility Boundary

LIE depends only on the frozen RIE artifact contracts it needs:

- `RepositorySnapshot 1.0.0`
- `LanguageInventory 1.0.0`
- `rie.Artifact`
- `rie.ArtifactStore`

LIE does not consume `rie.RunContext`, `rie.Report`, RIE summary objects, or mutable discovery entries.

## Core Engine Contract

```go
package lie

type Engine interface {
    Name() string
    Version() string
    Language() string
    ArtifactName() string
    Description() string
    Analyze(context.Context, Input) (LanguageArtifact, error)
}

type LanguageArtifact interface {
    rie.Artifact
    Language() string
}

type Input struct {
    Snapshot rie.RepositorySnapshot
    Languages language.LanguageInventory
}
```

`Input` contains immutable values. Engine configuration is supplied at construction time and copied defensively.

## Runner API

```go
func New(engines ...Engine) (*Runner, error)

func (runner *Runner) Register(engine Engine) error

func (runner *Runner) Run(
    ctx context.Context,
    artifacts *rie.ArtifactStore,
) (RunReport, error)
```

Runner behavior:

1. Validate context, store, configuration, and unique engine language/name/artifact identity.
2. Resolve and version-check both RIE prerequisites before invoking an engine.
3. Invoke engines in registration order.
4. Publish each successful immutable artifact through `ArtifactStore.Put`.
5. Record completed engines and fatal failure in `RunReport`.
6. Leave already published artifacts intact if a later engine fails; partial completion is explicit.

An empty registry is valid and produces an empty successful report. A language engine can be removed without changing runner logic or other engine artifacts.

## RunReport

```go
type RunReport struct {
    startedAt        time.Time
    finishedAt       time.Time
    engines          []EngineMetadata
    published        []rie.ArtifactReference
    fatalDiagnostics []Diagnostic
}
```

Timing is audit metadata, not part of deterministic functional artifacts. Public accessors return timestamps by value and collection fields through defensive copies.

## Go Engine API

```go
package golang

func DefaultConfig() Config
func New(configs ...Config) lie.Engine

func InventoryFrom(
    artifacts *rie.ArtifactStore,
) (GoLanguageInventory, bool)
```

Candidate configuration:

```go
type Config struct {
    MaxWorkers        int
    MaxSourceFileSize int64
    MaxDiagnostics    int
    IncludeTests      bool
}
```

Zero or negative safety limits are invalid; they never mean unlimited.

## Go Artifact Accessors

```go
func (GoLanguageInventory) ArtifactName() string
func (GoLanguageInventory) ArtifactVersion() string
func (GoLanguageInventory) Language() string
func (GoLanguageInventory) Metadata() Metadata
func (GoLanguageInventory) SourceArtifacts() []rie.ArtifactReference
func (GoLanguageInventory) Files() []GoFile
func (GoLanguageInventory) Packages() []GoPackage
func (GoLanguageInventory) Symbols() []GoSymbol
func (GoLanguageInventory) Diagnostics() []lie.Diagnostic
func (GoLanguageInventory) Statistics() Statistics
```

Allocation-sensitive visitor APIs may be added before the 1.0.0 freeze. They must expose values, not mutable internal references.

## Stable Error Categories

Candidate sentinel errors:

```text
ErrContextRequired
ErrArtifactStoreRequired
ErrSnapshotRequired
ErrLanguageInventoryRequired
ErrArtifactVersionMismatch
ErrEngineRequired
ErrDuplicateEngine
ErrInvalidConfig
ErrLanguageInventoryMismatch
```

Errors wrap contextual details while preserving `errors.Is` behavior. File-local parse/read failures belong in the produced artifact diagnostics and are not returned as fatal runner errors.

## Side Effects

Permitted:

- Read selected local source files beneath the authorized snapshot root.
- Publish immutable language artifacts into the supplied local artifact store.

Forbidden:

- Modify repository files.
- Execute repository or toolchain commands.
- Access the network.
- Download dependencies.
- Modify RIE artifacts.
- Persist raw source contents.

## Versioning Policy

- LIE runner and Go engine implementation versions evolve independently.
- `GoLanguageInventory` begins as `0.1.0` during implementation.
- Its public contract freezes at `1.0.0` only after the Phase 2.1 approval gate.
- Additive fields use a minor version after freeze.
- Changed meanings, IDs, ordering, or position semantics require a major version.
- LIE artifacts may reference RIE artifacts but do not change their versions.
