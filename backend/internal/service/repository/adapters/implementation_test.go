package adapters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/scan"
)

func TestReleasedRepositoryGoProfileMaterializesDeterministically(t *testing.T) {
	root := makeGoRepository(t)
	spool := t.TempDir()
	adapter, resolver, request := makeAdapter(t, root, spool, Config{})

	first, firstSession := analyze(t, adapter, request)
	firstBytes := readCandidates(t, first, root)
	if len(first.Candidates()) != 10 {
		t.Fatalf("candidate count = %d, want 10", len(first.Candidates()))
	}
	assertArtifactOrder(t, first.Candidates())
	assertDependencyGraph(t, first.Candidates())
	if err := firstSession.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertDirectoryEmpty(t, spool)

	second, secondSession := analyze(t, adapter, request)
	secondBytes := readCandidates(t, second, root)
	if len(firstBytes) != len(secondBytes) {
		t.Fatal("deterministic artifact count changed")
	}
	for name, left := range firstBytes {
		right, ok := secondBytes[name]
		if !ok || !bytes.Equal(left, right) {
			t.Fatalf("artifact %s is not deterministic", name)
		}
	}
	if err := secondSession.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if resolver.closeCount() != 2 {
		t.Fatalf("source close count = %d, want 2", resolver.closeCount())
	}
	assertDirectoryEmpty(t, spool)
}

func TestNonGoRepositoryProducesOnlyRIEArtifacts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# deterministic repository\n")
	adapter, _, request := makeAdapter(t, root, t.TempDir(), Config{})
	result, current := analyze(t, adapter, request)
	if len(result.Candidates()) != 7 {
		t.Fatalf("candidate count = %d, want 7", len(result.Candidates()))
	}
	for _, candidate := range result.Candidates() {
		if strings.HasPrefix(candidate.Name(), "go-") {
			t.Fatalf("unexpected Go artifact %s", candidate.Name())
		}
	}
	if err := current.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDurableBytesIgnoreDeploymentRootName(t *testing.T) {
	parent := t.TempDir()
	firstRoot, secondRoot := filepath.Join(parent, "windows-mount-name"), filepath.Join(parent, "linux-mount-name")
	populateGoRepository(t, firstRoot)
	populateGoRepository(t, secondRoot)
	firstAdapter, _, firstRequest := makeAdapter(t, firstRoot, t.TempDir(), Config{})
	secondAdapter, _, secondRequest := makeAdapter(t, secondRoot, t.TempDir(), Config{})
	first, firstSession := analyze(t, firstAdapter, firstRequest)
	second, secondSession := analyze(t, secondAdapter, secondRequest)
	left, right := readCandidates(t, first, firstRoot), readCandidates(t, second, secondRoot)
	names := make([]string, 0, len(left))
	for name := range left {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := left[name]
		if !bytes.Equal(value, right[name]) {
			t.Fatalf("deployment root changed %s", name)
		}
		t.Logf("%s bytes=%d sha256=%x", name, len(value), sha256.Sum256(value))
	}
	_ = firstSession.Close(context.Background())
	_ = secondSession.Close(context.Background())
}

func TestMaterializationLimitCancellationAndCleanup(t *testing.T) {
	root := makeGoRepository(t)
	adapter, resolver, request := makeAdapter(t, root, t.TempDir(), Config{MaxArtifactBytes: 32})
	sessionValue, err := adapter.Prepare(context.Background(), scan.NewAnalysisRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	_, err = sessionValue.Analyze(context.Background())
	if repository.KindOf(err) != repository.ErrorMaterializationFailed {
		t.Fatalf("kind = %s, want materialization_failed: %v", repository.KindOf(err), err)
	}
	if err = sessionValue.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if resolver.closeCount() != 1 {
		t.Fatal("source was not closed")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = adapter.Prepare(canceled, scan.NewAnalysisRequest(request))
	if repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled prepare kind = %s", repository.KindOf(err))
	}
}

func TestPrepareRejectsUnsupportedProfileAndInvalidSources(t *testing.T) {
	root := makeGoRepository(t)
	adapter, _, request := makeAdapter(t, root, t.TempDir(), Config{})
	otherArtifact, _ := repository.NewProfileArtifact("other", "1.0.0", repository.ArtifactIdentityScheme)
	otherDefinition, _ := repository.NewProfileDefinition("other", "1", []repository.ProfileArtifact{otherArtifact})
	contract, _ := repository.New()
	otherRequest, _ := contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: request.Scope(), RequestID: "other-request", RepositoryID: request.RepositoryID(), ScanID: "other-scan", SourceHandle: "source", Profile: otherDefinition.Profile()})
	_, err := adapter.Prepare(context.Background(), scan.NewAnalysisRequest(otherRequest))
	if repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("kind = %s", repository.KindOf(err))
	}

	badAdapter, _ := New(SourceResolverFunc(func(context.Context, repository.Scope, repository.SourceHandle) (AuthorizedSource, error) {
		return nil, nil
	}))
	_, err = badAdapter.Prepare(context.Background(), scan.NewAnalysisRequest(request))
	if repository.KindOf(err) != repository.ErrorSourceUnavailable {
		t.Fatalf("nil source kind = %s", repository.KindOf(err))
	}

	failingAdapter, _ := New(SourceResolverFunc(func(context.Context, repository.Scope, repository.SourceHandle) (AuthorizedSource, error) {
		return nil, errors.New("private path")
	}))
	_, err = failingAdapter.Prepare(context.Background(), scan.NewAnalysisRequest(request))
	if repository.KindOf(err) != repository.ErrorSourceUnavailable || strings.Contains(err.Error(), "private path") {
		t.Fatalf("unsafe source error: %v", err)
	}
}

