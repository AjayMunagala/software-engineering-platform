package health

import (
	"context"
	"errors"
	"sync"
	"time"
)

type monitor struct {
	mutex                    sync.RWMutex
	checker                  DatabaseChecker
	config                   Config
	state                    RuntimeState
	readiness                Signal
	lastDatabaseSuccess      time.Time
	lastCompatibilitySuccess time.Time
	consecutiveFailures      int
}

// NewMonitor constructs a transport-free health monitor in the new state.
func NewMonitor(checker DatabaseChecker, config Config) (Monitor, error) {
	if checker == nil || config.clock == nil || config.checkTimeout <= 0 ||
		config.checkInterval <= 0 || config.failureThreshold <= 0 {
		return nil, newError(ErrorInvalidInput, "health-monitor", nil)
	}
	return &monitor{
		checker: checker, config: config, state: StateNew,
		readiness: Signal{status: StatusUnknown, reason: ReasonStarting},
	}, nil
}

func (value *monitor) Transition(next RuntimeState) error {
	if value == nil {
		return newError(ErrorInvalidInput, "health-transition", nil)
	}
	value.mutex.Lock()
	defer value.mutex.Unlock()
	if next == value.state {
		return nil
	}
	if !allowedTransition(value.state, next) {
		value.state = StateFailed
		value.readiness = Signal{status: StatusUnhealthy, reason: ReasonInternal}
		return newError(ErrorInvalidTransition, "health-transition", nil)
	}
	value.state = next
	switch next {
	case StateReady:
		value.readiness = Signal{status: StatusHealthy, reason: ReasonReady}
		now := value.config.clock.Now().UTC()
		value.lastDatabaseSuccess = now
		value.lastCompatibilitySuccess = now
		value.consecutiveFailures = 0
	case StateDraining:
		value.readiness = Signal{status: StatusUnhealthy, reason: ReasonDraining}
	case StateStopping:
		value.readiness = Signal{status: StatusUnhealthy, reason: ReasonStopping}
	case StateStopped:
		value.readiness = Signal{status: StatusUnhealthy, reason: ReasonStopped}
	case StateFailed:
		value.readiness = Signal{status: StatusUnhealthy, reason: ReasonFailed}
	default:
		value.readiness = Signal{status: StatusUnknown, reason: ReasonStarting}
	}
	return nil
}

func (value *monitor) Liveness(ctx context.Context) HealthSnapshot {
	if value == nil {
		return invalidSnapshot(ReasonInternal)
	}
	if ctx == nil || ctx.Err() != nil {
		return value.snapshotWithLiveness(Signal{status: StatusUnknown, reason: ReasonCheckCanceled})
	}
	value.mutex.RLock()
	state := value.state
	value.mutex.RUnlock()
	liveness := Signal{status: StatusHealthy, reason: ReasonReady}
	if state == StateFailed {
		liveness = Signal{status: StatusUnhealthy, reason: ReasonFailed}
	} else if state == StateStopped {
		liveness = Signal{status: StatusUnhealthy, reason: ReasonStopped}
	} else if state != StateReady && state != StateDraining && state != StateStopping {
		liveness = Signal{status: StatusHealthy, reason: ReasonStarting}
	}
	return value.snapshotWithLiveness(liveness)
}

func (value *monitor) Readiness(ctx context.Context) HealthSnapshot {
	if value == nil {
		return invalidSnapshot(ReasonInternal)
	}
	value.mutex.RLock()
	state := value.state
	value.mutex.RUnlock()
	if state != StateReady {
		return value.Snapshot()
	}
	if ctx == nil || ctx.Err() != nil {
		return value.snapshotWithReadiness(Signal{status: StatusUnknown, reason: ReasonCheckCanceled})
	}
	checkCtx, cancel := boundedContext(ctx, value.config.checkTimeout)
	err := value.checker.Check(checkCtx)
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return value.snapshotWithReadiness(Signal{status: StatusUnknown, reason: ReasonCheckCanceled})
		}
		return value.recordFailure(classifyCheckFailure(err))
	}

	value.mutex.Lock()
	defer value.mutex.Unlock()
	if value.state != StateReady {
		return value.snapshotLocked()
	}
	now := value.config.clock.Now().UTC()
	value.lastDatabaseSuccess = now
	value.lastCompatibilitySuccess = now
	value.consecutiveFailures = 0
	value.readiness = Signal{status: StatusHealthy, reason: ReasonReady}
	return value.snapshotLocked()
}

