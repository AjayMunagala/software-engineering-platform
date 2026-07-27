package lifecycle

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository/conformance"
)

func TestLifecycleConformance(t *testing.T) {
	conformance.RunLifecycle(t, conformance.LifecycleFactoryFunc(openLifecycleFixture))
}

func TestRegisterResolvesClosesAndPersistsOnlyProof(t *testing.T) {
	fixture, cleanup, err := openLifecycleFixture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup(context.Background()) }()
	service := fixture.Service.(*Service)
	resolver := service.resolver.(*fakeResolver)
	resolvedBefore, closedBefore := resolver.resolveCount.Load(), resolver.closeCount.Load()
	contract := fixture.Contract
	request, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: fixture.Scenario.PrimaryScope, RequestID: "proof-register", RepositoryID: "proof-repository", DisplayName: "Proof Repository", SourceHandle: "sensitive-local-handle"})
	value, err := service.RegisterRepository(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if value.SourceKind() != "local" || value.FingerprintScheme() != "sha256/v1" || value.Fingerprint() != resolver.proof.Fingerprint() {
		t.Fatalf("repository=%+v", value)
	}
	if resolver.resolveCount.Load()-resolvedBefore != 1 || resolver.closeCount.Load()-closedBefore != 1 {
		t.Fatalf("resolve=%d close=%d", resolver.resolveCount.Load(), resolver.closeCount.Load())
	}
	if formatted := value.DisplayName() + value.SourceKind() + value.FingerprintScheme(); strings.Contains(formatted, "sensitive-local-handle") {
		t.Fatal("durable repository leaked source handle")
	}
}

func TestLifecycleDependencyFailuresAreSafe(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	contract, _ := repository.New()
	scope, _ := repository.NewScope("scope", "principal")
	proof, _ := NewSourceProof("local", "sha256/v1", repository.DigestBytes([]byte("proof")), "revision")
	request, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: "request", RepositoryID: "repository", DisplayName: "Repository", SourceHandle: "handle"})

	resolver := &fakeResolver{proof: proof, resolveErr: errors.New("path=C:\\secret password=value")}
	service, _ := New(newMemoryStore(), resolver, ClockFunc(func() time.Time { return now }))
	_, err := service.RegisterRepository(context.Background(), request)
	if repository.KindOf(err) != repository.ErrorSourceUnavailable || strings.Contains(err.Error(), "secret") {
		t.Fatalf("resolver error=%v", err)
	}
	resolver = &fakeResolver{proof: proof, closeErr: errors.New("secret cleanup")}
	service, _ = New(newMemoryStore(), resolver, ClockFunc(func() time.Time { return now }))
	_, err = service.RegisterRepository(context.Background(), request)
	if repository.KindOf(err) != repository.ErrorSourceUnavailable || resolver.closeCount.Load() != 1 {
		t.Fatalf("cleanup error=%v closes=%d", err, resolver.closeCount.Load())
	}
	store := newMemoryStore()
	store.failure = errors.New("SQLSTATE 08006 password=secret")
	service, _ = New(store, &fakeResolver{proof: proof}, ClockFunc(func() time.Time { return now }))
	_, err = service.RegisterRepository(context.Background(), request)
	if repository.KindOf(err) != repository.ErrorInternal || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "SQLSTATE") {
		t.Fatalf("store error=%v", err)
	}
}

func TestLifecycleValidationAndIntegrity(t *testing.T) {
	store := newMemoryStore()
	proof, _ := NewSourceProof("local", "sha256/v1", repository.DigestBytes([]byte("proof")), "revision")
	resolver := &fakeResolver{proof: proof}
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	clock := ClockFunc(func() time.Time { return now })
	for index, candidate := range []func() error{
		func() error { _, err := New(nil, resolver, clock); return err },
		func() error { _, err := New(store, nil, clock); return err },
		func() error { _, err := New(store, resolver, nil); return err },
		func() error { _, err := New(store, resolver, clock, DefaultConfig(), DefaultConfig()); return err },
		func() error {
			_, err := New(store, resolver, clock, Config{SourceCloseTimeout: time.Millisecond})
			return err
		},
		func() error {
			_, err := NewSourceProof("Bad", "sha256/v1", repository.DigestBytes([]byte("x")), "")
			return err
		},
		func() error { _, err := NewRepositoryList([]repository.Repository{{}}, ""); return err },
	} {
		if err := candidate(); err == nil {
			t.Fatalf("invalid candidate %d accepted", index)
		}
	}
	service, _ := New(store, resolver, clock)
	if _, err := service.GetRepository(nil, repository.RepositoryQuery{}); repository.KindOf(err) != repository.ErrorInvalidInput {
		t.Fatalf("nil context=%v", err)
	}
	contract, _ := repository.New()
	scope, _ := repository.NewScope("scope", "principal")
	request, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: "request", RepositoryID: "repo", DisplayName: "Repo", SourceHandle: "handle"})
	store.overrideResult = true
	store.resultOverride = repository.Repository{}
	if _, err := service.RegisterRepository(context.Background(), request); repository.KindOf(err) != repository.ErrorIntegrityFailure {
		t.Fatalf("mismatched result=%v", err)
	}
}

