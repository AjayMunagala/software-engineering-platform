package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

func BenchmarkFrameworkEngineExecute(b *testing.B) {
	repository := b.TempDir()
	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"dependencies":{"react":"1","express":"2"}}`), 0o600); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "go.mod"), []byte("module example\nrequire github.com/gin-gonic/gin v1.10.0\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	entries := make([]rie.RepositoryEntry, 0, 100002)
	entries = append(entries, rie.RepositoryEntry{Path: "package.json"}, rie.RepositoryEntry{Path: "go.mod"})
	for i := 0; i < 100000; i++ {
		entries = append(entries, rie.RepositoryEntry{Path: fmt.Sprintf("pkg/file-%06d.go", i)})
	}
	engine := New()
	prepared := readyRun(b, repository, entries)
	languageInventory, exists := prepared.Artifacts.Get("language-inventory")
	if !exists {
		b.Fatal("language inventory was not prepared")
	}
	snapshot, exists := prepared.Artifacts.Get(rie.RepositorySnapshotArtifactName)
	if !exists {
		b.Fatal("repository snapshot was not prepared")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run := rie.NewRunContext(repository, rie.DefaultConfig())
		run.Report.Repository.RootPath = repository
		run.Entries = entries
		if err := run.Artifacts.Put(languageInventory); err != nil {
			b.Fatal(err)
		}
		if err := run.Artifacts.Put(snapshot); err != nil {
			b.Fatal(err)
		}
		if err := engine.Execute(context.Background(), run); err != nil {
			b.Fatal(err)
		}
	}
}
