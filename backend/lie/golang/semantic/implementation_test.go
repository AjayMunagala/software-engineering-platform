package semantic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	golang "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

func TestVerifiedSourceProducesDeclarationsWithoutLaterRelationships(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"go.mod":  "module example.com/app\n\ngo 1.22\n",
		"main.go": "package main\nfunc main() {}\n",
	})
	inventory := resolve(t, input, nil)
	files := inventory.Files()
	if len(files) != 1 || files[0].Status != SemanticFilePartial || files[0].ContentDigest == "" {
		t.Fatalf("verified files = %+v", files)
	}
	declarations := inventory.Declarations()
	if len(declarations) != 1 || declarations[0].Name != "main" || declarations[0].Kind != DeclarationFunction || declarations[0].SyntaxSymbolID == "" || declarations[0].Status != ResolutionResolved {
		t.Fatalf("declarations = %+v", declarations)
	}
	if len(inventory.References()) != 0 || len(inventory.ReceiverBindings()) != 0 || len(inventory.ImportBindings()) != 0 || len(inventory.TypeRelations()) != 0 || len(inventory.InterfaceSatisfaction()) != 0 {
		t.Fatal("Phase 2.2.3 emitted unauthorized later semantic relationships")
	}
	statistics := inventory.Statistics()
	if statistics.CandidateFiles != 1 || statistics.PartialFiles != 1 || statistics.ResolvedDeclarations != 1 || statistics.Diagnostics != 0 {
		t.Fatalf("statistics = %+v", statistics)
	}
	if inventory.Metadata().IDSchemeVersion != IDSchemeVersion || inventory.Language() != "Go" {
		t.Fatalf("metadata = %+v language=%s", inventory.Metadata(), inventory.Language())
	}
}

func TestDeclarationReconciliationAndLocalDeclarationInventory(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"sample.go": `package sample

type Embedded[T any] struct{}
type Box[T any] struct {
	Value T
	*Embedded[T]
}
type Runner interface {
	Run(input string) error
}
type Count int
type Alias = string

const (
	Alpha = 1
	Beta = 2
)
var One, Two int

func Work[T any](input T) (result T) {
	value := input
	value, other := input, input
	{
		value := input
		_ = value
	}
	var explicit T
	const localConstant = 1
	type Local struct { Field T }
	for index, item := range []T{input} {
		_, _ = index, item
	}
Label:
	_, _, _, _ = value, other, explicit, localConstant
	_ = Local{}
	if result == input { goto Label }
	return input
}

func (box *Box[T]) Method(value T) {}
`,
	})
	inventory := resolve(t, input, nil)
	declarations := inventory.Declarations()
	if len(declarations) == 0 {
		t.Fatal("no semantic declarations emitted")
	}

	topLevel := make(map[string]SemanticDeclaration)
	for _, declaration := range declarations {
		if declaration.OwnerDeclarationID == "" {
			topLevel[declaration.Name] = declaration
		}
	}
	for _, name := range []string{"Embedded", "Box", "Runner", "Alpha", "Beta", "One", "Two", "Work", "Method"} {
		declaration, ok := topLevel[name]
		if !ok || declaration.SyntaxSymbolID == "" || declaration.Status != ResolutionResolved {
			t.Fatalf("top-level declaration %s was not reconciled: %+v", name, declaration)
		}
	}
	if topLevel["Count"].Kind != DeclarationDefinedType || topLevel["Count"].SyntaxSymbolID != "" {
		t.Fatalf("defined type = %+v", topLevel["Count"])
	}
	if topLevel["Alias"].Kind != DeclarationTypeAlias || topLevel["Alias"].SyntaxSymbolID != "" {
		t.Fatalf("type alias = %+v", topLevel["Alias"])
	}
	if topLevel["Work"].TypeDisplay == "" || topLevel["Box"].TypeDisplay == "" {
		t.Fatal("normalized type displays were not retained")
	}

	workID := topLevel["Work"].ID
	counts := make(map[string]int)
	for _, declaration := range declarations {
		if declaration.OwnerDeclarationID == workID {
			counts[declaration.Kind.String()+":"+declaration.Name]++
		}
	}
	for _, key := range []string{
		"type-parameter:T", "parameter:input", "result:result", "variable:other", "variable:explicit",
		"constant:localConstant", "struct:Local", "variable:index", "variable:item", "label:Label",
	} {
		if counts[key] == 0 {
			t.Fatalf("missing owned declaration %s; counts=%+v", key, counts)
		}
	}
	if counts["variable:value"] != 2 {
		t.Fatalf("lexical value declarations = %d, want outer and nested declarations only", counts["variable:value"])
	}

	local := findDeclaration(t, declarations, "Local", DeclarationStruct)
	field := findDeclaration(t, declarations, "Field", DeclarationField)
	if field.OwnerDeclarationID != local.ID {
		t.Fatalf("local field owner = %s, want %s", field.OwnerDeclarationID, local.ID)
	}
	box := topLevel["Box"]
	if findDeclaration(t, declarations, "Value", DeclarationField).OwnerDeclarationID != box.ID || findDeclaration(t, declarations, "Embedded", DeclarationField).OwnerDeclarationID != box.ID {
		t.Fatal("struct fields do not retain their type owner")
	}
	runner := topLevel["Runner"]
	runMethod := findDeclaration(t, declarations, "Run", DeclarationMethod)
	if runMethod.OwnerDeclarationID != runner.ID {
		t.Fatal("interface method does not retain its interface owner")
	}
	if !hasOwnedDeclaration(declarations, runMethod.ID, "input", DeclarationParameter) {
		t.Fatal("interface method parameter does not retain its method owner")
	}
	if inventory.Statistics().ResolvedDeclarations != len(declarations) || inventory.Statistics().PartialDeclarations != 0 {
		t.Fatalf("declaration statistics = %+v, declarations=%d", inventory.Statistics(), len(declarations))
	}
}

func TestPackageScopeConflictsAreExplicitAndInitIsAllowed(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"a.go": "package sample\nvar Shared int\nfunc init() {}\n",
		"b.go": "package sample\nfunc Shared() {}\nfunc init() {}\n",
	})
	inventory := resolve(t, input, nil)
	shared := make([]SemanticDeclaration, 0, 2)
	initCount := 0
	for _, declaration := range inventory.Declarations() {
		switch declaration.Name {
		case "Shared":
			shared = append(shared, declaration)
		case "init":
			initCount++
			if declaration.Status != ResolutionResolved {
				t.Fatalf("init declaration = %+v", declaration)
			}
		}
	}
	if len(shared) != 2 || shared[0].Status != ResolutionAmbiguous || shared[1].Status != ResolutionAmbiguous || initCount != 2 {
		t.Fatalf("shared=%+v initCount=%d", shared, initCount)
	}
	if !hasDiagnostic(inventory.Diagnostics(), "semantic_package_scope_conflict") || inventory.Statistics().AmbiguousDeclarations != 2 {
		t.Fatalf("diagnostics=%+v statistics=%+v", inventory.Diagnostics(), inventory.Statistics())
	}
}

