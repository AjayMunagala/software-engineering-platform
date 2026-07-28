package integration

import (
	"context"
	"io"
	"sort"

	serviceadapters "github.com/AjayMunagala/software-engineering-platform/backend/internal/service/repository/adapters"
	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/lifecycle"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/scan"
)

func New(dependencies Dependencies, configs ...Config) (*Bundle, error) {
	if dependencies.Runtime == nil || dependencies.Persistence == nil || dependencies.ServiceContract == nil || dependencies.SourceResolver == nil || dependencies.Clock == nil || len(configs) > 1 {
		return nil, repository.NewError(repository.ErrorInvalidInput, "new-integration", "invalid-dependencies", false, nil)
	}
	config := DefaultConfig()
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	ingest, read := dependencies.Runtime.Ingest(), dependencies.Runtime.Read()
	if ingest == nil || read == nil {
		return nil, repository.NewError(repository.ErrorInvalidInput, "new-integration", "invalid-runtime-capabilities", false, nil)
	}
	store, err := NewStore(ingest, ingest, read, dependencies.Persistence, dependencies.ServiceContract.Profiles(), config)
	if err != nil {
		return nil, err
	}
	proof := &SourceProofAdapter{resolver: dependencies.SourceResolver}
	lifecycleService, err := lifecycle.New(store, proof, lifecycle.ClockFunc(dependencies.Clock.Now))
	if err != nil {
		return nil, err
	}
	analysis, err := serviceadapters.New(dependencies.SourceResolver)
	if err != nil {
		return nil, err
	}
	scanService, err := scan.New(store, &Admission{runtime: dependencies.Runtime}, analysis, dependencies.Clock)
	if err != nil {
		return nil, err
	}
	return &Bundle{service: &composedService{lifecycle: lifecycleService, scans: scanService}}, nil
}

func NewStore(repositories persistence.RepositoryStore, ingest runtimeIngest, read runtimeRead, contract *persistence.Contract, profiles *repository.ProfileRegistry, configs ...Config) (*Store, error) {
	if repositories == nil || ingest == nil || read == nil || contract == nil || profiles == nil || len(configs) > 1 {
		return nil, repository.NewError(repository.ErrorInvalidInput, "new-persistence-store", "invalid-dependencies", false, nil)
	}
	config := DefaultConfig()
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if _, ok := profiles.Resolve(repository.DefaultRepositoryGoProfile().Profile().Name(), repository.DefaultRepositoryGoProfile().Profile().Version(), repository.DefaultRepositoryGoProfile().Profile().Digest()); !ok {
		return nil, repository.NewError(repository.ErrorInvalidInput, "new-persistence-store", "profile-unrecognized", false, nil)
	}
	return &Store{repositories: repositories, ingest: ingest, read: read, contract: contract, profiles: profiles.Clone(), config: config}, nil
}

func (admission *Admission) Acquire(ctx context.Context) (scan.WorkLease, error) {
	if admission == nil || admission.runtime == nil {
		return nil, repository.NewError(repository.ErrorInternal, "acquire-work", "runtime-admission-failed", false, nil)
	}
	work, err := admission.runtime.Admit(ctx)
	if err != nil {
		return nil, repository.NewError(repository.ErrorPersistenceUnavailable, "acquire-work", "runtime-admission-failed", true, err)
	}
	if work == nil {
		return nil, repository.NewError(repository.ErrorInternal, "acquire-work", "runtime-admission-failed", false, nil)
	}
	return &workLease{work: work}, nil
}

func (adapter *SourceProofAdapter) Resolve(ctx context.Context, scope repository.Scope, handle repository.SourceHandle) (lifecycle.SourceResolution, error) {
	if adapter == nil || adapter.resolver == nil {
		return nil, repository.NewError(repository.ErrorInternal, "resolve-source-proof", "invalid-resolver", false, nil)
	}
	source, err := adapter.resolver.Resolve(ctx, scope, handle)
	if err != nil {
		return nil, repository.NewError(repository.ErrorSourceUnavailable, "resolve-source-proof", "source-unavailable", false, err)
	}
	if source == nil || source.Fingerprint().IsZero() {
		if source != nil {
			_ = source.Close(context.WithoutCancel(ctx))
		}
		return nil, repository.NewError(repository.ErrorSourceUnavailable, "resolve-source-proof", "invalid-source-proof", false, nil)
	}
	proof, err := lifecycle.NewSourceProof("local", "sha256/v1", source.Fingerprint(), source.Revision())
	if err != nil {
		_ = source.Close(context.WithoutCancel(ctx))
		return nil, err
	}
	return &sourceResolution{source: source, proof: proof}, nil
}

