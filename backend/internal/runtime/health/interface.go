// Package health implements callable, transport-free liveness and readiness
// semantics. It owns no pool, SQL, listener, logger, metric sink, or process
// signal handling.
package health

import "context"

// ContractVersion identifies the frozen Runtime Health contract.
const ContractVersion = "1.0.0"

// DatabaseChecker is the opaque read-only capability supplied by the
// PostgreSQL runtime. Implementations expose no pool internals.
type DatabaseChecker interface {
	Check(context.Context) error
}

// Evaluator exposes callable health only. Network projection is deferred.
type Evaluator interface {
	Liveness(context.Context) HealthSnapshot
	Readiness(context.Context) HealthSnapshot
	Snapshot() HealthSnapshot
}

// StateRecorder is used only by the lifecycle owner.
type StateRecorder interface {
	Transition(RuntimeState) error
}

// Monitor composes lifecycle recording and health evaluation.
type Monitor interface {
	Evaluator
	StateRecorder
}