func TestReceiverBindingIsLocalExactAndExplicit(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"types.go": `package sample
type Worker struct{}
type Scalar int
type Generic[T any] struct{}
type Alias = Worker
type Contract interface{ Run() }
`,
		"methods.go": `package sample
func (Worker) Value() {}
func (*Scalar) Pointer() {}
func (value *Generic[T]) GenericMethod() {}
func (Missing) MissingMethod() {}
func (Alias) AliasMethod() {}
func (Contract) InterfaceMethod() {}
`,
	})
	inventory := resolve(t, input, nil)
	bindings := inventory.ReceiverBindings()
	if len(bindings) != 6 {
		t.Fatalf("receiver bindings = %+v", bindings)
	}
	worker := findDeclaration(t, inventory.Declarations(), "Worker", DeclarationStruct)
	scalar := findDeclaration(t, inventory.Declarations(), "Scalar", DeclarationDefinedType)
	generic := findDeclaration(t, inventory.Declarations(), "Generic", DeclarationStruct)

	value := findReceiverBinding(t, bindings, "Value")
	if value.Status != ResolutionResolved || value.ReceiverTypeDeclarationID != worker.ID || value.Pointer || value.Generic {
		t.Fatalf("value receiver = %+v", value)
	}
	pointer := findReceiverBinding(t, bindings, "Pointer")
	if pointer.Status != ResolutionResolved || pointer.ReceiverTypeDeclarationID != scalar.ID || !pointer.Pointer || pointer.Generic {
		t.Fatalf("pointer receiver = %+v", pointer)
	}
	genericBinding := findReceiverBinding(t, bindings, "GenericMethod")
	if genericBinding.Status != ResolutionResolved || genericBinding.ReceiverTypeDeclarationID != generic.ID || !genericBinding.Pointer || !genericBinding.Generic {
		t.Fatalf("generic receiver = %+v", genericBinding)
	}
	genericMethod := findDeclaration(t, inventory.Declarations(), "GenericMethod", DeclarationMethod)
	if !hasOwnedDeclaration(inventory.Declarations(), genericMethod.ID, "T", DeclarationTypeParameter) {
		t.Fatal("generic receiver type parameter was not declared in method scope")
	}
	for _, method := range []string{"MissingMethod", "AliasMethod", "InterfaceMethod"} {
		binding := findReceiverBinding(t, bindings, method)
		if binding.Status != ResolutionUnresolved || binding.ReceiverTypeDeclarationID != "" {
			t.Fatalf("unsupported receiver %s = %+v", method, binding)
		}
	}
	if inventory.Statistics().ReceiverBindings != len(bindings) || !hasDiagnostic(inventory.Diagnostics(), "semantic_receiver_unresolved") {
		t.Fatalf("statistics=%+v diagnostics=%+v", inventory.Statistics(), inventory.Diagnostics())
	}
}

func TestDuplicateReceiverTypeIsAmbiguous(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"a.go":      "package duplicate\ntype Duplicate struct{}\n",
		"b.go":      "package duplicate\ntype Duplicate int\n",
		"method.go": "package duplicate\nfunc (Duplicate) Method() {}\n",
	})
	inventory := resolve(t, input, nil)
	binding := findReceiverBinding(t, inventory.ReceiverBindings(), "Method")
	if binding.Status != ResolutionAmbiguous || binding.ReceiverTypeDeclarationID != "" || !hasDiagnostic(inventory.Diagnostics(), "semantic_receiver_ambiguous") {
		t.Fatalf("binding=%+v diagnostics=%+v", binding, inventory.Diagnostics())
	}
}

func TestLocalTypeRelationsAreBoundWithoutImportResolution(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"types.go": `package relations
type Numeric interface { ~int | ~int64 }
type Node[T any] struct { Next *Node[T] }
type Box[T Numeric] struct {
	*Node[T]
	Value T
}
type Alias = Box[int]
type Count int
type Handler func(Box[string]) *Node[int]
type External pkg.Type
`,
	})
	inventory := resolve(t, input, nil)
	declarations := inventory.Declarations()
	relations := inventory.TypeRelations()
	if len(relations) == 0 || inventory.Statistics().TypeRelations != len(relations) {
		t.Fatalf("relations=%+v statistics=%+v", relations, inventory.Statistics())
	}
	box := findDeclaration(t, declarations, "Box", DeclarationStruct)
	node := findDeclaration(t, declarations, "Node", DeclarationStruct)
	alias := findDeclaration(t, declarations, "Alias", DeclarationTypeAlias)
	count := findDeclaration(t, declarations, "Count", DeclarationDefinedType)
	external := findDeclaration(t, declarations, "External", DeclarationDefinedType)

	aliasRelation := findTypeRelation(t, relations, TypeRelationAliasOf, alias.ID)
	if aliasRelation.Status != ResolutionResolved || aliasRelation.TargetDeclarationID != box.ID || !reflect.DeepEqual(aliasRelation.TypeArgumentText, []string{"int"}) {
		t.Fatalf("alias relation = %+v", aliasRelation)
	}
	embedRelation := findTypeRelation(t, relations, TypeRelationEmbeds, box.ID)
	if embedRelation.Status != ResolutionResolved || embedRelation.TargetDeclarationID != node.ID || !reflect.DeepEqual(embedRelation.TypeArgumentText, []string{"T"}) {
		t.Fatalf("embed relation = %+v", embedRelation)
	}
	boxParameter := ownedDeclaration(t, declarations, box.ID, "T", DeclarationTypeParameter)
	constraint := findTypeRelation(t, relations, TypeRelationConstrains, boxParameter.ID)
	if constraint.Status != ResolutionResolved || constraint.TargetDeclarationID != findDeclaration(t, declarations, "Numeric", DeclarationInterface).ID {
		t.Fatalf("constraint relation = %+v", constraint)
	}
	countUse := findTypeRelation(t, relations, TypeRelationUses, count.ID)
	if countUse.Status != ResolutionExternal || countUse.TargetIdentity != "builtin:int" {
		t.Fatalf("predeclared use = %+v", countUse)
	}
	externalUse := findTypeRelation(t, relations, TypeRelationUses, external.ID)
	if externalUse.Status != ResolutionUnresolved || externalUse.TargetIdentity != "pkg.Type" || externalUse.TargetDeclarationID != "" {
		t.Fatalf("qualified use was guessed before import resolution: %+v", externalUse)
	}
	if !hasTypeRelationKind(relations, TypeRelationInstantiates) {
		t.Fatal("generic instantiation relations were not emitted")
	}
	if len(inventory.References()) == 0 || len(inventory.ImportBindings()) != 0 || len(inventory.InterfaceSatisfaction()) != 0 {
		t.Fatal("Phase 2.2.5 references should be present without inventing imports or interface results")
	}
}

