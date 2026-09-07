package spike

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

type runner struct {
	config     Config
	checkpoint func(int)
}

type edgeKey struct {
	graph      GraphKind
	kind       EdgeKind
	from       string
	to         string
	resolution Resolution
}

type edgeAccumulator struct {
	key         edgeKey
	evidence    []Evidence
	occurrences int
}

func (engine *runner) Run(ctx context.Context, input Input) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("%w: context is required", ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	nodes, nodeByLogical, err := normalizeNodes(ctx, input.Nodes, engine.checkpoint)
	if err != nil {
		return Result{}, err
	}
	for index, raw := range input.Edges {
		if index > 0 && index%checkpointInterval == 0 {
			if engine.checkpoint != nil {
				engine.checkpoint(index)
			}
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
		}
		for _, identity := range []NodeIdentity{raw.From, raw.To} {
			key := logicalNodeKey(identity)
			if _, ok := nodeByLogical[key]; !ok {
				return Result{}, fmt.Errorf("%w: edge references undeclared node %q", ErrInvalidInput, identity.QualifiedName)
			}
		}
	}

	partials := make([]map[edgeKey]*edgeAccumulator, engine.config.Workers)
	var wait sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	for worker := 0; worker < engine.config.Workers; worker++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			local := make(map[edgeKey]*edgeAccumulator)
			for item := index; item < len(input.Edges); item += engine.config.Workers {
				if item%checkpointInterval == 0 {
					if engine.checkpoint != nil {
						engine.checkpoint(item)
					}
					if err := ctx.Err(); err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
						return
					}
				}
				raw := input.Edges[item]
				key := edgeKey{raw.Graph, raw.Kind, nodeByLogical[logicalNodeKey(raw.From)].ID, nodeByLogical[logicalNodeKey(raw.To)].ID, raw.Resolution}
				entry := local[key]
				if entry == nil {
					entry = &edgeAccumulator{key: key}
					local[key] = entry
				}
				entry.occurrences++
				entry.evidence = append(entry.evidence, normalizeEvidence(raw.Evidence))
			}
			partials[index] = local
		}(worker)
	}
	wait.Wait()
	if firstErr != nil {
		return Result{}, firstErr
	}

	merged := make(map[edgeKey]*edgeAccumulator)
	for _, partial := range partials {
		for key, value := range partial {
			entry := merged[key]
			if entry == nil {
				entry = &edgeAccumulator{key: key}
				merged[key] = entry
			}
			entry.occurrences += value.occurrences
			entry.evidence = append(entry.evidence, value.evidence...)
		}
	}
	edges := make([]Edge, 0, len(merged))
	statistics := Statistics{Nodes: len(nodes)}
	for _, value := range merged {
		sortEvidence(value.evidence)
		value.evidence = compactEvidence(value.evidence)
		kept := value.evidence
		omitted := 0
		if len(kept) > engine.config.MaxEvidencePerEdge {
			omitted = len(kept) - engine.config.MaxEvidencePerEdge
			kept = kept[:engine.config.MaxEvidencePerEdge]
		}
		edges = append(edges, Edge{
			ID: edgeID(value.key), Graph: value.key.graph, Kind: value.key.kind,
			FromID: value.key.from, ToID: value.key.to, Resolution: value.key.resolution,
			Occurrences: value.occurrences, Evidence: append([]Evidence(nil), kept...), OmittedEvidence: omitted,
		})
		statistics.Occurrences += value.occurrences
		statistics.OmittedEvidence += omitted
	}
	sortEdges(edges)
	if len(edges) > engine.config.MaxEdges {
		statistics.OmittedEdges = len(edges) - engine.config.MaxEdges
		edges = edges[:engine.config.MaxEdges]
	}
	statistics.Edges = len(edges)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	components, cycles, err := analyzeComponents(ctx, nodes, edges, engine.checkpoint)
	if err != nil {
		return Result{}, err
	}
	statistics.Components = len(components)
	statistics.CyclicComponents = len(cycles)
	return Result{Nodes: nodes, Edges: edges, Components: components, Cycles: cycles, Statistics: statistics}, nil
}

