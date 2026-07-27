package conformance

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

type Suite struct{ config Config }

func New(configs ...Config) (*Suite, error) {
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
	return &Suite{config: config}, nil
}

func (suite *Suite) Run(t *testing.T, factory Factory) {
	t.Helper()
	if factory == nil {
		t.Fatal(ErrFactoryRequired)
	}
	suite.run(t, factory, "seeded-reads-and-export", suite.seededReads)
	suite.run(t, factory, "scope-isolation", suite.scopeIsolation)
	suite.run(t, factory, "repository-mutations", suite.repositoryMutations)
	suite.run(t, factory, "scan-execution-and-cancel", suite.scanExecutionAndCancel)
	suite.run(t, factory, "context-cancellation", suite.contextCancellation)
}

func (suite *Suite) RunLifecycle(t *testing.T, factory LifecycleFactory) {
	t.Helper()
	if factory == nil {
		t.Fatal(ErrFactoryRequired)
	}
	suite.runLifecycle(t, factory, "lifecycle-seeded-reads", suite.lifecycleSeededReads)
	suite.runLifecycle(t, factory, "lifecycle-scope-isolation", suite.lifecycleScopeIsolation)
	suite.runLifecycle(t, factory, "lifecycle-idempotency-and-archive", suite.lifecycleMutations)
	suite.runLifecycle(t, factory, "lifecycle-context-cancellation", suite.lifecycleCancellation)
}

func (suite *Suite) runLifecycle(t *testing.T, factory LifecycleFactory, name string, check func(*testing.T, LifecycleFixture)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		fixture, cleanup, err := factory.OpenLifecycle(context.Background())
		if err != nil {
			t.Fatalf("open lifecycle fixture: %v", err)
		}
		if fixture.Service == nil || fixture.Contract == nil || fixture.Scenario.Repository.RepositoryID() == "" || cleanup == nil {
			t.Fatal(ErrInvalidFixture)
		}
		t.Cleanup(func() {
			if err := cleanup(context.Background()); err != nil {
				t.Errorf("cleanup lifecycle fixture: %v", err)
			}
			if err := cleanup(context.Background()); err != nil {
				t.Errorf("idempotent lifecycle cleanup fixture: %v", err)
			}
		})
		check(t, LifecycleFixture{Service: fixture.Service, Contract: fixture.Contract, Scenario: fixture.Scenario.clone()})
	})
}

func (suite *Suite) lifecycleSeededReads(t *testing.T, fixture LifecycleFixture) {
	scenario := fixture.Scenario
	query, _ := repository.NewRepositoryQuery(scenario.PrimaryScope, scenario.Repository.RepositoryID())
	value, err := fixture.Service.GetRepository(context.Background(), query)
	if err != nil || value.RepositoryID() != scenario.Repository.RepositoryID() {
		t.Fatalf("get repository: %+v, %v", value, err)
	}
	request, _ := fixture.Contract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: scenario.PrimaryScope, PageSize: 1})
	page, err := fixture.Service.ListRepositories(context.Background(), request)
	if err != nil || len(page.Items()) != 1 || page.Items()[0].RepositoryID() != scenario.Repository.RepositoryID() {
		t.Fatalf("list repositories: %+v, %v", page, err)
	}
}

func (suite *Suite) lifecycleScopeIsolation(t *testing.T, fixture LifecycleFixture) {
	scenario := fixture.Scenario
	query, _ := repository.NewRepositoryQuery(scenario.OtherScope, scenario.Repository.RepositoryID())
	if _, err := fixture.Service.GetRepository(context.Background(), query); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("get scope escape: %v", err)
	}
	list, _ := fixture.Contract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: scenario.OtherScope, PageSize: 10})
	page, err := fixture.Service.ListRepositories(context.Background(), list)
	if err != nil || len(page.Items()) != 0 {
		t.Fatalf("list scope escape: %+v, %v", page, err)
	}
	archive, _ := repository.NewArchiveRepositoryRequest(repository.ArchiveRepositoryParams{Scope: scenario.OtherScope, RequestID: "lifecycle-scope-archive", RepositoryID: scenario.Repository.RepositoryID()})
	if _, err = fixture.Service.ArchiveRepository(context.Background(), archive); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("archive scope escape: %v", err)
	}
	register, _ := fixture.Contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scenario.OtherScope, RequestID: "lifecycle-scope-register", RepositoryID: scenario.Repository.RepositoryID(), DisplayName: "Other Scope", SourceHandle: scenario.SourceHandle})
	other, err := fixture.Service.RegisterRepository(context.Background(), register)
	if err != nil || other.DisplayName() != "Other Scope" {
		t.Fatalf("independent scope registration: %+v, %v", other, err)
	}
	primaryQuery, _ := repository.NewRepositoryQuery(scenario.PrimaryScope, scenario.Repository.RepositoryID())
	primary, err := fixture.Service.GetRepository(context.Background(), primaryQuery)
	if err != nil || primary.DisplayName() != scenario.Repository.DisplayName() {
		t.Fatalf("cross-scope mutation: %+v, %v", primary, err)
	}
}

