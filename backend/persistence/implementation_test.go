package persistence

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDefaultConfigAndBounds(t *testing.T) {
	config := DefaultConfig()
	if err := config.Validate(); err != nil {
		t.Fatalf("default configuration must validate: %v", err)
	}
	if config.MaxPayloadBytes != 4<<30 {
		t.Fatalf("unexpected operational limit: %d", config.MaxPayloadBytes)
	}
	invalid := []Config{
		{MaxPayloadBytes: 8<<30 + 1},
		{MaxArtifactsPerPublication: 257},
		{MaxDependenciesPerArtifact: 4_097},
		{MaxProjectionBytes: 8<<20 + 1},
		{MaxDiagnostics: 10_001},
		{MaxStatistics: 10_001},
		{MaxPageSize: 1_001},
		{MaxRetentionBatch: 1_001},
	}
	for _, candidate := range invalid {
		if _, err := New(candidate); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("expected invalid config for %+v, got %v", candidate, err)
		}
	}
	if _, err := New(Config{}, Config{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected multiple config rejection, got %v", err)
	}
}

func TestNeutralValues(t *testing.T) {
	scope := mustScope(t, "scope-a", "principal-a")
	if scope.ScopeID() != "scope-a" || scope.PrincipalID() != "principal-a" {
		t.Fatal("scope accessors changed values")
	}
	if _, err := NewScope("bad scope", "principal"); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("expected invalid scope, got %v", err)
	}
	name := mustName(t, "go-semantic-inventory", "1.0.0")
	if name.Name() != "go-semantic-inventory" || name.Version() != "1.0.0" {
		t.Fatal("versioned name changed values")
	}
	codec, err := NewCodec("json", "1.0.0", "application/json")
	if err != nil || codec.IsZero() {
		t.Fatalf("codec failed: %v", err)
	}
	digest := DigestBytes([]byte("payload"))
	parsed, err := ParseDigest(strings.ToUpper(digest.String()))
	if err != nil || parsed != digest {
		t.Fatalf("digest round trip failed: %v", err)
	}
	if _, err := ParseDigest("short"); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("invalid digest was accepted: %v", err)
	}
}

func TestRequestValidation(t *testing.T) {
	contract := mustContract(t)
	scope := mustScope(t, "scope-a", "principal-a")
	actor := mustActor(t)
	profile := DigestBytes([]byte("analysis-profile"))
	source := mustSource(t)

	repository, err := contract.NewRegisterRepositoryRequest(RegisterRepositoryParams{
		Scope: scope, RequestID: "request-1", RepositoryID: "repository-1",
		DisplayName: "Repository", Source: source, Actor: actor,
	})
	if err != nil || repository.RepositoryID() != "repository-1" {
		t.Fatalf("repository request failed: %v", err)
	}
	if _, err := contract.NewRepositoryListRequest(scope, 0, ""); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("zero page size accepted: %v", err)
	}
	begin, err := contract.NewBeginScanRequest(BeginScanParams{Scope: scope, RequestID: "request-2", RepositoryID: "repository-1", ScanID: "scan-1", AnalysisProfileDigest: profile, SourceRevision: "revision-1", Actor: actor})
	if err != nil || begin.ScanID() != "scan-1" {
		t.Fatalf("begin scan failed: %v", err)
	}
	if _, err := contract.NewStagePayloadRequest(StagePayloadParams{Scope: scope, RequestID: "request-3", RepositoryID: "repository-1", ScanID: "scan-1", Digest: DigestBytes([]byte("x")), ExpectedSize: 4<<30 + 1}); KindOf(err) != ErrorPayloadTooLarge {
		t.Fatalf("oversized payload accepted: %v", err)
	}
	if _, err := contract.NewFinishScanRequest(FinishScanParams{Scope: scope, RequestID: "request-4", RepositoryID: "repository-1", ScanID: "scan-1", ReasonCode: "failed", SafeMessage: "safe", Actor: actor}); err != nil {
		t.Fatalf("finish request failed: %v", err)
	}
}

