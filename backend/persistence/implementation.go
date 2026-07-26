package persistence

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	machineKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*$`)
	decimalPattern    = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	windowsAbsPattern = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
)

// Contract validates and constructs immutable neutral values.
type Contract struct{ config Config }

// New returns a persistence contract using explicit or default limits.
func New(configs ...Config) (*Contract, error) {
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
	return &Contract{config: config}, nil
}

// Config returns the immutable effective limits.
func (contract *Contract) Config() Config {
	if contract == nil {
		return Config{}
	}
	return contract.config
}

func NewScope(scopeID, principalID string) (Scope, error) {
	if err := validateMachine("scope ID", scopeID, 128); err != nil {
		return Scope{}, err
	}
	if err := validateOpaque("principal ID", principalID, 256); err != nil {
		return Scope{}, err
	}
	return Scope{scopeID: scopeID, principalID: principalID}, nil
}

func NewVersionedName(name, version string) (VersionedName, error) {
	if err := validateMachine("name", name, 128); err != nil {
		return VersionedName{}, err
	}
	if err := validateVersion(version); err != nil {
		return VersionedName{}, err
	}
	return VersionedName{name: name, version: version}, nil
}

func NewCodec(name, version, mediaType string) (Codec, error) {
	identity, err := NewVersionedName(name, version)
	if err != nil {
		return Codec{}, err
	}
	if strings.TrimSpace(mediaType) != mediaType || len(mediaType) < 3 || len(mediaType) > 128 || !strings.Contains(mediaType, "/") || hasControl(mediaType) {
		return Codec{}, invalid("codec media type")
	}
	return Codec{name: identity.name, version: identity.version, mediaType: mediaType}, nil
}

func NewSourceIdentity(kind, fingerprintScheme string, fingerprint Digest) (SourceIdentity, error) {
	if err := validateMachine("source kind", kind, 64); err != nil {
		return SourceIdentity{}, err
	}
	if err := validateMachine("source fingerprint scheme", fingerprintScheme, 128); err != nil {
		return SourceIdentity{}, err
	}
	if fingerprint.IsZero() {
		return SourceIdentity{}, invalid("source fingerprint")
	}
	return SourceIdentity{kind: kind, fingerprintScheme: fingerprintScheme, fingerprint: fingerprint}, nil
}

func NewAuditActor(kind, id string) (AuditActor, error) {
	if err := validateMachine("actor kind", kind, 64); err != nil {
		return AuditActor{}, err
	}
	if err := validateOpaque("actor ID", id, 256); err != nil {
		return AuditActor{}, err
	}
	return AuditActor{kind: kind, id: id}, nil
}

func (contract *Contract) NewRegisterRepositoryRequest(params RegisterRepositoryParams) (RegisterRepositoryRequest, error) {
	if err := validateBase(params.Scope, params.RequestID, params.RepositoryID, params.Actor); err != nil {
		return RegisterRepositoryRequest{}, err
	}
	if err := validateSafeText("display name", params.DisplayName, 256, false); err != nil {
		return RegisterRepositoryRequest{}, err
	}
	if params.Source.IsZero() {
		return RegisterRepositoryRequest{}, invalid("source identity")
	}
	return RegisterRepositoryRequest{params: params}, nil
}

func (contract *Contract) NewRepositoryQuery(scope Scope, repositoryID RepositoryID) (RepositoryQuery, error) {
	if err := validateScopeTarget(scope, repositoryID); err != nil {
		return RepositoryQuery{}, err
	}
	return RepositoryQuery{scope: scope, repositoryID: repositoryID}, nil
}

func (contract *Contract) NewRepositoryListRequest(scope Scope, pageSize int, cursor Cursor) (RepositoryListRequest, error) {
	if err := validateScope(scope); err != nil {
		return RepositoryListRequest{}, err
	}
	if err := contract.validatePage(pageSize, cursor); err != nil {
		return RepositoryListRequest{}, err
	}
	return RepositoryListRequest{scope: scope, pageSize: pageSize, cursor: cursor}, nil
}

func (contract *Contract) NewArchiveRepositoryRequest(scope Scope, requestID RequestID, repositoryID RepositoryID, actor AuditActor) (ArchiveRepositoryRequest, error) {
	if err := validateBase(scope, requestID, repositoryID, actor); err != nil {
		return ArchiveRepositoryRequest{}, err
	}
	return ArchiveRepositoryRequest{scope: scope, requestID: requestID, repositoryID: repositoryID, actor: actor}, nil
}

func (contract *Contract) NewBeginScanRequest(params BeginScanParams) (BeginScanRequest, error) {
	if err := validateBase(params.Scope, params.RequestID, params.RepositoryID, params.Actor); err != nil {
		return BeginScanRequest{}, err
	}
	if err := validateID("scan ID", string(params.ScanID)); err != nil {
		return BeginScanRequest{}, err
	}
	if params.AnalysisProfileDigest.IsZero() {
		return BeginScanRequest{}, invalid("analysis profile digest")
	}
	if err := validateSafeText("source revision", params.SourceRevision, 512, true); err != nil {
		return BeginScanRequest{}, err
	}
	return BeginScanRequest{params: params}, nil
}

func (contract *Contract) NewScanQuery(scope Scope, repositoryID RepositoryID, scanID ScanID) (ScanQuery, error) {
	if err := validateScopeTarget(scope, repositoryID); err != nil {
		return ScanQuery{}, err
	}
	if err := validateID("scan ID", string(scanID)); err != nil {
		return ScanQuery{}, err
	}
	return ScanQuery{scope: scope, repositoryID: repositoryID, scanID: scanID}, nil
}

func (contract *Contract) NewScanListRequest(scope Scope, repositoryID RepositoryID, pageSize int, cursor Cursor) (ScanListRequest, error) {
	if err := validateScopeTarget(scope, repositoryID); err != nil {
		return ScanListRequest{}, err
	}
	if err := contract.validatePage(pageSize, cursor); err != nil {
		return ScanListRequest{}, err
	}
	return ScanListRequest{scope: scope, repositoryID: repositoryID, pageSize: pageSize, cursor: cursor}, nil
}

func (contract *Contract) NewFinishScanRequest(params FinishScanParams) (FinishScanRequest, error) {
	if err := validateBase(params.Scope, params.RequestID, params.RepositoryID, params.Actor); err != nil {
		return FinishScanRequest{}, err
	}
	if err := validateID("scan ID", string(params.ScanID)); err != nil {
		return FinishScanRequest{}, err
	}
	if err := validateMachine("reason code", params.ReasonCode, 128); err != nil {
		return FinishScanRequest{}, err
	}
	if err := validateSafeText("safe message", params.SafeMessage, 4096, true); err != nil {
		return FinishScanRequest{}, err
	}
	return FinishScanRequest{params: params}, nil
}

func (contract *Contract) NewStagePayloadRequest(params StagePayloadParams) (StagePayloadRequest, error) {
	if err := validateScopeTarget(params.Scope, params.RepositoryID); err != nil {
		return StagePayloadRequest{}, err
	}
	if err := validateID("request ID", string(params.RequestID)); err != nil {
		return StagePayloadRequest{}, err
	}
	if err := validateID("scan ID", string(params.ScanID)); err != nil {
		return StagePayloadRequest{}, err
	}
	if params.Digest.IsZero() {
		return StagePayloadRequest{}, invalid("payload digest")
	}
	if params.ExpectedSize > contract.config.MaxPayloadBytes {
		return StagePayloadRequest{}, NewError(ErrorPayloadTooLarge, "new-stage-payload-request", false, nil)
	}
	return StagePayloadRequest{params: params}, nil
}

func (contract *Contract) NewArtifactSubmission(params ArtifactSubmissionParams) (ArtifactSubmission, error) {
	if err := validateID("artifact ID", string(params.ArtifactID)); err != nil {
		return ArtifactSubmission{}, err
	}
	if params.Artifact.IsZero() || params.Codec.IsZero() || params.Producer.IsZero() {
		return ArtifactSubmission{}, invalid("artifact identity, codec, and producer")
	}
	if params.StableIDScheme != "" {
		if err := validateMachine("stable ID scheme", params.StableIDScheme, 128); err != nil {
			return ArtifactSubmission{}, err
		}
	}
	if params.PayloadDigest.IsZero() {
		return ArtifactSubmission{}, invalid("payload digest")
	}
	if params.PayloadSize > contract.config.MaxPayloadBytes {
		return ArtifactSubmission{}, NewError(ErrorPayloadTooLarge, "new-artifact-submission", false, nil)
	}
	return ArtifactSubmission{params: params}, nil
}

func (contract *Contract) NewDependencySubmission(consumer ArtifactID, ordinal uint32, source ArtifactID, declared VersionedName) (DependencySubmission, error) {
	if err := validateID("consumer artifact ID", string(consumer)); err != nil {
		return DependencySubmission{}, err
	}
	if err := validateID("source artifact ID", string(source)); err != nil {
		return DependencySubmission{}, err
	}
	if consumer == source || declared.IsZero() || ordinal >= uint32(contract.config.MaxDependenciesPerArtifact) {
		return DependencySubmission{}, NewError(ErrorInvalidDependency, "new-dependency-submission", false, nil)
	}
	return DependencySubmission{consumerArtifactID: consumer, ordinal: ordinal, sourceArtifactID: source, declaredArtifact: declared}, nil
}

func (contract *Contract) NewProjectionSubmission(params ProjectionSubmissionParams) (ProjectionSubmission, error) {
	if err := validateID("projection ID", string(params.ProjectionID)); err != nil {
		return ProjectionSubmission{}, err
	}
	if err := validateID("artifact ID", string(params.ArtifactID)); err != nil {
		return ProjectionSubmission{}, err
	}
	if params.SourceDigest.IsZero() || params.ProjectionDigest.IsZero() || params.Projector.IsZero() {
		return ProjectionSubmission{}, invalid("projection identity or digest")
	}
	if err := validateVersion(params.SchemaVersion); err != nil {
		return ProjectionSubmission{}, err
	}
	if err := validateMachine("projection digest scheme", params.DigestScheme, 128); err != nil {
		return ProjectionSubmission{}, err
	}
	if len(params.CanonicalJSON) > int(contract.config.MaxProjectionBytes) {
		return ProjectionSubmission{}, NewError(ErrorPayloadTooLarge, "new-projection-submission", false, nil)
	}
	var document map[string]json.RawMessage
	if !json.Valid(params.CanonicalJSON) || json.Unmarshal(params.CanonicalJSON, &document) != nil || document == nil || DigestBytes(params.CanonicalJSON) != params.ProjectionDigest {
		return ProjectionSubmission{}, NewError(ErrorIntegrityFailure, "new-projection-submission", false, nil)
	}
	params.CanonicalJSON = append([]byte(nil), params.CanonicalJSON...)
	return ProjectionSubmission{params: params}, nil
}

func (contract *Contract) NewDiagnosticSubmission(projectionID ProjectionID, ordinal uint32, severity, code, engine, relativePath string, line, column uint32, message string) (DiagnosticSubmission, error) {
	if err := validateID("projection ID", string(projectionID)); err != nil {
		return DiagnosticSubmission{}, err
	}
	if severity != "info" && severity != "warning" && severity != "error" {
		return DiagnosticSubmission{}, invalid("diagnostic severity")
	}
	if err := validateMachine("diagnostic code", code, 128); err != nil {
		return DiagnosticSubmission{}, err
	}
	if err := validateMachine("diagnostic engine", engine, 128); err != nil {
		return DiagnosticSubmission{}, err
	}
	if err := validateRelativePath(relativePath); err != nil {
		return DiagnosticSubmission{}, err
	}
	if column > 0 && line == 0 {
		return DiagnosticSubmission{}, invalid("diagnostic location")
	}
	if err := validateSafeText("diagnostic message", message, 4096, false); err != nil {
		return DiagnosticSubmission{}, err
	}
	return DiagnosticSubmission{projectionID: projectionID, ordinal: ordinal, severity: severity, code: code, engine: engine, relativePath: relativePath, line: line, column: column, message: message}, nil
}

func NewIntegerStatistic(value int64) StatisticValue {
	return StatisticValue{kind: StatisticInteger, integer: value}
}

func NewDecimalStatistic(value string) (StatisticValue, error) {
	if len(value) > 128 || !decimalPattern.MatchString(value) {
		return StatisticValue{}, invalid("decimal statistic")
	}
	return StatisticValue{kind: StatisticDecimal, decimal: value}, nil
}

func NewBooleanStatistic(value bool) StatisticValue {
	return StatisticValue{kind: StatisticBoolean, boolean: value}
}

func NewTextStatistic(value string) (StatisticValue, error) {
	if err := validateSafeText("text statistic", value, 4096, true); err != nil {
		return StatisticValue{}, err
	}
	return StatisticValue{kind: StatisticText, text: value}, nil
}

func (contract *Contract) NewStatisticSubmission(projectionID ProjectionID, key string, value StatisticValue, unit string) (StatisticSubmission, error) {
	if err := validateID("projection ID", string(projectionID)); err != nil {
		return StatisticSubmission{}, err
	}
	if err := validateMachine("statistic key", key, 128); err != nil {
		return StatisticSubmission{}, err
	}
	if !value.kind.valid() {
		return StatisticSubmission{}, invalid("statistic value")
	}
	if unit != "" {
		if err := validateMachine("statistic unit", unit, 64); err != nil {
			return StatisticSubmission{}, err
		}
	}
	return StatisticSubmission{projectionID: projectionID, key: key, value: value, unit: unit}, nil
}

func (contract *Contract) NewPublishScanRequest(params PublishScanParams) (PublishScanRequest, error) {
	if err := validateBase(params.Scope, params.RequestID, params.RepositoryID, params.Actor); err != nil {
		return PublishScanRequest{}, err
	}
	if err := validateID("scan ID", string(params.ScanID)); err != nil {
		return PublishScanRequest{}, err
	}
	if err := validateMachine("manifest scheme", params.ManifestScheme, 128); err != nil {
		return PublishScanRequest{}, err
	}
	if params.ManifestDigest.IsZero() {
		return PublishScanRequest{}, invalid("manifest digest")
	}
	if len(params.Artifacts) < 1 || len(params.Artifacts) > contract.config.MaxArtifactsPerPublication {
		return PublishScanRequest{}, invalid("artifact count")
	}
	if len(params.Diagnostics) > contract.config.MaxDiagnostics || len(params.Statistics) > contract.config.MaxStatistics {
		return PublishScanRequest{}, invalid("projection child count")
	}
	if err := contract.validatePublicationGraph(params); err != nil {
		return PublishScanRequest{}, err
	}
	params.Artifacts = cloneArtifacts(params.Artifacts)
	params.Dependencies = append([]DependencySubmission(nil), params.Dependencies...)
	params.Projections = cloneProjections(params.Projections)
	params.Diagnostics = append([]DiagnosticSubmission(nil), params.Diagnostics...)
	params.Statistics = append([]StatisticSubmission(nil), params.Statistics...)
	return PublishScanRequest{params: params}, nil
}

func (contract *Contract) NewArtifactQuery(scope Scope, repositoryID RepositoryID, scanID ScanID, artifactID ArtifactID) (ArtifactQuery, error) {
	if err := validateTargetIDs(scope, repositoryID, scanID, artifactID); err != nil {
		return ArtifactQuery{}, err
	}
	return ArtifactQuery{scope: scope, repositoryID: repositoryID, scanID: scanID, artifactID: artifactID}, nil
}

func (contract *Contract) NewArtifactListRequest(scope Scope, repositoryID RepositoryID, scanID ScanID, pageSize int, cursor Cursor) (ArtifactListRequest, error) {
	if err := validateScopeTarget(scope, repositoryID); err != nil {
		return ArtifactListRequest{}, err
	}
	if err := validateID("scan ID", string(scanID)); err != nil {
		return ArtifactListRequest{}, err
	}
	if err := contract.validatePage(pageSize, cursor); err != nil {
		return ArtifactListRequest{}, err
	}
	return ArtifactListRequest{scope: scope, repositoryID: repositoryID, scanID: scanID, pageSize: pageSize, cursor: cursor}, nil
}

func (contract *Contract) NewPayloadQuery(scope Scope, repositoryID RepositoryID, scanID ScanID, artifactID ArtifactID, digest Digest) (PayloadQuery, error) {
	if err := validateTargetIDs(scope, repositoryID, scanID, artifactID); err != nil {
		return PayloadQuery{}, err
	}
	if digest.IsZero() {
		return PayloadQuery{}, invalid("payload digest")
	}
	return PayloadQuery{scope: scope, repositoryID: repositoryID, scanID: scanID, artifactID: artifactID, digest: digest}, nil
}

func (contract *Contract) NewMarkForPurgeRequest(scope Scope, requestID RequestID, repositoryID RepositoryID, actor AuditActor) (MarkForPurgeRequest, error) {
	if err := validateBase(scope, requestID, repositoryID, actor); err != nil {
		return MarkForPurgeRequest{}, err
	}
	return MarkForPurgeRequest{scope: scope, requestID: requestID, repositoryID: repositoryID, actor: actor}, nil
}

func (contract *Contract) NewPurgeBatchRequest(scope Scope, requestID RequestID, repositoryID RepositoryID, limit int, actor AuditActor) (PurgeBatchRequest, error) {
	if err := validateBase(scope, requestID, repositoryID, actor); err != nil {
		return PurgeBatchRequest{}, err
	}
	if limit < 1 || limit > contract.config.MaxRetentionBatch {
		return PurgeBatchRequest{}, invalid("purge batch limit")
	}
	return PurgeBatchRequest{scope: scope, requestID: requestID, repositoryID: repositoryID, limit: limit, actor: actor}, nil
}

func (contract *Contract) NewGarbageCollectionRequest(scope Scope, requestID RequestID, limit int, actor AuditActor) (GarbageCollectionRequest, error) {
	if err := validateScope(scope); err != nil {
		return GarbageCollectionRequest{}, err
	}
	if err := validateID("request ID", string(requestID)); err != nil {
		return GarbageCollectionRequest{}, err
	}
	if actor.IsZero() || limit < 1 || limit > contract.config.MaxRetentionBatch {
		return GarbageCollectionRequest{}, invalid("garbage collection request")
	}
	return GarbageCollectionRequest{scope: scope, requestID: requestID, limit: limit, actor: actor}, nil
}

// Record constructors are for adapter implementations. They validate durable
// values without exposing adapter row models.
func NewRepositoryRecord(scopeID string, repositoryID RepositoryID, displayName string, source SourceIdentity, state RepositoryState, currentScanID ScanID, createdAt, updatedAt time.Time) (RepositoryRecord, error) {
	if err := validateMachine("scope ID", scopeID, 128); err != nil || !state.valid() || createdAt.IsZero() || updatedAt.Before(createdAt) {
		return RepositoryRecord{}, invalid("repository record")
	}
	if err := validateID("repository ID", string(repositoryID)); err != nil {
		return RepositoryRecord{}, err
	}
	if err := validateSafeText("display name", displayName, 256, false); err != nil {
		return RepositoryRecord{}, err
	}
	if source.IsZero() {
		return RepositoryRecord{}, invalid("source identity")
	}
	if currentScanID != "" {
		if err := validateID("current scan ID", string(currentScanID)); err != nil {
			return RepositoryRecord{}, err
		}
	}
	return RepositoryRecord{scopeID: scopeID, repositoryID: repositoryID, displayName: displayName, source: source, state: state, currentScanID: currentScanID, createdAt: createdAt, updatedAt: updatedAt}, nil
}

func NewScanRecord(scopeID string, repositoryID RepositoryID, scanID ScanID, analysisProfileDigest Digest, sourceRevision string, state ScanState, reasonCode, safeMessage string, requestedAt, startedAt, finishedAt time.Time) (ScanRecord, error) {
	if err := validateMachine("scope ID", scopeID, 128); err != nil || !state.valid() || requestedAt.IsZero() || analysisProfileDigest.IsZero() {
		return ScanRecord{}, invalid("scan record")
	}
	if err := validateID("repository ID", string(repositoryID)); err != nil {
		return ScanRecord{}, err
	}
	if err := validateID("scan ID", string(scanID)); err != nil {
		return ScanRecord{}, err
	}
	if reasonCode != "" {
		if err := validateMachine("reason code", reasonCode, 128); err != nil {
			return ScanRecord{}, err
		}
	}
	if err := validateSafeText("safe message", safeMessage, 4096, true); err != nil {
		return ScanRecord{}, err
	}
	if err := validateSafeText("source revision", sourceRevision, 512, true); err != nil {
		return ScanRecord{}, err
	}
	if (!startedAt.IsZero() && startedAt.Before(requestedAt)) || (!finishedAt.IsZero() && (startedAt.IsZero() || finishedAt.Before(startedAt))) {
		return ScanRecord{}, invalid("scan timestamps")
	}
	switch state {
	case ScanRequested:
		if !startedAt.IsZero() || !finishedAt.IsZero() {
			return ScanRecord{}, invalid("requested scan timestamps")
		}
	case ScanRunning:
		if startedAt.IsZero() || !finishedAt.IsZero() {
			return ScanRecord{}, invalid("running scan timestamps")
		}
	case ScanSucceeded:
		if startedAt.IsZero() || finishedAt.IsZero() {
			return ScanRecord{}, invalid("succeeded scan timestamps")
		}
	case ScanFailed, ScanCancelled:
		if startedAt.IsZero() || finishedAt.IsZero() || reasonCode == "" {
			return ScanRecord{}, invalid("terminal scan evidence")
		}
	}
	return ScanRecord{scopeID: scopeID, repositoryID: repositoryID, scanID: scanID, analysisProfileDigest: analysisProfileDigest, sourceRevision: sourceRevision, state: state, reasonCode: reasonCode, safeMessage: safeMessage, requestedAt: requestedAt, startedAt: startedAt, finishedAt: finishedAt}, nil
}

func NewArtifactRecord(scopeID string, repositoryID RepositoryID, scanID ScanID, artifactID ArtifactID, artifact VersionedName, stableIDScheme string, codec Codec, producer VersionedName, digest Digest, size ByteCount, createdAt time.Time) (ArtifactRecord, error) {
	if err := validateMachine("scope ID", scopeID, 128); err != nil || artifact.IsZero() || codec.IsZero() || producer.IsZero() || digest.IsZero() || createdAt.IsZero() || size > defaultMaxPayloadBytes {
		return ArtifactRecord{}, invalid("artifact record")
	}
	if err := validateID("repository ID", string(repositoryID)); err != nil {
		return ArtifactRecord{}, err
	}
	if err := validateID("scan ID", string(scanID)); err != nil {
		return ArtifactRecord{}, err
	}
	if err := validateID("artifact ID", string(artifactID)); err != nil {
		return ArtifactRecord{}, err
	}
	if stableIDScheme != "" {
		if err := validateMachine("stable ID scheme", stableIDScheme, 128); err != nil {
			return ArtifactRecord{}, err
		}
	}
	return ArtifactRecord{scopeID: scopeID, repositoryID: repositoryID, scanID: scanID, artifactID: artifactID, artifact: artifact, stableIDScheme: stableIDScheme, codec: codec, producer: producer, payloadDigest: digest, payloadSize: size, createdAt: createdAt}, nil
}

func NewRepositoryPage(records []RepositoryRecord, next Cursor) RepositoryPage {
	return RepositoryPage{records: append([]RepositoryRecord(nil), records...), next: next}
}

func NewScanPage(records []ScanRecord, next Cursor) ScanPage {
	return ScanPage{records: append([]ScanRecord(nil), records...), next: next}
}

func NewArtifactPage(records []ArtifactRecord, next Cursor) ArtifactPage {
	return ArtifactPage{records: append([]ArtifactRecord(nil), records...), next: next}
}

func NewPayloadReceipt(digest Digest, size ByteCount, disposition Disposition) (PayloadReceipt, error) {
	if digest.IsZero() || size > defaultMaxPayloadBytes || !disposition.valid() {
		return PayloadReceipt{}, invalid("payload receipt")
	}
	return PayloadReceipt{digest: digest, size: size, disposition: disposition}, nil
}

func NewPublicationReceipt(scanID ScanID, manifestScheme string, manifestDigest Digest, artifactCount uint32, disposition Disposition) (PublicationReceipt, error) {
	if err := validateID("scan ID", string(scanID)); err != nil {
		return PublicationReceipt{}, err
	}
	if err := validateMachine("manifest scheme", manifestScheme, 128); err != nil {
		return PublicationReceipt{}, err
	}
	if manifestDigest.IsZero() || artifactCount < 1 || artifactCount > defaultMaxArtifactsPerPublication || !disposition.valid() {
		return PublicationReceipt{}, invalid("publication receipt")
	}
	return PublicationReceipt{scanID: scanID, manifestScheme: manifestScheme, manifestDigest: manifestDigest, artifactCount: artifactCount, disposition: disposition}, nil
}

func NewVerificationReceipt(digest Digest, size ByteCount) (VerificationReceipt, error) {
	if digest.IsZero() || size > defaultMaxPayloadBytes {
		return VerificationReceipt{}, invalid("verification receipt")
	}
	return VerificationReceipt{digest: digest, size: size}, nil
}

func NewPurgeReceipt(artifacts, scans uint64, complete bool) PurgeReceipt {
	return PurgeReceipt{removedArtifacts: artifacts, removedScans: scans, complete: complete}
}

func NewGarbageCollectionReceipt(payloads uint64, bytes ByteCount) GarbageCollectionReceipt {
	return GarbageCollectionReceipt{removedPayloads: payloads, removedBytes: bytes}
}

func (contract *Contract) validatePublicationGraph(params PublishScanParams) error {
	artifacts := make(map[ArtifactID]ArtifactSubmission, len(params.Artifacts))
	names := make(map[string]struct{}, len(params.Artifacts))
	for _, artifact := range params.Artifacts {
		if artifact.params.ArtifactID == "" || artifact.params.Artifact.IsZero() || artifact.params.PayloadDigest.IsZero() {
			return invalid("artifact submission")
		}
		if _, exists := artifacts[artifact.params.ArtifactID]; exists {
			return NewError(ErrorDuplicateArtifact, "new-publish-scan-request", false, nil)
		}
		if _, exists := names[artifact.params.Artifact.Name()]; exists {
			return NewError(ErrorDuplicateArtifact, "new-publish-scan-request", false, nil)
		}
		artifacts[artifact.params.ArtifactID] = artifact
		names[artifact.params.Artifact.Name()] = struct{}{}
	}
	ordinals := make(map[string]struct{}, len(params.Dependencies))
	sources := make(map[string]struct{}, len(params.Dependencies))
	counts := make(map[ArtifactID]int)
	for _, dependency := range params.Dependencies {
		consumer, consumerExists := artifacts[dependency.consumerArtifactID]
		source, sourceExists := artifacts[dependency.sourceArtifactID]
		if !consumerExists || !sourceExists || dependency.consumerArtifactID == dependency.sourceArtifactID ||
			dependency.declaredArtifact != source.params.Artifact {
			return NewError(ErrorInvalidDependency, "new-publish-scan-request", false, nil)
		}
		ordinalKey := fmt.Sprintf("%s/%d", dependency.consumerArtifactID, dependency.ordinal)
		sourceKey := fmt.Sprintf("%s/%s", dependency.consumerArtifactID, dependency.sourceArtifactID)
		if _, exists := ordinals[ordinalKey]; exists {
			return NewError(ErrorInvalidDependency, "new-publish-scan-request", false, nil)
		}
		if _, exists := sources[sourceKey]; exists {
			return NewError(ErrorInvalidDependency, "new-publish-scan-request", false, nil)
		}
		ordinals[ordinalKey] = struct{}{}
		sources[sourceKey] = struct{}{}
		counts[consumer.params.ArtifactID]++
		if counts[consumer.params.ArtifactID] > contract.config.MaxDependenciesPerArtifact {
			return NewError(ErrorInvalidDependency, "new-publish-scan-request", false, nil)
		}
	}
	projections := make(map[ProjectionID]struct{}, len(params.Projections))
	for _, projection := range params.Projections {
		artifact, exists := artifacts[projection.params.ArtifactID]
		var document map[string]json.RawMessage
		if !exists || artifact.params.PayloadDigest != projection.params.SourceDigest || !json.Valid(projection.params.CanonicalJSON) || json.Unmarshal(projection.params.CanonicalJSON, &document) != nil || document == nil || DigestBytes(projection.params.CanonicalJSON) != projection.params.ProjectionDigest {
			return NewError(ErrorIntegrityFailure, "new-publish-scan-request", false, nil)
		}
		if _, exists := projections[projection.params.ProjectionID]; exists {
			return invalid("duplicate projection")
		}
		projections[projection.params.ProjectionID] = struct{}{}
	}
	for _, diagnostic := range params.Diagnostics {
		if _, exists := projections[diagnostic.projectionID]; !exists {
			return invalid("diagnostic projection")
		}
	}
	for _, statistic := range params.Statistics {
		if _, exists := projections[statistic.projectionID]; !exists {
			return invalid("statistic projection")
		}
	}
	return nil
}

func (contract *Contract) validatePage(pageSize int, cursor Cursor) error {
	if pageSize < 1 || pageSize > contract.config.MaxPageSize {
		return invalid("page size")
	}
	if len(cursor) > 2048 || hasControl(string(cursor)) {
		return invalid("cursor")
	}
	return nil
}

func validateBase(scope Scope, requestID RequestID, repositoryID RepositoryID, actor AuditActor) error {
	if err := validateScopeTarget(scope, repositoryID); err != nil {
		return err
	}
	if err := validateID("request ID", string(requestID)); err != nil {
		return err
	}
	if actor.IsZero() {
		return invalid("audit actor")
	}
	return nil
}

func validateScopeTarget(scope Scope, repositoryID RepositoryID) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	return validateID("repository ID", string(repositoryID))
}

func validateTargetIDs(scope Scope, repositoryID RepositoryID, scanID ScanID, artifactID ArtifactID) error {
	if err := validateScopeTarget(scope, repositoryID); err != nil {
		return err
	}
	if err := validateID("scan ID", string(scanID)); err != nil {
		return err
	}
	return validateID("artifact ID", string(artifactID))
}

func validateScope(scope Scope) error {
	if scope.IsZero() {
		return invalid("scope")
	}
	return nil
}

func validateID(name, value string) error { return validateMachine(name, value, 128) }

func validateMachine(name, value string, maximum int) error {
	if len(value) < 1 || len(value) > maximum || !machineKeyPattern.MatchString(value) {
		return invalid(name)
	}
	return nil
}

func validateOpaque(name, value string, maximum int) error {
	if strings.TrimSpace(value) != value || len(value) < 1 || len(value) > maximum || hasControl(value) {
		return invalid(name)
	}
	return nil
}

func validateVersion(value string) error {
	if strings.TrimSpace(value) != value || len(value) < 1 || len(value) > 64 || strings.ContainsAny(value, " \t\r\n") || hasControl(value) {
		return invalid("version")
	}
	return nil
}

func validateSafeText(name, value string, maximum int, allowEmpty bool) error {
	if (!allowEmpty && value == "") || len(value) > maximum || strings.TrimSpace(value) != value || hasControl(value) {
		return invalid(name)
	}
	return nil
}

func validateRelativePath(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 4096 || strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\\`) || windowsAbsPattern.MatchString(value) || hasControl(value) {
		return invalid("relative path")
	}
	cleaned := path.Clean(strings.ReplaceAll(value, `\`, "/"))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return invalid("relative path")
	}
	return nil
}

func hasControl(value string) bool {
	return strings.IndexFunc(value, func(character rune) bool { return character < 0x20 || character == 0x7f }) >= 0
}

func invalid(subject string) error {
	return NewError(ErrorInvalidInput, "validate-"+strings.ReplaceAll(subject, " ", "-"), false, nil)
}

func (kind StatisticKind) valid() bool {
	switch kind {
	case StatisticInteger, StatisticDecimal, StatisticBoolean, StatisticText:
		return true
	default:
		return false
	}
}

func (state RepositoryState) valid() bool {
	return state == RepositoryActive || state == RepositoryArchived || state == RepositoryPurgePending
}

func (state ScanState) valid() bool {
	return state == ScanRequested || state == ScanRunning || state == ScanSucceeded || state == ScanFailed || state == ScanCancelled
}

func (disposition Disposition) valid() bool {
	return disposition == DispositionCreated || disposition == DispositionAlreadyPresent
}
