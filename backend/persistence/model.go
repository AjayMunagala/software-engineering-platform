package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type RepositoryID string
type ScanID string
type ArtifactID string
type ProjectionID string
type RequestID string
type Cursor string
type ByteCount uint64

// Digest is an exact SHA-256 value.
type Digest [sha256.Size]byte

// ParseDigest parses a lowercase or uppercase hexadecimal SHA-256 value.
func ParseDigest(value string) (Digest, error) {
	var digest Digest
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return digest, NewError(ErrorInvalidInput, "parse-digest", false, err)
	}
	copy(digest[:], decoded)
	return digest, nil
}

// DigestBytes computes SHA-256 over exact bytes.
func DigestBytes(value []byte) Digest { return sha256.Sum256(value) }

func (digest Digest) String() string { return hex.EncodeToString(digest[:]) }
func (digest Digest) IsZero() bool   { return digest == Digest{} }

// Scope is an already-authorized application namespace and actor. It is not a
// database role or tenancy model.
type Scope struct {
	scopeID     string
	principalID string
}

func (scope Scope) ScopeID() string     { return scope.scopeID }
func (scope Scope) PrincipalID() string { return scope.principalID }
func (scope Scope) IsZero() bool        { return scope.scopeID == "" || scope.principalID == "" }

// VersionedName identifies an artifact, producer, projector, or stable-ID
// scheme without importing its implementation package.
type VersionedName struct {
	name    string
	version string
}

func (value VersionedName) Name() string    { return value.name }
func (value VersionedName) Version() string { return value.version }
func (value VersionedName) IsZero() bool    { return value.name == "" || value.version == "" }

// Codec identifies the detached byte representation.
type Codec struct {
	name      string
	version   string
	mediaType string
}

func (codec Codec) Name() string      { return codec.name }
func (codec Codec) Version() string   { return codec.version }
func (codec Codec) MediaType() string { return codec.mediaType }
func (codec Codec) IsZero() bool {
	return codec.name == "" || codec.version == "" || codec.mediaType == ""
}

// SourceIdentity is a normalized non-secret repository source proof.
type SourceIdentity struct {
	kind              string
	fingerprintScheme string
	fingerprint       Digest
}

func (source SourceIdentity) Kind() string              { return source.kind }
func (source SourceIdentity) FingerprintScheme() string { return source.fingerprintScheme }
func (source SourceIdentity) Fingerprint() Digest       { return source.fingerprint }
func (source SourceIdentity) IsZero() bool {
	return source.kind == "" || source.fingerprintScheme == "" || source.fingerprint.IsZero()
}

// AuditActor is a safe opaque identity for an already-authorized operation.
type AuditActor struct {
	kind string
	id   string
}

func (actor AuditActor) Kind() string { return actor.kind }
func (actor AuditActor) ID() string   { return actor.id }
func (actor AuditActor) IsZero() bool { return actor.kind == "" || actor.id == "" }

// RepositoryState is the durable repository lifecycle.
type RepositoryState string

const (
	RepositoryActive       RepositoryState = "active"
	RepositoryArchived     RepositoryState = "archived"
	RepositoryPurgePending RepositoryState = "purge_pending"
)

// ScanState is the durable scan lifecycle.
type ScanState string

const (
	ScanRequested ScanState = "requested"
	ScanRunning   ScanState = "running"
	ScanSucceeded ScanState = "succeeded"
	ScanFailed    ScanState = "failed"
	ScanCancelled ScanState = "cancelled"
)

// Disposition distinguishes newly created state from idempotent success.
type Disposition string

const (
	DispositionCreated        Disposition = "created"
	DispositionAlreadyPresent Disposition = "already_present"
)

// RegisterRepositoryParams is mutable caller input used only by the
// constructor.
type RegisterRepositoryParams struct {
	Scope        Scope
	RequestID    RequestID
	RepositoryID RepositoryID
	DisplayName  string
	Source       SourceIdentity
	Actor        AuditActor
}

type RegisterRepositoryRequest struct{ params RegisterRepositoryParams }

