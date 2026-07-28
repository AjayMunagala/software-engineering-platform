package scan

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/conformance"
)

const (
	scanTestScopeID      = "00000000-0000-4000-8000-000000000011"
	scanTestOtherScopeID = "00000000-0000-4000-8000-000000000012"
	scanTestRepositoryID = "11111111-1111-4111-8111-111111111121"
	scanTestScanID       = "22222222-2222-4222-8222-222222222221"
	scanTestRunningID    = "22222222-2222-4222-8222-222222222222"
)

func TestScanConformance(t *testing.T) {
	conformance.RunScan(t, conformance.ScanFactoryFunc(func(ctx context.Context) (conformance.ScanFixture, conformance.Cleanup, error) {
		fixture := newFixture(t)
		published, err := fixture.service.ExecuteScan(ctx, fixture.request)
		if err != nil {
			return conformance.ScanFixture{}, nil, err
		}
		runningRequest, _ := fixture.contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: fixture.scope, RequestID: "seed-running-request", RepositoryID: fixture.request.RepositoryID(), ScanID: scanTestRunningID, SourceHandle: "source", Profile: fixture.profile})
		fixture.store.seedRunning(fixture.scope, runningRequest, fixture.preparer.fingerprint, fixture.preparer.revision, fixture.clock.Now())
		fixture.store.mu.RLock()
		running := fixture.store.scans[storeScanKey(fixture.scope, runningRequest.RepositoryID(), runningRequest.ScanID())]
		fixture.store.mu.RUnlock()
		otherScope, _ := repository.NewScope(scanTestOtherScopeID, "scan-other-principal")
		artifact := published.Artifacts()[0]
		payload := []byte("{\"alpha\":true}\n")
		var once sync.Once
		cleanup := func(context.Context) error {
			once.Do(func() {
				fixture.admission.cancel()
				fixture.store.mu.Lock()
				defer fixture.store.mu.Unlock()
				clear(fixture.store.scans)
				clear(fixture.store.artifacts)
				clear(fixture.store.requests)
			})
			return nil
		}
		return conformance.ScanFixture{Service: fixture.service, Contract: fixture.contract, Scenario: conformance.ScanScenario{PrimaryScope: fixture.scope, OtherScope: otherScope, RepositoryID: fixture.request.RepositoryID(), SucceededScan: published.Scan(), RunningScan: running, Artifact: artifact, Payload: payload, SourceHandle: "source", Profile: fixture.profile}}, cleanup, nil
	}))
}

func TestExecutePublishQueryAndRetry(t *testing.T) {
	fixture := newFixture(t)
	result, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
	if err != nil || result.Scan().State() != repository.ScanSucceeded || result.Disposition() != repository.DispositionCreated || len(result.Artifacts()) != 2 {
		t.Fatalf("execute result=%+v err=%v", result, err)
	}
	if result.Artifacts()[0].Name() != "alpha" || result.Artifacts()[1].Name() != "zeta" {
		t.Fatalf("artifact order=%v", []string{result.Artifacts()[0].Name(), result.Artifacts()[1].Name()})
	}
	if fixture.admission.acquired.Load() != 1 || fixture.admission.released.Load() != 1 || fixture.preparer.analyzed.Load() != 1 || fixture.preparer.closed.Load() != 1 {
		t.Fatalf("lifecycle acquire=%d release=%d analyze=%d close=%d", fixture.admission.acquired.Load(), fixture.admission.released.Load(), fixture.preparer.analyzed.Load(), fixture.preparer.closed.Load())
	}
	retried, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
	if err != nil || retried.Disposition() != repository.DispositionAlreadyPresent || fixture.preparer.analyzed.Load() != 1 {
		t.Fatalf("retry=%+v err=%v analyzed=%d", retried, err, fixture.preparer.analyzed.Load())
	}
	query, _ := repository.NewScanQuery(fixture.scope, fixture.request.RepositoryID(), fixture.request.ScanID())
	got, err := fixture.service.GetScan(context.Background(), query)
	if err != nil || got.State() != repository.ScanSucceeded {
		t.Fatalf("get=%+v err=%v", got, err)
	}
	listRequest, _ := fixture.contract.NewScanListRequest(repository.ScanListParams{Scope: fixture.scope, RepositoryID: fixture.request.RepositoryID(), PageSize: 1})
	page, err := fixture.service.ListScans(context.Background(), listRequest)
	if err != nil || len(page.Items()) != 1 {
		t.Fatalf("list=%+v err=%v", page, err)
	}
	artifact := result.Artifacts()[0]
	artifactQuery, _ := repository.NewArtifactQuery(fixture.scope, fixture.request.RepositoryID(), fixture.request.ScanID(), artifact.ArtifactID())
	gotArtifact, err := fixture.service.GetArtifact(context.Background(), artifactQuery)
	if err != nil || gotArtifact.PayloadDigest() != artifact.PayloadDigest() {
		t.Fatalf("artifact=%+v err=%v", gotArtifact, err)
	}
	artifactList, _ := fixture.contract.NewArtifactListRequest(repository.ArtifactListParams{Scope: fixture.scope, RepositoryID: fixture.request.RepositoryID(), ScanID: fixture.request.ScanID(), PageSize: 1})
	artifactPage, err := fixture.service.ListArtifacts(context.Background(), artifactList)
	if err != nil || len(artifactPage.Items()) != 1 || artifactPage.NextCursor() == "" {
		t.Fatalf("artifact page=%+v err=%v", artifactPage, err)
	}
	exportRequest, _ := repository.NewExportArtifactRequest(artifactQuery)
	var output bytes.Buffer
	receipt, err := fixture.service.ExportArtifact(context.Background(), exportRequest, &output)
	if err != nil || receipt.PayloadDigest() != artifact.PayloadDigest() || uint64(output.Len()) != artifact.PayloadSize() {
		t.Fatalf("export bytes=%d receipt=%+v err=%v", output.Len(), receipt, err)
	}
}