func (store *Store) Register(ctx context.Context, command lifecycle.RegisterCommand) (repository.Repository, error) {
	scope, actor, err := store.identity(command.Scope())
	if err != nil {
		return repository.Repository{}, err
	}
	value := command.Repository()
	source, err := persistence.NewSourceIdentity(value.SourceKind(), value.FingerprintScheme(), toPersistenceDigest(value.Fingerprint()))
	if err != nil {
		return repository.Repository{}, serviceFailure(err, "register-repository", "persistence-contract-failed")
	}
	request, err := store.contract.NewRegisterRepositoryRequest(persistence.RegisterRepositoryParams{Scope: scope, RequestID: persistence.RequestID(command.RequestID()), RepositoryID: persistence.RepositoryID(value.RepositoryID()), DisplayName: value.DisplayName(), Source: source, Actor: actor})
	if err != nil {
		return repository.Repository{}, serviceFailure(err, "register-repository", "persistence-contract-failed")
	}
	record, err := store.repositories.RegisterRepository(ctx, request)
	if err != nil {
		return repository.Repository{}, serviceFailure(err, "register-repository", "persistence-contract-failed")
	}
	return store.repositoryRecord(command.Scope(), record)
}

func (store *Store) Get(ctx context.Context, scopeValue repository.Scope, id repository.RepositoryID) (repository.Repository, error) {
	scope, err := toPersistenceScope(scopeValue)
	if err != nil {
		return repository.Repository{}, err
	}
	query, err := store.contract.NewRepositoryQuery(scope, persistence.RepositoryID(id))
	if err != nil {
		return repository.Repository{}, serviceFailure(err, "get-repository", "persistence-contract-failed")
	}
	record, err := store.repositories.GetRepository(ctx, query)
	if err != nil {
		return repository.Repository{}, serviceFailure(err, "get-repository", "persistence-contract-failed")
	}
	value, err := store.repositoryRecord(scopeValue, record)
	if err != nil {
		return repository.Repository{}, err
	}
	if value.RepositoryID() != id {
		return repository.Repository{}, integrity("get-repository", "record-mismatch")
	}
	return value, nil
}

func (store *Store) List(ctx context.Context, scopeValue repository.Scope, size int, cursor repository.Cursor) (lifecycle.RepositoryList, error) {
	scope, err := toPersistenceScope(scopeValue)
	if err != nil {
		return lifecycle.RepositoryList{}, err
	}
	request, err := store.contract.NewRepositoryListRequest(scope, size, persistence.Cursor(cursor))
	if err != nil {
		return lifecycle.RepositoryList{}, serviceFailure(err, "list-repositories", "persistence-contract-failed")
	}
	page, err := store.repositories.ListRepositories(ctx, request)
	if err != nil {
		return lifecycle.RepositoryList{}, serviceFailure(err, "list-repositories", "persistence-contract-failed")
	}
	values := make([]repository.Repository, 0, len(page.Records()))
	for _, record := range page.Records() {
		value, mapErr := store.repositoryRecord(scopeValue, record)
		if mapErr != nil {
			return lifecycle.RepositoryList{}, mapErr
		}
		values = append(values, value)
	}
	return lifecycle.NewRepositoryList(values, repository.Cursor(page.NextCursor()))
}

