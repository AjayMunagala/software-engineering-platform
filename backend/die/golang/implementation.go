package golang

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	syntax "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	identity "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/semantic"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

type engine struct {
	config Config
	core   die.Core
}

func (*engine) Name() string    { return "go-dependency" }
func (*engine) Version() string { return Version }
func (*engine) Description() string {
	return "Structural Go dependencies from released immutable artifacts"
}

// budget counts raw records before indexes and graph contributions before append.
type budget struct{ records, evidence, maxRecords, maxEvidence uint64 }

func (b *budget) add(n uint64, e bool) error {
	value, limit := &b.records, b.maxRecords
	if e {
		value, limit = &b.evidence, b.maxEvidence
	}
	if n > limit-*value {
		return fail(die.ErrorLimitExceeded, "input_budget_exceeded")
	}
	*value += n
	return nil
}

type facts struct {
	files         []syntax.GoFile
	packages      []syntax.GoPackage
	modules       []identity.ModuleIdentity
	contexts      []identity.ResolutionContext
	proofs        []identity.PackageIdentityProof
	semanticFiles []semantic.SemanticFile
	declarations  []semantic.SemanticDeclaration
	references    []semantic.SemanticReference
	imports       []semantic.ImportBinding
	diagnostics   []lie.Diagnostic
	omitted       bool
}

func extract(ctx context.Context, p InputParams, b *budget) (facts, error) {
	var f facts
	// Each clone is isolated by a context check. The released accessor owns its allocation.
	steps := []func() int{
		func() int { f.files = p.LanguageInventory.Files(); return len(f.files) }, func() int { f.packages = p.LanguageInventory.Packages(); return len(f.packages) },
		func() int { f.modules = p.PackageIdentity.Modules(); return len(f.modules) }, func() int { f.contexts = p.PackageIdentity.Contexts(); return len(f.contexts) }, func() int { f.proofs = p.PackageIdentity.Proofs(); return len(f.proofs) },
		func() int { f.semanticFiles = p.SemanticInventory.Files(); return len(f.semanticFiles) }, func() int { f.declarations = p.SemanticInventory.Declarations(); return len(f.declarations) }, func() int { f.references = p.SemanticInventory.References(); return len(f.references) }, func() int { f.imports = p.SemanticInventory.ImportBindings(); return len(f.imports) },
		func() int {
			d := p.LanguageInventory.Diagnostics()
			f.diagnostics = append(f.diagnostics, d...)
			return len(d)
		}, func() int {
			d := p.PackageIdentity.Diagnostics()
			f.diagnostics = append(f.diagnostics, d...)
			return len(d)
		}, func() int {
			d := p.SemanticInventory.Diagnostics()
			f.diagnostics = append(f.diagnostics, d...)
			return len(d)
		},
	}
	for _, step := range steps {
		if err := check(ctx); err != nil {
			return f, err
		}
		if err := b.add(uint64(step()), false); err != nil {
			return f, err
		}
	}
	for _, file := range f.files {
		if err := check(ctx); err != nil {
			return f, err
		}
		if err := b.add(uint64(len(file.Imports)), false); err != nil {
			return f, err
		}
	}
	for _, p := range f.packages {
		if err := check(ctx); err != nil {
			return f, err
		}
		if err := b.add(uint64(len(p.FileIDs)), false); err != nil {
			return f, err
		}
	}
	for _, p := range f.proofs {
		if err := check(ctx); err != nil {
			return f, err
		}
		if err := b.add(uint64(len(p.CandidatePackageIDs)+len(p.Kinds)), false); err != nil {
			return f, err
		}
	}
	for _, c := range f.contexts {
		if err := check(ctx); err != nil {
			return f, err
		}
		if err := b.add(uint64(len(c.MainModuleIDs)+len(c.ManifestFiles)), false); err != nil {
			return f, err
		}
	}
	for _, r := range f.references {
		if err := check(ctx); err != nil {
			return f, err
		}
		if err := b.add(uint64(len(r.CandidateDeclarationIDs)), false); err != nil {
			return f, err
		}
	}
	for _, ev := range evidenceLists(f) {
		if err := check(ctx); err != nil {
			return f, err
		}
		if err := b.add(uint64(len(ev)), true); err != nil {
			return f, err
		}
	}
	s := p.SemanticInventory.Statistics()
	f.omitted = s.OmittedRelationships > 0 || s.OmittedDiagnostics > 0 || p.LanguageInventory.Statistics().OmittedDiagnostics > 0 || p.PackageIdentity.Statistics().OmittedDiagnostics > 0
	return f, nil
}
func evidenceLists(f facts) [][]identity.PackageIdentityEvidence {
	out := make([][]identity.PackageIdentityEvidence, 0, len(f.modules)+len(f.contexts)+len(f.proofs))
	for _, v := range f.modules {
		out = append(out, v.Evidence)
	}
	for _, v := range f.contexts {
		out = append(out, v.Evidence)
	}
	for _, v := range f.proofs {
		out = append(out, v.Evidence)
	}
	return out
}

