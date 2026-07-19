package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	ignoreengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
)

func TestBuildEnginePublishesEmptyInventoryWithoutDiagnostics(t *testing.T) {
	t.Parallel()
	run := snapshotRun(t, t.TempDir(), []rie.RepositoryEntry{{Path: "README.md"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, exists := InventoryFrom(run)
	if !exists {
		t.Fatal("BuildInventory was not published")
	}
	if len(inventory.PackageManagers()) != 0 || len(inventory.BuildSystems()) != 0 || len(inventory.Workspaces()) != 0 || len(inventory.LockFiles()) != 0 || len(inventory.Toolchains()) != 0 {
		t.Errorf("inventory is not empty: %#v", inventory)
	}
	if len(run.Report.Warnings) != 0 || len(run.Report.Errors) != 0 {
		t.Errorf("diagnostics = %#v %#v", run.Report.Warnings, run.Report.Errors)
	}
	if run.Report.Build.PackageManagers == nil || run.Report.Build.BuildSystems == nil || run.Report.Build.Workspaces == nil || run.Report.Build.LockFiles == nil || run.Report.Build.Toolchains == nil {
		t.Error("empty report collections must be initialized")
	}
}

func TestBuildEngineDetectsMixedRepositoryWithLocationsAndVersions(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "frontend", "package.json"), `{"packageManager":"pnpm@9.12.0","engines":{"node":">=20"},"workspaces":["apps/*"]}`)
	mustWrite(t, filepath.Join(repository, "frontend", "pnpm-lock.yaml"), "lockfileVersion: '9.0'")
	mustWrite(t, filepath.Join(repository, "backend", "go.mod"), "module example\ngo 1.24\ntoolchain go1.24.1\n")
	mustWrite(t, filepath.Join(repository, "cli", "Cargo.toml"), "[package]\nrust-version = \"1.82\"\n[workspace]\nmembers = [\"core\", \"cmd\"]\n")
	mustWrite(t, filepath.Join(repository, "cli", "Cargo.lock"), "version = 4")
	entries := []rie.RepositoryEntry{{Path: "frontend/package.json"}, {Path: "frontend/pnpm-lock.yaml"}, {Path: "backend/go.mod"}, {Path: "cli/Cargo.toml"}, {Path: "cli/Cargo.lock"}}
	run := snapshotRun(t, repository, entries)
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	inventory, _ := InventoryFrom(run)
	assertTool(t, inventory.PackageManagers(), "pnpm", "frontend")
	assertTool(t, inventory.PackageManagers(), "go-modules", "backend")
	assertTool(t, inventory.PackageManagers(), "cargo", "cli")
	assertBuildSystem(t, inventory.BuildSystems(), "go-toolchain", "backend")
	assertBuildSystem(t, inventory.BuildSystems(), "cargo", "cli")
	if len(inventory.LockFiles()) != 2 || inventory.LockFiles()[0].Path != "cli/Cargo.lock" || inventory.LockFiles()[1].Path != "frontend/pnpm-lock.yaml" {
		t.Errorf("LockFiles = %#v", inventory.LockFiles())
	}
	if len(inventory.Workspaces()) != 2 {
		t.Fatalf("Workspaces = %#v", inventory.Workspaces())
	}
	if inventory.Workspaces()[0].Location != "cli" || inventory.Workspaces()[1].Location != "frontend" {
		t.Errorf("Workspace locations = %#v", inventory.Workspaces())
	}
	wantToolchains := map[string]string{"Go": "1.24", "Node.js": ">=20", "Rust": "1.82", "pnpm": "9.12.0"}
	for _, toolchain := range inventory.Toolchains() {
		if want, ok := wantToolchains[toolchain.Tool]; ok && toolchain.Constraint == want {
			delete(wantToolchains, toolchain.Tool)
		}
	}
	if len(wantToolchains) != 0 {
		t.Errorf("missing toolchains %v; got %#v", wantToolchains, inventory.Toolchains())
	}
	for _, manager := range inventory.PackageManagers() {
		if len(manager.Evidence()) == 0 || manager.Evidence()[0].File == "" {
			t.Errorf("missing evidence: %#v", manager)
		}
	}
}

func TestBuildEngineTreatsCargoAsTwoIndependentRoles(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "Cargo.toml"), "[package]\nname = \"demo\"\n")
	run := snapshotRun(t, repository, []rie.RepositoryEntry{{Path: "Cargo.toml"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	assertTool(t, inventory.PackageManagers(), "cargo", ".")
	assertBuildSystem(t, inventory.BuildSystems(), "cargo", ".")
}

func TestBuildEngineSupportsRemainingManifestEcosystems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		fileName       string
		content        string
		managerID      string
		buildSystemID  string
		workspaceID    string
		toolchainName  string
		toolchainValue string
	}{
		{name: "Go workspace", fileName: "go.work", content: "go 1.24\nuse (\n ./api\n ./worker\n)\n", workspaceID: "go-workspace", toolchainName: "Go", toolchainValue: "1.24"},
		{name: "Maven", fileName: "pom.xml", content: `<project><modules><module>api</module><module>worker</module></modules></project>`, managerID: "maven", buildSystemID: "maven", workspaceID: "maven-multi-module"},
		{name: "Gradle", fileName: "settings.gradle", content: `include ':api', ':worker'`, buildSystemID: "gradle", workspaceID: "gradle-multi-project"},
		{name: "pip", fileName: "requirements.txt", content: "flask==3.0", managerID: "pip"},
		{name: "Python project", fileName: "pyproject.toml", content: "[build-system]\nbuild-backend = \"setuptools.build_meta\"\n[project]\nrequires-python = \">=3.12\"\n[tool.poetry]\n", managerID: "poetry", buildSystemID: "setuptools", toolchainName: "Python", toolchainValue: ">=3.12"},
		{name: "Composer", fileName: "composer.json", content: `{"require":{"php":"^8.3"}}`, managerID: "composer", toolchainName: "PHP", toolchainValue: "^8.3"},
		{name: "pnpm workspace", fileName: "pnpm-workspace.yaml", content: "packages:\n  - 'apps/*'\n  - 'packages/*'\n", managerID: "pnpm", workspaceID: "pnpm-workspace"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := t.TempDir()
			mustWrite(t, filepath.Join(repository, test.fileName), test.content)
			run := snapshotRun(t, repository, []rie.RepositoryEntry{{Path: test.fileName}})
			if err := New().Execute(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			inventory, _ := InventoryFrom(run)
			if test.managerID != "" {
				assertTool(t, inventory.PackageManagers(), test.managerID, ".")
			}
			if test.buildSystemID != "" {
				assertBuildSystem(t, inventory.BuildSystems(), test.buildSystemID, ".")
			}
			if test.workspaceID != "" {
				found := false
				for _, workspace := range inventory.Workspaces() {
					if workspace.ID == test.workspaceID {
						found = true
					}
				}
				if !found {
					t.Errorf("missing workspace %s: %#v", test.workspaceID, inventory.Workspaces())
				}
			}
			if test.toolchainName != "" {
				found := false
				for _, toolchain := range inventory.Toolchains() {
					if toolchain.Tool == test.toolchainName && toolchain.Constraint == test.toolchainValue {
						found = true
					}
				}
				if !found {
					t.Errorf("missing toolchain %s %s: %#v", test.toolchainName, test.toolchainValue, inventory.Toolchains())
				}
			}
		})
	}
}