func TestReceiverAndTypeRelationsAreDeterministicAcrossWorkers(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"types.go":  "package deterministic\ntype Box[T any] struct { Value T }\n",
		"method.go": "package deterministic\nfunc (box *Box[T]) Value(input Box[T]) T { return box.Value }\n",
	})
	one := DefaultConfig()
	one.MaxWorkers = 1
	eight := DefaultConfig()
	eight.MaxWorkers = 8
	left, right := resolve(t, input, &one), resolve(t, input, &eight)
	if !reflect.DeepEqual(left.ReceiverBindings(), right.ReceiverBindings()) || !reflect.DeepEqual(left.TypeRelations(), right.TypeRelations()) || !reflect.DeepEqual(left.Diagnostics(), right.Diagnostics()) {
		t.Fatal("worker count changed receiver/type output")
	}
}

func TestReferencesAndDefaultImportResolveAcrossLocalPackages(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"go.mod":        "module example.com/app\n\ngo 1.26\n",
		"lib/lib.go":    "package lib\nfunc Value() int { return 1 }\n",
		"app/helper.go": "package app\nfunc helper() int { return 2 }\n",
		"app/main.go":   "package app\nimport \"example.com/app/lib\"\nfunc Run() int { return lib.Value() + helper() }\n",
	})
	inventory := resolve(t, input, nil)
	binding := findImportBinding(t, inventory.ImportBindings(), "example.com/app/lib")
	if binding.Status != ResolutionResolved || binding.LocalName != "lib" || binding.TargetPackageID == "" || binding.PackageIdentityProofID == "" {
		t.Fatalf("import binding = %+v", binding)
	}
	value := findDeclaration(t, inventory.Declarations(), "Value", DeclarationFunction)
	helper := findDeclaration(t, inventory.Declarations(), "helper", DeclarationFunction)
	if reference := findReference(t, inventory.References(), "Value", ReferenceSelector); reference.Status != ResolutionResolved || reference.TargetDeclarationID != value.ID {
		t.Fatalf("selector reference = %+v", reference)
	}
	if reference := findReference(t, inventory.References(), "helper", ReferenceIdentifier); reference.Status != ResolutionResolved || reference.TargetDeclarationID != helper.ID {
		t.Fatalf("cross-file reference = %+v", reference)
	}
	if inventory.Statistics().ReferencesByStatus[ResolutionResolved.String()] < 2 || inventory.Statistics().ImportBindingsByStatus[ResolutionResolved.String()] != 1 {
		t.Fatalf("statistics = %+v", inventory.Statistics())
	}
}

func TestNamedExternalBlankAndDotImportsAreIntentional(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.26\n",
		"shared/x.go": "package shared\nfunc Shared() int { return 1 }\n",
		"main.go": `package app
import (
    ext "example.net/external"
    _ "example.net/driver"
    . "example.com/app/shared"
)
func Run() int { ext.Call(); return Shared() }
`,
	})
	inventory := resolve(t, input, nil)
	external := findImportBinding(t, inventory.ImportBindings(), "example.net/external")
	blank := findImportBinding(t, inventory.ImportBindings(), "example.net/driver")
	dot := findImportBinding(t, inventory.ImportBindings(), "example.com/app/shared")
	if external.Status != ResolutionExternal || external.LocalName != "ext" || external.PackageIdentityProofID == "" {
		t.Fatalf("external import = %+v", external)
	}
	if blank.Status != ResolutionExternal || blank.LocalName != "_" || blank.AliasKind != "blank" {
		t.Fatalf("blank import = %+v", blank)
	}
	if dot.Status != ResolutionResolved || dot.LocalName != "." || dot.AliasKind != "dot" {
		t.Fatalf("dot import = %+v", dot)
	}
	call := findReference(t, inventory.References(), "Call", ReferenceSelector)
	if call.Status != ResolutionExternal || call.ExternalIdentity != "example.net/external.Call" {
		t.Fatalf("external selector = %+v", call)
	}
	shared := findDeclaration(t, inventory.Declarations(), "Shared", DeclarationFunction)
	used := findReference(t, inventory.References(), "Shared", ReferenceIdentifier)
	if used.Status != ResolutionResolved || used.TargetDeclarationID != shared.ID {
		t.Fatalf("dot-import reference = %+v", used)
	}
}

func TestLexicalShadowingWinsAndAmbiguousPackageReferencesStayExplicit(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"a.go": "package scope\nvar Value = 1\nvar Duplicate = 1\n",
		"b.go": "package scope\nvar Duplicate = 2\nfunc Run() int { Value := 3; _ = Value; return Duplicate }\n",
	})
	inventory := resolve(t, input, nil)
	run := findDeclaration(t, inventory.Declarations(), "Run", DeclarationFunction)
	local := ownedDeclaration(t, inventory.Declarations(), run.ID, "Value", DeclarationVariable)
	var shadowed SemanticReference
	for _, reference := range inventory.References() {
		if reference.Name == "Value" && reference.OwnerDeclarationID == run.ID && reference.Status == ResolutionResolved {
			shadowed = reference
		}
	}
	if shadowed.TargetDeclarationID != local.ID {
		t.Fatalf("shadowed reference = %+v, local = %+v", shadowed, local)
	}
	ambiguous := findReference(t, inventory.References(), "Duplicate", ReferenceIdentifier)
	if ambiguous.Status != ResolutionAmbiguous || len(ambiguous.CandidateDeclarationIDs) != 2 || !sort.StringsAreSorted(ambiguous.CandidateDeclarationIDs) {
		t.Fatalf("ambiguous reference = %+v", ambiguous)
	}
}

