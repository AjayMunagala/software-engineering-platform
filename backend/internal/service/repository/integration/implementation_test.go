package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	runtimeapp "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/app"
	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	runtimepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
	serviceadapters "github.com/AjayMunagala/software-engineering-platform/backend/internal/service/repository/adapters"
	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/lifecycle"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/scan"
)

const (
	goldenScopeID = "00000000-0000-4000-8000-000000000001"
	goldenRepoID  = "11111111-1111-4111-8111-111111111111"
	goldenScanID  = "22222222-2222-4222-8222-222222222222"
)

func TestFrozenPhysicalArtifactAndManifestVectors(t *testing.T) {
	profile, err := repository.NewAnalysisProfile("repository-go", "1", mustDigest(t, "63f2a1acd4bc3de83af9859d3308c7b62eae6b7b3e263581fcb0864a12296ba7"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	value, err := repository.NewScan(repository.ScanParams{RepositoryID: goldenRepoID, ScanID: goldenScanID, Profile: profile, SourceRevision: "commit:0123456789abcdef", State: repository.ScanSucceeded, RequestedAt: now, StartedAt: now, FinishedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	discoveryID := repository.ArtifactID("rsaid1_19546f40503fdddd85481edb5cf47f7189874a252c63bddbb6b39e8c9b032886")
	snapshotID := repository.ArtifactID("rsaid1_26ddb640b08b957c57512820bc99a18b2bd3b7f168edf5e6b95a77e12c2573b9")
	discovery := goldenArtifact(t, discoveryID, "discovery-inventory", "discovery", "0.1.1", []byte(`{"artifact":"discovery"}`), now)
	snapshot := goldenArtifact(t, snapshotID, "repository-snapshot", "ignore", "0.2.1", []byte(`{"artifact":"snapshot"}`), now)
	dependency, _ := scan.NewArtifactDependency("discovery-inventory", "1.0.0", 0)
	manifest, err := canonicalManifest(value, []manifestArtifact{testManifestArtifact{metadata: discovery}, testManifestArtifact{metadata: snapshot, dependencies: []scan.ArtifactDependency{dependency}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest) != 818 {
		t.Fatalf("manifest length=%d", len(manifest))
	}
	if digest := sha256.Sum256(manifest); strings.ToLower(repository.Digest(digest).String()) != "888b4f65c1a34881d2b762a474a65ada819d6064fd99d30f1958f6872c133598" {
		t.Fatalf("manifest digest=%x", digest)
	}
	physical, err := PhysicalArtifactID(discoveryID)
	if err != nil || physical != "628b097f-84cb-87cf-a81c-5627222e948c" {
		t.Fatalf("physical=%s err=%v", physical, err)
	}
	physical, err = PhysicalArtifactID(snapshotID)
	if err != nil || physical != "30594582-1bb4-8bef-bb5b-2aa43b338a9e" {
		t.Fatalf("physical=%s err=%v", physical, err)
	}
	changed, _ := PhysicalArtifactID(discoveryID + "a")
	if changed == "628b097f-84cb-87cf-a81c-5627222e948c" {
		t.Fatal("physical mapping ignored input change")
	}
	if _, err := canonicalManifest(value, []manifestArtifact{testManifestArtifact{metadata: snapshot, dependencies: []scan.ArtifactDependency{dependency}}, testManifestArtifact{metadata: discovery}}); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("reordered manifest=%v", err)
	}
}

func TestSourceProofNeverReadsRootAndCloses(t *testing.T) {
	scope, _ := repository.NewScope(goldenScopeID, "principal")
	handle, _ := repository.NewSourceHandle("opaque", 32)
	source := &fakeSource{fingerprint: repository.DigestBytes([]byte("source")), revision: "revision"}
	adapter := &SourceProofAdapter{resolver: fakeResolver{source: source}}
	resolution, err := adapter.Resolve(context.Background(), scope, handle)
	if err != nil || resolution.Proof().Fingerprint() != source.fingerprint || source.rootReads != 0 {
		t.Fatalf("resolution=%+v err=%v rootReads=%d", resolution, err, source.rootReads)
	}
	if err := resolution.Close(context.Background()); err != nil || source.closes != 1 {
		t.Fatalf("close err=%v count=%d", err, source.closes)
	}
}

func TestConfigurationIdentityAndSafeErrors(t *testing.T) {
	if DefaultConfig().Validate() != nil || (Config{ReadPageSize: 1001}).withDefaults().Validate() == nil {
		t.Fatal("configuration validation")
	}
	scope, _ := repository.NewScope(goldenScopeID, "principal")
	persisted, err := toPersistenceScope(scope)
	if err != nil || persisted.ScopeID() != goldenScopeID || persisted.PrincipalID() != "principal" {
		t.Fatalf("scope=%+v err=%v", persisted, err)
	}
	raw := persistence.NewError(persistence.ErrorUnavailable, "read", true, errors.New("password=secret SQLSTATE 08006"))
	mapped := serviceFailure(raw, "get-artifact", "persistence-contract-failed")
	if repository.KindOf(mapped) != repository.ErrorPersistenceUnavailable || strings.Contains(mapped.Error(), "secret") || !repository.IsRetryable(mapped) {
		t.Fatalf("mapped=%v", mapped)
	}
	if _, err := PhysicalArtifactID(""); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("empty physical id=%v", err)
	}
}

func TestBundleEndToEndWithNeutralPersistence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/service\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC)
	port := newMemoryPersistence(now)
	runtime := &fakeRuntime{port: port}
	resolver := reusableResolver{root: root, fingerprint: repository.DigestBytes([]byte("source-proof")), revision: "commit:0123456789abcdef"}
	persistenceContract, _ := persistence.New()
	serviceContract, _ := repository.New()
	bundle, err := New(Dependencies{Runtime: runtime, Persistence: persistenceContract, ServiceContract: serviceContract, SourceResolver: resolver, Clock: fixedClock{now}})
	if err != nil {
		t.Fatal(err)
	}
	scope, _ := repository.NewScope(goldenScopeID, "principal")
	register, _ := serviceContract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: "register", RepositoryID: goldenRepoID, DisplayName: "Example", SourceHandle: "source"})
	registered, err := bundle.Service().RegisterRepository(context.Background(), register)
	if err != nil || registered.RepositoryID() != goldenRepoID {
		t.Fatalf("register=%+v err=%v", registered, err)
	}
	query, _ := repository.NewRepositoryQuery(scope, goldenRepoID)
	if _, err := bundle.Service().GetRepository(context.Background(), query); err != nil {
		t.Fatal(err)
	}
	listRequest, _ := serviceContract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: scope, PageSize: 10})
	if page, err := bundle.Service().ListRepositories(context.Background(), listRequest); err != nil || len(page.Items()) != 1 {
		t.Fatalf("repositories=%+v err=%v", page, err)
	}
	execute, _ := serviceContract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: "execute", RepositoryID: goldenRepoID, ScanID: goldenScanID, SourceHandle: "source", Profile: repository.DefaultRepositoryGoProfile().Profile()})
	result, err := bundle.Service().ExecuteScan(context.Background(), execute)
	if err != nil || result.Scan().State() != repository.ScanSucceeded || len(result.Artifacts()) != 10 || runtime.admitted != 1 || port.staged != 10 {
		t.Fatalf("result artifacts=%d admitted=%d staged=%d err=%v", len(result.Artifacts()), runtime.admitted, port.staged, err)
	}
	retried, err := bundle.Service().ExecuteScan(context.Background(), execute)
	if err != nil || retried.Disposition() != repository.DispositionAlreadyPresent || runtime.admitted != 2 {
		t.Fatalf("retry=%+v admitted=%d err=%v", retried, runtime.admitted, err)
	}
	scanQuery, _ := repository.NewScanQuery(scope, goldenRepoID, goldenScanID)
	if value, err := bundle.Service().GetScan(context.Background(), scanQuery); err != nil || value.State() != repository.ScanSucceeded {
		t.Fatalf("scan=%+v err=%v", value, err)
	}
	scanList, _ := serviceContract.NewScanListRequest(repository.ScanListParams{Scope: scope, RepositoryID: goldenRepoID, PageSize: 10})
	if page, err := bundle.Service().ListScans(context.Background(), scanList); err != nil || len(page.Items()) != 1 {
		t.Fatalf("scans=%+v err=%v", page, err)
	}
	artifact := result.Artifacts()[0]
	artifactQuery, _ := repository.NewArtifactQuery(scope, goldenRepoID, goldenScanID, artifact.ArtifactID())
	if value, err := bundle.Service().GetArtifact(context.Background(), artifactQuery); err != nil || value.ArtifactID() != artifact.ArtifactID() {
		t.Fatalf("artifact=%+v err=%v", value, err)
	}
	artifactList, _ := serviceContract.NewArtifactListRequest(repository.ArtifactListParams{Scope: scope, RepositoryID: goldenRepoID, ScanID: goldenScanID, PageSize: 20})
	if page, err := bundle.Service().ListArtifacts(context.Background(), artifactList); err != nil || len(page.Items()) != 10 {
		t.Fatalf("artifacts=%d err=%v", len(page.Items()), err)
	}
	export, _ := repository.NewExportArtifactRequest(artifactQuery)
	var output bytes.Buffer
	if receipt, err := bundle.Service().ExportArtifact(context.Background(), export, &output); err != nil || receipt.PayloadDigest() != artifact.PayloadDigest() || output.Len() == 0 {
		t.Fatalf("export=%+v bytes=%d err=%v", receipt, output.Len(), err)
	}
	runningID := repository.ScanID("22222222-2222-4222-8222-222222222225")
	port.seedRunning(goldenScopeID, goldenRepoID, string(runningID), repository.DefaultRepositoryGoProfile().Profile().Digest(), resolver.revision)
	cancel, _ := repository.NewCancelScanRequest(repository.CancelScanParams{Scope: scope, RequestID: "cancel", RepositoryID: goldenRepoID, ScanID: runningID})
	if value, err := bundle.Service().CancelScan(context.Background(), cancel); err != nil || value.State() != repository.ScanCanceled {
		t.Fatalf("cancel=%+v err=%v", value, err)
	}
	store, _ := NewStore(port, port, port, persistenceContract, serviceContract.Profiles())
	if reconciled, err := store.Reconcile(context.Background(), scope, goldenRepoID, goldenScanID); err != nil || reconciled.Scan().State() != repository.ScanSucceeded || len(reconciled.Artifacts()) != 10 {
		t.Fatalf("reconcile=%+v err=%v", reconciled, err)
	}
	failedID := repository.ScanID("22222222-2222-4222-8222-222222222226")
	port.seedRunning(goldenScopeID, goldenRepoID, string(failedID), repository.DefaultRepositoryGoProfile().Profile().Digest(), resolver.revision)
	failed, _ := repository.NewScan(repository.ScanParams{RepositoryID: goldenRepoID, ScanID: failedID, Profile: repository.DefaultRepositoryGoProfile().Profile(), SourceRevision: resolver.revision, State: repository.ScanFailed, ReasonCode: "analysis-failed", RequestedAt: now, StartedAt: now, FinishedAt: now})
	if value, err := store.finish(context.Background(), scope, "finish", failed); err != nil || value.State() != repository.ScanFailed {
		t.Fatalf("finish=%+v err=%v", value, err)
	}
	port.stageFailure = persistence.NewError(persistence.ErrorUnavailable, "stage-payload", true, nil)
	stageRequest, _ := serviceContract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: "stage-failure", RepositoryID: goldenRepoID, ScanID: "22222222-2222-4222-8222-222222222227", SourceHandle: "source", Profile: repository.DefaultRepositoryGoProfile().Profile()})
	if _, err := bundle.Service().ExecuteScan(context.Background(), stageRequest); err == nil {
		t.Fatal("stage failure was accepted")
	}
	port.publishFailure = persistence.NewError(persistence.ErrorUnavailable, "publish-scan", true, nil)
	publishRequest, _ := serviceContract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: "publish-failure", RepositoryID: goldenRepoID, ScanID: "22222222-2222-4222-8222-222222222228", SourceHandle: "source", Profile: repository.DefaultRepositoryGoProfile().Profile()})
	if _, err := bundle.Service().ExecuteScan(context.Background(), publishRequest); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("publish failure=%v", err)
	}
	archive, _ := repository.NewArchiveRepositoryRequest(repository.ArchiveRepositoryParams{Scope: scope, RequestID: "archive", RepositoryID: goldenRepoID})
	if value, err := bundle.Service().ArchiveRepository(context.Background(), archive); err != nil || value.State() != repository.RepositoryArchived {
		t.Fatalf("archive=%+v err=%v", value, err)
	}
}