func (suite *Suite) lifecycleMutations(t *testing.T, fixture LifecycleFixture) {
	scenario := fixture.Scenario
	request, _ := fixture.Contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scenario.PrimaryScope, RequestID: "lifecycle-register", RepositoryID: "lifecycle-created", DisplayName: "Lifecycle Created", SourceHandle: scenario.SourceHandle})
	created, err := fixture.Service.RegisterRepository(context.Background(), request)
	if err != nil || created.State() != repository.RepositoryActive {
		t.Fatalf("register: %+v, %v", created, err)
	}
	retried, err := fixture.Service.RegisterRepository(context.Background(), request)
	if err != nil || retried.RepositoryID() != created.RepositoryID() || retried.CreatedAt() != created.CreatedAt() {
		t.Fatalf("register retry: %+v, %v", retried, err)
	}
	conflict, _ := fixture.Contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scenario.PrimaryScope, RequestID: "lifecycle-register", RepositoryID: "lifecycle-conflict", DisplayName: "Conflict", SourceHandle: scenario.SourceHandle})
	if _, err = fixture.Service.RegisterRepository(context.Background(), conflict); repository.KindOf(err) != repository.ErrorIdempotencyConflict {
		t.Fatalf("register conflict: %v", err)
	}
	archive, _ := repository.NewArchiveRepositoryRequest(repository.ArchiveRepositoryParams{Scope: scenario.PrimaryScope, RequestID: "lifecycle-archive", RepositoryID: created.RepositoryID()})
	archived, err := fixture.Service.ArchiveRepository(context.Background(), archive)
	if err != nil || archived.State() != repository.RepositoryArchived {
		t.Fatalf("archive: %+v, %v", archived, err)
	}
	retryArchive, err := fixture.Service.ArchiveRepository(context.Background(), archive)
	if err != nil || retryArchive.UpdatedAt() != archived.UpdatedAt() {
		t.Fatalf("archive retry: %+v, %v", retryArchive, err)
	}
	archiveConflict, _ := repository.NewArchiveRepositoryRequest(repository.ArchiveRepositoryParams{Scope: scenario.PrimaryScope, RequestID: "lifecycle-archive", RepositoryID: scenario.Repository.RepositoryID()})
	if _, err = fixture.Service.ArchiveRepository(context.Background(), archiveConflict); repository.KindOf(err) != repository.ErrorIdempotencyConflict {
		t.Fatalf("archive conflict: %v", err)
	}
}

func (suite *Suite) lifecycleCancellation(t *testing.T, fixture LifecycleFixture) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	query, _ := repository.NewRepositoryQuery(fixture.Scenario.PrimaryScope, fixture.Scenario.Repository.RepositoryID())
	if _, err := fixture.Service.GetRepository(ctx, query); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled lifecycle read: %v", err)
	}
}

func (suite *Suite) run(t *testing.T, factory Factory, name string, check func(*testing.T, Fixture)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		fixture, cleanup, err := factory.Open(context.Background())
		if err != nil {
			t.Fatalf("open fixture: %v", err)
		}
		if fixture.Service == nil || fixture.Contract == nil || fixture.Scenario.Repository.RepositoryID() == "" || cleanup == nil {
			t.Fatal(ErrInvalidFixture)
		}
		t.Cleanup(func() {
			if err := cleanup(context.Background()); err != nil {
				t.Errorf("cleanup fixture: %v", err)
			}
			if err := cleanup(context.Background()); err != nil {
				t.Errorf("idempotent cleanup fixture: %v", err)
			}
		})
		check(t, Fixture{Service: fixture.Service, Contract: fixture.Contract, Scenario: fixture.Scenario.clone()})
	})
}

