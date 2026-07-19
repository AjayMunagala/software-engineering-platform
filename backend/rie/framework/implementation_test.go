package framework

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	ignoreengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	languageengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

func TestFrameworkEngineDetectsManifestDependenciesWithEvidence(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{
  "dependencies": {"react": "1", "@reduxjs/toolkit": "2"}
}`)
	mustWrite(t, filepath.Join(repository, "go.mod"), "module example\nrequire github.com/gin-gonic/gin v1.10.0\n")
	mustWrite(t, filepath.Join(repository, "main.go"), "package main")

	run := rie.NewRunContext(repository, rie.DefaultConfig())
	pipeline := rie.New()
	mustRegister(t, pipeline, discovery.New())
	mustRegister(t, pipeline, ignoreengine.New())
	mustRegister(t, pipeline, languageengine.New())
	mustRegister(t, pipeline, New())
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	inventory, exists := InventoryFrom(run)
	if !exists {
		t.Fatal("FrameworkInventory artifact was not published")
	}
	if inventory.ArtifactVersion() != "1.0.0" || inventory.Metadata().EngineVersion != "0.4.2" {
		t.Errorf("Inventory metadata = %#v", inventory.Metadata())
	}
	items := inventory.Items()
	if len(items) != 3 {
		t.Fatalf("Items = %#v", items)
	}
	wantNames := []string{"Gin", "React", "Redux"}
	for index, want := range wantNames {
		if items[index].Name != want || len(items[index].Evidence()) != 1 {
			t.Errorf("Items[%d] = %#v evidence=%v", index, items[index], items[index].Evidence())
		}
	}
	if inventory.Summary().ManifestsInspected != 2 {
		t.Errorf("Summary = %#v", inventory.Summary())
	}
	if len(run.Report.Scan.Engines) != 4 || run.Report.Scan.Engines[3].Name != "framework" {
		t.Errorf("Scan.Engines = %#v", run.Report.Scan.Engines)
	}
}

func TestFrameworkEngineSupportsEveryManifestType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fileName  string
		content   string
		framework string
	}{
		{name: "Maven", fileName: "pom.xml", content: `<project><dependencies><dependency><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-web</artifactId></dependency></dependencies></project>`, framework: "Spring Boot"},
		{name: "Cargo", fileName: "Cargo.toml", content: "[dependencies]\naxum = \"0.7\"\n", framework: "Axum"},
		{name: "Composer", fileName: "composer.json", content: `{"require":{"laravel/framework":"^11"}}`, framework: "Laravel"},
		{name: "Python", fileName: "requirements.txt", content: "fastapi==0.115.0\n", framework: "FastAPI"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			detected, err := detectFrameworks(test.fileName, []byte(test.content))
			if err != nil {
				t.Fatalf("detectFrameworks() error = %v", err)
			}
			if len(detected) != 1 || detected[0].name != test.framework {
				t.Errorf("detected = %#v", detected)
			}
		})
	}
}

func TestFrameworkEngineDoesNotInspectIgnoredManifest(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, ".gitignore"), "vendor/\n")
	mustWrite(t, filepath.Join(repository, "vendor", "package.json"), `{"dependencies":{"react":"1"}}`)
	mustWrite(t, filepath.Join(repository, "main.go"), "package main")

	run := rie.NewRunContext(repository, rie.DefaultConfig())
	pipeline := rie.New()
	mustRegister(t, pipeline, discovery.New())
	mustRegister(t, pipeline, ignoreengine.New())
	mustRegister(t, pipeline, languageengine.New())
	mustRegister(t, pipeline, New())
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	inventory, _ := InventoryFrom(run)
	if len(inventory.Items()) != 0 || inventory.Summary().ManifestsInspected != 0 {
		t.Errorf("Inventory = %#v", inventory)
	}
}

func TestFrameworkEngineWarnsOnInvalidManifest(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{invalid`)
	run := readyRun(t, repository, []rie.RepositoryEntry{{Path: "package.json"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(run.Report.Warnings) != 1 || run.Report.Warnings[0].Code != "manifest_invalid" {
		t.Errorf("Warnings = %#v", run.Report.Warnings)
	}
}

func TestFrameworkInventoryIsImmutableToConsumers(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{"dependencies":{"react":"1"}}`)
	run := readyRun(t, repository, []rie.RepositoryEntry{{Path: "package.json"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, exists := InventoryFrom(run)
	if !exists {
		t.Fatal("FrameworkInventory artifact was not published")
	}
	items := inventory.Items()
	evidence := items[0].Evidence()
	evidence[0].File = "changed"
	items[0].Name = "changed"
	if inventory.Items()[0].Name != "React" || inventory.Items()[0].Evidence()[0].File != "package.json" {
		t.Error("consumer mutation changed FrameworkInventory")
	}
}

func TestFrameworkEngineRequiresLanguageEngine(t *testing.T) {
	t.Parallel()

	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	if err := New().Execute(context.Background(), run); err != ErrLanguageRequired {
		t.Errorf("Execute() error = %v, want %v", err, ErrLanguageRequired)
	}
}

func TestFrameworkEngineMetadata(t *testing.T) {
	t.Parallel()

	engine := New()
	if engine.Name() != "framework" || engine.Version() != "0.4.2" || engine.Description() == "" {
		t.Errorf("unexpected metadata: %s %s %q", engine.Name(), engine.Version(), engine.Description())
	}
}

func TestFrameworkEngineEmptyRepositoryPublishesInventory(t *testing.T) {
	t.Parallel()

	run := readyRun(t, t.TempDir(), nil)
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, exists := InventoryFrom(run)
	if !exists || len(inventory.Items()) != 0 || inventory.Summary().ManifestsInspected != 0 {
		t.Errorf("Inventory = %#v, exists = %v", inventory, exists)
	}
}

func TestFrameworkEngineDeduplicatesFrameworkAndCollectsEvidence(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{"dependencies":{"react":"1"}}`)
	mustWrite(t, filepath.Join(repository, "web", "package.json"), `{"dependencies":{"react":"1"}}`)
	run := readyRun(t, repository, []rie.RepositoryEntry{
		{Path: "package.json"}, {Path: "web/package.json"},
	})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, _ := InventoryFrom(run)
	items := inventory.Items()
	if len(items) != 1 {
		t.Fatalf("Items = %#v", items)
	}
	if len(items[0].Evidence()) != 2 {
		t.Errorf("Evidence = %v", items[0].Evidence())
	}
	wantFiles := []string{"package.json", "web/package.json"}
	for index, evidence := range items[0].Evidence() {
		if evidence.File != wantFiles[index] || evidence.Rule != "node.dependency" || evidence.Value != "react" {
			t.Errorf("Evidence[%d] = %#v", index, evidence)
		}
	}
}

func TestFrameworkEngineReportsRelatedFrameworksIndependently(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "frontend", "package.json"), `{
  "dependencies":{"react":"1","redux":"2","next":"3"}
}`)
	run := readyRun(t, repository, []rie.RepositoryEntry{{Path: "frontend/package.json"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, _ := InventoryFrom(run)
	items := inventory.Items()
	wantNames := []string{"Next.js", "React", "Redux"}
	if len(items) != len(wantNames) {
		t.Fatalf("Items = %#v", items)
	}
	for index, want := range wantNames {
		if items[index].Name != want {
			t.Errorf("Items[%d].Name = %q, want %q", index, items[index].Name, want)
		}
		evidence := items[index].Evidence()
		if len(evidence) != 1 || evidence[0].File != "frontend/package.json" {
			t.Errorf("Items[%d].Evidence = %#v", index, evidence)
		}
	}
}

func TestFrameworkEngineAllowsCoexistingFrameworks(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{
  "dependencies":{"react":"1","vue":"2"}
}`)
	run := readyRun(t, repository, []rie.RepositoryEntry{{Path: "package.json"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, _ := InventoryFrom(run)
	items := inventory.Items()
	if len(items) != 2 || items[0].Name != "React" || items[1].Name != "Vue" {
		t.Errorf("Items = %#v", items)
	}
	if len(run.Report.Warnings) != 0 {
		t.Errorf("Warnings = %#v", run.Report.Warnings)
	}
}

func TestFrameworkEnginePreservesMultipleProjectLocations(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "apps", "admin", "package.json"), `{"dependencies":{"react":"1"}}`)
	mustWrite(t, filepath.Join(repository, "apps", "store", "package.json"), `{"dependencies":{"react":"1"}}`)
	run := readyRun(t, repository, []rie.RepositoryEntry{
		{Path: "apps/admin/package.json"}, {Path: "apps/store/package.json"},
	})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, _ := InventoryFrom(run)
	items := inventory.Items()
	if len(items) != 1 {
		t.Fatalf("Items = %#v", items)
	}
	evidence := items[0].Evidence()
	if len(evidence) != 2 || evidence[0].File != "apps/admin/package.json" || evidence[1].File != "apps/store/package.json" {
		t.Errorf("Evidence = %#v", evidence)
	}
}

func TestFrameworkEngineDeduplicatesExactEvidence(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{
  "dependencies":{"react":"1"},
  "devDependencies":{"react":"1"}
}`)
	run := readyRun(t, repository, []rie.RepositoryEntry{
		{Path: "package.json"}, {Path: "package.json"},
	})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, _ := InventoryFrom(run)
	items := inventory.Items()
	if len(items) != 1 {
		t.Fatalf("Items = %#v", items)
	}
	if len(items[0].Evidence()) != 1 {
		t.Errorf("Evidence = %#v", items[0].Evidence())
	}
	if inventory.Summary().ManifestsInspected != 1 {
		t.Errorf("ManifestsInspected = %d", inventory.Summary().ManifestsInspected)
	}
}

func TestFrameworkEngineWarnsOnOversizedManifest(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{"dependencies":{"react":"1"}}`)
	run := readyRun(t, repository, []rie.RepositoryEntry{{Path: "package.json"}})
	config := DefaultConfig()
	config.MaxManifestSize = 5
	if err := New(config).Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(run.Report.Warnings) != 1 || run.Report.Warnings[0].Code != "manifest_too_large" {
		t.Errorf("Warnings = %#v", run.Report.Warnings)
	}
}

func TestFrameworkEngineRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	run := readyRun(t, t.TempDir(), nil)
	if err := New(Config{MaxManifestSize: 1}).Execute(context.Background(), run); err != ErrNoManifestNames {
		t.Errorf("empty manifest names error = %v", err)
	}
	if err := New(Config{ManifestNames: []string{"package.json"}}).Execute(context.Background(), run); err != ErrInvalidManifestLimit {
		t.Errorf("invalid size error = %v", err)
	}
}

func readyRun(t testing.TB, repository string, entries []rie.RepositoryEntry) *rie.RunContext {
	t.Helper()
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	run.Report.Repository.RootPath = repository
	run.Entries = entries
	statistics := rie.Statistics{}
	for _, entry := range entries {
		if entry.IsDir {
			statistics.Folders++
		} else {
			statistics.Files++
		}
	}
	if err := run.Artifacts.Put(rie.NewRepositorySnapshot(repository, entries, statistics, nil, "0.2.1")); err != nil {
		t.Fatalf("prepare RepositorySnapshot: %v", err)
	}
	if err := languageengine.New().Execute(context.Background(), run); err != nil {
		t.Fatalf("prepare LanguageInventory: %v", err)
	}
	return run
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
