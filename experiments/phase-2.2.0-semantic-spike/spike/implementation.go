package spike

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"sort"
	"strings"
)

type runner struct{ config Config }

func (engine *runner) Run(ctx context.Context, input Input) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("%w: context is required", ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	files := append([]SourceFile(nil), input.Files...)
	sort.Slice(files, func(i, j int) bool { return normalizePath(files[i].Path) < normalizePath(files[j].Path) })

	fileSet := token.NewFileSet()
	groups := make(map[string][]*ast.File)
	result := Result{DeclarationIDs: []string{}, TypeErrors: []string{}}
	for _, source := range files {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		normalized := normalizePath(source.Path)
		if normalized == "" || source.Content == "" {
			return Result{}, fmt.Errorf("%w: path and content are required", ErrInvalidInput)
		}
		parsed, err := parser.ParseFile(fileSet, normalized, source.Content, parser.AllErrors)
		result.ParseCount++
		if err != nil {
			return Result{}, fmt.Errorf("%w: parse %s: %v", ErrInvalidInput, normalized, err)
		}
		if _, err := inspectWithContext(ctx, parsed, engine.config.NodeCheckInterval, nil); err != nil {
			return Result{}, err
		}
		result.DeclarationIDs = append(result.DeclarationIDs, declarationIDs(fileSet, normalized, parsed)...)
		groupKey := path.Dir(normalized) + "#" + parsed.Name.Name
		groups[groupKey] = append(groups[groupKey], parsed)
	}

	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	for _, key := range groupKeys {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		packageResult, err := checkPackage(ctx, fileSet, key, groups[key], rejectingImporter{})
		if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			return Result{}, err
		}
		result.PackageChecks++
		result.Definitions += packageResult.Definitions
		result.Uses += packageResult.Uses
		result.GenericInstances += packageResult.GenericInstances
		result.TypeErrors = append(result.TypeErrors, packageResult.TypeErrors...)
	}
	sort.Strings(result.DeclarationIDs)
	result.DeclarationIDs = compactStrings(result.DeclarationIDs)
	sort.Strings(result.TypeErrors)
	result.TypeErrors = compactStrings(result.TypeErrors)
	return result, nil
}

type packageCheckResult struct {
	Definitions      int
	Uses             int
	GenericInstances int
	TypeErrors       []string
}

func checkPackage(ctx context.Context, fileSet *token.FileSet, packagePath string, files []*ast.File, sourceImporter types.Importer) (packageCheckResult, error) {
	if err := ctx.Err(); err != nil {
		return packageCheckResult{}, err
	}
	information := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Instances:  make(map[*ast.Ident]types.Instance),
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	typeErrors := make([]string, 0)
	configuration := &types.Config{
		Importer: sourceImporter,
		Error: func(err error) {
			typeErrors = append(typeErrors, normalizeTypeError(err.Error()))
		},
	}
	_, checkErr := configuration.Check(packagePath, fileSet, files, information)
	if err := ctx.Err(); err != nil {
		return packageCheckResult{}, err
	}
	if checkErr != nil && len(typeErrors) == 0 {
		typeErrors = append(typeErrors, normalizeTypeError(checkErr.Error()))
	}
	definitions := 0
	for _, object := range information.Defs {
		if object != nil {
			definitions++
		}
	}
	uses := 0
	for _, object := range information.Uses {
		if object != nil {
			uses++
		}
	}
	sort.Strings(typeErrors)
	return packageCheckResult{
		Definitions: definitions, Uses: uses, GenericInstances: len(information.Instances),
		TypeErrors: compactStrings(typeErrors),
	}, checkErr
}

type rejectingImporter struct{}

func (rejectingImporter) Import(importPath string) (*types.Package, error) {
	return nil, fmt.Errorf("external import blocked: %s", importPath)
}

