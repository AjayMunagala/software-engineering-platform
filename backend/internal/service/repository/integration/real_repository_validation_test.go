package integration

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	runtimeapp "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/app"
	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	serviceadapters "github.com/AjayMunagala/software-engineering-platform/backend/internal/service/repository/adapters"
	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/scan"
)

const realValidationDomain = "repository-service-real-validation/v1\x00"

type realValidationResult struct {
	Case                   string                     `json:"case"`
	Pass                   string                     `json:"pass"`
	Commit                 string                     `json:"commit"`
	Tree                   string                     `json:"tree"`
	GitVersion             string                     `json:"git_version"`
	SourceManifestSHA256   string                     `json:"source_manifest_sha256"`
	TrackedFiles           int                        `json:"tracked_files"`
	OperatingSystem        string                     `json:"operating_system"`
	Architecture           string                     `json:"architecture"`
	GoVersion              string                     `json:"go_version"`
	GOMAXPROCS             int                        `json:"gomaxprocs"`
	RepositoryID           string                     `json:"repository_id"`
	ScanID                 string                     `json:"scan_id"`
	Profile                string                     `json:"profile"`
	ArtifactCount          int                        `json:"artifact_count"`
	Artifacts              []realArtifactResult       `json:"artifacts"`
	Dependencies           []realDependencyResult     `json:"dependencies"`
	Publication            realPublicationResult      `json:"publication"`
	Database               realDatabaseResult         `json:"database"`
	ScanMilliseconds       float64                    `json:"scan_milliseconds"`
	ExportMilliseconds     float64                    `json:"export_milliseconds"`
	TotalMilliseconds      float64                    `json:"total_milliseconds"`
	PeakHeapBytes          uint64                     `json:"peak_heap_bytes"`
	AllocatedBytes         uint64                     `json:"allocated_bytes"`
	Allocations            uint64                     `json:"allocations"`
	NormalizedSHA256       string                     `json:"normalized_sha256"`
	SourcePrivacy          bool                       `json:"source_privacy"`
	ScopeIsolation         bool                       `json:"scope_isolation"`
	ExactExports           bool                       `json:"exact_exports"`
	RuntimeShutdown        runtimeapp.ShutdownOutcome `json:"runtime_shutdown"`
	RuntimeResourcesClosed bool                       `json:"runtime_resources_closed"`
}

type realArtifactResult struct {
	Ordinal         int                        `json:"ordinal"`
	ArtifactID      string                     `json:"artifact_id"`
	PhysicalID      string                     `json:"physical_id"`
	Name            string                     `json:"name"`
	Version         string                     `json:"version"`
	StableIDScheme  string                     `json:"stable_id_scheme"`
	CodecName       string                     `json:"codec_name"`
	CodecVersion    string                     `json:"codec_version"`
	MediaType       string                     `json:"media_type"`
	PayloadSHA256   string                     `json:"payload_sha256"`
	ExportSHA256    string                     `json:"export_sha256"`
	PayloadBytes    uint64                     `json:"payload_bytes"`
	ExportBytes     uint64                     `json:"export_bytes"`
	ProducerName    string                     `json:"producer_name"`
	ProducerVersion string                     `json:"producer_version"`
	SelectedFacts   map[string]json.RawMessage `json:"selected_facts,omitempty"`
}

type realDependencyResult struct {
	Consumer        string `json:"consumer"`
	Ordinal         int    `json:"ordinal"`
	Source          string `json:"source"`
	DeclaredName    string `json:"declared_name"`
	DeclaredVersion string `json:"declared_version"`
}

type realPublicationResult struct {
	Scheme        string `json:"scheme"`
	ManifestSHA   string `json:"manifest_sha256"`
	ArtifactCount int    `json:"artifact_count"`
}

type realDatabaseResult struct {
	Bytes         int64 `json:"bytes"`
	PayloadBytes  int64 `json:"payload_bytes"`
	PayloadChunks int64 `json:"payload_chunks"`
	WALBytes      int64 `json:"wal_bytes"`
	Connections   int64 `json:"connections"`
}

type realNormalizedResult struct {
	SourceManifest string                    `json:"source_manifest"`
	Profile        string                    `json:"profile"`
	Artifacts      []realNormalizedArtifact  `json:"artifacts"`
	Dependencies   []realDependencyResult    `json:"dependencies"`
	Publication    realNormalizedPublication `json:"publication"`
}

type realNormalizedArtifact struct {
	Ordinal         int                        `json:"ordinal"`
	Name            string                     `json:"name"`
	Version         string                     `json:"version"`
	StableIDScheme  string                     `json:"stable_id_scheme"`
	CodecName       string                     `json:"codec_name"`
	CodecVersion    string                     `json:"codec_version"`
	MediaType       string                     `json:"media_type"`
	PayloadSHA256   string                     `json:"payload_sha256"`
	PayloadBytes    uint64                     `json:"payload_bytes"`
	ProducerName    string                     `json:"producer_name"`
	ProducerVersion string                     `json:"producer_version"`
	SelectedFacts   map[string]json.RawMessage `json:"selected_facts,omitempty"`
}

type realNormalizedPublication struct {
	Scheme        string `json:"scheme"`
	ArtifactCount int    `json:"artifact_count"`
}

type realRecoveryResult struct {
	Case              string `json:"case"`
	Pass              string `json:"pass"`
	ScanID            string `json:"scan_id"`
	ArtifactCount     int    `json:"artifact_count"`
	PublicationSHA256 string `json:"publication_sha256"`
	ExactExports      bool   `json:"exact_exports"`
	Dependencies      int    `json:"dependencies"`
	DatabaseBytes     int64  `json:"database_bytes"`
	Recovery          string `json:"recovery"`
}

