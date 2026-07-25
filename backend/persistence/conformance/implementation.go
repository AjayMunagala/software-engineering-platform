package conformance

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
)

// Suite owns immutable conformance limits.
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
	t.Run("scope-isolation-every-public-operation", func(t *testing.T) {
		fixture := suite.open(t, factory)
		suite.scopeIsolation(t, fixture)
	})
	t.Run("exact-payload-read-and-verification", func(t *testing.T) {
		fixture := suite.open(t, factory)
		suite.exactRead(t, fixture)
	})
	t.Run("published-metadata-visible", func(t *testing.T) {
		fixture := suite.open(t, factory)
		suite.publishedMetadata(t, fixture)
	})
}

func (suite *Suite) open(t *testing.T, factory Factory) Fixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), suite.config.CaseTimeout)
	t.Cleanup(cancel)
	fixture, cleanup, err := factory.Open(ctx)
	if err != nil {
		t.Fatalf("open conformance fixture: %v", err)
	}
	if cleanup == nil {
		t.Fatal("open conformance fixture: cleanup is required")
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), suite.config.CaseTimeout)
		defer cleanupCancel()
		if err := cleanup(cleanupCtx); err != nil {
			t.Errorf("cleanup conformance fixture: %v", err)
		}
	})
	fixture.Scenario = fixture.Scenario.clone()
	fixture.context = ctx
	if err := validateFixture(fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (suite *Suite) exactRead(t *testing.T, fixture Fixture) {
	t.Helper()
	scenario := fixture.Scenario
	query := mustPayloadQuery(t, fixture.Contract, scenario.PrimaryScope, scenario)
	var output bytes.Buffer
	receipt, err := fixture.Port.ExportPayload(fixture.context, query, &output)
	if err != nil {
		t.Fatalf("export exact payload: %v", err)
	}
	if !bytes.Equal(output.Bytes(), scenario.Payload) || receipt.Digest() != scenario.Digest || receipt.Size() != persistence.ByteCount(len(scenario.Payload)) {
		t.Fatal("exact payload or receipt mismatch")
	}
	verified, err := fixture.Port.VerifyPayload(fixture.context, query)
	if err != nil || verified.Digest() != scenario.Digest || verified.Size() != persistence.ByteCount(len(scenario.Payload)) {
		t.Fatalf("verify payload: %v", err)
	}
}

func (suite *Suite) publishedMetadata(t *testing.T, fixture Fixture) {
	t.Helper()
	scenario := fixture.Scenario
	repositoryQuery, _ := fixture.Contract.NewRepositoryQuery(scenario.PrimaryScope, scenario.RepositoryID)
	repository, err := fixture.Port.GetRepository(fixture.context, repositoryQuery)
	if err != nil || repository.RepositoryID() != scenario.RepositoryID || repository.CurrentScanID() != scenario.ScanID {
		t.Fatalf("published repository metadata: %v", err)
	}
	scanQuery, _ := fixture.Contract.NewScanQuery(scenario.PrimaryScope, scenario.RepositoryID, scenario.ScanID)
	scan, err := fixture.Port.GetScan(fixture.context, scanQuery)
	if err != nil || scan.State() != persistence.ScanSucceeded {
		t.Fatalf("published scan metadata: %v", err)
	}
	artifactQuery, _ := fixture.Contract.NewArtifactQuery(scenario.PrimaryScope, scenario.RepositoryID, scenario.ScanID, scenario.ArtifactID)
	artifact, err := fixture.Port.GetArtifact(fixture.context, artifactQuery)
	if err != nil || artifact.PayloadDigest() != scenario.Digest || artifact.PublicationID() != scenario.PublicationID {
		t.Fatalf("published artifact metadata: %v", err)
	}
}

func (suite *Suite) scopeIsolation(t *testing.T, fixture Fixture) {
	t.Helper()
	scenario := fixture.Scenario
	contract := fixture.Contract
	other := scenario.OtherScope
	ctx := fixture.context

	register, _ := contract.NewRegisterRepositoryRequest(persistence.RegisterRepositoryParams{Scope: other, RequestID: "scope-register", RepositoryID: scenario.RepositoryID, DisplayName: "Hidden", CanonicalKey: "hidden:key", Actor: scenario.Actor})
	_, err := fixture.Port.RegisterRepository(ctx, register)
	expectHidden(t, OperationRegisterRepository, err)

	repositoryQuery, _ := contract.NewRepositoryQuery(other, scenario.RepositoryID)
	_, err = fixture.Port.GetRepository(ctx, repositoryQuery)
	expectHidden(t, OperationGetRepository, err)
	repositoryList, _ := contract.NewRepositoryListRequest(other, 100, "")
	repositories, err := fixture.Port.ListRepositories(ctx, repositoryList)
	expectNoLeak(t, OperationListRepositories, err, containsRepository(repositories.Records(), scenario.RepositoryID))
	archive, _ := contract.NewArchiveRepositoryRequest(other, "scope-archive", scenario.RepositoryID, scenario.Actor)
	_, err = fixture.Port.ArchiveRepository(ctx, archive)
	expectHidden(t, OperationArchiveRepository, err)

	begin, _ := contract.NewBeginScanRequest(persistence.BeginScanParams{Scope: other, RequestID: "scope-begin", RepositoryID: scenario.RepositoryID, ScanID: "scope-new-scan", Producer: scenario.Producer, Actor: scenario.Actor})
	_, err = fixture.Port.BeginScan(ctx, begin)
	expectHidden(t, OperationBeginScan, err)
	scanQuery, _ := contract.NewScanQuery(other, scenario.RepositoryID, scenario.ScanID)
	_, err = fixture.Port.GetScan(ctx, scanQuery)
	expectHidden(t, OperationGetScan, err)
	scanList, _ := contract.NewScanListRequest(other, scenario.RepositoryID, 100, "")
	scans, err := fixture.Port.ListScans(ctx, scanList)
	expectNoLeak(t, OperationListScans, err, containsScan(scans.Records(), scenario.ScanID))
	finish, _ := contract.NewFinishScanRequest(persistence.FinishScanParams{Scope: other, RequestID: "scope-finish", RepositoryID: scenario.RepositoryID, ScanID: scenario.ScanID, ReasonCode: "scope-test", SafeMessage: "scope isolation", Actor: scenario.Actor})
	_, err = fixture.Port.FailScan(ctx, finish)
	expectHidden(t, OperationFailScan, err)
	_, err = fixture.Port.CancelScan(ctx, finish)
	expectHidden(t, OperationCancelScan, err)

	stage, _ := contract.NewStagePayloadRequest(persistence.StagePayloadParams{Scope: other, RequestID: "scope-stage", RepositoryID: scenario.RepositoryID, ScanID: scenario.ScanID, Digest: scenario.Digest, ExpectedSize: persistence.ByteCount(len(scenario.Payload))})
	_, err = fixture.Port.StagePayload(ctx, stage, bytes.NewReader(scenario.Payload))
	expectHidden(t, OperationStagePayload, err)

	artifactSubmission, _ := contract.NewArtifactSubmission(persistence.ArtifactSubmissionParams{ArtifactID: scenario.ArtifactID, Artifact: scenario.Artifact, Codec: scenario.Codec, PayloadDigest: scenario.Digest, PayloadSize: persistence.ByteCount(len(scenario.Payload)), Producer: scenario.Producer})
	publish, _ := contract.NewPublishScanRequest(persistence.PublishScanParams{Scope: other, RequestID: "scope-publish", RepositoryID: scenario.RepositoryID, ScanID: scenario.ScanID, PublicationID: "scope-publication", ManifestDigest: persistence.DigestBytes([]byte("scope-manifest")), Artifacts: []persistence.ArtifactSubmission{artifactSubmission}, Actor: scenario.Actor})
	_, err = fixture.Port.PublishScan(ctx, publish)
	expectHidden(t, OperationPublishScan, err)

	artifactQuery, _ := contract.NewArtifactQuery(other, scenario.RepositoryID, scenario.ScanID, scenario.ArtifactID)
	_, err = fixture.Port.GetArtifact(ctx, artifactQuery)
	expectHidden(t, OperationGetArtifact, err)
	artifactList, _ := contract.NewArtifactListRequest(other, scenario.RepositoryID, scenario.ScanID, 100, "")
	artifacts, err := fixture.Port.ListArtifacts(ctx, artifactList)
	expectNoLeak(t, OperationListArtifacts, err, containsArtifact(artifacts.Records(), scenario.ArtifactID))
	payloadQuery := mustPayloadQuery(t, contract, other, scenario)
	var leaked bytes.Buffer
	_, err = fixture.Port.ExportPayload(ctx, payloadQuery, &leaked)
	expectHidden(t, OperationExportPayload, err)
	if leaked.Len() != 0 {
		t.Fatal("cross-scope export wrote payload bytes")
	}
	_, err = fixture.Port.VerifyPayload(ctx, payloadQuery)
	expectHidden(t, OperationVerifyPayload, err)

	mark, _ := contract.NewMarkForPurgeRequest(other, "scope-mark", scenario.RepositoryID, scenario.Actor)
	_, err = fixture.Port.MarkRepositoryForPurge(ctx, mark)
	expectHidden(t, OperationMarkForPurge, err)
	purge, _ := contract.NewPurgeBatchRequest(other, "scope-purge", scenario.RepositoryID, 10, scenario.Actor)
	_, err = fixture.Port.PurgeRepositoryBatch(ctx, purge)
	expectHidden(t, OperationPurgeBatch, err)
	gc, _ := contract.NewGarbageCollectionRequest(other, "scope-gc", 10, scenario.Actor)
	if _, err = fixture.Port.GarbageCollectPayloads(ctx, gc); err != nil && persistence.KindOf(err) != persistence.ErrorAuthorizationDenied {
		t.Fatalf("%s returned unexpected error: %v", OperationGarbageCollect, err)
	}
	primaryQuery := mustPayloadQuery(t, contract, scenario.PrimaryScope, scenario)
	if _, err = fixture.Port.VerifyPayload(ctx, primaryQuery); err != nil {
		t.Fatalf("%s affected another scope: %v", OperationGarbageCollect, err)
	}
}

func validateFixture(fixture Fixture) error {
	if fixture.Port == nil || fixture.Contract == nil {
		return ErrFixtureInvalid
	}
	scenario := fixture.Scenario
	if scenario.PrimaryScope.IsZero() || scenario.OtherScope.IsZero() || scenario.PrimaryScope.ScopeID() == scenario.OtherScope.ScopeID() ||
		scenario.RepositoryID == "" || scenario.ScanID == "" || scenario.ArtifactID == "" || scenario.PublicationID == "" ||
		scenario.Artifact.IsZero() || scenario.Producer.IsZero() || scenario.Codec.IsZero() || scenario.Digest.IsZero() ||
		len(scenario.Payload) == 0 || persistence.DigestBytes(scenario.Payload) != scenario.Digest || scenario.Actor.IsZero() {
		return ErrFixtureInvalid
	}
	return nil
}

func mustPayloadQuery(t *testing.T, contract *persistence.Contract, scope persistence.Scope, scenario Scenario) persistence.PayloadQuery {
	t.Helper()
	query, err := contract.NewPayloadQuery(scope, scenario.RepositoryID, scenario.ScanID, scenario.ArtifactID, scenario.Digest)
	if err != nil {
		t.Fatalf("construct payload query: %v", err)
	}
	return query
}

func expectHidden(t *testing.T, operation Operation, err error) {
	t.Helper()
	if persistence.KindOf(err) != persistence.ErrorNotFound {
		t.Fatalf("%s must hide cross-scope target as not_found, got %v", operation, err)
	}
}

func expectNoLeak(t *testing.T, operation Operation, err error, leaked bool) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s list failed: %v", operation, err)
	}
	if leaked {
		t.Fatalf("%s leaked cross-scope record", operation)
	}
}

func containsRepository(records []persistence.RepositoryRecord, id persistence.RepositoryID) bool {
	for _, record := range records {
		if record.RepositoryID() == id {
			return true
		}
	}
	return false
}

func containsScan(records []persistence.ScanRecord, id persistence.ScanID) bool {
	for _, record := range records {
		if record.ScanID() == id {
			return true
		}
	}
	return false
}

func containsArtifact(records []persistence.ArtifactRecord, id persistence.ArtifactID) bool {
	for _, record := range records {
		if record.ArtifactID() == id {
			return true
		}
	}
	return false
}
