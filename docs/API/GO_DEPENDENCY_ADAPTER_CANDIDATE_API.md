# Go Dependency Adapter Candidate API

Implementation candidate `0.1.0`. Design and implementation evidence at
`a0a1a779734b23a0f6c2a8f711130063822a9761` are engineering accepted.
This is not a production release or authorization for Phase 5.0.4.
This specification specializes the earlier engine API for Phase 5.0.3;
`backend/die` stays the accepted neutral core.

## Proposed package and surface

Package: `backend/die/golang`.

```go
type InputParams struct {
    RepositorySnapshot  rie.RepositorySnapshot
    LanguageInventory   golanguage.GoLanguageInventory
    PackageIdentity     packageidentity.GoPackageIdentityInventory
    SemanticInventory   semantic.GoSemanticInventory
}
type Inputs struct { /* private immutable artifact values */ }
func NewInputs(context.Context, InputParams) (Inputs, error)

type ConfigParams struct {
    CoreConfig         die.Config
    MaxInputRecords    uint64
    MaxInputEvidence   uint64
}
type Config struct { /* private validated values */ }
func NewConfig(ConfigParams) (Config, error)

type Engine interface {
    Name() string
    Version() string
    Description() string
    Analyze(context.Context, Inputs) (die.DependencyInventory, error)
}
func New(Config) (Engine, error)
```

`NewInputs` verifies metadata and prerequisite references and retains immutable
values. Config-dependent collection/index checks occur in Analyze before graph
construction. It exposes no mutable views or filesystem handles. A zero Inputs
value is invalid. Nil contexts fail `invalid_input` in both public operations.
Cancellation/deadlines preserve `errors.Is` through safe wrapping.

The implementation creates its core through `die.New`. No public dependency
injection hook, worker pool, parser, or transport type is needed. Calls rebuild
independently; simultaneous calls share only immutable configuration.

## Configuration semantics

Zero raw budgets select defaults: 2,000,000 records and 4,000,000 evidence
records. Proposed ceilings are 10,000,000 and 20,000,000 respectively. These
are hard input-count caps, not measured peak-memory guarantees. A zero core
configuration selects `die.DefaultConfig`; otherwise validate its accessors
through the accepted configuration constructor. No worker option is exposed:
the initial translator is serial and deterministic.

Records include every extracted file, package, module, context, proof,
declaration, reference, binding, and diagnostic processed by the adapter;
they also include snapshot entries, nested membership/candidate IDs and kinds,
syntax imports, and generated containment/edge/diagnostic contributions.
evidence includes all extracted proof/module/context evidence and generated
graph evidence contributions. Arithmetic is checked for overflow. Budgets do
not authorize source or evidence truncation before canonical normalization.
Input-owned storage and allocations inside released clone accessors are reported
separately from adapter index memory.

## Errors and results

Use an adapter error with private fields, `Kind() die.ErrorKind`, `Code() string`,
and constant redacted `Error()` text. Invalid/uninitialized/incompatible inputs
map to `invalid_input` with distinct codes; contradictory joins to
`integrity_failure`; raw budgets to `limit_exceeded`; cancellation to `canceled`.
Do not add new error kinds to the accepted core for adapter-specific details.
Only context errors may be exposed through Unwrap; never raw source values.

On success return exactly one core-normalized inventory. On any failure return
the zero inventory. The graph can be partial due to upstream limitations and
published core caps, with fixed diagnostics and exact available omission data.
It never claims absent upstream relationships are absent dependencies.

Graph input values are private temporary translation results, not an additional
public API. Source references use `die.ArtifactReference` (the implemented core
type), not a new dependency from the neutral core into RIE.

## Compatibility

Frozen node/edge/containment encodings remain unchanged. Proposed boundary-name
encoding is separately named `go-dependency-boundary-name/v1`: unsigned uint64
big-endian length-prefixed UTF-8 segments of domain, importing package ID,
context ID (empty when unavailable), and import path, hashed as lowercase SHA-256
and prefixed with that domain. Resolution also remains part of the frozen node
ID. Commit Unicode, empty-context, and collision-separation vectors before
emitting this boundary identity during authorized implementation. That gate was
completed in commit `7eb2a98271499f381dd60ca773ab1e6f02a11b01`; the independently
computed vectors are in `GO_DEPENDENCY_BOUNDARY_GOLDEN_VECTORS.md`.

The candidate stays `0.1.0`. SCC/cycle/impact APIs are outside this milestone.
The two core hardening backlog items remain open until their own closure tests
are accepted; adapter safeguards do not close them.