func TestMutationFingerprintsAndDetachedList(t *testing.T) {
	contract, _ := repository.New()
	scope, _ := repository.NewScope("scope", "principal")
	proof, _ := NewSourceProof("local", "sha256/v1", repository.DigestBytes([]byte("proof")), "revision")
	request, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: "request", RepositoryID: "repo", DisplayName: "Repo", SourceHandle: "handle"})
	first := registerFingerprint(request, proof)
	second := registerFingerprint(request, proof)
	changedProof, _ := NewSourceProof("local", "sha256/v1", repository.DigestBytes([]byte("changed")), "revision")
	if first != second || first == registerFingerprint(request, changedProof) {
		t.Fatal("registration fingerprint is not deterministic/sensitive")
	}
	archive, _ := repository.NewArchiveRepositoryRequest(repository.ArchiveRepositoryParams{Scope: scope, RequestID: "archive", RepositoryID: "repo"})
	if archiveFingerprint(archive) != archiveFingerprint(archive) {
		t.Fatal("archive fingerprint is not deterministic")
	}
	now := time.Now().UTC()
	value, _ := repository.NewRepository(repository.RepositoryParams{RepositoryID: "repo", DisplayName: "Repo", SourceKind: "local", FingerprintScheme: "sha256/v1", Fingerprint: proof.Fingerprint(), State: repository.RepositoryActive, CreatedAt: now, UpdatedAt: now})
	list, _ := NewRepositoryList([]repository.Repository{value}, "next")
	items := list.Items()
	items[0] = repository.Repository{}
	if list.Items()[0].RepositoryID() != "repo" || list.NextCursor() != "next" {
		t.Fatal("repository list was mutable")
	}
	command := newRegisterCommand(scope, "request", first, value)
	if command.Scope() != scope || command.RequestID() != "request" || command.MutationFingerprint() != first || command.Repository().RepositoryID() != "repo" {
		t.Fatal("register command accessors failed")
	}
	archiveCommand := newArchiveCommand(scope, "archive", "repo", archiveFingerprint(archive), now)
	if archiveCommand.Scope() != scope || archiveCommand.RequestID() != "archive" || archiveCommand.RepositoryID() != "repo" || archiveCommand.At() != now || archiveCommand.MutationFingerprint().IsZero() {
		t.Fatal("archive command accessors failed")
	}
}

func TestConcurrentIdempotencyPaginationAndConflicts(t *testing.T) {
	fixture, cleanup, err := openLifecycleFixture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup(context.Background()) }()
	service := fixture.Service
	contract := fixture.Contract
	scope := fixture.Scenario.PrimaryScope
	request, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: "concurrent-register", RepositoryID: "concurrent-repository", DisplayName: "Concurrent", SourceHandle: fixture.Scenario.SourceHandle})
	results := make(chan repository.Repository, 100)
	errorsFound := make(chan error, 100)
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value, callErr := service.RegisterRepository(context.Background(), request)
			if callErr != nil {
				errorsFound <- callErr
				return
			}
			results <- value
		}()
	}
	wait.Wait()
	close(results)
	close(errorsFound)
	if len(errorsFound) != 0 || len(results) != 100 {
		t.Fatalf("errors=%d results=%d", len(errorsFound), len(results))
	}
	var createdAt time.Time
	for value := range results {
		if createdAt.IsZero() {
			createdAt = value.CreatedAt()
		}
		if value.RepositoryID() != "concurrent-repository" || value.CreatedAt() != createdAt {
			t.Fatalf("non-idempotent result=%+v", value)
		}
	}
	duplicate, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: "different-request", RepositoryID: "concurrent-repository", DisplayName: "Concurrent", SourceHandle: fixture.Scenario.SourceHandle})
	if _, err = service.RegisterRepository(context.Background(), duplicate); repository.KindOf(err) != repository.ErrorConflict {
		t.Fatalf("duplicate repository=%v", err)
	}
	for index := range 3 {
		id := repository.RepositoryID("page-" + strconv.Itoa(index))
		item, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: scope, RequestID: repository.RequestID("page-request-" + strconv.Itoa(index)), RepositoryID: id, DisplayName: "Page", SourceHandle: fixture.Scenario.SourceHandle})
		if _, err = service.RegisterRepository(context.Background(), item); err != nil {
			t.Fatal(err)
		}
	}
	list, _ := contract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: scope, PageSize: 2})
	first, err := service.ListRepositories(context.Background(), list)
	if err != nil || len(first.Items()) != 2 || first.NextCursor() == "" {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	next, _ := contract.NewRepositoryListRequest(repository.RepositoryListParams{Scope: scope, PageSize: 2, Cursor: first.NextCursor()})
	second, err := service.ListRepositories(context.Background(), next)
	if err != nil || len(second.Items()) != 2 {
		t.Fatalf("second page=%+v err=%v", second, err)
	}
}

