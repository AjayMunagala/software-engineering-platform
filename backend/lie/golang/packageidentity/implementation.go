package packageidentity

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

type engine struct{ config Config }

func (*engine) Name() string         { return "go-package-identity" }
func (*engine) Version() string      { return engineVersion }
func (*engine) ArtifactName() string { return ArtifactName }
func (*engine) Description() string {
	return "Deterministically proves local Go package identities from snapshot-authorized manifests"
}

func (engine *engine) Analyze(ctx context.Context, input Input) (GoPackageIdentityInventory, error) {
	if ctx == nil {
		return GoPackageIdentityInventory{}, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return GoPackageIdentityInventory{}, err
	}
	if err := validateInput(input); err != nil {
		return GoPackageIdentityInventory{}, err
	}
	manifestPaths := collectManifestPaths(input.Snapshot.Entries())
	outcomes := make([]manifestOutcome, len(manifestPaths))
	if len(manifestPaths) > 0 {
		reader, err := newManifestReader(input.Snapshot.RootPath(), manifestPaths)
		if err != nil {
			return GoPackageIdentityInventory{}, err
		}
		defer reader.close()
		workerCount := min(engine.config.MaxWorkers, len(manifestPaths))
		jobs := make(chan int)
		var workers sync.WaitGroup
		workers.Add(workerCount)
		for range workerCount {
			go func() {
				defer workers.Done()
				for index := range jobs {
					if ctx.Err() != nil {
						return
					}
					outcomes[index] = engine.parseManifest(ctx, reader, manifestPaths[index])
				}
			}()
		}
		for index := range manifestPaths {
			select {
			case jobs <- index:
			case <-ctx.Done():
				close(jobs)
				workers.Wait()
				return GoPackageIdentityInventory{}, ctx.Err()
			}
		}
		close(jobs)
		workers.Wait()
		if err := ctx.Err(); err != nil {
			return GoPackageIdentityInventory{}, err
		}
	}

	modules, workspaces, vendors, diagnostics := collectManifestOutcomes(outcomes)
	packages := buildPackageIndex(input.Syntax)
	assignPackagesToModules(modules, packages)
	contexts, states, contextDiagnostics := buildContexts(modules, workspaces, vendors, packages)
	diagnostics = append(diagnostics, contextDiagnostics...)
	proofs := buildProofs(input.Syntax, modules, contexts, states, packages)

	sortModules(modules)
	sortContexts(contexts)
	sortProofs(proofs)
	sortDiagnostics(diagnostics)
	diagnostics, omitted := limitDiagnostics(diagnostics, engine.config.MaxDiagnosticsPerFile, engine.config.MaxDiagnostics)
	statistics := PackageIdentityStatistics{
		ManifestsInspected: len(manifestPaths), Contexts: len(contexts), Modules: len(modules),
		ProofsByStatus: map[string]int{}, Diagnostics: len(diagnostics), OmittedDiagnostics: omitted,
	}
	for _, proof := range proofs {
		statistics.ProofsByStatus[proof.Status.String()]++
	}
	moduleModels := make([]ModuleIdentity, len(modules))
	for index, module := range modules {
		moduleModels[index] = module.identity
	}
	return newInventory(contexts, moduleModels, proofs, diagnostics, statistics), nil
}

func validateInput(input Input) error {
	if input.Snapshot.RootPath() == "" {
		return ErrRepositorySnapshotRequired
	}
	if input.Snapshot.ArtifactName() != rie.RepositorySnapshotArtifactName || input.Snapshot.ArtifactVersion() != rie.RepositorySnapshotArtifactVersion {
		return ErrRepositorySnapshotIncompatible
	}
	metadata := input.Syntax.Metadata()
	if metadata.Name == "" {
		return ErrGoInventoryRequired
	}
	if input.Syntax.ArtifactName() != golang.ArtifactName || input.Syntax.ArtifactVersion() != golang.ArtifactVersion {
		return ErrGoInventoryIncompatible
	}
	foundSnapshot := false
	for _, source := range input.Syntax.SourceArtifacts() {
		if source.Name == input.Snapshot.ArtifactName() && source.Version == input.Snapshot.ArtifactVersion() {
			foundSnapshot = true
			break
		}
	}
	if !foundSnapshot {
		return ErrArtifactProvenanceMismatch
	}
	entries := make(map[string]bool)
	_ = input.Snapshot.ForEachEntry(func(entry rie.RepositoryEntry) error {
		entries[entry.Path] = entry.IsDir
		return nil
	})
	for _, file := range input.Syntax.Files() {
		isDirectory, exists := entries[file.Path]
		if !exists || isDirectory {
			return fmt.Errorf("%w: Go file %s is absent from RepositorySnapshot", ErrArtifactProvenanceMismatch, file.Path)
		}
	}
	return nil
}

type manifestKind uint8

const (
	manifestModule manifestKind = iota + 1
	manifestWorkspace
	manifestVendor
)

type manifestOutcome struct {
	path        string
	kind        manifestKind
	digest      string
	module      *moduleData
	workspace   *workspaceData
	vendor      *vendorData
	diagnostics []lie.Diagnostic
}

type moduleData struct {
	identity     ModuleIdentity
	manifestPath string
	requires     map[string]requireData
	replacements []replacementData
	packages     map[string][]packageRecord
}

type requireData struct {
	version  string
	evidence PackageIdentityEvidence
}

type replacementData struct {
	oldPath    string
	oldVersion string
	localRoot  string
	filesystem bool
	local      bool
	evidence   PackageIdentityEvidence
}

type workspaceData struct {
	path         string
	root         string
	goVersion    string
	evidence     []PackageIdentityEvidence
	uses         []workspaceUse
	replacements []replacementData
}

type workspaceUse struct {
	root     string
	evidence PackageIdentityEvidence
}

type vendorData struct {
	path     string
	root     string
	packages map[string][]vendorPackage
}

