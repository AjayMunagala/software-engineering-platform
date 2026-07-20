package golang_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	golang "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

func TestGoEngineExtractsApprovedFactsOnly(t *testing.T) {
	source := `package sample
import (
    "fmt"
    alias "example.com/module"
    _ "database/sql"
    . "strings"
)
const Public = "do-not-persist-this-secret"
const ( First = 1; Second = 2 )
var private = 42
var ( left int; right int )
type User struct { Password string ` + "`json:\"password\"`" + ` }
type Service interface { Run(context string) error }
func Exported(value string) { fmt.Println(value) }
func (u *User) Save(value string) {}
`
	inventory := analyze(t, map[string]string{"sample.go": source}, nil)
	if inventory.ArtifactVersion() != "0.1.0" || inventory.Language() != "Go" {
		t.Fatalf("artifact identity = %s/%s", inventory.ArtifactVersion(), inventory.Language())
	}
	files, packages, symbols := inventory.Files(), inventory.Packages(), inventory.Symbols()
	if len(files) != 1 || len(files[0].Imports) != 4 || len(packages) != 1 || len(symbols) != 10 {
		t.Fatalf("facts: files=%d imports=%d packages=%d symbols=%d", len(files), len(files[0].Imports), len(packages), len(symbols))
	}
	if files[0].ContentDigest == "" || !strings.HasPrefix(files[0].ContentDigest, "sha256:") {
		t.Fatalf("invalid digest %q", files[0].ContentDigest)
	}
	identifierPattern := regexp.MustCompile(`^go:symbol:sample\.go#[0-9]+:(struct|interface|function|method|constant|variable):[A-Za-z_][A-Za-z0-9_]*$`)
	for _, symbol := range symbols {
		if !identifierPattern.MatchString(symbol.ID) {
			t.Fatalf("invalid deterministic symbol ID %q", symbol.ID)
		}
	}
	encoded, err := json.Marshal(symbols)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"do-not-persist-this-secret", "password", "context string", "Fields"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("out-of-scope source content persisted: %q", forbidden)
		}
	}
}

func TestGoEngineHandlesEmptyAndNoGoRepositories(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"empty": {},
		"no-go": {"README.md": "notes"},
	} {
		t.Run(name, func(t *testing.T) {
			inventory := analyze(t, files, nil)
			if len(inventory.Files()) != 0 || len(inventory.Diagnostics()) != 0 || inventory.Statistics().CandidateFiles != 0 {
				t.Fatalf("expected empty inventory: %+v", inventory.Statistics())
			}
		})
	}
}

func TestGoEngineSupportsPackagesTestsUppercaseAndReceivers(t *testing.T) {
	files := map[string]string{
		"root.GO":           "package root\ntype Item[T any] struct{}\nfunc (i *Item[T]) Save() {}\nfunc (i *(Item[T])) Parenthesized() {}\n",
		"pkg/value.go":      "package pkg\nfunc Value() {}\n",
		"pkg/value_test.go": "package pkg_test\nfunc External() {}\n",
	}
	inventory := analyze(t, files, nil)
	if got := len(inventory.Packages()); got != 3 {
		t.Fatalf("packages = %d, want 3", got)
	}
	methods := map[string]golang.GoSymbol{}
	for _, symbol := range inventory.Symbols() {
		if symbol.Kind == golang.SymbolKindMethod {
			methods[symbol.Name] = symbol
		}
	}
	for _, name := range []string{"Save", "Parenthesized"} {
		method, ok := methods[name]
		if !ok || method.ReceiverBase != "Item" || !method.PointerReceiver || !method.GenericReceiver {
			t.Fatalf("method %s receiver = %+v", name, method)
		}
	}
}

func TestGoEngineIncludeTestsFalseStillChecksFullLanguageInventory(t *testing.T) {
	config := golang.DefaultConfig()
	config.IncludeTests = false
	inventory := analyze(t, map[string]string{"main.go": "package main\n", "main_test.go": "package main\n"}, &config)
	if len(inventory.Files()) != 1 || inventory.Files()[0].IsTest {
		t.Fatalf("files = %+v", inventory.Files())
	}
}

func TestGoEngineRejectsLanguageInventoryMismatch(t *testing.T) {
	oneSnapshot, _ := artifacts(t, map[string]string{"one.go": "package one\n"})
	_, twoLanguages := artifacts(t, map[string]string{"one.go": "package two\n", "two.go": "package two\n"})
	engine, _ := golang.New()
	_, err := engine.Analyze(context.Background(), lie.Input{Snapshot: oneSnapshot, Languages: twoLanguages})
	if !errors.Is(err, lie.ErrLanguageInventoryMismatch) {
		t.Fatalf("mismatch error = %v", err)
	}
}

