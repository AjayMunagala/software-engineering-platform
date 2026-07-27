package repository

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	ArtifactIdentityScheme = "repository-service-artifact-id/v1"
	artifactIdentityDomain = "repository-service-artifact-id/v1\x00"
	artifactIdentityPrefix = "rsaid1_"
	profileIdentityDomain  = "repository-service-profile/v1\x00"
)

// Contract validates and constructs neutral immutable values.
type Contract struct {
	config   Config
	profiles *ProfileRegistry
}

// New creates the Phase 4.0.2 candidate contract. It performs no I/O.
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
	profiles, err := NewProfileRegistry(DefaultRepositoryGoProfile())
	if err != nil {
		return nil, err
	}
	return &Contract{config: config, profiles: profiles}, nil
}

func (contract *Contract) Config() Config {
	if contract == nil {
		return Config{}
	}
	return contract.config
}
func (contract *Contract) Profiles() *ProfileRegistry {
	if contract == nil {
		return nil
	}
	return contract.profiles.Clone()
}

func NewScope(scopeID ScopeID, principalID PrincipalID) (Scope, error) {
	if !validMachine(string(scopeID), 128) || !validMachine(string(principalID), 128) {
		return Scope{}, NewError(ErrorInvalidInput, "new-scope", "invalid-scope", false, nil)
	}
	return Scope{scopeID: scopeID, principalID: principalID}, nil
}

func NewSourceHandle(value string, maximum int) (SourceHandle, error) {
	if maximum < 1 || !validSensitive(value, maximum) {
		return SourceHandle{}, NewError(ErrorInvalidInput, "new-source-handle", "invalid-source-handle", false, nil)
	}
	return SourceHandle{value: value}, nil
}

func NewAnalysisProfile(name, version string, digest Digest) (AnalysisProfile, error) {
	if !validName(name, 128) || !validVersion(version) || digest.IsZero() {
		return AnalysisProfile{}, NewError(ErrorInvalidInput, "new-analysis-profile", "invalid-profile", false, nil)
	}
	return AnalysisProfile{name: name, version: version, digest: digest}, nil
}

func (contract *Contract) NewRegisterRepositoryRequest(params RegisterRepositoryParams) (RegisterRepositoryRequest, error) {
	if contract == nil || params.Scope.IsZero() || !validMachine(string(params.RequestID), 128) || !validMachine(string(params.RepositoryID), 128) || !validSafeText(params.DisplayName, contract.config.MaxDisplayNameBytes) {
		return RegisterRepositoryRequest{}, NewError(ErrorInvalidInput, "register-repository", "invalid-request", false, nil)
	}
	handle, err := NewSourceHandle(params.SourceHandle, contract.config.MaxSourceHandleBytes)
	if err != nil {
		return RegisterRepositoryRequest{}, err
	}
	return RegisterRepositoryRequest{scope: params.Scope, requestID: params.RequestID, repositoryID: params.RepositoryID, displayName: params.DisplayName, source: handle}, nil
}

func NewRepositoryQuery(scope Scope, repositoryID RepositoryID) (RepositoryQuery, error) {
	if scope.IsZero() || !validMachine(string(repositoryID), 128) {
		return RepositoryQuery{}, invalid("get-repository")
	}
	return RepositoryQuery{scope: scope, repositoryID: repositoryID}, nil
}

func (contract *Contract) NewRepositoryListRequest(params RepositoryListParams) (RepositoryListRequest, error) {
	if contract == nil || params.Scope.IsZero() || params.PageSize < 1 || params.PageSize > contract.config.MaxPageSize || !validOptionalMachine(string(params.Cursor), 1024) {
		return RepositoryListRequest{}, invalid("list-repositories")
	}
	return RepositoryListRequest{scope: params.Scope, pageSize: params.PageSize, cursor: params.Cursor}, nil
}

func NewArchiveRepositoryRequest(params ArchiveRepositoryParams) (ArchiveRepositoryRequest, error) {
	if params.Scope.IsZero() || !validMachine(string(params.RequestID), 128) || !validMachine(string(params.RepositoryID), 128) {
		return ArchiveRepositoryRequest{}, invalid("archive-repository")
	}
	return ArchiveRepositoryRequest{scope: params.Scope, requestID: params.RequestID, repositoryID: params.RepositoryID}, nil
}

