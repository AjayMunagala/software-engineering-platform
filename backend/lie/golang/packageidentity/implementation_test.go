package packageidentity_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestSingleModuleProducesEvidenceBackedProofs(t *testing.T) {
	inventory := analyze(t, map[string]string{
		"go.mod":     "module example.com/app\n\ngo 1.22\n",
		"main.go":    "package main\nimport (\"example.com/app/lib\"; \"fmt\")\nfunc main() {}\n",
		"lib/lib.go": "package lib\nfunc Value() {}\n",
	}, nil)

	if inventory.ArtifactVersion() != "1.0.0" || inventory.Metadata().IDSchemeVersion != "go-package-proof-id/v1" {
		t.Fatalf("artifact metadata = %+v", inventory.Metadata())
	}
	modules := inventory.Modules()
	if len(modules) != 1 || modules[0].ModulePath != "example.com/app" || modules[0].GoVersion != "1.22" {
		t.Fatalf("modules = %+v", modules)
	}
	if len(modules[0].Evidence) < 2 || !hasEvidenceRule(modules[0].Evidence, "go.mod.module") || !hasEvidenceRule(modules[0].Evidence, "go.mod.go") || !strings.HasPrefix(modules[0].Evidence[0].ContentDigest, "sha256:") || modules[0].Evidence[0].Location == nil {
		t.Fatalf("module evidence = %+v", modules[0].Evidence)
	}
	local := findProof(t, inventory, packageidentity.ContextSingleModule, "example.com/app/lib")
	if local.Status != packageidentity.ProofResolved || local.TargetDirectory != "lib" || !containsKind(local.Kinds, packageidentity.ProofSameModule) {
		t.Fatalf("local proof = %+v", local)
	}
	const goldenProofID = "go:package-proof:v1:38:go:package-context:v1:single-module:0:#17:go:package:.#main#19:example.com/app/lib#same-module"
	if local.ID != goldenProofID {
		t.Fatalf("proof ID = %q, want %q", local.ID, goldenProofID)
	}
	external := findProof(t, inventory, packageidentity.ContextSingleModule, "fmt")
	if external.Status != packageidentity.ProofExternal || external.TargetPackageID != "" {
		t.Fatalf("external proof = %+v", external)
	}
}

func TestEngineAndArtifactPublicationContract(t *testing.T) {
	engine, err := packageidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	if engine.Name() != "go-package-identity" || engine.Version() != "1.0.0" || engine.ArtifactName() != packageidentity.ArtifactName || strings.TrimSpace(engine.Description()) == "" {
		t.Fatalf("engine metadata: %s %s %s %q", engine.Name(), engine.Version(), engine.ArtifactName(), engine.Description())
	}
	inventory := analyze(t, map[string]string{"go.mod": "module example.com/app\n", "main.go": "package main\n"}, nil)
	store := rie.NewArtifactStore()
	if err := store.Put(inventory); err != nil {
		t.Fatal(err)
	}
	published, ok := packageidentity.InventoryFrom(store)
	if !ok || published.ArtifactName() != packageidentity.ArtifactName {
		t.Fatalf("published artifact = %+v, %v", published.Metadata(), ok)
	}
	sources := published.SourceArtifacts()
	sources[0].Name = "changed"
	if published.SourceArtifacts()[0].Name == "changed" {
		t.Fatal("source artifacts are mutable")
	}
	encoded, err := json.Marshal(struct {
		Kind   packageidentity.ResolutionContextKind `json:"kind"`
		Status packageidentity.ProofStatus           `json:"status"`
	}{packageidentity.ContextSingleModule, packageidentity.ProofResolved})
	if err != nil || string(encoded) != `{"kind":"single-module","status":"resolved"}` {
		t.Fatalf("enum JSON = %s, %v", encoded, err)
	}
	encoded, err = json.Marshal(inventory)
	if err != nil || !strings.Contains(string(encoded), `"artifact"`) || !strings.Contains(string(encoded), `"proofs"`) {
		t.Fatalf("artifact JSON = %s, %v", encoded, err)
	}
}