func TestPublicationDefensiveCopiesAndValidation(t *testing.T) {
	contract := mustContract(t)
	scope := mustScope(t, "scope-a", "principal-a")
	actor := mustActor(t)
	artifactName := mustName(t, "go-semantic-inventory", "1.0.0")
	producer := mustName(t, "go-semantic", "1.0.0")
	codec, _ := NewCodec("json", "1.0.0", "application/json")
	payloadDigest := DigestBytes([]byte("payload"))
	artifact, err := contract.NewArtifactSubmission(ArtifactSubmissionParams{
		ArtifactID: "artifact-1", Artifact: artifactName, Codec: codec,
		PayloadDigest: payloadDigest, PayloadSize: 7, Producer: producer,
		StableIDScheme: "go-semantic-id/v1",
	})
	if err != nil {
		t.Fatalf("artifact failed: %v", err)
	}
	document := []byte(`{"packages":1}`)
	projectionDigest := DigestBytes(document)
	projection, err := contract.NewProjectionSubmission(ProjectionSubmissionParams{
		ProjectionID: "projection-1", ArtifactID: "artifact-1", SourceDigest: payloadDigest,
		Projector: mustName(t, "summary-projector", "1.0.0"), SchemaVersion: "1.0.0",
		DigestScheme: "sha256-json-v1", ProjectionDigest: projectionDigest,
		CanonicalJSON: document, RecordCount: 1,
	})
	if err != nil {
		t.Fatalf("projection failed: %v", err)
	}
	document[0] = 'x'
	if projection.CanonicalJSON()[0] != '{' {
		t.Fatal("projection retained caller bytes")
	}
	diagnostic, err := contract.NewDiagnosticSubmission("projection-1", 0, "warning", "unknown-value", "go-semantic", "backend/main.go", 1, 1, "value is unknown")
	if err != nil {
		t.Fatalf("diagnostic failed: %v", err)
	}
	statistic, err := contract.NewStatisticSubmission("projection-1", "packages", NewIntegerStatistic(1), "count")
	if err != nil {
		t.Fatalf("statistic failed: %v", err)
	}

	artifacts := []ArtifactSubmission{artifact}
	projections := []ProjectionSubmission{projection}
	request, err := contract.NewPublishScanRequest(PublishScanParams{
		Scope: scope, RequestID: "request-4", RepositoryID: "repository-1",
		ScanID: "scan-1", ManifestScheme: "artifact-manifest-sha256/v1", ManifestDigest: DigestBytes([]byte("manifest")),
		Artifacts: artifacts, Projections: projections, Diagnostics: []DiagnosticSubmission{diagnostic},
		Statistics: []StatisticSubmission{statistic}, MakeCurrent: true, Actor: actor,
	})
	if err != nil {
		t.Fatalf("publish request failed: %v", err)
	}
	artifacts[0] = ArtifactSubmission{}
	projections[0] = ProjectionSubmission{}
	returned := request.Artifacts()
	returned[0] = ArtifactSubmission{}
	if request.Artifacts()[0].ArtifactID() != "artifact-1" || request.Projections()[0].ProjectionID() != "projection-1" {
		t.Fatal("publish request is not detached")
	}
}

func TestPublicationRejectsInvalidGraph(t *testing.T) {
	contract := mustContract(t)
	artifact := mustArtifact(t, contract, "artifact-1", "inventory-a", []byte("a"))
	duplicate := artifact
	params := basePublishParams(t, artifact, duplicate)
	if _, err := contract.NewPublishScanRequest(params); KindOf(err) != ErrorDuplicateArtifact {
		t.Fatalf("duplicate artifact accepted: %v", err)
	}

	other := mustArtifact(t, contract, "artifact-2", "inventory-b", []byte("b"))
	wrongDeclared := mustName(t, "wrong", "1.0.0")
	dependency, err := contract.NewDependencySubmission("artifact-1", 0, "artifact-2", wrongDeclared)
	if err != nil {
		t.Fatalf("dependency construction failed unexpectedly: %v", err)
	}
	params = basePublishParams(t, artifact, other)
	params.Dependencies = []DependencySubmission{dependency}
	if _, err := contract.NewPublishScanRequest(params); KindOf(err) != ErrorInvalidDependency {
		t.Fatalf("wrong declared dependency accepted: %v", err)
	}
}

