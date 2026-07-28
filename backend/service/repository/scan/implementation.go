package scan

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

const (
	executeFingerprintDomain = "repository-service-execute/v1\x00"
	cancelFingerprintDomain  = "repository-service-cancel/v1\x00"
)

type Service struct {
	store     Store
	admission AdmissionController
	preparer  AnalysisPreparer
	clock     Clock
	config    Config
	mu        sync.Mutex
	flights   map[string]*flight
}

type flight struct {
	request    repository.ExecuteScanRequest
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	interested int
	result     repository.ScanResult
	err        error
}

func New(store Store, admission AdmissionController, preparer AnalysisPreparer, clock Clock, configs ...Config) (*Service, error) {
	if store == nil || admission == nil || preparer == nil || clock == nil {
		return nil, repository.NewError(repository.ErrorInvalidInput, "new-scan-service", "invalid-dependencies", false, nil)
	}
	if len(configs) > 1 {
		return nil, fmt.Errorf("%w: at most one configuration is accepted", ErrInvalidConfig)
	}
	config := DefaultConfig()
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Service{store: store, admission: admission, preparer: preparer, clock: clock, config: config, flights: make(map[string]*flight)}, nil
}

func (service *Service) ExecuteScan(ctx context.Context, request repository.ExecuteScanRequest) (repository.ScanResult, error) {
	if service == nil {
		return repository.ScanResult{}, repository.NewError(repository.ErrorInvalidInput, "execute-scan", "invalid-service", false, nil)
	}
	if err := contextFailure(ctx, "execute-scan"); err != nil {
		return repository.ScanResult{}, err
	}
	current, joined, err := service.joinFlight(ctx, request)
	if err != nil {
		return repository.ScanResult{}, err
	}
	select {
	case <-current.done:
		if err := ctx.Err(); err != nil {
			return repository.ScanResult{}, repository.NewError(repository.ErrorInternal, "execute-scan", "context-ended", false, err)
		}
		if current.err != nil {
			return repository.ScanResult{}, current.err
		}
		if joined {
			return repository.NewScanResult(current.result.Scan(), current.result.Artifacts(), repository.DispositionJoined)
		}
		return current.result, nil
	case <-ctx.Done():
		service.leaveFlight(current)
		return repository.ScanResult{}, repository.NewError(repository.ErrorInternal, "execute-scan", "context-ended", false, ctx.Err())
	}
}

func (service *Service) joinFlight(ctx context.Context, request repository.ExecuteScanRequest) (*flight, bool, error) {
	key := scanKey(request.Scope(), request.RepositoryID(), request.ScanID())
	service.mu.Lock()
	defer service.mu.Unlock()
	if existing, ok := service.flights[key]; ok {
		if requestsEquivalent(existing.request, request) {
			existing.interested++
			return existing, true, nil
		}
		kind := repository.ErrorScanAlreadyRunning
		if existing.request.RequestID() == request.RequestID() {
			kind = repository.ErrorIdempotencyConflict
		}
		return nil, false, repository.NewError(kind, "execute-scan", "conflicting-flight", false, nil)
	}
	flightCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	created := &flight{request: request, ctx: flightCtx, cancel: cancel, done: make(chan struct{}), interested: 1}
	service.flights[key] = created
	go service.runFlight(key, created)
	return created, false, nil
}

func (service *Service) leaveFlight(current *flight) {
	service.mu.Lock()
	defer service.mu.Unlock()
	select {
	case <-current.done:
		return
	default:
	}
	if current.interested > 0 {
		current.interested--
	}
	if current.interested == 0 {
		current.cancel()
	}
}

func (service *Service) runFlight(key string, current *flight) {
	result, err := service.execute(current.ctx, current.request)
	service.mu.Lock()
	current.result, current.err = result, err
	close(current.done)
	delete(service.flights, key)
	service.mu.Unlock()
	current.cancel()
}