func (e *engine) Analyze(ctx context.Context, input Inputs) (die.DependencyInventory, error) {
	if err := check(ctx); err != nil {
		return die.DependencyInventory{}, err
	}
	if !input.valid {
		return die.DependencyInventory{}, fail(die.ErrorInvalidInput, "invalid_inputs")
	}
	b := &budget{maxRecords: e.config.records, maxEvidence: e.config.evidence}
	f, err := extract(ctx, input.p, b)
	if err != nil {
		return die.DependencyInventory{}, err
	}
	graph, err := translate(ctx, input.p.RepositorySnapshot, f, b, e.config.core.MaxNodes())
	if err != nil {
		return die.DependencyInventory{}, safeError(err)
	}
	if err = check(ctx); err != nil {
		return die.DependencyInventory{}, err
	}
	out, err := e.core.Normalize(ctx, graph)
	if err != nil {
		return die.DependencyInventory{}, safeError(err)
	}
	if err = check(ctx); err != nil {
		return die.DependencyInventory{}, err
	}
	return out, nil
}

type index struct {
	omitted        bool
	files          map[string]syntax.GoFile
	packages       map[string]syntax.GoPackage
	sf             map[string]semantic.SemanticFile
	modules        map[string]identity.ModuleIdentity
	contexts       map[string]identity.ResolutionContext
	proofs         map[string]identity.PackageIdentityProof
	declarations   map[string]semantic.SemanticDeclaration
	bindings       map[importKey]semantic.ImportBinding
	byImport       map[string][]identity.PackageIdentityProof
	staleLocations map[string]bool
}
type importKey struct {
	file, imp, alias, loc string
	offset, line, column  int
}

func ikey(file, imp, alias string, l lie.SourceRange) importKey {
	return importKey{file, imp, alias, l.File, l.Start.Offset, l.Start.Line, l.Start.Column}
}
func locationKey(l lie.SourceRange) string {
	return fmt.Sprintf("%s:%d:%d:%d", l.File, l.Start.Offset, l.Start.Line, l.Start.Column)
}