func normalizeNodes(ctx context.Context, input []NodeIdentity, checkpoint func(int)) ([]Node, map[string]Node, error) {
	byLogical := make(map[string]Node, len(input))
	for index, raw := range input {
		if index > 0 && index%checkpointInterval == 0 {
			if checkpoint != nil {
				checkpoint(index)
			}
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
		}
		raw.Language = strings.TrimSpace(raw.Language)
		raw.QualifiedName = strings.TrimSpace(raw.QualifiedName)
		raw.Path = normalizePath(raw.Path)
		if raw.Kind == "" || raw.QualifiedName == "" || raw.Resolution == "" {
			return nil, nil, fmt.Errorf("%w: incomplete node identity", ErrInvalidInput)
		}
		key := logicalNodeKey(raw)
		if existing, ok := byLogical[key]; ok {
			if existing.NodeIdentity != raw {
				return nil, nil, fmt.Errorf("%w: conflicting node identity", ErrInvalidInput)
			}
			continue
		}
		byLogical[key] = Node{ID: nodeID(raw), NodeIdentity: raw}
	}
	nodes := make([]Node, 0, len(byLogical))
	for _, node := range byLogical {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind < nodes[j].Kind
		}
		if nodes[i].QualifiedName != nodes[j].QualifiedName {
			return nodes[i].QualifiedName < nodes[j].QualifiedName
		}
		if nodes[i].Path != nodes[j].Path {
			return nodes[i].Path < nodes[j].Path
		}
		return nodes[i].ID < nodes[j].ID
	})
	return nodes, byLogical, nil
}

func analyzeComponents(ctx context.Context, nodes []Node, edges []Edge, checkpoint func(int)) ([]Component, []Cycle, error) {
	components := make([]Component, 0)
	cycles := make([]Cycle, 0)
	nodeByID := make(map[string]Node, len(nodes))
	for _, node := range nodes {
		nodeByID[node.ID] = node
	}
	for _, graph := range []GraphKind{GraphModule, GraphPackage, GraphFile} {
		ids := make([]string, 0)
		for _, node := range nodes {
			if nodeMatchesGraph(node, graph) {
				ids = append(ids, node.ID)
			}
		}
		sort.Strings(ids)
		adj := make(map[string][]string, len(ids))
		self := make(map[string]bool)
		for _, id := range ids {
			adj[id] = []string{}
		}
		for _, edge := range edges {
			if edge.Graph != graph || edge.Resolution != ResolvedLocal {
				continue
			}
			if _, ok := adj[edge.FromID]; !ok {
				continue
			}
			if _, ok := adj[edge.ToID]; !ok {
				continue
			}
			adj[edge.FromID] = append(adj[edge.FromID], edge.ToID)
			if edge.FromID == edge.ToID {
				self[edge.FromID] = true
			}
		}
		for id := range adj {
			sort.Strings(adj[id])
			adj[id] = compactStrings(adj[id])
		}
		groups, err := tarjan(ctx, ids, adj, checkpoint)
		if err != nil {
			return nil, nil, err
		}
		for _, group := range groups {
			cyclic := len(group) > 1 || (len(group) == 1 && self[group[0]])
			component := Component{ID: componentID(graph, group), Graph: graph, NodeIDs: group, Cyclic: cyclic, SelfLoop: len(group) == 1 && self[group[0]]}
			components = append(components, component)
			if cyclic {
				classification, rule := classifyCycle(graph, group, nodeByID)
				cycles = append(cycles, Cycle{ID: cycleID(graph, component.ID), Graph: graph, ComponentID: component.ID, NodeIDs: append([]string(nil), group...), Classification: classification, Rule: rule})
			}
		}
	}
	sort.Slice(components, func(i, j int) bool {
		if components[i].Graph != components[j].Graph {
			return components[i].Graph < components[j].Graph
		}
		return components[i].ID < components[j].ID
	})
	sort.Slice(cycles, func(i, j int) bool {
		if cycles[i].Graph != cycles[j].Graph {
			return cycles[i].Graph < cycles[j].Graph
		}
		return cycles[i].ID < cycles[j].ID
	})
	return components, cycles, nil
}