func (contract *Contract) NewExecuteScanRequest(params ExecuteScanParams) (ExecuteScanRequest, error) {
	if contract == nil || params.Scope.IsZero() || !validMachine(string(params.RequestID), 128) || !validMachine(string(params.RepositoryID), 128) || !validMachine(string(params.ScanID), 128) || params.Profile.IsZero() {
		return ExecuteScanRequest{}, invalid("execute-scan")
	}
	if _, ok := contract.profiles.Resolve(params.Profile.Name(), params.Profile.Version(), params.Profile.Digest()); !ok {
		return ExecuteScanRequest{}, NewError(ErrorInvalidInput, "execute-scan", "unsupported-profile", false, nil)
	}
	handle, err := NewSourceHandle(params.SourceHandle, contract.config.MaxSourceHandleBytes)
	if err != nil {
		return ExecuteScanRequest{}, err
	}
	return ExecuteScanRequest{scope: params.Scope, requestID: params.RequestID, repositoryID: params.RepositoryID, scanID: params.ScanID, source: handle, profile: params.Profile}, nil
}

func NewScanQuery(scope Scope, repositoryID RepositoryID, scanID ScanID) (ScanQuery, error) {
	if scope.IsZero() || !validMachine(string(repositoryID), 128) || !validMachine(string(scanID), 128) {
		return ScanQuery{}, invalid("get-scan")
	}
	return ScanQuery{scope: scope, repositoryID: repositoryID, scanID: scanID}, nil
}

func (contract *Contract) NewScanListRequest(params ScanListParams) (ScanListRequest, error) {
	if contract == nil || params.Scope.IsZero() || !validMachine(string(params.RepositoryID), 128) || params.PageSize < 1 || params.PageSize > contract.config.MaxPageSize || !validOptionalMachine(string(params.Cursor), 1024) {
		return ScanListRequest{}, invalid("list-scans")
	}
	return ScanListRequest{scope: params.Scope, repositoryID: params.RepositoryID, pageSize: params.PageSize, cursor: params.Cursor}, nil
}

func NewCancelScanRequest(params CancelScanParams) (CancelScanRequest, error) {
	if params.Scope.IsZero() || !validMachine(string(params.RequestID), 128) || !validMachine(string(params.RepositoryID), 128) || !validMachine(string(params.ScanID), 128) {
		return CancelScanRequest{}, invalid("cancel-scan")
	}
	return CancelScanRequest{scope: params.Scope, requestID: params.RequestID, repositoryID: params.RepositoryID, scanID: params.ScanID}, nil
}

func NewArtifactQuery(scope Scope, repositoryID RepositoryID, scanID ScanID, artifactID ArtifactID) (ArtifactQuery, error) {
	if scope.IsZero() || !validMachine(string(repositoryID), 128) || !validMachine(string(scanID), 128) || !validMachine(string(artifactID), 256) {
		return ArtifactQuery{}, invalid("get-artifact")
	}
	return ArtifactQuery{scope: scope, repositoryID: repositoryID, scanID: scanID, artifactID: artifactID}, nil
}

func (contract *Contract) NewArtifactListRequest(params ArtifactListParams) (ArtifactListRequest, error) {
	if contract == nil || params.Scope.IsZero() || !validMachine(string(params.RepositoryID), 128) || !validMachine(string(params.ScanID), 128) || params.PageSize < 1 || params.PageSize > contract.config.MaxPageSize || !validOptionalMachine(string(params.Cursor), 1024) {
		return ArtifactListRequest{}, invalid("list-artifacts")
	}
	return ArtifactListRequest{scope: params.Scope, repositoryID: params.RepositoryID, scanID: params.ScanID, pageSize: params.PageSize, cursor: params.Cursor}, nil
}

func NewExportArtifactRequest(query ArtifactQuery) (ExportArtifactRequest, error) {
	if query.scope.IsZero() || query.repositoryID == "" || query.scanID == "" || query.artifactID == "" {
		return ExportArtifactRequest{}, invalid("export-artifact")
	}
	return ExportArtifactRequest{query: query}, nil
}