type vendorPackage struct {
	modulePath      string
	moduleVersion   string
	moduleEvidence  PackageIdentityEvidence
	packageEvidence PackageIdentityEvidence
}

func collectManifestPaths(entries []rie.RepositoryEntry) []string {
	result := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		base := path.Base(entry.Path)
		if base == "go.mod" || base == "go.work" || (base == "modules.txt" && path.Base(path.Dir(entry.Path)) == "vendor") {
			result = append(result, entry.Path)
		}
	}
	sort.Strings(result)
	return result
}

func (engine *engine) parseManifest(ctx context.Context, reader *manifestReader, manifestPath string) manifestOutcome {
	outcome := manifestOutcome{path: manifestPath, diagnostics: []lie.Diagnostic{}}
	data, err := reader.readFile(manifestPath, engine.config.MaxManifestSize)
	if err != nil {
		code := "package_manifest_unreadable"
		severity := lie.SeverityWarning
		switch {
		case errors.Is(err, ErrManifestMissing):
			code = "package_manifest_missing"
		case errors.Is(err, ErrManifestOversized):
			code = "package_manifest_oversized"
		case errors.Is(err, ErrManifestOutsideRoot):
			code, severity = "package_manifest_outside_root", lie.SeverityError
		}
		outcome.diagnostics = append(outcome.diagnostics, manifestDiagnostic(severity, code, err.Error(), manifestPath, nil))
		return outcome
	}
	if err := ctx.Err(); err != nil {
		return outcome
	}
	digestBytes := sha256.Sum256(data)
	outcome.digest = fmt.Sprintf("sha256:%x", digestBytes)
	switch {
	case path.Base(manifestPath) == "go.mod":
		outcome.kind = manifestModule
		outcome.module, outcome.diagnostics = parseModuleManifest(manifestPath, data, outcome.digest)
	case path.Base(manifestPath) == "go.work":
		outcome.kind = manifestWorkspace
		outcome.workspace, outcome.diagnostics = parseWorkspaceManifest(manifestPath, data, outcome.digest)
	default:
		outcome.kind = manifestVendor
		outcome.vendor, outcome.diagnostics = parseVendorManifest(manifestPath, data, outcome.digest)
	}
	return outcome
}

func parseModuleManifest(manifestPath string, data []byte, digest string) (*moduleData, []lie.Diagnostic) {
	parsed, err := modfile.Parse(manifestPath, data, nil)
	if err != nil {
		return nil, []lie.Diagnostic{manifestDiagnostic(lie.SeverityWarning, "go_mod_parse_error", err.Error(), manifestPath, nil)}
	}
	if parsed.Module == nil || parsed.Module.Mod.Path == "" {
		return nil, []lie.Diagnostic{manifestDiagnostic(lie.SeverityWarning, "go_mod_module_required", "go.mod has no module directive", manifestPath, nil)}
	}
	root := cleanDirectory(path.Dir(manifestPath))
	moduleEvidence := evidenceForLine(data, manifestPath, digest, "go.mod.module", parsed.Module.Mod.Path, parsed.Module.Syntax)
	goVersion := ""
	if parsed.Go != nil {
		goVersion = parsed.Go.Version
	}
	module := &moduleData{
		identity: ModuleIdentity{
			ID: moduleID(root, parsed.Module.Mod.Path), ModulePath: parsed.Module.Mod.Path, Root: root,
			GoVersion: goVersion, Evidence: []PackageIdentityEvidence{moduleEvidence},
		},
		manifestPath: manifestPath,
		requires:     make(map[string]requireData), replacements: []replacementData{}, packages: make(map[string][]packageRecord),
	}
	if parsed.Go != nil {
		module.identity.Evidence = append(module.identity.Evidence, evidenceForLine(data, manifestPath, digest, "go.mod.go", parsed.Go.Version, parsed.Go.Syntax))
	}
	if parsed.Toolchain != nil {
		module.identity.Evidence = append(module.identity.Evidence, evidenceForLine(data, manifestPath, digest, "go.mod.toolchain", parsed.Toolchain.Name, parsed.Toolchain.Syntax))
	}
	sortEvidence(module.identity.Evidence)
	for _, required := range parsed.Require {
		module.requires[required.Mod.Path] = requireData{
			version:  required.Mod.Version,
			evidence: evidenceForLine(data, manifestPath, digest, "go.mod.require", required.Mod.Path+"@"+required.Mod.Version, required.Syntax),
		}
	}
	for _, replacement := range parsed.Replace {
		module.replacements = append(module.replacements, buildReplacement(data, manifestPath, digest, root, replacement.Old.Path, replacement.Old.Version, replacement.New.Path, replacement.New.Version, replacement.Syntax, "go.mod.replace"))
	}
	return module, nil
}

func parseWorkspaceManifest(manifestPath string, data []byte, digest string) (*workspaceData, []lie.Diagnostic) {
	parsed, err := modfile.ParseWork(manifestPath, data, nil)
	if err != nil {
		return nil, []lie.Diagnostic{manifestDiagnostic(lie.SeverityWarning, "go_work_parse_error", err.Error(), manifestPath, nil)}
	}
	root := cleanDirectory(path.Dir(manifestPath))
	workspace := &workspaceData{path: manifestPath, root: root, evidence: []PackageIdentityEvidence{}, uses: []workspaceUse{}, replacements: []replacementData{}}
	if parsed.Go != nil {
		workspace.goVersion = parsed.Go.Version
		workspace.evidence = append(workspace.evidence, evidenceForLine(data, manifestPath, digest, "go.work.go", parsed.Go.Version, parsed.Go.Syntax))
	}
	if parsed.Toolchain != nil {
		workspace.evidence = append(workspace.evidence, evidenceForLine(data, manifestPath, digest, "go.work.toolchain", parsed.Toolchain.Name, parsed.Toolchain.Syntax))
	}
	diagnostics := make([]lie.Diagnostic, 0)
	for _, use := range parsed.Use {
		useRoot, ok := resolveLocalPath(root, use.Path)
		if !ok {
			diagnostics = append(diagnostics, manifestDiagnostic(lie.SeverityWarning, "go_work_use_outside_root", "workspace use path is outside the repository snapshot", manifestPath, rangeForLine(data, manifestPath, use.Syntax)))
			continue
		}
		workspace.uses = append(workspace.uses, workspaceUse{
			root: useRoot, evidence: evidenceForLine(data, manifestPath, digest, "go.work.use", use.Path, use.Syntax),
		})
		workspace.evidence = append(workspace.evidence, workspace.uses[len(workspace.uses)-1].evidence)
	}
	for _, replacement := range parsed.Replace {
		workspace.replacements = append(workspace.replacements, buildReplacement(data, manifestPath, digest, root, replacement.Old.Path, replacement.Old.Version, replacement.New.Path, replacement.New.Version, replacement.Syntax, "go.work.replace"))
	}
	sortEvidence(workspace.evidence)
	return workspace, diagnostics
}

