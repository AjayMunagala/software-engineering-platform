package die

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
)

const checkpointInterval = 1024

type core struct{ config Config }

func (*core) Name() string    { return EngineName }
func (*core) Version() string { return EngineVersion }

func (c *core) Normalize(ctx context.Context, input GraphInput) (DependencyInventory, error) {
	if err := ctx.Err(); err != nil {
		return DependencyInventory{}, contextError(err)
	}
	sources, err := normalizeSources(input.SourceArtifacts)
	if err != nil {
		return DependencyInventory{}, err
	}
	nodes, byIdentity, omittedNodeEvidence, err := c.normalizeNodes(ctx, input.Nodes)
	if err != nil {
		return DependencyInventory{}, err
	}
	containment, omittedContainment, omittedContainmentEvidence, err := c.normalizeContainment(ctx, input.Containment, byIdentity)
	if err != nil {
		return DependencyInventory{}, err
	}
	edges, omittedEdges, omittedEdgeEvidence, edgesByGraph, edgesByResolution, err := c.normalizeEdges(ctx, input.Dependencies, byIdentity)
	if err != nil {
		return DependencyInventory{}, err
	}
	diagnostics, omittedDiagnostics, err := c.normalizeDiagnostics(input.Diagnostics)
	if err != nil {
		return DependencyInventory{}, err
	}
	stats := DependencyStatistics{NodesByKind: map[string]uint64{}, EdgesByGraph: edgesByGraph, EdgesByResolution: edgesByResolution, Diagnostics: uint64(len(diagnostics)) + omittedDiagnostics, OmittedDiagnostics: omittedDiagnostics, OmittedContainment: omittedContainment, OmittedEdges: omittedEdges, OmittedEvidence: omittedNodeEvidence + omittedContainmentEvidence + omittedEdgeEvidence}
	for _, n := range nodes {
		stats.NodesByKind[string(n.Kind)]++
	}
	view := DependencyInventoryView{Artifact: ArtifactMetadata{Name: ArtifactName, Version: ArtifactVersion, EngineName: EngineName, EngineVersion: EngineVersion, NodeIDSchemeVersion: "dependency-node-id/v1", EdgeIDSchemeVersion: "dependency-edge-id/v1", ContainmentIDSchemeVersion: "dependency-containment-id/v1"}, SourceArtifacts: sources, Nodes: nodes, Containment: containment, Dependencies: edges, StrongComponents: []StrongComponent{}, Cycles: []DependencyCycle{}, Diagnostics: diagnostics, Statistics: stats}
	return DependencyInventory{view: cloneView(view)}, nil
}

func normalizeSources(in []ArtifactReference) ([]ArtifactReference, error) {
	out := clone(in)
	for _, v := range out {
		if !safeText(v.Name) || !safeText(v.Version) || v.Name == "" || v.Version == "" {
			return nil, newError(ErrorInvalidInput, "invalid_source_artifact", "source artifact identity is invalid", nil)
		}
		if v.Digest != "" && !validDigest(v.Digest) {
			return nil, newError(ErrorInvalidInput, "invalid_source_digest", "source artifact digest is invalid", nil)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].Version != out[j].Version {
			return out[i].Version < out[j].Version
		}
		return out[i].Digest < out[j].Digest
	})
	for i := 1; i < len(out); i++ {
		if out[i] == out[i-1] {
			return nil, newError(ErrorInvalidInput, "duplicate_source_artifact", "duplicate source artifact", nil)
		}
	}
	return out, nil
}

