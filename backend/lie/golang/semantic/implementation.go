package semantic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

type engine struct{ config Config }

type integrator struct{ resolver Engine }

func (*engine) Name() string         { return "go-semantic" }
func (*engine) Version() string      { return engineVersion }
func (*engine) Language() string     { return "Go" }
func (*engine) ArtifactName() string { return ArtifactName }
func (*engine) Description() string {
	return "Verifies snapshot-authorized Go source and produces a bounded semantic candidate artifact"
}

func (candidate *integrator) Run(ctx context.Context, store *rie.ArtifactStore) (GoSemanticInventory, error) {
	if ctx == nil {
		return GoSemanticInventory{}, ErrContextRequired
	}
	if store == nil {
		return GoSemanticInventory{}, ErrArtifactStoreRequired
	}
	if err := ctx.Err(); err != nil {
		return GoSemanticInventory{}, err
	}
	if _, exists := store.Get(ArtifactName); exists {
		return GoSemanticInventory{}, fmt.Errorf("%w: %s", rie.ErrArtifactAlreadyExists, ArtifactName)
	}

	input, err := inputFromStore(store)
	if err != nil {
		return GoSemanticInventory{}, err
	}
	if err := ctx.Err(); err != nil {
		return GoSemanticInventory{}, err
	}
	inventory, err := candidate.resolver.Resolve(ctx, input)
	if err != nil {
		return GoSemanticInventory{}, err
	}
	if err := ctx.Err(); err != nil {
		return GoSemanticInventory{}, err
	}
	if err := store.Put(inventory); err != nil {
		return GoSemanticInventory{}, err
	}
	return inventory, nil
}

func inputFromStore(store *rie.ArtifactStore) (Input, error) {
	snapshotArtifact, exists := store.Get(rie.RepositorySnapshotArtifactName)
	if !exists {
		return Input{}, ErrMissingRepositorySnapshot
	}
	snapshot, valid := snapshotArtifact.(rie.RepositorySnapshot)
	if !valid {
		return Input{}, ErrIncompatibleRepositorySnapshot
	}

	syntaxArtifact, exists := store.Get(golang.ArtifactName)
	if !exists {
		return Input{}, ErrMissingGoLanguageInventory
	}
	syntax, valid := syntaxArtifact.(golang.GoLanguageInventory)
	if !valid {
		return Input{}, ErrIncompatibleGoInventory
	}

	identityArtifact, exists := store.Get(packageidentity.ArtifactName)
	if !exists {
		return Input{}, ErrMissingPackageIdentityInventory
	}
	identities, valid := identityArtifact.(packageidentity.GoPackageIdentityInventory)
	if !valid {
		return Input{}, ErrIncompatiblePackageIdentity
	}
	return Input{Snapshot: snapshot, Syntax: syntax, PackageIdentities: identities}, nil
}

func (engine *engine) Resolve(ctx context.Context, input Input) (GoSemanticInventory, error) {
	if ctx == nil {
		return GoSemanticInventory{}, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return GoSemanticInventory{}, err
	}
	validated, err := validateAndSnapshotInput(input)
	if err != nil {
		return GoSemanticInventory{}, err
	}

	syntaxFiles := validated.syntaxFiles
	sort.Slice(syntaxFiles, func(i, j int) bool {
		if syntaxFiles[i].Path != syntaxFiles[j].Path {
			return syntaxFiles[i].Path < syntaxFiles[j].Path
		}
		return syntaxFiles[i].ID < syntaxFiles[j].ID
	})
	syntaxSymbols := symbolsByFile(validated.syntaxSymbols)
	candidatePaths := make([]string, 0, len(syntaxFiles))
	for _, file := range syntaxFiles {
		if file.Status == golang.FileStatusParsed {
			candidatePaths = append(candidatePaths, file.Path)
		}
	}
	for _, proof := range validated.identityProofs {
		for _, evidence := range proof.Evidence {
			if evidence.File != "" {
				candidatePaths = append(candidatePaths, evidence.File)
			}
		}
	}
	reader, err := newSourceReader(input.Snapshot.RootPath(), candidatePaths)
	if err != nil {
		return GoSemanticInventory{}, err
	}
	defer reader.close()

	outcomes := make([]fileOutcome, len(syntaxFiles))
	workerCount := min(engine.config.MaxWorkers, max(1, len(syntaxFiles)))
	batchSize := max(workerCount*32, 32)
	retainedReferenceCandidates := 0
	referenceCandidateOmitted := 0
	for batchStart := 0; batchStart < len(syntaxFiles); batchStart += batchSize {
		batchEnd := min(batchStart+batchSize, len(syntaxFiles))
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
					outcomes[index] = engine.verifyFile(ctx, reader, syntaxFiles[index], syntaxSymbols[syntaxFiles[index].ID])
				}
			}()
		}
		for index := batchStart; index < batchEnd; index++ {
			select {
			case jobs <- index:
			case <-ctx.Done():
				close(jobs)
				workers.Wait()
				return GoSemanticInventory{}, ctx.Err()
			}
		}
		close(jobs)
		workers.Wait()
		if err := ctx.Err(); err != nil {
			return GoSemanticInventory{}, err
		}
		for index := batchStart; index < batchEnd; index++ {
			remaining := max(engine.config.MaxRelationships-retainedReferenceCandidates, 0)
			bounded, omitted := compactAndLimitReferenceCandidates(outcomes[index].references, remaining)
			outcomes[index].references = bounded
			retainedReferenceCandidates += len(bounded)
			referenceCandidateOmitted += omitted
		}
	}

	files, declarations, referenceCandidates, receivers, typeRelationCandidates, diagnostics, statistics := collectOutcomes(outcomes)
	releaseCollectedOutcomeData(outcomes)
	imports, importDiagnostics, err := bindImports(ctx, reader, syntaxFiles, files, validated.syntaxPackages, validated.identityProofs, engine.config.MaxSourceFileSize)
	if err != nil {
		return GoSemanticInventory{}, err
	}
	diagnostics = append(diagnostics, importDiagnostics...)
	typeRelations := bindTypeRelations(declarations, typeRelationCandidates, imports)
	referenceLimit := engine.config.MaxRelationships - len(imports) - len(receivers) - len(typeRelations)
	if referenceLimit < 0 {
		referenceLimit = 0
	}
	references, referenceOmitted, err := bindReferences(ctx, declarations, referenceCandidates, imports, referenceLimit)
	if err != nil {
		return GoSemanticInventory{}, err
	}
	referenceCandidates = nil
	typeRelationCandidates = nil
	satisfaction, interfaceDiagnostics, interfaceOmitted, err := evaluateInterfaceSatisfaction(ctx, outcomes, declarations, typeRelations, engine.config)
	if err != nil {
		return GoSemanticInventory{}, err
	}
	releaseOutcomeSources(outcomes)
	outcomes = nil
	diagnostics = append(diagnostics, interfaceDiagnostics...)
	imports, receivers, typeRelations, references, satisfaction, omittedRelationships := limitSemanticRelationships(imports, receivers, typeRelations, references, satisfaction, engine.config.MaxRelationships)
	omittedRelationships += referenceCandidateOmitted + referenceOmitted + interfaceOmitted
	updateReferenceStatistics(files, references, &statistics)
	statistics.ReceiverBindings = len(receivers)
	statistics.ImportBindingsByStatus = statusesForImports(imports)
	statistics.TypeRelations = len(typeRelations)
	statistics.InterfaceChecksByStatus = statusesForInterfaceChecks(satisfaction)
	statistics.OmittedRelationships = omittedRelationships
	if omittedRelationships > 0 {
		diagnostics = append(diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_relationship_limit", fmt.Sprintf("%d semantic relationships omitted", omittedRelationships), ""))
	}
	sortDiagnostics(diagnostics)
	diagnostics, omitted := limitDiagnostics(diagnostics, engine.config.MaxDiagnosticsPerFile, engine.config.MaxDiagnostics)
	statistics.Diagnostics = len(diagnostics)
	statistics.OmittedDiagnostics = omitted
	if err := ctx.Err(); err != nil {
		return GoSemanticInventory{}, err
	}
	return newInventory(files, declarations, references, receivers, imports, typeRelations, satisfaction, diagnostics, statistics), nil
}

type validatedInputSnapshot struct {
	syntaxFiles    []golang.GoFile
	syntaxPackages []golang.GoPackage
	syntaxSymbols  []golang.GoSymbol
	identityProofs []packageidentity.PackageIdentityProof
}

func validateAndSnapshotInput(input Input) (validatedInputSnapshot, error) {
	validated := validatedInputSnapshot{}
	snapshotMetadata := input.Snapshot.Metadata()
	if input.Snapshot.RootPath() == "" || snapshotMetadata.Name == "" {
		return validated, ErrMissingRepositorySnapshot
	}
	if input.Snapshot.ArtifactName() != rie.RepositorySnapshotArtifactName ||
		input.Snapshot.ArtifactVersion() != rie.RepositorySnapshotArtifactVersion ||
		snapshotMetadata.Name != rie.RepositorySnapshotArtifactName ||
		snapshotMetadata.Version != rie.RepositorySnapshotArtifactVersion {
		return validated, ErrIncompatibleRepositorySnapshot
	}

	syntaxMetadata := input.Syntax.Metadata()
	if syntaxMetadata.Name == "" {
		return validated, ErrMissingGoLanguageInventory
	}
	if input.Syntax.ArtifactName() != golang.ArtifactName || input.Syntax.ArtifactVersion() != golang.ArtifactVersion ||
		syntaxMetadata.Name != golang.ArtifactName || syntaxMetadata.Version != golang.ArtifactVersion {
		return validated, ErrIncompatibleGoInventory
	}

	identityMetadata := input.PackageIdentities.Metadata()
	if identityMetadata.Name == "" {
		return validated, ErrMissingPackageIdentityInventory
	}
	if input.PackageIdentities.ArtifactName() != packageidentity.ArtifactName ||
		input.PackageIdentities.ArtifactVersion() != packageidentity.ArtifactVersion ||
		identityMetadata.Name != packageidentity.ArtifactName || identityMetadata.Version != packageidentity.ArtifactVersion {
		return validated, ErrIncompatiblePackageIdentity
	}

	snapshotReference := rie.ArtifactReference{Name: rie.RepositorySnapshotArtifactName, Version: rie.RepositorySnapshotArtifactVersion}
	syntaxReference := rie.ArtifactReference{Name: golang.ArtifactName, Version: golang.ArtifactVersion}
	if !hasArtifactReference(input.Syntax.SourceArtifacts(), snapshotReference) ||
		!hasExactArtifactReferences(input.PackageIdentities.SourceArtifacts(), []rie.ArtifactReference{snapshotReference, syntaxReference}) {
		return validated, ErrArtifactProvenanceMismatch
	}

	entries := make(map[string]bool)
	for _, entry := range input.Snapshot.Entries() {
		if _, duplicate := entries[entry.Path]; duplicate {
			return validated, fmt.Errorf("%w: duplicate snapshot entry %s", ErrArtifactProvenanceMismatch, entry.Path)
		}
		entries[entry.Path] = entry.IsDir
	}
	fileIDs := make(map[string]struct{})
	filePaths := make(map[string]struct{})
	filesByID := make(map[string]golang.GoFile)
	validated.syntaxFiles = input.Syntax.Files()
	for _, file := range validated.syntaxFiles {
		isDirectory, exists := entries[file.Path]
		if file.ID == "" || file.Path == "" || !exists || isDirectory {
			return validated, fmt.Errorf("%w: Go file %s is absent from RepositorySnapshot", ErrArtifactProvenanceMismatch, file.Path)
		}
		if _, duplicate := fileIDs[file.ID]; duplicate {
			return validated, fmt.Errorf("%w: duplicate Go file ID %s", ErrArtifactProvenanceMismatch, file.ID)
		}
		if _, duplicate := filePaths[file.Path]; duplicate {
			return validated, fmt.Errorf("%w: duplicate Go file path %s", ErrArtifactProvenanceMismatch, file.Path)
		}
		fileIDs[file.ID] = struct{}{}
		filePaths[file.Path] = struct{}{}
		filesByID[file.ID] = file
	}

	packageIDs := make(map[string]struct{})
	validated.syntaxPackages = input.Syntax.Packages()
	for _, pkg := range validated.syntaxPackages {
		if pkg.ID == "" {
			return validated, fmt.Errorf("%w: Go package has an empty ID", ErrArtifactProvenanceMismatch)
		}
		packageIDs[pkg.ID] = struct{}{}
	}
	symbolIDs := make(map[string]struct{})
	validated.syntaxSymbols = input.Syntax.Symbols()
	for _, symbol := range validated.syntaxSymbols {
		file, exists := filesByID[symbol.FileID]
		if symbol.ID == "" || symbol.Name == "" || !exists || symbol.PackageID != file.PackageID || symbol.Location.File != file.Path || symbol.Location.Start.Offset < 0 || symbol.Location.End.Offset < symbol.Location.Start.Offset || symbol.Kind.String() == "unknown" {
			return validated, fmt.Errorf("%w: invalid Go syntax symbol %s", ErrArtifactProvenanceMismatch, symbol.ID)
		}
		if _, exists := packageIDs[symbol.PackageID]; !exists {
			return validated, fmt.Errorf("%w: symbol %s belongs to unknown package %s", ErrArtifactProvenanceMismatch, symbol.ID, symbol.PackageID)
		}
		if _, duplicate := symbolIDs[symbol.ID]; duplicate {
			return validated, fmt.Errorf("%w: duplicate Go symbol ID %s", ErrArtifactProvenanceMismatch, symbol.ID)
		}
		symbolIDs[symbol.ID] = struct{}{}
	}
	validated.identityProofs = input.PackageIdentities.Proofs()
	for _, proof := range validated.identityProofs {
		if _, exists := packageIDs[proof.ImportingPackageID]; !exists {
			return validated, fmt.Errorf("%w: proof %s imports from unknown package %s", ErrArtifactProvenanceMismatch, proof.ID, proof.ImportingPackageID)
		}
		if proof.TargetPackageID != "" {
			if _, exists := packageIDs[proof.TargetPackageID]; !exists {
				return validated, fmt.Errorf("%w: proof %s targets unknown package %s", ErrArtifactProvenanceMismatch, proof.ID, proof.TargetPackageID)
			}
		}
		for _, candidate := range proof.CandidatePackageIDs {
			if _, exists := packageIDs[candidate]; !exists {
				return validated, fmt.Errorf("%w: proof %s names unknown candidate %s", ErrArtifactProvenanceMismatch, proof.ID, candidate)
			}
		}
		for _, evidence := range proof.Evidence {
			if evidence.File == "" {
				continue
			}
			if evidence.ContentDigest == "" {
				return validated, fmt.Errorf("%w: proof %s has incomplete manifest evidence", ErrArtifactProvenanceMismatch, proof.ID)
			}
			isDirectory, exists := entries[evidence.File]
			if !exists || isDirectory {
				return validated, fmt.Errorf("%w: proof %s evidence %s is absent from RepositorySnapshot", ErrArtifactProvenanceMismatch, proof.ID, evidence.File)
			}
		}
	}
	return validated, nil
}

