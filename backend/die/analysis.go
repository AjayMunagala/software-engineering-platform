package die

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
)

type graphAnalyzer struct{ config AnalysisConfig }
type graphIndex struct {
	view    DependencyInventoryView
	digest  string
	nodes   map[string]DependencyNode
	out, in map[GraphKind]map[string][]int
	summary map[GraphKind]GraphAnalysis
}

var analysisGraphs = []GraphKind{GraphFile, GraphModule, GraphPackage}

func localVertex(n DependencyNode, g GraphKind) bool {
	return n.Resolution == ResolvedLocal && string(n.Kind) == string(g)
}
func eligible(e DependencyEdge, x *graphIndex) bool {
	return e.Resolution == ResolvedLocal && localVertex(x.nodes[e.FromNodeID], e.Graph) && localVertex(x.nodes[e.ToNodeID], e.Graph)
}
func countAnalysis(current *uint64, n int, limit uint64) error {
	if n < 0 || *current > limit || uint64(n) > limit-*current {
		return analysisError(ErrorLimitExceeded, "analysis_limit_exceeded")
	}
	*current += uint64(n)
	return nil
}

func (a *graphAnalyzer) index(ctx context.Context, input DependencyInventory) (*graphIndex, error) {
	if err := analysisContext(ctx); err != nil {
		return nil, err
	}
	v := input.view
	m := v.Artifact
	if m.Name != ArtifactName || m.Version != ArtifactVersion || m.EngineName != EngineName || m.EngineVersion != EngineVersion || m.NodeIDSchemeVersion != "dependency-node-id/v1" || m.EdgeIDSchemeVersion != "dependency-edge-id/v1" || m.ContainmentIDSchemeVersion != "dependency-containment-id/v1" {
		return nil, analysisError(ErrorInvalidInput, "incompatible_inventory")
	}
	c := a.config.p
	if uint64(len(v.Nodes)) > c.MaxInputNodes || uint64(len(v.Dependencies)) > c.MaxInputEdges {
		return nil, analysisError(ErrorLimitExceeded, "analysis_limit_exceeded")
	}
	var evidence, aux uint64
	for _, n := range []int{len(v.Containment), len(v.SourceArtifacts), len(v.Diagnostics), len(v.StrongComponents), len(v.Cycles)} {
		if err := countAnalysis(&aux, n, c.MaxInputAuxRecords); err != nil {
			return nil, err
		}
	}
	for _, n := range v.Nodes {
		if err := analysisContext(ctx); err != nil {
			return nil, err
		}
		if err := countAnalysis(&evidence, len(n.Evidence), c.MaxInputEvidence); err != nil {
			return nil, err
		}
	}
	for _, e := range v.Dependencies {
		if err := analysisContext(ctx); err != nil {
			return nil, err
		}
		if err := countAnalysis(&evidence, len(e.Evidence), c.MaxInputEvidence); err != nil {
			return nil, err
		}
	}
	for _, e := range v.Containment {
		if err := analysisContext(ctx); err != nil {
			return nil, err
		}
		if err := countAnalysis(&evidence, len(e.Evidence), c.MaxInputEvidence); err != nil {
			return nil, err
		}
	}
	for _, s := range v.StrongComponents {
		if err := analysisContext(ctx); err != nil {
			return nil, err
		}
		if err := countAnalysis(&aux, len(s.NodeIDs), c.MaxInputAuxRecords); err != nil {
			return nil, err
		}
	}
	for _, s := range v.Cycles {
		if err := analysisContext(ctx); err != nil {
			return nil, err
		}
		if err := countAnalysis(&aux, len(s.NodeIDs), c.MaxInputAuxRecords); err != nil {
			return nil, err
		}
	}
	if v.Analysis != nil {
		if err := countAnalysis(&aux, len(v.Analysis.Graphs), c.MaxInputAuxRecords); err != nil {
			return nil, err
		}
		for _, s := range v.Analysis.Graphs {
			if err := analysisContext(ctx); err != nil {
				return nil, err
			}
			if err := countAnalysis(&aux, len(s.ReasonCodes), c.MaxInputAuxRecords); err != nil {
				return nil, err
			}
		}
	}
	x := &graphIndex{view: v, nodes: map[string]DependencyNode{}, out: map[GraphKind]map[string][]int{}, in: map[GraphKind]map[string][]int{}, summary: map[GraphKind]GraphAnalysis{}}
	for _, g := range analysisGraphs {
		x.out[g] = map[string][]int{}
		x.in[g] = map[string][]int{}
		s := GraphAnalysis{Graph: g, ReasonCodes: []string{}}
		st := v.Statistics
		for _, r := range []struct {
			b bool
			s string
		}{{st.OmittedEdges > 0, "input_edges_omitted"}, {st.OmittedNodes > 0, "input_nodes_omitted"}, {st.OmittedDiagnostics > 0, "input_diagnostics_omitted"}, {len(v.Diagnostics) > 0, "input_partial_diagnostics"}} {
			if r.b {
				s.TopologyLimited = true
				s.ReasonCodes = append(s.ReasonCodes, r.s)
			}
		}
		if st.OmittedEvidence > 0 {
			s.ExplanationLimited = true
			s.ReasonCodes = append(s.ReasonCodes, "input_evidence_omitted")
		}
		if st.OmittedContainment > 0 {
			s.ExplanationLimited = true
			s.ReasonCodes = append(s.ReasonCodes, "input_containment_omitted")
		}
		x.summary[g] = s
	}
	bad := func() (*graphIndex, error) { return nil, analysisError(ErrorIntegrity, "inconsistent_graph") }
	for _, n := range v.Nodes {
		if err := analysisContext(ctx); err != nil {
			return nil, err
		}
		id := NodeIdentity{Kind: n.Kind, Language: n.Language, QualifiedName: n.QualifiedName, RepositoryPath: n.RepositoryPath, Resolution: n.Resolution}
		norm, err := normalizeIdentity(id)
		if err != nil || norm != id || n.ID != nodeID(id) || x.nodes[n.ID].ID != "" {
			return bad()
		}
		x.nodes[n.ID] = n
		for _, g := range analysisGraphs {
			if localVertex(n, g) {
				s := x.summary[g]
				s.EligibleNodes++
				x.summary[g] = s
			}
		}
	}
	seen := map[string]bool{}
	for i, e := range v.Dependencies {
		if err := analysisContext(ctx); err != nil {
			return nil, err
		}
		if !validGraph(e.Graph) || !validDependency(e.Kind) || !validResolution(e.Resolution) || e.Occurrences == 0 || x.nodes[e.FromNodeID].ID == "" || x.nodes[e.ToNodeID].ID == "" || seen[e.ID] || e.ID != edgeID(e.Graph, e.Kind, e.FromNodeID, e.ToNodeID, e.Resolution) {
			return bad()
		}
		seen[e.ID] = true
		x.out[e.Graph][e.FromNodeID] = append(x.out[e.Graph][e.FromNodeID], i)
		x.in[e.Graph][e.ToNodeID] = append(x.in[e.Graph][e.ToNodeID], i)
		s := x.summary[e.Graph]
		if eligible(e, x) {
			s.EligibleEdges++
		} else {
			s.BoundaryEdges++
		}
		x.summary[e.Graph] = s
	}
	seen = map[string]bool{}
	for _, e := range v.Containment {
		if err := analysisContext(ctx); err != nil {
			return nil, err
		}
		if !validContainment(e.Kind) || x.nodes[e.ParentID].ID == "" || x.nodes[e.ChildID].ID == "" || seen[e.ID] || e.ID != containmentID(e.Kind, e.ParentID, e.ChildID) {
			return bad()
		}
		seen[e.ID] = true
	}
	for _, g := range analysisGraphs {
		for id, list := range x.out[g] {
			if err := analysisContext(ctx); err != nil {
				return nil, err
			}
			sort.Slice(list, func(i, j int) bool {
				a, b := v.Dependencies[list[i]], v.Dependencies[list[j]]
				if a.ToNodeID != b.ToNodeID {
					return a.ToNodeID < b.ToNodeID
				}
				return a.ID < b.ID
			})
			x.out[g][id] = list
		}
		for id, list := range x.in[g] {
			if err := analysisContext(ctx); err != nil {
				return nil, err
			}
			sort.Slice(list, func(i, j int) bool {
				a, b := v.Dependencies[list[i]], v.Dependencies[list[j]]
				if a.FromNodeID != b.FromNodeID {
					return a.FromNodeID < b.FromNodeID
				}
				return a.ID < b.ID
			})
			x.in[g][id] = list
		}
		s := x.summary[g]
		if s.BoundaryEdges > 0 {
			s.TopologyLimited = true
			s.ReasonCodes = append(s.ReasonCodes, "boundary_edges")
		}
		sort.Strings(s.ReasonCodes)
		x.summary[g] = s
	}
	var err error
	x.digest, err = analysisFingerprint(ctx, v)
	if err != nil {
		return nil, err
	}
	return x, analysisContext(ctx)
}