func TestSingleFlightOneHundredCallers(t *testing.T) {
	fixture := newFixture(t)
	gate := make(chan struct{})
	fixture.preparer.gate = gate
	type outcome struct {
		result repository.ScanResult
		err    error
	}
	outcomes := make(chan outcome, 100)
	for range 100 {
		go func() {
			result, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
			outcomes <- outcome{result: result, err: err}
		}()
	}
	waitFor(t, time.Second, func() bool {
		fixture.service.mu.Lock()
		defer fixture.service.mu.Unlock()
		for _, current := range fixture.service.flights {
			return current.interested == 100
		}
		return false
	})
	close(gate)
	created, joined := 0, 0
	for range 100 {
		outcome := <-outcomes
		if outcome.err != nil {
			t.Fatal(outcome.err)
		}
		switch outcome.result.Disposition() {
		case repository.DispositionCreated:
			created++
		case repository.DispositionJoined:
			joined++
		}
	}
	if created != 1 || joined != 99 || fixture.admission.acquired.Load() != 1 || fixture.preparer.analyzed.Load() != 1 || fixture.store.beginCount.Load() != 1 || fixture.store.publishCount.Load() != 1 {
		t.Fatalf("created=%d joined=%d acquire=%d analyze=%d begin=%d publish=%d", created, joined, fixture.admission.acquired.Load(), fixture.preparer.analyzed.Load(), fixture.store.beginCount.Load(), fixture.store.publishCount.Load())
	}
}

func TestWaiterCancellationDoesNotCancelLeader(t *testing.T) {
	fixture := newFixture(t)
	gate := make(chan struct{})
	fixture.preparer.gate = gate
	leader := make(chan error, 1)
	go func() { _, err := fixture.service.ExecuteScan(context.Background(), fixture.request); leader <- err }()
	waitFor(t, time.Second, func() bool { return fixture.preparer.analyzed.Load() == 1 })
	waiterCtx, cancel := context.WithCancel(context.Background())
	waiter := make(chan error, 1)
	go func() { _, err := fixture.service.ExecuteScan(waiterCtx, fixture.request); waiter <- err }()
	waitFor(t, time.Second, func() bool {
		fixture.service.mu.Lock()
		defer fixture.service.mu.Unlock()
		for _, current := range fixture.service.flights {
			return current.interested == 2
		}
		return false
	})
	cancel()
	if err := <-waiter; repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("waiter err=%v", err)
	}
	close(gate)
	if err := <-leader; err != nil {
		t.Fatalf("leader err=%v", err)
	}
}

func TestAllWaitersAndAdmissionCancellationFinalizeCanceled(t *testing.T) {
	for _, useAdmission := range []bool{false, true} {
		t.Run(strconv.FormatBool(useAdmission), func(t *testing.T) {
			fixture := newFixture(t)
			fixture.preparer.waitForCancel = true
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { _, err := fixture.service.ExecuteScan(ctx, fixture.request); done <- err }()
			waitFor(t, time.Second, func() bool { return fixture.store.beginCount.Load() == 1 })
			if useAdmission {
				fixture.admission.cancel()
			} else {
				cancel()
			}
			if err := <-done; repository.KindOf(err) != repository.ErrorCanceled {
				t.Fatalf("execute err=%v", err)
			}
			waitFor(t, time.Second, func() bool {
				fixture.store.mu.RLock()
				defer fixture.store.mu.RUnlock()
				return fixture.store.scans[storeScanKey(fixture.scope, fixture.request.RepositoryID(), fixture.request.ScanID())].State() == repository.ScanCanceled
			})
			cancel()
		})
	}
}

func TestExternalCancelCoordinatesFlight(t *testing.T) {
	fixture := newFixture(t)
	fixture.preparer.waitForCancel = true
	executed := make(chan error, 1)
	go func() { _, err := fixture.service.ExecuteScan(context.Background(), fixture.request); executed <- err }()
	waitFor(t, time.Second, func() bool { return fixture.store.beginCount.Load() == 1 })
	cancelRequest, _ := repository.NewCancelScanRequest(repository.CancelScanParams{Scope: fixture.scope, RequestID: "cancel-request", RepositoryID: fixture.request.RepositoryID(), ScanID: fixture.request.ScanID()})
	canceled, err := fixture.service.CancelScan(context.Background(), cancelRequest)
	if err != nil || canceled.State() != repository.ScanCanceled {
		t.Fatalf("cancel=%+v err=%v", canceled, err)
	}
	retried, err := fixture.service.CancelScan(context.Background(), cancelRequest)
	if err != nil || retried.FinishedAt() != canceled.FinishedAt() {
		t.Fatalf("retry cancel=%+v err=%v", retried, err)
	}
	if err = <-executed; repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("execution err=%v", err)
	}
}