func (request RegisterRepositoryRequest) Scope() Scope         { return request.params.Scope }
func (request RegisterRepositoryRequest) RequestID() RequestID { return request.params.RequestID }
func (request RegisterRepositoryRequest) RepositoryID() RepositoryID {
	return request.params.RepositoryID
}
func (request RegisterRepositoryRequest) DisplayName() string    { return request.params.DisplayName }
func (request RegisterRepositoryRequest) Source() SourceIdentity { return request.params.Source }
func (request RegisterRepositoryRequest) Actor() AuditActor      { return request.params.Actor }

type RepositoryQuery struct {
	scope        Scope
	repositoryID RepositoryID
}

func (query RepositoryQuery) Scope() Scope               { return query.scope }
func (query RepositoryQuery) RepositoryID() RepositoryID { return query.repositoryID }

type RepositoryListRequest struct {
	scope    Scope
	pageSize int
	cursor   Cursor
}

func (request RepositoryListRequest) Scope() Scope   { return request.scope }
func (request RepositoryListRequest) PageSize() int  { return request.pageSize }
func (request RepositoryListRequest) Cursor() Cursor { return request.cursor }

type ArchiveRepositoryRequest struct {
	scope        Scope
	requestID    RequestID
	repositoryID RepositoryID
	actor        AuditActor
}

func (request ArchiveRepositoryRequest) Scope() Scope               { return request.scope }
func (request ArchiveRepositoryRequest) RequestID() RequestID       { return request.requestID }
func (request ArchiveRepositoryRequest) RepositoryID() RepositoryID { return request.repositoryID }
func (request ArchiveRepositoryRequest) Actor() AuditActor          { return request.actor }

type BeginScanParams struct {
	Scope                 Scope
	RequestID             RequestID
	RepositoryID          RepositoryID
	ScanID                ScanID
	AnalysisProfileDigest Digest
	SourceRevision        string
	Actor                 AuditActor
}

type BeginScanRequest struct{ params BeginScanParams }

func (request BeginScanRequest) Scope() Scope               { return request.params.Scope }
func (request BeginScanRequest) RequestID() RequestID       { return request.params.RequestID }
func (request BeginScanRequest) RepositoryID() RepositoryID { return request.params.RepositoryID }
func (request BeginScanRequest) ScanID() ScanID             { return request.params.ScanID }
func (request BeginScanRequest) AnalysisProfileDigest() Digest {
	return request.params.AnalysisProfileDigest
}
func (request BeginScanRequest) SourceRevision() string { return request.params.SourceRevision }
func (request BeginScanRequest) Actor() AuditActor      { return request.params.Actor }

type ScanQuery struct {
	scope        Scope
	repositoryID RepositoryID
	scanID       ScanID
}

func (query ScanQuery) Scope() Scope               { return query.scope }
func (query ScanQuery) RepositoryID() RepositoryID { return query.repositoryID }
func (query ScanQuery) ScanID() ScanID             { return query.scanID }

type ScanListRequest struct {
	scope        Scope
	repositoryID RepositoryID
	pageSize     int
	cursor       Cursor
}

func (request ScanListRequest) Scope() Scope               { return request.scope }
func (request ScanListRequest) RepositoryID() RepositoryID { return request.repositoryID }
func (request ScanListRequest) PageSize() int              { return request.pageSize }
func (request ScanListRequest) Cursor() Cursor             { return request.cursor }

type FinishScanParams struct {
	Scope        Scope
	RequestID    RequestID
	RepositoryID RepositoryID
	ScanID       ScanID
	ReasonCode   string
	SafeMessage  string
	Actor        AuditActor
}

type FinishScanRequest struct{ params FinishScanParams }

func (request FinishScanRequest) Scope() Scope               { return request.params.Scope }
func (request FinishScanRequest) RequestID() RequestID       { return request.params.RequestID }
func (request FinishScanRequest) RepositoryID() RepositoryID { return request.params.RepositoryID }
func (request FinishScanRequest) ScanID() ScanID             { return request.params.ScanID }
func (request FinishScanRequest) ReasonCode() string         { return request.params.ReasonCode }
func (request FinishScanRequest) SafeMessage() string        { return request.params.SafeMessage }
func (request FinishScanRequest) Actor() AuditActor          { return request.params.Actor }

type StagePayloadParams struct {
	Scope        Scope
	RequestID    RequestID
	RepositoryID RepositoryID
	ScanID       ScanID
	Digest       Digest
	ExpectedSize ByteCount
}

type StagePayloadRequest struct{ params StagePayloadParams }

