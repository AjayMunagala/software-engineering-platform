package ignore

import (
	"context"
	"fmt"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

func BenchmarkIgnoreEngineExecute(b *testing.B) {
	entries := make([]rie.RepositoryEntry, 0, 10000)
	for i := 0; i < 10000; i++ {
		entries = append(entries, rie.RepositoryEntry{
			Path: fmt.Sprintf("package-%03d/file-%05d.go", i%100, i),
		})
	}
	config := rie.DefaultConfig()
	config.IgnorePatterns = []string{"package-010/", "package-020/", "*.tmp"}
	engine := New()
	repository := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run := rie.NewRunContext(repository, config)
		run.Report.Repository.RootPath = run.RepositoryPath
		run.Entries = append([]rie.RepositoryEntry(nil), entries...)
		if err := engine.Execute(context.Background(), run); err != nil {
			b.Fatal(err)
		}
	}
}