func TestProjectionDiagnosticAndStatisticValidation(t *testing.T) {
	contract := mustContract(t)
	digest := DigestBytes([]byte(`{}`))
	if _, err := contract.NewProjectionSubmission(ProjectionSubmissionParams{
		ProjectionID: "projection", ArtifactID: "artifact", SourceDigest: digest,
		Projector: mustName(t, "projector", "1.0.0"), SchemaVersion: "1.0.0",
		DigestScheme: "sha256-json-v1", ProjectionDigest: digest, CanonicalJSON: []byte(`not-json`),
	}); KindOf(err) != ErrorIntegrityFailure {
		t.Fatalf("invalid projection accepted: %v", err)
	}
	if _, err := contract.NewDiagnosticSubmission("projection", 0, "warning", "code", "engine", `C:\secret\file.go`, 1, 1, "safe"); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("absolute path accepted: %v", err)
	}
	if _, err := NewDecimalStatistic("NaN"); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("non-exact decimal accepted: %v", err)
	}
	value, err := NewTextStatistic("text")
	if err != nil || value.Kind() != StatisticText {
		t.Fatalf("text statistic failed: %v", err)
	}
}

func TestRecordsPagesAndReceiptsAreDetached(t *testing.T) {
	now := time.Now().UTC()
	record, err := NewRepositoryRecord("scope-a", "repository-1", "Repository", mustSource(t), RepositoryActive, "", now, now)
	if err != nil {
		t.Fatalf("repository record failed: %v", err)
	}
	records := []RepositoryRecord{record}
	page := NewRepositoryPage(records, "next")
	records[0] = RepositoryRecord{}
	copy := page.Records()
	copy[0] = RepositoryRecord{}
	if page.Records()[0].RepositoryID() != "repository-1" || page.NextCursor() != "next" {
		t.Fatal("repository page is not detached")
	}
	digest := DigestBytes([]byte("payload"))
	receipt, err := NewPayloadReceipt(digest, 7, DispositionCreated)
	if err != nil || receipt.Digest() != digest || receipt.Size() != 7 {
		t.Fatalf("payload receipt failed: %v", err)
	}
}

func TestSafeErrorsPreserveCancellation(t *testing.T) {
	secret := errors.New("password=do-not-print")
	err := NewError(ErrorUnavailable, "stage-payload", true, secret)
	if strings.Contains(err.Error(), "password") || KindOf(err) != ErrorUnavailable || !IsRetryable(err) {
		t.Fatalf("unsafe or incorrect error: %v", err)
	}
	cancelled := NewError(ErrorInternal, "publish", true, context.Canceled)
	if KindOf(cancelled) != ErrorCanceled || !errors.Is(cancelled, context.Canceled) || IsRetryable(cancelled) {
		t.Fatalf("cancellation mapping failed: %v", cancelled)
	}
	deadline := NewError(ErrorInternal, "read", false, context.DeadlineExceeded)
	if KindOf(deadline) != ErrorTimeout || !errors.Is(deadline, context.DeadlineExceeded) || !IsRetryable(deadline) {
		t.Fatalf("deadline mapping failed: %v", deadline)
	}
	unsafe := NewError(ErrorKind("unknown"), "password-leak", true, secret)
	var typed *Error
	if !errors.As(unsafe, &typed) || typed.Kind() != ErrorInternal || typed.Operation() != "password-leak" || errors.Is(unsafe, secret) {
		t.Fatalf("unknown error mapping or cause hiding failed: %v", unsafe)
	}
	unsafeOperation := NewError(ErrorInternal, "PASSWORD=secret", false, nil)
	if unsafeOperation.Error() != "persistence: internal" {
		t.Fatalf("unsafe operation leaked: %v", unsafeOperation)
	}
	if KindOf(nil) != "" || KindOf(context.Canceled) != ErrorCanceled || KindOf(context.DeadlineExceeded) != ErrorTimeout || KindOf(secret) != ErrorInternal {
		t.Fatal("direct error classification failed")
	}
	var nilFailure *Error
	consume(nilFailure.Error(), nilFailure.Unwrap(), nilFailure.Kind(), nilFailure.Operation(), nilFailure.Retryable())
}

