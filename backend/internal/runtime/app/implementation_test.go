package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
	runtimeobservability "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/observability"
	runtimepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
)

type testSecretProvider struct{}

func (testSecretProvider) Resolve(ctx context.Context, _ runtimeconfig.SecretReference) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []byte("disposable-test-value"), nil
}

type fakePostgreSQL struct {
	checkErr   atomic.Value
	checks     atomic.Int64
	closes     atomic.Int64
	closeBlock <-chan struct{}
}

func (runtime *fakePostgreSQL) Check(ctx context.Context) error {
	runtime.checks.Add(1)
	if err := ctx.Err(); err != nil {
		return err
	}
	if value := runtime.checkErr.Load(); value != nil {
		return value.(error)
	}
	return nil
}

func (*fakePostgreSQL) Ingest() runtimepostgres.IngestCapabilities       { return nil }
func (*fakePostgreSQL) Read() runtimepostgres.ReadCapabilities           { return nil }
func (*fakePostgreSQL) Retention() runtimepostgres.RetentionCapabilities { return nil }
func (runtime *fakePostgreSQL) Close() {
	runtime.closes.Add(1)
	if runtime.closeBlock != nil {
		<-runtime.closeBlock
	}
}

type fakeOpener struct {
	runtime PostgreSQLRuntime
	err     error
	wait    bool
}

type fakeObservabilityFactory struct {
	service runtimeobservability.Service
	err     error
}

func (factory fakeObservabilityFactory) Open(runtimeconfig.RuntimeConfig) (runtimeobservability.Service, error) {
	return factory.service, factory.err
}

type fakeObservability struct {
	mutex      sync.Mutex
	events     []runtimeobservability.EventParams
	starts     int
	stops      int
	closes     int
	source     runtimeobservability.Source
	startError error
}

func (value *fakeObservability) Event(_ context.Context, event runtimeobservability.EventParams) error {
	value.mutex.Lock()
	value.events = append(value.events, event)
	value.mutex.Unlock()
	return nil
}

func (value *fakeObservability) Start(_ context.Context, source runtimeobservability.Source) error {
	value.mutex.Lock()
	defer value.mutex.Unlock()
	value.starts++
	value.source = source
	return value.startError
}

func (value *fakeObservability) StopCollection(context.Context) error {
	value.mutex.Lock()
	value.stops++
	value.mutex.Unlock()
	return nil
}
func (value *fakeObservability) Close(context.Context) error {
	value.mutex.Lock()
	value.closes++
	value.mutex.Unlock()
	return nil
}
func (*fakeObservability) Statistics() runtimeobservability.Statistics {
	return runtimeobservability.Statistics{}
}

func (opener fakeOpener) Open(ctx context.Context, _ runtimeconfig.LoadedConfiguration) (PostgreSQLRuntime, error) {
	if opener.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return opener.runtime, opener.err
}

func TestStartupPublishesReadyAndZeroWorkShutdown(t *testing.T) {
	postgres := &fakePostgreSQL{}
	runtime := startTestRuntime(t, postgres)
	if runtime.State() != runtimehealth.StateReady || runtime.Liveness(context.Background()).Liveness().Status() != runtimehealth.StatusHealthy ||
		runtime.Readiness(context.Background()).Readiness().Status() != runtimehealth.StatusHealthy {
		t.Fatalf("runtime was not ready: state=%s", runtime.State())
	}
	view := runtime.Configuration()
	view.Sources["profile"] = runtimeconfig.SourceCommandLine
	if runtime.Configuration().Sources["profile"] == runtimeconfig.SourceCommandLine {
		t.Fatal("configuration view was not detached")
	}
	started := time.Now()
	result, err := runtime.Shutdown(context.Background())
	if err != nil || result.Outcome() != ShutdownGraceful || !result.ResourcesClosed() || result.InitialInFlight() != 0 ||
		result.CanceledWork() != 0 || result.StartedAt().IsZero() || result.FinishedAt().IsZero() || result.Duration() < 0 ||
		postgres.closes.Load() != 1 || time.Since(started) >= time.Second {
		t.Fatalf("Shutdown() = (%#v, %v), closes=%d", result, err, postgres.closes.Load())
	}
	if runtime.State() != runtimehealth.StateStopped || runtime.Liveness(context.Background()).Liveness().Status() != runtimehealth.StatusUnhealthy {
		t.Fatal("stopped runtime remained live")
	}
}