func TestEnumJSONContractsRejectUnknownValues(t *testing.T) {
	tests := []struct {
		name  string
		value any
		text  string
		out   any
	}{
		{"proof kind", packageidentity.ProofVendor, `"vendor"`, new(packageidentity.ProofKind)},
		{"proof status", packageidentity.ProofAmbiguous, `"ambiguous"`, new(packageidentity.ProofStatus)},
		{"context kind", packageidentity.ContextWorkspace, `"workspace"`, new(packageidentity.ResolutionContextKind)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.value)
			if err != nil || string(encoded) != test.text {
				t.Fatalf("marshal = %s, %v", encoded, err)
			}
			if err := json.Unmarshal(encoded, test.out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
		})
	}
	for _, invalid := range []any{packageidentity.ProofKind(0), packageidentity.ProofStatus(0), packageidentity.ResolutionContextKind(0)} {
		if _, err := json.Marshal(invalid); err == nil {
			t.Fatalf("invalid %T marshaled successfully", invalid)
		}
	}
	for _, output := range []any{new(packageidentity.ProofKind), new(packageidentity.ProofStatus), new(packageidentity.ResolutionContextKind)} {
		if err := json.Unmarshal([]byte(`"future-value"`), output); err == nil {
			t.Fatalf("unknown %T unmarshaled successfully", output)
		}
	}
}

func TestWorkspacePreservesIndependentResolutionContexts(t *testing.T) {
	inventory := analyze(t, map[string]string{
		"go.work":      "go 1.22\n\nuse (\n ./a\n ./b\n)\n",
		"a/go.mod":     "module example.com/a\n\ngo 1.22\n",
		"a/main.go":    "package a\nimport \"example.com/b/pkg\"\n",
		"b/go.mod":     "module example.com/b\n\ngo 1.22\n",
		"b/pkg/pkg.go": "package pkg\n",
	}, nil)

	workspace := findProof(t, inventory, packageidentity.ContextWorkspace, "example.com/b/pkg")
	if workspace.Status != packageidentity.ProofResolved || workspace.TargetDirectory != "b/pkg" || !containsKind(workspace.Kinds, packageidentity.ProofWorkspaceModule) {
		t.Fatalf("workspace proof = %+v", workspace)
	}
	standalone := findProof(t, inventory, packageidentity.ContextSingleModule, "example.com/b/pkg")
	if standalone.Status != packageidentity.ProofExternal {
		t.Fatalf("standalone proof = %+v", standalone)
	}
	if countContext(inventory.Contexts(), packageidentity.ContextWorkspace) != 1 {
		t.Fatalf("contexts = %+v", inventory.Contexts())
	}
	for _, current := range inventory.Contexts() {
		if current.Kind == packageidentity.ContextWorkspace && (!hasEvidenceRule(current.Evidence, "go.work.go") || !hasEvidenceRule(current.Evidence, "go.work.use")) {
			t.Fatalf("workspace context lacks selection evidence: %+v", current)
		}
	}
}

func TestLocalReplacementResolvesRepositoryModule(t *testing.T) {
	inventory := analyze(t, map[string]string{
		"app/go.mod":     "module example.com/app\n\ngo 1.22\nrequire example.com/dep v1.0.0\nreplace example.com/dep => ../dep\n",
		"app/app.go":     "package app\nimport \"example.com/dep/pkg\"\n",
		"dep/go.mod":     "module example.com/dep\n\ngo 1.22\n",
		"dep/pkg/pkg.go": "package pkg\n",
	}, nil)

	proof := findProofAtRoot(t, inventory, packageidentity.ContextSingleModule, "app", "example.com/dep/pkg")
	if proof.Status != packageidentity.ProofResolved || proof.TargetDirectory != "dep/pkg" || !containsKind(proof.Kinds, packageidentity.ProofLocalReplace) {
		t.Fatalf("replacement proof = %+v", proof)
	}
	if len(proof.Evidence) < 3 {
		t.Fatalf("replacement evidence = %+v", proof.Evidence)
	}
}

func TestExternalAndInvalidFilesystemReplacementsRemainHonest(t *testing.T) {
	external := analyze(t, map[string]string{
		"go.mod":  "module example.com/app\n\ngo 1.22\nrequire example.com/dep v1.0.0\nreplace example.com/dep => example.net/fork v2.0.0\n",
		"main.go": "package main\nimport \"example.com/dep/pkg\"\n",
	}, nil)
	externalProof := findProof(t, external, packageidentity.ContextSingleModule, "example.com/dep/pkg")
	if externalProof.Status != packageidentity.ProofExternal || containsKind(externalProof.Kinds, packageidentity.ProofLocalReplace) {
		t.Fatalf("external replacement proof = %+v", externalProof)
	}

	outside := analyze(t, map[string]string{
		"go.mod":  "module example.com/app\n\ngo 1.22\nrequire example.com/dep v1.0.0\nreplace example.com/dep => ../outside\n",
		"main.go": "package main\nimport \"example.com/dep/pkg\"\n",
	}, nil)
	outsideProof := findProof(t, outside, packageidentity.ContextSingleModule, "example.com/dep/pkg")
	if outsideProof.Status != packageidentity.ProofUnresolved || !containsKind(outsideProof.Kinds, packageidentity.ProofLocalReplace) {
		t.Fatalf("outside replacement proof = %+v", outsideProof)
	}
}

