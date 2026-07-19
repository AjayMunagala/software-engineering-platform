package discovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

func TestDiscoveryEngineScan(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWriteFile(t, filepath.Join(repository, "main.go"))
	mustWriteFile(t, filepath.Join(repository, "internal", "service.go"))
	mustWriteFile(t, filepath.Join(repository, ".git", "config"))

	report, err := New().Scan(repository)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if report.SchemaVersion != rie.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", report.SchemaVersion, rie.SchemaVersion)
	}
	if report.Repository.Name != filepath.Base(repository) {
		t.Errorf("Repository.Name = %q", report.Repository.Name)
	}
	if report.Statistics.Files != 2 || report.Statistics.Folders != 1 {
		t.Errorf("Statistics = %#v", report.Statistics)
	}
	if !report.Repository.Git {
		t.Error("Repository.Git = false, want true")
	}
	if report.Scan.ID == "" || report.Scan.FinishedAt.IsZero() {
		t.Error("scan metadata is incomplete")
	}
}

func TestDiscoveryEngineProducesNormalizedEntries(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	mustWriteFile(t, filepath.Join(repository, "internal", "service.go"))
	run := rie.NewRunContext(repository, rie.DefaultConfig())

	if err := New().Execute(context.Background(), run); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(run.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(run.Entries))
	}
	if run.Entries[0].Path != "internal" || run.Entries[1].Path != "internal/service.go" {
		t.Errorf("Entries = %#v", run.Entries)
	}
}

func TestDiscoveryEngineRejectsInvalidRoots(t *testing.T) {
	t.Parallel()

	if _, err := New().Scan(""); !errors.Is(err, ErrRepositoryPathRequired) {
		t.Errorf("Scan(\"\") error = %v", err)
	}
	file := filepath.Join(t.TempDir(), "not-a-directory")
	mustWriteFile(t, file)
	if _, err := New().Scan(file); !errors.Is(err, ErrRepositoryNotDirectory) {
		t.Errorf("Scan(file) error = %v", err)
	}
}

func TestDiscoveryEngineMetadata(t *testing.T) {
	t.Parallel()

	engine := New()
	if engine.Name() != "discovery" || engine.Version() != "0.1.0" || engine.Description() == "" {
		t.Errorf("unexpected metadata: %s %s %q", engine.Name(), engine.Version(), engine.Description())
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