func TestGoEngineClassifiesFileFailures(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		root, snapshot, languages := artifactRoot(t, map[string]string{"missing.go": "package gone\n"})
		if err := os.Remove(filepath.Join(root, "missing.go")); err != nil {
			t.Fatal(err)
		}
		inventory := analyzeInput(t, snapshot, languages, nil)
		assertDiagnostic(t, inventory, "go_source_missing", golang.FileStatusFailed)
	})
	t.Run("oversized", func(t *testing.T) {
		_, snapshot, languages := artifactRoot(t, map[string]string{"large.go": "package large\n" + strings.Repeat(" ", 100)})
		config := golang.DefaultConfig()
		config.MaxSourceFileSize = 20
		inventory := analyzeInput(t, snapshot, languages, &config)
		assertDiagnostic(t, inventory, "go_source_oversized", golang.FileStatusSkipped)
	})
	t.Run("parse", func(t *testing.T) {
		inventory := analyze(t, map[string]string{"broken.go": "package broken\nfunc {"}, nil)
		assertDiagnostic(t, inventory, "go_parse_error", golang.FileStatusFailed)
		if inventory.Files()[0].ContentDigest == "" {
			t.Fatal("parsed bytes must remain attributable after a parse failure")
		}
	})
}

func TestGoEngineBlocksLexicalAndSymlinkEscapes(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	outside := filepath.Join(base, "repo-escape")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "outside.go"), []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, languages := artifacts(t, map[string]string{"authorized.go": "package authorized\n"})
	t.Run("lexical sibling-prefix escape", func(t *testing.T) {
		snapshot := rie.NewRepositorySnapshot(root, []rie.RepositoryEntry{{Path: "../repo-escape/outside.go"}}, rie.Statistics{Files: 1}, nil, "0.2.1")
		inventory := analyzeInput(t, snapshot, languages, nil)
		assertDiagnostic(t, inventory, "go_source_outside_root", golang.FileStatusFailed)
	})
	t.Run("parent symlink escape", func(t *testing.T) {
		link := filepath.Join(root, "linked")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("directory symlinks unavailable: %v", err)
		}
		snapshot := rie.NewRepositorySnapshot(root, []rie.RepositoryEntry{{Path: "linked/outside.go"}}, rie.Statistics{Files: 1}, nil, "0.2.1")
		inventory := analyzeInput(t, snapshot, languages, nil)
		assertDiagnostic(t, inventory, "go_source_outside_root", golang.FileStatusFailed)
	})
}

func TestGoLanguageInventoryIsDeeplyImmutable(t *testing.T) {
	inventory := analyze(t, map[string]string{"main.go": "package main\nimport \"fmt\"\nfunc main(){fmt.Println()}\n"}, nil)
	files := inventory.Files()
	files[0].Path = "changed"
	files[0].Imports[0].Path = "changed"
	packages := inventory.Packages()
	packages[0].FileIDs[0] = "changed"
	statistics := inventory.Statistics()
	statistics.SymbolsByKind["function"] = 999
	if inventory.Files()[0].Path == "changed" || inventory.Files()[0].Imports[0].Path == "changed" || inventory.Packages()[0].FileIDs[0] == "changed" || inventory.Statistics().SymbolsByKind["function"] == 999 {
		t.Fatal("nested artifact collections are mutable")
	}

	broken := analyze(t, map[string]string{"broken.go": "package broken\nfunc {"}, nil)
	diagnostics := broken.Diagnostics()
	diagnostics[0].Location.File = "changed"
	if broken.Diagnostics()[0].Location.File == "changed" {
		t.Fatal("diagnostic locations are mutable")
	}
}

func TestGoEngineIsDeterministicAcrossWorkerCounts(t *testing.T) {
	files := map[string]string{}
	for index := 0; index < 30; index++ {
		files[filepath.ToSlash(filepath.Join("pkg", string(rune('a'+index%10)), fmt.Sprintf("file%d.go", index)))] = "package sample\nfunc Value() {}\n"
	}
	one := golang.DefaultConfig()
	one.MaxWorkers = 1
	eight := golang.DefaultConfig()
	eight.MaxWorkers = 8
	left, right := analyze(t, files, &one), analyze(t, files, &eight)
	if !reflect.DeepEqual(left.Files(), right.Files()) || !reflect.DeepEqual(left.Packages(), right.Packages()) || !reflect.DeepEqual(left.Symbols(), right.Symbols()) || !reflect.DeepEqual(left.Statistics(), right.Statistics()) {
		t.Fatal("worker count changed deterministic output")
	}
}

