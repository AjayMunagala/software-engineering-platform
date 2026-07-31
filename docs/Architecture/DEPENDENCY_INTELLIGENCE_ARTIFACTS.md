# Dependency Intelligence Artifact Specification

## Status

- Phase: 5.0 design
- Artifact: `dependency-inventory`
- Candidate version: `0.1.0`
- Stable-ID schemes: design candidates, not frozen

## Design goals

The artifact is language-neutral, immutable, deterministic, evidence-backed,
and suitable for future Dependency, Architecture, Reasoning, and Change Impact
consumers without exposing parser or adapter internals.

## Conceptual model

```go
type DependencyInventory struct {
    // private immutable fields
}

type DependencyInventoryView struct {
    Artifact          ArtifactMetadata      `json:"artifact"`
    SourceArtifacts   []ArtifactReference   `json:"source_artifacts"`
    Nodes             []DependencyNode      `json:"nodes"`
    Containment       []ContainmentEdge     `json:"containment"`
    Dependencies      []DependencyEdge      `json:"dependencies"`
    Components        []StrongComponent     `json:"strong_components"`
    Cycles            []DependencyCycle     `json:"cycles"`
    Diagnostics       []Diagnostic          `json:"diagnostics"`
    Statistics        DependencyStatistics  `json:"statistics"`
}
```

The production value hides slices and returns defensive copies. The view is a
detached presentation/serialization value and is never an engine input.

## Artifact metadata

```go
type ArtifactMetadata struct {
    Name                 string `json:"name"`                  // dependency-inventory
    Version              string `json:"version"`               // 0.1.0 candidate
    EngineName           string `json:"engine_name"`           // dependency-intelligence
    EngineVersion        string `json:"engine_version"`
    NodeIDSchemeVersion  string `json:"node_id_scheme_version"`
    EdgeIDSchemeVersion  string `json:"edge_id_scheme_version"`
    SCCIDSchemeVersion   string `json:"scc_id_scheme_version"`
}
```

Artifact, engine, codec, database schema, and ID-scheme versions evolve
independently.

## Nodes

```go
type NodeKind string

const (
    NodeModule          NodeKind = "module"
    NodePackage         NodeKind = "package"
    NodeFile            NodeKind = "file"
    NodeStandardLibrary NodeKind = "standard_library"
    NodeExternalPackage NodeKind = "external_package"
    NodeUnresolved      NodeKind = "unresolved"
)

type ResolutionState string

const (
    ResolvedLocal   ResolutionState = "resolved_local"
    StandardLibrary ResolutionState = "standard_library"
    External        ResolutionState = "external"
    Unresolved      ResolutionState = "unresolved"
    Ambiguous       ResolutionState = "ambiguous"
    Stale           ResolutionState = "stale"
)

type DependencyNode struct {
    ID               string             `json:"id"`
    Kind             NodeKind           `json:"kind"`
    Language         string             `json:"language,omitempty"`
    Name             string             `json:"name"`
    QualifiedName    string             `json:"qualified_name"`
    RepositoryPath   string             `json:"repository_path,omitempty"`
    Resolution       ResolutionState    `json:"resolution"`
    SourceIdentity   SourceIdentity     `json:"source_identity"`
    Evidence         []DependencyEvidence `json:"evidence"`
}
```

`RepositoryPath` is normalized and repository-relative. Absolute paths,
source handles, credentials, and host-specific values are forbidden.

## Containment

```go
type ContainmentKind string // module_contains_package | package_contains_file

type ContainmentEdge struct {
    ID       string             `json:"id"`
    Kind     ContainmentKind    `json:"kind"`
    ParentID string             `json:"parent_id"`
    ChildID  string             `json:"child_id"`
    Evidence []DependencyEvidence `json:"evidence"`
}
```

Containment is not counted as dependency coupling.

## Dependency edges

```go
type GraphKind string // module | package | file
type DependencyKind string // imports | references

type DependencyEdge struct {
    ID              string             `json:"id"`
    Graph           GraphKind          `json:"graph"`
    Kind            DependencyKind     `json:"kind"`
    FromNodeID      string             `json:"from_node_id"`
    ToNodeID        string             `json:"to_node_id"`
    Resolution      ResolutionState    `json:"resolution"`
    Occurrences     uint64             `json:"occurrences"`
    Evidence        []DependencyEvidence `json:"evidence"`
    OmittedEvidence uint64             `json:"omitted_evidence"`
}
```

