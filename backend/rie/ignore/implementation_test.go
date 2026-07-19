package ignore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
)

func TestIgnoreEngineFiltersRepositoryEntries(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, ".gitignore"), "node_modules/\n*.log\n!important.log\n")
	mustWrite(t, filepath.Join(repository, "main.go"), "package main")
	mustWrite(t, filepath.Join(repository, "debug.log"), "debug")
	mustWrite(t, filepath.Join(repository, "important.log"), "keep")
	mustWrite(t, filepath.Join(repository, "node_modules", "library.js"), "module")

	run := rie.NewRunContext(repository, rie.DefaultConfig())
	pipeline := rie.New()
	mustRegister(t, pipeline, discovery.New())
	mustRegister(t, pipeline, New())
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if run.Report.Statistics.Files != 3 { // .gitignore, main.go, important.log
		t.Errorf("Statistics.Files = %d, want 3", run.Report.Statistics.Files)
	}
	if run.Report.Statistics.Folders != 0 {
		t.Errorf("Statistics.Folders = %d, want 0", run.Report.Statistics.Folders)
	}
	if run.Report.Ignore.IgnoredFiles != 2 || run.Report.Ignore.IgnoredFolders != 1 {
		t.Errorf("Ignore = %#v", run.Report.Ignore)
	}
	if run.Report.Ignore.Rules != 3 {
		t.Errorf("Ignore.Rules = %d, want 3", run.Report.Ignore.Rules)
	}
	snapshot, exists := rie.RepositorySnapshotFrom(run)
	if !exists {
		t.Fatal("RepositorySnapshot was not published")
	}
	if snapshot.ArtifactVersion() != "1.0.0" || snapshot.Metadata().EngineVersion != "0.2.1" {
		t.Errorf("snapshot metadata = %#v", snapshot.Metadata())
	}
	entries := snapshot.Entries()
	entries[0].Path = "changed"
	_ = snapshot.ForEachEntry(func(entry rie.RepositoryEntry) error {
		entry.Path = "visitor-change"
		return nil
	})
	if snapshot.Entries()[0].Path == "changed" || snapshot.Statistics().Files != 3 {
		t.Error("RepositorySnapshot is mutable to consumers")
	}
	if snapshot.Entries()[0].Path == "visitor-change" {
		t.Error("RepositorySnapshot visitor exposed mutable state")
	}
}

func TestIgnoreEngineAppliesNestedRulesAndConfigOverrides(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "service", ".gitignore"), "generated/\n")
	mustWrite(t, filepath.Join(repository, "service", "generated", "file.go"), "generated")
	mustWrite(t, filepath.Join(repository, "service", "keep.tmp"), "temporary")

	config := rie.DefaultConfig()
	config.IgnorePatterns = []string{"*.tmp"}
	run := rie.NewRunContext(repository, config)
	pipeline := rie.New()
	mustRegister(t, pipeline, discovery.New())
	mustRegister(t, pipeline, New())
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if run.Report.Ignore.IgnoredFiles != 2 || run.Report.Ignore.IgnoredFolders != 1 {
		t.Errorf("Ignore = %#v", run.Report.Ignore)
	}
	if len(run.Report.Ignore.Sources) != 2 {
		t.Errorf("Ignore.Sources = %#v", run.Report.Ignore.Sources)
	}
}

func TestIgnoreEngineCanExcludeHiddenEntries(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWrite(t, filepath.Join(repository, "visible.go"), "visible")
	mustWrite(t, filepath.Join(repository, ".private", "hidden.go"), "hidden")
	config := rie.DefaultConfig()
	config.ScanHidden = false
	run := rie.NewRunContext(repository, config)
	pipeline := rie.New()
	mustRegister(t, pipeline, discovery.New())
	mustRegister(t, pipeline, New())
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Report.Statistics.Files != 1 || run.Report.Ignore.IgnoredFiles != 1 {
		t.Errorf("report = %#v", run.Report)
	}
}

func TestIgnoreEngineRequiresDiscovery(t *testing.T) {
	t.Parallel()

	run := rie.NewRunContext(t.TempDir(), rie.DefaultConfig())
	if err := New().Execute(context.Background(), run); err != ErrDiscoveryRequired {
		t.Errorf("Execute() error = %v, want %v", err, ErrDiscoveryRequired)
	}
}

func TestIgnoreEngineMetadata(t *testing.T) {
	t.Parallel()

	engine := New()
	if engine.Name() != "ignore" || engine.Version() != "0.2.1" || engine.Description() == "" {
		t.Errorf("unexpected metadata: %s %s %q", engine.Name(), engine.Version(), engine.Description())
	}
}

func TestIgnoreEngineDoubleStarMatchesRootAndNestedPaths(t *testing.T) {
	t.Parallel()

	compiledRule, ok, err := compileRule("**/generated.go", "", ".gitignore")
	if err != nil || !ok {
		t.Fatalf("compileRule() = %#v, %v, %v", compiledRule, ok, err)
	}
	for _, entry := range []rie.RepositoryEntry{
		{Path: "generated.go"},
		{Path: "internal/generated.go"},
	} {
		if !matchesRules(entry, []rule{compiledRule}) {
			t.Errorf("%q was not ignored", entry.Path)
		}
	}
}

func TestIgnoreEngineDirectoryRuleDoesNotIgnoreSameNamedFile(t *testing.T) {
	t.Parallel()

	compiledRule, ok, err := compileRule("cache*/", "", ".gitignore")
	if err != nil || !ok {
		t.Fatalf("compileRule() = %#v, %v, %v", compiledRule, ok, err)
	}
	if matchesRules(rie.RepositoryEntry{Path: "cache-one"}, []rule{compiledRule}) {
		t.Error("directory-only rule ignored a same-named file")
	}
	if !matchesRules(rie.RepositoryEntry{Path: "cache-one", IsDir: true}, []rule{compiledRule}) {
		t.Error("directory-only rule did not ignore a matching directory")
	}
	if !matchesRules(rie.RepositoryEntry{Path: "cache-one/data.bin"}, []rule{compiledRule}) {
		t.Error("directory-only rule did not ignore a descendant")
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