func buildReplacement(data []byte, manifestPath, digest, root, oldPath, oldVersion, newPath, newVersion string, line *modfile.Line, rule string) replacementData {
	localRoot, local := "", false
	filesystem := newVersion == ""
	if filesystem {
		localRoot, local = resolveLocalPath(root, newPath)
	}
	value := oldPath
	if oldVersion != "" {
		value += "@" + oldVersion
	}
	value += "=>" + newPath
	if newVersion != "" {
		value += "@" + newVersion
	}
	return replacementData{
		oldPath: oldPath, oldVersion: oldVersion,
		localRoot: localRoot, filesystem: filesystem, local: local, evidence: evidenceForLine(data, manifestPath, digest, rule, value, line),
	}
}

func parseVendorManifest(manifestPath string, data []byte, digest string) (*vendorData, []lie.Diagnostic) {
	vendor := &vendorData{path: manifestPath, root: cleanDirectory(path.Dir(manifestPath)), packages: make(map[string][]vendorPackage)}
	lines := bytes.Split(data, []byte("\n"))
	offset := 0
	currentModulePath, currentModuleVersion := "", ""
	var currentModuleEvidence PackageIdentityEvidence
	for lineIndex, raw := range lines {
		trimmed := strings.TrimSpace(strings.TrimSuffix(string(raw), "\r"))
		start := offset
		if trimmed != "" {
			start += bytes.Index(raw, []byte(trimmed))
		}
		end := start + len(trimmed)
		location := lie.SourceRange{
			File:  manifestPath,
			Start: lie.Position{Offset: start, Line: lineIndex + 1, Column: start - offset + 1},
			End:   lie.Position{Offset: end, Line: lineIndex + 1, Column: end - offset + 1},
		}
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "##") {
			declaration := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			left := strings.TrimSpace(strings.SplitN(declaration, "=>", 2)[0])
			fields := strings.Fields(left)
			currentModulePath, currentModuleVersion = "", ""
			if len(fields) > 0 {
				currentModulePath = fields[0]
			}
			if len(fields) > 1 {
				currentModuleVersion = fields[1]
			}
			currentModuleEvidence = PackageIdentityEvidence{File: manifestPath, ContentDigest: digest, Rule: "vendor.module", Value: left, Location: &location}
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "#") && currentModulePath != "" {
			packageEvidence := PackageIdentityEvidence{File: manifestPath, ContentDigest: digest, Rule: "vendor.package", Value: trimmed, Location: &location}
			vendor.packages[trimmed] = append(vendor.packages[trimmed], vendorPackage{
				modulePath: currentModulePath, moduleVersion: currentModuleVersion,
				moduleEvidence: currentModuleEvidence, packageEvidence: packageEvidence,
			})
		}
		offset += len(raw) + 1
	}
	for packagePath := range vendor.packages {
		sort.Slice(vendor.packages[packagePath], func(i, j int) bool {
			left, right := vendor.packages[packagePath][i], vendor.packages[packagePath][j]
			if left.modulePath != right.modulePath {
				return left.modulePath < right.modulePath
			}
			return left.moduleVersion < right.moduleVersion
		})
	}
	return vendor, nil
}

func collectManifestOutcomes(outcomes []manifestOutcome) ([]*moduleData, []*workspaceData, []*vendorData, []lie.Diagnostic) {
	modules := make([]*moduleData, 0)
	workspaces := make([]*workspaceData, 0)
	vendors := make([]*vendorData, 0)
	diagnostics := make([]lie.Diagnostic, 0)
	for _, outcome := range outcomes {
		diagnostics = append(diagnostics, outcome.diagnostics...)
		if outcome.module != nil {
			modules = append(modules, outcome.module)
		}
		if outcome.workspace != nil {
			workspaces = append(workspaces, outcome.workspace)
		}
		if outcome.vendor != nil {
			vendors = append(vendors, outcome.vendor)
		}
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].identity.ID < modules[j].identity.ID })
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].path < workspaces[j].path })
	sort.Slice(vendors, func(i, j int) bool { return vendors[i].path < vendors[j].path })
	return modules, workspaces, vendors, diagnostics
}

type packageRecord struct {
	id         string
	name       string
	directory  string
	importable bool
}

type packageIndex struct {
	byID        map[string]packageRecord
	byDirectory map[string][]packageRecord
}