func TestSessionIsSingleUseAndCloseIsIdempotent(t *testing.T) {
	root := makeGoRepository(t)
	adapter, resolver, request := makeAdapter(t, root, t.TempDir(), Config{})
	result, current := analyze(t, adapter, request)
	if len(result.Candidates()) == 0 {
		t.Fatal("missing candidates")
	}
	if _, err := current.Analyze(context.Background()); repository.KindOf(err) != repository.ErrorConflict {
		t.Fatalf("second analyze = %v", err)
	}
	if err := current.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := current.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if resolver.closeCount() != 1 {
		t.Fatalf("close count = %d", resolver.closeCount())
	}
	if _, err := result.Candidates()[0].Open(context.Background()); repository.KindOf(err) != repository.ErrorMaterializationFailed {
		t.Fatalf("closed payload error = %v", err)
	}
}

func TestPathRedactionScannerFindsBoundarySpanningPattern(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	if err := os.WriteFile(path, []byte("prefix-D:/private/repository-suffix"), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := fileContains(path, []byte("D:/private/repository"), 5)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	found, err = fileContains(path, []byte("not-present"), 5)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

func TestConfigurationAndConstructorValidation(t *testing.T) {
	if _, err := New(nil); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("nil resolver = %v", err)
	}
	resolver := SourceResolverFunc(func(context.Context, repository.Scope, repository.SourceHandle) (AuthorizedSource, error) {
		return nil, nil
	})
	if _, err := New(resolver, Config{}, Config{}); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("too many configs = %v", err)
	}
	invalid := []Config{
		{SpoolDirectory: "x", BufferBytes: 1, MaxArtifactBytes: 1, CleanupTimeout: time.Second, MaxArtifacts: 1},
		{SpoolDirectory: "x", BufferBytes: 4096, MaxArtifactBytes: maximumPayloadBytes + 1, CleanupTimeout: time.Second, MaxArtifacts: 1},
		{SpoolDirectory: "x", BufferBytes: 4096, MaxArtifactBytes: 1, CleanupTimeout: time.Millisecond, MaxArtifacts: 1},
		{SpoolDirectory: "x", BufferBytes: 4096, MaxArtifactBytes: 1, CleanupTimeout: time.Second, MaxArtifacts: 100},
	}
	for _, config := range invalid {
		if _, err := New(resolver, config); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("config accepted: %+v", config)
		}
	}
	valid, err := New(resolver, Config{})
	if err != nil || valid.config.BufferBytes != defaultBufferBytes {
		t.Fatalf("defaults = %+v err=%v", valid, err)
	}
}

