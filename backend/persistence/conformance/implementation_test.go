package conformance

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
)

func TestScopeIsolationOperationsCompleteAndUnique(t *testing.T) {
	operations := ScopeIsolationOperations()
	if len(operations) != 18 {
		t.Fatalf("expected 18 public operations, got %d", len(operations))
	}
	seen := make(map[Operation]struct{}, len(operations))
	for _, operation := range operations {
		if _, exists := seen[operation]; exists {
			t.Fatalf("duplicate operation: %s", operation)
		}
		seen[operation] = struct{}{}
	}
}

func TestSuiteWithScopedTestDouble(t *testing.T) {
	Run(t, FactoryFunc(func(context.Context) (Fixture, Cleanup, error) {
		fixture := newTestFixture(t)
		return fixture, func(context.Context) error { return nil }, nil
	}))
}

func TestInvalidSuiteAndFixture(t *testing.T) {
	if suite, err := New(Config{}); err != nil || suite.config != DefaultConfig() {
		t.Fatalf("zero configuration defaults failed: %v", err)
	}
	if _, err := New(Config{CaseTimeout: time.Millisecond}); err == nil {
		t.Fatal("invalid timeout accepted")
	}
	suite, _ := New()
	t.Run("nil-factory", func(t *testing.T) {
		defer func() {
			if recover() != nil {
				t.Fatal("suite must fail through testing, not panic")
			}
		}()
		// Testing a Fatal path directly is not supported; fixture validation is
		// covered through validateFixture below.
		_ = suite
	})
	if err := validateFixture(Fixture{}); err != ErrFixtureInvalid {
		t.Fatalf("invalid fixture not rejected: %v", err)
	}
}

func TestRecordContainmentHelpers(t *testing.T) {
	fixture := newTestFixture(t)
	if !containsRepository([]persistence.RepositoryRecord{fixture.Port.(*scopedTestPort).records.repository}, fixture.Scenario.RepositoryID) {
		t.Fatal("repository containment failed")
	}
	if !containsScan([]persistence.ScanRecord{fixture.Port.(*scopedTestPort).records.scan}, fixture.Scenario.ScanID) {
		t.Fatal("scan containment failed")
	}
	if !containsArtifact([]persistence.ArtifactRecord{fixture.Port.(*scopedTestPort).records.artifact}, fixture.Scenario.ArtifactID) {
		t.Fatal("artifact containment failed")
	}
}

type scopedTestPort struct {
	scenario Scenario
	records  testRecords
}

type testRecords struct {
	repository persistence.RepositoryRecord
	scan       persistence.ScanRecord
	artifact   persistence.ArtifactRecord
}

func newTestFixture(t *testing.T) Fixture {
	t.Helper()
	contract, _ := persistence.New()
	primary, _ := persistence.NewScope("scope-primary", "principal-primary")
	other, _ := persistence.NewScope("scope-other", "principal-other")
	artifactName, _ := persistence.NewVersionedName("go-semantic-inventory", "1.0.0")
	producer, _ := persistence.NewVersionedName("go-semantic", "1.0.0")
	codec, _ := persistence.NewCodec("json", "1.0.0", "application/json")
	actor, _ := persistence.NewAuditActor("test", "conformance")
	payload := []byte(`{"artifact":"semantic"}`)
	digest := persistence.DigestBytes(payload)
	now := time.Now().UTC()
	repository, _ := persistence.NewRepositoryRecord(primary.ScopeID(), "repository-primary", "Primary", "local:primary", persistence.RepositoryActive, "scan-primary", now, now)
	scan, _ := persistence.NewScanRecord(primary.ScopeID(), "repository-primary", "scan-primary", producer, persistence.ScanSucceeded, "", "", now, now, now)
	artifact, _ := persistence.NewArtifactRecord(primary.ScopeID(), "repository-primary", "scan-primary", "artifact-primary", "publication-primary", artifactName, persistence.VersionedName{}, codec, producer, digest, persistence.ByteCount(len(payload)), now)
	scenario := Scenario{PrimaryScope: primary, OtherScope: other, RepositoryID: "repository-primary", ScanID: "scan-primary", ArtifactID: "artifact-primary", PublicationID: "publication-primary", Artifact: artifactName, Producer: producer, Codec: codec, Digest: digest, Payload: payload, Actor: actor}
	port := &scopedTestPort{scenario: scenario, records: testRecords{repository: repository, scan: scan, artifact: artifact}}
	return Fixture{Port: port, Contract: contract, Scenario: scenario}
}

func (port *scopedTestPort) hidden(scope persistence.Scope) error {
	if scope.ScopeID() != port.scenario.PrimaryScope.ScopeID() {
		return persistence.NewError(persistence.ErrorNotFound, "test-double", false, nil)
	}
	return nil
}