func buildPackageIndex(inventory golang.GoLanguageInventory) packageIndex {
	files := make(map[string]golang.GoFile)
	for _, file := range inventory.Files() {
		files[file.ID] = file
	}
	result := packageIndex{byID: make(map[string]packageRecord), byDirectory: make(map[string][]packageRecord)}
	for _, pkg := range inventory.Packages() {
		importable := false
		for _, fileID := range pkg.FileIDs {
			file, exists := files[fileID]
			if exists && file.Status == golang.FileStatusParsed && !file.IsTest {
				importable = true
				break
			}
		}
		record := packageRecord{id: pkg.ID, name: pkg.Name, directory: cleanDirectory(pkg.Directory), importable: importable}
		result.byID[pkg.ID] = record
		result.byDirectory[record.directory] = append(result.byDirectory[record.directory], record)
	}
	for directory := range result.byDirectory {
		sort.Slice(result.byDirectory[directory], func(i, j int) bool { return result.byDirectory[directory][i].id < result.byDirectory[directory][j].id })
	}
	return result
}

func assignPackagesToModules(modules []*moduleData, packages packageIndex) {
	for _, records := range packages.byDirectory {
		for _, record := range records {
			if !record.importable {
				continue
			}
			owner := nearestModule(record.directory, modules)
			if owner == nil {
				continue
			}
			relative := relativeDirectory(owner.identity.Root, record.directory)
			owner.packages[relative] = append(owner.packages[relative], record)
		}
	}
	for _, module := range modules {
		for relative := range module.packages {
			sort.Slice(module.packages[relative], func(i, j int) bool { return module.packages[relative][i].id < module.packages[relative][j].id })
		}
	}
}

func nearestModule(directory string, modules []*moduleData) *moduleData {
	var selected *moduleData
	for _, module := range modules {
		if !containsDirectory(module.identity.Root, directory) {
			continue
		}
		if selected == nil || len(module.identity.Root) > len(selected.identity.Root) {
			selected = module
		}
	}
	return selected
}

type contextState struct {
	model             ResolutionContext
	modules           []*moduleData
	repositoryModules []*moduleData
	importing         *moduleData
	workspace         *workspaceData
	vendor            *vendorData
	packages          packageIndex
}

func buildContexts(modules []*moduleData, workspaces []*workspaceData, vendors []*vendorData, packages packageIndex) ([]ResolutionContext, map[string]contextState, []lie.Diagnostic) {
	contexts := make([]ResolutionContext, 0)
	states := make(map[string]contextState)
	diagnostics := make([]lie.Diagnostic, 0)
	moduleByRoot := make(map[string]*moduleData)
	for _, module := range modules {
		moduleByRoot[module.identity.Root] = module
		model := ResolutionContext{
			ID: contextID(ContextSingleModule, module.identity.Root), Kind: ContextSingleModule, Root: module.identity.Root,
			ManifestFiles: []string{module.manifestPath}, MainModuleIDs: []string{module.identity.ID}, Evidence: cloneEvidence(module.identity.Evidence),
		}
		contexts = append(contexts, model)
		states[model.ID] = contextState{model: model, modules: []*moduleData{module}, repositoryModules: modules, importing: module, packages: packages}
		if vendor := vendorForRoot(vendors, path.Join(module.identity.Root, "vendor")); vendor != nil && goVersionAtLeast(module.identity.GoVersion, 1, 14) {
			vendorModel := ResolutionContext{
				ID: contextID(ContextModuleVendor, module.identity.Root), Kind: ContextModuleVendor, Root: module.identity.Root,
				ManifestFiles: []string{module.manifestPath, vendor.path}, MainModuleIDs: []string{module.identity.ID}, Evidence: cloneEvidence(module.identity.Evidence),
			}
			contexts = append(contexts, vendorModel)
			states[vendorModel.ID] = contextState{model: vendorModel, modules: []*moduleData{module}, repositoryModules: modules, importing: module, vendor: vendor, packages: packages}
		}
	}
	for _, workspace := range workspaces {
		mainModules := make([]*moduleData, 0)
		manifestFiles := []string{workspace.path}
		for _, use := range workspace.uses {
			module := moduleByRoot[use.root]
			if module == nil {
				diagnostics = append(diagnostics, manifestDiagnostic(lie.SeverityWarning, "go_work_use_module_missing", "workspace use target has no valid go.mod", workspace.path, use.evidence.Location))
				continue
			}
			mainModules = append(mainModules, module)
			manifestFiles = append(manifestFiles, module.manifestPath)
		}
		sort.Slice(mainModules, func(i, j int) bool { return mainModules[i].identity.ID < mainModules[j].identity.ID })
		mainIDs := moduleIDs(mainModules)
		sort.Strings(manifestFiles)
		manifestFiles = compactStrings(manifestFiles)
		model := ResolutionContext{
			ID: contextID(ContextWorkspace, workspace.root), Kind: ContextWorkspace, Root: workspace.root,
			ManifestFiles: manifestFiles, MainModuleIDs: mainIDs, Evidence: cloneEvidence(workspace.evidence),
		}
		contexts = append(contexts, model)
		states[model.ID] = contextState{model: model, modules: mainModules, repositoryModules: modules, workspace: workspace, packages: packages}
		if vendor := vendorForRoot(vendors, path.Join(workspace.root, "vendor")); vendor != nil && goVersionAtLeast(workspace.goVersion, 1, 22) {
			vendorManifests := append(append([]string(nil), manifestFiles...), vendor.path)
			sort.Strings(vendorManifests)
			vendorModel := ResolutionContext{
				ID: contextID(ContextWorkspaceVendor, workspace.root), Kind: ContextWorkspaceVendor, Root: workspace.root,
				ManifestFiles: vendorManifests, MainModuleIDs: mainIDs, Evidence: cloneEvidence(workspace.evidence),
			}
			contexts = append(contexts, vendorModel)
			states[vendorModel.ID] = contextState{model: vendorModel, modules: mainModules, repositoryModules: modules, workspace: workspace, vendor: vendor, packages: packages}
		}
	}
	packageIDs := make([]string, 0, len(packages.byID))
	for packageID, record := range packages.byID {
		if record.importable && nearestModule(record.directory, modules) == nil {
			packageIDs = append(packageIDs, packageID)
		}
	}
	sort.Strings(packageIDs)
	for _, packageID := range packageIDs {
		record := packages.byID[packageID]
		model := ResolutionContext{
			ID: contextID(ContextUnmanaged, packageID), Kind: ContextUnmanaged, Root: record.directory,
			ManifestFiles: []string{}, MainModuleIDs: []string{}, Evidence: []PackageIdentityEvidence{},
		}
		contexts = append(contexts, model)
		states[model.ID] = contextState{model: model, repositoryModules: modules, packages: packages}
	}
	sortContexts(contexts)
	return contexts, states, diagnostics
}

