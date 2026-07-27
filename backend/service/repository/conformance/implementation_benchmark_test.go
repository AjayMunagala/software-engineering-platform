package conformance

import (
	"context"
	"io"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

func BenchmarkMemoryGetRepository(b *testing.B) {
	fixture, cleanup, err := NewMemoryFactory().Open(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup(context.Background())
	query, _ := repository.NewRepositoryQuery(fixture.Scenario.PrimaryScope, fixture.Scenario.Repository.RepositoryID())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := fixture.Service.GetRepository(context.Background(), query); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryExportArtifact(b *testing.B) {
	fixture, cleanup, err := NewMemoryFactory().Open(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup(context.Background())
	query, _ := repository.NewArtifactQuery(fixture.Scenario.PrimaryScope, fixture.Scenario.Repository.RepositoryID(), fixture.Scenario.SucceededScan.ScanID(), fixture.Scenario.Artifact.ArtifactID())
	request, _ := repository.NewExportArtifactRequest(query)
	b.SetBytes(int64(len(fixture.Scenario.Payload)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := fixture.Service.ExportArtifact(context.Background(), request, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}