func hasArtifactReference(values []rie.ArtifactReference, expected rie.ArtifactReference) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func hasExactArtifactReferences(values, expected []rie.ArtifactReference) bool {
	if len(values) != len(expected) {
		return false
	}
	remaining := make(map[rie.ArtifactReference]int, len(expected))
	for _, value := range expected {
		remaining[value]++
	}
	for _, value := range values {
		if remaining[value] == 0 {
			return false
		}
		remaining[value]--
	}
	return true
}

type fileOutcome struct {
	path          string
	file          SemanticFile
	declarations  []SemanticDeclaration
	references    []referenceCandidate
	receivers     []receiverCandidate
	typeRelations []typeRelationCandidate
	diagnostics   []lie.Diagnostic
	source        []byte
}

func releaseCollectedOutcomeData(outcomes []fileOutcome) {
	for index := range outcomes {
		outcomes[index].declarations = nil
		outcomes[index].references = nil
		outcomes[index].receivers = nil
		outcomes[index].typeRelations = nil
		outcomes[index].diagnostics = nil
	}
}

func releaseOutcomeSources(outcomes []fileOutcome) {
	for index := range outcomes {
		outcomes[index].source = nil
	}
}

func (engine *engine) verifyFile(ctx context.Context, reader *sourceReader, source golang.GoFile, syntaxSymbols []golang.GoSymbol) fileOutcome {
	outcome := fileOutcome{
		path:        source.Path,
		file:        SemanticFile{FileID: source.ID, PackageID: source.PackageID},
		diagnostics: []lie.Diagnostic{},
	}
	switch source.Status {
	case golang.FileStatusFailed:
		outcome.file.Status = SemanticFileFailed
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_prerequisite_file_failed", "Phase 2.1 did not produce a parsed source file", source.Path))
		return outcome
	case golang.FileStatusSkipped:
		outcome.file.Status = SemanticFileSkipped
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_prerequisite_file_skipped", "Phase 2.1 skipped this source file", source.Path))
		return outcome
	case golang.FileStatusParsed:
	default:
		outcome.file.Status = SemanticFileFailed
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityError, "semantic_prerequisite_file_invalid", "Phase 2.1 supplied an unknown file status", source.Path))
		return outcome
	}
	if source.SizeBytes > engine.config.MaxSourceFileSize {
		outcome.file.Status = SemanticFileSkipped
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_source_oversized", fmt.Sprintf("source exceeds the configured %d-byte limit", engine.config.MaxSourceFileSize), source.Path))
		return outcome
	}
	if source.ContentDigest == "" {
		outcome.file.Status = SemanticFileFailed
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityError, "semantic_digest_missing", "Phase 2.1 source digest is missing", source.Path))
		return outcome
	}
	if err := ctx.Err(); err != nil {
		return outcome
	}
	data, err := reader.readFile(source.Path, engine.config.MaxSourceFileSize)
	if err != nil {
		outcome.file.Status = SemanticFileFailed
		code, severity := "semantic_source_unreadable", lie.SeverityWarning
		switch {
		case errors.Is(err, ErrSourceOutsideRoot):
			code, severity = "semantic_source_outside_root", lie.SeverityError
		case errors.Is(err, ErrSourceMissing):
			code = "semantic_source_missing"
		case errors.Is(err, ErrSourceOversized):
			code = "semantic_source_oversized"
			outcome.file.Status = SemanticFileSkipped
		}
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(severity, code, err.Error(), source.Path))
		return outcome
	}
	if err := ctx.Err(); err != nil {
		return outcome
	}
	digestBytes := sha256.Sum256(data)
	digest := fmt.Sprintf("sha256:%x", digestBytes)
	if digest != source.ContentDigest {
		outcome.file.Status = SemanticFileStale
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_digest_mismatch", "source digest no longer matches GoLanguageInventory", source.Path))
		return outcome
	}
	outcome.file.Status = SemanticFilePartial
	outcome.file.ContentDigest = digest
	outcome.source = data
	declarations, references, receivers, typeRelations, diagnostics, err := reconcileDeclarations(ctx, data, source, syntaxSymbols)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return outcome
		}
		outcome.file.Status = SemanticFileFailed
		outcome.file.ContentDigest = ""
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_parse_error", err.Error(), source.Path))
		return outcome
	}
	outcome.declarations = declarations
	outcome.references = references
	outcome.receivers = receivers
	outcome.typeRelations = typeRelations
	outcome.diagnostics = append(outcome.diagnostics, diagnostics...)
	return outcome
}

func symbolsByFile(symbols []golang.GoSymbol) map[string][]golang.GoSymbol {
	result := make(map[string][]golang.GoSymbol)
	for _, symbol := range symbols {
		result[symbol.FileID] = append(result[symbol.FileID], symbol)
	}
	for fileID := range result {
		sort.Slice(result[fileID], func(i, j int) bool {
			left, right := result[fileID][i], result[fileID][j]
			if left.Location.Start.Offset != right.Location.Start.Offset {
				return left.Location.Start.Offset < right.Location.Start.Offset
			}
			return left.ID < right.ID
		})
	}
	return result
}

type syntaxDeclarationKey struct {
	kind       golang.SymbolKind
	name       string
	start, end int
}

type lexicalScopeKind uint8

const (
	scopePackage lexicalScopeKind = iota + 1
	scopeFile
	scopeFunction
	scopeBlock
	scopeType
)

type lexicalScope struct {
	kind         lexicalScopeKind
	parent       *lexicalScope
	declarations map[string][]string
}

func (scope *lexicalScope) lookup(name string) []string {
	for current := scope; current != nil; current = current.parent {
		if identifiers := current.declarations[name]; len(identifiers) > 0 {
			return append([]string(nil), identifiers...)
		}
	}
	return nil
}

func (scope *lexicalScope) lookupBeforePackage(name string) []string {
	for current := scope; current != nil && current.kind != scopePackage; current = current.parent {
		if identifiers := current.declarations[name]; len(identifiers) > 0 {
			return append([]string(nil), identifiers...)
		}
	}
	return nil
}

func newLexicalScope(kind lexicalScopeKind, parent *lexicalScope) *lexicalScope {
	return &lexicalScope{kind: kind, parent: parent, declarations: make(map[string][]string)}
}

func (scope *lexicalScope) declare(name, identifier string) {
	if name == "" || name == "_" {
		return
	}
	scope.declarations[name] = append(scope.declarations[name], identifier)
}

func (scope *lexicalScope) hasLocal(name string) bool {
	return len(scope.declarations[name]) > 0
}

func (scope *lexicalScope) nearestFunction() *lexicalScope {
	for current := scope; current != nil; current = current.parent {
		if current.kind == scopeFunction {
			return current
		}
	}
	return scope
}

type receiverCandidate struct {
	methodDeclarationID string
	packageID           string
	receiverName        string
	pointer             bool
	generic             bool
	location            lie.SourceRange
}

type typeRelationCandidate struct {
	kind               TypeRelationKind
	fileID             string
	packageID          string
	ownerDeclarationID string
	location           lie.SourceRange
	targetName         string
	targetIdentity     string
	targetCandidates   []string
	structural         bool
	typeArguments      []string
}

type referenceCandidate struct {
	name                string
	kind                ReferenceKind
	fileID              string
	packageID           string
	ownerDeclarationID  string
	location            lie.SourceRange
	localCandidates     []string
	qualifier           string
	qualifierCandidates []string
	selectorBase        bool
}

func compactAndLimitReferenceCandidates(candidates []referenceCandidate, maximum int) ([]referenceCandidate, int) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.location.File != right.location.File {
			return left.location.File < right.location.File
		}
		if left.location.Start.Offset != right.location.Start.Offset {
			return left.location.Start.Offset < right.location.Start.Offset
		}
		if left.kind != right.kind {
			return left.kind.String() < right.kind.String()
		}
		return left.name < right.name
	})
	result := make([]referenceCandidate, 0, min(maximum, len(candidates)))
	omitted := 0
	for index := 0; index < len(candidates); {
		end := index + 1
		for end < len(candidates) && sameReferenceCandidateIdentity(candidates[index], candidates[end]) {
			end++
		}
		if len(result) < maximum {
			result = append(result, candidates[end-1])
		} else {
			omitted++
		}
		index = end
	}
	return result, omitted
}

func sameReferenceCandidateIdentity(left, right referenceCandidate) bool {
	return left.fileID == right.fileID && left.location.Start.Offset == right.location.Start.Offset && left.kind == right.kind && left.name == right.name
}

type declarationCollector struct {
	ctx                  context.Context
	fileSet              *token.FileSet
	path                 string
	fileID               string
	packageID            string
	syntax               []golang.GoSymbol
	syntaxByKey          map[syntaxDeclarationKey][]golang.GoSymbol
	matched              map[string]bool
	declarations         []SemanticDeclaration
	references           []referenceCandidate
	receivers            []receiverCandidate
	typeRelations        []typeRelationCandidate
	diagnostics          []lie.Diagnostic
	packageScope         *lexicalScope
	fileScope            *lexicalScope
	declarationOffsets   map[int]bool
	typeReferenceOffsets map[int]bool
	selectorOffsets      map[int]bool
	selectorBaseOffsets  map[int]bool
}

func reconcileDeclarations(ctx context.Context, data []byte, source golang.GoFile, syntax []golang.GoSymbol) ([]SemanticDeclaration, []referenceCandidate, []receiverCandidate, []typeRelationCandidate, []lie.Diagnostic, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, source.Path, data, parser.SkipObjectResolution|parser.AllErrors)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	packageScope := newLexicalScope(scopePackage, nil)
	collector := &declarationCollector{
		ctx: ctx, fileSet: fileSet, path: source.Path, fileID: source.ID, packageID: source.PackageID,
		syntax: syntax, syntaxByKey: make(map[syntaxDeclarationKey][]golang.GoSymbol), matched: make(map[string]bool),
		declarations: []SemanticDeclaration{}, references: []referenceCandidate{}, receivers: []receiverCandidate{}, typeRelations: []typeRelationCandidate{}, diagnostics: []lie.Diagnostic{}, packageScope: packageScope,
		fileScope:          newLexicalScope(scopeFile, packageScope),
		declarationOffsets: make(map[int]bool), typeReferenceOffsets: make(map[int]bool), selectorOffsets: make(map[int]bool), selectorBaseOffsets: make(map[int]bool),
	}
	for _, symbol := range syntax {
		key := syntaxDeclarationKey{kind: symbol.Kind, name: symbol.Name, start: symbol.Location.Start.Offset, end: symbol.Location.End.Offset}
		collector.syntaxByKey[key] = append(collector.syntaxByKey[key], symbol)
	}
	for _, declaration := range parsed.Decls {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, nil, nil, err
		}
		collector.collectTopLevel(declaration)
	}
	collector.recordUnmatchedSyntax()
	sort.Slice(collector.declarations, func(i, j int) bool {
		if collector.declarations[i].Location.Start.Offset != collector.declarations[j].Location.Start.Offset {
			return collector.declarations[i].Location.Start.Offset < collector.declarations[j].Location.Start.Offset
		}
		return collector.declarations[i].ID < collector.declarations[j].ID
	})
	sort.Slice(collector.references, func(i, j int) bool {
		if collector.references[i].location.Start.Offset != collector.references[j].location.Start.Offset {
			return collector.references[i].location.Start.Offset < collector.references[j].location.Start.Offset
		}
		if collector.references[i].kind != collector.references[j].kind {
			return collector.references[i].kind < collector.references[j].kind
		}
		return collector.references[i].name < collector.references[j].name
	})
	return collector.declarations, collector.references, collector.receivers, collector.typeRelations, collector.diagnostics, nil
}