func declarationIDs(fileSet *token.FileSet, relativePath string, file *ast.File) []string {
	identifiers := make([]string, 0)
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			kind := "function"
			if typed.Recv != nil {
				kind = "method"
			}
			identifiers = append(identifiers, semanticID(relativePath, fileSet.Position(typed.Pos()).Offset, kind, typed.Name.Name))
		case *ast.GenDecl:
			for _, specification := range typed.Specs {
				switch spec := specification.(type) {
				case *ast.TypeSpec:
					kind := "defined-type"
					if spec.Assign.IsValid() {
						kind = "type-alias"
					} else {
						switch spec.Type.(type) {
						case *ast.StructType:
							kind = "struct"
						case *ast.InterfaceType:
							kind = "interface"
						}
					}
					identifiers = append(identifiers, semanticID(relativePath, fileSet.Position(spec.Pos()).Offset, kind, spec.Name.Name))
				case *ast.ValueSpec:
					kind := strings.ToLower(typed.Tok.String())
					for _, name := range spec.Names {
						identifiers = append(identifiers, semanticID(relativePath, fileSet.Position(name.Pos()).Offset, kind, name.Name))
					}
				}
			}
		}
	}
	return identifiers
}

func semanticID(relativePath string, offset int, kind, name string) string {
	normalized := normalizePath(relativePath)
	return fmt.Sprintf("go:semantic:v1:file:%d:%s#%d:%s:%s", len(normalized), normalized, offset, kind, name)
}

func relationID(sourceID, kind, targetID string) string {
	return fmt.Sprintf("go:semantic:v1:relation:%d:%s#%s#%d:%s", len(sourceID), sourceID, kind, len(targetID), targetID)
}

func packageProofID(contextID, importingPackageID, importPath string, kinds []proofKind) string {
	orderedKinds := append([]proofKind(nil), kinds...)
	sort.Slice(orderedKinds, func(i, j int) bool { return orderedKinds[i] < orderedKinds[j] })
	orderedKinds = compactProofKinds(orderedKinds)
	return fmt.Sprintf(
		"go:package-proof:v1:%d:%s#%d:%s#%d:%s#%s",
		len(contextID), contextID,
		len(importingPackageID), importingPackageID,
		len(importPath), importPath,
		strings.Join(proofKindsToStrings(orderedKinds), ","),
	)
}

func normalizePath(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(path.Clean("/"+strings.TrimLeft(value, "/")), "/")
	if value == "." {
		return ""
	}
	return value
}

func normalizeTypeError(message string) string {
	message = strings.ReplaceAll(message, "\\", "/")
	return strings.TrimSpace(message)
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

func inspectWithContext(ctx context.Context, root ast.Node, interval int, checkpoint func(int)) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	visited := 0
	var stopped error
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil || stopped != nil {
			return stopped == nil
		}
		visited++
		if visited == 1 || visited%interval == 0 {
			if checkpoint != nil {
				checkpoint(visited)
			}
			if err := ctx.Err(); err != nil {
				stopped = err
				return false
			}
		}
		return true
	})
	if stopped != nil {
		return visited, stopped
	}
	return visited, ctx.Err()
}

func processRelationships(ctx context.Context, total, interval int, checkpoint func(int)) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	processed := 0
	for processed < total {
		processed++
		if processed == 1 || processed%interval == 0 {
			if checkpoint != nil {
				checkpoint(processed)
			}
			if err := ctx.Err(); err != nil {
				return processed, err
			}
		}
	}
	return processed, ctx.Err()
}