func (c *core) normalizeNodes(ctx context.Context, in []NodeCandidate) ([]DependencyNode, map[string]string, uint64, error) {
	byID := make(map[string]DependencyNode, len(in))
	identities := make(map[string]string, len(in))
	for i, candidate := range in {
		if i%checkpointInterval == 0 {
			if err := ctx.Err(); err != nil {
				return nil, nil, 0, contextError(err)
			}
		}
		identity, err := normalizeIdentity(candidate.Identity)
		if err != nil {
			return nil, nil, 0, err
		}
		if err = validateSource(candidate.SourceIdentity); err != nil {
			return nil, nil, 0, err
		}
		id := nodeID(identity)
		key := identityKey(identity)
		ev, _, err := normalizeEvidence(candidate.Evidence, ^uint64(0))
		if err != nil {
			return nil, nil, 0, err
		}
		n := DependencyNode{ID: id, Kind: identity.Kind, Language: identity.Language, Name: candidate.Name, QualifiedName: identity.QualifiedName, RepositoryPath: identity.RepositoryPath, Resolution: identity.Resolution, SourceIdentity: candidate.SourceIdentity, Evidence: ev}
		if prior, ok := byID[id]; ok {
			if prior.Name != n.Name || prior.SourceIdentity != n.SourceIdentity {
				return nil, nil, 0, newError(ErrorIntegrity, "conflicting_node", "duplicate node identity has conflicting metadata", nil)
			}
			prior.Evidence = mergeEvidence(prior.Evidence, n.Evidence)
			byID[id] = prior
		} else {
			byID[id] = n
			identities[key] = id
			if uint64(len(byID)) > c.config.MaxNodes() {
				return nil, nil, 0, newError(ErrorLimitExceeded, "max_nodes_exceeded", fmt.Sprintf("normalized node count exceeds MaxNodes (%d)", c.config.MaxNodes()), nil)
			}
		}
	}
	out := make([]DependencyNode, 0, len(byID))
	var omittedEvidence uint64
	for _, v := range byID {
		var omitted uint64
		v.Evidence, omitted, _ = normalizeEvidence(v.Evidence, c.config.MaxEvidencePerItem())
		omittedEvidence += omitted
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.QualifiedName != b.QualifiedName {
			return a.QualifiedName < b.QualifiedName
		}
		if a.RepositoryPath != b.RepositoryPath {
			return a.RepositoryPath < b.RepositoryPath
		}
		return a.ID < b.ID
	})
	return out, identities, omittedEvidence, nil
}

type containmentAggregate struct {
	edge     ContainmentEdge
	evidence []DependencyEvidence
}

func (c *core) normalizeContainment(ctx context.Context, in []ContainmentCandidate, nodes map[string]string) ([]ContainmentEdge, uint64, uint64, error) {
	agg := map[string]containmentAggregate{}
	for i, v := range in {
		if i%checkpointInterval == 0 {
			if err := ctx.Err(); err != nil {
				return nil, 0, 0, contextError(err)
			}
		}
		if !validContainment(v.Kind) {
			return nil, 0, 0, newError(ErrorInvalidInput, "invalid_containment_kind", "containment kind is invalid", nil)
		}
		p, err := normalizeIdentity(v.Parent)
		if err != nil {
			return nil, 0, 0, err
		}
		ch, err := normalizeIdentity(v.Child)
		if err != nil {
			return nil, 0, 0, err
		}
		pid, pok := nodes[identityKey(p)]
		cid, cok := nodes[identityKey(ch)]
		if !pok || !cok {
			return nil, 0, 0, newError(ErrorIntegrity, "missing_containment_node", "containment endpoint is not a normalized node", nil)
		}
		id := containmentID(v.Kind, pid, cid)
		ev, _, err := normalizeEvidence(v.Evidence, ^uint64(0))
		if err != nil {
			return nil, 0, 0, err
		}
		a := agg[id]
		if a.edge.ID == "" {
			a.edge = ContainmentEdge{ID: id, Kind: v.Kind, ParentID: pid, ChildID: cid}
		}
		a.evidence = append(a.evidence, ev...)
		agg[id] = a
	}
	out := make([]ContainmentEdge, 0, len(agg))
	var omittedEvidence uint64
	for _, a := range agg {
		a.edge.Evidence, a.edge.OmittedEvidence, _ = normalizeEvidence(a.evidence, c.config.MaxEvidencePerItem())
		omittedEvidence += a.edge.OmittedEvidence
		out = append(out, a.edge)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.ParentID != b.ParentID {
			return a.ParentID < b.ParentID
		}
		if a.ChildID != b.ChildID {
			return a.ChildID < b.ChildID
		}
		return a.ID < b.ID
	})
	var omitted uint64
	if uint64(len(out)) > c.config.MaxEdges() {
		omitted = uint64(len(out)) - c.config.MaxEdges()
		out = out[:c.config.MaxEdges()]
	}
	return out, omitted, omittedEvidence, nil
}

type edgeAggregate struct {
	edge     DependencyEdge
	evidence []DependencyEvidence
}

