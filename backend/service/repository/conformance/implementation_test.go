package conformance

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

func TestMemoryAdapterPassesConformance(t *testing.T) { Run(t, NewMemoryFactory()) }

func TestMemoryAdapterPassesLifecycleConformance(t *testing.T) {
	RunLifecycle(t, NewMemoryLifecycleFactory())
}

func TestMemoryAdapterPassesScanConformance(t *testing.T) {
	RunScan(t, NewMemoryScanFactory())
}

func TestMemoryScanFactoryCancellationAndEdges(t *testing.T) {
	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := NewMemoryScanFactory().OpenScan(canceledContext); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled scan fixture: %v", err)
	}
	fixture, cleanup, err := NewMemoryScanFactory().OpenScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup(context.Background()) }()
	scenario := fixture.Scenario

	cancelSucceeded, _ := repository.NewCancelScanRequest(repository.CancelScanParams{Scope: scenario.PrimaryScope, RequestID: "cancel-succeeded", RepositoryID: scenario.RepositoryID, ScanID: scenario.SucceededScan.ScanID()})
	if _, err = fixture.Service.CancelScan(context.Background(), cancelSucceeded); repository.KindOf(err) != repository.ErrorConflict {
		t.Fatalf("cancel succeeded: %v", err)
	}
	missingExecution, _ := fixture.Contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scenario.PrimaryScope, RequestID: "missing-execute", RepositoryID: "missing-repository", ScanID: "missing-scan", SourceHandle: scenario.SourceHandle, Profile: scenario.Profile})
	if _, err = fixture.Service.ExecuteScan(context.Background(), missingExecution); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("missing repository execute: %v", err)
	}

	scanList, _ := fixture.Contract.NewScanListRequest(repository.ScanListParams{Scope: scenario.PrimaryScope, RepositoryID: scenario.RepositoryID, PageSize: 1, Cursor: "invalid"})
	if page, err := fixture.Service.ListScans(context.Background(), scanList); err != nil || len(page.Items()) != 0 {
		t.Fatalf("invalid scan cursor: %+v, %v", page, err)
	}
	artifactList, _ := fixture.Contract.NewArtifactListRequest(repository.ArtifactListParams{Scope: scenario.PrimaryScope, RepositoryID: scenario.RepositoryID, ScanID: scenario.SucceededScan.ScanID(), PageSize: 1, Cursor: "invalid"})
	if page, err := fixture.Service.ListArtifacts(context.Background(), artifactList); err != nil || len(page.Items()) != 0 {
		t.Fatalf("invalid artifact cursor: %+v, %v", page, err)
	}

	ctx, stop := context.WithCancel(context.Background())
	stop()
	if _, err = fixture.Service.ListScans(ctx, scanList); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled scan list: %v", err)
	}
	cancelRunning, _ := repository.NewCancelScanRequest(repository.CancelScanParams{Scope: scenario.PrimaryScope, RequestID: "cancel-canceled", RepositoryID: scenario.RepositoryID, ScanID: scenario.RunningScan.ScanID()})
	if _, err = fixture.Service.CancelScan(ctx, cancelRunning); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled cancel: %v", err)
	}
	artifactQuery, _ := repository.NewArtifactQuery(scenario.PrimaryScope, scenario.RepositoryID, scenario.SucceededScan.ScanID(), scenario.Artifact.ArtifactID())
	export, _ := repository.NewExportArtifactRequest(artifactQuery)
	if _, err = fixture.Service.ExportArtifact(ctx, export, io.Discard); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled export: %v", err)
	}
	if _, err = fixture.Service.ExportArtifact(context.Background(), export, failingWriter{}); repository.KindOf(err) != repository.ErrorInternal {
		t.Fatalf("failed writer: %v", err)
	}
}