func (collector *declarationCollector) collectTopLevel(node ast.Decl) {
	switch declaration := node.(type) {
	case *ast.FuncDecl:
		kind := DeclarationFunction
		syntaxKind := golang.SymbolKindFunction
		if declaration.Recv != nil {
			kind = DeclarationMethod
			syntaxKind = golang.SymbolKindMethod
		}
		location := collector.sourceRange(declaration.Pos(), declaration.End())
		declarationScope := collector.packageScope
		if kind == DeclarationMethod || declaration.Name.Name == "init" {
			declarationScope = collector.fileScope
		}
		semantic := collector.addDeclaration(declaration.Name.Name, kind, collector.render(declaration.Type), location, "", declarationScope, true, syntaxKind)
		functionScope := newLexicalScope(scopeFunction, collector.fileScope)
		if kind == DeclarationMethod {
			collector.collectReceiver(declaration.Recv, functionScope, semantic.ID)
		}
		collector.collectFieldList(declaration.Recv, DeclarationParameter, functionScope, semantic.ID)
		collector.collectTypeParameters(declaration.Type.TypeParams, functionScope, semantic.ID)
		collector.collectFieldList(declaration.Type.Params, DeclarationParameter, functionScope, semantic.ID)
		collector.collectFieldList(declaration.Type.Results, DeclarationResult, functionScope, semantic.ID)
		collector.collectBody(declaration.Body, functionScope, semantic.ID)
	case *ast.GenDecl:
		collector.collectGenDecl(declaration, collector.packageScope, "", true)
	}
}

func (collector *declarationCollector) collectGenDecl(declaration *ast.GenDecl, scope *lexicalScope, owner string, topLevel bool) {
	for _, raw := range declaration.Specs {
		switch specification := raw.(type) {
		case *ast.TypeSpec:
			collector.collectTypeSpec(specification, scope, owner, topLevel)
		case *ast.ValueSpec:
			kind := DeclarationVariable
			syntaxKind := golang.SymbolKindVariable
			if declaration.Tok == token.CONST {
				kind = DeclarationConstant
				syntaxKind = golang.SymbolKindConstant
			} else if declaration.Tok != token.VAR {
				continue
			}
			typeDisplay := collector.render(specification.Type)
			owners := make([]string, 0, len(specification.Names))
			for _, name := range specification.Names {
				location := collector.sourceRange(name.Pos(), name.End())
				collector.collectTypeUses(specification.Type, scope, semanticDeclarationID(collector.path, location.Start.Offset, kind, name.Name))
				semantic := collector.addDeclaration(name.Name, kind, typeDisplay, location, owner, scope, topLevel, syntaxKind)
				owners = append(owners, semantic.ID)
			}
			for index, value := range specification.Values {
				valueOwner := owner
				if len(owners) > 0 {
					valueOwner = owners[min(index, len(owners)-1)]
				}
				collector.collectExpressionReferences(value, scope, valueOwner)
			}
		}
	}
}

func (collector *declarationCollector) collectTypeSpec(specification *ast.TypeSpec, scope *lexicalScope, owner string, topLevel bool) {
	kind := DeclarationDefinedType
	syntaxKind := golang.SymbolKind(0)
	reconcile := false
	if specification.Assign.IsValid() {
		kind = DeclarationTypeAlias
	}
	switch specification.Type.(type) {
	case *ast.StructType:
		syntaxKind, reconcile = golang.SymbolKindStruct, topLevel
		if kind != DeclarationTypeAlias {
			kind = DeclarationStruct
		}
	case *ast.InterfaceType:
		syntaxKind, reconcile = golang.SymbolKindInterface, topLevel
		if kind != DeclarationTypeAlias {
			kind = DeclarationInterface
		}
	}
	location := collector.sourceRange(specification.Pos(), specification.End())
	semantic := collector.addDeclaration(specification.Name.Name, kind, collector.render(specification.Type), location, owner, scope, reconcile, syntaxKind)
	typeScope := newLexicalScope(scopeType, scope)
	collector.collectTypeParameters(specification.TypeParams, typeScope, semantic.ID)
	if kind == DeclarationTypeAlias {
		collector.addPrimaryTypeRelation(TypeRelationAliasOf, specification.Type, typeScope, semantic.ID)
		collector.collectTypeUses(specification.Type, typeScope, semantic.ID)
	} else if kind == DeclarationDefinedType {
		collector.collectTypeUses(specification.Type, typeScope, semantic.ID)
	}
	switch underlying := specification.Type.(type) {
	case *ast.StructType:
		collector.collectStructFields(underlying.Fields, typeScope, semantic.ID)
	case *ast.InterfaceType:
		collector.collectInterfaceMethods(underlying.Methods, typeScope, semantic.ID)
	}
}

func (collector *declarationCollector) collectStructFields(fields *ast.FieldList, scope *lexicalScope, owner string) {
	if fields == nil {
		return
	}
	memberScope := newLexicalScope(scopeType, scope)
	for _, field := range fields.List {
		typeDisplay := collector.render(field.Type)
		if len(field.Names) == 0 {
			name := embeddedFieldName(field.Type)
			if name != "" {
				declaration := collector.addDeclaration(name, DeclarationField, typeDisplay, collector.sourceRange(field.Type.Pos(), field.Type.End()), owner, memberScope, false, 0)
				collector.collectTypeUses(field.Type, scope, declaration.ID)
				collector.addPrimaryTypeRelation(TypeRelationEmbeds, field.Type, scope, owner)
			}
			continue
		}
		for _, name := range field.Names {
			if name.Name != "_" {
				declaration := collector.addDeclaration(name.Name, DeclarationField, typeDisplay, collector.sourceRange(name.Pos(), name.End()), owner, memberScope, false, 0)
				collector.collectTypeUses(field.Type, scope, declaration.ID)
			}
		}
	}
}

func (collector *declarationCollector) collectInterfaceMethods(fields *ast.FieldList, scope *lexicalScope, owner string) {
	if fields == nil {
		return
	}
	memberScope := newLexicalScope(scopeType, scope)
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			collector.addPrimaryTypeRelation(interfaceElementRelationKind(field.Type), field.Type, scope, owner)
			collector.collectTypeUses(field.Type, scope, owner)
			continue
		}
		kind := DeclarationField
		functionType, isMethod := field.Type.(*ast.FuncType)
		if isMethod {
			kind = DeclarationMethod
		}
		for _, name := range field.Names {
			if name.Name != "_" {
				declaration := collector.addDeclaration(name.Name, kind, collector.render(field.Type), collector.sourceRange(name.Pos(), name.End()), owner, memberScope, false, 0)
				if isMethod {
					methodScope := newLexicalScope(scopeFunction, scope)
					collector.collectFieldList(functionType.Params, DeclarationParameter, methodScope, declaration.ID)
					collector.collectFieldList(functionType.Results, DeclarationResult, methodScope, declaration.ID)
				} else {
					collector.collectTypeUses(field.Type, scope, declaration.ID)
				}
			}
		}
	}
}

func interfaceElementRelationKind(expression ast.Expr) TypeRelationKind {
	for {
		switch typed := expression.(type) {
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.Ident:
			if isPredeclaredType(typed.Name) {
				return TypeRelationConstrains
			}
			return TypeRelationEmbeds
		case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
			return TypeRelationEmbeds
		default:
			return TypeRelationConstrains
		}
	}
}

func (collector *declarationCollector) collectFieldList(fields *ast.FieldList, kind DeclarationKind, scope *lexicalScope, owner string) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		typeDisplay := collector.render(field.Type)
		if len(field.Names) == 0 {
			collector.collectTypeUses(field.Type, scope, owner)
		}
		for _, name := range field.Names {
			if name.Name != "_" {
				location := collector.sourceRange(name.Pos(), name.End())
				collector.collectTypeUses(field.Type, scope, semanticDeclarationID(collector.path, location.Start.Offset, kind, name.Name))
				collector.addDeclaration(name.Name, kind, typeDisplay, location, owner, scope, false, 0)
			}
		}
	}
}

func (collector *declarationCollector) collectTypeParameters(fields *ast.FieldList, scope *lexicalScope, owner string) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			if name.Name == "_" {
				continue
			}
			declaration := collector.addDeclaration(name.Name, DeclarationTypeParameter, collector.render(field.Type), collector.sourceRange(name.Pos(), name.End()), owner, scope, false, 0)
			collector.addPrimaryTypeRelation(TypeRelationConstrains, field.Type, scope, declaration.ID)
			collector.collectTypeUses(field.Type, scope, declaration.ID)
		}
	}
}

func (collector *declarationCollector) collectReceiver(fields *ast.FieldList, scope *lexicalScope, methodID string) {
	if fields == nil || len(fields.List) == 0 {
		collector.receivers = append(collector.receivers, receiverCandidate{methodDeclarationID: methodID, packageID: collector.packageID, location: lie.SourceRange{File: collector.path}})
		return
	}
	expression := fields.List[0].Type
	name, pointer, generic := receiverTypeDetails(expression)
	collector.receivers = append(collector.receivers, receiverCandidate{
		methodDeclarationID: methodID, packageID: collector.packageID, receiverName: name,
		pointer: pointer, generic: generic, location: collector.sourceRange(expression.Pos(), expression.End()),
	})
	for _, identifier := range receiverTypeParameterNames(expression) {
		if !scope.hasLocal(identifier.Name) {
			collector.addDeclaration(identifier.Name, DeclarationTypeParameter, "", collector.sourceRange(identifier.Pos(), identifier.End()), methodID, scope, false, 0)
		}
	}
}

func receiverTypeDetails(expression ast.Expr) (name string, pointer, generic bool) {
	for expression != nil {
		switch typed := expression.(type) {
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.StarExpr:
			pointer = true
			expression = typed.X
		case *ast.IndexExpr:
			generic = true
			expression = typed.X
		case *ast.IndexListExpr:
			generic = true
			expression = typed.X
		case *ast.Ident:
			return typed.Name, pointer, generic
		default:
			return "", pointer, generic
		}
	}
	return "", pointer, generic
}

func receiverTypeParameterNames(expression ast.Expr) []*ast.Ident {
	for {
		switch typed := expression.(type) {
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.StarExpr:
			expression = typed.X
		case *ast.IndexExpr:
			if identifier, ok := typed.Index.(*ast.Ident); ok {
				return []*ast.Ident{identifier}
			}
			return nil
		case *ast.IndexListExpr:
			result := make([]*ast.Ident, 0, len(typed.Indices))
			for _, index := range typed.Indices {
				identifier, ok := index.(*ast.Ident)
				if !ok {
					return nil
				}
				result = append(result, identifier)
			}
			return result
		default:
			return nil
		}
	}
}

func (collector *declarationCollector) addPrimaryTypeRelation(kind TypeRelationKind, expression ast.Expr, scope *lexicalScope, owner string) {
	if expression == nil {
		return
	}
	candidate := collector.typeRelationCandidate(kind, expression, scope, owner)
	collector.typeRelations = append(collector.typeRelations, candidate)
}

func (collector *declarationCollector) collectTypeUses(expression ast.Expr, scope *lexicalScope, owner string) {
	if expression == nil {
		return
	}
	walkTypeExpression(expression, func(kind TypeRelationKind, current ast.Expr) {
		collector.typeRelations = append(collector.typeRelations, collector.typeRelationCandidate(kind, current, scope, owner))
		collector.addTypeReference(kind, current, scope, owner)
	})
}

func (collector *declarationCollector) addTypeReference(relationKind TypeRelationKind, expression ast.Expr, scope *lexicalScope, owner string) {
	kind := ReferenceType
	if relationKind == TypeRelationInstantiates {
		kind = ReferenceInstantiation
	}
	for {
		switch typed := expression.(type) {
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.StarExpr:
			expression = typed.X
		case *ast.IndexExpr:
			expression = typed.X
		case *ast.IndexListExpr:
			expression = typed.X
		case *ast.Ident:
			collector.typeReferenceOffsets[collector.fileSet.Position(typed.Pos()).Offset] = true
			collector.addReference(typed.Name, kind, typed.Pos(), typed.End(), scope, owner, "")
			return
		case *ast.SelectorExpr:
			qualifier := ""
			if identifier, ok := typed.X.(*ast.Ident); ok {
				qualifier = identifier.Name
				collector.selectorBaseOffsets[collector.fileSet.Position(identifier.Pos()).Offset] = true
			}
			offset := collector.fileSet.Position(typed.Sel.Pos()).Offset
			collector.typeReferenceOffsets[offset] = true
			collector.selectorOffsets[offset] = true
			collector.addReference(typed.Sel.Name, kind, typed.Sel.Pos(), typed.Sel.End(), scope, owner, qualifier)
			return
		default:
			return
		}
	}
}