func buildProofs(inventory golang.GoLanguageInventory, modules []*moduleData, contexts []ResolutionContext, states map[string]contextState, packages packageIndex) []PackageIdentityProof {
	importsByPackage := make(map[string]map[string]struct{})
	for _, file := range inventory.Files() {
		if file.Status != golang.FileStatusParsed || file.PackageID == "" {
			continue
		}
		if importsByPackage[file.PackageID] == nil {
			importsByPackage[file.PackageID] = make(map[string]struct{})
		}
		for _, imported := range file.Imports {
			importsByPackage[file.PackageID][imported.Path] = struct{}{}
		}
	}
	moduleByPackage := make(map[string]*moduleData)
	for packageID, record := range packages.byID {
		moduleByPackage[packageID] = nearestModule(record.directory, modules)
	}
	proofs := make([]PackageIdentityProof, 0)
	packageIDs := make([]string, 0, len(importsByPackage))
	for packageID := range importsByPackage {
		packageIDs = append(packageIDs, packageID)
	}
	sort.Strings(packageIDs)
	for _, packageID := range packageIDs {
		importPaths := mapKeys(importsByPackage[packageID])
		owner := moduleByPackage[packageID]
		applicable := applicableContexts(packageID, owner, contexts, states)
		for _, importPath := range importPaths {
			for _, model := range applicable {
				state := states[model.ID]
				state.importing = owner
				proofs = append(proofs, resolveProof(packageID, importPath, state))
			}
		}
	}
	sortProofs(proofs)
	return proofs
}

func applicableContexts(packageID string, owner *moduleData, contexts []ResolutionContext, states map[string]contextState) []ResolutionContext {
	if owner == nil {
		for _, model := range contexts {
			if model.Kind == ContextUnmanaged && model.ID == contextID(ContextUnmanaged, packageID) {
				return []ResolutionContext{model}
			}
		}
		return nil
	}
	result := make([]ResolutionContext, 0)
	for _, model := range contexts {
		state := states[model.ID]
		for _, module := range state.modules {
			if module.identity.ID == owner.identity.ID {
				result = append(result, model)
				break
			}
		}
	}
	sortContexts(result)
	return result
}

func resolveProof(importingPackageID, importPath string, state contextState) PackageIdentityProof {
	proof := PackageIdentityProof{
		ResolutionContextID: state.model.ID, ImportingPackageID: importingPackageID, ImportPath: importPath,
		Kinds: []ProofKind{}, Status: ProofExternal, Evidence: []PackageIdentityEvidence{}, CandidatePackageIDs: []string{},
	}
	if state.model.Kind == ContextUnmanaged || state.importing == nil {
		proof.Status = ProofUnresolved
		proof.ID = proofID(proof)
		return proof
	}
	matchingModules := longestMatchingModules(importPath, state.modules)
	if len(matchingModules) > 0 {
		candidates, evidence := packageCandidates(importPath, matchingModules)
		proof.Evidence = evidence
		kind := ProofSameModule
		if state.model.Kind == ContextWorkspace || state.model.Kind == ContextWorkspaceVendor {
			kind = ProofWorkspaceModule
		}
		proof.Kinds = []ProofKind{kind}
		applyCandidates(&proof, candidates, state.packages)
		proof.ID = proofID(proof)
		return proof
	}
	if state.model.Kind == ContextModuleVendor || state.model.Kind == ContextWorkspaceVendor {
		applyVendorProof(&proof, state.importing, state.vendor, state.packages)
		proof.ID = proofID(proof)
		return proof
	}
	if state.workspace != nil {
		if applyReplacementProof(&proof, importPath, state.importing, state.workspace.replacements, state.repositoryModules, state.packages) {
			proof.ID = proofID(proof)
			return proof
		}
	}
	if applyReplacementProof(&proof, importPath, state.importing, state.importing.replacements, state.repositoryModules, state.packages) {
		proof.ID = proofID(proof)
		return proof
	}
	proof.ID = proofID(proof)
	return proof
}

func longestMatchingModules(importPath string, modules []*moduleData) []*moduleData {
	result := make([]*moduleData, 0)
	longest := -1
	for _, module := range modules {
		if !hasImportPrefix(importPath, module.identity.ModulePath) {
			continue
		}
		length := len(module.identity.ModulePath)
		if length > longest {
			result, longest = result[:0], length
		}
		if length == longest {
			result = append(result, module)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].identity.ID < result[j].identity.ID })
	return result
}

func packageCandidates(importPath string, modules []*moduleData) ([]packageRecord, []PackageIdentityEvidence) {
	candidates := make([]packageRecord, 0)
	evidence := make([]PackageIdentityEvidence, 0)
	for _, module := range modules {
		relative := strings.TrimPrefix(importPath, module.identity.ModulePath)
		relative = strings.TrimPrefix(relative, "/")
		candidates = append(candidates, module.packages[relative]...)
		evidence = append(evidence, module.identity.Evidence...)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].id < candidates[j].id })
	sortEvidence(evidence)
	return compactPackages(candidates), compactEvidence(evidence)
}

func applyCandidates(proof *PackageIdentityProof, candidates []packageRecord, packages packageIndex) {
	if len(candidates) == 1 {
		proof.Status = ProofResolved
		proof.TargetPackageID = candidates[0].id
		proof.TargetDirectory = candidates[0].directory
		return
	}
	if len(candidates) > 1 {
		proof.Status = ProofAmbiguous
		for _, candidate := range candidates {
			proof.CandidatePackageIDs = append(proof.CandidatePackageIDs, candidate.id)
		}
		return
	}
	proof.Status = ProofUnresolved
	_ = packages
}

