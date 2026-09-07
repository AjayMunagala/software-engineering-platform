package spike

type GraphKind string
type NodeKind string
type EdgeKind string
type Resolution string
type Direction string

const (
	GraphModule  GraphKind = "module"
	GraphPackage GraphKind = "package"
	GraphFile    GraphKind = "file"

	NodeModule   NodeKind = "module"
	NodePackage  NodeKind = "package"
	NodeFile     NodeKind = "file"
	NodeBoundary NodeKind = "boundary"

	EdgeImports    EdgeKind = "imports"
	EdgeReferences EdgeKind = "references"

	ResolvedLocal Resolution = "resolved_local"
	External      Resolution = "external"
	Standard      Resolution = "standard_library"
	Unresolved    Resolution = "unresolved"
	Ambiguous     Resolution = "ambiguous"
	Stale         Resolution = "stale"

	Dependencies Direction = "dependencies"
	Dependents   Direction = "dependents"
)

type NodeIdentity struct {
	Kind          NodeKind
	Language      string
	QualifiedName string
	Path          string
	Resolution    Resolution
}

type Evidence struct {
	Artifact string
	Version  string
	SourceID string
	File     string
	Line     int
	Column   int
	Rule     string
	Value    string
}

type RawEdge struct {
	Graph      GraphKind
	Kind       EdgeKind
	From       NodeIdentity
	To         NodeIdentity
	Resolution Resolution
	Evidence   Evidence
}

type Input struct {
	Nodes []NodeIdentity
	Edges []RawEdge
}

type Node struct {
	ID string
	NodeIdentity
}

type Edge struct {
	ID              string
	Graph           GraphKind
	Kind            EdgeKind
	FromID          string
	ToID            string
	Resolution      Resolution
	Occurrences     int
	Evidence        []Evidence
	OmittedEvidence int
}

type Component struct {
	ID       string
	Graph    GraphKind
	NodeIDs  []string
	Cyclic   bool
	SelfLoop bool
}

type Cycle struct {
	ID             string
	Graph          GraphKind
	ComponentID    string
	NodeIDs        []string
	Classification string
	Rule           string
}

type Statistics struct {
	Nodes            int
	Edges            int
	Occurrences      int
	Components       int
	CyclicComponents int
	OmittedEdges     int
	OmittedEvidence  int
}

type Result struct {
	Nodes      []Node
	Edges      []Edge
	Components []Component
	Cycles     []Cycle
	Statistics Statistics
}

type ImpactQuery struct {
	NodeID    string
	Graph     GraphKind
	Direction Direction
	MaxDepth  int
	MaxNodes  int
}

type ImpactNode struct {
	NodeID string
	Depth  int
}

type ImpactResult struct {
	Nodes          []ImpactNode
	TraversedEdges int
	Truncated      bool
}