func FuzzSourceProofNeverPanics(f *testing.F) {
	f.Add("local", "sha256/v1", "revision")
	f.Add("", "", "")
	f.Fuzz(func(t *testing.T, kind, scheme, revision string) {
		proof, err := NewSourceProof(kind, scheme, repository.DigestBytes([]byte("proof")), revision)
		if err == nil && proof.IsZero() {
			t.Fatal("accepted source proof is zero")
		}
	})
}

type fakeResolver struct {
	proof        SourceProof
	resolveErr   error
	closeErr     error
	resolveCount atomic.Int64
	closeCount   atomic.Int64
}

func (resolver *fakeResolver) Resolve(ctx context.Context, _ repository.Scope, _ repository.SourceHandle) (SourceResolution, error) {
	resolver.resolveCount.Add(1)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if resolver.resolveErr != nil {
		return nil, resolver.resolveErr
	}
	return &fakeResolution{proof: resolver.proof, err: resolver.closeErr, count: &resolver.closeCount}, nil
}

type fakeResolution struct {
	proof SourceProof
	err   error
	count *atomic.Int64
}

func (resolution *fakeResolution) Proof() SourceProof { return resolution.proof }
func (resolution *fakeResolution) Close(context.Context) error {
	resolution.count.Add(1)
	return resolution.err
}

type memoryRequest struct {
	fingerprint repository.Digest
	result      repository.Repository
}

type memoryStore struct {
	mu             sync.RWMutex
	repositories   map[string]repository.Repository
	requests       map[string]memoryRequest
	failure        error
	overrideResult bool
	resultOverride repository.Repository
}

func newMemoryStore() *memoryStore {
	return &memoryStore{repositories: map[string]repository.Repository{}, requests: map[string]memoryRequest{}}
}