func validate(ctx context.Context, snapshot rie.RepositorySnapshot, f facts, b *budget) (index, error) {
	x := index{files: map[string]syntax.GoFile{}, packages: map[string]syntax.GoPackage{}, sf: map[string]semantic.SemanticFile{}, modules: map[string]identity.ModuleIdentity{}, contexts: map[string]identity.ResolutionContext{}, proofs: map[string]identity.PackageIdentityProof{}, declarations: map[string]semantic.SemanticDeclaration{}, bindings: map[importKey]semantic.ImportBinding{}, byImport: map[string][]identity.PackageIdentityProof{}, staleLocations: map[string]bool{}}
	x.omitted = f.omitted
	bad := func() (index, error) { return x, fail(die.ErrorIntegrity, "inconsistent_artifacts") }
	paths := map[string]bool{}
	err := snapshot.ForEachEntry(func(entry rie.RepositoryEntry) error {
		if err := check(ctx); err != nil {
			return err
		}
		if err := b.add(1, false); err != nil {
			return err
		}
		p, ok := relative(entry.Path)
		if !ok {
			return fail(die.ErrorIntegrity, "invalid_snapshot_path")
		}
		if !entry.IsDir {
			paths[p] = true
		}
		return nil
	})
	if err != nil {
		return x, err
	}
	for _, p := range f.packages {
		if err := check(ctx); err != nil {
			return x, err
		}
		dir, ok := relative(p.Directory)
		if !ok || !safe(p.ID) || x.packages[p.ID].ID != "" {
			return bad()
		}
		p.Directory = dir
		x.packages[p.ID] = p
	}
	usedPaths := map[string]bool{}
	syntaxImports := map[importKey]syntax.GoImport{}
	for _, file := range f.files {
		if err := check(ctx); err != nil {
			return x, err
		}
		p, ok := relative(file.Path)
		if !ok || !safe(file.ID) || usedPaths[p] || !paths[p] || x.files[file.ID].ID != "" || file.Status < syntax.FileStatusParsed || file.Status > syntax.FileStatusSkipped {
			return bad()
		}
		usedPaths[p] = true
		file.Path = p
		if file.PackageID != "" {
			if _, ok := x.packages[file.PackageID]; !ok {
				return bad()
			}
		} else if file.Status == syntax.FileStatusParsed {
			return bad()
		}
		if file.ContentDigest != "" && !digest(file.ContentDigest) {
			return bad()
		}
		for _, im := range file.Imports {
			if !safe(im.Path) || im.AliasKind < syntax.ImportAliasDefault || im.AliasKind > syntax.ImportAliasDot || !location(im.Location, p) {
				return bad()
			}
			k := ikey(file.ID, im.Path, im.AliasKind.String(), im.Location)
			if _, exists := syntaxImports[k]; exists {
				return bad()
			}
			syntaxImports[k] = im
		}
		x.files[file.ID] = file
	}
	membership := map[string]bool{}
	for _, p := range x.packages {
		if err := check(ctx); err != nil {
			return x, err
		}
		for _, id := range p.FileIDs {
			file, ok := x.files[id]
			if !ok || file.PackageID != p.ID || membership[id] {
				return bad()
			}
			membership[id] = true
		}
	}
	for _, file := range x.files {
		if file.PackageID != "" && !membership[file.ID] {
			return bad()
		}
	}
	roots := map[string]bool{}
	for _, m := range f.modules {
		if err := check(ctx); err != nil {
			return x, err
		}
		root, ok := relative(m.Root)
		if !ok || !safe(m.ID) || !safe(m.ModulePath) || x.modules[m.ID].ID != "" || roots[root] {
			return bad()
		}
		m.Root = root
		roots[root] = true
		x.modules[m.ID] = m
	}
	for _, c := range f.contexts {
		if err := check(ctx); err != nil {
			return x, err
		}
		if !safe(c.ID) || c.Kind < identity.ContextSingleModule || c.Kind > identity.ContextUnmanaged || x.contexts[c.ID].ID != "" {
			return bad()
		}
		if _, ok := relative(c.Root); !ok {
			return bad()
		}
		for _, id := range c.MainModuleIDs {
			if _, ok := x.modules[id]; !ok {
				return bad()
			}
		}
		x.contexts[c.ID] = c
	}
	for _, list := range evidenceLists(f) {
		for _, ev := range list {
			if err := check(ctx); err != nil {
				return x, err
			}
			if ev.File != "" {
				p, ok := relative(ev.File)
				if !ok || !paths[p] || !digest(ev.ContentDigest) {
					return bad()
				}
			}
		}
	}
	for _, p := range f.proofs {
		if err := check(ctx); err != nil {
			return x, err
		}
		if !safe(p.ID) || !safe(p.ImportPath) || p.Status < identity.ProofResolved || p.Status > identity.ProofStale || x.proofs[p.ID].ID != "" {
			return bad()
		}
		if _, ok := x.packages[p.ImportingPackageID]; !ok {
			return bad()
		}
		if _, ok := x.contexts[p.ResolutionContextID]; !ok {
			return bad()
		}
		for _, k := range p.Kinds {
			if k < identity.ProofSameModule || k > identity.ProofStandardLibrary {
				return bad()
			}
		}
		if p.TargetPackageID != "" {
			target, ok := x.packages[p.TargetPackageID]
			if !ok {
				return bad()
			}
			dir, ok := relative(p.TargetDirectory)
			if !ok || dir != target.Directory {
				return bad()
			}
		}
		if p.Status == identity.ProofResolved && p.TargetPackageID == "" {
			return bad()
		}
		for _, id := range p.CandidatePackageIDs {
			if _, ok := x.packages[id]; !ok {
				return bad()
			}
		}
		x.proofs[p.ID] = p
		k := p.ImportingPackageID + "\x00" + p.ImportPath
		x.byImport[k] = append(x.byImport[k], p)
	}
	for k := range x.byImport {
		sort.Slice(x.byImport[k], func(i, j int) bool { return x.byImport[k][i].ID < x.byImport[k][j].ID })
	}
	for _, sf := range f.semanticFiles {
		if err := check(ctx); err != nil {
			return x, err
		}
		file, ok := x.files[sf.FileID]
		if !ok || x.sf[sf.FileID].FileID != "" || sf.PackageID != file.PackageID || sf.Status < semantic.SemanticFileResolved || sf.Status > semantic.SemanticFileSkipped {
			return bad()
		}
		if sf.ContentDigest != "" && !digest(sf.ContentDigest) {
			return bad()
		}
		if sf.Status == semantic.SemanticFileResolved || sf.Status == semantic.SemanticFilePartial {
			if !digest(file.ContentDigest) || sf.ContentDigest != file.ContentDigest {
				return bad()
			}
		}
		x.sf[sf.FileID] = sf
	}
	for _, d := range f.declarations {
		if err := check(ctx); err != nil {
			return x, err
		}
		file, ok := x.files[d.FileID]
		if !ok || !safe(d.ID) || x.declarations[d.ID].ID != "" || file.PackageID != d.PackageID || !location(d.Location, file.Path) || !status(d.Status) {
			return bad()
		}
		x.declarations[d.ID] = d
	}
	bindingIDs := map[string]bool{}
	for _, im := range f.imports {
		if err := check(ctx); err != nil {
			return x, err
		}
		file, ok := x.files[im.FileID]
		key := ikey(im.FileID, im.ImportPath, im.AliasKind, im.Location)
		source, sourceOK := syntaxImports[key]
		if !ok || !safe(im.ID) || bindingIDs[im.ID] || !sourceOK || source.Location != im.Location || x.bindings[key].ID != "" || !status(im.Status) {
			return bad()
		}
		if source.AliasKind != syntax.ImportAliasDefault && im.LocalName != source.Alias {
			return bad()
		}
		bindingIDs[im.ID] = true
		if im.PackageIdentityProofID != "" {
			p, ok := x.proofs[im.PackageIdentityProofID]
			if !ok {
				if !f.omitted {
					return bad()
				}
			} else if p.ImportingPackageID != file.PackageID || p.ImportPath != im.ImportPath || (im.TargetPackageID != "" && im.TargetPackageID != p.TargetPackageID) {
				return bad()
			}
		}
		if im.TargetPackageID != "" {
			if _, ok := x.packages[im.TargetPackageID]; !ok {
				return bad()
			}
		}
		x.bindings[key] = im
	}
	refs := map[string]bool{}
	for _, r := range f.references {
		if err := check(ctx); err != nil {
			return x, err
		}
		file, ok := x.files[r.FileID]
		if !ok || !safe(r.ID) || refs[r.ID] || !status(r.Status) || r.PackageID != file.PackageID || !location(r.Location, file.Path) {
			return bad()
		}
		refs[r.ID] = true
		ids := append([]string{}, r.CandidateDeclarationIDs...)
		if r.TargetDeclarationID != "" {
			ids = append(ids, r.TargetDeclarationID)
		}
		for _, id := range ids {
			if _, ok := x.declarations[id]; !ok && !f.omitted {
				return bad()
			}
		}
	}
	for _, d := range f.diagnostics {
		if d.Code == "semantic_package_proof_stale" && d.Location != nil {
			x.staleLocations[locationKey(*d.Location)] = true
		}
	}
	return x, nil
}