func TestObservedLifecyclePublishesBoundedEventsAndStopsCollection(t *testing.T) {
	observer := &fakeObservability{}
	postgres := &fakePostgreSQL{}
	starter, err := NewObservedStarter(runtimeconfig.NewLoader(), fakeOpener{runtime: postgres}, fakeObservabilityFactory{service: observer})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := starter.Start(context.Background(), testLoadRequest())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := observer.source.ObservabilitySnapshot(context.Background())
	if snapshot.State() != runtimehealth.StateReady || snapshot.Liveness() != runtimehealth.StatusHealthy || snapshot.Readiness() != runtimehealth.StatusHealthy || !snapshot.SchemaCompatible() {
		t.Fatalf("observability snapshot = state:%s live:%s ready:%s", snapshot.State(), snapshot.Liveness(), snapshot.Readiness())
	}
	postgres.checkErr.Store(errors.New("private database detail"))
	for range 3 {
		_ = runtime.Readiness(context.Background())
	}
	if runtime.Readiness(context.Background()).Readiness().Status() != runtimehealth.StatusUnhealthy {
		t.Fatal("readiness loss was not observed")
	}
	result, err := runtime.Shutdown(context.Background())
	if err != nil || !result.ResourcesClosed() {
		t.Fatalf("shutdown = (%#v, %v)", result, err)
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.starts != 1 || observer.stops != 1 || observer.closes != 1 {
		t.Fatalf("observer lifecycle starts=%d stops=%d closes=%d", observer.starts, observer.stops, observer.closes)
	}
	want := []runtimeobservability.EventName{runtimeobservability.EventStartup, runtimeobservability.EventReady, runtimeobservability.EventHealthLost, runtimeobservability.EventDraining, runtimeobservability.EventStopping, runtimeobservability.EventStopped}
	if len(observer.events) != len(want) {
		t.Fatalf("events = %#v", observer.events)
	}
	for index, event := range observer.events {
		if event.Event != want[index] {
			t.Fatalf("event[%d] = %s", index, event.Event)
		}
	}
}

func TestObservabilityStartupFailuresCleanResources(t *testing.T) {
	postgres := &fakePostgreSQL{}
	factoryFailure, err := NewObservedStarter(runtimeconfig.NewLoader(), fakeOpener{runtime: postgres}, fakeObservabilityFactory{err: errors.New("private observer detail")})
	if err != nil {
		t.Fatal(err)
	}
	if runtime, err := factoryFailure.Start(context.Background(), testLoadRequest()); runtime != nil || CodeOf(err) != ErrorStartup || postgres.closes.Load() != 0 {
		t.Fatalf("factory failure = (%v, %v), postgres closes=%d", runtime, err, postgres.closes.Load())
	}
	observer := &fakeObservability{startError: errors.New("private start detail")}
	startFailure, err := NewObservedStarter(runtimeconfig.NewLoader(), fakeOpener{runtime: postgres}, fakeObservabilityFactory{service: observer})
	if err != nil {
		t.Fatal(err)
	}
	if runtime, err := startFailure.Start(context.Background(), testLoadRequest()); runtime != nil || CodeOf(err) != ErrorStartup || postgres.closes.Load() != 1 || observer.closes != 1 {
		t.Fatalf("start failure = (%v, %v), postgres closes=%d observer closes=%d", runtime, err, postgres.closes.Load(), observer.closes)
	}
}

func TestIndependentRuntimeCyclesResetLifecycleState(t *testing.T) {
	const cycles = 100
	for index := 0; index < cycles; index++ {
		postgres := &fakePostgreSQL{}
		runtime := startTestRuntime(t, postgres)
		work, err := runtime.Admit(context.Background())
		if err != nil {
			t.Fatalf("cycle %d admit: %v", index, err)
		}
		work.Done()
		result, err := runtime.Shutdown(context.Background())
		if err != nil || result.Outcome() != ShutdownGraceful || runtime.State() != runtimehealth.StateStopped || runtime.InFlight() != 0 || postgres.closes.Load() != 1 {
			t.Fatalf("cycle %d = result:%#v error:%v state:%s inflight:%d closes:%d", index, result, err, runtime.State(), runtime.InFlight(), postgres.closes.Load())
		}
	}
}

func TestRuntimeContractVersionsAreFrozenAtOne(t *testing.T) {
	versions := map[string]string{
		"application-runtime":   ContractVersion,
		"runtime-configuration": runtimeconfig.ContractVersion,
		"runtime-health":        runtimehealth.ContractVersion,
		"runtime-observability": runtimeobservability.ContractVersion,
		"postgresql-runtime":    runtimepostgres.ContractVersion,
	}
	for name, version := range versions {
		if version != "1.0.0" {
			t.Fatalf("%s contract version = %q", name, version)
		}
	}
}

func TestGracefulDrainRejectsNewWorkAndWaits(t *testing.T) {
	postgres := &fakePostgreSQL{}
	runtime := startTestRuntime(t, postgres)
	work, err := runtime.Admit(context.Background())
	if err != nil || runtime.InFlight() != 1 {
		t.Fatalf("Admit() = (%v, %v)", work, err)
	}
	resultChannel := make(chan ShutdownResult, 1)
	errorChannel := make(chan error, 1)
	go func() {
		result, shutdownErr := runtime.Shutdown(context.Background())
		resultChannel <- result
		errorChannel <- shutdownErr
	}()
	waitForState(t, runtime, runtimehealth.StateDraining)
	if runtime.Readiness(context.Background()).Readiness().ReasonCode() != runtimehealth.ReasonDraining {
		t.Fatal("draining did not remove readiness")
	}
	if _, err := runtime.Admit(context.Background()); CodeOf(err) != ErrorDraining {
		t.Fatalf("new work during drain error = %v", err)
	}
	work.Done()
	work.Done()
	result := <-resultChannel
	if err := <-errorChannel; err != nil || result.Outcome() != ShutdownGraceful || result.InitialInFlight() != 1 || runtime.InFlight() != 0 {
		t.Fatalf("graceful result = %#v, error=%v", result, err)
	}
}

func TestForceCancelsAdmittedWork(t *testing.T) {
	postgres := &fakePostgreSQL{}
	runtime := startTestRuntime(t, postgres)
	work, err := runtime.Admit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resultChannel := make(chan ShutdownResult, 1)
	go func() {
		result, _ := runtime.Shutdown(context.Background())
		resultChannel <- result
	}()
	waitForState(t, runtime, runtimehealth.StateDraining)
	runtime.Force()
	runtime.Force()
	select {
	case <-work.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("forced shutdown did not cancel work")
	}
	work.Done()
	result := <-resultChannel
	if result.Outcome() != ShutdownForced || result.CanceledWork() != 1 || postgres.closes.Load() != 1 {
		t.Fatalf("forced result = %#v", result)
	}
}

func TestDrainTimeoutCancelsWorkThenCompletes(t *testing.T) {
	postgres := &fakePostgreSQL{}
	starter, err := NewStarter(runtimeconfig.NewLoader(), fakeOpener{runtime: postgres})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := starter.Start(context.Background(), timedLoadRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	work, err := runtime.Admit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		<-work.Context().Done()
		work.Done()
	}()
	started := time.Now()
	result, err := runtime.Shutdown(context.Background())
	if err != nil || result.Outcome() != ShutdownCanceled || result.CanceledWork() != 1 ||
		time.Since(started) < time.Second || time.Since(started) >= 2*time.Second || postgres.closes.Load() != 1 {
		t.Fatalf("timed drain result = %#v, error=%v, duration=%s", result, err, time.Since(started))
	}
}

func TestBlockedResourceCloseIsBoundedAndNotReportedStopped(t *testing.T) {
	block := make(chan struct{})
	postgres := &fakePostgreSQL{closeBlock: block}
	starter, err := NewStarter(runtimeconfig.NewLoader(), fakeOpener{runtime: postgres})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := starter.Start(context.Background(), timedLoadRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	result, err := runtime.Shutdown(context.Background())
	if err != nil || result.Outcome() != ShutdownForced || result.ResourcesClosed() ||
		runtime.State() != runtimehealth.StateStopping || time.Since(started) < time.Second || time.Since(started) >= 2*time.Second {
		t.Fatalf("blocked close result = %#v, error=%v, state=%s, duration=%s", result, err, runtime.State(), time.Since(started))
	}
	close(block)
}

func TestConcurrentShutdownReturnsOneStableResult(t *testing.T) {
	runtime := startTestRuntime(t, &fakePostgreSQL{})
	const callers = 50
	results := make([]ShutdownResult, callers)
	errorsFound := make([]error, callers)
	var wait sync.WaitGroup
	for caller := 0; caller < callers; caller++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errorsFound[index] = runtime.Shutdown(context.Background())
		}(caller)
	}
	wait.Wait()
	for index := 1; index < callers; index++ {
		if errorsFound[index] != nil || !reflect.DeepEqual(results[0], results[index]) {
			t.Fatalf("shutdown result %d differs: %#v / %v", index, results[index], errorsFound[index])
		}
	}
}