func TestLocalValueShadowsImportAliasAndUnexportedImportsDoNotResolve(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"go.mod":     "module example.com/app\n\ngo 1.26\n",
		"lib/lib.go": "package lib\nfunc Hidden() {}\nfunc hidden() {}\n",
		"main.go": `package app
import "example.com/app/lib"
func Imported() { lib.hidden() }
func Shadowed() { lib := struct{ Hidden func() }{}; lib.Hidden() }
`,
	})
	inventory := resolve(t, input, nil)
	for _, reference := range inventory.References() {
		if reference.Name == "hidden" && reference.Kind == ReferenceSelector && reference.Status != ResolutionUnresolved {
			t.Fatalf("unexported imported selector resolved: %+v", reference)
		}
	}
	shadowed := findDeclaration(t, inventory.Declarations(), "Shadowed", DeclarationFunction)
	local := ownedDeclaration(t, inventory.Declarations(), shadowed.ID, "lib", DeclarationVariable)
	base := findOwnedReference(t, inventory.References(), shadowed.ID, "lib", ReferenceIdentifier)
	if base.Status != ResolutionResolved || base.TargetDeclarationID != local.ID {
		t.Fatalf("shadowed import base = %+v", base)
	}
	selector := findOwnedReference(t, inventory.References(), shadowed.ID, "Hidden", ReferenceSelector)
	if selector.Status != ResolutionUnresolved || selector.ExternalIdentity != "" {
		t.Fatalf("local field selector was treated as import: %+v", selector)
	}
}

func TestStalePackageProofPreventsImportResolution(t *testing.T) {
	root, input := prerequisitesAt(t, t.TempDir(), map[string]string{
		"go.mod":     "module example.com/app\n\ngo 1.26\n",
		"lib/lib.go": "package lib\nfunc Value() {}\n",
		"main.go":    "package app\nimport \"example.com/app/lib\"\nfunc Run() { lib.Value() }\n",
	})
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/changed\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	inventory := resolve(t, input, nil)
	binding := findImportBinding(t, inventory.ImportBindings(), "example.com/app/lib")
	if binding.Status != ResolutionUnresolved || binding.TargetPackageID != "" || binding.PackageIdentityProofID != "" || !hasDiagnostic(inventory.Diagnostics(), "semantic_package_proof_stale") {
		t.Fatalf("binding=%+v diagnostics=%+v", binding, inventory.Diagnostics())
	}
}

func TestApplicablePackageProofsRequireUnanimousFreshAgreement(t *testing.T) {
	reader, err := newSourceReader(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.close()
	proof := func(id, target string, status packageidentity.ProofStatus) packageidentity.PackageIdentityProof {
		return packageidentity.PackageIdentityProof{ID: id, TargetPackageID: target, Status: status}
	}
	status, target, proofID, stale := combinePackageProofs(reader, []packageidentity.PackageIdentityProof{
		proof("a", "pkg", packageidentity.ProofResolved), proof("b", "pkg", packageidentity.ProofResolved),
	}, map[string]bool{}, defaultMaxSourceFileSize)
	if status != ResolutionResolved || target != "pkg" || proofID != "a" || stale {
		t.Fatalf("unanimous result = %s %q %q stale=%t", status, target, proofID, stale)
	}
	status, _, _, _ = combinePackageProofs(reader, []packageidentity.PackageIdentityProof{
		proof("a", "one", packageidentity.ProofResolved), proof("b", "two", packageidentity.ProofResolved),
	}, map[string]bool{}, defaultMaxSourceFileSize)
	if status != ResolutionAmbiguous {
		t.Fatalf("conflicting result = %s", status)
	}
	status, _, _, _ = combinePackageProofs(reader, []packageidentity.PackageIdentityProof{
		proof("a", "pkg", packageidentity.ProofResolved), proof("b", "", packageidentity.ProofExternal),
	}, map[string]bool{}, defaultMaxSourceFileSize)
	if status != ResolutionUnresolved {
		t.Fatalf("incomplete context result = %s", status)
	}
}

func TestReferencesAndImportsAreDeterministicAcrossWorkers(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"go.mod":     "module example.com/app\n\ngo 1.26\n",
		"lib/lib.go": "package lib\nfunc Value() int { return 1 }\n",
		"main.go":    "package app\nimport \"example.com/app/lib\"\nfunc Run() int { return lib.Value() }\n",
	})
	one, eight := DefaultConfig(), DefaultConfig()
	one.MaxWorkers, eight.MaxWorkers = 1, 8
	left, right := resolve(t, input, &one), resolve(t, input, &eight)
	if !reflect.DeepEqual(left.References(), right.References()) || !reflect.DeepEqual(left.ImportBindings(), right.ImportBindings()) || !reflect.DeepEqual(left.Diagnostics(), right.Diagnostics()) {
		t.Fatal("worker count changed reference/import output")
	}
}

func TestRelationshipBudgetPreservesImportsBeforeDerivedReferences(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"go.mod":     "module example.com/app\n\ngo 1.26\n",
		"lib/lib.go": "package lib\nfunc Value() int { return 1 }\n",
		"main.go":    "package app\nimport \"example.com/app/lib\"\nfunc Run() int { return lib.Value() + lib.Value() }\n",
	})
	config := DefaultConfig()
	config.MaxRelationships = 1
	inventory := resolve(t, input, &config)
	if len(inventory.ImportBindings()) != 1 || len(inventory.ReceiverBindings()) != 0 || len(inventory.TypeRelations()) != 0 || len(inventory.References()) != 0 {
		t.Fatalf("budgeted collections: imports=%d receivers=%d types=%d references=%d", len(inventory.ImportBindings()), len(inventory.ReceiverBindings()), len(inventory.TypeRelations()), len(inventory.References()))
	}
	if inventory.Statistics().OmittedRelationships == 0 || !hasDiagnostic(inventory.Diagnostics(), "semantic_relationship_limit") {
		t.Fatalf("statistics=%+v diagnostics=%+v", inventory.Statistics(), inventory.Diagnostics())
	}
}

func TestRelationshipBudgetIsDeterministicAndExplicit(t *testing.T) {
	input := prerequisites(t, map[string]string{
		"types.go": "package budget\ntype Box[T any] struct { Value T }\nfunc (box *Box[T]) Method(input Box[T]) {}\n",
	})
	config := DefaultConfig()
	config.MaxRelationships = 2
	inventory := resolve(t, input, &config)
	total := len(inventory.ReceiverBindings()) + len(inventory.TypeRelations())
	if total != 2 || inventory.Statistics().OmittedRelationships == 0 || !hasDiagnostic(inventory.Diagnostics(), "semantic_relationship_limit") {
		t.Fatalf("relationships=%d statistics=%+v diagnostics=%+v", total, inventory.Statistics(), inventory.Diagnostics())
	}
	if inventory.Statistics().ReceiverBindings != len(inventory.ReceiverBindings()) || inventory.Statistics().TypeRelations != len(inventory.TypeRelations()) {
		t.Fatalf("emitted relationship statistics are inconsistent: %+v", inventory.Statistics())
	}
}

