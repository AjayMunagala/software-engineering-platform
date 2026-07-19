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
	evidence[0] = "changed"
	items[0].Name = "changed"
	if inventory.Items()[0].Name != "React" || inventory.Items()[0].Evidence()[0] != "package.json" {
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
	if engine.Name() != "framework" || engine.Version() != "0.4.0" || engine.Description() == "" {
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
	run.CompletedEngines["ignore"] = "0.2.0"
	if err := languageengine.New().Execute(context.Background(), run); err != nil {
		t.Fatalf("prepare LanguageInventory: %v", err)
	}
	run.CompletedEngines["language"] = "0.3.1"
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
