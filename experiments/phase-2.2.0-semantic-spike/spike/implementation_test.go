package spike

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRunnerPerformsDeterministicFullRebuild(t *testing.T) {
	t.Parallel()
	engine := mustRunner(t)
	firstInput := Input{Files: []SourceFile{
		{Path: "b/b.go", Content: "package b\nconst C = 1\n"},
		{Path: "a/b.go", Content: "package a\nvar V = T{}\n"},
		{Path: "a/a.go", Content: "package a\ntype T struct{}\nfunc (T) M() {}\n"},
	}}
	secondInput := Input{Files: []SourceFile{firstInput.Files[2], firstInput.Files[0], firstInput.Files[1]}}

	first, err := engine.Run(context.Background(), firstInput)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := engine.Run(context.Background(), secondInput)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("shuffled full rebuild differs:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	if first.ParseCount != len(firstInput.Files) || second.ParseCount != len(secondInput.Files) {
		t.Fatalf("every run must parse every file: first=%d second=%d", first.ParseCount, second.ParseCount)
	}

	changed := firstInput
	changed.Files = append([]SourceFile(nil), firstInput.Files...)
	changed.Files[1].Content = "package a\nvar V = T{}\nvar Added = 1\n"
	third, err := engine.Run(context.Background(), changed)
	if err != nil {
		t.Fatalf("changed run: %v", err)
	}
	if third.ParseCount != len(changed.Files) {
		t.Fatalf("one changed file still requires full parse: got %d", third.ParseCount)
	}
}

func TestGoTypesRetainsPartialLocalInformationWhenImportIsBlocked(t *testing.T) {
	t.Parallel()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "partial.go", `package partial
import missing "example.com/not-present"
type Local struct{ Value int }
var _ missing.External
var Instance Local
`, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	imports := &recordingImporter{}
	result, checkErr := checkPackage(context.Background(), fileSet, "example.com/partial", []*ast.File{file}, imports)
	if checkErr == nil {
		t.Fatal("expected a type error for the blocked import")
	}
	if result.Definitions == 0 || result.Uses == 0 {
		t.Fatalf("expected partial local semantic facts, got defs=%d uses=%d", result.Definitions, result.Uses)
	}
	if !reflect.DeepEqual(imports.paths, []string{"example.com/not-present"}) {
		t.Fatalf("unexpected importer calls: %v", imports.paths)
	}
	if len(result.TypeErrors) == 0 || !strings.Contains(strings.Join(result.TypeErrors, "\n"), "external import blocked") {
		t.Fatalf("missing stable blocked-import error: %v", result.TypeErrors)
	}
}