Parallel facts with identical graph, kind, source, target, and resolution are
aggregated deterministically. Evidence is sorted before its configured cap is
applied. The exact occurrence count is retained when evidence is omitted.

## Evidence and provenance

```go
type SourceIdentity struct {
    ArtifactName    string `json:"artifact_name"`
    ArtifactVersion string `json:"artifact_version"`
    SourceID        string `json:"source_id"`
}

type DependencyEvidence struct {
    Source           SourceIdentity `json:"source"`
    File             string         `json:"file,omitempty"`
    StartLine        int            `json:"start_line,omitempty"`
    StartColumn      int            `json:"start_column,omitempty"`
    Rule             string         `json:"rule"`
    Value            string         `json:"value,omitempty"`
}
```

Evidence points to released artifact identities. It never embeds ASTs, source
text, absolute paths, raw repository handles, or database identities.

## Strongly connected components and cycles

```go
type StrongComponent struct {
    ID        string    `json:"id"`
    Graph     GraphKind `json:"graph"`
    NodeIDs   []string  `json:"node_ids"`
    Cyclic    bool      `json:"cyclic"`
    SelfLoop  bool      `json:"self_loop"`
}

type CycleClassification string

const (
    CycleStructural      CycleClassification = "structural"
    CycleLanguageInvalid CycleClassification = "language_invalid"
    CycleInformational   CycleClassification = "informational"
)

type DependencyCycle struct {
    ID             string              `json:"id"`
    Graph          GraphKind           `json:"graph"`
    ComponentID    string              `json:"component_id"`
    NodeIDs        []string            `json:"node_ids"`
    Classification CycleClassification `json:"classification"`
}
```

The artifact records SCC membership, not every possible cyclic path, because
the number of simple cycles can grow exponentially. A canonical witness cycle
may be added only if the design spike proves bounded deterministic behavior.

## Statistics

```go
type DependencyStatistics struct {
    NodesByKind              map[string]uint64 `json:"nodes_by_kind"`
    EdgesByGraph             map[string]uint64 `json:"edges_by_graph"`
    EdgesByResolution        map[string]uint64 `json:"edges_by_resolution"`
    StrongComponentsByGraph  map[string]uint64 `json:"strong_components_by_graph"`
    CyclicComponentsByGraph  map[string]uint64 `json:"cyclic_components_by_graph"`
    Diagnostics              uint64            `json:"diagnostics"`
    OmittedDiagnostics       uint64            `json:"omitted_diagnostics"`
    OmittedNodes             uint64            `json:"omitted_nodes"`
    OmittedEdges             uint64            `json:"omitted_edges"`
    OmittedEvidence          uint64            `json:"omitted_evidence"`
}
```

Counts are derived from the complete normalized candidate before presentation.
Maps serialize using canonical key ordering through the approved codec.

## Candidate stable identities

Candidate schemes:

- `dependency-node-id/v1`;
- `dependency-edge-id/v1`;
- `dependency-containment-id/v1`;
- `dependency-scc-id/v1`;
- `dependency-cycle-id/v1`.

Each ID is derived from a domain-separated canonical byte sequence containing
only stable logical identity, never display text, slice position, host path,
worker count, timestamps, or database keys. Phase 5.0.1 must freeze golden
vectors before production implementation.

Changing a frozen identity algorithm requires a new scheme version, parallel
publication during migration, and explicit consumer migration guidance.

## Ordering

Canonical order is:

1. nodes by kind, qualified name, repository path, then ID;
2. containment by kind, parent ID, child ID, then ID;
3. dependencies by graph, kind, source ID, target ID, resolution, then ID;
4. SCCs by graph, first node ID, cardinality, then ID;
5. cycles by graph, classification, component ID, then ID;
6. evidence by source artifact, source ID, file, location, rule, value;
7. diagnostics by code, graph, file, location, stable ID.

Empty collections serialize as `[]`, not `null`.

## Immutability

- constructors validate and defensively copy every collection;
- accessors return detached copies;
- no mutable AST, graph map, or cache is published;
- query indexes are private and rebuildable from the canonical artifact;
- source artifacts are never modified;
- serialization never changes the artifact.

## Compatibility

The candidate may change until `1.0.0`. At `1.0.0`, additive optional fields
may use a minor version; changed semantics, removed fields, changed ordering,
or changed required inputs require a major version. Stable-ID versions remain
independent and explicit.