func TestDatabaseOutageChangesReadinessNotLiveness(t *testing.T) {
	postgres := &fakePostgreSQL{}
	runtime := startTestRuntime(t, postgres)
	postgres.checkErr.Store(errors.New("private database detail"))
	for attempt := 0; attempt < 3; attempt++ {
		_ = runtime.Readiness(context.Background())
	}
	snapshot := runtime.Readiness(context.Background())
	if snapshot.Readiness().Status() != runtimehealth.StatusUnhealthy || snapshot.Liveness().Status() != runtimehealth.StatusHealthy ||
		runtime.State() != runtimehealth.StateReady {
		t.Fatalf("database outage snapshot = %#v", snapshot)
	}
}

func TestStartupFailuresAreBoundedAndClean(t *testing.T) {
	if _, err := NewStarter(nil, nil); CodeOf(err) != ErrorInvalidInput {
		t.Fatalf("nil dependencies error = %v", err)
	}
	if NewDefaultStarter() == nil {
		t.Fatal("default starter is nil")
	}
	loader := runtimeconfig.NewLoader()
	starter, err := NewStarter(loader, fakeOpener{err: errors.New("private opener detail")})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := starter.Start(context.Background(), testLoadRequest())
	if runtime != nil || CodeOf(err) != ErrorStartup || errors.Is(err, errors.New("private opener detail")) {
		t.Fatalf("Start() = (%v, %v)", runtime, err)
	}
	nilRuntimeStarter, err := NewStarter(loader, fakeOpener{})
	if err != nil {
		t.Fatal(err)
	}
	if runtime, err := nilRuntimeStarter.Start(context.Background(), testLoadRequest()); runtime != nil || CodeOf(err) != ErrorStartup {
		t.Fatalf("nil PostgreSQL runtime = (%v, %v)", runtime, err)
	}
	timeoutStarter, err := NewStarter(loader, fakeOpener{wait: true})
	if err != nil {
		t.Fatal(err)
	}
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer timeoutCancel()
	if runtime, err := timeoutStarter.Start(timeoutCtx, testLoadRequest()); runtime != nil || CodeOf(err) != ErrorTimeout {
		t.Fatalf("timed out PostgreSQL startup = (%v, %v)", runtime, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runtime, err = starter.Start(ctx, testLoadRequest())
	if runtime != nil || CodeOf(err) != ErrorCanceled || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Start() = (%v, %v)", runtime, err)
	}
}

func TestShutdownCallerCancellationDoesNotAbortCleanup(t *testing.T) {
	postgres := &fakePostgreSQL{}
	runtime := startTestRuntime(t, postgres)
	work, _ := runtime.Admit(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runtime.Shutdown(ctx); CodeOf(err) != ErrorCanceled {
		t.Fatalf("canceled shutdown wait error = %v", err)
	}
	runtime.Force()
	<-work.Context().Done()
	work.Done()
	result, err := runtime.Shutdown(context.Background())
	if err != nil || !result.ResourcesClosed() || postgres.closes.Load() != 1 {
		t.Fatalf("cleanup result = %#v, %v", result, err)
	}
}

func TestNilRuntimeAndErrorBoundary(t *testing.T) {
	var runtime *Runtime
	if runtime.Configuration().ContractVersion != "" || runtime.Ingest() != nil || runtime.Read() != nil || runtime.Retention() != nil ||
		runtime.State() != runtimehealth.StateFailed || runtime.InFlight() != 0 {
		t.Fatal("nil runtime getters are not safe")
	}
	if runtime.Liveness(context.Background()).RuntimeState() != "" || runtime.Readiness(context.Background()).RuntimeState() != "" {
		t.Fatal("nil runtime health is not safe")
	}
	if _, err := runtime.Admit(context.Background()); CodeOf(err) != ErrorInvalidInput {
		t.Fatalf("nil Admit error = %v", err)
	}
	if _, err := runtime.Shutdown(context.Background()); CodeOf(err) != ErrorInvalidInput {
		t.Fatalf("nil Shutdown error = %v", err)
	}
	runtime.Force()
	var work *admittedWork
	if work.Context() == nil {
		t.Fatal("nil work context is nil")
	}
	work.Done()
	var failure *Error
	if failure.Error() != "application-runtime: runtime_internal: runtime" || failure.Code() != ErrorInternal || failure.Unwrap() != nil ||
		CodeOf(errors.New("foreign")) != ErrorInternal {
		t.Fatal("application runtime error boundary is unstable")
	}
}

func readyTestRuntime(t testing.TB) *Runtime {
	t.Helper()
	return startTestRuntime(t, &fakePostgreSQL{})
}

func startTestRuntime(t testing.TB, postgres PostgreSQLRuntime) *Runtime {
	t.Helper()
	starter, err := NewStarter(runtimeconfig.NewLoader(), fakeOpener{runtime: postgres})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := starter.Start(context.Background(), testLoadRequest())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	return runtime
}

func testLoadRequest() runtimeconfig.LoadRequest {
	return runtimeconfig.NewLoadRequest(runtimeconfig.LoadRequestParams{
		Environment: []string{
			"AEGIS_PROFILE=ci", "AEGIS_DATABASE_HOST=127.0.0.1",
			"AEGIS_DATABASE_NAME=aegis_lifecycle_test", "AEGIS_DATABASE_USER=runtime_test",
		},
		SecretProvider: testSecretProvider{},
	})
}

func timedLoadRequest(t testing.TB) runtimeconfig.LoadRequest {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(path, []byte(`{"startup":{"drain_timeout":"1s","forced_shutdown_timeout":"1s"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return runtimeconfig.NewLoadRequest(runtimeconfig.LoadRequestParams{
		Environment: []string{
			"AEGIS_CONFIG_FILE=" + path,
			"AEGIS_PROFILE=ci", "AEGIS_DATABASE_HOST=127.0.0.1",
			"AEGIS_DATABASE_NAME=aegis_lifecycle_test", "AEGIS_DATABASE_USER=runtime_test",
		},
		SecretProvider: testSecretProvider{},
	})
}

func waitForState(t testing.TB, runtime *Runtime, state runtimehealth.RuntimeState) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runtime.State() == state {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("runtime did not reach %s; state=%s", state, runtime.State())
}