func (collector *declarationCollector) typeRelationCandidate(kind TypeRelationKind, expression ast.Expr, scope *lexicalScope, owner string) typeRelationCandidate {
	targetName, targetIdentity, structural, arguments := collector.typeTarget(expression)
	return typeRelationCandidate{
		kind: kind, fileID: collector.fileID, packageID: collector.packageID, ownerDeclarationID: owner,
		location: collector.sourceRange(expression.Pos(), expression.End()), targetName: targetName,
		targetIdentity: targetIdentity, targetCandidates: scope.lookup(targetName), structural: structural, typeArguments: arguments,
	}
}

func (collector *declarationCollector) typeTarget(expression ast.Expr) (name, identity string, structural bool, arguments []string) {
	for {
		switch typed := expression.(type) {
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.StarExpr:
			expression = typed.X
		case *ast.IndexExpr:
			arguments = []string{collector.render(typed.Index)}
			expression = typed.X
		case *ast.IndexListExpr:
			arguments = make([]string, 0, len(typed.Indices))
			for _, argument := range typed.Indices {
				arguments = append(arguments, collector.render(argument))
			}
			expression = typed.X
		case *ast.Ident:
			return typed.Name, typed.Name, false, arguments
		case *ast.SelectorExpr:
			return "", collector.render(typed), false, arguments
		default:
			return "", "type:" + collector.render(expression), true, arguments
		}
	}
}

func walkTypeExpression(expression ast.Expr, visit func(TypeRelationKind, ast.Expr)) {
	if expression == nil {
		return
	}
	switch typed := expression.(type) {
	case *ast.Ident, *ast.SelectorExpr:
		visit(TypeRelationUses, expression)
	case *ast.ParenExpr:
		walkTypeExpression(typed.X, visit)
	case *ast.StarExpr:
		walkTypeExpression(typed.X, visit)
	case *ast.ArrayType:
		walkTypeExpression(typed.Elt, visit)
	case *ast.MapType:
		walkTypeExpression(typed.Key, visit)
		walkTypeExpression(typed.Value, visit)
	case *ast.ChanType:
		walkTypeExpression(typed.Value, visit)
	case *ast.Ellipsis:
		walkTypeExpression(typed.Elt, visit)
	case *ast.IndexExpr:
		visit(TypeRelationInstantiates, expression)
		walkTypeExpression(typed.X, visit)
		walkTypeExpression(typed.Index, visit)
	case *ast.IndexListExpr:
		visit(TypeRelationInstantiates, expression)
		walkTypeExpression(typed.X, visit)
		for _, index := range typed.Indices {
			walkTypeExpression(index, visit)
		}
	case *ast.StructType:
		for _, field := range typed.Fields.List {
			walkTypeExpression(field.Type, visit)
		}
	case *ast.InterfaceType:
		for _, field := range typed.Methods.List {
			walkTypeExpression(field.Type, visit)
		}
	case *ast.FuncType:
		walkFieldTypes(typed.TypeParams, visit)
		walkFieldTypes(typed.Params, visit)
		walkFieldTypes(typed.Results, visit)
	case *ast.UnaryExpr:
		walkTypeExpression(typed.X, visit)
	case *ast.BinaryExpr:
		walkTypeExpression(typed.X, visit)
		walkTypeExpression(typed.Y, visit)
	}
}

func walkFieldTypes(fields *ast.FieldList, visit func(TypeRelationKind, ast.Expr)) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		walkTypeExpression(field.Type, visit)
	}
}

func (collector *declarationCollector) collectBody(body *ast.BlockStmt, initialScope *lexicalScope, owner string) {
	if body == nil {
		return
	}
	current := initialScope
	markers := make([]bool, 0, 32)
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			if len(markers) == 0 {
				return true
			}
			pushed := markers[len(markers)-1]
			markers = markers[:len(markers)-1]
			if pushed {
				current = current.parent
			}
			return true
		}
		pushed := false
		switch typed := node.(type) {
		case *ast.BlockStmt:
			if typed != body {
				current = newLexicalScope(scopeBlock, current)
				pushed = true
			}
		case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.CaseClause, *ast.CommClause:
			current = newLexicalScope(scopeBlock, current)
			pushed = true
		case *ast.FuncLit:
			current = newLexicalScope(scopeFunction, current)
			pushed = true
			collector.collectFieldList(typed.Type.Params, DeclarationParameter, current, owner)
			collector.collectFieldList(typed.Type.Results, DeclarationResult, current, owner)
		case *ast.DeclStmt:
			if declaration, ok := typed.Decl.(*ast.GenDecl); ok {
				collector.collectGenDecl(declaration, current, owner, false)
			}
		case *ast.AssignStmt:
			if typed.Tok == token.DEFINE {
				collector.collectShortDeclarations(typed.Lhs, current, owner)
			}
		case *ast.RangeStmt:
			current = newLexicalScope(scopeBlock, current)
			pushed = true
			if typed.Tok == token.DEFINE {
				collector.collectShortDeclarations([]ast.Expr{typed.Key, typed.Value}, current, owner)
			}
		case *ast.LabeledStmt:
			labelScope := current.nearestFunction()
			collector.addDeclaration(typed.Label.Name, DeclarationLabel, "", collector.sourceRange(typed.Label.Pos(), typed.Label.End()), owner, labelScope, false, 0)
		case *ast.SelectorExpr:
			qualifier := ""
			if identifier, ok := typed.X.(*ast.Ident); ok {
				qualifier = identifier.Name
			}
			offset := collector.fileSet.Position(typed.Sel.Pos()).Offset
			collector.selectorOffsets[offset] = true
			if !collector.typeReferenceOffsets[offset] {
				collector.addReference(typed.Sel.Name, ReferenceSelector, typed.Sel.Pos(), typed.Sel.End(), current, owner, qualifier)
			}
		case *ast.Ident:
			offset := collector.fileSet.Position(typed.Pos()).Offset
			if typed.Name != "_" && !collector.declarationOffsets[offset] && !collector.selectorOffsets[offset] && !collector.typeReferenceOffsets[offset] {
				collector.addReference(typed.Name, ReferenceIdentifier, typed.Pos(), typed.End(), current, owner, "")
			}
		}
		markers = append(markers, pushed)
		return collector.ctx.Err() == nil
	})
}

func (collector *declarationCollector) collectExpressionReferences(expression ast.Expr, scope *lexicalScope, owner string) {
	if expression == nil {
		return
	}
	selectorOffsets := make(map[int]bool)
	ast.Inspect(expression, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.SelectorExpr:
			qualifier := ""
			if identifier, ok := typed.X.(*ast.Ident); ok {
				qualifier = identifier.Name
				collector.selectorBaseOffsets[collector.fileSet.Position(identifier.Pos()).Offset] = true
			}
			offset := collector.fileSet.Position(typed.Sel.Pos()).Offset
			selectorOffsets[offset] = true
			collector.addReference(typed.Sel.Name, ReferenceSelector, typed.Sel.Pos(), typed.Sel.End(), scope, owner, qualifier)
		case *ast.Ident:
			offset := collector.fileSet.Position(typed.Pos()).Offset
			if typed.Name != "_" && !selectorOffsets[offset] {
				collector.addReference(typed.Name, ReferenceIdentifier, typed.Pos(), typed.End(), scope, owner, "")
			}
		}
		return collector.ctx.Err() == nil
	})
}

func (collector *declarationCollector) addReference(name string, kind ReferenceKind, start, end token.Pos, scope *lexicalScope, owner, qualifier string) {
	collector.references = append(collector.references, referenceCandidate{
		name: name, kind: kind, fileID: collector.fileID, packageID: collector.packageID,
		ownerDeclarationID: owner, location: collector.sourceRange(start, end),
		localCandidates: scope.lookupBeforePackage(name), qualifier: qualifier,
		qualifierCandidates: scope.lookupBeforePackage(qualifier),
		selectorBase:        collector.selectorBaseOffsets[collector.fileSet.Position(start).Offset],
	})
}

func (collector *declarationCollector) collectShortDeclarations(expressions []ast.Expr, scope *lexicalScope, owner string) {
	for _, expression := range expressions {
		identifier, ok := expression.(*ast.Ident)
		if !ok || identifier.Name == "_" || scope.hasLocal(identifier.Name) {
			continue
		}
		collector.addDeclaration(identifier.Name, DeclarationVariable, "", collector.sourceRange(identifier.Pos(), identifier.End()), owner, scope, false, 0)
	}
}

func (collector *declarationCollector) addDeclaration(name string, kind DeclarationKind, typeDisplay string, location lie.SourceRange, owner string, scope *lexicalScope, reconcile bool, syntaxKind golang.SymbolKind) SemanticDeclaration {
	declaration := SemanticDeclaration{
		ID: semanticDeclarationID(collector.path, location.Start.Offset, kind, name), Name: name,
		FileID: collector.fileID, PackageID: collector.packageID, OwnerDeclarationID: owner,
		Kind: kind, TypeDisplay: typeDisplay, Location: location, Status: ResolutionResolved,
	}
	if reconcile {
		key := syntaxDeclarationKey{kind: syntaxKind, name: name, start: location.Start.Offset, end: location.End.Offset}
		matches := collector.syntaxByKey[key]
		switch len(matches) {
		case 1:
			declaration.SyntaxSymbolID = matches[0].ID
			collector.matched[matches[0].ID] = true
		case 0:
			declaration.Status = ResolutionPartial
			collector.diagnostics = append(collector.diagnostics, declarationDiagnostic(lie.SeverityWarning, "semantic_declaration_unmatched", fmt.Sprintf("%s %s does not match a Phase 2.1 syntax symbol", kind.String(), name), location))
		default:
			declaration.Status = ResolutionAmbiguous
			collector.diagnostics = append(collector.diagnostics, declarationDiagnostic(lie.SeverityWarning, "semantic_declaration_ambiguous", fmt.Sprintf("%s %s matches multiple Phase 2.1 syntax symbols", kind.String(), name), location))
		}
	}
	collector.declarations = append(collector.declarations, declaration)
	collector.declarationOffsets[location.Start.Offset] = true
	if name != "_" {
		scope.declare(name, declaration.ID)
	}
	return declaration
}

func (collector *declarationCollector) recordUnmatchedSyntax() {
	for _, symbol := range collector.syntax {
		if collector.matched[symbol.ID] {
			continue
		}
		collector.diagnostics = append(collector.diagnostics, declarationDiagnostic(lie.SeverityWarning, "semantic_syntax_symbol_unmatched", fmt.Sprintf("Phase 2.1 symbol %s was not reconciled", symbol.ID), symbol.Location))
	}
}

func (collector *declarationCollector) sourceRange(start, end token.Pos) lie.SourceRange {
	left, right := collector.fileSet.Position(start), collector.fileSet.Position(end)
	return lie.SourceRange{File: collector.path, Start: lie.Position{Offset: left.Offset, Line: left.Line, Column: left.Column}, End: lie.Position{Offset: right.Offset, Line: right.Line, Column: right.Column}}
}

func (collector *declarationCollector) render(node ast.Node) string {
	if node == nil {
		return ""
	}
	var buffer bytes.Buffer
	if err := format.Node(&buffer, collector.fileSet, node); err != nil {
		return ""
	}
	return buffer.String()
}

func embeddedFieldName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	case *ast.StarExpr:
		return embeddedFieldName(typed.X)
	case *ast.IndexExpr:
		return embeddedFieldName(typed.X)
	case *ast.IndexListExpr:
		return embeddedFieldName(typed.X)
	default:
		return ""
	}
}

func semanticDeclarationID(file string, offset int, kind DeclarationKind, name string) string {
	return fmt.Sprintf("go:semantic:v1:file:%d:%s#%d:%s:%s", len(file), file, offset, kind.String(), name)
}

func declarationDiagnostic(severity lie.Severity, code, message string, location lie.SourceRange) lie.Diagnostic {
	return lie.Diagnostic{Engine: "go-semantic", Severity: severity, Code: code, Message: message, Location: &location}
}

