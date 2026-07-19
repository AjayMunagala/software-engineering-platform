package metadata

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	buildengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/build"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	frameworkengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/framework"
	ignoreengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	languageengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

func TestMetadataEngineSynthesizesRepositoryCoverPage(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, ".git", "HEAD"), "ref: refs/heads/feature/metadata\n")
	mustWrite(t, filepath.Join(repository, ".git", "refs", "remotes", "origin", "HEAD"), "ref: refs/remotes/origin/main\n")
	mustWrite(t, filepath.Join(repository, "README.md"), "fixture")
	mustWrite(t, filepath.Join(repository, "backend", "go.mod"), "module example\ngo 1.24\n")
	mustWrite(t, filepath.Join(repository, "backend", "main.go"), "package main")
	mustWrite(t, filepath.Join(repository, "frontend", "package.json"), `{"packageManager":"pnpm@9","workspaces":["apps/*","packages/*"],"dependencies":{"react":"19"}}`)
	mustWrite(t, filepath.Join(repository, "frontend", "app.tsx"), "export default App")

	run := rie.NewRunContext(repository, rie.DefaultConfig())
	pipeline := completePipeline(t, true)
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	inventory, exists := InventoryFrom(run)
	if !exists {
		t.Fatal("RepositoryMetadata was not published")
	}
	identity := inventory.Repository()
	if identity.Name != filepath.Base(repository) || identity.RootPath != repository || !identity.Git.Present || identity.Git.CurrentBranch != "feature/metadata" || identity.Git.DefaultBranch != "main" {
		t.Errorf("Repository = %#v", identity)
	}
	if !inventory.Monorepo() || inventory.WorkspaceCount() != 1 || inventory.DeclaredModuleCount() != 2 {
		t.Errorf("monorepo=%v workspaces=%d modules=%d", inventory.Monorepo(), inventory.WorkspaceCount(), inventory.DeclaredModuleCount())
	}
	layout := inventory.Layout()
	if len(layout.TopLevelDirectories()) != 2 || layout.TopLevelDirectories()[0] != "backend" || layout.TopLevelDirectories()[1] != "frontend" || len(layout.TopLevelFiles()) != 1 || layout.TopLevelFiles()[0] != "README.md" {
		t.Errorf("Layout = %#v dirs=%v files=%v", layout, layout.TopLevelDirectories(), layout.TopLevelFiles())
	}
	if len(inventory.Languages()) != 2 {
		t.Errorf("Languages = %#v", inventory.Languages())
	}
	frameworks := inventory.Frameworks()
	if len(frameworks) != 1 || frameworks[0].Name != "React" || len(frameworks[0].Locations()) != 1 || frameworks[0].Locations()[0] != "frontend" {
		t.Errorf("Frameworks = %#v", frameworks)
	}
	build := inventory.Build()
	if len(build.PackageManagers()) != 2 || len(build.BuildSystems()) != 1 || len(build.Toolchains()) < 2 {
		t.Errorf("Build = %#v", build)
	}
	if len(inventory.SourceArtifacts()) != 5 || inventory.ArtifactVersion() != "1.0.0" {
		t.Errorf("Sources = %#v metadata=%#v", inventory.SourceArtifacts(), inventory.Metadata())
	}
	if inventory.Metadata().EngineVersion != "0.6.0" {
		t.Errorf("Metadata = %#v", inventory.Metadata())
	}
	if run.Report.Metadata.Name != identity.Name || !run.Report.Metadata.Monorepo {
		t.Errorf("Report.Metadata = %#v", run.Report.Metadata)
	}
}

func TestMetadataEngineDoesNotConsumeMutablePresentationState(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "go.mod"), "module example\ngo 1.24\n")
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	if err := completePipeline(t, false).Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	run.Report.Repository.Name = "incorrect"
	run.Report.Repository.RootPath = "incorrect"
	run.Entries = nil
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	if inventory.Repository().Name != filepath.Base(repository) || inventory.Repository().RootPath != repository {
		t.Errorf("Repository = %#v", inventory.Repository())
	}
}