func TestWorkspaceReplacementOverridesModuleReplacement(t *testing.T) {
	inventory := analyze(t, map[string]string{
		"go.work":                  "go 1.22\nuse ./app\nreplace example.com/dep => ./dep-workspace\n",
		"app/go.mod":               "module example.com/app\n\ngo 1.22\nrequire example.com/dep v1.0.0\nreplace example.com/dep => ../dep-module\n",
		"app/app.go":               "package app\nimport \"example.com/dep/pkg\"\n",
		"dep-module/go.mod":        "module example.com/dep\n\ngo 1.22\n",
		"dep-module/pkg/pkg.go":    "package pkg\n",
		"dep-workspace/go.mod":     "module example.com/dep\n\ngo 1.22\n",
		"dep-workspace/pkg/pkg.go": "package pkg\n",
	}, nil)

	standalone := findProofAtRoot(t, inventory, packageidentity.ContextSingleModule, "app", "example.com/dep/pkg")
	if standalone.Status != packageidentity.ProofResolved || standalone.TargetDirectory != "dep-module/pkg" {
		t.Fatalf("module replacement proof = %+v", standalone)
	}
	workspace := findProof(t, inventory, packageidentity.ContextWorkspace, "example.com/dep/pkg")
	if workspace.Status != packageidentity.ProofResolved || workspace.TargetDirectory != "dep-workspace/pkg" {
		t.Fatalf("workspace replacement proof = %+v", workspace)
	}
}

func TestDuplicateWorkspaceModulePathsRemainAmbiguous(t *testing.T) {
	inventory := analyze(t, map[string]string{
		"go.work":          "go 1.22\nuse (\n ./a\n ./b\n ./consumer\n)\n",
		"a/go.mod":         "module example.com/shared\n\ngo 1.22\n",
		"a/pkg/pkg.go":     "package pkg\n",
		"b/go.mod":         "module example.com/shared\n\ngo 1.22\n",
		"b/pkg/pkg.go":     "package pkg\n",
		"consumer/go.mod":  "module example.com/consumer\n\ngo 1.22\n",
		"consumer/main.go": "package consumer\nimport \"example.com/shared/pkg\"\n",
	}, nil)

	proof := findProof(t, inventory, packageidentity.ContextWorkspace, "example.com/shared/pkg")
	if proof.Status != packageidentity.ProofAmbiguous || len(proof.CandidatePackageIDs) != 2 || proof.TargetPackageID != "" {
		t.Fatalf("ambiguous proof = %+v", proof)
	}
}

func TestNestedModuleBoundaryUsesNearestModule(t *testing.T) {
	inventory := analyze(t, map[string]string{
		"go.mod":               "module example.com/root\n\ngo 1.22\n",
		"root.go":              "package root\nimport \"example.com/root/nested/pkg\"\n",
		"nested/go.mod":        "module example.com/nested\n\ngo 1.22\n",
		"nested/pkg/pkg.go":    "package pkg\nimport \"example.com/nested/internal\"\n",
		"nested/internal/i.go": "package internal\n",
	}, nil)

	rootProof := findProofAtRoot(t, inventory, packageidentity.ContextSingleModule, "", "example.com/root/nested/pkg")
	if rootProof.Status != packageidentity.ProofUnresolved {
		t.Fatalf("nested module leaked into parent: %+v", rootProof)
	}
	nestedProof := findProofAtRoot(t, inventory, packageidentity.ContextSingleModule, "nested", "example.com/nested/internal")
	if nestedProof.Status != packageidentity.ProofResolved || nestedProof.TargetDirectory != "nested/internal" {
		t.Fatalf("nested proof = %+v", nestedProof)
	}
}