func collectOutcomes(outcomes []fileOutcome) ([]SemanticFile, []SemanticDeclaration, []referenceCandidate, []ReceiverBinding, []typeRelationCandidate, []lie.Diagnostic, SemanticStatistics) {
	sort.Slice(outcomes, func(i, j int) bool {
		if outcomes[i].path != outcomes[j].path {
			return outcomes[i].path < outcomes[j].path
		}
		return outcomes[i].file.FileID < outcomes[j].file.FileID
	})
	files := make([]SemanticFile, 0, len(outcomes))
	declarations := make([]SemanticDeclaration, 0)
	referenceCandidates := make([]referenceCandidate, 0)
	receiverCandidates := make([]receiverCandidate, 0)
	typeRelationCandidates := make([]typeRelationCandidate, 0)
	diagnostics := make([]lie.Diagnostic, 0)
	statistics := emptyStatistics()
	statistics.CandidateFiles = len(outcomes)
	for _, outcome := range outcomes {
		files = append(files, outcome.file)
		declarations = append(declarations, outcome.declarations...)
		referenceCandidates = append(referenceCandidates, outcome.references...)
		receiverCandidates = append(receiverCandidates, outcome.receivers...)
		typeRelationCandidates = append(typeRelationCandidates, outcome.typeRelations...)
		diagnostics = append(diagnostics, outcome.diagnostics...)
		switch outcome.file.Status {
		case SemanticFileResolved:
			statistics.ResolvedFiles++
		case SemanticFilePartial:
			statistics.PartialFiles++
		case SemanticFileFailed:
			statistics.FailedFiles++
		case SemanticFileStale:
			statistics.StaleFiles++
		case SemanticFileSkipped:
			statistics.SkippedFiles++
		}
	}
	declarations, scopeDiagnostics := reconcilePackageScopes(declarations)
	diagnostics = append(diagnostics, scopeDiagnostics...)
	receivers, receiverDiagnostics := bindReceivers(declarations, receiverCandidates)
	diagnostics = append(diagnostics, receiverDiagnostics...)
	sort.Slice(declarations, func(i, j int) bool {
		if declarations[i].Location.File != declarations[j].Location.File {
			return declarations[i].Location.File < declarations[j].Location.File
		}
		if declarations[i].Location.Start.Offset != declarations[j].Location.Start.Offset {
			return declarations[i].Location.Start.Offset < declarations[j].Location.Start.Offset
		}
		return declarations[i].ID < declarations[j].ID
	})
	for _, declaration := range declarations {
		switch declaration.Status {
		case ResolutionResolved:
			statistics.ResolvedDeclarations++
		case ResolutionUnresolved:
			statistics.UnresolvedDeclarations++
		case ResolutionAmbiguous:
			statistics.AmbiguousDeclarations++
		case ResolutionExternal:
			statistics.ExternalDeclarations++
		case ResolutionPartial:
			statistics.PartialDeclarations++
		}
	}
	statistics.ReceiverBindings = len(receivers)
	return files, declarations, referenceCandidates, receivers, typeRelationCandidates, diagnostics, statistics
}

func reconcilePackageScopes(declarations []SemanticDeclaration) ([]SemanticDeclaration, []lie.Diagnostic) {
	packageScopes := make(map[string]*lexicalScope)
	indices := make(map[string][]int)
	for index, declaration := range declarations {
		if declaration.Name == "_" || declaration.OwnerDeclarationID != "" || declaration.Kind == DeclarationMethod || (declaration.Kind == DeclarationFunction && declaration.Name == "init") {
			continue
		}
		scope := packageScopes[declaration.PackageID]
		if scope == nil {
			scope = newLexicalScope(scopePackage, nil)
			packageScopes[declaration.PackageID] = scope
		}
		scope.declare(declaration.Name, declaration.ID)
		key := declaration.PackageID + "\x00" + declaration.Name
		indices[key] = append(indices[key], index)
	}
	diagnostics := make([]lie.Diagnostic, 0)
	for key, matches := range indices {
		if len(matches) < 2 {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		for _, index := range matches {
			declarations[index].Status = ResolutionAmbiguous
		}
		first := declarations[matches[0]]
		diagnostics = append(diagnostics, declarationDiagnostic(lie.SeverityWarning, "semantic_package_scope_conflict", fmt.Sprintf("package %s declares %s %d times", parts[0], parts[1], len(matches)), first.Location))
	}
	return declarations, diagnostics
}

func bindReceivers(declarations []SemanticDeclaration, candidates []receiverCandidate) ([]ReceiverBinding, []lie.Diagnostic) {
	types := typeDeclarationsByPackageName(declarations)
	bindings := make([]ReceiverBinding, 0, len(candidates))
	diagnostics := make([]lie.Diagnostic, 0)
	for _, candidate := range candidates {
		binding := ReceiverBinding{
			ID: receiverBindingID(candidate), MethodDeclarationID: candidate.methodDeclarationID,
			ReceiverName: candidate.receiverName, Pointer: candidate.pointer, Generic: candidate.generic,
			Location: candidate.location, Status: ResolutionUnresolved,
		}
		matches := types[candidate.packageID+"\x00"+candidate.receiverName]
		valid := make([]SemanticDeclaration, 0, len(matches))
		for _, declaration := range matches {
			if declaration.Kind == DeclarationStruct || declaration.Kind == DeclarationDefinedType {
				valid = append(valid, declaration)
			}
		}
		switch len(valid) {
		case 1:
			if valid[0].Status == ResolutionResolved {
				binding.Status = ResolutionResolved
				binding.ReceiverTypeDeclarationID = valid[0].ID
			} else {
				binding.Status = ResolutionAmbiguous
			}
		case 0:
			binding.Status = ResolutionUnresolved
		default:
			binding.Status = ResolutionAmbiguous
		}
		if binding.Status != ResolutionResolved {
			code := "semantic_receiver_unresolved"
			if binding.Status == ResolutionAmbiguous {
				code = "semantic_receiver_ambiguous"
			}
			diagnostics = append(diagnostics, declarationDiagnostic(lie.SeverityWarning, code, fmt.Sprintf("receiver %s for method %s is %s", candidate.receiverName, candidate.methodDeclarationID, binding.Status.String()), candidate.location))
		}
		bindings = append(bindings, binding)
	}
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].MethodDeclarationID != bindings[j].MethodDeclarationID {
			return bindings[i].MethodDeclarationID < bindings[j].MethodDeclarationID
		}
		return bindings[i].ID < bindings[j].ID
	})
	return bindings, diagnostics
}

func bindImports(ctx context.Context, reader *sourceReader, syntaxFiles []golang.GoFile, semanticFiles []SemanticFile, packages []golang.GoPackage, proofs []packageidentity.PackageIdentityProof, maximumBytes int64) ([]ImportBinding, []lie.Diagnostic, error) {
	fileStatuses := make(map[string]SemanticFileStatus, len(semanticFiles))
	for _, file := range semanticFiles {
		fileStatuses[file.FileID] = file.Status
	}
	packageNames := make(map[string]string, len(packages))
	for _, pkg := range packages {
		packageNames[pkg.ID] = pkg.Name
	}
	proofsByImport := make(map[string][]packageidentity.PackageIdentityProof)
	for _, proof := range proofs {
		key := proof.ImportingPackageID + "\x00" + proof.ImportPath
		proofsByImport[key] = append(proofsByImport[key], proof)
	}
	for key := range proofsByImport {
		sort.Slice(proofsByImport[key], func(i, j int) bool { return proofsByImport[key][i].ID < proofsByImport[key][j].ID })
	}
	freshness := make(map[string]bool)
	bindings := make([]ImportBinding, 0)
	diagnostics := make([]lie.Diagnostic, 0)
	for _, file := range syntaxFiles {
		semanticStatus := fileStatuses[file.ID]
		if file.Status != golang.FileStatusParsed || (semanticStatus != SemanticFilePartial && semanticStatus != SemanticFileResolved) {
			continue
		}
		for _, source := range file.Imports {
			if len(bindings)%256 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, nil, err
				}
			}
			binding := ImportBinding{
				ID: importBindingID(file.ID, source), FileID: file.ID, ImportPath: source.Path,
				AliasKind: source.AliasKind.String(), Location: source.Location, Status: ResolutionUnresolved,
			}
			switch source.AliasKind {
			case golang.ImportAliasNamed:
				binding.LocalName = source.Alias
			case golang.ImportAliasBlank:
				binding.LocalName = "_"
			case golang.ImportAliasDot:
				binding.LocalName = "."
			}
			matching := proofsByImport[file.PackageID+"\x00"+source.Path]
			status, target, proofID, stale := combinePackageProofs(reader, matching, freshness, maximumBytes)
			binding.Status, binding.TargetPackageID, binding.PackageIdentityProofID = status, target, proofID
			if source.AliasKind == golang.ImportAliasDefault && status == ResolutionResolved {
				binding.LocalName = packageNames[target]
			}
			if stale {
				diagnostics = append(diagnostics, declarationDiagnostic(lie.SeverityWarning, "semantic_package_proof_stale", fmt.Sprintf("manifest evidence for import %s no longer matches the package-identity proof", source.Path), source.Location))
			}
			bindings = append(bindings, binding)
		}
	}
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].Location.File != bindings[j].Location.File {
			return bindings[i].Location.File < bindings[j].Location.File
		}
		if bindings[i].Location.Start.Offset != bindings[j].Location.Start.Offset {
			return bindings[i].Location.Start.Offset < bindings[j].Location.Start.Offset
		}
		return bindings[i].ID < bindings[j].ID
	})
	return bindings, diagnostics, nil
}

func combinePackageProofs(reader *sourceReader, proofs []packageidentity.PackageIdentityProof, freshness map[string]bool, maximumBytes int64) (ResolutionStatus, string, string, bool) {
	if len(proofs) == 0 {
		return ResolutionUnresolved, "", "", false
	}
	usable := make([]packageidentity.PackageIdentityProof, 0, len(proofs))
	stale := false
	for _, proof := range proofs {
		fresh, known := freshness[proof.ID]
		if !known {
			fresh = proofEvidenceFresh(reader, proof, maximumBytes)
			freshness[proof.ID] = fresh
		}
		if !fresh || proof.Status == packageidentity.ProofStale {
			stale = true
			continue
		}
		usable = append(usable, proof)
	}
	if stale || len(usable) == 0 {
		return ResolutionUnresolved, "", "", stale
	}
	targets := make(map[string]struct{})
	allResolved, allExternal, ambiguous := true, true, false
	for _, proof := range usable {
		switch proof.Status {
		case packageidentity.ProofResolved:
			allExternal = false
			if proof.TargetPackageID == "" {
				allResolved = false
			} else {
				targets[proof.TargetPackageID] = struct{}{}
			}
		case packageidentity.ProofExternal:
			allResolved = false
		case packageidentity.ProofAmbiguous:
			allResolved, allExternal, ambiguous = false, false, true
		default:
			allResolved, allExternal = false, false
		}
	}
	canonicalProof := usable[0].ID
	if ambiguous || len(targets) > 1 {
		return ResolutionAmbiguous, "", "", false
	}
	if allResolved && len(targets) == 1 {
		for target := range targets {
			return ResolutionResolved, target, canonicalProof, false
		}
	}
	if allExternal {
		return ResolutionExternal, "", canonicalProof, false
	}
	return ResolutionUnresolved, "", "", false
}

func proofEvidenceFresh(reader *sourceReader, proof packageidentity.PackageIdentityProof, maximumBytes int64) bool {
	for _, evidence := range proof.Evidence {
		if evidence.File == "" {
			continue
		}
		data, err := reader.readFile(evidence.File, maximumBytes)
		if err != nil {
			return false
		}
		digest := sha256.Sum256(data)
		if fmt.Sprintf("sha256:%x", digest) != evidence.ContentDigest {
			return false
		}
	}
	return true
}

func bindReferences(ctx context.Context, declarations []SemanticDeclaration, candidates []referenceCandidate, imports []ImportBinding, maximum int) ([]SemanticReference, int, error) {
	byID := make(map[string]SemanticDeclaration, len(declarations))
	packageDeclarations := make(map[string][]SemanticDeclaration)
	for _, declaration := range declarations {
		byID[declaration.ID] = declaration
		if declaration.OwnerDeclarationID == "" && declaration.Kind != DeclarationMethod && !(declaration.Kind == DeclarationFunction && declaration.Name == "init") {
			key := declaration.PackageID + "\x00" + declaration.Name
			packageDeclarations[key] = append(packageDeclarations[key], declaration)
		}
	}
	importsByAlias := importsByFileAndAlias(imports)
	dotImports := make(map[string][]ImportBinding)
	for _, binding := range imports {
		if binding.AliasKind == golang.ImportAliasDot.String() {
			dotImports[binding.FileID] = append(dotImports[binding.FileID], binding)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.location.File != right.location.File {
			return left.location.File < right.location.File
		}
		if left.location.Start.Offset != right.location.Start.Offset {
			return left.location.Start.Offset < right.location.Start.Offset
		}
		if left.kind != right.kind {
			return left.kind.String() < right.kind.String()
		}
		return left.name < right.name
	})
	result := make([]SemanticReference, 0, min(maximum, len(candidates)))
	indices := make(map[string]int, min(maximum, len(candidates)))
	omitted := 0
	for index, candidate := range candidates {
		if index%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
		}
		if candidate.selectorBase && len(candidate.localCandidates) == 0 {
			if _, imported := importsByAlias[candidate.fileID+"\x00"+candidate.name]; imported {
				continue
			}
		}
		reference := SemanticReference{
			Name: candidate.name, Kind: candidate.kind, FileID: candidate.fileID, PackageID: candidate.packageID,
			OwnerDeclarationID: candidate.ownerDeclarationID, Location: candidate.location, Status: ResolutionUnresolved,
		}
		if candidate.qualifier != "" && len(candidate.qualifierCandidates) == 0 {
			if binding, exists := importsByAlias[candidate.fileID+"\x00"+candidate.qualifier]; exists {
				reference.Status, reference.TargetDeclarationID, reference.ExternalIdentity = resolveImportedName(binding, candidate.name, packageDeclarations)
			} else {
				reference.Status = ResolutionUnresolved
			}
		} else if len(candidate.localCandidates) > 0 {
			applyDeclarationCandidates(&reference, declarationsForIDs(candidate.localCandidates, byID))
		} else {
			applyDeclarationCandidates(&reference, packageDeclarations[candidate.packageID+"\x00"+candidate.name])
			if reference.Status == ResolutionUnresolved {
				applyDotImportCandidates(&reference, dotImports[candidate.fileID], candidate.name, packageDeclarations)
			}
			if reference.Status == ResolutionUnresolved && isPredeclaredIdentifier(candidate.name) {
				reference.Status = ResolutionExternal
				reference.ExternalIdentity = "builtin:" + candidate.name
			}
		}
		reference.ID = semanticReferenceID(reference)
		if previous, exists := indices[reference.ID]; exists {
			if previous >= 0 {
				result[previous] = reference
			}
			continue
		}
		if len(result) < maximum {
			indices[reference.ID] = len(result)
			result = append(result, reference)
		} else {
			indices[reference.ID] = -1
			omitted++
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Location.File != result[j].Location.File {
			return result[i].Location.File < result[j].Location.File
		}
		if result[i].Location.Start.Offset != result[j].Location.Start.Offset {
			return result[i].Location.Start.Offset < result[j].Location.Start.Offset
		}
		return result[i].ID < result[j].ID
	})
	return result, omitted, nil
}