func (suite *Suite) seededReads(t *testing.T, fixture Fixture) {
	scenario := fixture.Scenario
	repositoryQuery, _ := repository.NewRepositoryQuery(scenario.PrimaryScope, scenario.Repository.RepositoryID())
	gotRepository, err := fixture.Service.GetRepository(context.Background(), repositoryQuery)
	if err != nil || gotRepository.RepositoryID() != scenario.Repository.RepositoryID() {
		t.Fatalf("get repository: %+v, %v", gotRepository, err)
	}
	repositoryList, _ := fixture.Contract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: scenario.PrimaryScope, PageSize: 10})
	repositoryPage, err := fixture.Service.ListRepositories(context.Background(), repositoryList)
	if err != nil || len(repositoryPage.Items()) == 0 {
		t.Fatalf("list repositories: %+v, %v", repositoryPage, err)
	}
	scanQuery, _ := repository.NewScanQuery(scenario.PrimaryScope, scenario.Repository.RepositoryID(), scenario.SucceededScan.ScanID())
	gotScan, err := fixture.Service.GetScan(context.Background(), scanQuery)
	if err != nil || gotScan.State() != repository.ScanSucceeded {
		t.Fatalf("get scan: %+v, %v", gotScan, err)
	}
	scanList, _ := fixture.Contract.NewScanListRequest(repository.ScanListParams{Scope: scenario.PrimaryScope, RepositoryID: scenario.Repository.RepositoryID(), PageSize: 10})
	scanPage, err := fixture.Service.ListScans(context.Background(), scanList)
	if err != nil || len(scanPage.Items()) < 2 {
		t.Fatalf("list scans: %+v, %v", scanPage, err)
	}
	artifactQuery, _ := repository.NewArtifactQuery(scenario.PrimaryScope, scenario.Repository.RepositoryID(), scenario.SucceededScan.ScanID(), scenario.Artifact.ArtifactID())
	gotArtifact, err := fixture.Service.GetArtifact(context.Background(), artifactQuery)
	if err != nil || gotArtifact.PayloadDigest() != scenario.Artifact.PayloadDigest() {
		t.Fatalf("get artifact: %+v, %v", gotArtifact, err)
	}
	artifactList, _ := fixture.Contract.NewArtifactListRequest(repository.ArtifactListParams{Scope: scenario.PrimaryScope, RepositoryID: scenario.Repository.RepositoryID(), ScanID: scenario.SucceededScan.ScanID(), PageSize: 10})
	artifactPage, err := fixture.Service.ListArtifacts(context.Background(), artifactList)
	if err != nil || len(artifactPage.Items()) != 1 {
		t.Fatalf("list artifacts: %+v, %v", artifactPage, err)
	}
	exportRequest, _ := repository.NewExportArtifactRequest(artifactQuery)
	var output bytes.Buffer
	receipt, err := fixture.Service.ExportArtifact(context.Background(), exportRequest, &output)
	if err != nil || !bytes.Equal(output.Bytes(), scenario.Payload) || receipt.PayloadDigest() != scenario.Artifact.PayloadDigest() || output.Len() > suite.config.MaxExportBytes {
		t.Fatalf("export: receipt=%+v bytes=%d err=%v", receipt, output.Len(), err)
	}
}

