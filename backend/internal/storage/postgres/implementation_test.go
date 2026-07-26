package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/AjayMunagala/software-engineering-platform/backend/persistence/conformance"
)

func TestConfigurationAndHelpers(t *testing.T) {
	if _, err := New(nil); err != ErrInvalidConfig {
		t.Fatalf("nil database accepted: %v", err)
	}
	if DefaultConfig().Validate() != nil || ChunkSize != 4<<20 {
		t.Fatal("accepted adapter configuration changed")
	}
	if (Config{}).Validate() != ErrInvalidConfig {
		t.Fatal("nil clock accepted")
	}
	if _, err := New(stubDatabase{}, Config{}); err != nil {
		t.Fatalf("zero config did not default: %v", err)
	}
	if _, err := New(stubDatabase{}, Config{}, Config{}); err != ErrInvalidConfig {
		t.Fatalf("multiple configs accepted: %v", err)
	}
	for _, offset := range []int{0, 1, 1000} {
		decoded, err := decodeCursor(encodeCursor(offset))
		if err != nil || decoded != offset {
			t.Fatalf("cursor %d: %v", offset, err)
		}
	}
	if _, err := decodeCursor("not/base64"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
	if _, err := decodeCursor("LTE"); err == nil {
		t.Fatal("negative cursor accepted")
	}
	if chunkCount(0) != 0 || chunkCount(1) != 1 || chunkCount(ChunkSize+1) != 2 {
		t.Fatal("chunk count changed")
	}
	if validateUUID(newUUID()) != nil || validateUUID("not-a-uuid") == nil {
		t.Fatal("UUID validation changed")
	}
	if _, err := parseDigest([]byte("short")); err == nil {
		t.Fatal("short digest accepted")
	}
	if !equalBytes([]byte{1}, []byte{1}) || equalBytes([]byte{1}, []byte{2}) || equalBytes([]byte{1}, []byte{1, 2}) {
		t.Fatal("constant-time equality changed")
	}
	if correlationUUID("request") == "" || uuidText(pgtype.UUID{}) != "" {
		t.Fatal("identifier conversion changed")
	}
	if persistence.KindOf(invalid("adapter-test")) != persistence.ErrorInvalidInput || persistence.KindOf(lifecycle("adapter-test")) != persistence.ErrorLifecycleConflict {
		t.Fatal("safe local error constructors changed")
	}
}

func TestSafePostgresErrorTranslation(t *testing.T) {
	cases := []struct {
		code string
		kind persistence.ErrorKind
	}{
		{"23503", persistence.ErrorInvalidDependency},
		{"23505", persistence.ErrorIdempotencyConflict},
		{"23514", persistence.ErrorInvalidInput},
		{"40001", persistence.ErrorUnavailable},
		{"42501", persistence.ErrorAuthorizationDenied},
	}
	for _, test := range cases {
		err := failure("adapter-test", &pgconn.PgError{Code: test.code, Message: "password=secret"})
		if persistence.KindOf(err) != test.kind || bytes.Contains([]byte(err.Error()), []byte("secret")) {
			t.Fatalf("code %s translated unsafely: %v", test.code, err)
		}
	}
	if persistence.KindOf(failure("adapter-test", context.Canceled)) != persistence.ErrorCanceled {
		t.Fatal("cancellation was not preserved")
	}
	if persistence.KindOf(failure("adapter-test", &pgconn.PgError{Code: "57014"})) != persistence.ErrorTimeout {
		t.Fatal("query cancellation was not mapped")
	}
	if persistence.KindOf(failure("adapter-test", fmt.Errorf("driver"))) != persistence.ErrorInternal {
		t.Fatal("unknown driver error leaked")
	}
}

func TestAdapterRejectsNonUUIDPhysicalIdentifiers(t *testing.T) {
	adapter, _ := New(stubDatabase{})
	contract, _ := persistence.New()
	scope, _ := persistence.NewScope("scope", "principal")
	actor, _ := persistence.NewAuditActor("test", "invalid-physical-id")
	source, _ := persistence.NewSourceIdentity("local", "sha256-v1", persistence.DigestBytes([]byte("source")))
	profile := persistence.DigestBytes([]byte("profile"))
	register, _ := contract.NewRegisterRepositoryRequest(persistence.RegisterRepositoryParams{Scope: scope, RequestID: "request", RepositoryID: "repository", DisplayName: "Repository", Source: source, Actor: actor})
	repositoryQuery, _ := contract.NewRepositoryQuery(scope, "repository")
	repositoryList, _ := contract.NewRepositoryListRequest(scope, 10, "")
	archive, _ := contract.NewArchiveRepositoryRequest(scope, "request", "repository", actor)
	begin, _ := contract.NewBeginScanRequest(persistence.BeginScanParams{Scope: scope, RequestID: "request", RepositoryID: "repository", ScanID: "scan", AnalysisProfileDigest: profile, Actor: actor})
	scanQuery, _ := contract.NewScanQuery(scope, "repository", "scan")
	scanList, _ := contract.NewScanListRequest(scope, "repository", 10, "")
	finish, _ := contract.NewFinishScanRequest(persistence.FinishScanParams{Scope: scope, RequestID: "request", RepositoryID: "repository", ScanID: "scan", ReasonCode: "failed", Actor: actor})
	stage, _ := contract.NewStagePayloadRequest(persistence.StagePayloadParams{Scope: scope, RequestID: "request", RepositoryID: "repository", ScanID: "scan", Digest: persistence.DigestBytes([]byte("payload")), ExpectedSize: 7})
	artifactName, _ := persistence.NewVersionedName("inventory", "1.0.0")
	codec, _ := persistence.NewCodec("json", "1.0.0", "application/json")
	artifact, _ := contract.NewArtifactSubmission(persistence.ArtifactSubmissionParams{ArtifactID: "artifact", Artifact: artifactName, Codec: codec, PayloadDigest: stage.Digest(), PayloadSize: 7, Producer: artifactName})
	publish, _ := contract.NewPublishScanRequest(persistence.PublishScanParams{Scope: scope, RequestID: "request", RepositoryID: "repository", ScanID: "scan", ManifestScheme: "artifact-manifest-sha256/v1", ManifestDigest: persistence.DigestBytes([]byte("manifest")), Artifacts: []persistence.ArtifactSubmission{artifact}, Actor: actor})
	artifactQuery, _ := contract.NewArtifactQuery(scope, "repository", "scan", "artifact")
	artifactList, _ := contract.NewArtifactListRequest(scope, "repository", "scan", 10, "")
	payloadQuery, _ := contract.NewPayloadQuery(scope, "repository", "scan", "artifact", stage.Digest())
	mark, _ := contract.NewMarkForPurgeRequest(scope, "request", "repository", actor)
	purge, _ := contract.NewPurgeBatchRequest(scope, "request", "repository", 10, actor)
	gc, _ := contract.NewGarbageCollectionRequest(scope, "request", 10, actor)
	checks := []error{}
	_, err := adapter.RegisterRepository(context.Background(), register)
	checks = append(checks, err)
	_, err = adapter.GetRepository(context.Background(), repositoryQuery)
	checks = append(checks, err)
	_, err = adapter.ListRepositories(context.Background(), repositoryList)
	checks = append(checks, err)
	_, err = adapter.ArchiveRepository(context.Background(), archive)
	checks = append(checks, err)
	_, err = adapter.BeginScan(context.Background(), begin)
	checks = append(checks, err)
	_, err = adapter.GetScan(context.Background(), scanQuery)
	checks = append(checks, err)
	_, err = adapter.ListScans(context.Background(), scanList)
	checks = append(checks, err)
	_, err = adapter.FailScan(context.Background(), finish)
	checks = append(checks, err)
	_, err = adapter.CancelScan(context.Background(), finish)
	checks = append(checks, err)
	_, err = adapter.StagePayload(context.Background(), stage, bytes.NewReader([]byte("payload")))
	checks = append(checks, err)
	_, err = adapter.PublishScan(context.Background(), publish)
	checks = append(checks, err)
	_, err = adapter.GetArtifact(context.Background(), artifactQuery)
	checks = append(checks, err)
	_, err = adapter.ListArtifacts(context.Background(), artifactList)
	checks = append(checks, err)
	_, err = adapter.ExportPayload(context.Background(), payloadQuery, &bytes.Buffer{})
	checks = append(checks, err)
	_, err = adapter.VerifyPayload(context.Background(), payloadQuery)
	checks = append(checks, err)
	_, err = adapter.MarkRepositoryForPurge(context.Background(), mark)
	checks = append(checks, err)
	_, err = adapter.PurgeRepositoryBatch(context.Background(), purge)
	checks = append(checks, err)
	_, err = adapter.GarbageCollectPayloads(context.Background(), gc)
	checks = append(checks, err)
	for index, err := range checks {
		if persistence.KindOf(err) != persistence.ErrorInvalidInput {
			t.Fatalf("operation %d: %v", index, err)
		}
	}
	if _, err := adapter.ExportPayload(context.Background(), payloadQuery, nil); persistence.KindOf(err) != persistence.ErrorInvalidInput {
		t.Fatalf("nil writer accepted: %v", err)
	}
}

func TestPostgresNeutralConformance(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		t.Skip("POSTGRES_TEST_URL is required for disposable PostgreSQL integration")
	}
	conformance.Run(t, conformance.FactoryFunc(func(ctx context.Context) (conformance.Fixture, conformance.Cleanup, error) {
		return newPostgresFixture(ctx, url)
	}))
}

