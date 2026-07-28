package scan

import (
	"context"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

type BeginStatus string

const (
	BeginStarted            BeginStatus = "started"
	BeginAlreadyPublished   BeginStatus = "already_published"
	BeginPreviouslyFailed   BeginStatus = "previously_failed"
	BeginPreviouslyCanceled BeginStatus = "previously_canceled"
	BeginOrphaned           BeginStatus = "orphaned"
)

type AnalysisRequest struct {
	scope        repository.Scope
	repositoryID repository.RepositoryID
	scanID       repository.ScanID
	source       repository.SourceHandle
	profile      repository.AnalysisProfile
}

// NewAnalysisRequest creates the immutable adapter input from an already
// validated public execute request.
func NewAnalysisRequest(request repository.ExecuteScanRequest) AnalysisRequest {
	return AnalysisRequest{scope: request.Scope(), repositoryID: request.RepositoryID(), scanID: request.ScanID(), source: request.SourceHandle(), profile: request.Profile()}
}
func (request AnalysisRequest) Scope() repository.Scope               { return request.scope }
func (request AnalysisRequest) RepositoryID() repository.RepositoryID { return request.repositoryID }
func (request AnalysisRequest) ScanID() repository.ScanID             { return request.scanID }
func (request AnalysisRequest) SourceHandle() repository.SourceHandle { return request.source }
func (request AnalysisRequest) Profile() repository.AnalysisProfile   { return request.profile }

type ArtifactCandidateParams struct {
	Name, Version, StableIDScheme string
	CodecName, CodecVersion       string
	MediaType                     string
	PayloadDigest                 repository.Digest
	PayloadSize                   uint64
	ProducerName, ProducerVersion string
	Dependencies                  []ArtifactDependency
	Payload                       PayloadSource
}

// ArtifactDependency is one deterministic edge in the frozen analysis
// profile. Ordinals are local to the dependent artifact and begin at zero.
type ArtifactDependency struct {
	name, version string
	ordinal       int
}

func NewArtifactDependency(name, version string, ordinal int) (ArtifactDependency, error) {
	if !validName(name, 128) || !validVersion(version) || ordinal < 0 || ordinal >= defaultMaxArtifacts {
		return ArtifactDependency{}, repository.NewError(repository.ErrorInvalidInput, "new-artifact-dependency", "invalid-dependency", false, nil)
	}
	return ArtifactDependency{name: name, version: version, ordinal: ordinal}, nil
}

func (dependency ArtifactDependency) Name() string    { return dependency.name }
func (dependency ArtifactDependency) Version() string { return dependency.version }
func (dependency ArtifactDependency) Ordinal() int    { return dependency.ordinal }

// ArtifactCandidate is immutable analysis metadata plus a reopenable
// exact payload source. It contains no engine artifact or filesystem path.
type ArtifactCandidate struct{ params ArtifactCandidateParams }

func NewArtifactCandidate(params ArtifactCandidateParams) (ArtifactCandidate, error) {
	if !validName(params.Name, 128) || !validVersion(params.Version) || !validName(params.StableIDScheme, 128) || !validName(params.CodecName, 128) || !validVersion(params.CodecVersion) || (params.MediaType != "application/json" && params.MediaType != "application/octet-stream") || params.PayloadDigest.IsZero() || params.PayloadSize == 0 || !validName(params.ProducerName, 128) || !validVersion(params.ProducerVersion) || params.Payload == nil {
		return ArtifactCandidate{}, repository.NewError(repository.ErrorInvalidInput, "new-artifact-candidate", "invalid-candidate", false, nil)
	}
	params.Dependencies = append([]ArtifactDependency(nil), params.Dependencies...)
	seen := make(map[string]struct{}, len(params.Dependencies))
	for index, dependency := range params.Dependencies {
		key := dependency.Name() + "\x00" + dependency.Version()
		if dependency.Name() == "" || dependency.Ordinal() != index {
			return ArtifactCandidate{}, repository.NewError(repository.ErrorInvalidInput, "new-artifact-candidate", "invalid-dependency-order", false, nil)
		}
		if _, exists := seen[key]; exists {
			return ArtifactCandidate{}, repository.NewError(repository.ErrorConflict, "new-artifact-candidate", "duplicate-dependency", false, nil)
		}
		seen[key] = struct{}{}
	}
	return ArtifactCandidate{params: params}, nil
}
func (candidate ArtifactCandidate) Name() string           { return candidate.params.Name }
func (candidate ArtifactCandidate) Version() string        { return candidate.params.Version }
func (candidate ArtifactCandidate) StableIDScheme() string { return candidate.params.StableIDScheme }
func (candidate ArtifactCandidate) CodecName() string      { return candidate.params.CodecName }
func (candidate ArtifactCandidate) CodecVersion() string   { return candidate.params.CodecVersion }
func (candidate ArtifactCandidate) MediaType() string      { return candidate.params.MediaType }
func (candidate ArtifactCandidate) PayloadDigest() repository.Digest {
	return candidate.params.PayloadDigest
}
func (candidate ArtifactCandidate) PayloadSize() uint64     { return candidate.params.PayloadSize }
func (candidate ArtifactCandidate) ProducerName() string    { return candidate.params.ProducerName }
func (candidate ArtifactCandidate) ProducerVersion() string { return candidate.params.ProducerVersion }
func (candidate ArtifactCandidate) Dependencies() []ArtifactDependency {
	return append([]ArtifactDependency(nil), candidate.params.Dependencies...)
}
func (candidate ArtifactCandidate) Open(ctx context.Context) (io.ReadCloser, error) {
	return candidate.params.Payload.Open(ctx)
}

type AnalysisResult struct {
	profile    repository.AnalysisProfile
	candidates []ArtifactCandidate
}

func NewAnalysisResult(profile repository.AnalysisProfile, candidates []ArtifactCandidate) (AnalysisResult, error) {
	if profile.IsZero() || len(candidates) == 0 || len(candidates) > defaultMaxArtifacts {
		return AnalysisResult{}, repository.NewError(repository.ErrorInvalidInput, "new-analysis-result", "invalid-result", false, nil)
	}
	copyCandidates := append([]ArtifactCandidate(nil), candidates...)
	seen := make(map[string]struct{}, len(copyCandidates))
	for _, candidate := range copyCandidates {
		key := candidate.Name() + "\x00" + candidate.Version()
		if candidate.params.Payload == nil || candidate.PayloadDigest().IsZero() {
			return AnalysisResult{}, repository.NewError(repository.ErrorInvalidInput, "new-analysis-result", "invalid-candidate", false, nil)
		}
		if _, exists := seen[key]; exists {
			return AnalysisResult{}, repository.NewError(repository.ErrorConflict, "new-analysis-result", "duplicate-artifact", false, nil)
		}
		seen[key] = struct{}{}
	}
	for _, candidate := range copyCandidates {
		for _, dependency := range candidate.Dependencies() {
			if _, exists := seen[dependency.Name()+"\x00"+dependency.Version()]; !exists {
				return AnalysisResult{}, repository.NewError(repository.ErrorInvalidInput, "new-analysis-result", "missing-dependency", false, nil)
			}
			if dependency.Name() == candidate.Name() && dependency.Version() == candidate.Version() {
				return AnalysisResult{}, repository.NewError(repository.ErrorInvalidInput, "new-analysis-result", "self-dependency", false, nil)
			}
		}
	}
	if !acyclicCandidates(copyCandidates) {
		return AnalysisResult{}, repository.NewError(repository.ErrorInvalidInput, "new-analysis-result", "cyclic-dependency", false, nil)
	}
	sort.Slice(copyCandidates, func(i, j int) bool {
		left := copyCandidates[i].Name() + "\x00" + copyCandidates[i].Version() + "\x00" + copyCandidates[i].StableIDScheme()
		right := copyCandidates[j].Name() + "\x00" + copyCandidates[j].Version() + "\x00" + copyCandidates[j].StableIDScheme()
		return left < right
	})
	return AnalysisResult{profile: profile, candidates: copyCandidates}, nil
}

func acyclicCandidates(candidates []ArtifactCandidate) bool {
	edges := make(map[string][]string, len(candidates))
	for _, candidate := range candidates {
		key := candidate.Name() + "\x00" + candidate.Version()
		for _, dependency := range candidate.Dependencies() {
			edges[key] = append(edges[key], dependency.Name()+"\x00"+dependency.Version())
		}
	}
	states := make(map[string]uint8, len(edges))
	var visit func(string) bool
	visit = func(key string) bool {
		if states[key] == 1 {
			return false
		}
		if states[key] == 2 {
			return true
		}
		states[key] = 1
		for _, dependency := range edges[key] {
			if !visit(dependency) {
				return false
			}
		}
		states[key] = 2
		return true
	}
	for key := range edges {
		if !visit(key) {
			return false
		}
	}
	return true
}
func (result AnalysisResult) Profile() repository.AnalysisProfile { return result.profile }
func (result AnalysisResult) Candidates() []ArtifactCandidate {
	return append([]ArtifactCandidate(nil), result.candidates...)
}

type BeginCommand struct {
	scope               repository.Scope
	requestID           repository.RequestID
	mutationFingerprint repository.Digest
	sourceFingerprint   repository.Digest
	scan                repository.Scan
}

func newBeginCommand(scope repository.Scope, requestID repository.RequestID, mutationFingerprint, sourceFingerprint repository.Digest, scan repository.Scan) BeginCommand {
	return BeginCommand{scope: scope, requestID: requestID, mutationFingerprint: mutationFingerprint, sourceFingerprint: sourceFingerprint, scan: scan}
}
func (command BeginCommand) Scope() repository.Scope         { return command.scope }
func (command BeginCommand) RequestID() repository.RequestID { return command.requestID }
func (command BeginCommand) MutationFingerprint() repository.Digest {
	return command.mutationFingerprint
}
func (command BeginCommand) SourceFingerprint() repository.Digest { return command.sourceFingerprint }
func (command BeginCommand) Scan() repository.Scan                { return command.scan }

type BeginResult struct {
	status    BeginStatus
	scan      repository.Scan
	artifacts []repository.Artifact
}

func NewBeginResult(status BeginStatus, scan repository.Scan, artifacts []repository.Artifact) (BeginResult, error) {
	if scan.ScanID() == "" || (status != BeginStarted && status != BeginAlreadyPublished && status != BeginPreviouslyFailed && status != BeginPreviouslyCanceled && status != BeginOrphaned) {
		return BeginResult{}, repository.NewError(repository.ErrorInvalidInput, "new-begin-result", "invalid-result", false, nil)
	}
	copyArtifacts := append([]repository.Artifact(nil), artifacts...)
	expectedState := map[BeginStatus]repository.ScanState{
		BeginStarted: repository.ScanRunning, BeginAlreadyPublished: repository.ScanSucceeded,
		BeginPreviouslyFailed: repository.ScanFailed, BeginPreviouslyCanceled: repository.ScanCanceled,
		BeginOrphaned: repository.ScanRunning,
	}[status]
	if scan.State() != expectedState {
		return BeginResult{}, repository.NewError(repository.ErrorInvalidInput, "new-begin-result", "invalid-state", false, nil)
	}
	if status != BeginAlreadyPublished && len(copyArtifacts) != 0 {
		return BeginResult{}, repository.NewError(repository.ErrorInvalidInput, "new-begin-result", "unexpected-artifacts", false, nil)
	}
	for _, artifact := range copyArtifacts {
		if artifact.ScanID() != scan.ScanID() {
			return BeginResult{}, repository.NewError(repository.ErrorInvalidInput, "new-begin-result", "artifact-scan-mismatch", false, nil)
		}
	}
	return BeginResult{status: status, scan: scan, artifacts: copyArtifacts}, nil
}
func (result BeginResult) Status() BeginStatus   { return result.status }
func (result BeginResult) Scan() repository.Scan { return result.scan }
func (result BeginResult) Artifacts() []repository.Artifact {
	return append([]repository.Artifact(nil), result.artifacts...)
}

type PublicationArtifact struct {
	metadata repository.Artifact
	payload  ArtifactCandidate
}

func newPublicationArtifact(metadata repository.Artifact, payload ArtifactCandidate) PublicationArtifact {
	return PublicationArtifact{metadata: metadata, payload: payload}
}
func (artifact PublicationArtifact) Metadata() repository.Artifact { return artifact.metadata }
func (artifact PublicationArtifact) Dependencies() []ArtifactDependency {
	return artifact.payload.Dependencies()
}
func (artifact PublicationArtifact) Open(ctx context.Context) (io.ReadCloser, error) {
	return artifact.payload.Open(ctx)
}

type PublishCommand struct {
	scope     repository.Scope
	requestID repository.RequestID
	scan      repository.Scan
	artifacts []PublicationArtifact
}

func newPublishCommand(scope repository.Scope, requestID repository.RequestID, scan repository.Scan, artifacts []PublicationArtifact) PublishCommand {
	return PublishCommand{scope: scope, requestID: requestID, scan: scan, artifacts: append([]PublicationArtifact(nil), artifacts...)}
}
func (command PublishCommand) Scope() repository.Scope         { return command.scope }
func (command PublishCommand) RequestID() repository.RequestID { return command.requestID }
func (command PublishCommand) Scan() repository.Scan           { return command.scan }
func (command PublishCommand) Artifacts() []PublicationArtifact {
	return append([]PublicationArtifact(nil), command.artifacts...)
}

type FinalizeCommand struct {
	scope     repository.Scope
	requestID repository.RequestID
	scan      repository.Scan
}

func newFinalizeCommand(scope repository.Scope, requestID repository.RequestID, scan repository.Scan) FinalizeCommand {
	return FinalizeCommand{scope: scope, requestID: requestID, scan: scan}
}
func (command FinalizeCommand) Scope() repository.Scope         { return command.scope }
func (command FinalizeCommand) RequestID() repository.RequestID { return command.requestID }
func (command FinalizeCommand) Scan() repository.Scan           { return command.scan }

type CancelCommand struct {
	scope        repository.Scope
	requestID    repository.RequestID
	fingerprint  repository.Digest
	repositoryID repository.RepositoryID
	scanID       repository.ScanID
	at           time.Time
}

func newCancelCommand(scope repository.Scope, requestID repository.RequestID, fingerprint repository.Digest, repositoryID repository.RepositoryID, scanID repository.ScanID, at time.Time) CancelCommand {
	return CancelCommand{scope: scope, requestID: requestID, fingerprint: fingerprint, repositoryID: repositoryID, scanID: scanID, at: at}
}
func (command CancelCommand) Scope() repository.Scope                { return command.scope }
func (command CancelCommand) RequestID() repository.RequestID        { return command.requestID }
func (command CancelCommand) MutationFingerprint() repository.Digest { return command.fingerprint }
func (command CancelCommand) RepositoryID() repository.RepositoryID  { return command.repositoryID }
func (command CancelCommand) ScanID() repository.ScanID              { return command.scanID }
func (command CancelCommand) At() time.Time                          { return command.at }

type ReconcileResult struct {
	scan      repository.Scan
	artifacts []repository.Artifact
}

func NewReconcileResult(scan repository.Scan, artifacts []repository.Artifact) (ReconcileResult, error) {
	if scan.ScanID() == "" {
		return ReconcileResult{}, repository.NewError(repository.ErrorInvalidInput, "new-reconcile-result", "invalid-result", false, nil)
	}
	copyArtifacts := append([]repository.Artifact(nil), artifacts...)
	if scan.State() != repository.ScanSucceeded && len(copyArtifacts) != 0 {
		return ReconcileResult{}, repository.NewError(repository.ErrorInvalidInput, "new-reconcile-result", "unexpected-artifacts", false, nil)
	}
	for _, artifact := range copyArtifacts {
		if artifact.ScanID() != scan.ScanID() {
			return ReconcileResult{}, repository.NewError(repository.ErrorInvalidInput, "new-reconcile-result", "artifact-scan-mismatch", false, nil)
		}
	}
	return ReconcileResult{scan: scan, artifacts: copyArtifacts}, nil
}
func (result ReconcileResult) Scan() repository.Scan { return result.scan }
func (result ReconcileResult) Artifacts() []repository.Artifact {
	return append([]repository.Artifact(nil), result.artifacts...)
}

type ScanList struct {
	items []repository.Scan
	next  repository.Cursor
}

func NewScanList(items []repository.Scan, next repository.Cursor) (ScanList, error) {
	page, err := repository.NewScanPage(items, next)
	if err != nil {
		return ScanList{}, err
	}
	return ScanList{items: page.Items(), next: page.NextCursor()}, nil
}
func (list ScanList) Items() []repository.Scan      { return append([]repository.Scan(nil), list.items...) }
func (list ScanList) NextCursor() repository.Cursor { return list.next }

type ArtifactList struct {
	items []repository.Artifact
	next  repository.Cursor
}

func NewArtifactList(items []repository.Artifact, next repository.Cursor) (ArtifactList, error) {
	page, err := repository.NewArtifactPage(items, next)
	if err != nil {
		return ArtifactList{}, err
	}
	return ArtifactList{items: page.Items(), next: page.NextCursor()}, nil
}
func (list ArtifactList) Items() []repository.Artifact {
	return append([]repository.Artifact(nil), list.items...)
}
func (list ArtifactList) NextCursor() repository.Cursor { return list.next }

func validName(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if !(unicode.IsLower(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '/' || character == '.') {
			return false
		}
	}
	return true
}
func validVersion(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if !(unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.') {
			return false
		}
	}
	return true
}
func safeRevision(value string) bool {
	if value == "" || len(value) > 1024 || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