func TestSessionAccessorsClosedAndInvalidBranches(t *testing.T) {
	root := makeGoRepository(t)
	adapter, resolver, request := makeAdapter(t, root, t.TempDir(), Config{})
	currentValue, err := adapter.Prepare(context.Background(), scan.NewAnalysisRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	current := currentValue.(*session)
	if current.SourceFingerprint().IsZero() || current.SourceRevision() != "revision-1" {
		t.Fatal("source evidence unavailable")
	}
	if err = current.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = current.Analyze(context.Background()); repository.KindOf(err) != repository.ErrorConflict {
		t.Fatalf("closed analyze = %v", err)
	}
	if resolver.closeCount() != 1 {
		t.Fatal("source not closed")
	}
	var nilSession *session
	if _, err = nilSession.Analyze(context.Background()); repository.KindOf(err) != repository.ErrorAnalysisFailed {
		t.Fatalf("nil analyze = %v", err)
	}
	if err = nilSession.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCodecContractAndArtifactCountFailuresCleanSpools(t *testing.T) {
	root := makeGoRepository(t)
	spool := t.TempDir()
	adapter, _, request := makeAdapter(t, root, spool, Config{})
	broken := adapter.specs["repository-snapshot"]
	broken.version = "2.0.0"
	adapter.specs["repository-snapshot"] = broken
	current, err := adapter.Prepare(context.Background(), scan.NewAnalysisRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	_, err = current.Analyze(context.Background())
	if repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("codec mismatch = %v", err)
	}
	_ = current.Close(context.Background())
	assertDirectoryEmpty(t, spool)

	limited, _, limitedRequest := makeAdapter(t, root, spool, Config{MaxArtifacts: 1})
	limitedSession, err := limited.Prepare(context.Background(), scan.NewAnalysisRequest(limitedRequest))
	if err != nil {
		t.Fatal(err)
	}
	_, err = limitedSession.Analyze(context.Background())
	if repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("artifact limit = %v", err)
	}
	_ = limitedSession.Close(context.Background())
	assertDirectoryEmpty(t, spool)
}

func TestInvalidRootAndInvalidSourceEvidence(t *testing.T) {
	validRoot := makeGoRepository(t)
	missing := filepath.Join(t.TempDir(), "missing")
	adapter, _, request := makeAdapter(t, missing, t.TempDir(), Config{})
	current, err := adapter.Prepare(context.Background(), scan.NewAnalysisRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	_, err = current.Analyze(context.Background())
	if repository.KindOf(err) != repository.ErrorAnalysisFailed {
		t.Fatalf("missing root = %v", err)
	}
	_ = current.Close(context.Background())

	zeroAdapter, _ := New(SourceResolverFunc(func(context.Context, repository.Scope, repository.SourceHandle) (AuthorizedSource, error) {
		return &staticSource{root: missing}, nil
	}))
	_, err = zeroAdapter.Prepare(context.Background(), scan.NewAnalysisRequest(request))
	if repository.KindOf(err) != repository.ErrorSourceUnavailable {
		t.Fatalf("zero proof = %v", err)
	}
	unsafeAdapter, _ := New(SourceResolverFunc(func(context.Context, repository.Scope, repository.SourceHandle) (AuthorizedSource, error) {
		return &staticSource{root: validRoot, digest: sha256.Sum256([]byte("proof")), revision: "revision:" + validRoot}, nil
	}))
	_, err = unsafeAdapter.Prepare(context.Background(), scan.NewAnalysisRequest(request))
	if repository.KindOf(err) != repository.ErrorSourceUnavailable {
		t.Fatalf("unsafe revision = %v", err)
	}
	unsafeSpool, _, unsafeSpoolRequest := makeAdapter(t, validRoot, validRoot, Config{})
	_, err = unsafeSpool.Prepare(context.Background(), scan.NewAnalysisRequest(unsafeSpoolRequest))
	if repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("unsafe spool = %v", err)
	}
}

func TestBoundedWriterJSONAndRedactionFailurePaths(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	writer := &boundedHashWriter{ctx: canceled, writer: io.Discard, hash: sha256.New(), maximum: 100}
	if _, err := writer.Write([]byte("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled writer = %v", err)
	}
	writer = &boundedHashWriter{ctx: context.Background(), writer: io.Discard, hash: sha256.New(), maximum: 1}
	if _, err := writer.Write([]byte("xx")); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("limit writer = %v", err)
	}
	writer = &boundedHashWriter{ctx: context.Background(), writer: errorWriter{}, hash: sha256.New(), maximum: 10}
	if _, err := writer.Write([]byte("x")); err == nil {
		t.Fatal("underlying writer failure accepted")
	}
	if err := jsonValue(make(chan int))(context.Background(), io.Discard); err == nil {
		t.Fatal("unsupported JSON accepted")
	}
	if err := jsonArray([]int{1})(canceled, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("array cancellation = %v", err)
	}
	if err := writeObject(context.Background(), errorWriter{}, jsonField{"x", jsonValue(1)}); err == nil {
		t.Fatal("object writer failure accepted")
	}

	root := t.TempDir()
	inside := filepath.Join(root, "pkg", "file.go")
	outside := filepath.Join(filepath.Dir(root), "outside.go")
	values := sanitizeRIEDiagnostics([]rie.Diagnostic{{Message: "failure at " + root, Path: inside}, {Message: "outside", Path: outside}}, root)
	if strings.Contains(values[0].Message, root) || values[0].Path != "pkg/file.go" || values[1].Path != "" {
		t.Fatalf("redaction failed: %+v", values)
	}
}