// TestDisposablePostgreSQLRepositoryService is opt-in. The accepted runtime
// harness creates, migrates, and destroys the PostgreSQL cluster and injects
// only disposable connection settings before enabling this test.
func TestDisposablePostgreSQLRepositoryService(t *testing.T) {
	if os.Getenv("AEGIS_REPOSITORY_SERVICE_INTEGRATION") != "1" {
		t.Skip("set only by the disposable PostgreSQL runtime harness")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/integration\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loadRequest := runtimeconfig.NewLoadRequest(runtimeconfig.LoadRequestParams{
		Environment: []string{
			"AEGIS_PROFILE=ci",
			"AEGIS_DATABASE_HOST=127.0.0.1",
			"AEGIS_DATABASE_PORT=" + requiredIntegrationValue(t, "AEGIS_RUNTIME_POSTGRES_PORT"),
			"AEGIS_DATABASE_NAME=" + requiredIntegrationValue(t, "AEGIS_RUNTIME_POSTGRES_DATABASE"),
			"AEGIS_DATABASE_USER=" + requiredIntegrationValue(t, "AEGIS_RUNTIME_POSTGRES_USER"),
		},
		SecretProvider: disposableSecretProvider{},
	})
	persistenceContract, _ := persistence.New()
	serviceContract, _ := repository.New()
	cycleDurations := make([]time.Duration, 0, 100)
	scanDurations := make([]time.Duration, 0, 100)
	for cycle := 0; cycle < 100; cycle++ {
		t.Run(fmt.Sprintf("cycle-%03d", cycle), func(t *testing.T) {
			cycleStarted := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			runtime, err := runtimeapp.NewDefaultStarter().Start(ctx, loadRequest)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer shutdownCancel()
				if _, shutdownErr := runtime.Shutdown(shutdownContext); shutdownErr != nil {
					t.Errorf("shutdown: %v", shutdownErr)
				}
			}()
			resolver := reusableResolver{root: root, fingerprint: repository.DigestBytes([]byte(fmt.Sprintf("disposable-source-proof-%03d", cycle))), revision: "commit:0123456789abcdef"}
			bundle, err := New(Dependencies{Runtime: runtime, Persistence: persistenceContract, ServiceContract: serviceContract, SourceResolver: resolver, Clock: scan.ClockFunc(func() time.Time { return time.Now().UTC() })})
			if err != nil {
				t.Fatal(err)
			}
			repositoryID := repository.RepositoryID(fmt.Sprintf("11111111-1111-4111-8111-%012d", cycle+1000))
			scanID := repository.ScanID(fmt.Sprintf("22222222-2222-4222-8222-%012d", cycle+1000))
			scope, _ := repository.NewScope(goldenScopeID, "disposable-principal")
			register, err := serviceContract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: repository.RequestID(fmt.Sprintf("register-%03d", cycle)), RepositoryID: repositoryID, DisplayName: "Disposable Integration", SourceHandle: "disposable-source"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = bundle.Service().RegisterRepository(ctx, register); err != nil {
				t.Fatal(err)
			}
			execute, err := serviceContract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: repository.RequestID(fmt.Sprintf("execute-%03d", cycle)), RepositoryID: repositoryID, ScanID: scanID, SourceHandle: "disposable-source", Profile: repository.DefaultRepositoryGoProfile().Profile()})
			if err != nil {
				t.Fatal(err)
			}
			scanStarted := time.Now()
			result, err := bundle.Service().ExecuteScan(ctx, execute)
			scanDurations = append(scanDurations, time.Since(scanStarted))
			if err != nil || result.Scan().State() != repository.ScanSucceeded || len(result.Artifacts()) != 10 {
				t.Fatalf("scan artifacts=%d err=%v", len(result.Artifacts()), err)
			}
			artifact := result.Artifacts()[0]
			query, _ := repository.NewArtifactQuery(scope, repositoryID, scanID, artifact.ArtifactID())
			exportRequest, _ := repository.NewExportArtifactRequest(query)
			var output bytes.Buffer
			receipt, err := bundle.Service().ExportArtifact(ctx, exportRequest, &output)
			if err != nil || receipt.PayloadDigest() != artifact.PayloadDigest() || uint64(output.Len()) != artifact.PayloadSize() {
				t.Fatalf("export bytes=%d err=%v", output.Len(), err)
			}
			cycleDurations = append(cycleDurations, time.Since(cycleStarted))
		})
	}
	t.Logf("repository_service_metrics cycles=%d scan_p50=%s scan_p95=%s scan_max=%s cycle_p50=%s cycle_p95=%s cycle_max=%s", len(scanDurations), durationPercentile(scanDurations, 50), durationPercentile(scanDurations, 95), durationPercentile(scanDurations, 100), durationPercentile(cycleDurations, 50), durationPercentile(cycleDurations, 95), durationPercentile(cycleDurations, 100))
}

