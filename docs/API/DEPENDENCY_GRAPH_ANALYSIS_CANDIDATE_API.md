# Phase 5.0.4 — Graph Analysis Candidate API

Design review candidate 0.1.0, 2026-09-09. No code or implementation tests authorized.
Proposed additions live in `backend/die`; accepted `Core` remains unchanged.

## Capabilities (proposed signatures)

```go
type Analyzer interface {
    Analyze(context.Context, DependencyInventory) (DependencyInventory, error)
}
type QueryEngine interface {
    DirectDependencies(context.Context, DependencyInventory, NodeQuery) (NodePage, error)
    DirectDependents(context.Context, DependencyInventory, NodeQuery) (NodePage, error)
    Impact(context.Context, DependencyInventory, ImpactQuery) (ImpactResult, error)
}
func NewAnalyzer(AnalysisConfig) (Analyzer, error)
func NewQueryEngine(AnalysisConfig) (QueryEngine, error)
func NewAnalysisConfig(AnalysisConfigParams) (AnalysisConfig, error)
func NewNodeQuery(NodeQueryParams) (NodeQuery, error)
func NewImpactQuery(ImpactQueryParams) (ImpactQuery, error)
```

Factories accept only constructed immutable configs; zero models/configs fail.
No public cache, transaction, AST, engine, or persistence capability. Each call
rebuilds its bounded private index. No `MaxWorkers`: initial algorithms are serial.
The analyzer computes all three projections, never silently selects one.

NodeQueryParams: NodeID string, Graph GraphKind, PageSize uint32, Cursor string.
Direction is the called method, and must match any cursor. ImpactQueryParams:
NodeIDs []string, Graph GraphKind, Direction (`dependencies` or `dependents`),
MaxDepth uint32, MaxNodes uint64, MaxEdges uint64. Copy/dedupe/sort seeds at
construction; reject empty/unknown IDs and unknown enums. Node IDs must use the
accepted v1 textual form. Membership is checked against the supplied inventory.

AnalysisConfigParams uses uint64 MaxInputNodes, MaxInputEdges, MaxInputEvidence,
MaxInputAuxRecords, MaxComponents, MaxCycles, MaxTraversalNodes, MaxTraversalEdges;
uint32 MaxTraversalDepth and MaxPageSize. Private AnalysisConfig accessors mirror
these fields. Constructors reject unsupported values, overflow, and invalid
relations. Zero numeric parameters select defaults, including query overrides;
depth zero does not mean 'unlimited'. Nonzero query overrides must not exceed config.
Query constructors retain zero limit fields as default sentinels; execution
resolves them against that engine's validated config, not hard-coded constructor
defaults. Zero PageSize similarly selects the configured default page size.

## Proposed count limits (not measured memory guarantees)

| Setting | Default | Maximum |
|---|---:|---:|
| MaxInputNodes | 1,000,000 | 10,000,000 |
| MaxInputEdges | 2,000,000 | 20,000,000 |
| MaxInputEvidence | 4,000,000 | 20,000,000 |
| MaxInputAuxRecords | 2,000,000 | 20,000,000 |
| MaxComponents | 1,000,000 | 10,000,000 |
| MaxCycles | 1,000,000 | 10,000,000 |
| MaxTraversalNodes | 100,000 | 1,000,000 |
| MaxTraversalEdges | 1,000,000 | 20,000,000 |
| MaxTraversalDepth | 64 | 4,096 |
| MaxPageSize | 100 | 1,000 |

Aux records include containment, source refs, diagnostics, existing derived
records and their nested membership/reason IDs; evidence includes all supplied
node/edge/containment evidence. Components/cycles count across three projections.
Require MaxCycles <= MaxComponents, MaxComponents <= MaxInputNodes, traversal
nodes <= input nodes and traversal edges <= input edges. Defaults are resolved
before relational validation; callers reducing a parent cap must explicitly
reduce dependent caps as needed. Document this rather than silently clamping.

Accessor cloning precedes returned-length inspection, an inherited limitation.
All expanded index/output counts must be checked before allocation/appending;
no preallocation from unchecked lengths. Byte sizes of existing strings are not
bounded by these count limits. No new hard-RAM promise is made.

SCC/input caps are hard errors, with zero output. Traversal caps are deterministic
partial results. Pagination bounds returned neighbors, not all input processing;
each call can cost O(V+E) indexing plus canonical sort. Do not imply O(page size).

## Results and errors

NodePage, ImpactResult, AnalysisMetadata, and GraphAnalysis have private backing
state, detached read accessors, and deterministic JSON views as specified in
`docs/Architecture/DEPENDENCY_GRAPH_ANALYSIS_ARTIFACTS.md`. Empty arrays are `[]`.
No mutable input setters or unvalidated public result constructors are needed.

Reuse the implemented `die.ErrorKind`: invalid_input, integrity_failure,
limit_exceeded, canceled, internal. Do not add obsolete illustrative error kinds.
Proposed fixed codes include invalid_analysis_config, invalid_query,
incompatible_inventory, inconsistent_graph, analysis_limit_exceeded,
cursor_mismatch, analysis_canceled. Unknown IDs and wrong graph roots are invalid
queries. Cursor decoding never returns raw decoder text. Only context causes
unwrap. Nil ctx is invalid_input; canceled/deadline preserve errors.Is and return
no result. Explicit partial traversal is not an error.

Analysis metadata distinguishes analyzed-empty from core-only inventory. Reanalysis
uses base graph data, yielding identical results with the same config. Base source
facts/evidence/IDs remain unchanged. Candidate schema additions and new golden
vectors require approval before implementation; no production code accompanies
this proposal.