func NewRepository(params RepositoryParams) (Repository, error) {
	if !validMachine(string(params.RepositoryID), 128) || !validSafeText(params.DisplayName, defaultMaxDisplayNameBytes) || !validName(params.SourceKind, 64) || !validName(params.FingerprintScheme, 128) || params.Fingerprint.IsZero() || !validRepositoryState(params.State) || !validOptionalMachine(string(params.CurrentScanID), 128) || params.CreatedAt.IsZero() || params.UpdatedAt.IsZero() || params.UpdatedAt.Before(params.CreatedAt) {
		return Repository{}, invalid("new-repository")
	}
	params.CreatedAt, params.UpdatedAt = params.CreatedAt.UTC(), params.UpdatedAt.UTC()
	return Repository{params: params}, nil
}

func NewScan(params ScanParams) (Scan, error) {
	if !validMachine(string(params.RepositoryID), 128) || !validMachine(string(params.ScanID), 128) || params.Profile.IsZero() || !validScanState(params.State) || (params.SourceRevision != "" && !validIdentityField(params.SourceRevision)) || params.RequestedAt.IsZero() || !validOptionalToken(params.ReasonCode, 128) || !validScanTimes(params) {
		return Scan{}, invalid("new-scan")
	}
	params.RequestedAt, params.StartedAt, params.FinishedAt = params.RequestedAt.UTC(), params.StartedAt.UTC(), params.FinishedAt.UTC()
	return Scan{params: params}, nil
}

func NewArtifact(params ArtifactParams) (Artifact, error) {
	if !validMachine(string(params.ArtifactID), 256) || !validMachine(string(params.ScanID), 128) || !validName(params.Name, 128) || !validVersion(params.Version) || !validName(params.StableIDScheme, 128) || !validName(params.CodecName, 128) || !validVersion(params.CodecVersion) || !validMediaType(params.MediaType) || params.PayloadDigest.IsZero() || params.PayloadSize == 0 || params.PayloadSize > defaultMaxArtifactBytes || !validName(params.ProducerName, 128) || !validVersion(params.ProducerVersion) || params.CreatedAt.IsZero() {
		return Artifact{}, invalid("new-artifact")
	}
	params.CreatedAt = params.CreatedAt.UTC()
	return Artifact{params: params}, nil
}

func NewRepositoryPage(items []Repository, next Cursor) (RepositoryPage, error) {
	if !validOptionalMachine(string(next), 1024) {
		return RepositoryPage{}, invalid("new-repository-page")
	}
	for _, item := range items {
		if item.RepositoryID() == "" {
			return RepositoryPage{}, invalid("new-repository-page")
		}
	}
	return RepositoryPage{items: append([]Repository(nil), items...), next: next}, nil
}
func NewScanPage(items []Scan, next Cursor) (ScanPage, error) {
	if !validOptionalMachine(string(next), 1024) {
		return ScanPage{}, invalid("new-scan-page")
	}
	for _, item := range items {
		if item.ScanID() == "" {
			return ScanPage{}, invalid("new-scan-page")
		}
	}
	return ScanPage{items: append([]Scan(nil), items...), next: next}, nil
}
func NewArtifactPage(items []Artifact, next Cursor) (ArtifactPage, error) {
	if !validOptionalMachine(string(next), 1024) {
		return ArtifactPage{}, invalid("new-artifact-page")
	}
	for _, item := range items {
		if item.ArtifactID() == "" {
			return ArtifactPage{}, invalid("new-artifact-page")
		}
	}
	return ArtifactPage{items: append([]Artifact(nil), items...), next: next}, nil
}
func NewScanResult(scan Scan, artifacts []Artifact, disposition Disposition) (ScanResult, error) {
	if scan.ScanID() == "" || !validDisposition(disposition) {
		return ScanResult{}, invalid("new-scan-result")
	}
	copyArtifacts := append([]Artifact(nil), artifacts...)
	for _, artifact := range copyArtifacts {
		if artifact.ScanID() != scan.ScanID() {
			return ScanResult{}, invalid("new-scan-result")
		}
	}
	return ScanResult{scan: scan, artifacts: copyArtifacts, disposition: disposition}, nil
}
func NewExportReceipt(digest Digest, size uint64) (ExportReceipt, error) {
	if digest.IsZero() || size == 0 || size > defaultMaxArtifactBytes {
		return ExportReceipt{}, invalid("new-export-receipt")
	}
	return ExportReceipt{digest: digest, size: size}, nil
}