func importsByFileAndAlias(imports []ImportBinding) map[string]ImportBinding {
	result := make(map[string]ImportBinding)
	for _, binding := range imports {
		if binding.LocalName == "" || binding.LocalName == "_" || binding.LocalName == "." {
			continue
		}
		key := binding.FileID + "\x00" + binding.LocalName
		if previous, exists := result[key]; exists && previous.ID != binding.ID {
			previous.Status = ResolutionAmbiguous
			previous.TargetPackageID = ""
			previous.PackageIdentityProofID = ""
			result[key] = previous
			continue
		}
		result[key] = binding
	}
	return result
}

func resolveImportedName(binding ImportBinding, name string, declarations map[string][]SemanticDeclaration) (ResolutionStatus, string, string) {
	if !token.IsExported(name) {
		return ResolutionUnresolved, "", ""
	}
	switch binding.Status {
	case ResolutionResolved:
		matches := declarations[binding.TargetPackageID+"\x00"+name]
		if len(matches) == 1 && matches[0].Status == ResolutionResolved {
			return ResolutionResolved, matches[0].ID, ""
		}
		if len(matches) > 1 || (len(matches) == 1 && matches[0].Status == ResolutionAmbiguous) {
			return ResolutionAmbiguous, "", ""
		}
		return ResolutionUnresolved, "", ""
	case ResolutionExternal:
		return ResolutionExternal, "", binding.ImportPath + "." + name
	case ResolutionAmbiguous:
		return ResolutionAmbiguous, "", ""
	default:
		return ResolutionUnresolved, "", ""
	}
}

func applyDotImportCandidates(reference *SemanticReference, bindings []ImportBinding, name string, declarations map[string][]SemanticDeclaration) {
	matches := make([]SemanticDeclaration, 0)
	external := false
	for _, binding := range bindings {
		switch binding.Status {
		case ResolutionResolved:
			for _, declaration := range declarations[binding.TargetPackageID+"\x00"+name] {
				if token.IsExported(declaration.Name) {
					matches = append(matches, declaration)
				}
			}
		case ResolutionExternal, ResolutionAmbiguous:
			external = true
		}
	}
	applyDeclarationCandidates(reference, matches)
	if reference.Status == ResolutionUnresolved && external {
		reference.Status = ResolutionUnresolved
	}
}

func applyDeclarationCandidates(reference *SemanticReference, matches []SemanticDeclaration) {
	if len(matches) == 0 {
		reference.Status = ResolutionUnresolved
		return
	}
	ids := make([]string, 0, len(matches))
	for _, declaration := range matches {
		ids = append(ids, declaration.ID)
	}
	sort.Strings(ids)
	ids = compactStrings(ids)
	if len(ids) == 1 {
		declaration := matches[0]
		if declaration.Status == ResolutionResolved {
			reference.Status = ResolutionResolved
			reference.TargetDeclarationID = ids[0]
			return
		}
		if declaration.Status == ResolutionPartial {
			reference.Status = ResolutionPartial
			reference.TargetDeclarationID = ids[0]
			return
		}
	}
	reference.Status = ResolutionAmbiguous
	reference.CandidateDeclarationIDs = ids
}

func declarationsForIDs(ids []string, declarations map[string]SemanticDeclaration) []SemanticDeclaration {
	result := make([]SemanticDeclaration, 0, len(ids))
	for _, id := range ids {
		if declaration, exists := declarations[id]; exists {
			result = append(result, declaration)
		}
	}
	return result
}

type interfaceCandidate struct {
	packageID              string
	concreteDeclarationID  string
	interfaceDeclarationID string
	concreteExpression     ast.Expr
	interfaceExpression    ast.Expr
	valueExpression        ast.Expr
	interfaceType          types.Type
	concreteType           types.Type
	pointerMode            bool
	location               lie.SourceRange
	evidence               []rie.Evidence
}

type parsedSemanticPackage struct {
	id         string
	fileSet    *token.FileSet
	files      []*ast.File
	candidates []interfaceCandidate
	typeErrors []error
}

func evaluateInterfaceSatisfaction(ctx context.Context, outcomes []fileOutcome, declarations []SemanticDeclaration, typeRelations []TypeRelation, config Config) ([]InterfaceSatisfaction, []lie.Diagnostic, int, error) {
	declarationsByPackageName := make(map[string][]SemanticDeclaration)
	declarationsByID := make(map[string]SemanticDeclaration, len(declarations))
	for _, declaration := range declarations {
		declarationsByID[declaration.ID] = declaration
		if declaration.OwnerDeclarationID == "" && isTypeDeclaration(declaration.Kind) {
			key := declaration.PackageID + "\x00" + declaration.Name
			declarationsByPackageName[key] = append(declarationsByPackageName[key], declaration)
		}
	}
	grouped := make(map[string][]fileOutcome)
	for _, outcome := range outcomes {
		if outcome.file.Status == SemanticFilePartial || outcome.file.Status == SemanticFileResolved {
			grouped[outcome.file.PackageID] = append(grouped[outcome.file.PackageID], outcome)
		}
	}
	packageIDs := make([]string, 0, len(grouped))
	for packageID := range grouped {
		packageIDs = append(packageIDs, packageID)
	}
	sort.Strings(packageIDs)
	results := make([]InterfaceSatisfaction, 0)
	diagnostics := make([]lie.Diagnostic, 0)
	omitted := 0
	for _, packageID := range packageIDs {
		if err := ctx.Err(); err != nil {
			return nil, nil, 0, err
		}
		packageOutcomes := grouped[packageID]
		bytesTotal := int64(0)
		for _, outcome := range packageOutcomes {
			bytesTotal += int64(len(outcome.source))
		}
		if len(packageOutcomes) > config.MaxPackageFiles || bytesTotal > config.MaxPackageBytes {
			diagnostics = append(diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_package_limit", fmt.Sprintf("package %s exceeds the configured interface-analysis limit", packageID), ""))
			continue
		}
		parsed, parseDiagnostics, err := parseSemanticPackage(ctx, packageID, packageOutcomes, declarationsByPackageName)
		if err != nil {
			return nil, nil, 0, err
		}
		diagnostics = append(diagnostics, parseDiagnostics...)
		info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object)}
		checkConfig := types.Config{Error: func(err error) { parsed.typeErrors = append(parsed.typeErrors, err) }}
		if err := ctx.Err(); err != nil {
			return nil, nil, 0, err
		}
		typedPackage, _ := checkConfig.Check(packageID, parsed.fileSet, parsed.files, info)
		if err := ctx.Err(); err != nil {
			return nil, nil, 0, err
		}
		typedCandidates, err := typedInterfaceCandidates(ctx, packageID, parsed.fileSet, parsed.files, info, declarationsByPackageName)
		if err != nil {
			return nil, nil, 0, err
		}
		parsed.candidates = append(parsed.candidates, typedCandidates...)
		parsed.candidates = append(parsed.candidates, embeddedInterfaceCandidates(packageID, typedPackage, typeRelations, declarationsByID)...)
		parsed.candidates = mergeInterfaceCandidates(parsed.candidates)
		if len(parsed.candidates) == 0 {
			continue
		}
		complete := typeErrorsAreCandidateOnly(parsed.typeErrors, parsed.candidates, parsed.fileSet)
		for index, candidate := range parsed.candidates {
			if index%256 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, nil, 0, err
				}
			}
			if len(results) >= config.MaxRelationships {
				omitted += len(parsed.candidates) - index
				break
			}
			results = append(results, evaluateInterfaceCandidate(candidate, typedPackage, info, complete))
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })
	results = mergeInterfaceSatisfaction(results)
	return results, diagnostics, omitted, nil
}

func parseSemanticPackage(ctx context.Context, packageID string, outcomes []fileOutcome, declarations map[string][]SemanticDeclaration) (parsedSemanticPackage, []lie.Diagnostic, error) {
	result := parsedSemanticPackage{id: packageID, fileSet: token.NewFileSet(), files: []*ast.File{}, candidates: []interfaceCandidate{}, typeErrors: []error{}}
	diagnostics := make([]lie.Diagnostic, 0)
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].path < outcomes[j].path })
	for _, outcome := range outcomes {
		if err := ctx.Err(); err != nil {
			return parsedSemanticPackage{}, nil, err
		}
		parsed, err := parser.ParseFile(result.fileSet, outcome.path, outcome.source, parser.SkipObjectResolution|parser.AllErrors)
		if err != nil {
			diagnostics = append(diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_interface_parse_error", err.Error(), outcome.path))
			continue
		}
		result.files = append(result.files, parsed)
		result.candidates = append(result.candidates, interfaceCandidatesFromFile(result.fileSet, outcome.path, packageID, parsed, declarations)...)
	}
	sort.Slice(result.candidates, func(i, j int) bool {
		left, right := result.candidates[i], result.candidates[j]
		if left.location.File != right.location.File {
			return left.location.File < right.location.File
		}
		if left.location.Start.Offset != right.location.Start.Offset {
			return left.location.Start.Offset < right.location.Start.Offset
		}
		if left.concreteDeclarationID != right.concreteDeclarationID {
			return left.concreteDeclarationID < right.concreteDeclarationID
		}
		return left.interfaceDeclarationID < right.interfaceDeclarationID
	})
	return result, diagnostics, nil
}

func interfaceCandidatesFromFile(fileSet *token.FileSet, filePath, packageID string, parsed *ast.File, declarations map[string][]SemanticDeclaration) []interfaceCandidate {
	result := make([]interfaceCandidate, 0)
	for _, rawDeclaration := range parsed.Decls {
		declaration, ok := rawDeclaration.(*ast.GenDecl)
		if !ok || declaration.Tok != token.VAR {
			continue
		}
		for _, rawSpecification := range declaration.Specs {
			specification, ok := rawSpecification.(*ast.ValueSpec)
			if !ok || specification.Type == nil || len(specification.Values) == 0 {
				continue
			}
			interfaceName := localTypeBaseName(specification.Type)
			interfaceDeclaration, ok := exactTypeDeclaration(declarations[packageID+"\x00"+interfaceName], DeclarationInterface)
			if !ok {
				continue
			}
			for index, value := range specification.Values {
				concreteExpression, pointerMode := concreteTypeExpression(value)
				concreteName := localTypeBaseName(concreteExpression)
				concreteDeclaration, ok := exactConcreteDeclaration(declarations[packageID+"\x00"+concreteName])
				if !ok {
					continue
				}
				positionStart, positionEnd := fileSet.Position(specification.Pos()), fileSet.Position(specification.End())
				location := lie.SourceRange{File: filePath, Start: lie.Position{Offset: positionStart.Offset, Line: positionStart.Line, Column: positionStart.Column}, End: lie.Position{Offset: positionEnd.Offset, Line: positionEnd.Line, Column: positionEnd.Column}}
				evidence := []rie.Evidence{}
				if index < len(specification.Names) && specification.Names[index].Name == "_" {
					evidence = append(evidence, rie.Evidence{File: filePath, Rule: "go.compile-time-interface-assertion", Value: interfaceName + "<-" + concreteName})
				}
				result = append(result, interfaceCandidate{
					packageID: packageID, concreteDeclarationID: concreteDeclaration.ID, interfaceDeclarationID: interfaceDeclaration.ID,
					concreteExpression: concreteExpression, interfaceExpression: specification.Type, valueExpression: value,
					pointerMode: pointerMode, location: location, evidence: evidence,
				})
			}
		}
	}
	return result
}