// Encode individual records, not the complete artifact: Encoder.Encode(view)
// internally buffers the entire view despite accepting an io.Writer.
func writeAnalysisArray[T any](ctx context.Context, w io.Writer, items []T) error {
	io.WriteString(w, "[")
	for i, item := range items {
		if err := analysisContext(ctx); err != nil {
			return err
		}
		if i > 0 {
			io.WriteString(w, ",")
		}
		// Public inventory views normalize nil evidence to []; reproduce that
		// representation without cloning the complete inventory. Only the local
		// record value changes; immutable input slices are never written.
		var record any = item
		switch v := record.(type) {
		case DependencyNode:
			if v.Evidence == nil {
				v.Evidence = []DependencyEvidence{}
			}
			record = v
		case DependencyEdge:
			if v.Evidence == nil {
				v.Evidence = []DependencyEvidence{}
			}
			record = v
		case ContainmentEdge:
			if v.Evidence == nil {
				v.Evidence = []DependencyEvidence{}
			}
			record = v
		}
		b, err := json.Marshal(record)
		if err != nil {
			return analysisError(ErrorIntegrity, "inconsistent_graph")
		}
		if _, err = w.Write(b); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}
func analysisFingerprint(ctx context.Context, v DependencyInventoryView) (string, error) {
	if err := analysisContext(ctx); err != nil {
		return "", err
	}
	h := sha256.New()
	var p [8]byte
	binary.BigEndian.PutUint64(p[:], uint64(len(AnalysisDigestScheme)))
	h.Write(p[:])
	io.WriteString(h, AnalysisDigestScheme)
	io.WriteString(h, `{"artifact":`)
	b, err := json.Marshal(v.Artifact)
	if err != nil {
		return "", analysisError(ErrorIntegrity, "inconsistent_graph")
	}
	h.Write(b)
	for _, s := range []struct {
		name  string
		write func() error
	}{
		{"source_artifacts", func() error { return writeAnalysisArray(ctx, h, v.SourceArtifacts) }}, {"nodes", func() error { return writeAnalysisArray(ctx, h, v.Nodes) }}, {"containment", func() error { return writeAnalysisArray(ctx, h, v.Containment) }}, {"dependencies", func() error { return writeAnalysisArray(ctx, h, v.Dependencies) }},
		{"strong_components", func() error { _, err := io.WriteString(h, "[]"); return err }}, {"cycles", func() error { _, err := io.WriteString(h, "[]"); return err }}, {"diagnostics", func() error { return writeAnalysisArray(ctx, h, v.Diagnostics) }},
	} {
		io.WriteString(h, `,"`+s.name+`":`)
		if err = s.write(); err != nil {
			return "", err
		}
	}
	io.WriteString(h, `,"statistics":`)
	b, err = json.Marshal(v.Statistics)
	if err != nil {
		return "", analysisError(ErrorIntegrity, "inconsistent_graph")
	}
	h.Write(b)
	io.WriteString(h, "}\n")
	return AnalysisDigestScheme + ":sha256:" + hex.EncodeToString(h.Sum(nil)), analysisContext(ctx)
}

func (a *graphAnalyzer) Analyze(ctx context.Context, input DependencyInventory) (DependencyInventory, error) {
	x, err := a.index(ctx, input)
	if err != nil {
		return DependencyInventory{}, err
	}
	return a.analyzeIndex(ctx, input, x)
}

// analyzeIndex owns its per-call index, including the derived summary counters.
// It is split from input/index extraction so validation can time both stages.
func (a *graphAnalyzer) analyzeIndex(ctx context.Context, input DependencyInventory, x *graphIndex) (DependencyInventory, error) {
	var err error
	components := []StrongComponent{}
	cycles := []DependencyCycle{}
	for _, g := range analysisGraphs {
		roots := []string{}
		for id, n := range x.nodes {
			if err := analysisContext(ctx); err != nil {
				return DependencyInventory{}, err
			}
			if localVertex(n, g) {
				roots = append(roots, id)
			}
		}
		sort.Strings(roots)
		indices, low := map[string]int{}, map[string]int{}
		on := map[string]bool{}
		stack := []string{}
		tick := 0
		type frame struct {
			id, parent string
			lastTarget string
			next       int
		}
		frames := []frame{}
		enter := func(id, parent string) {
			tick++
			indices[id] = tick
			low[id] = tick
			stack = append(stack, id)
			on[id] = true
			frames = append(frames, frame{id: id, parent: parent})
		}
		for _, root := range roots {
			if err := analysisContext(ctx); err != nil {
				return DependencyInventory{}, err
			}
			if indices[root] != 0 {
				continue
			}
			enter(root, "")
			for len(frames) > 0 {
				if err = analysisContext(ctx); err != nil {
					return DependencyInventory{}, err
				}
				fi := len(frames) - 1
				f := &frames[fi]
				list := x.out[g][f.id]
				if f.next < len(list) {
					e := x.view.Dependencies[list[f.next]]
					f.next++
					if !eligible(e, x) {
						continue
					}
					to := e.ToNodeID
					// Canonical adjacency groups endpoints; parallel kinds are one
					// SCC arc while original edge records remain queryable.
					if f.lastTarget == to {
						continue
					}
					f.lastTarget = to
					if indices[to] == 0 {
						enter(to, f.id)
					} else if on[to] && indices[to] < low[f.id] {
						low[f.id] = indices[to]
					}
					continue
				}
				id, parent := f.id, f.parent
				frames = frames[:fi]
				if parent != "" && low[id] < low[parent] {
					low[parent] = low[id]
				}
				if low[id] == indices[id] {
					members := []string{}
					for {
						if err := analysisContext(ctx); err != nil {
							return DependencyInventory{}, err
						}
						j := len(stack) - 1
						v := stack[j]
						stack = stack[:j]
						on[v] = false
						members = append(members, v)
						if v == id {
							break
						}
					}
					sort.Strings(members)
					self := false
					for _, v := range members {
						for _, ei := range x.out[g][v] {
							if err := analysisContext(ctx); err != nil {
								return DependencyInventory{}, err
							}
							e := x.view.Dependencies[ei]
							if eligible(e, x) && e.FromNodeID == e.ToNodeID {
								self = true
							}
						}
					}
					if uint64(len(components)) >= a.config.p.MaxComponents {
						return DependencyInventory{}, analysisError(ErrorLimitExceeded, "analysis_limit_exceeded")
					}
					fields := append([]string{string(g)}, members...)
					s := StrongComponent{ID: stableID("dependency-scc-id/v1", fields...), Graph: g, NodeIDs: members, Cyclic: len(members) > 1 || self, SelfLoop: self}
					components = append(components, s)
					summary := x.summary[g]
					summary.Components++
					if s.Cyclic {
						if uint64(len(cycles)) >= a.config.p.MaxCycles {
							return DependencyInventory{}, analysisError(ErrorLimitExceeded, "analysis_limit_exceeded")
						}
						cycles = append(cycles, DependencyCycle{ID: stableID("dependency-cycle-id/v1", string(g), s.ID), Graph: g, ComponentID: s.ID, NodeIDs: clone(members), Classification: "structural", Rule: "dependency-structural-scc/v1"})
						summary.CyclicComponents++
					}
					x.summary[g] = summary
				}
			}
		}
	}
	sort.Slice(components, func(i, j int) bool {
		a, b := components[i], components[j]
		if a.Graph != b.Graph {
			return a.Graph < b.Graph
		}
		if a.NodeIDs[0] != b.NodeIDs[0] {
			return a.NodeIDs[0] < b.NodeIDs[0]
		}
		if len(a.NodeIDs) != len(b.NodeIDs) {
			return len(a.NodeIDs) < len(b.NodeIDs)
		}
		return a.ID < b.ID
	})
	sort.Slice(cycles, func(i, j int) bool {
		a, b := cycles[i], cycles[j]
		if a.Graph != b.Graph {
			return a.Graph < b.Graph
		}
		return a.ComponentID < b.ComponentID
	})
	if err = analysisContext(ctx); err != nil {
		return DependencyInventory{}, err
	}
	v := cloneView(input.view)
	v.StrongComponents = components
	v.Cycles = cycles
	v.Analysis = &AnalysisMetadata{EngineName: "dependency-graph-analysis", EngineVersion: ContractVersion, InputDigest: x.digest, DigestScheme: AnalysisDigestScheme, SCCIDScheme: "dependency-scc-id/v1", CycleIDScheme: "dependency-cycle-id/v1", ProjectionPolicy: "dependency-local-projection/v1", Graphs: []GraphAnalysis{}}
	for _, g := range analysisGraphs {
		v.Analysis.Graphs = append(v.Analysis.Graphs, x.summary[g])
	}
	if err = analysisContext(ctx); err != nil {
		return DependencyInventory{}, err
	}
	return DependencyInventory{v}, nil
}