func applyVendorProof(proof *PackageIdentityProof, importing *moduleData, vendor *vendorData, packages packageIndex) {
	proof.Kinds = []ProofKind{ProofVendor}
	if importing == nil || vendor == nil {
		proof.Status = ProofUnresolved
		return
	}
	declarations := vendor.packages[proof.ImportPath]
	if len(declarations) == 0 {
		proof.Status = ProofExternal
		return
	}
	valid := make([]vendorPackage, 0, len(declarations))
	for _, declaration := range declarations {
		proof.Evidence = append(proof.Evidence, declaration.moduleEvidence, declaration.packageEvidence)
		requirement, required := importing.requires[declaration.modulePath]
		if required && requirement.version == declaration.moduleVersion && hasImportPrefix(proof.ImportPath, declaration.modulePath) {
			valid = append(valid, declaration)
			proof.Evidence = append(proof.Evidence, requirement.evidence)
		}
	}
	proof.Evidence = compactEvidenceSorted(proof.Evidence)
	if len(valid) == 0 {
		proof.Status = ProofUnresolved
		return
	}
	if len(valid) > 1 {
		proof.Status = ProofAmbiguous
		return
	}
	directory := cleanDirectory(path.Join(vendor.root, proof.ImportPath))
	candidates := importablePackages(packages.byDirectory[directory])
	applyCandidates(proof, candidates, packages)
}

func applyReplacementProof(proof *PackageIdentityProof, importPath string, importing *moduleData, replacements []replacementData, modules []*moduleData, packages packageIndex) bool {
	type replacementMatch struct {
		replacement replacementData
		requirement requireData
	}
	matches := make([]replacementMatch, 0)
	longest := -1
	for _, replacement := range replacements {
		requirement, required := importing.requires[replacement.oldPath]
		if !required || (replacement.oldVersion != "" && replacement.oldVersion != requirement.version) || !hasImportPrefix(importPath, replacement.oldPath) {
			continue
		}
		length := len(replacement.oldPath)
		if length > longest {
			matches, longest = matches[:0], length
		}
		if length == longest {
			matches = append(matches, replacementMatch{replacement: replacement, requirement: requirement})
		}
	}
	if len(matches) == 0 {
		return false
	}
	preferSpecific := false
	for _, match := range matches {
		if match.replacement.oldVersion != "" {
			preferSpecific = true
			break
		}
	}
	candidates := make([]packageRecord, 0)
	evidence := make([]PackageIdentityEvidence, 0)
	filesystemSelected := false
	for _, match := range matches {
		if (match.replacement.oldVersion != "") != preferSpecific {
			continue
		}
		evidence = append(evidence, match.requirement.evidence, match.replacement.evidence)
		filesystemSelected = filesystemSelected || match.replacement.filesystem
		if !match.replacement.local {
			continue
		}
		target := moduleAtRoot(modules, match.replacement.localRoot)
		if target == nil {
			continue
		}
		relative := strings.TrimPrefix(importPath, match.replacement.oldPath)
		relative = strings.TrimPrefix(relative, "/")
		candidates = append(candidates, target.packages[relative]...)
		evidence = append(evidence, target.identity.Evidence...)
	}
	proof.Evidence = compactEvidenceSorted(evidence)
	if !filesystemSelected {
		proof.Kinds = []ProofKind{}
		proof.Status = ProofExternal
		return true
	}
	proof.Kinds = []ProofKind{ProofLocalReplace}
	applyCandidates(proof, compactPackagesSorted(candidates), packages)
	return true
}

func proofID(proof PackageIdentityProof) string {
	kinds := append([]ProofKind(nil), proof.Kinds...)
	sort.Slice(kinds, func(i, j int) bool { return kinds[i].String() < kinds[j].String() })
	kinds = compactKinds(kinds)
	names := make([]string, len(kinds))
	for index, kind := range kinds {
		names[index] = kind.String()
	}
	return fmt.Sprintf(
		"go:package-proof:v1:%d:%s#%d:%s#%d:%s#%s",
		len(proof.ResolutionContextID), proof.ResolutionContextID,
		len(proof.ImportingPackageID), proof.ImportingPackageID,
		len(proof.ImportPath), proof.ImportPath,
		strings.Join(names, ","),
	)
}

func moduleID(root, modulePath string) string {
	return fmt.Sprintf("go:module:v1:%d:%s#%d:%s", len(root), root, len(modulePath), modulePath)
}

func contextID(kind ResolutionContextKind, root string) string {
	return fmt.Sprintf("go:package-context:v1:%s:%d:%s", kind.String(), len(root), root)
}

func evidenceForLine(data []byte, file, digest, rule, value string, line *modfile.Line) PackageIdentityEvidence {
	return PackageIdentityEvidence{File: file, ContentDigest: digest, Rule: rule, Value: value, Location: rangeForLine(data, file, line)}
}

func rangeForLine(data []byte, file string, line *modfile.Line) *lie.SourceRange {
	if line == nil {
		return nil
	}
	start, end := line.Span()
	startOffset := clampOffset(start.Byte, len(data))
	endOffset := clampOffset(end.Byte, len(data))
	location := lie.SourceRange{
		File:  file,
		Start: lie.Position{Offset: startOffset, Line: start.Line, Column: byteColumn(data, startOffset)},
		End:   lie.Position{Offset: endOffset, Line: end.Line, Column: byteColumn(data, endOffset)},
	}
	return &location
}

func byteColumn(data []byte, offset int) int {
	offset = clampOffset(offset, len(data))
	lineStart := bytes.LastIndexByte(data[:offset], '\n') + 1
	return offset - lineStart + 1
}

