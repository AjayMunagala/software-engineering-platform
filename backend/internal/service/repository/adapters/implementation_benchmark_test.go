package adapters

import (
	"context"
	"fmt"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

func BenchmarkSnapshotMaterialization10000Entries(b *testing.B) {
	root := b.TempDir()
	spool := b.TempDir()
	entries := make([]rie.RepositoryEntry, 10000)
	for index := range entries {
		entries[index] = rie.RepositoryEntry{Path: fmt.Sprintf("pkg/file%05d.go", index)}
	}
	artifact := rie.NewRepositorySnapshot(root, entries, rie.Statistics{Files: len(entries)}, nil, "0.2.1")
	adapter := &Adapter{config: Config{SpoolDirectory: spool, BufferBytes: 64 * 1024, MaxArtifactBytes: maximumPayloadBytes}}
	spec := frozenSpecs()[rie.RepositorySnapshotArtifactName]
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		payload, err := adapter.materialize(context.Background(), root, "benchmark-repository", encodedArtifact{spec: spec, value: artifact})
		if err != nil {
			b.Fatal(err)
		}
		if err = payload.close(); err != nil {
			b.Fatal(err)
		}
	}
}