func concreteTypeExpression(expression ast.Expr) (ast.Expr, bool) {
	switch typed := expression.(type) {
	case *ast.CompositeLit:
		return typed.Type, false
	case *ast.UnaryExpr:
		if typed.Op == token.AND {
			if nested, _ := concreteTypeExpression(typed.X); nested != nil {
				return nested, true
			}
		}
	case *ast.CallExpr:
		if identifier, ok := typed.Fun.(*ast.Ident); ok && identifier.Name == "new" && len(typed.Args) == 1 {
			return typed.Args[0], true
		}
		return stripPointerExpression(typed.Fun)
	}
	return nil, false
}

func stripPointerExpression(expression ast.Expr) (ast.Expr, bool) {
	pointer := false
	for expression != nil {
		switch typed := expression.(type) {
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.StarExpr:
			pointer = true
			expression = typed.X
		default:
			return expression, pointer
		}
	}
	return nil, pointer
}

func localTypeBaseName(expression ast.Expr) string {
	for expression != nil {
		switch typed := expression.(type) {
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.StarExpr:
			expression = typed.X
		case *ast.IndexExpr:
			expression = typed.X
		case *ast.IndexListExpr:
			expression = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return ""
		}
	}
	return ""
}

func exactTypeDeclaration(values []SemanticDeclaration, kind DeclarationKind) (SemanticDeclaration, bool) {
	if len(values) != 1 || values[0].Kind != kind || values[0].Status != ResolutionResolved {
		return SemanticDeclaration{}, false
	}
	return values[0], true
}

func exactConcreteDeclaration(values []SemanticDeclaration) (SemanticDeclaration, bool) {
	if len(values) != 1 || values[0].Status != ResolutionResolved || (values[0].Kind != DeclarationStruct && values[0].Kind != DeclarationDefinedType) {
		return SemanticDeclaration{}, false
	}
	return values[0], true
}

func typedInterfaceCandidates(ctx context.Context, packageID string, fileSet *token.FileSet, files []*ast.File, info *types.Info, declarations map[string][]SemanticDeclaration) ([]interfaceCandidate, error) {
	result := make([]interfaceCandidate, 0)
	nodes := 0
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			if node != nil {
				nodes++
				if nodes%1024 == 0 && ctx.Err() != nil {
					return false
				}
			}
			switch typed := node.(type) {
			case *ast.AssignStmt:
				count := min(len(typed.Lhs), len(typed.Rhs))
				for index := 0; index < count; index++ {
					if candidate, ok := interfaceCandidateFromTypes(packageID, fileSet, typed, info.TypeOf(typed.Lhs[index]), info.TypeOf(typed.Rhs[index]), declarations); ok {
						result = append(result, candidate)
					}
				}
			case *ast.CallExpr:
				if typeAndValue, exists := info.Types[typed.Fun]; exists && typeAndValue.IsType() && len(typed.Args) == 1 {
					if candidate, ok := interfaceCandidateFromTypes(packageID, fileSet, typed, typeAndValue.Type, info.TypeOf(typed.Args[0]), declarations); ok {
						result = append(result, candidate)
					}
					break
				}
				signature, ok := underlyingSignature(info.TypeOf(typed.Fun))
				if !ok {
					break
				}
				for index, argument := range typed.Args {
					parameterType := callParameterType(signature, index, len(typed.Args), typed.Ellipsis.IsValid())
					if candidate, ok := interfaceCandidateFromTypes(packageID, fileSet, typed, parameterType, info.TypeOf(argument), declarations); ok {
						result = append(result, candidate)
					}
				}
			case *ast.FuncDecl:
				object := info.Defs[typed.Name]
				if object == nil {
					break
				}
				function, ok := object.Type().(*types.Signature)
				if !ok || typed.Body == nil {
					break
				}
				ast.Inspect(typed.Body, func(bodyNode ast.Node) bool {
					if _, nested := bodyNode.(*ast.FuncLit); nested {
						return false
					}
					statement, ok := bodyNode.(*ast.ReturnStmt)
					if !ok {
						return true
					}
					count := min(len(statement.Results), function.Results().Len())
					for index := 0; index < count; index++ {
						if candidate, ok := interfaceCandidateFromTypes(packageID, fileSet, statement, function.Results().At(index).Type(), info.TypeOf(statement.Results[index]), declarations); ok {
							result = append(result, candidate)
						}
					}
					return true
				})
			}
			return true
		})
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func embeddedInterfaceCandidates(packageID string, typedPackage *types.Package, relations []TypeRelation, declarations map[string]SemanticDeclaration) []interfaceCandidate {
	if typedPackage == nil {
		return nil
	}
	result := make([]interfaceCandidate, 0)
	for _, relation := range relations {
		if relation.PackageID != packageID || relation.Kind != TypeRelationEmbeds || relation.Status != ResolutionResolved || relation.OwnerDeclarationID == "" || relation.TargetDeclarationID == "" {
			continue
		}
		concreteDeclaration, concreteExists := declarations[relation.OwnerDeclarationID]
		interfaceDeclaration, interfaceExists := declarations[relation.TargetDeclarationID]
		if !concreteExists || !interfaceExists || concreteDeclaration.Kind != DeclarationStruct || interfaceDeclaration.Kind != DeclarationInterface {
			continue
		}
		concreteObject := typedPackage.Scope().Lookup(concreteDeclaration.Name)
		interfaceObject := typedPackage.Scope().Lookup(interfaceDeclaration.Name)
		if concreteObject == nil || interfaceObject == nil {
			continue
		}
		concreteType, interfaceType := concreteObject.Type(), interfaceObject.Type()
		if _, ok := interfaceType.Underlying().(*types.Interface); !ok {
			continue
		}
		result = append(result, interfaceCandidate{
			packageID: packageID, concreteDeclarationID: concreteDeclaration.ID, interfaceDeclarationID: interfaceDeclaration.ID,
			interfaceType: interfaceType, concreteType: concreteType, pointerMode: false, location: relation.Location, evidence: []rie.Evidence{},
		})
	}
	return result
}

func interfaceCandidateFromTypes(packageID string, fileSet *token.FileSet, node ast.Node, interfaceType, concreteType types.Type, declarations map[string][]SemanticDeclaration) (interfaceCandidate, bool) {
	if interfaceType == nil || concreteType == nil {
		return interfaceCandidate{}, false
	}
	if _, ok := interfaceType.Underlying().(*types.Interface); !ok {
		return interfaceCandidate{}, false
	}
	interfaceObject := namedTypeObject(interfaceType)
	concreteObject := namedTypeObject(concreteType)
	if interfaceObject == nil || concreteObject == nil || interfaceObject.Pkg() == nil || concreteObject.Pkg() == nil || interfaceObject.Pkg().Path() != packageID || concreteObject.Pkg().Path() != packageID {
		return interfaceCandidate{}, false
	}
	interfaceDeclaration, ok := exactTypeDeclaration(declarations[packageID+"\x00"+interfaceObject.Name()], DeclarationInterface)
	if !ok {
		return interfaceCandidate{}, false
	}
	concreteDeclaration, ok := exactConcreteDeclaration(declarations[packageID+"\x00"+concreteObject.Name()])
	if !ok {
		return interfaceCandidate{}, false
	}
	start, end := fileSet.Position(node.Pos()), fileSet.Position(node.End())
	return interfaceCandidate{
		packageID: packageID, concreteDeclarationID: concreteDeclaration.ID, interfaceDeclarationID: interfaceDeclaration.ID,
		interfaceType: interfaceType, concreteType: concreteType, pointerMode: isPointerType(concreteType),
		location: lie.SourceRange{File: start.Filename, Start: lie.Position{Offset: start.Offset, Line: start.Line, Column: start.Column}, End: lie.Position{Offset: end.Offset, Line: end.Line, Column: end.Column}},
		evidence: []rie.Evidence{},
	}, true
}

func namedTypeObject(value types.Type) *types.TypeName {
	for value != nil {
		switch typed := value.(type) {
		case *types.Pointer:
			value = typed.Elem()
		case *types.Named:
			return typed.Obj()
		default:
			return nil
		}
	}
	return nil
}

func isPointerType(value types.Type) bool {
	_, ok := value.(*types.Pointer)
	return ok
}

func underlyingSignature(value types.Type) (*types.Signature, bool) {
	if value == nil {
		return nil, false
	}
	signature, ok := value.Underlying().(*types.Signature)
	return signature, ok
}

func callParameterType(signature *types.Signature, index, argumentCount int, ellipsis bool) types.Type {
	parameters := signature.Params()
	if parameters == nil || parameters.Len() == 0 {
		return nil
	}
	if index < parameters.Len()-1 || !signature.Variadic() {
		if index >= parameters.Len() {
			return nil
		}
		return parameters.At(index).Type()
	}
	last := parameters.At(parameters.Len() - 1).Type()
	if ellipsis && argumentCount == parameters.Len() {
		return last
	}
	if slice, ok := last.(*types.Slice); ok {
		return slice.Elem()
	}
	return last
}

func mergeInterfaceCandidates(values []interfaceCandidate) []interfaceCandidate {
	sort.Slice(values, func(i, j int) bool {
		left, right := interfaceSatisfactionID(values[i]), interfaceSatisfactionID(values[j])
		if left != right {
			return left < right
		}
		if values[i].location.File != values[j].location.File {
			return values[i].location.File < values[j].location.File
		}
		return values[i].location.Start.Offset < values[j].location.Start.Offset
	})
	result := make([]interfaceCandidate, 0, len(values))
	for _, value := range values {
		identifier := interfaceSatisfactionID(value)
		if len(result) == 0 || interfaceSatisfactionID(result[len(result)-1]) != identifier {
			result = append(result, value)
			continue
		}
		current := &result[len(result)-1]
		current.evidence = append(current.evidence, value.evidence...)
		if current.interfaceType == nil {
			current.interfaceType = value.interfaceType
		}
		if current.concreteType == nil {
			current.concreteType = value.concreteType
		}
	}
	return result
}

func typeErrorsAreCandidateOnly(values []error, candidates []interfaceCandidate, fileSet *token.FileSet) bool {
	for _, value := range values {
		message := value.Error()
		if !strings.Contains(message, "does not implement") && !strings.Contains(message, "cannot use") {
			return false
		}
		var typedError types.Error
		if !errors.As(value, &typedError) {
			return false
		}
		position := fileSet.Position(typedError.Pos)
		insideCandidate := false
		for _, candidate := range candidates {
			if candidate.location.File == position.Filename && position.Offset >= candidate.location.Start.Offset && position.Offset <= candidate.location.End.Offset {
				insideCandidate = true
				break
			}
		}
		if !insideCandidate {
			return false
		}
	}
	return true
}

func evaluateInterfaceCandidate(candidate interfaceCandidate, typedPackage *types.Package, info *types.Info, complete bool) InterfaceSatisfaction {
	result := InterfaceSatisfaction{
		ID: interfaceSatisfactionID(candidate), ConcreteTypeDeclarationID: candidate.concreteDeclarationID,
		InterfaceDeclarationID: candidate.interfaceDeclarationID, Status: SatisfactionUnknown,
		CompileTimeAssertions: append([]rie.Evidence(nil), candidate.evidence...),
	}
	if typedPackage == nil || !complete {
		return result
	}
	interfaceType := candidate.interfaceType
	if interfaceType == nil {
		interfaceType = info.TypeOf(candidate.interfaceExpression)
	}
	concreteType := candidate.concreteType
	if concreteType == nil {
		concreteType = info.TypeOf(candidate.valueExpression)
	}
	if interfaceType == nil || concreteType == nil {
		return result
	}
	contract, ok := interfaceType.Underlying().(*types.Interface)
	if !ok {
		return result
	}
	contract.Complete()
	base := concreteType
	if pointer, ok := concreteType.(*types.Pointer); ok {
		base = pointer.Elem()
	}
	valueImplements := types.Implements(base, contract)
	pointerImplements := types.Implements(types.NewPointer(base), contract)
	result.PointerRequired = !valueImplements && pointerImplements
	if types.Implements(concreteType, contract) {
		result.Status = SatisfactionProven
		return result
	}
	result.Status = SatisfactionDisproven
	result.MissingMethodNames = missingInterfaceMethods(concreteType, contract)
	return result
}

func missingInterfaceMethods(concrete types.Type, contract *types.Interface) []string {
	methodSet := types.NewMethodSet(concrete)
	missing := make([]string, 0)
	for index := 0; index < contract.NumMethods(); index++ {
		required := contract.Method(index)
		selection := methodSet.Lookup(required.Pkg(), required.Name())
		if selection == nil || !types.Identical(selection.Obj().Type(), required.Type()) {
			missing = append(missing, required.Name())
		}
	}
	sort.Strings(missing)
	return compactStrings(missing)
}

func interfaceSatisfactionID(candidate interfaceCandidate) string {
	return fmt.Sprintf("go:semantic:v1:satisfaction:%d:%s#%d:%s#%t", len(candidate.concreteDeclarationID), candidate.concreteDeclarationID, len(candidate.interfaceDeclarationID), candidate.interfaceDeclarationID, candidate.pointerMode)
}

