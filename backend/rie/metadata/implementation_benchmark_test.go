package metadata

import (
	"context"
	"fmt"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

func BenchmarkMetadataEngineExecute(b *testing.B) {
	repository := b.TempDir()
	prepared := rie.NewRunContext(repository, rie.DefaultConfig())
	if err := completePipeline(b, false).Run(context.Background(), prepared); err != nil {
		b.Fatal(err)
	}
	artifactNames := []string{"discovery-inventory", "language-inventory", "framework-inventory", "build-inventory"}
	artifacts := make([]rie.Artifact, 0, len(artifactNames))
	for _, name := range artifactNames {
		artifact, exists := prepared.Artifacts.Get(name)
		if !exists {
			b.Fatalf("missing %s", name)
		}
		artifacts = append(artifacts, artifact)
	}
	entries := make([]rie.RepositoryEntry, 0, 100000)
	for index := 0; index < 100000; index++ {
		entries = append(entries, rie.RepositoryEntry{Path: fmt.Sprintf("area-%02d/package-%03d/file-%06d.go", index%10, index%100, index)})
	}
	snapshot := rie.NewRepositorySnapshot(repository, entries, rie.Statistics{Files: len(entries), Folders: 110}, nil, "0.2.1")
	engine := New()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		run := rie.NewRunContext(repository, rie.DefaultConfig())
		for _, artifact := range artifacts {
			if err := run.Artifacts.Put(artifact); err != nil {
				b.Fatal(err)
			}
		}
		if err := run.Artifacts.Put(snapshot); err != nil {
			b.Fatal(err)
		}
		if err := engine.Execute(context.Background(), run); err != nil {
			b.Fatal(err)
		}
	}
}