func (suite *Suite) scopeIsolation(t *testing.T, fixture Fixture) {
	scenario := fixture.Scenario
	repositoryQuery, _ := repository.NewRepositoryQuery(scenario.OtherScope, scenario.Repository.RepositoryID())
	if _, err := fixture.Service.GetRepository(context.Background(), repositoryQuery); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("repository scope escape: %v", err)
	}
	scanQuery, _ := repository.NewScanQuery(scenario.OtherScope, scenario.Repository.RepositoryID(), scenario.SucceededScan.ScanID())
	if _, err := fixture.Service.GetScan(context.Background(), scanQuery); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("scan scope escape: %v", err)
	}
	artifactQuery, _ := repository.NewArtifactQuery(scenario.OtherScope, scenario.Repository.RepositoryID(), scenario.SucceededScan.ScanID(), scenario.Artifact.ArtifactID())
	if _, err := fixture.Service.GetArtifact(context.Background(), artifactQuery); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("artifact scope escape: %v", err)
	}
	exportRequest, _ := repository.NewExportArtifactRequest(artifactQuery)
	if _, err := fixture.Service.ExportArtifact(context.Background(), exportRequest, io.Discard); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("export scope escape: %v", err)
	}
	repositoryList, _ := fixture.Contract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: scenario.OtherScope, PageSize: 10})
	page, err := fixture.Service.ListRepositories(context.Background(), repositoryList)
	if err != nil || len(page.Items()) != 0 {
		t.Fatalf("list scope escape: %+v, %v", page, err)
	}
	scanList, _ := fixture.Contract.NewScanListRequest(repository.ScanListParams{Scope: scenario.OtherScope, RepositoryID: scenario.Repository.RepositoryID(), PageSize: 10})
	scanPage, err := fixture.Service.ListScans(context.Background(), scanList)
	if err != nil || len(scanPage.Items()) != 0 {
		t.Fatalf("scan list scope escape: %+v, %v", scanPage, err)
	}
	artifactList, _ := fixture.Contract.NewArtifactListRequest(repository.ArtifactListParams{Scope: scenario.OtherScope, RepositoryID: scenario.Repository.RepositoryID(), ScanID: scenario.SucceededScan.ScanID(), PageSize: 10})
	artifactPage, err := fixture.Service.ListArtifacts(context.Background(), artifactList)
	if err != nil || len(artifactPage.Items()) != 0 {
		t.Fatalf("artifact list scope escape: %+v, %v", artifactPage, err)
	}
	archive, _ := repository.NewArchiveRepositoryRequest(repository.ArchiveRepositoryParams{Scope: scenario.OtherScope, RequestID: "scope-archive", RepositoryID: scenario.Repository.RepositoryID()})
	if _, err = fixture.Service.ArchiveRepository(context.Background(), archive); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("archive scope escape: %v", err)
	}
	cancel, _ := repository.NewCancelScanRequest(repository.CancelScanParams{Scope: scenario.OtherScope, RequestID: "scope-cancel", RepositoryID: scenario.Repository.RepositoryID(), ScanID: scenario.RunningScan.ScanID()})
	if _, err = fixture.Service.CancelScan(context.Background(), cancel); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("cancel scope escape: %v", err)
	}
	execute, _ := fixture.Contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scenario.OtherScope, RequestID: "scope-execute", RepositoryID: scenario.Repository.RepositoryID(), ScanID: "scope-scan", SourceHandle: scenario.SourceHandle, Profile: scenario.Profile})
	if _, err = fixture.Service.ExecuteScan(context.Background(), execute); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("execute scope escape: %v", err)
	}
	register, _ := fixture.Contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scenario.OtherScope, RequestID: "scope-register", RepositoryID: scenario.Repository.RepositoryID(), DisplayName: "Other Scope", SourceHandle: scenario.SourceHandle})
	created, err := fixture.Service.RegisterRepository(context.Background(), register)
	if err != nil || created.DisplayName() != "Other Scope" {
		t.Fatalf("independent scope registration: %+v, %v", created, err)
	}
	sameIDs, _ := fixture.Contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scenario.OtherScope, RequestID: "scope-same-ids", RepositoryID: scenario.Repository.RepositoryID(), ScanID: scenario.SucceededScan.ScanID(), SourceHandle: scenario.SourceHandle, Profile: scenario.Profile})
	otherResult, err := fixture.Service.ExecuteScan(context.Background(), sameIDs)
	if err != nil || len(otherResult.Artifacts()) != 1 || otherResult.Artifacts()[0].ArtifactID() != scenario.Artifact.ArtifactID() {
		t.Fatalf("same-ID scoped scan: %+v, %v", otherResult, err)
	}
	otherArtifactQuery, _ := repository.NewArtifactQuery(scenario.OtherScope, scenario.Repository.RepositoryID(), scenario.SucceededScan.ScanID(), scenario.Artifact.ArtifactID())
	otherExport, _ := repository.NewExportArtifactRequest(otherArtifactQuery)
	var otherPayload bytes.Buffer
	if _, err = fixture.Service.ExportArtifact(context.Background(), otherExport, &otherPayload); err != nil || otherPayload.String() != "{\"fake\":true}\n" {
		t.Fatalf("same-ID scoped export: %q, %v", otherPayload.String(), err)
	}
	primaryArtifactQuery, _ := repository.NewArtifactQuery(scenario.PrimaryScope, scenario.Repository.RepositoryID(), scenario.SucceededScan.ScanID(), scenario.Artifact.ArtifactID())
	primaryExport, _ := repository.NewExportArtifactRequest(primaryArtifactQuery)
	var primaryPayload bytes.Buffer
	if _, err = fixture.Service.ExportArtifact(context.Background(), primaryExport, &primaryPayload); err != nil || !bytes.Equal(primaryPayload.Bytes(), scenario.Payload) {
		t.Fatalf("same-ID primary export changed: %q, %v", primaryPayload.String(), err)
	}
	primaryQuery, _ := repository.NewRepositoryQuery(scenario.PrimaryScope, scenario.Repository.RepositoryID())
	primary, err := fixture.Service.GetRepository(context.Background(), primaryQuery)
	if err != nil || primary.DisplayName() != scenario.Repository.DisplayName() {
		t.Fatalf("cross-scope mutation: %+v, %v", primary, err)
	}
}

func (suite *Suite) repositoryMutations(t *testing.T, fixture Fixture) {
	scenario := fixture.Scenario
	request, err := fixture.Contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scenario.PrimaryScope, RequestID: "conformance-register", RepositoryID: "conformance-repository", DisplayName: "Conformance Repository", SourceHandle: scenario.SourceHandle})
	if err != nil {
		t.Fatal(err)
	}
	created, err := fixture.Service.RegisterRepository(context.Background(), request)
	if err != nil || created.State() != repository.RepositoryActive {
		t.Fatalf("register: %+v, %v", created, err)
	}
	retried, err := fixture.Service.RegisterRepository(context.Background(), request)
	if err != nil || retried.RepositoryID() != created.RepositoryID() {
		t.Fatalf("idempotent register: %+v, %v", retried, err)
	}
	conflict, _ := fixture.Contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scenario.PrimaryScope, RequestID: "conformance-register", RepositoryID: "different-repository", DisplayName: "Different", SourceHandle: scenario.SourceHandle})
	if _, err = fixture.Service.RegisterRepository(context.Background(), conflict); repository.KindOf(err) != repository.ErrorIdempotencyConflict {
		t.Fatalf("idempotency conflict: %v", err)
	}
	archive, _ := repository.NewArchiveRepositoryRequest(repository.ArchiveRepositoryParams{Scope: scenario.PrimaryScope, RequestID: "conformance-archive", RepositoryID: created.RepositoryID()})
	archived, err := fixture.Service.ArchiveRepository(context.Background(), archive)
	if err != nil || archived.State() != repository.RepositoryArchived {
		t.Fatalf("archive: %+v, %v", archived, err)
	}
}