func (store *Store) Archive(ctx context.Context, command lifecycle.ArchiveCommand) (repository.Repository, error) {
	scope, actor, err := store.identity(command.Scope())
	if err != nil {
		return repository.Repository{}, err
	}
	request, err := store.contract.NewArchiveRepositoryRequest(scope, persistence.RequestID(command.RequestID()), persistence.RepositoryID(command.RepositoryID()), actor)
	if err != nil {
		return repository.Repository{}, serviceFailure(err, "archive-repository", "persistence-contract-failed")
	}
	record, err := store.repositories.ArchiveRepository(ctx, request)
	if err != nil {
		return repository.Repository{}, serviceFailure(err, "archive-repository", "persistence-contract-failed")
	}
	return store.repositoryRecord(command.Scope(), record)
}

func (store *Store) Begin(ctx context.Context, command scan.BeginCommand) (scan.BeginResult, error) {
	scope, actor, err := store.identity(command.Scope())
	if err != nil {
		return scan.BeginResult{}, err
	}
	repositoryQuery, _ := store.contract.NewRepositoryQuery(scope, persistence.RepositoryID(command.Scan().RepositoryID()))
	record, err := store.repositories.GetRepository(ctx, repositoryQuery)
	if err != nil {
		return scan.BeginResult{}, serviceFailure(err, "begin-scan", "persistence-contract-failed")
	}
	if toRepositoryDigest(record.Source().Fingerprint()) != command.SourceFingerprint() {
		return scan.BeginResult{}, repository.NewError(repository.ErrorSourceUnavailable, "begin-scan", "source-proof-mismatch", false, nil)
	}
	query, _ := store.contract.NewScanQuery(scope, persistence.RepositoryID(command.Scan().RepositoryID()), persistence.ScanID(command.Scan().ScanID()))
	existing, getErr := store.ingest.GetScan(ctx, query)
	if getErr == nil {
		return store.beginFromExisting(ctx, command.Scope(), existing)
	}
	if persistence.KindOf(getErr) != persistence.ErrorNotFound {
		return scan.BeginResult{}, serviceFailure(getErr, "begin-scan", "persistence-contract-failed")
	}
	request, err := store.contract.NewBeginScanRequest(persistence.BeginScanParams{Scope: scope, RequestID: persistence.RequestID(command.RequestID()), RepositoryID: persistence.RepositoryID(command.Scan().RepositoryID()), ScanID: persistence.ScanID(command.Scan().ScanID()), AnalysisProfileDigest: toPersistenceDigest(command.Scan().Profile().Digest()), SourceRevision: command.Scan().SourceRevision(), Actor: actor})
	if err != nil {
		return scan.BeginResult{}, serviceFailure(err, "begin-scan", "persistence-contract-failed")
	}
	created, err := store.ingest.BeginScan(ctx, request)
	if err != nil {
		return scan.BeginResult{}, serviceFailure(err, "begin-scan", "persistence-contract-failed")
	}
	value, err := store.scanRecord(command.Scope(), created)
	if err != nil {
		return scan.BeginResult{}, err
	}
	return scan.NewBeginResult(scan.BeginStarted, value, nil)
}

func (store *Store) beginFromExisting(ctx context.Context, scope repository.Scope, record persistence.ScanRecord) (scan.BeginResult, error) {
	value, err := store.scanRecord(scope, record)
	if err != nil {
		return scan.BeginResult{}, err
	}
	switch value.State() {
	case repository.ScanSucceeded:
		artifacts, listErr := store.allArtifacts(ctx, scope, value.RepositoryID(), value.ScanID())
		if listErr != nil {
			return scan.BeginResult{}, listErr
		}
		return scan.NewBeginResult(scan.BeginAlreadyPublished, value, artifacts)
	case repository.ScanFailed:
		return scan.NewBeginResult(scan.BeginPreviouslyFailed, value, nil)
	case repository.ScanCanceled:
		return scan.NewBeginResult(scan.BeginPreviouslyCanceled, value, nil)
	default:
		return scan.NewBeginResult(scan.BeginOrphaned, value, nil)
	}
}

