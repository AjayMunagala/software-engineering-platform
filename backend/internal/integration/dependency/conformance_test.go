package dependency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/AjayMunagala/software-engineering-platform/backend/die"
	p "github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

const scopeID = "11111111-1111-4111-8111-111111111111"
const repoID = "22222222-2222-4222-8222-222222222222"
const scanID = "33333333-3333-4333-8333-333333333333"

type fake struct {
	p.Port
	mu                                  sync.Mutex
	repository                          p.RepositoryRecord
	scan                                p.ScanRecord
	artifact                            p.ArtifactRecord
	payload                             []byte
	begins, stages, publishes, finishes int
	lost, corrupt, skipRead, stageFail  bool
}

func (f *fake) GetRepository(ctx context.Context, q p.RepositoryQuery) (p.RepositoryRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if q.Scope().ScopeID() != scopeID || string(q.RepositoryID()) != repoID {
		return p.RepositoryRecord{}, p.NewError(p.ErrorNotFound, "get", false, nil)
	}
	return f.repository, nil
}
func (f *fake) GetScan(ctx context.Context, q p.ScanQuery) (p.ScanRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.scan.State() == "" || q.Scope().ScopeID() != scopeID || string(q.ScanID()) != scanID {
		return p.ScanRecord{}, p.NewError(p.ErrorNotFound, "get", false, nil)
	}
	return f.scan, nil
}
func (f *fake) BeginScan(ctx context.Context, r p.BeginScanRequest) (p.ScanRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.begins++
	if f.scan.State() != "" {
		return f.scan, p.NewError(p.ErrorIdempotencyConflict, "begin", false, nil)
	}
	now := time.Now().UTC()
	var err error
	f.scan, err = p.NewScanRecord(scopeID, r.RepositoryID(), r.ScanID(), r.AnalysisProfileDigest(), r.SourceRevision(), p.ScanRunning, "", "", now, now, time.Time{})
	return f.scan, err
}
func (f *fake) StagePayload(ctx context.Context, r p.StagePayloadRequest, reader io.Reader) (p.PayloadReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stages++
	if f.stageFail {
		return p.PayloadReceipt{}, errors.New("private-path-secret")
	}
	if !f.skipRead {
		b, err := io.ReadAll(reader)
		if err != nil {
			return p.PayloadReceipt{}, err
		}
		if p.DigestBytes(b) != r.Digest() || uint64(len(b)) != uint64(r.ExpectedSize()) {
			return p.PayloadReceipt{}, p.NewError(p.ErrorIntegrityFailure, "stage", false, nil)
		}
		f.payload = b
	}
	return p.NewPayloadReceipt(r.Digest(), r.ExpectedSize(), p.DispositionCreated)
}
func (f *fake) PublishScan(ctx context.Context, r p.PublishScanRequest) (p.PublicationReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.publishes++
	if len(r.Artifacts()) != 1 || len(r.Dependencies()) != 0 || len(r.Projections()) != 0 || r.MakeCurrent() {
		return p.PublicationReceipt{}, errors.New("unexpected publication shape")
	}
	a := r.Artifacts()[0]
	if a.PayloadDigest() != p.DigestBytes(f.payload) {
		return p.PublicationReceipt{}, errors.New("unstaged")
	}
	now := time.Now().UTC()
	var err error
	f.scan, err = p.NewScanRecord(scopeID, r.RepositoryID(), r.ScanID(), f.scan.AnalysisProfileDigest(), f.scan.SourceRevision(), p.ScanSucceeded, "", "", f.scan.RequestedAt(), f.scan.StartedAt(), now)
	if err != nil {
		return p.PublicationReceipt{}, err
	}
	f.artifact, err = p.NewArtifactRecord(scopeID, r.RepositoryID(), r.ScanID(), a.ArtifactID(), a.Artifact(), a.StableIDScheme(), a.Codec(), a.Producer(), a.PayloadDigest(), a.PayloadSize(), now)
	if err != nil {
		return p.PublicationReceipt{}, err
	}
	if f.lost {
		return p.PublicationReceipt{}, p.NewError(p.ErrorUnavailable, "publish", true, nil)
	}
	return p.NewPublicationReceipt(r.ScanID(), r.ManifestScheme(), r.ManifestDigest(), 1, p.DispositionCreated)
}
func (f *fake) ListArtifacts(ctx context.Context, r p.ArtifactListRequest) (p.ArtifactPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Scope().ScopeID() != scopeID || string(r.ScanID()) != scanID {
		return p.ArtifactPage{}, p.NewError(p.ErrorNotFound, "list", false, nil)
	}
	if f.scan.State() != p.ScanSucceeded {
		return p.NewArtifactPage(nil, ""), nil
	}
	return p.NewArtifactPage([]p.ArtifactRecord{f.artifact}, ""), nil
}
func (f *fake) ExportPayload(ctx context.Context, q p.PayloadQuery, w io.Writer) (p.PayloadReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if q.Scope().ScopeID() != scopeID || q.Digest() != f.artifact.PayloadDigest() {
		return p.PayloadReceipt{}, p.NewError(p.ErrorNotFound, "export", false, nil)
	}
	b := append([]byte(nil), f.payload...)
	if f.corrupt {
		b[0] ^= 1
	}
	if _, err := w.Write(b); err != nil {
		return p.PayloadReceipt{}, err
	}
	return p.NewPayloadReceipt(f.artifact.PayloadDigest(), f.artifact.PayloadSize(), p.DispositionAlreadyPresent)
}
func (f *fake) finish(r p.FinishScanRequest, state p.ScanState) (p.ScanRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finishes++
	if f.scan.State() == p.ScanSucceeded {
		return f.scan, p.NewError(p.ErrorLifecycleConflict, "finish", false, nil)
	}
	var err error
	f.scan, err = p.NewScanRecord(scopeID, r.RepositoryID(), r.ScanID(), f.scan.AnalysisProfileDigest(), f.scan.SourceRevision(), state, r.ReasonCode(), r.SafeMessage(), f.scan.RequestedAt(), f.scan.StartedAt(), time.Now().UTC())
	return f.scan, err
}
func (f *fake) FailScan(ctx context.Context, r p.FinishScanRequest) (p.ScanRecord, error) {
	return f.finish(r, p.ScanFailed)
}
func (f *fake) CancelScan(ctx context.Context, r p.FinishScanRequest) (p.ScanRecord, error) {
	return f.finish(r, p.ScanCancelled)
}