func resolveAcrossContexts(importPath string, contexts []resolutionContext) identityDecision {
	if len(contexts) == 0 {
		return identityDecision{Status: proofUnresolved, Kinds: []proofKind{}, CandidatePackageIDs: []string{}}
	}
	ordered := append([]resolutionContext(nil), contexts...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	decisions := make([]identityDecision, 0, len(ordered))
	for _, resolutionContext := range ordered {
		decisions = append(decisions, resolveInContext(importPath, resolutionContext))
	}
	first := decisionIdentity(decisions[0])
	allSame := true
	for _, decision := range decisions[1:] {
		if decisionIdentity(decision) != first {
			allSame = false
			break
		}
	}
	if allSame {
		return decisions[0]
	}
	candidates := make([]string, 0)
	kinds := make([]proofKind, 0)
	for _, decision := range decisions {
		if decision.TargetPackageID != "" {
			candidates = append(candidates, decision.TargetPackageID)
		}
		kinds = append(kinds, decision.Kinds...)
	}
	sort.Strings(candidates)
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	return identityDecision{Status: proofAmbiguous, Kinds: compactProofKinds(kinds), CandidatePackageIDs: compactStrings(candidates)}
}

func resolveInContext(importPath string, current resolutionContext) identityDecision {
	if _, exists := current.ExactStandardLibrary[importPath]; exists {
		return identityDecision{Status: proofExternal, Kinds: []proofKind{proofStandardLibrary}, CandidatePackageIDs: []string{}}
	}
	for _, moduleID := range sortedStrings(current.MainModuleIDs) {
		module, exists := current.Modules[moduleID]
		if !exists {
			continue
		}
		if target := packageFromModule(module, importPath, module.Path); target != "" {
			kind := proofSameModule
			if current.Kind == contextWorkspace || current.Kind == contextWorkspaceVendor {
				kind = proofWorkspaceModule
			}
			return identityDecision{Status: proofResolved, TargetPackageID: target, Kinds: []proofKind{kind}, CandidatePackageIDs: []string{}}
		}
	}
	if current.Kind == contextModuleVendor || current.Kind == contextWorkspaceVendor {
		if target := current.VendorPackages[importPath]; target != "" {
			return identityDecision{Status: proofResolved, TargetPackageID: target, Kinds: []proofKind{proofVendor}, CandidatePackageIDs: []string{}}
		}
		return identityDecision{Status: proofExternal, Kinds: []proofKind{proofVendor}, CandidatePackageIDs: []string{}}
	}
	importingModule, exists := current.Modules[current.ImportingModuleID]
	if !exists {
		return identityDecision{Status: proofUnresolved, Kinds: []proofKind{}, CandidatePackageIDs: []string{}}
	}
	if current.Kind == contextWorkspace {
		if decision, matched := resolveReplacement(importPath, importingModule, current.WorkspaceReplaces, current.Modules); matched {
			return decision
		}
	}
	if decision, matched := resolveReplacement(importPath, importingModule, importingModule.Replaces, current.Modules); matched {
		return decision
	}
	return identityDecision{Status: proofExternal, Kinds: []proofKind{}, CandidatePackageIDs: []string{}}
}

func resolveReplacement(importPath string, importing moduleFact, replacements []replaceFact, modules map[string]moduleFact) (identityDecision, bool) {
	type match struct {
		replacement  replaceFact
		prefixLength int
	}
	matches := make([]match, 0)
	for _, replacement := range replacements {
		version, required := importing.Requires[replacement.OldPath]
		if !required || (replacement.OldVersion != "" && replacement.OldVersion != version) {
			continue
		}
		if hasImportPrefix(importPath, replacement.OldPath) {
			matches = append(matches, match{replacement: replacement, prefixLength: len(replacement.OldPath)})
		}
	}
	if len(matches) == 0 {
		return identityDecision{}, false
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].prefixLength != matches[j].prefixLength {
			return matches[i].prefixLength > matches[j].prefixLength
		}
		left, right := matches[i].replacement, matches[j].replacement
		if (left.OldVersion == "") != (right.OldVersion == "") {
			return left.OldVersion != ""
		}
		return left.TargetModuleID < right.TargetModuleID
	})
	longest := matches[0].prefixLength
	preferVersionSpecific := matches[0].replacement.OldVersion != ""
	targets := make([]string, 0)
	for _, candidate := range matches {
		if candidate.prefixLength != longest {
			break
		}
		if (candidate.replacement.OldVersion != "") != preferVersionSpecific {
			continue
		}
		targetModule, exists := modules[candidate.replacement.TargetModuleID]
		if !exists {
			continue
		}
		relative := strings.TrimPrefix(importPath, candidate.replacement.OldPath)
		relative = strings.TrimPrefix(relative, "/")
		if target := targetModule.PackagesByRelative[relative]; target != "" {
			targets = append(targets, target)
		}
	}
	sort.Strings(targets)
	targets = compactStrings(targets)
	if len(targets) == 1 {
		return identityDecision{Status: proofResolved, TargetPackageID: targets[0], Kinds: []proofKind{proofLocalReplace}, CandidatePackageIDs: []string{}}, true
	}
	if len(targets) > 1 {
		return identityDecision{Status: proofAmbiguous, Kinds: []proofKind{proofLocalReplace}, CandidatePackageIDs: targets}, true
	}
	return identityDecision{Status: proofUnresolved, Kinds: []proofKind{proofLocalReplace}, CandidatePackageIDs: []string{}}, true
}