func TestConcurrentImmutableReads(t *testing.T) {
	contract := mustContract(t)
	artifact := mustArtifact(t, contract, "artifact-1", "inventory", []byte("payload"))
	request, err := contract.NewPublishScanRequest(basePublishParams(t, artifact))
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 1_000; iteration++ {
				values := request.Artifacts()
				values[0] = ArtifactSubmission{}
				if request.Artifacts()[0].ArtifactID() != "artifact-1" {
					t.Error("immutable request changed")
					return
				}
			}
		}()
	}
	wait.Wait()
}

func TestCompletePublicModelSurface(t *testing.T) {
	contract := mustContract(t)
	if contract.Config() != DefaultConfig() {
		t.Fatal("effective configuration mismatch")
	}
	scope := mustScope(t, "scope-model", "principal-model")
	actor := mustActor(t)
	producer := mustName(t, "producer", "1.2.3")
	codec, _ := NewCodec("json", "1.0.0", "application/json")
	if codec.Name() != "json" || codec.Version() != "1.0.0" || codec.MediaType() != "application/json" {
		t.Fatal("codec accessors failed")
	}
	source := mustSource(t)
	if source.Kind() != "local" || source.FingerprintScheme() != "sha256-v1" || source.Fingerprint().IsZero() || actor.Kind() != "service" || actor.ID() != "test-runner" {
		t.Fatal("source identity or actor accessors failed")
	}

	register, _ := contract.NewRegisterRepositoryRequest(RegisterRepositoryParams{Scope: scope, RequestID: "req-register", RepositoryID: "repo-model", DisplayName: "Model", Source: source, Actor: actor})
	consume(register.Scope(), register.RequestID(), register.RepositoryID(), register.DisplayName(), register.Source(), register.Actor())
	repositoryQuery, _ := contract.NewRepositoryQuery(scope, "repo-model")
	consume(repositoryQuery.Scope(), repositoryQuery.RepositoryID())
	repositoryList, _ := contract.NewRepositoryListRequest(scope, 10, "cursor")
	consume(repositoryList.Scope(), repositoryList.PageSize(), repositoryList.Cursor())
	archive, _ := contract.NewArchiveRepositoryRequest(scope, "req-archive", "repo-model", actor)
	consume(archive.Scope(), archive.RequestID(), archive.RepositoryID(), archive.Actor())

	profile := DigestBytes([]byte("profile-model"))
	begin, _ := contract.NewBeginScanRequest(BeginScanParams{Scope: scope, RequestID: "req-begin", RepositoryID: "repo-model", ScanID: "scan-model", AnalysisProfileDigest: profile, SourceRevision: "revision-model", Actor: actor})
	consume(begin.Scope(), begin.RequestID(), begin.RepositoryID(), begin.ScanID(), begin.AnalysisProfileDigest(), begin.SourceRevision(), begin.Actor())
	scanQuery, _ := contract.NewScanQuery(scope, "repo-model", "scan-model")
	consume(scanQuery.Scope(), scanQuery.RepositoryID(), scanQuery.ScanID())
	scanList, _ := contract.NewScanListRequest(scope, "repo-model", 10, "cursor")
	consume(scanList.Scope(), scanList.RepositoryID(), scanList.PageSize(), scanList.Cursor())
	finish, _ := contract.NewFinishScanRequest(FinishScanParams{Scope: scope, RequestID: "req-finish", RepositoryID: "repo-model", ScanID: "scan-model", ReasonCode: "failed", SafeMessage: "safe", Actor: actor})
	consume(finish.Scope(), finish.RequestID(), finish.RepositoryID(), finish.ScanID(), finish.ReasonCode(), finish.SafeMessage(), finish.Actor())

	payload := []byte("payload-model")
	digest := DigestBytes(payload)
	stage, _ := contract.NewStagePayloadRequest(StagePayloadParams{Scope: scope, RequestID: "req-stage", RepositoryID: "repo-model", ScanID: "scan-model", Digest: digest, ExpectedSize: ByteCount(len(payload))})
	consume(stage.Scope(), stage.RequestID(), stage.RepositoryID(), stage.ScanID(), stage.Digest(), stage.ExpectedSize())

	artifactA := mustArtifact(t, contract, "artifact-a", "inventory-a", payload)
	artifactB := mustArtifact(t, contract, "artifact-b", "inventory-b", []byte("payload-b"))
	consume(artifactA.ArtifactID(), artifactA.Artifact(), artifactA.StableIDScheme(), artifactA.Codec(), artifactA.PayloadDigest(), artifactA.PayloadSize(), artifactA.Producer())
	dependency, _ := contract.NewDependencySubmission("artifact-a", 0, "artifact-b", artifactB.Artifact())
	consume(dependency.ConsumerArtifactID(), dependency.Ordinal(), dependency.SourceArtifactID(), dependency.DeclaredArtifact())

	document := []byte(`{"count":2}`)
	projectionDigest := DigestBytes(document)
	projection, _ := contract.NewProjectionSubmission(ProjectionSubmissionParams{ProjectionID: "projection-model", ArtifactID: "artifact-a", SourceDigest: artifactA.PayloadDigest(), Projector: mustName(t, "projector", "1.0.0"), SchemaVersion: "1.0.0", DigestScheme: "sha256-json-v1", ProjectionDigest: projectionDigest, CanonicalJSON: document, RecordCount: 2})
	consume(projection.ProjectionID(), projection.ArtifactID(), projection.SourceDigest(), projection.Projector(), projection.SchemaVersion(), projection.DigestScheme(), projection.ProjectionDigest(), projection.CanonicalJSON(), projection.RecordCount())
	diagnostic, _ := contract.NewDiagnosticSubmission("projection-model", 0, "info", "code", "engine", "file.go", 2, 3, "message")
	consume(diagnostic.ProjectionID(), diagnostic.Ordinal(), diagnostic.Severity(), diagnostic.Code(), diagnostic.Engine(), diagnostic.RelativePath(), diagnostic.Line(), diagnostic.Column(), diagnostic.Message())
	decimal, _ := NewDecimalStatistic("12.50")
	boolean := NewBooleanStatistic(true)
	text, _ := NewTextStatistic("text")
	consume(NewIntegerStatistic(4).Integer(), decimal.Decimal(), boolean.Boolean(), text.Text())
	statistic, _ := contract.NewStatisticSubmission("projection-model", "count", decimal, "items")
	consume(statistic.ProjectionID(), statistic.Key(), statistic.Value(), statistic.Unit())

	publish, err := contract.NewPublishScanRequest(PublishScanParams{Scope: scope, RequestID: "req-publish", RepositoryID: "repo-model", ScanID: "scan-model", ManifestScheme: "artifact-manifest-sha256/v1", ManifestDigest: DigestBytes([]byte("manifest-model")), Artifacts: []ArtifactSubmission{artifactA, artifactB}, Dependencies: []DependencySubmission{dependency}, Projections: []ProjectionSubmission{projection}, Diagnostics: []DiagnosticSubmission{diagnostic}, Statistics: []StatisticSubmission{statistic}, MakeCurrent: true, Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	consume(publish.Scope(), publish.RequestID(), publish.RepositoryID(), publish.ScanID(), publish.ManifestScheme(), publish.ManifestDigest(), publish.Artifacts(), publish.Dependencies(), publish.Projections(), publish.Diagnostics(), publish.Statistics(), publish.MakeCurrent(), publish.Actor())

	artifactQuery, _ := contract.NewArtifactQuery(scope, "repo-model", "scan-model", "artifact-a")
	consume(artifactQuery.Scope(), artifactQuery.RepositoryID(), artifactQuery.ScanID(), artifactQuery.ArtifactID())
	artifactList, _ := contract.NewArtifactListRequest(scope, "repo-model", "scan-model", 10, "cursor")
	consume(artifactList.Scope(), artifactList.RepositoryID(), artifactList.ScanID(), artifactList.PageSize(), artifactList.Cursor())
	payloadQuery, _ := contract.NewPayloadQuery(scope, "repo-model", "scan-model", "artifact-a", digest)
	consume(payloadQuery.Scope(), payloadQuery.RepositoryID(), payloadQuery.ScanID(), payloadQuery.ArtifactID(), payloadQuery.Digest())
	mark, _ := contract.NewMarkForPurgeRequest(scope, "req-mark", "repo-model", actor)
	consume(mark.Scope(), mark.RequestID(), mark.RepositoryID(), mark.Actor())
	purge, _ := contract.NewPurgeBatchRequest(scope, "req-purge", "repo-model", 10, actor)
	consume(purge.Scope(), purge.RequestID(), purge.RepositoryID(), purge.Limit(), purge.Actor())
	gc, _ := contract.NewGarbageCollectionRequest(scope, "req-gc", 10, actor)
	consume(gc.Scope(), gc.RequestID(), gc.Limit(), gc.Actor())

	now := time.Now().UTC()
	repositoryRecord, _ := NewRepositoryRecord(scope.ScopeID(), "repo-model", "Model", source, RepositoryActive, "scan-model", now, now)
	consume(repositoryRecord.ScopeID(), repositoryRecord.RepositoryID(), repositoryRecord.DisplayName(), repositoryRecord.Source(), repositoryRecord.State(), repositoryRecord.CurrentScanID(), repositoryRecord.CreatedAt(), repositoryRecord.UpdatedAt())
	scanRecord, _ := NewScanRecord(scope.ScopeID(), "repo-model", "scan-model", profile, "revision-model", ScanSucceeded, "", "", now, now, now)
	consume(scanRecord.ScopeID(), scanRecord.RepositoryID(), scanRecord.ScanID(), scanRecord.AnalysisProfileDigest(), scanRecord.SourceRevision(), scanRecord.State(), scanRecord.ReasonCode(), scanRecord.SafeMessage(), scanRecord.RequestedAt(), scanRecord.StartedAt(), scanRecord.FinishedAt())
	artifactRecord, _ := NewArtifactRecord(scope.ScopeID(), "repo-model", "scan-model", "artifact-a", artifactA.Artifact(), "go-semantic-id/v1", codec, producer, digest, ByteCount(len(payload)), now)
	consume(artifactRecord.ScopeID(), artifactRecord.RepositoryID(), artifactRecord.ScanID(), artifactRecord.ArtifactID(), artifactRecord.Artifact(), artifactRecord.StableIDScheme(), artifactRecord.Codec(), artifactRecord.Producer(), artifactRecord.PayloadDigest(), artifactRecord.PayloadSize(), artifactRecord.CreatedAt())
	consume(NewRepositoryPage([]RepositoryRecord{repositoryRecord}, "next").Records())
	scanPage := NewScanPage([]ScanRecord{scanRecord}, "next")
	consume(scanPage.Records(), scanPage.NextCursor())
	artifactPage := NewArtifactPage([]ArtifactRecord{artifactRecord}, "next")
	consume(artifactPage.Records(), artifactPage.NextCursor())

	payloadReceipt, _ := NewPayloadReceipt(digest, ByteCount(len(payload)), DispositionAlreadyPresent)
	consume(payloadReceipt.Digest(), payloadReceipt.Size(), payloadReceipt.Disposition())
	publicationReceipt, _ := NewPublicationReceipt("scan-model", publish.ManifestScheme(), publish.ManifestDigest(), 2, DispositionCreated)
	consume(publicationReceipt.ScanID(), publicationReceipt.ManifestScheme(), publicationReceipt.ManifestDigest(), publicationReceipt.ArtifactCount(), publicationReceipt.Disposition())
	verification, _ := NewVerificationReceipt(digest, ByteCount(len(payload)))
	consume(verification.Digest(), verification.Size())
	purgeReceipt := NewPurgeReceipt(2, 1, true)
	consume(purgeReceipt.RemovedArtifacts(), purgeReceipt.RemovedScans(), purgeReceipt.Complete())
	gcReceipt := NewGarbageCollectionReceipt(1, 42)
	consume(gcReceipt.RemovedPayloads(), gcReceipt.RemovedBytes())
	if !EqualJSON(document, append([]byte(nil), document...)) {
		t.Fatal("exact JSON equality failed")
	}
}

func TestValidationFailuresAcrossPublicConstructors(t *testing.T) {
	contract := mustContract(t)
	scope := mustScope(t, "scope", "principal")
	actor := mustActor(t)
	producer := mustName(t, "producer", "1.0.0")
	digest := DigestBytes([]byte("x"))
	codec, _ := NewCodec("json", "1.0.0", "application/json")

	cases := []struct {
		name string
		run  func() error
	}{
		{"empty name", func() error { _, err := NewVersionedName("", "1"); return err }},
		{"empty version", func() error { _, err := NewVersionedName("name", ""); return err }},
		{"bad codec", func() error { _, err := NewCodec("json", "1", "json"); return err }},
		{"bad source identity", func() error { _, err := NewSourceIdentity("bad kind", "sha256-v1", digest); return err }},
		{"bad actor", func() error { _, err := NewAuditActor("bad kind", "id"); return err }},
		{"bad repository query", func() error { _, err := contract.NewRepositoryQuery(Scope{}, "repo"); return err }},
		{"bad archive", func() error { _, err := contract.NewArchiveRepositoryRequest(scope, "", "repo", actor); return err }},
		{"bad begin", func() error {
			_, err := contract.NewBeginScanRequest(BeginScanParams{Scope: scope, RequestID: "request", RepositoryID: "repo", ScanID: "", AnalysisProfileDigest: digest, SourceRevision: "revision", Actor: actor})
			return err
		}},
		{"bad scan query", func() error { _, err := contract.NewScanQuery(scope, "repo", ""); return err }},
		{"bad scan list", func() error { _, err := contract.NewScanListRequest(scope, "repo", 2_000, ""); return err }},
		{"bad dependency self", func() error {
			_, err := contract.NewDependencySubmission("artifact", 0, "artifact", producer)
			return err
		}},
		{"bad diagnostic location", func() error {
			_, err := contract.NewDiagnosticSubmission("projection", 0, "info", "code", "engine", "file.go", 0, 1, "message")
			return err
		}},
		{"bad statistic", func() error {
			_, err := contract.NewStatisticSubmission("projection", "key", StatisticValue{}, "")
			return err
		}},
		{"bad artifact query", func() error { _, err := contract.NewArtifactQuery(scope, "repo", "", "artifact"); return err }},
		{"bad artifact list", func() error { _, err := contract.NewArtifactListRequest(scope, "repo", "scan", 0, ""); return err }},
		{"bad payload query", func() error {
			_, err := contract.NewPayloadQuery(scope, "repo", "scan", "artifact", Digest{})
			return err
		}},
		{"bad mark", func() error { _, err := contract.NewMarkForPurgeRequest(scope, "", "repo", actor); return err }},
		{"bad purge", func() error { _, err := contract.NewPurgeBatchRequest(scope, "request", "repo", 0, actor); return err }},
		{"bad gc", func() error { _, err := contract.NewGarbageCollectionRequest(scope, "request", 0, actor); return err }},
		{"bad artifact stable ID", func() error {
			_, err := contract.NewArtifactSubmission(ArtifactSubmissionParams{ArtifactID: "artifact", Artifact: producer, StableIDScheme: "bad scheme", Codec: codec, PayloadDigest: digest, PayloadSize: 1, Producer: producer})
			return err
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); KindOf(err) == "" {
				t.Fatal("invalid value was accepted")
			}
		})
	}

	now := time.Now().UTC()
	if _, err := NewRepositoryRecord("scope", "repo", "", mustSource(t), RepositoryActive, "", now, now); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("invalid repository record accepted: %v", err)
	}
	if _, err := NewScanRecord("scope", "repo", "scan", digest, "revision", ScanSucceeded, "", "", now, now, time.Time{}); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("invalid terminal scan accepted: %v", err)
	}
	if _, err := NewArtifactRecord("scope", "repo", "scan", "artifact", producer, "bad scheme", codec, producer, digest, 1, now); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("invalid artifact record accepted: %v", err)
	}
	if _, err := NewPayloadReceipt(Digest{}, 0, DispositionCreated); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("invalid receipt accepted: %v", err)
	}
	if _, err := NewPublicationReceipt("", "scan", digest, 1, DispositionCreated); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("invalid publication receipt accepted: %v", err)
	}
	if _, err := NewVerificationReceipt(Digest{}, 0); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("invalid verification receipt accepted: %v", err)
	}
}