func TestFailureClassificationAndConstructorGuards(t *testing.T) {
	if bundle, err := New(Dependencies{}); err == nil || bundle != nil {
		t.Fatal("accepted empty dependencies")
	}
	if store, err := NewStore(nil, nil, nil, nil, nil); err == nil || store != nil {
		t.Fatal("accepted empty store dependencies")
	}
	port := newMemoryPersistence(time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC))
	persistenceContract, _ := persistence.New()
	serviceContract, _ := repository.New()
	resolver := reusableResolver{root: t.TempDir(), fingerprint: repository.DigestBytes([]byte("source"))}
	clock := fixedClock{time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC)}
	dependencies := Dependencies{Runtime: &fakeRuntime{port: port}, Persistence: persistenceContract, ServiceContract: serviceContract, SourceResolver: resolver, Clock: clock}
	if bundle, err := New(dependencies, DefaultConfig(), DefaultConfig()); err == nil || bundle != nil {
		t.Fatal("accepted multiple integration configurations")
	}
	dependencies.Runtime = nullCapabilityRuntime{}
	if bundle, err := New(dependencies); err == nil || bundle != nil {
		t.Fatal("accepted runtime without capabilities")
	}
	invalidConfigs := []Config{
		{FinalizationTimeout: time.Millisecond},
		{FinalizationTimeout: time.Hour},
		{ReadPageSize: -1},
		{ReadPageSize: 1001},
		{MaxArtifacts: -1},
		{MaxArtifacts: 65},
		{MaxDependencies: -1},
		{MaxDependencies: 4097},
		{MaxPayloadBytes: (uint64(4) << 30) + 1},
	}
	for index, config := range invalidConfigs {
		if err := config.withDefaults().Validate(); err == nil {
			t.Fatalf("invalid config %d accepted", index)
		}
	}
	if DefaultConfig().Validate() != nil {
		t.Fatal("default config rejected")
	}
	kinds := []persistence.ErrorKind{persistence.ErrorNotFound, persistence.ErrorIdempotencyConflict, persistence.ErrorLifecycleConflict, persistence.ErrorInvalidDependency, persistence.ErrorPayloadTooLarge, persistence.ErrorAuthorizationDenied, persistence.ErrorTimeout, persistence.ErrorCanceled, persistence.ErrorUnavailable, persistence.ErrorInvalidInput, persistence.ErrorInternal}
	for _, kind := range kinds {
		err := serviceFailure(persistence.NewError(kind, "operation", kind == persistence.ErrorUnavailable, nil), "integration", "persistence-contract-failed")
		if err == nil || strings.Contains(err.Error(), "SQLSTATE") {
			t.Fatalf("kind %s: %v", kind, err)
		}
	}
	canceled := serviceFailure(context.Canceled, "integration", "failed")
	deadline := serviceFailure(context.DeadlineExceeded, "integration", "failed")
	if repository.KindOf(canceled) != repository.ErrorCanceled || repository.KindOf(deadline) != repository.ErrorTimeout || serviceFailure(nil, "integration", "failed") != nil {
		t.Fatal("context mapping")
	}
	admission := &Admission{runtime: &fakeRuntime{err: errors.New("unavailable")}}
	if _, err := admission.Acquire(context.Background()); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("admission=%v", err)
	}
	if _, err := (&Admission{runtime: &fakeRuntime{nilWork: true}}).Acquire(context.Background()); repository.KindOf(err) != repository.ErrorInternal {
		t.Fatalf("nil work=%v", err)
	}
	var nilBundle *Bundle
	if nilBundle.Service() != nil {
		t.Fatal("nil bundle")
	}
}