func TestSealedPayloadFailureBranches(t *testing.T) {
	var nilPayload *sealedPayload
	if _, err := nilPayload.Open(context.Background()); repository.KindOf(err) != repository.ErrorMaterializationFailed {
		t.Fatalf("nil open = %v", err)
	}
	if err := nilPayload.close(); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	payload := &sealedPayload{path: filepath.Join(t.TempDir(), "missing")}
	if _, err := payload.Open(canceled); repository.KindOf(err) != repository.ErrorCanceled {
		t.Fatalf("canceled open = %v", err)
	}
	if _, err := payload.Open(context.Background()); repository.KindOf(err) != repository.ErrorMaterializationFailed {
		t.Fatalf("missing open = %v", err)
	}
	if err := payload.close(); err != nil {
		t.Fatal(err)
	}
	if err := payload.close(); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactCandidateDependenciesAreDefensive(t *testing.T) {
	dependency, _ := scan.NewArtifactDependency("repository-snapshot", "1.0.0", 0)
	payload := &memoryPayload{data: []byte("{}")}
	candidate, err := scan.NewArtifactCandidate(scan.ArtifactCandidateParams{Name: "language-inventory", Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: codecName, CodecVersion: codecVersion, MediaType: "application/json", PayloadDigest: sha256.Sum256(payload.data), PayloadSize: 2, ProducerName: "language", ProducerVersion: "1.0.0", Dependencies: []scan.ArtifactDependency{dependency}, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	values := candidate.Dependencies()
	values[0], _ = scan.NewArtifactDependency("other", "1.0.0", 0)
	if candidate.Dependencies()[0].Name() != "repository-snapshot" {
		t.Fatal("dependencies are not detached")
	}
	_, err = scan.NewAnalysisResult(repository.DefaultRepositoryGoProfile().Profile(), []scan.ArtifactCandidate{candidate})
	if repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("missing dependency error = %v", err)
	}
}

func TestAnalysisResultRejectsSelfAndCyclicDependencies(t *testing.T) {
	profile := repository.DefaultRepositoryGoProfile().Profile()
	self, _ := scan.NewArtifactDependency("first", "1.0.0", 0)
	first := testCandidate(t, "first", []scan.ArtifactDependency{self})
	if _, err := scan.NewAnalysisResult(profile, []scan.ArtifactCandidate{first}); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("self dependency = %v", err)
	}
	firstToSecond, _ := scan.NewArtifactDependency("second", "1.0.0", 0)
	secondToFirst, _ := scan.NewArtifactDependency("first", "1.0.0", 0)
	first = testCandidate(t, "first", []scan.ArtifactDependency{firstToSecond})
	second := testCandidate(t, "second", []scan.ArtifactDependency{secondToFirst})
	if _, err := scan.NewAnalysisResult(profile, []scan.ArtifactCandidate{first, second}); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("cycle = %v", err)
	}
}

func FuzzForbiddenVariantsNeverPanic(f *testing.F) {
	f.Add(`D:\Projects\ERP`)
	f.Add("/srv/repositories/project")
	f.Add("")
	f.Fuzz(func(t *testing.T, value string) { _ = forbiddenVariants(value) })
}