func (store *Store) Publish(ctx context.Context, command scan.PublishCommand) (repository.ScanResult, error) {
	if len(command.Artifacts()) == 0 || len(command.Artifacts()) > store.config.MaxArtifacts {
		return repository.ScanResult{}, integrity("publish-scan", "manifest-build-failed")
	}
	scope, actor, err := store.identity(command.Scope())
	if err != nil {
		return repository.ScanResult{}, err
	}
	physical := make(map[repository.ArtifactID]persistence.ArtifactID, len(command.Artifacts()))
	for _, item := range command.Artifacts() {
		metadata := item.Metadata()
		id, mapErr := PhysicalArtifactID(metadata.ArtifactID())
		if mapErr != nil {
			return repository.ScanResult{}, mapErr
		}
		physical[metadata.ArtifactID()] = id
		reader, openErr := item.Open(ctx)
		if openErr != nil {
			return repository.ScanResult{}, repository.NewError(repository.ErrorMaterializationFailed, "stage-payload", "payload-open-failed", false, openErr)
		}
		stage, buildErr := store.contract.NewStagePayloadRequest(persistence.StagePayloadParams{Scope: scope, RequestID: stageRequestID(command.RequestID(), metadata.ArtifactID()), RepositoryID: persistence.RepositoryID(command.Scan().RepositoryID()), ScanID: persistence.ScanID(command.Scan().ScanID()), Digest: toPersistenceDigest(metadata.PayloadDigest()), ExpectedSize: persistence.ByteCount(metadata.PayloadSize())})
		if buildErr != nil {
			_ = reader.Close()
			return repository.ScanResult{}, serviceFailure(buildErr, "stage-payload", "persistence-contract-failed")
		}
		receipt, stageErr := store.ingest.StagePayload(ctx, stage, reader)
		closeErr := reader.Close()
		if stageErr != nil {
			return repository.ScanResult{}, serviceFailure(stageErr, "stage-payload", "persistence-contract-failed")
		}
		if closeErr != nil || receipt.Digest() != stage.Digest() || receipt.Size() != stage.ExpectedSize() {
			return repository.ScanResult{}, integrity("stage-payload", "payload-receipt-mismatch")
		}
	}
	artifacts, dependencies, err := store.submissions(command, physical)
	if err != nil {
		return repository.ScanResult{}, err
	}
	digest, err := ManifestDigest(command.Scan(), command.Artifacts())
	if err != nil {
		return repository.ScanResult{}, err
	}
	request, err := store.contract.NewPublishScanRequest(persistence.PublishScanParams{Scope: scope, RequestID: persistence.RequestID(command.RequestID()), RepositoryID: persistence.RepositoryID(command.Scan().RepositoryID()), ScanID: persistence.ScanID(command.Scan().ScanID()), ManifestScheme: ManifestScheme, ManifestDigest: toPersistenceDigest(digest), Artifacts: artifacts, Dependencies: dependencies, MakeCurrent: true, Actor: actor})
	if err != nil {
		return repository.ScanResult{}, serviceFailure(err, "publish-scan", "persistence-contract-failed")
	}
	receipt, err := store.ingest.PublishScan(ctx, request)
	if err != nil {
		return repository.ScanResult{}, serviceFailure(err, "publish-scan", "publication-ambiguous")
	}
	if receipt.ScanID() != request.ScanID() || receipt.ManifestScheme() != ManifestScheme || receipt.ManifestDigest() != request.ManifestDigest() || receipt.ArtifactCount() != uint32(len(artifacts)) {
		return repository.ScanResult{}, integrity("publish-scan", "record-mismatch")
	}
	return store.publishedResult(ctx, command.Scope(), command.Scan().RepositoryID(), command.Scan().ScanID(), repository.DispositionCreated)
}

func (store *Store) Finalize(ctx context.Context, command scan.FinalizeCommand) (repository.Scan, error) {
	return store.finish(ctx, command.Scope(), command.RequestID(), command.Scan())
}
func (store *Store) Cancel(ctx context.Context, command scan.CancelCommand) (repository.Scan, error) {
	current, err := store.GetScan(ctx, command.Scope(), command.RepositoryID(), command.ScanID())
	if err != nil {
		return repository.Scan{}, err
	}
	terminal, err := repository.NewScan(repository.ScanParams{RepositoryID: current.RepositoryID(), ScanID: current.ScanID(), Profile: current.Profile(), SourceRevision: current.SourceRevision(), State: repository.ScanCanceled, ReasonCode: "caller-canceled", RequestedAt: current.RequestedAt(), StartedAt: current.StartedAt(), FinishedAt: command.At().UTC()})
	if err != nil {
		return repository.Scan{}, err
	}
	return store.finish(ctx, command.Scope(), command.RequestID(), terminal)
}