func TestScanFailureAndExistingTerminalBranches(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	port := newMemoryPersistence(now)
	persistenceContract, _ := persistence.New()
	serviceContract, _ := repository.New()
	scope, _ := repository.NewScope(goldenScopeID, "principal")
	sourceDigest := repository.DigestBytes([]byte("source-proof"))
	port.seedRepository(goldenScopeID, goldenRepoID, sourceDigest)
	store, _ := NewStore(port, port, port, persistenceContract, serviceContract.Profiles())
	preparer := failingPreparer{fingerprint: sourceDigest, revision: "revision", err: errors.New("analysis failed")}
	coordinator, _ := scan.New(store, directAdmission{}, preparer, fixedClock{now})
	request, _ := serviceContract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: "failure", RepositoryID: goldenRepoID, ScanID: "22222222-2222-4222-8222-222222222240", SourceHandle: "source", Profile: repository.DefaultRepositoryGoProfile().Profile()})
	if _, err := coordinator.ExecuteScan(context.Background(), request); repository.KindOf(err) != repository.ErrorAnalysisFailed {
		t.Fatalf("analysis failure=%v", err)
	}
	profile := repository.DefaultRepositoryGoProfile().Profile().Digest()
	for index, state := range []persistence.ScanState{persistence.ScanRunning, persistence.ScanFailed, persistence.ScanCancelled} {
		id := repository.ScanID([]string{"22222222-2222-4222-8222-222222222241", "22222222-2222-4222-8222-222222222242", "22222222-2222-4222-8222-222222222243"}[index])
		port.seedScan(goldenScopeID, goldenRepoID, string(id), profile, "revision", state)
		candidate, _ := serviceContract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: repository.RequestID("existing-" + string(rune('a'+index))), RepositoryID: goldenRepoID, ScanID: id, SourceHandle: "source", Profile: repository.DefaultRepositoryGoProfile().Profile()})
		_, err := coordinator.ExecuteScan(context.Background(), candidate)
		if err == nil {
			t.Fatalf("state %s unexpectedly succeeded", state)
		}
	}
}

func TestManifestAndSourceFailureBranches(t *testing.T) {
	scope, _ := repository.NewScope(goldenScopeID, "principal")
	handle, _ := repository.NewSourceHandle("opaque", 32)
	adapter := &SourceProofAdapter{resolver: fakeResolver{err: errors.New("denied")}}
	if _, err := adapter.Resolve(context.Background(), scope, handle); repository.KindOf(err) != repository.ErrorSourceUnavailable {
		t.Fatalf("resolver error=%v", err)
	}
	bad := &SourceProofAdapter{resolver: fakeResolver{source: &fakeSource{}}}
	if _, err := bad.Resolve(context.Background(), scope, handle); repository.KindOf(err) != repository.ErrorSourceUnavailable {
		t.Fatalf("invalid proof=%v", err)
	}
	if _, err := (&Admission{}).Acquire(context.Background()); repository.KindOf(err) != repository.ErrorInternal {
		t.Fatalf("nil admission=%v", err)
	}
	if _, err := CanonicalManifest(repository.Scan{}, nil); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("invalid manifest=%v", err)
	}
}

func TestPersistenceReadAndLifecycleFailurePaths(t *testing.T) {
	now := time.Date(2026, 7, 28, 17, 0, 0, 0, time.UTC)
	port := newMemoryPersistence(now)
	persistenceContract, _ := persistence.New()
	serviceContract, _ := repository.New()
	scope, _ := repository.NewScope(goldenScopeID, "principal")
	sourceDigest := repository.DigestBytes([]byte("source"))
	port.seedRepository(goldenScopeID, goldenRepoID, sourceDigest)
	store, _ := NewStore(port, port, port, persistenceContract, serviceContract.Profiles())
	failure := func() error { return persistence.NewError(persistence.ErrorUnavailable, "test", true, nil) }
	port.failure = failure()
	if _, err := store.Get(context.Background(), scope, goldenRepoID); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("get=%v", err)
	}
	port.failure = failure()
	if _, err := store.List(context.Background(), scope, 10, ""); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("list=%v", err)
	}
	port.failure = failure()
	if _, err := store.GetScan(context.Background(), scope, goldenRepoID, goldenScanID); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("get scan=%v", err)
	}
	port.failure = failure()
	if _, err := store.ListScans(context.Background(), scope, goldenRepoID, 10, ""); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("list scans=%v", err)
	}
	publicID, _ := repository.NewArtifactID(goldenRepoID, goldenScanID, "artifact", "1.0.0", repository.ArtifactIdentityScheme)
	port.failure = failure()
	if _, err := store.GetArtifact(context.Background(), scope, goldenRepoID, goldenScanID, publicID); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("get artifact=%v", err)
	}
	port.failure = failure()
	if _, err := store.ListArtifacts(context.Background(), scope, goldenRepoID, goldenScanID, 10, ""); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("list artifacts=%v", err)
	}
	lifecycleService, _ := lifecycle.New(store, &SourceProofAdapter{resolver: reusableResolver{root: t.TempDir(), fingerprint: sourceDigest}}, fixedClock{now})
	register, _ := serviceContract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: "register-failure", RepositoryID: "11111111-1111-4111-8111-111111111166", DisplayName: "Failure", SourceHandle: "source"})
	port.failure = failure()
	if _, err := lifecycleService.RegisterRepository(context.Background(), register); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("register=%v", err)
	}
	archive, _ := repository.NewArchiveRepositoryRequest(repository.ArchiveRepositoryParams{Scope: scope, RequestID: "archive-failure", RepositoryID: goldenRepoID})
	port.failure = failure()
	if _, err := lifecycleService.ArchiveRepository(context.Background(), archive); repository.KindOf(err) != repository.ErrorPersistenceUnavailable {
		t.Fatalf("archive=%v", err)
	}
}