func analyze(t *testing.T, adapter *Adapter, request repository.ExecuteScanRequest) (scan.AnalysisResult, scan.AnalysisSession) {
	t.Helper()
	current, err := adapter.Prepare(context.Background(), scan.NewAnalysisRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	result, err := current.Analyze(context.Background())
	if err != nil {
		_ = current.Close(context.Background())
		t.Fatal(err)
	}
	return result, current
}

func readCandidates(t *testing.T, result scan.AnalysisResult, root string) map[string][]byte {
	t.Helper()
	output := make(map[string][]byte)
	variants := forbiddenVariants(root)
	for _, candidate := range result.Candidates() {
		reader, err := candidate.Open(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if uint64(len(data)) != candidate.PayloadSize() || sha256.Sum256(data) != candidate.PayloadDigest() {
			t.Fatalf("integrity mismatch for %s", candidate.Name())
		}
		if !json.Valid(data) {
			t.Fatalf("invalid JSON for %s", candidate.Name())
		}
		for _, variant := range variants {
			if variant != "" && bytes.Contains(data, []byte(variant)) {
				t.Fatalf("source path leaked in %s", candidate.Name())
			}
		}
		output[candidate.Name()] = data
	}
	return output
}

func assertArtifactOrder(t *testing.T, candidates []scan.ArtifactCandidate) {
	t.Helper()
	names := make([]string, len(candidates))
	for i, item := range candidates {
		names[i] = item.Name()
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("artifacts not sorted: %v", names)
	}
}

func assertDependencyGraph(t *testing.T, candidates []scan.ArtifactCandidate) {
	t.Helper()
	present := map[string]bool{}
	for _, item := range candidates {
		present[item.Name()] = true
	}
	for _, item := range candidates {
		for ordinal, dependency := range item.Dependencies() {
			if dependency.Ordinal() != ordinal || !present[dependency.Name()] {
				t.Fatalf("invalid dependency %s -> %s", item.Name(), dependency.Name())
			}
		}
	}
}

func testCandidate(t *testing.T, name string, dependencies []scan.ArtifactDependency) scan.ArtifactCandidate {
	t.Helper()
	payload := &memoryPayload{data: []byte("{}")}
	value, err := scan.NewArtifactCandidate(scan.ArtifactCandidateParams{Name: name, Version: "1.0.0", StableIDScheme: repository.ArtifactIdentityScheme, CodecName: codecName, CodecVersion: codecVersion, MediaType: "application/json", PayloadDigest: sha256.Sum256(payload.data), PayloadSize: 2, ProducerName: "test", ProducerVersion: "1.0.0", Dependencies: dependencies, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func makeAdapter(t *testing.T, root, spool string, override Config) (*Adapter, *fakeResolver, repository.ExecuteScanRequest) {
	t.Helper()
	resolver := &fakeResolver{root: root, fingerprint: sha256.Sum256([]byte("source-proof")), revision: "revision-1"}
	config := override
	config.SpoolDirectory = spool
	adapter, err := New(resolver, config)
	if err != nil {
		t.Fatal(err)
	}
	contract, _ := repository.New()
	scope, _ := repository.NewScope("tenant", "principal")
	request, err := contract.NewExecuteScanRequest(repository.ExecuteScanParams{Scope: scope, RequestID: "request", RepositoryID: "repository", ScanID: "scan", SourceHandle: "source", Profile: repository.DefaultRepositoryGoProfile().Profile()})
	if err != nil {
		t.Fatal(err)
	}
	return adapter, resolver, request
}

func makeGoRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	populateGoRepository(t, root)
	return root
}

func populateGoRepository(t *testing.T, root string) {
	t.Helper()
	writeFile(t, root, "go.mod", "module example.com/service\n\ngo 1.26\n")
	writeFile(t, root, "main.go", "package main\n\ntype Runner interface { Run() }\ntype Worker struct{}\nfunc (Worker) Run() {}\nvar _ Runner = Worker{}\nfunc main() { var r Runner = Worker{}; r.Run() }\n")
	writeFile(t, root, "README.md", "# service\n")
}

func writeFile(t *testing.T, root, relative, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("spool contains %d files", len(entries))
	}
}

type fakeResolver struct {
	root        string
	fingerprint repository.Digest
	revision    string
	mu          sync.Mutex
	closes      int
}

func (resolver *fakeResolver) Resolve(context.Context, repository.Scope, repository.SourceHandle) (AuthorizedSource, error) {
	return &fakeSource{resolver: resolver}, nil
}
func (resolver *fakeResolver) closeCount() int {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	return resolver.closes
}

type fakeSource struct {
	resolver *fakeResolver
	once     sync.Once
}

func (source *fakeSource) RootPath() string               { return source.resolver.root }
func (source *fakeSource) Fingerprint() repository.Digest { return source.resolver.fingerprint }
func (source *fakeSource) Revision() string               { return source.resolver.revision }
func (source *fakeSource) Close(context.Context) error {
	source.once.Do(func() { source.resolver.mu.Lock(); source.resolver.closes++; source.resolver.mu.Unlock() })
	return nil
}

type SourceResolverFunc func(context.Context, repository.Scope, repository.SourceHandle) (AuthorizedSource, error)

func (function SourceResolverFunc) Resolve(ctx context.Context, scope repository.Scope, source repository.SourceHandle) (AuthorizedSource, error) {
	return function(ctx, scope, source)
}

type memoryPayload struct{ data []byte }

func (payload *memoryPayload) Open(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(payload.data)), nil
}

type staticSource struct {
	root, revision string
	digest         repository.Digest
}

func (source *staticSource) RootPath() string               { return source.root }
func (source *staticSource) Fingerprint() repository.Digest { return source.digest }
func (source *staticSource) Revision() string               { return source.revision }
func (source *staticSource) Close(context.Context) error    { return nil }

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("private writer failure") }