func (store *Store) finish(ctx context.Context, scopeValue repository.Scope, requestID repository.RequestID, value repository.Scan) (repository.Scan, error) {
	scope, actor, err := store.identity(scopeValue)
	if err != nil {
		return repository.Scan{}, err
	}
	request, err := store.contract.NewFinishScanRequest(persistence.FinishScanParams{Scope: scope, RequestID: persistence.RequestID(requestID), RepositoryID: persistence.RepositoryID(value.RepositoryID()), ScanID: persistence.ScanID(value.ScanID()), ReasonCode: value.ReasonCode(), SafeMessage: value.ReasonCode(), Actor: actor})
	if err != nil {
		return repository.Scan{}, serviceFailure(err, "finish-scan", "persistence-contract-failed")
	}
	var record persistence.ScanRecord
	if value.State() == repository.ScanCanceled {
		record, err = store.ingest.CancelScan(ctx, request)
	} else {
		record, err = store.ingest.FailScan(ctx, request)
	}
	if err != nil {
		return repository.Scan{}, serviceFailure(err, "finish-scan", "persistence-contract-failed")
	}
	return store.scanRecord(scopeValue, record)
}

func (store *Store) Reconcile(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) (scan.ReconcileResult, error) {
	value, err := store.GetScan(ctx, scope, repositoryID, scanID)
	if err != nil {
		return scan.ReconcileResult{}, err
	}
	artifacts := []repository.Artifact(nil)
	if value.State() == repository.ScanSucceeded {
		artifacts, err = store.allArtifacts(ctx, scope, repositoryID, scanID)
		if err != nil {
			return scan.ReconcileResult{}, err
		}
	}
	return scan.NewReconcileResult(value, artifacts)
}
func (store *Store) GetScan(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) (repository.Scan, error) {
	ps, err := toPersistenceScope(scope)
	if err != nil {
		return repository.Scan{}, err
	}
	query, err := store.contract.NewScanQuery(ps, persistence.RepositoryID(repositoryID), persistence.ScanID(scanID))
	if err != nil {
		return repository.Scan{}, serviceFailure(err, "get-scan", "persistence-contract-failed")
	}
	record, err := store.ingest.GetScan(ctx, query)
	if err != nil {
		return repository.Scan{}, serviceFailure(err, "get-scan", "persistence-contract-failed")
	}
	value, err := store.scanRecord(scope, record)
	if err != nil {
		return repository.Scan{}, err
	}
	if value.RepositoryID() != repositoryID || value.ScanID() != scanID {
		return repository.Scan{}, integrity("get-scan", "record-mismatch")
	}
	return value, nil
}
func (store *Store) ListScans(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, size int, cursor repository.Cursor) (scan.ScanList, error) {
	ps, err := toPersistenceScope(scope)
	if err != nil {
		return scan.ScanList{}, err
	}
	request, err := store.contract.NewScanListRequest(ps, persistence.RepositoryID(repositoryID), size, persistence.Cursor(cursor))
	if err != nil {
		return scan.ScanList{}, serviceFailure(err, "list-scans", "persistence-contract-failed")
	}
	page, err := store.ingest.ListScans(ctx, request)
	if err != nil {
		return scan.ScanList{}, serviceFailure(err, "list-scans", "persistence-contract-failed")
	}
	values := make([]repository.Scan, 0, len(page.Records()))
	for _, record := range page.Records() {
		value, mapErr := store.scanRecord(scope, record)
		if mapErr != nil {
			return scan.ScanList{}, mapErr
		}
		if value.RepositoryID() != repositoryID {
			return scan.ScanList{}, integrity("list-scans", "record-mismatch")
		}
		values = append(values, value)
	}
	return scan.NewScanList(values, repository.Cursor(page.NextCursor()))
}

