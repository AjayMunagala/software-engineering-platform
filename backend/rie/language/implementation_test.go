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
	want := []LanguageItem{
		{Name: "Go", Count: 2, Percentage: 50},
		{Name: "SQL", Count: 1, Percentage: 25},
		{Name: "TypeScript", Count: 1, Percentage: 25},
	}
	inventory, exists := InventoryFrom(run)
	if !exists {
		t.Fatal("LanguageInventory artifact was not published")
	}
	items := inventory.Items()
	if len(items) != len(want) {
		t.Fatalf("Items = %#v", items)
	}
	for index := range want {
		if items[index] != want[index] {
			t.Errorf("Items[%d] = %#v, want %#v", index, items[index], want[index])
		}
	}
}

func TestLanguageEngineUsesCaseInsensitiveCustomMappings(t *testing.T) {
	t.Parallel()

	engine := New(Config{Extensions: map[string]string{"KT": "Kotlin"}})
	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	run.Entries = []rie.RepositoryEntry{{Path: "Main.KT"}}
	publishSnapshot(t, run)

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
	run.Entries = entries
	publishSnapshot(t, run)

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
	publishSnapshot(t, run)
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
	if engine.Name() != "language" || engine.Version() != "0.3.2" || engine.Description() == "" {
		t.Errorf("unexpected metadata: %s %s %q", engine.Name(), engine.Version(), engine.Description())
	}
}

func TestLanguageInventoryIsImmutableToConsumers(t *testing.T) {
	t.Parallel()

	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	run.Entries = []rie.RepositoryEntry{{Path: "main.go"}}
	publishSnapshot(t, run)
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, exists := InventoryFrom(run)
	if !exists {
		t.Fatal("LanguageInventory artifact was not published")
	}
	items := inventory.Items()
	items[0].Name = "Modified"
	if inventory.Items()[0].Name != "Go" {
		t.Error("consumer mutation changed LanguageInventory")
	}
	metadata := inventory.Metadata()
	if metadata.Version != LanguageInventoryArtifactVersion || metadata.EngineVersion != "0.3.2" {
		t.Errorf("Metadata = %#v", metadata)
	}
}

func TestLanguageEngineEmptyRepository(t *testing.T) {
	t.Parallel()

	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	publishSnapshot(t, run)
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, exists := InventoryFrom(run)
	if !exists || inventory.Summary() != (LanguageSummary{}) || len(inventory.Items()) != 0 {
		t.Errorf("Inventory = %#v, exists = %v", inventory, exists)
	}
}

func TestLanguageEngineCountsExtensionlessAndUnknownFiles(t *testing.T) {
	t.Parallel()

	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	run.Entries = []rie.RepositoryEntry{
		{Path: "abc.xyz"}, {Path: "myfile"}, {Path: "data.custom"},
	}
	publishSnapshot(t, run)
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, _ := InventoryFrom(run)
	if inventory.Summary().UnknownFiles != 3 || inventory.Summary().DetectedFiles != 0 {
		t.Errorf("Summary = %#v", inventory.Summary())
	}
	if len(run.Report.Warnings) != 0 || len(run.Report.Errors) != 0 {
		t.Errorf("unknown files created diagnostics: %#v %#v", run.Report.Warnings, run.Report.Errors)
	}
}

func TestLanguageEngineDefaultMappingIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	run.Entries = []rie.RepositoryEntry{
		{Path: "main.go"}, {Path: "MAIN.GO"}, {Path: "Main.Go"},
	}
	publishSnapshot(t, run)
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, _ := InventoryFrom(run)
	items := inventory.Items()
	if len(items) != 1 || items[0].Name != "Go" || items[0].Count != 3 {
		t.Errorf("Items = %#v", items)
	}
}

func mustRegister(t *testing.T, pipeline *rie.Pipeline, engine rie.Engine) {
	t.Helper()
	if err := pipeline.Register(engine); err != nil {
		t.Fatalf("Register(%s): %v", engine.Name(), err)
	}
}

func publishSnapshot(t testing.TB, run *rie.RunContext) {
	t.Helper()
	statistics := rie.Statistics{}
	for _, entry := range run.Entries {
		if entry.IsDir {
			statistics.Folders++
		} else {
			statistics.Files++
		}
	}
	if err := run.Artifacts.Put(rie.NewRepositorySnapshot(run.RepositoryPath, run.Entries, statistics, nil, "0.2.1")); err != nil {
		t.Fatal(err)
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
