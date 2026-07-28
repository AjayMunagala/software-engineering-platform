package adapters

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	goengine "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/semantic"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	buildengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/build"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	frameworkengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/framework"
	ignoreengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	languageengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
	metadataengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/metadata"
	summaryengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/summary"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/scan"
)

const (
	codecName    = "canonical-json"
	codecVersion = "1.0.0"
)

// Adapter composes only released intelligence engines and deterministic
// artifact codecs. Every Prepare returns an isolated, single-use session.
type Adapter struct {
	resolver SourceResolver
	config   Config
	specs    map[string]artifactSpec
}

func New(resolver SourceResolver, configs ...Config) (*Adapter, error) {
	if resolver == nil || len(configs) > 1 {
		return nil, repository.NewError(repository.ErrorInvalidInput, "new-intelligence-adapter", "invalid-dependencies", false, nil)
	}
	config := DefaultConfig()
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Adapter{resolver: resolver, config: config, specs: frozenSpecs()}, nil
}

func (adapter *Adapter) Prepare(ctx context.Context, request scan.AnalysisRequest) (scan.AnalysisSession, error) {
	if adapter == nil || request.SourceHandle().IsZero() || request.Scope().IsZero() {
		return nil, repository.NewError(repository.ErrorInvalidInput, "prepare-analysis", "invalid-request", false, nil)
	}
	if err := contextError(ctx, "prepare-analysis"); err != nil {
		return nil, err
	}
	expected := repository.DefaultRepositoryGoProfile().Profile()
	if request.Profile() != expected {
		return nil, repository.NewError(repository.ErrorInvalidInput, "prepare-analysis", "unsupported-profile", false, nil)
	}
	source, err := adapter.resolver.Resolve(ctx, request.Scope(), request.SourceHandle())
	if err != nil {
		return nil, serviceError(repository.ErrorSourceUnavailable, "prepare-analysis", "source-unavailable", err)
	}
	if source == nil || source.Fingerprint().IsZero() || source.RootPath() == "" {
		if source != nil {
			adapter.closeSource(source)
		}
		return nil, repository.NewError(repository.ErrorSourceUnavailable, "prepare-analysis", "invalid-source", false, nil)
	}
	root, err := filepath.Abs(filepath.Clean(source.RootPath()))
	if err != nil || root == "" {
		adapter.closeSource(source)
		return nil, repository.NewError(repository.ErrorSourceUnavailable, "prepare-analysis", "invalid-source", false, nil)
	}
	if spoolInsideRepository(root, adapter.config.SpoolDirectory) {
		adapter.closeSource(source)
		return nil, repository.NewError(repository.ErrorInvalidInput, "prepare-analysis", "unsafe-spool-directory", false, nil)
	}
	if !safeRevision(source.Revision(), root) {
		adapter.closeSource(source)
		return nil, repository.NewError(repository.ErrorSourceUnavailable, "prepare-analysis", "invalid-source-revision", false, nil)
	}
	return &session{adapter: adapter, source: source, root: root, fingerprint: source.Fingerprint(), revision: source.Revision(), profile: request.Profile(), repositoryID: request.RepositoryID(), payloads: []*sealedPayload{}}, nil
}

func spoolInsideRepository(root, spool string) bool {
	canonicalRoot, rootErr := filepath.EvalSymlinks(root)
	canonicalSpool, spoolErr := filepath.EvalSymlinks(spool)
	if rootErr != nil || spoolErr != nil {
		return false
	}
	relative, err := filepath.Rel(canonicalRoot, canonicalSpool)
	return err == nil && (relative == "." || (!filepath.IsAbs(relative) && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))))
}

func (adapter *Adapter) closeSource(source AuthorizedSource) {
	ctx, cancel := context.WithTimeout(context.Background(), adapter.config.CleanupTimeout)
	defer cancel()
	_ = source.Close(ctx)
}