func (service *Service) execute(ctx context.Context, request repository.ExecuteScanRequest) (repository.ScanResult, error) {
	lease, err := service.admission.Acquire(ctx)
	if err != nil {
		return repository.ScanResult{}, mapDependencyError(err, "execute-scan", "admission-failed", repository.ErrorInternal)
	}
	if lease == nil || lease.Context() == nil {
		if lease != nil {
			lease.Done()
		}
		return repository.ScanResult{}, repository.NewError(repository.ErrorInternal, "execute-scan", "invalid-lease", false, nil)
	}
	defer lease.Done()
	executionCtx, executionCancel := context.WithCancel(ctx)
	stopLease := context.AfterFunc(lease.Context(), executionCancel)
	defer func() { stopLease(); executionCancel() }()
	if err := lease.Context().Err(); err != nil {
		executionCancel()
		return repository.ScanResult{}, repository.NewError(repository.ErrorInternal, "execute-scan", "context-ended", false, err)
	}
	if err := contextFailure(executionCtx, "execute-scan"); err != nil {
		return repository.ScanResult{}, err
	}

	session, err := service.preparer.Prepare(executionCtx, NewAnalysisRequest(request))
	if err != nil {
		return repository.ScanResult{}, mapDependencyError(err, "execute-scan", "source-unavailable", repository.ErrorSourceUnavailable)
	}
	if session == nil {
		return repository.ScanResult{}, repository.NewError(repository.ErrorSourceUnavailable, "execute-scan", "invalid-analysis-session", false, nil)
	}
	defer service.closeSession(session)
	if session.SourceFingerprint().IsZero() || (session.SourceRevision() != "" && !safeRevision(session.SourceRevision())) {
		return repository.ScanResult{}, repository.NewError(repository.ErrorSourceUnavailable, "execute-scan", "invalid-source-proof", false, nil)
	}
	if err := contextFailure(executionCtx, "execute-scan"); err != nil {
		return repository.ScanResult{}, err
	}

	now := service.clock.Now().UTC()
	if now.IsZero() {
		return repository.ScanResult{}, repository.NewError(repository.ErrorInternal, "execute-scan", "invalid-clock", false, nil)
	}
	running, err := repository.NewScan(repository.ScanParams{RepositoryID: request.RepositoryID(), ScanID: request.ScanID(), Profile: request.Profile(), SourceRevision: session.SourceRevision(), State: repository.ScanRunning, RequestedAt: now, StartedAt: now})
	if err != nil {
		return repository.ScanResult{}, repository.NewError(repository.ErrorInternal, "execute-scan", "invalid-running-scan", false, nil)
	}
	fingerprint := executeFingerprint(request, session.SourceFingerprint(), session.SourceRevision())
	begin, err := service.store.Begin(executionCtx, newBeginCommand(request.Scope(), request.RequestID(), fingerprint, running))
	if err != nil {
		return repository.ScanResult{}, mapDependencyError(err, "execute-scan", "scan-begin-failed", repository.ErrorPersistenceUnavailable)
	}
	if begin.Scan().RepositoryID() != request.RepositoryID() || begin.Scan().ScanID() != request.ScanID() {
		return repository.ScanResult{}, repository.NewError(repository.ErrorIntegrityFailure, "execute-scan", "begin-result-mismatch", false, nil)
	}
	switch begin.Status() {
	case BeginAlreadyPublished:
		return repository.NewScanResult(begin.Scan(), begin.Artifacts(), repository.DispositionAlreadyPresent)
	case BeginPreviouslyFailed:
		return repository.ScanResult{}, repository.NewError(repository.ErrorAnalysisFailed, "execute-scan", begin.Scan().ReasonCode(), false, nil)
	case BeginPreviouslyCanceled:
		return repository.ScanResult{}, repository.NewError(repository.ErrorCanceled, "execute-scan", begin.Scan().ReasonCode(), false, context.Canceled)
	case BeginOrphaned:
		return repository.ScanResult{}, repository.NewError(repository.ErrorOrphanedScan, "execute-scan", "durable-running-without-leader", false, nil)
	case BeginStarted:
	default:
		return repository.ScanResult{}, repository.NewError(repository.ErrorIntegrityFailure, "execute-scan", "invalid-begin-status", false, nil)
	}

	analysis, err := session.Analyze(executionCtx)
	if err != nil {
		return repository.ScanResult{}, service.failRunning(request.Scope(), running, executionCtx, err, "analysis-failed")
	}
	if analysis.Profile() != request.Profile() || len(analysis.Candidates()) > service.config.MaxArtifacts {
		return repository.ScanResult{}, service.failRunning(request.Scope(), running, executionCtx, repository.NewError(repository.ErrorIntegrityFailure, "execute-scan", "analysis-result-mismatch", false, nil), "analysis-result-mismatch")
	}
	if err := contextFailure(executionCtx, "execute-scan"); err != nil {
		return repository.ScanResult{}, service.failRunning(request.Scope(), running, executionCtx, err, "canceled")
	}
	finished := service.clock.Now().UTC()
	if finished.IsZero() || finished.Before(running.StartedAt()) {
		return repository.ScanResult{}, service.failRunning(request.Scope(), running, executionCtx, repository.NewError(repository.ErrorInternal, "execute-scan", "invalid-clock", false, nil), "invalid-clock")
	}
	succeeded, publication, err := service.buildPublication(request, running, finished, analysis)
	if err != nil {
		return repository.ScanResult{}, service.failRunning(request.Scope(), running, executionCtx, err, "artifact-metadata-failed")
	}
	if err := contextFailure(executionCtx, "execute-scan"); err != nil {
		return repository.ScanResult{}, service.failRunning(request.Scope(), running, executionCtx, err, "canceled")
	}
	result, err := service.store.Publish(executionCtx, newPublishCommand(request.Scope(), succeeded, publication))
	if err == nil {
		if !publicationMatches(result, succeeded, publication) {
			return repository.ScanResult{}, repository.NewError(repository.ErrorIntegrityFailure, "execute-scan", "publication-result-mismatch", false, nil)
		}
		return repository.NewScanResult(result.Scan(), result.Artifacts(), repository.DispositionCreated)
	}
	return service.reconcilePublication(request, succeeded, publication, err)
}