func (c *core) normalizeEdges(ctx context.Context, in []DependencyCandidate, nodes map[string]string) ([]DependencyEdge, uint64, uint64, map[string]uint64, map[string]uint64, error) {
	agg := map[string]edgeAggregate{}
	for i, v := range in {
		if i%checkpointInterval == 0 {
			if err := ctx.Err(); err != nil {
				return nil, 0, 0, nil, nil, contextError(err)
			}
		}
		if !validGraph(v.Graph) || !validDependency(v.Kind) || !validResolution(v.Resolution) {
			return nil, 0, 0, nil, nil, newError(ErrorInvalidInput, "invalid_dependency", "dependency classification is invalid", nil)
		}
		from, err := normalizeIdentity(v.From)
		if err != nil {
			return nil, 0, 0, nil, nil, err
		}
		to, err := normalizeIdentity(v.To)
		if err != nil {
			return nil, 0, 0, nil, nil, err
		}
		fid, fok := nodes[identityKey(from)]
		tid, tok := nodes[identityKey(to)]
		if !fok || !tok {
			return nil, 0, 0, nil, nil, newError(ErrorIntegrity, "missing_dependency_node", "dependency endpoint is not a normalized node", nil)
		}
		id := edgeID(v.Graph, v.Kind, fid, tid, v.Resolution)
		ev, _, err := normalizeEvidence(v.Evidence, ^uint64(0))
		if err != nil {
			return nil, 0, 0, nil, nil, err
		}
		a := agg[id]
		if a.edge.ID == "" {
			a.edge = DependencyEdge{ID: id, Graph: v.Graph, Kind: v.Kind, FromNodeID: fid, ToNodeID: tid, Resolution: v.Resolution}
		}
		a.edge.Occurrences++
		a.evidence = append(a.evidence, ev...)
		agg[id] = a
	}
	out := make([]DependencyEdge, 0, len(agg))
	byGraph, byResolution := map[string]uint64{}, map[string]uint64{}
	var omittedEvidence uint64
	for _, a := range agg {
		a.edge.Evidence, a.edge.OmittedEvidence, _ = normalizeEvidence(a.evidence, c.config.MaxEvidencePerItem())
		omittedEvidence += a.edge.OmittedEvidence
		byGraph[string(a.edge.Graph)]++
		byResolution[string(a.edge.Resolution)]++
		out = append(out, a.edge)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Graph != b.Graph {
			return a.Graph < b.Graph
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.FromNodeID != b.FromNodeID {
			return a.FromNodeID < b.FromNodeID
		}
		if a.ToNodeID != b.ToNodeID {
			return a.ToNodeID < b.ToNodeID
		}
		if a.Resolution != b.Resolution {
			return a.Resolution < b.Resolution
		}
		return a.ID < b.ID
	})
	var omitted uint64
	if uint64(len(out)) > c.config.MaxEdges() {
		omitted = uint64(len(out)) - c.config.MaxEdges()
		out = out[:c.config.MaxEdges()]
	}
	return out, omitted, omittedEvidence, byGraph, byResolution, nil
}

func (c *core) normalizeDiagnostics(in []DiagnosticCandidate) ([]Diagnostic, uint64, error) {
	out := make([]Diagnostic, 0, len(in))
	for _, v := range in {
		file, err := normalizePath(v.File)
		if err != nil {
			return nil, 0, err
		}
		if v.Code == "" || !safeText(v.Code) || !safeText(v.Message) {
			return nil, 0, newError(ErrorInvalidInput, "invalid_diagnostic", "diagnostic is invalid", nil)
		}
		out = append(out, Diagnostic{v.Code, v.Graph, file, v.StartLine, v.StartColumn, v.StableID, v.Message})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Graph != b.Graph {
			return a.Graph < b.Graph
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		if a.StartColumn != b.StartColumn {
			return a.StartColumn < b.StartColumn
		}
		return a.StableID < b.StableID
	})
	out = dedupe(out, func(v Diagnostic) string {
		return fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d\x00%s", v.Code, v.Graph, v.File, v.StartLine, v.StartColumn, v.StableID)
	})
	var omitted uint64
	if uint64(len(out)) > c.config.MaxDiagnostics() {
		omitted = uint64(len(out)) - c.config.MaxDiagnostics()
		out = out[:c.config.MaxDiagnostics()]
	}
	return out, omitted, nil
}