func (suite *Suite) scanExecutionAndCancel(t *testing.T, fixture Fixture) {
	scenario := fixture.Scenario
	execute, err := fixture.Contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scenario.PrimaryScope, RequestID: "conformance-execute", RepositoryID: scenario.Repository.RepositoryID(), ScanID: "conformance-scan", SourceHandle: scenario.SourceHandle, Profile: scenario.Profile})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.Service.ExecuteScan(context.Background(), execute)
	if err != nil || result.Scan().State() != repository.ScanSucceeded || result.Disposition() != repository.DispositionCreated || len(result.Artifacts()) != 1 {
		t.Fatalf("execute: %+v, %v", result, err)
	}
	retried, err := fixture.Service.ExecuteScan(context.Background(), execute)
	if err != nil || retried.Disposition() != repository.DispositionAlreadyPresent {
		t.Fatalf("retry execute: %+v, %v", retried, err)
	}
	cancelRequest, _ := repository.NewCancelScanRequest(repository.CancelScanParams{Scope: scenario.PrimaryScope, RequestID: "conformance-cancel", RepositoryID: scenario.Repository.RepositoryID(), ScanID: scenario.RunningScan.ScanID()})
	canceled, err := fixture.Service.CancelScan(context.Background(), cancelRequest)
	if err != nil || canceled.State() != repository.ScanCanceled {
		t.Fatalf("cancel: %+v, %v", canceled, err)
	}
}

func (suite *Suite) contextCancellation(t *testing.T, fixture Fixture) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	query, _ := repository.NewRepositoryQuery(fixture.Scenario.PrimaryScope, fixture.Scenario.Repository.RepositoryID())
	if _, err := fixture.Service.GetRepository(ctx, query); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled operation: %v", err)
	}
}

// NewMemoryFactory returns the thread-safe Phase 4.0.2 fake adapter.
func NewMemoryFactory() Factory { return FactoryFunc(openMemoryFixture) }

// NewMemoryLifecycleFactory returns the lifecycle-only view of the
// thread-safe Phase 4.0.2 fake adapter.
func NewMemoryLifecycleFactory() LifecycleFactory {
	return LifecycleFactoryFunc(func(ctx context.Context) (LifecycleFixture, Cleanup, error) {
		fixture, cleanup, err := openMemoryFixture(ctx)
		if err != nil {
			return LifecycleFixture{}, nil, err
		}
		return LifecycleFixture{Service: fixture.Service, Contract: fixture.Contract, Scenario: LifecycleScenario{
			PrimaryScope: fixture.Scenario.PrimaryScope, OtherScope: fixture.Scenario.OtherScope,
			Repository: fixture.Scenario.Repository, SourceHandle: fixture.Scenario.SourceHandle,
		}}, cleanup, nil
	})
}

func openMemoryFixture(ctx context.Context) (Fixture, Cleanup, error) {
	if err := contextError(ctx, "open-fixture"); err != nil {
		return Fixture{}, nil, err
	}
	contract, err := repository.New()
	if err != nil {
		return Fixture{}, nil, err
	}
	primary, _ := repository.NewScope("scope-primary", "principal-primary")
	other, _ := repository.NewScope("scope-other", "principal-other")
	profile := contract.Profiles().Definitions()[0].Profile()
	now := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	fingerprint := repository.DigestBytes([]byte("source-proof"))
	repositoryValue, _ := repository.NewRepository(repository.RepositoryParams{RepositoryID: "repository-seeded", DisplayName: "Seeded Repository", SourceKind: "local", FingerprintScheme: "sha256/v1", Fingerprint: fingerprint, State: repository.RepositoryActive, CurrentScanID: "scan-succeeded", CreatedAt: now, UpdatedAt: now})
	succeeded, _ := repository.NewScan(repository.ScanParams{RepositoryID: repositoryValue.RepositoryID(), ScanID: "scan-succeeded", Profile: profile, SourceRevision: "revision-1", State: repository.ScanSucceeded, RequestedAt: now, StartedAt: now, FinishedAt: now})
	running, _ := repository.NewScan(repository.ScanParams{RepositoryID: repositoryValue.RepositoryID(), ScanID: "scan-running", Profile: profile, SourceRevision: "revision-1", State: repository.ScanRunning, RequestedAt: now, StartedAt: now})
	payload := []byte("{\"seeded\":true}\n")
	digest := repository.DigestBytes(payload)
	artifactID, _ := repository.NewArtifactID(repositoryValue.RepositoryID(), succeeded.ScanID(), "repository-intelligence-summary", "1.0.0", repository.ArtifactIdentityScheme)
	artifact, _ := repository.NewArtifact(repository.ArtifactParams{ArtifactID: artifactID, ScanID: succeeded.ScanID(), Name: "repository-intelligence-summary", Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "0.1.0", MediaType: "application/json", PayloadDigest: digest, PayloadSize: uint64(len(payload)), ProducerName: "repository-summary", ProducerVersion: "1.0.0", CreatedAt: now})
	service := &memoryService{contract: contract, now: now, repositories: make(map[string]repository.Repository), scans: make(map[string]repository.Scan), artifacts: make(map[string]repository.Artifact), payloads: make(map[string][]byte), requests: make(map[string]string)}
	service.repositories[repositoryKey(primary, repositoryValue.RepositoryID())] = repositoryValue
	service.scans[scanKey(primary, repositoryValue.RepositoryID(), succeeded.ScanID())] = succeeded
	service.scans[scanKey(primary, repositoryValue.RepositoryID(), running.ScanID())] = running
	service.artifacts[artifactKey(primary, repositoryValue.RepositoryID(), succeeded.ScanID(), artifact.ArtifactID())] = artifact
	service.payloads[artifactKey(primary, repositoryValue.RepositoryID(), succeeded.ScanID(), artifact.ArtifactID())] = append([]byte(nil), payload...)
	var once sync.Once
	cleanup := func(context.Context) error { once.Do(func() { service.close() }); return nil }
	return Fixture{Service: service, Contract: contract, Scenario: Scenario{PrimaryScope: primary, OtherScope: other, Repository: repositoryValue, SucceededScan: succeeded, RunningScan: running, Artifact: artifact, Payload: payload, SourceHandle: "local-conformance-source", Profile: profile}}, cleanup, nil
}