type admission struct {
	mu                 sync.Mutex
	acquired, released int
	ctx                context.Context
	err                error
}
type lease struct{ a *admission }

func (a *admission) Acquire(ctx context.Context) (Lease, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.acquired++
	if a.err != nil {
		return nil, a.err
	}
	return &lease{a}, nil
}
func (l *lease) Context() context.Context { return l.a.ctx }
func (l *lease) Done()                    { l.a.mu.Lock(); defer l.a.mu.Unlock(); l.a.released++ }
func fixture(t testing.TB) (*Service, *fake, *admission, PublishRequest, die.DependencyInventory) {
	t.Helper()
	scope, _ := p.NewScope(scopeID, "tester")
	q, _ := NewQuery(scope, repoID, scanID)
	request, _ := NewPublishRequest(q, "44444444-4444-4444-8444-444444444444", "fixture-π")
	source, _ := p.NewSourceIdentity("local", "sha256/v1", p.DigestBytes([]byte("source")))
	record, err := p.NewRepositoryRecord(scopeID, p.RepositoryID(repoID), "fixture", source, p.RepositoryActive, "", time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	f := &fake{repository: record}
	a := &admission{ctx: context.Background()}
	contract, _ := p.New()
	spools, _ := NewSpoolFactory(t.TempDir(), 32, 1<<20)
	config, _ := NewConfig([]Route{{scopeID, repoID}}, 0)
	service, err := New(Dependencies{f, f, f, f, f, a, contract, spools}, config)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := die.NewConfig(die.ConfigParams{})
	core, _ := die.New(c)
	v, err := core.Normalize(context.Background(), die.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	return service, f, a, request, v
}
func TestFakeConformance(t *testing.T) {
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{false: "publish", true: "lost-reply"}[lost], func(t *testing.T) {
			s, f, a, r, v := fixture(t)
			f.lost = lost
			got, err := s.Publish(context.Background(), r, v)
			if err != nil {
				t.Fatal(err)
			}
			if got.Publication().Size() != 614 {
				t.Fatal("size")
			}
			var out bytes.Buffer
			receipt, err := s.Export(context.Background(), r.query, &out)
			if err != nil || receipt.Size() != 614 || sha256.Sum256(out.Bytes()) != receipt.Digest() {
				t.Fatal(err)
			}
			retry, err := s.Publish(context.Background(), r, v)
			if err != nil || retry.Disposition() != "reconciled" {
				t.Fatal(err)
			}
			if f.publishes != 1 || f.finishes != 0 || a.acquired != a.released {
				t.Fatal("ownership/publication")
			}
			items, _ := os.ReadDir(s.d.Spools.directory)
			if len(items) != 0 {
				t.Fatal("spool leak")
			}
			f.corrupt = true
			out.Reset()
			if _, err = s.Export(context.Background(), r.query, &out); err != Integrity || out.Len() != 0 {
				t.Fatal("unverified bytes exposed", err)
			}
			changed := got.Publication().Expected()
			changed.digest[0] ^= 1
			if _, err = s.Reconcile(context.Background(), changed); err != Integrity {
				t.Fatal(err)
			}
		})
	}
}
func TestScopeNilAndFailureConformance(t *testing.T) {
	s, f, a, r, v := fixture(t)
	if _, err := s.Publish(nil, r, v); err != Invalid {
		t.Fatal(err)
	}
	scope, _ := p.NewScope("55555555-5555-4555-8555-555555555555", "tester")
	q, _ := NewQuery(scope, repoID, scanID)
	if _, err := s.Get(context.Background(), q); err != Missing {
		t.Fatal(err)
	}
	if _, err := s.Export(context.Background(), q, io.Discard); err != Missing {
		t.Fatal(err)
	}
	if _, err := s.Reconcile(context.Background(), ExpectedPublication{query: q}); err != Missing {
		t.Fatal(err)
	}
	if f.begins != 0 || a.acquired != 0 {
		t.Fatal("scope bypass")
	}
	f.skipRead = true
	if _, err := s.Publish(context.Background(), r, v); err != Integrity {
		t.Fatal(err)
	}
	if f.finishes != 1 || f.publishes != 0 || a.acquired != a.released {
		t.Fatal("failure cleanup")
	}
}
func TestIdentityLiterals(t *testing.T) {
	b, err := os.ReadFile("../../../../experiments/dependency-platform-vectors/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Profile string `json:"profile_sha256"`
		Vectors []struct {
			Scope    string `json:"scope_id"`
			Repo     string `json:"repository_id"`
			Scan     string `json:"scan_id"`
			Parent   string `json:"parent_request_id"`
			Revision string `json:"source_revision"`
			Size     uint64 `json:"payload_size"`
			Digest   string `json:"payload_sha256"`
			UUID     string `json:"artifact_uuid"`
			Manifest string `json:"manifest_sha256"`
			Children map[string]struct {
				ID string `json:"request_id"`
			} `json:"child_operations"`
		}
	}
	if err = json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	profile := profileDigest()
	if hex.EncodeToString(profile[:]) != data.Profile {
		t.Fatal("profile")
	}
	for _, v := range data.Vectors {
		scope, _ := p.NewScope(v.Scope, "tester")
		q, _ := NewQuery(scope, v.Repo, v.Scan)
		r, _ := NewPublishRequest(q, v.Parent, v.Revision)
		digest, _ := hex.DecodeString(v.Digest)
		var d [32]byte
		copy(d[:], digest)
		e, _ := NewExpected(q, v.Revision, v.Size, d)
		if artifactID(q) != v.UUID {
			t.Fatal("UUID")
		}
		m := manifest(e)
		if hex.EncodeToString(m[:]) != v.Manifest {
			t.Fatal("manifest")
		}
		for op, c := range v.Children {
			if child(r, op, e) != c.ID {
				t.Fatal("child", op)
			}
		}
	}
}
