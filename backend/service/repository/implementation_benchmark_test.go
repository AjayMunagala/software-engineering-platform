package repository

import "testing"

func BenchmarkArtifactIdentity(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewArtifactID("repo-001", "scan-01", "go-semantic-inventory", "1.0.0", "go-semantic-id/v1"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegisterRequestValidation(b *testing.B) {
	contract, _ := New()
	scope, _ := NewScope("scope-a", "principal-a")
	params := RegisterRepositoryParams{Scope: scope, RequestID: "request-1", RepositoryID: "repository-1", DisplayName: "Example Repository", SourceHandle: "local-source-token"}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := contract.NewRegisterRepositoryRequest(params); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProfileResolve(b *testing.B) {
	contract, _ := New()
	profile := contract.Profiles().Definitions()[0].Profile()
	registry := contract.Profiles()
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := registry.Resolve(profile.Name(), profile.Version(), profile.Digest()); !ok {
			b.Fatal("profile unavailable")
		}
	}
}
