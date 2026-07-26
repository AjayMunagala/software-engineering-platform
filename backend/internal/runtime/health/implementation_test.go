package health

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

type manualClock struct {
	mutex sync.RWMutex
	now   time.Time
}

func (clock *manualClock) Now() time.Time {
	clock.mutex.RLock()
	defer clock.mutex.RUnlock()
	return clock.now
}

func (clock *manualClock) Advance(duration time.Duration) {
	clock.mutex.Lock()
	clock.now = clock.now.Add(duration)
	clock.mutex.Unlock()
}

type checkerFunc func(context.Context) error

func (function checkerFunc) Check(ctx context.Context) error { return function(ctx) }

type reasonError struct{ reason string }

func (failure reasonError) Error() string        { return "safe-check-failure" }
func (failure reasonError) HealthReason() string { return failure.reason }

func TestHealthTransitionsAndRecovery(t *testing.T) {
	clock := &manualClock{now: time.Unix(100, 0).UTC()}
	var fail atomic.Bool
	monitor := newTestMonitor(t, clock, 3, checkerFunc(func(context.Context) error {
		if fail.Load() {
			return errors.New("database driver detail")
		}
		return nil
	}))
	advanceToReady(t, monitor)

	ready := monitor.Readiness(context.Background())
	if ready.Readiness().Status() != StatusHealthy || ready.Readiness().ReasonCode() != ReasonReady ||
		ready.Liveness().Status() != StatusHealthy || ready.LastSuccessfulDatabaseCheck().IsZero() {
		t.Fatalf("unexpected ready snapshot: %#v", ready)
	}
	fail.Store(true)
	for attempt := 1; attempt <= 2; attempt++ {
		snapshot := monitor.Readiness(context.Background())
		if snapshot.Readiness().Status() != StatusHealthy || snapshot.Readiness().ReasonCode() != ReasonFailureGrace ||
			snapshot.ConsecutiveReadinessFailures() != attempt {
			t.Fatalf("grace attempt %d: %#v", attempt, snapshot)
		}
	}
	unavailable := monitor.Readiness(context.Background())
	if unavailable.Readiness().Status() != StatusUnhealthy || unavailable.Readiness().ReasonCode() != ReasonDatabaseUnavailable ||
		unavailable.Liveness().Status() != StatusHealthy {
		t.Fatalf("database outage incorrectly affected health: %#v", unavailable)
	}
	fail.Store(false)
	recovered := monitor.Readiness(context.Background())
	if recovered.Readiness().Status() != StatusHealthy || recovered.ConsecutiveReadinessFailures() != 0 {
		t.Fatalf("readiness did not recover: %#v", recovered)
	}
}

func TestHealthStalenessDrainAndStop(t *testing.T) {
	clock := &manualClock{now: time.Unix(200, 0).UTC()}
	monitor := newTestMonitor(t, clock, 3, checkerFunc(func(context.Context) error { return nil }))
	advanceToReady(t, monitor)
	_ = monitor.Readiness(context.Background())
	clock.Advance(2*time.Second + time.Nanosecond)
	stale := monitor.Snapshot()
	if stale.Readiness().Status() != StatusUnhealthy || stale.Readiness().ReasonCode() != ReasonStale {
		t.Fatalf("stale snapshot = %#v", stale)
	}
	if err := monitor.Transition(StateDraining); err != nil {
		t.Fatal(err)
	}
	draining := monitor.Liveness(context.Background())
	if draining.Readiness().ReasonCode() != ReasonDraining || draining.Liveness().Status() != StatusHealthy {
		t.Fatalf("draining snapshot = %#v", draining)
	}
	if err := monitor.Transition(StateStopping); err != nil {
		t.Fatal(err)
	}
	if err := monitor.Transition(StateStopped); err != nil {
		t.Fatal(err)
	}
	stopped := monitor.Liveness(context.Background())
	if stopped.Liveness().Status() != StatusUnhealthy || stopped.Liveness().ReasonCode() != ReasonStopped {
		t.Fatalf("stopped snapshot = %#v", stopped)
	}
}

