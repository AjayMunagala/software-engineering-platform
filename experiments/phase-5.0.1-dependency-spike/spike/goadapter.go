package spike

import (
	"fmt"
	"sort"
	"strings"

	golang "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/semantic"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// NormalizeReleasedGo converts released Go artifact facts into experimental
// language-neutral graph input. It performs no source or repository I/O.
func NormalizeReleasedGo(snapshot rie.RepositorySnapshot, syntax golang.GoLanguageInventory, identities packageidentity.GoPackageIdentityInventory, semantics semantic.GoSemanticInventory) (Input, error) {
	if snapshot.ArtifactVersion() != rie.RepositorySnapshotArtifactVersion || syntax.ArtifactVersion() != golang.ArtifactVersion || identities.ArtifactVersion() != packageidentity.ArtifactVersion || semantics.ArtifactVersion() != semantic.ArtifactVersion {
		return Input{}, fmt.Errorf("%w: released artifact version mismatch", ErrInvalidInput)
	}
	nodes := make(map[string]NodeIdentity)
	packages := make(map[string]golang.GoPackage)
	files := make(map[string]golang.GoFile)
	declarations := make(map[string]semantic.SemanticDeclaration)
	modules := identities.Modules()

	for _, module := range modules {
		identity := NodeIdentity{Kind: NodeModule, Language: "Go", QualifiedName: module.ModulePath, Path: module.Root, Resolution: ResolvedLocal}
		nodes[logicalNodeKey(identity)] = identity
	}
	for _, item := range syntax.Packages() {
		packages[item.ID] = item
		identity := NodeIdentity{Kind: NodePackage, Language: "Go", QualifiedName: item.ID, Path: item.Directory, Resolution: ResolvedLocal}
		nodes[logicalNodeKey(identity)] = identity
	}
	for _, item := range syntax.Files() {
		files[item.ID] = item
		identity := NodeIdentity{Kind: NodeFile, Language: "Go", QualifiedName: item.ID, Path: item.Path, Resolution: ResolvedLocal}
		nodes[logicalNodeKey(identity)] = identity
	}
	for _, item := range semantics.Declarations() {
		declarations[item.ID] = item
	}

	proofs := make(map[string]packageidentity.PackageIdentityProof)
	for _, proof := range identities.Proofs() {
		proofs[proof.ID] = proof
	}
	edges := make([]RawEdge, 0)
	packageEdges := make([]RawEdge, 0)
	for _, binding := range semantics.ImportBindings() {
		sourceFile, ok := files[binding.FileID]
		if !ok {
			continue
		}
		sourcePackage, ok := packages[sourceFile.PackageID]
		if !ok {
			continue
		}
		from := NodeIdentity{Kind: NodePackage, Language: "Go", QualifiedName: sourcePackage.ID, Path: sourcePackage.Directory, Resolution: ResolvedLocal}
		resolution := semanticResolution(binding.Status)
		to, targetLocal := localPackageIdentity(binding.TargetPackageID, packages)
		proof := proofs[binding.PackageIdentityProofID]
		if hasProofKind(proof, packageidentity.ProofStandardLibrary) {
			resolution = Standard
		}
		if !targetLocal {
			if resolution == ResolvedLocal {
				resolution = External
			}
			kind := NodeBoundary
			qualified := binding.ImportPath
			if qualified == "" {
				qualified = binding.TargetPackageID
			}
			to = NodeIdentity{Kind: kind, Language: "Go", QualifiedName: qualified, Resolution: resolution}
		}
		nodes[logicalNodeKey(to)] = to
		evidence := Evidence{Artifact: semantic.ArtifactName, Version: semantic.ArtifactVersion, SourceID: binding.ID, File: sourceFile.Path, Line: binding.Location.Start.Line, Column: binding.Location.Start.Column, Rule: "go-import-binding", Value: binding.ImportPath}
		edge := RawEdge{Graph: GraphPackage, Kind: EdgeImports, From: from, To: to, Resolution: resolution, Evidence: evidence}
		edges = append(edges, edge)
		packageEdges = append(packageEdges, edge)
	}

	for _, reference := range semantics.References() {
		if reference.TargetDeclarationID == "" {
			continue
		}
		target, ok := declarations[reference.TargetDeclarationID]
		if !ok {
			continue
		}
		fromFile, fromOK := files[reference.FileID]
		targetFile, targetOK := files[target.FileID]
		if !fromOK || !targetOK || fromFile.ID == targetFile.ID {
			continue
		}
		from := NodeIdentity{Kind: NodeFile, Language: "Go", QualifiedName: fromFile.ID, Path: fromFile.Path, Resolution: ResolvedLocal}
		to := NodeIdentity{Kind: NodeFile, Language: "Go", QualifiedName: targetFile.ID, Path: targetFile.Path, Resolution: ResolvedLocal}
		edges = append(edges, RawEdge{Graph: GraphFile, Kind: EdgeReferences, From: from, To: to, Resolution: semanticResolution(reference.Status), Evidence: Evidence{Artifact: semantic.ArtifactName, Version: semantic.ArtifactVersion, SourceID: reference.ID, File: reference.Location.File, Line: reference.Location.Start.Line, Column: reference.Location.Start.Column, Rule: "go-semantic-reference", Value: reference.Name}})
	}

	packageModule := make(map[string]NodeIdentity)
	for packageID, item := range packages {
		if module, ok := owningModule(item.Directory, modules); ok {
			packageModule[packageID] = NodeIdentity{Kind: NodeModule, Language: "Go", QualifiedName: module.ModulePath, Path: module.Root, Resolution: ResolvedLocal}
		}
	}
	for _, item := range packageEdges {
		fromModule, fromOK := packageModule[item.From.QualifiedName]
		toModule, toOK := packageModule[item.To.QualifiedName]
		if !fromOK || !toOK || logicalNodeKey(fromModule) == logicalNodeKey(toModule) {
			continue
		}
		edges = append(edges, RawEdge{Graph: GraphModule, Kind: EdgeImports, From: fromModule, To: toModule, Resolution: item.Resolution, Evidence: item.Evidence})
	}

	ordered := make([]NodeIdentity, 0, len(nodes))
	for _, item := range nodes {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool { return logicalNodeKey(ordered[i]) < logicalNodeKey(ordered[j]) })
	return Input{Nodes: ordered, Edges: edges}, nil
}

func semanticResolution(status semantic.ResolutionStatus) Resolution {
	switch status {
	case semantic.ResolutionResolved:
		return ResolvedLocal
	case semantic.ResolutionExternal:
		return External
	case semantic.ResolutionAmbiguous:
		return Ambiguous
	case semantic.ResolutionPartial:
		return Stale
	default:
		return Unresolved
	}
}

func localPackageIdentity(id string, packages map[string]golang.GoPackage) (NodeIdentity, bool) {
	item, ok := packages[id]
	if !ok {
		return NodeIdentity{}, false
	}
	return NodeIdentity{Kind: NodePackage, Language: "Go", QualifiedName: item.ID, Path: item.Directory, Resolution: ResolvedLocal}, true
}

func hasProofKind(proof packageidentity.PackageIdentityProof, kind packageidentity.ProofKind) bool {
	for _, item := range proof.Kinds {
		if item == kind {
			return true
		}
	}
	return false
}

func owningModule(directory string, modules []packageidentity.ModuleIdentity) (packageidentity.ModuleIdentity, bool) {
	directory = normalizePath(directory)
	ordered := append([]packageidentity.ModuleIdentity(nil), modules...)
	sort.Slice(ordered, func(i, j int) bool { return len(normalizePath(ordered[i].Root)) > len(normalizePath(ordered[j].Root)) })
	for _, module := range ordered {
		root := normalizePath(module.Root)
		if root == "" || directory == root || strings.HasPrefix(directory, root+"/") {
			return module, true
		}
	}
	return packageidentity.ModuleIdentity{}, false
}