// CanonicalArtifactIdentity freezes repository-service-artifact-id/v1 bytes.
func CanonicalArtifactIdentity(repositoryID RepositoryID, scanID ScanID, name, version, stableIDScheme string) ([]byte, error) {
	fields := []string{string(repositoryID), string(scanID), name, version, stableIDScheme}
	result := append([]byte(nil), artifactIdentityDomain...)
	var length [4]byte
	for _, field := range fields {
		if !validIdentityField(field) {
			return nil, invalid("artifact-identity")
		}
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		result = append(result, length[:]...)
		result = append(result, field...)
	}
	return result, nil
}

func NewArtifactID(repositoryID RepositoryID, scanID ScanID, name, version, stableIDScheme string) (ArtifactID, error) {
	canonical, err := CanonicalArtifactIdentity(repositoryID, scanID, name, version, stableIDScheme)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return ArtifactID(artifactIdentityPrefix + hex.EncodeToString(digest[:])), nil
}

func NewProfileArtifact(name, version, stableIDScheme string) (ProfileArtifact, error) {
	if !validName(name, 128) || !validVersion(version) || !validName(stableIDScheme, 128) {
		return ProfileArtifact{}, invalid("new-profile-artifact")
	}
	return ProfileArtifact{name: name, version: version, stableIDScheme: stableIDScheme}, nil
}

func NewProfileDefinition(name, version string, artifacts []ProfileArtifact) (ProfileDefinition, error) {
	if !validName(name, 128) || !validVersion(version) || len(artifacts) == 0 || len(artifacts) > defaultMaxArtifactsPerScan {
		return ProfileDefinition{}, invalid("new-profile-definition")
	}
	copyArtifacts := append([]ProfileArtifact(nil), artifacts...)
	seen := make(map[string]struct{}, len(copyArtifacts))
	for _, artifact := range copyArtifacts {
		key := artifact.Name() + "\x00" + artifact.Version()
		if !validName(artifact.Name(), 128) || !validVersion(artifact.Version()) || !validName(artifact.StableIDScheme(), 128) {
			return ProfileDefinition{}, invalid("new-profile-definition")
		}
		if _, exists := seen[key]; exists {
			return ProfileDefinition{}, NewError(ErrorConflict, "new-profile-definition", "duplicate-artifact", false, nil)
		}
		seen[key] = struct{}{}
	}
	canonical := append([]byte(nil), profileIdentityDomain...)
	canonical = appendCanonicalField(canonical, name)
	canonical = appendCanonicalField(canonical, version)
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(copyArtifacts)))
	canonical = append(canonical, count[:]...)
	for _, artifact := range copyArtifacts {
		canonical = appendCanonicalField(canonical, artifact.Name())
		canonical = appendCanonicalField(canonical, artifact.Version())
		canonical = appendCanonicalField(canonical, artifact.StableIDScheme())
	}
	profile, err := NewAnalysisProfile(name, version, sha256.Sum256(canonical))
	if err != nil {
		return ProfileDefinition{}, err
	}
	return ProfileDefinition{profile: profile, artifacts: copyArtifacts}, nil
}

func appendCanonicalField(destination []byte, value string) []byte {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	destination = append(destination, length[:]...)
	return append(destination, value...)
}

// ProfileRegistry is immutable after construction.
type ProfileRegistry struct{ definitions map[string]ProfileDefinition }

func NewProfileRegistry(definitions ...ProfileDefinition) (*ProfileRegistry, error) {
	registry := &ProfileRegistry{definitions: make(map[string]ProfileDefinition, len(definitions))}
	for _, definition := range definitions {
		if definition.Profile().IsZero() {
			return nil, invalid("new-profile-registry")
		}
		key := profileKey(definition.Profile().Name(), definition.Profile().Version())
		if _, exists := registry.definitions[key]; exists {
			return nil, NewError(ErrorConflict, "new-profile-registry", "duplicate-profile", false, nil)
		}
		registry.definitions[key] = cloneProfileDefinition(definition)
	}
	return registry, nil
}

