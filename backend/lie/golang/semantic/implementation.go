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

func (*engine) Name() string         { return "go-semantic" }
func (*engine) Version() string      { return engineVersion }
func (*engine) Language() string     { return "Go" }
func (*engine) ArtifactName() string { return ArtifactName }
func (*engine) Description() string {
	return "Verifies snapshot-authorized Go source and produces a bounded semantic candidate artifact"
}

func (engine *engine) Resolve(ctx context.Context, input Input) (GoSemanticInventory, error) {
	if ctx == nil {
		return GoSemanticInventory{}, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return GoSemanticInventory{}, err
	}
	if err := validateInput(input); err != nil {
		return GoSemanticInventory{}, err
	}

	syntaxFiles := input.Syntax.Files()
	syntaxSymbols := symbolsByFile(input.Syntax.Symbols())
	candidatePaths := make([]string, 0, len(syntaxFiles))
	for _, file := range syntaxFiles {
		if file.Status == golang.FileStatusParsed {
			candidatePaths = append(candidatePaths, file.Path)
		}
	}
	reader, err := newSourceReader(input.Snapshot.RootPath(), candidatePaths)
	if err != nil {
		return GoSemanticInventory{}, err
	}
	defer reader.close()

	outcomes := make([]fileOutcome, len(syntaxFiles))
	jobs := make(chan int)
	workerCount := min(engine.config.MaxWorkers, max(1, len(syntaxFiles)))
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
	for index := range syntaxFiles {
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

	files, declarations, receivers, typeRelations, diagnostics, statistics := collectOutcomes(outcomes)
	receivers, typeRelations, omittedRelationships := limitSemanticRelationships(receivers, typeRelations, engine.config.MaxRelationships)
	statistics.ReceiverBindings = len(receivers)
	statistics.TypeRelations = len(typeRelations)
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
	return newInventory(files, declarations, receivers, typeRelations, diagnostics, statistics), nil
}

func validateInput(input Input) error {
	snapshotMetadata := input.Snapshot.Metadata()
	if input.Snapshot.RootPath() == "" || snapshotMetadata.Name == "" {
		return ErrMissingRepositorySnapshot
	}
	if input.Snapshot.ArtifactName() != rie.RepositorySnapshotArtifactName ||
		input.Snapshot.ArtifactVersion() != rie.RepositorySnapshotArtifactVersion ||
		snapshotMetadata.Name != rie.RepositorySnapshotArtifactName ||
		snapshotMetadata.Version != rie.RepositorySnapshotArtifactVersion {
		return ErrIncompatibleRepositorySnapshot
	}

	syntaxMetadata := input.Syntax.Metadata()
	if syntaxMetadata.Name == "" {
		return ErrMissingGoLanguageInventory
	}
	if input.Syntax.ArtifactName() != golang.ArtifactName || input.Syntax.ArtifactVersion() != golang.ArtifactVersion ||
		syntaxMetadata.Name != golang.ArtifactName || syntaxMetadata.Version != golang.ArtifactVersion {
		return ErrIncompatibleGoInventory
	}

	identityMetadata := input.PackageIdentities.Metadata()
	if identityMetadata.Name == "" {
		return ErrMissingPackageIdentityInventory
	}
	if input.PackageIdentities.ArtifactName() != packageidentity.ArtifactName ||
		input.PackageIdentities.ArtifactVersion() != packageidentity.ArtifactVersion ||
		identityMetadata.Name != packageidentity.ArtifactName || identityMetadata.Version != packageidentity.ArtifactVersion {
		return ErrIncompatiblePackageIdentity
	}

	snapshotReference := rie.ArtifactReference{Name: rie.RepositorySnapshotArtifactName, Version: rie.RepositorySnapshotArtifactVersion}
	syntaxReference := rie.ArtifactReference{Name: golang.ArtifactName, Version: golang.ArtifactVersion}
	if !hasArtifactReference(input.Syntax.SourceArtifacts(), snapshotReference) ||
		!hasExactArtifactReferences(input.PackageIdentities.SourceArtifacts(), []rie.ArtifactReference{snapshotReference, syntaxReference}) {
		return ErrArtifactProvenanceMismatch
	}

	entries := make(map[string]bool)
	for _, entry := range input.Snapshot.Entries() {
		if _, duplicate := entries[entry.Path]; duplicate {
			return fmt.Errorf("%w: duplicate snapshot entry %s", ErrArtifactProvenanceMismatch, entry.Path)
		}
		entries[entry.Path] = entry.IsDir
	}
	fileIDs := make(map[string]struct{})
	filePaths := make(map[string]struct{})
	filesByID := make(map[string]golang.GoFile)
	for _, file := range input.Syntax.Files() {
		isDirectory, exists := entries[file.Path]
		if file.ID == "" || file.Path == "" || !exists || isDirectory {
			return fmt.Errorf("%w: Go file %s is absent from RepositorySnapshot", ErrArtifactProvenanceMismatch, file.Path)
		}
		if _, duplicate := fileIDs[file.ID]; duplicate {
			return fmt.Errorf("%w: duplicate Go file ID %s", ErrArtifactProvenanceMismatch, file.ID)
		}
		if _, duplicate := filePaths[file.Path]; duplicate {
			return fmt.Errorf("%w: duplicate Go file path %s", ErrArtifactProvenanceMismatch, file.Path)
		}
		fileIDs[file.ID] = struct{}{}
		filePaths[file.Path] = struct{}{}
		filesByID[file.ID] = file
	}

	packageIDs := make(map[string]struct{})
	for _, pkg := range input.Syntax.Packages() {
		if pkg.ID == "" {
			return fmt.Errorf("%w: Go package has an empty ID", ErrArtifactProvenanceMismatch)
		}
		packageIDs[pkg.ID] = struct{}{}
	}
	symbolIDs := make(map[string]struct{})
	for _, symbol := range input.Syntax.Symbols() {
		file, exists := filesByID[symbol.FileID]
		if symbol.ID == "" || symbol.Name == "" || !exists || symbol.PackageID != file.PackageID || symbol.Location.File != file.Path || symbol.Location.Start.Offset < 0 || symbol.Location.End.Offset < symbol.Location.Start.Offset || symbol.Kind.String() == "unknown" {
			return fmt.Errorf("%w: invalid Go syntax symbol %s", ErrArtifactProvenanceMismatch, symbol.ID)
		}
		if _, exists := packageIDs[symbol.PackageID]; !exists {
			return fmt.Errorf("%w: symbol %s belongs to unknown package %s", ErrArtifactProvenanceMismatch, symbol.ID, symbol.PackageID)
		}
		if _, duplicate := symbolIDs[symbol.ID]; duplicate {
			return fmt.Errorf("%w: duplicate Go symbol ID %s", ErrArtifactProvenanceMismatch, symbol.ID)
		}
		symbolIDs[symbol.ID] = struct{}{}
	}
	for _, proof := range input.PackageIdentities.Proofs() {
		if _, exists := packageIDs[proof.ImportingPackageID]; !exists {
			return fmt.Errorf("%w: proof %s imports from unknown package %s", ErrArtifactProvenanceMismatch, proof.ID, proof.ImportingPackageID)
		}
		if proof.TargetPackageID != "" {
			if _, exists := packageIDs[proof.TargetPackageID]; !exists {
				return fmt.Errorf("%w: proof %s targets unknown package %s", ErrArtifactProvenanceMismatch, proof.ID, proof.TargetPackageID)
			}
		}
		for _, candidate := range proof.CandidatePackageIDs {
			if _, exists := packageIDs[candidate]; !exists {
				return fmt.Errorf("%w: proof %s names unknown candidate %s", ErrArtifactProvenanceMismatch, proof.ID, candidate)
			}
		}
	}
	return nil
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
	receivers     []receiverCandidate
	typeRelations []typeRelationCandidate
	diagnostics   []lie.Diagnostic
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
	declarations, receivers, typeRelations, diagnostics, err := reconcileDeclarations(ctx, data, source, syntaxSymbols)
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

type declarationCollector struct {
	ctx           context.Context
	fileSet       *token.FileSet
	path          string
	fileID        string
	packageID     string
	syntax        []golang.GoSymbol
	syntaxByKey   map[syntaxDeclarationKey][]golang.GoSymbol
	matched       map[string]bool
	declarations  []SemanticDeclaration
	receivers     []receiverCandidate
	typeRelations []typeRelationCandidate
	diagnostics   []lie.Diagnostic
	packageScope  *lexicalScope
	fileScope     *lexicalScope
}

func reconcileDeclarations(ctx context.Context, data []byte, source golang.GoFile, syntax []golang.GoSymbol) ([]SemanticDeclaration, []receiverCandidate, []typeRelationCandidate, []lie.Diagnostic, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, source.Path, data, parser.SkipObjectResolution|parser.AllErrors)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	packageScope := newLexicalScope(scopePackage, nil)
	collector := &declarationCollector{
		ctx: ctx, fileSet: fileSet, path: source.Path, fileID: source.ID, packageID: source.PackageID,
		syntax: syntax, syntaxByKey: make(map[syntaxDeclarationKey][]golang.GoSymbol), matched: make(map[string]bool),
		declarations: []SemanticDeclaration{}, receivers: []receiverCandidate{}, typeRelations: []typeRelationCandidate{}, diagnostics: []lie.Diagnostic{}, packageScope: packageScope,
		fileScope: newLexicalScope(scopeFile, packageScope),
	}
	for _, symbol := range syntax {
		key := syntaxDeclarationKey{kind: symbol.Kind, name: symbol.Name, start: symbol.Location.Start.Offset, end: symbol.Location.End.Offset}
		collector.syntaxByKey[key] = append(collector.syntaxByKey[key], symbol)
	}
	for _, declaration := range parsed.Decls {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, nil, err
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
	return collector.declarations, collector.receivers, collector.typeRelations, collector.diagnostics, nil
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
			for _, name := range specification.Names {
				if name.Name == "_" {
					continue
				}
				location := collector.sourceRange(name.Pos(), name.End())
				collector.collectTypeUses(specification.Type, scope, semanticDeclarationID(collector.path, location.Start.Offset, kind, name.Name))
				collector.addDeclaration(name.Name, kind, typeDisplay, location, owner, scope, topLevel, syntaxKind)
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
	} else {
		switch specification.Type.(type) {
		case *ast.StructType:
			kind, syntaxKind, reconcile = DeclarationStruct, golang.SymbolKindStruct, topLevel
		case *ast.InterfaceType:
			kind, syntaxKind, reconcile = DeclarationInterface, golang.SymbolKindInterface, topLevel
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
	})
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
		}
		markers = append(markers, pushed)
		return collector.ctx.Err() == nil
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
	scope.declare(name, declaration.ID)
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

func collectOutcomes(outcomes []fileOutcome) ([]SemanticFile, []SemanticDeclaration, []ReceiverBinding, []TypeRelation, []lie.Diagnostic, SemanticStatistics) {
	sort.Slice(outcomes, func(i, j int) bool {
		if outcomes[i].path != outcomes[j].path {
			return outcomes[i].path < outcomes[j].path
		}
		return outcomes[i].file.FileID < outcomes[j].file.FileID
	})
	files := make([]SemanticFile, 0, len(outcomes))
	declarations := make([]SemanticDeclaration, 0)
	receiverCandidates := make([]receiverCandidate, 0)
	typeRelationCandidates := make([]typeRelationCandidate, 0)
	diagnostics := make([]lie.Diagnostic, 0)
	statistics := emptyStatistics()
	statistics.CandidateFiles = len(outcomes)
	for _, outcome := range outcomes {
		files = append(files, outcome.file)
		declarations = append(declarations, outcome.declarations...)
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
	typeRelations := bindTypeRelations(declarations, typeRelationCandidates)
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
	statistics.TypeRelations = len(typeRelations)
	return files, declarations, receivers, typeRelations, diagnostics, statistics
}

func reconcilePackageScopes(declarations []SemanticDeclaration) ([]SemanticDeclaration, []lie.Diagnostic) {
	packageScopes := make(map[string]*lexicalScope)
	indices := make(map[string][]int)
	for index, declaration := range declarations {
		if declaration.OwnerDeclarationID != "" || declaration.Kind == DeclarationMethod || (declaration.Kind == DeclarationFunction && declaration.Name == "init") {
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

func bindTypeRelations(declarations []SemanticDeclaration, candidates []typeRelationCandidate) []TypeRelation {
	byID := make(map[string]SemanticDeclaration, len(declarations))
	for _, declaration := range declarations {
		byID[declaration.ID] = declaration
	}
	packageTypes := typeDeclarationsByPackageName(declarations)
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
			relation.Status = ResolutionUnresolved
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

func limitSemanticRelationships(receivers []ReceiverBinding, relations []TypeRelation, maximum int) ([]ReceiverBinding, []TypeRelation, int) {
	total := len(receivers) + len(relations)
	if total <= maximum {
		return receivers, relations, 0
	}
	omitted := total - maximum
	if len(receivers) >= maximum {
		return receivers[:maximum], []TypeRelation{}, omitted
	}
	remaining := maximum - len(receivers)
	return receivers, relations[:remaining], omitted
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