func TestGoEngineCancellationAndConfiguration(t *testing.T) {
	if _, err := golang.New(golang.Config{}); !errors.Is(err, lie.ErrInvalidConfig) {
		t.Fatalf("invalid config error = %v", err)
	}
	tooManyWorkers := golang.DefaultConfig()
	tooManyWorkers.MaxWorkers = 9
	if _, err := golang.New(tooManyWorkers); !errors.Is(err, lie.ErrInvalidConfig) {
		t.Fatalf("worker cap error = %v", err)
	}
	if _, err := golang.New(golang.DefaultConfig(), golang.DefaultConfig()); !errors.Is(err, lie.ErrInvalidConfig) {
		t.Fatalf("multiple config error = %v", err)
	}
	snapshot, languages := artifacts(t, map[string]string{"main.go": "package main\n"})
	engine, _ := golang.New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.Analyze(ctx, lie.Input{Snapshot: snapshot, Languages: languages}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

func TestGoEngineDiagnosticLimit(t *testing.T) {
	root, snapshot, languages := artifactRoot(t, map[string]string{"a.go": "package a\n", "b.go": "package a\n", "c.go": "package a\n"})
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	config := golang.DefaultConfig()
	config.MaxDiagnostics = 2
	inventory := analyzeInput(t, snapshot, languages, &config)
	diagnostics := inventory.Diagnostics()
	if len(diagnostics) != 2 || diagnostics[1].Code != "go_diagnostic_limit" || inventory.Statistics().OmittedDiagnostics != 2 {
		t.Fatalf("diagnostic limit: diagnostics=%+v statistics=%+v", diagnostics, inventory.Statistics())
	}
}

func TestRunnerExecutesRealGoEngine(t *testing.T) {
	store := artifactStore(t, map[string]string{"main.go": "package main\nfunc main() {}\n"})
	goEngine, _ := golang.New()
	runner, err := lie.New(goEngine)
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Published()) != 1 {
		t.Fatalf("published = %+v", report.Published())
	}
	if _, ok := golang.InventoryFrom(store); !ok {
		t.Fatal("GoLanguageInventory was not published")
	}
}

func assertDiagnostic(t *testing.T, inventory golang.GoLanguageInventory, code string, status golang.FileStatus) {
	t.Helper()
	if len(inventory.Files()) != 1 || inventory.Files()[0].Status != status || len(inventory.Diagnostics()) != 1 || inventory.Diagnostics()[0].Code != code {
		t.Fatalf("files=%+v diagnostics=%+v", inventory.Files(), inventory.Diagnostics())
	}
}

func analyze(t testing.TB, files map[string]string, config *golang.Config) golang.GoLanguageInventory {
	t.Helper()
	snapshot, languages := artifacts(t, files)
	return analyzeInput(t, snapshot, languages, config)
}

func analyzeInput(t testing.TB, snapshot rie.RepositorySnapshot, languages language.LanguageInventory, config *golang.Config) golang.GoLanguageInventory {
	t.Helper()
	var engine lie.Engine
	var err error
	if config == nil {
		engine, err = golang.New()
	} else {
		engine, err = golang.New(*config)
	}
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := engine.Analyze(context.Background(), lie.Input{Snapshot: snapshot, Languages: languages})
	if err != nil {
		t.Fatal(err)
	}
	inventory, ok := artifact.(golang.GoLanguageInventory)
	if !ok {
		t.Fatalf("artifact type = %T", artifact)
	}
	return inventory
}

func artifacts(t testing.TB, files map[string]string) (rie.RepositorySnapshot, language.LanguageInventory) {
	t.Helper()
	store := artifactStore(t, files)
	snapshot, ok := rie.ArtifactAs[rie.RepositorySnapshot](store, rie.RepositorySnapshotArtifactName)
	if !ok {
		t.Fatal("RepositorySnapshot unavailable")
	}
	languages, ok := rie.ArtifactAs[language.LanguageInventory](store, language.LanguageInventoryArtifactName)
	if !ok {
		t.Fatal("LanguageInventory unavailable")
	}
	return snapshot, languages
}

func artifactRoot(t testing.TB, files map[string]string) (string, rie.RepositorySnapshot, language.LanguageInventory) {
	t.Helper()
	root := t.TempDir()
	store := runRIE(t, root, files)
	snapshot, _ := rie.ArtifactAs[rie.RepositorySnapshot](store, rie.RepositorySnapshotArtifactName)
	languages, _ := rie.ArtifactAs[language.LanguageInventory](store, language.LanguageInventoryArtifactName)
	return root, snapshot, languages
}

func artifactStore(t testing.TB, files map[string]string) *rie.ArtifactStore {
	t.Helper()
	return runRIE(t, t.TempDir(), files)
}

func runRIE(t testing.TB, root string, files map[string]string) *rie.ArtifactStore {
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
	return run.Artifacts
}
