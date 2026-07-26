package repositoryservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
)

const canonicalIdentityDomain = "repository-service-artifact-id/v1\x00"

// CanonicalIdentityBytes freezes the artifact-ID preimage. It is the ASCII
// domain including its NUL terminator, followed by five fixed-order fields.
// Each field is a four-byte unsigned big-endian length and exact UTF-8 bytes.
func CanonicalIdentityBytes(input IdentityInput) ([]byte, error) {
	fields := []string{
		input.RepositoryID,
		input.ScanID,
		input.ArtifactName,
		input.ArtifactVersion,
		input.StableIDScheme,
	}
	result := make([]byte, 0, len(canonicalIdentityDomain)+128)
	result = append(result, canonicalIdentityDomain...)
	var length [4]byte
	for _, field := range fields {
		if err := validateIdentityField(field); err != nil {
			return nil, err
		}
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		result = append(result, length[:]...)
		result = append(result, field...)
	}
	return result, nil
}

// ArtifactID returns the lowercase SHA-256 identity with a scheme-specific prefix.
func ArtifactID(input IdentityInput) (string, error) {
	canonical, err := CanonicalIdentityBytes(input)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return identityPrefix + hex.EncodeToString(digest[:]), nil
}

func validateIdentityField(value string) error {
	if value == "" || strings.TrimSpace(value) != value || len(value) > 1024 || !utf8.ValidString(value) {
		return ErrInvalidIdentity
	}
	for _, current := range value {
		if current < 0x20 || current == 0x7f {
			return ErrInvalidIdentity
		}
	}
	return nil
}

// Materializer produces sealed exact-byte spools without an artifact-sized
// in-memory copy.
type Materializer struct {
	config Config
}

func NewMaterializer(configs ...Config) (*Materializer, error) {
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
	return &Materializer{config: config}, nil
}