func TestMemoryLifecycleCancellationEdges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := NewMemoryLifecycleFactory().OpenLifecycle(ctx); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled lifecycle factory: %v", err)
	}
	fixture, cleanup, err := NewMemoryLifecycleFactory().OpenLifecycle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup(context.Background()) }()
	scenario := fixture.Scenario
	register, _ := fixture.Contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scenario.PrimaryScope, RequestID: "canceled-register", RepositoryID: "canceled-repository", DisplayName: "Canceled", SourceHandle: scenario.SourceHandle})
	if _, err = fixture.Service.RegisterRepository(ctx, register); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled register: %v", err)
	}
	list, _ := fixture.Contract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: scenario.PrimaryScope, PageSize: 10})
	if _, err = fixture.Service.ListRepositories(ctx, list); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled list: %v", err)
	}
	archive, _ := repository.NewArchiveRepositoryRequest(repository.ArchiveRepositoryParams{Scope: scenario.PrimaryScope, RequestID: "canceled-archive", RepositoryID: scenario.Repository.RepositoryID()})
	if _, err = fixture.Service.ArchiveRepository(ctx, archive); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled archive: %v", err)
	}
}

func TestMemoryScanAndArtifactCancellationEdges(t *testing.T) {
	fixture, cleanup, err := NewMemoryFactory().Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup(context.Background()) }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scenario := fixture.Scenario
	scanQuery, _ := repository.NewScanQuery(scenario.PrimaryScope, scenario.Repository.RepositoryID(), scenario.SucceededScan.ScanID())
	if _, err = fixture.Service.GetScan(ctx, scanQuery); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled get scan: %v", err)
	}
	artifactQuery, _ := repository.NewArtifactQuery(scenario.PrimaryScope, scenario.Repository.RepositoryID(), scenario.SucceededScan.ScanID(), scenario.Artifact.ArtifactID())
	if _, err = fixture.Service.GetArtifact(ctx, artifactQuery); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled get artifact: %v", err)
	}
	list, _ := fixture.Contract.NewArtifactListRequest(repository.ArtifactListParams{Scope: scenario.PrimaryScope, RepositoryID: scenario.Repository.RepositoryID(), ScanID: scenario.SucceededScan.ScanID(), PageSize: 10})
	if _, err = fixture.Service.ListArtifacts(ctx, list); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled list artifacts: %v", err)
	}
}

func TestConfigurationAndFactoryValidation(t *testing.T) {
	if _, err := New(DefaultConfig(), DefaultConfig()); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("multiple configs: %v", err)
	}
	if _, err := New(Config{MaxExportBytes: defaultMaxExportBytes + 1}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("invalid limit: %v", err)
	}
	if _, err := New(Config{MaxExportBytes: 1}); err != nil {
		t.Fatalf("partial config: %v", err)
	}
	if _, err := New(Config{}); err != nil {
		t.Fatalf("zero config defaults: %v", err)
	}
	if _, _, err := NewMemoryFactory().Open(nil); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("nil context: %v", err)
	}
}

func TestMemoryAdapterMissingIdempotentResult(t *testing.T) {
	fixture, cleanup, err := NewMemoryFactory().Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup(context.Background()) }()
	service := fixture.Service.(*memoryService)
	request, _ := fixture.Contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: fixture.Scenario.PrimaryScope, RequestID: "missing-result-request", RepositoryID: "missing-result-repository", DisplayName: "Missing Result", SourceHandle: fixture.Scenario.SourceHandle})
	if _, err = service.RegisterRepository(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	delete(service.repositories, repositoryKey(request.Scope(), request.RepositoryID()))
	service.mu.Unlock()
	if _, err = service.RegisterRepository(context.Background(), request); repository.KindOf(err) != repository.ErrorConflict {
		t.Fatalf("missing idempotent result: %v", err)
	}
}

