package lifecycle

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

func BenchmarkLifecycleGetRepository(b *testing.B) {
	fixture, cleanup, err := openLifecycleFixture(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = cleanup(context.Background()) }()
	query, _ := repository.NewRepositoryQuery(fixture.Scenario.PrimaryScope, fixture.Scenario.Repository.RepositoryID())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := fixture.Service.GetRepository(context.Background(), query); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistrationFingerprint(b *testing.B) {
	contract, _ := repository.New()
	scope, _ := repository.NewScope("scope", "principal")
	request, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: "request", RepositoryID: "repository", DisplayName: "Repository", SourceHandle: "handle"})
	proof, _ := NewSourceProof("local", "sha256/v1", repository.DigestBytes([]byte("proof")), "revision")
	b.ReportAllocs()
	for b.Loop() {
		if registerFingerprint(request, proof).IsZero() {
			b.Fatal("zero fingerprint")
		}
	}
}

func BenchmarkLifecycleRegisterIndependent(b *testing.B) {
	contract, _ := repository.New()
	scope, _ := repository.NewScope("scope", "principal")
	proof, _ := NewSourceProof("local", "sha256/v1", repository.DigestBytes([]byte("proof")), "revision")
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	b.ReportAllocs()
	for index := 0; b.Loop(); index++ {
		store := newMemoryStore()
		service, _ := New(store, &fakeResolver{proof: proof}, ClockFunc(func() time.Time { return now }))
		id := repository.RepositoryID("repository-" + strconv.Itoa(index))
		requestID := repository.RequestID("request-" + strconv.Itoa(index))
		request, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: requestID, RepositoryID: id, DisplayName: "Repository", SourceHandle: "handle"})
		if _, err := service.RegisterRepository(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}