func (request StagePayloadRequest) Scope() Scope               { return request.params.Scope }
func (request StagePayloadRequest) RequestID() RequestID       { return request.params.RequestID }
func (request StagePayloadRequest) RepositoryID() RepositoryID { return request.params.RepositoryID }
func (request StagePayloadRequest) ScanID() ScanID             { return request.params.ScanID }
func (request StagePayloadRequest) Digest() Digest             { return request.params.Digest }
func (request StagePayloadRequest) ExpectedSize() ByteCount    { return request.params.ExpectedSize }

type ArtifactSubmissionParams struct {
	ArtifactID     ArtifactID
	Artifact       VersionedName
	StableIDScheme string
	Codec          Codec
	PayloadDigest  Digest
	PayloadSize    ByteCount
	Producer       VersionedName
}

type ArtifactSubmission struct{ params ArtifactSubmissionParams }

func (submission ArtifactSubmission) ArtifactID() ArtifactID  { return submission.params.ArtifactID }
func (submission ArtifactSubmission) Artifact() VersionedName { return submission.params.Artifact }
func (submission ArtifactSubmission) StableIDScheme() string {
	return submission.params.StableIDScheme
}
func (submission ArtifactSubmission) Codec() Codec            { return submission.params.Codec }
func (submission ArtifactSubmission) PayloadDigest() Digest   { return submission.params.PayloadDigest }
func (submission ArtifactSubmission) PayloadSize() ByteCount  { return submission.params.PayloadSize }
func (submission ArtifactSubmission) Producer() VersionedName { return submission.params.Producer }

type DependencySubmission struct {
	consumerArtifactID ArtifactID
	ordinal            uint32
	sourceArtifactID   ArtifactID
	declaredArtifact   VersionedName
}

func (submission DependencySubmission) ConsumerArtifactID() ArtifactID {
	return submission.consumerArtifactID
}
func (submission DependencySubmission) Ordinal() uint32 { return submission.ordinal }
func (submission DependencySubmission) SourceArtifactID() ArtifactID {
	return submission.sourceArtifactID
}
func (submission DependencySubmission) DeclaredArtifact() VersionedName {
	return submission.declaredArtifact
}

type ProjectionSubmissionParams struct {
	ProjectionID     ProjectionID
	ArtifactID       ArtifactID
	SourceDigest     Digest
	Projector        VersionedName
	SchemaVersion    string
	DigestScheme     string
	ProjectionDigest Digest
	CanonicalJSON    []byte
	RecordCount      uint64
}

type ProjectionSubmission struct{ params ProjectionSubmissionParams }

func (submission ProjectionSubmission) ProjectionID() ProjectionID {
	return submission.params.ProjectionID
}
func (submission ProjectionSubmission) ArtifactID() ArtifactID   { return submission.params.ArtifactID }
func (submission ProjectionSubmission) SourceDigest() Digest     { return submission.params.SourceDigest }
func (submission ProjectionSubmission) Projector() VersionedName { return submission.params.Projector }
func (submission ProjectionSubmission) SchemaVersion() string    { return submission.params.SchemaVersion }
func (submission ProjectionSubmission) DigestScheme() string     { return submission.params.DigestScheme }
func (submission ProjectionSubmission) ProjectionDigest() Digest {
	return submission.params.ProjectionDigest
}
func (submission ProjectionSubmission) CanonicalJSON() []byte {
	return append([]byte(nil), submission.params.CanonicalJSON...)
}
func (submission ProjectionSubmission) RecordCount() uint64 { return submission.params.RecordCount }

type DiagnosticSubmission struct {
	projectionID ProjectionID
	ordinal      uint32
	severity     string
	code         string
	engine       string
	relativePath string
	line         uint32
	column       uint32
	message      string
}

func (submission DiagnosticSubmission) ProjectionID() ProjectionID { return submission.projectionID }
func (submission DiagnosticSubmission) Ordinal() uint32            { return submission.ordinal }
func (submission DiagnosticSubmission) Severity() string           { return submission.severity }
func (submission DiagnosticSubmission) Code() string               { return submission.code }
func (submission DiagnosticSubmission) Engine() string             { return submission.engine }
func (submission DiagnosticSubmission) RelativePath() string       { return submission.relativePath }
func (submission DiagnosticSubmission) Line() uint32               { return submission.line }
func (submission DiagnosticSubmission) Column() uint32             { return submission.column }
func (submission DiagnosticSubmission) Message() string            { return submission.message }

