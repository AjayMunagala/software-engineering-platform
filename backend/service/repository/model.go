package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type ScopeID string
type PrincipalID string
type RequestID string
type RepositoryID string
type ScanID string
type ArtifactID string
type Cursor string

// Digest is one exact SHA-256 value.
type Digest [sha256.Size]byte

func ParseDigest(value string) (Digest, error) {
	var digest Digest
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return digest, NewError(ErrorInvalidInput, "parse-digest", "invalid-sha256", false, err)
	}
	copy(digest[:], decoded)
	return digest, nil
}
func DigestBytes(value []byte) Digest { return sha256.Sum256(value) }
func (digest Digest) String() string  { return hex.EncodeToString(digest[:]) }
func (digest Digest) IsZero() bool    { return digest == Digest{} }

// Scope is an already-authorized namespace and principal.
type Scope struct {
	scopeID     ScopeID
	principalID PrincipalID
}

func (scope Scope) ScopeID() ScopeID         { return scope.scopeID }
func (scope Scope) PrincipalID() PrincipalID { return scope.principalID }
func (scope Scope) IsZero() bool             { return scope.scopeID == "" || scope.principalID == "" }

// SourceHandle is sensitive process-local routing data. Its formatting methods
// are always redacted; Reveal is reserved for the authorized resolver adapter.
type SourceHandle struct{ value string }

func (handle SourceHandle) Reveal() string { return handle.value }
func (handle SourceHandle) IsZero() bool   { return handle.value == "" }
func (SourceHandle) String() string        { return "<redacted-source-handle>" }
func (SourceHandle) GoString() string      { return "<redacted-source-handle>" }

// AnalysisProfile identifies one immutable orchestration profile.
type AnalysisProfile struct {
	name, version string
	digest        Digest
}

func (profile AnalysisProfile) Name() string    { return profile.name }
func (profile AnalysisProfile) Version() string { return profile.version }
func (profile AnalysisProfile) Digest() Digest  { return profile.digest }
func (profile AnalysisProfile) IsZero() bool {
	return profile.name == "" || profile.version == "" || profile.digest.IsZero()
}

type RepositoryState string

const (
	RepositoryActive       RepositoryState = "active"
	RepositoryArchived     RepositoryState = "archived"
	RepositoryPurgePending RepositoryState = "purge_pending"
)

type ScanState string

const (
	ScanRequested ScanState = "requested"
	ScanRunning   ScanState = "running"
	ScanSucceeded ScanState = "succeeded"
	ScanFailed    ScanState = "failed"
	ScanCanceled  ScanState = "canceled"
)

type Disposition string

const (
	DispositionCreated        Disposition = "created"
	DispositionAlreadyPresent Disposition = "already_present"
	DispositionJoined         Disposition = "joined"
)

type RegisterRepositoryParams struct {
	Scope        Scope
	RequestID    RequestID
	RepositoryID RepositoryID
	DisplayName  string
	SourceHandle string
}
type RegisterRepositoryRequest struct {
	scope        Scope
	requestID    RequestID
	repositoryID RepositoryID
	displayName  string
	source       SourceHandle
}

func (request RegisterRepositoryRequest) Scope() Scope               { return request.scope }
func (request RegisterRepositoryRequest) RequestID() RequestID       { return request.requestID }
func (request RegisterRepositoryRequest) RepositoryID() RepositoryID { return request.repositoryID }
func (request RegisterRepositoryRequest) DisplayName() string        { return request.displayName }
func (request RegisterRepositoryRequest) SourceHandle() SourceHandle { return request.source }
func (request RegisterRepositoryRequest) String() string {
	return "register-repository-request{" + string(request.repositoryID) + ",source=<redacted>}"
}
func (request RegisterRepositoryRequest) GoString() string { return request.String() }

type RepositoryQuery struct {
	scope        Scope
	repositoryID RepositoryID
}

func (query RepositoryQuery) Scope() Scope               { return query.scope }
func (query RepositoryQuery) RepositoryID() RepositoryID { return query.repositoryID }

type RepositoryListParams struct {
	Scope    Scope
	PageSize int
	Cursor   Cursor
}
type RepositoryListRequest struct {
	scope    Scope
	pageSize int
	cursor   Cursor
}