func TestPostgresRollbackAndChunkedRoundTrip(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		t.Skip("POSTGRES_TEST_URL is required for disposable PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture, cleanup, err := newRunningFixture(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(context.Background())
	payload := make([]byte, ChunkSize+17)
	for index := range payload {
		payload[index] = byte(index % 251)
	}
	copy(payload, []byte(fixture.scanID))
	digest := persistence.DigestBytes(payload)
	request, _ := fixture.contract.NewStagePayloadRequest(persistence.StagePayloadParams{Scope: fixture.scope, RequestID: persistence.RequestID(newUUID()), RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, Digest: digest, ExpectedSize: persistence.ByteCount(len(payload))})
	if _, err := fixture.adapter.StagePayload(ctx, request, bytes.NewReader(payload[:len(payload)-1])); persistence.KindOf(err) != persistence.ErrorIntegrityFailure {
		t.Fatalf("short stream did not roll back: %v", err)
	}
	receipt, err := fixture.adapter.StagePayload(ctx, request, bytes.NewReader(payload))
	if err != nil || receipt.Disposition() != persistence.DispositionCreated {
		t.Fatalf("retry after rollback: %v", err)
	}
	var chunks int
	if err := fixture.pool.QueryRow(ctx, `SELECT chunk_count FROM platform.artifact_payloads WHERE payload_digest=$1`, digestBytes(digest)).Scan(&chunks); err != nil || chunks != 2 {
		t.Fatalf("ordered chunk contract: chunks=%d err=%v", chunks, err)
	}
}

func TestPostgresIdempotencyAtomicityAndIntegrity(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		t.Skip("POSTGRES_TEST_URL is required for disposable PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture, cleanup, err := newRunningFixture(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(context.Background())

	if _, err := fixture.adapter.RegisterRepository(ctx, fixture.register); err != nil {
		t.Fatalf("repository retry: %v", err)
	}
	conflictingRegister, _ := fixture.contract.NewRegisterRepositoryRequest(persistence.RegisterRepositoryParams{Scope: fixture.scope, RequestID: fixture.register.RequestID(), RepositoryID: fixture.repositoryID, DisplayName: "Changed", Source: fixture.register.Source(), Actor: fixture.actor})
	if _, err := fixture.adapter.RegisterRepository(ctx, conflictingRegister); persistence.KindOf(err) != persistence.ErrorIdempotencyConflict {
		t.Fatalf("repository conflict: %v", err)
	}
	if _, err := fixture.adapter.BeginScan(ctx, fixture.begin); err != nil {
		t.Fatalf("scan retry: %v", err)
	}
	conflictingBegin, _ := fixture.contract.NewBeginScanRequest(persistence.BeginScanParams{Scope: fixture.scope, RequestID: fixture.begin.RequestID(), RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, AnalysisProfileDigest: persistence.DigestBytes([]byte("different-profile")), SourceRevision: fixture.begin.SourceRevision(), Actor: fixture.actor})
	if _, err := fixture.adapter.BeginScan(ctx, conflictingBegin); persistence.KindOf(err) != persistence.ErrorIdempotencyConflict {
		t.Fatalf("scan conflict: %v", err)
	}
	payload := []byte(fmt.Sprintf(`{"complete":true,"scan":%q}`, fixture.scanID))
	digest := persistence.DigestBytes(payload)
	stage, _ := fixture.contract.NewStagePayloadRequest(persistence.StagePayloadParams{Scope: fixture.scope, RequestID: "stage-retry", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, Digest: digest, ExpectedSize: persistence.ByteCount(len(payload))})
	first, err := fixture.adapter.StagePayload(ctx, stage, bytes.NewReader(payload))
	if err != nil || first.Disposition() != persistence.DispositionCreated {
		t.Fatalf("first stage: %v", err)
	}
	second, err := fixture.adapter.StagePayload(ctx, stage, bytes.NewReader(payload))
	if err != nil || second.Disposition() != persistence.DispositionAlreadyPresent {
		t.Fatalf("stage retry: %v", err)
	}

	artifactName, _ := persistence.NewVersionedName("go-semantic-inventory", "1.0.0")
	producer, _ := persistence.NewVersionedName("go-semantic", "1.0.0")
	codec, _ := persistence.NewCodec("json", "1.0.0", "application/json")
	artifactID := persistence.ArtifactID(newUUID())
	missingDigest := persistence.DigestBytes([]byte("not-staged"))
	missing, _ := fixture.contract.NewArtifactSubmission(persistence.ArtifactSubmissionParams{ArtifactID: artifactID, Artifact: artifactName, Codec: codec, PayloadDigest: missingDigest, PayloadSize: 10, Producer: producer})
	invalidPublish, _ := fixture.contract.NewPublishScanRequest(persistence.PublishScanParams{Scope: fixture.scope, RequestID: "publish-invalid", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, ManifestScheme: "artifact-manifest-sha256/v1", ManifestDigest: persistence.DigestBytes([]byte("invalid-manifest")), Artifacts: []persistence.ArtifactSubmission{missing}, Actor: fixture.actor})
	if _, err := fixture.adapter.PublishScan(ctx, invalidPublish); persistence.KindOf(err) != persistence.ErrorNotFound {
		t.Fatalf("missing payload publication: %v", err)
	}
	query, _ := fixture.contract.NewScanQuery(fixture.scope, fixture.repositoryID, fixture.scanID)
	if scan, err := fixture.adapter.GetScan(ctx, query); err != nil || scan.State() != persistence.ScanRunning {
		t.Fatalf("failed publication was visible: %v", err)
	}

	artifact, _ := fixture.contract.NewArtifactSubmission(persistence.ArtifactSubmissionParams{ArtifactID: artifactID, Artifact: artifactName, StableIDScheme: "go-semantic-id/v1", Codec: codec, PayloadDigest: digest, PayloadSize: persistence.ByteCount(len(payload)), Producer: producer})
	publish, _ := fixture.contract.NewPublishScanRequest(persistence.PublishScanParams{Scope: fixture.scope, RequestID: "publish-valid", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, ManifestScheme: "artifact-manifest-sha256/v1", ManifestDigest: persistence.DigestBytes([]byte("valid-manifest")), Artifacts: []persistence.ArtifactSubmission{artifact}, MakeCurrent: true, Actor: fixture.actor})
	created, err := fixture.adapter.PublishScan(ctx, publish)
	if err != nil || created.Disposition() != persistence.DispositionCreated {
		t.Fatalf("valid publication: %v", err)
	}
	retry, err := fixture.adapter.PublishScan(ctx, publish)
	if err != nil || retry.Disposition() != persistence.DispositionAlreadyPresent {
		t.Fatalf("publication retry: %v", err)
	}
	conflictingPublish, _ := fixture.contract.NewPublishScanRequest(persistence.PublishScanParams{Scope: fixture.scope, RequestID: "publish-conflict", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, ManifestScheme: publish.ManifestScheme(), ManifestDigest: persistence.DigestBytes([]byte("different-manifest")), Artifacts: []persistence.ArtifactSubmission{artifact}, Actor: fixture.actor})
	if _, err := fixture.adapter.PublishScan(ctx, conflictingPublish); persistence.KindOf(err) != persistence.ErrorIdempotencyConflict {
		t.Fatalf("publication conflict: %v", err)
	}

	if _, err := fixture.pool.Exec(ctx, `UPDATE platform.artifact_payload_chunks SET chunk_bytes=set_byte(chunk_bytes,0,(get_byte(chunk_bytes,0)+1)%256) WHERE payload_digest=$1 AND chunk_ordinal=0`, digestBytes(digest)); err != nil {
		t.Fatal(err)
	}
	payloadQuery, _ := fixture.contract.NewPayloadQuery(fixture.scope, fixture.repositoryID, fixture.scanID, artifactID, digest)
	if _, err := fixture.adapter.VerifyPayload(ctx, payloadQuery); persistence.KindOf(err) != persistence.ErrorIntegrityFailure {
		t.Fatalf("corruption not detected: %v", err)
	}
}

func TestPostgresConcurrentStageAndRetention(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		t.Skip("POSTGRES_TEST_URL is required for disposable PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture, cleanup, err := newRunningFixture(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(context.Background())
	payload := append(bytes.Repeat([]byte("concurrent"), 1024), []byte(fixture.scanID)...)
	digest := persistence.DigestBytes(payload)
	request, _ := fixture.contract.NewStagePayloadRequest(persistence.StagePayloadParams{Scope: fixture.scope, RequestID: "concurrent-stage", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, Digest: digest, ExpectedSize: persistence.ByteCount(len(payload))})
	type result struct {
		receipt persistence.PayloadReceipt
		err     error
	}
	results := make(chan result, 2)
	for worker := 0; worker < 2; worker++ {
		go func() {
			receipt, err := fixture.adapter.StagePayload(ctx, request, bytes.NewReader(payload))
			results <- result{receipt, err}
		}()
	}
	created, present := 0, 0
	for worker := 0; worker < 2; worker++ {
		value := <-results
		if value.err != nil {
			t.Fatal(value.err)
		}
		if value.receipt.Disposition() == persistence.DispositionCreated {
			created++
		} else {
			present++
		}
	}
	if created != 1 || present != 1 {
		t.Fatalf("concurrent dispositions: created=%d existing=%d", created, present)
	}

	fail, _ := fixture.contract.NewFinishScanRequest(persistence.FinishScanParams{Scope: fixture.scope, RequestID: "finish-idempotent", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, ReasonCode: "validation", SafeMessage: "complete", Actor: fixture.actor})
	if _, err := fixture.adapter.FailScan(ctx, fail); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.adapter.FailScan(ctx, fail); err != nil {
		t.Fatalf("terminal retry: %v", err)
	}
	differentFinish, _ := fixture.contract.NewFinishScanRequest(persistence.FinishScanParams{Scope: fixture.scope, RequestID: "finish-different", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, ReasonCode: "different", SafeMessage: "different", Actor: fixture.actor})
	if _, err := fixture.adapter.FailScan(ctx, differentFinish); persistence.KindOf(err) != persistence.ErrorLifecycleConflict {
		t.Fatalf("terminal conflict: %v", err)
	}
	archive, _ := fixture.contract.NewArchiveRepositoryRequest(fixture.scope, "archive", fixture.repositoryID, fixture.actor)
	if _, err := fixture.adapter.ArchiveRepository(ctx, archive); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.adapter.ArchiveRepository(ctx, archive); err != nil {
		t.Fatalf("archive retry: %v", err)
	}
	mark, _ := fixture.contract.NewMarkForPurgeRequest(fixture.scope, "mark", fixture.repositoryID, fixture.actor)
	if _, err := fixture.adapter.MarkRepositoryForPurge(ctx, mark); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.adapter.MarkRepositoryForPurge(ctx, mark); err != nil {
		t.Fatalf("mark retry: %v", err)
	}
	purge, _ := fixture.contract.NewPurgeBatchRequest(fixture.scope, "purge", fixture.repositoryID, 10, fixture.actor)
	receipt, err := fixture.adapter.PurgeRepositoryBatch(ctx, purge)
	if err != nil || !receipt.Complete() || receipt.RemovedScans() != 1 {
		t.Fatalf("purge: %+v %v", receipt, err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE platform.artifact_payloads SET created_at=now()-interval '25 hours' WHERE payload_digest=$1`, digestBytes(digest)); err != nil {
		t.Fatal(err)
	}
	gc, _ := fixture.contract.NewGarbageCollectionRequest(fixture.scope, "gc", 10, fixture.actor)
	collected, err := fixture.adapter.GarbageCollectPayloads(ctx, gc)
	if err != nil || collected.RemovedPayloads() != 1 {
		t.Fatalf("garbage collection: %+v %v", collected, err)
	}
}

func TestPostgresCompletePublicationAndPagination(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		t.Skip("POSTGRES_TEST_URL is required for disposable PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture, cleanup, err := newRunningFixture(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(context.Background())

	producer, _ := persistence.NewVersionedName("go-semantic", "1.0.0")
	codec, _ := persistence.NewCodec("json", "1.0.0", "application/json")
	payloads := [][]byte{[]byte(`{"artifact":"one"}`), []byte(`{"artifact":"two"}`)}
	artifacts := make([]persistence.ArtifactSubmission, 2)
	for index, payload := range payloads {
		digest := persistence.DigestBytes(payload)
		stage, _ := fixture.contract.NewStagePayloadRequest(persistence.StagePayloadParams{Scope: fixture.scope, RequestID: persistence.RequestID(fmt.Sprintf("stage-%d", index)), RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, Digest: digest, ExpectedSize: persistence.ByteCount(len(payload))})
		if _, err := fixture.adapter.StagePayload(ctx, stage, bytes.NewReader(payload)); err != nil {
			t.Fatal(err)
		}
		name, _ := persistence.NewVersionedName(fmt.Sprintf("inventory-%d", index), "1.0.0")
		stableIDScheme := ""
		if index == 0 {
			stableIDScheme = "go-semantic-id/v1"
		}
		artifacts[index], _ = fixture.contract.NewArtifactSubmission(persistence.ArtifactSubmissionParams{ArtifactID: persistence.ArtifactID(newUUID()), Artifact: name, StableIDScheme: stableIDScheme, Codec: codec, PayloadDigest: digest, PayloadSize: persistence.ByteCount(len(payload)), Producer: producer})
	}
	dependency, _ := fixture.contract.NewDependencySubmission(artifacts[0].ArtifactID(), 0, artifacts[1].ArtifactID(), artifacts[1].Artifact())
	document := []byte(`{"packages":2}`)
	projection, _ := fixture.contract.NewProjectionSubmission(persistence.ProjectionSubmissionParams{ProjectionID: persistence.ProjectionID(newUUID()), ArtifactID: artifacts[0].ArtifactID(), SourceDigest: artifacts[0].PayloadDigest(), Projector: producer, SchemaVersion: "1.0.0", DigestScheme: "sha256-json-v1", ProjectionDigest: persistence.DigestBytes(document), CanonicalJSON: document, RecordCount: 2})
	diagnostic, _ := fixture.contract.NewDiagnosticSubmission(projection.ProjectionID(), 0, "warning", "unknown", "semantic", "main.go", 1, 2, "unknown value")
	decimal, _ := persistence.NewDecimalStatistic("12.50")
	textValue, _ := persistence.NewTextStatistic("stable")
	statistics := make([]persistence.StatisticSubmission, 0, 4)
	for _, item := range []struct {
		key   string
		value persistence.StatisticValue
	}{{"integer", persistence.NewIntegerStatistic(2)}, {"decimal", decimal}, {"boolean", persistence.NewBooleanStatistic(true)}, {"text", textValue}} {
		stat, _ := fixture.contract.NewStatisticSubmission(projection.ProjectionID(), item.key, item.value, "count")
		statistics = append(statistics, stat)
	}
	publish, _ := fixture.contract.NewPublishScanRequest(persistence.PublishScanParams{Scope: fixture.scope, RequestID: "complete-publish", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, ManifestScheme: "artifact-manifest-sha256/v1", ManifestDigest: persistence.DigestBytes([]byte("complete-manifest")), Artifacts: artifacts, Dependencies: []persistence.DependencySubmission{dependency}, Projections: []persistence.ProjectionSubmission{projection}, Diagnostics: []persistence.DiagnosticSubmission{diagnostic}, Statistics: statistics, MakeCurrent: true, Actor: fixture.actor})
	if _, err := fixture.adapter.PublishScan(ctx, publish); err != nil {
		t.Fatal(err)
	}

	artifactList, _ := fixture.contract.NewArtifactListRequest(fixture.scope, fixture.repositoryID, fixture.scanID, 1, "")
	page, err := fixture.adapter.ListArtifacts(ctx, artifactList)
	if err != nil || len(page.Records()) != 1 || page.NextCursor() == "" {
		t.Fatalf("artifact page: %v", err)
	}
	artifactList, _ = fixture.contract.NewArtifactListRequest(fixture.scope, fixture.repositoryID, fixture.scanID, 1, page.NextCursor())
	page, err = fixture.adapter.ListArtifacts(ctx, artifactList)
	if err != nil || len(page.Records()) != 1 {
		t.Fatalf("artifact next page: %v", err)
	}

	secondRepository := persistence.RepositoryID(newUUID())
	source := sourceFor(secondRepository)
	register, _ := fixture.contract.NewRegisterRepositoryRequest(persistence.RegisterRepositoryParams{Scope: fixture.scope, RequestID: "second-repository", RepositoryID: secondRepository, DisplayName: "Second", Source: source, Actor: fixture.actor})
	if _, err := fixture.adapter.RegisterRepository(ctx, register); err != nil {
		t.Fatal(err)
	}
	sameSourceRepository := persistence.RepositoryID(newUUID())
	sameSourceRequest, _ := fixture.contract.NewRegisterRepositoryRequest(persistence.RegisterRepositoryParams{Scope: fixture.scope, RequestID: "same-source", RepositoryID: sameSourceRepository, DisplayName: "Duplicate source", Source: source, Actor: fixture.actor})
	if _, err := fixture.adapter.RegisterRepository(ctx, sameSourceRequest); persistence.KindOf(err) != persistence.ErrorIdempotencyConflict {
		t.Fatalf("same-scope source conflict: %v", err)
	}
	repositoryList, _ := fixture.contract.NewRepositoryListRequest(fixture.scope, 1, "")
	repositoryPage, err := fixture.adapter.ListRepositories(ctx, repositoryList)
	if err != nil || len(repositoryPage.Records()) != 1 || repositoryPage.NextCursor() == "" {
		t.Fatalf("repository page: %v", err)
	}
	repositoryList, _ = fixture.contract.NewRepositoryListRequest(fixture.scope, 1, repositoryPage.NextCursor())
	if page, err := fixture.adapter.ListRepositories(ctx, repositoryList); err != nil || len(page.Records()) != 1 {
		t.Fatalf("repository next page: %v", err)
	}
	invalidRepositoryCursor, _ := fixture.contract.NewRepositoryListRequest(fixture.scope, 1, "*")
	if _, err := fixture.adapter.ListRepositories(ctx, invalidRepositoryCursor); persistence.KindOf(err) != persistence.ErrorInvalidInput {
		t.Fatalf("invalid repository cursor: %v", err)
	}

	secondScan := persistence.ScanID(newUUID())
	begin, _ := fixture.contract.NewBeginScanRequest(persistence.BeginScanParams{Scope: fixture.scope, RequestID: "second-scan", RepositoryID: fixture.repositoryID, ScanID: secondScan, AnalysisProfileDigest: profileFor(secondScan), SourceRevision: "second", Actor: fixture.actor})
	if _, err := fixture.adapter.BeginScan(ctx, begin); err != nil {
		t.Fatal(err)
	}
	scanList, _ := fixture.contract.NewScanListRequest(fixture.scope, fixture.repositoryID, 1, "")
	scanPage, err := fixture.adapter.ListScans(ctx, scanList)
	if err != nil || len(scanPage.Records()) != 1 || scanPage.NextCursor() == "" {
		t.Fatalf("scan page: %v", err)
	}
	scanList, _ = fixture.contract.NewScanListRequest(fixture.scope, fixture.repositoryID, 1, scanPage.NextCursor())
	if page, err := fixture.adapter.ListScans(ctx, scanList); err != nil || len(page.Records()) != 1 {
		t.Fatalf("scan next page: %v", err)
	}
	invalidScanCursor, _ := fixture.contract.NewScanListRequest(fixture.scope, fixture.repositoryID, 1, "*")
	if _, err := fixture.adapter.ListScans(ctx, invalidScanCursor); persistence.KindOf(err) != persistence.ErrorInvalidInput {
		t.Fatalf("invalid scan cursor: %v", err)
	}
	invalidArtifactCursor, _ := fixture.contract.NewArtifactListRequest(fixture.scope, fixture.repositoryID, fixture.scanID, 1, "*")
	if _, err := fixture.adapter.ListArtifacts(ctx, invalidArtifactCursor); persistence.KindOf(err) != persistence.ErrorInvalidInput {
		t.Fatalf("invalid artifact cursor: %v", err)
	}

	var dependencyCount, diagnosticCount, statisticCount int
	if err := fixture.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM platform.artifact_dependencies WHERE scan_id=$1::uuid),(SELECT count(*) FROM platform.projected_diagnostics WHERE projection_id=$2::uuid),(SELECT count(*) FROM platform.projected_statistics WHERE projection_id=$2::uuid)`, string(fixture.scanID), string(projection.ProjectionID())).Scan(&dependencyCount, &diagnosticCount, &statisticCount); err != nil || dependencyCount != 1 || diagnosticCount != 1 || statisticCount != 4 {
		t.Fatalf("complete projection persistence: %v", err)
	}
	markActive, _ := fixture.contract.NewMarkForPurgeRequest(fixture.scope, "mark-active", secondRepository, fixture.actor)
	if _, err := fixture.adapter.MarkRepositoryForPurge(ctx, markActive); persistence.KindOf(err) != persistence.ErrorLifecycleConflict {
		t.Fatalf("active purge mark: %v", err)
	}
	archiveSecond, _ := fixture.contract.NewArchiveRepositoryRequest(fixture.scope, "archive-second", secondRepository, fixture.actor)
	if _, err := fixture.adapter.ArchiveRepository(ctx, archiveSecond); err != nil {
		t.Fatal(err)
	}
	archivedScan := persistence.ScanID(newUUID())
	archivedBegin, _ := fixture.contract.NewBeginScanRequest(persistence.BeginScanParams{Scope: fixture.scope, RequestID: "archived-scan", RepositoryID: secondRepository, ScanID: archivedScan, AnalysisProfileDigest: profileFor(archivedScan), Actor: fixture.actor})
	if _, err := fixture.adapter.BeginScan(ctx, archivedBegin); persistence.KindOf(err) != persistence.ErrorLifecycleConflict {
		t.Fatalf("scan on archived repository: %v", err)
	}
}

func TestPostgresReleasedScalePayload(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_URL")
	declared := os.Getenv("POSTGRES_LARGE_TEST_BYTES")
	if url == "" || declared == "" {
		t.Skip("disposable PostgreSQL URL and explicit large byte count are required")
	}
	size, err := strconv.ParseInt(declared, 10, 64)
	if err != nil || size <= ChunkSize {
		t.Fatalf("invalid large payload size %q", declared)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	fixture, cleanup, err := newRunningFixture(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(context.Background())
	hasher := sha256.New()
	if _, err := io.CopyN(hasher, zeroReader{}, size); err != nil {
		t.Fatal(err)
	}
	var digest persistence.Digest
	copy(digest[:], hasher.Sum(nil))
	stage, _ := fixture.contract.NewStagePayloadRequest(persistence.StagePayloadParams{Scope: fixture.scope, RequestID: "released-scale-stage", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, Digest: digest, ExpectedSize: persistence.ByteCount(size)})
	started := time.Now()
	receipt, err := fixture.adapter.StagePayload(ctx, stage, io.LimitReader(zeroReader{}, size))
	stageDuration := time.Since(started)
	if err != nil || receipt.Size() != persistence.ByteCount(size) {
		t.Fatalf("large stage: %v", err)
	}
	artifactName, _ := persistence.NewVersionedName("go-semantic-inventory", "1.0.0")
	producer, _ := persistence.NewVersionedName("go-semantic", "1.0.0")
	codec, _ := persistence.NewCodec("json", "1.0.0", "application/json")
	artifactID := persistence.ArtifactID(newUUID())
	artifact, _ := fixture.contract.NewArtifactSubmission(persistence.ArtifactSubmissionParams{ArtifactID: artifactID, Artifact: artifactName, StableIDScheme: "go-semantic-id/v1", Codec: codec, PayloadDigest: digest, PayloadSize: persistence.ByteCount(size), Producer: producer})
	publish, _ := fixture.contract.NewPublishScanRequest(persistence.PublishScanParams{Scope: fixture.scope, RequestID: "released-scale-publish", RepositoryID: fixture.repositoryID, ScanID: fixture.scanID, ManifestScheme: "artifact-manifest-sha256/v1", ManifestDigest: persistence.DigestBytes([]byte("released-scale-manifest")), Artifacts: []persistence.ArtifactSubmission{artifact}, Actor: fixture.actor})
	if _, err := fixture.adapter.PublishScan(ctx, publish); err != nil {
		t.Fatal(err)
	}
	query, _ := fixture.contract.NewPayloadQuery(fixture.scope, fixture.repositoryID, fixture.scanID, artifactID, digest)
	started = time.Now()
	if _, err := fixture.adapter.ExportPayload(ctx, query, io.Discard); err != nil {
		t.Fatal(err)
	}
	readDuration := time.Since(started)
	var chunks int
	if err := fixture.pool.QueryRow(ctx, `SELECT chunk_count FROM platform.artifact_payloads WHERE payload_digest=$1`, digestBytes(digest)).Scan(&chunks); err != nil || chunks != chunkCount(persistence.ByteCount(size)) {
		t.Fatalf("large chunk count: %d %v", chunks, err)
	}
	t.Logf("bytes=%d chunks=%d stage=%s stage_mib_s=%.2f read=%s read_mib_s=%.2f", size, chunks, stageDuration, float64(size)/(1<<20)/stageDuration.Seconds(), readDuration, float64(size)/(1<<20)/readDuration.Seconds())
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) { clear(buffer); return len(buffer), nil }

type stubDatabase struct{}

func (stubDatabase) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, fmt.Errorf("stub")
}
func (stubDatabase) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, fmt.Errorf("stub")
}
func (stubDatabase) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("stub")
}
func (stubDatabase) QueryRow(context.Context, string, ...any) pgx.Row { return stubRow{} }

type stubRow struct{}

func (stubRow) Scan(...any) error { return fmt.Errorf("stub") }

type runningFixture struct {
	pool         *pgxpool.Pool
	adapter      *Adapter
	contract     *persistence.Contract
	scope        persistence.Scope
	repositoryID persistence.RepositoryID
	scanID       persistence.ScanID
	actor        persistence.AuditActor
	register     persistence.RegisterRepositoryRequest
	begin        persistence.BeginScanRequest
}

func newPostgresFixture(ctx context.Context, url string) (conformance.Fixture, conformance.Cleanup, error) {
	running, cleanup, err := newRunningFixture(ctx, url)
	if err != nil {
		return conformance.Fixture{}, nil, err
	}
	payload := []byte(`{"artifact":"semantic"}`)
	digest := persistence.DigestBytes(payload)
	stage, _ := running.contract.NewStagePayloadRequest(persistence.StagePayloadParams{Scope: running.scope, RequestID: persistence.RequestID(newUUID()), RepositoryID: running.repositoryID, ScanID: running.scanID, Digest: digest, ExpectedSize: persistence.ByteCount(len(payload))})
	if _, err := running.adapter.StagePayload(ctx, stage, bytes.NewReader(payload)); err != nil {
		cleanup(ctx)
		return conformance.Fixture{}, nil, err
	}
	artifactName, _ := persistence.NewVersionedName("go-semantic-inventory", "1.0.0")
	producer, _ := persistence.NewVersionedName("go-semantic", "1.0.0")
	codec, _ := persistence.NewCodec("json", "1.0.0", "application/json")
	artifactID := persistence.ArtifactID(newUUID())
	artifact, _ := running.contract.NewArtifactSubmission(persistence.ArtifactSubmissionParams{ArtifactID: artifactID, Artifact: artifactName, StableIDScheme: "go-semantic-id/v1", Codec: codec, PayloadDigest: digest, PayloadSize: persistence.ByteCount(len(payload)), Producer: producer})
	actor, _ := persistence.NewAuditActor("test", "postgres-conformance")
	publish, _ := running.contract.NewPublishScanRequest(persistence.PublishScanParams{Scope: running.scope, RequestID: persistence.RequestID(newUUID()), RepositoryID: running.repositoryID, ScanID: running.scanID, ManifestScheme: "artifact-manifest-sha256/v1", ManifestDigest: persistence.DigestBytes([]byte("manifest" + string(running.scanID))), Artifacts: []persistence.ArtifactSubmission{artifact}, MakeCurrent: true, Actor: actor})
	if _, err := running.adapter.PublishScan(ctx, publish); err != nil {
		cleanup(ctx)
		return conformance.Fixture{}, nil, err
	}
	other, _ := persistence.NewScope(newUUID(), newUUID())
	scenario := conformance.Scenario{PrimaryScope: running.scope, OtherScope: other, RepositoryID: running.repositoryID, ScanID: running.scanID, ArtifactID: artifactID, Artifact: artifactName, Producer: producer, Codec: codec, Source: sourceFor(running.repositoryID), AnalysisProfileDigest: profileFor(running.scanID), ManifestScheme: publish.ManifestScheme(), Digest: digest, Payload: payload, Actor: actor}
	return conformance.Fixture{Port: running.adapter, Contract: running.contract, Scenario: scenario}, cleanup, nil
}

func newRunningFixture(ctx context.Context, url string) (*runningFixture, conformance.Cleanup, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, nil, err
	}
	adapter, err := New(pool)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	contract, _ := persistence.New()
	scope, _ := persistence.NewScope(newUUID(), newUUID())
	repositoryID := persistence.RepositoryID(newUUID())
	scanID := persistence.ScanID(newUUID())
	actor, _ := persistence.NewAuditActor("test", "postgres-integration")
	register, _ := contract.NewRegisterRepositoryRequest(persistence.RegisterRepositoryParams{Scope: scope, RequestID: persistence.RequestID(newUUID()), RepositoryID: repositoryID, DisplayName: "PostgreSQL conformance", Source: sourceFor(repositoryID), Actor: actor})
	if _, err := adapter.RegisterRepository(ctx, register); err != nil {
		pool.Close()
		return nil, nil, err
	}
	begin, _ := contract.NewBeginScanRequest(persistence.BeginScanParams{Scope: scope, RequestID: persistence.RequestID(newUUID()), RepositoryID: repositoryID, ScanID: scanID, AnalysisProfileDigest: profileFor(scanID), SourceRevision: "integration-revision", Actor: actor})
	if _, err := adapter.BeginScan(ctx, begin); err != nil {
		pool.Close()
		return nil, nil, err
	}
	cleanup := func(context.Context) error { pool.Close(); return nil }
	return &runningFixture{pool: pool, adapter: adapter, contract: contract, scope: scope, repositoryID: repositoryID, scanID: scanID, actor: actor, register: register, begin: begin}, cleanup, nil
}

func sourceFor(repositoryID persistence.RepositoryID) persistence.SourceIdentity {
	source, _ := persistence.NewSourceIdentity("local", "sha256-v1", persistence.DigestBytes([]byte(repositoryID)))
	return source
}

func profileFor(scanID persistence.ScanID) persistence.Digest {
	return persistence.DigestBytes([]byte("profile:" + string(scanID)))
}

func newUUID() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}