func (store *Store) GetArtifact(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, artifactID repository.ArtifactID) (repository.Artifact, error) {
	ps, err := toPersistenceScope(scope)
	if err != nil {
		return repository.Artifact{}, err
	}
	physical, err := PhysicalArtifactID(artifactID)
	if err != nil {
		return repository.Artifact{}, err
	}
	query, err := store.contract.NewArtifactQuery(ps, persistence.RepositoryID(repositoryID), persistence.ScanID(scanID), physical)
	if err != nil {
		return repository.Artifact{}, serviceFailure(err, "get-artifact", "persistence-contract-failed")
	}
	record, err := store.read.GetArtifact(ctx, query)
	if err != nil {
		return repository.Artifact{}, serviceFailure(err, "get-artifact", "persistence-contract-failed")
	}
	value, err := store.artifactRecord(scope, repositoryID, scanID, record)
	if err != nil {
		return repository.Artifact{}, err
	}
	if value.ArtifactID() != artifactID {
		return repository.Artifact{}, integrity("get-artifact", "physical-artifact-id-mismatch")
	}
	return value, nil
}
func (store *Store) ListArtifacts(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, size int, cursor repository.Cursor) (scan.ArtifactList, error) {
	ps, err := toPersistenceScope(scope)
	if err != nil {
		return scan.ArtifactList{}, err
	}
	request, err := store.contract.NewArtifactListRequest(ps, persistence.RepositoryID(repositoryID), persistence.ScanID(scanID), size, persistence.Cursor(cursor))
	if err != nil {
		return scan.ArtifactList{}, serviceFailure(err, "list-artifacts", "persistence-contract-failed")
	}
	page, err := store.read.ListArtifacts(ctx, request)
	if err != nil {
		return scan.ArtifactList{}, serviceFailure(err, "list-artifacts", "persistence-contract-failed")
	}
	values := make([]repository.Artifact, 0, len(page.Records()))
	for _, record := range page.Records() {
		value, mapErr := store.artifactRecord(scope, repositoryID, scanID, record)
		if mapErr != nil {
			return scan.ArtifactList{}, mapErr
		}
		values = append(values, value)
	}
	return scan.NewArtifactList(values, repository.Cursor(page.NextCursor()))
}
func (store *Store) ExportArtifact(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, artifactID repository.ArtifactID, writer io.Writer) (repository.ExportReceipt, error) {
	if writer == nil {
		return repository.ExportReceipt{}, repository.NewError(repository.ErrorInvalidInput, "export-artifact", "invalid-writer", false, nil)
	}
	metadata, err := store.GetArtifact(ctx, scope, repositoryID, scanID, artifactID)
	if err != nil {
		return repository.ExportReceipt{}, err
	}
	ps, _ := toPersistenceScope(scope)
	physical, _ := PhysicalArtifactID(artifactID)
	query, err := store.contract.NewPayloadQuery(ps, persistence.RepositoryID(repositoryID), persistence.ScanID(scanID), physical, toPersistenceDigest(metadata.PayloadDigest()))
	if err != nil {
		return repository.ExportReceipt{}, serviceFailure(err, "export-artifact", "persistence-contract-failed")
	}
	receipt, err := store.read.ExportPayload(ctx, query, writer)
	if err != nil {
		return repository.ExportReceipt{}, serviceFailure(err, "export-artifact", "persistence-contract-failed")
	}
	if receipt.Digest() != query.Digest() || uint64(receipt.Size()) != metadata.PayloadSize() {
		return repository.ExportReceipt{}, integrity("export-artifact", "payload-receipt-mismatch")
	}
	return repository.NewExportReceipt(metadata.PayloadDigest(), metadata.PayloadSize())
}