func TestValidationAndRecordMismatchBranches(t *testing.T) {
	now := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	port := newMemoryPersistence(now)
	persistenceContract, _ := persistence.New()
	serviceContract, _ := repository.New()
	store, _ := NewStore(port, port, port, persistenceContract, serviceContract.Profiles())
	if _, err := store.Publish(context.Background(), scan.PublishCommand{}); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("empty publication=%v", err)
	}
	if _, err := ManifestDigest(repository.Scan{}, nil); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("empty manifest digest=%v", err)
	}
	zero := repository.Scope{}
	if _, err := store.Get(context.Background(), zero, goldenRepoID); err == nil {
		t.Fatal("zero get scope")
	}
	if _, err := store.List(context.Background(), zero, 10, ""); err == nil {
		t.Fatal("zero list scope")
	}
	if _, err := store.GetScan(context.Background(), zero, goldenRepoID, goldenScanID); err == nil {
		t.Fatal("zero scan scope")
	}
	if _, err := store.ListScans(context.Background(), zero, goldenRepoID, 10, ""); err == nil {
		t.Fatal("zero scan-list scope")
	}
	if _, err := store.GetArtifact(context.Background(), zero, goldenRepoID, goldenScanID, "artifact"); err == nil {
		t.Fatal("zero artifact scope")
	}
	if _, err := store.ListArtifacts(context.Background(), zero, goldenRepoID, goldenScanID, 10, ""); err == nil {
		t.Fatal("zero artifact-list scope")
	}
	if _, err := store.ExportArtifact(context.Background(), zero, goldenRepoID, goldenScanID, "artifact", nil); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("nil writer=%v", err)
	}
	if _, err := (&SourceProofAdapter{}).Resolve(context.Background(), zero, repository.SourceHandle{}); repository.KindOf(err) != repository.ErrorInternal {
		t.Fatalf("nil resolver=%v", err)
	}
	emptyProfiles, _ := repository.NewProfileRegistry()
	if _, err := NewStore(port, port, port, persistenceContract, emptyProfiles); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("empty profiles=%v", err)
	}
	if _, err := NewStore(port, port, port, persistenceContract, serviceContract.Profiles(), DefaultConfig(), DefaultConfig()); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("multiple config=%v", err)
	}
	otherScope, _ := repository.NewScope("00000000-0000-4000-8000-000000000099", "principal")
	source, _ := persistence.NewSourceIdentity("local", "sha256/v1", persistence.DigestBytes([]byte("source")))
	repoRecord, _ := persistence.NewRepositoryRecord(goldenScopeID, goldenRepoID, "Repository", source, persistence.RepositoryActive, "", now, now)
	if _, err := store.repositoryRecord(otherScope, repoRecord); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("repository scope mismatch=%v", err)
	}
	mismatchedRepo, _ := persistence.NewRepositoryRecord(goldenScopeID, "11111111-1111-4111-8111-111111111199", "Repository", source, persistence.RepositoryActive, "", now, now)
	port.repositories[persistenceKey(goldenScopeID, goldenRepoID)] = mismatchedRepo
	if _, err := store.Get(context.Background(), mustScopeValue(t), goldenRepoID); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("repository record mismatch=%v", err)
	}
	unknownProfile := persistence.DigestBytes([]byte("unknown"))
	scanRecord, _ := persistence.NewScanRecord(goldenScopeID, goldenRepoID, goldenScanID, unknownProfile, "", persistence.ScanRunning, "", "", now, now, time.Time{})
	if _, err := store.scanRecord(mustScopeValue(t), scanRecord); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("unknown profile=%v", err)
	}
	validProfile := toPersistenceDigest(repository.DefaultRepositoryGoProfile().Profile().Digest())
	otherScan, _ := persistence.NewScanRecord("00000000-0000-4000-8000-000000000099", goldenRepoID, goldenScanID, validProfile, "", persistence.ScanRunning, "", "", now, now, time.Time{})
	if _, err := store.scanRecord(mustScopeValue(t), otherScan); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("scan scope mismatch=%v", err)
	}
	mismatchedScan, _ := persistence.NewScanRecord(goldenScopeID, goldenRepoID, "22222222-2222-4222-8222-222222222299", validProfile, "", persistence.ScanRunning, "", "", now, now, time.Time{})
	port.scans[persistenceKey(goldenScopeID, goldenRepoID, string(goldenScanID))] = mismatchedScan
	if _, err := store.GetScan(context.Background(), mustScopeValue(t), goldenRepoID, goldenScanID); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("scan record mismatch=%v", err)
	}
	artifactName, _ := persistence.NewVersionedName("artifact", "1.0.0")
	codec, _ := persistence.NewCodec("canonical-json", "1.0.0", "application/json")
	producer, _ := persistence.NewVersionedName("producer", "1.0.0")
	wrongArtifact, _ := persistence.NewArtifactRecord(goldenScopeID, goldenRepoID, goldenScanID, "00000000-0000-8000-8000-000000000001", artifactName, repository.ArtifactIdentityScheme, codec, producer, persistence.DigestBytes([]byte("payload")), 7, now)
	if _, err := store.artifactRecord(mustScopeValue(t), goldenRepoID, goldenScanID, wrongArtifact); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("artifact mapping mismatch=%v", err)
	}
	var nilResolution *sourceResolution
	if err := nilResolution.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func FuzzPhysicalArtifactIDNeverPanics(f *testing.F) {
	f.Add("")
	f.Add("rsaid1_19546c9e510fd1b314a29e8dc8af31a6318ab8ed7f8b2bc0377fd514a6772886")
	f.Add("{hostile}\x00identifier")
	f.Fuzz(func(t *testing.T, input string) {
		first, firstErr := PhysicalArtifactID(repository.ArtifactID(input))
		second, secondErr := PhysicalArtifactID(repository.ArtifactID(input))
		if (firstErr == nil) != (secondErr == nil) || first != second {
			t.Fatal("mapping is not deterministic")
		}
		if firstErr == nil && (len(first) != 36 || first[14] != '8' || !strings.Contains("89ab", string(first[19]))) {
			t.Fatalf("invalid RFC 9562 UUID: %q", first)
		}
	})
}

func FuzzCanonicalManifestNeverPanics(f *testing.F) {
	f.Add([]byte("payload"), "revision")
	f.Add([]byte{}, "")
	f.Fuzz(func(t *testing.T, payload []byte, revision string) {
		profile := repository.DefaultRepositoryGoProfile().Profile()
		now := time.Date(2026, 7, 28, 19, 0, 0, 0, time.UTC)
		value, err := repository.NewScan(repository.ScanParams{RepositoryID: goldenRepoID, ScanID: goldenScanID, Profile: profile, SourceRevision: revision, State: repository.ScanRunning, RequestedAt: now, StartedAt: now})
		if err != nil {
			return
		}
		id, _ := repository.NewArtifactID(goldenRepoID, goldenScanID, "artifact", "1.0.0", repository.ArtifactIdentityScheme)
		artifact, err := repository.NewArtifact(repository.ArtifactParams{ArtifactID: id, ScanID: goldenScanID, Name: "artifact", Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "1.0.0", MediaType: "application/json", PayloadDigest: repository.DigestBytes(payload), PayloadSize: uint64(len(payload)), ProducerName: "producer", ProducerVersion: "1.0.0", CreatedAt: now})
		if err != nil {
			return
		}
		items := []manifestArtifact{testManifestArtifact{metadata: artifact}}
		first, firstErr := canonicalManifest(value, items)
		second, secondErr := canonicalManifest(value, items)
		if (firstErr == nil) != (secondErr == nil) || !bytes.Equal(first, second) {
			t.Fatal("manifest is not deterministic")
		}
	})
}

