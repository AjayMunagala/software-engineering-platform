package lie_test

import (
	"context"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
)

func BenchmarkRunnerEmptyRegistry(b *testing.B) {
	store := repositoryArtifacts(b, map[string]string{"README.md": "notes"})
	runner, err := lie.New()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := runner.Run(context.Background(), store); err != nil {
			b.Fatal(err)
		}
	}
}