func normalizeIdentity(v NodeIdentity) (NodeIdentity, error) {
	if !validNode(v.Kind) || !validResolution(v.Resolution) || v.QualifiedName == "" || !safeText(v.Language) || !safeText(v.QualifiedName) {
		return NodeIdentity{}, newError(ErrorInvalidInput, "invalid_node_identity", "node identity is invalid", nil)
	}
	p, err := normalizePath(v.RepositoryPath)
	if err != nil {
		return NodeIdentity{}, err
	}
	v.RepositoryPath = p
	return v, nil
}
func validateSource(v SourceIdentity) error {
	if v.ArtifactName == "" || v.ArtifactVersion == "" || v.SourceID == "" || !safeText(v.ArtifactName) || !safeText(v.ArtifactVersion) || !safeText(v.SourceID) {
		return newError(ErrorInvalidInput, "invalid_source_identity", "source identity is invalid", nil)
	}
	return nil
}
func normalizeEvidence(in []DependencyEvidence, limit uint64) ([]DependencyEvidence, uint64, error) {
	out := clone(in)
	for i := range out {
		if err := validateSource(out[i].Source); err != nil {
			return nil, 0, err
		}
		p, err := normalizePath(out[i].File)
		if err != nil {
			return nil, 0, err
		}
		out[i].File = p
		if out[i].Rule == "" || !safeText(out[i].Rule) || !safeText(out[i].Value) || out[i].StartLine < 0 || out[i].StartColumn < 0 {
			return nil, 0, newError(ErrorInvalidInput, "invalid_evidence", "evidence is invalid", nil)
		}
	}
	sort.Slice(out, func(i, j int) bool { return evidenceKey(out[i]) < evidenceKey(out[j]) })
	out = dedupe(out, evidenceKey)
	var omitted uint64
	if uint64(len(out)) > limit {
		omitted = uint64(len(out)) - limit
		out = out[:limit]
	}
	return out, omitted, nil
}
func mergeEvidence(a, b []DependencyEvidence) []DependencyEvidence { return append(clone(a), b...) }
func evidenceKey(v DependencyEvidence) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%010d\x00%010d\x00%s\x00%s", v.Source.ArtifactName, v.Source.ArtifactVersion, v.Source.SourceID, v.File, v.StartLine, v.StartColumn, v.Rule, v.Value)
}
func identityKey(v NodeIdentity) string {
	return string(v.Kind) + "\x00" + v.Language + "\x00" + v.QualifiedName + "\x00" + v.RepositoryPath + "\x00" + string(v.Resolution)
}
func nodeID(v NodeIdentity) string {
	return stableID("dependency-node-id/v1", string(v.Kind), v.Language, v.QualifiedName, v.RepositoryPath, string(v.Resolution))
}
func edgeID(g GraphKind, k DependencyKind, from, to string, r ResolutionState) string {
	return stableID("dependency-edge-id/v1", string(g), string(k), from, to, string(r))
}
func containmentID(k ContainmentKind, parent, child string) string {
	return stableID("dependency-containment-id/v1", string(k), parent, child)
}
func stableID(domain string, fields ...string) string {
	h := sha256.New()
	parts := append([]string{domain}, fields...)
	var b [8]byte
	for _, v := range parts {
		binary.BigEndian.PutUint64(b[:], uint64(len([]byte(v))))
		_, _ = h.Write(b[:])
		_, _ = h.Write([]byte(v))
	}
	return domain + ":sha256:" + hex.EncodeToString(h.Sum(nil))
}
func normalizePath(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	v = strings.ReplaceAll(v, "\\", "/")
	if strings.HasPrefix(v, "/") || strings.Contains(v, ":") || strings.ContainsRune(v, 0) {
		return "", newError(ErrorInvalidInput, "unsafe_repository_path", "repository path must be relative", nil)
	}
	clean := path.Clean(v)
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", newError(ErrorInvalidInput, "unsafe_repository_path", "repository path escapes repository", nil)
	}
	return clean, nil
}
func safeText(v string) bool { return !strings.ContainsRune(v, 0) && !strings.ContainsAny(v, "\r\n") }
func validDigest(v string) bool {
	if len(v) != 64 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil && v == strings.ToLower(v)
}
func validGraph(v GraphKind) bool { return v == GraphModule || v == GraphPackage || v == GraphFile }
func validNode(v NodeKind) bool {
	return v == NodeModule || v == NodePackage || v == NodeFile || v == NodeStandardLibrary || v == NodeExternalPackage || v == NodeUnresolved
}
func validDependency(v DependencyKind) bool {
	return v == DependencyImports || v == DependencyReferences
}
func validContainment(v ContainmentKind) bool {
	return v == ContainmentModulePackage || v == ContainmentPackageFile
}
func validResolution(v ResolutionState) bool {
	return v == ResolvedLocal || v == StandardLibrary || v == External || v == Unresolved || v == Ambiguous || v == Stale
}
func dedupe[T any](in []T, key func(T) string) []T {
	if len(in) == 0 {
		return []T{}
	}
	out := in[:0]
	last := ""
	for i, v := range in {
		k := key(v)
		if i == 0 || k != last {
			out = append(out, v)
			last = k
		}
	}
	return out
}
