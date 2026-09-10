package die

import (
	"context"
	"encoding/hex"
	"sort"
	"strings"
)

type AnalysisConfigParams struct {
	MaxInputNodes, MaxInputEdges, MaxInputEvidence, MaxInputAuxRecords uint64
	MaxComponents, MaxCycles, MaxTraversalNodes, MaxTraversalEdges     uint64
	MaxTraversalDepth, MaxPageSize                                     uint32
}
type AnalysisConfig struct {
	p     AnalysisConfigParams
	valid bool
}

func NewAnalysisConfig(p AnalysisConfigParams) (AnalysisConfig, error) {
	for _, v := range []struct {
		p    *uint64
		d, m uint64
	}{
		{&p.MaxInputNodes, 1000000, 10000000}, {&p.MaxInputEdges, 2000000, 20000000}, {&p.MaxInputEvidence, 4000000, 20000000}, {&p.MaxInputAuxRecords, 2000000, 20000000},
		{&p.MaxComponents, 1000000, 10000000}, {&p.MaxCycles, 1000000, 10000000}, {&p.MaxTraversalNodes, 100000, 1000000}, {&p.MaxTraversalEdges, 1000000, 20000000},
	} {
		if *v.p == 0 {
			*v.p = v.d
		}
		if *v.p > v.m {
			return AnalysisConfig{}, analysisError(ErrorInvalidInput, "invalid_analysis_config")
		}
	}
	if p.MaxTraversalDepth == 0 {
		p.MaxTraversalDepth = 64
	}
	if p.MaxPageSize == 0 {
		p.MaxPageSize = 100
	}
	if p.MaxTraversalDepth > 4096 || p.MaxPageSize > 1000 || p.MaxCycles > p.MaxComponents || p.MaxComponents > p.MaxInputNodes || p.MaxTraversalNodes > p.MaxInputNodes || p.MaxTraversalEdges > p.MaxInputEdges {
		return AnalysisConfig{}, analysisError(ErrorInvalidInput, "invalid_analysis_config")
	}
	return AnalysisConfig{p, true}, nil
}
func (c AnalysisConfig) MaxInputNodes() uint64      { return c.p.MaxInputNodes }
func (c AnalysisConfig) MaxInputEdges() uint64      { return c.p.MaxInputEdges }
func (c AnalysisConfig) MaxInputEvidence() uint64   { return c.p.MaxInputEvidence }
func (c AnalysisConfig) MaxInputAuxRecords() uint64 { return c.p.MaxInputAuxRecords }
func (c AnalysisConfig) MaxComponents() uint64      { return c.p.MaxComponents }
func (c AnalysisConfig) MaxCycles() uint64          { return c.p.MaxCycles }
func (c AnalysisConfig) MaxTraversalNodes() uint64  { return c.p.MaxTraversalNodes }
func (c AnalysisConfig) MaxTraversalEdges() uint64  { return c.p.MaxTraversalEdges }
func (c AnalysisConfig) MaxTraversalDepth() uint32  { return c.p.MaxTraversalDepth }
func (c AnalysisConfig) MaxPageSize() uint32        { return c.p.MaxPageSize }

type TraversalDirection string

const (
	Dependencies TraversalDirection = "dependencies"
	Dependents   TraversalDirection = "dependents"
)

type NodeQueryParams struct {
	NodeID   string
	Graph    GraphKind
	PageSize uint32
	Cursor   string
}
type NodeQuery struct {
	p     NodeQueryParams
	valid bool
}

func NewNodeQuery(p NodeQueryParams) (NodeQuery, error) {
	if !analysisNodeID(p.NodeID) || !validGraph(p.Graph) || p.PageSize > 1000 || len(p.Cursor) > 2048 {
		return NodeQuery{}, analysisError(ErrorInvalidInput, "invalid_query")
	}
	return NodeQuery{p, true}, nil
}

type ImpactQueryParams struct {
	NodeIDs            []string
	Graph              GraphKind
	Direction          TraversalDirection
	MaxDepth           uint32
	MaxNodes, MaxEdges uint64
}
type ImpactQuery struct {
	p     ImpactQueryParams
	valid bool
}

func NewImpactQuery(p ImpactQueryParams) (ImpactQuery, error) {
	if len(p.NodeIDs) == 0 || len(p.NodeIDs) > 1000000 || !validGraph(p.Graph) || (p.Direction != Dependencies && p.Direction != Dependents) || p.MaxDepth > 4096 || p.MaxNodes > 1000000 || p.MaxEdges > 20000000 {
		return ImpactQuery{}, analysisError(ErrorInvalidInput, "invalid_query")
	}
	for _, id := range p.NodeIDs {
		if !analysisNodeID(id) {
			return ImpactQuery{}, analysisError(ErrorInvalidInput, "invalid_query")
		}
	}
	p.NodeIDs = clone(p.NodeIDs)
	sort.Strings(p.NodeIDs)
	p.NodeIDs = dedupe(p.NodeIDs, func(s string) string { return s })
	return ImpactQuery{p, true}, nil
}
func analysisNodeID(s string) bool {
	const prefix = "dependency-node-id/v1:sha256:"
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	v := strings.TrimPrefix(s, prefix)
	if len(v) != 64 || strings.ToLower(v) != v {
		return false
	}
	_, e := hex.DecodeString(v)
	return e == nil
}
func analysisError(k ErrorKind, code string) error {
	return newError(k, code, "dependency analysis: "+code, nil)
}
func analysisContext(ctx context.Context) error {
	if ctx == nil {
		return analysisError(ErrorInvalidInput, "nil_context")
	}
	return contextError(ctx.Err())
}