func TestVendorProofRequiresManifestEvidence(t *testing.T) {
	inventory := analyze(t, map[string]string{
		"go.mod":                            "module example.com/app\n\ngo 1.22\nrequire example.com/dep v1.0.0\n",
		"main.go":                           "package main\nimport \"example.com/dep/pkg\"\n",
		"vendor/modules.txt":                "# example.com/dep v1.0.0\nexample.com/dep/pkg\n",
		"vendor/example.com/dep/pkg/pkg.go": "package pkg\n",
	}, nil)

	proof := findProof(t, inventory, packageidentity.ContextModuleVendor, "example.com/dep/pkg")
	if proof.Status != packageidentity.ProofResolved || proof.TargetDirectory != "vendor/example.com/dep/pkg" || !containsKind(proof.Kinds, packageidentity.ProofVendor) {
		t.Fatalf("vendor proof = %+v", proof)
	}
	if !hasEvidenceRule(proof.Evidence, "vendor.module") || !hasEvidenceRule(proof.Evidence, "vendor.package") || !hasEvidenceRule(proof.Evidence, "go.mod.require") || proof.Evidence[0].Location == nil {
		t.Fatalf("vendor evidence = %+v", proof.Evidence)
	}
}

func TestInconsistentVendorManifestDoesNotResolve(t *testing.T) {
	inventory := analyze(t, map[string]string{
		"go.mod":                            "module example.com/app\n\ngo 1.22\nrequire example.com/dep v1.0.0\n",
		"main.go":                           "package main\nimport \"example.com/dep/pkg\"\n",
		"vendor/modules.txt":                "# example.com/dep v2.0.0\nexample.com/dep/pkg\n",
		"vendor/example.com/dep/pkg/pkg.go": "package pkg\n",
	}, nil)
	proof := findProof(t, inventory, packageidentity.ContextModuleVendor, "example.com/dep/pkg")
	if proof.Status != packageidentity.ProofUnresolved || proof.TargetPackageID != "" {
		t.Fatalf("inconsistent vendor proof = %+v", proof)
	}
}

func TestEmptyAndUnmanagedRepositoriesAreValid(t *testing.T) {
	empty := analyze(t, map[string]string{"README.md": "notes"}, nil)
	if len(empty.Modules()) != 0 || len(empty.Contexts()) != 0 || len(empty.Proofs()) != 0 || len(empty.Diagnostics()) != 0 {
		t.Fatalf("empty inventory = modules:%+v contexts:%+v proofs:%+v diagnostics:%+v", empty.Modules(), empty.Contexts(), empty.Proofs(), empty.Diagnostics())
	}
	unmanaged := analyze(t, map[string]string{"main.go": "package main\nimport \"example.com/unknown\"\n"}, nil)
	proof := findProof(t, unmanaged, packageidentity.ContextUnmanaged, "example.com/unknown")
	if proof.Status != packageidentity.ProofUnresolved || len(unmanaged.Modules()) != 0 {
		t.Fatalf("unmanaged proof = %+v", proof)
	}
}

func TestMalformedAndOutsideWorkspaceManifestsProduceDiagnostics(t *testing.T) {
	malformed := analyze(t, map[string]string{"go.mod": "module\n", "main.go": "package main\n"}, nil)
	assertDiagnostic(t, malformed, "go_mod_parse_error")
	outside := analyze(t, map[string]string{"go.work": "go 1.22\nuse ../outside\n", "main.go": "package main\n"}, nil)
	assertDiagnostic(t, outside, "go_work_use_outside_root")
}

func TestInventoryIsDeeplyImmutable(t *testing.T) {
	inventory := analyze(t, map[string]string{
		"go.mod":     "module example.com/app\n\ngo 1.22\n",
		"main.go":    "package main\nimport \"example.com/app/lib\"\n",
		"lib/lib.go": "package lib\n",
	}, nil)
	contexts := inventory.Contexts()
	contexts[0].ManifestFiles[0] = "changed"
	contexts[0].Evidence[0].File = "changed"
	modules := inventory.Modules()
	modules[0].Evidence[0].File = "changed"
	modules[0].Evidence[0].Location.File = "changed"
	proofs := inventory.Proofs()
	proofs[0].Kinds[0] = packageidentity.ProofVendor
	proofs[0].CandidatePackageIDs = append(proofs[0].CandidatePackageIDs, "changed")
	statistics := inventory.Statistics()
	statistics.ProofsByStatus["resolved"] = 999
	view := inventory.View()
	view.Contexts[0].ManifestFiles[0] = "view-changed"
	view.Modules[0].Evidence[0].File = "view-changed"
	if inventory.Contexts()[0].ManifestFiles[0] == "changed" || inventory.Contexts()[0].Evidence[0].File == "changed" || inventory.Modules()[0].Evidence[0].File == "changed" || inventory.Modules()[0].Evidence[0].Location.File == "changed" || inventory.Proofs()[0].Kinds[0] == packageidentity.ProofVendor || inventory.Statistics().ProofsByStatus["resolved"] == 999 {
		t.Fatal("artifact exposes mutable nested state")
	}
	if inventory.Contexts()[0].ManifestFiles[0] == "view-changed" || inventory.Modules()[0].Evidence[0].File == "view-changed" {
		t.Fatal("artifact view exposes mutable nested state")
	}
}