func (request RepositoryListRequest) Scope() Scope   { return request.scope }
func (request RepositoryListRequest) PageSize() int  { return request.pageSize }
func (request RepositoryListRequest) Cursor() Cursor { return request.cursor }

type ArchiveRepositoryParams struct {
	Scope        Scope
	RequestID    RequestID
	RepositoryID RepositoryID
}
type ArchiveRepositoryRequest struct {
	scope        Scope
	requestID    RequestID
	repositoryID RepositoryID
}

func (request ArchiveRepositoryRequest) Scope() Scope               { return request.scope }
func (request ArchiveRepositoryRequest) RequestID() RequestID       { return request.requestID }
func (request ArchiveRepositoryRequest) RepositoryID() RepositoryID { return request.repositoryID }

type ExecuteScanParams struct {
	Scope        Scope
	RequestID    RequestID
	RepositoryID RepositoryID
	ScanID       ScanID
	SourceHandle string
	Profile      AnalysisProfile
}
type ExecuteScanRequest struct {
	scope        Scope
	requestID    RequestID
	repositoryID RepositoryID
	scanID       ScanID
	source       SourceHandle
	profile      AnalysisProfile
}

func (request ExecuteScanRequest) Scope() Scope               { return request.scope }
func (request ExecuteScanRequest) RequestID() RequestID       { return request.requestID }
func (request ExecuteScanRequest) RepositoryID() RepositoryID { return request.repositoryID }
func (request ExecuteScanRequest) ScanID() ScanID             { return request.scanID }
func (request ExecuteScanRequest) SourceHandle() SourceHandle { return request.source }
func (request ExecuteScanRequest) Profile() AnalysisProfile   { return request.profile }
func (request ExecuteScanRequest) String() string {
	return "execute-scan-request{" + string(request.repositoryID) + "," + string(request.scanID) + ",source=<redacted>}"
}
func (request ExecuteScanRequest) GoString() string { return request.String() }

type ScanQuery struct {
	scope        Scope
	repositoryID RepositoryID
	scanID       ScanID
}

func (query ScanQuery) Scope() Scope               { return query.scope }
func (query ScanQuery) RepositoryID() RepositoryID { return query.repositoryID }
func (query ScanQuery) ScanID() ScanID             { return query.scanID }

type ScanListParams struct {
	Scope        Scope
	RepositoryID RepositoryID
	PageSize     int
	Cursor       Cursor
}
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

type CancelScanParams struct {
	Scope        Scope
	RequestID    RequestID
	RepositoryID RepositoryID
	ScanID       ScanID
}
type CancelScanRequest struct {
	scope        Scope
	requestID    RequestID
	repositoryID RepositoryID
	scanID       ScanID
}

func (request CancelScanRequest) Scope() Scope               { return request.scope }
func (request CancelScanRequest) RequestID() RequestID       { return request.requestID }
func (request CancelScanRequest) RepositoryID() RepositoryID { return request.repositoryID }
func (request CancelScanRequest) ScanID() ScanID             { return request.scanID }

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

type ArtifactListParams struct {
	Scope        Scope
	RepositoryID RepositoryID
	ScanID       ScanID
	PageSize     int
	Cursor       Cursor
}
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

type ExportArtifactRequest struct{ query ArtifactQuery }

func (request ExportArtifactRequest) Query() ArtifactQuery { return request.query }