func TestGoTypesSupportsGenericsAndInstances(t *testing.T) {
	t.Parallel()
	engine := mustRunner(t)
	result, err := engine.Run(context.Background(), Input{Files: []SourceFile{{Path: "generic.go", Content: `package generic
type Number interface { ~int | ~int64 }
func Identity[T Number](value T) T { return value }
var Value = Identity(1)
`}}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(result.TypeErrors) != 0 {
		t.Fatalf("unexpected type errors: %v", result.TypeErrors)
	}
	if result.GenericInstances != 1 {
		t.Fatalf("expected one generic instance, got %d", result.GenericInstances)
	}
}

func TestEmbeddedInterfaceAndPointerValueMethodSets(t *testing.T) {
	t.Parallel()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "methods.go", `package methods
type Base interface { M() }
type Combined interface { Base; N() }
type T struct{}
func (*T) M() {}
func (*T) N() {}
`, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	checked, err := (&types.Config{Importer: rejectingImporter{}}).Check("example.com/methods", fileSet, []*ast.File{file}, nil)
	if err != nil {
		t.Fatalf("type check: %v", err)
	}
	typeT := checked.Scope().Lookup("T").Type()
	base := checked.Scope().Lookup("Base").Type().Underlying().(*types.Interface).Complete()
	combined := checked.Scope().Lookup("Combined").Type().Underlying().(*types.Interface).Complete()
	if types.Implements(typeT, base) || types.Implements(typeT, combined) {
		t.Fatal("value T must not satisfy pointer-receiver interfaces")
	}
	pointerT := types.NewPointer(typeT)
	if !types.Implements(pointerT, base) || !types.Implements(pointerT, combined) {
		t.Fatal("*T must satisfy both embedded interfaces")
	}
}

func TestSemanticIDGoldenVectorsAndSourceOffset(t *testing.T) {
	t.Parallel()
	want := "go:semantic:v1:file:11:pkg/main.go#11:function:Run"
	if got := semanticID(`pkg\main.go`, 11, "function", "Run"); got != want {
		t.Fatalf("golden ID mismatch: got %q want %q", got, want)
	}
	if semanticID("pkg/main.go", 12, "function", "Run") == want {
		t.Fatal("offset change must re-key the semantic declaration")
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "pkg/main.go", "package p\n\nfunc Run() {}\n", parser.AllErrors)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ids := declarationIDs(fileSet, "pkg/main.go", file)
	if !reflect.DeepEqual(ids, []string{want}) {
		t.Fatalf("Phase 2.1-compatible byte offset changed: %v", ids)
	}

	target := "go:semantic:v1:file:12:pkg/types.go#10:struct:Worker"
	wantRelation := "go:semantic:v1:relation:50:" + want + "#implements#52:" + target
	if got := relationID(want, "implements", target); got != wantRelation {
		t.Fatalf("golden relation ID mismatch: got %q want %q", got, wantRelation)
	}
	wantProof := "go:package-proof:v1:14:workspace:root#18:go:package:app#app#20:example.com/dep/util#local-replace,workspace-module"
	gotProof := packageProofID("workspace:root", "go:package:app#app", "example.com/dep/util", []proofKind{proofWorkspaceModule, proofLocalReplace, proofLocalReplace})
	if gotProof != wantProof {
		t.Fatalf("golden package-proof ID mismatch: got %q want %q", gotProof, wantProof)
	}
}

func TestPackageIdentityProofPrecedence(t *testing.T) {
	t.Parallel()
	modules := baseModules()

	t.Run("same module", func(t *testing.T) {
		decision := resolveAcrossContexts("example.com/app/lib", []resolutionContext{{
			ID: "single", Kind: contextSingleModule, ImportingModuleID: "app", MainModuleIDs: []string{"app"}, Modules: modules,
		}})
		assertDecision(t, decision, proofResolved, "pkg:app/lib", proofSameModule)
	})

	t.Run("workspace module", func(t *testing.T) {
		decision := resolveAcrossContexts("example.com/nested/pkg", []resolutionContext{{
			ID: "workspace", Kind: contextWorkspace, ImportingModuleID: "app", MainModuleIDs: []string{"app", "nested"}, Modules: modules,
		}})
		assertDecision(t, decision, proofResolved, "pkg:nested", proofWorkspaceModule)
	})

	t.Run("workspace replace overrides module replace", func(t *testing.T) {
		decision := resolveAcrossContexts("example.com/dep/util", []resolutionContext{{
			ID: "workspace", Kind: contextWorkspace, ImportingModuleID: "app", MainModuleIDs: []string{"app"}, Modules: modules,
			WorkspaceReplaces: []replaceFact{{OldPath: "example.com/dep", TargetModuleID: "workspace-dep"}},
		}})
		assertDecision(t, decision, proofResolved, "pkg:workspace-dep/util", proofLocalReplace)
	})

	t.Run("version-specific replace overrides wildcard", func(t *testing.T) {
		copyModules := baseModules()
		app := copyModules["app"]
		app.Replaces = append(app.Replaces, replaceFact{OldPath: "example.com/dep", OldVersion: "v1.0.0", TargetModuleID: "workspace-dep"})
		copyModules["app"] = app
		decision := resolveAcrossContexts("example.com/dep/util", []resolutionContext{{
			ID: "single", Kind: contextSingleModule, ImportingModuleID: "app", MainModuleIDs: []string{"app"}, Modules: copyModules,
		}})
		assertDecision(t, decision, proofResolved, "pkg:workspace-dep/util", proofLocalReplace)
	})

	t.Run("vendor selected before local replacement", func(t *testing.T) {
		decision := resolveAcrossContexts("example.com/dep/util", []resolutionContext{{
			ID: "vendor", Kind: contextModuleVendor, ImportingModuleID: "app", MainModuleIDs: []string{"app"}, Modules: modules,
			VendorPackages: map[string]string{"example.com/dep/util": "pkg:vendor/dep/util"},
		}})
		assertDecision(t, decision, proofResolved, "pkg:vendor/dep/util", proofVendor)
	})

	t.Run("nested module is not owned by parent", func(t *testing.T) {
		single := resolveAcrossContexts("example.com/nested/pkg", []resolutionContext{{
			ID: "single", Kind: contextSingleModule, ImportingModuleID: "app", MainModuleIDs: []string{"app"}, Modules: modules,
		}})
		if single.Status != proofExternal || single.TargetPackageID != "" {
			t.Fatalf("parent crossed nested boundary: %#v", single)
		}
	})

	t.Run("stdlib needs exact index", func(t *testing.T) {
		decision := resolveAcrossContexts("fmt", []resolutionContext{{
			ID: "go1.26.2", Kind: contextSingleModule, ImportingModuleID: "app", MainModuleIDs: []string{"app"}, Modules: modules,
			ExactStandardLibrary: map[string]struct{}{"fmt": {}},
		}})
		assertDecision(t, decision, proofExternal, "", proofStandardLibrary)
	})

	t.Run("replace alone is not active", func(t *testing.T) {
		copyModules := baseModules()
		app := copyModules["app"]
		app.Requires = map[string]string{}
		copyModules["app"] = app
		decision := resolveAcrossContexts("example.com/dep/util", []resolutionContext{{
			ID: "single", Kind: contextSingleModule, ImportingModuleID: "app", MainModuleIDs: []string{"app"}, Modules: copyModules,
		}})
		if decision.Status != proofExternal || decision.TargetPackageID != "" {
			t.Fatalf("replace without require became proof: %#v", decision)
		}
	})

	t.Run("conflicting contexts are ambiguous", func(t *testing.T) {
		left := resolutionContext{ID: "left", Kind: contextWorkspace, ImportingModuleID: "app", MainModuleIDs: []string{"app"}, Modules: modules,
			WorkspaceReplaces: []replaceFact{{OldPath: "example.com/dep", TargetModuleID: "workspace-dep"}}}
		right := resolutionContext{ID: "right", Kind: contextWorkspace, ImportingModuleID: "app", MainModuleIDs: []string{"app"}, Modules: modules,
			WorkspaceReplaces: []replaceFact{{OldPath: "example.com/dep", TargetModuleID: "module-dep"}}}
		decision := resolveAcrossContexts("example.com/dep/util", []resolutionContext{right, left})
		if decision.Status != proofAmbiguous || !reflect.DeepEqual(decision.CandidatePackageIDs, []string{"pkg:module-dep/util", "pkg:workspace-dep/util"}) {
			t.Fatalf("conflict not preserved: %#v", decision)
		}
	})
}

func TestInterfaceCandidatesAreEvidenceBounded(t *testing.T) {
	t.Parallel()
	events := []candidateEvent{
		{Kind: "assignment", ConcreteDeclaration: "T", InterfaceDeclaration: "I"},
		{Kind: "assignment", ConcreteDeclaration: "T", InterfaceDeclaration: "I"},
		{Kind: "assertion", ConcreteDeclaration: "U", InterfaceDeclaration: "I", Pointer: true},
		{Kind: "embedding", ConcreteDeclaration: "V", InterfaceDeclaration: "J"},
		{Kind: "cartesian-scan", ConcreteDeclaration: "Noise", InterfaceDeclaration: "Everything"},
	}
	candidates, omitted := deriveInterfaceCandidates(events, 2)
	if len(candidates) != 2 || omitted != 1 {
		t.Fatalf("unexpected bounded candidates: %#v omitted=%d", candidates, omitted)
	}
	for _, candidate := range candidates {
		if candidate.ConcreteDeclaration == "Noise" {
			t.Fatal("unsupported Cartesian candidate was accepted")
		}
	}
}

func TestCancellationCheckpoints(t *testing.T) {
	t.Parallel()
	var source strings.Builder
	source.WriteString("package cancellation\nfunc F() {\n")
	for index := 0; index < 3000; index++ {
		source.WriteString("var value")
		source.WriteString(strings.Repeat("x", index%7))
		source.WriteString(" = 1\n")
	}
	source.WriteString("}\n")
	file, err := parser.ParseFile(token.NewFileSet(), "cancellation.go", source.String(), parser.AllErrors)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	visited, err := inspectWithContext(ctx, file, 1024, func(count int) {
		if count == 1024 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || visited != 1024 {
		t.Fatalf("AST cancellation boundary changed: visited=%d err=%v", visited, err)
	}

	relationshipContext, cancelRelationships := context.WithCancel(context.Background())
	processed, err := processRelationships(relationshipContext, 1000, 256, func(count int) {
		if count == 256 {
			cancelRelationships()
		}
	})
	if !errors.Is(err, context.Canceled) || processed != 256 {
		t.Fatalf("relationship cancellation boundary changed: processed=%d err=%v", processed, err)
	}
}

func TestPackageCheckHonorsCancellationBeforeSynchronousCall(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := checkPackage(ctx, token.NewFileSet(), "cancelled", nil, rejectingImporter{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestDiagnosticStabilityAndLimits(t *testing.T) {
	t.Parallel()
	items := []diagnostic{
		{File: "b.go", Start: 3, End: 4, Severity: "warning", Code: "z", Message: "third"},
		{File: "a.go", Start: 2, End: 3, Severity: "error", Code: "b", Message: "second"},
		{File: "a.go", Start: 1, End: 2, Severity: "warning", Code: "a", Message: "first"},
		{File: "a.go", Start: 1, End: 2, Severity: "warning", Code: "a", Message: "first"},
		{File: "a.go", Start: 4, End: 5, Severity: "warning", Code: "c", Message: "per-file overflow"},
		{File: "b.go", Start: 5, End: 6, Severity: "warning", Code: "y", Message: "global overflow"},
	}
	result := stabilizeDiagnostics(items, 2, 3)
	if len(result.Items) != 3 || result.Omitted != 3 {
		t.Fatalf("unexpected diagnostic limits: %#v", result)
	}
	if result.Items[0].Message != "first" || result.Items[1].Message != "second" {
		t.Fatalf("ordinary diagnostics not stable: %#v", result.Items)
	}
	if result.Items[2].Code != "semantic_diagnostic_limit" || result.Items[2].Message != "3 diagnostics omitted" {
		t.Fatalf("aggregate must be final: %#v", result.Items[2])
	}
	one := stabilizeDiagnostics(items, 2, 1)
	if len(one.Items) != 1 || one.Items[0].Code != "semantic_diagnostic_limit" {
		t.Fatalf("limit one must retain only aggregate: %#v", one)
	}
}

func TestZeroConfigIsForwardCompatibleDefault(t *testing.T) {
	t.Parallel()
	engine, err := New(Config{})
	if err != nil || engine == nil {
		t.Fatalf("zero config should use defaults: engine=%v err=%v", engine, err)
	}
	_, err = New(Config{NodeCheckInterval: -1})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("negative value must fail: %v", err)
	}
}

func TestSpikeHasNoCommandNetworkOrThirdPartyImports(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package: %v", err)
	}
	forbidden := map[string]struct{}{
		"os/exec": {}, "net": {}, "net/http": {}, "net/url": {}, "golang.org/x/tools/go/packages": {},
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse imports in %s: %v", entry.Name(), parseErr)
		}
		for _, specification := range file.Imports {
			importPath := strings.Trim(specification.Path.Value, `"`)
			if _, blocked := forbidden[importPath]; blocked {
				t.Fatalf("forbidden import %q in %s", importPath, entry.Name())
			}
			if strings.Contains(importPath, ".") {
				t.Fatalf("third-party import %q in %s", importPath, entry.Name())
			}
		}
	}
	moduleFile, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatalf("read spike go.mod: %v", err)
	}
	if strings.Contains(string(moduleFile), "require ") || strings.Contains(string(moduleFile), "replace ") {
		t.Fatalf("spike module must have no external requirements:\n%s", moduleFile)
	}
}

type recordingImporter struct{ paths []string }

func (recorder *recordingImporter) Import(importPath string) (*types.Package, error) {
	recorder.paths = append(recorder.paths, importPath)
	return nil, errors.New("external import blocked: " + importPath)
}

func mustRunner(t *testing.T) Runner {
	t.Helper()
	engine, err := New()
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	return engine
}

func baseModules() map[string]moduleFact {
	return map[string]moduleFact{
		"app": {
			ID: "app", Path: "example.com/app", PackagesByRelative: map[string]string{"": "pkg:app", "lib": "pkg:app/lib"},
			Requires: map[string]string{"example.com/dep": "v1.0.0"},
			Replaces: []replaceFact{{OldPath: "example.com/dep", TargetModuleID: "module-dep"}},
		},
		"nested":        {ID: "nested", Path: "example.com/nested", PackagesByRelative: map[string]string{"pkg": "pkg:nested"}},
		"module-dep":    {ID: "module-dep", Path: "local/module-dep", PackagesByRelative: map[string]string{"util": "pkg:module-dep/util"}},
		"workspace-dep": {ID: "workspace-dep", Path: "local/workspace-dep", PackagesByRelative: map[string]string{"util": "pkg:workspace-dep/util"}},
	}
}

func assertDecision(t *testing.T, decision identityDecision, status proofStatus, target string, kind proofKind) {
	t.Helper()
	if decision.Status != status || decision.TargetPackageID != target || !reflect.DeepEqual(decision.Kinds, []proofKind{kind}) {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestResultOrderingIsAlreadySorted(t *testing.T) {
	t.Parallel()
	result, err := mustRunner(t).Run(context.Background(), Input{Files: []SourceFile{
		{Path: "z.go", Content: "package p\nvar Z = 1\n"},
		{Path: "a.go", Content: "package p\nvar A = 1\n"},
	}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !sort.StringsAreSorted(result.DeclarationIDs) || !sort.StringsAreSorted(result.TypeErrors) {
		t.Fatalf("result is not stable-sorted: %#v", result)
	}
}
