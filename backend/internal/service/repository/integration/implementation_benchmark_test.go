package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

func BenchmarkPhysicalArtifactID(b *testing.B) {
	public := repository.ArtifactID("rsaid1_19546f40503fdddd85481edb5cf47f7189874a252c63bddbb6b39e8c9b032886")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := PhysicalArtifactID(public); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCanonicalManifestTenArtifacts(b *testing.B) {
	value, artifacts := benchmarkManifestFixture(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := canonicalManifest(value, artifacts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIntegrationTranslationTenArtifacts(b *testing.B) {
	value, artifacts := benchmarkManifestFixture(b)
	b.ReportAllocs()
	for b.Loop() {
		for _, artifact := range artifacts {
			if _, err := PhysicalArtifactID(artifact.Metadata().ArtifactID()); err != nil {
				b.Fatal(err)
			}
		}
		if _, err := canonicalManifest(value, artifacts); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkManifestFixture(b *testing.B) (repository.Scan, []manifestArtifact) {
	b.Helper()
	now := time.Date(2026, 7, 28, 21, 0, 0, 0, time.UTC)
	value, err := repository.NewScan(repository.ScanParams{RepositoryID: goldenRepoID, ScanID: goldenScanID, Profile: repository.DefaultRepositoryGoProfile().Profile(), SourceRevision: "commit:0123456789abcdef", State: repository.ScanSucceeded, RequestedAt: now, StartedAt: now, FinishedAt: now.Add(time.Second)})
	if err != nil {
		b.Fatal(err)
	}
	artifacts := make([]manifestArtifact, 0, 10)
	for index := 0; index < 10; index++ {
		name := fmt.Sprintf("artifact-%02d", index)
		id, buildErr := repository.NewArtifactID(goldenRepoID, goldenScanID, name, "1.0.0", repository.ArtifactIdentityScheme)
		if buildErr != nil {
			b.Fatal(buildErr)
		}
		payload := []byte(fmt.Sprintf("payload-%02d", index))
		metadata, buildErr := repository.NewArtifact(repository.ArtifactParams{ArtifactID: id, ScanID: goldenScanID, Name: name, Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "1.0.0", MediaType: "application/json", PayloadDigest: repository.DigestBytes(payload), PayloadSize: uint64(len(payload)), ProducerName: "benchmark", ProducerVersion: "1.0.0", CreatedAt: now.Add(time.Second)})
		if buildErr != nil {
			b.Fatal(buildErr)
		}
		artifacts = append(artifacts, testManifestArtifact{metadata: metadata})
	}
	return value, artifacts
}
