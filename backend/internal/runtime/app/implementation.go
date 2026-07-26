package app

import (
	"context"
	"time"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
	runtimeobservability "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/observability"
	runtimepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
)

type starter struct {
	loader        runtimeconfig.Loader
	postgres      PostgreSQLOpener
	observability ObservabilityFactory
}

func (value *starter) Start(ctx context.Context, request runtimeconfig.LoadRequest) (*Runtime, error) {
	if value == nil || value.loader == nil || value.postgres == nil || ctx == nil {
		return nil, newError(ErrorInvalidInput, "startup", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, newError(ErrorCanceled, "startup", err)
	}
	loaded, err := value.loader.Load(ctx, request)
	if err != nil {
		return nil, newError(ErrorStartup, "configuration", err)
	}
	configuration := loaded.Config()
	var observer runtimeobservability.Service
	if value.observability != nil {
		observer, err = value.observability.Open(configuration)
		if err != nil || observer == nil {
			return nil, newError(ErrorStartup, "observability", err)
		}
		_ = observer.Event(ctx, runtimeobservability.EventParams{Level: runtimeobservability.LevelInfo, Component: runtimeobservability.ComponentRuntime, Event: runtimeobservability.EventStartup, Outcome: runtimeobservability.OutcomeSuccess, RuntimeState: runtimehealth.StateLoading})
	}
	startupCtx, cancel := boundedContext(ctx, configuration.Startup().StartupTimeout())
	defer cancel()
	observerOwned := observer != nil
	defer func() {
		if observerOwned {
			closeContext, closeCancel := context.WithTimeout(context.Background(), configuration.Startup().ForcedShutdownTimeout())
			_ = observer.Close(closeContext)
			closeCancel()
		}
	}()

	postgresRuntime, err := value.postgres.Open(startupCtx, loaded)
	if err != nil {
		return nil, newError(ErrorStartup, "postgresql", err)
	}
	if postgresRuntime == nil {
		return nil, newError(ErrorStartup, "postgresql", nil)
	}
	cleanup := true
	defer func() {
		if cleanup {
			postgresRuntime.Close()
		}
	}()

	healthConfiguration, err := runtimehealth.NewConfig(runtimehealth.ConfigParams{
		CheckTimeout:     configuration.Health().CheckTimeout(),
		CheckInterval:    configuration.Health().CheckInterval(),
		FailureThreshold: configuration.Health().FailureThreshold(),
	})
	if err != nil {
		return nil, newError(ErrorStartup, "health-config", err)
	}
	monitor, err := runtimehealth.NewMonitor(postgresRuntime, healthConfiguration)
	if err != nil {
		return nil, newError(ErrorStartup, "health-monitor", err)
	}
	for _, state := range []runtimehealth.RuntimeState{
		runtimehealth.StateLoading,
		runtimehealth.StateValidating,
		runtimehealth.StateConnecting,
		runtimehealth.StateCompatibilityChecking,
		runtimehealth.StateReady,
	} {
		if err := monitor.Transition(state); err != nil {
			return nil, newError(ErrorInternal, "startup-transition", err)
		}
	}
	zeroWork := make(chan struct{})
	close(zeroWork)
	runtime := &Runtime{
		configuration: configuration, postgres: postgresRuntime, health: monitor, observability: observer,
		lastReadiness: runtimehealth.StatusHealthy,
		accepting:     true, inFlight: make(map[uint64]context.CancelFunc), zeroWork: zeroWork,
		shutdownDone: make(chan struct{}), force: make(chan struct{}),
	}
	if observer != nil {
		if err := observer.Start(startupCtx, runtime); err != nil {
			return nil, newError(ErrorStartup, "observability-start", err)
		}
		_ = observer.Event(ctx, runtimeobservability.EventParams{Level: runtimeobservability.LevelInfo, Component: runtimeobservability.ComponentRuntime, Event: runtimeobservability.EventReady, Outcome: runtimeobservability.OutcomeSuccess, RuntimeState: runtimehealth.StateReady})
	}
	cleanup = false
	observerOwned = false
	return runtime, nil
}

func (runtime *Runtime) Liveness(ctx context.Context) runtimehealth.HealthSnapshot {
	if runtime == nil || runtime.health == nil {
		return runtimehealth.HealthSnapshot{}
	}
	return runtime.health.Liveness(ctx)
}

func (runtime *Runtime) Readiness(ctx context.Context) runtimehealth.HealthSnapshot {
	if runtime == nil || runtime.health == nil {
		return runtimehealth.HealthSnapshot{}
	}
	snapshot := runtime.health.Readiness(ctx)
	runtime.recordReadiness(snapshot)
	return snapshot
}

func (runtime *Runtime) recordReadiness(snapshot runtimehealth.HealthSnapshot) {
	if runtime == nil || runtime.observability == nil {
		return
	}
	current := snapshot.Readiness().Status()
	runtime.healthObservabilityMutex.Lock()
	previous := runtime.lastReadiness
	if current == previous {
		runtime.healthObservabilityMutex.Unlock()
		return
	}
	runtime.lastReadiness = current
	runtime.healthObservabilityMutex.Unlock()
	event := runtimeobservability.EventHealthLost
	level := runtimeobservability.LevelWarn
	outcome := runtimeobservability.OutcomeError
	if current == runtimehealth.StatusHealthy {
		event, level, outcome = runtimeobservability.EventHealthReady, runtimeobservability.LevelInfo, runtimeobservability.OutcomeSuccess
	}
	_ = runtime.observability.Event(context.Background(), runtimeobservability.EventParams{
		Level: level, Component: runtimeobservability.ComponentHealth, Event: event, Outcome: outcome,
		ErrorKind: string(snapshot.Readiness().ReasonCode()), RuntimeState: snapshot.RuntimeState(),
	})
}

// ObservabilitySnapshot returns a detached, bounded view and performs one
// readiness proof. It exposes no pool, credential, SQL, path, or payload.
func (runtime *Runtime) ObservabilitySnapshot(ctx context.Context) runtimeobservability.RuntimeSnapshot {
	if runtime == nil {
		return runtimeobservability.RuntimeSnapshot{}
	}
	startedAt := time.Now()
	health := runtime.Readiness(ctx)
	pools := make([]runtimeobservability.PoolSnapshot, 0)
	if provider, ok := runtime.postgres.(interface {
		Statistics() []runtimepostgres.PoolStatistics
	}); ok {
		for _, statistic := range provider.Statistics() {
			pool, err := runtimeobservability.NewPoolSnapshot(runtimeobservability.PoolSnapshotParams{
				Name: string(statistic.Capability()), Acquired: statistic.Acquired(), Idle: statistic.Idle(), Constructing: statistic.Constructing(),
				Total: statistic.Total(), Maximum: statistic.Maximum(), AcquireCount: statistic.AcquireCount(), AcquireDuration: statistic.AcquireDuration(),
				EmptyAcquireCount: statistic.EmptyAcquireCount(), EmptyAcquireWait: statistic.EmptyAcquireWait(), ConnectionsCreated: statistic.ConnectionsCreated(),
				ConnectionsDestroyedIdle: statistic.ConnectionsDestroyedIdle(), ConnectionsDestroyedLifetime: statistic.ConnectionsDestroyedLifetime(),
			})
			if err == nil {
				pools = append(pools, pool)
			}
		}
	}
	snapshot, _ := runtimeobservability.NewRuntimeSnapshot(runtimeobservability.RuntimeSnapshotParams{
		ObservedAt: time.Now().UTC(), State: health.RuntimeState(), Liveness: health.Liveness().Status(), Readiness: health.Readiness().Status(),
		InFlight: runtime.InFlight(), SchemaCompatible: health.Readiness().Status() == runtimehealth.StatusHealthy, HealthDuration: time.Since(startedAt), Pools: pools,
	})
	return snapshot
}

func (runtime *Runtime) Admit(ctx context.Context) (Work, error) {
	if runtime == nil || ctx == nil {
		return nil, newError(ErrorInvalidInput, "admit", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, newError(ErrorCanceled, "admit", err)
	}
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	if !runtime.accepting {
		return nil, newError(ErrorDraining, "admit", nil)
	}
	if len(runtime.inFlight) == 0 {
		runtime.zeroWork = make(chan struct{})
	}
	runtime.nextWorkID++
	id := runtime.nextWorkID
	workCtx, cancel := context.WithCancel(ctx)
	runtime.inFlight[id] = cancel
	return &admittedWork{
		context: workCtx,
		done:    func() { runtime.release(id) },
	}, nil
}

func (runtime *Runtime) release(id uint64) {
	runtime.mutex.Lock()
	cancel, found := runtime.inFlight[id]
	if found {
		delete(runtime.inFlight, id)
		cancel()
		if len(runtime.inFlight) == 0 {
			close(runtime.zeroWork)
		}
	}
	runtime.mutex.Unlock()
}

// Shutdown starts one deterministic drain. Concurrent/repeated callers wait
// for and receive the same completed result. Canceling a caller does not abort
// the owner cleanup already in progress.
func (runtime *Runtime) Shutdown(ctx context.Context) (ShutdownResult, error) {
	if runtime == nil || ctx == nil {
		return ShutdownResult{}, newError(ErrorInvalidInput, "shutdown", nil)
	}
	runtime.shutdownOnce.Do(func() { go runtime.runShutdown() })
	select {
	case <-runtime.shutdownDone:
		return runtime.result, runtime.shutdownErr
	case <-ctx.Done():
		return ShutdownResult{}, newError(ErrorCanceled, "shutdown-wait", ctx.Err())
	}
}

// Force requests immediate cancellation of admitted work. Process signal
// handling remains the responsibility of the future command host.
func (runtime *Runtime) Force() {
	if runtime == nil {
		return
	}
	runtime.forceOnce.Do(func() { close(runtime.force) })
}

func (runtime *Runtime) runShutdown() {
	startedAt := time.Now().UTC()
	_ = runtime.health.Transition(runtimehealth.StateDraining)
	runtime.mutex.Lock()
	runtime.accepting = false
	initial := len(runtime.inFlight)
	zeroWork := runtime.zeroWork
	runtime.mutex.Unlock()
	if runtime.observability != nil {
		_ = runtime.observability.Event(context.Background(), runtimeobservability.EventParams{Level: runtimeobservability.LevelInfo, Component: runtimeobservability.ComponentRuntime, Event: runtimeobservability.EventDraining, Outcome: runtimeobservability.OutcomeSuccess, RuntimeState: runtimehealth.StateDraining, Count: int64(initial)})
	}

	outcome := ShutdownGraceful
	canceledWork := 0
	drainTimer := time.NewTimer(runtime.configuration.Startup().DrainTimeout())
	select {
	case <-zeroWork:
		stopTimer(drainTimer)
	case <-drainTimer.C:
		outcome = ShutdownCanceled
		canceledWork = runtime.cancelWork()
	case <-runtime.force:
		stopTimer(drainTimer)
		outcome = ShutdownForced
		canceledWork = runtime.cancelWork()
	}

	_ = runtime.health.Transition(runtimehealth.StateStopping)
	if runtime.observability != nil {
		_ = runtime.observability.Event(context.Background(), runtimeobservability.EventParams{Level: runtimeobservability.LevelInfo, Component: runtimeobservability.ComponentRuntime, Event: runtimeobservability.EventStopping, Outcome: runtimeobservability.OutcomeSuccess, RuntimeState: runtimehealth.StateStopping})
	}
	if canceledWork != 0 {
		runtime.mutex.Lock()
		zeroWork = runtime.zeroWork
		runtime.mutex.Unlock()
		forcedTimer := time.NewTimer(runtime.configuration.Startup().ForcedShutdownTimeout())
		select {
		case <-zeroWork:
			stopTimer(forcedTimer)
		case <-forcedTimer.C:
			outcome = ShutdownForced
		}
	}

	resourcesClosed := runtime.closeResources(runtime.configuration.Startup().ForcedShutdownTimeout())
	if resourcesClosed {
		_ = runtime.health.Transition(runtimehealth.StateStopped)
	} else {
		outcome = ShutdownForced
	}
	runtime.result = ShutdownResult{
		outcome: outcome, startedAt: startedAt, finishedAt: time.Now().UTC(),
		initialInFlight: initial, canceledWork: canceledWork, resourcesClosed: resourcesClosed,
	}
	if runtime.observability != nil {
		level, eventOutcome := runtimeobservability.LevelInfo, runtimeobservability.OutcomeSuccess
		if outcome == ShutdownForced {
			level, eventOutcome = runtimeobservability.LevelError, runtimeobservability.OutcomeForced
		}
		_ = runtime.observability.Event(context.Background(), runtimeobservability.EventParams{Level: level, Component: runtimeobservability.ComponentRuntime, Event: runtimeobservability.EventStopped, Outcome: eventOutcome, Duration: runtime.result.Duration(), RuntimeState: runtime.State(), Count: int64(canceledWork)})
		closeContext, closeCancel := context.WithTimeout(context.Background(), runtime.configuration.Startup().ForcedShutdownTimeout())
		_ = runtime.observability.Close(closeContext)
		closeCancel()
	}
	close(runtime.shutdownDone)
}

func (runtime *Runtime) closeResources(timeout time.Duration) bool {
	closed := make(chan struct{})
	go func() {
		if runtime.observability != nil {
			stopContext, stopCancel := context.WithTimeout(context.Background(), timeout)
			_ = runtime.observability.StopCollection(stopContext)
			stopCancel()
		}
		runtime.postgres.Close()
		close(closed)
	}()
	timer := time.NewTimer(timeout)
	defer stopTimer(timer)
	select {
	case <-closed:
		return true
	case <-timer.C:
		return false
	}
}

func (runtime *Runtime) cancelWork() int {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	count := len(runtime.inFlight)
	for _, cancel := range runtime.inFlight {
		cancel()
	}
	return count
}

func boundedContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if deadline, ok := parent.Deadline(); ok && time.Until(deadline) <= timeout {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

func stopTimer(timer *time.Timer) {
	if timer != nil && !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
