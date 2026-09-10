package die

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
)

func pageCursor(d string, g GraphKind, dir TraversalDirection, root string, size uint32, last string) string {
	b, _ := json.Marshal([]any{AnalysisCursorScheme, d, g, dir, root, size, last})
	return base64.RawURLEncoding.EncodeToString(b)
}
func cursorLast(cursor, d string, g GraphKind, dir TraversalDirection, root string, size uint32) (string, error) {
	bad := func() (string, error) { return "", analysisError(ErrorInvalidInput, "cursor_mismatch") }
	if len(cursor) > 2048 {
		return bad()
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(cursor)
	if err != nil {
		return bad()
	}
	var fields []json.RawMessage
	if json.Unmarshal(b, &fields) != nil || len(fields) != 7 {
		return bad()
	}
	var last string
	if json.Unmarshal(fields[6], &last) != nil || !analysisNodeID(last) {
		return bad()
	}
	if pageCursor(d, g, dir, root, size, last) != cursor {
		return bad()
	}
	return last, nil
}
func (a *graphAnalyzer) DirectDependencies(ctx context.Context, i DependencyInventory, q NodeQuery) (NodePage, error) {
	return a.direct(ctx, i, q, Dependencies)
}
func (a *graphAnalyzer) DirectDependents(ctx context.Context, i DependencyInventory, q NodeQuery) (NodePage, error) {
	return a.direct(ctx, i, q, Dependents)
}
func (a *graphAnalyzer) direct(ctx context.Context, input DependencyInventory, q NodeQuery, dir TraversalDirection) (NodePage, error) {
	if err := analysisContext(ctx); err != nil {
		return NodePage{}, err
	}
	if !q.valid {
		return NodePage{}, analysisError(ErrorInvalidInput, "invalid_query")
	}
	p := q.p
	if p.PageSize == 0 {
		p.PageSize = a.config.p.MaxPageSize
	}
	if p.PageSize > a.config.p.MaxPageSize {
		return NodePage{}, analysisError(ErrorInvalidInput, "invalid_query")
	}
	x, err := a.index(ctx, input)
	if err != nil {
		return NodePage{}, err
	}
	n, exists := x.nodes[p.NodeID]
	if !exists || (!localVertex(n, p.Graph) && len(x.out[p.Graph][p.NodeID]) == 0 && len(x.in[p.Graph][p.NodeID]) == 0) {
		return NodePage{}, analysisError(ErrorInvalidInput, "invalid_query")
	}
	list := x.out[p.Graph][p.NodeID]
	if dir == Dependents {
		list = x.in[p.Graph][p.NodeID]
	}
	neighbors := []Neighbor{}
	for _, ei := range list {
		if err = analysisContext(ctx); err != nil {
			return NodePage{}, err
		}
		e := x.view.Dependencies[ei]
		id := e.ToNodeID
		if dir == Dependents {
			id = e.FromNodeID
		}
		if len(neighbors) == 0 || neighbors[len(neighbors)-1].NodeID != id {
			neighbors = append(neighbors, Neighbor{NodeID: id, Resolution: x.nodes[id].Resolution, EdgeIDs: []string{}})
		}
		j := len(neighbors) - 1
		neighbors[j].EdgeIDs = append(neighbors[j].EdgeIDs, e.ID)
	}
	start := 0
	if p.Cursor != "" {
		last, err := cursorLast(p.Cursor, x.digest, p.Graph, dir, p.NodeID, p.PageSize)
		if err != nil {
			return NodePage{}, err
		}
		at := sort.Search(len(neighbors), func(i int) bool { return neighbors[i].NodeID >= last })
		if at == len(neighbors) || neighbors[at].NodeID != last {
			return NodePage{}, analysisError(ErrorInvalidInput, "cursor_mismatch")
		}
		start = at + 1
	}
	end := start + int(p.PageSize)
	if end > len(neighbors) {
		end = len(neighbors)
	}
	s := x.summary[p.Graph]
	v := NodePageView{InputDigest: x.digest, Graph: p.Graph, Direction: dir, RootNodeID: p.NodeID, Neighbors: clone(neighbors[start:end]), HasMore: end < len(neighbors), TopologyLimited: s.TopologyLimited, ExplanationLimited: s.ExplanationLimited, ReasonCodes: clone(s.ReasonCodes)}
	if v.HasMore {
		v.NextCursor = pageCursor(x.digest, p.Graph, dir, p.NodeID, p.PageSize, neighbors[end-1].NodeID)
	}
	if err = analysisContext(ctx); err != nil {
		return NodePage{}, err
	}
	return NodePage{v}, nil
}
func (a *graphAnalyzer) Impact(ctx context.Context, input DependencyInventory, q ImpactQuery) (ImpactResult, error) {
	if err := analysisContext(ctx); err != nil {
		return ImpactResult{}, err
	}
	if !q.valid {
		return ImpactResult{}, analysisError(ErrorInvalidInput, "invalid_query")
	}
	p := q.p
	c := a.config.p
	if p.MaxNodes == 0 {
		p.MaxNodes = c.MaxTraversalNodes
	}
	if p.MaxEdges == 0 {
		p.MaxEdges = c.MaxTraversalEdges
	}
	if p.MaxDepth == 0 {
		p.MaxDepth = c.MaxTraversalDepth
	}
	if p.MaxNodes > c.MaxTraversalNodes || p.MaxEdges > c.MaxTraversalEdges || p.MaxDepth > c.MaxTraversalDepth || uint64(len(p.NodeIDs)) > p.MaxNodes {
		return ImpactResult{}, analysisError(ErrorInvalidInput, "invalid_query")
	}
	x, err := a.index(ctx, input)
	if err != nil {
		return ImpactResult{}, err
	}
	s := x.summary[p.Graph]
	v := ImpactResultView{InputDigest: x.digest, Graph: p.Graph, Direction: p.Direction, Seeds: clone(p.NodeIDs), Reached: []ReachedNode{}, BoundaryNodeIDs: []string{}, EdgeIDs: []string{}, ReasonCodes: clone(s.ReasonCodes), TopologyLimited: s.TopologyLimited, ExplanationLimited: s.ExplanationLimited}
	seen := map[string]bool{}
	localSeen := map[string]bool{}
	boundary := map[string]bool{}
	edges := map[string]bool{}
	for _, id := range p.NodeIDs {
		if err := analysisContext(ctx); err != nil {
			return ImpactResult{}, err
		}
		if !localVertex(x.nodes[id], p.Graph) {
			return ImpactResult{}, analysisError(ErrorInvalidInput, "invalid_query")
		}
		seen[id] = true
		localSeen[id] = true
	}
	v.VisitedNodes = uint64(len(seen))
	front := clone(p.NodeIDs)
	stop := func(reason string) { v.Truncated = true; v.ReasonCodes = append(v.ReasonCodes, reason) }
outer:
	for depth := uint32(0); len(front) > 0; depth++ {
		if err = analysisContext(ctx); err != nil {
			return ImpactResult{}, err
		}
		if depth >= p.MaxDepth {
			stop("depth_limit")
			break
		}
		next := []string{}
		for _, id := range front {
			if err := analysisContext(ctx); err != nil {
				return ImpactResult{}, err
			}
			list := x.out[p.Graph][id]
			if p.Direction == Dependents {
				list = x.in[p.Graph][id]
			}
			for _, ei := range list {
				if err = analysisContext(ctx); err != nil {
					return ImpactResult{}, err
				}
				if v.ScannedEdges == p.MaxEdges {
					stop("edge_limit")
					break outer
				}
				v.ScannedEdges++
				e := x.view.Dependencies[ei]
				target := e.ToNodeID
				if p.Direction == Dependents {
					target = e.FromNodeID
				}
				// An uncertain edge cannot grant local reachability, even if its target node
				// exists locally. Record that target as a terminal boundary instead.
				isLocal := eligible(e, x)
				if !seen[target] {
					if v.VisitedNodes == p.MaxNodes {
						stop("node_limit")
						break outer
					}
					seen[target] = true
					v.VisitedNodes++
				}
				if isLocal && !localSeen[target] {
					localSeen[target] = true
					delete(boundary, target)
					v.Reached = append(v.Reached, ReachedNode{target, depth + 1})
					next = append(next, target)
					if depth+1 > v.MaxDepthReached {
						v.MaxDepthReached = depth + 1
					}
				} else if !isLocal && !localSeen[target] {
					boundary[target] = true
				}
				edges[e.ID] = true
			}
		}
		sort.Strings(next)
		front = next
	}
	for id := range boundary {
		if err := analysisContext(ctx); err != nil {
			return ImpactResult{}, err
		}
		v.BoundaryNodeIDs = append(v.BoundaryNodeIDs, id)
	}
	for id := range edges {
		if err := analysisContext(ctx); err != nil {
			return ImpactResult{}, err
		}
		v.EdgeIDs = append(v.EdgeIDs, id)
	}
	sort.Strings(v.EdgeIDs)
	sort.Strings(v.BoundaryNodeIDs)
	sort.Slice(v.Reached, func(i, j int) bool {
		a, b := v.Reached[i], v.Reached[j]
		if a.Depth != b.Depth {
			return a.Depth < b.Depth
		}
		return a.NodeID < b.NodeID
	})
	sort.Strings(v.ReasonCodes)
	if err = analysisContext(ctx); err != nil {
		return ImpactResult{}, err
	}
	return ImpactResult{v}, nil
}
