package die

import "encoding/json"

type GraphKind string
type NodeKind string
type DependencyKind string
type ContainmentKind string
type ResolutionState string

const (
	GraphModule              GraphKind       = "module"
	GraphPackage             GraphKind       = "package"
	GraphFile                GraphKind       = "file"
	NodeModule               NodeKind        = "module"
	NodePackage              NodeKind        = "package"
	NodeFile                 NodeKind        = "file"
	NodeStandardLibrary      NodeKind        = "standard_library"
	NodeExternalPackage      NodeKind        = "external_package"
	NodeUnresolved           NodeKind        = "unresolved"
	DependencyImports        DependencyKind  = "imports"
	DependencyReferences     DependencyKind  = "references"
	ContainmentModulePackage ContainmentKind = "module_contains_package"
	ContainmentPackageFile   ContainmentKind = "package_contains_file"
	ResolvedLocal            ResolutionState = "resolved_local"
	StandardLibrary          ResolutionState = "standard_library"
	External                 ResolutionState = "external"
	Unresolved               ResolutionState = "unresolved"
	Ambiguous                ResolutionState = "ambiguous"
	Stale                    ResolutionState = "stale"
)

type ArtifactReference struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"digest,omitempty"`
}
type ArtifactMetadata struct {
	Name                       string `json:"name"`
	Version                    string `json:"version"`
	EngineName                 string `json:"engine_name"`
	EngineVersion              string `json:"engine_version"`
	NodeIDSchemeVersion        string `json:"node_id_scheme_version"`
	EdgeIDSchemeVersion        string `json:"edge_id_scheme_version"`
	ContainmentIDSchemeVersion string `json:"containment_id_scheme_version"`
}
type SourceIdentity struct {
	ArtifactName    string `json:"artifact_name"`
	ArtifactVersion string `json:"artifact_version"`
	SourceID        string `json:"source_id"`
}
type DependencyEvidence struct {
	Source      SourceIdentity `json:"source"`
	File        string         `json:"file,omitempty"`
	StartLine   int            `json:"start_line,omitempty"`
	StartColumn int            `json:"start_column,omitempty"`
	Rule        string         `json:"rule"`
	Value       string         `json:"value,omitempty"`
}
type NodeIdentity struct {
	Kind           NodeKind        `json:"kind"`
	Language       string          `json:"language,omitempty"`
	QualifiedName  string          `json:"qualified_name"`
	RepositoryPath string          `json:"repository_path,omitempty"`
	Resolution     ResolutionState `json:"resolution"`
}
type NodeCandidate struct {
	Identity       NodeIdentity
	Name           string
	SourceIdentity SourceIdentity
	Evidence       []DependencyEvidence
}
type ContainmentCandidate struct {
	Kind     ContainmentKind
	Parent   NodeIdentity
	Child    NodeIdentity
	Evidence []DependencyEvidence
}
type DependencyCandidate struct {
	Graph      GraphKind
	Kind       DependencyKind
	From       NodeIdentity
	To         NodeIdentity
	Resolution ResolutionState
	Evidence   []DependencyEvidence
}
type DiagnosticCandidate struct {
	Code        string
	Graph       GraphKind
	File        string
	StartLine   int
	StartColumn int
	StableID    string
	Message     string
}
type GraphInput struct {
	SourceArtifacts []ArtifactReference
	Nodes           []NodeCandidate
	Containment     []ContainmentCandidate
	Dependencies    []DependencyCandidate
	Diagnostics     []DiagnosticCandidate
}

type DependencyNode struct {
	ID             string               `json:"id"`
	Kind           NodeKind             `json:"kind"`
	Language       string               `json:"language,omitempty"`
	Name           string               `json:"name"`
	QualifiedName  string               `json:"qualified_name"`
	RepositoryPath string               `json:"repository_path,omitempty"`
	Resolution     ResolutionState      `json:"resolution"`
	SourceIdentity SourceIdentity       `json:"source_identity"`
	Evidence       []DependencyEvidence `json:"evidence"`
}
type ContainmentEdge struct {
	ID              string               `json:"id"`
	Kind            ContainmentKind      `json:"kind"`
	ParentID        string               `json:"parent_id"`
	ChildID         string               `json:"child_id"`
	Evidence        []DependencyEvidence `json:"evidence"`
	OmittedEvidence uint64               `json:"omitted_evidence"`
}
type DependencyEdge struct {
	ID              string               `json:"id"`
	Graph           GraphKind            `json:"graph"`
	Kind            DependencyKind       `json:"kind"`
	FromNodeID      string               `json:"from_node_id"`
	ToNodeID        string               `json:"to_node_id"`
	Resolution      ResolutionState      `json:"resolution"`
	Occurrences     uint64               `json:"occurrences"`
	Evidence        []DependencyEvidence `json:"evidence"`
	OmittedEvidence uint64               `json:"omitted_evidence"`
}
type StrongComponent struct {
	ID       string    `json:"id"`
	Graph    GraphKind `json:"graph"`
	NodeIDs  []string  `json:"node_ids"`
	Cyclic   bool      `json:"cyclic"`
	SelfLoop bool      `json:"self_loop"`
}
type DependencyCycle struct {
	ID             string    `json:"id"`
	Graph          GraphKind `json:"graph"`
	ComponentID    string    `json:"component_id"`
	NodeIDs        []string  `json:"node_ids"`
	Classification string    `json:"classification"`
	Rule           string    `json:"rule"`
}
type Diagnostic struct {
	Code        string    `json:"code"`
	Graph       GraphKind `json:"graph,omitempty"`
	File        string    `json:"file,omitempty"`
	StartLine   int       `json:"start_line,omitempty"`
	StartColumn int       `json:"start_column,omitempty"`
	StableID    string    `json:"stable_id,omitempty"`
	Message     string    `json:"message"`
}
type DependencyStatistics struct {
	NodesByKind        map[string]uint64 `json:"nodes_by_kind"`
	EdgesByGraph       map[string]uint64 `json:"edges_by_graph"`
	EdgesByResolution  map[string]uint64 `json:"edges_by_resolution"`
	Diagnostics        uint64            `json:"diagnostics"`
	OmittedDiagnostics uint64            `json:"omitted_diagnostics"`
	OmittedNodes       uint64            `json:"omitted_nodes"`
	OmittedContainment uint64            `json:"omitted_containment"`
	OmittedEdges       uint64            `json:"omitted_edges"`
	OmittedEvidence    uint64            `json:"omitted_evidence"`
}
type DependencyInventoryView struct {
	Artifact         ArtifactMetadata     `json:"artifact"`
	SourceArtifacts  []ArtifactReference  `json:"source_artifacts"`
	Nodes            []DependencyNode     `json:"nodes"`
	Containment      []ContainmentEdge    `json:"containment"`
	Dependencies     []DependencyEdge     `json:"dependencies"`
	StrongComponents []StrongComponent    `json:"strong_components"`
	Cycles           []DependencyCycle    `json:"cycles"`
	Diagnostics      []Diagnostic         `json:"diagnostics"`
	Statistics       DependencyStatistics `json:"statistics"`
}

type DependencyInventory struct{ view DependencyInventoryView }

func (i DependencyInventory) ArtifactName() string       { return i.view.Artifact.Name }
func (i DependencyInventory) ArtifactVersion() string    { return i.view.Artifact.Version }
func (i DependencyInventory) Metadata() ArtifactMetadata { return i.view.Artifact }
func (i DependencyInventory) SourceArtifacts() []ArtifactReference {
	return clone(i.view.SourceArtifacts)
}
func (i DependencyInventory) Nodes() []DependencyNode { return cloneNodes(i.view.Nodes) }
func (i DependencyInventory) Containment() []ContainmentEdge {
	return cloneContainment(i.view.Containment)
}
func (i DependencyInventory) Dependencies() []DependencyEdge {
	return cloneDependencies(i.view.Dependencies)
}
func (i DependencyInventory) StrongComponents() []StrongComponent {
	return cloneComponents(i.view.StrongComponents)
}
func (i DependencyInventory) Cycles() []DependencyCycle { return cloneCycles(i.view.Cycles) }
func (i DependencyInventory) Diagnostics() []Diagnostic { return clone(i.view.Diagnostics) }
func (i DependencyInventory) Statistics() DependencyStatistics {
	return cloneStatistics(i.view.Statistics)
}
func (i DependencyInventory) View() DependencyInventoryView { return cloneView(i.view) }
func (i DependencyInventory) MarshalJSON() ([]byte, error)  { return json.Marshal(i.View()) }

func clone[T any](in []T) []T {
	if len(in) == 0 {
		return []T{}
	}
	return append([]T(nil), in...)
}
func cloneEvidence(in []DependencyEvidence) []DependencyEvidence { return clone(in) }
func cloneNodes(in []DependencyNode) []DependencyNode {
	out := clone(in)
	for x := range out {
		out[x].Evidence = cloneEvidence(out[x].Evidence)
	}
	return out
}
func cloneContainment(in []ContainmentEdge) []ContainmentEdge {
	out := clone(in)
	for x := range out {
		out[x].Evidence = cloneEvidence(out[x].Evidence)
	}
	return out
}
func cloneDependencies(in []DependencyEdge) []DependencyEdge {
	out := clone(in)
	for x := range out {
		out[x].Evidence = cloneEvidence(out[x].Evidence)
	}
	return out
}
func cloneComponents(in []StrongComponent) []StrongComponent {
	out := clone(in)
	for x := range out {
		out[x].NodeIDs = clone(out[x].NodeIDs)
	}
	return out
}
func cloneCycles(in []DependencyCycle) []DependencyCycle {
	out := clone(in)
	for x := range out {
		out[x].NodeIDs = clone(out[x].NodeIDs)
	}
	return out
}
func cloneMap(in map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func cloneStatistics(in DependencyStatistics) DependencyStatistics {
	in.NodesByKind = cloneMap(in.NodesByKind)
	in.EdgesByGraph = cloneMap(in.EdgesByGraph)
	in.EdgesByResolution = cloneMap(in.EdgesByResolution)
	return in
}
func cloneView(in DependencyInventoryView) DependencyInventoryView {
	in.SourceArtifacts = clone(in.SourceArtifacts)
	in.Nodes = cloneNodes(in.Nodes)
	in.Containment = cloneContainment(in.Containment)
	in.Dependencies = cloneDependencies(in.Dependencies)
	in.StrongComponents = cloneComponents(in.StrongComponents)
	in.Cycles = cloneCycles(in.Cycles)
	in.Diagnostics = clone(in.Diagnostics)
	in.Statistics = cloneStatistics(in.Statistics)
	return in
}
