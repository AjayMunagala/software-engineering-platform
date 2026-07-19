package language

import (
	"context"
	"fmt"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

func BenchmarkLanguageEngineExecute(b *testing.B) {
	extensions := []string{"go", "ts", "tsx", "js", "py", "java", "cs", "sql", "md"}
	entries := make([]rie.RepositoryEntry, 0, 100000)
	for i := 0; i < 100000; i++ {
		entries = append(entries, rie.RepositoryEntry{
			Path: fmt.Sprintf("package-%03d/file-%06d.%s", i%100, i, extensions[i%len(extensions)]),
		})
	}
	engine := New()
	snapshot := rie.NewRepositorySnapshot("benchmark", entries, rie.Statistics{Files: len(entries)}, nil, "0.2.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run := rie.NewRunContext("benchmark", rie.DefaultConfig())
		if err := run.Artifacts.Put(snapshot); err != nil {
			b.Fatal(err)
		}
		if err := engine.Execute(context.Background(), run); err != nil {
			b.Fatal(err)
		}
	}
}