func mergeInterfaceSatisfaction(values []InterfaceSatisfaction) []InterfaceSatisfaction {
	if len(values) == 0 {
		return values
	}
	result := make([]InterfaceSatisfaction, 0, len(values))
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1].ID != value.ID {
			result = append(result, value)
			continue
		}
		current := &result[len(result)-1]
		current.CompileTimeAssertions = append(current.CompileTimeAssertions, value.CompileTimeAssertions...)
		sort.Slice(current.CompileTimeAssertions, func(i, j int) bool {
			left, right := current.CompileTimeAssertions[i], current.CompileTimeAssertions[j]
			if left.File != right.File {
				return left.File < right.File
			}
			if left.Rule != right.Rule {
				return left.Rule < right.Rule
			}
			return left.Value < right.Value
		})
	}
	return result
}

func bindTypeRelations(declarations []SemanticDeclaration, candidates []typeRelationCandidate, imports []ImportBinding) []TypeRelation {
	byID := make(map[string]SemanticDeclaration, len(declarations))
	for _, declaration := range declarations {
		byID[declaration.ID] = declaration
	}
	packageTypes := typeDeclarationsByPackageName(declarations)
	importsByFileAlias := importsByFileAndAlias(imports)
	unique := make(map[string]TypeRelation)
	for _, candidate := range candidates {
		relation := TypeRelation{
			Kind: candidate.kind, FileID: candidate.fileID, PackageID: candidate.packageID,
			OwnerDeclarationID: candidate.ownerDeclarationID, Location: candidate.location,
			TargetIdentity: candidate.targetIdentity, TypeArgumentText: append([]string(nil), candidate.typeArguments...),
			Status: ResolutionUnresolved,
		}
		matches := typeCandidates(candidate.targetCandidates, byID)
		if len(candidate.targetCandidates) == 0 && candidate.targetName != "" {
			matches = packageTypes[candidate.packageID+"\x00"+candidate.targetName]
		}
		switch {
		case candidate.structural:
			relation.Status = ResolutionResolved
		case len(matches) == 1 && matches[0].Status == ResolutionResolved:
			relation.Status = ResolutionResolved
			relation.TargetDeclarationID = matches[0].ID
			relation.TargetIdentity = ""
		case len(matches) > 1 || (len(matches) == 1 && matches[0].Status == ResolutionAmbiguous):
			relation.Status = ResolutionAmbiguous
		case candidate.targetName != "" && isPredeclaredType(candidate.targetName):
			relation.Status = ResolutionExternal
			relation.TargetIdentity = "builtin:" + candidate.targetName
		default:
			if qualifier, name, ok := splitQualifiedIdentity(candidate.targetIdentity); ok {
				binding, exists := importsByFileAlias[candidate.fileID+"\x00"+qualifier]
				if exists {
					relation.Status, relation.TargetDeclarationID, relation.TargetIdentity = resolveImportedName(binding, name, packageTypes)
				} else {
					relation.Status = ResolutionUnresolved
				}
			} else {
				relation.Status = ResolutionUnresolved
			}
		}
		relation.ID = typeRelationID(relation)
		unique[relation.ID] = relation
	}
	result := make([]TypeRelation, 0, len(unique))
	for _, relation := range unique {
		result = append(result, relation)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Location.File != result[j].Location.File {
			return result[i].Location.File < result[j].Location.File
		}
		if result[i].Location.Start.Offset != result[j].Location.Start.Offset {
			return result[i].Location.Start.Offset < result[j].Location.Start.Offset
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func limitSemanticRelationships(imports []ImportBinding, receivers []ReceiverBinding, relations []TypeRelation, references []SemanticReference, satisfaction []InterfaceSatisfaction, maximum int) ([]ImportBinding, []ReceiverBinding, []TypeRelation, []SemanticReference, []InterfaceSatisfaction, int) {
	total := len(imports) + len(receivers) + len(relations) + len(references) + len(satisfaction)
	if total <= maximum {
		return imports, receivers, relations, references, satisfaction, 0
	}
	omitted := total - maximum
	if len(imports) >= maximum {
		return imports[:maximum], []ReceiverBinding{}, []TypeRelation{}, []SemanticReference{}, []InterfaceSatisfaction{}, omitted
	}
	remaining := maximum - len(imports)
	if len(receivers) >= remaining {
		return imports, receivers[:remaining], []TypeRelation{}, []SemanticReference{}, []InterfaceSatisfaction{}, omitted
	}
	remaining -= len(receivers)
	if len(relations) >= remaining {
		return imports, receivers, relations[:remaining], []SemanticReference{}, []InterfaceSatisfaction{}, omitted
	}
	remaining -= len(relations)
	if len(references) >= remaining {
		return imports, receivers, relations, references[:remaining], []InterfaceSatisfaction{}, omitted
	}
	remaining -= len(references)
	return imports, receivers, relations, references, satisfaction[:remaining], omitted
}

func updateReferenceStatistics(files []SemanticFile, references []SemanticReference, statistics *SemanticStatistics) {
	indices := make(map[string]int, len(files))
	for index := range files {
		indices[files[index].FileID] = index
	}
	statistics.ReferencesByStatus = make(map[string]int)
	for _, reference := range references {
		statistics.ReferencesByStatus[reference.Status.String()]++
		if index, exists := indices[reference.FileID]; exists {
			files[index].ReferenceCount++
			if reference.Status != ResolutionResolved && reference.Status != ResolutionExternal {
				files[index].UnresolvedCount++
			}
		}
	}
}

func statusesForImports(imports []ImportBinding) map[string]int {
	result := make(map[string]int)
	for _, binding := range imports {
		result[binding.Status.String()]++
	}
	return result
}

func statusesForInterfaceChecks(values []InterfaceSatisfaction) map[string]int {
	result := make(map[string]int)
	for _, value := range values {
		result[value.Status.String()]++
	}
	return result
}

func splitQualifiedIdentity(value string) (string, string, bool) {
	if strings.Count(value, ".") != 1 {
		return "", "", false
	}
	parts := strings.SplitN(value, ".", 2)
	return parts[0], parts[1], parts[0] != "" && parts[1] != ""
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func typeDeclarationsByPackageName(declarations []SemanticDeclaration) map[string][]SemanticDeclaration {
	result := make(map[string][]SemanticDeclaration)
	for _, declaration := range declarations {
		if declaration.OwnerDeclarationID != "" || !isTypeDeclaration(declaration.Kind) {
			continue
		}
		key := declaration.PackageID + "\x00" + declaration.Name
		result[key] = append(result[key], declaration)
	}
	return result
}

func typeCandidates(identifiers []string, declarations map[string]SemanticDeclaration) []SemanticDeclaration {
	result := make([]SemanticDeclaration, 0, len(identifiers))
	for _, identifier := range identifiers {
		declaration, exists := declarations[identifier]
		if exists && isTypeDeclaration(declaration.Kind) {
			result = append(result, declaration)
		}
	}
	return result
}

func isTypeDeclaration(kind DeclarationKind) bool {
	return kind == DeclarationStruct || kind == DeclarationInterface || kind == DeclarationDefinedType || kind == DeclarationTypeAlias || kind == DeclarationTypeParameter
}

func isPredeclaredType(name string) bool {
	switch name {
	case "any", "bool", "byte", "comparable", "complex64", "complex128", "error", "float32", "float64", "int", "int8", "int16", "int32", "int64", "rune", "string", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return true
	default:
		return false
	}
}

func isPredeclaredIdentifier(name string) bool {
	if isPredeclaredType(name) {
		return true
	}
	switch name {
	case "append", "cap", "clear", "close", "complex", "copy", "delete", "false", "imag", "iota", "len", "make", "max", "min", "new", "nil", "panic", "print", "println", "real", "recover", "true":
		return true
	default:
		return false
	}
}

func receiverBindingID(candidate receiverCandidate) string {
	return fmt.Sprintf("go:semantic:v1:receiver:%d:%s#%d:%s#%t#%t", len(candidate.methodDeclarationID), candidate.methodDeclarationID, len(candidate.receiverName), candidate.receiverName, candidate.pointer, candidate.generic)
}

func typeRelationID(relation TypeRelation) string {
	target := relation.TargetDeclarationID
	if target == "" {
		target = relation.TargetIdentity
	}
	return fmt.Sprintf("go:semantic:v1:relation:%d:%s#%s#%d#%d:%s", len(relation.OwnerDeclarationID), relation.OwnerDeclarationID, relation.Kind.String(), relation.Location.Start.Offset, len(target), target)
}

func importBindingID(fileID string, imported golang.GoImport) string {
	return fmt.Sprintf("go:semantic:v1:import:%d:%s#%d:%d:%s", len(fileID), fileID, imported.Location.Start.Offset, len(imported.Path), imported.Path)
}

func semanticReferenceID(reference SemanticReference) string {
	return fmt.Sprintf("go:semantic:v1:reference:%d:%s#%d#%s#%d:%s", len(reference.FileID), reference.FileID, reference.Location.Start.Offset, reference.Kind.String(), len(reference.Name), reference.Name)
}

func semanticDiagnostic(severity lie.Severity, code, message, file string) lie.Diagnostic {
	location := &lie.SourceRange{File: file}
	if file == "" {
		location = nil
	}
	return lie.Diagnostic{Engine: "go-semantic", Severity: severity, Code: code, Message: message, Location: location}
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
	kept = append(kept, semanticDiagnostic(lie.SeverityWarning, "semantic_diagnostic_limit", fmt.Sprintf("%d diagnostics omitted", omitted), ""))
	return kept, omitted
}

type sourceReader struct {
	root        *os.Root
	canonical   string
	directories map[string]sourceDirectory
}

type sourceDirectory struct {
	root     *os.Root
	absolute string
	symlinks map[string]bool
	err      error
}

func newSourceReader(root string, candidates []string) (*sourceReader, error) {
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("%w: resolve repository root: %v", ErrInvalidRepositoryRoot, err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve repository root: %v", ErrInvalidRepositoryRoot, err)
	}
	rootHandle, err := os.OpenRoot(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: open repository root: %v", ErrInvalidRepositoryRoot, err)
	}
	reader := &sourceReader{root: rootHandle, canonical: canonicalRoot, directories: make(map[string]sourceDirectory)}
	for _, candidate := range candidates {
		cleanPath := filepath.Clean(filepath.FromSlash(candidate))
		if !safeRelativePath(candidate, cleanPath) {
			continue
		}
		directoryName := filepath.Dir(cleanPath)
		if _, exists := reader.directories[directoryName]; exists {
			continue
		}
		absoluteDirectory, resolveErr := filepath.EvalSymlinks(filepath.Join(canonicalRoot, directoryName))
		directory := sourceDirectory{absolute: absoluteDirectory, symlinks: map[string]bool{}, err: resolveErr}
		if resolveErr == nil && !withinRoot(canonicalRoot, absoluteDirectory) {
			directory.err = ErrSourceOutsideRoot
		}
		if directory.err == nil {
			directory.root, directory.err = rootHandle.OpenRoot(directoryName)
		}
		if directory.err == nil {
			entries, readErr := os.ReadDir(absoluteDirectory)
			if readErr != nil {
				directory.err = readErr
			} else {
				for _, entry := range entries {
					directory.symlinks[entry.Name()] = entry.Type()&os.ModeSymlink != 0
				}
			}
		}
		reader.directories[directoryName] = directory
	}
	return reader, nil
}

func (reader *sourceReader) readFile(relativePath string, maximumBytes int64) ([]byte, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(relativePath))
	if !safeRelativePath(relativePath, cleanPath) {
		return nil, fmt.Errorf("%w: %s", ErrSourceOutsideRoot, relativePath)
	}
	directory := reader.directories[filepath.Dir(cleanPath)]
	if directory.root == nil && directory.err == nil {
		return nil, fmt.Errorf("%w: %s was not authorized by the repository snapshot", ErrSourceUnreadable, relativePath)
	}
	if directory.err != nil {
		if errors.Is(directory.err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrSourceMissing, relativePath)
		}
		if errors.Is(directory.err, ErrSourceOutsideRoot) || errors.Is(directory.err, os.ErrPermission) || errors.Is(directory.err, os.ErrInvalid) {
			return nil, fmt.Errorf("%w: %s", ErrSourceOutsideRoot, relativePath)
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, directory.err)
	}
	baseName := filepath.Base(cleanPath)
	if directory.symlinks[baseName] {
		resolved, err := filepath.EvalSymlinks(filepath.Join(directory.absolute, baseName))
		if err != nil || !withinRoot(reader.canonical, resolved) {
			return nil, fmt.Errorf("%w: %s", ErrSourceOutsideRoot, relativePath)
		}
	}
	flags := os.O_RDONLY
	if runtime.GOOS == "windows" {
		flags |= 0x08000000
	}
	file, err := directory.root.OpenFile(baseName, flags, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrSourceMissing, relativePath)
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, err)
	}
	defer file.Close()
	information, err := file.Stat()
	if err != nil || !information.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrSourceUnreadable, relativePath)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, err)
	}
	if int64(len(data)) > maximumBytes {
		return nil, fmt.Errorf("%w: %s exceeds %d bytes", ErrSourceOversized, relativePath, maximumBytes)
	}
	return data, nil
}

func (reader *sourceReader) close() {
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