func TestHealthTimeoutCancellationAndCodedReasons(t *testing.T) {
	clock := fixedClock{now: time.Unix(300, 0).UTC()}
	tests := []struct {
		name    string
		checker DatabaseChecker
		context func() context.Context
		reason  ReasonCode
	}{
		{
			"timeout", checkerFunc(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }),
			context.Background, ReasonDatabaseTimeout,
		},
		{
			"schema", checkerFunc(func(context.Context) error { return reasonError{string(ReasonSchemaIncompatible)} }),
			context.Background, ReasonSchemaIncompatible,
		},
		{
			"privilege", checkerFunc(func(context.Context) error { return reasonError{string(ReasonPrivilegeIncompatible)} }),
			context.Background, ReasonPrivilegeIncompatible,
		},
		{
			"canceled", checkerFunc(func(context.Context) error { return context.Canceled }),
			func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx },
			ReasonCheckCanceled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configuration, err := NewConfig(ConfigParams{
				CheckTimeout: time.Millisecond, CheckInterval: time.Second,
				FailureThreshold: 1, Clock: clock,
			})
			if err != nil {
				t.Fatal(err)
			}
			monitor, err := NewMonitor(test.checker, configuration)
			if err != nil {
				t.Fatal(err)
			}
			advanceToReady(t, monitor)
			snapshot := monitor.Readiness(test.context())
			wantStatus := StatusUnhealthy
			if test.reason == ReasonCheckCanceled {
				wantStatus = StatusUnknown
			}
			if snapshot.Readiness().ReasonCode() != test.reason || snapshot.Readiness().Status() != wantStatus {
				t.Fatalf("snapshot = %#v", snapshot)
			}
			if test.reason == ReasonCheckCanceled && snapshot.ConsecutiveReadinessFailures() != 0 {
				t.Fatal("caller cancellation changed shared readiness failures")
			}
		})
	}
}

func TestHealthRejectsInvalidConfigurationAndTransition(t *testing.T) {
	if _, err := NewConfig(ConfigParams{}); CodeOf(err) != ErrorInvalidConfig {
		t.Fatalf("zero config error = %v", err)
	}
	configuration, err := NewConfig(ConfigParams{
		CheckTimeout: time.Second, CheckInterval: time.Second, FailureThreshold: 3,
	})
	if err != nil || configuration.CheckTimeout() != time.Second || configuration.CheckInterval() != time.Second ||
		configuration.FailureThreshold() != 3 {
		t.Fatalf("config = %#v, %v", configuration, err)
	}
	if _, err := NewMonitor(nil, configuration); CodeOf(err) != ErrorInvalidInput {
		t.Fatalf("nil checker error = %v", err)
	}
	monitor, err := NewMonitor(checkerFunc(func(context.Context) error { return nil }), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := monitor.Transition(StateReady); CodeOf(err) != ErrorInvalidTransition {
		t.Fatalf("invalid transition error = %v", err)
	}
	if snapshot := monitor.Liveness(context.Background()); snapshot.RuntimeState() != StateFailed || snapshot.Liveness().Status() != StatusUnhealthy {
		t.Fatalf("invalid transition did not fail closed: %#v", snapshot)
	}
}

func TestConcurrentHealthSnapshots(t *testing.T) {
	clock := fixedClock{now: time.Unix(400, 0).UTC()}
	monitor := newTestMonitor(t, clock, 3, checkerFunc(func(context.Context) error { return nil }))
	advanceToReady(t, monitor)
	var wait sync.WaitGroup
	for worker := 0; worker < 100; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 100; iteration++ {
				if monitor.Readiness(context.Background()).RuntimeState() != StateReady ||
					monitor.Liveness(context.Background()).Liveness().Status() != StatusHealthy {
					t.Error("concurrent health returned inconsistent state")
					return
				}
			}
		}()
	}
	wait.Wait()
}

func TestNilMonitorAndErrorBoundary(t *testing.T) {
	var monitor *monitor
	if monitor.Liveness(context.Background()).Liveness().ReasonCode() != ReasonInternal ||
		monitor.Readiness(context.Background()).Readiness().ReasonCode() != ReasonInternal ||
		monitor.Snapshot().RuntimeState() != StateFailed || CodeOf(monitor.Transition(StateReady)) != ErrorInvalidInput {
		t.Fatal("nil monitor boundary is not stable")
	}
	var failure *Error
	if failure.Error() != "runtime-health: invalid_input: health" || failure.Code() != ErrorInvalidInput ||
		CodeOf(errors.New("foreign")) != ErrorInvalidInput {
		t.Fatal("health error boundary is not stable")
	}
}

func newTestMonitor(t testing.TB, clock Clock, threshold int, checker DatabaseChecker) Monitor {
	t.Helper()
	configuration, err := NewConfig(ConfigParams{
		CheckTimeout: time.Millisecond, CheckInterval: time.Second,
		FailureThreshold: threshold, Clock: clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := NewMonitor(checker, configuration)
	if err != nil {
		t.Fatal(err)
	}
	return monitor
}

func advanceToReady(t testing.TB, monitor Monitor) {
	t.Helper()
	for _, state := range []RuntimeState{
		StateLoading, StateValidating, StateConnecting,
		StateCompatibilityChecking, StateReady,
	} {
		if err := monitor.Transition(state); err != nil {
			t.Fatalf("Transition(%s): %v", state, err)
		}
	}
}
