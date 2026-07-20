package lie_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

type testArtifact struct {
	name, version, language string
}

type prerequisiteArtifact struct{ name, version string }

func (artifact prerequisiteArtifact) ArtifactName() string    { return artifact.name }
func (artifact prerequisiteArtifact) ArtifactVersion() string { return artifact.version }

func (artifact testArtifact) ArtifactName() string    { return artifact.name }
func (artifact testArtifact) ArtifactVersion() string { return artifact.version }
func (artifact testArtifact) Language() string        { return artifact.language }

type testEngine struct {
	name, version, language, artifactName string
	artifact                              lie.LanguageArtifact
	err                                   error
}

func (engine testEngine) Name() string         { return engine.name }
func (engine testEngine) Version() string      { return engine.version }
func (engine testEngine) Language() string     { return engine.language }
func (engine testEngine) ArtifactName() string { return engine.artifactName }
func (testEngine) Description() string         { return "test language engine" }
func (engine testEngine) Analyze(context.Context, lie.Input) (lie.LanguageArtifact, error) {
	return engine.artifact, engine.err
}

func TestNewAndRegistration(t *testing.T) {
	valid := testEngine{name: "go", version: "0.1.0", language: "Go", artifactName: "go-test", artifact: testArtifact{name: "go-test", version: "0.1.0", language: "Go"}}
	runner, err := lie.New(valid)
	if err != nil || len(runner.Engines()) != 1 {
		t.Fatalf("New(valid) = (%v, %v)", runner, err)
	}
	if err := runner.Register(nil); !errors.Is(err, lie.ErrNilEngine) {
		t.Fatalf("Register(nil) error = %v", err)
	}
	for _, duplicate := range []testEngine{
		{name: "go", version: "1", language: "Rust", artifactName: "rust"},
		{name: "rust", version: "1", language: "Go", artifactName: "rust"},
	} {
		if err := runner.Register(duplicate); !errors.Is(err, lie.ErrDuplicateEngine) {
			t.Fatalf("Register duplicate error = %v", err)
		}
	}
	if err := runner.Register(testEngine{name: "rust", version: "1", language: "Rust", artifactName: "go-test"}); !errors.Is(err, lie.ErrDuplicateArtifactName) {
		t.Fatalf("Register duplicate artifact error = %v", err)
	}
	if _, err := lie.New(testEngine{}); !errors.Is(err, lie.ErrInvalidEngineMetadata) {
		t.Fatalf("New(invalid metadata) error = %v", err)
	}
}

func TestRunnerRequiresArtifactsAndPreflightsConflicts(t *testing.T) {
	runner, _ := lie.New()
	if _, err := runner.Run(context.Background(), rie.NewArtifactStore()); !errors.Is(err, lie.ErrSnapshotRequired) {
		t.Fatalf("missing snapshot error = %v", err)
	}
	store := rie.NewArtifactStore()
	if err := store.Put(rie.NewRepositorySnapshot(t.TempDir(), nil, rie.Statistics{}, nil, "0.2.1")); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), store); !errors.Is(err, lie.ErrLanguageInventoryRequired) {
		t.Fatalf("missing language inventory error = %v", err)
	}

	store = repositoryArtifacts(t, map[string]string{"main.go": "package main\n"})
	engine := testEngine{name: "go", version: "0.1.0", language: "Go", artifactName: "go-test", artifact: testArtifact{name: "go-test", version: "0.1.0", language: "Go"}}
	runner, _ = lie.New(engine)
	if err := store.Put(testArtifact{name: "go-test", version: "old", language: "Go"}); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), store); !errors.Is(err, rie.ErrArtifactAlreadyExists) {
		t.Fatalf("preflight conflict error = %v", err)
	}
}

func TestRunnerRejectsPrerequisiteVersionMismatch(t *testing.T) {
	runner, _ := lie.New()
	store := rie.NewArtifactStore()
	if err := store.Put(prerequisiteArtifact{name: rie.RepositorySnapshotArtifactName, version: "0.9.0"}); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), store); !errors.Is(err, lie.ErrArtifactVersionMismatch) {
		t.Fatalf("snapshot version error = %v", err)
	}

	store = rie.NewArtifactStore()
	if err := store.Put(rie.NewRepositorySnapshot(t.TempDir(), nil, rie.Statistics{}, nil, "0.2.1")); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(prerequisiteArtifact{name: language.LanguageInventoryArtifactName, version: "0.9.0"}); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), store); !errors.Is(err, lie.ErrArtifactVersionMismatch) {
		t.Fatalf("language version error = %v", err)
	}
}

