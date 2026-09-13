package dependency

import (
	"context"
	"crypto/sha256"
	"github.com/AjayMunagala/software-engineering-platform/backend/die"
	"github.com/AjayMunagala/software-engineering-platform/backend/die/codec"
	p "github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"io"
)

func NewExpected(q Query, revision string, size uint64, digest [32]byte) (ExpectedPublication, error) {
	if _, err := NewPublishRequest(q, "expected", revision); err != nil {
		return ExpectedPublication{}, err
	}
	if size == 0 || size > codec.MaximumBytes || digest == [32]byte{} {
		return ExpectedPublication{}, Invalid
	}
	return ExpectedPublication{q, revision, size, digest}, nil
}
func (s *Service) scan(ctx context.Context, q Query) (p.ScanRecord, error) {
	request, err := s.d.Contract.NewScanQuery(q.scope, p.RepositoryID(q.repository), p.ScanID(q.scan))
	if err != nil {
		return p.ScanRecord{}, Invalid
	}
	r, err := s.d.Scans.GetScan(ctx, request)
	if err != nil {
		return r, safe(err)
	}
	if r.ScopeID() != q.scope.ScopeID() || string(r.RepositoryID()) != q.repository || string(r.ScanID()) != q.scan {
		return r, Integrity
	}
	return r, nil
}
func (s *Service) Get(ctx context.Context, q Query) (Publication, error) {
	if err := s.check(ctx, q); err != nil {
		return Publication{}, err
	}
	record, err := s.scan(ctx, q)
	if err != nil {
		return Publication{}, err
	}
	if record.State() != p.ScanSucceeded {
		return Publication{}, Conflict
	}
	if record.AnalysisProfileDigest() != p.Digest(profileDigest()) {
		return Publication{}, Integrity
	}
	req, err := s.d.Contract.NewArtifactListRequest(q.scope, p.RepositoryID(q.repository), p.ScanID(q.scan), 2, "")
	if err != nil {
		return Publication{}, Invalid
	}
	page, err := s.d.Reader.ListArtifacts(ctx, req)
	if err != nil {
		return Publication{}, safe(err)
	}
	items := page.Records()
	if len(items) != 1 || page.NextCursor() != "" {
		return Publication{}, Integrity
	}
	a := items[0]
	if a.ScopeID() != q.scope.ScopeID() || string(a.RepositoryID()) != q.repository || string(a.ScanID()) != q.scan || string(a.ArtifactID()) != artifactID(q) || a.Artifact().Name() != die.ArtifactName || a.Artifact().Version() != Version || a.StableIDScheme() != Scheme || a.Codec().Name() != codec.Name || a.Codec().Version() != Version || a.Codec().MediaType() != "application/json" || a.Producer().Name() != "dependency-platform-integration" || a.Producer().Version() != Version {
		return Publication{}, Integrity
	}
	expected, err := NewExpected(q, record.SourceRevision(), uint64(a.PayloadSize()), [32]byte(a.PayloadDigest()))
	if err != nil || expected.size > s.d.Spools.maximum {
		return Publication{}, Integrity
	}
	return Publication{expected, record, a, manifest(expected)}, nil
}
func (s *Service) Reconcile(ctx context.Context, e ExpectedPublication) (Reconciliation, error) {
	if err := s.check(ctx, e.query); err != nil {
		return Reconciliation{}, err
	}
	if e.size == 0 || e.digest == [32]byte{} {
		return Reconciliation{}, Invalid
	}
	r, err := s.scan(ctx, e.query)
	if err == Missing {
		return Reconciliation{state: "missing_unknown"}, nil
	}
	if err != nil {
		return Reconciliation{}, err
	}
	if r.AnalysisProfileDigest() != p.Digest(profileDigest()) || r.SourceRevision() != e.revision {
		return Reconciliation{}, Integrity
	}
	switch r.State() {
	case p.ScanRequested, p.ScanRunning:
		return Reconciliation{state: "running_unknown"}, nil
	case p.ScanFailed:
		return Reconciliation{state: "failed"}, nil
	case p.ScanCancelled:
		return Reconciliation{state: "canceled"}, nil
	case p.ScanSucceeded:
		out, err := s.Get(ctx, e.query)
		if err != nil {
			return Reconciliation{}, err
		}
		if out.expected != e {
			return Reconciliation{}, Integrity
		}
		return Reconciliation{"published_exact", out}, nil
	}
	return Reconciliation{}, Integrity
}
func (s *Service) reconcileDetached(e ExpectedPublication) (Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.finalization)
	defer cancel()
	r, err := s.Reconcile(ctx, e)
	if err != nil {
		if err == Unavailable || err == Timeout || err == Canceled {
			return Receipt{}, &Uncertain{e}
		}
		return Receipt{}, err
	}
	if r.state == "published_exact" {
		return Receipt{r.publication, "reconciled"}, nil
	}
	if r.state == "failed" || r.state == "canceled" {
		return Receipt{}, Conflict
	}
	return Receipt{}, &Uncertain{e}
}
func (s *Service) Publish(ctx context.Context, r PublishRequest, inventory die.DependencyInventory) (receipt Receipt, err error) {
	if err = s.check(ctx, r.query); err != nil {
		return
	}
	if r.request == "" {
		return Receipt{}, Invalid
	}
	work, done, err := s.admit(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer done()
	sp, err := s.d.Spools.create(work)
	if err != nil {
		return Receipt{}, err
	}
	defer func() {
		if closeErr := sp.close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	encoder, _ := codec.New(s.d.Spools.maximum)
	encoded, encodeErr := encoder.Encode(work, inventory, sp)
	if encodeErr != nil {
		switch encodeErr {
		case codec.Limit:
			return Receipt{}, Limit
		case codec.Invalid:
			return Receipt{}, Invalid
		case codec.Unsupported:
			return Receipt{}, Unsupported
		}
		return Receipt{}, safe(encodeErr)
	}
	if err = sp.seal(); err != nil {
		return
	}
	if sp.size != encoded.Size() || sp.digest != encoded.Digest() {
		return Receipt{}, Integrity
	}
	e := ExpectedPublication{r.query, r.revision, sp.size, sp.digest}
	q := r.query
	repositoryQuery, _ := s.d.Contract.NewRepositoryQuery(q.scope, p.RepositoryID(q.repository))
	repositoryRecord, getErr := s.d.Repositories.GetRepository(work, repositoryQuery)
	if getErr != nil {
		return Receipt{}, safe(getErr)
	}
	if repositoryRecord.ScopeID() != q.scope.ScopeID() || string(repositoryRecord.RepositoryID()) != q.repository {
		return Receipt{}, Integrity
	}
	if repositoryRecord.State() != p.RepositoryActive {
		return Receipt{}, Conflict
	}
	if _, getErr = s.scan(work, q); getErr == nil {
		return s.reconcileDetached(e)
	} else if getErr != Missing {
		return Receipt{}, getErr
	}
	actor, actorErr := p.NewAuditActor("principal", q.scope.PrincipalID())
	if actorErr != nil {
		return Receipt{}, Invalid
	}
	begin, buildErr := s.d.Contract.NewBeginScanRequest(p.BeginScanParams{Scope: q.scope, RequestID: p.RequestID(child(r, "begin", e)), RepositoryID: p.RepositoryID(q.repository), ScanID: p.ScanID(q.scan), AnalysisProfileDigest: p.Digest(profileDigest()), SourceRevision: r.revision, Actor: actor})
	if buildErr != nil {
		return Receipt{}, Invalid
	}
	started, beginErr := s.d.Scans.BeginScan(work, begin)
	if beginErr != nil {
		return s.reconcileDetached(e)
	}
	if started.ScopeID() != q.scope.ScopeID() || string(started.RepositoryID()) != q.repository || string(started.ScanID()) != q.scan || started.AnalysisProfileDigest() != p.Digest(profileDigest()) || started.SourceRevision() != r.revision {
		return Receipt{}, Integrity
	}
	if started.State() != p.ScanRunning && started.State() != p.ScanRequested {
		return s.reconcileDetached(e)
	}
	attemptedPublish := false
	defer func() {
		if err == nil || attemptedPublish {
			return
		}
		finishCtx, cancel := context.WithTimeout(context.Background(), s.config.finalization)
		defer cancel()
		operation := "fail"
		if work.Err() != nil {
			operation = "cancel"
		}
		finish, finishErr := s.d.Contract.NewFinishScanRequest(p.FinishScanParams{Scope: q.scope, RequestID: p.RequestID(child(r, operation, e)), RepositoryID: p.RepositoryID(q.repository), ScanID: p.ScanID(q.scan), ReasonCode: "dependency_publication_failed", SafeMessage: "dependency publication did not complete", Actor: actor})
		if finishErr == nil {
			if operation == "cancel" {
				_, _ = s.d.Scans.CancelScan(finishCtx, finish)
			} else {
				_, _ = s.d.Scans.FailScan(finishCtx, finish)
			}
		}
	}()
	reader, openErr := sp.open(work)
	if openErr != nil {
		return Receipt{}, openErr
	}
	stage, buildErr := s.d.Contract.NewStagePayloadRequest(p.StagePayloadParams{Scope: q.scope, RequestID: p.RequestID(child(r, "stage", e)), RepositoryID: p.RepositoryID(q.repository), ScanID: p.ScanID(q.scan), Digest: p.Digest(e.digest), ExpectedSize: p.ByteCount(e.size)})
	if buildErr != nil {
		reader.Close()
		return Receipt{}, Invalid
	}
	counter := &countReader{reader: reader, ctx: work}
	staged, stageErr := s.d.Stager.StagePayload(work, stage, counter)
	closeErr := reader.Close()
	if stageErr != nil {
		return Receipt{}, safe(stageErr)
	}
	if closeErr != nil {
		return Receipt{}, closeErr
	}
	if !counter.eof || counter.n != e.size || staged.Digest() != p.Digest(e.digest) || uint64(staged.Size()) != e.size {
		return Receipt{}, Integrity
	}
	artifact, _ := p.NewVersionedName(die.ArtifactName, Version)
	producer, _ := p.NewVersionedName("dependency-platform-integration", Version)
	encoding, _ := p.NewCodec(codec.Name, Version, "application/json")
	submission, buildErr := s.d.Contract.NewArtifactSubmission(p.ArtifactSubmissionParams{ArtifactID: p.ArtifactID(artifactID(q)), Artifact: artifact, StableIDScheme: Scheme, Codec: encoding, PayloadDigest: p.Digest(e.digest), PayloadSize: p.ByteCount(e.size), Producer: producer})
	if buildErr != nil {
		return Receipt{}, Invalid
	}
	m := manifest(e)
	publish, buildErr := s.d.Contract.NewPublishScanRequest(p.PublishScanParams{Scope: q.scope, RequestID: p.RequestID(child(r, "publish", e)), RepositoryID: p.RepositoryID(q.repository), ScanID: p.ScanID(q.scan), ManifestScheme: ManifestScheme, ManifestDigest: p.Digest(m), Artifacts: []p.ArtifactSubmission{submission}, Actor: actor})
	if buildErr != nil {
		return Receipt{}, Invalid
	}
	attemptedPublish = true
	published, publishErr := s.d.Publications.PublishScan(work, publish)
	if publishErr != nil {
		return s.reconcileDetached(e)
	}
	if published.ScanID() != p.ScanID(q.scan) || published.ManifestScheme() != ManifestScheme || published.ManifestDigest() != p.Digest(m) || published.ArtifactCount() != 1 {
		return Receipt{}, Integrity
	}
	out, reconcileErr := s.reconcileDetached(e)
	if reconcileErr != nil {
		return Receipt{}, reconcileErr
	}
	out.disposition = "published"
	return out, nil
}

type countReader struct {
	reader io.Reader
	ctx    context.Context
	n      uint64
	eof    bool
}

func (r *countReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(b)
	r.n += uint64(n)
	if err == io.EOF {
		r.eof = true
	}
	return n, err
}
func (s *Service) Export(ctx context.Context, q Query, w io.Writer) (receipt ExportReceipt, err error) {
	if err = s.check(ctx, q); err != nil {
		return
	}
	if nilValue(w) {
		return ExportReceipt{}, Invalid
	}
	work, done, err := s.admit(ctx)
	if err != nil {
		return ExportReceipt{}, err
	}
	defer done()
	out, err := s.Get(work, q)
	if err != nil {
		return ExportReceipt{}, err
	}
	sp, err := s.d.Spools.create(work)
	if err != nil {
		return ExportReceipt{}, err
	}
	defer func() {
		if x := sp.close(); err == nil {
			err = x
		}
	}()
	query, err := s.d.Contract.NewPayloadQuery(q.scope, p.RepositoryID(q.repository), p.ScanID(q.scan), p.ArtifactID(artifactID(q)), p.Digest(out.expected.digest))
	if err != nil {
		return ExportReceipt{}, Invalid
	}
	got, err := s.d.Reader.ExportPayload(work, query, sp)
	if err != nil {
		return ExportReceipt{}, safe(err)
	}
	if err = sp.seal(); err != nil {
		return
	}
	if sp.digest != out.expected.digest || sp.size != out.expected.size || got.Digest() != p.Digest(sp.digest) || uint64(got.Size()) != sp.size {
		return ExportReceipt{}, Integrity
	}
	r, err := sp.open(work)
	if err != nil {
		return ExportReceipt{}, err
	}
	h := sha256.New()
	n, copyErr := copyContext(work, io.MultiWriter(w, h), r)
	closeErr := r.Close()
	if copyErr != nil {
		return ExportReceipt{}, safe(copyErr)
	}
	if closeErr != nil {
		return ExportReceipt{}, closeErr
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	if n != sp.size || digest != sp.digest {
		return ExportReceipt{}, Integrity
	}
	return ExportReceipt{digest, n}, nil
}
func copyContext(ctx context.Context, w io.Writer, r io.Reader) (uint64, error) {
	buffer := make([]byte, 1<<20)
	var total uint64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := r.Read(buffer)
		if n > 0 {
			if x := ctx.Err(); x != nil {
				return total, x
			}
			written, x := w.Write(buffer[:n])
			if written < 0 || written > n {
				return total, Unavailable
			}
			total += uint64(written)
			if x != nil {
				return total, x
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrNoProgress
		}
	}
}
