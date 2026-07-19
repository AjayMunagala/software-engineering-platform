package build

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

func BenchmarkBuildEngineExecute(b *testing.B) {
	repository := b.TempDir()
	mustWrite(b, filepath.Join(repository, "frontend", "package.json"), `{"packageManager":"pnpm@9","engines":{"node":">=20"}}`)
	mustWrite(b, filepath.Join(repository, "frontend", "pnpm-lock.yaml"), "lockfileVersion: '9.0'")
	mustWrite(b, filepath.Join(repository, "backend", "go.mod"), "module example\ngo 1.24\n")
	entries := make([]rie.RepositoryEntry, 0, 100003)
	entries = append(entries, rie.RepositoryEntry{Path: "frontend/package.json"}, rie.RepositoryEntry{Path: "frontend/pnpm-lock.yaml"}, rie.RepositoryEntry{Path: "backend/go.mod"})
	for index := 0; index < 100000; index++ {
		entries = append(entries, rie.RepositoryEntry{Path: fmt.Sprintf("src/file-%06d.go", index)})
	}
	snapshot := rie.NewRepositorySnapshot(repository, entries, rie.Statistics{Files: len(entries)}, nil, "0.2.1")
	engine := New()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		run := rie.NewRunContext(repository, rie.DefaultConfig())
		if err := run.Artifacts.Put(snapshot); err != nil {
			b.Fatal(err)
		}
		if err := engine.Execute(context.Background(), run); err != nil {
			b.Fatal(err)
		}
	}
}