func (service *Service) buildPublication(request repository.ExecuteScanRequest, running repository.Scan, finished time.Time, analysis AnalysisResult) (repository.Scan, []PublicationArtifact, error) {
	candidates := analysis.Candidates()
	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i].Name() + "\x00" + candidates[i].Version() + "\x00" + candidates[i].StableIDScheme()
		right := candidates[j].Name() + "\x00" + candidates[j].Version() + "\x00" + candidates[j].StableIDScheme()
		return left < right
	})
	publication := make([]PublicationArtifact, 0, len(candidates))
	for _, candidate := range candidates {
		id, err := repository.NewArtifactID(request.RepositoryID(), request.ScanID(), candidate.Name(), candidate.Version(), candidate.StableIDScheme())
		if err != nil {
			return repository.Scan{}, nil, err
		}
		metadata, err := repository.NewArtifact(repository.ArtifactParams{ArtifactID: id, ScanID: request.ScanID(), Name: candidate.Name(), Version: candidate.Version(), StableIDScheme: candidate.StableIDScheme(), CodecName: candidate.CodecName(), CodecVersion: candidate.CodecVersion(), MediaType: candidate.MediaType(), PayloadDigest: candidate.PayloadDigest(), PayloadSize: candidate.PayloadSize(), ProducerName: candidate.ProducerName(), ProducerVersion: candidate.ProducerVersion(), CreatedAt: finished})
		if err != nil {
			return repository.Scan{}, nil, err
		}
		publication = append(publication, newPublicationArtifact(metadata, candidate))
	}
	succeeded, err := repository.NewScan(repository.ScanParams{RepositoryID: running.RepositoryID(), ScanID: running.ScanID(), Profile: running.Profile(), SourceRevision: running.SourceRevision(), State: repository.ScanSucceeded, RequestedAt: running.RequestedAt(), StartedAt: running.StartedAt(), FinishedAt: finished})
	return succeeded, publication, err
}