// TestRealRepositoryValidation is an opt-in Phase 4.0.7 validation harness.
// It consumes a pre-fetched, preflighted local tree and never fetches, executes,
// builds, tests, installs, or mutates repository content.
func TestRealRepositoryValidation(t *testing.T) {
	if os.Getenv("AEGIS_REPOSITORY_SERVICE_REAL_VALIDATION") != "1" {
		t.Skip("set only by the accepted Phase 4.0.7 disposable harness")
	}
	totalStarted := time.Now()
	root := requiredRealValue(t, "AEGIS_REAL_ROOT")
	caseName := requiredRealValue(t, "AEGIS_REAL_CASE")
	passName := requiredRealValue(t, "AEGIS_REAL_PASS")
	commit := requiredRealValue(t, "AEGIS_REAL_COMMIT")
	tree := requiredRealValue(t, "AEGIS_REAL_TREE")
	gitVersion := requiredRealValue(t, "AEGIS_REAL_GIT_VERSION")
	trackedList := requiredRealValue(t, "AEGIS_REAL_TRACKED_LIST")
	outputPath := requiredRealValue(t, "AEGIS_REAL_OUTPUT")
	adminURL := requiredRealValue(t, "POSTGRES_TEST_URL")
	if pathInside(root, outputPath) || pathInside(root, trackedList) {
		t.Fatal("validation output overlaps authorized source")
	}

	manifestDigest, trackedFiles, err := realSourceManifest(root, trackedList)
	if err != nil {
		t.Fatalf("source manifest preflight failed: %v", safeValidationError(err))
	}
	if expected := strings.TrimSpace(os.Getenv("AEGIS_REAL_MANIFEST_SHA256")); expected != "" && expected != manifestDigest {
		t.Fatal("source manifest does not match approved preflight")
	}
	fingerprint, err := repository.ParseDigest(manifestDigest)
	if err != nil {
		t.Fatal("source fingerprint is invalid")
	}

	ctx, cancel := context.WithTimeout(context.Background(), realValidationTimeout(t))
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal("disposable PostgreSQL audit connection failed")
	}
	defer admin.Close(context.Background())
	var walStart string
	if err = admin.QueryRow(ctx, "SELECT pg_current_wal_lsn()::text").Scan(&walStart); err != nil {
		t.Fatal("PostgreSQL WAL baseline failed")
	}

	loadRequest := runtimeconfig.NewLoadRequest(runtimeconfig.LoadRequestParams{
		Environment: []string{
			"AEGIS_PROFILE=ci",
			"AEGIS_DATABASE_HOST=127.0.0.1",
			"AEGIS_DATABASE_PORT=" + requiredRealValue(t, "AEGIS_RUNTIME_POSTGRES_PORT"),
			"AEGIS_DATABASE_NAME=" + requiredRealValue(t, "AEGIS_RUNTIME_POSTGRES_DATABASE"),
			"AEGIS_DATABASE_USER=" + requiredRealValue(t, "AEGIS_RUNTIME_POSTGRES_USER"),
		},
		SecretProvider: realValidationSecretProvider{},
	})
	application, err := runtimeapp.NewDefaultStarter().Start(ctx, loadRequest)
	if err != nil {
		t.Fatal("runtime startup failed")
	}
	shutdownComplete := false
	defer func() {
		if !shutdownComplete {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()
			_, _ = application.Shutdown(shutdownCtx)
		}
	}()

	contract, err := repository.New()
	if err != nil {
		t.Fatal(err)
	}
	persistenceContract, err := persistence.New()
	if err != nil {
		t.Fatal(err)
	}
	handleText := "phase407-source-canary-" + shortDigest(caseName+"\x00"+passName)
	resolver := &realValidationResolver{root: root, fingerprint: fingerprint, revision: "git:" + commit + ";tree:" + tree + ";manifest:" + manifestDigest, handle: handleText}
	bundle, err := New(Dependencies{Runtime: application, Persistence: persistenceContract, ServiceContract: contract, SourceResolver: resolver, Clock: scan.ClockFunc(func() time.Time { return time.Now().UTC() })})
	if err != nil {
		t.Fatal("repository service composition failed")
	}

	scope, _ := repository.NewScope(repository.ScopeID(validationUUID("scope-primary")), "phase407-validation")
	otherScope, _ := repository.NewScope(repository.ScopeID(validationUUID("scope-other")), "phase407-other")
	repositoryID := repository.RepositoryID(validationUUID("repository\x00" + caseName))
	scanID := repository.ScanID(validationUUID("scan\x00" + caseName + "\x00" + passName))
	register, err := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: repository.RequestID("register-" + shortDigest(caseName)), RepositoryID: repositoryID, DisplayName: "Phase 4.0.7 " + caseName, SourceHandle: handleText})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bundle.Service().RegisterRepository(ctx, register); err != nil {
		t.Fatalf("repository registration failed: %v", safeValidationError(err))
	}
	if _, err = bundle.Service().RegisterRepository(ctx, register); err != nil {
		t.Fatal("idempotent registration failed")
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	peakStop := make(chan struct{})
	peakResult := make(chan realHeapSample, 1)
	go sampleRealPeakHeap(peakStop, peakResult, realMemoryCeiling(t), cancel)
	profile := repository.DefaultRepositoryGoProfile().Profile()
	execute, err := contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: repository.RequestID("scan-" + shortDigest(caseName+"\x00"+passName)), RepositoryID: repositoryID, ScanID: scanID, SourceHandle: handleText, Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	scanStarted := time.Now()
	result, err := bundle.Service().ExecuteScan(ctx, execute)
	scanDuration := time.Since(scanStarted)
	close(peakStop)
	heapSample := <-peakResult
	if err != nil {
		if heapSample.Exceeded {
			t.Fatal("memory ceiling exceeded")
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatal("validation timeout exceeded")
		}
		t.Fatalf("real repository scan failed: %v", safeValidationError(err))
	}
	if result.Scan().State() != repository.ScanSucceeded || result.Scan().ScanID() != scanID {
		t.Fatal("scan did not reach the expected succeeded state")
	}

	artifacts := result.Artifacts()
	wantArtifacts := 7
	if realHasGoArtifact(artifacts) {
		wantArtifacts = 10
	}
	if len(artifacts) != wantArtifacts {
		t.Fatalf("artifact count = %d, want %d", len(artifacts), wantArtifacts)
	}
	if err = verifyProfileOrder(artifacts, profile); err != nil {
		t.Fatal(err)
	}
	listRequest, _ := contract.NewArtifactListRequest(repository.ArtifactListParams{Scope: scope, RepositoryID: repositoryID, ScanID: scanID, PageSize: 64})
	listed, err := bundle.Service().ListArtifacts(ctx, listRequest)
	if err != nil || len(listed.Items()) != len(artifacts) || !artifactsNameOrdered(listed.Items()) {
		t.Fatal("persisted artifact listing is incomplete or nondeterministic")
	}

	exportStarted := time.Now()
	artifactResults := make([]realArtifactResult, 0, len(artifacts))
	for ordinal, artifact := range artifacts {
		artifactResult, exportErr := exportRealArtifact(ctx, bundle.Service(), scope, repositoryID, scanID, artifact, ordinal, root, handleText, filepath.Dir(outputPath))
		if exportErr != nil {
			t.Fatalf("artifact export failed for %s: %v", artifact.Name(), safeValidationError(exportErr))
		}
		artifactResults = append(artifactResults, artifactResult)
	}
	exportDuration := time.Since(exportStarted)

	scopeIsolation := verifyRealScopeIsolation(ctx, bundle.Service(), contract, otherScope, repositoryID, scanID, artifacts[0])
	if !scopeIsolation {
		t.Fatal("repository scope isolation failed")
	}
	publication, dependencies, database, err := auditRealDatabase(ctx, admin, scanID, walStart)
	if err != nil {
		t.Fatalf("database audit failed: %v", safeValidationError(err))
	}
	if publication.ArtifactCount != len(artifacts) {
		t.Fatal("publication artifact count mismatch")
	}
	if err = verifyDependencyContract(artifacts, dependencies); err != nil {
		t.Fatal(err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	shutdown, shutdownErr := application.Shutdown(shutdownCtx)
	shutdownCancel()
	shutdownComplete = true
	if shutdownErr != nil || !shutdown.ResourcesClosed() {
		t.Fatal("runtime shutdown failed")
	}
	if resolver.opened.Load() != resolver.closed.Load() || resolver.opened.Load() < 2 {
		t.Fatal("authorized source lifecycle is unbalanced")
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	validation := realValidationResult{
		Case: caseName, Pass: passName, Commit: commit, Tree: tree, GitVersion: gitVersion,
		SourceManifestSHA256: manifestDigest, TrackedFiles: trackedFiles,
		OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, GoVersion: runtime.Version(), GOMAXPROCS: runtime.GOMAXPROCS(0),
		RepositoryID: string(repositoryID), ScanID: string(scanID), Profile: profile.Name() + "/" + profile.Version(),
		ArtifactCount: len(artifacts), Artifacts: artifactResults, Dependencies: dependencies, Publication: publication, Database: database,
		ScanMilliseconds: realMilliseconds(scanDuration), ExportMilliseconds: realMilliseconds(exportDuration), TotalMilliseconds: realMilliseconds(time.Since(totalStarted)),
		PeakHeapBytes: heapSample.Peak, AllocatedBytes: after.TotalAlloc - before.TotalAlloc, Allocations: after.Mallocs - before.Mallocs,
		SourcePrivacy: true, ScopeIsolation: scopeIsolation, ExactExports: true,
		RuntimeShutdown: shutdown.Outcome(), RuntimeResourcesClosed: shutdown.ResourcesClosed(),
	}
	validation.NormalizedSHA256, err = normalizedRealDigest(validation)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(validation, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if containsAny(encoded, forbiddenRealValues(root, handleText)) {
		t.Fatal("validation result contains protected source information")
	}
	if err = os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		t.Fatal("validation result directory unavailable")
	}
	if err = os.WriteFile(outputPath, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal("validation result write failed")
	}
	t.Logf("phase407_result case=%s pass=%s artifacts=%d normalized_sha256=%s scan=%s peak_heap=%d", caseName, passName, len(artifacts), validation.NormalizedSHA256, scanDuration, heapSample.Peak)
}

// TestRealRepositoryCrashRecovery runs only after the disposable PostgreSQL
// cluster has been stopped in immediate mode and restarted. It proves that the
// already-published scan remains complete and exactly exportable.
func TestRealRepositoryCrashRecovery(t *testing.T) {
	if os.Getenv("AEGIS_REPOSITORY_SERVICE_REAL_RECOVERY") != "1" {
		t.Skip("set only after the Phase 4.0.7 crash-recovery step")
	}
	inputPath := requiredRealValue(t, "AEGIS_REAL_RECOVERY_INPUT")
	outputPath := requiredRealValue(t, "AEGIS_REAL_RECOVERY_OUTPUT")
	root := requiredRealValue(t, "AEGIS_REAL_ROOT")
	if pathInside(root, inputPath) || pathInside(root, outputPath) {
		t.Fatal("recovery evidence overlaps authorized source")
	}
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal("recovery input is unavailable")
	}
	var original realValidationResult
	if err = json.Unmarshal(raw, &original); err != nil {
		t.Fatal("recovery input is invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	loadRequest := runtimeconfig.NewLoadRequest(runtimeconfig.LoadRequestParams{
		Environment: []string{
			"AEGIS_PROFILE=ci",
			"AEGIS_DATABASE_HOST=127.0.0.1",
			"AEGIS_DATABASE_PORT=" + requiredRealValue(t, "AEGIS_RUNTIME_POSTGRES_PORT"),
			"AEGIS_DATABASE_NAME=" + requiredRealValue(t, "AEGIS_RUNTIME_POSTGRES_DATABASE"),
			"AEGIS_DATABASE_USER=" + requiredRealValue(t, "AEGIS_RUNTIME_POSTGRES_USER"),
		},
		SecretProvider: realValidationSecretProvider{},
	})
	application, err := runtimeapp.NewDefaultStarter().Start(ctx, loadRequest)
	if err != nil {
		t.Fatal("runtime restart after PostgreSQL recovery failed")
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		_, _ = application.Shutdown(shutdownCtx)
	}()
	contract, _ := repository.New()
	persistenceContract, _ := persistence.New()
	fingerprint, err := repository.ParseDigest(original.SourceManifestSHA256)
	if err != nil {
		t.Fatal("recovery source fingerprint is invalid")
	}
	handle := "phase407-recovery-placeholder"
	resolver := &realValidationResolver{root: root, fingerprint: fingerprint, revision: "recovery-verification", handle: handle}
	bundle, err := New(Dependencies{Runtime: application, Persistence: persistenceContract, ServiceContract: contract, SourceResolver: resolver, Clock: scan.ClockFunc(func() time.Time { return time.Now().UTC() })})
	if err != nil {
		t.Fatal("recovery service composition failed")
	}
	scope, _ := repository.NewScope(repository.ScopeID(validationUUID("scope-primary")), "phase407-validation")
	repositoryID := repository.RepositoryID(original.RepositoryID)
	scanID := repository.ScanID(original.ScanID)
	scanQuery, _ := repository.NewScanQuery(scope, repositoryID, scanID)
	scanValue, err := bundle.Service().GetScan(ctx, scanQuery)
	if err != nil || scanValue.State() != repository.ScanSucceeded {
		t.Fatal("published scan did not survive PostgreSQL recovery")
	}
	list, _ := contract.NewArtifactListRequest(repository.ArtifactListParams{Scope: scope, RepositoryID: repositoryID, ScanID: scanID, PageSize: 64})
	page, err := bundle.Service().ListArtifacts(ctx, list)
	if err != nil || len(page.Items()) != len(original.Artifacts) {
		t.Fatal("artifact envelopes did not survive PostgreSQL recovery")
	}
	expected := make(map[repository.ArtifactID]realArtifactResult, len(original.Artifacts))
	for _, artifact := range original.Artifacts {
		expected[repository.ArtifactID(artifact.ArtifactID)] = artifact
	}
	for _, artifact := range page.Items() {
		want, ok := expected[artifact.ArtifactID()]
		if !ok || want.PayloadSHA256 != artifact.PayloadDigest().String() || want.PayloadBytes != artifact.PayloadSize() {
			t.Fatal("recovered artifact metadata mismatch")
		}
		query, _ := repository.NewArtifactQuery(scope, repositoryID, scanID, artifact.ArtifactID())
		export, _ := repository.NewExportArtifactRequest(query)
		hasher := sha256.New()
		writer := &realAuditWriter{writer: hasher, forbidden: forbiddenRealValues(root, handle)}
		receipt, exportErr := bundle.Service().ExportArtifact(ctx, export, writer)
		if exportErr != nil || writer.size != want.PayloadBytes || hex.EncodeToString(hasher.Sum(nil)) != want.PayloadSHA256 || receipt.PayloadDigest().String() != want.PayloadSHA256 {
			t.Fatal("exact payload did not survive PostgreSQL recovery")
		}
	}
	admin, err := pgx.Connect(ctx, requiredRealValue(t, "POSTGRES_TEST_URL"))
	if err != nil {
		t.Fatal("recovery database audit connection failed")
	}
	defer admin.Close(context.Background())
	var walStart string
	_ = admin.QueryRow(ctx, "SELECT pg_current_wal_lsn()::text").Scan(&walStart)
	publication, dependencies, database, err := auditRealDatabase(ctx, admin, scanID, walStart)
	if err != nil || publication.ManifestSHA != original.Publication.ManifestSHA || publication.ArtifactCount != original.ArtifactCount {
		t.Fatal("publication proof did not survive PostgreSQL recovery")
	}
	recovery := realRecoveryResult{Case: original.Case, Pass: original.Pass, ScanID: original.ScanID, ArtifactCount: len(page.Items()), PublicationSHA256: publication.ManifestSHA, ExactExports: true, Dependencies: len(dependencies), DatabaseBytes: database.Bytes, Recovery: "pass"}
	encoded, _ := json.MarshalIndent(recovery, "", "  ")
	if err = os.WriteFile(outputPath, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal("recovery result write failed")
	}
	t.Logf("phase407_recovery case=%s artifacts=%d publication_sha256=%s", original.Case, len(page.Items()), publication.ManifestSHA)
}

type realValidationSecretProvider struct{}

func (realValidationSecretProvider) Resolve(ctx context.Context, _ runtimeconfig.SecretReference) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []byte("disposable-phase407-only"), nil
}

type realValidationResolver struct {
	root, revision, handle string
	fingerprint            repository.Digest
	opened, closed         atomic.Int64
}

func (resolver *realValidationResolver) Resolve(_ context.Context, _ repository.Scope, handle repository.SourceHandle) (serviceadapters.AuthorizedSource, error) {
	if resolver == nil || handle.Reveal() != resolver.handle {
		return nil, errors.New("authorized source unavailable")
	}
	resolver.opened.Add(1)
	return &realValidationSource{resolver: resolver}, nil
}

type realValidationSource struct {
	resolver *realValidationResolver
	closed   atomic.Bool
}

func (source *realValidationSource) RootPath() string { return source.resolver.root }
func (source *realValidationSource) Fingerprint() repository.Digest {
	return source.resolver.fingerprint
}
func (source *realValidationSource) Revision() string { return source.resolver.revision }
func (source *realValidationSource) Close(context.Context) error {
	if source != nil && source.resolver != nil && source.closed.CompareAndSwap(false, true) {
		source.resolver.closed.Add(1)
	}
	return nil
}

func exportRealArtifact(ctx context.Context, service repository.Service, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, artifact repository.Artifact, ordinal int, root, handle, outputDirectory string) (realArtifactResult, error) {
	query, err := repository.NewArtifactQuery(scope, repositoryID, scanID, artifact.ArtifactID())
	if err != nil {
		return realArtifactResult{}, err
	}
	request, err := repository.NewExportArtifactRequest(query)
	if err != nil {
		return realArtifactResult{}, err
	}
	file, err := os.CreateTemp(outputDirectory, "phase407-export-*.json")
	if err != nil {
		return realArtifactResult{}, err
	}
	path := file.Name()
	defer os.Remove(path)
	hasher := sha256.New()
	writer := &realAuditWriter{writer: io.MultiWriter(file, hasher), forbidden: forbiddenRealValues(root, handle)}
	receipt, exportErr := service.ExportArtifact(ctx, request, writer)
	closeErr := file.Close()
	if exportErr != nil {
		return realArtifactResult{}, exportErr
	}
	if closeErr != nil {
		return realArtifactResult{}, closeErr
	}
	exportDigest := hex.EncodeToString(hasher.Sum(nil))
	if writer.size != artifact.PayloadSize() || receipt.PayloadSize() != artifact.PayloadSize() || exportDigest != artifact.PayloadDigest().String() || receipt.PayloadDigest() != artifact.PayloadDigest() {
		return realArtifactResult{}, errors.New("export integrity mismatch")
	}
	facts, err := extractRealFacts(path)
	if err != nil {
		return realArtifactResult{}, err
	}
	physical, err := PhysicalArtifactID(artifact.ArtifactID())
	if err != nil {
		return realArtifactResult{}, err
	}
	return realArtifactResult{
		Ordinal: ordinal, ArtifactID: string(artifact.ArtifactID()), PhysicalID: string(physical), Name: artifact.Name(), Version: artifact.Version(), StableIDScheme: artifact.StableIDScheme(), CodecName: artifact.CodecName(), CodecVersion: artifact.CodecVersion(), MediaType: artifact.MediaType(), PayloadSHA256: artifact.PayloadDigest().String(), ExportSHA256: exportDigest, PayloadBytes: artifact.PayloadSize(), ExportBytes: writer.size, ProducerName: artifact.ProducerName(), ProducerVersion: artifact.ProducerVersion(), SelectedFacts: facts,
	}, nil
}

type realAuditWriter struct {
	writer    io.Writer
	forbidden [][]byte
	tail      []byte
	size      uint64
}

func (writer *realAuditWriter) Write(data []byte) (int, error) {
	combined := append(append([]byte(nil), writer.tail...), data...)
	for _, value := range writer.forbidden {
		if len(value) > 0 && bytes.Contains(combined, value) {
			return 0, errors.New("protected source value detected")
		}
	}
	maximum := 0
	for _, value := range writer.forbidden {
		if len(value) > maximum {
			maximum = len(value)
		}
	}
	keep := maximum - 1
	if keep < 0 {
		keep = 0
	}
	if keep > len(combined) {
		keep = len(combined)
	}
	writer.tail = append(writer.tail[:0], combined[len(combined)-keep:]...)
	written, err := writer.writer.Write(data)
	writer.size += uint64(written)
	return written, err
}

func extractRealFacts(path string) (map[string]json.RawMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(bufio.NewReaderSize(file, 128*1024))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, errors.New("artifact JSON object is invalid")
	}
	targets := map[string]struct{}{"statistics": {}, "summary": {}}
	result := map[string]json.RawMessage{}
	for decoder.More() {
		key, keyErr := decoder.Token()
		if keyErr != nil {
			return nil, keyErr
		}
		name, ok := key.(string)
		if !ok {
			return nil, errors.New("artifact JSON key is invalid")
		}
		if _, selected := targets[name]; selected {
			var raw json.RawMessage
			if err = decoder.Decode(&raw); err != nil {
				return nil, err
			}
			result[name] = append(json.RawMessage(nil), raw...)
			continue
		}
		if err = skipRealJSONValue(decoder); err != nil {
			return nil, err
		}
	}
	if _, err = decoder.Token(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func skipRealJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		for decoder.More() {
			if _, err = decoder.Token(); err != nil {
				return err
			}
			if err = skipRealJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err = skipRealJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}

func auditRealDatabase(ctx context.Context, connection *pgx.Conn, scanID repository.ScanID, walStart string) (realPublicationResult, []realDependencyResult, realDatabaseResult, error) {
	var publication realPublicationResult
	if err := connection.QueryRow(ctx, `SELECT manifest_scheme,encode(artifact_set_digest,'hex'),artifact_count FROM platform.scan_publications WHERE scan_id=$1::uuid`, string(scanID)).Scan(&publication.Scheme, &publication.ManifestSHA, &publication.ArtifactCount); err != nil {
		return publication, nil, realDatabaseResult{}, err
	}
	rows, err := connection.Query(ctx, `SELECT consumer.artifact_name,d.dependency_ordinal,source.artifact_name,d.declared_name,d.declared_version FROM platform.artifact_dependencies d JOIN platform.artifact_envelopes consumer ON consumer.scan_id=d.scan_id AND consumer.artifact_id=d.consumer_artifact_id JOIN platform.artifact_envelopes source ON source.scan_id=d.scan_id AND source.artifact_id=d.source_artifact_id WHERE d.scan_id=$1::uuid ORDER BY consumer.artifact_name,d.dependency_ordinal`, string(scanID))
	if err != nil {
		return publication, nil, realDatabaseResult{}, err
	}
	dependencies := []realDependencyResult{}
	for rows.Next() {
		var dependency realDependencyResult
		if err = rows.Scan(&dependency.Consumer, &dependency.Ordinal, &dependency.Source, &dependency.DeclaredName, &dependency.DeclaredVersion); err != nil {
			rows.Close()
			return publication, nil, realDatabaseResult{}, err
		}
		dependencies = append(dependencies, dependency)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return publication, nil, realDatabaseResult{}, err
	}
	rows.Close()
	var database realDatabaseResult
	err = connection.QueryRow(ctx, `SELECT pg_database_size(current_database()),COALESCE(sum(e.payload_size),0),COALESCE(sum(p.chunk_count),0),(SELECT pg_wal_lsn_diff(pg_current_wal_lsn(),$2::pg_lsn)::bigint),(SELECT count(*) FROM pg_stat_activity WHERE datname=current_database()) FROM platform.artifact_envelopes e JOIN platform.artifact_payloads p ON p.payload_digest=e.payload_digest WHERE e.scan_id=$1::uuid`, string(scanID), walStart).Scan(&database.Bytes, &database.PayloadBytes, &database.PayloadChunks, &database.WALBytes, &database.Connections)
	return publication, dependencies, database, err
}

func verifyDependencyContract(artifacts []repository.Artifact, dependencies []realDependencyResult) error {
	available := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		available[artifact.Name()] = artifact.Version()
	}
	seen := map[string]struct{}{}
	last := ""
	lastOrdinal := -1
	for _, dependency := range dependencies {
		if available[dependency.Consumer] == "" || available[dependency.Source] == "" || available[dependency.DeclaredName] != dependency.DeclaredVersion || dependency.Source != dependency.DeclaredName || dependency.Consumer == dependency.Source {
			return errors.New("durable dependency contract mismatch")
		}
		key := fmt.Sprintf("%s\x00%d", dependency.Consumer, dependency.Ordinal)
		if _, exists := seen[key]; exists {
			return errors.New("duplicate durable dependency ordinal")
		}
		seen[key] = struct{}{}
		if dependency.Consumer == last && dependency.Ordinal <= lastOrdinal {
			return errors.New("durable dependency order mismatch")
		}
		if dependency.Consumer != last {
			last, lastOrdinal = dependency.Consumer, -1
		}
		lastOrdinal = dependency.Ordinal
	}
	return nil
}

func verifyProfileOrder(artifacts []repository.Artifact, profile repository.AnalysisProfile) error {
	definition := repository.DefaultRepositoryGoProfile()
	expected := definition.Artifacts()
	if profile != definition.Profile() {
		return errors.New("profile contract mismatch")
	}
	if len(artifacts) != 7 && len(artifacts) != len(expected) {
		return errors.New("artifact profile length mismatch")
	}
	if len(artifacts) == 7 {
		expected = expected[:7]
	}
	sort.Slice(expected, func(left, right int) bool {
		return expected[left].Name() < expected[right].Name()
	})
	for index, artifact := range artifacts {
		if artifact.Name() != expected[index].Name() || artifact.Version() != expected[index].Version() || artifact.StableIDScheme() != expected[index].StableIDScheme() {
			return errors.New("artifact profile order mismatch")
		}
	}
	return nil
}

func artifactsNameOrdered(values []repository.Artifact) bool {
	for index := 1; index < len(values); index++ {
		if values[index-1].Name() > values[index].Name() {
			return false
		}
	}
	return true
}

func realHasGoArtifact(values []repository.Artifact) bool {
	for _, value := range values {
		if value.Name() == "go-language-inventory" {
			return true
		}
	}
	return false
}

func verifyRealScopeIsolation(ctx context.Context, service repository.Service, contract *repository.Contract, other repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, artifact repository.Artifact) bool {
	query, _ := repository.NewRepositoryQuery(other, repositoryID)
	if _, err := service.GetRepository(ctx, query); repository.KindOf(err) != repository.ErrorNotFound {
		return false
	}
	list, _ := contract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: other, PageSize: 10})
	page, err := service.ListRepositories(ctx, list)
	if err != nil || len(page.Items()) != 0 {
		return false
	}
	artifactQuery, _ := repository.NewArtifactQuery(other, repositoryID, scanID, artifact.ArtifactID())
	export, _ := repository.NewExportArtifactRequest(artifactQuery)
	var output bytes.Buffer
	if _, err = service.ExportArtifact(ctx, export, &output); repository.KindOf(err) != repository.ErrorNotFound || output.Len() != 0 {
		return false
	}
	return true
}

