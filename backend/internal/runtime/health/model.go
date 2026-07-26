package health

import "time"

type RuntimeState string

const (
	StateNew                   RuntimeState = "new"
	StateLoading               RuntimeState = "loading"
	StateValidating            RuntimeState = "validating"
	StateConnecting            RuntimeState = "connecting"
	StateCompatibilityChecking RuntimeState = "compatibility-checking"
	StateReady                 RuntimeState = "ready"
	StateDraining              RuntimeState = "draining"
	StateStopping              RuntimeState = "stopping"
	StateStopped               RuntimeState = "stopped"
	StateFailed                RuntimeState = "failed"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusUnknown   Status = "unknown"
)

type ReasonCode string

const (
	ReasonStarting              ReasonCode = "runtime_starting"
	ReasonReady                 ReasonCode = "runtime_ready"
	ReasonDraining              ReasonCode = "runtime_draining"
	ReasonStopping              ReasonCode = "runtime_stopping"
	ReasonStopped               ReasonCode = "runtime_stopped"
	ReasonFailed                ReasonCode = "runtime_failed"
	ReasonDatabaseUnavailable   ReasonCode = "database_unavailable"
	ReasonDatabaseTimeout       ReasonCode = "database_timeout"
	ReasonCheckCanceled         ReasonCode = "health_check_canceled"
	ReasonSchemaIncompatible    ReasonCode = "schema_incompatible"
	ReasonPrivilegeIncompatible ReasonCode = "privilege_incompatible"
	ReasonFailureGrace          ReasonCode = "readiness_failure_grace"
	ReasonStale                 ReasonCode = "readiness_stale"
	ReasonInternal              ReasonCode = "runtime_internal"
)

// Signal is an immutable health value.
type Signal struct {
	status Status
	reason ReasonCode
}

func (signal Signal) Status() Status         { return signal.status }
func (signal Signal) ReasonCode() ReasonCode { return signal.reason }

// HealthSnapshot is an immutable point-in-time health view.
type HealthSnapshot struct {
	observedAt                       time.Time
	runtimeState                     RuntimeState
	liveness                         Signal
	readiness                        Signal
	lastSuccessfulDatabaseCheck      time.Time
	lastSuccessfulCompatibilityCheck time.Time
	consecutiveReadinessFailures     int
}

func (snapshot HealthSnapshot) ObservedAt() time.Time      { return snapshot.observedAt }
func (snapshot HealthSnapshot) RuntimeState() RuntimeState { return snapshot.runtimeState }
func (snapshot HealthSnapshot) Liveness() Signal           { return snapshot.liveness }
func (snapshot HealthSnapshot) Readiness() Signal          { return snapshot.readiness }
func (snapshot HealthSnapshot) LastSuccessfulDatabaseCheck() time.Time {
	return snapshot.lastSuccessfulDatabaseCheck
}
func (snapshot HealthSnapshot) LastSuccessfulCompatibilityCheck() time.Time {
	return snapshot.lastSuccessfulCompatibilityCheck
}
func (snapshot HealthSnapshot) ConsecutiveReadinessFailures() int {
	return snapshot.consecutiveReadinessFailures
}