func TestFailureOrphanAndPublicationReconciliation(t *testing.T) {
	t.Run("analysis failure", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.preparer.failure = errors.New("unsafe /secret/path")
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorAnalysisFailed || strings.Contains(err.Error(), "secret") {
			t.Fatalf("err=%v", err)
		}
		query, _ := repository.NewScanQuery(fixture.scope, fixture.request.RepositoryID(), fixture.request.ScanID())
		value, _ := fixture.service.GetScan(context.Background(), query)
		if value.State() != repository.ScanFailed || value.ReasonCode() != "analysis-failed" {
			t.Fatalf("scan=%+v", value)
		}
	})
	t.Run("orphan", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store.seedRunning(fixture.scope, fixture.request, fixture.preparer.fingerprint, fixture.preparer.revision, fixture.clock.Now())
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorOrphanedScan {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("commit then response lost", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store.publishMode = publishCommitThenError
		result, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if err != nil || result.Scan().State() != repository.ScanSucceeded {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if fixture.store.finalizeCount.Load() != 0 {
			t.Fatal("published scan was finalized")
		}
	})
	t.Run("ambiguous nonterminal", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store.publishMode = publishErrorBeforeCommit
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorPersistenceUnavailable || !strings.Contains(err.Error(), "publication-ambiguous") {
			t.Fatalf("err=%v", err)
		}
		if fixture.store.finalizeCount.Load() != 0 {
			t.Fatal("ambiguous scan was finalized")
		}
	})
}

func TestFlightConflictsScopeAndIntegrity(t *testing.T) {
	fixture := newFixture(t)
	gate := make(chan struct{})
	fixture.preparer.gate = gate
	done := make(chan error, 1)
	go func() { _, err := fixture.service.ExecuteScan(context.Background(), fixture.request); done <- err }()
	waitFor(t, time.Second, func() bool { return fixture.preparer.analyzed.Load() == 1 })
	conflicting, _ := fixture.contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: fixture.scope, RequestID: "different-request", RepositoryID: fixture.request.RepositoryID(), ScanID: fixture.request.ScanID(), SourceHandle: "source", Profile: fixture.profile})
	if _, err := fixture.service.ExecuteScan(context.Background(), conflicting); repository.KindOf(err) != repository.ErrorScanAlreadyRunning {
		t.Fatalf("conflict=%v", err)
	}
	sameRequestDifferentSource, _ := fixture.contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: fixture.scope, RequestID: fixture.request.RequestID(), RepositoryID: fixture.request.RepositoryID(), ScanID: fixture.request.ScanID(), SourceHandle: "other-source", Profile: fixture.profile})
	if _, err := fixture.service.ExecuteScan(context.Background(), sameRequestDifferentSource); repository.KindOf(err) != repository.ErrorIdempotencyConflict {
		t.Fatalf("idempotency=%v", err)
	}
	otherScope, _ := repository.NewScope(scanTestOtherScopeID, "other-principal")
	query, _ := repository.NewScanQuery(otherScope, fixture.request.RepositoryID(), fixture.request.ScanID())
	if _, err := fixture.service.GetScan(context.Background(), query); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("scope=%v", err)
	}
	close(gate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	fixture.store.overrideGet = true
	fixture.store.getOverride = repository.Scan{}
	query, _ = repository.NewScanQuery(fixture.scope, fixture.request.RepositoryID(), fixture.request.ScanID())
	if _, err := fixture.service.GetScan(context.Background(), query); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("integrity=%v", err)
	}
}

func TestConstructionModelsAndErrorBoundaries(t *testing.T) {
	fixture := newFixture(t)
	if _, err := New(nil, fixture.admission, fixture.preparer, fixture.clock); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatal(err)
	}
	if _, err := New(fixture.store, fixture.admission, fixture.preparer, fixture.clock, Config{CleanupTimeout: time.Nanosecond}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatal(err)
	}
	if _, err := New(fixture.store, fixture.admission, fixture.preparer, fixture.clock, Config{}, Config{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatal(err)
	}
	if _, err := NewArtifactCandidate(ArtifactCandidateParams{}); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatal(err)
	}
	candidate := fixture.preparer.result.Candidates()[0]
	if _, err := NewAnalysisResult(fixture.profile, []ArtifactCandidate{candidate, candidate}); repository.KindOf(err) != repository.ErrorConflict {
		t.Fatal(err)
	}
	if _, err := NewBeginResult("invalid", repository.Scan{}, nil); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatal(err)
	}
	if _, err := NewScanList([]repository.Scan{{}}, ""); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatal(err)
	}
	if _, err := NewArtifactList([]repository.Artifact{{}}, ""); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatal(err)
	}
	fixture.store.failure = errors.New("driver password=unsafe")
	query, _ := repository.NewScanQuery(fixture.scope, fixture.request.RepositoryID(), fixture.request.ScanID())
	if _, err := fixture.service.GetScan(context.Background(), query); repository.KindOf(err) != repository.ErrorPersistenceUnavailable || strings.Contains(err.Error(), "password") {
		t.Fatalf("err=%v", err)
	}
	var nilService *Service
	if _, err := nilService.ExecuteScan(context.Background(), fixture.request); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatal(err)
	}
	if _, err := fixture.service.ExportArtifact(context.Background(), repository.ExportArtifactRequest{}, nil); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatal(err)
	}
}