func (value *monitor) Snapshot() HealthSnapshot {
	if value == nil {
		return invalidSnapshot(ReasonInternal)
	}
	value.mutex.Lock()
	defer value.mutex.Unlock()
	if value.state == StateReady && !value.lastDatabaseSuccess.IsZero() &&
		value.config.clock.Now().UTC().Sub(value.lastDatabaseSuccess) > 2*value.config.checkInterval {
		value.readiness = Signal{status: StatusUnhealthy, reason: ReasonStale}
	}
	return value.snapshotLocked()
}

func (value *monitor) recordFailure(reason ReasonCode) HealthSnapshot {
	value.mutex.Lock()
	defer value.mutex.Unlock()
	if value.state != StateReady {
		return value.snapshotLocked()
	}
	value.consecutiveFailures++
	fresh := !value.lastDatabaseSuccess.IsZero() &&
		value.config.clock.Now().UTC().Sub(value.lastDatabaseSuccess) <= 2*value.config.checkInterval
	if value.consecutiveFailures < value.config.failureThreshold && fresh {
		value.readiness = Signal{status: StatusHealthy, reason: ReasonFailureGrace}
	} else {
		value.readiness = Signal{status: StatusUnhealthy, reason: reason}
	}
	return value.snapshotLocked()
}

func (value *monitor) snapshotWithLiveness(liveness Signal) HealthSnapshot {
	value.mutex.RLock()
	defer value.mutex.RUnlock()
	snapshot := value.snapshotLocked()
	snapshot.liveness = liveness
	return snapshot
}

func (value *monitor) snapshotWithReadiness(readiness Signal) HealthSnapshot {
	value.mutex.RLock()
	defer value.mutex.RUnlock()
	snapshot := value.snapshotLocked()
	snapshot.readiness = readiness
	return snapshot
}

func (value *monitor) snapshotLocked() HealthSnapshot {
	return HealthSnapshot{
		observedAt: value.config.clock.Now().UTC(), runtimeState: value.state,
		liveness: livenessForState(value.state), readiness: value.readiness,
		lastSuccessfulDatabaseCheck:      value.lastDatabaseSuccess,
		lastSuccessfulCompatibilityCheck: value.lastCompatibilitySuccess,
		consecutiveReadinessFailures:     value.consecutiveFailures,
	}
}

func livenessForState(state RuntimeState) Signal {
	switch state {
	case StateFailed:
		return Signal{status: StatusUnhealthy, reason: ReasonFailed}
	case StateStopped:
		return Signal{status: StatusUnhealthy, reason: ReasonStopped}
	case StateReady:
		return Signal{status: StatusHealthy, reason: ReasonReady}
	case StateDraining:
		return Signal{status: StatusHealthy, reason: ReasonDraining}
	case StateStopping:
		return Signal{status: StatusHealthy, reason: ReasonStopping}
	default:
		return Signal{status: StatusHealthy, reason: ReasonStarting}
	}
}

func allowedTransition(current, next RuntimeState) bool {
	switch current {
	case StateNew:
		return next == StateLoading || next == StateFailed || next == StateStopped
	case StateLoading:
		return next == StateValidating || next == StateFailed || next == StateStopping
	case StateValidating:
		return next == StateConnecting || next == StateFailed || next == StateStopping
	case StateConnecting:
		return next == StateCompatibilityChecking || next == StateFailed || next == StateStopping
	case StateCompatibilityChecking:
		return next == StateReady || next == StateFailed || next == StateStopping
	case StateReady:
		return next == StateDraining || next == StateFailed || next == StateStopping
	case StateDraining:
		return next == StateStopping || next == StateStopped
	case StateStopping, StateFailed:
		return next == StateStopped
	default:
		return false
	}
}

func boundedContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if deadline, ok := parent.Deadline(); ok && time.Until(deadline) <= timeout {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

func classifyCheckFailure(err error) ReasonCode {
	if errors.Is(err, context.Canceled) {
		return ReasonCheckCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ReasonDatabaseTimeout
	}
	type reasonCoder interface{ HealthReason() string }
	var coded reasonCoder
	if errors.As(err, &coded) {
		switch coded.HealthReason() {
		case string(ReasonDatabaseTimeout):
			return ReasonDatabaseTimeout
		case string(ReasonSchemaIncompatible):
			return ReasonSchemaIncompatible
		case string(ReasonPrivilegeIncompatible):
			return ReasonPrivilegeIncompatible
		}
	}
	return ReasonDatabaseUnavailable
}

func invalidSnapshot(reason ReasonCode) HealthSnapshot {
	return HealthSnapshot{
		observedAt: time.Now().UTC(), runtimeState: StateFailed,
		liveness:  Signal{status: StatusUnhealthy, reason: reason},
		readiness: Signal{status: StatusUnhealthy, reason: reason},
	}
}