// Materialize serializes once, measures while writing, scans for forbidden
// source values, then exposes a sealed reopenable stream.
func (materializer *Materializer) Materialize(ctx context.Context, identity IdentityInput, encode EncodeFunc, forbidden ...string) (*SealedArtifact, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if encode == nil {
		return nil, ErrEncodeRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	artifactID, err := ArtifactID(identity)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(materializer.config.SpoolDirectory, "aegis-repository-service-spike-*")
	if err != nil {
		return nil, fmt.Errorf("create bounded artifact spool: %w", err)
	}
	path := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if err = file.Chmod(0o600); err != nil {
		cleanup()
		return nil, fmt.Errorf("restrict artifact spool: %w", err)
	}
	digest := sha256.New()
	writer := &measuredWriter{ctx: ctx, destination: file, digest: digest, maximum: materializer.config.MaxArtifactBytes}
	if err = encode(ctx, writer); err != nil {
		cleanup()
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		cleanup()
		return nil, err
	}
	if err = file.Sync(); err != nil {
		cleanup()
		return nil, fmt.Errorf("sync artifact spool: %w", err)
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("seal artifact spool: %w", err)
	}
	for _, value := range normalizeForbidden(forbidden) {
		found, scanErr := fileContains(path, []byte(value), materializer.config.BufferBytes)
		if scanErr != nil {
			_ = os.Remove(path)
			return nil, fmt.Errorf("verify artifact redaction: %w", scanErr)
		}
		if found {
			_ = os.Remove(path)
			return nil, ErrForbiddenSourceValue
		}
	}
	return &SealedArtifact{
		descriptor: ArtifactDescriptor{
			ArtifactID: artifactID, Name: identity.ArtifactName,
			Version: identity.ArtifactVersion, StableIDScheme: identity.StableIDScheme,
			PayloadDigest: hex.EncodeToString(digest.Sum(nil)), PayloadSize: writer.size,
		},
		path: path, bufferSize: materializer.config.BufferBytes,
	}, nil
}

type measuredWriter struct {
	ctx         context.Context
	destination io.Writer
	digest      hash.Hash
	size        uint64
	maximum     uint64
}

func (writer *measuredWriter) Write(data []byte) (int, error) {
	if err := writer.ctx.Err(); err != nil {
		return 0, err
	}
	if uint64(len(data)) > writer.maximum-writer.size {
		return 0, ErrArtifactTooLarge
	}
	written, err := writer.destination.Write(data)
	if written > 0 {
		_, _ = writer.digest.Write(data[:written])
		writer.size += uint64(written)
	}
	return written, err
}

// VerifyAndCopy simulates Persistence Port's independent exact-byte stage proof.
func VerifyAndCopy(ctx context.Context, artifact *SealedArtifact, destination io.Writer) (ArtifactDescriptor, error) {
	if ctx == nil {
		return ArtifactDescriptor{}, ErrContextRequired
	}
	if artifact == nil || destination == nil {
		return ArtifactDescriptor{}, ErrArtifactIntegrity
	}
	reader, err := artifact.Open(ctx)
	if err != nil {
		return ArtifactDescriptor{}, err
	}
	defer reader.Close()
	digest := sha256.New()
	buffer := make([]byte, artifact.bufferSize)
	written, err := io.CopyBuffer(io.MultiWriter(destination, digest), &contextReader{ctx: ctx, reader: reader}, buffer)
	if err != nil {
		return ArtifactDescriptor{}, err
	}
	descriptor := artifact.Descriptor()
	if uint64(written) != descriptor.PayloadSize || hex.EncodeToString(digest.Sum(nil)) != descriptor.PayloadDigest {
		return ArtifactDescriptor{}, ErrArtifactIntegrity
	}
	return descriptor, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(data []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(data)
}

func normalizeForbidden(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values)*4)
	for _, value := range values {
		for _, candidate := range []string{
			value,
			filepath.ToSlash(value),
			filepath.FromSlash(value),
			strings.ReplaceAll(value, `\`, `\\`),
		} {
			if candidate == "" {
				continue
			}
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			result = append(result, candidate)
		}
	}
	return result
}

func fileContains(path string, pattern []byte, bufferSize int) (bool, error) {
	if len(pattern) == 0 {
		return false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	buffer := make([]byte, bufferSize)
	tail := make([]byte, 0, len(pattern)-1)
	for {
		count, readErr := file.Read(buffer)
		if count > 0 {
			window := make([]byte, 0, len(tail)+count)
			window = append(window, tail...)
			window = append(window, buffer[:count]...)
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

// DurableRIEReport removes per-run and deployment-local values from the RIE
// presentation contract before experimental durable serialization.
func DurableRIEReport(report rie.Report, root string) rie.Report {
	result := report
	result.Scan.ID = ""
	result.Scan.StartedAt = time.Time{}
	result.Scan.FinishedAt = time.Time{}
	result.Scan.DurationMilliseconds = 0
	result.Metrics = rie.Metrics{}
	result.Repository.RootPath = ""
	result.Metadata.RootPath = ""
	result.Warnings = sanitizeDiagnostics(result.Warnings, root)
	result.Errors = sanitizeDiagnostics(result.Errors, root)
	return result
}

func sanitizeDiagnostics(source []rie.Diagnostic, root string) []rie.Diagnostic {
	result := append([]rie.Diagnostic(nil), source...)
	variants := normalizeForbidden([]string{root})
	for index := range result {
		for _, value := range variants {
			result[index].Message = strings.ReplaceAll(result[index].Message, value, "<repository>")
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

// EncodeJSON provides the deterministic spike JSON codec candidate.
func EncodeJSON(value any) EncodeFunc {
	return func(ctx context.Context, destination io.Writer) error {
		if ctx == nil {
			return ErrContextRequired
		}
		encoder := json.NewEncoder(&contextWriter{ctx: ctx, writer: destination})
		encoder.SetEscapeHTML(false)
		return encoder.Encode(value)
	}
}

type contextWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (writer *contextWriter) Write(data []byte) (int, error) {
	if err := writer.ctx.Err(); err != nil {
		return 0, err
	}
	return writer.writer.Write(data)
}

// DetachedGoLanguageView produces an explicit JSON-safe syntax view.
func DetachedGoLanguageView(inventory goengine.GoLanguageInventory) GoLanguageInventoryView {
	return GoLanguageInventoryView{
		Artifact: inventory.Metadata(), SourceArtifacts: present(inventory.SourceArtifacts()),
		Files: present(inventory.Files()), Packages: present(inventory.Packages()),
		Symbols: present(inventory.Symbols()), Diagnostics: present(inventory.Diagnostics()),
		Statistics: inventory.Statistics(),
	}
}

func present[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

// RunReleasedProfile proves that the released engines compose without changing
// any released API. Every invocation owns a fresh artifact store.
func RunReleasedProfile(ctx context.Context, root string) (SpikeAnalysis, error) {
	if ctx == nil {
		return SpikeAnalysis{}, ErrContextRequired
	}
	run := rie.NewRunContext(root, rie.DefaultConfig())
	pipeline := rie.New()
	engines := []rie.Engine{
		discovery.New(), ignoreengine.New(), languageengine.New(), frameworkengine.New(),
		buildengine.New(), metadataengine.New(), summaryengine.New(),
	}
	for _, engine := range engines {
		if err := pipeline.Register(engine); err != nil {
			return SpikeAnalysis{}, err
		}
	}
	if err := pipeline.Run(ctx, run); err != nil {
		return SpikeAnalysis{}, err
	}
	languages, ok := rie.ArtifactAs[languageengine.LanguageInventory](run.Artifacts, languageengine.LanguageInventoryArtifactName)
	if !ok {
		return SpikeAnalysis{}, fmt.Errorf("language inventory unavailable after RIE")
	}
	goPresent := false
	for _, item := range languages.Items() {
		if item.Name == "Go" && item.Count > 0 {
			goPresent = true
			break
		}
	}
	result := SpikeAnalysis{Report: run.Report, GoPresent: goPresent}
	if !goPresent {
		return result, nil
	}
	goAnalyzer, err := goengine.New()
	if err != nil {
		return SpikeAnalysis{}, err
	}
	runner, err := lie.New(goAnalyzer)
	if err != nil {
		return SpikeAnalysis{}, err
	}
	if _, err = runner.Run(ctx, run.Artifacts); err != nil {
		return SpikeAnalysis{}, err
	}
	result.Syntax, ok = goengine.InventoryFrom(run.Artifacts)
	if !ok {
		return SpikeAnalysis{}, fmt.Errorf("Go syntax inventory unavailable")
	}
	identityEngine, err := packageidentity.New()
	if err != nil {
		return SpikeAnalysis{}, err
	}
	snapshot, ok := rie.ArtifactAs[rie.RepositorySnapshot](run.Artifacts, rie.RepositorySnapshotArtifactName)
	if !ok {
		return SpikeAnalysis{}, fmt.Errorf("repository snapshot unavailable")
	}
	result.PackageIdentities, err = identityEngine.Analyze(ctx, packageidentity.Input{Snapshot: snapshot, Syntax: result.Syntax})
	if err != nil {
		return SpikeAnalysis{}, err
	}
	if err = run.Artifacts.Put(result.PackageIdentities); err != nil {
		return SpikeAnalysis{}, err
	}
	integrator, err := semantic.NewIntegrator()
	if err != nil {
		return SpikeAnalysis{}, err
	}
	result.Semantics, err = integrator.Run(ctx, run.Artifacts)
	if err != nil {
		return SpikeAnalysis{}, err
	}
	return result, nil
}

// FlightGroup proves keyed, cancellation-aware in-process single-flight.
type FlightGroup[T any] struct {
	mu      sync.Mutex
	flights map[string]*flight[T]
}

type flight[T any] struct {
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	value     T
	err       error
	waiters   int
	completed bool
}

func (group *FlightGroup[T]) Do(ctx context.Context, key string, function func(context.Context) (T, error)) (T, FlightDisposition, error) {
	var zero T
	if ctx == nil {
		return zero, "", ErrContextRequired
	}
	if key == "" {
		return zero, "", ErrFlightKeyRequired
	}
	if function == nil {
		return zero, "", ErrFlightFuncRequired
	}
	if err := ctx.Err(); err != nil {
		return zero, "", err
	}
	group.mu.Lock()
	if group.flights == nil {
		group.flights = make(map[string]*flight[T])
	}
	current, exists := group.flights[key]
	disposition := FlightJoined
	if !exists {
		flightContext, cancel := context.WithCancel(context.Background())
		current = &flight[T]{ctx: flightContext, cancel: cancel, done: make(chan struct{})}
		group.flights[key] = current
		disposition = FlightCreated
		go group.execute(key, current, function)
	}
	current.waiters++
	group.mu.Unlock()

	select {
	case <-current.done:
		return current.value, disposition, current.err
	case <-ctx.Done():
		group.leave(key, current)
		return zero, disposition, ctx.Err()
	}
}

func (group *FlightGroup[T]) execute(key string, current *flight[T], function func(context.Context) (T, error)) {
	value, err := function(current.ctx)
	group.mu.Lock()
	current.value = value
	current.err = err
	current.completed = true
	if group.flights[key] == current {
		delete(group.flights, key)
	}
	close(current.done)
	current.cancel()
	group.mu.Unlock()
}

func (group *FlightGroup[T]) leave(key string, current *flight[T]) {
	group.mu.Lock()
	defer group.mu.Unlock()
	if current.completed {
		return
	}
	if current.waiters > 0 {
		current.waiters--
	}
	if current.waiters == 0 {
		if group.flights[key] == current {
			delete(group.flights, key)
		}
		current.cancel()
	}
}

// ReconcilePublication resolves a possibly committed publication without
// exposing the raw persistence error.
func ReconcilePublication(ctx context.Context, scanID string, publishErr error, reader PublicationStateReader) (PublicationOutcome, error) {
	if ctx == nil {
		return PublicationOutcome{}, ErrContextRequired
	}
	if publishErr == nil {
		return PublicationOutcome{Published: true}, nil
	}
	if reader == nil || scanID == "" {
		return PublicationOutcome{}, ErrPublicationAmbiguous
	}
	state, err := reader.ScanState(ctx, scanID)
	if err == nil && state == PublicationSucceeded {
		return PublicationOutcome{Published: true, Reconciled: true}, nil
	}
	return PublicationOutcome{}, ErrPublicationAmbiguous
}