func (service *Service) failRunning(scope repository.Scope, running repository.Scan, executionCtx context.Context, cause error, reason string) error {
	state, kind := repository.ScanFailed, repository.ErrorAnalysisFailed
	if executionCtx.Err() != nil || repository.KindOf(cause) == repository.ErrorCanceled || repository.KindOf(cause) == repository.ErrorTimeout {
		state, kind, reason = repository.ScanCanceled, repository.ErrorCanceled, "canceled"
	}
	finished := service.clock.Now().UTC()
	if finished.IsZero() || finished.Before(running.StartedAt()) {
		finished = running.StartedAt()
	}
	terminal, err := repository.NewScan(repository.ScanParams{RepositoryID: running.RepositoryID(), ScanID: running.ScanID(), Profile: running.Profile(), SourceRevision: running.SourceRevision(), State: state, ReasonCode: reason, RequestedAt: running.RequestedAt(), StartedAt: running.StartedAt(), FinishedAt: finished})
	if err != nil {
		return repository.NewError(repository.ErrorInternal, "execute-scan", "terminal-model-failed", false, nil)
	}
	finalizeCtx, cancel := context.WithTimeout(context.Background(), service.config.FinalizationTimeout)
	defer cancel()
	if _, err = service.store.Finalize(finalizeCtx, newFinalizeCommand(scope, terminal)); err != nil {
		return mapDependencyError(err, "execute-scan", "terminal-finalization-failed", repository.ErrorPersistenceUnavailable)
	}
	if kind == repository.ErrorCanceled {
		return repository.NewError(kind, "execute-scan", reason, false, context.Canceled)
	}
	return repository.NewError(kind, "execute-scan", reason, false, nil)
}

func (service *Service) reconcilePublication(request repository.ExecuteScanRequest, expected repository.Scan, publication []PublicationArtifact, publishErr error) (repository.ScanResult, error) {
	reconcileCtx, cancel := context.WithTimeout(context.Background(), service.config.FinalizationTimeout)
	defer cancel()
	resolved, err := service.store.Reconcile(reconcileCtx, request.Scope(), request.RepositoryID(), request.ScanID())
	if err != nil {
		return repository.ScanResult{}, repository.NewError(repository.ErrorPersistenceUnavailable, "execute-scan", "publication-ambiguous", true, nil)
	}
	switch resolved.Scan().State() {
	case repository.ScanSucceeded:
		candidate, buildErr := repository.NewScanResult(resolved.Scan(), resolved.Artifacts(), repository.DispositionCreated)
		if buildErr != nil || !publicationMatches(candidate, expected, publication) {
			return repository.ScanResult{}, repository.NewError(repository.ErrorIntegrityFailure, "execute-scan", "reconciled-publication-mismatch", false, nil)
		}
		return repository.NewScanResult(resolved.Scan(), resolved.Artifacts(), repository.DispositionCreated)
	case repository.ScanCanceled:
		return repository.ScanResult{}, repository.NewError(repository.ErrorCanceled, "execute-scan", resolved.Scan().ReasonCode(), false, context.Canceled)
	case repository.ScanFailed:
		return repository.ScanResult{}, repository.NewError(repository.ErrorAnalysisFailed, "execute-scan", resolved.Scan().ReasonCode(), false, nil)
	default:
		_ = publishErr
		return repository.ScanResult{}, repository.NewError(repository.ErrorPersistenceUnavailable, "execute-scan", "publication-ambiguous", true, nil)
	}
}

func publicationMatches(result repository.ScanResult, expected repository.Scan, publication []PublicationArtifact) bool {
	actualScan := result.Scan()
	if actualScan.RepositoryID() != expected.RepositoryID() || actualScan.ScanID() != expected.ScanID() || actualScan.Profile() != expected.Profile() || actualScan.SourceRevision() != expected.SourceRevision() || actualScan.State() != repository.ScanSucceeded || actualScan.RequestedAt() != expected.RequestedAt() || actualScan.StartedAt() != expected.StartedAt() || actualScan.FinishedAt() != expected.FinishedAt() {
		return false
	}
	artifacts := result.Artifacts()
	if len(artifacts) != len(publication) {
		return false
	}
	for index, item := range publication {
		left, right := artifacts[index], item.Metadata()
		if left.ArtifactID() != right.ArtifactID() || left.ScanID() != right.ScanID() || left.Name() != right.Name() || left.Version() != right.Version() || left.StableIDScheme() != right.StableIDScheme() || left.CodecName() != right.CodecName() || left.CodecVersion() != right.CodecVersion() || left.MediaType() != right.MediaType() || left.PayloadDigest() != right.PayloadDigest() || left.PayloadSize() != right.PayloadSize() || left.ProducerName() != right.ProducerName() || left.ProducerVersion() != right.ProducerVersion() || left.CreatedAt() != right.CreatedAt() {
			return false
		}
	}
	return true
}