type memoryService struct {
	mu           sync.RWMutex
	contract     *repository.Contract
	now          time.Time
	repositories map[string]repository.Repository
	scans        map[string]repository.Scan
	artifacts    map[string]repository.Artifact
	payloads     map[string][]byte
	requests     map[string]string
	closed       bool
}

func (service *memoryService) RegisterRepository(ctx context.Context, request repository.RegisterRepositoryRequest) (repository.Repository, error) {
	if err := contextError(ctx, "register-repository"); err != nil {
		return repository.Repository{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	fingerprint := fmt.Sprintf("%s|%s|%s", request.RepositoryID(), request.DisplayName(), request.SourceHandle().Reveal())
	if result, handled, err := service.idempotent(request.Scope(), request.RequestID(), fingerprint, repositoryKey(request.Scope(), request.RepositoryID())); handled {
		return result, err
	}
	value, err := repository.NewRepository(repository.RepositoryParams{RepositoryID: request.RepositoryID(), DisplayName: request.DisplayName(), SourceKind: "local", FingerprintScheme: "sha256/v1", Fingerprint: repository.DigestBytes([]byte(request.SourceHandle().Reveal())), State: repository.RepositoryActive, CreatedAt: service.now, UpdatedAt: service.now})
	if err != nil {
		return repository.Repository{}, err
	}
	service.repositories[repositoryKey(request.Scope(), request.RepositoryID())] = value
	service.requests[requestKey(request.Scope(), request.RequestID())] = fingerprint
	return value, nil
}

func (service *memoryService) idempotent(scope repository.Scope, requestID repository.RequestID, fingerprint, key string) (repository.Repository, bool, error) {
	previous, exists := service.requests[requestKey(scope, requestID)]
	if !exists {
		return repository.Repository{}, false, nil
	}
	if previous != fingerprint {
		return repository.Repository{}, true, repository.NewError(repository.ErrorIdempotencyConflict, "register-repository", "request-reused", false, nil)
	}
	value, ok := service.repositories[key]
	if !ok {
		return repository.Repository{}, true, repository.NewError(repository.ErrorConflict, "register-repository", "missing-idempotent-result", false, nil)
	}
	return value, true, nil
}

func (service *memoryService) GetRepository(ctx context.Context, query repository.RepositoryQuery) (repository.Repository, error) {
	if err := contextError(ctx, "get-repository"); err != nil {
		return repository.Repository{}, err
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	value, ok := service.repositories[repositoryKey(query.Scope(), query.RepositoryID())]
	if !ok {
		return repository.Repository{}, notFound("get-repository")
	}
	return value, nil
}

func (service *memoryService) ListRepositories(ctx context.Context, request repository.RepositoryListRequest) (repository.RepositoryPage, error) {
	if err := contextError(ctx, "list-repositories"); err != nil {
		return repository.RepositoryPage{}, err
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	values := []repository.Repository{}
	prefix := scopeKey(request.Scope()) + "|"
	for key, value := range service.repositories {
		if strings.HasPrefix(key, prefix) {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].RepositoryID() < values[j].RepositoryID() })
	values, next := pageRepositories(values, request.PageSize(), request.Cursor())
	return repository.NewRepositoryPage(values, next)
}

func (service *memoryService) ArchiveRepository(ctx context.Context, request repository.ArchiveRepositoryRequest) (repository.Repository, error) {
	if err := contextError(ctx, "archive-repository"); err != nil {
		return repository.Repository{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	fingerprint := "archive|" + string(request.RepositoryID())
	requestKeyValue := requestKey(request.Scope(), request.RequestID())
	if previous, exists := service.requests[requestKeyValue]; exists && previous != fingerprint {
		return repository.Repository{}, repository.NewError(repository.ErrorIdempotencyConflict, "archive-repository", "request-reused", false, nil)
	}
	key := repositoryKey(request.Scope(), request.RepositoryID())
	current, ok := service.repositories[key]
	if !ok {
		return repository.Repository{}, notFound("archive-repository")
	}
	value, _ := repository.NewRepository(repository.RepositoryParams{RepositoryID: current.RepositoryID(), DisplayName: current.DisplayName(), SourceKind: current.SourceKind(), FingerprintScheme: current.FingerprintScheme(), Fingerprint: current.Fingerprint(), State: repository.RepositoryArchived, CurrentScanID: current.CurrentScanID(), CreatedAt: current.CreatedAt(), UpdatedAt: service.now.Add(time.Second)})
	service.repositories[key] = value
	service.requests[requestKeyValue] = fingerprint
	return value, nil
}

func (service *memoryService) ExecuteScan(ctx context.Context, request repository.ExecuteScanRequest) (repository.ScanResult, error) {
	if err := contextError(ctx, "execute-scan"); err != nil {
		return repository.ScanResult{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, ok := service.repositories[repositoryKey(request.Scope(), request.RepositoryID())]; !ok {
		return repository.ScanResult{}, notFound("execute-scan")
	}
	key := scanKey(request.Scope(), request.RepositoryID(), request.ScanID())
	if existing, ok := service.scans[key]; ok {
		artifacts := service.artifactsFor(request.Scope(), request.RepositoryID(), request.ScanID())
		return repository.NewScanResult(existing, artifacts, repository.DispositionAlreadyPresent)
	}
	scan, _ := repository.NewScan(repository.ScanParams{RepositoryID: request.RepositoryID(), ScanID: request.ScanID(), Profile: request.Profile(), SourceRevision: "fake-revision", State: repository.ScanSucceeded, RequestedAt: service.now, StartedAt: service.now, FinishedAt: service.now})
	payload := []byte("{\"fake\":true}\n")
	digest := repository.DigestBytes(payload)
	artifactID, _ := repository.NewArtifactID(request.RepositoryID(), request.ScanID(), "repository-intelligence-summary", "1.0.0", repository.ArtifactIdentityScheme)
	artifact, _ := repository.NewArtifact(repository.ArtifactParams{ArtifactID: artifactID, ScanID: request.ScanID(), Name: "repository-intelligence-summary", Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "0.1.0", MediaType: "application/json", PayloadDigest: digest, PayloadSize: uint64(len(payload)), ProducerName: "repository-summary", ProducerVersion: "1.0.0", CreatedAt: service.now})
	service.scans[key] = scan
	service.artifacts[artifactKey(request.Scope(), request.RepositoryID(), request.ScanID(), artifactID)] = artifact
	service.payloads[artifactKey(request.Scope(), request.RepositoryID(), request.ScanID(), artifactID)] = append([]byte(nil), payload...)
	return repository.NewScanResult(scan, []repository.Artifact{artifact}, repository.DispositionCreated)
}

func (service *memoryService) GetScan(ctx context.Context, query repository.ScanQuery) (repository.Scan, error) {
	if err := contextError(ctx, "get-scan"); err != nil {
		return repository.Scan{}, err
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	value, ok := service.scans[scanKey(query.Scope(), query.RepositoryID(), query.ScanID())]
	if !ok {
		return repository.Scan{}, notFound("get-scan")
	}
	return value, nil
}

func (service *memoryService) ListScans(ctx context.Context, request repository.ScanListRequest) (repository.ScanPage, error) {
	if err := contextError(ctx, "list-scans"); err != nil {
		return repository.ScanPage{}, err
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	values := []repository.Scan{}
	prefix := scopeKey(request.Scope()) + "|" + string(request.RepositoryID()) + "|"
	for key, value := range service.scans {
		if strings.HasPrefix(key, prefix) {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ScanID() < values[j].ScanID() })
	values, next := pageScans(values, request.PageSize(), request.Cursor())
	return repository.NewScanPage(values, next)
}

func (service *memoryService) CancelScan(ctx context.Context, request repository.CancelScanRequest) (repository.Scan, error) {
	if err := contextError(ctx, "cancel-scan"); err != nil {
		return repository.Scan{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	key := scanKey(request.Scope(), request.RepositoryID(), request.ScanID())
	current, ok := service.scans[key]
	if !ok {
		return repository.Scan{}, notFound("cancel-scan")
	}
	if current.State() == repository.ScanSucceeded {
		return repository.Scan{}, repository.NewError(repository.ErrorConflict, "cancel-scan", "already-published", false, nil)
	}
	value, _ := repository.NewScan(repository.ScanParams{RepositoryID: current.RepositoryID(), ScanID: current.ScanID(), Profile: current.Profile(), SourceRevision: current.SourceRevision(), State: repository.ScanCanceled, ReasonCode: "caller-canceled", RequestedAt: current.RequestedAt(), StartedAt: current.StartedAt(), FinishedAt: service.now})
	service.scans[key] = value
	return value, nil
}

func (service *memoryService) GetArtifact(ctx context.Context, query repository.ArtifactQuery) (repository.Artifact, error) {
	if err := contextError(ctx, "get-artifact"); err != nil {
		return repository.Artifact{}, err
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	value, ok := service.artifacts[artifactKey(query.Scope(), query.RepositoryID(), query.ScanID(), query.ArtifactID())]
	if !ok {
		return repository.Artifact{}, notFound("get-artifact")
	}
	return value, nil
}

func (service *memoryService) ListArtifacts(ctx context.Context, request repository.ArtifactListRequest) (repository.ArtifactPage, error) {
	if err := contextError(ctx, "list-artifacts"); err != nil {
		return repository.ArtifactPage{}, err
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	values := service.artifactsFor(request.Scope(), request.RepositoryID(), request.ScanID())
	values, next := pageArtifacts(values, request.PageSize(), request.Cursor())
	return repository.NewArtifactPage(values, next)
}

func (service *memoryService) ExportArtifact(ctx context.Context, request repository.ExportArtifactRequest, writer io.Writer) (repository.ExportReceipt, error) {
	if err := contextError(ctx, "export-artifact"); err != nil {
		return repository.ExportReceipt{}, err
	}
	if writer == nil {
		return repository.ExportReceipt{}, repository.NewError(repository.ErrorInvalidInput, "export-artifact", "writer-required", false, nil)
	}
	query := request.Query()
	service.mu.RLock()
	artifact, ok := service.artifacts[artifactKey(query.Scope(), query.RepositoryID(), query.ScanID(), query.ArtifactID())]
	payload := append([]byte(nil), service.payloads[artifactKey(query.Scope(), query.RepositoryID(), query.ScanID(), query.ArtifactID())]...)
	service.mu.RUnlock()
	if !ok {
		return repository.ExportReceipt{}, notFound("export-artifact")
	}
	if _, err := writer.Write(payload); err != nil {
		return repository.ExportReceipt{}, repository.NewError(repository.ErrorInternal, "export-artifact", "write-failed", false, err)
	}
	return repository.NewExportReceipt(artifact.PayloadDigest(), uint64(len(payload)))
}

func (service *memoryService) artifactsFor(scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) []repository.Artifact {
	values := []repository.Artifact{}
	prefix := scopeKey(scope) + "|" + string(repositoryID) + "|" + string(scanID) + "|"
	for key, value := range service.artifacts {
		if strings.HasPrefix(key, prefix) {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ArtifactID() < values[j].ArtifactID() })
	return values
}

func (service *memoryService) close() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.closed = true
	clear(service.repositories)
	clear(service.scans)
	clear(service.artifacts)
	clear(service.payloads)
	clear(service.requests)
}

func contextError(ctx context.Context, operation string) error {
	if ctx == nil {
		return repository.NewError(repository.ErrorInvalidInput, operation, "context-required", false, nil)
	}
	if err := ctx.Err(); err != nil {
		return repository.NewError(repository.ErrorInternal, operation, "context-ended", false, err)
	}
	return nil
}
func notFound(operation string) error {
	return repository.NewError(repository.ErrorNotFound, operation, "not-found", false, nil)
}
func scopeKey(scope repository.Scope) string { return string(scope.ScopeID()) }
func repositoryKey(scope repository.Scope, repositoryID repository.RepositoryID) string {
	return scopeKey(scope) + "|" + string(repositoryID)
}
func scanKey(scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) string {
	return repositoryKey(scope, repositoryID) + "|" + string(scanID)
}
func artifactKey(scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, artifactID repository.ArtifactID) string {
	return scanKey(scope, repositoryID, scanID) + "|" + string(artifactID)
}
func requestKey(scope repository.Scope, requestID repository.RequestID) string {
	return scopeKey(scope) + "|" + string(requestID)
}

func pageStart(cursor repository.Cursor, length int) int {
	if cursor == "" {
		return 0
	}
	value, err := strconv.Atoi(string(cursor))
	if err != nil || value < 0 || value > length {
		return length
	}
	return value
}
func nextCursor(end, length int) repository.Cursor {
	if end >= length {
		return ""
	}
	return repository.Cursor(strconv.Itoa(end))
}
func pageRepositories(values []repository.Repository, size int, cursor repository.Cursor) ([]repository.Repository, repository.Cursor) {
	start := pageStart(cursor, len(values))
	end := min(start+size, len(values))
	return append([]repository.Repository(nil), values[start:end]...), nextCursor(end, len(values))
}
func pageScans(values []repository.Scan, size int, cursor repository.Cursor) ([]repository.Scan, repository.Cursor) {
	start := pageStart(cursor, len(values))
	end := min(start+size, len(values))
	return append([]repository.Scan(nil), values[start:end]...), nextCursor(end, len(values))
}
func pageArtifacts(values []repository.Artifact, size int, cursor repository.Cursor) ([]repository.Artifact, repository.Cursor) {
	start := pageStart(cursor, len(values))
	end := min(start+size, len(values))
	return append([]repository.Artifact(nil), values[start:end]...), nextCursor(end, len(values))
}