func (registry *ProfileRegistry) Resolve(name, version string, digest Digest) (ProfileDefinition, bool) {
	if registry == nil {
		return ProfileDefinition{}, false
	}
	definition, ok := registry.definitions[profileKey(name, version)]
	if !ok || definition.Profile().Digest() != digest {
		return ProfileDefinition{}, false
	}
	return cloneProfileDefinition(definition), true
}
func (registry *ProfileRegistry) Definitions() []ProfileDefinition {
	if registry == nil {
		return []ProfileDefinition{}
	}
	result := make([]ProfileDefinition, 0, len(registry.definitions))
	for _, definition := range registry.definitions {
		result = append(result, cloneProfileDefinition(definition))
	}
	sortProfileDefinitions(result)
	return result
}
func (registry *ProfileRegistry) Clone() *ProfileRegistry {
	if registry == nil {
		return nil
	}
	result, _ := NewProfileRegistry(registry.Definitions()...)
	return result
}

func DefaultRepositoryGoProfile() ProfileDefinition {
	names := []struct{ name, version, scheme string }{
		{"discovery-inventory", "1.0.0", ArtifactIdentityScheme},
		{"repository-snapshot", "1.0.0", ArtifactIdentityScheme},
		{"language-inventory", "1.0.0", ArtifactIdentityScheme},
		{"framework-inventory", "1.0.0", ArtifactIdentityScheme},
		{"build-inventory", "1.0.0", ArtifactIdentityScheme},
		{"repository-metadata", "1.0.0", ArtifactIdentityScheme},
		{"repository-intelligence-summary", "1.0.0", ArtifactIdentityScheme},
		{"go-language-inventory", "1.0.0", ArtifactIdentityScheme},
		{"go-package-identity-inventory", "1.0.0", "go-package-proof-id/v1"},
		{"go-semantic-inventory", "1.0.0", "go-semantic-id/v1"},
	}
	artifacts := make([]ProfileArtifact, 0, len(names))
	for _, item := range names {
		artifact, _ := NewProfileArtifact(item.name, item.version, item.scheme)
		artifacts = append(artifacts, artifact)
	}
	definition, _ := NewProfileDefinition("repository-go", "1", artifacts)
	return definition
}

func cloneProfileDefinition(value ProfileDefinition) ProfileDefinition {
	return ProfileDefinition{profile: value.profile, artifacts: value.Artifacts()}
}
func profileKey(name, version string) string { return name + "\x00" + version }
func sortProfileDefinitions(values []ProfileDefinition) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && profileKey(values[j].Profile().Name(), values[j].Profile().Version()) < profileKey(values[j-1].Profile().Name(), values[j-1].Profile().Version()); j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func invalid(operation string) error {
	return NewError(ErrorInvalidInput, operation, "invalid-request", false, nil)
}
func validMachine(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if !(unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.' || character == ':') {
			return false
		}
	}
	return true
}
func validOptionalMachine(value string, maximum int) bool {
	return value == "" || validMachine(value, maximum)
}
func validSensitive(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
func validSafeText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
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
func validVersion(value string) bool { return validMachine(value, 64) }
func validMediaType(value string) bool {
	return value == "application/json" || value == "application/octet-stream"
}
func validOptionalToken(value string, maximum int) bool {
	return value == "" || safeToken(value, maximum)
}
func safeToken(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}
func validIdentityField(value string) bool {
	if value == "" || len(value) > 1024 || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
func validRepositoryState(state RepositoryState) bool {
	return state == RepositoryActive || state == RepositoryArchived || state == RepositoryPurgePending
}
func validScanState(state ScanState) bool {
	return state == ScanRequested || state == ScanRunning || state == ScanSucceeded || state == ScanFailed || state == ScanCanceled
}
func validDisposition(value Disposition) bool {
	return value == DispositionCreated || value == DispositionAlreadyPresent || value == DispositionJoined
}

func validScanTimes(params ScanParams) bool {
	if !params.StartedAt.IsZero() && params.StartedAt.Before(params.RequestedAt) {
		return false
	}
	if !params.FinishedAt.IsZero() && (params.StartedAt.IsZero() || params.FinishedAt.Before(params.StartedAt)) {
		return false
	}
	switch params.State {
	case ScanRequested:
		return params.StartedAt.IsZero() && params.FinishedAt.IsZero()
	case ScanRunning:
		return !params.StartedAt.IsZero() && params.FinishedAt.IsZero()
	case ScanSucceeded, ScanFailed, ScanCanceled:
		return !params.StartedAt.IsZero() && !params.FinishedAt.IsZero()
	default:
		return false
	}
}