type StatisticKind string

const (
	StatisticInteger StatisticKind = "integer"
	StatisticDecimal StatisticKind = "decimal"
	StatisticBoolean StatisticKind = "boolean"
	StatisticText    StatisticKind = "text"
)

type StatisticValue struct {
	kind    StatisticKind
	integer int64
	decimal string
	boolean bool
	text    string
}

func (value StatisticValue) Kind() StatisticKind { return value.kind }
func (value StatisticValue) Integer() int64      { return value.integer }
func (value StatisticValue) Decimal() string     { return value.decimal }
func (value StatisticValue) Boolean() bool       { return value.boolean }
func (value StatisticValue) Text() string        { return value.text }

type StatisticSubmission struct {
	projectionID ProjectionID
	key          string
	value        StatisticValue
	unit         string
}

func (submission StatisticSubmission) ProjectionID() ProjectionID { return submission.projectionID }
func (submission StatisticSubmission) Key() string                { return submission.key }
func (submission StatisticSubmission) Value() StatisticValue      { return submission.value }
func (submission StatisticSubmission) Unit() string               { return submission.unit }

type PublishScanParams struct {
	Scope          Scope
	RequestID      RequestID
	RepositoryID   RepositoryID
	ScanID         ScanID
	ManifestScheme string
	ManifestDigest Digest
	Artifacts      []ArtifactSubmission
	Dependencies   []DependencySubmission
	Projections    []ProjectionSubmission
	Diagnostics    []DiagnosticSubmission
	Statistics     []StatisticSubmission
	MakeCurrent    bool
	Actor          AuditActor
}

type PublishScanRequest struct{ params PublishScanParams }

func (request PublishScanRequest) Scope() Scope               { return request.params.Scope }
func (request PublishScanRequest) RequestID() RequestID       { return request.params.RequestID }
func (request PublishScanRequest) RepositoryID() RepositoryID { return request.params.RepositoryID }
func (request PublishScanRequest) ScanID() ScanID             { return request.params.ScanID }
func (request PublishScanRequest) ManifestScheme() string     { return request.params.ManifestScheme }
func (request PublishScanRequest) ManifestDigest() Digest     { return request.params.ManifestDigest }
func (request PublishScanRequest) Artifacts() []ArtifactSubmission {
	return cloneArtifacts(request.params.Artifacts)
}
func (request PublishScanRequest) Dependencies() []DependencySubmission {
	return append([]DependencySubmission(nil), request.params.Dependencies...)
}
func (request PublishScanRequest) Projections() []ProjectionSubmission {
	return cloneProjections(request.params.Projections)
}
func (request PublishScanRequest) Diagnostics() []DiagnosticSubmission {
	return append([]DiagnosticSubmission(nil), request.params.Diagnostics...)
}
func (request PublishScanRequest) Statistics() []StatisticSubmission {
	return append([]StatisticSubmission(nil), request.params.Statistics...)
}
func (request PublishScanRequest) MakeCurrent() bool { return request.params.MakeCurrent }
func (request PublishScanRequest) Actor() AuditActor { return request.params.Actor }

type ArtifactQuery struct {
	scope        Scope
	repositoryID RepositoryID
	scanID       ScanID
	artifactID   ArtifactID
}

func (query ArtifactQuery) Scope() Scope               { return query.scope }
func (query ArtifactQuery) RepositoryID() RepositoryID { return query.repositoryID }
func (query ArtifactQuery) ScanID() ScanID             { return query.scanID }
func (query ArtifactQuery) ArtifactID() ArtifactID     { return query.artifactID }

type ArtifactListRequest struct {
	scope        Scope
	repositoryID RepositoryID
	scanID       ScanID
	pageSize     int
	cursor       Cursor
}

func (request ArtifactListRequest) Scope() Scope               { return request.scope }
func (request ArtifactListRequest) RepositoryID() RepositoryID { return request.repositoryID }
func (request ArtifactListRequest) ScanID() ScanID             { return request.scanID }
func (request ArtifactListRequest) PageSize() int              { return request.pageSize }
func (request ArtifactListRequest) Cursor() Cursor             { return request.cursor }