func clampOffset(value, maximum int) int {
	if value < 0 {
		return 0
	}
	if value > maximum {
		return maximum
	}
	return value
}

func resolveLocalPath(base, value string) (string, bool) {
	native := filepath.FromSlash(value)
	if value == "" || path.IsAbs(strings.ReplaceAll(value, "\\", "/")) || filepath.IsAbs(native) || filepath.VolumeName(native) != "" {
		return "", false
	}
	clean := cleanDirectory(path.Join(base, strings.ReplaceAll(value, "\\", "/")))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func cleanDirectory(value string) string {
	clean := path.Clean(strings.ReplaceAll(value, "\\", "/"))
	if clean == "." {
		return ""
	}
	return strings.TrimPrefix(clean, "./")
}

func containsDirectory(root, directory string) bool {
	if root == "" {
		return directory != ".." && !strings.HasPrefix(directory, "../")
	}
	return directory == root || strings.HasPrefix(directory, root+"/")
}

func relativeDirectory(root, directory string) string {
	if root == "" {
		return directory
	}
	return strings.TrimPrefix(strings.TrimPrefix(directory, root), "/")
}

func vendorForRoot(vendors []*vendorData, vendorRoot string) *vendorData {
	vendorRoot = cleanDirectory(vendorRoot)
	for _, vendor := range vendors {
		if vendor.root == vendorRoot {
			return vendor
		}
	}
	return nil
}

func moduleAtRoot(modules []*moduleData, root string) *moduleData {
	for _, module := range modules {
		if module.identity.Root == root {
			return module
		}
	}
	return nil
}

func moduleIDs(modules []*moduleData) []string {
	result := make([]string, len(modules))
	for index, module := range modules {
		result[index] = module.identity.ID
	}
	sort.Strings(result)
	return compactStrings(result)
}

func goVersionAtLeast(value string, major, minor int) bool {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) < 2 {
		return false
	}
	parsedMajor, errMajor := strconv.Atoi(parts[0])
	parsedMinor, errMinor := strconv.Atoi(numericPrefix(parts[1]))
	if errMajor != nil || errMinor != nil {
		return false
	}
	return parsedMajor > major || (parsedMajor == major && parsedMinor >= minor)
}

func numericPrefix(value string) string {
	index := 0
	for index < len(value) && value[index] >= '0' && value[index] <= '9' {
		index++
	}
	return value[:index]
}

func hasImportPrefix(importPath, modulePath string) bool {
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}

func importablePackages(source []packageRecord) []packageRecord {
	result := make([]packageRecord, 0, len(source))
	for _, record := range source {
		if record.importable {
			result = append(result, record)
		}
	}
	return result
}

func compactPackagesSorted(source []packageRecord) []packageRecord {
	sort.Slice(source, func(i, j int) bool { return source[i].id < source[j].id })
	return compactPackages(source)
}

func compactPackages(source []packageRecord) []packageRecord {
	if len(source) == 0 {
		return []packageRecord{}
	}
	write := 1
	for read := 1; read < len(source); read++ {
		if source[read].id == source[write-1].id {
			continue
		}
		source[write] = source[read]
		write++
	}
	return source[:write]
}

func compactKinds(source []ProofKind) []ProofKind {
	if len(source) == 0 {
		return []ProofKind{}
	}
	write := 1
	for read := 1; read < len(source); read++ {
		if source[read] == source[write-1] {
			continue
		}
		source[write] = source[read]
		write++
	}
	return source[:write]
}

func compactEvidenceSorted(source []PackageIdentityEvidence) []PackageIdentityEvidence {
	sortEvidence(source)
	return compactEvidence(source)
}

func compactEvidence(source []PackageIdentityEvidence) []PackageIdentityEvidence {
	if len(source) == 0 {
		return []PackageIdentityEvidence{}
	}
	write := 1
	for read := 1; read < len(source); read++ {
		if evidenceKey(source[read]) == evidenceKey(source[write-1]) {
			continue
		}
		source[write] = source[read]
		write++
	}
	return source[:write]
}

func evidenceKey(value PackageIdentityEvidence) string {
	offset := 0
	if value.Location != nil {
		offset = value.Location.Start.Offset
	}
	return fmt.Sprintf("%s#%d#%s#%s", value.File, offset, value.Rule, value.Value)
}

func sortEvidence(values []PackageIdentityEvidence) {
	sort.Slice(values, func(i, j int) bool { return evidenceKey(values[i]) < evidenceKey(values[j]) })
}

func sortModules(values []*moduleData) {
	sort.Slice(values, func(i, j int) bool { return values[i].identity.ID < values[j].identity.ID })
	for _, module := range values {
		sortEvidence(module.identity.Evidence)
	}
}

func sortContexts(values []ResolutionContext) {
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
}