func TestPublicationGraphFailureBranches(t *testing.T) {
	contract := mustContract(t)
	a := mustArtifact(t, contract, "artifact-a", "same-name", []byte("a"))
	b := mustArtifact(t, contract, "artifact-b", "same-name", []byte("b"))
	if _, err := contract.NewPublishScanRequest(basePublishParams(t, a, b)); KindOf(err) != ErrorDuplicateArtifact {
		t.Fatalf("duplicate artifact names accepted: %v", err)
	}

	b = mustArtifact(t, contract, "artifact-b", "other-name", []byte("b"))
	declared := b.Artifact()
	dependency0, _ := contract.NewDependencySubmission("artifact-a", 0, "artifact-b", declared)
	dependencySameOrdinal := DependencySubmission{consumerArtifactID: "artifact-a", ordinal: 0, sourceArtifactID: "artifact-a-missing", declaredArtifact: declared}
	params := basePublishParams(t, a, b)
	params.Dependencies = []DependencySubmission{dependency0, dependencySameOrdinal}
	if _, err := contract.NewPublishScanRequest(params); KindOf(err) != ErrorInvalidDependency {
		t.Fatalf("invalid dependency target accepted: %v", err)
	}

	projection := ProjectionSubmission{params: ProjectionSubmissionParams{ProjectionID: "projection", ArtifactID: "artifact-a", SourceDigest: DigestBytes([]byte("wrong")), Projector: mustName(t, "projector", "1.0.0"), SchemaVersion: "1.0.0", DigestScheme: "sha256-json-v1", ProjectionDigest: DigestBytes([]byte(`{}`)), CanonicalJSON: []byte(`{}`)}}
	params = basePublishParams(t, a, b)
	params.Projections = []ProjectionSubmission{projection}
	if _, err := contract.NewPublishScanRequest(params); KindOf(err) != ErrorIntegrityFailure {
		t.Fatalf("wrong projection source accepted: %v", err)
	}

	diagnostic := DiagnosticSubmission{projectionID: "missing", severity: "info", code: "code", engine: "engine", message: "safe"}
	params = basePublishParams(t, a)
	params.Diagnostics = []DiagnosticSubmission{diagnostic}
	if _, err := contract.NewPublishScanRequest(params); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("orphan diagnostic accepted: %v", err)
	}
	statistic := StatisticSubmission{projectionID: "missing", key: "key", value: NewIntegerStatistic(1)}
	params.Diagnostics = nil
	params.Statistics = []StatisticSubmission{statistic}
	if _, err := contract.NewPublishScanRequest(params); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("orphan statistic accepted: %v", err)
	}
}