func TestUnicodeGroupedDeclarationsAndStableIDs(t *testing.T) {
	const relativePath = "pkg/μ.go"
	input := prerequisites(t, map[string]string{
		relativePath: "package unicode\n\n// grouped Unicode declarations\nvar (\n\tα int\n\tβ string\n)\n\ntype Pair[Τ any] struct { Value Τ }\n",
	})
	one := DefaultConfig()
	one.MaxWorkers = 1
	eight := DefaultConfig()
	eight.MaxWorkers = 8
	left, right := resolve(t, input, &one), resolve(t, input, &eight)
	if !reflect.DeepEqual(left.Declarations(), right.Declarations()) {
		t.Fatal("worker count changed Unicode declaration output")
	}
	syntaxByID := make(map[string]golang.GoSymbol)
	for _, symbol := range input.Syntax.Symbols() {
		syntaxByID[symbol.ID] = symbol
	}
	for _, name := range []string{"α", "β", "Pair"} {
		declaration := findDeclarationByName(t, left.Declarations(), name)
		if declaration.SyntaxSymbolID == "" || declaration.Location != syntaxByID[declaration.SyntaxSymbolID].Location {
			t.Fatalf("declaration %s location/reconciliation = %+v, syntax=%+v", name, declaration, syntaxByID[declaration.SyntaxSymbolID])
		}
		expectedPrefix := fmt.Sprintf("go:semantic:v1:file:%d:%s#%d:", len(relativePath), relativePath, declaration.Location.Start.Offset)
		if !strings.HasPrefix(declaration.ID, expectedPrefix) {
			t.Fatalf("declaration ID %q does not use byte-length/byte-offset prefix %q", declaration.ID, expectedPrefix)
		}
	}
	parameter := findDeclaration(t, left.Declarations(), "Τ", DeclarationTypeParameter)
	if parameter.OwnerDeclarationID != findDeclarationByName(t, left.Declarations(), "Pair").ID {
		t.Fatal("generic type parameter owner is not stable")
	}
}

func TestEmptyRepositoryProducesValidEmptyArtifact(t *testing.T) {
	inventory := resolve(t, prerequisites(t, map[string]string{"README.md": "notes"}), nil)
	if len(inventory.Files()) != 0 || len(inventory.Diagnostics()) != 0 || inventory.Statistics().CandidateFiles != 0 {
		t.Fatalf("empty artifact files=%+v diagnostics=%+v statistics=%+v", inventory.Files(), inventory.Diagnostics(), inventory.Statistics())
	}
}

func TestSourceMutationOutcomes(t *testing.T) {
	t.Run("stale", func(t *testing.T) {
		root, input := prerequisitesAt(t, t.TempDir(), map[string]string{"main.go": "package one\n"})
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package two\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		inventory := resolve(t, input, nil)
		assertFileOutcome(t, inventory, SemanticFileStale, "semantic_digest_mismatch")
		if inventory.Files()[0].ContentDigest != "" {
			t.Fatal("stale source published an analyzed digest")
		}
	})

	t.Run("missing", func(t *testing.T) {
		root, input := prerequisitesAt(t, t.TempDir(), map[string]string{"main.go": "package main\n"})
		if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
			t.Fatal(err)
		}
		assertFileOutcome(t, resolve(t, input, nil), SemanticFileFailed, "semantic_source_missing")
	})

	t.Run("non-regular", func(t *testing.T) {
		root, input := prerequisitesAt(t, t.TempDir(), map[string]string{"main.go": "package main\n"})
		if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(root, "main.go"), 0o755); err != nil {
			t.Fatal(err)
		}
		assertFileOutcome(t, resolve(t, input, nil), SemanticFileFailed, "semantic_source_unreadable")
	})

	t.Run("grew-oversized", func(t *testing.T) {
		root, input := prerequisitesAt(t, t.TempDir(), map[string]string{"main.go": "package main\n"})
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"+strings.Repeat(" ", 100)), 0o600); err != nil {
			t.Fatal(err)
		}
		config := DefaultConfig()
		config.MaxSourceFileSize = 20
		assertFileOutcome(t, resolve(t, input, &config), SemanticFileSkipped, "semantic_source_oversized")
	})

	t.Run("frozen-size-oversized", func(t *testing.T) {
		input := prerequisites(t, map[string]string{"main.go": "package main\n" + strings.Repeat(" ", 100)})
		config := DefaultConfig()
		config.MaxSourceFileSize = 20
		assertFileOutcome(t, resolve(t, input, &config), SemanticFileSkipped, "semantic_source_oversized")
	})
}

func TestSourceSymlinkEscapeIsRejected(t *testing.T) {
	base := t.TempDir()
	repository := filepath.Join(base, "repository")
	outside := filepath.Join(base, "outside.go")
	if err := os.MkdirAll(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, input := prerequisitesAt(t, repository, map[string]string{"main.go": "package main\n"})
	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "main.go")); err != nil {
		t.Skipf("file symlinks unavailable: %v", err)
	}
	assertFileOutcome(t, resolve(t, input, nil), SemanticFileFailed, "semantic_source_outside_root")
}

func TestFailedAndSkippedSyntaxFilesRemainExplicit(t *testing.T) {
	failed := resolve(t, prerequisites(t, map[string]string{"broken.go": "package broken\nfunc {"}), nil)
	assertFileOutcome(t, failed, SemanticFileFailed, "semantic_prerequisite_file_failed")

	root := t.TempDir()
	goConfig := golang.DefaultConfig()
	goConfig.MaxSourceFileSize = 20
	input := prerequisitesWithGoConfig(t, root, map[string]string{"large.go": "package large\n" + strings.Repeat(" ", 100)}, goConfig)
	skipped := resolve(t, input, nil)
	assertFileOutcome(t, skipped, SemanticFileSkipped, "semantic_prerequisite_file_skipped")
}