func safeRevision(revision, root string) bool {
	if len(revision) > 1024 || !utf8.ValidString(revision) {
		return false
	}
	for _, character := range revision {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	for _, value := range forbiddenVariants(root) {
		if value != "" && strings.Contains(revision, value) {
			return false
		}
	}
	return true
}

func (current *session) Analyze(ctx context.Context) (scan.AnalysisResult, error) {
	if current == nil || current.adapter == nil {
		return scan.AnalysisResult{}, repository.NewError(repository.ErrorAnalysisFailed, "analyze-repository", "invalid-session", false, nil)
	}
	if err := contextError(ctx, "analyze-repository"); err != nil {
		return scan.AnalysisResult{}, err
	}
	current.mu.Lock()
	if current.closed || current.analyzed {
		current.mu.Unlock()
		return scan.AnalysisResult{}, repository.NewError(repository.ErrorConflict, "analyze-repository", "session-not-reusable", false, nil)
	}
	current.analyzed = true
	current.mu.Unlock()

	store, goPresent, err := runReleasedProfile(ctx, current.root)
	if err != nil {
		return scan.AnalysisResult{}, serviceError(repository.ErrorAnalysisFailed, "analyze-repository", "engine-failed", err)
	}
	definition := repository.DefaultRepositoryGoProfile()
	candidates := make([]scan.ArtifactCandidate, 0, len(definition.Artifacts()))
	for _, contract := range definition.Artifacts() {
		artifact, exists := store.Get(contract.Name())
		if !exists {
			if !goPresent && isGoArtifact(contract.Name()) {
				continue
			}
			current.cleanupPayloads()
			return scan.AnalysisResult{}, repository.NewError(repository.ErrorIntegrityFailure, "analyze-repository", "required-artifact-missing", false, nil)
		}
		if artifact.ArtifactVersion() != contract.Version() {
			current.cleanupPayloads()
			return scan.AnalysisResult{}, repository.NewError(repository.ErrorIntegrityFailure, "analyze-repository", "artifact-version-mismatch", false, nil)
		}
		spec, ok := current.adapter.specs[artifact.ArtifactName()]
		if !ok || spec.version != artifact.ArtifactVersion() || spec.scheme != contract.StableIDScheme() {
			current.cleanupPayloads()
			return scan.AnalysisResult{}, repository.NewError(repository.ErrorIntegrityFailure, "analyze-repository", "codec-contract-mismatch", false, nil)
		}
		producerName, producerVersion, producerOK := producerOf(artifact)
		if !producerOK || producerName != spec.producer || producerVersion != spec.producerVersion {
			current.cleanupPayloads()
			return scan.AnalysisResult{}, repository.NewError(repository.ErrorIntegrityFailure, "analyze-repository", "producer-contract-mismatch", false, nil)
		}
		payload, materializeErr := current.adapter.materialize(ctx, current.root, string(current.repositoryID), encodedArtifact{spec: spec, value: artifact})
		if materializeErr != nil {
			current.cleanupPayloads()
			return scan.AnalysisResult{}, serviceError(repository.ErrorMaterializationFailed, "analyze-repository", "materialization-failed", materializeErr)
		}
		current.mu.Lock()
		current.payloads = append(current.payloads, payload)
		current.mu.Unlock()
		deps, dependencyErr := dependencies(spec.dependencies)
		if dependencyErr != nil {
			current.cleanupPayloads()
			return scan.AnalysisResult{}, dependencyErr
		}
		candidate, candidateErr := scan.NewArtifactCandidate(scan.ArtifactCandidateParams{
			Name: spec.name, Version: spec.version, StableIDScheme: spec.scheme,
			CodecName: codecName, CodecVersion: codecVersion, MediaType: "application/json",
			PayloadDigest: payload.digest, PayloadSize: payload.size,
			ProducerName: spec.producer, ProducerVersion: spec.producerVersion,
			Dependencies: deps, Payload: payload,
		})
		if candidateErr != nil {
			current.cleanupPayloads()
			return scan.AnalysisResult{}, candidateErr
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 || len(candidates) > current.adapter.config.MaxArtifacts {
		current.cleanupPayloads()
		return scan.AnalysisResult{}, repository.NewError(repository.ErrorIntegrityFailure, "analyze-repository", "invalid-artifact-count", false, nil)
	}
	result, err := scan.NewAnalysisResult(current.profile, candidates)
	if err != nil {
		current.cleanupPayloads()
		return scan.AnalysisResult{}, err
	}
	return result, nil
}

func producerOf(artifact rie.Artifact) (string, string, bool) {
	switch value := artifact.(type) {
	case discovery.DiscoveryInventory:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	case rie.RepositorySnapshot:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	case languageengine.LanguageInventory:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	case frameworkengine.FrameworkInventory:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	case buildengine.BuildInventory:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	case metadataengine.RepositoryMetadata:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	case summaryengine.RepositoryIntelligenceSummary:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	case goengine.GoLanguageInventory:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	case packageidentity.GoPackageIdentityInventory:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	case semantic.GoSemanticInventory:
		item := value.Metadata()
		return item.EngineName, item.EngineVersion, true
	default:
		return "", "", false
	}
}

func (current *session) Close(ctx context.Context) error {
	if current == nil {
		return nil
	}
	if err := contextError(ctx, "close-analysis-session"); err != nil {
		return err
	}
	current.mu.Lock()
	if current.closed {
		current.mu.Unlock()
		return nil
	}
	current.closed = true
	payloads := append([]*sealedPayload(nil), current.payloads...)
	current.payloads = nil
	source := current.source
	current.mu.Unlock()
	var first error
	for index := len(payloads) - 1; index >= 0; index-- {
		if err := payloads[index].close(); err != nil && first == nil {
			first = err
		}
	}
	if source != nil {
		if err := source.Close(ctx); err != nil && first == nil {
			first = err
		}
	}
	if first != nil {
		return serviceError(repository.ErrorMaterializationFailed, "close-analysis-session", "cleanup-failed", first)
	}
	return nil
}

func (current *session) cleanupPayloads() {
	current.mu.Lock()
	payloads := append([]*sealedPayload(nil), current.payloads...)
	current.payloads = nil
	current.mu.Unlock()
	for index := len(payloads) - 1; index >= 0; index-- {
		_ = payloads[index].close()
	}
}

func runReleasedProfile(ctx context.Context, root string) (*rie.ArtifactStore, bool, error) {
	run := rie.NewRunContext(root, rie.DefaultConfig())
	pipeline := rie.New()
	for _, engine := range []rie.Engine{discovery.New(), ignoreengine.New(), languageengine.New(), frameworkengine.New(), buildengine.New(), metadataengine.New(), summaryengine.New()} {
		if err := pipeline.Register(engine); err != nil {
			return nil, false, err
		}
	}
	if err := pipeline.Run(ctx, run); err != nil {
		return nil, false, err
	}
	languages, ok := rie.ArtifactAs[languageengine.LanguageInventory](run.Artifacts, languageengine.LanguageInventoryArtifactName)
	if !ok {
		return nil, false, fmt.Errorf("language inventory unavailable")
	}
	goPresent := false
	for _, item := range languages.Items() {
		if item.Name == "Go" && item.Count > 0 {
			goPresent = true
			break
		}
	}
	if !goPresent {
		return run.Artifacts, false, nil
	}
	analyzer, err := goengine.New()
	if err != nil {
		return nil, false, err
	}
	runner, err := lie.New(analyzer)
	if err != nil {
		return nil, false, err
	}
	if _, err = runner.Run(ctx, run.Artifacts); err != nil {
		return nil, false, err
	}
	syntax, ok := goengine.InventoryFrom(run.Artifacts)
	if !ok {
		return nil, false, fmt.Errorf("Go syntax inventory unavailable")
	}
	snapshot, ok := rie.ArtifactAs[rie.RepositorySnapshot](run.Artifacts, rie.RepositorySnapshotArtifactName)
	if !ok {
		return nil, false, fmt.Errorf("repository snapshot unavailable")
	}
	identityEngine, err := packageidentity.New()
	if err != nil {
		return nil, false, err
	}
	identities, err := identityEngine.Analyze(ctx, packageidentity.Input{Snapshot: snapshot, Syntax: syntax})
	if err != nil {
		return nil, false, err
	}
	if err = run.Artifacts.Put(identities); err != nil {
		return nil, false, err
	}
	integrator, err := semantic.NewIntegrator()
	if err != nil {
		return nil, false, err
	}
	if _, err = integrator.Run(ctx, run.Artifacts); err != nil {
		return nil, false, err
	}
	return run.Artifacts, true, nil
}

func isGoArtifact(name string) bool {
	return name == goengine.ArtifactName || name == packageidentity.ArtifactName || name == semantic.ArtifactName
}

func frozenSpecs() map[string]artifactSpec {
	ref := func(name string) artifactRef { return artifactRef{name: name, version: "1.0.0"} }
	values := []artifactSpec{
		{name: discovery.DiscoveryInventoryArtifactName, version: "1.0.0", scheme: repository.ArtifactIdentityScheme, producer: "discovery", producerVersion: "0.1.1"},
		{name: rie.RepositorySnapshotArtifactName, version: "1.0.0", scheme: repository.ArtifactIdentityScheme, producer: "ignore", producerVersion: "0.2.1", dependencies: []artifactRef{ref(discovery.DiscoveryInventoryArtifactName)}},
		{name: languageengine.LanguageInventoryArtifactName, version: "1.0.0", scheme: repository.ArtifactIdentityScheme, producer: "language", producerVersion: "0.3.2", dependencies: []artifactRef{ref(rie.RepositorySnapshotArtifactName)}},
		{name: frameworkengine.FrameworkInventoryArtifactName, version: "1.0.0", scheme: repository.ArtifactIdentityScheme, producer: "framework", producerVersion: "0.4.2", dependencies: []artifactRef{ref(rie.RepositorySnapshotArtifactName), ref(languageengine.LanguageInventoryArtifactName)}},
		{name: buildengine.BuildInventoryArtifactName, version: "1.0.0", scheme: repository.ArtifactIdentityScheme, producer: "build-package", producerVersion: "0.5.0", dependencies: []artifactRef{ref(rie.RepositorySnapshotArtifactName)}},
		{name: metadataengine.RepositoryMetadataArtifactName, version: "1.0.0", scheme: repository.ArtifactIdentityScheme, producer: "repository-metadata", producerVersion: "0.6.0", dependencies: []artifactRef{ref(discovery.DiscoveryInventoryArtifactName), ref(rie.RepositorySnapshotArtifactName), ref(languageengine.LanguageInventoryArtifactName), ref(frameworkengine.FrameworkInventoryArtifactName), ref(buildengine.BuildInventoryArtifactName)}},
		{name: summaryengine.RepositoryIntelligenceSummaryArtifactName, version: "1.0.0", scheme: repository.ArtifactIdentityScheme, producer: "repository-intelligence-summary", producerVersion: "0.7.0", dependencies: []artifactRef{ref(metadataengine.RepositoryMetadataArtifactName)}},
		{name: goengine.ArtifactName, version: "1.0.0", scheme: repository.ArtifactIdentityScheme, producer: "golang", producerVersion: "1.0.0", dependencies: []artifactRef{ref(rie.RepositorySnapshotArtifactName), ref(languageengine.LanguageInventoryArtifactName)}},
		{name: packageidentity.ArtifactName, version: "1.0.0", scheme: packageidentity.ProofIDSchemeVersion, producer: "go-package-identity", producerVersion: "1.0.0", dependencies: []artifactRef{ref(rie.RepositorySnapshotArtifactName), ref(goengine.ArtifactName)}},
		{name: semantic.ArtifactName, version: "1.0.0", scheme: semantic.IDSchemeVersion, producer: "go-semantic", producerVersion: "1.0.0", dependencies: []artifactRef{ref(rie.RepositorySnapshotArtifactName), ref(goengine.ArtifactName), ref(packageidentity.ArtifactName)}},
	}
	result := make(map[string]artifactSpec, len(values))
	for _, value := range values {
		result[value.name] = value
	}
	return result
}

type boundedHashWriter struct {
	ctx           context.Context
	writer        io.Writer
	hash          hash.Hash
	size, maximum uint64
}

func (writer *boundedHashWriter) Write(data []byte) (int, error) {
	if err := writer.ctx.Err(); err != nil {
		return 0, err
	}
	if writer.size+uint64(len(data)) > writer.maximum {
		return 0, ErrPayloadTooLarge
	}
	written, err := writer.writer.Write(data)
	if written > 0 {
		_, _ = writer.hash.Write(data[:written])
		writer.size += uint64(written)
	}
	return written, err
}

func (adapter *Adapter) materialize(ctx context.Context, root, repositoryName string, artifact encodedArtifact) (*sealedPayload, error) {
	file, err := os.CreateTemp(adapter.config.SpoolDirectory, "aegis-repository-service-*")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err = file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, err
	}
	digest := sha256.New()
	bounded := &boundedHashWriter{ctx: ctx, writer: file, hash: digest, maximum: adapter.config.MaxArtifactBytes}
	buffered := bufio.NewWriterSize(bounded, adapter.config.BufferBytes)
	if err = encodeArtifact(ctx, buffered, artifact.value, root, repositoryName); err == nil {
		err = buffered.Flush()
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	information, err := os.Stat(path)
	if err != nil || uint64(information.Size()) != bounded.size {
		return nil, fmt.Errorf("sealed payload size mismatch")
	}
	for _, forbidden := range forbiddenVariants(root) {
		found, scanErr := fileContains(path, []byte(forbidden), adapter.config.BufferBytes)
		if scanErr != nil {
			return nil, scanErr
		}
		if found {
			return nil, ErrForbiddenContent
		}
	}
	var value repository.Digest
	copy(value[:], digest.Sum(nil))
	remove = false
	return &sealedPayload{path: path, digest: value, size: bounded.size}, nil
}

func forbiddenVariants(root string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range []string{root, filepath.ToSlash(root), filepath.FromSlash(root), strings.ReplaceAll(root, `\`, `\\`)} {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func fileContains(path string, pattern []byte, bufferBytes int) (bool, error) {
	if len(pattern) == 0 {
		return false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	buffer := make([]byte, bufferBytes)
	tail := make([]byte, 0, len(pattern)-1)
	for {
		count, readErr := file.Read(buffer)
		if count > 0 {
			window := append(append(make([]byte, 0, len(tail)+count), tail...), buffer[:count]...)
			if bytes.Contains(window, pattern) {
				return true, nil
			}
			keep := len(pattern) - 1
			if keep > len(window) {
				keep = len(window)
			}
			tail = append(tail[:0], window[len(window)-keep:]...)
		}
		if readErr == io.EOF {
			return false, nil
		}
		if readErr != nil {
			return false, readErr
		}
	}
}

type jsonField struct {
	name   string
	encode EncodeFunc
}

func writeObject(ctx context.Context, writer io.Writer, fields ...jsonField) error {
	if _, err := io.WriteString(writer, "{"); err != nil {
		return err
	}
	for index, field := range fields {
		if index > 0 {
			if _, err := io.WriteString(writer, ","); err != nil {
				return err
			}
		}
		name, _ := json.Marshal(field.name)
		if _, err := writer.Write(name); err != nil {
			return err
		}
		if _, err := io.WriteString(writer, ":"); err != nil {
			return err
		}
		if err := field.encode(ctx, writer); err != nil {
			return err
		}
	}
	_, err := io.WriteString(writer, "}")
	return err
}

func jsonValue(value any) EncodeFunc {
	return func(ctx context.Context, writer io.Writer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	}
}

func jsonArray[T any](values []T) EncodeFunc {
	return func(ctx context.Context, writer io.Writer) error {
		if _, err := io.WriteString(writer, "["); err != nil {
			return err
		}
		for index, value := range values {
			if err := ctx.Err(); err != nil {
				return err
			}
			if index > 0 {
				if _, err := io.WriteString(writer, ","); err != nil {
					return err
				}
			}
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if _, err = writer.Write(data); err != nil {
				return err
			}
		}
		_, err := io.WriteString(writer, "]")
		return err
	}
}

func encodeArtifact(ctx context.Context, writer io.Writer, artifact rie.Artifact, root, repositoryName string) error {
	switch value := artifact.(type) {
	case discovery.DiscoveryInventory:
		repositoryValue := value.Repository()
		view := struct {
			Name          string `json:"name"`
			Git           bool   `json:"git"`
			CurrentBranch string `json:"current_branch,omitempty"`
			DefaultBranch string `json:"default_branch,omitempty"`
		}{repositoryName, repositoryValue.Git, repositoryValue.CurrentBranch, repositoryValue.DefaultBranch}
		return writeObject(ctx, writer, jsonField{"artifact", jsonValue(metadata(value.Metadata()))}, jsonField{"repository", jsonValue(view)}, jsonField{"statistics", jsonValue(value.Statistics())})
	case rie.RepositorySnapshot:
		return writeObject(ctx, writer, jsonField{"artifact", jsonValue(metadata(value.Metadata()))}, jsonField{"entries", jsonArray(value.Entries())}, jsonField{"statistics", jsonValue(value.Statistics())}, jsonField{"diagnostics", jsonArray(sanitizeRIEDiagnostics(value.Diagnostics(), root))})
	case languageengine.LanguageInventory:
		return writeObject(ctx, writer, jsonField{"artifact", jsonValue(value.Metadata())}, jsonField{"items", jsonArray(value.Items())}, jsonField{"summary", jsonValue(value.Summary())})
	case frameworkengine.FrameworkInventory:
		type item struct {
			Name      string         `json:"name"`
			Ecosystem string         `json:"ecosystem"`
			Evidence  []rie.Evidence `json:"evidence"`
		}
		items := make([]item, 0, len(value.Items()))
		for _, current := range value.Items() {
			items = append(items, item{current.Name, current.Ecosystem, current.Evidence()})
		}
		return writeObject(ctx, writer, jsonField{"artifact", jsonValue(value.Metadata())}, jsonField{"items", jsonArray(items)}, jsonField{"summary", jsonValue(value.Summary())})
	case buildengine.BuildInventory:
		return encodeBuild(ctx, writer, value)
	case metadataengine.RepositoryMetadata:
		return encodeMetadata(ctx, writer, value, repositoryName)
	case summaryengine.RepositoryIntelligenceSummary:
		ref := rie.ArtifactReference{Name: value.RepositoryMetadata().ArtifactName(), Version: value.RepositoryMetadata().ArtifactVersion()}
		return writeObject(ctx, writer, jsonField{"artifact", jsonValue(metadata(value.Metadata()))}, jsonField{"repository_metadata", jsonValue(ref)}, jsonField{"sections", jsonArray(value.Sections())}, jsonField{"capabilities", jsonArray(value.Capabilities())})
	case goengine.GoLanguageInventory:
		return writeObject(ctx, writer, jsonField{"artifact", jsonValue(value.Metadata())}, jsonField{"source_artifacts", jsonArray(value.SourceArtifacts())}, jsonField{"files", jsonArray(value.Files())}, jsonField{"packages", jsonArray(value.Packages())}, jsonField{"symbols", jsonArray(value.Symbols())}, jsonField{"diagnostics", jsonArray(value.Diagnostics())}, jsonField{"statistics", jsonValue(value.Statistics())})
	case packageidentity.GoPackageIdentityInventory:
		return writeObject(ctx, writer, jsonField{"artifact", jsonValue(value.Metadata())}, jsonField{"source_artifacts", jsonArray(value.SourceArtifacts())}, jsonField{"contexts", jsonArray(value.Contexts())}, jsonField{"modules", jsonArray(value.Modules())}, jsonField{"proofs", jsonArray(value.Proofs())}, jsonField{"diagnostics", jsonArray(value.Diagnostics())}, jsonField{"statistics", jsonValue(value.Statistics())})
	case semantic.GoSemanticInventory:
		return writeObject(ctx, writer, jsonField{"artifact", jsonValue(value.Metadata())}, jsonField{"source_artifacts", jsonArray(value.SourceArtifacts())}, jsonField{"files", jsonArray(value.Files())}, jsonField{"declarations", jsonArray(value.Declarations())}, jsonField{"references", jsonArray(value.References())}, jsonField{"receiver_bindings", jsonArray(value.ReceiverBindings())}, jsonField{"import_bindings", jsonArray(value.ImportBindings())}, jsonField{"type_relations", jsonArray(value.TypeRelations())}, jsonField{"interface_satisfaction", jsonArray(value.InterfaceSatisfaction())}, jsonField{"diagnostics", jsonArray(value.Diagnostics())}, jsonField{"statistics", jsonValue(value.Statistics())})
	default:
		return fmt.Errorf("unsupported artifact %s", artifact.ArtifactName())
	}
}

func encodeBuild(ctx context.Context, writer io.Writer, value buildengine.BuildInventory) error {
	type tool struct {
		ID       string         `json:"id"`
		Name     string         `json:"name"`
		Location string         `json:"location"`
		Evidence []rie.Evidence `json:"evidence"`
	}
	type workspace struct {
		ID       string         `json:"id"`
		Kind     string         `json:"kind"`
		Location string         `json:"location"`
		Members  []string       `json:"members"`
		Evidence []rie.Evidence `json:"evidence"`
	}
	type lock struct {
		PackageManagerID string         `json:"package_manager_id"`
		Path             string         `json:"path"`
		Location         string         `json:"location"`
		Evidence         []rie.Evidence `json:"evidence"`
	}
	type toolchain struct {
		Tool       string         `json:"tool"`
		Constraint string         `json:"constraint"`
		Location   string         `json:"location"`
		Evidence   []rie.Evidence `json:"evidence"`
	}
	managers := []tool{}
	for _, item := range value.PackageManagers() {
		managers = append(managers, tool{item.ID, item.Name, item.Location, item.Evidence()})
	}
	systems := []tool{}
	for _, item := range value.BuildSystems() {
		systems = append(systems, tool{item.ID, item.Name, item.Location, item.Evidence()})
	}
	workspaces := []workspace{}
	for _, item := range value.Workspaces() {
		workspaces = append(workspaces, workspace{item.ID, item.Kind, item.Location, item.Members(), item.Evidence()})
	}
	locks := []lock{}
	for _, item := range value.LockFiles() {
		locks = append(locks, lock{item.PackageManagerID, item.Path, item.Location, item.Evidence()})
	}
	toolchains := []toolchain{}
	for _, item := range value.Toolchains() {
		toolchains = append(toolchains, toolchain{item.Tool, item.Constraint, item.Location, item.Evidence()})
	}
	return writeObject(ctx, writer, jsonField{"artifact", jsonValue(value.Metadata())}, jsonField{"package_managers", jsonArray(managers)}, jsonField{"build_systems", jsonArray(systems)}, jsonField{"workspaces", jsonArray(workspaces)}, jsonField{"lock_files", jsonArray(locks)}, jsonField{"toolchains", jsonArray(toolchains)})
}

func encodeMetadata(ctx context.Context, writer io.Writer, value metadataengine.RepositoryMetadata, repositoryName string) error {
	type repositoryView struct {
		Name string                        `json:"name"`
		Git  metadataengine.GitInformation `json:"git"`
	}
	type layoutView struct {
		TopLevelDirectories []string `json:"top_level_directories"`
		TopLevelFiles       []string `json:"top_level_files"`
		MaximumDepth        int      `json:"maximum_depth"`
	}
	type frameworkView struct {
		Name      string   `json:"name"`
		Ecosystem string   `json:"ecosystem"`
		Locations []string `json:"locations"`
	}
	type technologyView struct {
		ID        string   `json:"id"`
		Name      string   `json:"name"`
		Locations []string `json:"locations"`
	}
	type toolchainView struct {
		Tool        string   `json:"tool"`
		Constraints []string `json:"constraints"`
		Locations   []string `json:"locations"`
	}
	repositoryValue, layout, build := value.Repository(), value.Layout(), value.Build()
	frameworks := []frameworkView{}
	for _, item := range value.Frameworks() {
		frameworks = append(frameworks, frameworkView{item.Name, item.Ecosystem, item.Locations()})
	}
	managers := []technologyView{}
	for _, item := range build.PackageManagers() {
		managers = append(managers, technologyView{item.ID, item.Name, item.Locations()})
	}
	systems := []technologyView{}
	for _, item := range build.BuildSystems() {
		systems = append(systems, technologyView{item.ID, item.Name, item.Locations()})
	}
	toolchains := []toolchainView{}
	for _, item := range build.Toolchains() {
		toolchains = append(toolchains, toolchainView{item.Tool, item.Constraints(), item.Locations()})
	}
	buildView := struct {
		PackageManagers []technologyView `json:"package_managers"`
		BuildSystems    []technologyView `json:"build_systems"`
		Toolchains      []toolchainView  `json:"toolchains"`
		LockFileCount   int              `json:"lock_file_count"`
	}{managers, systems, toolchains, build.LockFileCount}
	return writeObject(ctx, writer,
		jsonField{"artifact", jsonValue(metadata(value.Metadata()))}, jsonField{"repository", jsonValue(repositoryView{repositoryName, repositoryValue.Git})}, jsonField{"statistics", jsonValue(value.Statistics())},
		jsonField{"layout", jsonValue(layoutView{layout.TopLevelDirectories(), layout.TopLevelFiles(), layout.MaximumDepth})}, jsonField{"monorepo", jsonValue(value.Monorepo())}, jsonField{"workspace_count", jsonValue(value.WorkspaceCount())}, jsonField{"declared_module_count", jsonValue(value.DeclaredModuleCount())},
		jsonField{"languages", jsonArray(value.Languages())}, jsonField{"frameworks", jsonArray(frameworks)}, jsonField{"build", jsonValue(buildView)}, jsonField{"source_artifacts", jsonArray(value.SourceArtifacts())})
}

func sanitizeRIEDiagnostics(values []rie.Diagnostic, root string) []rie.Diagnostic {
	result := append([]rie.Diagnostic(nil), values...)
	variants := forbiddenVariants(root)
	for index := range result {
		for _, variant := range variants {
			result[index].Message = strings.ReplaceAll(result[index].Message, variant, "<repository>")
		}
		if filepath.IsAbs(result[index].Path) {
			relative, err := filepath.Rel(root, result[index].Path)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				result[index].Path = ""
			} else {
				result[index].Path = filepath.ToSlash(relative)
			}
		}
	}
	return result
}
