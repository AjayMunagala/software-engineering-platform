package dependency

import (
	"context"
	"fmt"
	app "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/app"
	rp "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
	p "github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"io"
	"strings"
	"testing"
	"time"
)

// Only a fake implementing public runtime capabilities. No runtime is started.
type runtimeFake struct {
	f *fake
	a *admission
}

func (r runtimeFake) Admit(ctx context.Context) (app.Work, error) {
	v, err := r.a.Acquire(ctx)
	return v, err
}
func (r runtimeFake) Ingest() rp.IngestCapabilities { return r.f }
func (r runtimeFake) Read() rp.ReadCapabilities     { return r.f }
func TestFakeRuntimeCapabilityBridge(t *testing.T) {
	s, f, a, r, v := fixture(t)
	bridge, err := FromRuntime(runtimeFake{f, a}, s.d.Contract, s.d.Spools, s.config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bridge.Publish(context.Background(), r, v); err != nil {
		t.Fatal(err)
	}
	if a.acquired != a.released {
		t.Fatal("lease")
	}
	var missing *fake
	deps := s.d
	deps.Reader = missing
	if _, err = New(deps, s.config); err != Invalid {
		t.Fatal(err)
	}
}
func TestDurableMetadataMismatches(t *testing.T) {
	s, f, _, r, v := fixture(t)
	receipt, err := s.Publish(context.Background(), r, v)
	if err != nil {
		t.Fatal(err)
	}
	original := f.artifact
	for _, value := range []any{Receipt{publication: Publication{expected: ExpectedPublication{revision: "private-revision"}}}, Reconciliation{publication: Publication{expected: ExpectedPublication{revision: "private-revision"}}}} {
		for _, format := range []string{"%v", "%+v", "%#v"} {
			if strings.Contains(fmt.Sprintf(format, value), "private-revision") {
				t.Fatal("formatting leaked revision")
			}
		}
	}
	for _, field := range []string{"scope", "repository", "scan", "id", "name", "version", "scheme", "codec", "codec_version", "media", "producer", "producer_version", "size", "digest"} {
		t.Run(field, func(t *testing.T) {
			a := original
			scope, repo, scan, id := a.ScopeID(), a.RepositoryID(), a.ScanID(), a.ArtifactID()
			artifact, producer, encoding, scheme, size, digest := a.Artifact(), a.Producer(), a.Codec(), a.StableIDScheme(), a.PayloadSize(), a.PayloadDigest()
			switch field {
			case "scope":
				scope = "55555555-5555-4555-8555-555555555555"
			case "repository":
				repo = "55555555-5555-4555-8555-555555555555"
			case "scan":
				scan = "55555555-5555-4555-8555-555555555555"
			case "id":
				id = "55555555-5555-4555-8555-555555555555"
			case "name":
				artifact, _ = p.NewVersionedName("other", "0.1.0")
			case "version":
				artifact, _ = p.NewVersionedName(artifact.Name(), "0.2.0")
			case "scheme":
				scheme = "other/v1"
			case "codec":
				encoding, _ = p.NewCodec("other", encoding.Version(), encoding.MediaType())
			case "codec_version":
				encoding, _ = p.NewCodec(encoding.Name(), "0.2.0", encoding.MediaType())
			case "media":
				encoding, _ = p.NewCodec(encoding.Name(), encoding.Version(), "application/other")
			case "producer":
				producer, _ = p.NewVersionedName("other", producer.Version())
			case "producer_version":
				producer, _ = p.NewVersionedName(producer.Name(), "0.2.0")
			case "size":
				size++
			case "digest":
				digest[0] ^= 1
			}
			f.artifact, err = p.NewArtifactRecord(scope, repo, scan, id, artifact, scheme, encoding, producer, digest, size, a.CreatedAt())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Reconcile(context.Background(), receipt.Publication().Expected()); err != Integrity {
				t.Fatal(field, err)
			}
			f.artifact = original
		})
	}
	if _, err = s.Export(context.Background(), r.query, nil); err != Invalid {
		t.Fatal(err)
	}
	originalScan := f.scan
	for _, state := range []p.ScanState{p.ScanRunning, p.ScanCancelled} {
		finish, reason := time.Time{}, ""
		if state == p.ScanCancelled {
			finish = time.Now()
			reason = "canceled"
		}
		f.scan, err = p.NewScanRecord(scopeID, p.RepositoryID(repoID), p.ScanID(scanID), originalScan.AnalysisProfileDigest(), originalScan.SourceRevision(), state, reason, "", originalScan.RequestedAt(), originalScan.StartedAt(), finish)
		if err != nil {
			t.Fatal(err)
		}
		out, err := s.Reconcile(context.Background(), receipt.Publication().Expected())
		if err != nil {
			t.Fatal(err)
		}
		if state == p.ScanRunning && out.State() != "running_unknown" {
			t.Fatal(out.State())
		}
		if state == p.ScanCancelled && out.State() != "canceled" {
			t.Fatal(out.State())
		}
		if _, err = s.Get(context.Background(), r.query); err != Conflict {
			t.Fatal(err)
		}
	}
	f.scan = originalScan
	e := receipt.Publication().Expected()
	e.revision = "other"
	if _, err = s.Reconcile(context.Background(), e); err != Integrity {
		t.Fatal(err)
	}
	if receipt.Publication().Digest() == [32]byte{} || receipt.Publication().Manifest() == [32]byte{} {
		t.Fatal("getters")
	}
	recon, err := s.Reconcile(context.Background(), receipt.Publication().Expected())
	if err != nil || recon.Publication().Digest() != receipt.Publication().Digest() {
		t.Fatal(err)
	}
}
func TestEarlyPublicationFailures(t *testing.T) {
	var absent *Uncertain
	if absent.Expected().Query().ScanID() != "" {
		t.Fatal("nil outcome")
	}
	uncertain := &Uncertain{}
	if uncertain.Error() != string(Unknown) || uncertain.GoString() != string(Unknown) || uncertain.Unwrap() != Unknown {
		t.Fatal("uncertain error")
	}
	s, f, a, r, v := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Publish(ctx, r, v); err != Canceled {
		t.Fatal(err)
	}
	if _, err := s.Get(nil, r.query); err != Invalid {
		t.Fatal(err)
	}
	bad := PublishRequest{query: r.query}
	if _, err := s.Publish(context.Background(), bad, v); err != Invalid {
		t.Fatal(err)
	}
	s.d.Spools.maximum = 1
	if _, err := s.Publish(context.Background(), r, v); err != Limit {
		t.Fatal(err)
	}
	if f.begins != 0 || a.acquired != a.released {
		t.Fatal("early write")
	}
	s.d.Spools.maximum = 1 << 20
	f.repository, _ = p.NewRepositoryRecord(scopeID, p.RepositoryID(repoID), "fixture", f.repository.Source(), p.RepositoryArchived, "", time.Now(), time.Now())
	if _, err := s.Publish(context.Background(), r, v); err != Conflict {
		t.Fatal(err)
	}
	if _, err := (*Service)(nil).Get(context.Background(), r.query); err != Invalid {
		t.Fatal(err)
	}
	if _, err := s.Export(nil, r.query, io.Discard); err != Invalid {
		t.Fatal(err)
	}
}