type builder struct {
	err      error
	g        die.GraphInput
	nodes    map[die.NodeIdentity]bool
	budget   *budget
	maxNodes uint64
}

func (b *builder) node(n die.NodeCandidate) error {
	if b.nodes[n.Identity] {
		return nil
	}
	if uint64(len(b.nodes)) >= b.maxNodes {
		return fail(die.ErrorLimitExceeded, "max_nodes_exceeded")
	}
	if err := b.budget.add(uint64(len(n.Evidence)), true); err != nil {
		return err
	}
	b.nodes[n.Identity] = true
	b.g.Nodes = append(b.g.Nodes, n)
	return nil
}
func (b *builder) edge(v die.DependencyCandidate) error {
	if err := b.budget.add(1, false); err != nil {
		return err
	}
	if err := b.budget.add(uint64(len(v.Evidence)), true); err != nil {
		return err
	}
	b.g.Dependencies = append(b.g.Dependencies, v)
	return nil
}
func (b *builder) contain(k die.ContainmentKind, p, c die.NodeIdentity, ev die.DependencyEvidence) error {
	if err := b.budget.add(1, false); err != nil {
		return err
	}
	if err := b.budget.add(1, true); err != nil {
		return err
	}
	b.g.Containment = append(b.g.Containment, die.ContainmentCandidate{Kind: k, Parent: p, Child: c, Evidence: []die.DependencyEvidence{ev}})
	return nil
}
func (b *builder) diag(code string, g die.GraphKind, file, id string) {
	if b.err != nil {
		return
	}
	if b.err = b.budget.add(1, false); b.err != nil {
		return
	}
	b.g.Diagnostics = append(b.g.Diagnostics, die.DiagnosticCandidate{Code: code, Graph: g, File: file, StableID: id, Message: code})
}
func src(name, id string) die.SourceIdentity {
	return die.SourceIdentity{ArtifactName: name, ArtifactVersion: "1.0.0", SourceID: id}
}
func ev(name, id, file, rule string, l lie.SourceRange) die.DependencyEvidence {
	return die.DependencyEvidence{Source: src(name, id), File: file, StartLine: l.Start.Line, StartColumn: l.Start.Column, Rule: rule}
}
func nid(kind die.NodeKind, name, p string) die.NodeIdentity {
	return die.NodeIdentity{Kind: kind, Language: "Go", QualifiedName: name, RepositoryPath: p, Resolution: die.ResolvedLocal}
}