type PayloadQuery struct {
	scope        Scope
	repositoryID RepositoryID
	scanID       ScanID
	artifactID   ArtifactID
	digest       Digest
}

func (query PayloadQuery) Scope() Scope               { return query.scope }
func (query PayloadQuery) RepositoryID() RepositoryID { return query.repositoryID }
func (query PayloadQuery) ScanID() ScanID             { return query.scanID }
func (query PayloadQuery) ArtifactID() ArtifactID     { return query.artifactID }
func (query PayloadQuery) Digest() Digest             { return query.digest }

type MarkForPurgeRequest struct {
	scope        Scope
	requestID    RequestID
	repositoryID RepositoryID
	actor        AuditActor
}

func (request MarkForPurgeRequest) Scope() Scope               { return request.scope }
func (request MarkForPurgeRequest) RequestID() RequestID       { return request.requestID }
func (request MarkForPurgeRequest) RepositoryID() RepositoryID { return request.repositoryID }
func (request MarkForPurgeRequest) Actor() AuditActor          { return request.actor }

type PurgeBatchRequest struct {
	scope        Scope
	requestID    RequestID
	repositoryID RepositoryID
	limit        int
	actor        AuditActor
}

func (request PurgeBatchRequest) Scope() Scope               { return request.scope }
func (request PurgeBatchRequest) RequestID() RequestID       { return request.requestID }
func (request PurgeBatchRequest) RepositoryID() RepositoryID { return request.repositoryID }
func (request PurgeBatchRequest) Limit() int                 { return request.limit }
func (request PurgeBatchRequest) Actor() AuditActor          { return request.actor }

type GarbageCollectionRequest struct {
	scope     Scope
	requestID RequestID
	limit     int
	actor     AuditActor
}

func (request GarbageCollectionRequest) Scope() Scope         { return request.scope }
func (request GarbageCollectionRequest) RequestID() RequestID { return request.requestID }
func (request GarbageCollectionRequest) Limit() int           { return request.limit }
func (request GarbageCollectionRequest) Actor() AuditActor    { return request.actor }

// RepositoryRecord is a detached durable lifecycle view.
type RepositoryRecord struct {
	scopeID       string
	repositoryID  RepositoryID
	displayName   string
	source        SourceIdentity
	state         RepositoryState
	currentScanID ScanID
	createdAt     time.Time
	updatedAt     time.Time
}

func (record RepositoryRecord) ScopeID() string            { return record.scopeID }
func (record RepositoryRecord) RepositoryID() RepositoryID { return record.repositoryID }
func (record RepositoryRecord) DisplayName() string        { return record.displayName }
func (record RepositoryRecord) Source() SourceIdentity     { return record.source }
func (record RepositoryRecord) State() RepositoryState     { return record.state }
func (record RepositoryRecord) CurrentScanID() ScanID      { return record.currentScanID }
func (record RepositoryRecord) CreatedAt() time.Time       { return record.createdAt }
func (record RepositoryRecord) UpdatedAt() time.Time       { return record.updatedAt }

type ScanRecord struct {
	scopeID               string
	repositoryID          RepositoryID
	scanID                ScanID
	analysisProfileDigest Digest
	sourceRevision        string
	state                 ScanState
	reasonCode            string
	safeMessage           string
	requestedAt           time.Time
	startedAt             time.Time
	finishedAt            time.Time
}

func (record ScanRecord) ScopeID() string               { return record.scopeID }
func (record ScanRecord) RepositoryID() RepositoryID    { return record.repositoryID }
func (record ScanRecord) ScanID() ScanID                { return record.scanID }
func (record ScanRecord) AnalysisProfileDigest() Digest { return record.analysisProfileDigest }
func (record ScanRecord) SourceRevision() string        { return record.sourceRevision }
func (record ScanRecord) State() ScanState              { return record.state }
func (record ScanRecord) ReasonCode() string            { return record.reasonCode }
func (record ScanRecord) SafeMessage() string           { return record.safeMessage }
func (record ScanRecord) RequestedAt() time.Time        { return record.requestedAt }
func (record ScanRecord) StartedAt() time.Time          { return record.startedAt }
func (record ScanRecord) FinishedAt() time.Time         { return record.finishedAt }