func FuzzRecordAndErrorTranslationNeverPanics(f *testing.F) {
	f.Add("Repository", "raw SQLSTATE 99999 secret")
	f.Add("", "")
	f.Fuzz(func(t *testing.T, displayName, raw string) {
		now := time.Date(2026, 7, 28, 20, 0, 0, 0, time.UTC)
		port := newMemoryPersistence(now)
		persistenceContract, _ := persistence.New()
		serviceContract, _ := repository.New()
		store, _ := NewStore(port, port, port, persistenceContract, serviceContract.Profiles())
		scope := mustScopeValue(t)
		source, _ := persistence.NewSourceIdentity("local", "sha256/v1", persistence.DigestBytes([]byte("source")))
		if record, err := persistence.NewRepositoryRecord(goldenScopeID, goldenRepoID, displayName, source, persistence.RepositoryActive, "", now, now); err == nil {
			_, _ = store.repositoryRecord(scope, record)
		}
		markerDigest := sha256.Sum256([]byte(raw))
		marker := fmt.Sprintf("raw-cause-%x", markerDigest[:8])
		mapped := serviceFailure(persistence.NewError(persistence.ErrorInternal, "storage", false, errors.New(marker)), "repository-service", "persistence-contract-failed")
		if strings.Contains(mapped.Error(), marker) {
			t.Fatal("raw persistence error leaked")
		}
	})
}