func translate(ctx context.Context, snapshot rie.RepositorySnapshot, f facts, budget *budget, maxNodes uint64) (die.GraphInput, error) {
	x, err := validate(ctx, snapshot, f, budget)
	if err != nil {
		return die.GraphInput{}, err
	}
	b := builder{nodes: map[die.NodeIdentity]bool{}, budget: budget, maxNodes: maxNodes}
	for _, name := range []string{rie.RepositorySnapshotArtifactName, syntax.ArtifactName, identity.ArtifactName, semantic.ArtifactName} {
		b.g.SourceArtifacts = append(b.g.SourceArtifacts, die.ArtifactReference{Name: name, Version: "1.0.0"})
	}
	mods := map[string]die.NodeIdentity{}
	pkgs := map[string]die.NodeIdentity{}
	files := map[string]die.NodeIdentity{}
	owners := map[string]die.NodeIdentity{}
	rootOwners := map[string]string{}
	for _, m := range x.modules {
		if err = check(ctx); err != nil {
			return die.GraphInput{}, err
		}
		rootOwners[m.Root] = m.ID
		n := nid(die.NodeModule, m.ModulePath, m.Root)
		mods[m.ID] = n
		if err = b.node(die.NodeCandidate{Identity: n, Name: m.ModulePath, SourceIdentity: src(identity.ArtifactName, m.ID), Evidence: []die.DependencyEvidence{ev(identity.ArtifactName, m.ID, "", "go-module/v1", lie.SourceRange{})}}); err != nil {
			return die.GraphInput{}, err
		}
	}
	for _, p := range x.packages {
		if err = check(ctx); err != nil {
			return die.GraphInput{}, err
		}
		n := nid(die.NodePackage, p.ID, p.Directory)
		pkgs[p.ID] = n
		if err = b.node(die.NodeCandidate{Identity: n, Name: p.Name, SourceIdentity: src(syntax.ArtifactName, p.ID)}); err != nil {
			return die.GraphInput{}, err
		}
		owner := ""
		for dir := p.Directory; ; {
			if id, ok := rootOwners[dir]; ok {
				owner = id
				break
			}
			if dir == "" {
				break
			}
			dir = path.Dir(dir)
			if dir == "." {
				dir = ""
			}
		}
		if owner != "" {
			owners[p.ID] = mods[owner]
			if err = b.contain(die.ContainmentModulePackage, mods[owner], n, ev(syntax.ArtifactName, p.ID, "", "go-module-ownership/v1", lie.SourceRange{})); err != nil {
				return die.GraphInput{}, err
			}
		}
	}
	for _, file := range x.files {
		if err = check(ctx); err != nil {
			return die.GraphInput{}, err
		}
		n := nid(die.NodeFile, file.ID, file.Path)
		files[file.ID] = n
		if err = b.node(die.NodeCandidate{Identity: n, Name: path.Base(file.Path), SourceIdentity: src(syntax.ArtifactName, file.ID)}); err != nil {
			return die.GraphInput{}, err
		}
		if p, ok := pkgs[file.PackageID]; ok {
			if err = b.contain(die.ContainmentPackageFile, p, n, ev(syntax.ArtifactName, file.ID, file.Path, "go-package-membership/v1", lie.SourceRange{})); err != nil {
				return die.GraphInput{}, err
			}
		}
		sf, ok := x.sf[file.ID]
		if !ok || sf.Status != semantic.SemanticFileResolved || file.Status != syntax.FileStatusParsed {
			b.diag("go_file_partial", die.GraphFile, file.Path, file.ID)
		}
	}
	// Traverse source imports exactly once, retaining missing binding boundaries.
	for _, file := range x.files {
		for _, im := range file.Imports {
			if err = check(ctx); err != nil {
				return die.GraphInput{}, err
			}
			binding, exists := x.bindings[ikey(file.ID, im.Path, im.AliasKind.String(), im.Location)]
			proofs := x.byImport[file.PackageID+"\x00"+im.Path]
			resolution, target, contextID, err := resolveContext(ctx, file, im, binding, exists, proofs, x)
			if err != nil {
				return die.GraphInput{}, err
			}
			targetNode, local := pkgs[target]
			if resolution != die.ResolvedLocal || !local {
				kind := die.NodeUnresolved
				if resolution == die.StandardLibrary {
					kind = die.NodeStandardLibrary
				}
				if resolution == die.External {
					kind = die.NodeExternalPackage
				}
				name := boundaryName(file.PackageID, contextID, im.Path)
				targetNode = die.NodeIdentity{Kind: kind, Language: "Go", QualifiedName: name, Resolution: resolution}
				if err = b.node(die.NodeCandidate{Identity: targetNode, Name: im.Path, SourceIdentity: src(syntax.ArtifactName, file.PackageID)}); err != nil {
					return die.GraphInput{}, err
				}
			}
			count := uint64(1 + len(proofs))
			if exists {
				count++
			}
			if count > budget.maxEvidence-budget.evidence {
				return die.GraphInput{}, fail(die.ErrorLimitExceeded, "input_budget_exceeded")
			}
			evidence := make([]die.DependencyEvidence, 0, int(count))
			evidence = append(evidence, ev(syntax.ArtifactName, file.ID, file.Path, "go-import/v1", im.Location))
			if exists {
				evidence = append(evidence, ev(semantic.ArtifactName, binding.ID, file.Path, "go-import-binding/v1", binding.Location))
			}
			for _, p := range proofs {
				if err = check(ctx); err != nil {
					return die.GraphInput{}, err
				}
				evidence = append(evidence, ev(identity.ArtifactName, p.ID, file.Path, "go-package-proof/v1", im.Location))
			}
			if !exists {
				b.diag("go_missing_import_binding", die.GraphPackage, file.Path, locationKey(im.Location))
			}
			if resolution != die.ResolvedLocal {
				b.diag("go_import_"+string(resolution), die.GraphPackage, file.Path, locationKey(im.Location))
			}
			if source, ok := pkgs[file.PackageID]; ok {
				if err = b.edge(die.DependencyCandidate{Graph: die.GraphPackage, Kind: die.DependencyImports, From: source, To: targetNode, Resolution: resolution, Evidence: evidence}); err != nil {
					return die.GraphInput{}, err
				}
			}
			if err = b.edge(die.DependencyCandidate{Graph: die.GraphFile, Kind: die.DependencyImports, From: files[file.ID], To: targetNode, Resolution: resolution, Evidence: evidence}); err != nil {
				return die.GraphInput{}, err
			}
			from, fromOK := owners[file.PackageID]
			to, toOK := owners[target]
			if resolution == die.ResolvedLocal && fromOK && toOK && from != to {
				if err = b.edge(die.DependencyCandidate{Graph: die.GraphModule, Kind: die.DependencyImports, From: from, To: to, Resolution: resolution, Evidence: evidence}); err != nil {
					return die.GraphInput{}, err
				}
			}
		}
	}
	for _, r := range f.references {
		if err = check(ctx); err != nil {
			return die.GraphInput{}, err
		}
		d, ok := x.declarations[r.TargetDeclarationID]
		if r.Status != semantic.ResolutionResolved || !ok || d.Status != semantic.ResolutionResolved || !verified(x.sf[r.FileID]) || !verified(x.sf[d.FileID]) {
			b.diag("go_reference_unresolved", die.GraphFile, x.files[r.FileID].Path, r.ID)
			continue
		}
		if r.FileID == d.FileID {
			continue
		}
		if err = b.edge(die.DependencyCandidate{Graph: die.GraphFile, Kind: die.DependencyReferences, From: files[r.FileID], To: files[d.FileID], Resolution: die.ResolvedLocal, Evidence: []die.DependencyEvidence{ev(semantic.ArtifactName, r.ID, x.files[r.FileID].Path, "go-reference/v1", r.Location)}}); err != nil {
			return die.GraphInput{}, err
		}
	}
	if f.omitted {
		b.diag("go_upstream_omissions", "", "", "")
	}
	if len(f.diagnostics) > 0 {
		b.diag("go_upstream_diagnostics", "", "", "")
	}
	if b.err != nil {
		return die.GraphInput{}, b.err
	}
	return b.g, check(ctx)
}