func TestDeterminismAcrossWorkerCounts(t *testing.T) {
	files := map[string]string{}
	for index := 0; index < 20; index++ {
		name := string(rune('a' + index))
		root := "module-" + name
		modulePath := "example.com/" + name
		files[root+"/go.mod"] = "module " + modulePath + "\n\ngo 1.22\n"
		files[root+"/main.go"] = "package " + name + "\nimport \"fmt\"\n"
	}
	one := packageidentity.DefaultConfig()
	one.MaxWorkers = 1
	eight := packageidentity.DefaultConfig()
	eight.MaxWorkers = 8
	left, right := analyze(t, files, &one), analyze(t, files, &eight)
	if !reflect.DeepEqual(left.Contexts(), right.Contexts()) || !reflect.DeepEqual(left.Modules(), right.Modules()) || !reflect.DeepEqual(left.Proofs(), right.Proofs()) || !reflect.DeepEqual(left.Diagnostics(), right.Diagnostics()) || !reflect.DeepEqual(left.Statistics(), right.Statistics()) {
		t.Fatal("worker count changed deterministic output")
	}
}

func TestConfigurationCancellationAndProvenance(t *testing.T) {
	if _, err := packageidentity.New(packageidentity.Config{MaxWorkers: -1}); !errors.Is(err, packageidentity.ErrInvalidConfig) {
		t.Fatalf("invalid configuration error = %v", err)
	}
	tooMany := packageidentity.DefaultConfig()
	tooMany.MaxWorkers = 9
	if _, err := packageidentity.New(tooMany); !errors.Is(err, packageidentity.ErrInvalidConfig) {
		t.Fatalf("worker cap error = %v", err)
	}
	if _, err := packageidentity.New(packageidentity.DefaultConfig(), packageidentity.DefaultConfig()); !errors.Is(err, packageidentity.ErrInvalidConfig) {
		t.Fatalf("multiple configurations error = %v", err)
	}

	snapshot, syntax := prerequisites(t, map[string]string{"main.go": "package main\n"})
	engine, _ := packageidentity.New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.Analyze(ctx, packageidentity.Input{Snapshot: snapshot, Syntax: syntax}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}

	otherSnapshot, _ := prerequisites(t, map[string]string{"other.go": "package other\n"})
	if _, err := engine.Analyze(context.Background(), packageidentity.Input{Snapshot: otherSnapshot, Syntax: syntax}); !errors.Is(err, packageidentity.ErrArtifactProvenanceMismatch) {
		t.Fatalf("provenance error = %v", err)
	}
}

func TestManifestMutationAfterSnapshotIsReported(t *testing.T) {
	root, snapshot, syntax := prerequisiteRoot(t, map[string]string{
		"go.mod":  "module example.com/app\n\ngo 1.22\n",
		"main.go": "package main\n",
	})
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	inventory := analyzeInput(t, snapshot, syntax, nil)
	assertDiagnostic(t, inventory, "package_manifest_missing")
}

func TestManifestSymlinkEscapeIsRejected(t *testing.T) {
	base := t.TempDir()
	repository := filepath.Join(base, "repository")
	outside := filepath.Join(base, "outside.mod")
	if err := os.MkdirAll(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("module example.com/outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, snapshot, syntax := prerequisiteRootAt(t, repository, map[string]string{
		"go.mod":  "module example.com/app\n\ngo 1.22\n",
		"main.go": "package main\n",
	})
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "go.mod")); err != nil {
		t.Skipf("file symlinks unavailable: %v", err)
	}
	inventory := analyzeInput(t, snapshot, syntax, nil)
	assertDiagnostic(t, inventory, "package_manifest_outside_root")
}