func TestMetadataEngineProducesValidEmptySummary(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	if err := completePipeline(t, true).Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	if inventory.Monorepo() || inventory.WorkspaceCount() != 0 || inventory.DeclaredModuleCount() != 0 || len(inventory.Languages()) != 0 || len(inventory.Frameworks()) != 0 || len(inventory.Build().PackageManagers()) != 0 {
		t.Errorf("Inventory = %#v", inventory)
	}
	if len(run.Report.Warnings) != 0 || len(run.Report.Errors) != 0 {
		t.Errorf("diagnostics = %#v %#v", run.Report.Warnings, run.Report.Errors)
	}
}

func TestMetadataInventoryIsImmutable(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{"packageManager":"pnpm@9","workspaces":["apps/*","packages/*"],"dependencies":{"react":"19"}}`)
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	if err := completePipeline(t, true).Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	layout := inventory.Layout()
	directories := layout.TopLevelDirectories()
	if len(directories) > 0 {
		directories[0] = "changed"
	}
	frameworks := inventory.Frameworks()
	locations := frameworks[0].Locations()
	locations[0] = "changed"
	build := inventory.Build()
	managers := build.PackageManagers()
	managerLocations := managers[0].Locations()
	managerLocations[0] = "changed"
	sources := inventory.SourceArtifacts()
	sources[0].Name = "changed"
	if inventory.Frameworks()[0].Locations()[0] == "changed" || inventory.Build().PackageManagers()[0].Locations()[0] == "changed" || inventory.SourceArtifacts()[0].Name == "changed" {
		t.Error("consumer mutation changed RepositoryMetadata")
	}
}

func TestMetadataEngineRequiresEveryArtifact(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	tests := []struct {
		name        string
		engineCount int
		want        error
	}{
		{name: "discovery", engineCount: 0, want: ErrDiscoveryRequired},
		{name: "snapshot", engineCount: 1, want: ErrSnapshotRequired},
		{name: "language", engineCount: 2, want: ErrLanguageRequired},
		{name: "framework", engineCount: 3, want: ErrFrameworkRequired},
		{name: "build", engineCount: 4, want: ErrBuildRequired},
	}
	engines := []rie.Engine{discovery.New(), ignoreengine.New(), languageengine.New(), frameworkengine.New(), buildengine.New()}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			run := rie.NewRunContext(repository, rie.DefaultConfig())
			pipeline := rie.New()
			for _, engine := range engines[:test.engineCount] {
				mustRegister(t, pipeline, engine)
			}
			mustRegister(t, pipeline, New())
			if err := pipeline.Run(context.Background(), run); !errors.Is(err, test.want) {
				t.Errorf("Run() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestMetadataEngineRejectsInvalidConfig(t *testing.T) {
	t.Parallel()
	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	if err := New(Config{}).Execute(context.Background(), run); err != ErrInvalidConfig {
		t.Errorf("Execute() error = %v", err)
	}
}

func TestMetadataEngineMetadata(t *testing.T) {
	t.Parallel()
	engine := New()
	if engine.Name() != "repository-metadata" || engine.Version() != "0.6.0" || engine.Description() == "" {
		t.Errorf("metadata = %s %s %q", engine.Name(), engine.Version(), engine.Description())
	}
}

func completePipeline(t testing.TB, includeMetadata bool) *rie.Pipeline {
	t.Helper()
	pipeline := rie.New()
	for _, engine := range []rie.Engine{discovery.New(), ignoreengine.New(), languageengine.New(), frameworkengine.New(), buildengine.New()} {
		mustRegister(t, pipeline, engine)
	}
	if includeMetadata {
		mustRegister(t, pipeline, New())
	}
	return pipeline
}

func mustRegister(t testing.TB, pipeline *rie.Pipeline, engine rie.Engine) {
	t.Helper()
	if err := pipeline.Register(engine); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t testing.TB, filePath, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
