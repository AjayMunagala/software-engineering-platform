package app

import (
	"context"
	"sync"
	"time"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
	runtimeobservability "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/observability"
	runtimepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
)

type ShutdownOutcome string

const (
	ShutdownGraceful ShutdownOutcome = "graceful"
	ShutdownCanceled ShutdownOutcome = "work_canceled"
	ShutdownForced   ShutdownOutcome = "forced"
)

// ShutdownResult is immutable and contains bounded operational counts only.
type ShutdownResult struct {
	outcome         ShutdownOutcome
	startedAt       time.Time
	finishedAt      time.Time
	initialInFlight int
	canceledWork    int
	resourcesClosed bool
}

func (result ShutdownResult) Outcome() ShutdownOutcome { return result.outcome }
func (result ShutdownResult) StartedAt() time.Time     { return result.startedAt }
func (result ShutdownResult) FinishedAt() time.Time    { return result.finishedAt }
func (result ShutdownResult) Duration() time.Duration  { return result.finishedAt.Sub(result.startedAt) }
func (result ShutdownResult) InitialInFlight() int     { return result.initialInFlight }
func (result ShutdownResult) CanceledWork() int        { return result.canceledWork }
func (result ShutdownResult) ResourcesClosed() bool    { return result.resourcesClosed }

type admittedWork struct {
	context context.Context
	done    func()
	once    sync.Once
}

func (work *admittedWork) Context() context.Context {
	if work == nil {
		return context.Background()
	}
	return work.context
}

func (work *admittedWork) Done() {
	if work != nil {
		work.once.Do(work.done)
	}
}

// Runtime is the sole application owner of PostgreSQL resources, admission,
// health, drain, and shutdown completion.
type Runtime struct {
	configuration runtimeconfig.RuntimeConfig
	postgres      PostgreSQLRuntime
	health        runtimehealth.Monitor
	observability runtimeobservability.Service

	healthObservabilityMutex sync.Mutex
	lastReadiness            runtimehealth.Status

	mutex        sync.Mutex
	accepting    bool
	nextWorkID   uint64
	inFlight     map[uint64]context.CancelFunc
	zeroWork     chan struct{}
	shutdownOnce sync.Once
	shutdownDone chan struct{}
	forceOnce    sync.Once
	force        chan struct{}
	result       ShutdownResult
	shutdownErr  error
}

func (runtime *Runtime) Configuration() runtimeconfig.SafeView {
	if runtime == nil {
		return runtimeconfig.SafeView{}
	}
	return runtime.configuration.SafeView()
}

func (runtime *Runtime) Ingest() runtimepostgres.IngestCapabilities {
	if runtime == nil || runtime.postgres == nil {
		return nil
	}
	return runtime.postgres.Ingest()
}

func (runtime *Runtime) Read() runtimepostgres.ReadCapabilities {
	if runtime == nil || runtime.postgres == nil {
		return nil
	}
	return runtime.postgres.Read()
}

func (runtime *Runtime) Retention() runtimepostgres.RetentionCapabilities {
	if runtime == nil || runtime.postgres == nil {
		return nil
	}
	return runtime.postgres.Retention()
}

func (runtime *Runtime) State() runtimehealth.RuntimeState {
	if runtime == nil || runtime.health == nil {
		return runtimehealth.StateFailed
	}
	return runtime.health.Snapshot().RuntimeState()
}

func (runtime *Runtime) InFlight() int {
	if runtime == nil {
		return 0
	}
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	return len(runtime.inFlight)
}