type ArtifactRecord struct {
	scopeID        string
	repositoryID   RepositoryID
	scanID         ScanID
	artifactID     ArtifactID
	artifact       VersionedName
	stableIDScheme string
	codec          Codec
	producer       VersionedName
	payloadDigest  Digest
	payloadSize    ByteCount
	createdAt      time.Time
}

func (record ArtifactRecord) ScopeID() string            { return record.scopeID }
func (record ArtifactRecord) RepositoryID() RepositoryID { return record.repositoryID }
func (record ArtifactRecord) ScanID() ScanID             { return record.scanID }
func (record ArtifactRecord) ArtifactID() ArtifactID     { return record.artifactID }
func (record ArtifactRecord) Artifact() VersionedName    { return record.artifact }
func (record ArtifactRecord) StableIDScheme() string     { return record.stableIDScheme }
func (record ArtifactRecord) Codec() Codec               { return record.codec }
func (record ArtifactRecord) Producer() VersionedName    { return record.producer }
func (record ArtifactRecord) PayloadDigest() Digest      { return record.payloadDigest }
func (record ArtifactRecord) PayloadSize() ByteCount     { return record.payloadSize }
func (record ArtifactRecord) CreatedAt() time.Time       { return record.createdAt }

type RepositoryPage struct {
	records []RepositoryRecord
	next    Cursor
}

func (page RepositoryPage) Records() []RepositoryRecord {
	return append([]RepositoryRecord(nil), page.records...)
}
func (page RepositoryPage) NextCursor() Cursor { return page.next }

type ScanPage struct {
	records []ScanRecord
	next    Cursor
}

func (page ScanPage) Records() []ScanRecord { return append([]ScanRecord(nil), page.records...) }
func (page ScanPage) NextCursor() Cursor    { return page.next }

type ArtifactPage struct {
	records []ArtifactRecord
	next    Cursor
}

func (page ArtifactPage) Records() []ArtifactRecord {
	return append([]ArtifactRecord(nil), page.records...)
}
func (page ArtifactPage) NextCursor() Cursor { return page.next }

type PayloadReceipt struct {
	digest      Digest
	size        ByteCount
	disposition Disposition
}

func (receipt PayloadReceipt) Digest() Digest           { return receipt.digest }
func (receipt PayloadReceipt) Size() ByteCount          { return receipt.size }
func (receipt PayloadReceipt) Disposition() Disposition { return receipt.disposition }

type PublicationReceipt struct {
	scanID         ScanID
	manifestScheme string
	manifestDigest Digest
	artifactCount  uint32
	disposition    Disposition
}

func (receipt PublicationReceipt) ScanID() ScanID           { return receipt.scanID }
func (receipt PublicationReceipt) ManifestScheme() string   { return receipt.manifestScheme }
func (receipt PublicationReceipt) ManifestDigest() Digest   { return receipt.manifestDigest }
func (receipt PublicationReceipt) ArtifactCount() uint32    { return receipt.artifactCount }
func (receipt PublicationReceipt) Disposition() Disposition { return receipt.disposition }

type VerificationReceipt struct {
	digest Digest
	size   ByteCount
}

func (receipt VerificationReceipt) Digest() Digest  { return receipt.digest }
func (receipt VerificationReceipt) Size() ByteCount { return receipt.size }

type PurgeReceipt struct {
	removedArtifacts uint64
	removedScans     uint64
	complete         bool
}

func (receipt PurgeReceipt) RemovedArtifacts() uint64 { return receipt.removedArtifacts }
func (receipt PurgeReceipt) RemovedScans() uint64     { return receipt.removedScans }
func (receipt PurgeReceipt) Complete() bool           { return receipt.complete }

type GarbageCollectionReceipt struct {
	removedPayloads uint64
	removedBytes    ByteCount
}

func (receipt GarbageCollectionReceipt) RemovedPayloads() uint64 { return receipt.removedPayloads }
func (receipt GarbageCollectionReceipt) RemovedBytes() ByteCount { return receipt.removedBytes }

func cloneArtifacts(values []ArtifactSubmission) []ArtifactSubmission {
	return append([]ArtifactSubmission(nil), values...)
}

func cloneProjections(values []ProjectionSubmission) []ProjectionSubmission {
	result := append([]ProjectionSubmission(nil), values...)
	for index := range result {
		result[index].params.CanonicalJSON = append([]byte(nil), result[index].params.CanonicalJSON...)
	}
	return result
}