func mustScopeValue(t *testing.T) repository.Scope {
	t.Helper()
	value, err := repository.NewScope(goldenScopeID, "principal")
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func goldenArtifact(t *testing.T, id repository.ArtifactID, name, producer, producerVersion string, payload []byte, now time.Time) repository.Artifact {
	t.Helper()
	value, err := repository.NewArtifact(repository.ArtifactParams{ArtifactID: id, ScanID: goldenScanID, Name: name, Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "1.0.0", MediaType: "application/json", PayloadDigest: repository.DigestBytes(payload), PayloadSize: uint64(len(payload)), ProducerName: producer, ProducerVersion: producerVersion, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustDigest(t *testing.T, value string) repository.Digest {
	t.Helper()
	digest, err := repository.ParseDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

type testManifestArtifact struct {
	metadata     repository.Artifact
	dependencies []scan.ArtifactDependency
}

func (artifact testManifestArtifact) Metadata() repository.Artifact { return artifact.metadata }
func (artifact testManifestArtifact) Dependencies() []scan.ArtifactDependency {
	return append([]scan.ArtifactDependency(nil), artifact.dependencies...)
}

type fakeResolver struct {
	source *fakeSource
	err    error
}

func (resolver fakeResolver) Resolve(context.Context, repository.Scope, repository.SourceHandle) (serviceadapters.AuthorizedSource, error) {
	return resolver.source, resolver.err
}

type fakeSource struct {
	fingerprint       repository.Digest
	revision          string
	rootReads, closes int
}

func (source *fakeSource) RootPath() string               { source.rootReads++; return `D:\secret` }
func (source *fakeSource) Fingerprint() repository.Digest { return source.fingerprint }
func (source *fakeSource) Revision() string               { return source.revision }
func (source *fakeSource) Close(context.Context) error    { source.closes++; return nil }

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

type disposableSecretProvider struct{}

func (disposableSecretProvider) Resolve(ctx context.Context, _ runtimeconfig.SecretReference) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []byte("disposable-cluster-only"), nil
}

func requiredIntegrationValue(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("required disposable integration setting %s is absent", name)
	}
	return value
}

func durationPercentile(values []time.Duration, percentile int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]time.Duration(nil), values...)
	sort.Slice(copyValues, func(left, right int) bool { return copyValues[left] < copyValues[right] })
	index := (percentile*len(copyValues) + 99) / 100
	if index < 1 {
		index = 1
	}
	if index > len(copyValues) {
		index = len(copyValues)
	}
	return copyValues[index-1]
}

type reusableResolver struct {
	root        string
	fingerprint repository.Digest
	revision    string
}

func (resolver reusableResolver) Resolve(context.Context, repository.Scope, repository.SourceHandle) (serviceadapters.AuthorizedSource, error) {
	return &reusableSource{root: resolver.root, fingerprint: resolver.fingerprint, revision: resolver.revision}, nil
}

type reusableSource struct {
	root        string
	fingerprint repository.Digest
	revision    string
}

func (source *reusableSource) RootPath() string               { return source.root }
func (source *reusableSource) Fingerprint() repository.Digest { return source.fingerprint }
func (source *reusableSource) Revision() string               { return source.revision }
func (*reusableSource) Close(context.Context) error           { return nil }

type fakeRuntime struct {
	port     *memoryPersistence
	admitted int
	err      error
	nilWork  bool
}

type nullCapabilityRuntime struct{}

func (nullCapabilityRuntime) Admit(context.Context) (runtimeapp.Work, error) { return nil, nil }
func (nullCapabilityRuntime) Ingest() runtimepostgres.IngestCapabilities     { return nil }
func (nullCapabilityRuntime) Read() runtimepostgres.ReadCapabilities         { return nil }

func (runtime *fakeRuntime) Admit(ctx context.Context) (runtimeapp.Work, error) {
	if runtime.err != nil {
		return nil, runtime.err
	}
	if runtime.nilWork {
		return nil, nil
	}
	runtime.admitted++
	return &fakeWork{ctx: ctx}, nil
}
func (runtime *fakeRuntime) Ingest() runtimepostgres.IngestCapabilities { return runtime.port }
func (runtime *fakeRuntime) Read() runtimepostgres.ReadCapabilities     { return runtime.port }

type fakeWork struct{ ctx context.Context }

func (work *fakeWork) Context() context.Context { return work.ctx }
func (*fakeWork) Done()                         {}

type memoryPersistence struct {
	mu             sync.Mutex
	now            time.Time
	repositories   map[string]persistence.RepositoryRecord
	scans          map[string]persistence.ScanRecord
	artifacts      map[string][]persistence.ArtifactRecord
	payloads       map[persistence.Digest][]byte
	staged         int
	failure        error
	stageFailure   error
	publishFailure error
}

func (port *memoryPersistence) takeFailure() error {
	port.mu.Lock()
	defer port.mu.Unlock()
	err := port.failure
	port.failure = nil
	return err
}

func newMemoryPersistence(now time.Time) *memoryPersistence {
	return &memoryPersistence{now: now, repositories: map[string]persistence.RepositoryRecord{}, scans: map[string]persistence.ScanRecord{}, artifacts: map[string][]persistence.ArtifactRecord{}, payloads: map[persistence.Digest][]byte{}}
}

func (port *memoryPersistence) seedRunning(scopeID, repositoryID, scanID string, profile repository.Digest, revision string) {
	port.mu.Lock()
	defer port.mu.Unlock()
	value, _ := persistence.NewScanRecord(scopeID, persistence.RepositoryID(repositoryID), persistence.ScanID(scanID), toPersistenceDigest(profile), revision, persistence.ScanRunning, "", "", port.now, port.now, time.Time{})
	port.scans[persistenceKey(scopeID, repositoryID, scanID)] = value
}

func (port *memoryPersistence) seedRepository(scopeID, repositoryID string, digest repository.Digest) {
	port.mu.Lock()
	defer port.mu.Unlock()
	source, _ := persistence.NewSourceIdentity("local", "sha256/v1", toPersistenceDigest(digest))
	value, _ := persistence.NewRepositoryRecord(scopeID, persistence.RepositoryID(repositoryID), "Repository", source, persistence.RepositoryActive, "", port.now, port.now)
	port.repositories[persistenceKey(scopeID, repositoryID)] = value
}
func (port *memoryPersistence) seedScan(scopeID, repositoryID, scanID string, profile repository.Digest, revision string, state persistence.ScanState) {
	port.mu.Lock()
	defer port.mu.Unlock()
	finished := time.Time{}
	reason := ""
	if state == persistence.ScanFailed || state == persistence.ScanCancelled {
		finished = port.now.Add(time.Second)
		reason = "terminal"
	}
	value, _ := persistence.NewScanRecord(scopeID, persistence.RepositoryID(repositoryID), persistence.ScanID(scanID), toPersistenceDigest(profile), revision, state, reason, reason, port.now, port.now, finished)
	port.scans[persistenceKey(scopeID, repositoryID, scanID)] = value
}

type directAdmission struct{}

func (directAdmission) Acquire(ctx context.Context) (scan.WorkLease, error) {
	return directLease{ctx}, nil
}

type directLease struct{ ctx context.Context }

func (lease directLease) Context() context.Context { return lease.ctx }
func (directLease) Done()                          {}

type failingPreparer struct {
	fingerprint repository.Digest
	revision    string
	err         error
}

func (preparer failingPreparer) Prepare(context.Context, scan.AnalysisRequest) (scan.AnalysisSession, error) {
	return failingSession(preparer), nil
}

type failingSession failingPreparer

func (session failingSession) SourceFingerprint() repository.Digest { return session.fingerprint }
func (session failingSession) SourceRevision() string               { return session.revision }
func (session failingSession) Analyze(context.Context) (scan.AnalysisResult, error) {
	return scan.AnalysisResult{}, session.err
}
func (failingSession) Close(context.Context) error { return nil }
func persistenceKey(scope string, values ...string) string {
	return scope + "|" + strings.Join(values, "|")
}

func (port *memoryPersistence) RegisterRepository(_ context.Context, request persistence.RegisterRepositoryRequest) (persistence.RepositoryRecord, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.RepositoryRecord{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	key := persistenceKey(request.Scope().ScopeID(), string(request.RepositoryID()))
	if value, ok := port.repositories[key]; ok {
		return value, nil
	}
	value, err := persistence.NewRepositoryRecord(request.Scope().ScopeID(), request.RepositoryID(), request.DisplayName(), request.Source(), persistence.RepositoryActive, "", port.now, port.now)
	if err == nil {
		port.repositories[key] = value
	}
	return value, err
}
func (port *memoryPersistence) GetRepository(_ context.Context, query persistence.RepositoryQuery) (persistence.RepositoryRecord, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.RepositoryRecord{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	value, ok := port.repositories[persistenceKey(query.Scope().ScopeID(), string(query.RepositoryID()))]
	if !ok {
		return persistence.RepositoryRecord{}, persistence.NewError(persistence.ErrorNotFound, "get-repository", false, nil)
	}
	return value, nil
}
func (port *memoryPersistence) ListRepositories(_ context.Context, request persistence.RepositoryListRequest) (persistence.RepositoryPage, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.RepositoryPage{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	values := []persistence.RepositoryRecord{}
	prefix := request.Scope().ScopeID() + "|"
	for key, value := range port.repositories {
		if strings.HasPrefix(key, prefix) {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].RepositoryID() < values[j].RepositoryID() })
	return persistence.NewRepositoryPage(values, ""), nil
}
func (port *memoryPersistence) ArchiveRepository(_ context.Context, request persistence.ArchiveRepositoryRequest) (persistence.RepositoryRecord, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.RepositoryRecord{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	key := persistenceKey(request.Scope().ScopeID(), string(request.RepositoryID()))
	current, ok := port.repositories[key]
	if !ok {
		return persistence.RepositoryRecord{}, persistence.NewError(persistence.ErrorNotFound, "archive-repository", false, nil)
	}
	value, err := persistence.NewRepositoryRecord(current.ScopeID(), current.RepositoryID(), current.DisplayName(), current.Source(), persistence.RepositoryArchived, current.CurrentScanID(), current.CreatedAt(), port.now.Add(4*time.Second))
	if err == nil {
		port.repositories[key] = value
	}
	return value, err
}
func (port *memoryPersistence) BeginScan(_ context.Context, request persistence.BeginScanRequest) (persistence.ScanRecord, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.ScanRecord{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	key := persistenceKey(request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID()))
	if value, ok := port.scans[key]; ok {
		return value, nil
	}
	value, err := persistence.NewScanRecord(request.Scope().ScopeID(), request.RepositoryID(), request.ScanID(), request.AnalysisProfileDigest(), request.SourceRevision(), persistence.ScanRunning, "", "", port.now, port.now, time.Time{})
	if err == nil {
		port.scans[key] = value
	}
	return value, err
}
func (port *memoryPersistence) GetScan(_ context.Context, query persistence.ScanQuery) (persistence.ScanRecord, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.ScanRecord{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	value, ok := port.scans[persistenceKey(query.Scope().ScopeID(), string(query.RepositoryID()), string(query.ScanID()))]
	if !ok {
		return persistence.ScanRecord{}, persistence.NewError(persistence.ErrorNotFound, "get-scan", false, nil)
	}
	return value, nil
}
func (port *memoryPersistence) ListScans(_ context.Context, request persistence.ScanListRequest) (persistence.ScanPage, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.ScanPage{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	values := []persistence.ScanRecord{}
	prefix := persistenceKey(request.Scope().ScopeID(), string(request.RepositoryID())) + "|"
	for key, value := range port.scans {
		if strings.HasPrefix(key, prefix) {
			values = append(values, value)
		}
	}
	return persistence.NewScanPage(values, ""), nil
}
func (port *memoryPersistence) FailScan(_ context.Context, request persistence.FinishScanRequest) (persistence.ScanRecord, error) {
	return port.finish(request, persistence.ScanFailed)
}
func (port *memoryPersistence) CancelScan(_ context.Context, request persistence.FinishScanRequest) (persistence.ScanRecord, error) {
	return port.finish(request, persistence.ScanCancelled)
}
func (port *memoryPersistence) finish(request persistence.FinishScanRequest, state persistence.ScanState) (persistence.ScanRecord, error) {
	port.mu.Lock()
	defer port.mu.Unlock()
	key := persistenceKey(request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID()))
	current, ok := port.scans[key]
	if !ok {
		return persistence.ScanRecord{}, persistence.NewError(persistence.ErrorNotFound, "finish-scan", false, nil)
	}
	value, err := persistence.NewScanRecord(current.ScopeID(), current.RepositoryID(), current.ScanID(), current.AnalysisProfileDigest(), current.SourceRevision(), state, request.ReasonCode(), request.SafeMessage(), current.RequestedAt(), current.StartedAt(), port.now.Add(3*time.Second))
	if err == nil {
		port.scans[key] = value
	}
	return value, err
}
func (port *memoryPersistence) StagePayload(_ context.Context, request persistence.StagePayloadRequest, reader io.Reader) (persistence.PayloadReceipt, error) {
	if port.stageFailure != nil {
		err := port.stageFailure
		port.stageFailure = nil
		return persistence.PayloadReceipt{}, err
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return persistence.PayloadReceipt{}, err
	}
	if persistence.DigestBytes(data) != request.Digest() || uint64(len(data)) != uint64(request.ExpectedSize()) {
		return persistence.PayloadReceipt{}, persistence.NewError(persistence.ErrorIntegrityFailure, "stage-payload", false, nil)
	}
	port.mu.Lock()
	port.payloads[request.Digest()] = append([]byte(nil), data...)
	port.staged++
	port.mu.Unlock()
	return persistence.NewPayloadReceipt(request.Digest(), request.ExpectedSize(), persistence.DispositionCreated)
}
func (port *memoryPersistence) PublishScan(_ context.Context, request persistence.PublishScanRequest) (persistence.PublicationReceipt, error) {
	if port.publishFailure != nil {
		err := port.publishFailure
		port.publishFailure = nil
		return persistence.PublicationReceipt{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	key := persistenceKey(request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID()))
	current, ok := port.scans[key]
	if !ok {
		return persistence.PublicationReceipt{}, persistence.NewError(persistence.ErrorNotFound, "publish-scan", false, nil)
	}
	succeeded, err := persistence.NewScanRecord(current.ScopeID(), current.RepositoryID(), current.ScanID(), current.AnalysisProfileDigest(), current.SourceRevision(), persistence.ScanSucceeded, "", "", current.RequestedAt(), current.StartedAt(), port.now.Add(2*time.Second))
	if err != nil {
		return persistence.PublicationReceipt{}, err
	}
	records := make([]persistence.ArtifactRecord, 0, len(request.Artifacts()))
	for _, submission := range request.Artifacts() {
		record, recordErr := persistence.NewArtifactRecord(request.Scope().ScopeID(), request.RepositoryID(), request.ScanID(), submission.ArtifactID(), submission.Artifact(), submission.StableIDScheme(), submission.Codec(), submission.Producer(), submission.PayloadDigest(), submission.PayloadSize(), port.now.Add(2*time.Second))
		if recordErr != nil {
			return persistence.PublicationReceipt{}, recordErr
		}
		records = append(records, record)
	}
	port.scans[key] = succeeded
	port.artifacts[key] = records
	repoKey := persistenceKey(request.Scope().ScopeID(), string(request.RepositoryID()))
	repo := port.repositories[repoKey]
	updated, _ := persistence.NewRepositoryRecord(repo.ScopeID(), repo.RepositoryID(), repo.DisplayName(), repo.Source(), repo.State(), request.ScanID(), repo.CreatedAt(), port.now.Add(2*time.Second))
	port.repositories[repoKey] = updated
	return persistence.NewPublicationReceipt(request.ScanID(), request.ManifestScheme(), request.ManifestDigest(), uint32(len(records)), persistence.DispositionCreated)
}
func (port *memoryPersistence) GetArtifact(_ context.Context, query persistence.ArtifactQuery) (persistence.ArtifactRecord, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.ArtifactRecord{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	for _, value := range port.artifacts[persistenceKey(query.Scope().ScopeID(), string(query.RepositoryID()), string(query.ScanID()))] {
		if value.ArtifactID() == query.ArtifactID() {
			return value, nil
		}
	}
	return persistence.ArtifactRecord{}, persistence.NewError(persistence.ErrorNotFound, "get-artifact", false, nil)
}
func (port *memoryPersistence) ListArtifacts(_ context.Context, request persistence.ArtifactListRequest) (persistence.ArtifactPage, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.ArtifactPage{}, err
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	values := append([]persistence.ArtifactRecord(nil), port.artifacts[persistenceKey(request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID()))]...)
	sort.Slice(values, func(i, j int) bool { return values[i].Artifact().Name() < values[j].Artifact().Name() })
	return persistence.NewArtifactPage(values, ""), nil
}
func (port *memoryPersistence) ExportPayload(_ context.Context, query persistence.PayloadQuery, writer io.Writer) (persistence.PayloadReceipt, error) {
	if err := port.takeFailure(); err != nil {
		return persistence.PayloadReceipt{}, err
	}
	port.mu.Lock()
	data := append([]byte(nil), port.payloads[query.Digest()]...)
	port.mu.Unlock()
	if len(data) == 0 {
		return persistence.PayloadReceipt{}, persistence.NewError(persistence.ErrorNotFound, "export-payload", false, nil)
	}
	if _, err := writer.Write(data); err != nil {
		return persistence.PayloadReceipt{}, err
	}
	return persistence.NewPayloadReceipt(query.Digest(), persistence.ByteCount(len(data)), persistence.DispositionCreated)
}
func (port *memoryPersistence) VerifyPayload(_ context.Context, query persistence.PayloadQuery) (persistence.VerificationReceipt, error) {
	port.mu.Lock()
	defer port.mu.Unlock()
	data := port.payloads[query.Digest()]
	if len(data) == 0 {
		return persistence.VerificationReceipt{}, persistence.NewError(persistence.ErrorNotFound, "verify-payload", false, nil)
	}
	return persistence.NewVerificationReceipt(query.Digest(), persistence.ByteCount(len(data)))
}

var _ runtimepostgres.IngestCapabilities = (*memoryPersistence)(nil)
var _ runtimepostgres.ReadCapabilities = (*memoryPersistence)(nil)
