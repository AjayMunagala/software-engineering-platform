# Dependency Intelligence Engine Candidate API

## Status

- Phase: 5.0 design
- Contract version: `0.1.0`
- Artifact version: `0.1.0`
- Phase 5.0.2 neutral core implementation: engineering accepted 2026-09-08
- Phase 5.0.3 and production release: not authorized
- Transport: none
- Phase 5.0.1 spike: engineering accepted 2026-09-08

This is a Go contract candidate. It is not an HTTP, gRPC, persistence, runtime,
or authorization API.

## Contract identity

```go
const (
    ContractVersion = "0.1.0"
    ArtifactName     = "dependency-inventory"
    ArtifactVersion  = "0.1.0"
)
```

## Engine metadata

```go
type Engine interface {
    Name() string
    Version() string
    Description() string
    Analyze(context.Context, Inputs) (DependencyInventory, error)
}
```

`Analyze` performs a fresh deterministic rebuild. No incremental cache is part
of the candidate contract.

## Inputs

```go
type Inputs struct {
    // private immutable fields
}

type InputParams struct {
    RepositorySnapshot       rie.RepositorySnapshot
    GoLanguageInventory      golang.GoLanguageInventory
    GoPackageIdentity        packageidentity.GoPackageIdentityInventory
    GoSemanticInventory      semantic.GoSemanticInventory
}

func NewInputs(InputParams) (Inputs, error)
```

The constructor validates exact artifact names and versions, source-artifact
consistency, and repository identity consistency. Values are detached. The
engine does not accept raw paths, source readers, ASTs, mutable contexts, or
presentation summaries.

Future languages are added through new language-adapter input constructors or
narrow provider interfaces, not optional `any` fields added to this frozen
shape.

## Configuration

```go
type ConfigParams struct {
    MaxWorkers         int
    MaxNodes           uint64
    MaxEdges           uint64
    MaxEvidencePerEdge uint64
    MaxDiagnostics     uint64
    MaxTraversalDepth  uint32
    MaxTraversalNodes  uint64
}

type Config struct { /* private immutable fields */ }

func NewConfig(ConfigParams) (Config, error)
func New(Config) (Engine, error)
```

Zero values select documented defaults. Workers are capped at eight. Safety
limits are bounded by implementation constants and validated before analysis.
`MaxNodes` is a mandatory production input bound: the core must enforce it
before publishing any graph and return `limit_exceeded` rather than silently
inheriting the spike's unbounded-node behavior. Any deterministic omission
policy for other bounded collections must publish exact omission counts.

## Immutable artifact access

```go
type DependencyInventory struct {
    // private immutable fields
}

func (DependencyInventory) ArtifactName() string
func (DependencyInventory) ArtifactVersion() string
func (DependencyInventory) Metadata() ArtifactMetadata
func (DependencyInventory) SourceArtifacts() []rie.ArtifactReference
func (DependencyInventory) Nodes() []DependencyNode
func (DependencyInventory) Containment() []ContainmentEdge
func (DependencyInventory) Dependencies() []DependencyEdge
func (DependencyInventory) StrongComponents() []StrongComponent
func (DependencyInventory) Cycles() []DependencyCycle
func (DependencyInventory) Diagnostics() []Diagnostic
func (DependencyInventory) Statistics() DependencyStatistics
func (DependencyInventory) View() DependencyInventoryView
```

The Phase 5.0.1 spike confirmed that a concrete immutable value matches the
released artifact conventions without weakening consumer isolation.
Observable behavior is the contract. Every accessor returns a defensive copy.

## Query capability

Queries are pure operations over a supplied immutable inventory.

```go
type QueryEngine interface {
    DirectDependencies(context.Context, DependencyInventory, NodeQuery) (NodePage, error)
    DirectDependents(context.Context, DependencyInventory, NodeQuery) (NodePage, error)
    Impact(context.Context, DependencyInventory, ImpactQuery) (ImpactResult, error)
}

type NodeQueryParams struct {
    NodeID    string
    Graph     GraphKind
    PageSize  uint32
    Cursor    string
}

type ImpactQueryParams struct {
    NodeIDs   []string
    Graph     GraphKind
    Direction TraversalDirection // dependencies | dependents
    MaxDepth  uint32
    MaxNodes  uint64
}
```

Results include visited nodes, traversed edges, depth, truncation state, and a
deterministic continuation cursor where supported. Traversal order is
canonical breadth-first order. A result never claims full impact when a limit
or unresolved boundary was encountered.

The inventory does not persist all-pairs reachability.

## Errors

```go
type ErrorKind string

const (
    ErrorInvalidInput         ErrorKind = "invalid_input"
    ErrorMissingArtifact      ErrorKind = "missing_artifact"
    ErrorIncompatibleArtifact ErrorKind = "incompatible_artifact"
    ErrorIntegrity            ErrorKind = "integrity_failure"
    ErrorLimitExceeded        ErrorKind = "limit_exceeded"
    ErrorCanceled             ErrorKind = "canceled"
    ErrorInternal             ErrorKind = "internal"
)

type Error struct {
    // stable kind/code and redacted message; implementation cause is private
}
```

Errors never contain source text, absolute paths, credentials, SQL, raw driver
errors, or mutable artifact values. `errors.Is` must preserve context
cancellation and deadline semantics.

## Determinism

For identical prerequisite artifact bytes and configuration, Windows and
Ubuntu, one and eight workers, repeated processes, and randomized internal map
iteration must produce identical:

- artifact bytes;
- stable IDs;
- node/edge/SCC ordering;
- statistics;
- diagnostics;
- omission counts;
- query results and cursors.

Timing, memory, and runtime host data are metrics outside the authoritative
artifact.

## Side effects

The engine has no filesystem, network, process execution, database,
persistence, logging, or repository mutation capability. Orchestration may
serialize and persist the returned immutable artifact after analysis.

## Candidate evolution

Phase 5.0.1 may refine signatures using measured spike evidence. Production
implementation begins only after the architecture package and spike evidence
are accepted together. The public API remains `0.1.0` until stabilization.

Stable-ID canonical bytes and the accepted cross-platform node, edge, SCC, and
cycle vectors are specified in
`DEPENDENCY_INTELLIGENCE_GOLDEN_VECTORS.md`. Phase 5.0.2 froze containment's
canonical identity and golden vector before publishing containment records.

Before the candidate may be promoted to `1.0.0`, implementation hardening must
bound intermediate evidence accumulation as well as published evidence, and a
nil `context.Context` must return a stable `invalid_input` error without panic.