func TestArtifactDependencyValidationAndGraph(t *testing.T) {
	if _, err := NewArtifactDependency("", "1.0.0", 0); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("empty dependency = %v", err)
	}
	if _, err := NewArtifactDependency("first", "1.0.0", -1); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("negative ordinal = %v", err)
	}
	firstToSecond, _ := NewArtifactDependency("second", "1.0.0", 0)
	secondToFirst, _ := NewArtifactDependency("first", "1.0.0", 0)
	if firstToSecond.Name() != "second" || firstToSecond.Version() != "1.0.0" || firstToSecond.Ordinal() != 0 {
		t.Fatal("dependency accessors failed")
	}
	badOrder, _ := NewArtifactDependency("second", "1.0.0", 1)
	if _, err := dependencyCandidate("first", []ArtifactDependency{badOrder}); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("bad order = %v", err)
	}
	duplicateAtOne, _ := NewArtifactDependency("second", "1.0.0", 1)
	if _, err := dependencyCandidate("first", []ArtifactDependency{firstToSecond, duplicateAtOne}); repository.KindOf(err) != repository.ErrorConflict {
		t.Fatalf("duplicate = %v", err)
	}
	first, _ := dependencyCandidate("first", []ArtifactDependency{firstToSecond})
	if first.Dependencies()[0].Name() != "second" {
		t.Fatal("dependency copy failed")
	}
	profile := repository.DefaultRepositoryGoProfile().Profile()
	if _, err := NewAnalysisResult(profile, []ArtifactCandidate{first}); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("missing dependency = %v", err)
	}
	self, _ := NewArtifactDependency("first", "1.0.0", 0)
	selfCandidate, _ := dependencyCandidate("first", []ArtifactDependency{self})
	if _, err := NewAnalysisResult(profile, []ArtifactCandidate{selfCandidate}); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("self dependency = %v", err)
	}
	second, _ := dependencyCandidate("second", []ArtifactDependency{secondToFirst})
	if _, err := NewAnalysisResult(profile, []ArtifactCandidate{first, second}); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("cycle = %v", err)
	}
	second, _ = dependencyCandidate("second", nil)
	if result, err := NewAnalysisResult(profile, []ArtifactCandidate{first, second}); err != nil || len(result.Candidates()) != 2 {
		t.Fatalf("acyclic result = %v", err)
	}
}

func dependencyCandidate(name string, dependencies []ArtifactDependency) (ArtifactCandidate, error) {
	payload := []byte("{}")
	return NewArtifactCandidate(ArtifactCandidateParams{Name: name, Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "1.0.0", MediaType: "application/json", PayloadDigest: sha256.Sum256(payload), PayloadSize: uint64(len(payload)), ProducerName: "test", ProducerVersion: "1.0.0", Dependencies: dependencies, Payload: byteSource(payload)})
}

