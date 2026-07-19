package language

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	ignoreengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
)

func TestLanguageEngineDetectsFilteredRepositoryLanguages(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, ".gitignore"), "generated.py\n")
	mustWrite(t, filepath.Join(repository, "main.go"), "package main")
	mustWrite(t, filepath.Join(repository, "service.go"), "package service")
	mustWrite(t, filepath.Join(repository, "web", "app.tsx"), "export default App")
	mustWrite(t, filepath.Join(repository, "schema.sql"), "create table example")
	mustWrite(t, filepath.Join(repository, "generated.py"), "print('ignored')")
	mustWrite(t, filepath.Join(repository, "README.md"), "fixture")

	run := rie.NewRunContext(repository, rie.DefaultConfig())
	pipeline := rie.New()
	mustRegister(t, pipeline, discovery.New())
	mustRegister(t, pipeline, ignoreengine.New())
	mustRegister(t, pipeline, New())
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if run.Report.Languages.DetectedFiles != 4 {
		t.Errorf("DetectedFiles = %d, want 4", run.Report.Languages.DetectedFiles)
	}
	if run.Report.Languages.UnknownFiles != 2 { // .gitignore and README.md
		t.Errorf("UnknownFiles = %d, want 2", run.Report.Languages.UnknownFiles)
	}
	want := []Detection{
		{Name: "Go", FileCount: 2, Percentage: 50},
		{Name: "SQL", FileCount: 1, Percentage: 25},
		{Name: "TypeScript", FileCount: 1, Percentage: 25},
	}
	if len(run.Report.Languages.Items) != len(want) {
		t.Fatalf("Items = %#v", run.Report.Languages.Items)
	}
	for index := range want {
		if run.Report.Languages.Items[index] != want[index] {
			t.Errorf("Items[%d] = %#v, want %#v", index, run.Report.Languages.Items[index], want[index])
		}
	}
}

func TestLanguageEngineUsesCaseInsensitiveCustomMappings(t *testing.T) {
	t.Parallel()

	engine := New(Config{Extensions: map[string]string{"KT": "Kotlin"}})
	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	run.CompletedEngines["ignore"] = "0.2.0"
	run.Entries = []rie.RepositoryEntry{{Path: "Main.KT"}}

	if err := engine.Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(run.Report.Languages.Items) != 1 || run.Report.Languages.Items[0].Name != "Kotlin" {
		t.Errorf("Items = %#v", run.Report.Languages.Items)
	}
}

func TestLanguageEngineSupportsEveryV03Extension(t *testing.T) {
	t.Parallel()

	entries := []rie.RepositoryEntry{
		{Path: "main.go"},
		{Path: "app.ts"},
		{Path: "view.tsx"},
		{Path: "index.js"},
		{Path: "widget.jsx"},
		{Path: "tool.py"},
		{Path: "Main.java"},
		{Path: "Program.cs"},
		{Path: "schema.sql"},
	}
	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	run.CompletedEngines["ignore"] = "0.2.0"
	run.Entries = entries

	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if run.Report.Languages.DetectedFiles != len(entries) || run.Report.Languages.UnknownFiles != 0 {
		t.Errorf("Languages = %#v", run.Report.Languages)
	}
	if len(run.Report.Languages.Items) != 7 {
		t.Errorf("len(Items) = %d, want 7", len(run.Report.Languages.Items))
	}
}

func TestLanguageEngineRejectsEmptyMappings(t *testing.T) {
	t.Parallel()

	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	run.CompletedEngines["ignore"] = "0.2.0"
	if err := New(Config{}).Execute(context.Background(), run); err != ErrNoExtensionMappings {
		t.Errorf("Execute() error = %v, want %v", err, ErrNoExtensionMappings)
	}
}

func TestLanguageEngineRequiresIgnoreEngine(t *testing.T) {
	t.Parallel()

	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	if err := New().Execute(context.Background(), run); err != ErrIgnoreRequired {
		t.Errorf("Execute() error = %v, want %v", err, ErrIgnoreRequired)
	}
}

func TestLanguageEngineMetadata(t *testing.T) {
	t.Parallel()

	engine := New()
	if engine.Name() != "language" || engine.Version() != "0.3.0" || engine.Description() == "" {
		t.Errorf("unexpected metadata: %s %s %q", engine.Name(), engine.Version(), engine.Description())
	}
}

func mustRegister(t *testing.T, pipeline *rie.Pipeline, engine rie.Engine) {
	t.Helper()
	if err := pipeline.Register(engine); err != nil {
		t.Fatalf("Register(%s): %v", engine.Name(), err)
	}
}

func mustWrite(t *testing.T, filePath, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