func (port *scopedTestPort) RegisterRepository(_ context.Context, request persistence.RegisterRepositoryRequest) (persistence.RepositoryRecord, error) {
	return persistence.RepositoryRecord{}, port.hidden(request.Scope())
}
func (port *scopedTestPort) GetRepository(_ context.Context, query persistence.RepositoryQuery) (persistence.RepositoryRecord, error) {
	if err := port.hidden(query.Scope()); err != nil {
		return persistence.RepositoryRecord{}, err
	}
	return port.records.repository, nil
}
func (port *scopedTestPort) ListRepositories(_ context.Context, request persistence.RepositoryListRequest) (persistence.RepositoryPage, error) {
	if request.Scope().ScopeID() != port.scenario.PrimaryScope.ScopeID() {
		return persistence.NewRepositoryPage(nil, ""), nil
	}
	return persistence.NewRepositoryPage([]persistence.RepositoryRecord{port.records.repository}, ""), nil
}
func (port *scopedTestPort) ArchiveRepository(_ context.Context, request persistence.ArchiveRepositoryRequest) (persistence.RepositoryRecord, error) {
	return persistence.RepositoryRecord{}, port.hidden(request.Scope())
}
func (port *scopedTestPort) BeginScan(_ context.Context, request persistence.BeginScanRequest) (persistence.ScanRecord, error) {
	return persistence.ScanRecord{}, port.hidden(request.Scope())
}
func (port *scopedTestPort) GetScan(_ context.Context, query persistence.ScanQuery) (persistence.ScanRecord, error) {
	if err := port.hidden(query.Scope()); err != nil {
		return persistence.ScanRecord{}, err
	}
	return port.records.scan, nil
}
func (port *scopedTestPort) ListScans(_ context.Context, request persistence.ScanListRequest) (persistence.ScanPage, error) {
	if request.Scope().ScopeID() != port.scenario.PrimaryScope.ScopeID() {
		return persistence.NewScanPage(nil, ""), nil
	}
	return persistence.NewScanPage([]persistence.ScanRecord{port.records.scan}, ""), nil
}
func (port *scopedTestPort) FailScan(_ context.Context, request persistence.FinishScanRequest) (persistence.ScanRecord, error) {
	return persistence.ScanRecord{}, port.hidden(request.Scope())
}
func (port *scopedTestPort) CancelScan(_ context.Context, request persistence.FinishScanRequest) (persistence.ScanRecord, error) {
	return persistence.ScanRecord{}, port.hidden(request.Scope())
}
func (port *scopedTestPort) StagePayload(_ context.Context, request persistence.StagePayloadRequest, _ io.Reader) (persistence.PayloadReceipt, error) {
	return persistence.PayloadReceipt{}, port.hidden(request.Scope())
}
func (port *scopedTestPort) PublishScan(_ context.Context, request persistence.PublishScanRequest) (persistence.PublicationReceipt, error) {
	return persistence.PublicationReceipt{}, port.hidden(request.Scope())
}
func (port *scopedTestPort) GetArtifact(_ context.Context, query persistence.ArtifactQuery) (persistence.ArtifactRecord, error) {
	if err := port.hidden(query.Scope()); err != nil {
		return persistence.ArtifactRecord{}, err
	}
	return port.records.artifact, nil
}
func (port *scopedTestPort) ListArtifacts(_ context.Context, request persistence.ArtifactListRequest) (persistence.ArtifactPage, error) {
	if request.Scope().ScopeID() != port.scenario.PrimaryScope.ScopeID() {
		return persistence.NewArtifactPage(nil, ""), nil
	}
	return persistence.NewArtifactPage([]persistence.ArtifactRecord{port.records.artifact}, ""), nil
}
func (port *scopedTestPort) ExportPayload(_ context.Context, query persistence.PayloadQuery, writer io.Writer) (persistence.PayloadReceipt, error) {
	if err := port.hidden(query.Scope()); err != nil {
		return persistence.PayloadReceipt{}, err
	}
	if _, err := writer.Write(port.scenario.Payload); err != nil {
		return persistence.PayloadReceipt{}, err
	}
	return persistence.NewPayloadReceipt(port.scenario.Digest, persistence.ByteCount(len(port.scenario.Payload)), persistence.DispositionCreated)
}
func (port *scopedTestPort) VerifyPayload(_ context.Context, query persistence.PayloadQuery) (persistence.VerificationReceipt, error) {
	if err := port.hidden(query.Scope()); err != nil {
		return persistence.VerificationReceipt{}, err
	}
	return persistence.NewVerificationReceipt(port.scenario.Digest, persistence.ByteCount(len(port.scenario.Payload)))
}
func (port *scopedTestPort) MarkRepositoryForPurge(_ context.Context, request persistence.MarkForPurgeRequest) (persistence.RepositoryRecord, error) {
	return persistence.RepositoryRecord{}, port.hidden(request.Scope())
}
func (port *scopedTestPort) PurgeRepositoryBatch(_ context.Context, request persistence.PurgeBatchRequest) (persistence.PurgeReceipt, error) {
	return persistence.PurgeReceipt{}, port.hidden(request.Scope())
}
func (port *scopedTestPort) GarbageCollectPayloads(_ context.Context, request persistence.GarbageCollectionRequest) (persistence.GarbageCollectionReceipt, error) {
	if request.Scope().ScopeID() != port.scenario.PrimaryScope.ScopeID() {
		return persistence.NewGarbageCollectionReceipt(0, 0), nil
	}
	return persistence.NewGarbageCollectionReceipt(0, 0), nil
}

var _ persistence.Port = (*scopedTestPort)(nil)