func TestInputValidationConfigurationAndCancellation(t *testing.T) {
	if _, err := New(DefaultConfig(), DefaultConfig()); !errors.Is(err, ErrTooManyConfigs) {
		t.Fatalf("too many configs error = %v", err)
	}
	invalidConfigs := []Config{
		{MaxWorkers: -1}, {MaxSourceFileSize: -1}, {MaxPackageFiles: -1}, {MaxPackageBytes: -1},
		{MaxDiagnostics: -1}, {MaxDiagnosticsPerFile: -1}, {MaxRelationships: -1},
	}
	for _, config := range invalidConfigs {
		if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("config %+v error = %v", config, err)
		}
	}

	engine, _ := New()
	if _, err := engine.Resolve(nil, Input{}); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("nil context error = %v", err)
	}
	if _, err := engine.Resolve(context.Background(), Input{}); !errors.Is(err, ErrMissingRepositorySnapshot) {
		t.Fatalf("missing snapshot error = %v", err)
	}
	input := prerequisites(t, map[string]string{"main.go": "package main\n"})
	if _, err := engine.Resolve(context.Background(), Input{Snapshot: input.Snapshot}); !errors.Is(err, ErrMissingGoLanguageInventory) {
		t.Fatalf("missing syntax error = %v", err)
	}
	if _, err := engine.Resolve(context.Background(), Input{Snapshot: input.Snapshot, Syntax: input.Syntax}); !errors.Is(err, ErrMissingPackageIdentityInventory) {
		t.Fatalf("missing identity error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.Resolve(ctx, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}

	other := prerequisites(t, map[string]string{"other.go": "package other\n"})
	input.Snapshot = other.Snapshot
	if _, err := engine.Resolve(context.Background(), input); !errors.Is(err, ErrArtifactProvenanceMismatch) {
		t.Fatalf("provenance error = %v", err)
	}
}

func TestInvalidRepositoryRootIsFatal(t *testing.T) {
	input := prerequisites(t, map[string]string{"main.go": "package main\n"})
	missingRoot := filepath.Join(t.TempDir(), "missing")
	input.Snapshot = rie.NewRepositorySnapshot(missingRoot, input.Snapshot.Entries(), input.Snapshot.Statistics(), nil, "0.2.1")
	engine, _ := New()
	if _, err := engine.Resolve(context.Background(), input); !errors.Is(err, ErrInvalidRepositoryRoot) {
		t.Fatalf("invalid root error = %v", err)
	}
}

func TestArtifactIsDeeplyImmutable(t *testing.T) {
	input := prerequisites(t, map[string]string{"main.go": "package main\n"})
	inventory := resolve(t, input, nil)
	files := inventory.Files()
	files[0].FileID = "changed"
	sources := inventory.SourceArtifacts()
	sources[0].Name = "changed"
	statistics := inventory.Statistics()
	statistics.ReferencesByStatus["resolved"] = 999
	if inventory.Files()[0].FileID == "changed" || inventory.SourceArtifacts()[0].Name == "changed" || inventory.Statistics().ReferencesByStatus["resolved"] == 999 {
		t.Fatal("Phase 2.2.2 artifact state is mutable")
	}

	location := lie.SourceRange{File: "main.go"}
	constructed := GoSemanticInventory{
		references:            []SemanticReference{{CandidateDeclarationIDs: []string{"one"}}},
		typeRelations:         []TypeRelation{{TypeArgumentText: []string{"T"}}},
		interfaceSatisfaction: []InterfaceSatisfaction{{MissingMethodNames: []string{"Run"}, CompileTimeAssertions: []rie.Evidence{{File: "main.go"}}}},
		diagnostics:           []lie.Diagnostic{{Location: &location}},
	}
	references := constructed.References()
	references[0].CandidateDeclarationIDs[0] = "changed"
	relations := constructed.TypeRelations()
	relations[0].TypeArgumentText[0] = "changed"
	satisfaction := constructed.InterfaceSatisfaction()
	satisfaction[0].MissingMethodNames[0] = "changed"
	satisfaction[0].CompileTimeAssertions[0].File = "changed"
	diagnostics := constructed.Diagnostics()
	diagnostics[0].Location.File = "changed"
	if constructed.References()[0].CandidateDeclarationIDs[0] == "changed" || constructed.TypeRelations()[0].TypeArgumentText[0] == "changed" || constructed.InterfaceSatisfaction()[0].MissingMethodNames[0] == "changed" || constructed.InterfaceSatisfaction()[0].CompileTimeAssertions[0].File == "changed" || constructed.Diagnostics()[0].Location.File == "changed" {
		t.Fatal("future semantic collections are not deeply immutable")
	}
}

func TestDeterminismAcrossWorkerCountsAndInputImmutability(t *testing.T) {
	files := map[string]string{"go.mod": "module example.com/app\n\ngo 1.22\n"}
	for index := 0; index < 40; index++ {
		files[fmt.Sprintf("pkg/p%02d/value.go", index)] = fmt.Sprintf("package p%02d\nfunc Value() {}\n", index)
	}
	input := prerequisites(t, files)
	beforeSyntax := input.Syntax.Files()
	beforeProofs := input.PackageIdentities.Proofs()
	one := DefaultConfig()
	one.MaxWorkers = 1
	eight := DefaultConfig()
	eight.MaxWorkers = 8
	left, right := resolve(t, input, &one), resolve(t, input, &eight)
	if !reflect.DeepEqual(left.Files(), right.Files()) || !reflect.DeepEqual(left.Diagnostics(), right.Diagnostics()) || !reflect.DeepEqual(left.Statistics(), right.Statistics()) {
		t.Fatal("worker count changed deterministic output")
	}
	if !reflect.DeepEqual(beforeSyntax, input.Syntax.Files()) || !reflect.DeepEqual(beforeProofs, input.PackageIdentities.Proofs()) {
		t.Fatal("semantic resolution mutated prerequisite artifacts")
	}
}

func TestDiagnosticLimitAndArtifactPublication(t *testing.T) {
	root, input := prerequisitesAt(t, t.TempDir(), map[string]string{
		"a.go": "package sample\n", "b.go": "package sample\n", "c.go": "package sample\n",
	})
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	config := DefaultConfig()
	config.MaxDiagnostics = 2
	inventory := resolve(t, input, &config)
	diagnostics := inventory.Diagnostics()
	if len(diagnostics) != 2 || diagnostics[1].Code != "semantic_diagnostic_limit" || inventory.Statistics().OmittedDiagnostics != 2 {
		t.Fatalf("diagnostic limit diagnostics=%+v statistics=%+v", diagnostics, inventory.Statistics())
	}
	store := rie.NewArtifactStore()
	if err := store.Put(inventory); err != nil {
		t.Fatal(err)
	}
	published, ok := InventoryFrom(store)
	if !ok || published.ArtifactName() != ArtifactName || published.ArtifactVersion() != ArtifactVersion {
		t.Fatalf("published artifact = %+v, %v", published.Metadata(), ok)
	}
}

func TestEnumJSONContracts(t *testing.T) {
	cases := []struct {
		value any
		text  string
	}{
		{ResolutionResolved, "resolved"}, {SemanticFileStale, "stale"}, {DeclarationTypeParameter, "type-parameter"},
		{ReferenceInstantiation, "instantiation"}, {TypeRelationConstrains, "constrains"}, {SatisfactionUnknown, "unknown"},
	}
	for _, test := range cases {
		encoded, err := json.Marshal(test.value)
		if err != nil || string(encoded) != `"`+test.text+`"` {
			t.Fatalf("marshal %T = %s, %v", test.value, encoded, err)
		}
	}
	var status SemanticFileStatus
	if err := json.Unmarshal([]byte(`"partial"`), &status); err != nil || status != SemanticFilePartial {
		t.Fatalf("file status unmarshal = %s, %v", status, err)
	}
	if err := json.Unmarshal([]byte(`"future"`), &status); err == nil {
		t.Fatal("unknown enum value was accepted")
	}
	if _, err := json.Marshal(SemanticFileStatus(255)); err == nil {
		t.Fatal("unknown numeric enum value was marshaled")
	}
}

func TestEveryEnumJSONValueAndInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		values  []any
		texts   []string
		decode  func([]byte) (any, error)
		invalid any
	}{
		{
			name:   "resolution",
			values: []any{ResolutionResolved, ResolutionUnresolved, ResolutionAmbiguous, ResolutionExternal, ResolutionPartial},
			texts:  []string{"resolved", "unresolved", "ambiguous", "external", "partial"},
			decode: func(data []byte) (any, error) {
				var value ResolutionStatus
				err := json.Unmarshal(data, &value)
				return value, err
			},
			invalid: ResolutionStatus(0),
		},
		{
			name:   "file",
			values: []any{SemanticFileResolved, SemanticFilePartial, SemanticFileFailed, SemanticFileStale, SemanticFileSkipped},
			texts:  []string{"resolved", "partial", "failed", "stale", "skipped"},
			decode: func(data []byte) (any, error) {
				var value SemanticFileStatus
				err := json.Unmarshal(data, &value)
				return value, err
			},
			invalid: SemanticFileStatus(0),
		},
		{
			name:   "declaration",
			values: []any{DeclarationStruct, DeclarationInterface, DeclarationDefinedType, DeclarationTypeAlias, DeclarationFunction, DeclarationMethod, DeclarationField, DeclarationParameter, DeclarationResult, DeclarationVariable, DeclarationConstant, DeclarationLabel, DeclarationTypeParameter},
			texts:  []string{"struct", "interface", "defined-type", "type-alias", "function", "method", "field", "parameter", "result", "variable", "constant", "label", "type-parameter"},
			decode: func(data []byte) (any, error) {
				var value DeclarationKind
				err := json.Unmarshal(data, &value)
				return value, err
			},
			invalid: DeclarationKind(0),
		},
		{
			name:   "reference",
			values: []any{ReferenceIdentifier, ReferenceSelector, ReferenceType, ReferenceInstantiation},
			texts:  []string{"identifier", "selector", "type", "instantiation"},
			decode: func(data []byte) (any, error) {
				var value ReferenceKind
				err := json.Unmarshal(data, &value)
				return value, err
			},
			invalid: ReferenceKind(0),
		},
		{
			name:   "relation",
			values: []any{TypeRelationUses, TypeRelationEmbeds, TypeRelationAliasOf, TypeRelationInstantiates, TypeRelationConstrains},
			texts:  []string{"uses", "embeds", "alias-of", "instantiates", "constrains"},
			decode: func(data []byte) (any, error) {
				var value TypeRelationKind
				err := json.Unmarshal(data, &value)
				return value, err
			},
			invalid: TypeRelationKind(0),
		},
		{
			name:   "satisfaction",
			values: []any{SatisfactionProven, SatisfactionDisproven, SatisfactionUnknown},
			texts:  []string{"proven", "disproven", "unknown"},
			decode: func(data []byte) (any, error) {
				var value SatisfactionStatus
				err := json.Unmarshal(data, &value)
				return value, err
			},
			invalid: SatisfactionStatus(0),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for index, expected := range test.values {
				data := []byte(`"` + test.texts[index] + `"`)
				actual, err := test.decode(data)
				if err != nil || !reflect.DeepEqual(actual, expected) {
					t.Fatalf("decode %s = %#v, %v; want %#v", data, actual, err, expected)
				}
				encoded, err := json.Marshal(expected)
				if err != nil || string(encoded) != string(data) {
					t.Fatalf("encode %#v = %s, %v; want %s", expected, encoded, err, data)
				}
			}
			if _, err := json.Marshal(test.invalid); err == nil {
				t.Fatalf("invalid value %#v was marshaled", test.invalid)
			}
			if _, err := test.decode([]byte(`"future"`)); err == nil {
				t.Fatal("unknown textual value was accepted")
			}
			if _, err := test.decode([]byte(`123`)); err == nil {
				t.Fatal("non-string JSON value was accepted")
			}
		})
	}
}