func (store *Store) identity(value repository.Scope) (persistence.Scope, persistence.AuditActor, error) {
	scope, err := toPersistenceScope(value)
	if err != nil {
		return persistence.Scope{}, persistence.AuditActor{}, err
	}
	actor, err := persistence.NewAuditActor("principal", string(value.PrincipalID()))
	if err != nil {
		return persistence.Scope{}, persistence.AuditActor{}, serviceFailure(err, "map-identity", "identifier-incompatible")
	}
	return scope, actor, nil
}
func toPersistenceScope(value repository.Scope) (persistence.Scope, error) {
	if value.IsZero() {
		return persistence.Scope{}, repository.NewError(repository.ErrorInvalidInput, "map-scope", "identifier-incompatible", false, nil)
	}
	scope, err := persistence.NewScope(string(value.ScopeID()), string(value.PrincipalID()))
	if err != nil {
		return persistence.Scope{}, serviceFailure(err, "map-scope", "identifier-incompatible")
	}
	return scope, nil
}

func (store *Store) repositoryRecord(scope repository.Scope, record persistence.RepositoryRecord) (repository.Repository, error) {
	if record.ScopeID() != string(scope.ScopeID()) {
		return repository.Repository{}, integrity("map-repository", "record-mismatch")
	}
	state := repository.RepositoryState(record.State())
	value, err := repository.NewRepository(repository.RepositoryParams{RepositoryID: repository.RepositoryID(record.RepositoryID()), DisplayName: record.DisplayName(), SourceKind: record.Source().Kind(), FingerprintScheme: record.Source().FingerprintScheme(), Fingerprint: toRepositoryDigest(record.Source().Fingerprint()), State: state, CurrentScanID: repository.ScanID(record.CurrentScanID()), CreatedAt: record.CreatedAt().UTC(), UpdatedAt: record.UpdatedAt().UTC()})
	if err != nil {
		return repository.Repository{}, integrity("map-repository", "record-invalid")
	}
	return value, nil
}
func (store *Store) scanRecord(scope repository.Scope, record persistence.ScanRecord) (repository.Scan, error) {
	if record.ScopeID() != string(scope.ScopeID()) {
		return repository.Scan{}, integrity("map-scan", "record-mismatch")
	}
	digest := toRepositoryDigest(record.AnalysisProfileDigest())
	var profile repository.AnalysisProfile
	found := false
	for _, definition := range store.profiles.Definitions() {
		candidate := definition.Profile()
		if candidate.Digest() == digest {
			profile, found = candidate, true
			break
		}
	}
	if !found {
		return repository.Scan{}, integrity("map-scan", "profile-unrecognized")
	}
	state, ok := map[persistence.ScanState]repository.ScanState{persistence.ScanRunning: repository.ScanRunning, persistence.ScanSucceeded: repository.ScanSucceeded, persistence.ScanFailed: repository.ScanFailed, persistence.ScanCancelled: repository.ScanCanceled}[record.State()]
	if !ok {
		return repository.Scan{}, integrity("map-scan", "record-invalid")
	}
	value, err := repository.NewScan(repository.ScanParams{RepositoryID: repository.RepositoryID(record.RepositoryID()), ScanID: repository.ScanID(record.ScanID()), Profile: profile, SourceRevision: record.SourceRevision(), State: state, ReasonCode: record.ReasonCode(), RequestedAt: record.RequestedAt().UTC(), StartedAt: record.StartedAt().UTC(), FinishedAt: record.FinishedAt().UTC()})
	if err != nil {
		return repository.Scan{}, integrity("map-scan", "record-invalid")
	}
	return value, nil
}
func (store *Store) artifactRecord(scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, record persistence.ArtifactRecord) (repository.Artifact, error) {
	public, err := repository.NewArtifactID(repositoryID, scanID, record.Artifact().Name(), record.Artifact().Version(), record.StableIDScheme())
	if err != nil {
		return repository.Artifact{}, integrity("map-artifact", "record-invalid")
	}
	physical, _ := PhysicalArtifactID(public)
	if record.ScopeID() != string(scope.ScopeID()) || physical != record.ArtifactID() || record.RepositoryID() != persistence.RepositoryID(repositoryID) || record.ScanID() != persistence.ScanID(scanID) {
		return repository.Artifact{}, integrity("map-artifact", "physical-artifact-id-mismatch")
	}
	value, err := repository.NewArtifact(repository.ArtifactParams{ArtifactID: public, ScanID: scanID, Name: record.Artifact().Name(), Version: record.Artifact().Version(), StableIDScheme: record.StableIDScheme(), CodecName: record.Codec().Name(), CodecVersion: record.Codec().Version(), MediaType: record.Codec().MediaType(), PayloadDigest: toRepositoryDigest(record.PayloadDigest()), PayloadSize: uint64(record.PayloadSize()), ProducerName: record.Producer().Name(), ProducerVersion: record.Producer().Version(), CreatedAt: record.CreatedAt().UTC()})
	if err != nil {
		return repository.Artifact{}, integrity("map-artifact", "record-invalid")
	}
	return value, nil
}