func sortProofs(values []PackageIdentityProof) {
	for index := range values {
		sort.Slice(values[index].Kinds, func(i, j int) bool { return values[index].Kinds[i].String() < values[index].Kinds[j].String() })
		values[index].Kinds = compactKinds(values[index].Kinds)
		sort.Strings(values[index].CandidatePackageIDs)
		values[index].CandidatePackageIDs = compactStrings(values[index].CandidatePackageIDs)
		sortEvidence(values[index].Evidence)
		values[index].Evidence = compactEvidence(values[index].Evidence)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	write := 1
	for read := 1; read < len(values); read++ {
		if values[read] == values[write-1] {
			continue
		}
		values[write] = values[read]
		write++
	}
	return values[:write]
}

func mapKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func manifestDiagnostic(severity lie.Severity, code, message, file string, location *lie.SourceRange) lie.Diagnostic {
	if location == nil && file != "" {
		location = &lie.SourceRange{File: file}
	}
	return lie.Diagnostic{Engine: "go-package-identity", Severity: severity, Code: code, Message: message, Location: location}
}

func sortDiagnostics(values []lie.Diagnostic) {
	sort.Slice(values, func(i, j int) bool { return diagnosticKey(values[i]) < diagnosticKey(values[j]) })
}

func diagnosticKey(value lie.Diagnostic) string {
	file, start, end := "", 0, 0
	if value.Location != nil {
		file, start, end = value.Location.File, value.Location.Start.Offset, value.Location.End.Offset
	}
	return fmt.Sprintf("%s#%012d#%012d#%d#%s#%s", file, start, end, value.Severity, value.Code, value.Message)
}

func limitDiagnostics(values []lie.Diagnostic, maximumPerFile, maximum int) ([]lie.Diagnostic, int) {
	unique := make([]lie.Diagnostic, 0, len(values))
	for _, value := range values {
		if len(unique) > 0 && diagnosticKey(unique[len(unique)-1]) == diagnosticKey(value) {
			continue
		}
		unique = append(unique, value)
	}
	perFile := make(map[string]int)
	kept := make([]lie.Diagnostic, 0, len(unique))
	omitted := 0
	for _, value := range unique {
		file := ""
		if value.Location != nil {
			file = value.Location.File
		}
		if perFile[file] >= maximumPerFile {
			omitted++
			continue
		}
		perFile[file]++
		kept = append(kept, value)
	}
	if omitted == 0 && len(kept) <= maximum {
		return kept, 0
	}
	ordinaryLimit := maximum - 1
	if len(kept) > ordinaryLimit {
		omitted += len(kept) - ordinaryLimit
		kept = kept[:ordinaryLimit]
	}
	kept = append(kept, manifestDiagnostic(lie.SeverityWarning, "package_identity_diagnostic_limit", fmt.Sprintf("%d diagnostics omitted", omitted), "", nil))
	return kept, omitted
}

type manifestReader struct {
	root        *os.Root
	canonical   string
	directories map[string]manifestDirectory
}

type manifestDirectory struct {
	root     *os.Root
	absolute string
	symlinks map[string]bool
	err      error
}

func newManifestReader(root string, candidates []string) (*manifestReader, error) {
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("%w: resolve repository root: %v", ErrRepositoryRootInvalid, err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve repository root: %v", ErrRepositoryRootInvalid, err)
	}
	rootHandle, err := os.OpenRoot(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: open repository root: %v", ErrRepositoryRootInvalid, err)
	}
	reader := &manifestReader{root: rootHandle, canonical: canonicalRoot, directories: make(map[string]manifestDirectory)}
	for _, candidate := range candidates {
		cleanPath := filepath.Clean(filepath.FromSlash(candidate))
		if !safeRelativePath(candidate, cleanPath) {
			continue
		}
		directory := filepath.Dir(cleanPath)
		if _, exists := reader.directories[directory]; exists {
			continue
		}
		absoluteDirectory, resolveErr := filepath.EvalSymlinks(filepath.Join(canonicalRoot, directory))
		record := manifestDirectory{absolute: absoluteDirectory, symlinks: map[string]bool{}, err: resolveErr}
		if resolveErr == nil && !withinRoot(canonicalRoot, absoluteDirectory) {
			record.err = ErrManifestOutsideRoot
		}
		if record.err == nil {
			record.root, record.err = rootHandle.OpenRoot(directory)
		}
		if record.err == nil {
			entries, readErr := os.ReadDir(absoluteDirectory)
			if readErr != nil {
				record.err = readErr
			} else {
				for _, entry := range entries {
					record.symlinks[entry.Name()] = entry.Type()&os.ModeSymlink != 0
				}
			}
		}
		reader.directories[directory] = record
	}
	return reader, nil
}

func (reader *manifestReader) readFile(relativePath string, maximumBytes int64) ([]byte, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(relativePath))
	if !safeRelativePath(relativePath, cleanPath) {
		return nil, fmt.Errorf("%w: %s", ErrManifestOutsideRoot, relativePath)
	}
	directory := reader.directories[filepath.Dir(cleanPath)]
	if directory.root == nil && directory.err == nil {
		return nil, fmt.Errorf("%w: %s was not authorized by the repository snapshot", ErrManifestUnreadable, relativePath)
	}
	if directory.err != nil {
		if errors.Is(directory.err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrManifestMissing, relativePath)
		}
		if errors.Is(directory.err, ErrManifestOutsideRoot) || errors.Is(directory.err, os.ErrPermission) || errors.Is(directory.err, os.ErrInvalid) {
			return nil, fmt.Errorf("%w: %s", ErrManifestOutsideRoot, relativePath)
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrManifestUnreadable, relativePath, directory.err)
	}
	baseName := filepath.Base(cleanPath)
	if directory.symlinks[baseName] {
		resolved, err := filepath.EvalSymlinks(filepath.Join(directory.absolute, baseName))
		if err != nil || !withinRoot(reader.canonical, resolved) {
			return nil, fmt.Errorf("%w: %s", ErrManifestOutsideRoot, relativePath)
		}
	}
	flags := os.O_RDONLY
	if runtime.GOOS == "windows" {
		flags |= 0x08000000
	}
	file, err := directory.root.OpenFile(baseName, flags, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrManifestMissing, relativePath)
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrManifestUnreadable, relativePath, err)
	}
	defer file.Close()
	information, err := file.Stat()
	if err != nil || !information.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrManifestUnreadable, relativePath)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrManifestUnreadable, relativePath, err)
	}
	if int64(len(data)) > maximumBytes {
		return nil, fmt.Errorf("%w: %s exceeds %d bytes", ErrManifestOversized, relativePath, maximumBytes)
	}
	return data, nil
}

func (reader *manifestReader) close() {
	for _, directory := range reader.directories {
		if directory.root != nil {
			_ = directory.root.Close()
		}
	}
	if reader.root != nil {
		_ = reader.root.Close()
	}
}

func safeRelativePath(original, clean string) bool {
	return original != "" && clean != "." && !filepath.IsAbs(clean) && filepath.VolumeName(clean) == "" && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func withinRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