func TestDiagnosticLimitIsDeterministic(t *testing.T) {
	files := map[string]string{
		"a/go.mod": "module\n",
		"b/go.mod": "module\n",
		"c/go.mod": "module\n",
	}
	config := packageidentity.DefaultConfig()
	config.MaxDiagnostics = 2
	inventory := analyze(t, files, &config)
	diagnostics := inventory.Diagnostics()
	if len(diagnostics) != 2 || diagnostics[1].Code != "package_identity_diagnostic_limit" || inventory.Statistics().OmittedDiagnostics != 2 {
		t.Fatalf("diagnostic limit: diagnostics=%+v statistics=%+v", diagnostics, inventory.Statistics())
	}
}

func assertDiagnostic(t *testing.T, inventory packageidentity.GoPackageIdentityInventory, code string) {
	t.Helper()
	for _, diagnostic := range inventory.Diagnostics() {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostic %q not found in %+v", code, inventory.Diagnostics())
}

func findProof(t *testing.T, inventory packageidentity.GoPackageIdentityInventory, kind packageidentity.ResolutionContextKind, importPath string) packageidentity.PackageIdentityProof {
	t.Helper()
	contexts := map[string]packageidentity.ResolutionContextKind{}
	for _, current := range inventory.Contexts() {
		contexts[current.ID] = current.Kind
	}
	for _, proof := range inventory.Proofs() {
		if contexts[proof.ResolutionContextID] == kind && proof.ImportPath == importPath {
			return proof
		}
	}
	t.Fatalf("proof kind=%s import=%q not found; contexts=%+v proofs=%+v", kind, importPath, inventory.Contexts(), inventory.Proofs())
	return packageidentity.PackageIdentityProof{}
}

func findProofAtRoot(t *testing.T, inventory packageidentity.GoPackageIdentityInventory, kind packageidentity.ResolutionContextKind, root, importPath string) packageidentity.PackageIdentityProof {
	t.Helper()
	contextIDs := map[string]bool{}
	for _, current := range inventory.Contexts() {
		if current.Kind == kind && current.Root == root {
			contextIDs[current.ID] = true
		}
	}
	for _, proof := range inventory.Proofs() {
		if contextIDs[proof.ResolutionContextID] && proof.ImportPath == importPath {
			return proof
		}
	}
	t.Fatalf("proof kind=%s root=%q import=%q not found", kind, root, importPath)
	return packageidentity.PackageIdentityProof{}
}

func containsKind(kinds []packageidentity.ProofKind, expected packageidentity.ProofKind) bool {
	for _, kind := range kinds {
		if kind == expected {
			return true
		}
	}
	return false
}

func hasEvidenceRule(evidence []packageidentity.PackageIdentityEvidence, rule string) bool {
	for _, item := range evidence {
		if item.Rule == rule {
			return true
		}
	}
	return false
}

func countContext(contexts []packageidentity.ResolutionContext, kind packageidentity.ResolutionContextKind) int {
	count := 0
	for _, current := range contexts {
		if current.Kind == kind {
			count++
		}
	}
	return count
}

func analyze(t testing.TB, files map[string]string, config *packageidentity.Config) packageidentity.GoPackageIdentityInventory {
	t.Helper()
	snapshot, syntax := prerequisites(t, files)
	return analyzeInput(t, snapshot, syntax, config)
}

func analyzeInput(t testing.TB, snapshot rie.RepositorySnapshot, syntax golang.GoLanguageInventory, config *packageidentity.Config) packageidentity.GoPackageIdentityInventory {
	t.Helper()
	var engine packageidentity.Engine
	var err error
	if config == nil {
		engine, err = packageidentity.New()
	} else {
		engine, err = packageidentity.New(*config)
	}
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := engine.Analyze(context.Background(), packageidentity.Input{Snapshot: snapshot, Syntax: syntax})
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func prerequisites(t testing.TB, files map[string]string) (rie.RepositorySnapshot, golang.GoLanguageInventory) {
	t.Helper()
	_, snapshot, syntax := prerequisiteRoot(t, files)
	return snapshot, syntax
}

func prerequisiteRoot(t testing.TB, files map[string]string) (string, rie.RepositorySnapshot, golang.GoLanguageInventory) {
	t.Helper()
	return prerequisiteRootAt(t, t.TempDir(), files)
}

func prerequisiteRootAt(t testing.TB, root string, files map[string]string) (string, rie.RepositorySnapshot, golang.GoLanguageInventory) {
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
	goEngine, err := golang.New()
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
	return root, snapshot, syntax
}