func (store *Store) submissions(command scan.PublishCommand, physical map[repository.ArtifactID]persistence.ArtifactID) ([]persistence.ArtifactSubmission, []persistence.DependencySubmission, error) {
	submissions := make([]persistence.ArtifactSubmission, 0, len(command.Artifacts()))
	dependencies := []persistence.DependencySubmission{}
	byName := map[string]repository.ArtifactID{}
	for _, item := range command.Artifacts() {
		metadata := item.Metadata()
		byName[metadata.Name()+"\x00"+metadata.Version()] = metadata.ArtifactID()
		artifact, _ := persistence.NewVersionedName(metadata.Name(), metadata.Version())
		codec, _ := persistence.NewCodec(metadata.CodecName(), metadata.CodecVersion(), metadata.MediaType())
		producer, _ := persistence.NewVersionedName(metadata.ProducerName(), metadata.ProducerVersion())
		value, err := store.contract.NewArtifactSubmission(persistence.ArtifactSubmissionParams{ArtifactID: physical[metadata.ArtifactID()], Artifact: artifact, StableIDScheme: metadata.StableIDScheme(), Codec: codec, PayloadDigest: toPersistenceDigest(metadata.PayloadDigest()), PayloadSize: persistence.ByteCount(metadata.PayloadSize()), Producer: producer})
		if err != nil {
			return nil, nil, serviceFailure(err, "publish-scan", "persistence-contract-failed")
		}
		submissions = append(submissions, value)
	}
	for _, item := range command.Artifacts() {
		for _, dep := range item.Dependencies() {
			sourcePublic, ok := byName[dep.Name()+"\x00"+dep.Version()]
			if !ok {
				return nil, nil, integrity("publish-scan", "manifest-build-failed")
			}
			declared, _ := persistence.NewVersionedName(dep.Name(), dep.Version())
			value, err := store.contract.NewDependencySubmission(physical[item.Metadata().ArtifactID()], uint32(dep.Ordinal()), physical[sourcePublic], declared)
			if err != nil {
				return nil, nil, serviceFailure(err, "publish-scan", "persistence-contract-failed")
			}
			dependencies = append(dependencies, value)
		}
	}
	if len(dependencies) > store.config.MaxDependencies {
		return nil, nil, integrity("publish-scan", "manifest-build-failed")
	}
	return submissions, dependencies, nil
}
func (store *Store) allArtifacts(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID) ([]repository.Artifact, error) {
	cursor := repository.Cursor("")
	result := []repository.Artifact{}
	for {
		page, err := store.ListArtifacts(ctx, scope, repositoryID, scanID, store.config.ReadPageSize, cursor)
		if err != nil {
			return nil, err
		}
		result = append(result, page.Items()...)
		if page.NextCursor() == "" {
			break
		}
		cursor = page.NextCursor()
		if len(result) > store.config.MaxArtifacts {
			return nil, integrity("list-artifacts", "record-mismatch")
		}
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		return left.Name()+"\x00"+left.Version()+"\x00"+left.StableIDScheme() < right.Name()+"\x00"+right.Version()+"\x00"+right.StableIDScheme()
	})
	return result, nil
}
func (store *Store) publishedResult(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID, scanID repository.ScanID, disposition repository.Disposition) (repository.ScanResult, error) {
	value, err := store.GetScan(ctx, scope, repositoryID, scanID)
	if err != nil {
		return repository.ScanResult{}, err
	}
	artifacts, err := store.allArtifacts(ctx, scope, repositoryID, scanID)
	if err != nil {
		return repository.ScanResult{}, err
	}
	return repository.NewScanResult(value, artifacts, disposition)
}