func (service *Service) closeSession(session AnalysisSession) {
	closeCtx, cancel := context.WithTimeout(context.Background(), service.config.CleanupTimeout)
	defer cancel()
	_ = session.Close(closeCtx)
}

func (service *Service) GetScan(ctx context.Context, query repository.ScanQuery) (repository.Scan, error) {
	if service == nil {
		return repository.Scan{}, repository.NewError(repository.ErrorInvalidInput, "get-scan", "invalid-service", false, nil)
	}
	if err := contextFailure(ctx, "get-scan"); err != nil {
		return repository.Scan{}, err
	}
	value, err := service.store.GetScan(ctx, query.Scope(), query.RepositoryID(), query.ScanID())
	if err != nil {
		return repository.Scan{}, mapDependencyError(err, "get-scan", "store-failed", repository.ErrorPersistenceUnavailable)
	}
	if value.RepositoryID() != query.RepositoryID() || value.ScanID() != query.ScanID() {
		return repository.Scan{}, repository.NewError(repository.ErrorIntegrityFailure, "get-scan", "store-result-mismatch", false, nil)
	}
	return value, nil
}

func (service *Service) ListScans(ctx context.Context, request repository.ScanListRequest) (repository.ScanPage, error) {
	if service == nil {
		return repository.ScanPage{}, repository.NewError(repository.ErrorInvalidInput, "list-scans", "invalid-service", false, nil)
	}
	if err := contextFailure(ctx, "list-scans"); err != nil {
		return repository.ScanPage{}, err
	}
	list, err := service.store.ListScans(ctx, request.Scope(), request.RepositoryID(), request.PageSize(), request.Cursor())
	if err != nil {
		return repository.ScanPage{}, mapDependencyError(err, "list-scans", "store-failed", repository.ErrorPersistenceUnavailable)
	}
	return repository.NewScanPage(list.Items(), list.NextCursor())
}

func (service *Service) CancelScan(ctx context.Context, request repository.CancelScanRequest) (repository.Scan, error) {
	if service == nil {
		return repository.Scan{}, repository.NewError(repository.ErrorInvalidInput, "cancel-scan", "invalid-service", false, nil)
	}
	if err := contextFailure(ctx, "cancel-scan"); err != nil {
		return repository.Scan{}, err
	}
	now := service.clock.Now().UTC()
	if now.IsZero() {
		return repository.Scan{}, repository.NewError(repository.ErrorInternal, "cancel-scan", "invalid-clock", false, nil)
	}
	fingerprint := cancelFingerprint(request)
	value, err := service.store.Cancel(ctx, newCancelCommand(request.Scope(), request.RequestID(), fingerprint, request.RepositoryID(), request.ScanID(), now))
	if err != nil {
		return repository.Scan{}, mapDependencyError(err, "cancel-scan", "store-failed", repository.ErrorPersistenceUnavailable)
	}
	if value.RepositoryID() != request.RepositoryID() || value.ScanID() != request.ScanID() || value.State() != repository.ScanCanceled {
		return repository.Scan{}, repository.NewError(repository.ErrorIntegrityFailure, "cancel-scan", "store-result-mismatch", false, nil)
	}
	service.cancelFlight(request.Scope(), request.RepositoryID(), request.ScanID())
	return value, nil
}

func (service *Service) cancelFlight(scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if current, ok := service.flights[scanKey(scope, repositoryID, scanID)]; ok {
		current.cancel()
	}
}