func resolveContext(ctx context.Context, file syntax.GoFile, im syntax.GoImport, b semantic.ImportBinding, exists bool, proofs []identity.PackageIdentityProof, x index) (die.ResolutionState, string, string, error) {
	contextID := ""
	for _, p := range proofs {
		if err := check(ctx); err != nil {
			return "", "", "", err
		}
		if contextID == "" {
			contextID = p.ResolutionContextID
		} else if contextID != p.ResolutionContextID {
			contextID = ""
			break
		}
	}
	sf := x.sf[file.ID]
	if sf.Status == semantic.SemanticFileStale || x.staleLocations[locationKey(im.Location)] {
		return die.Stale, "", contextID, nil
	}
	for _, p := range proofs {
		if p.Status == identity.ProofStale {
			return die.Stale, "", contextID, nil
		}
	}
	if !exists {
		return die.Unresolved, "", contextID, nil
	}
	if b.Status == semantic.ResolutionResolved {
		p, ok := x.proofs[b.PackageIdentityProofID]
		if !ok && x.omitted {
			return die.Unresolved, "", contextID, nil
		}
		if !ok || p.Status != identity.ProofResolved || p.TargetPackageID != b.TargetPackageID || b.TargetPackageID == "" || !verified(sf) {
			return "", "", "", fail(die.ErrorIntegrity, "unproved_local_import")
		}
		for _, other := range proofs {
			if err := check(ctx); err != nil {
				return "", "", "", err
			}
			if other.Status != identity.ProofResolved || other.TargetPackageID != p.TargetPackageID {
				return "", "", "", fail(die.ErrorIntegrity, "conflicting_import_proofs")
			}
		}
		target := x.packages[b.TargetPackageID]
		for _, id := range target.FileIDs {
			if err := check(ctx); err != nil {
				return "", "", "", err
			}
			if x.sf[id].Status == semantic.SemanticFileStale {
				return die.Stale, "", contextID, nil
			}
			if !verified(x.sf[id]) {
				return die.Unresolved, "", contextID, nil
			}
		}
		return die.ResolvedLocal, b.TargetPackageID, contextID, nil
	}
	if b.TargetPackageID != "" {
		return "", "", "", fail(die.ErrorIntegrity, "unexpected_import_target")
	}
	if b.Status == semantic.ResolutionAmbiguous {
		return die.Ambiguous, "", contextID, nil
	}
	if b.Status == semantic.ResolutionExternal {
		if len(proofs) == 0 {
			if x.omitted {
				return die.Unresolved, "", contextID, nil
			}
			return "", "", "", fail(die.ErrorIntegrity, "unproved_external_import")
		}
		stdlib := true
		for _, p := range proofs {
			if err := check(ctx); err != nil {
				return "", "", "", err
			}
			if p.Status != identity.ProofExternal {
				return "", "", "", fail(die.ErrorIntegrity, "conflicting_import_proofs")
			}
			provenStandard := false
			for _, k := range p.Kinds {
				if k == identity.ProofStandardLibrary {
					provenStandard = true
				}
			}
			stdlib = stdlib && provenStandard
		}
		if stdlib {
			return die.StandardLibrary, "", contextID, nil
		}
		return die.External, "", contextID, nil
	}
	return die.Unresolved, "", contextID, nil
}
func verified(f semantic.SemanticFile) bool {
	return (f.Status == semantic.SemanticFileResolved || f.Status == semantic.SemanticFilePartial) && digest(f.ContentDigest)
}
func status(s semantic.ResolutionStatus) bool {
	return s >= semantic.ResolutionResolved && s <= semantic.ResolutionPartial
}
func safe(s string) bool {
	if s == "" || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func relative(s string) (string, bool) {
	if s == "" || s == "." {
		return "", true
	}
	if !safe(s) {
		return "", false
	}
	s = strings.ReplaceAll(s, "\\", "/")
	if strings.HasPrefix(s, "/") || strings.Contains(s, ":") {
		return "", false
	}
	for _, segment := range strings.Split(s, "/") {
		if segment == ".." {
			return "", false
		}
	}
	return path.Clean(s), true
}
func digest(s string) bool {
	s = strings.TrimPrefix(s, "sha256:")
	if len(s) != 64 || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func location(l lie.SourceRange, file string) bool {
	p, ok := relative(l.File)
	return ok && p == file && l.Start.Line > 0 && l.Start.Column > 0 && l.Start.Offset >= 0
}
func boundaryName(pkg, ctx, imp string) string {
	h := sha256.New()
	var prefix [8]byte
	for _, s := range []string{BoundaryNameScheme, pkg, ctx, imp} {
		binary.BigEndian.PutUint64(prefix[:], uint64(len(s)))
		h.Write(prefix[:])
		h.Write([]byte(s))
	}
	return BoundaryNameScheme + ":sha256:" + hex.EncodeToString(h.Sum(nil))
}
