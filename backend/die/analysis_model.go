package die

import (
	"context"
	"encoding/json"
)

const AnalysisDigestScheme = "dependency-analysis-input/v1"
const AnalysisCursorScheme = "dependency-node-page/v1"

type Analyzer interface {
	Analyze(context.Context, DependencyInventory) (DependencyInventory, error)
}
type QueryEngine interface {
	DirectDependencies(context.Context, DependencyInventory, NodeQuery) (NodePage, error)
	DirectDependents(context.Context, DependencyInventory, NodeQuery) (NodePage, error)
	Impact(context.Context, DependencyInventory, ImpactQuery) (ImpactResult, error)
}

func NewAnalyzer(c AnalysisConfig) (Analyzer, error) {
	if !c.valid {
		return nil, analysisError(ErrorInvalidInput, "invalid_analysis_config")
	}
	return &graphAnalyzer{c}, nil
}
func NewQueryEngine(c AnalysisConfig) (QueryEngine, error) {
	if !c.valid {
		return nil, analysisError(ErrorInvalidInput, "invalid_analysis_config")
	}
	return &graphAnalyzer{c}, nil
}

type AnalysisMetadata struct {
	EngineName       string          `json:"engine_name"`
	EngineVersion    string          `json:"engine_version"`
	InputDigest      string          `json:"input_digest"`
	DigestScheme     string          `json:"digest_scheme"`
	SCCIDScheme      string          `json:"scc_id_scheme"`
	CycleIDScheme    string          `json:"cycle_id_scheme"`
	ProjectionPolicy string          `json:"projection_policy"`
	Graphs           []GraphAnalysis `json:"graphs"`
}
type GraphAnalysis struct {
	Graph              GraphKind `json:"graph"`
	EligibleNodes      uint64    `json:"eligible_nodes"`
	EligibleEdges      uint64    `json:"eligible_edges"`
	Components         uint64    `json:"components"`
	CyclicComponents   uint64    `json:"cyclic_components"`
	BoundaryEdges      uint64    `json:"boundary_edges"`
	TopologyLimited    bool      `json:"topology_limited"`
	ExplanationLimited bool      `json:"explanation_limited"`
	ReasonCodes        []string  `json:"reason_codes"`
}

func cloneAnalysis(p *AnalysisMetadata) *AnalysisMetadata {
	if p == nil {
		return nil
	}
	out := *p
	out.Graphs = clone(p.Graphs)
	for i := range out.Graphs {
		out.Graphs[i].ReasonCodes = clone(out.Graphs[i].ReasonCodes)
	}
	return &out
}
func (i DependencyInventory) Analysis() *AnalysisMetadata { return cloneAnalysis(i.view.Analysis) }

type Neighbor struct {
	NodeID     string          `json:"node_id"`
	Resolution ResolutionState `json:"resolution"`
	EdgeIDs    []string        `json:"edge_ids"`
}
type NodePageView struct {
	InputDigest        string             `json:"input_digest"`
	Graph              GraphKind          `json:"graph"`
	Direction          TraversalDirection `json:"direction"`
	RootNodeID         string             `json:"root_node_id"`
	Neighbors          []Neighbor         `json:"neighbors"`
	HasMore            bool               `json:"has_more"`
	NextCursor         string             `json:"next_cursor"`
	TopologyLimited    bool               `json:"topology_limited"`
	ExplanationLimited bool               `json:"explanation_limited"`
	ReasonCodes        []string           `json:"reason_codes"`
}
type NodePage struct{ view NodePageView }

func (p NodePage) View() NodePageView {
	v := p.view
	v.Neighbors = clone(v.Neighbors)
	for i := range v.Neighbors {
		v.Neighbors[i].EdgeIDs = clone(v.Neighbors[i].EdgeIDs)
	}
	v.ReasonCodes = clone(v.ReasonCodes)
	return v
}
func (p NodePage) MarshalJSON() ([]byte, error) { return json.Marshal(p.View()) }

type ReachedNode struct {
	NodeID string `json:"node_id"`
	Depth  uint32 `json:"depth"`
}
type ImpactResultView struct {
	InputDigest        string             `json:"input_digest"`
	Graph              GraphKind          `json:"graph"`
	Direction          TraversalDirection `json:"direction"`
	Seeds              []string           `json:"seeds"`
	Reached            []ReachedNode      `json:"reached"`
	BoundaryNodeIDs    []string           `json:"boundary_node_ids"`
	EdgeIDs            []string           `json:"edge_ids"`
	VisitedNodes       uint64             `json:"visited_nodes"`
	ScannedEdges       uint64             `json:"scanned_edges"`
	MaxDepthReached    uint32             `json:"max_depth_reached"`
	Truncated          bool               `json:"truncated"`
	TopologyLimited    bool               `json:"topology_limited"`
	ExplanationLimited bool               `json:"explanation_limited"`
	ReasonCodes        []string           `json:"reason_codes"`
}
type ImpactResult struct{ view ImpactResultView }

func (r ImpactResult) View() ImpactResultView {
	v := r.view
	v.Seeds = clone(v.Seeds)
	v.Reached = clone(v.Reached)
	v.BoundaryNodeIDs = clone(v.BoundaryNodeIDs)
	v.EdgeIDs = clone(v.EdgeIDs)
	v.ReasonCodes = clone(v.ReasonCodes)
	return v
}
func (r ImpactResult) MarshalJSON() ([]byte, error) { return json.Marshal(r.View()) }