func TestExecutionDependencyAndStateBranches(t *testing.T) {
	t.Run("admission failure", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.admission.failure = errors.New("unsafe admission detail")
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorInternal || strings.Contains(err.Error(), "unsafe") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("invalid lease", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.admission.invalid = true
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorInternal {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("prepare failure and nil", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.preparer.prepareFailure = errors.New("unsafe source")
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorSourceUnavailable || strings.Contains(err.Error(), "unsafe") {
			t.Fatalf("err=%v", err)
		}
		fixture = newFixture(t)
		fixture.preparer.nilSession = true
		_, err = fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorSourceUnavailable {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("invalid proof and clock", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.preparer.fingerprint = repository.Digest{}
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorSourceUnavailable {
			t.Fatalf("err=%v", err)
		}
		fixture = newFixture(t)
		fixture.preparer.revision = " bad "
		_, err = fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorSourceUnavailable {
			t.Fatalf("err=%v", err)
		}
		fixture = newFixture(t)
		fixture.service.clock = ClockFunc(func() time.Time { return time.Time{} })
		_, err = fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorInternal {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("begin failure and terminal retries", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store.beginFailure = errors.New("unsafe store")
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorPersistenceUnavailable || strings.Contains(err.Error(), "unsafe") {
			t.Fatalf("err=%v", err)
		}
		for _, state := range []repository.ScanState{repository.ScanFailed, repository.ScanCanceled} {
			fixture = newFixture(t)
			terminal := terminalScan(t, fixture, state)
			status := BeginPreviouslyFailed
			if state == repository.ScanCanceled {
				status = BeginPreviouslyCanceled
			}
			value, _ := NewBeginResult(status, terminal, nil)
			fixture.store.beginOverride = &value
			_, err = fixture.service.ExecuteScan(context.Background(), fixture.request)
			expected := repository.ErrorAnalysisFailed
			if state == repository.ScanCanceled {
				expected = repository.ErrorCanceled
			}
			if repository.KindOf(err) != expected {
				t.Fatalf("state=%s err=%v", state, err)
			}
		}
	})
	t.Run("analysis mismatch and finalization failure", func(t *testing.T) {
		fixture := newFixture(t)
		other, _ := repository.NewAnalysisProfile("other", "1", repository.DigestBytes([]byte("other")))
		fixture.preparer.result.profile = other
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorAnalysisFailed {
			t.Fatalf("err=%v", err)
		}
		fixture = newFixture(t)
		fixture.preparer.failure = errors.New("analysis")
		fixture.store.finalizeFailure = errors.New("database")
		_, err = fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("publication result mismatch", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.store.publishInvalidResult = true
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorIntegrityFailure {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("reconciled failed canceled and unavailable", func(t *testing.T) {
		for _, state := range []repository.ScanState{repository.ScanFailed, repository.ScanCanceled} {
			fixture := newFixture(t)
			fixture.store.publishMode = publishErrorBeforeCommit
			terminal := terminalScan(t, fixture, state)
			value, _ := NewReconcileResult(terminal, nil)
			fixture.store.reconcileOverride = &value
			_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
			expected := repository.ErrorAnalysisFailed
			if state == repository.ScanCanceled {
				expected = repository.ErrorCanceled
			}
			if repository.KindOf(err) != expected {
				t.Fatalf("state=%s err=%v", state, err)
			}
		}
		fixture := newFixture(t)
		fixture.store.publishMode = publishErrorBeforeCommit
		fixture.store.reconcileFailure = errors.New("unavailable")
		_, err := fixture.service.ExecuteScan(context.Background(), fixture.request)
		if repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestQueryCancellationErrorsAndModelAccessors(t *testing.T) {
	fixture := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	query, _ := repository.NewScanQuery(fixture.scope, fixture.request.RepositoryID(), fixture.request.ScanID())
	if _, err := fixture.service.GetScan(ctx, query); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatal(err)
	}
	listRequest, _ := fixture.contract.NewScanListRequest(repository.ScanListParams{Scope: fixture.scope, RepositoryID: fixture.request.RepositoryID(), PageSize: 1})
	if _, err := fixture.service.ListScans(ctx, listRequest); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatal(err)
	}
	artifactQuery, _ := repository.NewArtifactQuery(fixture.scope, fixture.request.RepositoryID(), fixture.request.ScanID(), "artifact")
	if _, err := fixture.service.GetArtifact(ctx, artifactQuery); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatal(err)
	}
	artifactList, _ := fixture.contract.NewArtifactListRequest(repository.ArtifactListParams{Scope: fixture.scope, RepositoryID: fixture.request.RepositoryID(), ScanID: fixture.request.ScanID(), PageSize: 1})
	if _, err := fixture.service.ListArtifacts(ctx, artifactList); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatal(err)
	}
	export, _ := repository.NewExportArtifactRequest(artifactQuery)
	if _, err := fixture.service.ExportArtifact(ctx, export, io.Discard); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatal(err)
	}
	analysisRequest := NewAnalysisRequest(fixture.request)
	if analysisRequest.Scope() != fixture.scope || analysisRequest.RepositoryID() != fixture.request.RepositoryID() || analysisRequest.ScanID() != fixture.request.ScanID() || analysisRequest.SourceHandle() != fixture.request.SourceHandle() || analysisRequest.Profile() != fixture.profile {
		t.Fatal("analysis accessors")
	}
	clock := ClockFunc(func() time.Time { return fixture.clock.current })
	_ = clock.Now()
	config := Config{}.withDefaults()
	if config != DefaultConfig() {
		t.Fatalf("defaults=%+v", config)
	}
	badConfigs := []Config{{CleanupTimeout: time.Hour, FinalizationTimeout: time.Second, MaxArtifacts: 1}, {CleanupTimeout: time.Second, FinalizationTimeout: time.Hour, MaxArtifacts: 1}, {CleanupTimeout: time.Second, FinalizationTimeout: time.Second, MaxArtifacts: 65}}
	for _, bad := range badConfigs {
		if bad.Validate() == nil {
			t.Fatalf("accepted=%+v", bad)
		}
	}
}

func terminalScan(t *testing.T, fixture *testFixture, state repository.ScanState) repository.Scan {
	t.Helper()
	reason := "analysis-failed"
	if state == repository.ScanCanceled {
		reason = "canceled"
	}
	value, err := repository.NewScan(repository.ScanParams{RepositoryID: fixture.request.RepositoryID(), ScanID: fixture.request.ScanID(), Profile: fixture.profile, SourceRevision: fixture.preparer.revision, State: state, ReasonCode: reason, RequestedAt: fixture.clock.current, StartedAt: fixture.clock.current, FinishedAt: fixture.clock.current.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func FuzzArtifactCandidateNeverPanics(f *testing.F) {
	f.Add("artifact", "1.0.0", "application/json")
	f.Add("../bad", "", "text/plain")
	f.Fuzz(func(t *testing.T, name, version, media string) {
		payload := []byte("{}")
		_, _ = NewArtifactCandidate(ArtifactCandidateParams{Name: name, Version: version, StableIDScheme: repository.ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "1.0.0", MediaType: media, PayloadDigest: sha256.Sum256(payload), PayloadSize: uint64(len(payload)), ProducerName: "fake-analysis", ProducerVersion: "1.0.0", Payload: byteSource(payload)})
	})
}

type testFixture struct {
	contract  *repository.Contract
	scope     repository.Scope
	profile   repository.AnalysisProfile
	request   repository.ExecuteScanRequest
	clock     *stepClock
	store     *memoryStore
	admission *fakeAdmission
	preparer  *fakePreparer
	service   *Service
}

func newFixture(t *testing.T) *testFixture {
	t.Helper()
	contract, _ := repository.New()
	scope, _ := repository.NewScope(scanTestScopeID, "scan-principal")
	profile := contract.Profiles().Definitions()[0].Profile()
	request, _ := contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: "execute-request", RepositoryID: scanTestRepositoryID, ScanID: scanTestScanID, SourceHandle: "source", Profile: profile})
	clock := &stepClock{current: time.Date(2026, 7, 27, 15, 0, 0, 0, time.UTC)}
	store := newMemoryStore()
	store.addRepository(scope, request.RepositoryID(), repository.RepositoryActive)
	admission := newFakeAdmission()
	preparer := newFakePreparer(t, profile)
	service, err := New(store, admission, preparer, clock)
	if err != nil {
		t.Fatal(err)
	}
	return &testFixture{contract: contract, scope: scope, profile: profile, request: request, clock: clock, store: store, admission: admission, preparer: preparer, service: service}
}

type stepClock struct {
	mu      sync.Mutex
	current time.Time
}

func (clock *stepClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	value := clock.current
	clock.current = clock.current.Add(time.Millisecond)
	return value
}

type fakeAdmission struct {
	acquired, released atomic.Int64
	ctx                context.Context
	cancel             context.CancelFunc
	failure            error
	invalid            bool
}

func newFakeAdmission() *fakeAdmission {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeAdmission{ctx: ctx, cancel: cancel}
}
func (admission *fakeAdmission) Acquire(ctx context.Context) (WorkLease, error) {
	if admission.failure != nil {
		return nil, admission.failure
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if admission.invalid {
		return &fakeLease{released: &admission.released}, nil
	}
	admission.acquired.Add(1)
	return &fakeLease{ctx: admission.ctx, released: &admission.released}, nil
}

type fakeLease struct {
	ctx      context.Context
	released *atomic.Int64
	once     sync.Once
}

func (lease *fakeLease) Context() context.Context { return lease.ctx }
func (lease *fakeLease) Done()                    { lease.once.Do(func() { lease.released.Add(1) }) }

type fakePreparer struct {
	profile                    repository.AnalysisProfile
	fingerprint                repository.Digest
	revision                   string
	result                     AnalysisResult
	failure                    error
	prepareFailure             error
	nilSession                 bool
	gate                       chan struct{}
	waitForCancel              bool
	prepared, analyzed, closed atomic.Int64
}

func newFakePreparer(t *testing.T, profile repository.AnalysisProfile) *fakePreparer {
	t.Helper()
	makeCandidate := func(name, payload string) ArtifactCandidate {
		value := []byte(payload)
		candidate, err := NewArtifactCandidate(ArtifactCandidateParams{Name: name, Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "1.0.0", MediaType: "application/json", PayloadDigest: sha256.Sum256(value), PayloadSize: uint64(len(value)), ProducerName: "fake-analysis", ProducerVersion: "1.0.0", Payload: byteSource(value)})
		if err != nil {
			t.Fatal(err)
		}
		return candidate
	}
	result, _ := NewAnalysisResult(profile, []ArtifactCandidate{makeCandidate("zeta", "{\"zeta\":true}\n"), makeCandidate("alpha", "{\"alpha\":true}\n")})
	return &fakePreparer{profile: profile, fingerprint: repository.DigestBytes([]byte("source-proof")), revision: "revision-1", result: result}
}
func (preparer *fakePreparer) Prepare(ctx context.Context, _ AnalysisRequest) (AnalysisSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	preparer.prepared.Add(1)
	if preparer.prepareFailure != nil {
		return nil, preparer.prepareFailure
	}
	if preparer.nilSession {
		return nil, nil
	}
	return &fakeSession{owner: preparer}, nil
}

type fakeSession struct {
	owner *fakePreparer
	once  sync.Once
}

func (session *fakeSession) SourceFingerprint() repository.Digest { return session.owner.fingerprint }
func (session *fakeSession) SourceRevision() string               { return session.owner.revision }
func (session *fakeSession) Analyze(ctx context.Context) (AnalysisResult, error) {
	session.owner.analyzed.Add(1)
	if session.owner.waitForCancel {
		<-ctx.Done()
		return AnalysisResult{}, ctx.Err()
	}
	if session.owner.gate != nil {
		select {
		case <-session.owner.gate:
		case <-ctx.Done():
			return AnalysisResult{}, ctx.Err()
		}
	}
	if session.owner.failure != nil {
		return AnalysisResult{}, session.owner.failure
	}
	return session.owner.result, nil
}
func (session *fakeSession) Close(context.Context) error {
	session.once.Do(func() { session.owner.closed.Add(1) })
	return nil
}

type byteSource []byte

func (source byteSource) Open(ctx context.Context) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), source...))), nil
}

type requestRecord struct {
	fingerprint repository.Digest
	scanKey     string
}
type storedArtifact struct {
	metadata repository.Artifact
	payload  []byte
}
type publishBehavior int

const (
	publishNormal publishBehavior = iota
	publishCommitThenError
	publishErrorBeforeCommit
)

type memoryStore struct {
	mu                                      sync.RWMutex
	repositories                            map[string]repository.RepositoryState
	scans                                   map[string]repository.Scan
	requests                                map[string]requestRecord
	artifacts                               map[string]storedArtifact
	publishMode                             publishBehavior
	failure                                 error
	beginFailure                            error
	beginOverride                           *BeginResult
	finalizeFailure                         error
	reconcileFailure                        error
	reconcileOverride                       *ReconcileResult
	publishInvalidResult                    bool
	overrideGet                             bool
	getOverride                             repository.Scan
	beginCount, publishCount, finalizeCount atomic.Int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{repositories: map[string]repository.RepositoryState{}, scans: map[string]repository.Scan{}, requests: map[string]requestRecord{}, artifacts: map[string]storedArtifact{}}
}
func (store *memoryStore) addRepository(scope repository.Scope, id repository.RepositoryID, state repository.RepositoryState) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.repositories[storeRepositoryKey(scope, id)] = state
}
func (store *memoryStore) Begin(ctx context.Context, command BeginCommand) (BeginResult, error) {
	if err := ctx.Err(); err != nil {
		return BeginResult{}, err
	}
	store.beginCount.Add(1)
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.beginFailure != nil {
		return BeginResult{}, store.beginFailure
	}
	if store.beginOverride != nil {
		return *store.beginOverride, nil
	}
	if store.failure != nil {
		return BeginResult{}, store.failure
	}
	if store.repositories[storeRepositoryKey(command.Scope(), command.Scan().RepositoryID())] != repository.RepositoryActive {
		return BeginResult{}, repository.NewError(repository.ErrorConflict, "begin-scan", "repository-not-active", false, nil)
	}
	rkey := storeRequestKey(command.Scope(), command.RequestID())
	skey := storeScanKey(command.Scope(), command.Scan().RepositoryID(), command.Scan().ScanID())
	if prior, ok := store.requests[rkey]; ok && (prior.fingerprint != command.MutationFingerprint() || prior.scanKey != skey) {
		return BeginResult{}, repository.NewError(repository.ErrorIdempotencyConflict, "begin-scan", "request-reused", false, nil)
	}
	if current, ok := store.scans[skey]; ok {
		if prior, ok := store.requests[rkey]; !ok || prior.fingerprint != command.MutationFingerprint() {
			return BeginResult{}, repository.NewError(repository.ErrorScanAlreadyRunning, "begin-scan", "scan-id-reused", false, nil)
		}
		switch current.State() {
		case repository.ScanSucceeded:
			return NewBeginResult(BeginAlreadyPublished, current, store.artifactsForLocked(command.Scope(), current.RepositoryID(), current.ScanID()))
		case repository.ScanFailed:
			return NewBeginResult(BeginPreviouslyFailed, current, nil)
		case repository.ScanCanceled:
			return NewBeginResult(BeginPreviouslyCanceled, current, nil)
		default:
			return NewBeginResult(BeginOrphaned, current, nil)
		}
	}
	store.scans[skey] = command.Scan()
	store.requests[rkey] = requestRecord{fingerprint: command.MutationFingerprint(), scanKey: skey}
	return NewBeginResult(BeginStarted, command.Scan(), nil)
}
func (store *memoryStore) Publish(ctx context.Context, command PublishCommand) (repository.ScanResult, error) {
	if err := ctx.Err(); err != nil {
		return repository.ScanResult{}, err
	}
	store.publishCount.Add(1)
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.publishMode == publishErrorBeforeCommit {
		return repository.ScanResult{}, errors.New("response unavailable")
	}
	skey := storeScanKey(command.Scope(), command.Scan().RepositoryID(), command.Scan().ScanID())
	current, ok := store.scans[skey]
	if !ok || current.State() != repository.ScanRunning {
		return repository.ScanResult{}, repository.NewError(repository.ErrorConflict, "publish-scan", "scan-not-running", false, nil)
	}
	metadata := make([]repository.Artifact, 0, len(command.Artifacts()))
	staged := make(map[string]storedArtifact, len(command.Artifacts()))
	for _, item := range command.Artifacts() {
		reader, err := item.Open(ctx)
		if err != nil {
			return repository.ScanResult{}, err
		}
		payload, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			return repository.ScanResult{}, err
		}
		if closeErr != nil {
			return repository.ScanResult{}, closeErr
		}
		artifact := item.Metadata()
		if uint64(len(payload)) != artifact.PayloadSize() || sha256.Sum256(payload) != artifact.PayloadDigest() {
			return repository.ScanResult{}, repository.NewError(repository.ErrorIntegrityFailure, "publish-scan", "payload-mismatch", false, nil)
		}
		key := storeArtifactKey(command.Scope(), command.Scan().RepositoryID(), command.Scan().ScanID(), artifact.ArtifactID())
		staged[key] = storedArtifact{metadata: artifact, payload: append([]byte(nil), payload...)}
		metadata = append(metadata, artifact)
	}
	store.scans[skey] = command.Scan()
	for key, value := range staged {
		store.artifacts[key] = value
	}
	result, _ := repository.NewScanResult(command.Scan(), metadata, repository.DispositionCreated)
	if store.publishInvalidResult {
		return repository.ScanResult{}, nil
	}
	if store.publishMode == publishCommitThenError {
		return repository.ScanResult{}, errors.New("response lost after commit")
	}
	return result, nil
}
func (store *memoryStore) Finalize(ctx context.Context, command FinalizeCommand) (repository.Scan, error) {
	if err := ctx.Err(); err != nil {
		return repository.Scan{}, err
	}
	store.finalizeCount.Add(1)
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.finalizeFailure != nil {
		return repository.Scan{}, store.finalizeFailure
	}
	key := storeScanKey(command.Scope(), command.Scan().RepositoryID(), command.Scan().ScanID())
	current, ok := store.scans[key]
	if !ok {
		return repository.Scan{}, notFound("finalize-scan")
	}
	if current.State() == repository.ScanSucceeded {
		return repository.Scan{}, repository.NewError(repository.ErrorConflict, "finalize-scan", "already-published", false, nil)
	}
	if current.State() == repository.ScanFailed || current.State() == repository.ScanCanceled {
		return current, nil
	}
	store.scans[key] = command.Scan()
	return command.Scan(), nil
}
func (store *memoryStore) Reconcile(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) (ReconcileResult, error) {
	if store.reconcileFailure != nil {
		return ReconcileResult{}, store.reconcileFailure
	}
	if store.reconcileOverride != nil {
		return *store.reconcileOverride, nil
	}
	value, err := store.GetScan(ctx, scope, repositoryID, scanID)
	if err != nil {
		return ReconcileResult{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return NewReconcileResult(value, store.artifactsForLocked(scope, repositoryID, scanID))
}
func (store *memoryStore) GetScan(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) (repository.Scan, error) {
	if err := ctx.Err(); err != nil {
		return repository.Scan{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.failure != nil {
		return repository.Scan{}, store.failure
	}
	if store.overrideGet {
		return store.getOverride, nil
	}
	value, ok := store.scans[storeScanKey(scope, repositoryID, scanID)]
	if !ok {
		return repository.Scan{}, notFound("get-scan")
	}
	return value, nil
}
func (store *memoryStore) ListScans(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, size int, cursor repository.Cursor) (ScanList, error) {
	if err := ctx.Err(); err != nil {
		return ScanList{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	values := []repository.Scan{}
	prefix := storeScopeKey(scope) + "|" + string(repositoryID) + "|"
	for key, value := range store.scans {
		if strings.HasPrefix(key, prefix) {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ScanID() < values[j].ScanID() })
	start, end, next := pageBounds(len(values), size, cursor)
	return NewScanList(values[start:end], next)
}
func (store *memoryStore) Cancel(ctx context.Context, command CancelCommand) (repository.Scan, error) {
	if err := ctx.Err(); err != nil {
		return repository.Scan{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	rkey := storeRequestKey(command.Scope(), command.RequestID())
	skey := storeScanKey(command.Scope(), command.RepositoryID(), command.ScanID())
	if prior, ok := store.requests[rkey]; ok && (prior.fingerprint != command.MutationFingerprint() || prior.scanKey != skey) {
		return repository.Scan{}, repository.NewError(repository.ErrorIdempotencyConflict, "cancel-scan", "request-reused", false, nil)
	}
	current, ok := store.scans[skey]
	if !ok {
		return repository.Scan{}, notFound("cancel-scan")
	}
	if current.State() == repository.ScanSucceeded {
		return repository.Scan{}, repository.NewError(repository.ErrorConflict, "cancel-scan", "already-published", false, nil)
	}
	if current.State() == repository.ScanCanceled {
		return current, nil
	}
	canceled, _ := repository.NewScan(repository.ScanParams{RepositoryID: current.RepositoryID(), ScanID: current.ScanID(), Profile: current.Profile(), SourceRevision: current.SourceRevision(), State: repository.ScanCanceled, ReasonCode: "caller-canceled", RequestedAt: current.RequestedAt(), StartedAt: current.StartedAt(), FinishedAt: command.At()})
	store.scans[skey] = canceled
	store.requests[rkey] = requestRecord{fingerprint: command.MutationFingerprint(), scanKey: skey}
	return canceled, nil
}
func (store *memoryStore) GetArtifact(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, artifactID repository.ArtifactID) (repository.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return repository.Artifact{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	value, ok := store.artifacts[storeArtifactKey(scope, repositoryID, scanID, artifactID)]
	if !ok {
		return repository.Artifact{}, notFound("get-artifact")
	}
	return value.metadata, nil
}
func (store *memoryStore) ListArtifacts(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, size int, cursor repository.Cursor) (ArtifactList, error) {
	if err := ctx.Err(); err != nil {
		return ArtifactList{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	values := store.artifactsForLocked(scope, repositoryID, scanID)
	start, end, next := pageBounds(len(values), size, cursor)
	return NewArtifactList(values[start:end], next)
}
func (store *memoryStore) ExportArtifact(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, artifactID repository.ArtifactID, writer io.Writer) (repository.ExportReceipt, error) {
	if err := ctx.Err(); err != nil {
		return repository.ExportReceipt{}, err
	}
	store.mu.RLock()
	value, ok := store.artifacts[storeArtifactKey(scope, repositoryID, scanID, artifactID)]
	payload := append([]byte(nil), value.payload...)
	store.mu.RUnlock()
	if !ok {
		return repository.ExportReceipt{}, notFound("export-artifact")
	}
	if _, err := writer.Write(payload); err != nil {
		return repository.ExportReceipt{}, err
	}
	return repository.NewExportReceipt(value.metadata.PayloadDigest(), uint64(len(payload)))
}
func (store *memoryStore) artifactsForLocked(scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) []repository.Artifact {
	values := []repository.Artifact{}
	prefix := storeScanKey(scope, repositoryID, scanID) + "|"
	for key, value := range store.artifacts {
		if strings.HasPrefix(key, prefix) {
			values = append(values, value.metadata)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		left := values[i].Name() + "\x00" + values[i].Version() + "\x00" + values[i].StableIDScheme()
		right := values[j].Name() + "\x00" + values[j].Version() + "\x00" + values[j].StableIDScheme()
		return left < right
	})
	return values
}
func (store *memoryStore) seedRunning(scope repository.Scope, request repository.ExecuteScanRequest, source repository.Digest, revision string, now time.Time) {
	running, _ := repository.NewScan(repository.ScanParams{RepositoryID: request.RepositoryID(), ScanID: request.ScanID(), Profile: request.Profile(), SourceRevision: revision, State: repository.ScanRunning, RequestedAt: now, StartedAt: now})
	fingerprint := executeFingerprint(request, source, revision)
	store.mu.Lock()
	defer store.mu.Unlock()
	key := storeScanKey(scope, request.RepositoryID(), request.ScanID())
	store.scans[key] = running
	store.requests[storeRequestKey(scope, request.RequestID())] = requestRecord{fingerprint: fingerprint, scanKey: key}
}
func storeScopeKey(scope repository.Scope) string {
	return string(scope.ScopeID()) + "|" + string(scope.PrincipalID())
}
func storeRepositoryKey(scope repository.Scope, id repository.RepositoryID) string {
	return storeScopeKey(scope) + "|" + string(id)
}
func storeScanKey(scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) string {
	return storeRepositoryKey(scope, repositoryID) + "|" + string(scanID)
}
func storeArtifactKey(scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, artifactID repository.ArtifactID) string {
	return storeScanKey(scope, repositoryID, scanID) + "|" + string(artifactID)
}
func storeRequestKey(scope repository.Scope, requestID repository.RequestID) string {
	return storeScopeKey(scope) + "|" + string(requestID)
}
func pageBounds(length, size int, cursor repository.Cursor) (int, int, repository.Cursor) {
	start := 0
	if cursor != "" {
		start, _ = strconv.Atoi(string(cursor))
		if start < 0 || start > length {
			start = length
		}
	}
	end := min(start+size, length)
	next := repository.Cursor("")
	if end < length {
		next = repository.Cursor(strconv.Itoa(end))
	}
	return start, end, next
}
func notFound(operation string) error {
	return repository.NewError(repository.ErrorNotFound, operation, "not-found", false, nil)
}
func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not reached")
}