func TestBuildEngineSupportsEveryLockFile(t *testing.T) {
	t.Parallel()
	tests := []struct{ fileName, managerID string }{
		{fileName: "package-lock.json", managerID: "npm"},
		{fileName: "pnpm-lock.yaml", managerID: "pnpm"},
		{fileName: "yarn.lock", managerID: "yarn"},
		{fileName: "bun.lock", managerID: "bun"},
		{fileName: "Cargo.lock", managerID: "cargo"},
		{fileName: "poetry.lock", managerID: "poetry"},
		{fileName: "uv.lock", managerID: "uv"},
		{fileName: "composer.lock", managerID: "composer"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.fileName, func(t *testing.T) {
			t.Parallel()
			repository := t.TempDir()
			mustWrite(t, filepath.Join(repository, test.fileName), "")
			run := snapshotRun(t, repository, []rie.RepositoryEntry{{Path: test.fileName}})
			if err := New().Execute(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			inventory, _ := InventoryFrom(run)
			assertTool(t, inventory.PackageManagers(), test.managerID, ".")
			if len(inventory.LockFiles()) != 1 || inventory.LockFiles()[0].PackageManagerID != test.managerID {
				t.Errorf("LockFiles = %#v", inventory.LockFiles())
			}
		})
	}
}

func TestBuildEngineUsesCanonicalManifestCasing(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "PACKAGE.JSON"), `{"packageManager":"pnpm@9"}`)
	run := snapshotRun(t, repository, []rie.RepositoryEntry{{Path: "PACKAGE.JSON"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	if len(inventory.PackageManagers()) != 0 {
		t.Errorf("PackageManagers = %#v", inventory.PackageManagers())
	}
}

func TestBuildEngineDoesNotGuessNPMFromPackageJSON(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{"name":"notes"}`)
	run := snapshotRun(t, repository, []rie.RepositoryEntry{{Path: "package.json"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	if len(inventory.PackageManagers()) != 0 {
		t.Errorf("PackageManagers = %#v", inventory.PackageManagers())
	}
}

func TestBuildEngineRetainsConflictingNodeManagersWithWarning(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package-lock.json"), `{}`)
	mustWrite(t, filepath.Join(repository, "yarn.lock"), "")
	run := snapshotRun(t, repository, []rie.RepositoryEntry{{Path: "package-lock.json"}, {Path: "yarn.lock"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	if len(inventory.PackageManagers()) != 2 {
		t.Errorf("PackageManagers = %#v", inventory.PackageManagers())
	}
	if len(run.Report.Warnings) != 1 || run.Report.Warnings[0].Code != "multiple_package_managers" {
		t.Errorf("Warnings = %#v", run.Report.Warnings)
	}
}

func TestBuildEngineConsumesSnapshotInsteadOfMutableRunEntries(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "go.mod"), "module example\n")
	run := snapshotRun(t, repository, []rie.RepositoryEntry{{Path: "go.mod"}})
	run.Entries = []rie.RepositoryEntry{{Path: "unrelated.txt"}}
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	assertTool(t, inventory.PackageManagers(), "go-modules", ".")
}

func TestBuildEngineIgnoresFilesRemovedFromSnapshot(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, ".gitignore"), "vendor/\n")
	mustWrite(t, filepath.Join(repository, "vendor", "package-lock.json"), `{}`)
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	pipeline := rie.New()
	mustRegister(t, pipeline, discovery.New())
	mustRegister(t, pipeline, ignoreengine.New())
	mustRegister(t, pipeline, New())
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	if len(inventory.PackageManagers()) != 0 || len(inventory.LockFiles()) != 0 {
		t.Errorf("Inventory = %#v", inventory)
	}
}

func TestBuildInventoryIsImmutable(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{"packageManager":"pnpm@9","workspaces":["apps/*"]}`)
	run := snapshotRun(t, repository, []rie.RepositoryEntry{{Path: "package.json"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	inventory, _ := InventoryFrom(run)
	managers := inventory.PackageManagers()
	managers[0].Name = "changed"
	managerEvidence := managers[0].Evidence()
	managerEvidence[0].File = "changed"
	workspaces := inventory.Workspaces()
	members := workspaces[0].Members()
	members[0] = "changed"
	if inventory.PackageManagers()[0].Name == "changed" || inventory.PackageManagers()[0].Evidence()[0].File == "changed" || inventory.Workspaces()[0].Members()[0] == "changed" {
		t.Error("consumer mutation changed BuildInventory")
	}
	if inventory.ArtifactVersion() != "1.0.0" || inventory.Metadata().EngineVersion != "0.5.0" {
		t.Errorf("Metadata = %#v", inventory.Metadata())
	}
}

func TestBuildEngineWarnsOnInvalidAndOversizedManifest(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "package.json"), `{invalid`)
	run := snapshotRun(t, repository, []rie.RepositoryEntry{{Path: "package.json"}})
	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	if len(run.Report.Warnings) != 1 || run.Report.Warnings[0].Code != "build_manifest_invalid" {
		t.Errorf("Warnings = %#v", run.Report.Warnings)
	}
	config := DefaultConfig()
	config.MaxManifestSize = 2
	run = snapshotRun(t, repository, []rie.RepositoryEntry{{Path: "package.json"}})
	if err := New(config).Execute(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	if len(run.Report.Warnings) != 1 || run.Report.Warnings[0].Code != "build_manifest_too_large" {
		t.Errorf("Warnings = %#v", run.Report.Warnings)
	}
}

func TestBuildEngineValidatesPrerequisiteAndRegistry(t *testing.T) {
	t.Parallel()
	if err := New().Execute(context.Background(), rie.NewRunContext(t.TempDir(), rie.DefaultConfig())); err != ErrSnapshotRequired {
		t.Errorf("missing snapshot error = %v", err)
	}
	run := snapshotRun(t, t.TempDir(), nil)
	config := DefaultConfig()
	config.MaxManifestSize = 0
	if err := New(config).Execute(context.Background(), run); err != ErrInvalidManifestSize {
		t.Errorf("size error = %v", err)
	}
	config = DefaultConfig()
	config.Detectors = nil
	if err := New(config).Execute(context.Background(), run); err != ErrNoDetectors {
		t.Errorf("detector error = %v", err)
	}
	config = DefaultConfig()
	config.Detectors = append(config.Detectors, config.Detectors[0])
	if err := New(config).Execute(context.Background(), run); err != ErrInvalidDetector {
		t.Errorf("duplicate detector error = %v", err)
	}
}

func TestBuildEngineMetadata(t *testing.T) {
	t.Parallel()
	engine := New()
	if engine.Name() != "build-package" || engine.Version() != "0.5.0" || engine.Description() == "" {
		t.Errorf("metadata = %s %s %q", engine.Name(), engine.Version(), engine.Description())
	}
}

func snapshotRun(t testing.TB, repository string, entries []rie.RepositoryEntry) *rie.RunContext {
	t.Helper()
	run := rie.NewRunContext(repository, rie.DefaultConfig())
	statistics := rie.Statistics{}
	for _, entry := range entries {
		if entry.IsDir {
			statistics.Folders++
		} else {
			statistics.Files++
		}
	}
	snapshot := rie.NewRepositorySnapshot(repository, entries, statistics, nil, "0.2.1")
	if err := run.Artifacts.Put(snapshot); err != nil {
		t.Fatal(err)
	}
	return run
}

func assertTool(t *testing.T, items []PackageManager, id, location string) {
	t.Helper()
	for _, item := range items {
		if item.ID == id && item.Location == location {
			return
		}
	}
	t.Errorf("missing package manager %s at %s: %#v", id, location, items)
}

func assertBuildSystem(t *testing.T, items []BuildSystem, id, location string) {
	t.Helper()
	for _, item := range items {
		if item.ID == id && item.Location == location {
			return
		}
	}
	t.Errorf("missing build system %s at %s: %#v", id, location, items)
}

func mustRegister(t *testing.T, pipeline *rie.Pipeline, engine rie.Engine) {
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