func TestRunnerValidatesPublishedArtifactContract(t *testing.T) {
	store := repositoryArtifacts(t, map[string]string{"main.go": "package main\n"})
	engine := testEngine{name: "go", version: "0.1.0", language: "Go", artifactName: "expected", artifact: testArtifact{name: "wrong", version: "0.1.0", language: "Rust"}}
	runner, _ := lie.New(engine)
	if _, err := runner.Run(context.Background(), store); !errors.Is(err, lie.ErrArtifactContractMismatch) {
		t.Fatalf("artifact contract error = %v", err)
	}

	store = repositoryArtifacts(t, map[string]string{"main.go": "package main\n"})
	engine.artifact = nil
	runner, _ = lie.New(engine)
	if _, err := runner.Run(context.Background(), store); !errors.Is(err, lie.ErrArtifactRequired) {
		t.Fatalf("nil artifact error = %v", err)
	}
}

func TestRunnerReportsEngineFailureWithImmutableDiagnostic(t *testing.T) {
	store := repositoryArtifacts(t, map[string]string{"main.go": "package main\n"})
	engine := testEngine{name: "go", version: "0.1.0", language: "Go", artifactName: "go-test", err: errors.New("analysis failed")}
	runner, _ := lie.New(engine)
	report, err := runner.Run(context.Background(), store)
	if err == nil {
		t.Fatal("expected engine failure")
	}
	diagnostics := report.FatalDiagnostics()
	if len(diagnostics) != 1 || diagnostics[0].Code != "engine_analysis_failed" {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
	diagnostics[0].Message = "changed"
	if report.FatalDiagnostics()[0].Message == "changed" {
		t.Fatal("fatal diagnostics are mutable")
	}
}

func TestRunnerPublishesAndRecordsCompletedEngine(t *testing.T) {
	store := repositoryArtifacts(t, map[string]string{"main.go": "package main\n"})
	engine := testEngine{name: "go", version: "0.1.0", language: "Go", artifactName: "go-test", artifact: testArtifact{name: "go-test", version: "0.1.0", language: "Go"}}
	runner, _ := lie.New(engine)
	report, err := runner.Run(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Engines()) != 1 || len(report.Published()) != 1 || report.FinishedAt().IsZero() {
		t.Fatalf("unexpected report: engines=%v published=%v finished=%v", report.Engines(), report.Published(), report.FinishedAt())
	}
	engines := report.Engines()
	engines[0].Name = "changed"
	if report.Engines()[0].Name != "go" {
		t.Fatal("RunReport engines are mutable")
	}
}

func TestRunnerEmptyRegistryIsValid(t *testing.T) {
	store := repositoryArtifacts(t, map[string]string{"README.md": "notes"})
	runner, _ := lie.New()
	report, err := runner.Run(context.Background(), store)
	if err != nil || len(report.Engines()) != 0 || len(report.Published()) != 0 {
		t.Fatalf("empty runner = (%+v, %v)", report, err)
	}
}

func TestDiagnosticFormatting(t *testing.T) {
	diagnostic := lie.Diagnostic{Engine: "go", Severity: lie.SeverityError, Code: "parse", Message: "bad syntax", Location: &lie.SourceRange{File: "main.go", Start: lie.Position{Line: 2, Column: 3}, End: lie.Position{Line: 2, Column: 4}}}
	if got, want := diagnostic.String(), "[go][error] parse (main.go:2:3-2:4): bad syntax"; got != want {
		t.Fatalf("Diagnostic.String() = %q, want %q", got, want)
	}
}

func repositoryArtifacts(t testing.TB, files map[string]string) *rie.ArtifactStore {
	t.Helper()
	root := t.TempDir()
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
	for _, engine := range []rie.Engine{discovery.New(), ignore.New(), language.New()} {
		if err := pipeline.Register(engine); err != nil {
			t.Fatal(err)
		}
	}
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	return run.Artifacts
}