func TestEngineMetadataAndEmptyAccessors(t *testing.T) {
	created, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if created.Name() != "go-semantic" || created.Version() != engineVersion || created.Language() != "Go" || created.ArtifactName() != ArtifactName || created.Description() == "" {
		t.Fatalf("engine metadata is incomplete: %s %s %s %s %s", created.Name(), created.Version(), created.Language(), created.ArtifactName(), created.Description())
	}
	inventory := resolve(t, prerequisites(t, map[string]string{"README.md": "notes"}), nil)
	if inventory.ArtifactName() != ArtifactName || inventory.ArtifactVersion() != ArtifactVersion {
		t.Fatalf("artifact identity = %s@%s", inventory.ArtifactName(), inventory.ArtifactVersion())
	}
	if len(inventory.ReceiverBindings()) != 0 || len(inventory.ImportBindings()) != 0 || len(inventory.TypeRelations()) != 0 || len(inventory.InterfaceSatisfaction()) != 0 {
		t.Fatal("Phase 2.2.2 emitted future semantic collections")
	}
}

func TestDiagnosticDeduplicationAndPerFileLimit(t *testing.T) {
	duplicate := semanticDiagnostic(lie.SeverityWarning, "duplicate", "same", "a.go")
	values := []lie.Diagnostic{
		duplicate,
		duplicate,
		semanticDiagnostic(lie.SeverityWarning, "second", "second", "a.go"),
		semanticDiagnostic(lie.SeverityWarning, "third", "third", "b.go"),
	}
	sortDiagnostics(values)
	kept, omitted := limitDiagnostics(values, 1, 10)
	if omitted != 1 || len(kept) != 3 || kept[2].Code != "semantic_diagnostic_limit" || kept[2].Location != nil {
		t.Fatalf("limited diagnostics = %+v, omitted=%d", kept, omitted)
	}
}