func consume(...any) {}

func mustContract(t *testing.T) *Contract {
	t.Helper()
	contract, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func mustScope(t *testing.T, scopeID, principalID string) Scope {
	t.Helper()
	scope, err := NewScope(scopeID, principalID)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func mustActor(t *testing.T) AuditActor {
	t.Helper()
	actor, err := NewAuditActor("service", "test-runner")
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func mustName(t *testing.T, name, version string) VersionedName {
	t.Helper()
	value, err := NewVersionedName(name, version)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustArtifact(t *testing.T, contract *Contract, id ArtifactID, name string, payload []byte) ArtifactSubmission {
	t.Helper()
	codec, _ := NewCodec("json", "1.0.0", "application/json")
	artifact, err := contract.NewArtifactSubmission(ArtifactSubmissionParams{
		ArtifactID: id, Artifact: mustName(t, name, "1.0.0"), Codec: codec,
		PayloadDigest: DigestBytes(payload), PayloadSize: ByteCount(len(payload)),
		Producer: mustName(t, "test-producer", "1.0.0"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}

func mustCodec(t *testing.T) Codec {
	t.Helper()
	codec, err := NewCodec("json", "1.0.0", "application/json")
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

func basePublishParams(t *testing.T, artifacts ...ArtifactSubmission) PublishScanParams {
	t.Helper()
	return PublishScanParams{
		Scope: mustScope(t, "scope-a", "principal-a"), RequestID: "request-publish",
		RepositoryID: "repository-1", ScanID: "scan-1", ManifestScheme: "artifact-manifest-sha256/v1",
		ManifestDigest: DigestBytes([]byte("manifest")), Artifacts: artifacts,
		MakeCurrent: true, Actor: mustActor(t),
	}
}

func mustSource(t *testing.T) SourceIdentity {
	t.Helper()
	source, err := NewSourceIdentity("local", "sha256-v1", DigestBytes([]byte("repository-source")))
	if err != nil {
		t.Fatal(err)
	}
	return source
}
