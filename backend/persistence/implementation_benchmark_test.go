package persistence

import (
	"fmt"
	"testing"
)

func BenchmarkPublishRequestConstruction(b *testing.B) {
	contract, err := New()
	if err != nil {
		b.Fatal(err)
	}
	scope, _ := NewScope("benchmark-scope", "benchmark-principal")
	actor, _ := NewAuditActor("benchmark", "runner")
	codec, _ := NewCodec("json", "1.0.0", "application/json")
	producer, _ := NewVersionedName("benchmark-producer", "1.0.0")
	artifacts := make([]ArtifactSubmission, 100)
	for index := range artifacts {
		name, _ := NewVersionedName(fmt.Sprintf("artifact-%03d", index), "1.0.0")
		artifacts[index], err = contract.NewArtifactSubmission(ArtifactSubmissionParams{
			ArtifactID: ArtifactID(fmt.Sprintf("artifact-id-%03d", index)), Artifact: name,
			Codec: codec, PayloadDigest: DigestBytes([]byte(fmt.Sprintf("payload-%03d", index))),
			PayloadSize: 11, Producer: producer,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
	params := PublishScanParams{
		Scope: scope, RequestID: "request-benchmark", RepositoryID: "repository-benchmark",
		ScanID: "scan-benchmark", ManifestScheme: "artifact-manifest-sha256/v1",
		ManifestDigest: DigestBytes([]byte("manifest")), Artifacts: artifacts,
		MakeCurrent: true, Actor: actor,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, err := contract.NewPublishScanRequest(params); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProjectionDefensiveCopy(b *testing.B) {
	contract, _ := New()
	document := make([]byte, 1<<20)
	document[0], document[len(document)-1] = '[', ']'
	for index := 1; index < len(document)-1; index++ {
		document[index] = ' '
	}
	digest := DigestBytes(document)
	projector, _ := NewVersionedName("benchmark-projector", "1.0.0")
	params := ProjectionSubmissionParams{
		ProjectionID: "projection-benchmark", ArtifactID: "artifact-benchmark",
		SourceDigest: DigestBytes([]byte("source")), Projector: projector,
		SchemaVersion: "1.0.0", DigestScheme: "sha256-json-v1",
		ProjectionDigest: digest, CanonicalJSON: document,
	}
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, err := contract.NewProjectionSubmission(params); err != nil {
			b.Fatal(err)
		}
	}
}
