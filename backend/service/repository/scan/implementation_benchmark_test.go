package scan

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

func BenchmarkScanGet(b *testing.B) {
	contract, _ := repository.New()
	scope, _ := repository.NewScope(scanTestScopeID, "benchmark-principal")
	profile := contract.Profiles().Definitions()[0].Profile()
	clock := &stepClock{current: time.Date(2026, 7, 27, 18, 0, 0, 0, time.UTC)}
	store := newMemoryStore()
	store.addRepository(scope, scanTestRepositoryID, repository.RepositoryActive)
	preparer := benchmarkPreparer(profile)
	service, _ := New(store, newFakeAdmission(), preparer, clock)
	request, _ := contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: "request", RepositoryID: scanTestRepositoryID, ScanID: scanTestScanID, SourceHandle: "source", Profile: profile})
	_, _ = service.ExecuteScan(context.Background(), request)
	query, _ := repository.NewScanQuery(scope, scanTestRepositoryID, scanTestScanID)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = service.GetScan(context.Background(), query)
	}
}

func BenchmarkScanExecuteIndependent(b *testing.B) {
	contract, _ := repository.New()
	scope, _ := repository.NewScope(scanTestScopeID, "benchmark-principal")
	profile := contract.Profiles().Definitions()[0].Profile()
	clock := &stepClock{current: time.Date(2026, 7, 27, 18, 0, 0, 0, time.UTC)}
	store := newMemoryStore()
	store.addRepository(scope, scanTestRepositoryID, repository.RepositoryActive)
	service, _ := New(store, newFakeAdmission(), benchmarkPreparer(profile), clock)
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		request, _ := contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: repository.RequestID(fmt.Sprintf("request-%d", index)), RepositoryID: scanTestRepositoryID, ScanID: repository.ScanID(fmt.Sprintf("22222222-2222-4222-8222-%012d", index)), SourceHandle: "source", Profile: profile})
		_, _ = service.ExecuteScan(context.Background(), request)
	}
}

func BenchmarkExecuteFingerprint(b *testing.B) {
	contract, _ := repository.New()
	scope, _ := repository.NewScope(scanTestScopeID, "benchmark-principal")
	profile := contract.Profiles().Definitions()[0].Profile()
	request, _ := contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: "request", RepositoryID: scanTestRepositoryID, ScanID: scanTestScanID, SourceHandle: "source", Profile: profile})
	source := repository.DigestBytes([]byte("source"))
	b.ReportAllocs()
	for range b.N {
		_ = executeFingerprint(request, source, "revision-1")
	}
}

func benchmarkPreparer(profile repository.AnalysisProfile) *fakePreparer {
	payload := []byte("{}\n")
	candidate, _ := NewArtifactCandidate(ArtifactCandidateParams{Name: "benchmark-artifact", Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "1.0.0", MediaType: "application/json", PayloadDigest: sha256.Sum256(payload), PayloadSize: uint64(len(payload)), ProducerName: "fake-analysis", ProducerVersion: "1.0.0", Payload: byteSource(payload)})
	result, _ := NewAnalysisResult(profile, []ArtifactCandidate{candidate})
	return &fakePreparer{profile: profile, fingerprint: repository.DigestBytes([]byte("source-proof")), revision: "revision-1", result: result}
}