func normalizedRealDigest(result realValidationResult) (string, error) {
	artifacts := make([]realNormalizedArtifact, len(result.Artifacts))
	for index, artifact := range result.Artifacts {
		artifacts[index] = realNormalizedArtifact{Ordinal: artifact.Ordinal, Name: artifact.Name, Version: artifact.Version, StableIDScheme: artifact.StableIDScheme, CodecName: artifact.CodecName, CodecVersion: artifact.CodecVersion, MediaType: artifact.MediaType, PayloadSHA256: artifact.PayloadSHA256, PayloadBytes: artifact.PayloadBytes, ProducerName: artifact.ProducerName, ProducerVersion: artifact.ProducerVersion, SelectedFacts: artifact.SelectedFacts}
	}
	value := realNormalizedResult{SourceManifest: result.SourceManifestSHA256, Profile: result.Profile, Artifacts: artifacts, Dependencies: append([]realDependencyResult(nil), result.Dependencies...), Publication: realNormalizedPublication{Scheme: result.Publication.Scheme, ArtifactCount: result.Publication.ArtifactCount}}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func realSourceManifest(root, trackedList string) (string, int, error) {
	raw, err := os.ReadFile(trackedList)
	if err != nil {
		return "", 0, err
	}
	parts := bytes.Split(raw, []byte{0})
	type trackedEntry struct {
		path string
		mode string
		oid  string
	}
	entries := make([]trackedEntry, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		mode, oid, path := "", "", string(part)
		if tab := bytes.IndexByte(part, '\t'); tab >= 0 {
			metadata := strings.Fields(string(part[:tab]))
			if len(metadata) != 3 || metadata[2] != "0" {
				return "", 0, errors.New("invalid staged tracked entry")
			}
			mode, oid, path = metadata[0], metadata[1], string(part[tab+1:])
		}
		path = filepath.ToSlash(path)
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(filepath.FromSlash(path)) {
			return "", 0, errors.New("tracked path escapes source root")
		}
		entries = append(entries, trackedEntry{path: clean, mode: mode, oid: oid})
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].path < entries[right].path })
	type digestResult struct {
		kind   byte
		size   int64
		digest []byte
		err    error
	}
	results := make([]digestResult, len(entries))
	jobs := make(chan int)
	// Manifest I/O concurrency is a validation-harness concern, independent of
	// the product engine's one-worker/eight-worker determinism setting.
	workers := 16
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for index := range jobs {
				entry := entries[index]
				absolute := filepath.Join(root, filepath.FromSlash(entry.path))
				result := digestResult{kind: 'f'}
				contentHash := sha256.New()
				if entry.mode == "160000" {
					result.kind = 'g'
					result.size = int64(len(entry.oid))
					_, _ = io.WriteString(contentHash, entry.oid)
				} else if entry.mode == "120000" {
					result.kind = 'l'
					var targetBytes []byte
					info, statErr := os.Lstat(absolute)
					if statErr != nil {
						result.err = statErr
						results[index] = result
						continue
					}
					if info.Mode()&os.ModeSymlink != 0 {
						target, readErr := os.Readlink(absolute)
						if readErr != nil {
							result.err = readErr
							results[index] = result
							continue
						}
						targetBytes = []byte(filepath.ToSlash(target))
					} else {
						var readErr error
						targetBytes, readErr = os.ReadFile(absolute)
						if readErr != nil {
							result.err = readErr
							results[index] = result
							continue
						}
					}
					result.size = int64(len(targetBytes))
					_, _ = contentHash.Write(targetBytes)
				} else {
					info, statErr := os.Lstat(absolute)
					if statErr != nil || !info.Mode().IsRegular() {
						if statErr != nil {
							result.err = statErr
						} else {
							result.err = errors.New("unsupported tracked file kind")
						}
						results[index] = result
						continue
					}
					file, openErr := os.Open(absolute)
					if openErr != nil {
						result.err = openErr
						results[index] = result
						continue
					}
					result.size, result.err = io.Copy(contentHash, file)
					if closeErr := file.Close(); result.err == nil {
						result.err = closeErr
					}
				}
				result.digest = contentHash.Sum(nil)
				results[index] = result
			}
		}()
	}
	for index := range entries {
		jobs <- index
	}
	close(jobs)
	group.Wait()
	manifest := sha256.New()
	_, _ = io.WriteString(manifest, realValidationDomain)
	for index, entry := range entries {
		result := results[index]
		if result.err != nil {
			return "", 0, result.err
		}
		writeRealField(manifest, []byte(entry.path))
		_, _ = manifest.Write([]byte{result.kind})
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(result.size))
		_, _ = manifest.Write(length[:])
		_, _ = manifest.Write(result.digest)
	}
	return hex.EncodeToString(manifest.Sum(nil)), len(entries), nil
}