func (service *Service) GetArtifact(ctx context.Context, query repository.ArtifactQuery) (repository.Artifact, error) {
	if service == nil {
		return repository.Artifact{}, repository.NewError(repository.ErrorInvalidInput, "get-artifact", "invalid-service", false, nil)
	}
	if err := contextFailure(ctx, "get-artifact"); err != nil {
		return repository.Artifact{}, err
	}
	value, err := service.store.GetArtifact(ctx, query.Scope(), query.RepositoryID(), query.ScanID(), query.ArtifactID())
	if err != nil {
		return repository.Artifact{}, mapDependencyError(err, "get-artifact", "store-failed", repository.ErrorPersistenceUnavailable)
	}
	if value.ScanID() != query.ScanID() || value.ArtifactID() != query.ArtifactID() {
		return repository.Artifact{}, repository.NewError(repository.ErrorIntegrityFailure, "get-artifact", "store-result-mismatch", false, nil)
	}
	return value, nil
}

func (service *Service) ListArtifacts(ctx context.Context, request repository.ArtifactListRequest) (repository.ArtifactPage, error) {
	if service == nil {
		return repository.ArtifactPage{}, repository.NewError(repository.ErrorInvalidInput, "list-artifacts", "invalid-service", false, nil)
	}
	if err := contextFailure(ctx, "list-artifacts"); err != nil {
		return repository.ArtifactPage{}, err
	}
	list, err := service.store.ListArtifacts(ctx, request.Scope(), request.RepositoryID(), request.ScanID(), request.PageSize(), request.Cursor())
	if err != nil {
		return repository.ArtifactPage{}, mapDependencyError(err, "list-artifacts", "store-failed", repository.ErrorPersistenceUnavailable)
	}
	return repository.NewArtifactPage(list.Items(), list.NextCursor())
}

func (service *Service) ExportArtifact(ctx context.Context, request repository.ExportArtifactRequest, writer io.Writer) (repository.ExportReceipt, error) {
	if service == nil || writer == nil {
		return repository.ExportReceipt{}, repository.NewError(repository.ErrorInvalidInput, "export-artifact", "invalid-request", false, nil)
	}
	if err := contextFailure(ctx, "export-artifact"); err != nil {
		return repository.ExportReceipt{}, err
	}
	query := request.Query()
	receipt, err := service.store.ExportArtifact(ctx, query.Scope(), query.RepositoryID(), query.ScanID(), query.ArtifactID(), writer)
	if err != nil {
		return repository.ExportReceipt{}, mapDependencyError(err, "export-artifact", "store-failed", repository.ErrorPersistenceUnavailable)
	}
	return receipt, nil
}

func requestsEquivalent(left, right repository.ExecuteScanRequest) bool {
	return left.Scope() == right.Scope() && left.RequestID() == right.RequestID() && left.RepositoryID() == right.RepositoryID() && left.ScanID() == right.ScanID() && left.SourceHandle() == right.SourceHandle() && left.Profile() == right.Profile()
}

func executeFingerprint(request repository.ExecuteScanRequest, source repository.Digest, revision string) repository.Digest {
	return fingerprint(executeFingerprintDomain, string(request.RequestID()), string(request.RepositoryID()), string(request.ScanID()), request.Profile().Name(), request.Profile().Version(), request.Profile().Digest().String(), source.String(), revision)
}

func cancelFingerprint(request repository.CancelScanRequest) repository.Digest {
	return fingerprint(cancelFingerprintDomain, string(request.RepositoryID()), string(request.ScanID()))
}

func fingerprint(domain string, fields ...string) repository.Digest {
	value := append([]byte(nil), domain...)
	var length [4]byte
	for _, field := range fields {
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		value = append(value, length[:]...)
		value = append(value, field...)
	}
	return sha256.Sum256(value)
}

func scanKey(scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) string {
	return string(scope.ScopeID()) + "\x00" + string(scope.PrincipalID()) + "\x00" + string(repositoryID) + "\x00" + string(scanID)
}