func classifyCycle(graph GraphKind, members []string, nodes map[string]Node) (string, string) {
	if graph == GraphFile {
		return "informational", "file-cycle-is-structural"
	}
	if graph == GraphPackage {
		allGo := len(members) > 0
		for _, member := range members {
			if nodes[member].Language != "Go" {
				allGo = false
				break
			}
		}
		if allGo {
			return "language_invalid", "go-import-cycle"
		}
	}
	return "structural", "structural-cycle"
}

func tarjan(ctx context.Context, nodes []string, adjacency map[string][]string, checkpoint func(int)) ([][]string, error) {
	index := 0
	indices := make(map[string]int, len(nodes))
	low := make(map[string]int, len(nodes))
	onStack := make(map[string]bool, len(nodes))
	stack := make([]string, 0, len(nodes))
	groups := make([][]string, 0)
	processed := 0
	check := func() error {
		processed++
		if processed%checkpointInterval == 0 {
			if checkpoint != nil {
				checkpoint(processed)
			}
			return ctx.Err()
		}
		return nil
	}
	for _, node := range nodes {
		indices[node] = -1
	}
	var visit func(string) error
	visit = func(node string) error {
		if err := check(); err != nil {
			return err
		}
		indices[node] = index
		low[node] = index
		index++
		stack = append(stack, node)
		onStack[node] = true
		for _, target := range adjacency[node] {
			if err := check(); err != nil {
				return err
			}
			if indices[target] == -1 {
				if err := visit(target); err != nil {
					return err
				}
				if low[target] < low[node] {
					low[node] = low[target]
				}
			} else if onStack[target] && indices[target] < low[node] {
				low[node] = indices[target]
			}
		}
		if low[node] == indices[node] {
			group := make([]string, 0)
			for {
				last := len(stack) - 1
				member := stack[last]
				stack = stack[:last]
				onStack[member] = false
				group = append(group, member)
				if member == node {
					break
				}
			}
			sort.Strings(group)
			groups = append(groups, group)
		}
		return nil
	}
	for _, node := range nodes {
		if indices[node] == -1 {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i][0] < groups[j][0] })
	return groups, ctx.Err()
}

func (engine *runner) Impact(ctx context.Context, result Result, query ImpactQuery) (ImpactResult, error) {
	if ctx == nil {
		return ImpactResult{}, fmt.Errorf("%w: context is required", ErrInvalidInput)
	}
	if query.MaxDepth == 0 {
		query.MaxDepth = engine.config.MaxTraversalDepth
	}
	if query.MaxNodes == 0 {
		query.MaxNodes = engine.config.MaxTraversalNodes
	}
	if query.MaxDepth < 1 || query.MaxNodes < 1 || (query.Direction != Dependencies && query.Direction != Dependents) {
		return ImpactResult{}, fmt.Errorf("%w: invalid impact query", ErrInvalidInput)
	}
	found := false
	for _, node := range result.Nodes {
		if node.ID == query.NodeID {
			found = true
			break
		}
	}
	if !found {
		return ImpactResult{}, ErrNodeNotFound
	}
	adj := make(map[string][]string)
	for _, edge := range result.Edges {
		if edge.Graph != query.Graph || edge.Resolution != ResolvedLocal {
			continue
		}
		from, to := edge.FromID, edge.ToID
		if query.Direction == Dependents {
			from, to = to, from
		}
		adj[from] = append(adj[from], to)
	}
	for id := range adj {
		sort.Strings(adj[id])
		adj[id] = compactStrings(adj[id])
	}
	queue := []ImpactNode{{NodeID: query.NodeID, Depth: 0}}
	seen := map[string]bool{query.NodeID: true}
	output := make([]ImpactNode, 0)
	traversed := 0
	truncated := false
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.Depth >= query.MaxDepth {
			if len(adj[current.NodeID]) > 0 {
				truncated = true
			}
			continue
		}
		for _, target := range adj[current.NodeID] {
			traversed++
			if traversed%checkpointInterval == 0 {
				if engine.checkpoint != nil {
					engine.checkpoint(traversed)
				}
				if err := ctx.Err(); err != nil {
					return ImpactResult{}, err
				}
			}
			if seen[target] {
				continue
			}
			if len(output) >= query.MaxNodes {
				truncated = true
				continue
			}
			seen[target] = true
			next := ImpactNode{NodeID: target, Depth: current.Depth + 1}
			output = append(output, next)
			queue = append(queue, next)
		}
	}
	return ImpactResult{Nodes: output, TraversedEdges: traversed, Truncated: truncated}, ctx.Err()
}