func (store *memoryStore) Register(ctx context.Context, command RegisterCommand) (repository.Repository, error) {
	if err := ctx.Err(); err != nil {
		return repository.Repository{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.failure != nil {
		return repository.Repository{}, store.failure
	}
	requestKey := lifecycleRequestKey(command.Scope(), command.RequestID())
	if prior, ok := store.requests[requestKey]; ok {
		if prior.fingerprint != command.MutationFingerprint() {
			return repository.Repository{}, repository.NewError(repository.ErrorIdempotencyConflict, "register-repository", "request-reused", false, nil)
		}
		return prior.result, nil
	}
	key := lifecycleRepositoryKey(command.Scope(), command.Repository().RepositoryID())
	if _, exists := store.repositories[key]; exists {
		return repository.Repository{}, repository.NewError(repository.ErrorConflict, "register-repository", "repository-exists", false, nil)
	}
	value := command.Repository()
	if store.overrideResult {
		return store.resultOverride, nil
	}
	store.repositories[key] = value
	store.requests[requestKey] = memoryRequest{fingerprint: command.MutationFingerprint(), result: value}
	return value, nil
}

func (store *memoryStore) Get(ctx context.Context, scope repository.Scope, repositoryID repository.RepositoryID) (repository.Repository, error) {
	if err := ctx.Err(); err != nil {
		return repository.Repository{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.failure != nil {
		return repository.Repository{}, store.failure
	}
	value, ok := store.repositories[lifecycleRepositoryKey(scope, repositoryID)]
	if !ok {
		return repository.Repository{}, repository.NewError(repository.ErrorNotFound, "get-repository", "not-found", false, nil)
	}
	return value, nil
}

func (store *memoryStore) List(ctx context.Context, scope repository.Scope, size int, cursor repository.Cursor) (RepositoryList, error) {
	if err := ctx.Err(); err != nil {
		return RepositoryList{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	values := []repository.Repository{}
	prefix := string(scope.ScopeID()) + "|"
	for key, value := range store.repositories {
		if strings.HasPrefix(key, prefix) {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].RepositoryID() < values[j].RepositoryID() })
	start := 0
	if cursor != "" {
		start, _ = strconv.Atoi(string(cursor))
		if start < 0 || start > len(values) {
			start = len(values)
		}
	}
	end := min(start+size, len(values))
	next := repository.Cursor("")
	if end < len(values) {
		next = repository.Cursor(strconv.Itoa(end))
	}
	return NewRepositoryList(values[start:end], next)
}

func (store *memoryStore) Archive(ctx context.Context, command ArchiveCommand) (repository.Repository, error) {
	if err := ctx.Err(); err != nil {
		return repository.Repository{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	requestKey := lifecycleRequestKey(command.Scope(), command.RequestID())
	if prior, ok := store.requests[requestKey]; ok {
		if prior.fingerprint != command.MutationFingerprint() {
			return repository.Repository{}, repository.NewError(repository.ErrorIdempotencyConflict, "archive-repository", "request-reused", false, nil)
		}
		return prior.result, nil
	}
	key := lifecycleRepositoryKey(command.Scope(), command.RepositoryID())
	current, ok := store.repositories[key]
	if !ok {
		return repository.Repository{}, repository.NewError(repository.ErrorNotFound, "archive-repository", "not-found", false, nil)
	}
	value := current
	if current.State() != repository.RepositoryArchived {
		value, _ = repository.NewRepository(repository.RepositoryParams{RepositoryID: current.RepositoryID(), DisplayName: current.DisplayName(), SourceKind: current.SourceKind(), FingerprintScheme: current.FingerprintScheme(), Fingerprint: current.Fingerprint(), State: repository.RepositoryArchived, CurrentScanID: current.CurrentScanID(), CreatedAt: current.CreatedAt(), UpdatedAt: command.At()})
		store.repositories[key] = value
	}
	store.requests[requestKey] = memoryRequest{fingerprint: command.MutationFingerprint(), result: value}
	return value, nil
}

func lifecycleRepositoryKey(scope repository.Scope, id repository.RepositoryID) string {
	return string(scope.ScopeID()) + "|" + string(id)
}
func lifecycleRequestKey(scope repository.Scope, id repository.RequestID) string {
	return string(scope.ScopeID()) + "|" + string(id)
}

func openLifecycleFixture(ctx context.Context) (conformance.LifecycleFixture, conformance.Cleanup, error) {
	contract, _ := repository.New()
	primary, _ := repository.NewScope("lifecycle-primary", "principal-primary")
	other, _ := repository.NewScope("lifecycle-other", "principal-other")
	proof, _ := NewSourceProof("local", "sha256/v1", repository.DigestBytes([]byte("source-proof")), "revision-1")
	resolver := &fakeResolver{proof: proof}
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	service, err := New(store, resolver, ClockFunc(func() time.Time { return now }))
	if err != nil {
		return conformance.LifecycleFixture{}, nil, err
	}
	request, _ := contract.NewRegisterRepositoryRequest(repository.RegisterRepositoryParams{Scope: primary, RequestID: "seed-register", RepositoryID: "repository-seeded", DisplayName: "Seeded Repository", SourceHandle: "seed-source"})
	seeded, err := service.RegisterRepository(ctx, request)
	if err != nil {
		return conformance.LifecycleFixture{}, nil, err
	}
	var once sync.Once
	cleanup := func(context.Context) error {
		once.Do(func() {
			store.mu.Lock()
			defer store.mu.Unlock()
			clear(store.repositories)
			clear(store.requests)
		})
		return nil
	}
	return conformance.LifecycleFixture{Service: service, Contract: contract, Scenario: conformance.LifecycleScenario{PrimaryScope: primary, OtherScope: other, Repository: seeded, SourceHandle: "lifecycle-source"}}, cleanup, nil
}