func packageFromModule(module moduleFact, importPath, modulePath string) string {
	if !hasImportPrefix(importPath, modulePath) {
		return ""
	}
	relative := strings.TrimPrefix(importPath, modulePath)
	relative = strings.TrimPrefix(relative, "/")
	return module.PackagesByRelative[relative]
}

func hasImportPrefix(importPath, modulePath string) bool {
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}

func decisionIdentity(decision identityDecision) string {
	return string(decision.Status) + "#" + decision.TargetPackageID + "#" + strings.Join(proofKindsToStrings(decision.Kinds), ",")
}

func proofKindsToStrings(kinds []proofKind) []string {
	values := make([]string, len(kinds))
	for index, kind := range kinds {
		values[index] = string(kind)
	}
	return values
}

func compactProofKinds(values []proofKind) []proofKind {
	if len(values) == 0 {
		return []proofKind{}
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

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func deriveInterfaceCandidates(events []candidateEvent, maximum int) ([]interfaceCandidate, int) {
	allowed := map[string]struct{}{
		"assertion": {}, "assignment": {}, "conversion": {}, "argument": {}, "return": {},
		"embedding": {}, "receiver-selector": {}, "type-relation": {},
	}
	seen := make(map[string]interfaceCandidate)
	for _, event := range events {
		if _, ok := allowed[event.Kind]; !ok || event.ConcreteDeclaration == "" || event.InterfaceDeclaration == "" {
			continue
		}
		key := fmt.Sprintf("%s#%s#%t", event.ConcreteDeclaration, event.InterfaceDeclaration, event.Pointer)
		seen[key] = interfaceCandidate{ConcreteDeclaration: event.ConcreteDeclaration, InterfaceDeclaration: event.InterfaceDeclaration, Pointer: event.Pointer}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	omitted := 0
	if len(keys) > maximum {
		omitted = len(keys) - maximum
		keys = keys[:maximum]
	}
	result := make([]interfaceCandidate, 0, len(keys))
	for _, key := range keys {
		result = append(result, seen[key])
	}
	return result, omitted
}

func stabilizeDiagnostics(items []diagnostic, maximumPerFile, maximum int) diagnosticResult {
	ordered := append([]diagnostic(nil), items...)
	sort.Slice(ordered, func(i, j int) bool { return diagnosticLess(ordered[i], ordered[j]) })
	unique := make([]diagnostic, 0, len(ordered))
	for _, item := range ordered {
		if len(unique) > 0 && diagnosticKey(unique[len(unique)-1]) == diagnosticKey(item) {
			continue
		}
		unique = append(unique, item)
	}
	perFile := make(map[string]int)
	kept := make([]diagnostic, 0, len(unique))
	omitted := 0
	for _, item := range unique {
		if perFile[item.File] >= maximumPerFile {
			omitted++
			continue
		}
		perFile[item.File]++
		kept = append(kept, item)
	}
	if omitted == 0 && len(kept) <= maximum {
		return diagnosticResult{Items: kept}
	}
	ordinaryLimit := maximum - 1
	if ordinaryLimit < 0 {
		ordinaryLimit = 0
	}
	if len(kept) > ordinaryLimit {
		omitted += len(kept) - ordinaryLimit
		kept = kept[:ordinaryLimit]
	}
	aggregate := diagnostic{Severity: "warning", Code: "semantic_diagnostic_limit", Message: fmt.Sprintf("%d diagnostics omitted", omitted)}
	kept = append(kept, aggregate)
	return diagnosticResult{Items: kept, Omitted: omitted}
}

func diagnosticLess(left, right diagnostic) bool {
	if left.File != right.File {
		return left.File < right.File
	}
	if left.Start != right.Start {
		return left.Start < right.Start
	}
	if left.End != right.End {
		return left.End < right.End
	}
	if left.Severity != right.Severity {
		return left.Severity < right.Severity
	}
	if left.Code != right.Code {
		return left.Code < right.Code
	}
	return left.Message < right.Message
}

func diagnosticKey(value diagnostic) string {
	return fmt.Sprintf("%s#%d#%d#%s#%s", value.File, value.Start, value.End, value.Code, value.Message)
}