func nodeID(identity NodeIdentity) string {
	return stableID("dependency-node-id/v1", string(identity.Kind), identity.Language, identity.QualifiedName, normalizePath(identity.Path), string(identity.Resolution))
}
func edgeID(key edgeKey) string {
	return stableID("dependency-edge-id/v1", string(key.graph), string(key.kind), key.from, key.to, string(key.resolution))
}
func componentID(graph GraphKind, nodes []string) string {
	return stableID("dependency-scc-id/v1", append([]string{string(graph)}, nodes...)...)
}
func cycleID(graph GraphKind, component string) string {
	return stableID("dependency-cycle-id/v1", string(graph), component)
}

func stableID(domain string, segments ...string) string {
	hash := sha256.New()
	writeSegment(hash, domain)
	for _, segment := range segments {
		writeSegment(hash, segment)
	}
	return domain + ":sha256:" + hex.EncodeToString(hash.Sum(nil))
}

type byteWriter interface{ Write([]byte) (int, error) }

func writeSegment(writer byteWriter, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len([]byte(value))))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write([]byte(value))
}

func logicalNodeKey(value NodeIdentity) string {
	return strings.Join([]string{string(value.Kind), value.Language, value.QualifiedName, normalizePath(value.Path), string(value.Resolution)}, "\x00")
}
func normalizePath(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(path.Clean("/"+strings.TrimLeft(value, "/")), "/")
	if value == "." {
		return ""
	}
	return value
}
func normalizeEvidence(value Evidence) Evidence {
	value.Artifact = strings.TrimSpace(value.Artifact)
	value.Version = strings.TrimSpace(value.Version)
	value.SourceID = strings.TrimSpace(value.SourceID)
	value.File = normalizePath(value.File)
	value.Rule = strings.TrimSpace(value.Rule)
	value.Value = strings.TrimSpace(value.Value)
	return value
}
func evidenceKey(value Evidence) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%010d\x00%010d\x00%s\x00%s", value.Artifact, value.Version, value.SourceID, value.File, value.Line, value.Column, value.Rule, value.Value)
}
func sortEvidence(values []Evidence) {
	sort.Slice(values, func(i, j int) bool { return evidenceKey(values[i]) < evidenceKey(values[j]) })
}
func compactEvidence(values []Evidence) []Evidence {
	if len(values) == 0 {
		return []Evidence{}
	}
	output := values[:1]
	for _, value := range values[1:] {
		if value != output[len(output)-1] {
			output = append(output, value)
		}
	}
	return output
}
func compactStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	output := values[:1]
	for _, value := range values[1:] {
		if value != output[len(output)-1] {
			output = append(output, value)
		}
	}
	return output
}
func sortEdges(values []Edge) {
	sort.Slice(values, func(i, j int) bool {
		a, b := values[i], values[j]
		if a.Graph != b.Graph {
			return a.Graph < b.Graph
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.FromID != b.FromID {
			return a.FromID < b.FromID
		}
		if a.ToID != b.ToID {
			return a.ToID < b.ToID
		}
		if a.Resolution != b.Resolution {
			return a.Resolution < b.Resolution
		}
		return a.ID < b.ID
	})
}
func nodeMatchesGraph(node Node, graph GraphKind) bool {
	return (graph == GraphModule && node.Kind == NodeModule) || (graph == GraphPackage && node.Kind == NodePackage) || (graph == GraphFile && node.Kind == NodeFile)
}
