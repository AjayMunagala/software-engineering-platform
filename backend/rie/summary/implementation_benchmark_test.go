package summary

import (
	"context"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

func BenchmarkSummaryEngineExecute(b *testing.B) {
	repository := b.TempDir()
	prepared := rie.NewRunContext(repository, rie.DefaultConfig())
	if err := fullPipeline(b, false).Run(context.Background(), prepared); err != nil {
		b.Fatal(err)
	}
	metadataArtifact, exists := prepared.Artifacts.Get("repository-metadata")
	if !exists {
		b.Fatal("repository metadata was not prepared")
	}
	engine := New()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		run := rie.NewRunContext(repository, rie.DefaultConfig())
		if err := run.Artifacts.Put(metadataArtifact); err != nil {
			b.Fatal(err)
		}
		if err := engine.Execute(context.Background(), run); err != nil {
			b.Fatal(err)
		}
	}
}