func assertFileOutcome(t *testing.T, inventory GoSemanticInventory, status SemanticFileStatus, diagnosticCode string) {
	t.Helper()
	if len(inventory.Files()) != 1 || inventory.Files()[0].Status != status {
		t.Fatalf("files = %+v, want status %s", inventory.Files(), status)
	}
	if diagnosticCode == "" {
		return
	}
	for _, diagnostic := range inventory.Diagnostics() {
		if diagnostic.Code == diagnosticCode {
			return
		}
	}
	t.Fatalf("diagnostic %q not found in %+v", diagnosticCode, inventory.Diagnostics())
}

func findDeclaration(t *testing.T, declarations []SemanticDeclaration, name string, kind DeclarationKind) SemanticDeclaration {
	t.Helper()
	for _, declaration := range declarations {
		if declaration.Name == name && declaration.Kind == kind {
			return declaration
		}
	}
	t.Fatalf("declaration %s (%s) not found in %+v", name, kind, declarations)
	return SemanticDeclaration{}
}

func findDeclarationByName(t *testing.T, declarations []SemanticDeclaration, name string) SemanticDeclaration {
	t.Helper()
	for _, declaration := range declarations {
		if declaration.Name == name {
			return declaration
		}
	}
	t.Fatalf("declaration %s not found in %+v", name, declarations)
	return SemanticDeclaration{}
}

func hasOwnedDeclaration(declarations []SemanticDeclaration, owner, name string, kind DeclarationKind) bool {
	for _, declaration := range declarations {
		if declaration.OwnerDeclarationID == owner && declaration.Name == name && declaration.Kind == kind {
			return true
		}
	}
	return false
}

func hasDiagnostic(diagnostics []lie.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func findReceiverBinding(t *testing.T, bindings []ReceiverBinding, methodName string) ReceiverBinding {
	t.Helper()
	for _, binding := range bindings {
		if strings.HasSuffix(binding.MethodDeclarationID, ":method:"+methodName) {
			return binding
		}
	}
	t.Fatalf("receiver binding for %s not found in %+v", methodName, bindings)
	return ReceiverBinding{}
}

func findImportBinding(t *testing.T, bindings []ImportBinding, importPath string) ImportBinding {
	t.Helper()
	for _, binding := range bindings {
		if binding.ImportPath == importPath {
			return binding
		}
	}
	t.Fatalf("import binding %s not found in %+v", importPath, bindings)
	return ImportBinding{}
}

func findReference(t *testing.T, references []SemanticReference, name string, kind ReferenceKind) SemanticReference {
	t.Helper()
	for _, reference := range references {
		if reference.Name == name && reference.Kind == kind {
			return reference
		}
	}
	t.Fatalf("reference %s (%s) not found in %+v", name, kind, references)
	return SemanticReference{}
}

func findOwnedReference(t *testing.T, references []SemanticReference, owner, name string, kind ReferenceKind) SemanticReference {
	t.Helper()
	for _, reference := range references {
		if reference.OwnerDeclarationID == owner && reference.Name == name && reference.Kind == kind {
			return reference
		}
	}
	t.Fatalf("owned reference %s (%s) under %s not found in %+v", name, kind, owner, references)
	return SemanticReference{}
}

func ownedDeclaration(t *testing.T, declarations []SemanticDeclaration, owner, name string, kind DeclarationKind) SemanticDeclaration {
	t.Helper()
	for _, declaration := range declarations {
		if declaration.OwnerDeclarationID == owner && declaration.Name == name && declaration.Kind == kind {
			return declaration
		}
	}
	t.Fatalf("owned declaration %s (%s) under %s not found", name, kind, owner)
	return SemanticDeclaration{}
}

func findTypeRelation(t *testing.T, relations []TypeRelation, kind TypeRelationKind, owner string) TypeRelation {
	t.Helper()
	for _, relation := range relations {
		if relation.Kind == kind && relation.OwnerDeclarationID == owner {
			return relation
		}
	}
	t.Fatalf("type relation %s under %s not found in %+v", kind, owner, relations)
	return TypeRelation{}
}

func hasTypeRelationKind(relations []TypeRelation, kind TypeRelationKind) bool {
	for _, relation := range relations {
		if relation.Kind == kind {
			return true
		}
	}
	return false
}

func resolve(t testing.TB, input Input, config *Config) GoSemanticInventory {
	t.Helper()
	var engine Engine
	var err error
	if config == nil {
		engine, err = New()
	} else {
		engine, err = New(*config)
	}
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := engine.Resolve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func prerequisites(t testing.TB, files map[string]string) Input {
	t.Helper()
	_, input := prerequisitesAt(t, t.TempDir(), files)
	return input
}

func prerequisitesAt(t testing.TB, root string, files map[string]string) (string, Input) {
	t.Helper()
	return root, prerequisitesWithGoConfig(t, root, files, golang.DefaultConfig())
}

func prerequisitesWithGoConfig(t testing.TB, root string, files map[string]string, goConfig golang.Config) Input {
	t.Helper()
	for name, content := range files {
		absolute := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run := rie.NewRunContext(root, rie.DefaultConfig())
	pipeline := rie.New()
	for _, current := range []rie.Engine{discovery.New(), ignore.New(), language.New()} {
		if err := pipeline.Register(current); err != nil {
			t.Fatal(err)
		}
	}
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := rie.ArtifactAs[rie.RepositorySnapshot](run.Artifacts, rie.RepositorySnapshotArtifactName)
	if !ok {
		t.Fatal("RepositorySnapshot unavailable")
	}
	languages, ok := rie.ArtifactAs[language.LanguageInventory](run.Artifacts, language.LanguageInventoryArtifactName)
	if !ok {
		t.Fatal("LanguageInventory unavailable")
	}
	goEngine, err := golang.New(goConfig)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := goEngine.Analyze(context.Background(), lie.Input{Snapshot: snapshot, Languages: languages})
	if err != nil {
		t.Fatal(err)
	}
	syntax, ok := artifact.(golang.GoLanguageInventory)
	if !ok {
		t.Fatalf("Go artifact type = %T", artifact)
	}
	identityEngine, err := packageidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	identities, err := identityEngine.Analyze(context.Background(), packageidentity.Input{Snapshot: snapshot, Syntax: syntax})
	if err != nil {
		t.Fatal(err)
	}
	return Input{Snapshot: snapshot, Syntax: syntax, PackageIdentities: identities}
}