func TestMemoryAdapterConcurrentReadsAndCleanup(t *testing.T) {
	fixture, cleanup, err := NewMemoryFactory().Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	query, _ := repository.NewRepositoryQuery(fixture.Scenario.PrimaryScope, fixture.Scenario.Repository.RepositoryID())
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value, readErr := fixture.Service.GetRepository(context.Background(), query)
			if readErr != nil || value.RepositoryID() == "" {
				t.Errorf("read=%+v err=%v", value, readErr)
			}
		}()
	}
	wait.Wait()
	if err := cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryAdapterAdditionalContractEdges(t *testing.T) {
	fixture, cleanup, err := NewMemoryFactory().Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup(context.Background()) }()

	other := fixture.Scenario.OtherScope
	repositoryID := fixture.Scenario.Repository.RepositoryID()
	scanID := fixture.Scenario.SucceededScan.ScanID()
	artifactID := fixture.Scenario.Artifact.ArtifactID()
	repositoryQuery, _ := repository.NewRepositoryQuery(other, repositoryID)
	if _, err = fixture.Service.GetRepository(context.Background(), repositoryQuery); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("cross-scope repository read: %v", err)
	}
	scanQuery, _ := repository.NewScanQuery(other, repositoryID, scanID)
	if _, err = fixture.Service.GetScan(context.Background(), scanQuery); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("cross-scope scan read: %v", err)
	}
	artifactQuery, _ := repository.NewArtifactQuery(other, repositoryID, scanID, artifactID)
	if _, err = fixture.Service.GetArtifact(context.Background(), artifactQuery); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("cross-scope artifact read: %v", err)
	}

	primary := fixture.Scenario.PrimaryScope
	listRepositories, _ := fixture.Contract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: primary, PageSize: 1, Cursor: "invalid"})
	page, err := fixture.Service.ListRepositories(context.Background(), listRepositories)
	if err != nil || len(page.Items()) != 0 {
		t.Fatalf("invalid cursor page=%+v err=%v", page, err)
	}
	listScans, _ := fixture.Contract.NewScanListRequest(repository.ScanListParams{Scope: primary, RepositoryID: repositoryID, PageSize: 1})
	firstScans, err := fixture.Service.ListScans(context.Background(), listScans)
	if err != nil || len(firstScans.Items()) != 1 || firstScans.NextCursor() == "" {
		t.Fatalf("first scan page=%+v err=%v", firstScans, err)
	}
	listArtifacts, _ := fixture.Contract.NewArtifactListRequest(repository.ArtifactListParams{Scope: other, RepositoryID: repositoryID, ScanID: scanID, PageSize: 10})
	artifacts, err := fixture.Service.ListArtifacts(context.Background(), listArtifacts)
	if err != nil || len(artifacts.Items()) != 0 {
		t.Fatalf("cross-scope artifacts=%+v err=%v", artifacts, err)
	}

	export, _ := repository.NewExportArtifactRequest(mustArtifactQuery(t, primary, repositoryID, scanID, artifactID))
	if _, err = fixture.Service.ExportArtifact(context.Background(), export, nil); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("nil writer: %v", err)
	}
	if _, err = fixture.Service.ExportArtifact(context.Background(), export, failingWriter{}); repository.KindOf(err) != repository.ErrorInternal {
		t.Fatalf("writer failure: %v", err)
	}
	var output bytes.Buffer
	receipt, err := fixture.Service.ExportArtifact(context.Background(), export, &output)
	if err != nil || !bytes.Equal(output.Bytes(), fixture.Scenario.Payload) || receipt.PayloadDigest() != fixture.Scenario.Artifact.PayloadDigest() {
		t.Fatalf("export receipt=%+v err=%v", receipt, err)
	}

	cancelSucceeded, _ := repository.NewCancelScanRequest(repository.CancelScanParams{Scope: primary, RequestID: "cancel-succeeded", RepositoryID: repositoryID, ScanID: scanID})
	if _, err = fixture.Service.CancelScan(context.Background(), cancelSucceeded); repository.KindOf(err) != repository.ErrorConflict {
		t.Fatalf("cancel succeeded scan: %v", err)
	}
	missingExecute, _ := fixture.Contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: primary, RequestID: "execute-missing", RepositoryID: "missing-repository", ScanID: "missing-scan", SourceHandle: fixture.Scenario.SourceHandle, Profile: fixture.Scenario.Profile})
	if _, err = fixture.Service.ExecuteScan(context.Background(), missingExecute); repository.KindOf(err) != repository.ErrorNotFound {
		t.Fatalf("execute missing repository: %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func mustArtifactQuery(t *testing.T, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, artifactID repository.ArtifactID) repository.ArtifactQuery {
	t.Helper()
	query, err := repository.NewArtifactQuery(scope, repositoryID, scanID, artifactID)
	if err != nil {
		t.Fatal(err)
	}
	return query
}
