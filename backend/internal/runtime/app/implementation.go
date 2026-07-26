package app

import (
	"context"
	"time"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
)

type starter struct {
	loader   runtimeconfig.Loader
	postgres PostgreSQLOpener
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
	startupCtx, cancel := boundedContext(ctx, configuration.Startup().StartupTimeout())
	defer cancel()

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
		configuration: configuration, postgres: postgresRuntime, health: monitor,
		accepting: true, inFlight: make(map[uint64]context.CancelFunc), zeroWork: zeroWork,
		shutdownDone: make(chan struct{}), force: make(chan struct{}),
	}
	cleanup = false
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
	return runtime.health.Readiness(ctx)
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
	close(runtime.shutdownDone)
}

func (runtime *Runtime) closeResources(timeout time.Duration) bool {
	closed := make(chan struct{})
	go func() {
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