type RepositoryParams struct {
	RepositoryID      RepositoryID
	DisplayName       string
	SourceKind        string
	FingerprintScheme string
	Fingerprint       Digest
	State             RepositoryState
	CurrentScanID     ScanID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
type Repository struct{ params RepositoryParams }

func (value Repository) RepositoryID() RepositoryID { return value.params.RepositoryID }
func (value Repository) DisplayName() string        { return value.params.DisplayName }
func (value Repository) SourceKind() string         { return value.params.SourceKind }
func (value Repository) FingerprintScheme() string  { return value.params.FingerprintScheme }
func (value Repository) Fingerprint() Digest        { return value.params.Fingerprint }
func (value Repository) State() RepositoryState     { return value.params.State }
func (value Repository) CurrentScanID() ScanID      { return value.params.CurrentScanID }
func (value Repository) CreatedAt() time.Time       { return value.params.CreatedAt }
func (value Repository) UpdatedAt() time.Time       { return value.params.UpdatedAt }

type ScanParams struct {
	RepositoryID   RepositoryID
	ScanID         ScanID
	Profile        AnalysisProfile
	SourceRevision string
	State          ScanState
	ReasonCode     string
	RequestedAt    time.Time
	StartedAt      time.Time
	FinishedAt     time.Time
}
type Scan struct{ params ScanParams }

func (value Scan) RepositoryID() RepositoryID { return value.params.RepositoryID }
func (value Scan) ScanID() ScanID             { return value.params.ScanID }
func (value Scan) Profile() AnalysisProfile   { return value.params.Profile }
func (value Scan) SourceRevision() string     { return value.params.SourceRevision }
func (value Scan) State() ScanState           { return value.params.State }
func (value Scan) ReasonCode() string         { return value.params.ReasonCode }
func (value Scan) RequestedAt() time.Time     { return value.params.RequestedAt }
func (value Scan) StartedAt() time.Time       { return value.params.StartedAt }
func (value Scan) FinishedAt() time.Time      { return value.params.FinishedAt }

type ArtifactParams struct {
	ArtifactID      ArtifactID
	ScanID          ScanID
	Name            string
	Version         string
	StableIDScheme  string
	CodecName       string
	CodecVersion    string
	MediaType       string
	PayloadDigest   Digest
	PayloadSize     uint64
	ProducerName    string
	ProducerVersion string
	CreatedAt       time.Time
}
type Artifact struct{ params ArtifactParams }

func (value Artifact) ArtifactID() ArtifactID  { return value.params.ArtifactID }
func (value Artifact) ScanID() ScanID          { return value.params.ScanID }
func (value Artifact) Name() string            { return value.params.Name }
func (value Artifact) Version() string         { return value.params.Version }
func (value Artifact) StableIDScheme() string  { return value.params.StableIDScheme }
func (value Artifact) CodecName() string       { return value.params.CodecName }
func (value Artifact) CodecVersion() string    { return value.params.CodecVersion }
func (value Artifact) MediaType() string       { return value.params.MediaType }
func (value Artifact) PayloadDigest() Digest   { return value.params.PayloadDigest }
func (value Artifact) PayloadSize() uint64     { return value.params.PayloadSize }
func (value Artifact) ProducerName() string    { return value.params.ProducerName }
func (value Artifact) ProducerVersion() string { return value.params.ProducerVersion }
func (value Artifact) CreatedAt() time.Time    { return value.params.CreatedAt }

type RepositoryPage struct {
	items []Repository
	next  Cursor
}

func (page RepositoryPage) Items() []Repository { return append([]Repository(nil), page.items...) }
func (page RepositoryPage) NextCursor() Cursor  { return page.next }

type ScanPage struct {
	items []Scan
	next  Cursor
}

func (page ScanPage) Items() []Scan      { return append([]Scan(nil), page.items...) }
func (page ScanPage) NextCursor() Cursor { return page.next }

type ArtifactPage struct {
	items []Artifact
	next  Cursor
}

func (page ArtifactPage) Items() []Artifact  { return append([]Artifact(nil), page.items...) }
func (page ArtifactPage) NextCursor() Cursor { return page.next }

type ScanResult struct {
	scan        Scan
	artifacts   []Artifact
	disposition Disposition
}

func (result ScanResult) Scan() Scan               { return result.scan }
func (result ScanResult) Artifacts() []Artifact    { return append([]Artifact(nil), result.artifacts...) }
func (result ScanResult) Disposition() Disposition { return result.disposition }

type ExportReceipt struct {
	digest Digest
	size   uint64
}

func (receipt ExportReceipt) PayloadDigest() Digest { return receipt.digest }
func (receipt ExportReceipt) PayloadSize() uint64   { return receipt.size }

// ProfileArtifact identifies one ordered required output contract.
type ProfileArtifact struct{ name, version, stableIDScheme string }

func (artifact ProfileArtifact) Name() string           { return artifact.name }
func (artifact ProfileArtifact) Version() string        { return artifact.version }
func (artifact ProfileArtifact) StableIDScheme() string { return artifact.stableIDScheme }

type ProfileDefinition struct {
	profile   AnalysisProfile
	artifacts []ProfileArtifact
}

func (definition ProfileDefinition) Profile() AnalysisProfile { return definition.profile }
func (definition ProfileDefinition) Artifacts() []ProfileArtifact {
	return append([]ProfileArtifact(nil), definition.artifacts...)
}