func writeRealField(writer hash.Hash, value []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(value)
}

func forbiddenRealValues(root, handle string) [][]byte {
	clean := filepath.Clean(root)
	values := []string{clean, filepath.ToSlash(clean), strings.ReplaceAll(clean, `\`, `\\`), handle}
	result := make([][]byte, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, []byte(value))
	}
	return result
}

func containsAny(data []byte, values [][]byte) bool {
	for _, value := range values {
		if len(value) > 0 && bytes.Contains(data, value) {
			return true
		}
	}
	return false
}

func validationUUID(value string) string {
	digest := sha256.Sum256([]byte(realValidationDomain + value))
	digest[6] = (digest[6] & 0x0f) | 0x40
	digest[8] = (digest[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(digest[:16])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}

func shortDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}

func pathInside(root, target string) bool {
	absoluteRoot, rootErr := filepath.Abs(filepath.Clean(root))
	absoluteTarget, targetErr := filepath.Abs(filepath.Clean(target))
	if rootErr != nil || targetErr != nil {
		return true
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteTarget)
	return err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)))
}

func requiredRealValue(t testing.TB, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Fatalf("required Phase 4.0.7 setting %s is absent", name)
	}
	return value
}

func realValidationTimeout(t testing.TB) time.Duration {
	t.Helper()
	value := strings.TrimSpace(os.Getenv("AEGIS_REAL_TIMEOUT"))
	if value == "" {
		return 30 * time.Minute
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < time.Minute || duration > 6*time.Hour {
		t.Fatal("invalid Phase 4.0.7 timeout")
	}
	return duration
}

type realHeapSample struct {
	Peak     uint64
	Exceeded bool
}

func sampleRealPeakHeap(stop <-chan struct{}, result chan<- realHeapSample, ceiling uint64, cancel context.CancelFunc) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	value := realHeapSample{}
	for {
		var current runtime.MemStats
		runtime.ReadMemStats(&current)
		if current.HeapAlloc > value.Peak {
			value.Peak = current.HeapAlloc
		}
		if ceiling > 0 && current.Sys >= ceiling && !value.Exceeded {
			value.Exceeded = true
			cancel()
		}
		select {
		case <-stop:
			result <- value
			return
		case <-ticker.C:
		}
	}
}

func realMemoryCeiling(t testing.TB) uint64 {
	t.Helper()
	value := strings.TrimSpace(os.Getenv("AEGIS_REAL_MEMORY_CEILING_BYTES"))
	if value == "" {
		return 0
	}
	var parsed uint64
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed < 512*1024*1024 {
		t.Fatal("invalid Phase 4.0.7 memory ceiling")
	}
	return parsed
}

func realMilliseconds(value time.Duration) float64 { return float64(value) / float64(time.Millisecond) }

func safeValidationError(err error) error {
	if err == nil {
		return nil
	}
	if kind := repository.KindOf(err); kind != "" {
		return fmt.Errorf("repository-service error kind %s", kind)
	}
	return errors.New("validation dependency failed")
}

func TestRealValidationHelpers(t *testing.T) {
	if validationUUID("same") != validationUUID("same") || validationUUID("same") == validationUUID("different") {
		t.Fatal("validation UUID derivation is not deterministic")
	}
	temporary := t.TempDir()
	root := filepath.Join(temporary, "source")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(temporary, "tracked")
	if err := os.WriteFile(list, append([]byte("a.txt"), 0), 0o600); err != nil {
		t.Fatal(err)
	}
	left, count, err := realSourceManifest(root, list)
	if err != nil || count != 1 {
		t.Fatalf("manifest count=%d err=%v", count, err)
	}
	right, _, _ := realSourceManifest(root, list)
	if left != right {
		t.Fatal("source manifest is not deterministic")
	}
	jsonPath := filepath.Join(temporary, "artifact.json")
	if err := os.WriteFile(jsonPath, []byte(`{"items":[{"x":1}],"statistics":{"files":1},"summary":{"unknown":0}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	facts, err := extractRealFacts(jsonPath)
	if err != nil || string(facts["statistics"]) != `{"files":1}` || string(facts["summary"]) != `{"unknown":0}` {
		t.Fatalf("facts=%s err=%v", facts, err)
	}
}

func FuzzRealSourceManifestNeverPanics(f *testing.F) {
	f.Add([]byte("100644 0000000000000000000000000000000000000000 0\ta.txt\x00"))
	f.Add([]byte("../escape\x00"))
	f.Fuzz(func(t *testing.T, tracked []byte) {
		if len(tracked) > 4096 {
			t.Skip()
		}
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o600); err != nil {
			t.Fatal(err)
		}
		list := filepath.Join(t.TempDir(), "tracked.bin")
		if err := os.WriteFile(list, tracked, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, _ = realSourceManifest(root, list)
	})
}

func FuzzRealNormalizationNeverPanics(f *testing.F) {
	f.Add("manifest", "artifact", "digest", uint64(1))
	f.Fuzz(func(t *testing.T, manifest, name, digest string, size uint64) {
		if len(manifest)+len(name)+len(digest) > 4096 {
			t.Skip()
		}
		_, _ = normalizedRealDigest(realValidationResult{
			SourceManifestSHA256: manifest,
			Profile:              "repository-go/v1",
			Artifacts: []realArtifactResult{{
				Name: name, PayloadSHA256: digest, PayloadBytes: size,
			}},
		})
	})
}
